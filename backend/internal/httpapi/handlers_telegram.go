package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/rafael/estus-vault/backend/internal/assistant"
	"github.com/rafael/estus-vault/backend/internal/domain"
	"github.com/rafael/estus-vault/backend/internal/telegram"
)

type TelegramHandlers struct{ bot *telegram.Bot }

func NewTelegramHandlers(bot *telegram.Bot) *TelegramHandlers { return &TelegramHandlers{bot: bot} }

// writeTelegramError sends a validation problem as the plain sentence the
// settings screen shows.
func writeTelegramError(w http.ResponseWriter, err error) {
	if errors.Is(err, domain.ErrValidation) {
		writeJSON(w, http.StatusUnprocessableEntity, errorBody{Error: assistant.ToolErrorMessage(err)})
		return
	}
	writeError(w, err)
}

func (h *TelegramHandlers) Settings(w http.ResponseWriter, r *http.Request) {
	view, err := h.bot.View(r.Context())
	if err != nil {
		writeTelegramError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

func (h *TelegramHandlers) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	var in telegram.SettingsInput
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&in); err != nil {
		writeTelegramError(w, fmt.Errorf("%w: dados inválidos", domain.ErrValidation))
		return
	}
	if err := h.bot.Update(r.Context(), in); err != nil {
		writeTelegramError(w, err)
		return
	}
	h.Settings(w, r)
}

func (h *TelegramHandlers) Pair(w http.ResponseWriter, r *http.Request) {
	pairing, err := h.bot.StartPairing(r.Context())
	if err != nil {
		writeTelegramError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, pairing)
}

func (h *TelegramHandlers) Unpair(w http.ResponseWriter, r *http.Request) {
	if err := h.bot.Unpair(r.Context()); err != nil {
		writeTelegramError(w, err)
		return
	}
	writeJSON(w, http.StatusNoContent, nil)
}

func (h *TelegramHandlers) Test(w http.ResponseWriter, r *http.Request) {
	if err := h.bot.SendTest(r.Context()); err != nil {
		writeTelegramError(w, err)
		return
	}
	writeJSON(w, http.StatusNoContent, nil)
}
