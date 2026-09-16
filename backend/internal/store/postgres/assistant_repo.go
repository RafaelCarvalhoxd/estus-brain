package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/rafael/estus-vault/backend/internal/domain"
)

type AssistantSettings struct {
	Provider  string
	Models    map[string]string
	Secrets   map[string]string // provider → base64(nonce|ciphertext)
	OllamaURL string
	UpdatedAt time.Time
}

type Conversation struct {
	ID        string
	Title     string
	Module    string
	Provider  string
	SessionID string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type ConversationMessage struct {
	ID             string
	ConversationID string
	Role           string
	Content        string
	Data           json.RawMessage
	Provider       string
	CreatedAt      time.Time
}

type AssistantRepo struct{ db *DB }

func NewAssistantRepo(db *DB) *AssistantRepo { return &AssistantRepo{db: db} }

func assistantErr(op string, err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "22P02" {
		return domain.ErrNotFound
	}
	return fmt.Errorf("%s: %w", op, err)
}

func (r *AssistantRepo) Settings(ctx context.Context) (AssistantSettings, error) {
	var s AssistantSettings
	var models, secrets []byte
	err := r.db.Pool.QueryRow(ctx, `select provider, models, secrets, ollama_url, updated_at from assistant_settings where id = 1`).
		Scan(&s.Provider, &models, &secrets, &s.OllamaURL, &s.UpdatedAt)
	if err != nil {
		return AssistantSettings{}, assistantErr("assistant settings", err)
	}
	s.Models, s.Secrets = map[string]string{}, map[string]string{}
	_ = json.Unmarshal(models, &s.Models)
	_ = json.Unmarshal(secrets, &s.Secrets)
	return s, nil
}

func (r *AssistantRepo) SaveSettings(ctx context.Context, s AssistantSettings) error {
	models, _ := json.Marshal(s.Models)
	secrets, _ := json.Marshal(s.Secrets)
	_, err := r.db.Pool.Exec(ctx, `
		update assistant_settings
		set provider = $1, models = $2::jsonb, secrets = $3::jsonb, ollama_url = $4, updated_at = now()
		where id = 1`, s.Provider, string(models), string(secrets), s.OllamaURL)
	if err != nil {
		return fmt.Errorf("save assistant settings: %w", err)
	}
	return nil
}

const conversationColumns = `id, title, module, provider, session_id, created_at, updated_at`

func scanConversation(row scanner, c *Conversation) error {
	return row.Scan(&c.ID, &c.Title, &c.Module, &c.Provider, &c.SessionID, &c.CreatedAt, &c.UpdatedAt)
}

func (r *AssistantRepo) ListConversations(ctx context.Context, limit int) ([]Conversation, error) {
	rows, err := r.db.Pool.Query(ctx, `select `+conversationColumns+` from assistant_conversations order by updated_at desc limit $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("list conversations: %w", err)
	}
	defer rows.Close()
	var out []Conversation
	for rows.Next() {
		var c Conversation
		if err := scanConversation(rows, &c); err != nil {
			return nil, fmt.Errorf("scan conversation: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *AssistantRepo) GetConversation(ctx context.Context, id string) (Conversation, error) {
	var c Conversation
	if err := scanConversation(r.db.Pool.QueryRow(ctx, `select `+conversationColumns+` from assistant_conversations where id = $1`, id), &c); err != nil {
		return Conversation{}, assistantErr("get conversation", err)
	}
	return c, nil
}

func (r *AssistantRepo) CreateConversation(ctx context.Context, title, module string) (Conversation, error) {
	var c Conversation
	err := scanConversation(r.db.Pool.QueryRow(ctx, `
		insert into assistant_conversations (title, module) values ($1, $2)
		returning `+conversationColumns, title, module), &c)
	if err != nil {
		return Conversation{}, fmt.Errorf("create conversation: %w", err)
	}
	return c, nil
}

// TouchConversation records the engine (and its session) that last answered.
func (r *AssistantRepo) TouchConversation(ctx context.Context, id, provider, sessionID string) error {
	_, err := r.db.Pool.Exec(ctx, `
		update assistant_conversations
		set provider = case when $2 = '' then provider else $2 end,
		    session_id = $3,
		    updated_at = now()
		where id = $1`, id, provider, sessionID)
	if err != nil {
		return assistantErr("touch conversation", err)
	}
	return nil
}

func (r *AssistantRepo) DeleteConversation(ctx context.Context, id string) error {
	tag, err := r.db.Pool.Exec(ctx, `delete from assistant_conversations where id = $1`, id)
	if err != nil {
		return assistantErr("delete conversation", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *AssistantRepo) AddMessage(ctx context.Context, m ConversationMessage) (ConversationMessage, error) {
	data := m.Data
	if len(data) == 0 {
		data = json.RawMessage(`{}`)
	}
	err := r.db.Pool.QueryRow(ctx, `
		insert into assistant_messages (conversation_id, role, content, data, provider)
		values ($1, $2, $3, $4::jsonb, $5)
		returning id, created_at`, m.ConversationID, m.Role, m.Content, string(data), m.Provider).Scan(&m.ID, &m.CreatedAt)
	if err != nil {
		return ConversationMessage{}, assistantErr("add message", err)
	}
	_, _ = r.db.Pool.Exec(ctx, `update assistant_conversations set updated_at = now() where id = $1`, m.ConversationID)
	return m, nil
}

func (r *AssistantRepo) Messages(ctx context.Context, conversationID string, limit int) ([]ConversationMessage, error) {
	rows, err := r.db.Pool.Query(ctx, `
		select id, conversation_id, role, content, data, provider, created_at from (
			select * from assistant_messages where conversation_id = $1 order by created_at desc limit $2
		) recent order by created_at`, conversationID, limit)
	if err != nil {
		return nil, assistantErr("list messages", err)
	}
	defer rows.Close()
	var out []ConversationMessage
	for rows.Next() {
		var m ConversationMessage
		var data []byte
		if err := rows.Scan(&m.ID, &m.ConversationID, &m.Role, &m.Content, &data, &m.Provider, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan message: %w", err)
		}
		m.Data = data
		out = append(out, m)
	}
	return out, rows.Err()
}
