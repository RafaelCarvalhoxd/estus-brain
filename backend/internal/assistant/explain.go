package assistant

import (
	"context"
	"errors"
	"fmt"

	"github.com/rafael/estus-vault/backend/internal/domain"
)

// Explanation is a failed answer put in words the person can act on — what
// went wrong and what to do — with the raw error kept as Detail for the
// "detalhes técnicos" disclosure.
type Explanation struct {
	Message string
	Detail  string
}

const settingsHint = "nos ajustes do chat"

// explainFailure says why the agent gave no answer; err is nil when it
// answered with nothing at all.
func explainFailure(err error) Explanation {
	if err == nil {
		return Explanation{Message: "O agente não respondeu nada. Tente reformular o pedido."}
	}
	return Explanation{Message: describeError(err), Detail: err.Error()}
}

func describeError(err error) string {
	var down *agentUnreachableError
	var api *apiError
	switch {
	case errors.Is(err, context.Canceled):
		return "Você interrompeu a resposta antes de ela terminar."
	case errors.Is(err, context.DeadlineExceeded):
		return "O agente demorou demais para responder e a resposta foi cancelada. Tente de novo."
	case errors.Is(err, errAgentNotConfigured):
		return "Configure o endereço do agente " + settingsHint + "."
	case errors.As(err, &down):
		return fmt.Sprintf("O agente não respondeu em %s. Confira se ele está rodando e se o endereço %s está certo.", down.URL, settingsHint)
	case errors.As(err, &api) && (api.Status == 401 || api.Status == 403):
		return "O agente recusou o token. Confira o token " + settingsHint + "."
	case errors.As(err, &api) && api.Status == 404:
		return "O agente não aceitou o pedido (404). No OpenClaw, ligue o endpoint chatCompletions do Gateway; confira também o modelo " + settingsHint + "."
	case errors.As(err, &api):
		return fmt.Sprintf("O agente respondeu %d. Veja os detalhes técnicos.", api.Status)
	}
	return "O agente não conseguiu responder por um erro inesperado. Tente de novo; se continuar, veja os detalhes técnicos."
}

// SendErrorMessage explains an error Send returned before any answer started:
// no agent, a deleted conversation, a bad message, or the server itself.
func SendErrorMessage(err error) string {
	switch {
	case errors.Is(err, ErrNoProvider):
		return "Nenhum agente ligado. Configure o agente nos ajustes do chat."
	case errors.Is(err, domain.ErrValidation):
		return "Não deu para enviar: " + ToolErrorMessage(err) + "."
	case errors.Is(err, domain.ErrNotFound):
		return "Esta conversa não existe mais (pode ter sido apagada). Comece uma nova conversa e mande de novo."
	default:
		return "Não consegui responder por um problema no servidor do Estus (o banco de dados pode estar fora do ar). Tente de novo em instantes."
	}
}
