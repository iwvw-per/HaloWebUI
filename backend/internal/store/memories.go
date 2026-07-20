package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

// Memory is intentionally stored as plain text. Embeddings remain an optional
// remote capability; the control process still provides deterministic lexical
// search when no vector worker is configured.
type Memory struct {
	ID        string `json:"id"`
	UserID    string `json:"user_id"`
	Content   string `json:"content"`
	CreatedAt int64  `json:"created_at"`
	UpdatedAt int64  `json:"updated_at"`
}

var ErrMemoryNotFound = errors.New("memory not found")

func (s *Store) migrateMemories(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS memory (
		id TEXT PRIMARY KEY,
		user_id TEXT NOT NULL,
		content TEXT NOT NULL,
		created_at INTEGER NOT NULL,
		updated_at INTEGER NOT NULL
	)`)
	return err
}

func (s *Store) CreateMemory(ctx context.Context, memory Memory) (Memory, error) {
	now := time.Now().Unix()
	if memory.CreatedAt == 0 {
		memory.CreatedAt = now
	}
	memory.UpdatedAt = now
	_, err := s.db.ExecContext(ctx, `INSERT INTO memory (id, user_id, content, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
		memory.ID, memory.UserID, memory.Content, memory.CreatedAt, memory.UpdatedAt)
	if err != nil {
		return Memory{}, err
	}
	return s.MemoryByID(ctx, memory.ID, memory.UserID)
}

func (s *Store) MemoryByID(ctx context.Context, id, userID string) (Memory, error) {
	var memory Memory
	err := s.db.QueryRowContext(ctx, `SELECT id, user_id, content, created_at, updated_at FROM memory WHERE id = ? AND user_id = ?`, id, userID).
		Scan(&memory.ID, &memory.UserID, &memory.Content, &memory.CreatedAt, &memory.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Memory{}, ErrMemoryNotFound
	}
	return memory, err
}

func (s *Store) ListMemories(ctx context.Context, userID string) ([]Memory, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, user_id, content, created_at, updated_at FROM memory WHERE user_id = ? ORDER BY updated_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	memories := make([]Memory, 0)
	for rows.Next() {
		var memory Memory
		if err := rows.Scan(&memory.ID, &memory.UserID, &memory.Content, &memory.CreatedAt, &memory.UpdatedAt); err != nil {
			return nil, err
		}
		memories = append(memories, memory)
	}
	return memories, rows.Err()
}

func (s *Store) UpdateMemory(ctx context.Context, id, userID, content string) (Memory, error) {
	result, err := s.db.ExecContext(ctx, `UPDATE memory SET content = ?, updated_at = ? WHERE id = ? AND user_id = ?`, content, time.Now().Unix(), id, userID)
	if err != nil {
		return Memory{}, err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return Memory{}, ErrMemoryNotFound
	}
	return s.MemoryByID(ctx, id, userID)
}

func (s *Store) DeleteMemory(ctx context.Context, id, userID string) (bool, error) {
	result, err := s.db.ExecContext(ctx, `DELETE FROM memory WHERE id = ? AND user_id = ?`, id, userID)
	if err != nil {
		return false, err
	}
	affected, _ := result.RowsAffected()
	return affected != 0, nil
}

func (s *Store) DeleteMemories(ctx context.Context, userID string) (bool, error) {
	result, err := s.db.ExecContext(ctx, `DELETE FROM memory WHERE user_id = ?`, userID)
	if err != nil {
		return false, err
	}
	affected, _ := result.RowsAffected()
	return affected != 0, nil
}

// SearchMemories uses a bounded token-overlap score. It is deterministic,
// cheap on a 256 MiB host, and is replaced by a remote vector adapter when
// one is configured without changing the HTTP contract.
func (s *Store) SearchMemories(ctx context.Context, userID, query string, limit int) ([]Memory, []float64, error) {
	memories, err := s.ListMemories(ctx, userID)
	if err != nil {
		return nil, nil, err
	}
	if limit < 1 || limit > 100 {
		limit = 1
	}
	queryTokens := tokenSet(query)
	type scored struct {
		memory Memory
		score  float64
	}
	scoredMemories := make([]scored, 0, len(memories))
	for _, memory := range memories {
		tokens := tokenSet(memory.Content)
		if len(queryTokens) == 0 || len(tokens) == 0 {
			continue
		}
		matches := 0
		for token := range queryTokens {
			if _, ok := tokens[token]; ok {
				matches++
			}
		}
		if matches > 0 {
			scoredMemories = append(scoredMemories, scored{memory: memory, score: float64(matches) / float64(len(queryTokens))})
		}
	}
	// Stable insertion sort avoids importing a heavier ranking package and
	// keeps newer memories first for ties.
	for i := 1; i < len(scoredMemories); i++ {
		current := scoredMemories[i]
		j := i
		for j > 0 && (scoredMemories[j-1].score < current.score || (scoredMemories[j-1].score == current.score && scoredMemories[j-1].memory.UpdatedAt < current.memory.UpdatedAt)) {
			scoredMemories[j] = scoredMemories[j-1]
			j--
		}
		scoredMemories[j] = current
	}
	if len(scoredMemories) > limit {
		scoredMemories = scoredMemories[:limit]
	}
	result := make([]Memory, 0, len(scoredMemories))
	scores := make([]float64, 0, len(scoredMemories))
	for _, item := range scoredMemories {
		result = append(result, item.memory)
		scores = append(scores, 1-item.score)
	}
	return result, scores, nil
}

func tokenSet(value string) map[string]struct{} {
	result := make(map[string]struct{})
	for _, token := range strings.Fields(strings.ToLower(value)) {
		token = strings.Trim(token, ".,!?;:()[]{}\"'")
		if len([]rune(token)) >= 2 {
			result[token] = struct{}{}
		}
	}
	return result
}
