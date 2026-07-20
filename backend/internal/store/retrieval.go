package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

type RetrievalDocument struct {
	ID           string
	Collection   string
	UserID       string
	Filename     string
	Text         string
	MetadataJSON string
	CreatedAt    int64
	UpdatedAt    int64
}

var ErrRetrievalDocumentNotFound = errors.New("retrieval document not found")

func (s *Store) migrateRetrieval(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS retrieval_document (
		id TEXT NOT NULL,
		collection TEXT NOT NULL,
		user_id TEXT NOT NULL,
		filename TEXT NOT NULL DEFAULT '',
		text TEXT NOT NULL,
		metadata TEXT NOT NULL DEFAULT '{}',
		created_at INTEGER NOT NULL,
		updated_at INTEGER NOT NULL,
		PRIMARY KEY(collection, id)
	)`)
	return err
}

func (s *Store) UpsertRetrievalDocument(ctx context.Context, doc RetrievalDocument) (RetrievalDocument, error) {
	now := time.Now().Unix()
	if doc.CreatedAt == 0 {
		doc.CreatedAt = now
	}
	doc.UpdatedAt = now
	if doc.MetadataJSON == "" {
		doc.MetadataJSON = "{}"
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO retrieval_document (id, collection, user_id, filename, text, metadata, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(collection, id) DO UPDATE SET user_id=excluded.user_id, filename=excluded.filename, text=excluded.text, metadata=excluded.metadata, updated_at=excluded.updated_at`,
		doc.ID, doc.Collection, doc.UserID, doc.Filename, doc.Text, doc.MetadataJSON, doc.CreatedAt, doc.UpdatedAt)
	if err != nil {
		return RetrievalDocument{}, err
	}
	return s.RetrievalDocumentByID(ctx, doc.Collection, doc.ID)
}

func (s *Store) RetrievalDocumentByID(ctx context.Context, collection, id string) (RetrievalDocument, error) {
	var doc RetrievalDocument
	err := s.db.QueryRowContext(ctx, `SELECT id, collection, user_id, filename, text, metadata, created_at, updated_at FROM retrieval_document WHERE collection = ? AND id = ?`, collection, id).
		Scan(&doc.ID, &doc.Collection, &doc.UserID, &doc.Filename, &doc.Text, &doc.MetadataJSON, &doc.CreatedAt, &doc.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return RetrievalDocument{}, ErrRetrievalDocumentNotFound
	}
	return doc, err
}

func (s *Store) RetrievalDocuments(ctx context.Context, collections []string, userID string, query string, limit int) ([]RetrievalDocument, []float64, error) {
	if limit < 1 || limit > 200 {
		limit = 4
	}
	if len(collections) == 0 {
		return []RetrievalDocument{}, []float64{}, nil
	}
	placeholders := make([]string, len(collections))
	args := make([]any, 0, len(collections)+1)
	for i, collection := range collections {
		placeholders[i] = "?"
		args = append(args, collection)
	}
	args = append(args, userID)
	rows, err := s.db.QueryContext(ctx, `SELECT id, collection, user_id, filename, text, metadata, created_at, updated_at FROM retrieval_document WHERE collection IN (`+strings.Join(placeholders, ",")+`) AND user_id = ? ORDER BY updated_at DESC`, args...)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	queryTokens := tokenSet(query)
	type scored struct {
		doc   RetrievalDocument
		score float64
	}
	items := make([]scored, 0)
	for rows.Next() {
		var doc RetrievalDocument
		if err := rows.Scan(&doc.ID, &doc.Collection, &doc.UserID, &doc.Filename, &doc.Text, &doc.MetadataJSON, &doc.CreatedAt, &doc.UpdatedAt); err != nil {
			return nil, nil, err
		}
		score := 0.0
		if len(queryTokens) > 0 {
			matches := 0
			for token := range queryTokens {
				if strings.Contains(strings.ToLower(doc.Text), token) || strings.Contains(strings.ToLower(doc.Filename), token) {
					matches++
				}
			}
			if matches == 0 {
				continue
			}
			score = float64(matches) / float64(len(queryTokens))
		}
		items = append(items, scored{doc: doc, score: score})
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	for i := 1; i < len(items); i++ {
		current := items[i]
		j := i
		for j > 0 && (items[j-1].score < current.score || (items[j-1].score == current.score && items[j-1].doc.UpdatedAt < current.doc.UpdatedAt)) {
			items[j] = items[j-1]
			j--
		}
		items[j] = current
	}
	if len(items) > limit {
		items = items[:limit]
	}
	documents := make([]RetrievalDocument, 0, len(items))
	distances := make([]float64, 0, len(items))
	for _, item := range items {
		documents = append(documents, item.doc)
		distances = append(distances, 1-item.score)
	}
	return documents, distances, nil
}

func (s *Store) DeleteRetrievalCollection(ctx context.Context, collection, userID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM retrieval_document WHERE collection = ? AND user_id = ?`, collection, userID)
	return err
}

func (s *Store) DeleteAllRetrieval(ctx context.Context, userID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM retrieval_document WHERE user_id = ?`, userID)
	return err
}
