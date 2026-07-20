package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strings"

	"github.com/iwvw-per/HaloWebUI/backend/internal/auth"
	"github.com/iwvw-per/HaloWebUI/backend/internal/store"
)

var pluginIDPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func (a *App) handleFunctions(w http.ResponseWriter, r *http.Request) {
	user, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	if strings.HasSuffix(r.URL.Path, "/export") && user.Role != "admin" {
		writeError(w, http.StatusForbidden, "Access prohibited")
		return
	}
	resources, err := a.store.ListResources(r.Context(), "function", false)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list functions")
		return
	}
	result := make([]json.RawMessage, 0, len(resources))
	for _, resource := range resources {
		result = append(result, resourceResponse(resource))
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *App) handleFunctionCreate(w http.ResponseWriter, r *http.Request) {
	ok, userID := a.requireAdmin(w, r)
	if !ok {
		return
	}
	var body map[string]any
	if !decodeJSON(w, r, &body) {
		return
	}
	id := strings.ToLower(stringField(body, "id"))
	if !pluginIDPattern.MatchString(id) {
		writeError(w, http.StatusBadRequest, "The id must start with a letter or underscore, and may contain only letters, numbers, and underscores.")
		return
	}
	if _, err := a.store.ResourceByID(r.Context(), "function", id); err == nil {
		writeError(w, http.StatusBadRequest, "Function id is already taken")
		return
	}
	body["id"], body["user_id"], body["is_active"], body["is_global"] = id, userID, true, false
	encoded, _ := json.Marshal(body)
	resource, err := a.store.PutResource(r.Context(), store.Resource{Kind: "function", ID: id, UserID: userID, Key: id, Body: encoded, Active: true})
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to create function")
		return
	}
	writeRawJSON(w, http.StatusOK, resourceResponse(resource))
}

func (a *App) handleFunctionByID(w http.ResponseWriter, r *http.Request) {
	if ok, _ := a.requireAdmin(w, r); !ok {
		return
	}
	id := r.PathValue("id")
	resource, err := a.store.ResourceByID(r.Context(), "function", id)
	if errors.Is(err, store.ErrResourceNotFound) {
		writeError(w, http.StatusNotFound, "Function not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load function")
		return
	}
	switch {
	case r.Method == http.MethodGet:
		writeRawJSON(w, http.StatusOK, resourceResponse(resource))
	case r.Method == http.MethodDelete:
		if err := a.store.DeleteResource(r.Context(), "function", id); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to delete function")
			return
		}
		writeJSON(w, http.StatusOK, true)
	case strings.HasSuffix(r.URL.Path, "/toggle/global"):
		var body map[string]any
		_ = json.Unmarshal(resource.Body, &body)
		current, _ := body["is_global"].(bool)
		body["is_global"] = !current
		resource.Body, _ = json.Marshal(body)
		updated, updateErr := a.store.PutResource(r.Context(), resource)
		if updateErr != nil {
			writeError(w, http.StatusInternalServerError, "failed to toggle function")
			return
		}
		writeRawJSON(w, http.StatusOK, resourceResponse(updated))
	case strings.HasSuffix(r.URL.Path, "/toggle"):
		updated, updateErr := a.store.ToggleResource(r.Context(), "function", id)
		if updateErr != nil {
			writeError(w, http.StatusInternalServerError, "failed to toggle function")
			return
		}
		writeRawJSON(w, http.StatusOK, resourceResponse(updated))
	default:
		var patch map[string]any
		if !decodeJSON(w, r, &patch) {
			return
		}
		var body map[string]any
		_ = json.Unmarshal(resource.Body, &body)
		mergeJSONMap(body, patch)
		resource.Body, _ = json.Marshal(body)
		updated, updateErr := a.store.PutResource(r.Context(), resource)
		if updateErr != nil {
			writeError(w, http.StatusInternalServerError, "failed to update function")
			return
		}
		writeRawJSON(w, http.StatusOK, resourceResponse(updated))
	}
}

func (a *App) handlePluginValves(kind string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := a.requireUser(w, r)
		if !ok {
			return
		}
		id := r.PathValue("id")
		resource, err := a.store.ResourceByID(r.Context(), kind, id)
		if errors.Is(err, store.ErrResourceNotFound) {
			writeError(w, http.StatusNotFound, strings.Title(kind)+" not found")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to load "+kind)
			return
		}
		userScoped := strings.Contains(r.URL.Path, "/valves/user")
		if userScoped && !resourceReadableBy(resource, user, kind) {
			writeError(w, http.StatusForbidden, "Access prohibited")
			return
		}
		if !userScoped && resource.UserID != user.ID && user.Role != "admin" {
			writeError(w, http.StatusForbidden, "Access prohibited")
			return
		}
		var body map[string]any
		_ = json.Unmarshal(resource.Body, &body)
		if strings.HasSuffix(r.URL.Path, "/spec") {
			field := "valves_spec"
			if userScoped {
				field = "user_valves_spec"
			}
			writeJSON(w, http.StatusOK, body[field])
			return
		}
		if userScoped {
			key := kind + ":" + id + ":" + user.ID
			setting, settingErr := a.store.ResourceByKey(r.Context(), kind+"_user_valves", key)
			if r.Method == http.MethodGet {
				if errors.Is(settingErr, store.ErrResourceNotFound) {
					writeJSON(w, http.StatusOK, map[string]any{})
					return
				}
				if settingErr != nil {
					writeError(w, http.StatusInternalServerError, "failed to load user valves")
					return
				}
				writeRawJSON(w, http.StatusOK, setting.Body)
				return
			}
			var valves map[string]any
			if !decodeJSON(w, r, &valves) {
				return
			}
			if errors.Is(settingErr, store.ErrResourceNotFound) {
				setting = store.Resource{Kind: kind + "_user_valves", ID: auth.RandomIDForInternalUse(), UserID: user.ID, Key: key, Active: true}
			}
			setting.Body, _ = json.Marshal(valves)
			if _, err := a.store.PutResource(r.Context(), setting); err != nil {
				writeError(w, http.StatusInternalServerError, "failed to save user valves")
				return
			}
			writeJSON(w, http.StatusOK, valves)
			return
		}
		if r.Method == http.MethodGet {
			if valves, ok := body["valves"]; ok {
				writeJSON(w, http.StatusOK, valves)
			} else {
				writeJSON(w, http.StatusOK, map[string]any{})
			}
			return
		}
		var valves map[string]any
		if !decodeJSON(w, r, &valves) {
			return
		}
		body["valves"] = valves
		resource.Body, _ = json.Marshal(body)
		if _, err := a.store.PutResource(r.Context(), resource); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to save valves")
			return
		}
		writeJSON(w, http.StatusOK, valves)
	}
}

func (a *App) handleToolExport(w http.ResponseWriter, r *http.Request) {
	user, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	resources, err := a.store.ListResources(r.Context(), "tool", false)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to export tools")
		return
	}
	payload := make([]json.RawMessage, 0, len(resources))
	for _, resource := range resources {
		if resource.UserID == user.ID || user.Role == "admin" {
			payload = append(payload, resourceResponse(resource))
		}
	}
	writeJSON(w, http.StatusOK, payload)
}
