package server

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/iwvw-per/HaloWebUI/backend/internal/auth"
	"github.com/iwvw-per/HaloWebUI/backend/internal/store"
)

type knowledgeResponse struct {
	store.Knowledge
	Files []store.File `json:"files"`
}

func (a *App) canReadKnowledge(user store.User, knowledge store.Knowledge) bool {
	if user.Role == "admin" || knowledge.UserID == user.ID || len(knowledge.AccessControl) == 0 || string(knowledge.AccessControl) == "null" {
		return true
	}
	var access map[string]any
	if json.Unmarshal(knowledge.AccessControl, &access) != nil || len(access) == 0 {
		return false
	}
	for _, permission := range []string{"read", "write"} {
		entry, _ := access[permission].(map[string]any)
		users, _ := entry["user_ids"].([]any)
		for _, candidate := range users {
			if candidate == user.ID {
				return true
			}
		}
	}
	return false
}

func (a *App) canWriteKnowledge(user store.User, knowledge store.Knowledge) bool {
	if user.Role == "admin" || knowledge.UserID == user.ID {
		return true
	}
	var access map[string]any
	_ = json.Unmarshal(knowledge.AccessControl, &access)
	entry, _ := access["write"].(map[string]any)
	users, _ := entry["user_ids"].([]any)
	for _, candidate := range users {
		if candidate == user.ID {
			return true
		}
	}
	return false
}

func (a *App) knowledgeWithFiles(r *http.Request, knowledge store.Knowledge) knowledgeResponse {
	files := make([]store.File, 0)
	validIDs := make([]string, 0)
	for _, id := range store.KnowledgeFileIDs(knowledge) {
		file, err := a.store.FileByID(r.Context(), id)
		if err == nil {
			files = append(files, file)
			validIDs = append(validIDs, id)
		}
	}
	if len(validIDs) != len(store.KnowledgeFileIDs(knowledge)) {
		var data map[string]any
		_ = json.Unmarshal(knowledge.Data, &data)
		if data == nil {
			data = map[string]any{}
		}
		data["file_ids"] = validIDs
		knowledge.Data, _ = json.Marshal(data)
		knowledge, _ = a.store.UpdateKnowledge(r.Context(), knowledge)
	}
	return knowledgeResponse{Knowledge: knowledge, Files: files}
}

func (a *App) handleKnowledgeList(w http.ResponseWriter, r *http.Request) {
	user, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	items, err := a.store.ListKnowledge(r.Context(), "")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load knowledge bases")
		return
	}
	visible := make([]knowledgeResponse, 0, len(items))
	for _, item := range items {
		if a.canReadKnowledge(user, item) {
			visible = append(visible, a.knowledgeWithFiles(r, item))
		}
	}
	if page := pageQuery(r, 0); page > 0 {
		limit := queryInt(r, "limit", 50, 1, 200)
		start := (page - 1) * limit
		if start >= len(visible) {
			visible = []knowledgeResponse{}
		} else {
			end := start + limit
			if end > len(visible) {
				end = len(visible)
			}
			visible = visible[start:end]
		}
	}
	writeJSON(w, http.StatusOK, visible)
}

func (a *App) handleKnowledgeSearch(w http.ResponseWriter, r *http.Request) {
	user, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	items, err := a.store.ListKnowledge(r.Context(), r.URL.Query().Get("query"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to search knowledge bases")
		return
	}
	view := r.URL.Query().Get("view_option")
	visible := make([]knowledgeResponse, 0)
	for _, item := range items {
		if !a.canReadKnowledge(user, item) || (view == "created" && item.UserID != user.ID) || (view == "shared" && item.UserID == user.ID) {
			continue
		}
		visible = append(visible, a.knowledgeWithFiles(r, item))
	}
	total := len(visible)
	visible = paginateKnowledge(visible, queryInt(r, "page", 1, 1, 100000), queryInt(r, "limit", 30, 1, 100))
	writeJSON(w, http.StatusOK, map[string]any{"items": visible, "total": total})
}

func (a *App) handleKnowledgeFileSearch(w http.ResponseWriter, r *http.Request) {
	user, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	knowledge, err := a.store.ListKnowledge(r.Context(), "")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to search knowledge files")
		return
	}
	query := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("query")))
	items := make([]map[string]any, 0)
	seen := map[string]bool{}
	for _, item := range knowledge {
		if !a.canReadKnowledge(user, item) {
			continue
		}
		for _, fileID := range store.KnowledgeFileIDs(item) {
			if seen[fileID] {
				continue
			}
			file, fileErr := a.store.FileByID(r.Context(), fileID)
			if fileErr != nil {
				continue
			}
			if query != "" && !strings.Contains(strings.ToLower(file.Filename+" "+item.Name+" "+item.Description), query) {
				continue
			}
			seen[fileID] = true
			items = append(items, map[string]any{
				"id": file.ID, "meta": rawObject(file.Meta), "filename": file.Filename,
				"name": file.Filename, "type": "file", "created_at": file.CreatedAt, "updated_at": file.UpdatedAt,
				"collection": map[string]any{"id": item.ID, "name": item.Name, "description": item.Description},
			})
		}
	}
	total := len(items)
	page, limit := queryInt(r, "page", 1, 1, 100000), queryInt(r, "limit", 30, 1, 100)
	start := (page - 1) * limit
	if start >= len(items) {
		items = []map[string]any{}
	} else {
		end := start + limit
		if end > len(items) {
			end = len(items)
		}
		items = items[start:end]
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "total": total})
}

func (a *App) handleKnowledgeCreate(w http.ResponseWriter, r *http.Request) {
	user, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	var form struct {
		Name          string          `json:"name"`
		Description   string          `json:"description"`
		Data          json.RawMessage `json:"data"`
		AccessControl json.RawMessage `json:"access_control"`
	}
	if !decodeJSON(w, r, &form) {
		return
	}
	if strings.TrimSpace(form.Name) == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	knowledge, err := a.store.CreateKnowledge(r.Context(), store.Knowledge{
		ID: auth.RandomIDForInternalUse(), UserID: user.ID, Name: strings.TrimSpace(form.Name),
		Description: form.Description, Data: form.Data, AccessControl: form.AccessControl,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to create knowledge base")
		return
	}
	writeJSON(w, http.StatusOK, a.knowledgeWithFiles(r, knowledge))
}

func (a *App) knowledgeForRequest(w http.ResponseWriter, r *http.Request, write bool) (store.User, store.Knowledge, bool) {
	user, ok := a.requireUser(w, r)
	if !ok {
		return store.User{}, store.Knowledge{}, false
	}
	knowledge, err := a.store.KnowledgeByID(r.Context(), r.PathValue("id"))
	if errors.Is(err, store.ErrKnowledgeNotFound) {
		writeError(w, http.StatusNotFound, "Knowledge base not found")
		return store.User{}, store.Knowledge{}, false
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load knowledge base")
		return store.User{}, store.Knowledge{}, false
	}
	allowed := a.canReadKnowledge(user, knowledge)
	if write {
		allowed = a.canWriteKnowledge(user, knowledge)
	}
	if !allowed {
		writeError(w, http.StatusForbidden, "Access prohibited")
		return store.User{}, store.Knowledge{}, false
	}
	return user, knowledge, true
}

func (a *App) handleKnowledgeByID(w http.ResponseWriter, r *http.Request) {
	_, knowledge, ok := a.knowledgeForRequest(w, r, r.Method != http.MethodGet)
	if !ok {
		return
	}
	if r.Method == http.MethodGet {
		writeJSON(w, http.StatusOK, a.knowledgeWithFiles(r, knowledge))
		return
	}
	if r.Method == http.MethodDelete {
		deleted, err := a.store.DeleteKnowledge(r.Context(), knowledge.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to delete knowledge base")
			return
		}
		_ = a.store.DeleteRetrievalCollection(r.Context(), knowledge.ID, knowledge.UserID)
		writeJSON(w, http.StatusOK, deleted)
		return
	}
	var patch map[string]json.RawMessage
	if !decodeJSON(w, r, &patch) {
		return
	}
	if raw, ok := patch["name"]; ok {
		_ = json.Unmarshal(raw, &knowledge.Name)
	}
	if raw, ok := patch["description"]; ok {
		_ = json.Unmarshal(raw, &knowledge.Description)
	}
	if raw, ok := patch["data"]; ok {
		knowledge.Data = raw
	}
	if raw, ok := patch["access_control"]; ok {
		knowledge.AccessControl = raw
	}
	updated, err := a.store.UpdateKnowledge(r.Context(), knowledge)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update knowledge base")
		return
	}
	writeJSON(w, http.StatusOK, a.knowledgeWithFiles(r, updated))
}

func (a *App) handleKnowledgeFile(w http.ResponseWriter, r *http.Request) {
	user, knowledge, ok := a.knowledgeForRequest(w, r, true)
	if !ok {
		return
	}
	var form struct {
		FileID string `json:"file_id"`
	}
	if !decodeJSON(w, r, &form) {
		return
	}
	file, err := a.store.FileByID(r.Context(), form.FileID)
	if err != nil || (file.UserID != user.ID && user.Role != "admin") {
		writeError(w, http.StatusNotFound, "File not found")
		return
	}
	add := !strings.HasSuffix(r.URL.Path, "/file/remove")
	updated, err := a.store.SetKnowledgeFile(r.Context(), knowledge.ID, form.FileID, add)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update knowledge files")
		return
	}
	owner, ownerErr := a.store.UserByID(r.Context(), knowledge.UserID)
	if ownerErr != nil {
		owner = user
	}
	if add {
		_ = a.indexStoredFile(r, owner, file, knowledge.ID)
	} else {
		_ = a.store.DeleteRetrievalCollection(r.Context(), knowledge.ID, owner.ID)
		for _, existing := range store.KnowledgeFileIDs(updated) {
			if candidate, loadErr := a.store.FileByID(r.Context(), existing); loadErr == nil {
				_ = a.indexStoredFile(r, owner, candidate, knowledge.ID)
			}
		}
	}
	writeJSON(w, http.StatusOK, a.knowledgeWithFiles(r, updated))
}

func (a *App) handleKnowledgeReset(w http.ResponseWriter, r *http.Request) {
	user, knowledge, ok := a.knowledgeForRequest(w, r, true)
	if !ok {
		return
	}
	owner, ownerErr := a.store.UserByID(r.Context(), knowledge.UserID)
	if ownerErr != nil {
		owner = user
	}
	if err := a.store.DeleteRetrievalCollection(r.Context(), knowledge.ID, owner.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to reset knowledge base")
		return
	}
	for _, fileID := range store.KnowledgeFileIDs(knowledge) {
		if file, err := a.store.FileByID(r.Context(), fileID); err == nil {
			_ = a.indexStoredFile(r, owner, file, knowledge.ID)
		}
	}
	writeJSON(w, http.StatusOK, a.knowledgeWithFiles(r, knowledge))
}

func (a *App) handleKnowledgeReindex(w http.ResponseWriter, r *http.Request) {
	if ok, _ := a.requireAdmin(w, r); !ok {
		return
	}
	items, err := a.store.ListKnowledge(r.Context(), "")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to reindex knowledge bases")
		return
	}
	for _, knowledge := range items {
		_ = a.store.DeleteRetrievalCollection(r.Context(), knowledge.ID, knowledge.UserID)
		owner, ownerErr := a.store.UserByID(r.Context(), knowledge.UserID)
		if ownerErr != nil {
			continue
		}
		for _, fileID := range store.KnowledgeFileIDs(knowledge) {
			if file, fileErr := a.store.FileByID(r.Context(), fileID); fileErr == nil {
				_ = a.indexStoredFile(r, owner, file, knowledge.ID)
			}
		}
	}
	writeJSON(w, http.StatusOK, true)
}

func (a *App) handleKnowledgeExport(w http.ResponseWriter, r *http.Request) {
	_, knowledge, ok := a.knowledgeForRequest(w, r, false)
	if !ok {
		return
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="knowledge-`+knowledge.ID+`.zip"`)
	archive := zip.NewWriter(w)
	manifest, _ := archive.Create("knowledge.json")
	_ = json.NewEncoder(manifest).Encode(a.knowledgeWithFiles(r, knowledge))
	for _, fileID := range store.KnowledgeFileIDs(knowledge) {
		file, err := a.store.FileByID(r.Context(), fileID)
		if err != nil || file.Path == nil {
			continue
		}
		path, safe := safeDataPath(a.config.DataDir, *file.Path)
		if !safe {
			continue
		}
		source, err := os.Open(path)
		if err != nil {
			continue
		}
		target, createErr := archive.Create("files/" + filepath.Base(strings.ReplaceAll(file.Filename, "\\", "/")))
		if createErr == nil {
			_, _ = io.CopyN(target, source, a.config.FileMaxSizeBytes)
		}
		_ = source.Close()
	}
	_ = archive.Close()
}

func paginateKnowledge(items []knowledgeResponse, page, limit int) []knowledgeResponse {
	start := (page - 1) * limit
	if start >= len(items) {
		return []knowledgeResponse{}
	}
	end := start + limit
	if end > len(items) {
		end = len(items)
	}
	return items[start:end]
}

func queryInt(r *http.Request, key string, fallback, min, max int) int {
	value, err := strconv.Atoi(r.URL.Query().Get(key))
	if err != nil || value < min {
		return fallback
	}
	if value > max {
		return max
	}
	return value
}

func rawObject(raw json.RawMessage) any {
	if len(raw) == 0 {
		return map[string]any{}
	}
	var value any
	if json.Unmarshal(raw, &value) != nil {
		return map[string]any{}
	}
	return value
}
