package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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
