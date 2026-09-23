package assistant

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
)

// The engines reached over HTTP: Anthropic and OpenAI with an API key, and
// Ollama running locally. Each runs the tool loop itself, against the same
// registry the MCP server exposes.

const maxToolRounds = 10

// userContentAnthropic is req.Message alone, or — when an attachment carried
// real image bytes (see resolveAttachment) — Anthropic's block form: an
// image block plus a text block, which is the only shape its API accepts
// once a message has more than plain text.
func userContentAnthropic(req ChatRequest) any {
	if req.Attachment == nil || req.Attachment.ImageBase64 == "" {
		return req.Message
	}
	type block = map[string]any
	return []block{
		{"type": "image", "source": block{"type": "base64", "media_type": req.Attachment.ImageMediaType, "data": req.Attachment.ImageBase64}},
		{"type": "text", "text": req.Message},
	}
}

// ---------------------------------------------------------------- Anthropic

type anthropicProvider struct{ chat *Chat }

func (p *anthropicProvider) ID() string { return "anthropic" }

func (p *anthropicProvider) key(ctx context.Context) string {
	return p.chat.apiKey(ctx, "anthropic", os.Getenv("ANTHROPIC_API_KEY"))
}

func (p *anthropicProvider) Status(ctx context.Context) ProviderStatus {
	s, _ := p.chat.repo.Settings(ctx)
	has := p.key(ctx) != ""
	st := ProviderStatus{
		ID: p.ID(), Name: "Claude (API key)", Kind: "api", NeedsKey: true, HasKey: has, Available: has, Model: p.chat.model(s, p.ID()),
		Capabilities: Capabilities{SupportsVision: true, SupportsImageGen: true},
	}
	if has {
		st.Detail = "Chave configurada"
	} else {
		st.Detail = "Informe uma API key da Anthropic"
	}
	return st
}

func (p *anthropicProvider) Chat(ctx context.Context, req ChatRequest, emit func(Event)) (ChatOutcome, error) {
	key := p.key(ctx)
	if key == "" {
		return ChatOutcome{}, fmt.Errorf("sem API key da Anthropic")
	}
	type block = map[string]any
	var tools []block
	for _, t := range p.chat.toolsFor(req.Module, req.Message, false) {
		tools = append(tools, block{"name": t.Name, "description": t.Description, "input_schema": t.Input})
	}
	messages := []block{}
	for _, m := range req.History {
		messages = append(messages, block{"role": m.Role, "content": m.Content})
	}
	messages = append(messages, block{"role": "user", "content": userContentAnthropic(req)})

	var out ChatOutcome
	var texts []string
	for round := 0; round < maxToolRounds; round++ {
		var res struct {
			Content []struct {
				Type  string          `json:"type"`
				Text  string          `json:"text"`
				ID    string          `json:"id"`
				Name  string          `json:"name"`
				Input json.RawMessage `json:"input"`
			} `json:"content"`
			StopReason string `json:"stop_reason"`
		}
		err := postJSON(ctx, "https://api.anthropic.com/v1/messages", map[string]string{
			"x-api-key":         key,
			"anthropic-version": "2023-06-01",
		}, block{"model": req.Model, "max_tokens": 4096, "system": req.System, "tools": tools, "messages": messages}, &res)
		if err != nil {
			out.Text = strings.Join(texts, "\n\n")
			return out, err
		}
		var assistantBlocks []block
		var results []block
		for _, c := range res.Content {
			switch c.Type {
			case "text":
				if strings.TrimSpace(c.Text) != "" {
					emit(Event{Type: "text", Text: c.Text})
					texts = append(texts, c.Text)
				}
				assistantBlocks = append(assistantBlocks, block{"type": "text", "text": c.Text})
			case "tool_use":
				assistantBlocks = append(assistantBlocks, block{"type": "tool_use", "id": c.ID, "name": c.Name, "input": c.Input})
				result := p.chat.runTool(ctx, c.ID, c.Name, c.Input, emit, &out.ToolCalls)
				results = append(results, block{"type": "tool_result", "tool_use_id": c.ID, "content": result})
			}
		}
		if res.StopReason != "tool_use" || len(results) == 0 {
			break
		}
		messages = append(messages, block{"role": "assistant", "content": assistantBlocks}, block{"role": "user", "content": results})
	}
	out.Text = strings.Join(texts, "\n\n")
	return out, nil
}

// ---------------------------------------------------------------- OpenAI-style

// openAIChat runs the chat-completions tool loop shared by OpenAI and Ollama.
// Ollama's native /api/chat has the same message shape, except that it takes
// tool arguments as objects and returns them that way too.
func (c *Chat) openAIChat(ctx context.Context, req ChatRequest, emit func(Event), small bool, call func(body map[string]any) (openAIMessage, error)) (ChatOutcome, error) {
	type msg = map[string]any
	var tools []msg
	for _, t := range c.toolsFor(req.Module, req.Message, small) {
		tools = append(tools, msg{"type": "function", "function": msg{"name": t.Name, "description": t.Description, "parameters": t.Input}})
	}
	messages := []msg{{"role": "system", "content": req.System}}
	for _, m := range req.History {
		messages = append(messages, msg{"role": m.Role, "content": m.Content})
	}
	messages = append(messages, msg{"role": "user", "content": userContentOpenAI(req)})

	var out ChatOutcome
	var texts []string
	for round := 0; round < maxToolRounds; round++ {
		reply, err := call(msg{"model": req.Model, "messages": messages, "tools": tools})
		if err != nil {
			out.Text = strings.Join(texts, "\n\n")
			return out, err
		}
		if strings.TrimSpace(reply.Content) != "" {
			emit(Event{Type: "text", Text: reply.Content})
			texts = append(texts, reply.Content)
		}
		if len(reply.ToolCalls) == 0 {
			break
		}
		assistant := msg{"role": "assistant", "content": reply.Content, "tool_calls": reply.rawToolCalls}
		messages = append(messages, assistant)
		for i, tc := range reply.ToolCalls {
			id := tc.ID
			if id == "" {
				id = fmt.Sprintf("call_%d_%d", round, i)
			}
			result := c.runTool(ctx, id, tc.Name, tc.Args, emit, &out.ToolCalls)
			messages = append(messages, msg{"role": "tool", "tool_call_id": id, "tool_name": tc.Name, "content": result})
		}
	}
	out.Text = strings.Join(texts, "\n\n")
	return out, nil
}

type openAIToolCall struct {
	ID   string
	Name string
	Args json.RawMessage
}

type openAIMessage struct {
	Content      string
	ToolCalls    []openAIToolCall
	rawToolCalls json.RawMessage
}

type openAIProvider struct{ chat *Chat }

func (p *openAIProvider) ID() string { return "openai" }

func (p *openAIProvider) key(ctx context.Context) string {
	return p.chat.apiKey(ctx, "openai", os.Getenv("OPENAI_API_KEY"))
}

func (p *openAIProvider) Status(ctx context.Context) ProviderStatus {
	s, _ := p.chat.repo.Settings(ctx)
	has := p.key(ctx) != ""
	st := ProviderStatus{
		ID: p.ID(), Name: "GPT (API key)", Kind: "api", NeedsKey: true, HasKey: has, Available: has, Model: p.chat.model(s, p.ID()),
		Capabilities: Capabilities{SupportsImageGen: true, SupportsVision: true},
	}
	if has {
		st.Detail = "Chave configurada"
	} else {
		st.Detail = "Informe uma API key da OpenAI"
	}
	return st
}

func (p *openAIProvider) Chat(ctx context.Context, req ChatRequest, emit func(Event)) (ChatOutcome, error) {
	key := p.key(ctx)
	if key == "" {
		return ChatOutcome{}, fmt.Errorf("sem API key da OpenAI")
	}
	return p.chat.openAIChat(ctx, req, emit, false, func(body map[string]any) (openAIMessage, error) {
		var res struct {
			Choices []struct {
				Message struct {
					Content   string          `json:"content"`
					ToolCalls json.RawMessage `json:"tool_calls"`
				} `json:"message"`
			} `json:"choices"`
		}
		if err := postJSON(ctx, "https://api.openai.com/v1/chat/completions", map[string]string{"Authorization": "Bearer " + key}, body, &res); err != nil {
			return openAIMessage{}, err
		}
		if len(res.Choices) == 0 {
			return openAIMessage{}, fmt.Errorf("resposta vazia da OpenAI")
		}
		m := res.Choices[0].Message
		out := openAIMessage{Content: m.Content, rawToolCalls: m.ToolCalls}
		var calls []struct {
			ID       string `json:"id"`
			Function struct {
				Name      string `json:"name"`
				Arguments string `json:"arguments"`
			} `json:"function"`
		}
		_ = json.Unmarshal(m.ToolCalls, &calls)
		for _, c := range calls {
			out.ToolCalls = append(out.ToolCalls, openAIToolCall{c.ID, c.Function.Name, json.RawMessage(c.Function.Arguments)})
		}
		return out, nil
	})
}

// ---------------------------------------------------------------- Ollama

type ollamaProvider struct{ chat *Chat }

func (p *ollamaProvider) ID() string { return "ollama" }

func (p *ollamaProvider) baseURL(ctx context.Context) string {
	s, err := p.chat.repo.Settings(ctx)
	if err != nil || s.OllamaURL == "" {
		return "http://127.0.0.1:11434"
	}
	return s.OllamaURL
}

func (p *ollamaProvider) models(ctx context.Context) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL(ctx)+"/api/tags", nil)
	if err != nil {
		return nil, err
	}
	res, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	var tags struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.NewDecoder(res.Body).Decode(&tags); err != nil {
		return nil, err
	}
	var out []string
	for _, m := range tags.Models {
		out = append(out, m.Name)
	}
	return out, nil
}

func (p *ollamaProvider) Status(ctx context.Context) ProviderStatus {
	s, _ := p.chat.repo.Settings(ctx)
	st := ProviderStatus{
		ID: p.ID(), Name: "Ollama (local)", Kind: "local", Model: p.chat.model(s, p.ID()),
		Capabilities: Capabilities{SupportsImageGen: true},
	}
	models, err := p.models(ctx)
	if err != nil {
		st.Detail = "Ollama não está rodando em " + p.baseURL(ctx)
		return st
	}
	st.Models = models
	if len(models) == 0 {
		st.Detail = "Nenhum modelo baixado (ex.: ollama pull qwen3)"
		return st
	}
	if st.Model == "" {
		st.Model = models[0]
	}
	st.Available = true
	st.Detail = fmt.Sprintf("%d modelo(s) disponível(is)", len(models))
	return st
}

func (p *ollamaProvider) Chat(ctx context.Context, req ChatRequest, emit func(Event)) (ChatOutcome, error) {
	if req.Model == "" {
		models, err := p.models(ctx)
		if err != nil || len(models) == 0 {
			return ChatOutcome{}, fmt.Errorf("Ollama sem modelos disponíveis")
		}
		req.Model = models[0]
	}
	base := p.baseURL(ctx)
	return p.chat.openAIChat(ctx, req, emit, true, func(body map[string]any) (openAIMessage, error) {
		body["stream"] = false
		var res struct {
			Message struct {
				Content   string          `json:"content"`
				ToolCalls json.RawMessage `json:"tool_calls"`
			} `json:"message"`
		}
		if err := postJSON(ctx, base+"/api/chat", nil, body, &res); err != nil {
			return openAIMessage{}, err
		}
		out := openAIMessage{Content: res.Message.Content, rawToolCalls: res.Message.ToolCalls}
		var calls []struct {
			Function struct {
				Name      string          `json:"name"`
				Arguments json.RawMessage `json:"arguments"`
			} `json:"function"`
		}
		_ = json.Unmarshal(res.Message.ToolCalls, &calls)
		for _, c := range calls {
			out.ToolCalls = append(out.ToolCalls, openAIToolCall{"", c.Function.Name, c.Function.Arguments})
		}
		return out, nil
	})
}
