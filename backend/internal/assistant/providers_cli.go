package assistant

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// The engines that run on the owner's own subscription, through the official
// CLIs they're already signed in to: Claude Code (Claude Pro/Max) and Codex
// (ChatGPT). Neither gets a shell or file access — only the Estus Brain MCP
// server. These are for personal use: ASSISTANT_MULTI_USER switches them off.

const cliTimeout = 5 * time.Minute

func (c *Chat) workDir(name string) (string, error) {
	dir, err := filepath.Abs(filepath.Join(c.cfg.Dir, name))
	if err != nil {
		return "", err
	}
	return dir, os.MkdirAll(dir, 0o700)
}

func runStatus(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

// scanLines feeds each stdout line of a started command to fn.
func scanLines(cmd *exec.Cmd, fn func(line []byte)) (stderr *strings.Builder, err error) {
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr = &strings.Builder{}
	cmd.Stderr = &limitedWriter{b: stderr, max: 8000}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	sc := bufio.NewScanner(stdout)
	sc.Buffer(make([]byte, 1<<20), 16<<20)
	for sc.Scan() {
		fn(sc.Bytes())
	}
	return stderr, cmd.Wait()
}

type limitedWriter struct {
	b   *strings.Builder
	max int
}

func (w *limitedWriter) Write(p []byte) (int, error) {
	if room := w.max - w.b.Len(); room > 0 {
		if len(p) > room {
			w.b.Write(p[:room])
		} else {
			w.b.Write(p)
		}
	}
	return len(p), nil
}

// historyPrompt folds earlier turns into the prompt when a CLI can't resume
// its own session (a new engine for this conversation, or a lost session).
func historyPrompt(req ChatRequest) string {
	if len(req.History) == 0 {
		return req.Message
	}
	var b strings.Builder
	b.WriteString("Conversa até aqui:\n")
	start := max(0, len(req.History)-12)
	for _, m := range req.History[start:] {
		who := "Pessoa"
		if m.Role == "assistant" {
			who = "Assistente"
		}
		fmt.Fprintf(&b, "%s: %s\n", who, truncate(m.Content, 1200))
	}
	b.WriteString("\nNova mensagem da pessoa: ")
	b.WriteString(req.Message)
	return b.String()
}

// toolText pulls the text out of an MCP tool result, parsing it as JSON when it is.
func toolText(content json.RawMessage) any {
	var parts []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	text := ""
	if json.Unmarshal(content, &parts) == nil {
		for _, p := range parts {
			text += p.Text
		}
	} else {
		var s string
		if json.Unmarshal(content, &s) == nil {
			text = s
		}
	}
	var parsed any
	if json.Unmarshal([]byte(text), &parsed) == nil {
		return parsed
	}
	return text
}

// ---------------------------------------------------------------- Claude Code

type claudeCodeProvider struct{ chat *Chat }

func (p *claudeCodeProvider) ID() string { return "claude_code" }

func (p *claudeCodeProvider) Status(ctx context.Context) ProviderStatus {
	s, _ := p.chat.repo.Settings(ctx)
	st := ProviderStatus{
		ID: p.ID(), Name: "Claude (sua conta)", Kind: "login", Model: p.chat.model(s, p.ID()), Models: []string{"sonnet", "opus", "haiku"},
		Capabilities: Capabilities{SupportsVision: true, SupportsImageGen: true},
	}
	if p.chat.cfg.MultiUser {
		st.Detail = "Desligado no modo multiusuário"
		return st
	}
	if _, err := exec.LookPath("claude"); err != nil {
		st.Detail = "Instale o Claude Code e rode `claude` uma vez para entrar na sua conta"
		return st
	}
	out, err := runStatus(ctx, "claude", "auth", "status")
	if err != nil || strings.Contains(strings.ToLower(out), "not logged") || strings.Contains(out, `"loggedIn": false`) {
		st.Detail = "Claude Code instalado, mas sem login: rode `claude` e entre na sua conta"
		return st
	}
	st.Available = true
	st.Detail = "Usando o login do Claude Code nesta máquina"
	return st
}

func (p *claudeCodeProvider) Chat(ctx context.Context, req ChatRequest, emit func(Event)) (ChatOutcome, error) {
	out, err := p.run(ctx, req, emit, req.SessionID)
	if err != nil && req.SessionID != "" && out.Text == "" && len(out.ToolCalls) == 0 {
		// The saved session may be gone; start over with the history inline.
		return p.run(ctx, req, emit, "")
	}
	return out, err
}

func (p *claudeCodeProvider) run(ctx context.Context, req ChatRequest, emit func(Event), session string) (ChatOutcome, error) {
	dir, err := p.chat.workDir("claude-work")
	if err != nil {
		return ChatOutcome{}, err
	}
	mcpConfig, _ := json.Marshal(map[string]any{"mcpServers": map[string]any{"estus": map[string]any{
		"type": "http", "url": p.chat.cfg.MCPURL, "headers": map[string]string{"Authorization": "Bearer " + p.chat.cfg.MCPToken},
	}}})
	prompt := req.Message
	if session == "" {
		prompt = historyPrompt(req)
	}
	args := []string{
		"-p", prompt,
		"--output-format", "stream-json", "--verbose", "--include-partial-messages",
		"--mcp-config", string(mcpConfig), "--strict-mcp-config",
		"--tools", "", "--allowedTools", "mcp__estus",
		"--setting-sources", "",
		"--append-system-prompt", req.System,
	}
	if req.Model != "" {
		args = append(args, "--model", req.Model)
	}
	if session != "" {
		args = append(args, "--resume", session)
	}
	ctx, cancel := context.WithTimeout(ctx, cliTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "claude", args...)
	cmd.Dir = dir

	var out ChatOutcome
	var streamed strings.Builder
	var resultText, resultErr string
	pending := map[string]string{} // tool_use id → name
	stderr, runErr := scanLines(cmd, func(line []byte) {
		var e struct {
			Type      string `json:"type"`
			Subtype   string `json:"subtype"`
			SessionID string `json:"session_id"`
			Event     struct {
				Type  string `json:"type"`
				Delta struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"delta"`
			} `json:"event"`
			Message struct {
				Content []struct {
					Type      string          `json:"type"`
					ID        string          `json:"id"`
					Name      string          `json:"name"`
					Input     json.RawMessage `json:"input"`
					ToolUseID string          `json:"tool_use_id"`
					Content   json.RawMessage `json:"content"`
					IsError   bool            `json:"is_error"`
				} `json:"content"`
			} `json:"message"`
			Result  string `json:"result"`
			IsError bool   `json:"is_error"`
		}
		if json.Unmarshal(line, &e) != nil {
			return
		}
		if e.SessionID != "" {
			out.SessionID = e.SessionID
		}
		switch e.Type {
		case "stream_event":
			if e.Event.Type == "content_block_delta" && e.Event.Delta.Type == "text_delta" {
				streamed.WriteString(e.Event.Delta.Text)
				emit(Event{Type: "text", Text: e.Event.Delta.Text})
			}
		case "assistant":
			for _, c := range e.Message.Content {
				if c.Type == "tool_use" {
					name := strings.TrimPrefix(c.Name, "mcp__estus__")
					pending[c.ID] = name
					emit(Event{Type: "tool", ToolID: c.ID, Tool: name, Args: c.Input})
					out.ToolCalls = append(out.ToolCalls, ToolCall{ID: c.ID, Name: name, Args: c.Input})
				}
				if c.Type == "text" && streamed.Len() > 0 {
					// Separate text blocks around tool calls.
					streamed.WriteString("\n\n")
					emit(Event{Type: "text", Text: "\n\n"})
				}
			}
		case "user":
			for _, c := range e.Message.Content {
				if c.Type != "tool_result" {
					continue
				}
				result := toolText(c.Content)
				ev := Event{Type: "tool_result", ToolID: c.ToolUseID, Tool: pending[c.ToolUseID]}
				for i := range out.ToolCalls {
					if out.ToolCalls[i].ID == c.ToolUseID {
						if c.IsError {
							out.ToolCalls[i].Error = fmt.Sprint(result)
						} else {
							out.ToolCalls[i].Result = result
						}
					}
				}
				if c.IsError {
					ev.Error = fmt.Sprint(result)
				} else {
					ev.Result = result
				}
				emit(ev)
			}
		case "result":
			resultText = e.Result
			if e.IsError {
				resultErr = e.Result
				if resultErr == "" {
					resultErr = e.Subtype
				}
			}
		}
	})
	out.Text = strings.TrimSpace(streamed.String())
	if out.Text == "" && resultErr == "" {
		out.Text = strings.TrimSpace(resultText)
		if out.Text != "" {
			emit(Event{Type: "text", Text: out.Text})
		}
	}
	if resultErr != "" {
		return out, fmt.Errorf("Claude Code: %s", truncate(resultErr, 300))
	}
	if runErr != nil && out.Text == "" {
		return out, fmt.Errorf("Claude Code falhou: %s", cliError(runErr, stderr))
	}
	return out, nil
}

func cliError(err error, stderr *strings.Builder) string {
	if errors.Is(err, context.DeadlineExceeded) {
		return "demorou demais para responder"
	}
	msg := ""
	if stderr != nil {
		for _, line := range strings.Split(stderr.String(), "\n") {
			// Codex logs noisy cache warnings; keep only real errors.
			if strings.TrimSpace(line) != "" && !strings.Contains(line, "models_manager") {
				msg = line
			}
		}
	}
	if msg == "" {
		msg = err.Error()
	}
	return truncate(msg, 300)
}

// ---------------------------------------------------------------- Codex

type codexProvider struct{ chat *Chat }

func (p *codexProvider) ID() string { return "codex" }

func (p *codexProvider) Status(ctx context.Context) ProviderStatus {
	s, _ := p.chat.repo.Settings(ctx)
	st := ProviderStatus{
		ID: p.ID(), Name: "GPT (sua conta ChatGPT)", Kind: "login", Model: p.chat.model(s, p.ID()),
		// Codex's own built-in image_generation is turned off (codexDisabled
		// below); SupportsImageGen is true anyway because it reaches
		// generate_image the same way it reaches every tool, via MCP.
		Capabilities: Capabilities{SupportsVision: true, SupportsImageGen: true},
	}
	if p.chat.cfg.MultiUser {
		st.Detail = "Desligado no modo multiusuário"
		return st
	}
	if _, err := exec.LookPath("codex"); err != nil {
		st.Detail = "Instale o Codex CLI e rode `codex login` com sua conta ChatGPT"
		return st
	}
	out, err := runStatus(ctx, "codex", "login", "status")
	if err != nil || !strings.Contains(strings.ToLower(out), "logged in") {
		st.Detail = "Codex instalado, mas sem login: rode `codex login`"
		return st
	}
	st.Available = true
	st.Detail = "Usando o login do ChatGPT no Codex nesta máquina"
	return st
}

// Everything Codex could do besides talking to the MCP server is switched off.
var codexDisabled = []string{"shell_tool", "unified_exec", "browser_use", "browser_use_external", "computer_use", "apps", "plugins", "image_generation", "multi_agent", "hooks", "in_app_browser", "memories"}

func (p *codexProvider) Chat(ctx context.Context, req ChatRequest, emit func(Event)) (ChatOutcome, error) {
	out, err := p.run(ctx, req, emit, req.SessionID)
	if err != nil && req.SessionID != "" && out.Text == "" && len(out.ToolCalls) == 0 {
		return p.run(ctx, req, emit, "")
	}
	return out, err
}

func (p *codexProvider) run(ctx context.Context, req ChatRequest, emit func(Event), session string) (ChatOutcome, error) {
	dir, err := p.chat.workDir("codex-work")
	if err != nil {
		return ChatOutcome{}, err
	}
	flags := []string{
		"--json", "--skip-git-repo-check",
		"-c", `approval_policy="never"`,
		"-c", fmt.Sprintf("mcp_servers.estus.url=%q", p.chat.cfg.MCPURL),
		"-c", `mcp_servers.estus.bearer_token_env_var="ESTUS_MCP_TOKEN"`,
	}
	for _, f := range codexDisabled {
		flags = append(flags, "--disable", f)
	}
	if req.Model != "" {
		flags = append(flags, "-m", req.Model)
	}
	var args []string
	if session != "" {
		args = append([]string{"exec", "resume"}, flags...)
		args = append(args, session, req.Message)
	} else {
		args = append([]string{"exec", "-s", "read-only"}, flags...)
		args = append(args, "Instruções do sistema:\n"+req.System+"\n\n"+historyPrompt(req))
	}
	ctx, cancel := context.WithTimeout(ctx, cliTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "codex", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "ESTUS_MCP_TOKEN="+p.chat.cfg.MCPToken)
	cmd.Stdin = strings.NewReader("")

	var out ChatOutcome
	var texts []string
	var failure string
	stderr, runErr := scanLines(cmd, func(line []byte) {
		var e struct {
			Type     string `json:"type"`
			ThreadID string `json:"thread_id"`
			Message  string `json:"message"`
			Error    struct {
				Message string `json:"message"`
			} `json:"error"`
			Item struct {
				ID        string          `json:"id"`
				Type      string          `json:"type"`
				Tool      string          `json:"tool"`
				Arguments json.RawMessage `json:"arguments"`
				Result    *struct {
					Content json.RawMessage `json:"content"`
				} `json:"result"`
				Error json.RawMessage `json:"error"`
				Text  string          `json:"text"`
			} `json:"item"`
		}
		if json.Unmarshal(line, &e) != nil {
			return
		}
		switch e.Type {
		case "thread.started":
			out.SessionID = e.ThreadID
		case "item.started":
			if e.Item.Type == "mcp_tool_call" {
				emit(Event{Type: "tool", ToolID: e.Item.ID, Tool: e.Item.Tool, Args: e.Item.Arguments})
				out.ToolCalls = append(out.ToolCalls, ToolCall{ID: e.Item.ID, Name: e.Item.Tool, Args: e.Item.Arguments})
			}
		case "item.completed":
			switch e.Item.Type {
			case "mcp_tool_call":
				ev := Event{Type: "tool_result", ToolID: e.Item.ID, Tool: e.Item.Tool}
				errText := strings.Trim(string(e.Item.Error), `"`)
				if errText == "null" {
					errText = ""
				}
				var result any
				if e.Item.Result != nil {
					result = toolText(e.Item.Result.Content)
				}
				if errText != "" {
					ev.Error = errText
				} else {
					ev.Result = result
				}
				for i := range out.ToolCalls {
					if out.ToolCalls[i].ID == e.Item.ID {
						out.ToolCalls[i].Result, out.ToolCalls[i].Error = result, errText
					}
				}
				emit(ev)
			case "agent_message":
				if strings.TrimSpace(e.Item.Text) != "" {
					if len(texts) > 0 {
						emit(Event{Type: "text", Text: "\n\n"})
					}
					texts = append(texts, e.Item.Text)
					emit(Event{Type: "text", Text: e.Item.Text})
				}
			}
		case "turn.failed", "error":
			failure = e.Error.Message
			if failure == "" {
				failure = e.Message
			}
		}
	})
	out.Text = strings.Join(texts, "\n\n")
	if failure != "" && out.Text == "" {
		return out, fmt.Errorf("Codex: %s", truncate(failure, 300))
	}
	if runErr != nil && out.Text == "" {
		return out, fmt.Errorf("Codex falhou: %s", cliError(runErr, stderr))
	}
	return out, nil
}
