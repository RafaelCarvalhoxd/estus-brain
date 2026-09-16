package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// DefaultBaseURL is the Bot API; tests point the client at a fake one.
const DefaultBaseURL = "https://api.telegram.org"

// Client is the small slice of the Bot API the bot uses, over plain HTTP.
type Client struct {
	base  string
	token string
	http  *http.Client
}

func NewClient(baseURL, token string) *Client {
	// Longer than a getUpdates long poll, which Telegram holds for up to 30 s.
	return &Client{base: strings.TrimRight(baseURL, "/"), token: token, http: &http.Client{Timeout: 45 * time.Second}}
}

type User struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name"`
	Username  string `json:"username"`
}

type Chat struct {
	ID   int64  `json:"id"`
	Type string `json:"type"` // private | group | supergroup | channel
}

type Message struct {
	MessageID int64 `json:"message_id"`
	From      *User `json:"from"`
	Chat      Chat  `json:"chat"`
	// Date is when the message was sent, in Unix seconds. It matters because
	// Telegram queues messages for up to 24h while the bot is off: what
	// arrives now may have been written hours ago.
	Date  int64  `json:"date"`
	Text  string `json:"text"`
	Voice *Voice `json:"voice"`
	Audio *Audio `json:"audio"`
}

// Voice is a voice note recorded in Telegram (Ogg/Opus).
type Voice struct {
	FileID   string `json:"file_id"`
	Duration int    `json:"duration"` // seconds
	MimeType string `json:"mime_type"`
}

// Audio is an audio file sent as such (a memo shared from another app).
type Audio struct {
	FileID   string `json:"file_id"`
	Duration int    `json:"duration"`
	MimeType string `json:"mime_type"`
	FileName string `json:"file_name"`
}

// File is where a file can be downloaded from, for about an hour.
type File struct {
	FileID   string `json:"file_id"`
	FileSize int64  `json:"file_size"`
	FilePath string `json:"file_path"`
}

type CallbackQuery struct {
	ID      string   `json:"id"`
	From    User     `json:"from"`
	Message *Message `json:"message"`
	Data    string   `json:"data"`
}

type Update struct {
	UpdateID      int64          `json:"update_id"`
	Message       *Message       `json:"message"`
	CallbackQuery *CallbackQuery `json:"callback_query"`
}

// Button is an inline keyboard button that calls back with CallbackData.
type Button struct {
	Text         string `json:"text"`
	CallbackData string `json:"callback_data"`
}

// APIError is a request Telegram refused; Code is its error_code.
type APIError struct {
	Code        int
	Description string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("telegram respondeu %d: %s", e.Code, e.Description)
}

func (c *Client) call(ctx context.Context, method string, params, out any) error {
	body, err := json.Marshal(params)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+"/bot"+c.token+"/"+method, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("telegram %s: invalid request", method)
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := c.http.Do(req)
	if err != nil {
		// *url.Error prints the URL, and the URL carries the token.
		var urlErr *url.Error
		if errors.As(err, &urlErr) {
			err = urlErr.Err
		}
		return fmt.Errorf("telegram %s: %w", method, err)
	}
	defer res.Body.Close()
	var envelope struct {
		OK          bool            `json:"ok"`
		Result      json.RawMessage `json:"result"`
		ErrorCode   int             `json:"error_code"`
		Description string          `json:"description"`
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, 8<<20)).Decode(&envelope); err != nil {
		return &APIError{Code: res.StatusCode, Description: "resposta inválida"}
	}
	if !envelope.OK {
		code := envelope.ErrorCode
		if code == 0 {
			code = res.StatusCode
		}
		return &APIError{Code: code, Description: envelope.Description}
	}
	if out == nil || len(envelope.Result) == 0 {
		return nil
	}
	return json.Unmarshal(envelope.Result, out)
}

func (c *Client) GetMe(ctx context.Context) (User, error) {
	var u User
	err := c.call(ctx, "getMe", struct{}{}, &u)
	return u, err
}

// GetUpdates long-polls for new messages and button taps from offset on.
func (c *Client) GetUpdates(ctx context.Context, offset int64, timeout int) ([]Update, error) {
	var updates []Update
	err := c.call(ctx, "getUpdates", map[string]any{
		"offset":          offset,
		"timeout":         timeout,
		"allowed_updates": []string{"message", "callback_query"},
	}, &updates)
	return updates, err
}

func (c *Client) SendMessage(ctx context.Context, chatID int64, text string, html bool, buttons []Button) (Message, error) {
	params := map[string]any{
		"chat_id":              chatID,
		"text":                 text,
		"link_preview_options": map[string]bool{"is_disabled": true},
	}
	if html {
		params["parse_mode"] = "HTML"
	}
	if len(buttons) > 0 {
		params["reply_markup"] = map[string]any{"inline_keyboard": [][]Button{buttons}}
	}
	var m Message
	err := c.call(ctx, "sendMessage", params, &m)
	return m, err
}

// SendKeyboard sends a message with a menu under it, one button per entry
// of each row.
func (c *Client) SendKeyboard(ctx context.Context, chatID int64, text string, rows [][]Button) (Message, error) {
	var m Message
	err := c.call(ctx, "sendMessage", map[string]any{
		"chat_id":              chatID,
		"text":                 text,
		"parse_mode":           "HTML",
		"link_preview_options": map[string]bool{"is_disabled": true},
		"reply_markup":         map[string]any{"inline_keyboard": rows},
	}, &m)
	return m, err
}

// EditKeyboard replaces a menu message in place, so walking the menu leaves
// one message in the chat instead of a trail.
func (c *Client) EditKeyboard(ctx context.Context, chatID, messageID int64, text string, rows [][]Button) error {
	return c.call(ctx, "editMessageText", map[string]any{
		"chat_id":      chatID,
		"message_id":   messageID,
		"text":         text,
		"parse_mode":   "HTML",
		"reply_markup": map[string]any{"inline_keyboard": rows},
	}, nil)
}

// SendChatAction shows "digitando…"; Telegram clears it after about 5 s.
func (c *Client) SendChatAction(ctx context.Context, chatID int64) error {
	return c.call(ctx, "sendChatAction", map[string]any{"chat_id": chatID, "action": "typing"}, nil)
}

func (c *Client) AnswerCallbackQuery(ctx context.Context, id, text string) error {
	return c.call(ctx, "answerCallbackQuery", map[string]any{"callback_query_id": id, "text": text}, nil)
}

func (c *Client) EditMessageText(ctx context.Context, chatID, messageID int64, text string, html bool) error {
	params := map[string]any{"chat_id": chatID, "message_id": messageID, "text": text}
	if html {
		params["parse_mode"] = "HTML"
	}
	return c.call(ctx, "editMessageText", params, nil)
}

func (c *Client) GetFile(ctx context.Context, fileID string) (File, error) {
	var f File
	err := c.call(ctx, "getFile", map[string]any{"file_id": fileID}, &f)
	return f, err
}

// maxDownload is the Bot API's own cap on files a bot can download.
const maxDownload = 20 << 20

// DownloadFile fetches a file GetFile pointed at.
func (c *Client) DownloadFile(ctx context.Context, filePath string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+"/file/bot"+c.token+"/"+filePath, nil)
	if err != nil {
		return nil, errors.New("telegram download: invalid request")
	}
	res, err := c.http.Do(req)
	if err != nil {
		// *url.Error prints the URL, and the URL carries the token.
		var urlErr *url.Error
		if errors.As(err, &urlErr) {
			err = urlErr.Err
		}
		return nil, fmt.Errorf("telegram download: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, &APIError{Code: res.StatusCode, Description: "download failed"}
	}
	data, err := io.ReadAll(io.LimitReader(res.Body, maxDownload+1))
	if err != nil {
		return nil, fmt.Errorf("telegram download: %w", err)
	}
	if len(data) > maxDownload {
		return nil, &APIError{Code: http.StatusRequestEntityTooLarge, Description: "file too large"}
	}
	return data, nil
}
