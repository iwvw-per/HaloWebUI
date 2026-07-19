package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

func (s *Store) ListUsers(ctx context.Context, query string, limit int) ([]User, error) {
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	pattern := "%" + strings.ToLower(strings.TrimSpace(query)) + "%"
	rows, err := s.db.QueryContext(ctx, `SELECT id FROM user
		WHERE ? = '%%' OR lower(name) LIKE ? OR lower(email) LIKE ?
		ORDER BY created_at DESC LIMIT ?`, pattern, pattern, pattern, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	users := make([]User, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		user, err := s.UserByID(ctx, id)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, rows.Err()
}

func (s *Store) UpdateUser(ctx context.Context, id, name, email, role, profileImageURL string) (User, error) {
	result, err := s.db.ExecContext(ctx, `UPDATE user SET
		name = COALESCE(NULLIF(?, ''), name),
		email = COALESCE(NULLIF(lower(?), ''), email),
		role = COALESCE(NULLIF(?, ''), role),
		profile_image_url = COALESCE(NULLIF(?, ''), profile_image_url),
		updated_at = ? WHERE id = ?`,
		name, email, role, profileImageURL, time.Now().Unix(), id,
	)
	if err != nil {
		return User{}, err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return User{}, ErrUserNotFound
	}
	if email != "" {
		_, _ = s.db.ExecContext(ctx, `UPDATE auth SET email = lower(?) WHERE id = ?`, email, id)
	}
	return s.UserByID(ctx, id)
}

func (s *Store) DeleteUser(ctx context.Context, id string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, statement := range []string{
		`DELETE FROM chat WHERE user_id = ?`,
		`DELETE FROM auth WHERE id = ?`,
		`DELETE FROM user WHERE id = ?`,
	} {
		if _, err := tx.ExecContext(ctx, statement, id); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) UserSettings(ctx context.Context, id string) (json.RawMessage, error) {
	var value sql.NullString
	err := s.db.QueryRowContext(ctx, `SELECT settings FROM user WHERE id = ?`, id).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}
	return json.RawMessage(defaultJSON(value.String)), nil
}

func (s *Store) SetUserSettings(ctx context.Context, id string, value json.RawMessage) (json.RawMessage, error) {
	if !json.Valid(value) {
		return nil, errors.New("invalid settings JSON")
	}
	result, err := s.db.ExecContext(ctx, `UPDATE user SET settings = ?, updated_at = ? WHERE id = ?`, string(value), time.Now().Unix(), id)
	if err != nil {
		return nil, err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return nil, ErrUserNotFound
	}
	return value, nil
}

func (s *Store) UserInfo(ctx context.Context, id string) (json.RawMessage, error) {
	var value sql.NullString
	err := s.db.QueryRowContext(ctx, `SELECT info FROM user WHERE id = ?`, id).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}
	return json.RawMessage(defaultJSON(value.String)), nil
}

func (s *Store) SetUserInfo(ctx context.Context, id string, value json.RawMessage) (json.RawMessage, error) {
	if !json.Valid(value) {
		return nil, errors.New("invalid info JSON")
	}
	result, err := s.db.ExecContext(ctx, `UPDATE user SET info = ?, updated_at = ? WHERE id = ?`, string(value), time.Now().Unix(), id)
	if err != nil {
		return nil, err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return nil, ErrUserNotFound
	}
	return value, nil
}
