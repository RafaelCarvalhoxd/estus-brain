package telegram

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const testToken = "SECRET-TOKEN"

// fakeBotAPI serves /bot<token>/<method>, handing each call's JSON body to reply.
func fakeBotAPI(t *testing.T, reply func(method string, body map[string]any) (int, string)) (*Client, func()) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		prefix := "/bot" + testToken + "/"
		if !strings.HasPrefix(r.URL.Path, prefix) {
			http.NotFound(w, r)
			return
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		status, out := reply(strings.TrimPrefix(r.URL.Path, prefix), body)
		w.WriteHeader(status)
		_, _ = io.WriteString(w, out)
	}))
	return NewClient(srv.URL, testToken), srv.Close
}

func TestClientGetMe(t *testing.T) {
	c, stop := fakeBotAPI(t, func(method string, _ map[string]any) (int, string) {
		if method != "getMe" {
			t.Errorf("method = %q", method)
		}
		return 200, `{"ok":true,"result":{"id":1,"first_name":"Estus","username":"estus_bot"}}`
	})
	defer stop()
	me, err := c.GetMe(context.Background())
	if err != nil || me.Username != "estus_bot" {
		t.Fatalf("GetMe = %+v, %v", me, err)
	}
}

func TestClientGetUpdates(t *testing.T) {
	c, stop := fakeBotAPI(t, func(method string, body map[string]any) (int, string) {
		if method != "getUpdates" || body["offset"] != float64(42) || body["timeout"] != float64(30) {
			t.Errorf("%s %v", method, body)
		}
		return 200, `{"ok":true,"result":[{"update_id":42,"message":{"message_id":7,"from":{"id":99,"first_name":"Rafa"},"chat":{"id":99,"type":"private"},"text":"oi"}}]}`
	})
	defer stop()
	updates, err := c.GetUpdates(context.Background(), 42, 30)
	if err != nil || len(updates) != 1 || updates[0].Message.Text != "oi" || updates[0].Message.From.FirstName != "Rafa" {
		t.Fatalf("GetUpdates = %+v, %v", updates, err)
	}
}

func TestClientSendMessageWithButtons(t *testing.T) {
	c, stop := fakeBotAPI(t, func(method string, body map[string]any) (int, string) {
		raw, _ := json.Marshal(body["reply_markup"])
		if method != "sendMessage" || body["parse_mode"] != "HTML" || body["chat_id"] != float64(99) ||
			string(raw) != `{"inline_keyboard":[[{"callback_data":"done:r1","text":"✅ Concluído"}]]}` {
			t.Errorf("%s %v %s", method, body, raw)
		}
		return 200, `{"ok":true,"result":{"message_id":5,"chat":{"id":99,"type":"private"}}}`
	})
	defer stop()
	m, err := c.SendMessage(context.Background(), 99, "<b>oi</b>", true, []Button{{Text: "✅ Concluído", CallbackData: "done:r1"}})
	if err != nil || m.MessageID != 5 {
		t.Fatalf("SendMessage = %+v, %v", m, err)
	}
}

func TestClientAPIError(t *testing.T) {
	c, stop := fakeBotAPI(t, func(string, map[string]any) (int, string) {
		return 401, `{"ok":false,"error_code":401,"description":"Unauthorized"}`
	})
	defer stop()
	_, err := c.GetMe(context.Background())
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Code != 401 {
		t.Fatalf("err = %v", err)
	}
}

func TestClientErrorsNeverCarryTheToken(t *testing.T) {
	c, stop := fakeBotAPI(t, func(string, map[string]any) (int, string) { return 200, `{"ok":true}` })
	stop() // nothing listens any more: the request fails in transport
	_, err := c.GetMe(context.Background())
	if err == nil || strings.Contains(err.Error(), testToken) {
		t.Fatalf("err = %v", err)
	}
}

func TestClientDownloadsAFile(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/bot" + testToken + "/getFile":
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body["file_id"] != "f1" {
				t.Errorf("file_id = %v", body["file_id"])
			}
			_, _ = io.WriteString(w, `{"ok":true,"result":{"file_id":"f1","file_size":3,"file_path":"voice/file_7.oga"}}`)
		case "/file/bot" + testToken + "/voice/file_7.oga":
			_, _ = io.WriteString(w, "OGG")
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	c := NewClient(srv.URL, testToken)

	f, err := c.GetFile(context.Background(), "f1")
	if err != nil || f.FilePath != "voice/file_7.oga" {
		t.Fatalf("GetFile = %+v, %v", f, err)
	}
	data, err := c.DownloadFile(context.Background(), f.FilePath)
	if err != nil || string(data) != "OGG" {
		t.Fatalf("DownloadFile = %q, %v", data, err)
	}
	_, err = c.DownloadFile(context.Background(), "voice/missing.oga")
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Code != 404 || strings.Contains(err.Error(), testToken) {
		t.Fatalf("missing file err = %v", err)
	}
}

func TestClientDownloadErrorsNeverCarryTheToken(t *testing.T) {
	c, stop := fakeBotAPI(t, func(string, map[string]any) (int, string) { return 200, `{"ok":true}` })
	stop()
	_, err := c.DownloadFile(context.Background(), "voice/file_7.oga")
	if err == nil || strings.Contains(err.Error(), testToken) {
		t.Fatalf("err = %v", err)
	}
}
