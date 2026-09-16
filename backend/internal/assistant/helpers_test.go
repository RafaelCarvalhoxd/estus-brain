package assistant

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/rafael/estus-vault/backend/internal/domain"
)

func testRegistry(t *testing.T) *Registry {
	t.Helper()
	loc, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		t.Fatal(err)
	}
	return New(Deps{Location: loc})
}

func TestNormalizeAndMatch(t *testing.T) {
	if got := normalize("  Alimentação "); got != "alimentacao" {
		t.Fatalf("normalize = %q", got)
	}
	type cat struct{ id, name string }
	cats := []cat{{"1", "Alimentação"}, {"2", "Assinaturas"}, {"3", "Transporte"}}
	id := func(c cat) string { return c.id }
	name := func(c cat) string { return c.name }

	cases := map[string]string{"alimentacao": "1", "3": "3", "trans": "3", "natu": "2"}
	for q, want := range cases {
		got, ok, _ := match(cats, q, id, name)
		if !ok || got.id != want {
			t.Errorf("match(%q) = %v %v, want %s", q, got, ok, want)
		}
	}
	// "a" is a prefix of two names: ambiguous, so no match.
	if _, ok, names := match(cats, "a", id, name); ok || len(names) != 3 {
		t.Errorf("ambiguous query matched")
	}
}

func TestParseMonthAndDay(t *testing.T) {
	r := testRegistry(t)
	now := domain.YearMonthOf(r.now())
	for q, want := range map[string]domain.YearMonth{"": now, "proximo": now.Add(1), "mês que vem": now.Add(1), "passado": now.Add(-1), "2026-01": {Year: 2026, Month: 1}} {
		got, err := r.parseMonth(q)
		if err != nil || got != want {
			t.Errorf("parseMonth(%q) = %v %v, want %v", q, got, err, want)
		}
	}
	if _, err := r.parseMonth("janeiro"); !errors.Is(err, domain.ErrValidation) {
		t.Errorf("bad month accepted")
	}
	if d, err := r.parseDay("ontem"); err != nil || !d.Equal(r.today().AddDate(0, 0, -1)) {
		t.Errorf("parseDay(ontem) = %v %v", d, err)
	}
	at, err := r.parseLocalTime("2026-09-15T11:00")
	if err != nil || at.Location().String() != "America/Sao_Paulo" || at.Hour() != 11 {
		t.Errorf("parseLocalTime = %v %v", at, err)
	}
}

func TestDestructiveToolsNeedConfirmation(t *testing.T) {
	r := testRegistry(t)
	called := false
	r.add(Tool{Name: "x_delete", Module: "x", Destructive: true, Input: object(map[string]any{"id": str("id")}, "id"),
		run: func(context.Context, json.RawMessage) (any, error) { called = true; return nil, nil }})

	if _, err := r.Call(context.Background(), "x_delete", json.RawMessage(`{"id":"1"}`)); !errors.Is(err, domain.ErrValidation) || called {
		t.Fatalf("ran without confirm: %v", err)
	}
	if _, err := r.Call(context.Background(), "x_delete", json.RawMessage(`{"id":"1","confirm":true}`)); err != nil || !called {
		t.Fatalf("did not run with confirm: %v", err)
	}
	if req := r.byName["x_delete"].Input["required"].([]string); req[len(req)-1] != "confirm" {
		t.Fatalf("confirm not required in schema: %v", req)
	}
	if _, err := r.Call(context.Background(), "nope", nil); !errors.Is(err, ErrUnknownTool) {
		t.Fatalf("unknown tool: %v", err)
	}
}

func TestTextDoc(t *testing.T) {
	var doc struct {
		Type    string `json:"type"`
		Content []struct {
			Type string `json:"type"`
		} `json:"content"`
	}
	if err := json.Unmarshal(textDoc("linha 1\n\nlinha 3"), &doc); err != nil || doc.Type != "doc" || len(doc.Content) != 3 {
		t.Fatalf("textDoc = %+v %v", doc, err)
	}
}
