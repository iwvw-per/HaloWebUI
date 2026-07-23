package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

type Chat struct {
	ID          string          `json:"id"`
	UserID      string          `json:"user_id"`
	Title       string          `json:"title"`
	Chat        json.RawMessage `json:"chat"`
	CreatedAt   int64           `json:"created_at"`
	UpdatedAt   int64           `json:"updated_at"`
	ShareID     *string         `json:"share_id,omitempty"`
	Archived    bool            `json:"archived"`
	Pinned      bool            `json:"pinned"`
	Meta        json.RawMessage `json:"meta"`
	FolderID    *string         `json:"folder_id,omitempty"`
	AssistantID *string         `json:"assistant_id,omitempty"`
}

func (s *Store) migrateChats(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS chat (
		id TEXT PRIMARY KEY,
		user_id TEXT NOT NULL,
		title TEXT NOT NULL DEFAULT 'New Chat',
		chat TEXT NOT NULL DEFAULT '{}',
		created_at INTEGER NOT NULL,
		updated_at INTEGER NOT NULL,
		share_id TEXT UNIQUE,
		archived INTEGER NOT NULL DEFAULT 0,
		pinned INTEGER NOT NULL DEFAULT 0,
		meta TEXT NOT NULL DEFAULT '{}',
		folder_id TEXT,
		assistant_id TEXT
	)`)
	if err != nil {
		return fmt.Errorf("create chat schema: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS chat_user_updated_idx ON chat(user_id, updated_at DESC)`)
	return err
}

func (s *Store) CreateChat(ctx context.Context, chat Chat) (Chat, error) {
	if len(chat.Chat) == 0 {
		chat.Chat = json.RawMessage(`{}`)
	}
	if len(chat.Meta) == 0 {
		chat.Meta = json.RawMessage(`{}`)
	}
	if chat.Title == "" {
		chat.Title = "New Chat"
	}
	now := time.Now().UnixMilli()
	if chat.CreatedAt == 0 {
		chat.CreatedAt = now
	}
	chat.UpdatedAt = now
	_, err := s.db.ExecContext(ctx, `INSERT INTO chat (
		id, user_id, title, chat, created_at, updated_at, share_id,
		archived, pinned, meta, folder_id, assistant_id
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		chat.ID, chat.UserID, chat.Title, string(chat.Chat), chat.CreatedAt, chat.UpdatedAt,
		nullString(chat.ShareID), chat.Archived, chat.Pinned, string(chat.Meta),
		nullString(chat.FolderID), nullString(chat.AssistantID),
	)
	if err != nil {
		return Chat{}, err
	}
	return s.ChatByID(ctx, chat.ID)
}

func (s *Store) ChatByID(ctx context.Context, id string) (Chat, error) {
	chat, err := scanChat(s.db.QueryRowContext(ctx, `SELECT
		id, user_id, title, chat, created_at, updated_at, share_id,
		archived, pinned, meta, folder_id, assistant_id
		FROM chat WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Chat{}, ErrChatNotFound
	}
	return chat, err
}

func scanChat(scanner rowScanner) (Chat, error) {
	var chat Chat
	var shareID, folderID, assistantID sql.NullString
	var archived, pinned int
	var rawChat, rawMeta string
	err := scanner.Scan(
		&chat.ID, &chat.UserID, &chat.Title, &rawChat, &chat.CreatedAt, &chat.UpdatedAt,
		&shareID, &archived, &pinned, &rawMeta, &folderID, &assistantID,
	)
	if err != nil {
		return Chat{}, err
	}
	chat.Chat = json.RawMessage(defaultJSON(rawChat))
	chat.Meta = json.RawMessage(defaultJSON(rawMeta))
	chat.Archived = archived != 0
	chat.Pinned = pinned != 0
	chat.ShareID = nullableString(shareID)
	chat.FolderID = nullableString(folderID)
	chat.AssistantID = nullableString(assistantID)
	return chat, nil
}

func (s *Store) ListChats(ctx context.Context, userID string, archived *bool, page, limit int) ([]Chat, error) {
	return s.ListChatsWithFilter(ctx, userID, ChatFilter{Archived: archived}, page, limit)
}

// ChatFilter keeps the common chat list queries in one bounded SQL path.
// Nil booleans mean that the corresponding field is not filtered.
type ChatFilter struct {
	Archived  *bool
	Pinned    *bool
	FolderID  *string
	Assistant *string
	Search    string
	Shared    *bool
}

func (s *Store) ListChatsWithFilter(ctx context.Context, userID string, filter ChatFilter, page, limit int) ([]Chat, error) {
	if limit <= 0 || limit > 200 {
		limit = 60
	}
	if page < 1 {
		page = 1
	}
	conditions := []string{"user_id = ?"}
	args := []any{userID}
	if filter.Archived != nil {
		conditions = append(conditions, "archived = ?")
		args = append(args, *filter.Archived)
	}
	if filter.Pinned != nil {
		conditions = append(conditions, "pinned = ?")
		args = append(args, *filter.Pinned)
	}
	if filter.FolderID != nil {
		if *filter.FolderID == "" {
			conditions = append(conditions, "(folder_id IS NULL OR folder_id = '')")
		} else {
			conditions = append(conditions, "folder_id = ?")
			args = append(args, *filter.FolderID)
		}
	}
	if filter.Assistant != nil {
		conditions = append(conditions, "assistant_id = ?")
		args = append(args, *filter.Assistant)
	}
	if filter.Search != "" {
		pattern := "%" + strings.ToLower(strings.TrimSpace(filter.Search)) + "%"
		conditions = append(conditions, "(lower(title) LIKE ? OR lower(chat) LIKE ?)")
		args = append(args, pattern, pattern)
	}
	if filter.Shared != nil {
		if *filter.Shared {
			conditions = append(conditions, "share_id IS NOT NULL AND share_id <> ''")
		} else {
			conditions = append(conditions, "(share_id IS NULL OR share_id = '')")
		}
	}
	args = append(args, limit, (page-1)*limit)
	rows, err := s.db.QueryContext(ctx, `SELECT
		id, user_id, title, chat, created_at, updated_at, share_id,
		archived, pinned, meta, folder_id, assistant_id
		FROM chat WHERE `+strings.Join(conditions, " AND ")+` ORDER BY updated_at DESC LIMIT ? OFFSET ?`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	chats := make([]Chat, 0)
	for rows.Next() {
		chat, err := scanChat(rows)
		if err != nil {
			return nil, err
		}
		chats = append(chats, chat)
	}
	return chats, rows.Err()
}

func (s *Store) ListSharedChats(ctx context.Context, userID string) ([]Chat, error) {
	shared := true
	return s.ListChatsWithFilter(ctx, userID, ChatFilter{Shared: &shared}, 1, 200)
}

func (s *Store) ListAllChats(ctx context.Context, limit int) ([]Chat, error) {
	if limit <= 0 || limit > 1000 {
		limit = 1000
	}
	rows, err := s.db.QueryContext(ctx, `SELECT
		id, user_id, title, chat, created_at, updated_at, share_id,
		archived, pinned, meta, folder_id, assistant_id
		FROM chat ORDER BY updated_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	chats := make([]Chat, 0)
	for rows.Next() {
		chat, err := scanChat(rows)
		if err != nil {
			return nil, err
		}
		chats = append(chats, chat)
	}
	return chats, rows.Err()
}

func (s *Store) ChatByShareID(ctx context.Context, shareID string) (Chat, error) {
	var id string
	err := s.db.QueryRowContext(ctx, `SELECT id FROM chat WHERE share_id = ?`, shareID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return Chat{}, ErrChatNotFound
	}
	if err != nil {
		return Chat{}, err
	}
	return s.ChatByID(ctx, id)
}

func (s *Store) DeleteAllChats(ctx context.Context, userID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM chat WHERE user_id = ?`, userID)
	return err
}

func (s *Store) ArchiveAllChats(ctx context.Context, userID string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE chat SET archived = 1, updated_at = ? WHERE user_id = ?`, time.Now().UnixMilli(), userID)
	return err
}

func (s *Store) UpdateChat(ctx context.Context, chat Chat) (Chat, error) {
	chat.UpdatedAt = time.Now().UnixMilli()
	result, err := s.db.ExecContext(ctx, `UPDATE chat SET
		title = ?, chat = ?, updated_at = ?, share_id = ?, archived = ?,
		pinned = ?, meta = ?, folder_id = ?, assistant_id = ? WHERE id = ?`,
		chat.Title, string(chat.Chat), chat.UpdatedAt, nullString(chat.ShareID), chat.Archived,
		chat.Pinned, string(chat.Meta), nullString(chat.FolderID), nullString(chat.AssistantID), chat.ID,
	)
	if err != nil {
		return Chat{}, err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return Chat{}, ErrChatNotFound
	}
	return s.ChatByID(ctx, chat.ID)
}

func (s *Store) DeleteChat(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM chat WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return ErrChatNotFound
	}
	return nil
}

func (s *Store) SetChatField(ctx context.Context, id, field string, value any) (Chat, error) {
	allowed := map[string]bool{"pinned": true, "archived": true, "folder_id": true, "share_id": true, "title": true}
	if !allowed[field] {
		return Chat{}, errors.New("unsupported chat field")
	}
	result, err := s.db.ExecContext(ctx, `UPDATE chat SET `+field+` = ?, updated_at = ? WHERE id = ?`, value, time.Now().UnixMilli(), id)
	if err != nil {
		return Chat{}, err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return Chat{}, ErrChatNotFound
	}
	return s.ChatByID(ctx, id)
}

func nullString(value *string) any {
	if value == nil {
		return nil
	}
	return *value
}

func nullableString(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	copy := value.String
	return &copy
}

func defaultJSON(value string) string {
	if strings.TrimSpace(value) == "" {
		return "{}"
	}
	return value
}
