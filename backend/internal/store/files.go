package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

type File struct {
	ID            string          `json:"id"`
	UserID        string          `json:"user_id"`
	Hash          *string         `json:"hash,omitempty"`
	Filename      string          `json:"filename"`
	Path          *string         `json:"path,omitempty"`
	Data          json.RawMessage `json:"data,omitempty"`
	Meta          json.RawMessage `json:"meta,omitempty"`
	AccessControl json.RawMessage `json:"access_control,omitempty"`
	CreatedAt     int64           `json:"created_at"`
	UpdatedAt     int64           `json:"updated_at"`
}

var ErrFileNotFound = errors.New("file not found")

func (s *Store) migrateFiles(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS file (
		id TEXT PRIMARY KEY,
		user_id TEXT NOT NULL,
		hash TEXT,
		filename TEXT NOT NULL,
		path TEXT,
		data TEXT,
		meta TEXT,
		access_control TEXT,
		created_at INTEGER NOT NULL,
		updated_at INTEGER NOT NULL
	)`)
	return err
}

func (s *Store) CreateFile(ctx context.Context, file File) (File, error) {
	now := time.Now().Unix()
	file.CreatedAt, file.UpdatedAt = now, now
	_, err := s.db.ExecContext(ctx, `INSERT INTO file (
		id, user_id, hash, filename, path, data, meta, access_control, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		file.ID, file.UserID, nullString(file.Hash), file.Filename, nullString(file.Path),
		nullableJSON(file.Data), nullableJSON(file.Meta), nullableJSON(file.AccessControl),
		file.CreatedAt, file.UpdatedAt,
	)
	if err != nil {
		return File{}, err
	}
	return s.FileByID(ctx, file.ID)
}

func (s *Store) FileByID(ctx context.Context, id string) (File, error) {
	var file File
	var hash, path, data, meta, access sql.NullString
	err := s.db.QueryRowContext(ctx, `SELECT id, user_id, hash, filename, path,
		data, meta, access_control, created_at, updated_at FROM file WHERE id = ?`, id).Scan(
		&file.ID, &file.UserID, &hash, &file.Filename, &path, &data, &meta, &access,
		&file.CreatedAt, &file.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return File{}, ErrFileNotFound
	}
	if err != nil {
		return File{}, err
	}
	file.Hash, file.Path = nullableString(hash), nullableString(path)
	if data.Valid {
		file.Data = json.RawMessage(data.String)
	}
	if meta.Valid {
		file.Meta = json.RawMessage(meta.String)
	}
	if access.Valid {
		file.AccessControl = json.RawMessage(access.String)
	}
	return file, nil
}

func (s *Store) ListFiles(ctx context.Context, userID string) ([]File, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id FROM file WHERE user_id = ? ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	files := make([]File, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		file, err := s.FileByID(ctx, id)
		if err != nil {
			return nil, err
		}
		files = append(files, file)
	}
	return files, rows.Err()
}

func (s *Store) UpdateFileData(ctx context.Context, id string, data json.RawMessage) (File, error) {
	result, err := s.db.ExecContext(ctx, `UPDATE file SET data = ?, updated_at = ? WHERE id = ?`, string(data), time.Now().Unix(), id)
	if err != nil {
		return File{}, err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return File{}, ErrFileNotFound
	}
	return s.FileByID(ctx, id)
}

func (s *Store) DeleteFile(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM file WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return ErrFileNotFound
	}
	return nil
}

func (s *Store) DeleteAllFiles(ctx context.Context, userID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM file WHERE user_id = ?`, userID)
	return err
}
