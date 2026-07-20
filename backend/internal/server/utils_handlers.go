package server

import (
	"archive/zip"
	"bufio"
	"bytes"
	"context"
	"crypto/md5"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/iwvw-per/HaloWebUI/backend/internal/auth"
	"github.com/iwvw-per/HaloWebUI/backend/internal/store"
)

const (
	backupKindSQLite = "sqlite"
	backupKindFull   = "full"
	restoreConfirm   = "RESTORE DATABASE"
	mergeConfirm     = "MERGE DATABASE"
)

type restoreSession struct {
	Path        string
	UploadsPath string
	UserID      string
	Kind        string
	Filename    string
	Expires     time.Time
}

const fullBackupUploadLimit = int64(512 << 20)

func adminExportEnabled() bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv("ENABLE_ADMIN_EXPORT")))
	return value != "false" && value != "0" && value != "no"
}

func (a *App) handleUtils(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/api/v1/utils/")
	switch name {
	case "gravatar":
		if _, ok := a.requireUser(w, r); !ok {
			return
		}
		email := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("email")))
		sum := md5.Sum([]byte(email))
		writeJSON(w, http.StatusOK, "https://www.gravatar.com/avatar/"+hex.EncodeToString(sum[:]))
	case "code/format":
		a.handleCodeFormat(w, r)
	case "code/execute":
		a.handleCodeExecute(w, r)
	case "markdown":
		a.handleMarkdown(w, r)
	case "pdf":
		a.handleChatPDF(w, r)
	case "db/download":
		a.handleDatabaseDownload(w, r)
	case "db/restore/inspect":
		a.handleDatabaseInspect(w, r, false)
	case "db/merge/inspect":
		a.handleDatabaseInspect(w, r, true)
	case "db/restore":
		a.handleDatabaseRestore(w, r, false)
	case "db/merge":
		a.handleDatabaseRestore(w, r, true)
	case "litellm/config":
		a.handleLiteLLMConfig(w, r)
	default:
		writeError(w, http.StatusNotFound, "Not found")
	}
}

func (a *App) handleCodeFormat(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.requireUser(w, r); !ok {
		return
	}
	var form struct {
		Code string `json:"code"`
	}
	if !decodeJSON(w, r, &form) {
		return
	}
	// Python formatters are intentionally not bundled in the slim image. This
	// deterministic normalizer keeps the endpoint useful without pretending to
	// be Black and preserves source when it cannot safely change it.
	code := strings.TrimRight(strings.ReplaceAll(form.Code, "\r\n", "\n"), "\n")
	if code != "" {
		code += "\n"
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": code, "formatter_unavailable": true, "detail": "Python formatter is disabled in the Go slim profile"})
}

func (a *App) handleCodeExecute(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.requireUser(w, r); !ok {
		return
	}
	var form struct {
		Code string `json:"code"`
	}
	if !decodeJSON(w, r, &form) {
		return
	}
	endpoint := strings.TrimRight(strings.TrimSpace(os.Getenv("CODE_EXECUTION_JUPYTER_URL")), "/")
	if endpoint == "" {
		writeError(w, http.StatusNotImplemented, "configure CODE_EXECUTION_JUPYTER_URL for remote code execution")
		return
	}
	payload, _ := json.Marshal(map[string]string{"code": form.Code})
	request, _ := http.NewRequestWithContext(r.Context(), http.MethodPost, endpoint, bytes.NewReader(payload))
	request.Header.Set("Content-Type", "application/json")
	if token := os.Getenv("CODE_EXECUTION_JUPYTER_AUTH_TOKEN"); token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	client := &http.Client{Timeout: 60 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		writeError(w, http.StatusBadGateway, "remote code execution failed: "+err.Error())
		return
	}
	defer response.Body.Close()
	w.Header().Set("Content-Type", response.Header.Get("Content-Type"))
	w.WriteHeader(response.StatusCode)
	_, _ = io.Copy(w, io.LimitReader(response.Body, 2<<20))
}

func (a *App) handleMarkdown(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.requireUser(w, r); !ok {
		return
	}
	var form struct {
		Markdown string `json:"md"`
	}
	if !decodeJSON(w, r, &form) {
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"html": renderLightMarkdown(form.Markdown)})
}

var markdownLinkPattern = regexp.MustCompile(`\[([^\]]+)\]\(([^\s)]+)\)`)

func renderLightMarkdown(source string) string {
	var out strings.Builder
	inFence := false
	var code strings.Builder
	flushParagraph := func(lines []string) {
		if len(lines) == 0 {
			return
		}
		text := strings.TrimSpace(strings.Join(lines, " "))
		if text != "" {
			out.WriteString("<p>" + markdownInline(text) + "</p>\n")
		}
	}
	paragraph := make([]string, 0, 2)
	scanner := bufio.NewScanner(strings.NewReader(strings.ReplaceAll(source, "\r\n", "\n")))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			if inFence {
				out.WriteString("<pre><code>" + html.EscapeString(code.String()) + "</code></pre>\n")
				code.Reset()
				inFence = false
			} else {
				flushParagraph(paragraph)
				paragraph = paragraph[:0]
				inFence = true
			}
			continue
		}
		if inFence {
			code.WriteString(line)
			code.WriteByte('\n')
			continue
		}
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			flushParagraph(paragraph)
			paragraph = paragraph[:0]
			continue
		}
		if strings.HasPrefix(trimmed, "#") {
			level := 0
			for level < len(trimmed) && trimmed[level] == '#' {
				level++
			}
			if level <= 6 && len(trimmed) > level && trimmed[level] == ' ' {
				flushParagraph(paragraph)
				paragraph = paragraph[:0]
				out.WriteString(fmt.Sprintf("<h%d>%s</h%d>\n", level, markdownInline(strings.TrimSpace(trimmed[level:])), level))
				continue
			}
		}
		if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") {
			flushParagraph(paragraph)
			paragraph = paragraph[:0]
			out.WriteString("<ul><li>" + markdownInline(strings.TrimSpace(trimmed[2:])) + "</li></ul>\n")
			continue
		}
		paragraph = append(paragraph, trimmed)
	}
	if inFence {
		out.WriteString("<pre><code>" + html.EscapeString(code.String()) + "</code></pre>\n")
	}
	flushParagraph(paragraph)
	return out.String()
}

func markdownInline(value string) string {
	value = html.EscapeString(value)
	value = markdownLinkPattern.ReplaceAllString(value, `<a href="$2" rel="noreferrer noopener">$1</a>`)
	value = regexp.MustCompile(`\*\*([^*]+)\*\*`).ReplaceAllString(value, `<strong>$1</strong>`)
	value = regexp.MustCompile(`\*([^*]+)\*`).ReplaceAllString(value, `<em>$1</em>`)
	return value
}

func (a *App) handleChatPDF(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.requireUser(w, r); !ok {
		return
	}
	var form struct {
		Title    string           `json:"title"`
		Messages []map[string]any `json:"messages"`
	}
	if !decodeJSON(w, r, &form) {
		return
	}
	data := buildMinimalPDF(form.Title, form.Messages)
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", `attachment;filename="chat.pdf"`)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func buildMinimalPDF(title string, messages []map[string]any) []byte {
	lines := []string{title}
	for _, message := range messages {
		role, _ := message["role"].(string)
		content := fmt.Sprint(message["content"])
		if content == "<nil>" {
			continue
		}
		for _, line := range strings.Split(content, "\n") {
			lines = append(lines, role+": "+line)
		}
	}
	if len(lines) > 52 {
		lines = lines[:52]
	}
	var stream strings.Builder
	stream.WriteString("BT /F1 10 Tf 50 780 Td ")
	for i, line := range lines {
		if i > 0 {
			stream.WriteString("0 -14 Td ")
		}
		stream.WriteString("(" + pdfEscape(asciiText(line)) + ") Tj ")
	}
	stream.WriteString("ET")
	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Resources << /Font << /F1 5 0 R >> >> /Contents 4 0 R >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(stream.String()), stream.String()),
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
	}
	var output bytes.Buffer
	output.WriteString("%PDF-1.4\n")
	offsets := make([]int, len(objects)+1)
	for index, object := range objects {
		offsets[index+1] = output.Len()
		fmt.Fprintf(&output, "%d 0 obj\n%s\nendobj\n", index+1, object)
	}
	xref := output.Len()
	fmt.Fprintf(&output, "xref\n0 %d\n0000000000 65535 f \n", len(objects)+1)
	for index := 1; index <= len(objects); index++ {
		fmt.Fprintf(&output, "%010d 00000 n \n", offsets[index])
	}
	fmt.Fprintf(&output, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objects)+1, xref)
	return output.Bytes()
}

func asciiText(value string) string {
	var out strings.Builder
	for _, r := range value {
		if r >= 32 && r <= 126 {
			out.WriteRune(r)
		} else {
			out.WriteRune('?')
		}
	}
	return out.String()
}

func pdfEscape(value string) string {
	return strings.NewReplacer(`\`, `\\`, `(`, `\(`, `)`, `\)`).Replace(value)
}

func (a *App) handleDatabaseDownload(w http.ResponseWriter, r *http.Request) {
	if !adminExportEnabled() {
		writeError(w, http.StatusUnauthorized, "database export is disabled")
		return
	}
	if ok, _ := a.requireAdmin(w, r); !ok {
		return
	}
	kind := r.URL.Query().Get("kind")
	if kind == "" {
		kind = backupKindSQLite
	}
	path, cleanup, err := a.createBackup(kind)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	defer cleanup()
	filename := "webui.db"
	if kind == backupKindFull {
		filename = "halo-webui-full-backup.hwbk"
	}
	serveTempFile(w, path, filename, "application/octet-stream")
}

func (a *App) createBackup(kind string) (string, func(), error) {
	if kind != backupKindSQLite && kind != backupKindFull {
		return "", func() {}, errors.New("unsupported backup kind")
	}
	if _, err := os.Stat(filepath.Join(a.config.DataDir, "webui.db")); err != nil {
		return "", func() {}, err
	}
	temp, err := os.CreateTemp(a.config.DataDir, "halowebui-backup-")
	if err != nil {
		return "", func() {}, err
	}
	path := temp.Name()
	temp.Close()
	cleanup := func() { _ = os.Remove(path) }
	if kind == backupKindSQLite {
		_ = os.Remove(path)
		db, err := sql.Open("sqlite", filepath.Join(a.config.DataDir, "webui.db"))
		if err != nil {
			cleanup()
			return "", func() {}, err
		}
		_, err = db.Exec(`VACUUM INTO ?`, path)
		_ = db.Close()
		if err != nil {
			cleanup()
			return "", func() {}, err
		}
		return path, cleanup, nil
	}
	file, err := os.OpenFile(path, os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		cleanup()
		return "", func() {}, err
	}
	archive := zip.NewWriter(file)
	if err := addFileToZip(archive, filepath.Join(a.config.DataDir, "webui.db"), "webui.db"); err != nil {
		archive.Close()
		file.Close()
		cleanup()
		return "", func() {}, err
	}
	uploads := filepath.Join(a.config.DataDir, "uploads")
	_ = filepath.WalkDir(uploads, func(entry string, info os.DirEntry, walkErr error) error {
		if walkErr != nil || info.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(a.config.DataDir, entry)
		return addFileToZip(archive, entry, filepath.ToSlash(rel))
	})
	if err := archive.Close(); err != nil {
		file.Close()
		cleanup()
		return "", func() {}, err
	}
	if err := file.Close(); err != nil {
		cleanup()
		return "", func() {}, err
	}
	return path, cleanup, nil
}

func addFileToZip(archive *zip.Writer, source, name string) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	entry, err := archive.Create(name)
	if err != nil {
		return err
	}
	_, err = io.Copy(entry, input)
	return err
}

func serveTempFile(w http.ResponseWriter, path, filename, contentType string) {
	file, err := os.Open(path)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	w.Header().Set("Content-Length", strconv.FormatInt(info.Size(), 10))
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, file)
}

func (a *App) handleDatabaseInspect(w http.ResponseWriter, r *http.Request, merge bool) {
	if !adminExportEnabled() {
		writeError(w, http.StatusUnauthorized, "database export is disabled")
		return
	}
	user, ok := a.requireAdminUser(w, r)
	if !ok {
		return
	}
	kind := r.FormValue("expected_kind")
	if kind == "" {
		kind = backupKindSQLite
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "backup file is required")
		return
	}
	defer file.Close()
	path, cleanup, err := stageBackup(file, header.Filename, a.config.DataDir, kind)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	defer cleanup()
	summary, err := inspectSQLite(path)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if merge {
		summary["merge"] = map[string]any{"mode": "insert_only", "table_count": summary["table_count"], "source_rows": 0, "insert_rows": 0, "skip_existing_rows": 0, "skip_conflict_rows": 0, "tables": []any{}, "uploads": map[string]any{"copy_count": 0, "reuse_count": 0, "rename_count": 0, "missing_count": 0, "bytes_to_copy": 0}}
	}
	token := auth.RandomIDForInternalUse()
	staged := filepath.Join(a.config.DataDir, "restore-"+token+".db")
	if err := copyFile(path, staged); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	stagedUploads := ""
	if kind == backupKindFull {
		stagedUploads = filepath.Join(a.config.DataDir, "restore-uploads-"+token)
		if err := copyDirectory(path+".uploads", stagedUploads); err != nil {
			_ = os.Remove(staged)
			writeError(w, http.StatusInternalServerError, "failed to stage backup uploads: "+err.Error())
			return
		}
	}
	a.restoreMu.Lock()
	a.restoreSessions[token] = restoreSession{Path: staged, UploadsPath: stagedUploads, UserID: user.ID, Kind: kind, Filename: header.Filename, Expires: time.Now().Add(15 * time.Minute)}
	a.restoreMu.Unlock()
	response := map[string]any{"token": token, "compatible": true, "kind": kind, "filename": header.Filename, "size": header.Size, "warnings": []string{}, "summary": summary, "confirmation": restoreConfirm}
	if merge {
		response["confirmation"] = mergeConfirm
		response["merge"] = summary["merge"]
	}
	writeJSON(w, http.StatusOK, response)
}

func (a *App) requireAdminUser(w http.ResponseWriter, r *http.Request) (store.User, bool) {
	user, ok := a.requireUser(w, r)
	if !ok {
		return store.User{}, false
	}
	if user.Role != "admin" {
		writeError(w, http.StatusForbidden, "Access prohibited")
		return store.User{}, false
	}
	return user, true
}

func stageBackup(input io.Reader, filename, dataDir, expected string) (string, func(), error) {
	temp, err := os.CreateTemp(dataDir, "incoming-backup-")
	if err != nil {
		return "", func() {}, err
	}
	raw := temp.Name()
	if _, err := io.Copy(temp, io.LimitReader(input, 2<<30)); err != nil {
		temp.Close()
		os.Remove(raw)
		return "", func() {}, err
	}
	temp.Close()
	if expected == backupKindSQLite {
		return raw, func() { _ = os.Remove(raw) }, nil
	}
	if expected != backupKindFull {
		os.Remove(raw)
		return "", func() {}, errors.New("unsupported backup kind")
	}
	archive, err := zip.OpenReader(raw)
	if err != nil {
		os.Remove(raw)
		return "", func() {}, errors.New("invalid full backup archive")
	}
	defer archive.Close()
	var entry *zip.File
	for _, candidate := range archive.File {
		if filepath.ToSlash(candidate.Name) == "webui.db" {
			entry = candidate
			break
		}
	}
	if entry == nil {
		os.Remove(raw)
		return "", func() {}, errors.New("full backup does not contain webui.db")
	}
	dbPath := raw + ".db"
	uploadsPath := dbPath + ".uploads"
	if err := os.MkdirAll(uploadsPath, 0o700); err != nil {
		os.Remove(raw)
		return "", func() {}, err
	}
	cleanup := func() {
		_ = os.Remove(dbPath)
		_ = os.RemoveAll(uploadsPath)
	}
	source, err := entry.Open()
	if err != nil {
		os.Remove(raw)
		cleanup()
		return "", func() {}, err
	}
	target, err := os.OpenFile(dbPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err == nil {
		_, err = io.Copy(target, source)
		_ = target.Close()
	}
	_ = source.Close()
	if err != nil {
		os.Remove(raw)
		cleanup()
		return "", func() {}, err
	}
	var extractedBytes int64
	for _, candidate := range archive.File {
		name := filepath.ToSlash(candidate.Name)
		if candidate.FileInfo().IsDir() || !strings.HasPrefix(name, "uploads/") {
			continue
		}
		relative := strings.TrimPrefix(name, "uploads/")
		if relative == "" || strings.Contains(relative, "\x00") {
			continue
		}
		target, ok := safeDataPath(uploadsPath, relative)
		if !ok {
			os.Remove(raw)
			cleanup()
			return "", func() {}, errors.New("full backup contains an unsafe upload path")
		}
		if candidate.UncompressedSize64 > uint64(fullBackupUploadLimit-extractedBytes) {
			os.Remove(raw)
			cleanup()
			return "", func() {}, errors.New("full backup uploads exceed the 512 MiB restore limit")
		}
		if err := extractZipFile(candidate, target, fullBackupUploadLimit-extractedBytes); err != nil {
			os.Remove(raw)
			cleanup()
			return "", func() {}, err
		}
		extractedBytes += int64(candidate.UncompressedSize64)
	}
	os.Remove(raw)
	return dbPath, cleanup, nil
}

func inspectSQLite(path string) (map[string]any, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	rows, err := db.Query(`SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	tables := []string{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		tables = append(tables, name)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	preview := append([]string(nil), tables...)
	if len(preview) > 20 {
		preview = preview[:20]
	}
	has := func(name string) bool {
		for _, t := range tables {
			if t == name {
				return true
			}
		}
		return false
	}
	return map[string]any{"table_count": len(tables), "tables_preview": preview, "has_chat_table": has("chat"), "has_config_table": has("config"), "has_user_table": has("user")}, nil
}

func (a *App) handleDatabaseRestore(w http.ResponseWriter, r *http.Request, merge bool) {
	if !adminExportEnabled() {
		writeError(w, http.StatusUnauthorized, "database export is disabled")
		return
	}
	user, ok := a.requireAdminUser(w, r)
	if !ok {
		return
	}
	var form struct {
		Token        string `json:"token"`
		Confirmation string `json:"confirmation"`
	}
	if !decodeJSON(w, r, &form) {
		return
	}
	want := restoreConfirm
	if merge {
		want = mergeConfirm
	}
	if form.Confirmation != want {
		writeError(w, http.StatusBadRequest, "The confirmation phrase is incorrect.")
		return
	}
	a.restoreMu.Lock()
	session, exists := a.restoreSessions[form.Token]
	if exists {
		delete(a.restoreSessions, form.Token)
	}
	a.restoreMu.Unlock()
	if !exists || session.UserID != user.ID || time.Now().After(session.Expires) {
		if exists {
			removeRestoreSessionFiles(session)
		}
		writeError(w, http.StatusBadRequest, "The restore session has expired. Please inspect the backup again.")
		return
	}
	defer removeRestoreSessionFiles(session)
	a.restoreMu.Lock()
	defer a.restoreMu.Unlock()
	current := filepath.Join(a.config.DataDir, "webui.db")
	rollback := current + ".rollback-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	if err := copyFile(current, rollback); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create rollback backup: "+err.Error())
		return
	}
	if merge {
		if err := mergeSQLite(session.Path, current); err != nil {
			_ = copyFile(rollback, current)
			_ = os.Remove(rollback)
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if session.Kind == backupKindFull {
			if err := a.mergeUploads(session.UploadsPath); err != nil {
				writeError(w, http.StatusBadRequest, "failed to merge uploads: "+err.Error())
				return
			}
		}
	} else {
		if err := a.replaceDatabase(session.Path, current); err != nil {
			_ = copyFile(rollback, current)
			_ = os.Remove(rollback)
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if session.Kind == backupKindFull {
			if err := a.replaceUploads(session.UploadsPath); err != nil {
				_ = copyFile(rollback, current)
				_ = os.Remove(rollback)
				writeError(w, http.StatusBadRequest, "failed to restore uploads: "+err.Error())
				return
			}
		}
	}
	_ = os.Remove(rollback)
	writeJSON(w, http.StatusOK, map[string]any{"restored": !merge, "merged": merge, "requires_reload": true})
}

func (a *App) replaceDatabase(source, target string) error {
	if a.store != nil {
		if err := a.store.Close(); err != nil {
			return err
		}
	}
	if err := copyFile(source, target); err != nil {
		return err
	}
	next, err := store.Open(rContext(), a.config.DataDir)
	if err != nil {
		return err
	}
	a.store = next
	return nil
}

func rContext() context.Context { return context.Background() }

func copyFile(source, target string) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	temp := target + ".new"
	output, err := os.OpenFile(temp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(output, input)
	closeErr := output.Close()
	if copyErr != nil {
		os.Remove(temp)
		return copyErr
	}
	if closeErr != nil {
		os.Remove(temp)
		return closeErr
	}
	return os.Rename(temp, target)
}

func removeRestoreSessionFiles(session restoreSession) {
	if session.Path != "" {
		_ = os.Remove(session.Path)
	}
	if session.UploadsPath != "" {
		_ = os.RemoveAll(session.UploadsPath)
	}
}

func extractZipFile(entry *zip.File, target string, limit int64) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		return err
	}
	source, err := entry.Open()
	if err != nil {
		return err
	}
	defer source.Close()
	temp := target + ".part"
	output, err := os.OpenFile(temp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	written, copyErr := io.Copy(output, io.LimitReader(source, limit+1))
	closeErr := output.Close()
	if copyErr != nil {
		_ = os.Remove(temp)
		return copyErr
	}
	if closeErr != nil {
		_ = os.Remove(temp)
		return closeErr
	}
	if written > limit {
		_ = os.Remove(temp)
		return errors.New("full backup upload exceeds the restore limit")
	}
	return os.Rename(temp, target)
}

func copyDirectory(source, target string) error {
	info, err := os.Stat(source)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return errors.New("backup uploads path is not a directory")
	}
	if err := os.MkdirAll(target, 0o700); err != nil {
		return err
	}
	return filepath.WalkDir(source, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		if relative == "." {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return errors.New("backup uploads may not contain symbolic links")
		}
		destination, ok := safeDataPath(target, relative)
		if !ok {
			return errors.New("backup uploads contain an unsafe path")
		}
		if entry.IsDir() {
			return os.MkdirAll(destination, 0o700)
		}
		return copyFile(path, destination)
	})
}

func (a *App) replaceUploads(source string) error {
	target := filepath.Join(a.config.DataDir, "uploads")
	if source == "" {
		return errors.New("full backup uploads are unavailable")
	}
	rollback := target + ".rollback-" + auth.RandomIDForInternalUse()
	hadTarget := false
	if _, err := os.Stat(target); err == nil {
		hadTarget = true
		if err := os.Rename(target, rollback); err != nil {
			return err
		}
	}
	if err := os.Rename(source, target); err != nil {
		if hadTarget {
			_ = os.Rename(rollback, target)
		}
		return err
	}
	if hadTarget {
		_ = os.RemoveAll(rollback)
	}
	return nil
}

func (a *App) mergeUploads(source string) error {
	if source == "" {
		return nil
	}
	target := filepath.Join(a.config.DataDir, "uploads")
	if err := os.MkdirAll(target, 0o700); err != nil {
		return err
	}
	return filepath.WalkDir(source, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(source, path)
		if err != nil || relative == "." {
			return err
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return errors.New("backup uploads may not contain symbolic links")
		}
		destination, ok := safeDataPath(target, relative)
		if !ok {
			return errors.New("backup uploads contain an unsafe path")
		}
		if entry.IsDir() {
			return os.MkdirAll(destination, 0o700)
		}
		if _, statErr := os.Stat(destination); statErr == nil {
			return nil
		}
		return copyFile(path, destination)
	})
}

func mergeSQLite(source, target string) error {
	db, err := sql.Open("sqlite", target)
	if err != nil {
		return err
	}
	defer db.Close()
	if _, err := db.Exec(`ATTACH DATABASE ? AS backup`, source); err != nil {
		return err
	}
	defer db.Exec(`DETACH DATABASE backup`)
	rows, err := db.Query(`SELECT name FROM backup.sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'`)
	if err != nil {
		return err
	}
	defer rows.Close()
	tables := []string{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return err
		}
		tables = append(tables, name)
	}
	for _, table := range tables {
		targetCols, err := tableColumns(db, "main", table)
		if err != nil {
			continue
		}
		sourceCols, err := tableColumns(db, "backup", table)
		if err != nil {
			continue
		}
		allowed := map[string]bool{}
		for _, c := range sourceCols {
			allowed[c] = true
		}
		cols := []string{}
		for _, c := range targetCols {
			if allowed[c] {
				cols = append(cols, c)
			}
		}
		if len(cols) == 0 {
			continue
		}
		quoted := make([]string, len(cols))
		for i, c := range cols {
			quoted[i] = quoteIdentifier(c)
		}
		query := `INSERT OR IGNORE INTO main.` + quoteIdentifier(table) + ` (` + strings.Join(quoted, ",") + ") SELECT " + strings.Join(quoted, ",") + ` FROM backup.` + quoteIdentifier(table)
		if _, err := db.Exec(query); err != nil {
			return fmt.Errorf("merge table %s: %w", table, err)
		}
	}
	return nil
}

func tableColumns(db *sql.DB, schema, table string) ([]string, error) {
	rows, err := db.Query(`PRAGMA ` + schema + `.table_info(` + quoteIdentifier(table) + `)`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var cols []string
	for rows.Next() {
		var cid int
		var name, columnType string
		var notNull int
		var defaultValue any
		var primaryKey int
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			return nil, err
		}
		cols = append(cols, name)
	}
	return cols, rows.Err()
}
func quoteIdentifier(value string) string { return `"` + strings.ReplaceAll(value, `"`, `""`) + `"` }

func (a *App) handleLiteLLMConfig(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.requireAdminUser(w, r); !ok {
		return
	}
	path := filepath.Join(a.config.DataDir, "litellm", "config.yaml")
	file, err := os.Open(path)
	if err != nil {
		writeError(w, http.StatusNotFound, "LiteLLM config.yaml not found")
		return
	}
	defer file.Close()
	w.Header().Set("Content-Type", "application/yaml")
	w.Header().Set("Content-Disposition", `attachment; filename="config.yaml"`)
	_, _ = io.Copy(w, file)
}
