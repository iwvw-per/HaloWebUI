package server

import (
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

const terminalMaxTextBytes = 10 << 20

func (a *App) requireTerminalAdmin(w http.ResponseWriter, r *http.Request) bool {
	if ok, _ := a.requireAdmin(w, r); !ok {
		return false
	}
	if !a.config.EnableTerminal {
		writeError(w, http.StatusForbidden, "Terminal feature is disabled")
		return false
	}
	return true
}

func (a *App) terminalRoot() string {
	return filepath.Join(a.config.DataDir, "workspace")
}

func (a *App) terminalPath(relative string) (string, bool) {
	clean := strings.TrimLeft(strings.ReplaceAll(relative, "\\", "/"), "/")
	return safeDataPath(a.terminalRoot(), clean)
}

func (a *App) handleTerminalConfig(w http.ResponseWriter, r *http.Request) {
	if ok, _ := a.requireAdmin(w, r); !ok {
		return
	}
	if r.Method == http.MethodPost {
		enabled := r.URL.Query().Get("enabled")
		if enabled != "" {
			writeJSON(w, http.StatusOK, map[string]any{"enabled": enabled == "true" || enabled == "1"})
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"enabled": a.config.EnableTerminal, "workspace": a.terminalRoot()})
}

func (a *App) handleTerminalFiles(w http.ResponseWriter, r *http.Request) {
	if !a.requireTerminalAdmin(w, r) {
		return
	}
	path, ok := a.terminalPath(r.URL.Query().Get("path"))
	if !ok {
		writeError(w, http.StatusForbidden, "Path traversal not allowed")
		return
	}
	if r.Method == http.MethodDelete {
		if filepath.Clean(path) == filepath.Clean(a.terminalRoot()) {
			writeError(w, http.StatusForbidden, "Cannot delete workspace root")
			return
		}
		if err := os.RemoveAll(path); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to delete path")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"deleted": true, "path": filepath.ToSlash(strings.TrimPrefix(path, a.terminalRoot()+string(filepath.Separator)))})
		return
	}
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		writeError(w, http.StatusNotFound, "Directory not found")
		return
	}
	if err != nil || !info.IsDir() {
		writeError(w, http.StatusBadRequest, "Path is not a directory")
		return
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		writeError(w, http.StatusForbidden, "Permission denied")
		return
	}
	type fileEntry struct {
		Name        string  `json:"name"`
		Path        string  `json:"path"`
		IsDir       bool    `json:"is_dir"`
		Size        int64   `json:"size"`
		Modified    float64 `json:"modified"`
		Permissions string  `json:"permissions"`
	}
	result := make([]fileEntry, 0, len(entries))
	for _, entry := range entries {
		entryInfo, statErr := entry.Info()
		if statErr != nil {
			continue
		}
		relative, _ := filepath.Rel(a.terminalRoot(), filepath.Join(path, entry.Name()))
		result = append(result, fileEntry{Name: entry.Name(), Path: filepath.ToSlash(relative), IsDir: entryInfo.IsDir(), Size: func() int64 {
			if entryInfo.IsDir() {
				return 0
			}
			return entryInfo.Size()
		}(), Modified: float64(entryInfo.ModTime().UnixNano()) / 1e9, Permissions: entryInfo.Mode().Perm().String()})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].IsDir != result[j].IsDir {
			return result[i].IsDir
		}
		return strings.ToLower(result[i].Name) < strings.ToLower(result[j].Name)
	})
	writeJSON(w, http.StatusOK, result)
}

func (a *App) handleTerminalFileContent(w http.ResponseWriter, r *http.Request) {
	if !a.requireTerminalAdmin(w, r) {
		return
	}
	if r.Method == http.MethodPost {
		var form struct {
			Path    string `json:"path"`
			Content string `json:"content"`
		}
		if !decodeJSON(w, r, &form) {
			return
		}
		if int64(len([]byte(form.Content))) > terminalMaxTextBytes {
			writeError(w, http.StatusRequestEntityTooLarge, "File too large (max 10MB)")
			return
		}
		path, ok := a.terminalPath(form.Path)
		if !ok {
			writeError(w, http.StatusForbidden, "Path traversal not allowed")
			return
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to create directory")
			return
		}
		if err := os.WriteFile(path, []byte(form.Content), 0o600); err != nil {
			writeError(w, http.StatusForbidden, "Permission denied")
			return
		}
		info, _ := os.Stat(path)
		writeJSON(w, http.StatusOK, map[string]any{"path": form.Path, "size": info.Size(), "modified": float64(info.ModTime().UnixNano()) / 1e9})
		return
	}
	path, ok := a.terminalPath(r.URL.Query().Get("path"))
	if !ok {
		writeError(w, http.StatusForbidden, "Path traversal not allowed")
		return
	}
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		writeError(w, http.StatusNotFound, "File not found")
		return
	}
	if err != nil || info.IsDir() {
		writeError(w, http.StatusBadRequest, "Cannot read a directory")
		return
	}
	if info.Size() > terminalMaxTextBytes {
		writeError(w, http.StatusRequestEntityTooLarge, "File too large (max 10MB)")
		return
	}
	content, err := os.ReadFile(path)
	if err != nil {
		writeError(w, http.StatusForbidden, "Permission denied")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"path": r.URL.Query().Get("path"), "content": string(content)})
}

func (a *App) handleTerminalFileBinary(w http.ResponseWriter, r *http.Request) {
	a.serveTerminalFile(w, r, true)
}
func (a *App) handleTerminalRaw(w http.ResponseWriter, r *http.Request) {
	a.serveTerminalFile(w, r, false)
}

func (a *App) serveTerminalFile(w http.ResponseWriter, r *http.Request, binaryOnly bool) {
	if !a.requireTerminalAdmin(w, r) {
		return
	}
	path, ok := a.terminalPath(r.URL.Query().Get("path"))
	if !ok {
		writeError(w, http.StatusForbidden, "Path traversal not allowed")
		return
	}
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		writeError(w, http.StatusNotFound, "File not found")
		return
	}
	if err != nil || info.IsDir() {
		writeError(w, http.StatusBadRequest, "Cannot read a directory")
		return
	}
	limit := int64(200 << 20)
	if binaryOnly {
		limit = 50 << 20
	}
	if info.Size() > limit {
		writeError(w, http.StatusRequestEntityTooLarge, "File too large")
		return
	}
	if binaryOnly {
		ext := strings.ToLower(filepath.Ext(path))
		switch ext {
		case ".xlsx", ".xls", ".docx", ".pptx":
		default:
			writeError(w, http.StatusBadRequest, "Binary preview not supported")
			return
		}
	}
	if contentType := mime.TypeByExtension(filepath.Ext(path)); contentType != "" {
		w.Header().Set("Content-Type", contentType)
	} else {
		w.Header().Set("Content-Type", "application/octet-stream")
	}
	w.Header().Set("Content-Disposition", fmt.Sprintf(`inline; filename="%s"`, strings.ReplaceAll(filepath.Base(path), `"`, "")))
	http.ServeFile(w, r, path)
}

func (a *App) handleTerminalMkdir(w http.ResponseWriter, r *http.Request) {
	if !a.requireTerminalAdmin(w, r) {
		return
	}
	var form struct {
		Path string `json:"path"`
	}
	if !decodeJSON(w, r, &form) {
		return
	}
	path, ok := a.terminalPath(form.Path)
	if !ok {
		writeError(w, http.StatusForbidden, "Path traversal not allowed")
		return
	}
	if err := os.MkdirAll(path, 0o700); err != nil {
		writeError(w, http.StatusConflict, "Path already exists")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"path": form.Path, "created": true})
}
func (a *App) handleTerminalRename(w http.ResponseWriter, r *http.Request) {
	if !a.requireTerminalAdmin(w, r) {
		return
	}
	var form struct {
		OldPath string `json:"old_path"`
		NewPath string `json:"new_path"`
	}
	if !decodeJSON(w, r, &form) {
		return
	}
	old, ok1 := a.terminalPath(form.OldPath)
	new, ok2 := a.terminalPath(form.NewPath)
	if !ok1 || !ok2 {
		writeError(w, http.StatusForbidden, "Path traversal not allowed")
		return
	}
	if err := os.MkdirAll(filepath.Dir(new), 0o700); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create directory")
		return
	}
	if err := os.Rename(old, new); err != nil {
		writeError(w, http.StatusNotFound, "Source path not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"old_path": form.OldPath, "new_path": form.NewPath})
}

func (a *App) handleTerminalUpload(w http.ResponseWriter, r *http.Request) {
	if !a.requireTerminalAdmin(w, r) {
		return
	}
	path, ok := a.terminalPath(r.URL.Query().Get("path"))
	if !ok {
		writeError(w, http.StatusForbidden, "Path traversal not allowed")
		return
	}
	if err := os.MkdirAll(path, 0o700); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create directory")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, a.config.FileMaxSizeBytes+(1<<20))
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "file is too large or invalid")
		return
	}
	src, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "file is required")
		return
	}
	defer src.Close()
	target := filepath.Join(path, filepath.Base(header.Filename))
	out, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		writeError(w, http.StatusConflict, "file already exists")
		return
	}
	written, copyErr := io.Copy(out, io.LimitReader(src, a.config.FileMaxSizeBytes+1))
	closeErr := out.Close()
	if copyErr != nil || closeErr != nil || written > a.config.FileMaxSizeBytes {
		_ = os.Remove(target)
		writeError(w, http.StatusBadRequest, "file exceeds configured size limit")
		return
	}
	relative, _ := filepath.Rel(a.terminalRoot(), target)
	writeJSON(w, http.StatusOK, map[string]any{"path": filepath.ToSlash(relative), "size": written, "modified": time.Now().Unix()})
}

func (a *App) handleTerminalPorts(w http.ResponseWriter, r *http.Request) {
	if !a.requireTerminalAdmin(w, r) {
		return
	}
	writeJSON(w, http.StatusOK, []map[string]any{{"port": a.config.Port, "pid": os.Getpid(), "process_name": "halowebui", "address": a.config.Host + ":" + strconv.Itoa(a.config.Port), "runtime": runtime.GOOS}})
}
