package httpapi

import "net/http"

// Modules bundles every module's handler group so NewRouter has one thing to
// take instead of a growing positional parameter list. Each field is
// optional (nil-safe) so a module can be wired in independently — useful in
// development when, say, GOOGLE_CLIENT_ID isn't set yet but you still want
// the rest of the API up.
type Modules struct {
	Bills     *BillHandlers
	Vault     *VaultHandlers
	Notes     *NoteHandlers
	Reminders *ReminderHandlers
	Events    *EventHandlers
}

// NewRouter wires the API surface. It is intentionally reachable only from
// the Next.js server, never from a browser directly — see the top-level
// README's architecture section — so there is no CORS layer here: adding
// one would be defending against a request path that the deployment
// topology doesn't allow to exist.
func NewRouter(h *Handlers, m Modules) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	mux.HandleFunc("GET /api/categories", h.ListCategories)
	mux.HandleFunc("GET /api/credit-cards", h.ListCreditCards)
	mux.HandleFunc("GET /api/months/{month}", h.MonthSummary)
	mux.HandleFunc("POST /api/transactions", h.CreateTransaction)
	mux.HandleFunc("PATCH /api/transactions/{id}", h.UpdateTransaction)
	mux.HandleFunc("DELETE /api/transactions/{id}", h.DeleteTransaction)

	if b := m.Bills; b != nil {
		mux.HandleFunc("GET /api/bills", b.List)
		mux.HandleFunc("POST /api/bills", b.Create)
		mux.HandleFunc("GET /api/bills/summary", b.Summary)
		mux.HandleFunc("POST /api/bills/{id}/paid", b.MarkPaid)
		mux.HandleFunc("PUT /api/bills/{id}", b.Update)
		mux.HandleFunc("DELETE /api/bills/{id}", b.Delete)
	}

	if v := m.Vault; v != nil {
		mux.HandleFunc("GET /api/vault", v.List)
		mux.HandleFunc("POST /api/vault", v.Create)
		mux.HandleFunc("PUT /api/vault/{id}", v.Update)
		mux.HandleFunc("DELETE /api/vault/{id}", v.Delete)
		mux.HandleFunc("GET /api/vault/webauthn-status", v.WebAuthnStatus)
		mux.HandleFunc("POST /api/vault/webauthn/register/begin", v.RegisterBegin)
		mux.HandleFunc("POST /api/vault/webauthn/register/finish", v.RegisterFinish)
		mux.HandleFunc("POST /api/vault/{id}/reveal/begin", v.RevealBegin)
		mux.HandleFunc("POST /api/vault/{id}/reveal/finish", v.RevealFinish)
	}

	if n := m.Notes; n != nil {
		mux.HandleFunc("GET /api/notes", n.List)
		mux.HandleFunc("POST /api/notes", n.Create)
		mux.HandleFunc("GET /api/notes/{id}", n.Get)
		mux.HandleFunc("PUT /api/notes/{id}", n.Update)
		mux.HandleFunc("DELETE /api/notes/{id}", n.Delete)
	}

	if rm := m.Reminders; rm != nil {
		mux.HandleFunc("GET /api/reminders", rm.List)
		mux.HandleFunc("POST /api/reminders", rm.Create)
		mux.HandleFunc("PATCH /api/reminders/{id}", rm.SetDone)
		mux.HandleFunc("PUT /api/reminders/{id}", rm.Update)
		mux.HandleFunc("DELETE /api/reminders/{id}", rm.Delete)
	}

	if e := m.Events; e != nil {
		mux.HandleFunc("GET /api/events", e.ListRange)
		mux.HandleFunc("POST /api/events", e.Create)
		mux.HandleFunc("PUT /api/events/{id}", e.Update)
		mux.HandleFunc("DELETE /api/events/{id}", e.Delete)
		mux.HandleFunc("GET /api/google/status", e.GoogleStatus)
		mux.HandleFunc("GET /api/google/oauth/start", e.GoogleAuthStart)
		mux.HandleFunc("GET /api/google/oauth/callback", e.GoogleCallback)
		mux.HandleFunc("POST /api/google/sync", e.GoogleSync)
	}

	return withMiddleware(mux)
}
