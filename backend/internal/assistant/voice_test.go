package assistant

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"unicode/utf8"
)

// fakeBridge serves /health as an Apple Intelligence that is switched off —
// voice must work anyway — and hands every other request to handler.
func fakeBridge(t *testing.T, handler http.HandlerFunc) *Voice {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			_, _ = io.WriteString(w, `{"available":false,"reason":"appleIntelligenceNotEnabled"}`)
			return
		}
		handler(w, r)
	}))
	t.Cleanup(srv.Close)
	return newVoice(&AppleBridge{bin: "/nonexistent/estus-apple-bridge", url: srv.URL}, "Luciana")
}

func TestVoiceTranscribe(t *testing.T) {
	v := fakeBridge(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/speak" { // Status probes the voice surface
			w.WriteHeader(http.StatusUnprocessableEntity)
			return
		}
		body, _ := io.ReadAll(r.Body)
		if r.URL.Path != "/transcribe" || r.Header.Get("X-Filename") != "gravacao.wav" || string(body) != "RIFF" {
			t.Errorf("%s %q %q", r.URL.Path, r.Header.Get("X-Filename"), body)
		}
		_, _ = io.WriteString(w, `{"text":"  gastei 30 no uber ","seconds":2.5}`)
	})
	got, err := v.Transcribe(context.Background(), []byte("RIFF"), "gravacao.wav")
	if err != nil || got.Text != "gastei 30 no uber" || got.Seconds != 2.5 {
		t.Fatalf("Transcribe = %+v, %v", got, err)
	}
	if s := v.Status(context.Background()); !s.Available || !strings.Contains(s.Detail, "Luciana") {
		t.Fatalf("Status = %+v", s)
	}
}

func TestVoiceSpeakSendsPlainText(t *testing.T) {
	v := fakeBridge(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct{ Text, Voice string }
		_ = json.NewDecoder(r.Body).Decode(&req)
		if r.URL.Path != "/speak" || req.Text != "Total: R$ 10,00" || req.Voice != "Luciana" {
			t.Errorf("%s %+v", r.URL.Path, req)
		}
		w.Header().Set("Content-Type", "audio/mp4")
		_, _ = w.Write([]byte("AAC"))
	})
	audio, err := v.Speak(context.Background(), "**Total**: R$ 10,00")
	if err != nil || string(audio) != "AAC" {
		t.Fatalf("Speak = %q, %v", audio, err)
	}
	if _, err := v.Speak(context.Background(), " ** "); !errors.Is(err, ErrVoiceRejected) {
		t.Fatalf("empty Speak err = %v", err)
	}
}

func TestVoiceRejection(t *testing.T) {
	v := fakeBridge(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = io.WriteString(w, `{"error":"Áudio longo demais — mande até 3 minutos."}`)
	})
	_, err := v.Transcribe(context.Background(), []byte("x"), "a.ogg")
	if !errors.Is(err, ErrVoiceRejected) || VoiceErrorMessage(err) != "Áudio longo demais — mande até 3 minutos." {
		t.Fatalf("err = %v, message = %q", err, VoiceErrorMessage(err))
	}
}

const outdatedMessage = "A ponte do Apple está desatualizada — recompile apple-bridge (swift build -c release) e reinicie o backend."

func TestAnOldBridgeCountsAsOutdated(t *testing.T) {
	v := fakeBridge(t, http.NotFound)
	_, err := v.Transcribe(context.Background(), []byte("x"), "a.wav")
	if !errors.Is(err, ErrVoiceOutdated) || VoiceErrorMessage(err) != outdatedMessage {
		t.Fatalf("err = %v, message = %q", err, VoiceErrorMessage(err))
	}
}

// A bridge built before voice still answers /health, so Status has to ask the
// voice surface itself — otherwise the settings screen advertises a voice that
// fails on the first recording.
func TestStatusSpotsAnOutdatedBridge(t *testing.T) {
	v := fakeBridge(t, http.NotFound)
	if s := v.Status(context.Background()); s.Available || s.Detail != outdatedMessage {
		t.Fatalf("Status = %+v", s)
	}
}

func TestStatusProbesOncePerBridgeProcess(t *testing.T) {
	var probes atomic.Int32
	v := fakeBridge(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/speak" {
			t.Errorf("unexpected request %s", r.URL.Path)
			return
		}
		probes.Add(1)
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = io.WriteString(w, `{"error":"Nada para ler."}`)
	})
	for range 3 {
		if s := v.Status(context.Background()); !s.Available {
			t.Fatalf("Status = %+v", s)
		}
	}
	if got := probes.Load(); got != 1 {
		t.Fatalf("probed %d times, want 1", got)
	}
}

// Whatever goes wrong starting the bridge, the chat must not show a raw Go
// error (here: exec refusing to run a directory).
func TestStatusHidesRawErrors(t *testing.T) {
	v := newVoice(&AppleBridge{bin: t.TempDir(), url: "http://127.0.0.1:1"}, "Luciana")
	if s := v.Status(context.Background()); s.Available || s.Detail != "Voz indisponível: a ponte do Apple não respondeu a tempo." {
		t.Fatalf("Status = %+v", s)
	}
}

func TestVoiceWithoutABridge(t *testing.T) {
	v := newVoice(&AppleBridge{bin: "/nonexistent/estus-apple-bridge", url: "http://127.0.0.1:1"}, "")
	if s := v.Status(context.Background()); s.Available || !strings.Contains(s.Detail, "não compilada") {
		t.Fatalf("Status = %+v", s)
	}
	_, err := v.Transcribe(context.Background(), []byte("x"), "a.wav")
	if !errors.Is(err, ErrVoiceUnavailable) || VoiceErrorMessage(err) != "A voz não está disponível nesta máquina." {
		t.Fatalf("err = %v", err)
	}
}

func TestSpeakableText(t *testing.T) {
	cases := []struct{ in, want string }{
		{"**Total**: R$ 1.500,00", "Total: R$ 1.500,00"},
		{"## Hoje\n- ler\n- *correr*\n1. nadar", "Hoje\nler\ncorrer\nnadar"},
		{"veja [o site](https://x.com/a)", "veja o site"},
		{"rode `ls` e\n```go\nfmt.Println()\n```\nfim", "rode ls e\n(código omitido)\nfim"},
		{"   \n\n  ", ""},
	}
	for _, tc := range cases {
		if got := SpeakableText(tc.in); got != tc.want {
			t.Errorf("SpeakableText(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
	long := SpeakableText(strings.Repeat("Uma frase curta aqui. ", 300))
	if utf8.RuneCountInString(long) > 4000 || !strings.HasSuffix(long, ".") {
		t.Fatalf("long text: %d runes, ends %q", utf8.RuneCountInString(long), long[len(long)-10:])
	}
}
