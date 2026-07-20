package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/iwvw-per/HaloWebUI/backend/internal/store"
)

func (a *App) handleWorkspaceModels(response http.ResponseWriter, request *http.Request) {
	if _, ok := a.requireUser(response, request); !ok {
		return
	}
	models, err := a.store.ListModels(request.Context(), false)
	if err != nil {
		writeError(response, http.StatusInternalServerError, "failed to list models")
		return
	}
	writeJSON(response, http.StatusOK, models)
}

func (a *App) handleWorkspaceModelCreate(response http.ResponseWriter, request *http.Request) {
	user, ok := a.requireUser(response, request)
	if !ok {
		return
	}
	var model store.Model
	if !decodeJSON(response, request, &model) {
		return
	}
	if model.ID == "" || model.Name == "" {
		writeError(response, http.StatusBadRequest, "model id and name are required")
		return
	}
	if _, err := a.store.ModelByID(request.Context(), model.ID); err == nil {
		writeError(response, http.StatusBadRequest, "Model ID already exists")
		return
	}
	model.UserID = user.ID
	model.IsActive = true
	created, err := a.store.UpsertModel(request.Context(), model)
	if err != nil {
		writeError(response, http.StatusInternalServerError, "failed to create model")
		return
	}
	writeJSON(response, http.StatusOK, created)
}

func (a *App) handleWorkspaceModel(response http.ResponseWriter, request *http.Request) {
	user, ok := a.requireUser(response, request)
	if !ok {
		return
	}
	id := request.URL.Query().Get("id")
	model, err := a.store.ModelByID(request.Context(), id)
	if errors.Is(err, store.ErrModelNotFound) {
		writeError(response, http.StatusNotFound, "Model not found")
		return
	}
	if err != nil {
		writeError(response, http.StatusInternalServerError, "failed to load model")
		return
	}
	if request.Method == http.MethodGet {
		writeJSON(response, http.StatusOK, model)
		return
	}
	if model.UserID != user.ID && user.Role != "admin" {
		writeError(response, http.StatusForbidden, "Access prohibited")
		return
	}
	switch request.Method {
	case http.MethodPost:
		var patch map[string]json.RawMessage
		if !decodeJSON(response, request, &patch) {
			return
		}
		applyModelPatch(&model, patch)
		updated, err := a.store.UpsertModel(request.Context(), model)
		if err != nil {
			writeError(response, http.StatusInternalServerError, "failed to update model")
			return
		}
		writeJSON(response, http.StatusOK, updated)
	case http.MethodDelete:
		if err := a.store.DeleteModel(request.Context(), id); err != nil {
			writeError(response, http.StatusInternalServerError, "failed to delete model")
			return
		}
		writeJSON(response, http.StatusOK, map[string]bool{"status": true})
	}
}

func (a *App) handleWorkspaceModelToggle(response http.ResponseWriter, request *http.Request) {
	if _, ok := a.requireUser(response, request); !ok {
		return
	}
	model, err := a.store.ToggleModel(request.Context(), request.URL.Query().Get("id"))
	if err != nil {
		writeError(response, http.StatusNotFound, "Model not found")
		return
	}
	writeJSON(response, http.StatusOK, model)
}

func (a *App) handleWorkspaceModelsDeleteAll(response http.ResponseWriter, request *http.Request) {
	user, ok := a.requireUser(response, request)
	if !ok {
		return
	}
	if user.Role != "admin" {
		writeError(response, http.StatusForbidden, "Access prohibited")
		return
	}
	if err := a.store.DeleteAllModels(request.Context()); err != nil {
		writeError(response, http.StatusInternalServerError, "failed to delete models")
		return
	}
	writeJSON(response, http.StatusOK, map[string]bool{"status": true})
}

func (a *App) handleWorkspaceModelsBulkUpdate(response http.ResponseWriter, request *http.Request) {
	user, ok := a.requireUser(response, request)
	if !ok {
		return
	}
	if user.Role != "admin" {
		writeError(response, http.StatusForbidden, "Access prohibited")
		return
	}
	var form struct {
		Items []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"items"`
		Patch struct {
			IsActive      *bool           `json:"is_active"`
			AccessControl json.RawMessage `json:"access_control"`
			Meta          json.RawMessage `json:"meta"`
		} `json:"patch"`
	}
	if !decodeJSON(response, request, &form) {
		return
	}
	if len(form.Patch.Meta) > 0 && string(form.Patch.Meta) != "null" && !jsonObject(form.Patch.Meta) {
		writeError(response, http.StatusBadRequest, "meta must be a JSON object")
		return
	}

	result := map[string]int{"updated": 0, "created": 0, "skipped": 0}
	seen := make(map[string]struct{}, len(form.Items))
	for _, item := range form.Items {
		item.ID = strings.TrimSpace(item.ID)
		if item.ID == "" {
			result["skipped"]++
			continue
		}
		if _, duplicate := seen[item.ID]; duplicate {
			result["skipped"]++
			continue
		}
		seen[item.ID] = struct{}{}

		model, err := a.store.ModelByID(request.Context(), item.ID)
		created := errors.Is(err, store.ErrModelNotFound)
		if err != nil && !created {
			writeError(response, http.StatusInternalServerError, "failed to load model")
			return
		}
		if created {
			name := strings.TrimSpace(item.Name)
			if name == "" {
				name = item.ID
			}
			model = store.Model{
				ID: item.ID, UserID: user.ID, Name: name, Params: json.RawMessage(`{}`),
				Meta: json.RawMessage(`{}`), IsActive: true,
			}
		} else if model.BaseModelID != nil {
			result["skipped"]++
			continue
		}

		if form.Patch.IsActive != nil {
			model.IsActive = *form.Patch.IsActive
		}
		if len(form.Patch.AccessControl) > 0 {
			if string(form.Patch.AccessControl) == "null" {
				model.AccessControl = nil
			} else {
				model.AccessControl = append(json.RawMessage(nil), form.Patch.AccessControl...)
			}
		}
		if len(form.Patch.Meta) > 0 && string(form.Patch.Meta) != "null" {
			model.Meta = mergeJSONObject(model.Meta, form.Patch.Meta)
		}
		if _, err := a.store.UpsertModel(request.Context(), model); err != nil {
			writeError(response, http.StatusInternalServerError, "failed to save model")
			return
		}
		if created {
			result["created"]++
		} else {
			result["updated"]++
		}
	}
	writeJSON(response, http.StatusOK, result)
}

func jsonObject(value json.RawMessage) bool {
	var object map[string]any
	return json.Unmarshal(value, &object) == nil && object != nil
}

func mergeJSONObject(current, patch json.RawMessage) json.RawMessage {
	merged := map[string]any{}
	_ = json.Unmarshal(current, &merged)
	var values map[string]any
	_ = json.Unmarshal(patch, &values)
	for key, value := range values {
		merged[key] = value
	}
	encoded, _ := json.Marshal(merged)
	return encoded
}

func applyModelPatch(model *store.Model, patch map[string]json.RawMessage) {
	if value, ok := patch["name"]; ok {
		_ = json.Unmarshal(value, &model.Name)
	}
	if value, ok := patch["base_model_id"]; ok {
		_ = json.Unmarshal(value, &model.BaseModelID)
	}
	if value, ok := patch["params"]; ok {
		model.Params = value
	}
	if value, ok := patch["meta"]; ok {
		model.Meta = value
	}
	if value, ok := patch["access_control"]; ok {
		model.AccessControl = value
	}
	if value, ok := patch["is_active"]; ok {
		_ = json.Unmarshal(value, &model.IsActive)
	}
}
