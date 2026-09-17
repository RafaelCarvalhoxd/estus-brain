package assistant

import (
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

// This file decides which modules' tools a small, on-device model is handed
// for one message.
//
// It exists because of a real failure: asked to create a habit, the model was
// only ever given the finance and bills tools — habits_create was registered
// but never offered — so it booked a R$1,00 expense called "Leitura" and then
// reported that the habit had been created. The model was not at fault: we
// hid the tool and blamed it. A capable model still gets everything; this
// routing only narrows the toolbox for the ones that cannot hold 33 tools.

// subjectWords maps a module to the words that give its subject away. They
// are matched against a lowercased, accent-stripped message, so each entry is
// written that way. Stems are deliberate: "lembr" catches lembra, lembre,
// lembrete, lembretes.
var subjectWords = map[string][]string{
	"habitos":    {"habito", "habitos", "rotina", "todo dia", "todos os dias", "diariamente", "sequencia", "streak"},
	"treino":     {"treino", "treinar", "treinei", "exercicio", "serie de", "repeticoes", "academia", "musculacao"},
	"dieta":      {"dieta", "refeicao", "refeicoes", "almoco", "jantar", "cafe da manha", "caloria", "macro", "proteina", "comi", "comer"},
	"notas":      {"nota", "notas", "anota", "anotar", "anotacao", "caderno", "escrever sobre"},
	"lembretes":  {"lembr", "me avisa", "nao me deixa esquecer"},
	"agenda":     {"agenda", "compromisso", "reuniao", "evento", "calendario", "marcado para"},
	"documentos": {"documento", "documentos", "arquivo", "comprovante", "pdf", "onde esta meu"},
	"quadros":    {"quadro", "quadros", "lousa", "diagrama", "fluxo de"},
	"contas":     {"conta de", "contas", "boleto", "fatura", "vencimento", "vence", "a pagar", "a receber", "paguei"},
	"financeiro": {"gasto", "gastos", "gastei", "gastar", "comprei", "compra", "despesa", "cartao", "categoria", "orcamento", "quanto custou", "quanto gastei", "paguei", "pix", "debito", "credito", "mercado", "dinheiro", "reais", "transferencia", "recebi"},
}

// fallbackModules is what an unrecognised message gets: the everyday case,
// which is money. It is the behaviour that existed before this router, kept
// deliberately — an empty toolbox would be worse than a narrow one.
var fallbackModules = []string{"financeiro", "contas"}

// modulesFor is the modules whose tools should be offered for message.
// "geral" is always included: it holds the day overview, useful whatever the
// subject. A message about two subjects gets both, rather than the router
// picking a winner it has no basis to pick.
func modulesFor(message string) []string {
	plain := foldForMatch(message)
	out := []string{"geral"}
	for _, module := range moduleOrder {
		for _, word := range subjectWords[module] {
			if strings.Contains(plain, word) {
				out = append(out, module)
				break
			}
		}
	}
	if len(out) == 1 {
		return append(out, fallbackModules...)
	}
	return out
}

// moduleOrder keeps the result stable across runs; ranging over the map
// alone would shuffle it, and a tool list that changes order between
// identical messages makes a model's behaviour impossible to reproduce.
var moduleOrder = []string{
	"habitos", "treino", "dieta", "notas", "lembretes",
	"agenda", "documentos", "quadros", "contas", "financeiro",
}

// foldForMatch lowercases and strips accents, so "HÁBITO" and "habito" are
// the same word — which is how people actually type.
func foldForMatch(s string) string {
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	folded, _, err := transform.String(t, strings.ToLower(s))
	if err != nil {
		return strings.ToLower(s)
	}
	return folded
}
