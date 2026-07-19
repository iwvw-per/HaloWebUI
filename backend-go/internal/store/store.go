package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

var (
	ErrEmailTaken      = errors.New("email is already registered")
	ErrInvalidPassword = errors.New("invalid email or password")
	ErrUserNotFound    = errors.New("user not found")
	ErrChatNotFound    = errors.New("chat not found")
	ErrModelNotFound   = errors.New("model not found")
)

type Store struct {
	db *sql.DB
}

type User struct {
	ID              string         `json:"id"`
	Name            string         `json:"name"`
	Email           string         `json:"email"`
	Role            string         `json:"role"`
	ProfileImageURL string         `json:"profile_image_url"`
	LastActiveAt    int64          `json:"last_active_at"`
	UpdatedAt       int64          `json:"updated_at"`
	CreatedAt       int64          `json:"created_at"`
	APIKey          sql.NullString `json:"-"`
	Settings        sql.NullString `json:"-"`
	Info            sql.NullString `json:"-"`
	OAuthSub        sql.NullString `json:"-"`
	Note            sql.NullString `json:"-"`
}

func Open(ctx context.Context, dataDir string) (*Store, error) {
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return nil, fmt.Errorf("create data directory: %w", err)
	}
	path := filepath.Join(dataDir, "webui.db")
	dsn := path + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)&_pragma=synchronous(NORMAL)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite database: %w", err)
	}
	// Reads may hydrate related records while an iterator is open. Keep the pool
	// small for the 256 MiB target while avoiding self-deadlock at one connection.
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)
	store := &Store{db: db}
	if err := store.migrate(ctx); err != nil {
		db.Close()
		return nil, err
	}
	if err := store.migrateChats(ctx); err != nil {
		db.Close()
		return nil, err
	}
	if err := store.migrateModels(ctx); err != nil {
		db.Close()
		return nil, err
	}
	if err := store.migrateResources(ctx); err != nil {
		db.Close()
		return nil, err
	}
	if err := store.migrateFiles(ctx); err != nil {
		db.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) migrate(ctx context.Context) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS auth (
			id TEXT PRIMARY KEY,
			email TEXT NOT NULL,
			password TEXT NOT NULL,
			active INTEGER NOT NULL DEFAULT 1
		)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS auth_email_uq ON auth(email)`,
		`CREATE TABLE IF NOT EXISTS user (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			email TEXT NOT NULL,
			role TEXT NOT NULL DEFAULT 'pending',
			profile_image_url TEXT NOT NULL DEFAULT '/user.png',
			last_active_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL,
			created_at INTEGER NOT NULL,
			api_key TEXT UNIQUE,
			settings TEXT,
			info TEXT,
			oauth_sub TEXT UNIQUE,
			note TEXT
		)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS user_email_uq ON user(email)`,
	}
	for _, statement := range statements {
		if _, err := s.db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("apply core schema: %w", err)
		}
	}
	return nil
}

func (s *Store) UserCount(ctx context.Context) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM user`).Scan(&count)
	return count, err
}

func (s *Store) CreateUser(
	ctx context.Context,
	id, name, email, passwordHash, profileImageURL, defaultRole string,
) (User, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return User{}, err
	}
	defer tx.Rollback()

	email = strings.ToLower(strings.TrimSpace(email))
	var existing int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM user WHERE email = ?`, email).Scan(&existing); err != nil {
		return User{}, err
	}
	if existing != 0 {
		return User{}, ErrEmailTaken
	}
	var count int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM user`).Scan(&count); err != nil {
		return User{}, err
	}
	role := defaultRole
	if count == 0 {
		role = "admin"
	}
	now := time.Now().Unix()
	if _, err := tx.ExecContext(
		ctx,
		`INSERT INTO auth (id, email, password, active) VALUES (?, ?, ?, 1)`,
		id, email, passwordHash,
	); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return User{}, ErrEmailTaken
		}
		return User{}, err
	}
	if _, err := tx.ExecContext(
		ctx,
		`INSERT INTO user (
			id, name, email, role, profile_image_url,
			last_active_at, updated_at, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		id, name, email, role, profileImageURL, now, now, now,
	); err != nil {
		return User{}, err
	}
	if err := tx.Commit(); err != nil {
		return User{}, err
	}
	return s.UserByID(ctx, id)
}

func (s *Store) Authenticate(ctx context.Context, email string) (User, string, error) {
	var user User
	var password string
	err := s.db.QueryRowContext(
		ctx,
		`SELECT
			u.id, u.name, u.email, u.role, u.profile_image_url,
			u.last_active_at, u.updated_at, u.created_at,
			u.api_key, u.settings, u.info, u.oauth_sub, u.note,
			a.password
		 FROM user u JOIN auth a ON a.id = u.id
		 WHERE lower(u.email) = lower(?) AND a.active = 1`,
		strings.TrimSpace(email),
	).Scan(
		&user.ID, &user.Name, &user.Email, &user.Role, &user.ProfileImageURL,
		&user.LastActiveAt, &user.UpdatedAt, &user.CreatedAt,
		&user.APIKey, &user.Settings, &user.Info, &user.OAuthSub, &user.Note,
		&password,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, "", ErrInvalidPassword
	}
	return user, password, err
}

func (s *Store) UserByID(ctx context.Context, id string) (User, error) {
	var user User
	err := s.db.QueryRowContext(
		ctx,
		`SELECT id, name, email, role, profile_image_url,
			last_active_at, updated_at, created_at,
			api_key, settings, info, oauth_sub, note
		 FROM user WHERE id = ?`,
		id,
	).Scan(
		&user.ID, &user.Name, &user.Email, &user.Role, &user.ProfileImageURL,
		&user.LastActiveAt, &user.UpdatedAt, &user.CreatedAt,
		&user.APIKey, &user.Settings, &user.Info, &user.OAuthSub, &user.Note,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrUserNotFound
	}
	return user, err
}

func (s *Store) TouchUser(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(
		ctx,
		`UPDATE user SET last_active_at = ? WHERE id = ?`,
		time.Now().Unix(), id,
	)
	return err
}
