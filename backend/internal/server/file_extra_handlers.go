package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"

	"github.com/iwvw-per/HaloWebUI/backend/internal/store"
)

func (a *App) handleFileSearch(w http.ResponseWriter, r *http.Request) {
	user, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	files, err := a.store.ListFiles(r.Context(), user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list files")
		return
	}
	query := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("query")))
	if query != "" {
		filtered := files[:0]
		for _, file := range files {
			if strings.Contains(strings.ToLower(file.Filename), query) {
				filtered = append(filtered, file)
			}
		}
		files = filtered
	}
	writeJSON(w, http.StatusOK, files)
}

func (a *App) handleFileUploadDir(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.requireUser(w, r); !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": true, "files": []any{}})
}

func (a *App) handleAllFilesDelete(w http.ResponseWriter, r *http.Request) {
	user, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	files, err := a.store.ListFiles(r.Context(), user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list files")
		return
	}
	for _, file := range files {
		if file.Path != nil {
			if path, safe := safeDataPath(a.config.DataDir, *file.Path); safe {
				_ = removePath(path)
			}
		}
	}
	if err := a.store.DeleteAllFiles(r.Context(), user.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete files")
		return
	}
	writeJSON(w, http.StatusOK, true)
}

func (a *App) handleFileDataContent(w http.ResponseWriter, r *http.Request) {
	user, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	file, err := a.store.FileByID(r.Context(), r.PathValue("id"))
	if errors.Is(err, store.ErrFileNotFound) || (err == nil && file.UserID != user.ID && user.Role != "admin") {
		writeError(w, http.StatusNotFound, "File not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load file")
		return
	}
	if len(file.Data) == 0 {
		writeJSON(w, http.StatusOK, map[string]string{"content": ""})
		return
	}
	var data map[string]any
	if json.Unmarshal(file.Data, &data) == nil {
		writeJSON(w, http.StatusOK, data)
		return
	}
	writeRawJSON(w, http.StatusOK, file.Data)
}

func removePath(path string) error {
	return os.Remove(path)
}
