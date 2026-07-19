package server

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"strings"
	"time"

	"github.com/iwvw-per/HaloWebUI/backend-go/internal/auth"
)

func (a *App) handleAdminDetails(w http.ResponseWriter, r *http.Request) {
	if ok, _ := a.requireAdmin(w, r); !ok {
		return
	}
	users, err := a.store.ListUsers(r.Context(), "", 1)
	if err != nil || len(users) == 0 {
		writeError(w, http.StatusNotFound, "Admin not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"name": users[0].Name, "email": users[0].Email})
}

func (a *App) handleAdminConfig(w http.ResponseWriter, r *http.Request) {
	if ok, _ := a.requireAdmin(w, r); !ok {
		return
	}
	if r.Method == http.MethodPost {
		var ignored map[string]any
		if !decodeJSON(w, r, &ignored) {
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"SHOW_ADMIN_DETAILS": true,
		"ENABLE_SIGNUP":      a.config.EnableSignup,
		"DEFAULT_USER_ROLE":  a.config.DefaultUserRole,
		"JWT_EXPIRES_IN":     expiryString(a.config.JWTExpiresAfter),
	})
}

func (a *App) handleAddUser(w http.ResponseWriter, r *http.Request) {
	if ok, _ := a.requireAdmin(w, r); !ok {
		return
	}
	var form struct {
		Name     string `json:"name"`
		Email    string `json:"email"`
		Password string `json:"password"`
		Role     string `json:"role"`
	}
	if !decodeJSON(w, r, &form) {
		return
	}
	if form.Name == "" || !strings.Contains(form.Email, "@") || form.Password == "" {
		writeError(w, http.StatusBadRequest, "Invalid user")
		return
	}
	hash, err := auth.HashPassword(form.Password)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid password")
		return
	}
	user, err := a.store.CreateUser(r.Context(), auth.RandomIDForInternalUse(), form.Name, form.Email, hash, "/user.png", form.Role)
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to create user")
		return
	}
	writeJSON(w, http.StatusOK, user)
}

func (a *App) handleProfileUpdate(w http.ResponseWriter, r *http.Request) {
	user, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	var form struct {
		Name            string `json:"name"`
		ProfileImageURL string `json:"profile_image_url"`
	}
	if !decodeJSON(w, r, &form) {
		return
	}
	updated, err := a.store.UpdateUser(r.Context(), user.ID, form.Name, "", "", form.ProfileImageURL)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update profile")
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (a *App) handlePasswordUpdate(w http.ResponseWriter, r *http.Request) {
	user, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	var form struct {
		Password    string `json:"password"`
		NewPassword string `json:"new_password"`
	}
	if !decodeJSON(w, r, &form) {
		return
	}
	_, currentHash, err := a.store.Authenticate(r.Context(), user.Email)
	if err != nil || !auth.VerifyPassword(currentHash, form.Password) {
		writeError(w, http.StatusBadRequest, "Invalid password")
		return
	}
	newHash, err := auth.HashPassword(form.NewPassword)
	if err != nil || form.NewPassword == "" {
		writeError(w, http.StatusBadRequest, "Invalid password")
		return
	}
	if err := a.store.UpdatePassword(r.Context(), user.ID, newHash); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update password")
		return
	}
	writeJSON(w, http.StatusOK, true)
}

func (a *App) handleSignupConfig(w http.ResponseWriter, r *http.Request) {
	if ok, _ := a.requireAdmin(w, r); !ok {
		return
	}
	if strings.HasSuffix(r.URL.Path, "/role") {
		writeJSON(w, http.StatusOK, map[string]string{"role": a.config.DefaultUserRole})
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"enabled": a.config.EnableSignup})
}

func (a *App) handleTokenExpiry(w http.ResponseWriter, r *http.Request) {
	if ok, _ := a.requireAdmin(w, r); !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"duration": expiryString(a.config.JWTExpiresAfter)})
}

func (a *App) handleAPIKey(w http.ResponseWriter, r *http.Request) {
	user, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	switch r.Method {
	case http.MethodGet:
		if !user.APIKey.Valid || user.APIKey.String == "" {
			writeError(w, http.StatusNotFound, "API key not found")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"api_key": user.APIKey.String})
	case http.MethodPost:
		if !a.config.EnableAPIKey {
			writeError(w, http.StatusForbidden, "API key creation is disabled")
			return
		}
		value := make([]byte, 32)
		if _, err := rand.Read(value); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to create API key")
			return
		}
		apiKey := "sk-" + base64.RawURLEncoding.EncodeToString(value)
		if err := a.store.SetAPIKey(r.Context(), user.ID, &apiKey); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to save API key")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"api_key": apiKey})
	case http.MethodDelete:
		if err := a.store.SetAPIKey(r.Context(), user.ID, nil); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to delete API key")
			return
		}
		writeJSON(w, http.StatusOK, true)
	}
}

func expiryString(duration time.Duration) string {
	if duration <= 0 {
		return "-1"
	}
	return duration.String()
}
