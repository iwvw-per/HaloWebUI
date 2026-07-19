package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/iwvw-per/HaloWebUI/backend-go/internal/auth"
	"github.com/iwvw-per/HaloWebUI/backend-go/internal/store"
)

func (a *App) handleResourceList(kind string, activeOnly bool) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		prefix := map[string]string{
			"prompt": "/api/v1/prompts",
			"tool":   "/api/v1/tools",
			"skill":  "/api/v1/skills",
			"note":   "/api/v1/notes",
		}[kind]
		if request.URL.Path != prefix+"/" && request.URL.Path != prefix+"/list" {
			a.handleCompatibility(response, request)
			return
		}
		if _, ok := a.requireUser(response, request); !ok {
			return
		}
		resources, err := a.store.ListResources(request.Context(), kind, activeOnly)
		if err != nil {
			writeError(response, http.StatusInternalServerError, "failed to list "+kind)
			return
		}
		payload := make([]json.RawMessage, 0, len(resources))
		for _, resource := range resources {
			payload = append(payload, resourceResponse(resource))
		}
		writeJSON(response, http.StatusOK, payload)
	}
}

func (a *App) handleResourceCreate(kind string) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		user, ok := a.requireUser(response, request)
		if !ok {
			return
		}
		var body map[string]any
		if !decodeJSON(response, request, &body) {
			return
		}
		id := stringField(body, "id")
		if id == "" {
			id = auth.RandomIDForInternalUse()
		}
		key := resourceKey(kind, body, id)
		body["id"], body["user_id"], body["is_active"] = id, user.ID, true
		encoded, _ := json.Marshal(body)
		resource, err := a.store.PutResource(request.Context(), store.Resource{
			Kind: kind, ID: id, UserID: user.ID, Key: key, Body: encoded, Active: true,
		})
		if err != nil {
			writeError(response, http.StatusBadRequest, "failed to create "+kind)
			return
		}
		writeRawJSON(response, http.StatusOK, resourceResponse(resource))
	}
}

func (a *App) handleResourceByID(kind, operation string) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		user, ok := a.requireUser(response, request)
		if !ok {
			return
		}
		id := request.PathValue("id")
		resource, err := a.store.ResourceByID(request.Context(), kind, id)
		if errors.Is(err, store.ErrResourceNotFound) {
			writeError(response, http.StatusNotFound, strings.Title(kind)+" not found")
			return
		}
		if err != nil {
			writeError(response, http.StatusInternalServerError, "failed to load "+kind)
			return
		}
		if resource.UserID != user.ID && user.Role != "admin" {
			writeError(response, http.StatusForbidden, "Access prohibited")
			return
		}
		switch operation {
		case "get":
			writeRawJSON(response, http.StatusOK, resourceResponse(resource))
		case "delete":
			if err := a.store.DeleteResource(request.Context(), kind, id); err != nil {
				writeError(response, http.StatusInternalServerError, "failed to delete "+kind)
				return
			}
			writeJSON(response, http.StatusOK, true)
		case "toggle":
			resource, err = a.store.ToggleResource(request.Context(), kind, id)
			if err != nil {
				writeError(response, http.StatusInternalServerError, "failed to toggle "+kind)
				return
			}
			writeRawJSON(response, http.StatusOK, resourceResponse(resource))
		case "update":
			var patch map[string]any
			if !decodeJSON(response, request, &patch) {
				return
			}
			var current map[string]any
			_ = json.Unmarshal(resource.Body, &current)
			for key, value := range patch {
				current[key] = value
			}
			resource.Key = resourceKey(kind, current, id)
			resource.Body, _ = json.Marshal(current)
			resource, err = a.store.PutResource(request.Context(), resource)
			if err != nil {
				writeError(response, http.StatusInternalServerError, "failed to update "+kind)
				return
			}
			writeRawJSON(response, http.StatusOK, resourceResponse(resource))
		}
	}
}

func resourceResponse(resource store.Resource) json.RawMessage {
	var body map[string]any
	if json.Unmarshal(resource.Body, &body) != nil {
		body = map[string]any{}
	}
	body["id"] = resource.ID
	body["user_id"] = resource.UserID
	body["is_active"] = resource.Active
	body["created_at"] = resource.CreatedAt
	body["updated_at"] = resource.UpdatedAt
	encoded, _ := json.Marshal(body)
	return encoded
}

func resourceKey(kind string, body map[string]any, fallback string) string {
	for _, field := range []string{"command", "name", "title", "id"} {
		if value := stringField(body, field); value != "" {
			if kind == "prompt" && field == "command" {
				return strings.TrimPrefix(value, "/")
			}
			return value
		}
	}
	return fallback
}

func stringField(body map[string]any, field string) string {
	value, _ := body[field].(string)
	return strings.TrimSpace(value)
}
