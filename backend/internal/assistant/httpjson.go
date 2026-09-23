// HTTP helpers shared by the agent engine and the image tools.
package assistant

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

var httpClient = &http.Client{Timeout: 3 * time.Minute}

func postJSON(ctx context.Context, url string, headers map[string]string, body any, out any) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	res, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 8<<20))
	if err != nil {
		return err
	}
	if res.StatusCode >= 300 {
		var e struct {
			Error json.RawMessage `json:"error"`
		}
		_ = json.Unmarshal(raw, &e)
		msg := string(e.Error)
		if msg == "" {
			msg = truncate(string(raw), 300)
		}
		return &apiError{Host: hostOf(url), Status: res.StatusCode, Message: msg}
	}
	return json.Unmarshal(raw, out)
}

// apiError is a non-2xx reply from an engine's HTTP API; the status lets the
// chat explain the failure without guessing from the text.
type apiError struct {
	Host    string
	Status  int
	Message string
}

func (e *apiError) Error() string {
	return fmt.Sprintf("%s respondeu %d: %s", e.Host, e.Status, e.Message)
}

func hostOf(url string) string {
	s := strings.TrimPrefix(strings.TrimPrefix(url, "https://"), "http://")
	if i := strings.Index(s, "/"); i >= 0 {
		s = s[:i]
	}
	return s
}

// userContentOpenAI is the new user turn in the OpenAI format: plain text,
// or text plus the image as a data URL in an image_url block.
func userContentOpenAI(req ChatRequest) any {
	if req.Attachment == nil || req.Attachment.ImageBase64 == "" {
		return req.Message
	}
	type block = map[string]any
	return []block{
		{"type": "text", "text": req.Message},
		{"type": "image_url", "image_url": block{"url": fmt.Sprintf("data:%s;base64,%s", req.Attachment.ImageMediaType, req.Attachment.ImageBase64)}},
	}
}
