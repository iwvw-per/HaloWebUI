package server

import (
	"context"
	"time"

	"github.com/iwvw-per/HaloWebUI/backend/internal/auth"
)

type taskEntry struct {
	ID               string
	UserID           string
	ChatID           string
	BlocksCompletion bool
	CreatedAt        time.Time
	Cancel           context.CancelFunc
}

func (a *App) beginTask(parent context.Context, userID, chatID string, blocksCompletion bool) (string, context.Context, func()) {
	ctx, cancel := context.WithCancel(parent)
	entry := &taskEntry{
		ID:               auth.RandomIDForInternalUse(),
		UserID:           userID,
		ChatID:           chatID,
		BlocksCompletion: blocksCompletion,
		CreatedAt:        time.Now(),
		Cancel:           cancel,
	}
	a.tasksMu.Lock()
	a.tasks[entry.ID] = entry
	a.tasksMu.Unlock()
	finish := func() {
		a.tasksMu.Lock()
		if current, ok := a.tasks[entry.ID]; ok && current == entry {
			delete(a.tasks, entry.ID)
		}
		a.tasksMu.Unlock()
		cancel()
	}
	return entry.ID, ctx, finish
}

func (a *App) stopTask(id, userID string) bool {
	a.tasksMu.Lock()
	entry, ok := a.tasks[id]
	if !ok || entry.UserID != userID {
		a.tasksMu.Unlock()
		return false
	}
	delete(a.tasks, id)
	a.tasksMu.Unlock()
	entry.Cancel()
	return true
}

func (a *App) taskIDsForUserChat(userID, chatID string) []string {
	a.tasksMu.Lock()
	defer a.tasksMu.Unlock()
	result := make([]string, 0)
	for id, entry := range a.tasks {
		if entry.UserID == userID && entry.ChatID == chatID && entry.BlocksCompletion {
			result = append(result, id)
		}
	}
	return result
}

func (a *App) taskSnapshot(userID string) []map[string]any {
	a.tasksMu.Lock()
	defer a.tasksMu.Unlock()
	result := make([]map[string]any, 0)
	for _, entry := range a.tasks {
		if entry.UserID != userID {
			continue
		}
		result = append(result, map[string]any{
			"id":                entry.ID,
			"chat_id":           entry.ChatID,
			"blocks_completion": entry.BlocksCompletion,
			"created_at":        entry.CreatedAt.UTC().Format(time.RFC3339),
		})
	}
	return result
}
