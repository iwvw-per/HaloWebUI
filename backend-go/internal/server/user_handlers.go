package server

import (
	"encoding/csv"
	"encoding/json"
	"net/http"
	"strconv"
)

func (a *App) requireAdmin(response http.ResponseWriter, request *http.Request) (bool, string) {
	user, ok := a.requireUser(response, request)
	if !ok {
		return false, ""
	}
	if user.Role != "admin" {
		writeError(response, http.StatusForbidden, "Access prohibited")
		return false, ""
	}
	return true, user.ID
}

func (a *App) handleUsers(response http.ResponseWriter, request *http.Request) {
	if ok, _ := a.requireAdmin(response, request); !ok {
		return
	}
	limit, _ := strconv.Atoi(request.URL.Query().Get("limit"))
	users, err := a.store.ListUsers(request.Context(), request.URL.Query().Get("query"), limit)
	if err != nil {
		writeError(response, http.StatusInternalServerError, "failed to list users")
		return
	}
	writeJSON(response, http.StatusOK, users)
}

func (a *App) handleUserByID(response http.ResponseWriter, request *http.Request) {
	if ok, _ := a.requireAdmin(response, request); !ok {
		return
	}
	id := request.PathValue("id")
	switch request.Method {
	case http.MethodGet:
		user, err := a.store.UserByID(request.Context(), id)
		if err != nil {
			writeError(response, http.StatusNotFound, "User not found")
			return
		}
		writeJSON(response, http.StatusOK, user)
	case http.MethodDelete:
		if err := a.store.DeleteUser(request.Context(), id); err != nil {
			writeError(response, http.StatusInternalServerError, "failed to delete user")
			return
		}
		writeJSON(response, http.StatusOK, map[string]bool{"status": true})
	case http.MethodPost:
		var form struct {
			Name            string `json:"name"`
			Email           string `json:"email"`
			Role            string `json:"role"`
			ProfileImageURL string `json:"profile_image_url"`
		}
		if !decodeJSON(response, request, &form) {
			return
		}
		user, err := a.store.UpdateUser(request.Context(), id, form.Name, form.Email, form.Role, form.ProfileImageURL)
		if err != nil {
			writeError(response, http.StatusInternalServerError, "failed to update user")
			return
		}
		writeJSON(response, http.StatusOK, user)
	}
}

func (a *App) handleUserRole(response http.ResponseWriter, request *http.Request) {
	if ok, _ := a.requireAdmin(response, request); !ok {
		return
	}
	var form struct {
		ID   string `json:"id"`
		Role string `json:"role"`
	}
	if !decodeJSON(response, request, &form) {
		return
	}
	user, err := a.store.UpdateUser(request.Context(), form.ID, "", "", form.Role, "")
	if err != nil {
		writeError(response, http.StatusInternalServerError, "failed to update role")
		return
	}
	writeJSON(response, http.StatusOK, user)
}

func (a *App) handleCurrentUserSettings(response http.ResponseWriter, request *http.Request) {
	user, ok := a.requireUser(response, request)
	if !ok {
		return
	}
	if request.Method == http.MethodGet {
		settings, err := a.store.UserSettings(request.Context(), user.ID)
		if err != nil {
			writeError(response, http.StatusInternalServerError, "failed to load settings")
			return
		}
		writeRawJSON(response, http.StatusOK, settings)
		return
	}
	var settings json.RawMessage
	if !decodeJSON(response, request, &settings) {
		return
	}
	updated, err := a.store.SetUserSettings(request.Context(), user.ID, settings)
	if err != nil {
		writeError(response, http.StatusInternalServerError, "failed to update settings")
		return
	}
	writeRawJSON(response, http.StatusOK, updated)
}

func (a *App) handleCurrentUserInfo(response http.ResponseWriter, request *http.Request) {
	user, ok := a.requireUser(response, request)
	if !ok {
		return
	}
	if request.Method == http.MethodGet {
		info, err := a.store.UserInfo(request.Context(), user.ID)
		if err != nil {
			writeError(response, http.StatusInternalServerError, "failed to load info")
			return
		}
		writeRawJSON(response, http.StatusOK, info)
		return
	}
	var info json.RawMessage
	if !decodeJSON(response, request, &info) {
		return
	}
	updated, err := a.store.SetUserInfo(request.Context(), user.ID, info)
	if err != nil {
		writeError(response, http.StatusInternalServerError, "failed to update info")
		return
	}
	writeRawJSON(response, http.StatusOK, updated)
}

func (a *App) handleUsersCSV(response http.ResponseWriter, request *http.Request) {
	if ok, _ := a.requireAdmin(response, request); !ok {
		return
	}
	users, err := a.store.ListUsers(request.Context(), "", 500)
	if err != nil {
		writeError(response, http.StatusInternalServerError, "failed to list users")
		return
	}
	response.Header().Set("Content-Type", "text/csv; charset=utf-8")
	response.Header().Set("Content-Disposition", `attachment; filename="users.csv"`)
	writer := csv.NewWriter(response)
	_ = writer.Write([]string{"id", "name", "email", "role", "created_at"})
	for _, user := range users {
		_ = writer.Write([]string{user.ID, user.Name, user.Email, user.Role, strconv.FormatInt(user.CreatedAt, 10)})
	}
	writer.Flush()
}
