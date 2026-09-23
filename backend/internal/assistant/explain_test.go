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
		name string
		err  error
		want string
	}{
		{"silent", nil, "não respondeu nada"},
		{"canceled", context.Canceled, "interrompeu"},
		{"timeout", context.DeadlineExceeded, "demorou demais"},
		{"not configured", errAgentNotConfigured, "Configure o endereço"},
		{"down", &agentUnreachableError{URL: "http://localhost:8642", Err: errors.New("connection refused")}, "http://localhost:8642"},
		{"bad token", &apiError{Host: "localhost:8642", Status: 401, Message: "no"}, "recusou o token"},
		{"not found", &apiError{Host: "localhost:8642", Status: 404, Message: "no"}, "chatCompletions"},
		{"server error", &apiError{Host: "localhost:8642", Status: 500, Message: "boom"}, "respondeu 500"},
		{"unexpected", errors.New("boom"), "erro inesperado"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := explainFailure(tc.err)
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
		{ErrNoProvider, "Nenhum agente"},
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
