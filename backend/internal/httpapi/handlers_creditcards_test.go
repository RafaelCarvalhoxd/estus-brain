package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/rafael/estus-vault/backend/internal/service"
	"github.com/rafael/estus-vault/backend/internal/store/postgres"
)

// creditCardRouter builds the real router over the real repositories — the
// only way to exercise these routes, since Handlers takes concrete repos.
func creditCardRouter(t *testing.T) (http.Handler, *postgres.DB) {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	ctx := context.Background()
	db, err := postgres.Connect(ctx, url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(db.Close)
	categories := postgres.NewCategoryRepo(db)
	cards := postgres.NewCreditCardRepo(db)
	transactions := postgres.NewTransactionRepo(db)
	handlers := NewHandlers(
		categories,
		cards,
		service.NewTransactionService(transactions, categories, cards),
		service.NewDashboardService(transactions),
	)
	return NewRouter(handlers, Modules{}), db
}

func postCard(t *testing.T, router http.Handler, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/credit-cards", strings.NewReader(body))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestCreateCreditCardRejectsBadInput(t *testing.T) {
	router, _ := creditCardRouter(t)
	for _, body := range []string{
		`{"name":"Nubank","closing_day":0,"due_day":20}`,
		`{"name":"Nubank","closing_day":29,"due_day":20}`,
		`{"name":"Nubank","closing_day":10,"due_day":0}`,
		`{"name":"Nubank","closing_day":10,"due_day":29}`,
		`{"name":"   ","closing_day":10,"due_day":20}`,
		`{`,
	} {
		// writeError maps ErrValidation to 422, not 400 — see respond.go.
		if rec := postCard(t, router, body); rec.Code != http.StatusUnprocessableEntity {
			t.Errorf("POST %s = %d, want 422", body, rec.Code)
		}
	}
}

func TestCreateAndDeleteCreditCardRoundTrip(t *testing.T) {
	router, _ := creditCardRouter(t)
	rec := postCard(t, router, `{"name":"Cartão de rota","closing_day":10,"due_day":20}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST = %d, want 201; body %s", rec.Code, rec.Body.String())
	}
	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil || created.ID == "" {
		t.Fatalf("resposta = %s, %v", rec.Body.String(), err)
	}

	put := httptest.NewRequest(http.MethodPut, "/api/credit-cards/"+created.ID,
		strings.NewReader(`{"name":"Renomeado","closing_day":5,"due_day":25}`))
	putRec := httptest.NewRecorder()
	router.ServeHTTP(putRec, put)
	if putRec.Code != http.StatusOK {
		t.Fatalf("PUT = %d, want 200; body %s", putRec.Code, putRec.Body.String())
	}

	del := httptest.NewRequest(http.MethodDelete, "/api/credit-cards/"+created.ID, nil)
	delRec := httptest.NewRecorder()
	router.ServeHTTP(delRec, del)
	if delRec.Code != http.StatusNoContent {
		t.Fatalf("DELETE = %d, want 204; body %s", delRec.Code, delRec.Body.String())
	}
}
