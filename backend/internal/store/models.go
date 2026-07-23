package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

type Model struct {
	ID            string          `json:"id"`
	UserID        string          `json:"user_id"`
	BaseModelID   *string         `json:"base_model_id,omitempty"`
	Name          string          `json:"name"`
	Params        json.RawMessage `json:"params"`
	Meta          json.RawMessage `json:"meta"`
	AccessControl json.RawMessage `json:"access_control,omitempty"`
	IsActive      bool            `json:"is_active"`
	UpdatedAt     int64           `json:"updated_at"`
	CreatedAt     int64           `json:"created_at"`
}

func (s *Store) migrateModels(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS model (
		id TEXT PRIMARY KEY,
		user_id TEXT NOT NULL,
		base_model_id TEXT,
		name TEXT NOT NULL,
		params TEXT NOT NULL DEFAULT '{}',
		meta TEXT NOT NULL DEFAULT '{}',
		access_control TEXT,
		is_active INTEGER NOT NULL DEFAULT 1,
		updated_at INTEGER NOT NULL,
		created_at INTEGER NOT NULL
	)`)
	return err
}

func (s *Store) UpsertModel(ctx context.Context, model Model) (Model, error) {
	now := time.Now().Unix()
	if model.CreatedAt == 0 {
		model.CreatedAt = now
	}
	model.UpdatedAt = now
	if len(model.Params) == 0 {
		model.Params = json.RawMessage(`{}`)
	}
	if len(model.Meta) == 0 {
		model.Meta = json.RawMessage(`{}`)
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO model (
		id, user_id, base_model_id, name, params, meta, access_control,
		is_active, updated_at, created_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET
		user_id=excluded.user_id, base_model_id=excluded.base_model_id,
		name=excluded.name, params=excluded.params, meta=excluded.meta,
		access_control=excluded.access_control, is_active=excluded.is_active,
		updated_at=excluded.updated_at`,
		model.ID, model.UserID, nullString(model.BaseModelID), model.Name,
		string(model.Params), string(model.Meta), nullableJSON(model.AccessControl),
		model.IsActive, model.UpdatedAt, model.CreatedAt,
	)
	if err != nil {
		return Model{}, err
	}
	return s.ModelByID(ctx, model.ID)
}

func (s *Store) ModelByID(ctx context.Context, id string) (Model, error) {
	model, err := scanModel(s.db.QueryRowContext(ctx, `SELECT id, user_id, base_model_id, name,
		params, meta, access_control, is_active, updated_at, created_at
		FROM model WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Model{}, ErrModelNotFound
	}
	return model, err
}

func scanModel(scanner rowScanner) (Model, error) {
	var model Model
	var baseModel sql.NullString
	var params, meta string
	var access sql.NullString
	var active int
	err := scanner.Scan(
		&model.ID, &model.UserID, &baseModel, &model.Name, &params, &meta,
		&access, &active, &model.UpdatedAt, &model.CreatedAt,
	)
	if err != nil {
		return Model{}, err
	}
	model.BaseModelID = nullableString(baseModel)
	model.Params = json.RawMessage(defaultJSON(params))
	model.Meta = json.RawMessage(defaultJSON(meta))
	if access.Valid {
		model.AccessControl = json.RawMessage(access.String)
	}
	model.IsActive = active != 0
	return model, nil
}

func (s *Store) ListModels(ctx context.Context, activeOnly bool) ([]Model, error) {
	query := `SELECT id, user_id, base_model_id, name, params, meta, access_control, is_active, updated_at, created_at FROM model`
	if activeOnly {
		query += ` WHERE is_active = 1`
	}
	query += ` ORDER BY updated_at DESC`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	models := make([]Model, 0)
	for rows.Next() {
		model, err := scanModel(rows)
		if err != nil {
			return nil, err
		}
		models = append(models, model)
	}
	return models, rows.Err()
}

func (s *Store) ToggleModel(ctx context.Context, id string) (Model, error) {
	result, err := s.db.ExecContext(ctx, `UPDATE model SET is_active = NOT is_active, updated_at = ? WHERE id = ?`, time.Now().Unix(), id)
	if err != nil {
		return Model{}, err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return Model{}, ErrModelNotFound
	}
	return s.ModelByID(ctx, id)
}

func (s *Store) DeleteModel(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM model WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return ErrModelNotFound
	}
	return nil
}

func (s *Store) DeleteAllModels(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM model`)
	return err
}

func nullableJSON(value json.RawMessage) any {
	if len(value) == 0 || string(value) == "null" {
		return nil
	}
	return string(value)
}
