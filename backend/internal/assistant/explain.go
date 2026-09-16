package assistant

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/rafael/estus-vault/backend/internal/domain"
)

// Explanation is a failed answer put in words the person can act on — what
// went wrong and what to do — with the raw error kept as Detail for the
// "detalhes técnicos" disclosure.
type Explanation struct {
	Message string
	Detail  string
}

var engineNames = map[string]string{
	"claude_code": "Claude",
	"codex":       "Codex",
	"anthropic":   "Claude (API)",
	"openai":      "OpenAI",
	"ollama":      "Ollama",
	"apple":       "Apple Intelligence",
}

const settingsHint = "em Motor de IA e conexões"

// explainFailure says why an engine gave no answer. err may be nil when the
// engine simply returned nothing — then a failed tool, or not having
// understood the request, is the explanation.
func explainFailure(provider, model string, err error, outcome ChatOutcome) Explanation {
	engine := engineNames[provider]
	if engine == "" {
		engine = "O motor de IA"
	}
	toolNote := failedToolsNote(outcome.ToolCalls)

	if err == nil {
		if toolNote != "" {
			return Explanation{Message: "Não consegui concluir o pedido. " + toolNote + " Corrija ou complete essa informação e mande de novo."}
		}
		return Explanation{Message: fmt.Sprintf("Não entendi o pedido: %s não soube o que responder. Tente reformular com mais detalhes (por exemplo: o valor, a data ou a categoria).", engine)}
	}

	e := Explanation{Message: describeError(provider, engine, model, err), Detail: err.Error()}
	if toolNote != "" {
		e.Message += " " + toolNote
	}
	return e
}

func describeError(provider, engine, model string, err error) string {
	raw := normalize(err.Error())
	has := func(words ...string) bool {
		for _, w := range words {
			if strings.Contains(raw, w) {
				return true
			}
		}
		return false
	}
	cli := provider == "claude_code" || provider == "codex"

	switch {
	case errors.Is(err, context.Canceled):
		return "Você interrompeu a resposta antes de ela terminar."
	case errors.Is(err, context.DeadlineExceeded), has("demorou demais", "timeout", "deadline exceeded"):
		return fmt.Sprintf("%s demorou demais para responder e a resposta foi cancelada. Tente de novo, de preferência com um pedido mais curto.", engine)
	case has("sem api key"):
		return fmt.Sprintf("Falta a chave de API do %s. Adicione a chave %s para usar este motor.", engine, settingsHint)
	}

	// Apple Intelligence: the on-device model and its helper.
	switch {
	case has("appleintelligencenotenabled"):
		return "O Apple Intelligence está desativado neste Mac. Ative em Ajustes do Sistema > Apple Intelligence e Siri, ou escolha outro motor."
	case has("modelnotready", "assetsunavailable", "recursos do modelo local"):
		return "O modelo do Apple Intelligence ainda não está pronto (provavelmente baixando). Tente de novo em alguns minutos."
	case has("devicenoteligible"):
		return "Este Mac não é compatível com o Apple Intelligence. Escolha outro motor " + settingsHint + "."
	case has("ponte do apple intelligence nao compilada"):
		return "A ponte do Apple Intelligence não está compilada. Rode `swift build -c release` na pasta apple-bridge/ e tente de novo."
	case has("ponte do apple intelligence nao respondeu"):
		return "A ponte do Apple Intelligence não iniciou. Tente de novo; se continuar, recompile a pasta apple-bridge/."
	}

	// Limits that aren't about the request itself.
	switch {
	case has("janela de contexto", "context window", "context_length", "context length", "prompt is too long", "too many tokens", "maximum context"):
		return fmt.Sprintf("A conversa ficou longa demais para o %s. Comece uma nova conversa e mande o pedido de novo.", engine)
	case has("credit balance", "insufficient_quota", "billing", "exceeded your current quota"):
		return fmt.Sprintf("A conta do %s está sem créditos ou cota. Confira o faturamento no site do provedor, ou escolha outro motor %s.", engine, settingsHint)
	case has("usage limit", "limit reached", "rate limit", "rate_limit", "quota"):
		if cli {
			return fmt.Sprintf("Você atingiu o limite de uso da sua assinatura no %s. Espere o limite renovar ou escolha outro motor %s.", engine, settingsHint)
		}
		return fmt.Sprintf("Muitos pedidos seguidos: o %s pediu para esperar. Tente de novo em alguns segundos.", engine)
	case has("guardrail", "protecoes de seguranca", "recusou", "refusal", "safety"):
		return fmt.Sprintf("%s recusou o pedido pelas proteções de segurança do modelo. Reformule o pedido de outro jeito.", engine)
	case has("idioma ou regiao", "unsupported language", "unsupportedlanguage"):
		return fmt.Sprintf("%s não entende esse idioma. Escreva em português ou inglês.", engine)
	}

	// Being signed in, installed and reachable.
	switch {
	case cli && has("/login", "not logged", "login", "unauthorized", "authenticat", "invalid api key"):
		cmd := "claude"
		if provider == "codex" {
			cmd = "codex login"
		}
		return fmt.Sprintf("O %s está sem login nesta máquina. Rode `%s` no terminal, entre na sua conta e tente de novo.", engine, cmd)
	case has("executable file not found"):
		return fmt.Sprintf("O %s não está instalado nesta máquina. Instale-o ou escolha outro motor %s.", engine, settingsHint)
	case has("connection refused", "no such host", "dial tcp", "network is unreachable", "connection reset"):
		if provider == "ollama" {
			return "Não consegui conectar ao Ollama. Confira se ele está aberto e se o endereço " + settingsHint + " está certo."
		}
		return fmt.Sprintf("Não consegui conectar ao %s. Confira a internet e tente de novo.", engine)
	}

	var api *apiError
	if errors.As(err, &api) {
		switch {
		case api.Status == 401 || api.Status == 403:
			return fmt.Sprintf("O %s recusou a chave de API (inválida, expirada ou sem permissão). Confira a chave %s.", engine, settingsHint)
		case api.Status == 404 || has("model") && has("not found", "does not exist"):
			return fmt.Sprintf("O modelo %q não existe ou a sua conta não tem acesso a ele. Troque o modelo %s.", model, settingsHint)
		case api.Status == 413:
			return fmt.Sprintf("O pedido ficou grande demais para o %s. Comece uma nova conversa ou mande um pedido mais curto.", engine)
		case api.Status >= 500 && provider != "apple":
			return fmt.Sprintf("O serviço do %s está instável ou sobrecarregado agora. Tente de novo em instantes.", engine)
		}
	}

	// The small on-device model gets lost on requests it can't parse into a
	// tool call, and reports it only as a generic generation error.
	if provider == "apple" && has("generationerror", "decodificada", "decoding", "erro de geracao", "erro inesperado", "falha ao executar a ferramenta") {
		return "O Apple Intelligence não conseguiu entender ou processar esse pedido — é um modelo pequeno e se perde com várias informações de uma vez. Tente de forma mais simples e direta (ex.: \"gastei 1500 em Compras no pix\"), ou use um motor maior."
	}

	return fmt.Sprintf("%s não conseguiu responder por um erro inesperado. Tente de novo; se continuar, veja os detalhes técnicos.", engine)
}

// failedToolsNote tells which of the answer's tools failed and why — usually
// a missing or wrong piece of information (an unknown category, a bad date).
func failedToolsNote(calls []ToolCall) string {
	var reasons []string
	for _, c := range calls {
		if msg := strings.TrimRight(strings.TrimSpace(c.Error), ". "); msg != "" {
			reasons = append(reasons, truncate(msg, 240))
		}
	}
	switch len(reasons) {
	case 0:
		return ""
	case 1:
		return "O que deu errado: " + reasons[0] + "."
	default:
		return "O que deu errado: " + strings.Join(reasons, "; ") + "."
	}
}

// SendErrorMessage explains an error Send returned before any answer started:
// no engine, a deleted conversation, a bad message, or the server itself.
func SendErrorMessage(err error) string {
	switch {
	case errors.Is(err, ErrNoProvider):
		return "Nenhum motor de IA está selecionado. Escolha um no botão ao lado da caixa de mensagem, ou em Motor de IA e conexões."
	case errors.Is(err, domain.ErrValidation):
		return "Não deu para enviar: " + ToolErrorMessage(err) + "."
	case errors.Is(err, domain.ErrNotFound):
		return "Esta conversa não existe mais (pode ter sido apagada). Comece uma nova conversa e mande de novo."
	default:
		return "Não consegui responder por um problema no servidor do Estus (o banco de dados pode estar fora do ar). Tente de novo em instantes."
	}
}
