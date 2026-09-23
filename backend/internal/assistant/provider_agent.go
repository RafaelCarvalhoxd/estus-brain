package assistant

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// The one engine: an external agent (Hermes Agent, OpenClaw…) reached over
// the OpenAI chat format. It runs its own model and reaches Estus's tools
// through /mcp, so this side only relays the conversation and its answer.

const agentID = "agent"

var errAgentNotConfigured = errors.New("agente externo sem endereço configurado")

// agentConfig is where the agent lives. An empty Model means "the first one
// the agent lists", since each agent names its models its own way.
type agentConfig struct {
	URL, Token, Model string
}

// agentUnreachableError is a failure to connect at all, kept apart from an
// HTTP error so the chat can say "is it running?" with the address.
type agentUnreachableError struct {
	URL string
	Err error
}

func (e *agentUnreachableError) Error() string { return fmt.Sprintf("agente em %s: %v", e.URL, e.Err) }
func (e *agentUnreachableError) Unwrap() error { return e.Err }

// agentStreamError is an error the agent reported inside a 200 reply — mid
// stream or as its whole JSON body — rather than as an HTTP status.
type agentStreamError struct {
	Message string
}

func (e *agentStreamError) Error() string { return "o agente devolveu um erro: " + e.Message }

// agentErrorMessage reads an OpenAI-style "error" field, which agents send
// as {"message": …} or as a bare string.
func agentErrorMessage(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var obj struct {
		Message string `json:"message"`
	}
	if json.Unmarshal(raw, &obj) == nil && obj.Message != "" {
		return obj.Message
	}
	var str string
	if json.Unmarshal(raw, &str) == nil && str != "" {
		return str
	}
	return truncate(string(raw), 300)
}

type agentProvider struct {
	config func(ctx context.Context) (agentConfig, error)
	client *http.Client
}

func (p *agentProvider) Status(ctx context.Context) ProviderStatus {
	st := ProviderStatus{ID: agentID, Name: "Agente externo"}
	cfg, err := p.config(ctx)
	if err != nil {
		st.Detail = err.Error()
		return st
	}
	st.Model = cfg.Model
	if cfg.URL == "" {
		st.Detail = "Configure o endereço do agente"
		return st
	}
	models, err := p.models(ctx, cfg)
	if err != nil {
		st.Detail = agentStatusDetail(cfg.URL, err)
		return st
	}
	st.Available, st.Models = true, models
	st.Detail = "Conectado a " + cfg.URL
	return st
}

func agentStatusDetail(url string, err error) string {
	var api *apiError
	switch {
	case errors.As(err, &api) && (api.Status == http.StatusUnauthorized || api.Status == http.StatusForbidden):
		return "O agente recusou o token"
	case errors.As(err, &api):
		return fmt.Sprintf("O agente respondeu %d", api.Status)
	default:
		return "Agente não respondeu em " + url
	}
}

func (p *agentProvider) do(ctx context.Context, cfg agentConfig, method, path string, body []byte) (*http.Response, error) {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(cfg.URL, "/")+path, reader)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "text/event-stream")
	}
	if cfg.Token != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.Token)
	}
	res, err := p.client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, err
		}
		return nil, &agentUnreachableError{URL: cfg.URL, Err: err}
	}
	if res.StatusCode >= 300 {
		defer res.Body.Close()
		raw, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
		return nil, &apiError{Host: hostOf(cfg.URL), Status: res.StatusCode, Message: truncate(string(raw), 300)}
	}
	return res, nil
}

func (p *agentProvider) models(ctx context.Context, cfg agentConfig) ([]string, error) {
	res, err := p.do(ctx, cfg, http.MethodGet, "/v1/models", nil)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	var list struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&list); err != nil {
		return nil, fmt.Errorf("lista de modelos do agente: %w", err)
	}
	out := make([]string, 0, len(list.Data))
	for _, m := range list.Data {
		out = append(out, m.ID)
	}
	return out, nil
}

func (p *agentProvider) Chat(ctx context.Context, req ChatRequest, emit func(Event)) (ChatOutcome, error) {
	cfg, err := p.config(ctx)
	if err != nil {
		return ChatOutcome{}, err
	}
	if cfg.URL == "" {
		return ChatOutcome{}, errAgentNotConfigured
	}
	if cfg.Model == "" {
		models, err := p.models(ctx, cfg)
		if err != nil {
			return ChatOutcome{}, err
		}
		if len(models) == 0 {
			return ChatOutcome{}, fmt.Errorf("o agente não lista nenhum modelo; informe um nos ajustes")
		}
		cfg.Model = models[0]
	}

	messages := make([]map[string]any, 0, len(req.History)+2)
	if req.System != "" {
		messages = append(messages, map[string]any{"role": "system", "content": req.System})
	}
	for _, m := range req.History {
		messages = append(messages, map[string]any{"role": m.Role, "content": m.Content})
	}
	messages = append(messages, map[string]any{"role": "user", "content": userContentOpenAI(req)})
	body, err := json.Marshal(map[string]any{"model": cfg.Model, "stream": true, "messages": messages})
	if err != nil {
		return ChatOutcome{}, err
	}

	res, err := p.do(ctx, cfg, http.MethodPost, "/v1/chat/completions", body)
	if err != nil {
		return ChatOutcome{}, err
	}
	defer res.Body.Close()

	// Some agents ignore stream:true and answer with one JSON body.
	if strings.HasPrefix(res.Header.Get("Content-Type"), "application/json") {
		var reply struct {
			Choices []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			} `json:"choices"`
			Error json.RawMessage `json:"error"`
		}
		if err := json.NewDecoder(io.LimitReader(res.Body, 8<<20)).Decode(&reply); err != nil {
			return ChatOutcome{}, fmt.Errorf("resposta do agente: %w", err)
		}
		if msg := agentErrorMessage(reply.Error); msg != "" {
			return ChatOutcome{}, &agentStreamError{Message: msg}
		}
		if len(reply.Choices) == 0 || reply.Choices[0].Message.Content == "" {
			return ChatOutcome{}, nil
		}
		text := reply.Choices[0].Message.Content
		emit(Event{Type: "text", Text: text})
		return ChatOutcome{Text: text}, nil
	}

	var text strings.Builder
	scanner := bufio.NewScanner(res.Body)
	scanner.Buffer(make([]byte, 64<<10), 4<<20)
	for scanner.Scan() {
		// Only "data:" lines carry OpenAI chunks; "event:" lines, keep-alive
		// comments and an agent's own events (hermes.tool.progress) are skipped.
		data, ok := strings.CutPrefix(scanner.Text(), "data:")
		if !ok {
			continue
		}
		data = strings.TrimSpace(data)
		if data == "[DONE]" {
			break
		}
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
			Error json.RawMessage `json:"error"`
		}
		if json.Unmarshal([]byte(data), &chunk) != nil {
			continue
		}
		if msg := agentErrorMessage(chunk.Error); msg != "" {
			return ChatOutcome{Text: text.String()}, &agentStreamError{Message: msg}
		}
		if len(chunk.Choices) == 0 || chunk.Choices[0].Delta.Content == "" {
			continue
		}
		piece := chunk.Choices[0].Delta.Content
		text.WriteString(piece)
		emit(Event{Type: "text", Text: piece})
	}
	return ChatOutcome{Text: text.String()}, scanner.Err()
}
