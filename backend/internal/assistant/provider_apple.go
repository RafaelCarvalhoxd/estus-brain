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
		// Not SupportsImageGen: apple-bridge has a /generate-image route
		// (Images.swift) using Image Playground, but ImageCreator refuses to
		// run in a plain background process — confirmed live, not assumed.
		// generate_image still tries it as a last resort when no OpenAI key
		// is set, and fails with a clear reason; this flag just doesn't
		// promise something that doesn't work today.
		Capabilities: Capabilities{SupportsVoice: true},
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

func (p *appleProvider) Chat(ctx context.Context, req ChatRequest, emit func(Event)) (ChatOutcome, error) {
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
	for _, t := range p.chat.toolsFor(p.ID(), req.Module, req.Message, true) {
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
