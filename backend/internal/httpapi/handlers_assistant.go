package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/rafael/estus-vault/backend/internal/assistant"
	"github.com/rafael/estus-vault/backend/internal/domain"
)

type AssistantHandlers struct {
	tools *assistant.Registry
	chat  *assistant.Chat
}

func NewAssistantHandlers(tools *assistant.Registry, chat *assistant.Chat) *AssistantHandlers {
	return &AssistantHandlers{tools: tools, chat: chat}
}

func (h *AssistantHandlers) ListTools(w http.ResponseWriter, r *http.Request) {
	modules := r.URL.Query()["module"]
	writeJSON(w, http.StatusOK, map[string]any{"tools": h.tools.Tools(modules...)})
}

// CallTool runs one tool. Replies {"result": …}, or {"error": "…"} with a
// status that says whose fault it was — the shape the chat's ready-made
// flows and the Apple Intelligence bridge both read.
func (h *AssistantHandlers) CallTool(w http.ResponseWriter, r *http.Request) {
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		writeJSON(w, http.StatusRequestEntityTooLarge, map[string]string{"error": "entrada grande demais"})
		return
	}
	if len(raw) > 0 && !json.Valid(raw) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "JSON inválido"})
		return
	}
	result, err := h.tools.Call(r.Context(), r.PathValue("name"), raw)
	if err != nil {
		status := http.StatusInternalServerError
		switch {
		case errors.Is(err, assistant.ErrUnknownTool), errors.Is(err, domain.ErrNotFound):
			status = http.StatusNotFound
		case errors.Is(err, domain.ErrValidation):
			status = http.StatusUnprocessableEntity
		case errors.Is(err, domain.ErrConflict):
			status = http.StatusConflict
		}
		if status == http.StatusInternalServerError {
			writeError(w, err)
			return
		}
		writeJSON(w, status, map[string]string{"error": assistant.ToolErrorMessage(err)})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"result": result})
}

// ---- chat

func (h *AssistantHandlers) Settings(w http.ResponseWriter, r *http.Request) {
	view, err := h.chat.Settings(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

func (h *AssistantHandlers) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	var u assistant.SettingsUpdate
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&u); err != nil {
		writeError(w, errors.Join(domain.ErrValidation, err))
		return
	}
	if err := h.chat.UpdateSettings(r.Context(), u); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusNoContent, nil)
}

func (h *AssistantHandlers) Conversations(w http.ResponseWriter, r *http.Request) {
	list, err := h.chat.Conversations(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	if list == nil {
		list = []assistant.ConversationView{}
	}
	writeJSON(w, http.StatusOK, list)
}

func (h *AssistantHandlers) Conversation(w http.ResponseWriter, r *http.Request) {
	conv, messages, err := h.chat.Conversation(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	if messages == nil {
		messages = []assistant.MessageView{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"conversation": conv, "messages": messages})
}

func (h *AssistantHandlers) DeleteConversation(w http.ResponseWriter, r *http.Request) {
	if err := h.chat.DeleteConversation(r.Context(), r.PathValue("id")); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusNoContent, nil)
}

// Record keeps a ready-made flow's turn in the conversation history.
func (h *AssistantHandlers) Record(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ConversationID string          `json:"conversation_id"`
		Module         string          `json:"module"`
		User           string          `json:"user"`
		Assistant      string          `json:"assistant"`
		Data           json.RawMessage `json:"data"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeError(w, errors.Join(domain.ErrValidation, err))
		return
	}
	id, err := h.chat.Record(r.Context(), req.ConversationID, req.Module, req.User, req.Assistant, req.Data)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"conversation_id": id})
}

// Send streams the answer as server-sent events, one JSON event per message.
func (h *AssistantHandlers) Send(w http.ResponseWriter, r *http.Request) {
	var req assistant.SendRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 256<<10)).Decode(&req); err != nil {
		writeError(w, errors.Join(domain.ErrValidation, err))
		return
	}
	flusher, _ := w.(http.Flusher)
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	emit := func(e assistant.Event) {
		b, err := json.Marshal(e)
		if err != nil {
			return
		}
		_, _ = w.Write([]byte("data: "))
		_, _ = w.Write(b)
		_, _ = w.Write([]byte("\n\n"))
		if flusher != nil {
			flusher.Flush()
		}
	}
	if err := h.chat.Send(r.Context(), req, emit); err != nil {
		if !errors.Is(err, assistant.ErrNoProvider) && !errors.Is(err, domain.ErrValidation) && !errors.Is(err, domain.ErrNotFound) {
			slog.Error("assistant send failed", "error", err)
		}
		emit(assistant.Event{Type: "error", Error: assistant.SendErrorMessage(err), Detail: err.Error()})
		emit(assistant.Event{Type: "done"})
	}
}

// ---- voice

// Transcribe turns a recording into text with the Mac's speech.
func (h *AssistantHandlers) Transcribe(w http.ResponseWriter, r *http.Request) {
	audio, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 10<<20))
	if err != nil {
		writeJSON(w, http.StatusRequestEntityTooLarge, map[string]string{"error": "Áudio longo demais — mande até 3 minutos."})
		return
	}
	started := time.Now()
	t, err := h.chat.Voice().Transcribe(r.Context(), audio, r.Header.Get("X-Filename"))
	if err != nil {
		writeVoiceError(w, err)
		return
	}
	slog.Info("voice transcribed", "audio_seconds", t.Seconds, "took_ms", time.Since(started).Milliseconds())
	writeJSON(w, http.StatusOK, t)
}

// Speak reads an answer aloud, as AAC audio.
func (h *AssistantHandlers) Speak(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 256<<10)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "JSON inválido"})
		return
	}
	audio, err := h.chat.Voice().Speak(r.Context(), req.Text)
	if err != nil {
		writeVoiceError(w, err)
		return
	}
	w.Header().Set("Content-Type", "audio/mp4")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(audio)
}

// writeVoiceError answers with the sentence the chat shows; only unexpected
// failures are logged (never the audio or the text).
func writeVoiceError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, assistant.ErrVoiceRejected):
		status = http.StatusUnprocessableEntity
	case errors.Is(err, assistant.ErrVoiceUnavailable), errors.Is(err, assistant.ErrVoiceOutdated):
		status = http.StatusServiceUnavailable
	default:
		slog.Error("voice failed", "error", err)
	}
	writeJSON(w, status, map[string]string{"error": assistant.VoiceErrorMessage(err)})
}
