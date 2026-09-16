package assistant

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"
)

// The Mac's own speech, through the Apple bridge: recorded audio to text, and
// answers to audio. Nothing leaves the machine, and it works whatever text
// engine is chosen.

var (
	// ErrVoiceUnavailable means this machine has no speech: no bridge, or not a Mac.
	ErrVoiceUnavailable = errors.New("voice unavailable")
	// ErrVoiceRejected is a request the bridge refused; the message says why.
	ErrVoiceRejected = errors.New("voice request rejected")
	// ErrVoiceOutdated is a bridge built before it could listen and speak: it
	// answers /health like any other, so only the voice endpoints give it away.
	ErrVoiceOutdated = errors.New("voice bridge outdated")
)

const (
	voiceUnavailableMessage = "A voz não está disponível nesta máquina."
	voiceOutdatedMessage    = "A ponte do Apple está desatualizada — recompile apple-bridge (swift build -c release) e reinicie o backend."
	voiceTimeoutMessage     = "Voz indisponível: a ponte do Apple não respondeu a tempo."
)

type Transcript struct {
	Text    string  `json:"text"`
	Seconds float64 `json:"seconds"`
}

type VoiceStatus struct {
	Available bool   `json:"available"`
	Detail    string `json:"detail"`
}

type Voice struct {
	bridge *appleBridge
	name   string
	client *http.Client

	mu        sync.Mutex
	probed    bool
	probedGen uint64
	probeErr  error
}

func newVoice(bridge *appleBridge, name string) *Voice {
	if strings.TrimSpace(name) == "" {
		name = "Luciana"
	}
	return &Voice{bridge: bridge, name: name, client: &http.Client{Timeout: 2 * time.Minute}}
}

func (v *Voice) Status(ctx context.Context) VoiceStatus {
	if _, err := v.bridge.ensure(ctx); err != nil {
		// Only failures we wrote for the owner are worth showing; a raw Go
		// error (a timeout, exec refusing to start it) says nothing to anyone.
		var explained bridgeError
		if !errors.As(err, &explained) {
			return VoiceStatus{Detail: voiceTimeoutMessage}
		}
		return VoiceStatus{Detail: "Voz indisponível: " + err.Error()}
	}
	if err := v.probe(ctx); err != nil {
		return VoiceStatus{Detail: VoiceErrorMessage(err)}
	}
	return VoiceStatus{Available: true, Detail: fmt.Sprintf("Voz do Mac (%s), offline", v.name)}
}

// probe asks the bridge whether it has the voice endpoints at all: /health
// alone would advertise a pre-voice bridge, and every recording would then
// fail. An empty /speak is the cheapest question — the current bridge refuses
// it with 422, an old one 404s. The answer is kept for as long as that bridge
// process lives, so loading the settings doesn't POST every time.
func (v *Voice) probe(ctx context.Context) error {
	gen := v.bridge.generation()
	v.mu.Lock()
	if v.probed && v.probedGen == gen {
		defer v.mu.Unlock()
		return v.probeErr
	}
	v.mu.Unlock()

	payload, _ := json.Marshal(map[string]string{"text": ""})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, v.bridge.url+"/speak", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := v.client.Do(req)
	if err != nil {
		// A transport failure says nothing about the bridge's age; don't cache it.
		return fmt.Errorf("%w: %v", ErrVoiceUnavailable, err)
	}
	defer res.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 4<<10))
	var outdated error
	if res.StatusCode == http.StatusNotFound {
		outdated = fmt.Errorf("%w: /speak", ErrVoiceOutdated)
	}
	v.mu.Lock()
	v.probed, v.probedGen, v.probeErr = true, gen, outdated
	v.mu.Unlock()
	return outdated
}

// Available is Status as a yes or no.
func (v *Voice) Available(ctx context.Context) bool { return v.Status(ctx).Available }

// Transcribe turns recorded audio into text; filename's extension tells the format.
func (v *Voice) Transcribe(ctx context.Context, audio []byte, filename string) (Transcript, error) {
	var t Transcript
	raw, err := v.post(ctx, "/transcribe", audio, map[string]string{"Content-Type": "application/octet-stream", "X-Filename": filename})
	if err != nil {
		return t, err
	}
	if err := json.Unmarshal(raw, &t); err != nil {
		return t, fmt.Errorf("voice transcribe: %w", err)
	}
	t.Text = strings.TrimSpace(t.Text)
	return t, nil
}

// Speak reads an answer aloud: Markdown is dropped first, so the voice
// doesn't spell out the formatting. It returns AAC audio.
func (v *Voice) Speak(ctx context.Context, text string) ([]byte, error) {
	text = SpeakableText(text)
	if text == "" {
		return nil, fmt.Errorf("%w: Nada para ler.", ErrVoiceRejected)
	}
	payload, _ := json.Marshal(map[string]string{"text": text, "voice": v.name})
	return v.post(ctx, "/speak", payload, map[string]string{"Content-Type": "application/json"})
}

func (v *Voice) post(ctx context.Context, path string, body []byte, headers map[string]string) ([]byte, error) {
	if _, err := v.bridge.ensure(ctx); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrVoiceUnavailable, err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, v.bridge.url+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	for k, val := range headers {
		req.Header.Set(k, val)
	}
	res, err := v.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrVoiceUnavailable, err)
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 20<<20))
	if err != nil {
		return nil, fmt.Errorf("voice %s: %w", path, err)
	}
	switch res.StatusCode {
	case http.StatusOK:
		return raw, nil
	case http.StatusNotFound:
		// A bridge built before it could listen and speak.
		return nil, fmt.Errorf("%w: %s", ErrVoiceOutdated, path)
	case http.StatusRequestEntityTooLarge:
		return nil, fmt.Errorf("%w: Áudio longo demais — mande até 3 minutos.", ErrVoiceRejected)
	}
	var e struct {
		Error string `json:"error"`
	}
	_ = json.Unmarshal(raw, &e)
	if res.StatusCode == http.StatusUnprocessableEntity && e.Error != "" {
		return nil, fmt.Errorf("%w: %s", ErrVoiceRejected, e.Error)
	}
	return nil, fmt.Errorf("voice %s: bridge answered %d: %s", path, res.StatusCode, e.Error)
}

// VoiceErrorMessage is the sentence to show for a voice failure.
func VoiceErrorMessage(err error) string {
	switch {
	case errors.Is(err, ErrVoiceRejected):
		return strings.TrimPrefix(err.Error(), ErrVoiceRejected.Error()+": ")
	case errors.Is(err, ErrVoiceOutdated):
		return voiceOutdatedMessage
	case errors.Is(err, ErrVoiceUnavailable):
		return voiceUnavailableMessage
	default:
		return "Não consegui processar o áudio agora. Tente de novo."
	}
}

const maxSpeakable = 4000

var (
	mdFence   = regexp.MustCompile("(?s)```.*?```")
	mdLink    = regexp.MustCompile(`\[([^\]]+)\]\([^)]*\)`)
	mdCode    = regexp.MustCompile("`([^`]*)`")
	mdHeading = regexp.MustCompile(`^#{1,6}\s+`)
	mdBullet  = regexp.MustCompile(`^(?:[-*+]|\d+[.)])\s+`)
	mdEmph    = regexp.MustCompile(`[*_~]+`)
	mdSpaces  = regexp.MustCompile(`[ \t]+`)
)

// SpeakableText is Markdown as it should be read aloud: no formatting marks,
// code replaced by a mention, at most maxSpeakable characters, cut at the end
// of a sentence.
func SpeakableText(md string) string {
	s := mdFence.ReplaceAllString(md, "\n(código omitido)\n")
	s = mdLink.ReplaceAllString(s, "$1")
	s = mdCode.ReplaceAllString(s, "$1")
	var lines []string
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		line = mdHeading.ReplaceAllString(line, "")
		line = mdBullet.ReplaceAllString(line, "")
		line = mdEmph.ReplaceAllString(line, "")
		line = strings.TrimSpace(mdSpaces.ReplaceAllString(line, " "))
		if line != "" {
			lines = append(lines, line)
		}
	}
	s = strings.Join(lines, "\n")
	runes := []rune(s)
	if len(runes) <= maxSpeakable {
		return s
	}
	cut := string(runes[:maxSpeakable])
	if i := strings.LastIndexAny(cut, ".!?\n"); i > len(cut)/2 {
		cut = cut[:i+1]
	}
	return strings.TrimSpace(cut)
}
