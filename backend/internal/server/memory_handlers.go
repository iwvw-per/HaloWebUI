package server

import (
	"errors"
	"net/http"
	"strings"

	"github.com/iwvw-per/HaloWebUI/backend/internal/auth"
	"github.com/iwvw-per/HaloWebUI/backend/internal/store"
)

func (a *App) handleMemories(w http.ResponseWriter, r *http.Request) {
	user, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	memories, err := a.store.ListMemories(r.Context(), user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load memories")
		return
	}
	writeJSON(w, http.StatusOK, memories)
}

func (a *App) handleMemoryAdd(w http.ResponseWriter, r *http.Request) {
	user, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	var form struct {
		Content string `json:"content"`
	}
	if !decodeJSON(w, r, &form) {
		return
	}
	form.Content = strings.TrimSpace(form.Content)
	if form.Content == "" {
		writeError(w, http.StatusBadRequest, "content is required")
		return
	}
	memory, err := a.store.CreateMemory(r.Context(), store.Memory{
		ID: auth.RandomIDForInternalUse(), UserID: user.ID, Content: form.Content,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to add memory")
		return
	}
	writeJSON(w, http.StatusOK, memory)
}

func (a *App) handleMemoryByID(w http.ResponseWriter, r *http.Request) {
	user, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	id := r.PathValue("memory_id")
	if r.Method == http.MethodDelete {
		deleted, err := a.store.DeleteMemory(r.Context(), id, user.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to delete memory")
			return
		}
		writeJSON(w, http.StatusOK, deleted)
		return
	}
	var form struct {
		Content *string `json:"content"`
	}
	if !decodeJSON(w, r, &form) {
		return
	}
	if form.Content == nil || strings.TrimSpace(*form.Content) == "" {
		writeError(w, http.StatusBadRequest, "content is required")
		return
	}
	memory, err := a.store.UpdateMemory(r.Context(), id, user.ID, strings.TrimSpace(*form.Content))
	if errors.Is(err, store.ErrMemoryNotFound) {
		writeError(w, http.StatusNotFound, "Memory not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update memory")
		return
	}
	writeJSON(w, http.StatusOK, memory)
}

func (a *App) handleMemoryDeleteAll(w http.ResponseWriter, r *http.Request) {
	user, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	deleted, err := a.store.DeleteMemories(r.Context(), user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to clear memories")
		return
	}
	writeJSON(w, http.StatusOK, deleted)
}

func (a *App) handleMemoryQuery(w http.ResponseWriter, r *http.Request) {
	user, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	var form struct {
		Content string `json:"content"`
		K       int    `json:"k"`
	}
	if !decodeJSON(w, r, &form) {
		return
	}
	memories, distances, err := a.store.SearchMemories(r.Context(), user.ID, form.Content, form.K)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "memory query failed")
		return
	}
	ids := make([]string, 0, len(memories))
	documents := make([]string, 0, len(memories))
	metadata := make([]map[string]any, 0, len(memories))
	for _, memory := range memories {
		ids = append(ids, memory.ID)
		documents = append(documents, memory.Content)
		metadata = append(metadata, map[string]any{
			"created_at": memory.CreatedAt, "updated_at": memory.UpdatedAt,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ids": [][]string{ids}, "documents": [][]string{documents},
		"metadatas": [][]map[string]any{metadata}, "distances": [][]float64{distances},
	})
}

func (a *App) handleMemoryReset(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.requireUser(w, r); !ok {
		return
	}
	// Local lexical search reads directly from SQLite, so rebuilding its index
	// is a no-op. Remote vector adapters may hook this route later.
	writeJSON(w, http.StatusOK, true)
}
