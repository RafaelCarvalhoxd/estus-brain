package assistant

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// fakeAgent answers like an OpenAI-compatible agent: /v1/models lists one
// model and /v1/chat/completions streams chunks, with a Hermes-style custom
// event in the middle that the provider must skip.
func fakeAgent(t *testing.T, token string, got *map[string]any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+token {
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprint(w, `{"error":{"message":"bad token"}}`)
			return
		}
		switch r.URL.Path {
		case "/v1/models":
			fmt.Fprint(w, `{"data":[{"id":"hermes-agent"},{"id":"outro"}]}`)
		case "/v1/chat/completions":
			body, _ := io.ReadAll(r.Body)
			if got != nil {
				_ = json.Unmarshal(body, got)
			}
			w.Header().Set("Content-Type", "text/event-stream")
			fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"role\":\"assistant\"}}]}\n\n")
			fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"Olá, \"}}]}\n\n")
			fmt.Fprint(w, "event: hermes.tool.progress\ndata: {\"tool\":\"finance_summary\"}\n\n")
			fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"tudo certo.\"}}]}\n\n")
			fmt.Fprint(w, "data: [DONE]\n\n")
		default:
			http.NotFound(w, r)
		}
	}))
}

func agentFor(cfg agentConfig) *agentProvider {
	return &agentProvider{
		config: func(context.Context) (agentConfig, error) { return cfg, nil },
		client: http.DefaultClient,
	}
}

func TestAgentChatStreamsText(t *testing.T) {
	var got map[string]any
	srv := fakeAgent(t, "tok", &got)
	defer srv.Close()

	var events []Event
	out, err := agentFor(agentConfig{URL: srv.URL, Token: "tok", Model: "hermes-agent"}).Chat(context.Background(), ChatRequest{
		System:  "sys",
		History: []Message{{Role: "user", Content: "oi"}, {Role: "assistant", Content: "oi!"}},
		Message: "como estão as contas?",
	}, func(e Event) { events = append(events, e) })
	if err != nil {
		t.Fatal(err)
	}
	if out.Text != "Olá, tudo certo." {
		t.Fatalf("text = %q", out.Text)
	}
	if len(events) != 2 || events[0].Type != "text" || events[1].Text != "tudo certo." {
		t.Fatalf("events = %+v", events)
	}
	if got["model"] != "hermes-agent" || got["stream"] != true {
		t.Fatalf("body = %v", got)
	}
	msgs := got["messages"].([]any)
	if len(msgs) != 4 {
		t.Fatalf("messages = %v, want system + 2 history + new", msgs)
	}
	first := msgs[0].(map[string]any)
	last := msgs[3].(map[string]any)
	if first["role"] != "system" || last["content"] != "como estão as contas?" {
		t.Fatalf("messages = %v", msgs)
	}
}

func TestAgentChatSendsImageAsImageURL(t *testing.T) {
	var got map[string]any
	srv := fakeAgent(t, "tok", &got)
	defer srv.Close()

	_, err := agentFor(agentConfig{URL: srv.URL, Token: "tok", Model: "m"}).Chat(context.Background(), ChatRequest{
		Message:    "o que é isso?",
		Attachment: &Attachment{ImageBase64: "AAAA", ImageMediaType: "image/png"},
	}, func(Event) {})
	if err != nil {
		t.Fatal(err)
	}
	msgs := got["messages"].([]any)
	parts := msgs[len(msgs)-1].(map[string]any)["content"].([]any)
	img := parts[1].(map[string]any)["image_url"].(map[string]any)["url"]
	if img != "data:image/png;base64,AAAA" {
		t.Fatalf("image url = %v", img)
	}
}

func TestAgentChatEmptyModelUsesFirstListed(t *testing.T) {
	var got map[string]any
	srv := fakeAgent(t, "tok", &got)
	defer srv.Close()

	if _, err := agentFor(agentConfig{URL: srv.URL, Token: "tok"}).Chat(context.Background(), ChatRequest{Message: "oi"}, func(Event) {}); err != nil {
		t.Fatal(err)
	}
	if got["model"] != "hermes-agent" {
		t.Fatalf("model = %v, want the first from /v1/models", got["model"])
	}
}

func TestAgentChatErrors(t *testing.T) {
	srv := fakeAgent(t, "tok", nil)
	defer srv.Close()
	closed := httptest.NewServer(http.NotFoundHandler())
	closedURL := closed.URL
	closed.Close()

	cases := []struct {
		name  string
		cfg   agentConfig
		check func(error) bool
	}{
		{"not configured", agentConfig{}, func(err error) bool { return errors.Is(err, errAgentNotConfigured) }},
		{"bad token", agentConfig{URL: srv.URL, Token: "errado", Model: "m"}, func(err error) bool {
			var api *apiError
			return errors.As(err, &api) && api.Status == http.StatusUnauthorized
		}},
		{"unreachable", agentConfig{URL: closedURL, Model: "m"}, func(err error) bool {
			var down *agentUnreachableError
			return errors.As(err, &down) && down.URL == closedURL
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := agentFor(tc.cfg).Chat(context.Background(), ChatRequest{Message: "oi"}, func(Event) {})
			if err == nil || !tc.check(err) {
				t.Fatalf("err = %v", err)
			}
		})
	}
}

func TestAgentStatus(t *testing.T) {
	srv := fakeAgent(t, "tok", nil)
	defer srv.Close()

	ok := agentFor(agentConfig{URL: srv.URL, Token: "tok"}).Status(context.Background())
	if !ok.Available || strings.Join(ok.Models, ",") != "hermes-agent,outro" || !strings.Contains(ok.Detail, srv.URL) {
		t.Fatalf("status = %+v", ok)
	}
	bad := agentFor(agentConfig{URL: srv.URL, Token: "errado"}).Status(context.Background())
	if bad.Available || !strings.Contains(bad.Detail, "token") {
		t.Fatalf("status = %+v", bad)
	}
	none := agentFor(agentConfig{}).Status(context.Background())
	if none.Available || !strings.Contains(none.Detail, "endereço") {
		t.Fatalf("status = %+v", none)
	}
}
