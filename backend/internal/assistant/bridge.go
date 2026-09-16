package assistant

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// appleBridge is the Swift helper in apple-bridge/, started on demand. It
// serves both the Apple Intelligence engine and the Mac's speech.
type appleBridge struct {
	bin string
	url string

	mu   sync.Mutex
	proc *exec.Cmd
	// done is closed when proc exits: the goroutine waiting on it owns
	// proc.ProcessState, so liveness is read from here instead.
	done chan struct{}
	// gen counts the processes we started, so whoever caches something about
	// the running bridge can tell a new one apart.
	gen uint64
}

// bridgeError is a failure written for the owner, in pt-BR. Everything else the
// bridge can hit is a raw Go error and must not reach the screen.
type bridgeError string

func (e bridgeError) Error() string { return string(e) }

type appleHealth struct {
	Available bool   `json:"available"`
	Reason    string `json:"reason"`
}

func (b *appleBridge) health(ctx context.Context) (appleHealth, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, b.url+"/health", nil)
	if err != nil {
		return appleHealth{}, err
	}
	client := &http.Client{Timeout: 2 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return appleHealth{}, err
	}
	defer res.Body.Close()
	var h appleHealth
	return h, json.NewDecoder(res.Body).Decode(&h)
}

// ensure starts the helper if it isn't answering and its binary exists.
func (b *appleBridge) ensure(ctx context.Context) (appleHealth, error) {
	if h, err := b.health(ctx); err == nil {
		return h, nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if h, err := b.health(ctx); err == nil {
		return h, nil
	}
	if _, err := os.Stat(b.bin); err != nil {
		return appleHealth{}, bridgeError(fmt.Sprintf("ponte do Apple Intelligence não compilada (%s)", b.bin))
	}
	if !b.running() {
		cmd := exec.Command(b.bin)
		port := "8765"
		if i := strings.LastIndexByte(b.url, ':'); i > 0 {
			port = b.url[i+1:]
		}
		cmd.Env = append(os.Environ(), "APPLE_BRIDGE_PORT="+port)
		if err := cmd.Start(); err != nil {
			return appleHealth{}, err
		}
		done := make(chan struct{})
		b.proc, b.done = cmd, done
		b.gen++
		go func() {
			_ = cmd.Wait()
			close(done)
		}()
		slog.Info("apple intelligence bridge started", "pid", cmd.Process.Pid)
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if h, err := b.health(ctx); err == nil {
			return h, nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return appleHealth{}, bridgeError("a ponte do Apple Intelligence não respondeu")
}

// running reports whether the process we started is still alive. Callers hold
// b.mu; only the goroutine in ensure touches proc.ProcessState.
func (b *appleBridge) running() bool {
	if b.proc == nil || b.done == nil {
		return false
	}
	select {
	case <-b.done:
		return false
	default:
		return true
	}
}

// generation identifies the bridge process currently running.
func (b *appleBridge) generation() uint64 {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.gen
}

func (b *appleBridge) stop() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.running() && b.proc.Process != nil {
		_ = b.proc.Process.Kill()
	}
}
