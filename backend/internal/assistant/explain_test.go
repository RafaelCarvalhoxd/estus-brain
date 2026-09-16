package assistant

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/rafael/estus-vault/backend/internal/domain"
)

func TestExplainFailure(t *testing.T) {
	cases := []struct {
		name     string
		provider string
		err      error
		outcome  ChatOutcome
		want     string // a fragment the explanation must carry
	}{
		{"missing key", "anthropic", errors.New("sem API key da Anthropic"), ChatOutcome{}, "Falta a chave de API"},
		{"rejected key", "openai", &apiError{Host: "api.openai.com", Status: 401, Message: `{"code":"invalid_api_key"}`}, ChatOutcome{}, "recusou a chave"},
		{"no credits", "anthropic", &apiError{Host: "api.anthropic.com", Status: 400, Message: `{"message":"Your credit balance is too low"}`}, ChatOutcome{}, "créditos"},
		{"rate limited", "openai", &apiError{Host: "api.openai.com", Status: 429, Message: `{"message":"Rate limit reached"}`}, ChatOutcome{}, "Muitos pedidos"},
		{"overloaded", "anthropic", &apiError{Host: "api.anthropic.com", Status: 529, Message: `{"type":"overloaded_error"}`}, ChatOutcome{}, "instável"},
		{"unknown model", "ollama", &apiError{Host: "localhost:11434", Status: 404, Message: `"model 'llama9' not found"`}, ChatOutcome{}, "modelo"},
		{"timeout", "codex", fmt.Errorf("Codex falhou: %s", "demorou demais para responder"), ChatOutcome{}, "demorou demais"},
		{"offline ollama", "ollama", errors.New(`Post "http://localhost:11434/api/chat": dial tcp [::1]:11434: connect: connection refused`), ChatOutcome{}, "Ollama"},
		{"cli not logged in", "claude_code", errors.New("Claude Code: Invalid API key · Please run /login"), ChatOutcome{}, "login"},
		{"subscription limit", "claude_code", errors.New("Claude Code: Claude AI usage limit reached|1757955600"), ChatOutcome{}, "limite de uso"},
		{"apple disabled", "apple", errors.New("Apple Intelligence indisponível: appleIntelligenceNotEnabled"), ChatOutcome{}, "Ajustes do Sistema"},
		{"apple lost", "apple", fmt.Errorf("Apple Intelligence: %w", &apiError{Host: "127.0.0.1:8765", Status: 500, Message: `"Erro inesperado: The operation couldn’t be completed. (FoundationModels.LanguageModelSession.GenerationError error -1.)"`}), ChatOutcome{}, "não conseguiu entender"},
		{"apple context", "apple", fmt.Errorf("Apple Intelligence: %w", &apiError{Host: "127.0.0.1:8765", Status: 422, Message: `"A conversa excedeu a janela de contexto do modelo local (~4k tokens)."`}), ChatOutcome{}, "nova conversa"},
		{"guardrail", "apple", fmt.Errorf("Apple Intelligence: %w", &apiError{Host: "127.0.0.1:8765", Status: 422, Message: `"O pedido foi bloqueado pelas proteções de segurança do Apple Intelligence."`}), ChatOutcome{}, "Reformule"},
		{"canceled", "openai", context.Canceled, ChatOutcome{}, "interrompeu"},
		{"silent with failed tool", "apple", nil, ChatOutcome{ToolCalls: []ToolCall{{Name: "finance_create_transaction", Error: `categoria "eletronicos" não encontrada; existentes: Compras, Lazer`}}}, `categoria "eletronicos" não encontrada`},
		{"silent", "openai", nil, ChatOutcome{}, "Não entendi"},
		{"unexpected", "openai", errors.New("boom"), ChatOutcome{}, "erro inesperado"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := explainFailure(tc.provider, "modelo-x", tc.err, tc.outcome)
			if !strings.Contains(got.Message, tc.want) {
				t.Fatalf("message = %q, want it to contain %q", got.Message, tc.want)
			}
			if tc.err != nil && got.Detail != tc.err.Error() {
				t.Fatalf("detail = %q, want the raw error", got.Detail)
			}
		})
	}
}

func TestSendErrorMessage(t *testing.T) {
	cases := []struct {
		err  error
		want string
	}{
		{ErrNoProvider, "Nenhum motor de IA"},
		{fmt.Errorf("%w: a mensagem está vazia", domain.ErrValidation), "Não deu para enviar: a mensagem está vazia."},
		{domain.ErrNotFound, "Esta conversa não existe mais"},
		{errors.New("connection refused"), "problema no servidor do Estus"},
	}
	for _, tc := range cases {
		if got := SendErrorMessage(tc.err); !strings.Contains(got, tc.want) {
			t.Errorf("SendErrorMessage(%v) = %q, want it to contain %q", tc.err, got, tc.want)
		}
	}
}
