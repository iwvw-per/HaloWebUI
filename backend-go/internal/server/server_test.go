package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func testApp(t *testing.T) *App {
	t.Helper()
	frontend := t.TempDir()
	if err := os.WriteFile(filepath.Join(frontend, "index.html"), []byte("<h1>Halo</h1>"), 0o600); err != nil {
		t.Fatal(err)
	}
	immutable := filepath.Join(frontend, "_app", "immutable")
	if err := os.MkdirAll(immutable, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(immutable, "app.js"), []byte("ok"), 0o600); err != nil {
		t.Fatal(err)
	}
	app, err := New(Config{
		Version:         "1.2.3",
		WebUIName:       "HaloWebUI",
		DefaultLocale:   "zh-CN",
		FrontendDir:     frontend,
		DataDir:         t.TempDir(),
		SecretKey:       []byte("test-secret-that-is-long-enough-for-hmac"),
		EnableSignup:    true,
		EnableLoginForm: true,
		EnableAPIKey:    true,
		DefaultUserRole: "pending",
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = app.Close() })
	return app
}

func TestHealthContract(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/health", nil)
	response := httptest.NewRecorder()
	testApp(t).ServeHTTP(response, request)

	if response.Code != http.StatusOK || response.Body.String() != "{\"status\":true}\n" {
		t.Fatalf("unexpected health response: %d %q", response.Code, response.Body.String())
	}
}

func TestVersionContract(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/version", nil)
	response := httptest.NewRecorder()
	testApp(t).ServeHTTP(response, request)

	var payload map[string]string
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if payload["version"] != "1.2.3" {
		t.Fatalf("unexpected version payload: %#v", payload)
	}
}

func TestConfigDisablesHeavyCapabilities(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	response := httptest.NewRecorder()
	testApp(t).ServeHTTP(response, request)

	var payload struct {
		Status   bool           `json:"status"`
		Features map[string]any `json:"features"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if !payload.Status || payload.Features["enable_image_generation"] != false {
		t.Fatalf("unexpected config payload: %#v", payload)
	}
}

func TestFrontendSPAFallbackAndImmutableCaching(t *testing.T) {
	app := testApp(t)
	spa := httptest.NewRecorder()
	app.ServeHTTP(spa, httptest.NewRequest(http.MethodGet, "/chat/new", nil))
	if spa.Code != http.StatusOK || spa.Body.String() != "<h1>Halo</h1>" {
		t.Fatalf("unexpected SPA response: %d %q", spa.Code, spa.Body.String())
	}

	asset := httptest.NewRecorder()
	app.ServeHTTP(asset, httptest.NewRequest(http.MethodGet, "/_app/immutable/app.js", nil))
	if asset.Header().Get("Cache-Control") != "public, max-age=31536000, immutable" {
		t.Fatalf("unexpected asset cache policy: %q", asset.Header().Get("Cache-Control"))
	}
}

func TestNewRequiresFrontendIndex(t *testing.T) {
	_, err := New(Config{FrontendDir: t.TempDir(), DataDir: t.TempDir()})
	if err == nil {
		t.Fatal("expected missing frontend index to fail")
	}
}

func TestSignupAndSessionContract(t *testing.T) {
	app := testApp(t)
	signup := httptest.NewRecorder()
	app.ServeHTTP(
		signup,
		httptest.NewRequest(
			http.MethodPost,
			"/api/v1/auths/signup",
			bytes.NewBufferString(`{"name":"Admin","email":"admin@example.com","password":"secret"}`),
		),
	)
	if signup.Code != http.StatusOK {
		t.Fatalf("signup failed: %d %s", signup.Code, signup.Body.String())
	}
	var session map[string]any
	if err := json.NewDecoder(signup.Body).Decode(&session); err != nil {
		t.Fatal(err)
	}
	if session["role"] != "admin" || session["token"] == "" {
		t.Fatalf("unexpected signup response: %#v", session)
	}

	request := httptest.NewRequest(http.MethodGet, "/api/v1/auths/", nil)
	request.Header.Set("Authorization", "Bearer "+session["token"].(string))
	current := httptest.NewRecorder()
	app.ServeHTTP(current, request)
	if current.Code != http.StatusOK {
		t.Fatalf("session lookup failed: %d %s", current.Code, current.Body.String())
	}
}

func TestAuthenticatedChatLifecycle(t *testing.T) {
	app := testApp(t)
	token := signupToken(t, app)

	createRequest := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/chats/new",
		bytes.NewBufferString(`{"chat":{"title":"First chat","messages":[]}}`),
	)
	createRequest.Header.Set("Authorization", "Bearer "+token)
	created := httptest.NewRecorder()
	app.ServeHTTP(created, createRequest)
	if created.Code != http.StatusOK {
		t.Fatalf("create chat failed: %d %s", created.Code, created.Body.String())
	}
	var chat map[string]any
	if err := json.NewDecoder(created.Body).Decode(&chat); err != nil {
		t.Fatal(err)
	}
	if chat["title"] != "First chat" || chat["id"] == "" {
		t.Fatalf("unexpected chat: %#v", chat)
	}

	listRequest := httptest.NewRequest(http.MethodGet, "/api/v1/chats/", nil)
	listRequest.Header.Set("Authorization", "Bearer "+token)
	listed := httptest.NewRecorder()
	app.ServeHTTP(listed, listRequest)
	if listed.Code != http.StatusOK {
		t.Fatalf("list chats failed: %d %s", listed.Code, listed.Body.String())
	}
	var chats []map[string]any
	if err := json.NewDecoder(listed.Body).Decode(&chats); err != nil {
		t.Fatal(err)
	}
	if len(chats) != 1 || chats[0]["id"] != chat["id"] {
		t.Fatalf("unexpected chat list: %#v", chats)
	}
}

func TestExtendedChatLifecycle(t *testing.T) {
	app := testApp(t)
	token := signupToken(t, app)
	created := authenticatedJSON(t, app, token, http.MethodPost, "/api/v1/chats/new", `{"chat":{"title":"Roadmap","messages":[]}}`)
	chatID, _ := created["id"].(string)
	if chatID == "" {
		t.Fatalf("chat id missing: %#v", created)
	}

	search := authenticatedRequest(t, app, token, http.MethodGet, "/api/v1/chats/search?text=road", "")
	var searchResults []map[string]any
	if err := json.NewDecoder(search.Body).Decode(&searchResults); err != nil {
		t.Fatal(err)
	}
	if search.Code != http.StatusOK || len(searchResults) != 1 {
		t.Fatalf("unexpected search response: %d %s", search.Code, search.Body.String())
	}

	pinned := authenticatedJSON(t, app, token, http.MethodPost, "/api/v1/chats/"+chatID+"/pin", "")
	if pinned["pinned"] != true {
		t.Fatalf("chat was not pinned: %#v", pinned)
	}

	tags := authenticatedRequest(t, app, token, http.MethodPost, "/api/v1/chats/"+chatID+"/tags", `{"name":"Release Plan"}`)
	var tagList []map[string]any
	if err := json.NewDecoder(tags.Body).Decode(&tagList); err != nil {
		t.Fatal(err)
	}
	if tags.Code != http.StatusOK || len(tagList) != 1 || tagList[0]["id"] != "release_plan" {
		t.Fatalf("unexpected tag response: %d %s", tags.Code, tags.Body.String())
	}

	shared := authenticatedJSON(t, app, token, http.MethodPost, "/api/v1/chats/"+chatID+"/share", "")
	shareID, _ := shared["share_id"].(string)
	if shareID == "" {
		t.Fatalf("share id missing: %#v", shared)
	}
	shareResponse := authenticatedRequest(t, app, token, http.MethodGet, "/api/v1/chats/share/"+shareID, "")
	if shareResponse.Code != http.StatusOK {
		t.Fatalf("shared chat lookup failed: %d %s", shareResponse.Code, shareResponse.Body.String())
	}

	cloned := authenticatedJSON(t, app, token, http.MethodPost, "/api/v1/chats/"+chatID+"/clone", `{"title":"Roadmap copy"}`)
	if cloned["id"] == chatID || cloned["title"] != "Roadmap copy" {
		t.Fatalf("unexpected clone response: %#v", cloned)
	}
}

func authenticatedJSON(t *testing.T, app *App, token, method, path, body string) map[string]any {
	t.Helper()
	response := authenticatedRequest(t, app, token, method, path, body)
	if response.Code != http.StatusOK {
		t.Fatalf("%s %s failed: %d %s", method, path, response.Code, response.Body.String())
	}
	var payload map[string]any
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	return payload
}

func authenticatedRequest(t *testing.T, app *App, token, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	request.Header.Set("Authorization", "Bearer "+token)
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	response := httptest.NewRecorder()
	app.ServeHTTP(response, request)
	return response
}

func TestAPIKeyAuthentication(t *testing.T) {
	app := testApp(t)
	token := signupToken(t, app)
	createRequest := httptest.NewRequest(http.MethodPost, "/api/v1/auths/api_key", nil)
	createRequest.Header.Set("Authorization", "Bearer "+token)
	created := httptest.NewRecorder()
	app.ServeHTTP(created, createRequest)
	if created.Code != http.StatusOK {
		t.Fatalf("create API key failed: %d %s", created.Code, created.Body.String())
	}
	var payload map[string]string
	if err := json.NewDecoder(created.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if payload["api_key"] == "" {
		t.Fatal("API key is empty")
	}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/auths/", nil)
	request.Header.Set("Authorization", "Bearer "+payload["api_key"])
	response := httptest.NewRecorder()
	app.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("API key session failed: %d %s", response.Code, response.Body.String())
	}
}

func TestCompatibilityResourcePersists(t *testing.T) {
	app := testApp(t)
	token := signupToken(t, app)
	createRequest := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/folders/",
		bytes.NewBufferString(`{"name":"Work"}`),
	)
	createRequest.Header.Set("Authorization", "Bearer "+token)
	created := httptest.NewRecorder()
	app.ServeHTTP(created, createRequest)
	if created.Code != http.StatusOK {
		t.Fatalf("create folder failed: %d %s", created.Code, created.Body.String())
	}
	listRequest := httptest.NewRequest(http.MethodGet, "/api/v1/folders/", nil)
	listRequest.Header.Set("Authorization", "Bearer "+token)
	listed := httptest.NewRecorder()
	app.ServeHTTP(listed, listRequest)
	if listed.Code != http.StatusOK {
		t.Fatalf("list folders failed: %d %s", listed.Code, listed.Body.String())
	}
	var folders []map[string]any
	if err := json.NewDecoder(listed.Body).Decode(&folders); err != nil {
		t.Fatal(err)
	}
	if len(folders) != 1 || folders[0]["name"] != "Work" {
		t.Fatalf("unexpected folders: %#v", folders)
	}
}

func TestAdminDefaultSettingsPersist(t *testing.T) {
	app := testApp(t)
	adminToken := signupToken(t, app)

	defaults := authenticatedRequest(t, app, adminToken, http.MethodGet, "/api/v1/users/default/permissions", "")
	if defaults.Code != http.StatusOK || !strings.Contains(defaults.Body.String(), `"temporary_enforced":false`) {
		t.Fatalf("unexpected default permissions: %d %s", defaults.Code, defaults.Body.String())
	}

	updated := authenticatedRequest(t, app, adminToken, http.MethodPost, "/api/v1/users/default/settings", `{"enabled":true,"roles":["admin"],"ui":{"temporaryChatByDefault":true,"connections":{"openai":{"OPENAI_API_KEYS":["secret"]}}},"tools":{"native_tools":{"TOOL_CALLING_MODE":"native"}}}`)
	if updated.Code != http.StatusOK {
		t.Fatalf("update default settings failed: %d %s", updated.Code, updated.Body.String())
	}
	var saved map[string]any
	if err := json.NewDecoder(updated.Body).Decode(&saved); err != nil {
		t.Fatal(err)
	}
	if saved["configured"] != true || saved["enabled"] != true {
		t.Fatalf("unexpected saved defaults: %#v", saved)
	}
	roles, _ := saved["roles"].([]any)
	ui, _ := saved["ui"].(map[string]any)
	if len(roles) != 2 || ui["connections"] != nil {
		t.Fatalf("unsafe new-user defaults were not sanitized: %#v", saved)
	}

	loaded := authenticatedRequest(t, app, adminToken, http.MethodGet, "/api/v1/users/default/settings", "")
	if loaded.Code != http.StatusOK || !strings.Contains(loaded.Body.String(), `"temporaryChatByDefault":true`) {
		t.Fatalf("default settings were not persisted: %d %s", loaded.Code, loaded.Body.String())
	}

	nonAdminToken := signupTokenFor(t, app, "User", "user@example.com")
	userSettings := authenticatedRequest(t, app, nonAdminToken, http.MethodGet, "/api/v1/users/user/settings", "")
	if userSettings.Code != http.StatusOK || !strings.Contains(userSettings.Body.String(), `"temporaryChatByDefault":true`) {
		t.Fatalf("new user did not receive defaults: %d %s", userSettings.Code, userSettings.Body.String())
	}
	forbidden := authenticatedRequest(t, app, nonAdminToken, http.MethodGet, "/api/v1/users/default/settings", "")
	if forbidden.Code != http.StatusForbidden {
		t.Fatalf("non-admin accessed default settings: %d %s", forbidden.Code, forbidden.Body.String())
	}
}

func TestAdminBulkModelUpdate(t *testing.T) {
	app := testApp(t)
	adminToken := signupToken(t, app)

	created := authenticatedRequest(t, app, adminToken, http.MethodPost, "/api/v1/models/bulk/update", `{"items":[{"id":"gpt-4o","name":"GPT-4o"},{"id":"gpt-4o"},{"id":""}],"patch":{"is_active":false,"meta":{"group":"cloud"}}}`)
	if created.Code != http.StatusOK {
		t.Fatalf("bulk create failed: %d %s", created.Code, created.Body.String())
	}
	var result map[string]int
	if err := json.NewDecoder(created.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if result["created"] != 1 || result["updated"] != 0 || result["skipped"] != 2 {
		t.Fatalf("unexpected bulk create result: %#v", result)
	}

	updated := authenticatedRequest(t, app, adminToken, http.MethodPost, "/api/v1/models/bulk/update", `{"items":[{"id":"gpt-4o"}],"patch":{"is_active":true,"meta":{"tier":"fast"},"access_control":{"read":{"group_ids":["staff"]}}}}`)
	if updated.Code != http.StatusOK {
		t.Fatalf("bulk update failed: %d %s", updated.Code, updated.Body.String())
	}
	model := authenticatedJSON(t, app, adminToken, http.MethodGet, "/api/v1/models/model?id=gpt-4o", "")
	meta, _ := model["meta"].(map[string]any)
	if model["is_active"] != true || meta["group"] != "cloud" || meta["tier"] != "fast" {
		t.Fatalf("bulk patch did not preserve shallow metadata: %#v", model)
	}

	nonAdminToken := signupTokenFor(t, app, "User", "user@example.com")
	forbidden := authenticatedRequest(t, app, nonAdminToken, http.MethodPost, "/api/v1/models/bulk/update", `{"items":[],"patch":{}}`)
	if forbidden.Code != http.StatusForbidden {
		t.Fatalf("non-admin performed bulk model update: %d %s", forbidden.Code, forbidden.Body.String())
	}
}

func TestFrontendAPIDomainsHaveGoOwners(t *testing.T) {
	owners := map[string]bool{
		"analytics": true, "auths": true, "channels": true, "chats": true, "configs": true,
		"files": true, "folders": true, "functions": true, "groups": true,
		"knowledge": true, "memories": true, "models": true, "notes": true,
		"prompts": true, "skills": true, "terminal": true, "tools": true,
		"users": true, "utils": true,
	}
	pattern := regexp.MustCompile(`WEBUI_API_BASE_URL\}/([a-zA-Z0-9_-]+)`)
	root := filepath.Join("..", "..", "..", "src", "lib", "apis")
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || filepath.Ext(path) != ".ts" {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, match := range pattern.FindAllSubmatch(content, -1) {
			domain := string(match[1])
			if !owners[domain] {
				t.Errorf("frontend API domain %q in %s has no Go owner", domain, path)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestTerminalWorkspaceIsSandboxed(t *testing.T) {
	app := testApp(t)
	app.config.EnableTerminal = true
	token := signupToken(t, app)

	mkdir := authenticatedRequest(t, app, token, http.MethodPost, "/api/v1/terminal/files/mkdir", `{"path":"notes"}`)
	if mkdir.Code != http.StatusOK {
		t.Fatalf("mkdir failed: %d %s", mkdir.Code, mkdir.Body.String())
	}
	write := authenticatedRequest(t, app, token, http.MethodPost, "/api/v1/terminal/files/content", `{"path":"notes/todo.txt","content":"ship Go"}`)
	if write.Code != http.StatusOK {
		t.Fatalf("write failed: %d %s", write.Code, write.Body.String())
	}
	read := authenticatedRequest(t, app, token, http.MethodGet, "/api/v1/terminal/files/content?path=notes%2Ftodo.txt", "")
	if read.Code != http.StatusOK || !strings.Contains(read.Body.String(), "ship Go") {
		t.Fatalf("read failed: %d %s", read.Code, read.Body.String())
	}
	traversal := authenticatedRequest(t, app, token, http.MethodGet, "/api/v1/terminal/files/content?path=..%2Fsecret", "")
	if traversal.Code != http.StatusForbidden {
		t.Fatalf("path traversal was not rejected: %d %s", traversal.Code, traversal.Body.String())
	}
}

func signupToken(t *testing.T, app *App) string {
	t.Helper()
	response := httptest.NewRecorder()
	app.ServeHTTP(
		response,
		httptest.NewRequest(
			http.MethodPost,
			"/api/v1/auths/signup",
			bytes.NewBufferString(`{"name":"Admin","email":"admin@example.com","password":"secret"}`),
		),
	)
	if response.Code != http.StatusOK {
		t.Fatalf("signup failed: %d %s", response.Code, response.Body.String())
	}
	var session map[string]any
	if err := json.NewDecoder(response.Body).Decode(&session); err != nil {
		t.Fatal(err)
	}
	return session["token"].(string)
}

func signupTokenFor(t *testing.T, app *App, name, email string) string {
	t.Helper()
	response := httptest.NewRecorder()
	body, _ := json.Marshal(map[string]string{"name": name, "email": email, "password": "secret"})
	app.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/v1/auths/signup", bytes.NewReader(body)))
	if response.Code != http.StatusOK {
		t.Fatalf("signup failed: %d %s", response.Code, response.Body.String())
	}
	var session map[string]any
	if err := json.NewDecoder(response.Body).Decode(&session); err != nil {
		t.Fatal(err)
	}
	return session["token"].(string)
}
