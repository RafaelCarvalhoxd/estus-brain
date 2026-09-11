package httpapi

import "net/http"

// NewRouter wires the API surface. It is intentionally reachable only from
// the Next.js server, never from a browser directly — see the top-level
// README's architecture section — so there is no CORS layer here: adding
// one would be defending against a request path that the deployment
// topology doesn't allow to exist.
func NewRouter(h *Handlers) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	mux.HandleFunc("GET /api/categories", h.ListCategories)
	mux.HandleFunc("GET /api/credit-cards", h.ListCreditCards)
	mux.HandleFunc("GET /api/months/{month}", h.MonthSummary)
	mux.HandleFunc("POST /api/transactions", h.CreateTransaction)

	return withMiddleware(mux)
}
