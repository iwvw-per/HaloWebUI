package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

type Resource struct {
	Kind      string          `json:"kind"`
	ID        string          `json:"id"`
	UserID    string          `json:"user_id"`
	Key       string          `json:"key"`
	Body      json.RawMessage `json:"body"`
	Active    bool            `json:"active"`
	CreatedAt int64           `json:"created_at"`
	UpdatedAt int64           `json:"updated_at"`
}

var ErrResourceNotFound = errors.New("resource not found")

func (s *Store) migrateResources(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS halo_resource (
		kind TEXT NOT NULL,
		id TEXT NOT NULL,
		user_id TEXT NOT NULL,
		resource_key TEXT NOT NULL,
		body TEXT NOT NULL DEFAULT '{}',
		active INTEGER NOT NULL DEFAULT 1,
		created_at INTEGER NOT NULL,
		updated_at INTEGER NOT NULL,
		PRIMARY KEY(kind, id),
		UNIQUE(kind, resource_key)
	)`)
	return err
}

func (s *Store) PutResource(ctx context.Context, resource Resource) (Resource, error) {
	now := time.Now().Unix()
	if resource.CreatedAt == 0 {
		resource.CreatedAt = now
	}
	resource.UpdatedAt = now
	if len(resource.Body) == 0 {
		resource.Body = json.RawMessage(`{}`)
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO halo_resource (
		kind, id, user_id, resource_key, body, active, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(kind, id) DO UPDATE SET
		user_id=excluded.user_id, resource_key=excluded.resource_key,
		body=excluded.body, active=excluded.active, updated_at=excluded.updated_at`,
		resource.Kind, resource.ID, resource.UserID, resource.Key, string(resource.Body),
		resource.Active, resource.CreatedAt, resource.UpdatedAt,
	)
	if err != nil {
		return Resource{}, err
	}
	return s.ResourceByID(ctx, resource.Kind, resource.ID)
}

func (s *Store) ResourceByID(ctx context.Context, kind, id string) (Resource, error) {
	return s.resource(ctx, `SELECT kind, id, user_id, resource_key, body, active, created_at, updated_at
		FROM halo_resource WHERE kind = ? AND id = ?`, kind, id)
}

func (s *Store) ResourceByKey(ctx context.Context, kind, key string) (Resource, error) {
	return s.resource(ctx, `SELECT kind, id, user_id, resource_key, body, active, created_at, updated_at
		FROM halo_resource WHERE kind = ? AND resource_key = ?`, kind, key)
}

func (s *Store) resource(ctx context.Context, query string, args ...any) (Resource, error) {
	var resource Resource
	var body string
	var active int
	err := s.db.QueryRowContext(ctx, query, args...).Scan(
		&resource.Kind, &resource.ID, &resource.UserID, &resource.Key, &body,
		&active, &resource.CreatedAt, &resource.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Resource{}, ErrResourceNotFound
	}
	resource.Body = json.RawMessage(defaultJSON(body))
	resource.Active = active != 0
	return resource, err
}

func (s *Store) ListResources(ctx context.Context, kind string, activeOnly bool) ([]Resource, error) {
	query := `SELECT id FROM halo_resource WHERE kind = ?`
	if activeOnly {
		query += ` AND active = 1`
	}
	query += ` ORDER BY updated_at DESC`
	rows, err := s.db.QueryContext(ctx, query, kind)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	resources := make([]Resource, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		resource, err := s.ResourceByID(ctx, kind, id)
		if err != nil {
			return nil, err
		}
		resources = append(resources, resource)
	}
	return resources, rows.Err()
}

func (s *Store) ToggleResource(ctx context.Context, kind, id string) (Resource, error) {
	result, err := s.db.ExecContext(ctx, `UPDATE halo_resource SET active = NOT active, updated_at = ? WHERE kind = ? AND id = ?`, time.Now().Unix(), kind, id)
	if err != nil {
		return Resource{}, err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return Resource{}, ErrResourceNotFound
	}
	return s.ResourceByID(ctx, kind, id)
}

func (s *Store) DeleteResource(ctx context.Context, kind, id string) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM halo_resource WHERE kind = ? AND id = ?`, kind, id)
	if err != nil {
		return err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return ErrResourceNotFound
	}
	return nil
}
