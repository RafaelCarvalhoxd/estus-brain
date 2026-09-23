package assistant

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rafael/estus-vault/backend/internal/domain"
)

const serverInstructions = `Ferramentas do Estus Brain, o app pessoal do dono: finanças (gastos, categorias, cartões), contas a pagar e receber, notas, lembretes, agenda, hábitos, treino, dieta, documentos e quadros.
Valores em reais; datas AAAA-MM-DD e horários no fuso de São Paulo. Para listar ou editar algo por id, liste antes.
Antes de qualquer ferramenta que exclui (confirm), pergunte à pessoa e só chame com confirm=true depois do sim.
` + appRules

// appRules is how the app works, for any model using the tools — the chat's
// system prompt and MCP clients read the same text, so they agree with the
// screens.
const appRules = `Como o app funciona:
- Lançamento aparece no mês da compra. No crédito, também entra na fatura do cartão (pelo dia de fechamento e de vencimento); cada cartão tem uma conta a pagar "Fatura <cartão>" por mês, que é a soma das compras e se atualiza sozinha. Pagar a fatura (bills_pay) não cria despesa nova.
- Parcelado: installments com o total em amount, ou o valor da parcela com per_installment=true. Cada parcela aparece no seu mês. Não marque recurring junto. Excluir ou editar uma parcela vale para a compra inteira.
- Categorias têm tipo: despesa conta como gasto; receita conta como entrada.
- Saídas do mês = dinheiro que saiu de fato: débito, pix e faturas pagas. Compra no crédito só sai quando a fatura é paga. Entradas = contas recebidas + lançamentos em receita. Saldo = entradas − saídas.
- Fixos = lançamentos recorrentes, parcelas (até a última) e contas a pagar recorrentes; o resto é variável.
- Contas: "já paguei X" é bills_pay (registra a despesa); conta a receber é bills_mark_received. Conta recorrente se repete todo mês até bills_end_series (delete_following exclui as seguintes não pagas).
- Lembretes podem repetir em dias da semana ou num dia do mês; concluir um repetido o move para a próxima vez.
- Agenda: horário é opcional; sem horário o evento ocupa o dia todo.`

// MCPServer exposes every tool over the Model Context Protocol, for AI apps
// the owner connects (Claude Desktop, Claude Code, Codex…) and for the chat's
// own CLI-based engines.
func (r *Registry) MCPServer() *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: "estus-brain", Title: "Estus Brain", Version: "1.0.0"}, &mcp.ServerOptions{
		Instructions: serverInstructions,
	})
	for _, t := range r.tools {
		tool := t
		destructive := tool.Destructive
		server.AddTool(&mcp.Tool{
			Name:        tool.Name,
			Title:       tool.Title,
			Description: tool.Description,
			InputSchema: tool.Input,
			Annotations: &mcp.ToolAnnotations{
				Title:           tool.Title,
				ReadOnlyHint:    tool.ReadOnly,
				DestructiveHint: &destructive,
			},
		}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			result, err := r.Call(ctx, tool.Name, req.Params.Arguments)
			if err != nil {
				return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: ToolErrorMessage(err)}}}, nil
			}
			text, _ := json.Marshal(result)
			return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(text)}}}, nil
		})
	}
	return server
}

// MCPHandler serves MCP over streamable HTTP, accepting only requests that
// carry one of the given bearer tokens.
func (r *Registry) MCPHandler(tokens func() []string) http.Handler {
	server := r.MCPServer()
	h := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, &mcp.StreamableHTTPOptions{
		Stateless:    true,
		JSONResponse: true,
		// The endpoint is reached through the app's own proxy and tunnels, so
		// the Host header is not localhost; the token is the protection.
		DisableLocalhostProtection: true,
	})
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if !Authorized(req, tokens()) {
			w.Header().Set("WWW-Authenticate", `Bearer realm="estus-brain"`)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		h.ServeHTTP(w, req)
	})
}

// Authorized reports whether the request carries one of the tokens, either
// as "Authorization: Bearer" or as ?token= (for clients that can't set headers).
func Authorized(req *http.Request, tokens []string) bool {
	got := strings.TrimSpace(strings.TrimPrefix(req.Header.Get("Authorization"), "Bearer "))
	if got == "" {
		got = req.URL.Query().Get("token")
	}
	if got == "" {
		return false
	}
	for _, t := range tokens {
		if t != "" && subtle.ConstantTimeCompare([]byte(got), []byte(t)) == 1 {
			return true
		}
	}
	return false
}

// ToolErrorMessage turns a tool failure into the text shown to a person or
// model: validation messages as written, not-found plainly, anything else
// generic (internals stay in the server log).
func ToolErrorMessage(err error) string {
	switch {
	case errors.Is(err, ErrUnknownTool):
		return "ferramenta desconhecida"
	case errors.Is(err, domain.ErrValidation):
		return strings.TrimPrefix(err.Error(), domain.ErrValidation.Error()+": ")
	case errors.Is(err, domain.ErrNotFound):
		return "não encontrado — confira o id listando de novo"
	case errors.Is(err, domain.ErrConflict):
		return "conflito: " + err.Error()
	default:
		return "erro interno ao executar a ferramenta"
	}
}
