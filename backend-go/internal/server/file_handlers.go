package server

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/iwvw-per/HaloWebUI/backend-go/internal/auth"
	"github.com/iwvw-per/HaloWebUI/backend-go/internal/store"
)

func (a *App) handleFiles(response http.ResponseWriter, request *http.Request) {
	user, ok := a.requireUser(response, request)
	if !ok {
		return
	}
	if request.Method == http.MethodGet {
		files, err := a.store.ListFiles(request.Context(), user.ID)
		if err != nil {
			writeError(response, http.StatusInternalServerError, "failed to list files")
			return
		}
		writeJSON(response, http.StatusOK, files)
		return
	}
	request.Body = http.MaxBytesReader(response, request.Body, a.config.FileMaxSizeBytes+(1<<20))
	if err := request.ParseMultipartForm(1 << 20); err != nil {
		writeError(response, http.StatusBadRequest, "file is too large or invalid")
		return
	}
	source, header, err := request.FormFile("file")
	if err != nil {
		writeError(response, http.StatusBadRequest, "file is required")
		return
	}
	defer source.Close()
	id := auth.RandomIDForInternalUse()
	filename := filepath.Base(strings.ReplaceAll(header.Filename, "\\", "/"))
	if filename == "." || filename == "" {
		filename = "upload"
	}
	uploadDir := filepath.Join(a.config.DataDir, "uploads")
	if err := os.MkdirAll(uploadDir, 0o700); err != nil {
		writeError(response, http.StatusInternalServerError, "failed to create upload directory")
		return
	}
	relativePath := filepath.Join("uploads", id+"_"+filename)
	absolutePath := filepath.Join(a.config.DataDir, relativePath)
	target, err := os.OpenFile(absolutePath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		writeError(response, http.StatusInternalServerError, "failed to store file")
		return
	}
	hash := sha256.New()
	written, copyErr := io.Copy(io.MultiWriter(target, hash), io.LimitReader(source, a.config.FileMaxSizeBytes+1))
	closeErr := target.Close()
	if copyErr != nil || closeErr != nil || written > a.config.FileMaxSizeBytes {
		_ = os.Remove(absolutePath)
		writeError(response, http.StatusBadRequest, "file exceeds configured size limit")
		return
	}
	hashValue := hex.EncodeToString(hash.Sum(nil))
	contentType := header.Header.Get("Content-Type")
	metadata, _ := json.Marshal(map[string]any{"content_type": contentType, "size": written})
	file, err := a.store.CreateFile(request.Context(), store.File{
		ID: id, UserID: user.ID, Hash: &hashValue, Filename: filename,
		Path: &relativePath, Meta: metadata,
	})
	if err != nil {
		_ = os.Remove(absolutePath)
		writeError(response, http.StatusInternalServerError, "failed to save file metadata")
		return
	}
	writeJSON(response, http.StatusOK, file)
}

func (a *App) handleFileByID(response http.ResponseWriter, request *http.Request) {
	user, ok := a.requireUser(response, request)
	if !ok {
		return
	}
	file, err := a.store.FileByID(request.Context(), request.PathValue("id"))
	if errors.Is(err, store.ErrFileNotFound) {
		writeError(response, http.StatusNotFound, "File not found")
		return
	}
	if err != nil {
		writeError(response, http.StatusInternalServerError, "failed to load file")
		return
	}
	if file.UserID != user.ID && user.Role != "admin" {
		writeError(response, http.StatusForbidden, "Access prohibited")
		return
	}
	if request.Method == http.MethodGet {
		writeJSON(response, http.StatusOK, file)
		return
	}
	if file.Path != nil {
		if path, ok := safeDataPath(a.config.DataDir, *file.Path); ok {
			_ = os.Remove(path)
		}
	}
	if err := a.store.DeleteFile(request.Context(), file.ID); err != nil {
		writeError(response, http.StatusInternalServerError, "failed to delete file")
		return
	}
	writeJSON(response, http.StatusOK, map[string]bool{"status": true})
}

func (a *App) handleFileContent(response http.ResponseWriter, request *http.Request) {
	user, ok := a.requireUser(response, request)
	if !ok {
		return
	}
	file, err := a.store.FileByID(request.Context(), request.PathValue("id"))
	if err != nil || file.Path == nil {
		writeError(response, http.StatusNotFound, "File not found")
		return
	}
	if file.UserID != user.ID && user.Role != "admin" {
		writeError(response, http.StatusForbidden, "Access prohibited")
		return
	}
	path, safe := safeDataPath(a.config.DataDir, *file.Path)
	if !safe {
		writeError(response, http.StatusForbidden, "Invalid file path")
		return
	}
	if contentType := mime.TypeByExtension(filepath.Ext(file.Filename)); contentType != "" {
		response.Header().Set("Content-Type", contentType)
	}
	response.Header().Set("Content-Disposition", `inline; filename="`+strings.ReplaceAll(file.Filename, `"`, "")+`"`)
	http.ServeFile(response, request, path)
}

func safeDataPath(dataDir, relative string) (string, bool) {
	clean := filepath.Clean(relative)
	if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", false
	}
	root, err := filepath.Abs(dataDir)
	if err != nil {
		return "", false
	}
	path, err := filepath.Abs(filepath.Join(root, clean))
	if err != nil || (path != root && !strings.HasPrefix(path, root+string(filepath.Separator))) {
		return "", false
	}
	return path, true
}

func (a *App) handleFileData(response http.ResponseWriter, request *http.Request) {
	user, ok := a.requireUser(response, request)
	if !ok {
		return
	}
	file, err := a.store.FileByID(request.Context(), request.PathValue("id"))
	if err != nil || (file.UserID != user.ID && user.Role != "admin") {
		writeError(response, http.StatusNotFound, "File not found")
		return
	}
	var body map[string]any
	if !decodeJSON(response, request, &body) {
		return
	}
	data, _ := json.Marshal(body)
	updated, err := a.store.UpdateFileData(request.Context(), file.ID, data)
	if err != nil {
		writeError(response, http.StatusInternalServerError, "failed to update file")
		return
	}
	writeJSON(response, http.StatusOK, updated)
}
