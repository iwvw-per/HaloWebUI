package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

type Knowledge struct {
	ID            string          `json:"id"`
	UserID        string          `json:"user_id"`
	Name          string          `json:"name"`
	Description   string          `json:"description"`
	Data          json.RawMessage `json:"data,omitempty"`
	Meta          json.RawMessage `json:"meta,omitempty"`
	AccessControl json.RawMessage `json:"access_control,omitempty"`
	CreatedAt     int64           `json:"created_at"`
	UpdatedAt     int64           `json:"updated_at"`
}

var ErrKnowledgeNotFound = errors.New("knowledge base not found")

func (s *Store) migrateKnowledge(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS knowledge (
		id TEXT PRIMARY KEY,
		user_id TEXT NOT NULL,
		name TEXT NOT NULL,
		description TEXT NOT NULL DEFAULT '',
		data TEXT,
		meta TEXT,
		access_control TEXT,
		created_at INTEGER NOT NULL,
		updated_at INTEGER NOT NULL
	)`)
	return err
}

func (s *Store) CreateKnowledge(ctx context.Context, knowledge Knowledge) (Knowledge, error) {
	now := time.Now().Unix()
	knowledge.CreatedAt, knowledge.UpdatedAt = now, now
	if len(knowledge.Data) == 0 {
		knowledge.Data = json.RawMessage(`{"file_ids":[]}`)
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO knowledge (id, user_id, name, description, data, meta, access_control, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		knowledge.ID, knowledge.UserID, knowledge.Name, knowledge.Description,
		nullableJSON(knowledge.Data), nullableJSON(knowledge.Meta), nullableJSON(knowledge.AccessControl),
		knowledge.CreatedAt, knowledge.UpdatedAt)
	if err != nil {
		return Knowledge{}, err
	}
	return s.KnowledgeByID(ctx, knowledge.ID)
}

func (s *Store) KnowledgeByID(ctx context.Context, id string) (Knowledge, error) {
	knowledge, err := scanKnowledge(s.db.QueryRowContext(ctx, `SELECT id, user_id, name, description, data, meta, access_control, created_at, updated_at FROM knowledge WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Knowledge{}, ErrKnowledgeNotFound
	}
	return knowledge, err
}

func scanKnowledge(scanner rowScanner) (Knowledge, error) {
	var knowledge Knowledge
	var data, meta, access sql.NullString
	err := scanner.Scan(&knowledge.ID, &knowledge.UserID, &knowledge.Name, &knowledge.Description, &data, &meta, &access, &knowledge.CreatedAt, &knowledge.UpdatedAt)
	if err != nil {
		return Knowledge{}, err
	}
	if data.Valid {
		knowledge.Data = json.RawMessage(data.String)
	}
	if meta.Valid {
		knowledge.Meta = json.RawMessage(meta.String)
	}
	if access.Valid {
		knowledge.AccessControl = json.RawMessage(access.String)
	}
	return knowledge, nil
}

func (s *Store) ListKnowledge(ctx context.Context, query string) ([]Knowledge, error) {
	search := strings.ToLower(strings.TrimSpace(query))
	statement := `SELECT id, user_id, name, description, data, meta, access_control, created_at, updated_at FROM knowledge`
	args := make([]any, 0, 2)
	if search != "" {
		pattern := "%" + search + "%"
		statement += ` WHERE lower(name) LIKE ? OR lower(description) LIKE ?`
		args = []any{pattern, pattern}
	}
	statement += ` ORDER BY updated_at DESC`
	rows, err := s.db.QueryContext(ctx, statement, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Knowledge, 0)
	for rows.Next() {
		item, err := scanKnowledge(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) UpdateKnowledge(ctx context.Context, knowledge Knowledge) (Knowledge, error) {
	result, err := s.db.ExecContext(ctx, `UPDATE knowledge SET name = ?, description = ?, data = ?, meta = ?, access_control = ?, updated_at = ? WHERE id = ?`,
		knowledge.Name, knowledge.Description, nullableJSON(knowledge.Data), nullableJSON(knowledge.Meta), nullableJSON(knowledge.AccessControl), time.Now().Unix(), knowledge.ID)
	if err != nil {
		return Knowledge{}, err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return Knowledge{}, ErrKnowledgeNotFound
	}
	return s.KnowledgeByID(ctx, knowledge.ID)
}

func (s *Store) DeleteKnowledge(ctx context.Context, id string) (bool, error) {
	result, err := s.db.ExecContext(ctx, `DELETE FROM knowledge WHERE id = ?`, id)
	if err != nil {
		return false, err
	}
	affected, _ := result.RowsAffected()
	return affected != 0, nil
}

func KnowledgeFileIDs(knowledge Knowledge) []string {
	var data struct {
		FileIDs []string `json:"file_ids"`
	}
	_ = json.Unmarshal(knowledge.Data, &data)
	if data.FileIDs == nil {
		return []string{}
	}
	return data.FileIDs
}

func (s *Store) SetKnowledgeFile(ctx context.Context, id, fileID string, add bool) (Knowledge, error) {
	knowledge, err := s.KnowledgeByID(ctx, id)
	if err != nil {
		return Knowledge{}, err
	}
	var data map[string]any
	if json.Unmarshal(knowledge.Data, &data) != nil || data == nil {
		data = map[string]any{}
	}
	ids := KnowledgeFileIDs(knowledge)
	next := make([]string, 0, len(ids)+1)
	found := false
	for _, current := range ids {
		if current == fileID {
			found = true
			if !add {
				continue
			}
		}
		next = append(next, current)
	}
	if add && !found {
		next = append(next, fileID)
	}
	data["file_ids"] = next
	knowledge.Data, _ = json.Marshal(data)
	return s.UpdateKnowledge(ctx, knowledge)
}
