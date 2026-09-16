package assistant

import (
	"strings"
	"unicode"

	"github.com/rafael/estus-vault/backend/internal/domain"
)

// Choosing a category for an expense. The owner's own categories come first:
// a ledger with forty near-duplicate categories says less than one with
// seven, so a new one is only invented when nothing here fits.

// categoryHints are words that give a description away, grouped by the
// category they point at. They only ever select among the categories the
// owner already has — nothing is created from this list.
var categoryHints = map[string][]string{
	"alimentacao": {"mercado", "supermercado", "feira", "padaria", "acougue", "hortifruti", "ifood", "rappi", "restaurante", "lanche", "lanchonete", "almoco", "jantar", "pizza", "hamburguer", "cafe", "bar"},
	"transporte":  {"uber", "99", "taxi", "gasolina", "combustivel", "etanol", "estacionamento", "pedagio", "onibus", "metro", "passagem", "oficina", "mecanico", "pneu", "seguro do carro"},
	"saude":       {"farmacia", "remedio", "medico", "dentista", "consulta", "exame", "hospital", "plano de saude", "psicologo", "terapia", "academia"},
	"moradia":     {"aluguel", "condominio", "luz", "energia", "agua", "gas", "internet", "iptu", "faxina", "diarista", "reforma", "movel"},
	"assinaturas": {"netflix", "spotify", "assinatura", "icloud", "prime", "disney", "hbo", "youtube", "chatgpt", "mensalidade"},
	"lazer":       {"cinema", "show", "viagem", "hotel", "passeio", "parque", "teatro", "balada", "jogo"},
	"compras":     {"roupa", "tenis", "camisa", "presente", "eletronico", "celular", "notebook", "livro", "amazon", "shopee", "mercado livre"},
}

// categoryPalette is the ring new categories draw their colour from — the
// hues the dashboard's legend was designed around, then a few more.
var categoryPalette = []string{
	"#2a78d6", "#eb6834", "#1baf7a", "#c98d00", "#e2588e",
	"#008300", "#4a3aa7", "#0f9bd7", "#b4530a", "#7a4ad1",
}

// maxCategoryName keeps an invented name short enough for the chart legend.
const maxCategoryName = 24

// guessCategory picks the best fit for a description among the owner's
// categories: the category's own name when it appears in the description,
// otherwise a hint word. The longest match wins, so "mercado livre" is
// shopping rather than groceries.
func guessCategory(description string, cats []domain.Category) (domain.Category, bool) {
	text := normalize(description)
	var best domain.Category
	bestLen := 0
	consider := func(c domain.Category, word string) {
		if len(word) > bestLen && containsWord(text, word) {
			best, bestLen = c, len(word)
		}
	}
	for _, c := range cats {
		key := normalize(c.Name)
		consider(c, key)
		for _, hint := range categoryHints[key] {
			consider(c, hint)
		}
	}
	return best, bestLen > 0
}

// containsWord reports whether text contains word as a whole word, or as a
// phrase when the hint itself has spaces — "cafeteira" is not "cafe".
func containsWord(text, word string) bool {
	if word == "" {
		return false
	}
	if strings.Contains(word, " ") {
		return strings.Contains(text, word)
	}
	for _, token := range strings.FieldsFunc(text, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	}) {
		if token == word {
			return true
		}
	}
	return false
}

// nextCategoryColor picks a hue no category uses yet, cycling once they're
// all taken.
func nextCategoryColor(cats []domain.Category) string {
	used := make(map[string]bool, len(cats))
	for _, c := range cats {
		used[strings.ToLower(strings.TrimSpace(c.Color))] = true
	}
	for _, color := range categoryPalette {
		if !used[color] {
			return color
		}
	}
	return categoryPalette[len(cats)%len(categoryPalette)]
}

// categoryName tidies a name before it becomes a category: one space
// between words, a capital at the start, and short enough to read in the
// legend.
func categoryName(raw string) string {
	name := strings.Join(strings.Fields(raw), " ")
	if r := []rune(name); len(r) > maxCategoryName {
		name = strings.TrimSpace(string(r[:maxCategoryName]))
	}
	if name == "" {
		return ""
	}
	r := []rune(name)
	return string(unicode.ToUpper(r[0])) + string(r[1:])
}
