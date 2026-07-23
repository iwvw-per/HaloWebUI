package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

type Channel struct {
	ID            string          `json:"id"`
	UserID        string          `json:"user_id"`
	Type          *string         `json:"type,omitempty"`
	Name          string          `json:"name"`
	Description   *string         `json:"description,omitempty"`
	Data          json.RawMessage `json:"data,omitempty"`
	Meta          json.RawMessage `json:"meta,omitempty"`
	AccessControl json.RawMessage `json:"access_control,omitempty"`
	CreatedAt     int64           `json:"created_at"`
	UpdatedAt     int64           `json:"updated_at"`
}

type ChannelMessage struct {
	ID        string          `json:"id"`
	UserID    string          `json:"user_id"`
	ChannelID string          `json:"channel_id"`
	ParentID  *string         `json:"parent_id,omitempty"`
	Content   string          `json:"content"`
	Data      json.RawMessage `json:"data,omitempty"`
	Meta      json.RawMessage `json:"meta,omitempty"`
	CreatedAt int64           `json:"created_at"`
	UpdatedAt int64           `json:"updated_at"`
}

type Reaction struct {
	Name    string   `json:"name"`
	UserIDs []string `json:"user_ids"`
	Count   int      `json:"count"`
}

var ErrChannelNotFound = errors.New("channel not found")
var ErrChannelMessageNotFound = errors.New("channel message not found")

func (s *Store) migrateChannels(ctx context.Context) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS channel (
			id TEXT PRIMARY KEY, user_id TEXT NOT NULL, type TEXT, name TEXT NOT NULL,
			description TEXT, data TEXT, meta TEXT, access_control TEXT,
			created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS message (
			id TEXT PRIMARY KEY, user_id TEXT NOT NULL, channel_id TEXT NOT NULL,
			parent_id TEXT, content TEXT NOT NULL, data TEXT, meta TEXT,
			created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS message_channel_created_idx ON message(channel_id, created_at DESC)`,
		`CREATE TABLE IF NOT EXISTS message_reaction (
			id TEXT PRIMARY KEY, user_id TEXT NOT NULL, message_id TEXT NOT NULL,
			name TEXT NOT NULL, created_at INTEGER NOT NULL,
			UNIQUE(user_id, message_id, name)
		)`,
	}
	for _, statement := range statements {
		if _, err := s.db.ExecContext(ctx, statement); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) CreateChannel(ctx context.Context, channel Channel) (Channel, error) {
	now := time.Now().UnixNano()
	channel.CreatedAt, channel.UpdatedAt = now, now
	_, err := s.db.ExecContext(ctx, `INSERT INTO channel (id,user_id,type,name,description,data,meta,access_control,created_at,updated_at) VALUES (?,?,?,?,?,?,?,?,?,?)`,
		channel.ID, channel.UserID, nullString(channel.Type), channel.Name, nullString(channel.Description), nullableJSON(channel.Data), nullableJSON(channel.Meta), nullableJSON(channel.AccessControl), channel.CreatedAt, channel.UpdatedAt)
	if err != nil {
		return Channel{}, err
	}
	return s.ChannelByID(ctx, channel.ID)
}

func (s *Store) ChannelByID(ctx context.Context, id string) (Channel, error) {
	var channel Channel
	var typ, description, data, meta, access sql.NullString
	err := s.db.QueryRowContext(ctx, `SELECT id,user_id,type,name,description,data,meta,access_control,created_at,updated_at FROM channel WHERE id=?`, id).
		Scan(&channel.ID, &channel.UserID, &typ, &channel.Name, &description, &data, &meta, &access, &channel.CreatedAt, &channel.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Channel{}, ErrChannelNotFound
	}
	if err != nil {
		return Channel{}, err
	}
	channel.Type, channel.Description = nullableString(typ), nullableString(description)
	if data.Valid {
		channel.Data = json.RawMessage(data.String)
	}
	if meta.Valid {
		channel.Meta = json.RawMessage(meta.String)
	}
	if access.Valid {
		channel.AccessControl = json.RawMessage(access.String)
	}
	return channel, nil
}

func (s *Store) ListChannels(ctx context.Context) ([]Channel, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,user_id,type,name,description,data,meta,access_control,created_at,updated_at FROM channel ORDER BY updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	channels := make([]Channel, 0)
	for rows.Next() {
		channel, err := scanChannel(rows)
		if err != nil {
			return nil, err
		}
		channels = append(channels, channel)
	}
	return channels, rows.Err()
}

func scanChannel(scanner rowScanner) (Channel, error) {
	var channel Channel
	var typ, description, data, meta, access sql.NullString
	err := scanner.Scan(&channel.ID, &channel.UserID, &typ, &channel.Name, &description, &data, &meta, &access, &channel.CreatedAt, &channel.UpdatedAt)
	channel.Type, channel.Description = nullableString(typ), nullableString(description)
	if data.Valid {
		channel.Data = json.RawMessage(data.String)
	}
	if meta.Valid {
		channel.Meta = json.RawMessage(meta.String)
	}
	if access.Valid {
		channel.AccessControl = json.RawMessage(access.String)
	}
	return channel, err
}

func (s *Store) UpdateChannel(ctx context.Context, channel Channel) (Channel, error) {
	result, err := s.db.ExecContext(ctx, `UPDATE channel SET name=?,description=?,data=?,meta=?,access_control=?,updated_at=? WHERE id=?`,
		channel.Name, nullString(channel.Description), nullableJSON(channel.Data), nullableJSON(channel.Meta), nullableJSON(channel.AccessControl), time.Now().UnixNano(), channel.ID)
	if err != nil {
		return Channel{}, err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return Channel{}, ErrChannelNotFound
	}
	return s.ChannelByID(ctx, channel.ID)
}

func (s *Store) DeleteChannel(ctx context.Context, id string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM message_reaction WHERE message_id IN (SELECT id FROM message WHERE channel_id=?)`, id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM message WHERE channel_id=?`, id); err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `DELETE FROM channel WHERE id=?`, id)
	if err != nil {
		return err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return ErrChannelNotFound
	}
	return tx.Commit()
}

func (s *Store) CreateChannelMessage(ctx context.Context, message ChannelMessage) (ChannelMessage, error) {
	now := time.Now().UnixNano()
	message.CreatedAt, message.UpdatedAt = now, now
	_, err := s.db.ExecContext(ctx, `INSERT INTO message (id,user_id,channel_id,parent_id,content,data,meta,created_at,updated_at) VALUES (?,?,?,?,?,?,?,?,?)`,
		message.ID, message.UserID, message.ChannelID, nullString(message.ParentID), message.Content, nullableJSON(message.Data), nullableJSON(message.Meta), message.CreatedAt, message.UpdatedAt)
	if err != nil {
		return ChannelMessage{}, err
	}
	return s.ChannelMessageByID(ctx, message.ID)
}

func (s *Store) ChannelMessageByID(ctx context.Context, id string) (ChannelMessage, error) {
	var message ChannelMessage
	var parent, data, meta sql.NullString
	err := s.db.QueryRowContext(ctx, `SELECT id,user_id,channel_id,parent_id,content,data,meta,created_at,updated_at FROM message WHERE id=?`, id).
		Scan(&message.ID, &message.UserID, &message.ChannelID, &parent, &message.Content, &data, &meta, &message.CreatedAt, &message.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return ChannelMessage{}, ErrChannelMessageNotFound
	}
	if err != nil {
		return ChannelMessage{}, err
	}
	message.ParentID = nullableString(parent)
	if data.Valid {
		message.Data = json.RawMessage(data.String)
	}
	if meta.Valid {
		message.Meta = json.RawMessage(meta.String)
	}
	return message, nil
}

func (s *Store) ListChannelMessages(ctx context.Context, channelID string, parentID *string, skip, limit int) ([]ChannelMessage, error) {
	if skip < 0 {
		skip = 0
	}
	if limit < 1 || limit > 200 {
		limit = 50
	}
	query := `SELECT id,user_id,channel_id,parent_id,content,data,meta,created_at,updated_at FROM message WHERE channel_id=? AND parent_id IS NULL ORDER BY created_at DESC LIMIT ? OFFSET ?`
	args := []any{channelID, limit, skip}
	if parentID != nil {
		query = `SELECT id,user_id,channel_id,parent_id,content,data,meta,created_at,updated_at FROM message WHERE channel_id=? AND parent_id=? ORDER BY created_at DESC LIMIT ? OFFSET ?`
		args = []any{channelID, *parentID, limit, skip}
	}
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]ChannelMessage, 0)
	for rows.Next() {
		message, err := scanChannelMessage(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, message)
	}
	return result, rows.Err()
}

func scanChannelMessage(scanner rowScanner) (ChannelMessage, error) {
	var message ChannelMessage
	var parent, data, meta sql.NullString
	err := scanner.Scan(&message.ID, &message.UserID, &message.ChannelID, &parent, &message.Content, &data, &meta, &message.CreatedAt, &message.UpdatedAt)
	message.ParentID = nullableString(parent)
	if data.Valid {
		message.Data = json.RawMessage(data.String)
	}
	if meta.Valid {
		message.Meta = json.RawMessage(meta.String)
	}
	return message, err
}

func (s *Store) UpdateChannelMessage(ctx context.Context, message ChannelMessage) (ChannelMessage, error) {
	result, err := s.db.ExecContext(ctx, `UPDATE message SET content=?,data=?,meta=?,updated_at=? WHERE id=?`, message.Content, nullableJSON(message.Data), nullableJSON(message.Meta), time.Now().UnixNano(), message.ID)
	if err != nil {
		return ChannelMessage{}, err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return ChannelMessage{}, ErrChannelMessageNotFound
	}
	return s.ChannelMessageByID(ctx, message.ID)
}

func (s *Store) DeleteChannelMessage(ctx context.Context, id string) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM message_reaction WHERE message_id=?`, id); err != nil {
		return err
	}
	result, err := s.db.ExecContext(ctx, `DELETE FROM message WHERE id=?`, id)
	if err != nil {
		return err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return ErrChannelMessageNotFound
	}
	return nil
}

func (s *Store) AddReaction(ctx context.Context, userID, messageID, name, id string) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO message_reaction (id,user_id,message_id,name,created_at) VALUES (?,?,?,?,?) ON CONFLICT(user_id,message_id,name) DO NOTHING`, id, userID, messageID, name, time.Now().UnixNano())
	return err
}

func (s *Store) RemoveReaction(ctx context.Context, userID, messageID, name string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM message_reaction WHERE user_id=? AND message_id=? AND name=?`, userID, messageID, name)
	return err
}

func (s *Store) Reactions(ctx context.Context, messageID string) ([]Reaction, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT name,user_id FROM message_reaction WHERE message_id=? ORDER BY created_at`, messageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	byName := map[string][]string{}
	order := make([]string, 0)
	for rows.Next() {
		var name, userID string
		if err := rows.Scan(&name, &userID); err != nil {
			return nil, err
		}
		if _, ok := byName[name]; !ok {
			order = append(order, name)
		}
		byName[name] = append(byName[name], userID)
	}
	result := make([]Reaction, 0, len(order))
	for _, name := range order {
		result = append(result, Reaction{Name: name, UserIDs: byName[name], Count: len(byName[name])})
	}
	return result, rows.Err()
}
