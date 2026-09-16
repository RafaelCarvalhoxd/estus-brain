package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/rafael/estus-vault/backend/internal/domain"
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
	// Registered immediately, right after we have the ID: this is the real
	// database, and if the PUT or an assertion below fails with t.Fatalf,
	// the explicit DELETE later in this test never runs. This cleanup is
	// what guarantees the row created above never survives the test. The
	// explicit DELETE below is still what asserts the 204 response; by the
	// time it runs the card may already be gone if a prior step failed, so
	// this cleanup tolerates any outcome (a second delete on an already
	// deleted row is expected to 404, not a bug).
	t.Cleanup(func() {
		del := httptest.NewRequest(http.MethodDelete, "/api/credit-cards/"+created.ID, nil)
		router.ServeHTTP(httptest.NewRecorder(), del)
	})

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

// TestDeleteCreditCardWithTransactionsConflicts proves the one behavior this
// task exists to add: deleting a card that still has transactions must
// surface as 409, not a bare 500 from an unhandled foreign key violation.
// The fixture (card, category, transaction) is built directly through the
// repositories/services — the same ones the router.go handlers use — rather
// than raw SQL, so it goes through the same validation as real data. Every
// row is named obviously as test data and cleaned up in t.Cleanup as soon as
// it exists, in reverse creation order (t.Cleanup runs LIFO), so a failure
// partway through still leaves the real database untouched.
func TestDeleteCreditCardWithTransactionsConflicts(t *testing.T) {
	router, db := creditCardRouter(t)
	ctx := context.Background()
	categories := postgres.NewCategoryRepo(db)
	cards := postgres.NewCreditCardRepo(db)
	transactions := postgres.NewTransactionRepo(db)
	txService := service.NewTransactionService(transactions, categories, cards)

	card, err := cards.Create(ctx, domain.CreditCard{Name: "Cartão de teste (conflito)", ClosingDay: 10, DueDay: 20})
	if err != nil {
		t.Fatalf("create card: %v", err)
	}
	t.Cleanup(func() { cards.Delete(ctx, card.ID) })

	cat, err := categories.Create(ctx, domain.Category{
		Name:   "Categoria de teste (conflito)",
		Nature: domain.NatureDiscretionary,
		Color:  "#123456",
	})
	if err != nil {
		t.Fatalf("create category: %v", err)
	}
	t.Cleanup(func() { categories.Delete(ctx, cat.ID) })

	txns, err := txService.Create(ctx, service.NewTransactionInput{
		Description:   "Compra de teste (conflito)",
		AmountCents:   1000,
		CategoryID:    cat.ID,
		PaymentMethod: domain.PaymentCredit,
		PurchaseDate:  time.Now(),
		CreditCardID:  card.ID,
		Installments:  1,
	})
	if err != nil {
		t.Fatalf("create transaction: %v", err)
	}
	t.Cleanup(func() {
		for _, txn := range txns {
			transactions.Delete(ctx, txn.ID)
		}
	})

	del := httptest.NewRequest(http.MethodDelete, "/api/credit-cards/"+card.ID, nil)
	delRec := httptest.NewRecorder()
	router.ServeHTTP(delRec, del)
	if delRec.Code != http.StatusConflict {
		t.Fatalf("DELETE = %d, want 409; body %s", delRec.Code, delRec.Body.String())
	}
	if delRec.Body.Len() == 0 {
		t.Error("resposta do 409 veio vazia, sem explicação")
	}
}
