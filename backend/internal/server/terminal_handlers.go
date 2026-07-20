package server

import (
	"database/sql"
	"encoding/hex"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
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

var terminalSQLiteReadPattern = regexp.MustCompile(`(?is)^\s*(select|pragma|explain)\b`)

func (a *App) openTerminalSQLite(w http.ResponseWriter, r *http.Request, relative string) (*sql.DB, bool) {
	if !a.requireTerminalAdmin(w, r) {
		return nil, false
	}
	path, ok := a.terminalPath(relative)
	if !ok {
		writeError(w, http.StatusForbidden, "Path traversal not allowed")
		return nil, false
	}
	ext := strings.ToLower(filepath.Ext(path))
	if ext != ".db" && ext != ".sqlite" && ext != ".sqlite3" {
		writeError(w, http.StatusBadRequest, "Not a SQLite database file")
		return nil, false
	}
	if info, err := os.Stat(path); err != nil || info.IsDir() {
		writeError(w, http.StatusNotFound, "Database file not found")
		return nil, false
	}
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(path)+"?mode=ro&_pragma=query_only(1)&_pragma=busy_timeout(3000)")
	if err != nil {
		writeError(w, http.StatusBadRequest, "Cannot open database")
		return nil, false
	}
	if err := db.PingContext(r.Context()); err != nil {
		db.Close()
		writeError(w, http.StatusBadRequest, "Cannot open database: "+err.Error())
		return nil, false
	}
	return db, true
}

func (a *App) handleTerminalSQLiteTables(w http.ResponseWriter, r *http.Request) {
	db, ok := a.openTerminalSQLite(w, r, r.URL.Query().Get("path"))
	if !ok {
		return
	}
	defer db.Close()
	rows, err := db.QueryContext(r.Context(), `SELECT name FROM sqlite_master WHERE type='table' ORDER BY name`)
	if err != nil {
		writeError(w, http.StatusBadRequest, "SQLite error: "+err.Error())
		return
	}
	names := []string{}
	for rows.Next() {
		var name string
		if rows.Scan(&name) == nil {
			names = append(names, name)
		}
	}
	rows.Close()
	tables := make([]map[string]any, 0, len(names))
	for _, name := range names {
		columnsRows, err := db.QueryContext(r.Context(), `PRAGMA table_info(`+quoteIdentifier(name)+`)`)
		if err != nil {
			continue
		}
		columns := []map[string]any{}
		for columnsRows.Next() {
			var cid, notNull, pk int
			var columnName, columnType string
			var defaultValue any
			if columnsRows.Scan(&cid, &columnName, &columnType, &notNull, &defaultValue, &pk) == nil {
				columns = append(columns, map[string]any{"name": columnName, "type": columnType, "notnull": notNull != 0, "pk": pk != 0})
			}
		}
		columnsRows.Close()
		tables = append(tables, map[string]any{"name": name, "columns": columns})
	}
	writeJSON(w, http.StatusOK, tables)
}

func (a *App) handleTerminalSQLiteQuery(w http.ResponseWriter, r *http.Request) {
	var form struct {
		Path  string `json:"path"`
		Query string `json:"query"`
		Limit int    `json:"limit"`
	}
	if !decodeJSON(w, r, &form) {
		return
	}
	if !terminalSQLiteReadPattern.MatchString(form.Query) {
		writeError(w, http.StatusBadRequest, "Only SELECT, PRAGMA, and EXPLAIN statements are allowed")
		return
	}
	if strings.Contains(form.Query, ";") {
		trimmed := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(form.Query), ";"))
		if strings.Contains(trimmed, ";") {
			writeError(w, http.StatusBadRequest, "Only one statement is allowed")
			return
		}
		form.Query = trimmed
	}
	if form.Limit < 1 {
		form.Limit = 100
	}
	if form.Limit > 1000 {
		form.Limit = 1000
	}
	db, ok := a.openTerminalSQLite(w, r, form.Path)
	if !ok {
		return
	}
	defer db.Close()
	rows, err := db.QueryContext(r.Context(), form.Query)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Query error: "+err.Error())
		return
	}
	defer rows.Close()
	columns, err := rows.Columns()
	if err != nil {
		writeError(w, http.StatusBadRequest, "Query error: "+err.Error())
		return
	}
	result := make([][]any, 0)
	for rows.Next() && len(result) < form.Limit {
		values := make([]any, len(columns))
		targets := make([]any, len(columns))
		for i := range values {
			targets[i] = &values[i]
		}
		if err := rows.Scan(targets...); err != nil {
			writeError(w, http.StatusBadRequest, "Query error: "+err.Error())
			return
		}
		for i, value := range values {
			if bytesValue, ok := value.([]byte); ok {
				values[i] = hex.EncodeToString(bytesValue)
			}
		}
		result = append(result, values)
	}
	writeJSON(w, http.StatusOK, map[string]any{"columns": columns, "rows": result, "rowCount": len(result)})
}
