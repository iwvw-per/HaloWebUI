package server

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/iwvw-per/HaloWebUI/backend/internal/auth"
	"github.com/iwvw-per/HaloWebUI/backend/internal/store"
)

func (a *App) handleFolders(w http.ResponseWriter, r *http.Request) {
	user, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	if r.Method == http.MethodGet {
		resources, err := a.store.ListResources(r.Context(), "folder", false)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to load folders")
			return
		}
		result := make([]json.RawMessage, 0)
		for _, resource := range resources {
			if resource.UserID == user.ID {
				result = append(result, resourceResponse(resource))
			}
		}
		writeJSON(w, http.StatusOK, result)
		return
	}
	var body map[string]any
	if !decodeJSON(w, r, &body) {
		return
	}
	id := auth.RandomIDForInternalUse()
	body["id"], body["user_id"] = id, user.ID
	if _, ok := body["name"].(string); !ok {
		body["name"] = "New Folder"
	}
	encoded, _ := json.Marshal(body)
	resource, err := a.store.PutResource(r.Context(), store.Resource{Kind: "folder", ID: id, UserID: user.ID, Key: id, Body: encoded, Active: true})
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to create folder")
		return
	}
	writeRawJSON(w, http.StatusOK, resourceResponse(resource))
}

func (a *App) handleFolderByID(w http.ResponseWriter, r *http.Request) {
	user, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	resource, err := a.store.ResourceByID(r.Context(), "folder", r.PathValue("id"))
	if errors.Is(err, store.ErrResourceNotFound) {
		writeError(w, http.StatusNotFound, "Folder not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load folder")
		return
	}
	if resource.UserID != user.ID {
		writeError(w, http.StatusForbidden, "Access prohibited")
		return
	}
	if r.Method == http.MethodGet {
		writeRawJSON(w, http.StatusOK, resourceResponse(resource))
		return
	}
	if r.Method == http.MethodDelete {
		if err := a.store.DeleteResource(r.Context(), "folder", resource.ID); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to delete folder")
			return
		}
		writeJSON(w, http.StatusOK, true)
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
		writeError(w, http.StatusInternalServerError, "failed to update folder")
		return
	}
	writeRawJSON(w, http.StatusOK, resourceResponse(updated))
}

func (a *App) handleGroups(w http.ResponseWriter, r *http.Request) {
	ok, userID := a.requireAdmin(w, r)
	if !ok {
		return
	}
	if r.Method == http.MethodGet {
		resources, err := a.store.ListResources(r.Context(), "group", false)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to load groups")
			return
		}
		result := make([]json.RawMessage, 0, len(resources))
		for _, resource := range resources {
			result = append(result, resourceResponse(resource))
		}
		writeJSON(w, http.StatusOK, result)
		return
	}
	var body map[string]any
	if !decodeJSON(w, r, &body) {
		return
	}
	id := auth.RandomIDForInternalUse()
	body["id"], body["user_id"] = id, userID
	encoded, _ := json.Marshal(body)
	resource, err := a.store.PutResource(r.Context(), store.Resource{Kind: "group", ID: id, UserID: userID, Key: id, Body: encoded, Active: true})
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to create group")
		return
	}
	writeRawJSON(w, http.StatusOK, resourceResponse(resource))
}

func (a *App) handleGroupByID(w http.ResponseWriter, r *http.Request) {
	if ok, _ := a.requireAdmin(w, r); !ok {
		return
	}
	resource, err := a.store.ResourceByID(r.Context(), "group", r.PathValue("id"))
	if errors.Is(err, store.ErrResourceNotFound) {
		writeError(w, http.StatusNotFound, "Group not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load group")
		return
	}
	if r.Method == http.MethodGet {
		writeRawJSON(w, http.StatusOK, resourceResponse(resource))
		return
	}
	if r.Method == http.MethodDelete {
		if err := a.store.DeleteResource(r.Context(), "group", resource.ID); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to delete group")
			return
		}
		writeJSON(w, http.StatusOK, true)
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
		writeError(w, http.StatusInternalServerError, "failed to update group")
		return
	}
	writeRawJSON(w, http.StatusOK, resourceResponse(updated))
}

func (a *App) handleUserGroups(w http.ResponseWriter, r *http.Request) {
	user, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	resources, err := a.store.ListResources(r.Context(), "group", false)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load groups")
		return
	}
	result := make([]json.RawMessage, 0)
	for _, resource := range resources {
		var body map[string]any
		_ = json.Unmarshal(resource.Body, &body)
		for _, id := range stringSlice(body["user_ids"]) {
			if id == user.ID {
				result = append(result, resourceResponse(resource))
				break
			}
		}
	}
	writeJSON(w, http.StatusOK, result)
}
