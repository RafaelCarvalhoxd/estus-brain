package assistant

import (
	"context"
	"encoding/json"
	"fmt"
)

// Apple Intelligence runs on-device through a small Swift helper
// (apple-bridge/) that speaks HTTP on localhost; the chat starts it on demand
// (bridge.go, shared with the voice client) and it calls tools back through
// /api/assistant/tools. Its model is small, so it only gets the tools of the
// module in use.

type appleProvider struct {
	chat *Chat
}

func (p *appleProvider) ID() string { return "apple" }

func (p *appleProvider) Status(ctx context.Context) ProviderStatus {
	st := ProviderStatus{
		ID: p.ID(), Name: "Apple Intelligence", Kind: "local", Model: "on-device",
		// Image Playground itself was tried (making this process a real
		// foreground app) and pulled back out — unreliable even working, and
		// it took over the Mac's Dock the whole time. SupportsImageGen is
		// still true: generate_image reaches OpenAI directly, the same way
		// it does from every other engine.
		Capabilities: Capabilities{SupportsVoice: true, SupportsImageGen: true},
	}
	h, err := p.chat.bridge.ensure(ctx)
	switch {
	case err != nil:
		st.Detail = err.Error()
	case !h.Available:
		st.Detail = "Indisponível: " + h.Reason
	default:
		st.Available = true
		st.Detail = "Modelo on-device do macOS, privado e offline"
	}
	return st
}

// Chat retries once, restarting the bridge process first, when the first
// attempt fails outright (as opposed to /health reporting the model itself
// unavailable). Confirmed live: the on-device runtime can wedge into
// answering a GenerationError for every request — including a bare "oi",
// nothing to do with content — until the process is restarted; a request
// that really is refused by the content guardrail fails the same way both
// times, so retrying never hides a real refusal, only wastes one round trip
// telling it apart from a wedged process.
func (p *appleProvider) Chat(ctx context.Context, req ChatRequest, emit func(Event)) (ChatOutcome, error) {
	out, err := p.attempt(ctx, req, emit)
	if err == nil {
		return out, nil
	}
	p.chat.bridge.stop()
	return p.attempt(ctx, req, emit)
}

func (p *appleProvider) attempt(ctx context.Context, req ChatRequest, emit func(Event)) (ChatOutcome, error) {
	h, err := p.chat.bridge.ensure(ctx)
	if err != nil {
		return ChatOutcome{}, err
	}
	if !h.Available {
		return ChatOutcome{}, fmt.Errorf("Apple Intelligence indisponível: %s", h.Reason)
	}
	type tool struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Parameters  Schema `json:"parameters"`
	}
	var tools []tool
	for _, t := range p.chat.toolsFor(req.Module, req.Message, true) {
		tools = append(tools, tool{t.Name, t.Description, t.Input})
	}
	messages := append([]Message{}, req.History...)
	if len(messages) > 8 {
		messages = messages[len(messages)-8:]
	}
	messages = append(messages, Message{Role: "user", Content: req.Message})

	var res struct {
		Text      string `json:"text"`
		Error     string `json:"error"`
		ToolCalls []struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
			Result    any             `json:"result"`
			Error     string          `json:"error"`
		} `json:"tool_calls"`
	}
	err = postJSON(ctx, p.chat.bridge.url+"/chat", nil, map[string]any{
		"instructions":  req.System,
		"messages":      messages,
		"tools":         tools,
		"tool_endpoint": p.chat.cfg.ToolsURL,
		"tool_token":    p.chat.cfg.MCPToken,
	}, &res)
	if err != nil {
		return ChatOutcome{}, fmt.Errorf("Apple Intelligence: %w", err)
	}
	var out ChatOutcome
	for i, c := range res.ToolCalls {
		id := fmt.Sprintf("apple_%d", i)
		emit(Event{Type: "tool", ToolID: id, Tool: c.Name, Args: c.Arguments})
		emit(Event{Type: "tool_result", ToolID: id, Tool: c.Name, Result: c.Result, Error: c.Error})
		out.ToolCalls = append(out.ToolCalls, ToolCall{ID: id, Name: c.Name, Args: c.Arguments, Result: c.Result, Error: c.Error})
	}
	out.Text = res.Text
	if res.Text != "" {
		emit(Event{Type: "text", Text: res.Text})
	}
	return out, nil
}
