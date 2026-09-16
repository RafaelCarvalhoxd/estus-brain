package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rafael/estus-vault/backend/internal/assistant"
)

// Every voice failure reaches the chat as a sentence the owner can act on, with
// the status code that says whose fault it was.
func TestWriteVoiceError(t *testing.T) {
	// The unexpected case logs; keep the test output clean.
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })

	cases := []struct {
		name    string
		err     error
		status  int
		message string
	}{
		{
			name:    "rejected keeps the bridge's reason",
			err:     fmt.Errorf("%w: Áudio longo demais — mande até 3 minutos.", assistant.ErrVoiceRejected),
			status:  http.StatusUnprocessableEntity,
			message: "Áudio longo demais — mande até 3 minutos.",
		},
		{
			name:    "unavailable",
			err:     fmt.Errorf("%w: dial tcp 127.0.0.1:8765: connection refused", assistant.ErrVoiceUnavailable),
			status:  http.StatusServiceUnavailable,
			message: "A voz não está disponível nesta máquina.",
		},
		{
			name:    "outdated bridge says how to fix it",
			err:     fmt.Errorf("%w: /transcribe", assistant.ErrVoiceOutdated),
			status:  http.StatusServiceUnavailable,
			message: "A ponte do Apple está desatualizada — recompile apple-bridge (swift build -c release) e reinicie o backend.",
		},
		{
			name:    "anything else",
			err:     errors.New("json: cannot unmarshal"),
			status:  http.StatusInternalServerError,
			message: "Não consegui processar o áudio agora. Tente de novo.",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			writeVoiceError(rec, tc.err)
			if rec.Code != tc.status {
				t.Errorf("status = %d, want %d", rec.Code, tc.status)
			}
			var body struct {
				Error string `json:"error"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("body %q: %v", rec.Body.String(), err)
			}
			if body.Error != tc.message {
				t.Errorf("error = %q, want %q", body.Error, tc.message)
			}
		})
	}
}
