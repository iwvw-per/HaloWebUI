package server

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/iwvw-per/HaloWebUI/backend/internal/auth"
	"github.com/iwvw-per/HaloWebUI/backend/internal/store"
)

var skillSlugPattern = regexp.MustCompile(`[^a-z0-9_-]+`)

func skillID(body map[string]any) string {
	for _, field := range []string{"id", "identifier", "name"} {
		value := strings.ToLower(strings.TrimSpace(stringField(body, field)))
		value = strings.Trim(skillSlugPattern.ReplaceAllString(value, "-"), "-")
		if value != "" {
			return value
		}
	}
	return auth.RandomIDForInternalUse()
}

func (a *App) skillResources(r *http.Request, user store.User) ([]store.Resource, error) {
	resources, err := a.store.ListResources(r.Context(), "skill", false)
	if err != nil {
		return nil, err
	}
	visible := resources[:0]
	for _, resource := range resources {
		if resourceReadableBy(resource, user, "skill") {
			visible = append(visible, resource)
		}
	}
	return visible, nil
}

func (a *App) handleSkills(w http.ResponseWriter, r *http.Request) {
	user, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	resources, err := a.skillResources(r, user)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load skills")
		return
	}
	query := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("query")))
	items := make([]json.RawMessage, 0, len(resources))
	for _, resource := range resources {
		response := resourceResponse(resource)
		if query != "" && !strings.Contains(strings.ToLower(string(response)), query) {
			continue
		}
		items = append(items, response)
	}
	if strings.HasSuffix(r.URL.Path, "/list") {
		total := len(items)
		page, limit := queryInt(r, "page", 1, 1, 100000), queryInt(r, "limit", 30, 1, 100)
		start := (page - 1) * limit
		if start >= len(items) {
			items = []json.RawMessage{}
		} else {
			end := start + limit
			if end > len(items) {
				end = len(items)
			}
			items = items[start:end]
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items, "total": total})
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (a *App) createOrImportSkill(w http.ResponseWriter, r *http.Request, body map[string]any, source string) {
	user, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	id := skillID(body)
	if body["name"] == nil {
		body["name"] = id
	}
	if body["description"] == nil {
		body["description"] = ""
	}
	body["id"], body["identifier"], body["user_id"], body["source"], body["is_active"] = id, id, user.ID, source, true
	status := "created"
	resource, err := a.store.ResourceByID(r.Context(), "skill", id)
	if err == nil {
		if resource.UserID != user.ID && user.Role != "admin" {
			writeError(w, http.StatusForbidden, "Access prohibited")
			return
		}
		status = "updated"
	} else if errors.Is(err, store.ErrResourceNotFound) {
		resource = store.Resource{Kind: "skill", ID: id, UserID: user.ID, Key: id, Active: true}
	} else {
		writeError(w, http.StatusInternalServerError, "failed to load skill")
		return
	}
	resource.Body, _ = json.Marshal(body)
	resource.UserID, resource.Key = user.ID, id
	updated, err := a.store.PutResource(r.Context(), resource)
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to save skill")
		return
	}
	if strings.Contains(r.URL.Path, "/import/") {
		var skill any
		_ = json.Unmarshal(resourceResponse(updated), &skill)
		writeJSON(w, http.StatusOK, map[string]any{"skill": skill, "status": status})
		return
	}
	writeRawJSON(w, http.StatusOK, resourceResponse(updated))
}

func (a *App) handleSkillCreate(w http.ResponseWriter, r *http.Request) {
	var body map[string]any
	if !decodeJSON(w, r, &body) {
		return
	}
	source := "manual"
	if value, _ := body["source"].(string); value != "" {
		source = value
	}
	a.createOrImportSkill(w, r, body, source)
}

func (a *App) handleSkillRemoteImport(source string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := a.requireUser(w, r); !ok {
			return
		}
		var form struct {
			URL string `json:"url"`
		}
		if !decodeJSON(w, r, &form) {
			return
		}
		parsed, err := url.Parse(strings.TrimSpace(form.URL))
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || blockedRemoteHost(parsed.Hostname()) {
			writeError(w, http.StatusBadRequest, "unsupported or unsafe skill URL")
			return
		}
		request, _ := http.NewRequestWithContext(r.Context(), http.MethodGet, parsed.String(), nil)
		response, err := (&http.Client{Timeout: 20 * time.Second}).Do(request)
		if err != nil {
			writeError(w, http.StatusBadGateway, "failed to download skill")
			return
		}
		defer response.Body.Close()
		if response.StatusCode >= 400 {
			writeError(w, http.StatusBadGateway, "skill source returned an error")
			return
		}
		content, _ := io.ReadAll(io.LimitReader(response.Body, 2*1024*1024))
		name := strings.TrimSuffix(filepath.Base(parsed.Path), filepath.Ext(parsed.Path))
		if name == "" || name == "." || strings.EqualFold(name, "SKILL") {
			name = parsed.Hostname()
		}
		a.createOrImportSkill(w, r, map[string]any{"name": name, "content": string(content), "source_url": parsed.String()}, source)
	}
}

func (a *App) handleSkillZipImport(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.requireUser(w, r); !ok {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 8*1024*1024)
	if err := r.ParseMultipartForm(8 * 1024 * 1024); err != nil {
		writeError(w, http.StatusBadRequest, "invalid skill archive")
		return
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "file is required")
		return
	}
	defer file.Close()
	archiveData, _ := io.ReadAll(io.LimitReader(file, 8*1024*1024))
	archive, err := zip.NewReader(bytes.NewReader(archiveData), int64(len(archiveData)))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid ZIP archive")
		return
	}
	var content []byte
	name := "imported-skill"
	for _, entry := range archive.File {
		if strings.EqualFold(filepath.Base(entry.Name), "SKILL.md") {
			reader, openErr := entry.Open()
			if openErr != nil {
				continue
			}
			content, _ = io.ReadAll(io.LimitReader(reader, 2*1024*1024))
			_ = reader.Close()
			if directory := filepath.Base(filepath.Dir(entry.Name)); directory != "." && directory != "" {
				name = directory
			}
			break
		}
	}
	if len(content) == 0 {
		writeError(w, http.StatusBadRequest, "ZIP archive does not contain SKILL.md")
		return
	}
	a.createOrImportSkill(w, r, map[string]any{"name": name, "content": string(content)}, "zip")
}

func (a *App) handleSkillByID(w http.ResponseWriter, r *http.Request) {
	user, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	id := r.PathValue("skill_id")
	if id == "" {
		id = r.PathValue("id")
	}
	resource, err := a.store.ResourceByID(r.Context(), "skill", id)
	if errors.Is(err, store.ErrResourceNotFound) {
		writeError(w, http.StatusNotFound, "Skill not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load skill")
		return
	}
	if !resourceReadableBy(resource, user, "skill") {
		writeError(w, http.StatusForbidden, "Access prohibited")
		return
	}
	if r.Method == http.MethodGet {
		writeRawJSON(w, http.StatusOK, resourceResponse(resource))
		return
	}
	if resource.UserID != user.ID && user.Role != "admin" {
		writeError(w, http.StatusForbidden, "Access prohibited")
		return
	}
	if r.Method == http.MethodDelete {
		if err := a.store.DeleteResource(r.Context(), "skill", resource.ID); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to delete skill")
			return
		}
		writeJSON(w, http.StatusOK, true)
		return
	}
	if strings.HasSuffix(r.URL.Path, "/toggle") {
		updated, err := a.store.ToggleResource(r.Context(), "skill", resource.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to toggle skill")
			return
		}
		writeRawJSON(w, http.StatusOK, resourceResponse(updated))
		return
	}
	var patch map[string]any
	if !decodeJSON(w, r, &patch) {
		return
	}
	var body map[string]any
	_ = json.Unmarshal(resource.Body, &body)
	mergeJSONMap(body, patch)
	resource.Body, _ = json.Marshal(body)
	updated, err := a.store.PutResource(r.Context(), resource)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update skill")
		return
	}
	writeRawJSON(w, http.StatusOK, resourceResponse(updated))
}

func (a *App) handleSkillCatalog(w http.ResponseWriter, r *http.Request) {
	user, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	resources, err := a.skillResources(r, user)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load skill catalog")
		return
	}
	result := make([]map[string]any, 0, len(resources))
	for _, resource := range resources {
		var body map[string]any
		_ = json.Unmarshal(resource.Body, &body)
		status := "disabled"
		if resource.Active {
			status = "enabled"
		}
		result = append(result, map[string]any{"id": resource.ID, "kind": "prompt_skill", "source": body["source"], "title": body["name"], "description": body["description"], "status": status, "editable": resource.UserID == user.ID || user.Role == "admin", "manage_href": "/workspace/skills/" + resource.ID, "meta": body["meta"]})
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *App) handleSkillRuntimeCapabilities(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.requireUser(w, r); !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"profile": "go-slim", "install_allowed": false, "python": map[string]any{"available": false, "uv": nil, "python": nil}, "node": map[string]any{"available": false, "node": nil, "npm": nil}})
}

func (a *App) handleSkillRuntimeInstall(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.requireUser(w, r); !ok {
		return
	}
	writeError(w, http.StatusConflict, "local skill runtimes are disabled in the Go slim profile")
}

func (a *App) handleLegacySkills(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.requireUser(w, r); !ok {
		return
	}
	if r.Method == http.MethodPost {
		writeJSON(w, http.StatusOK, map[string]int{"migrated": 0, "skipped": 0})
		return
	}
	writeJSON(w, http.StatusOK, []any{})
}
