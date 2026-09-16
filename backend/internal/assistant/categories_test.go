package assistant

import (
	"strings"
	"testing"

	"github.com/rafael/estus-vault/backend/internal/domain"
)

func seedCategories() []domain.Category {
	return []domain.Category{
		{ID: "1", Name: "Moradia", Color: "#2a78d6"},
		{ID: "2", Name: "Alimentação", Color: "#eb6834"},
		{ID: "3", Name: "Transporte", Color: "#1baf7a"},
		{ID: "4", Name: "Assinaturas", Color: "#c98d00"},
		{ID: "5", Name: "Lazer", Color: "#e2588e"},
		{ID: "6", Name: "Saúde", Color: "#008300"},
		{ID: "7", Name: "Compras", Color: "#4a3aa7"},
	}
}

func TestGuessCategory(t *testing.T) {
	cats := seedCategories()
	cases := []struct {
		description string
		want        string // "" = no guess
	}{
		{"mercado", "Alimentação"},
		{"Feira da esquina", "Alimentação"},
		{"almoço com a equipe", "Alimentação"},
		{"uber pro aeroporto", "Transporte"},
		{"gasolina", "Transporte"},
		{"farmácia", "Saúde"},
		{"conta de luz", "Moradia"},
		{"netflix", "Assinaturas"},
		{"cinema no sábado", "Lazer"},
		{"tênis novo", "Compras"},
		// The longer phrase wins, so a marketplace isn't groceries.
		{"compra no mercado livre", "Compras"},
		// The category's own name in the description is enough.
		{"gasto de lazer", "Lazer"},
		// Nothing to go on: no guess, so the caller creates one.
		{"petshop", ""},
		{"", ""},
		// A word that merely contains a hint doesn't count.
		{"cafeteira nova", ""},
	}
	for _, tc := range cases {
		got, ok := guessCategory(tc.description, cats)
		switch {
		case tc.want == "" && ok:
			t.Errorf("guessCategory(%q) = %q, want no guess", tc.description, got.Name)
		case tc.want != "" && (!ok || got.Name != tc.want):
			t.Errorf("guessCategory(%q) = %q (%v), want %q", tc.description, got.Name, ok, tc.want)
		}
	}
}

func TestGuessCategoryOnlyOffersWhatExists(t *testing.T) {
	// The owner deleted Transporte: an Uber has nowhere to go.
	cats := []domain.Category{{ID: "2", Name: "Alimentação"}}
	if got, ok := guessCategory("uber pro aeroporto", cats); ok {
		t.Fatalf("guessCategory = %q, want no guess", got.Name)
	}
	if got, ok := guessCategory("mercado", cats); !ok || got.Name != "Alimentação" {
		t.Fatalf("guessCategory = %q (%v)", got.Name, ok)
	}
}

func TestNextCategoryColor(t *testing.T) {
	cats := seedCategories()
	color := nextCategoryColor(cats)
	for _, c := range cats {
		if strings.EqualFold(c.Color, color) {
			t.Fatalf("nextCategoryColor = %s, already used by %s", color, c.Name)
		}
	}
	if nextCategoryColor(nil) != categoryPalette[0] {
		t.Fatalf("first category should take the first colour")
	}
	// Every hue taken: it cycles instead of returning nothing.
	var all []domain.Category
	for i, hue := range categoryPalette {
		all = append(all, domain.Category{ID: string(rune('a' + i)), Name: hue, Color: hue})
	}
	if got := nextCategoryColor(all); got == "" {
		t.Fatal("nextCategoryColor returned no colour")
	}
}

func TestCategoryName(t *testing.T) {
	cases := []struct{ in, want string }{
		{"pets", "Pets"},
		{"  cuidados   com o pet ", "Cuidados com o pet"},
		{"Educação", "Educação"},
		{strings.Repeat("nome muito longo ", 5), "Nome muito longo nome mu"},
		{"   ", ""},
	}
	for _, tc := range cases {
		if got := categoryName(tc.in); got != tc.want {
			t.Errorf("categoryName(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
