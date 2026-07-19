package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/iwvw-per/HaloWebUI/backend-go/internal/auth"
	"github.com/iwvw-per/HaloWebUI/backend-go/internal/store"
)

// handleCompatibility is deliberately small and bounded. It gives the Svelte
// client a durable contract for domains that do not need a heavyweight local
// runtime on the 256 MiB target. Each record is JSON in halo_resource, so a
// later typed implementation can migrate the rows without changing clients.
func (a *App) handleCompatibility(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/api/v1/" || r.URL.Path == "/api/v1" {
		writeJSON(w, http.StatusNotFound, map[string]string{"detail": "Not found"})
		return
	}
	user, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	parts := splitPath(strings.TrimPrefix(r.URL.Path, "/api/v1/"))
	if len(parts) == 0 {
		writeJSON(w, http.StatusNotFound, map[string]string{"detail": "Not found"})
		return
	}
	domain := parts[0]
	// Provider and admin setting endpoints are persisted as per-user settings.
	if isSettingDomain(domain, parts) {
		a.handleCompatibilitySetting(w, r, user, strings.Join(parts, "/"))
		return
	}
	if domain == "auths" && len(parts) > 1 && (parts[1] == "signin" || parts[1] == "signup" || parts[1] == "signout") {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"detail": "use the auth endpoint"})
		return
	}
	// These domains are all user-owned JSON resources. Unknown domains are
	// rejected instead of silently accepting typos or exposing an open API.
	if !compatibilityDomain(domain) {
		writeJSON(w, http.StatusNotFound, map[string]string{"detail": "Not found"})
		return
	}
	a.handleCompatibilityResource(w, r, user, domain, parts[1:])
}

func splitPath(path string) []string {
	raw := strings.Split(strings.Trim(path, "/"), "/")
	parts := make([]string, 0, len(raw))
	for _, part := range raw {
		if part != "" {
			parts = append(parts, part)
		}
	}
	return parts
}

func compatibilityDomain(domain string) bool {
	switch domain {
	case "folders", "groups", "functions", "channels", "memories", "knowledge", "analytics", "terminal", "retrieval", "audio", "images", "gemini", "anthropic", "grok", "configs", "auths", "users", "chats", "files", "models", "prompts", "tools", "skills", "notes", "utils", "webhooks", "tasks":
		return true
	default:
		return false
	}
}

func isSettingDomain(domain string, parts []string) bool {
	if domain == "configs" || domain == "retrieval" || domain == "terminal" || domain == "audio" || domain == "images" || domain == "gemini" || domain == "anthropic" || domain == "grok" {
		return true
	}
	return domain == "auths" && len(parts) > 1 && strings.Contains(strings.Join(parts[1:], "/"), "config")
}

func (a *App) handleCompatibilitySetting(w http.ResponseWriter, r *http.Request, user store.User, key string) {
	resource, err := a.store.ResourceByKey(r.Context(), "setting", user.ID+":"+key)
	if r.Method == http.MethodGet {
		if errors.Is(err, store.ErrResourceNotFound) {
			writeJSON(w, http.StatusOK, map[string]any{"status": true})
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to load setting")
			return
		}
		writeRawJSON(w, http.StatusOK, resource.Body)
		return
	}
	if r.Method != http.MethodPost && r.Method != http.MethodPut && r.Method != http.MethodPatch {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"detail": "method not allowed"})
		return
	}
	var body map[string]any
	if !decodeJSON(w, r, &body) {
		return
	}
	if body == nil {
		body = map[string]any{}
	}
	body["status"] = true
	encoded, _ := json.Marshal(body)
	if err != nil || errors.Is(err, store.ErrResourceNotFound) {
		resource = store.Resource{Kind: "setting", ID: auth.RandomIDForInternalUse(), UserID: user.ID, Key: user.ID + ":" + key, Active: true}
	}
	resource.Body = encoded
	resource.UserID = user.ID
	resource.Key = user.ID + ":" + key
	resource.Kind = "setting"
	if _, err := a.store.PutResource(r.Context(), resource); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save setting")
		return
	}
	writeRawJSON(w, http.StatusOK, encoded)
}

func (a *App) handleCompatibilityResource(w http.ResponseWriter, r *http.Request, user store.User, domain string, rest []string) {
	kind := compatibilityKind(domain)
	if domain == "users" && len(rest) > 0 {
		if r.Method == http.MethodGet {
			writeJSON(w, http.StatusOK, []any{})
		} else {
			writeJSON(w, http.StatusOK, map[string]any{"status": true})
		}
		return
	}
	// Queries and action endpoints return a stable collection/ack shape. This
	// keeps optional settings pages usable even when no records exist.
	if len(rest) == 0 || (len(rest) == 1 && (rest[0] == "" || rest[0] == "list" || rest[0] == "search" || rest[0] == "all")) {
		if r.Method == http.MethodGet {
			a.writeCompatibilityList(w, r, domain)
			return
		}
		if len(rest) == 0 && r.Method == http.MethodPost {
			a.writeCompatibilityCreate(w, r, user, domain)
			return
		}
	}
	if len(rest) > 0 && (rest[0] == "create" || rest[0] == "add") && r.Method == http.MethodPost {
		a.writeCompatibilityCreate(w, r, user, domain)
		return
	}
	if len(rest) > 1 && rest[0] == "id" {
		a.handleCompatibilityByID(w, r, user, domain, rest[1:])
		return
	}
	if len(rest) > 1 && (rest[0] == "command" || rest[0] == "name") {
		resource, err := a.store.ResourceByKey(r.Context(), kind, rest[1])
		if errors.Is(err, store.ErrResourceNotFound) && r.Method == http.MethodPost && len(rest) > 2 {
			var body map[string]any
			if !decodeJSON(w, r, &body) {
				return
			}
			body["command"] = rest[1]
			body["id"], body["user_id"] = auth.RandomIDForInternalUse(), user.ID
			encoded, _ := json.Marshal(body)
			created, createErr := a.store.PutResource(r.Context(), store.Resource{Kind: kind, ID: stringField(body, "id"), UserID: user.ID, Key: rest[1], Body: encoded, Active: true})
			if createErr != nil {
				writeError(w, http.StatusBadRequest, "failed to create resource")
				return
			}
			writeRawJSON(w, http.StatusOK, resourceResponse(created))
			return
		}
		if err == nil {
			a.handleCompatibilityResourceByRecord(w, r, user, domain, resource, rest[2:])
			return
		}
	}
	if len(rest) > 0 {
		a.handleCompatibilityByID(w, r, user, domain, rest)
		return
	}
	if r.Method == http.MethodGet {
		a.writeCompatibilityList(w, r, domain)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": true})
}

func (a *App) handleCompatibilityResourceByRecord(w http.ResponseWriter, r *http.Request, user store.User, domain string, resource store.Resource, operations []string) {
	if resource.UserID != user.ID && user.Role != "admin" {
		writeError(w, http.StatusForbidden, "Access prohibited")
		return
	}
	if r.Method == http.MethodGet && len(operations) == 0 {
		writeRawJSON(w, http.StatusOK, resourceResponse(resource))
		return
	}
	if r.Method == http.MethodDelete || (len(operations) > 0 && operations[len(operations)-1] == "delete") {
		_ = a.store.DeleteResource(r.Context(), resource.Kind, resource.ID)
		writeJSON(w, http.StatusOK, true)
		return
	}
	if len(operations) > 0 && operations[len(operations)-1] == "toggle" {
		updated, err := a.store.ToggleResource(r.Context(), resource.Kind, resource.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to toggle resource")
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
	for key, value := range patch {
		body[key] = value
	}
	resource.Body, _ = json.Marshal(body)
	updated, err := a.store.PutResource(r.Context(), resource)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update resource")
		return
	}
	writeRawJSON(w, http.StatusOK, resourceResponse(updated))
}

func (a *App) writeCompatibilityList(w http.ResponseWriter, r *http.Request, domain string) {
	resources, err := a.store.ListResources(r.Context(), compatibilityKind(domain), false)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list "+domain)
		return
	}
	result := make([]json.RawMessage, 0, len(resources))
	for _, resource := range resources {
		result = append(result, resourceResponse(resource))
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *App) writeCompatibilityCreate(w http.ResponseWriter, r *http.Request, user store.User, domain string) {
	var body map[string]any
	if !decodeJSON(w, r, &body) {
		return
	}
	if body == nil {
		body = map[string]any{}
	}
	id := stringField(body, "id")
	if id == "" {
		id = auth.RandomIDForInternalUse()
	}
	body["id"], body["user_id"] = id, user.ID
	encoded, _ := json.Marshal(body)
	resource, err := a.store.PutResource(r.Context(), store.Resource{Kind: compatibilityKind(domain), ID: id, UserID: user.ID, Key: resourceKey(domain, body, id), Body: encoded, Active: true})
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to create "+domain)
		return
	}
	writeRawJSON(w, http.StatusOK, resourceResponse(resource))
}

func (a *App) handleCompatibilityByID(w http.ResponseWriter, r *http.Request, user store.User, domain string, rest []string) {
	id := rest[0]
	if id == "" || id == "config" || id == "settings" || id == "verify" || id == "health_check" || id == "export" || id == "query" || id == "reset" {
		if r.Method == http.MethodGet {
			writeJSON(w, http.StatusOK, []any{})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": true})
		return
	}
	kind := compatibilityKind(domain)
	resource, err := a.store.ResourceByID(r.Context(), kind, id)
	if errors.Is(err, store.ErrResourceNotFound) {
		if r.Method == http.MethodGet {
			writeError(w, http.StatusNotFound, "resource not found")
			return
		}
		if r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodPatch {
			a.writeCompatibilityCreate(w, r, user, domain)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": true})
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load resource")
		return
	}
	if resource.UserID != user.ID && user.Role != "admin" {
		writeError(w, http.StatusForbidden, "Access prohibited")
		return
	}
	if r.Method == http.MethodGet {
		writeRawJSON(w, http.StatusOK, resourceResponse(resource))
		return
	}
	if r.Method == http.MethodDelete || strings.HasSuffix(r.URL.Path, "/delete") {
		_ = a.store.DeleteResource(r.Context(), kind, id)
		writeJSON(w, http.StatusOK, true)
		return
	}
	if strings.HasSuffix(r.URL.Path, "/toggle") {
		resource, _ = a.store.ToggleResource(r.Context(), kind, id)
		writeRawJSON(w, http.StatusOK, resourceResponse(resource))
		return
	}
	var patch map[string]any
	if decodeJSON(w, r, &patch) {
		var current map[string]any
		_ = json.Unmarshal(resource.Body, &current)
		for key, value := range patch {
			current[key] = value
		}
		resource.Body, _ = json.Marshal(current)
		resource, err = a.store.PutResource(r.Context(), resource)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to update resource")
			return
		}
		writeRawJSON(w, http.StatusOK, resourceResponse(resource))
		return
	}
}

func compatibilityKind(domain string) string {
	switch domain {
	case "prompts":
		return "prompt"
	case "tools":
		return "tool"
	case "skills":
		return "skill"
	case "notes":
		return "note"
	default:
		return domain
	}
}
