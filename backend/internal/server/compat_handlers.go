package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/iwvw-per/HaloWebUI/backend/internal/auth"
	"github.com/iwvw-per/HaloWebUI/backend/internal/store"
)

// handleCompatibility is a narrow boundary for legacy configuration paths.
// Resource CRUD is intentionally absent: an unowned endpoint must fail loudly
// instead of returning a fabricated success payload.
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
	if isSettingDomain(parts[0], parts) {
		a.handleCompatibilitySetting(w, r, user, strings.Join(parts, "/"))
		return
	}
	if parts[0] == "auths" && len(parts) > 1 && (parts[1] == "signin" || parts[1] == "signup" || parts[1] == "signout") {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"detail": "use the auth endpoint"})
		return
	}
	if !compatibilityDomain(parts[0]) {
		writeJSON(w, http.StatusNotFound, map[string]string{"detail": "Not found"})
		return
	}
	writeError(w, http.StatusNotImplemented, "endpoint is not implemented in the Go slim profile")
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
	case "folders", "groups", "functions", "channels", "memories", "knowledge", "analytics", "terminal", "retrieval", "audio", "images", "gemini", "anthropic", "grok", "configs", "auths", "users", "chats", "files", "models", "prompts", "tools", "skills", "notes", "utils", "webhooks", "tasks", "haloclaw", "external_api":
		return true
	default:
		return false
	}
}

func isSettingDomain(domain string, parts []string) bool {
	if domain == "configs" {
		if len(parts) != 2 {
			return false
		}
		for _, name := range []string{"direct_connections", "connections", "tool_servers", "native_tools", "mcp_servers", "code_execution", "models", "banners", "suggestions"} {
			if parts[1] == name {
				return true
			}
		}
		return false
	}
	if domain == "retrieval" {
		return len(parts) == 2 && (parts[1] == "config" || parts[1] == "embedding" || parts[1] == "reranking" || parts[1] == "query")
	}
	if domain == "terminal" || domain == "audio" || domain == "images" || domain == "gemini" || domain == "anthropic" || domain == "grok" {
		return len(parts) == 2 && (parts[1] == "config" || parts[1] == "config/update" || parts[1] == "models" || parts[1] == "voices")
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
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
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
	if errors.Is(err, store.ErrResourceNotFound) || err != nil {
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
