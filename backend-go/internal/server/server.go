package server

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/iwvw-per/HaloWebUI/backend-go/internal/auth"
	"github.com/iwvw-per/HaloWebUI/backend-go/internal/store"
)

type App struct {
	config   Config
	store    *store.Store
	frontend fs.FS
	index    []byte
	mux      *http.ServeMux
}

func New(config Config) (*App, error) {
	dataDir := config.DataDir
	if dataDir == "" {
		dataDir = filepath.Join(os.TempDir(), "halowebui-go")
	}
	dataStore, err := store.Open(context.Background(), dataDir)
	if err != nil {
		return nil, err
	}
	frontend := os.DirFS(config.FrontendDir)
	index, err := fs.ReadFile(frontend, "index.html")
	if err != nil {
		_ = dataStore.Close()
		return nil, errors.New("frontend index.html is unavailable in FRONTEND_DIR")
	}
	app := &App{
		config:   config,
		store:    dataStore,
		frontend: frontend,
		index:    index,
		mux:      http.NewServeMux(),
	}
	app.registerRoutes()
	return app, nil
}

func (a *App) Close() error {
	return a.store.Close()
}

func (a *App) registerRoutes() {
	a.mux.HandleFunc("GET /health", a.handleHealth)
	a.mux.HandleFunc("GET /api/version", a.handleVersion)
	a.mux.HandleFunc("GET /api/config", a.handleConfig)
	a.mux.HandleFunc("GET /api/v1/auths/", a.handleSession)
	a.mux.HandleFunc("POST /api/v1/auths/signin", a.handleSignin)
	a.mux.HandleFunc("POST /api/v1/auths/signup", a.handleSignup)
	a.mux.HandleFunc("GET /api/v1/auths/signout", a.handleSignout)
	a.mux.HandleFunc("GET /api/v1/auths/admin/details", a.handleAdminDetails)
	a.mux.HandleFunc("GET /api/v1/auths/admin/config", a.handleAdminConfig)
	a.mux.HandleFunc("POST /api/v1/auths/admin/config", a.handleAdminConfig)
	a.mux.HandleFunc("GET /api/v1/auths/admin/config/ldap", a.handleAuthAdminSetting)
	a.mux.HandleFunc("POST /api/v1/auths/admin/config/ldap", a.handleAuthAdminSetting)
	a.mux.HandleFunc("GET /api/v1/auths/admin/config/ldap/server", a.handleAuthAdminSetting)
	a.mux.HandleFunc("POST /api/v1/auths/admin/config/ldap/server", a.handleAuthAdminSetting)
	a.mux.HandleFunc("POST /api/v1/auths/add", a.handleAddUser)
	a.mux.HandleFunc("POST /api/v1/auths/update/profile", a.handleProfileUpdate)
	a.mux.HandleFunc("POST /api/v1/auths/update/password", a.handlePasswordUpdate)
	a.mux.HandleFunc("GET /api/v1/auths/signup/enabled", a.handleSignupConfig)
	a.mux.HandleFunc("GET /api/v1/auths/signup/enabled/toggle", a.handleSignupConfig)
	a.mux.HandleFunc("GET /api/v1/auths/signup/user/role", a.handleSignupConfig)
	a.mux.HandleFunc("POST /api/v1/auths/signup/user/role", a.handleSignupConfig)
	a.mux.HandleFunc("GET /api/v1/auths/token/expires", a.handleTokenExpiry)
	a.mux.HandleFunc("POST /api/v1/auths/token/expires/update", a.handleTokenExpiry)
	a.mux.HandleFunc("GET /api/v1/auths/api_key", a.handleAPIKey)
	a.mux.HandleFunc("POST /api/v1/auths/api_key", a.handleAPIKey)
	a.mux.HandleFunc("DELETE /api/v1/auths/api_key", a.handleAPIKey)
	a.mux.HandleFunc("POST /api/v1/chats/new", a.handleChatNew)
	a.mux.HandleFunc("POST /api/v1/chats/import", a.handleChatImport)
	a.mux.HandleFunc("GET /api/v1/chats/archived", a.handleChatList)
	a.mux.HandleFunc("GET /api/v1/chats/all/archived", a.handleChatList)
	a.mux.HandleFunc("GET /api/v1/chats/all", a.handleChatList)
	a.mux.HandleFunc("GET /api/v1/chats/", a.handleChatList)
	a.mux.HandleFunc("GET /api/v1/chats/{id}/context", a.handleChatContext)
	a.mux.HandleFunc("POST /api/v1/chats/{id}/archive", a.handleChatField)
	a.mux.HandleFunc("POST /api/v1/chats/{id}/pin", a.handleChatField)
	a.mux.HandleFunc("POST /api/v1/chats/{id}/share", a.handleChatField)
	a.mux.HandleFunc("POST /api/v1/chats/{id}/folder", a.handleChatField)
	a.mux.HandleFunc("POST /api/v1/chats/{id}/title", a.handleChatField)
	a.mux.HandleFunc("GET /api/v1/chats/{id}", a.handleChatByID)
	a.mux.HandleFunc("PUT /api/v1/chats/{id}", a.handleChatByID)
	a.mux.HandleFunc("DELETE /api/v1/chats/{id}", a.handleChatByID)
	a.mux.HandleFunc("POST /openai/chat/completions", a.handleOpenAIChat)
	a.mux.HandleFunc("GET /openai/models", a.handleOpenAIModels)
	a.mux.HandleFunc("GET /openai/models/{index}", a.handleOpenAIModels)
	a.mux.HandleFunc("/openai/", a.handleOpenAICompatibility)
	a.mux.HandleFunc("POST /api/chat/completions", a.handleUnifiedChat)
	a.mux.HandleFunc("POST /api/chat/completed", a.handleChatCompleted)
	a.mux.HandleFunc("GET /api/models", a.handleModels)
	a.mux.HandleFunc("GET /api/models/base", a.handleModels)
	a.mux.HandleFunc("GET /api/v1/models/", a.handleWorkspaceModels)
	a.mux.HandleFunc("GET /api/v1/models/base", a.handleWorkspaceModels)
	a.mux.HandleFunc("POST /api/v1/models/create", a.handleWorkspaceModelCreate)
	a.mux.HandleFunc("GET /api/v1/models/model", a.handleWorkspaceModel)
	a.mux.HandleFunc("POST /api/v1/models/model/update", a.handleWorkspaceModel)
	a.mux.HandleFunc("DELETE /api/v1/models/model/delete", a.handleWorkspaceModel)
	a.mux.HandleFunc("POST /api/v1/models/model/toggle", a.handleWorkspaceModelToggle)
	a.mux.HandleFunc("DELETE /api/v1/models/delete/all", a.handleWorkspaceModelsDeleteAll)
	a.mux.HandleFunc("GET /api/v1/users/", a.handleUsers)
	a.mux.HandleFunc("GET /api/v1/users/search", a.handleUsers)
	a.mux.HandleFunc("GET /api/v1/users/export/csv", a.handleUsersCSV)
	a.mux.HandleFunc("POST /api/v1/users/update/role", a.handleUserRole)
	a.mux.HandleFunc("GET /api/v1/users/groups", a.handleCompatibility)
	a.mux.HandleFunc("GET /api/v1/users/default/permissions", a.handleCompatibility)
	a.mux.HandleFunc("POST /api/v1/users/default/permissions", a.handleCompatibility)
	a.mux.HandleFunc("GET /api/v1/users/default/settings", a.handleCompatibility)
	a.mux.HandleFunc("POST /api/v1/users/default/settings", a.handleCompatibility)
	a.mux.HandleFunc("GET /api/v1/users/user/settings", a.handleCurrentUserSettings)
	a.mux.HandleFunc("POST /api/v1/users/user/settings/update", a.handleCurrentUserSettings)
	a.mux.HandleFunc("GET /api/v1/users/user/info", a.handleCurrentUserInfo)
	a.mux.HandleFunc("POST /api/v1/users/user/info/update", a.handleCurrentUserInfo)
	a.mux.HandleFunc("GET /api/v1/users/{id}", a.handleUserByID)
	a.mux.HandleFunc("DELETE /api/v1/users/{id}", a.handleUserByID)
	a.mux.HandleFunc("POST /api/v1/users/{id}/update", a.handleUserByID)
	a.registerResourceRoutes()
	a.mux.HandleFunc("GET /api/v1/files/", a.handleFiles)
	a.mux.HandleFunc("POST /api/v1/files/", a.handleFiles)
	a.mux.HandleFunc("GET /api/v1/files/{id}", a.handleFileByID)
	a.mux.HandleFunc("DELETE /api/v1/files/{id}", a.handleFileByID)
	a.mux.HandleFunc("GET /api/v1/files/{id}/content", a.handleFileContent)
	a.mux.HandleFunc("POST /api/v1/files/{id}/data/content/update", a.handleFileData)
	a.mux.HandleFunc("POST /ollama/api/chat", a.handleOllamaChat)
	a.mux.HandleFunc("GET /ollama/api/tags", a.handleOllamaTags)
	a.mux.HandleFunc("/ollama/", a.handleOllamaCompatibility)
	a.mux.HandleFunc("/anthropic/", a.handleDisabledProvider("anthropic"))
	a.mux.HandleFunc("/gemini/", a.handleDisabledProvider("gemini"))
	a.mux.HandleFunc("/grok/", a.handleDisabledProvider("grok"))
	a.mux.HandleFunc("GET /api/v1/providers/health", a.handleProviderHealth)
	// The compatibility handler covers the remaining UI domains while their
	// storage and provider adapters are migrated to typed Go packages. More
	// specific patterns above always win in net/http's ServeMux.
	a.mux.HandleFunc("/api/v1/", a.handleCompatibility)
	a.mux.HandleFunc("/", a.handleFrontend)
}

func (a *App) registerResourceRoutes() {
	for _, resource := range []struct {
		kind   string
		prefix string
	}{
		{kind: "prompt", prefix: "/api/v1/prompts"},
		{kind: "tool", prefix: "/api/v1/tools"},
		{kind: "skill", prefix: "/api/v1/skills"},
		{kind: "note", prefix: "/api/v1/notes"},
	} {
		a.mux.HandleFunc("GET "+resource.prefix+"/", a.handleResourceList(resource.kind, false))
		a.mux.HandleFunc("GET "+resource.prefix+"/list", a.handleResourceList(resource.kind, true))
		a.mux.HandleFunc("POST "+resource.prefix+"/create", a.handleResourceCreate(resource.kind))
		a.mux.HandleFunc("GET "+resource.prefix+"/id/{id}", a.handleResourceByID(resource.kind, "get"))
		a.mux.HandleFunc("POST "+resource.prefix+"/id/{id}/update", a.handleResourceByID(resource.kind, "update"))
		a.mux.HandleFunc("POST "+resource.prefix+"/id/{id}/toggle", a.handleResourceByID(resource.kind, "toggle"))
		a.mux.HandleFunc("DELETE "+resource.prefix+"/id/{id}/delete", a.handleResourceByID(resource.kind, "delete"))
		if resource.kind == "note" {
			a.mux.HandleFunc("GET "+resource.prefix+"/{id}", a.handleResourceByID(resource.kind, "get"))
			a.mux.HandleFunc("POST "+resource.prefix+"/{id}/update", a.handleResourceByID(resource.kind, "update"))
			a.mux.HandleFunc("DELETE "+resource.prefix+"/{id}/delete", a.handleResourceByID(resource.kind, "delete"))
		}
	}
}

func (a *App) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	response.Header().Set("X-Content-Type-Options", "nosniff")
	response.Header().Set("X-Frame-Options", "SAMEORIGIN")
	response.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
	a.mux.ServeHTTP(response, request)
}

func (a *App) handleHealth(response http.ResponseWriter, _ *http.Request) {
	writeJSON(response, http.StatusOK, map[string]bool{"status": true})
}

func (a *App) handleVersion(response http.ResponseWriter, _ *http.Request) {
	writeJSON(response, http.StatusOK, map[string]string{"version": a.config.Version})
}

func (a *App) handleConfig(response http.ResponseWriter, request *http.Request) {
	userCount, err := a.store.UserCount(request.Context())
	if err != nil {
		writeError(response, http.StatusInternalServerError, "database unavailable")
		return
	}
	payload := map[string]any{
		"onboarding":                 userCount == 0,
		"status":                     true,
		"name":                       a.config.WebUIName,
		"version":                    a.config.Version,
		"default_locale":             a.config.DefaultLocale,
		"default_models":             "",
		"default_prompt_suggestions": []any{},
		"oauth": map[string]any{
			"providers": map[string]string{},
		},
		"features": map[string]any{
			"auth":                            true,
			"auth_trusted_header":             false,
			"enable_api_key":                  a.config.EnableAPIKey,
			"enable_signup":                   a.config.EnableSignup,
			"enable_login_form":               a.config.EnableLoginForm,
			"enable_websocket":                a.config.EnableWebSocket,
			"enable_web_search":               false,
			"enable_halo_web_search":          false,
			"enable_native_web_search":        false,
			"default_web_search_mode":         "off",
			"enable_google_drive_integration": false,
			"enable_onedrive_integration":     false,
			"enable_image_generation":         false,
			"enable_admin_export":             false,
			"enable_admin_chat_access":        a.config.EnableAdminChatAccess,
			"enable_community_sharing":        false,
			"enable_autocomplete_generation":  false,
		},
	}
	writeJSON(response, http.StatusOK, payload)
}

func (a *App) handleSession(response http.ResponseWriter, request *http.Request) {
	user, ok := a.currentUser(request)
	if !ok {
		writeError(response, http.StatusUnauthorized, "Not authenticated")
		return
	}
	token, claims, err := auth.NewToken(a.secretKey(), user.ID, a.config.JWTExpiresAfter)
	if err != nil {
		writeError(response, http.StatusInternalServerError, "failed to create session")
		return
	}
	a.setSessionCookie(response, token, claims.ExpiresAt)
	_ = a.store.TouchUser(request.Context(), user.ID)
	a.writeSession(response, user, token, claims.ExpiresAt)
}

func (a *App) handleSignin(response http.ResponseWriter, request *http.Request) {
	var form struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !decodeJSON(response, request, &form) {
		return
	}
	user, passwordHash, err := a.store.Authenticate(request.Context(), form.Email)
	if err != nil || !auth.VerifyPassword(passwordHash, form.Password) {
		writeError(response, http.StatusBadRequest, "Invalid email or password")
		return
	}
	a.issueSession(response, request, user)
}

func (a *App) handleSignup(response http.ResponseWriter, request *http.Request) {
	var form struct {
		Name            string `json:"name"`
		Email           string `json:"email"`
		Password        string `json:"password"`
		ProfileImageURL string `json:"profile_image_url"`
	}
	if !decodeJSON(response, request, &form) {
		return
	}
	if !a.config.EnableSignup || !a.config.EnableLoginForm {
		writeError(response, http.StatusForbidden, "Access prohibited")
		return
	}
	if strings.TrimSpace(form.Name) == "" || !strings.Contains(form.Email, "@") {
		writeError(response, http.StatusBadRequest, "Invalid email format")
		return
	}
	if len([]byte(form.Password)) > 72 || len(form.Password) < 1 {
		writeError(response, http.StatusBadRequest, "Invalid password")
		return
	}
	passwordHash, err := auth.HashPassword(form.Password)
	if err != nil {
		writeError(response, http.StatusInternalServerError, "failed to hash password")
		return
	}
	if form.ProfileImageURL == "" {
		form.ProfileImageURL = "/user.png"
	}
	user, err := a.store.CreateUser(
		request.Context(), auth.RandomIDForInternalUse(), form.Name,
		form.Email, passwordHash, form.ProfileImageURL, a.config.DefaultUserRole,
	)
	if errors.Is(err, store.ErrEmailTaken) {
		writeError(response, http.StatusBadRequest, "Email already registered")
		return
	}
	if err != nil {
		writeError(response, http.StatusInternalServerError, "failed to create user")
		return
	}
	a.issueSession(response, request, user)
}

func (a *App) handleSignout(response http.ResponseWriter, _ *http.Request) {
	http.SetCookie(response, &http.Cookie{
		Name:     "token",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})
	writeJSON(response, http.StatusOK, map[string]bool{"status": true})
}

func (a *App) currentUser(request *http.Request) (store.User, bool) {
	token := ""
	if cookie, err := request.Cookie("token"); err == nil {
		token = cookie.Value
	}
	if token == "" {
		token = strings.TrimSpace(request.Header.Get("X-API-Key"))
	}
	if token == "" {
		const prefix = "Bearer "
		header := request.Header.Get("Authorization")
		if strings.HasPrefix(header, prefix) {
			token = strings.TrimSpace(strings.TrimPrefix(header, prefix))
		}
	}
	if strings.HasPrefix(token, "sk-") {
		user, err := a.store.UserByAPIKey(request.Context(), token)
		return user, err == nil
	}
	claims, err := auth.ParseToken(a.secretKey(), token)
	if err != nil {
		return store.User{}, false
	}
	user, err := a.store.UserByID(request.Context(), claims.ID)
	return user, err == nil
}

func (a *App) issueSession(response http.ResponseWriter, request *http.Request, user store.User) {
	token, claims, err := auth.NewToken(a.secretKey(), user.ID, a.config.JWTExpiresAfter)
	if err != nil {
		writeError(response, http.StatusInternalServerError, "failed to create session")
		return
	}
	a.setSessionCookie(response, token, claims.ExpiresAt)
	_ = a.store.TouchUser(request.Context(), user.ID)
	a.writeSession(response, user, token, claims.ExpiresAt)
}

func (a *App) writeSession(response http.ResponseWriter, user store.User, token string, expiresAt int64) {
	payload := map[string]any{
		"token":             token,
		"token_type":        "Bearer",
		"expires_at":        nil,
		"id":                user.ID,
		"email":             user.Email,
		"name":              user.Name,
		"role":              user.Role,
		"profile_image_url": user.ProfileImageURL,
		"permissions":       map[string]any{},
	}
	if expiresAt > 0 {
		payload["expires_at"] = expiresAt
	}
	writeJSON(response, http.StatusOK, payload)
}

func (a *App) setSessionCookie(response http.ResponseWriter, token string, expiresAt int64) {
	cookie := &http.Cookie{
		Name:     "token",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   a.config.CookieSecure,
		SameSite: parseSameSite(a.config.CookieSameSite),
	}
	if expiresAt > 0 {
		cookie.Expires = time.Unix(expiresAt, 0).UTC()
	}
	http.SetCookie(response, cookie)
}

func (a *App) secretKey() []byte {
	if len(a.config.SecretKey) != 0 {
		return a.config.SecretKey
	}
	return []byte("development-only-secret")
}

func parseSameSite(value string) http.SameSite {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "strict":
		return http.SameSiteStrictMode
	case "none":
		return http.SameSiteNoneMode
	default:
		return http.SameSiteLaxMode
	}
}

func (a *App) handleFrontend(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet && request.Method != http.MethodHead {
		http.NotFound(response, request)
		return
	}
	path := strings.TrimPrefix(filepath.ToSlash(request.URL.Path), "/")
	if path == "" {
		a.writeIndex(response, request)
		return
	}
	if strings.Contains(path, "..") {
		http.NotFound(response, request)
		return
	}
	file, err := fs.ReadFile(a.frontend, path)
	if err != nil {
		a.writeIndex(response, request)
		return
	}
	if extension := filepath.Ext(path); extension != "" {
		if contentType := mime.TypeByExtension(extension); contentType != "" {
			response.Header().Set("Content-Type", contentType)
		}
	}
	if strings.HasPrefix(path, "_app/immutable/") {
		response.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		response.Header().Set("Cache-Control", "public, max-age=86400")
	}
	response.Header().Set("Content-Length", integerString(len(file)))
	if request.Method == http.MethodGet {
		_, _ = response.Write(file)
	}
}

func (a *App) writeIndex(response http.ResponseWriter, request *http.Request) {
	response.Header().Set("Content-Type", "text/html; charset=utf-8")
	response.Header().Set("Cache-Control", "no-cache")
	response.Header().Set("Content-Length", integerString(len(a.index)))
	if request.Method == http.MethodGet {
		_, _ = response.Write(a.index)
	}
}

func writeJSON(response http.ResponseWriter, status int, payload any) {
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(status)
	_ = json.NewEncoder(response).Encode(payload)
}

func writeRawJSON(response http.ResponseWriter, status int, payload json.RawMessage) {
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(status)
	_, _ = response.Write(payload)
}

func writeError(response http.ResponseWriter, status int, detail string) {
	writeJSON(response, status, map[string]string{"detail": detail})
}

func decodeJSON(response http.ResponseWriter, request *http.Request, target any) bool {
	request.Body = http.MaxBytesReader(response, request.Body, 1<<20)
	if err := json.NewDecoder(request.Body).Decode(target); err != nil {
		writeError(response, http.StatusBadRequest, "Invalid JSON body")
		return false
	}
	return true
}

func integerString(value int) string {
	// Avoid fmt in the request path.
	if value == 0 {
		return "0"
	}
	buffer := [20]byte{}
	position := len(buffer)
	for value > 0 {
		position--
		buffer[position] = byte('0' + value%10)
		value /= 10
	}
	return string(buffer[position:])
}

func CheckHealth(url string, timeout time.Duration) error {
	client := &http.Client{Timeout: timeout}
	response, err := client.Get(url)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return errors.New("health endpoint returned a non-200 response")
	}
	return nil
}
