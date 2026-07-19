package server

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/iwvw-per/HaloWebUI/backend-go/internal/store"
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
