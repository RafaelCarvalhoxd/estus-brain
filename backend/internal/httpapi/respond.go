package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/rafael/estus-vault/backend/internal/domain"
)

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if body == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(body); err != nil {
		slog.Error("encode response", "error", err)
	}
}

type errorBody struct {
	Error string `json:"error"`
}

// writeError maps a domain sentinel error to the right HTTP status. Handlers
// never choose a status code themselves for a service-layer failure — this
// is the one place that decision is made, so every endpoint fails the same
// way for the same class of error.
func writeError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, domain.ErrNotFound):
		status = http.StatusNotFound
	case errors.Is(err, domain.ErrValidation), errors.Is(err, domain.ErrCreditCardMissing):
		status = http.StatusUnprocessableEntity
	case errors.Is(err, domain.ErrConflict):
		status = http.StatusConflict
	default:
		slog.Error("internal error", "error", err)
	}
	writeJSON(w, status, errorBody{Error: err.Error()})
}
