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
	Documents *DocumentHandlers
	Training  *TrainingHandlers
	Diet      *DietHandlers
	Boards    *BoardHandlers
	Habits    *HabitHandlers
	Assistant *AssistantHandlers
	Telegram  *TelegramHandlers
	// MCP serves the Model Context Protocol; it checks its own token.
	MCP http.Handler
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
	mux.HandleFunc("POST /api/categories", h.CreateCategory)
	mux.HandleFunc("PUT /api/categories/{id}", h.UpdateCategory)
	mux.HandleFunc("DELETE /api/categories/{id}", h.DeleteCategory)
	mux.HandleFunc("PATCH /api/categories/{id}/budget", h.UpdateCategoryBudget)
	mux.HandleFunc("GET /api/credit-cards", h.ListCreditCards)
	mux.HandleFunc("POST /api/credit-cards", h.CreateCreditCard)
	mux.HandleFunc("PUT /api/credit-cards/{id}", h.UpdateCreditCard)
	mux.HandleFunc("DELETE /api/credit-cards/{id}", h.DeleteCreditCard)
	mux.HandleFunc("GET /api/months/{month}", h.MonthSummary)
	mux.HandleFunc("POST /api/transactions", h.CreateTransaction)
	mux.HandleFunc("PATCH /api/transactions/{id}", h.UpdateTransaction)
	mux.HandleFunc("DELETE /api/transactions/{id}", h.DeleteTransaction)

	if b := m.Bills; b != nil {
		mux.HandleFunc("GET /api/bills", b.List)
		mux.HandleFunc("POST /api/bills", b.Create)
		mux.HandleFunc("GET /api/bills/summary", b.Summary)
		mux.HandleFunc("GET /api/bills/received", b.ReceivedTotal)
		mux.HandleFunc("POST /api/bills/{id}/paid", b.MarkPaid)
		mux.HandleFunc("POST /api/bills/{id}/pay", b.Pay)
		mux.HandleFunc("DELETE /api/bills/{id}/paid", b.Unpay)
		mux.HandleFunc("POST /api/bills/{id}/end-series", b.EndSeries)
		mux.HandleFunc("DELETE /api/bills/{id}/end-series", b.ResumeSeries)
		mux.HandleFunc("PUT /api/bills/{id}", b.Update)
		mux.HandleFunc("DELETE /api/bills/{id}", b.Delete)
	}

	if v := m.Vault; v != nil {
		mux.HandleFunc("GET /api/vault", v.List)
		mux.HandleFunc("POST /api/vault", v.Create)
		mux.HandleFunc("PUT /api/vault/{id}", v.Update)
		mux.HandleFunc("DELETE /api/vault/{id}", v.Delete)
		mux.HandleFunc("POST /api/vault/{id}/reveal", v.Reveal)
	}

	if n := m.Notes; n != nil {
		mux.HandleFunc("GET /api/notes", n.List)
		mux.HandleFunc("POST /api/notes", n.Create)
		mux.HandleFunc("GET /api/notes/{id}", n.Get)
		mux.HandleFunc("PUT /api/notes/{id}", n.Update)
		mux.HandleFunc("DELETE /api/notes/{id}", n.Delete)
		mux.HandleFunc("GET /api/note-categories", n.ListCategories)
		mux.HandleFunc("POST /api/note-categories", n.CreateCategory)
		mux.HandleFunc("PUT /api/note-categories/{id}", n.UpdateCategory)
		mux.HandleFunc("DELETE /api/note-categories/{id}", n.DeleteCategory)
	}

	if rm := m.Reminders; rm != nil {
		mux.HandleFunc("GET /api/reminders", rm.List)
		mux.HandleFunc("POST /api/reminders", rm.Create)
		mux.HandleFunc("PATCH /api/reminders/{id}", rm.SetDone)
		mux.HandleFunc("PUT /api/reminders/{id}", rm.Update)
		mux.HandleFunc("DELETE /api/reminders/{id}", rm.Delete)
	}

	if d := m.Documents; d != nil {
		mux.HandleFunc("GET /api/document-folders", d.ListFolders)
		mux.HandleFunc("POST /api/document-folders", d.CreateFolder)
		mux.HandleFunc("PUT /api/document-folders/{id}", d.RenameFolder)
		mux.HandleFunc("DELETE /api/document-folders/{id}", d.DeleteFolder)
		mux.HandleFunc("GET /api/documents", d.List)
		mux.HandleFunc("POST /api/documents", d.Upload)
		mux.HandleFunc("GET /api/documents/count", d.Count)
		mux.HandleFunc("GET /api/documents/{id}/content", d.Content)
		mux.HandleFunc("PUT /api/documents/{id}", d.Move)
		mux.HandleFunc("DELETE /api/documents/{id}", d.Delete)
	}

	if t := m.Training; t != nil {
		mux.HandleFunc("GET /api/workouts", t.List)
		mux.HandleFunc("POST /api/workouts", t.Create)
		mux.HandleFunc("PUT /api/workouts/{id}", t.Update)
		mux.HandleFunc("DELETE /api/workouts/{id}", t.Delete)
	}

	if dt := m.Diet; dt != nil {
		mux.HandleFunc("GET /api/meals", dt.List)
		mux.HandleFunc("POST /api/meals", dt.Create)
		mux.HandleFunc("PUT /api/meals/{id}", dt.Update)
		mux.HandleFunc("DELETE /api/meals/{id}", dt.Delete)
		mux.HandleFunc("GET /api/diet/targets", dt.Targets)
		mux.HandleFunc("PUT /api/diet/targets", dt.SetTargets)
	}

	if a := m.Assistant; a != nil {
		mux.HandleFunc("GET /api/assistant/tools", a.ListTools)
		mux.HandleFunc("POST /api/assistant/tools/{name}", a.CallTool)
		mux.HandleFunc("GET /api/assistant/settings", a.Settings)
		mux.HandleFunc("PUT /api/assistant/settings", a.UpdateSettings)
		mux.HandleFunc("GET /api/assistant/conversations", a.Conversations)
		mux.HandleFunc("GET /api/assistant/conversations/{id}", a.Conversation)
		mux.HandleFunc("DELETE /api/assistant/conversations/{id}", a.DeleteConversation)
		mux.HandleFunc("POST /api/assistant/record", a.Record)
		mux.HandleFunc("POST /api/assistant/chat", a.Send)
		mux.HandleFunc("POST /api/assistant/voice/transcribe", a.Transcribe)
		mux.HandleFunc("POST /api/assistant/voice/speak", a.Speak)
	}
	if tg := m.Telegram; tg != nil {
		mux.HandleFunc("GET /api/assistant/telegram/settings", tg.Settings)
		mux.HandleFunc("PUT /api/assistant/telegram/settings", tg.UpdateSettings)
		mux.HandleFunc("POST /api/assistant/telegram/pair", tg.Pair)
		mux.HandleFunc("DELETE /api/assistant/telegram/pair", tg.Unpair)
		mux.HandleFunc("POST /api/assistant/telegram/test", tg.Test)
	}
	if m.MCP != nil {
		mux.Handle("/mcp", m.MCP)
	}

	if hb := m.Habits; hb != nil {
		mux.HandleFunc("GET /api/habits", hb.List)
		mux.HandleFunc("POST /api/habits", hb.Create)
		mux.HandleFunc("PUT /api/habits/{id}", hb.Update)
		mux.HandleFunc("DELETE /api/habits/{id}", hb.Delete)
		mux.HandleFunc("PUT /api/habits/{id}/log", hb.SetLog)
	}

	if bd := m.Boards; bd != nil {
		mux.HandleFunc("GET /api/boards", bd.List)
		mux.HandleFunc("POST /api/boards", bd.Create)
		mux.HandleFunc("GET /api/boards/{id}", bd.Get)
		mux.HandleFunc("PATCH /api/boards/{id}", bd.Rename)
		mux.HandleFunc("PUT /api/boards/{id}/scene", bd.SaveScene)
		mux.HandleFunc("DELETE /api/boards/{id}", bd.Delete)
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
