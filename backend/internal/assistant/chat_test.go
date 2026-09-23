package assistant

import (
	"errors"
	"reflect"
	"testing"

	"github.com/rafael/estus-vault/backend/internal/domain"
	"github.com/rafael/estus-vault/backend/internal/store/postgres"
)

func TestMergeAgentConfig(t *testing.T) {
	env := agentConfig{URL: "http://env:8642", Token: "env-tok", Model: "env-model"}
	cases := []struct {
		name  string
		saved agentConfig
		want  agentConfig
	}{
		{"nothing saved", agentConfig{}, env},
		{"all saved", agentConfig{URL: "http://s", Token: "s-tok", Model: "s-model"}, agentConfig{URL: "http://s", Token: "s-tok", Model: "s-model"}},
		{"only url saved", agentConfig{URL: "http://s"}, agentConfig{URL: "http://s", Token: "env-tok", Model: "env-model"}},
		{"only token saved", agentConfig{Token: "s-tok"}, agentConfig{URL: "http://env:8642", Token: "s-tok", Model: "env-model"}},
		{"only model saved", agentConfig{Model: "s-model"}, agentConfig{URL: "http://env:8642", Token: "env-tok", Model: "s-model"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := mergeAgentConfig(tc.saved, env); got != tc.want {
				t.Fatalf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestSecretRoundTrip(t *testing.T) {
	var key [32]byte
	copy(key[:], "0123456789abcdef0123456789abcdef")
	blob, err := sealSecret(key, "segredo")
	if err != nil {
		t.Fatal(err)
	}
	if got := openSecret(&key, blob); got != "segredo" {
		t.Fatalf("openSecret = %q", got)
	}
	if got := openSecret(nil, blob); got != "" {
		t.Fatalf("without key = %q, want empty", got)
	}
	var other [32]byte
	if got := openSecret(&other, blob); got != "" {
		t.Fatalf("wrong key = %q, want empty", got)
	}
	if got := openSecret(&key, "não é base64"); got != "" {
		t.Fatalf("garbage = %q, want empty", got)
	}
}

func TestNormalizeAgentURL(t *testing.T) {
	ok := map[string]string{
		"":                         "",
		"  ":                       "",
		"http://127.0.0.1:8642/":   "http://127.0.0.1:8642",
		" https://agente.local/v ": "https://agente.local/v",
	}
	for in, want := range ok {
		got, err := normalizeAgentURL(in)
		if err != nil || got != want {
			t.Errorf("normalizeAgentURL(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, in := range []string{"127.0.0.1:8642", "ftp://host", "http://", "localhost", "javascript:alert(1)"} {
		if _, err := normalizeAgentURL(in); !errors.Is(err, domain.ErrValidation) {
			t.Errorf("normalizeAgentURL(%q) err = %v, want ErrValidation", in, err)
		}
	}
}

func TestAgentHostChanged(t *testing.T) {
	cases := []struct {
		old, new string
		want     bool
	}{
		{"http://127.0.0.1:8642", "http://127.0.0.1:8642/v1", false},
		{"http://127.0.0.1:8642", "https://127.0.0.1:8642", false},
		{"http://127.0.0.1:8642", "http://127.0.0.1:9000", true},
		{"http://a.local", "http://b.local", true},
		{"", "http://a.local", true},
		{"http://a.local", "", true},
		{"", "", false},
	}
	for _, tc := range cases {
		if got := agentHostChanged(tc.old, tc.new); got != tc.want {
			t.Errorf("agentHostChanged(%q, %q) = %v, want %v", tc.old, tc.new, got, tc.want)
		}
	}
}

func TestBuildHistory(t *testing.T) {
	m := func(role, content string) postgres.ConversationMessage {
		return postgres.ConversationMessage{Role: role, Content: content}
	}
	cases := []struct {
		name        string
		msgs        []postgres.ConversationMessage
		wantHistory []Message
		wantMessage string
	}{
		{"empty", nil, nil, "nova"},
		{"alternating", []postgres.ConversationMessage{m("user", "oi"), m("assistant", "olá")},
			[]Message{{"user", "oi"}, {"assistant", "olá"}}, "nova"},
		{"leading assistant dropped", []postgres.ConversationMessage{m("assistant", "bem-vindo"), m("user", "oi"), m("assistant", "olá")},
			[]Message{{"user", "oi"}, {"assistant", "olá"}}, "nova"},
		{"same side merged, blanks skipped", []postgres.ConversationMessage{m("user", "a"), m("user", "b"), m("assistant", " "), m("assistant", "c"), m("assistant", "d")},
			[]Message{{"user", "a\n\nb"}, {"assistant", "c\n\nd"}}, "nova"},
		{"trailing user folded into message", []postgres.ConversationMessage{m("user", "oi"), m("assistant", "olá"), m("user", "sem resposta")},
			[]Message{{"user", "oi"}, {"assistant", "olá"}}, "sem resposta\n\nnova"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			history, message := buildHistory(tc.msgs, "nova")
			if !reflect.DeepEqual(history, tc.wantHistory) || message != tc.wantMessage {
				t.Fatalf("got %+v, %q; want %+v, %q", history, message, tc.wantHistory, tc.wantMessage)
			}
		})
	}
}
