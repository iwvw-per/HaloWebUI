package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/iwvw-per/HaloWebUI/backend/internal/store"
)

type stubLDAPAuthenticator struct {
	identity ldapIdentity
	err      error
	calls    int
	config   ldapServerConfig
	username string
	password string
}

type stubYouTubeTranscriptLoader struct {
	transcript string
	language   string
	err        error
	videoID    string
	config     map[string]any
}

func (s *stubYouTubeTranscriptLoader) Load(_ context.Context, videoID string, config map[string]any) (string, string, error) {
	s.videoID = videoID
	s.config = config
	return s.transcript, s.language, s.err
}

func (s *stubLDAPAuthenticator) Authenticate(_ context.Context, config ldapServerConfig, username, password string) (ldapIdentity, error) {
	s.calls++
	s.config = config
	s.username = username
	s.password = password
	return s.identity, s.err
}

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

func TestLDAPConfigurationAndSigninContract(t *testing.T) {
	app := testApp(t)
	adminToken := signupToken(t, app)
	memberToken := signupTokenFor(t, app, "Member", "member@example.com")

	forbidden := authenticatedRequest(t, app, memberToken, http.MethodGet, "/api/v1/auths/admin/config/ldap", "")
	if forbidden.Code != http.StatusForbidden {
		t.Fatalf("non-admin read LDAP config: %d %s", forbidden.Code, forbidden.Body.String())
	}

	serverConfig := `{
		"label":"Corporate LDAP","host":"ldap.example.com","port":636,
		"attribute_for_mail":"mail","attribute_for_username":"uid",
		"app_dn":"cn=reader,dc=example,dc=com","app_dn_password":"reader-secret",
		"search_base":"ou=people,dc=example,dc=com","search_filters":"(objectClass=person)",
		"use_tls":true,"certificate_path":null,"ciphers":"ALL"
	}`
	configured := authenticatedJSON(t, app, adminToken, http.MethodPost, "/api/v1/auths/admin/config/ldap/server", serverConfig)
	if configured["host"] != "ldap.example.com" || configured["app_dn_password"] != "reader-secret" {
		t.Fatalf("LDAP server config did not persist: %#v", configured)
	}
	enabled := authenticatedJSON(t, app, adminToken, http.MethodPost, "/api/v1/auths/admin/config/ldap", `{"enable_ldap":true}`)
	if enabled["ENABLE_LDAP"] != true {
		t.Fatalf("LDAP was not enabled: %#v", enabled)
	}

	publicConfig := httptest.NewRecorder()
	app.ServeHTTP(publicConfig, httptest.NewRequest(http.MethodGet, "/api/config", nil))
	if publicConfig.Code != http.StatusOK || !strings.Contains(publicConfig.Body.String(), `"enable_ldap":true`) {
		t.Fatalf("public config did not expose LDAP state: %d %s", publicConfig.Code, publicConfig.Body.String())
	}

	stub := &stubLDAPAuthenticator{identity: ldapIdentity{Username: "alice", Email: "alice@example.com", Name: "Alice LDAP"}}
	app.ldapAuth = stub
	signin := httptest.NewRecorder()
	signinRequest := httptest.NewRequest(http.MethodPost, "/api/v1/auths/ldap", strings.NewReader(`{"user":"Alice","password":"user-secret"}`))
	signinRequest.Header.Set("Content-Type", "application/json")
	app.ServeHTTP(signin, signinRequest)
	if signin.Code != http.StatusOK || !strings.Contains(signin.Body.String(), `"email":"alice@example.com"`) {
		t.Fatalf("LDAP signin failed: %d %s", signin.Code, signin.Body.String())
	}
	if stub.calls != 1 || stub.username != "Alice" || stub.password != "user-secret" || stub.config.AppDNPassword != "reader-secret" {
		t.Fatalf("LDAP adapter received the wrong contract: %#v", stub)
	}
	if cookie := signin.Header().Get("Set-Cookie"); !strings.Contains(cookie, "token=") || !strings.Contains(cookie, "HttpOnly") {
		t.Fatalf("LDAP signin did not issue a secure session cookie: %q", cookie)
	}
	created, err := app.store.UserByEmail(t.Context(), "alice@example.com")
	if err != nil || created.Name != "Alice LDAP" || created.Role != "pending" {
		t.Fatalf("LDAP user was not created with the default role: %#v %v", created, err)
	}

	stub.err = errors.New("bind tcp ldap.example.com:636: secret infrastructure detail")
	failed := httptest.NewRecorder()
	failedRequest := httptest.NewRequest(http.MethodPost, "/api/v1/auths/ldap", strings.NewReader(`{"user":"Alice","password":"wrong"}`))
	failedRequest.Header.Set("Content-Type", "application/json")
	app.ServeHTTP(failed, failedRequest)
	if failed.Code != http.StatusBadRequest || strings.Contains(failed.Body.String(), "infrastructure") {
		t.Fatalf("LDAP failure contract leaked details: %d %s", failed.Code, failed.Body.String())
	}
}

func TestLDAPSigninRejectsDisabledAndInvalidConfiguration(t *testing.T) {
	app := testApp(t)
	stub := &stubLDAPAuthenticator{}
	app.ldapAuth = stub
	disabled := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auths/ldap", strings.NewReader(`{"user":"alice","password":"secret"}`))
	request.Header.Set("Content-Type", "application/json")
	app.ServeHTTP(disabled, request)
	if disabled.Code != http.StatusBadRequest || stub.calls != 0 {
		t.Fatalf("disabled LDAP reached adapter: %d calls=%d", disabled.Code, stub.calls)
	}

	token := signupToken(t, app)
	invalid := authenticatedRequest(t, app, token, http.MethodPost, "/api/v1/auths/admin/config/ldap/server", `{"label":"LDAP","host":"localhost","attribute_for_mail":"mail)(uid=*","attribute_for_username":"uid","search_base":"dc=example,dc=com"}`)
	if invalid.Code != http.StatusBadRequest {
		t.Fatalf("unsafe LDAP attribute was accepted: %d %s", invalid.Code, invalid.Body.String())
	}
}

func TestCodeExecutionUsesPersistedConfig(t *testing.T) {
	root, err := jupyterRoot("http://jupyter.example.test:8558/lab?")
	if err != nil || root != "http://jupyter.example.test:8558" {
		t.Fatalf("Jupyter Lab URL normalization regressed: %q %v", root, err)
	}
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/kernels":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":"kernel-test"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/kernels/kernel-test/channels":
			connection, err := upgrader.Upgrade(w, r, nil)
			if err != nil {
				return
			}
			defer connection.Close()
			_, requestData, _ := connection.ReadMessage()
			var requestEnvelope struct {
				Header struct {
					MsgID string `json:"msg_id"`
				} `json:"header"`
			}
			_ = json.Unmarshal(requestData, &requestEnvelope)
			_ = connection.WriteJSON(map[string]any{"header": map[string]any{"msg_type": "stream"}, "parent_header": map[string]any{}, "content": map[string]any{"name": "stdout", "text": "persisted-config-ok\n"}})
			_ = connection.WriteJSON(map[string]any{"header": map[string]any{"msg_type": "status"}, "parent_header": map[string]any{"msg_id": requestEnvelope.Header.MsgID}, "content": map[string]any{"execution_state": "idle"}})
		case r.Method == http.MethodDelete && r.URL.Path == "/api/kernels/kernel-test":
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	}))
	defer upstream.Close()
	app := testApp(t)
	token := signupToken(t, app)
	authenticatedJSON(t, app, token, http.MethodPost, "/api/v1/configs/code_execution", `{"ENABLE_CODE_EXECUTION":true,"CODE_EXECUTION_ENGINE":"jupyter","CODE_EXECUTION_JUPYTER_URL":"`+upstream.URL+`/lab?workspace=test","CODE_EXECUTION_JUPYTER_AUTH":"token","CODE_EXECUTION_JUPYTER_AUTH_TOKEN":"persisted-token"}`)

	request := authenticatedRequest(t, app, token, http.MethodPost, "/api/v1/utils/code/execute", `{"code":"1+1"}`)
	if request.Code != http.StatusOK || !strings.Contains(request.Body.String(), "persisted-config-ok") {
		t.Fatalf("persisted code execution config was ignored: %d %s", request.Code, request.Body.String())
	}
	publicConfig := httptest.NewRecorder()
	app.ServeHTTP(publicConfig, httptest.NewRequest(http.MethodGet, "/api/config", nil))
	if publicConfig.Code != http.StatusOK || !strings.Contains(publicConfig.Body.String(), `"enable_code_execution":true`) || !strings.Contains(publicConfig.Body.String(), `"engine":"jupyter"`) {
		t.Fatalf("code execution state was not exposed: %d %s", publicConfig.Code, publicConfig.Body.String())
	}
}

func TestLiveJupyterCodeExecution(t *testing.T) {
	endpoint := strings.TrimSpace(os.Getenv("HALO_TEST_JUPYTER_URL"))
	token := strings.TrimSpace(os.Getenv("HALO_TEST_JUPYTER_TOKEN"))
	if endpoint == "" || token == "" {
		t.Skip("set HALO_TEST_JUPYTER_URL and HALO_TEST_JUPYTER_TOKEN to run the live adapter test")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	defer cancel()
	result, err := executeJupyterCode(ctx, endpoint, map[string]any{
		"CODE_EXECUTION_JUPYTER_AUTH":       "token",
		"CODE_EXECUTION_JUPYTER_AUTH_TOKEN": token,
	}, `print("HALO_LIVE_JUPYTER_OK", 6 * 7)`)
	if err != nil {
		t.Fatal(err)
	}
	stdout, _ := result["stdout"].(string)
	if !strings.Contains(stdout, "HALO_LIVE_JUPYTER_OK 42") {
		t.Fatalf("unexpected live Jupyter output: %#v", result)
	}
}

func TestTavilySearchIndexesResultsAndRejectsUnsupportedEngines(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search" || r.Method != http.MethodPost {
			t.Fatalf("unexpected Tavily request: %s %s", r.Method, r.URL.Path)
		}
		var request map[string]any
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil || request["api_key"] != "test-key" || request["query"] == "" {
			t.Fatalf("invalid Tavily request: %#v %v", request, err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results":[{"title":"Go result","url":"https://example.com/go","content":"Go uses small static binaries."}]}`))
	}))
	defer upstream.Close()
	app := testApp(t)
	token := signupToken(t, app)
	authenticatedJSON(t, app, token, http.MethodPost, "/api/v1/retrieval/config/update", `{"web":{"WEB_SEARCH_ENGINE":"tavily","TAVILY_API_KEY":"test-key","TAVILY_SEARCH_API_BASE_URL":"`+upstream.URL+`/search","TAVILY_SEARCH_API_FORCE_MODE":true,"WEB_SEARCH_RESULT_COUNT":1}}`)
	result := authenticatedJSON(t, app, token, http.MethodPost, "/api/v1/retrieval/process/web/search", `{"query":"golang","collection_name":"search-test"}`)
	if result["status"] != true || result["collection_name"] != "search-test" || result["loaded_count"] != float64(1) {
		t.Fatalf("Tavily search response contract failed: %#v", result)
	}
	query := authenticatedJSON(t, app, token, http.MethodPost, "/api/v1/retrieval/query/doc", `{"collection_name":"search-test","query":"static binaries","k":4}`)
	if !strings.Contains(fmt.Sprint(query["documents"]), "small static binaries") {
		t.Fatalf("Tavily result was not indexed for retrieval: %#v", query)
	}
	verified := authenticatedJSON(t, app, token, http.MethodPost, "/api/v1/retrieval/config/web/verify", `{"WEB_SEARCH_ENGINE":"tavily","TAVILY_API_KEY":"test-key","TAVILY_SEARCH_API_BASE_URL":"`+upstream.URL+`/search","TAVILY_SEARCH_API_FORCE_MODE":true,"WEB_SEARCH_RESULT_COUNT":1}`)
	searchStatus, _ := verified["search"].(map[string]any)
	if searchStatus["enabled"] != true || searchStatus["ok"] != true {
		t.Fatalf("Tavily verification did not use the adapter: %#v", verified)
	}
	unsupported := authenticatedRequest(t, app, token, http.MethodPost, "/api/v1/retrieval/config/update", `{"web":{"WEB_SEARCH_ENGINE":"brave"}}`)
	if unsupported.Code != http.StatusBadRequest {
		t.Fatalf("unsupported web search engine was accepted: %d %s", unsupported.Code, unsupported.Body.String())
	}
}

func TestYouTubeProcessingIndexesTranscriptThroughLoader(t *testing.T) {
	app := testApp(t)
	token := signupToken(t, app)
	loader := &stubYouTubeTranscriptLoader{transcript: "hello from captions", language: "en"}
	app.youtubeLoader = loader
	result := authenticatedJSON(t, app, token, http.MethodPost, "/api/v1/retrieval/process/youtube", `{"url":"https://www.youtube.com/watch?v=dQw4w9WgXcQ","collection_name":"youtube-test"}`)
	if result["status"] != true || result["collection_name"] != "youtube-test" || !strings.Contains(fmt.Sprint(result["file"]), "hello from captions") {
		t.Fatalf("YouTube response contract failed: %#v", result)
	}
	if loader.videoID != "dQw4w9WgXcQ" {
		t.Fatalf("YouTube video ID was not normalized: %q", loader.videoID)
	}
	query := authenticatedJSON(t, app, token, http.MethodPost, "/api/v1/retrieval/query/doc", `{"collection_name":"youtube-test","query":"captions","k":4}`)
	if !strings.Contains(fmt.Sprint(query["documents"]), "hello from captions") {
		t.Fatalf("YouTube transcript was not indexed: %#v", query)
	}
	invalid := authenticatedRequest(t, app, token, http.MethodPost, "/api/v1/retrieval/process/youtube", `{"url":"https://example.com/watch?v=dQw4w9WgXcQ"}`)
	if invalid.Code != http.StatusBadRequest {
		t.Fatalf("non-YouTube URL was accepted: %d %s", invalid.Code, invalid.Body.String())
	}
}

func TestYouTubeTranscriptParsers(t *testing.T) {
	json3 := []byte(`{"events":[{"segs":[{"utf8":"Hello "},{"utf8":"world"}]},{"segs":[{"utf8":"!"}]}]}`)
	parsed, err := parseYouTubeTranscript(json3)
	if err != nil || parsed != "Hello world !" {
		t.Fatalf("json3 transcript parser failed: %q %v", parsed, err)
	}
	xmlTranscript, err := parseYouTubeTranscript([]byte(`<transcript><text>Hello &amp; world</text><text>Second line</text></transcript>`))
	if err != nil || xmlTranscript != "Hello & world Second line" {
		t.Fatalf("XML transcript parser failed: %q %v", xmlTranscript, err)
	}
}

func TestRootSystemContractsPersistAndRequireAdmin(t *testing.T) {
	app := testApp(t)
	adminToken := signupToken(t, app)
	userToken := signupTokenFor(t, app, "Member", "member@example.com")

	changelog := httptest.NewRecorder()
	app.ServeHTTP(changelog, httptest.NewRequest(http.MethodGet, "/api/changelog", nil))
	if changelog.Code != http.StatusOK || !strings.Contains(changelog.Body.String(), "0.0.1") {
		t.Fatalf("unexpected changelog response: %d %s", changelog.Code, changelog.Body.String())
	}

	updates := authenticatedJSON(t, app, adminToken, http.MethodGet, "/api/version/updates", "")
	if updates["current"] != "1.2.3" || updates["latest"] != "1.2.3" {
		t.Fatalf("unexpected update response: %#v", updates)
	}

	forbidden := authenticatedRequest(t, app, userToken, http.MethodGet, "/api/webhook", "")
	if forbidden.Code != http.StatusForbidden {
		t.Fatalf("non-admin loaded webhook config: %d %s", forbidden.Code, forbidden.Body.String())
	}

	webhook := authenticatedJSON(t, app, adminToken, http.MethodPost, "/api/webhook", `{"url":"https://hooks.example.test/halo"}`)
	if webhook["url"] != "https://hooks.example.test/halo" {
		t.Fatalf("webhook was not saved: %#v", webhook)
	}
	loadedWebhook := authenticatedJSON(t, app, adminToken, http.MethodGet, "/api/webhook", "")
	if loadedWebhook["url"] != webhook["url"] {
		t.Fatalf("webhook was not persisted: %#v", loadedWebhook)
	}

	filter := authenticatedJSON(t, app, adminToken, http.MethodPost, "/api/config/model/filter", `{"enabled":true,"models":["model-a"]}`)
	models, _ := filter["models"].([]any)
	if filter["enabled"] != true || len(models) != 1 || models[0] != "model-a" {
		t.Fatalf("model filter was not saved: %#v", filter)
	}

	modelConfig := authenticatedJSON(t, app, adminToken, http.MethodPost, "/api/config/models", `{"models":[{"id":"model-a"}]}`)
	configured, _ := modelConfig["models"].([]any)
	if len(configured) != 1 {
		t.Fatalf("global model config was not saved: %#v", modelConfig)
	}

	community := authenticatedJSON(t, app, adminToken, http.MethodGet, "/api/community_sharing/toggle", "")
	if community["enabled"] != true {
		t.Fatalf("community sharing was not toggled: %#v", community)
	}
	community = authenticatedJSON(t, app, adminToken, http.MethodGet, "/api/community_sharing", "")
	if community["enabled"] != true {
		t.Fatalf("community sharing was not persisted: %#v", community)
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

func TestAudioConfigContractAndPersistence(t *testing.T) {
	app := testApp(t)
	adminToken := signupToken(t, app)

	config := authenticatedJSON(t, app, adminToken, http.MethodGet, "/api/v1/audio/config", "")
	stt, _ := config["stt"].(map[string]any)
	tts, _ := config["tts"].(map[string]any)
	if stt["ENGINE"] != "openai" || tts["ENGINE"] != "" || tts["SPLIT_ON"] != "punctuation" {
		t.Fatalf("unexpected lightweight audio defaults: %#v", config)
	}

	updated := authenticatedJSON(t, app, adminToken, http.MethodPost, "/api/v1/audio/config/update", `{
		"tts":{"OPENAI_API_BASE_URL":"https://example.test/v1","OPENAI_API_KEY":"tts-secret","API_KEY":"","ENGINE":"openai","MODEL":"tts-1","VOICE":"alloy","SPLIT_ON":"sentence","AZURE_SPEECH_REGION":"","AZURE_SPEECH_OUTPUT_FORMAT":""},
		"stt":{"OPENAI_API_BASE_URL":"https://example.test/v1","OPENAI_API_KEY":"stt-secret","ENGINE":"openai","MODEL":"whisper-1","WHISPER_MODEL":"","DEEPGRAM_API_KEY":"","AZURE_API_KEY":"","AZURE_REGION":"","AZURE_LOCALES":""}
	}`)
	updatedTTS, _ := updated["tts"].(map[string]any)
	if updatedTTS["VOICE"] != "alloy" {
		t.Fatalf("audio update response is incomplete: %#v", updated)
	}

	reloaded := authenticatedJSON(t, app, adminToken, http.MethodGet, "/api/v1/audio/config", "")
	reloadedTTS, _ := reloaded["tts"].(map[string]any)
	if reloadedTTS["OPENAI_API_BASE_URL"] != "https://example.test/v1" || reloadedTTS["OPENAI_API_KEY"] != "tts-secret" {
		t.Fatalf("audio config was not persisted: %#v", reloaded)
	}

	backendConfig := authenticatedJSON(t, app, adminToken, http.MethodGet, "/api/config", "")
	audio, _ := backendConfig["audio"].(map[string]any)
	publicTTS, _ := audio["tts"].(map[string]any)
	publicSTT, _ := audio["stt"].(map[string]any)
	if publicTTS["engine"] != "openai" || publicTTS["voice"] != "alloy" || publicSTT["engine"] != "openai" {
		t.Fatalf("backend config did not expose audio capabilities: %#v", backendConfig)
	}

	models := authenticatedJSON(t, app, adminToken, http.MethodGet, "/api/v1/audio/models", "")
	voices := authenticatedJSON(t, app, adminToken, http.MethodGet, "/api/v1/audio/voices", "")
	if len(models["models"].([]any)) != 2 || len(voices["voices"].([]any)) != 6 {
		t.Fatalf("audio discovery contract is incomplete: models=%#v voices=%#v", models, voices)
	}
}

func TestAudioConfigRequiresAdmin(t *testing.T) {
	app := testApp(t)
	signupToken(t, app)
	userToken := signupTokenFor(t, app, "Audio User", "audio-user@example.com")

	response := authenticatedRequest(t, app, userToken, http.MethodGet, "/api/v1/audio/config", "")
	if response.Code != http.StatusForbidden {
		t.Fatalf("non-admin accessed global audio config: %d %s", response.Code, response.Body.String())
	}
}

func TestUnifiedChatTranslatesHaloEnvelope(t *testing.T) {
	var upstreamBody map[string]any
	var upstreamAuthorization string
	upstream := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		upstreamAuthorization = request.Header.Get("Authorization")
		if err := json.NewDecoder(request.Body).Decode(&upstreamBody); err != nil {
			t.Errorf("upstream received invalid JSON: %v", err)
		}
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(`{"id":"chatcmpl-test","choices":[{"message":{"role":"assistant","content":"GO_UI_CHAT_OK"}}]}`))
	}))
	defer upstream.Close()

	app := testApp(t)
	app.config.OpenAIBaseURL = "http://127.0.0.1:1"
	app.config.OpenAIAPIKey = "wrong-deployment-key"
	token := signupToken(t, app)
	settings := authenticatedJSON(t, app, token, http.MethodPost, "/api/v1/users/user/settings/update", `{
		"ui":{"connections":{"openai":{
			"OPENAI_API_BASE_URLS":["`+upstream.URL+`"],
			"OPENAI_API_KEYS":["legacy-user-key"],
			"OPENAI_API_CONFIGS":{"0":{"enable":true,"api_key_pool":{"keys":[{"key":"pooled-user-key","enabled":true}]}}}
		}}}
	}`)
	if settings["revision"] != float64(1) {
		t.Fatalf("failed to save user connection: %#v", settings)
	}
	secondPatch := authenticatedJSON(t, app, token, http.MethodPost, "/api/v1/users/user/settings/update", `{"ui":{"models":["gemini-3.1-flash-lite"]},"revision":1}`)
	ui, _ := secondPatch["ui"].(map[string]any)
	if secondPatch["revision"] != float64(2) || ui["connections"] == nil || ui["models"] == nil {
		t.Fatalf("settings patch overwrote sibling fields: %#v", secondPatch)
	}
	response := authenticatedRequest(t, app, token, http.MethodPost, "/api/chat/completions", `{
		"model":"gemini-3.1-flash-lite",
		"messages":[{"role":"user","content":"hello"}],
		"stream":false,
		"params":{"temperature":0.2,"max_tokens":32},
		"files":[],"chat_id":"ui-only","background_tasks":{"title_generation":true}
	}`)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "GO_UI_CHAT_OK") {
		t.Fatalf("unified chat failed: %d %s", response.Code, response.Body.String())
	}
	if upstreamBody["temperature"] != 0.2 || upstreamBody["max_tokens"] != float64(32) {
		t.Fatalf("nested params were not flattened: %#v", upstreamBody)
	}
	if _, ok := upstreamBody["params"]; ok {
		t.Fatalf("internal params leaked upstream: %#v", upstreamBody)
	}
	if _, ok := upstreamBody["chat_id"]; ok {
		t.Fatalf("internal chat id leaked upstream: %#v", upstreamBody)
	}
	if upstreamAuthorization != "Bearer pooled-user-key" {
		t.Fatalf("chat did not use the enabled user connection key: %q", upstreamAuthorization)
	}
}

func TestOpenAICompatibilityConfigPersistsPerUser(t *testing.T) {
	app := testApp(t)
	token := signupToken(t, app)
	payload := `{"ENABLE_OPENAI_API":true,"OPENAI_API_BASE_URLS":["https://api.example.test/v1"],"OPENAI_API_KEYS":["user-key"],"OPENAI_API_CONFIGS":{"0":{"enable":true}}}`
	saved := authenticatedRequest(t, app, token, http.MethodPost, "/openai/config/update", payload)
	if saved.Code != http.StatusOK {
		t.Fatalf("OpenAI config update failed: %d %s", saved.Code, saved.Body.String())
	}
	loaded := authenticatedRequest(t, app, token, http.MethodGet, "/openai/config", "")
	if loaded.Code != http.StatusOK || !strings.Contains(loaded.Body.String(), "api.example.test") || !strings.Contains(loaded.Body.String(), "user-key") {
		t.Fatalf("OpenAI config was not persisted: %d %s", loaded.Code, loaded.Body.String())
	}
	urls := authenticatedRequest(t, app, token, http.MethodGet, "/openai/urls", "")
	if urls.Code != http.StatusOK || !strings.Contains(urls.Body.String(), "api.example.test") {
		t.Fatalf("OpenAI URLs did not reflect persisted config: %d %s", urls.Code, urls.Body.String())
	}
}

func TestOllamaCompatibilityConfigPersistsAndUsesSelectedConnection(t *testing.T) {
	var requestedPath string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPath = r.URL.Path
		writeJSON(w, http.StatusOK, map[string]any{"models": []map[string]string{{"name": "go-ollama"}}})
	}))
	defer upstream.Close()

	app := testApp(t)
	app.config.OllamaBaseURL = "http://127.0.0.1:1"
	token := signupToken(t, app)
	payload := `{"ENABLE_OLLAMA_API":true,"OLLAMA_BASE_URLS":["http://127.0.0.1:1","` + upstream.URL + `"],"OLLAMA_API_CONFIGS":{"0":{"enable":false},"1":{"enable":true}}}`
	saved := authenticatedRequest(t, app, token, http.MethodPost, "/ollama/config/update", payload)
	if saved.Code != http.StatusOK {
		t.Fatalf("Ollama config update failed: %d %s", saved.Code, saved.Body.String())
	}
	loaded := authenticatedRequest(t, app, token, http.MethodGet, "/ollama/config", "")
	if loaded.Code != http.StatusOK || !strings.Contains(loaded.Body.String(), upstream.URL) {
		t.Fatalf("Ollama config was not persisted: %d %s", loaded.Code, loaded.Body.String())
	}
	models := authenticatedRequest(t, app, token, http.MethodGet, "/ollama/api/tags/1", "")
	if models.Code != http.StatusOK || !strings.Contains(models.Body.String(), "go-ollama") || requestedPath != "/api/tags" {
		t.Fatalf("indexed Ollama connection failed: %d %s path=%s", models.Code, models.Body.String(), requestedPath)
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

func TestEmptyChatCollectionsAreJSONArrays(t *testing.T) {
	app := testApp(t)
	token := signupToken(t, app)
	for _, path := range []string{"/api/v1/chats/?page=1", "/api/v1/chats/pinned", "/api/v1/chats/all", "/api/v1/chats/archived"} {
		response := authenticatedRequest(t, app, token, http.MethodGet, path, "")
		if response.Code != http.StatusOK {
			t.Fatalf("%s failed: %d %s", path, response.Code, response.Body.String())
		}
		var chats []map[string]any
		if err := json.NewDecoder(response.Body).Decode(&chats); err != nil {
			t.Fatalf("%s returned invalid collection: %v", path, err)
		}
		if chats == nil {
			t.Fatalf("%s returned JSON null instead of []", path)
		}
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

func TestTypedWorkspaceResourceLifecycles(t *testing.T) {
	app := testApp(t)
	token := signupToken(t, app)
	for _, resource := range []struct {
		name   string
		prefix string
		body   string
	}{
		{name: "prompt", prefix: "/api/v1/prompts", body: `{"command":"go-audit","title":"Go audit","content":"check"}`},
		{name: "tool", prefix: "/api/v1/tools", body: `{"name":"go-audit-tool","content":"{}"}`},
		{name: "skill", prefix: "/api/v1/skills", body: `{"name":"go-audit-skill","content":"steps"}`},
		{name: "note", prefix: "/api/v1/notes", body: `{"title":"Go audit note","data":{"content":"draft"}}`},
	} {
		t.Run(resource.name, func(t *testing.T) {
			created := authenticatedJSON(t, app, token, http.MethodPost, resource.prefix+"/create", resource.body)
			id, _ := created["id"].(string)
			if id == "" || created["is_active"] != true {
				t.Fatalf("create response is incomplete: %#v", created)
			}
			loaded := authenticatedJSON(t, app, token, http.MethodGet, resource.prefix+"/id/"+id, "")
			if loaded["id"] != id {
				t.Fatalf("resource lookup failed: %#v", loaded)
			}
			updated := authenticatedJSON(t, app, token, http.MethodPost, resource.prefix+"/id/"+id+"/update", `{"description":"updated by Go"}`)
			if updated["description"] != "updated by Go" {
				t.Fatalf("resource update failed: %#v", updated)
			}
			toggled := authenticatedJSON(t, app, token, http.MethodPost, resource.prefix+"/id/"+id+"/toggle", "")
			if toggled["is_active"] != false {
				t.Fatalf("resource toggle failed: %#v", toggled)
			}
			deleted := authenticatedRequest(t, app, token, http.MethodDelete, resource.prefix+"/id/"+id+"/delete", "")
			if deleted.Code != http.StatusOK || strings.TrimSpace(deleted.Body.String()) != "true" {
				t.Fatalf("resource delete failed: %d %s", deleted.Code, deleted.Body.String())
			}
		})
	}
}

func TestWorkspaceModelLifecycle(t *testing.T) {
	app := testApp(t)
	token := signupToken(t, app)
	created := authenticatedJSON(t, app, token, http.MethodPost, "/api/v1/models/create", `{"id":"go-e2e-model","name":"Go E2E Model","params":{},"meta":{}}`)
	if created["id"] != "go-e2e-model" || created["is_active"] != true {
		t.Fatalf("model create failed: %#v", created)
	}
	updated := authenticatedJSON(t, app, token, http.MethodPost, "/api/v1/models/model/update?id=go-e2e-model", `{"name":"Go E2E Updated"}`)
	if updated["name"] != "Go E2E Updated" {
		t.Fatalf("model update failed: %#v", updated)
	}
	toggled := authenticatedJSON(t, app, token, http.MethodPost, "/api/v1/models/model/toggle?id=go-e2e-model", "")
	if toggled["is_active"] != false {
		t.Fatalf("model toggle failed: %#v", toggled)
	}
	deleted := authenticatedJSON(t, app, token, http.MethodDelete, "/api/v1/models/model/delete?id=go-e2e-model", "")
	if deleted["status"] != true {
		t.Fatalf("model delete failed: %#v", deleted)
	}
}

func TestFileUploadReadUpdateDeleteLifecycle(t *testing.T) {
	app := testApp(t)
	app.config.FileMaxSizeBytes = 1 << 20
	token := signupToken(t, app)
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "audit.txt")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = part.Write([]byte("Go file backend works"))
	_ = writer.Close()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/files/", &body)
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	response := httptest.NewRecorder()
	app.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("file upload failed: %d %s", response.Code, response.Body.String())
	}
	var uploaded map[string]any
	if err := json.NewDecoder(response.Body).Decode(&uploaded); err != nil {
		t.Fatal(err)
	}
	id, _ := uploaded["id"].(string)
	content := authenticatedRequest(t, app, token, http.MethodGet, "/api/v1/files/"+id+"/content", "")
	if content.Code != http.StatusOK || content.Body.String() != "Go file backend works" {
		t.Fatalf("file content failed: %d %q", content.Code, content.Body.String())
	}
	updated := authenticatedJSON(t, app, token, http.MethodPost, "/api/v1/files/"+id+"/data/content/update", `{"content":"indexed"}`)
	data, _ := updated["data"].(map[string]any)
	if data["content"] != "indexed" {
		t.Fatalf("file data update failed: %#v", updated)
	}
	deleted := authenticatedJSON(t, app, token, http.MethodDelete, "/api/v1/files/"+id, "")
	if deleted["status"] != true {
		t.Fatalf("file delete failed: %#v", deleted)
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

func TestAdminOptionalDomainContracts(t *testing.T) {
	app := testApp(t)
	token := signupToken(t, app)

	for _, path := range []string{
		"/api/v1/analytics/models?days=30",
		"/api/v1/analytics/users?days=30",
		"/api/v1/analytics/daily?days=30",
		"/api/v1/haloclaw/gateways",
		"/api/v1/external_api/clients",
		"/api/v1/external_api/logs",
	} {
		response := authenticatedRequest(t, app, token, http.MethodGet, path, "")
		if response.Code != http.StatusOK || !strings.HasPrefix(response.Body.String(), "[") {
			t.Fatalf("%s did not return a collection: %d %s", path, response.Code, response.Body.String())
		}
	}

	haloConfig := authenticatedJSON(t, app, token, http.MethodGet, "/api/v1/haloclaw/config", "")
	if haloConfig["enabled"] != false || haloConfig["max_history"] != float64(20) {
		t.Fatalf("unexpected HaloClaw config: %#v", haloConfig)
	}
	externalConfig := authenticatedJSON(t, app, token, http.MethodGet, "/api/v1/external_api/config", "")
	if externalConfig["enabled"] != false || externalConfig["default_rpm_limit"] != float64(60) {
		t.Fatalf("unexpected external API config: %#v", externalConfig)
	}

	gateway := authenticatedJSON(t, app, token, http.MethodPost, "/api/v1/haloclaw/gateways", `{"name":"Local Telegram","platform":"telegram","enabled":false}`)
	gatewayID, _ := gateway["id"].(string)
	if gatewayID == "" {
		t.Fatalf("gateway id missing: %#v", gateway)
	}
	toggled := authenticatedJSON(t, app, token, http.MethodPost, "/api/v1/haloclaw/gateways/"+gatewayID+"/toggle", `{"enabled":true}`)
	if toggled["enabled"] != true {
		t.Fatalf("gateway was not toggled: %#v", toggled)
	}

	clientResponse := authenticatedJSON(t, app, token, http.MethodPost, "/api/v1/external_api/clients", `{"name":"Local client","allowed_model_ids":["gemini-3.1-flash-lite"]}`)
	client, _ := clientResponse["client"].(map[string]any)
	clientID, _ := client["id"].(string)
	if clientID == "" || clientResponse["api_key"] == "" {
		t.Fatalf("external client response is incomplete: %#v", clientResponse)
	}
	deleted := authenticatedRequest(t, app, token, http.MethodDelete, "/api/v1/external_api/clients/"+clientID, "")
	if deleted.Code != http.StatusOK || strings.TrimSpace(deleted.Body.String()) != "true" {
		t.Fatalf("external client delete failed: %d %s", deleted.Code, deleted.Body.String())
	}
}

func TestHaloClawExternalUsersAndLogsPersist(t *testing.T) {
	app := testApp(t)
	token := signupToken(t, app)
	gateway := authenticatedJSON(t, app, token, http.MethodPost, "/api/v1/haloclaw/gateways", `{"name":"Gateway","platform":"telegram","enabled":false}`)
	gatewayID, _ := gateway["id"].(string)
	if gatewayID == "" {
		t.Fatalf("gateway id missing: %#v", gateway)
	}

	externalUserID := "external-user-1"
	userBody, _ := json.Marshal(map[string]any{
		"gateway_id": gatewayID, "platform": "telegram", "platform_user_id": "42",
		"platform_username": "go-user", "model_override": "model-a", "is_blocked": false,
	})
	if _, err := app.store.PutResource(t.Context(), store.Resource{
		Kind: haloClawExternalUserKind, ID: externalUserID, UserID: "system",
		Key: gatewayID + ":telegram:42", Body: userBody, Active: true,
	}); err != nil {
		t.Fatal(err)
	}
	logBody, _ := json.Marshal(map[string]any{
		"gateway_id": gatewayID, "external_user_id": externalUserID,
		"platform_chat_id": "chat-1", "direction": "inbound", "role": "user", "content": "hello",
	})
	if _, err := app.store.PutResource(t.Context(), store.Resource{
		Kind: haloClawMessageLogKind, ID: "message-1", UserID: "system",
		Key: "message-1", Body: logBody, Active: true,
	}); err != nil {
		t.Fatal(err)
	}

	users := authenticatedRequest(t, app, token, http.MethodGet, "/api/v1/haloclaw/gateways/"+gatewayID+"/users", "")
	if users.Code != http.StatusOK || !strings.Contains(users.Body.String(), `"platform_user_id":"42"`) {
		t.Fatalf("external users were not listed: %d %s", users.Code, users.Body.String())
	}
	updated := authenticatedJSON(t, app, token, http.MethodPost, "/api/v1/haloclaw/users/"+externalUserID+"/model-override", `{"model_override":null}`)
	if value, exists := updated["model_override"]; !exists || value != nil {
		t.Fatalf("model override was not cleared: %#v", updated)
	}
	logs := authenticatedRequest(t, app, token, http.MethodGet, "/api/v1/haloclaw/gateways/"+gatewayID+"/users/"+externalUserID+"/logs", "")
	if logs.Code != http.StatusOK || !strings.Contains(logs.Body.String(), `"content":"hello"`) {
		t.Fatalf("user logs were not listed: %d %s", logs.Code, logs.Body.String())
	}

	deleted := authenticatedRequest(t, app, token, http.MethodDelete, "/api/v1/haloclaw/gateways/"+gatewayID, "")
	if deleted.Code != http.StatusOK {
		t.Fatalf("gateway delete failed: %d %s", deleted.Code, deleted.Body.String())
	}
	usersAfterDelete := authenticatedRequest(t, app, token, http.MethodGet, "/api/v1/haloclaw/gateways/"+gatewayID+"/users", "")
	if strings.TrimSpace(usersAfterDelete.Body.String()) != "[]" {
		t.Fatalf("gateway delete did not cascade users: %s", usersAfterDelete.Body.String())
	}

	feishu := authenticatedJSON(t, app, token, http.MethodPost, "/api/v1/haloclaw/gateways", `{"name":"Feishu","platform":"feishu","config":{"verification_token":"verify-me"}}`)
	feishuID, _ := feishu["id"].(string)
	challenge := httptest.NewRecorder()
	challengeRequest := httptest.NewRequest(http.MethodPost, "/api/v1/haloclaw/webhook/feishu/"+feishuID, strings.NewReader(`{"type":"url_verification","token":"verify-me","challenge":"challenge-ok"}`))
	challengeRequest.Header.Set("Content-Type", "application/json")
	app.ServeHTTP(challenge, challengeRequest)
	if challenge.Code != http.StatusOK || !strings.Contains(challenge.Body.String(), `"challenge":"challenge-ok"`) {
		t.Fatalf("Feishu webhook challenge failed: %d %s", challenge.Code, challenge.Body.String())
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

type frontendEndpoint struct {
	Method string
	Path   string
	Source string
}

func TestEveryConcreteFrontendAPIEndpointHasGoOwner(t *testing.T) {
	app := testApp(t)
	token := signupToken(t, app)
	endpoints := collectFrontendEndpoints(t)
	if len(endpoints) < 200 {
		t.Fatalf("frontend endpoint inventory is unexpectedly small: %d", len(endpoints))
	}
	inventory := map[string]bool{}
	for _, endpoint := range endpoints {
		inventory[endpoint.Method+" "+endpoint.Path] = true
	}
	for _, required := range []string{
		"POST /api/v1/auths/ldap",
		"GET /api/v1/chats/folder/test",
		"GET /api/v1/chats/share/test",
	} {
		if !inventory[required] {
			t.Errorf("frontend endpoint parser missed required contract %s", required)
		}
	}

	for _, endpoint := range endpoints {
		response := authenticatedRequest(t, app, token, endpoint.Method, endpoint.Path, "{}")
		if response.Header().Get(compatibilityFallbackHeader) == "true" {
			t.Errorf("%s %s from %s has no explicit Go route", endpoint.Method, endpoint.Path, endpoint.Source)
		}
	}
}

func collectFrontendEndpoints(t *testing.T) []frontendEndpoint {
	t.Helper()
	root := filepath.Join("..", "..", "..", "src", "lib", "apis")
	methodPattern := regexp.MustCompile(`(?i)method\s*:\s*['\"](GET|POST|PUT|PATCH|DELETE)['\"]`)
	seen := map[string]bool{}
	result := make([]frontendEndpoint, 0, 256)
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || filepath.Ext(path) != ".ts" || strings.HasSuffix(path, ".test.ts") {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for offset := 0; ; {
			fetchOffset := bytes.Index(content[offset:], []byte("fetch("))
			if fetchOffset < 0 {
				break
			}
			fetchOffset += offset
			cursor := fetchOffset + len("fetch(")
			for cursor < len(content) && (content[cursor] == ' ' || content[cursor] == '\t' || content[cursor] == '\r' || content[cursor] == '\n') {
				cursor++
			}
			if cursor >= len(content) || content[cursor] != '`' {
				offset = cursor
				continue
			}
			url, end, ok := parseFrontendTemplate(content, cursor)
			if !ok {
				offset = cursor + 1
				continue
			}
			windowEnd := min(len(content), end+1200)
			method := http.MethodGet
			if match := methodPattern.FindSubmatch(content[end:windowEnd]); len(match) == 2 {
				method = strings.ToUpper(string(match[1]))
			}
			if query := strings.IndexByte(url, '?'); query >= 0 {
				url = url[:query]
			}
			url = strings.ReplaceAll(url, "//", "/")
			if !strings.HasPrefix(url, "/") || strings.Contains(url, "{dynamic-path}") {
				offset = end
				continue
			}
			key := method + " " + url
			if !seen[key] {
				seen[key] = true
				result = append(result, frontendEndpoint{Method: method, Path: url, Source: filepath.ToSlash(path)})
			}
			offset = end
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, endpoint := range []frontendEndpoint{
		{Method: http.MethodGet, Path: "/api/v1/skills/", Source: "src/lib/apis/skills/index.ts"},
		{Method: http.MethodGet, Path: "/api/v1/skills/list", Source: "src/lib/apis/skills/index.ts"},
		{Method: http.MethodGet, Path: "/api/v1/skills/catalog", Source: "src/lib/apis/skills/index.ts"},
		{Method: http.MethodGet, Path: "/api/v1/skills/runtime/capabilities", Source: "src/lib/apis/skills/index.ts"},
		{Method: http.MethodGet, Path: "/api/v1/skills/legacy-prompts", Source: "src/lib/apis/skills/index.ts"},
		{Method: http.MethodPost, Path: "/api/v1/skills/legacy-prompts/migrate", Source: "src/lib/apis/skills/index.ts"},
		{Method: http.MethodGet, Path: "/api/v1/skills/test", Source: "src/lib/apis/skills/index.ts"},
		{Method: http.MethodPost, Path: "/api/v1/skills/create", Source: "src/lib/apis/skills/index.ts"},
		{Method: http.MethodPost, Path: "/api/v1/skills/test/update", Source: "src/lib/apis/skills/index.ts"},
		{Method: http.MethodDelete, Path: "/api/v1/skills/test/delete", Source: "src/lib/apis/skills/index.ts"},
		{Method: http.MethodPost, Path: "/api/v1/skills/test/runtime/install", Source: "src/lib/apis/skills/index.ts"},
		{Method: http.MethodDelete, Path: "/api/v1/skills/test/runtime/install", Source: "src/lib/apis/skills/index.ts"},
		{Method: http.MethodPost, Path: "/api/v1/skills/test/auto", Source: "src/lib/apis/skills/index.ts"},
		{Method: http.MethodPost, Path: "/api/v1/skills/import", Source: "src/lib/apis/skills/index.ts"},
		{Method: http.MethodPost, Path: "/api/v1/skills/import/url", Source: "src/lib/apis/skills/index.ts"},
		{Method: http.MethodPost, Path: "/api/v1/skills/import/github", Source: "src/lib/apis/skills/index.ts"},
		{Method: http.MethodPost, Path: "/api/v1/skills/import/zip", Source: "src/lib/apis/skills/index.ts"},
	} {
		key := endpoint.Method + " " + endpoint.Path
		if !seen[key] {
			seen[key] = true
			result = append(result, endpoint)
		}
	}
	return result
}

func parseFrontendTemplate(content []byte, start int) (string, int, bool) {
	var result strings.Builder
	for cursor := start + 1; cursor < len(content); {
		switch {
		case content[cursor] == '\\':
			if cursor+1 < len(content) {
				result.WriteByte(content[cursor+1])
				cursor += 2
				continue
			}
		case content[cursor] == '`':
			return result.String(), cursor + 1, true
		case content[cursor] == '$' && cursor+1 < len(content) && content[cursor+1] == '{':
			expression, end, ok := parseFrontendExpression(content, cursor+2)
			if !ok {
				return "", cursor + 1, false
			}
			result.WriteString(frontendExpressionValue(expression))
			cursor = end
			continue
		}
		result.WriteByte(content[cursor])
		cursor++
	}
	return "", len(content), false
}

func parseFrontendExpression(content []byte, start int) (string, int, bool) {
	depth := 1
	quote := byte(0)
	escaped := false
	for cursor := start; cursor < len(content); cursor++ {
		char := content[cursor]
		if quote != 0 {
			if escaped {
				escaped = false
				continue
			}
			if char == '\\' {
				escaped = true
				continue
			}
			if char == quote {
				quote = 0
			}
			continue
		}
		if char == '\'' || char == '"' || char == '`' {
			quote = char
			continue
		}
		switch char {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return string(content[start:cursor]), cursor + 1, true
			}
		}
	}
	return "", len(content), false
}

func frontendExpressionValue(expression string) string {
	expression = strings.TrimSpace(expression)
	bases := map[string]string{
		"WEBUI_BASE_URL":              "",
		"WEBUI_API_BASE_URL":          "/api/v1",
		"OLLAMA_API_BASE_URL":         "/ollama",
		"OPENAI_API_BASE_URL":         "/openai",
		"GEMINI_API_BASE_URL":         "/gemini",
		"GROK_API_BASE_URL":           "/grok",
		"ANTHROPIC_API_BASE_URL":      "/anthropic",
		"AUDIO_API_BASE_URL":          "/api/v1/audio",
		"IMAGES_API_BASE_URL":         "/api/v1/images",
		"RETRIEVAL_API_BASE_URL":      "/api/v1/retrieval",
		"HALOCLAW_API_BASE_URL":       "/api/v1/haloclaw",
		"EXTERNAL_API_ADMIN_BASE_URL": "/api/v1/external_api",
	}
	if value, ok := bases[expression]; ok {
		return value
	}
	lower := strings.ToLower(expression)
	if strings.Contains(lower, "query") || strings.Contains(lower, "params") || strings.Contains(lower, "suffix") {
		return ""
	}
	if expression == "path" {
		return "{dynamic-path}"
	}
	return "test"
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

func TestMemoryCRUDQueryAndUserIsolation(t *testing.T) {
	app := testApp(t)
	adminToken := signupToken(t, app)
	userToken := signupTokenFor(t, app, "Memory User", "memory-user@example.com")

	created := authenticatedJSON(t, app, adminToken, http.MethodPost, "/api/v1/memories/add", `{"content":"Project Alpha launch checklist"}`)
	memoryID, _ := created["id"].(string)
	if memoryID == "" || created["content"] != "Project Alpha launch checklist" {
		t.Fatalf("memory create contract is incomplete: %#v", created)
	}
	query := authenticatedJSON(t, app, adminToken, http.MethodPost, "/api/v1/memories/query", `{"content":"alpha project","k":3}`)
	ids, _ := query["ids"].([]any)
	if len(ids) != 1 || len(ids[0].([]any)) != 1 || ids[0].([]any)[0] != memoryID {
		t.Fatalf("memory query did not return the owned match: %#v", query)
	}
	updated := authenticatedJSON(t, app, adminToken, http.MethodPost, "/api/v1/memories/"+memoryID+"/update", `{"content":"Project Alpha shipped"}`)
	if updated["content"] != "Project Alpha shipped" {
		t.Fatalf("memory was not updated: %#v", updated)
	}
	otherList := authenticatedRequest(t, app, userToken, http.MethodGet, "/api/v1/memories/", "")
	if otherList.Code != http.StatusOK || strings.TrimSpace(otherList.Body.String()) != "[]" {
		t.Fatalf("memory leaked across users: %d %s", otherList.Code, otherList.Body.String())
	}
	otherDelete := authenticatedRequest(t, app, userToken, http.MethodDelete, "/api/v1/memories/"+memoryID, "")
	if otherDelete.Code != http.StatusOK || strings.TrimSpace(otherDelete.Body.String()) != "false" {
		t.Fatalf("another user deleted a memory: %d %s", otherDelete.Code, otherDelete.Body.String())
	}
	deleted := authenticatedRequest(t, app, adminToken, http.MethodDelete, "/api/v1/memories/delete/user", "")
	if deleted.Code != http.StatusOK || strings.TrimSpace(deleted.Body.String()) != "true" {
		t.Fatalf("memory clear failed: %d %s", deleted.Code, deleted.Body.String())
	}
}

func TestTaskConfigAuthorizationPersistenceAndProviderProxy(t *testing.T) {
	var upstreamBody map[string]any
	upstream := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/chat/completions" {
			t.Errorf("unexpected task provider path: %s", request.URL.Path)
		}
		if err := json.NewDecoder(request.Body).Decode(&upstreamBody); err != nil {
			t.Errorf("task provider received invalid JSON: %v", err)
		}
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"Alpha title"}}]}`))
	}))
	defer upstream.Close()

	app := testApp(t)
	app.config.OpenAIBaseURL = upstream.URL
	app.config.OpenAIAPIKey = "task-key"
	adminToken := signupToken(t, app)
	userToken := signupTokenFor(t, app, "Task User", "task-user@example.com")

	forbidden := authenticatedRequest(t, app, userToken, http.MethodPost, "/api/v1/tasks/config/update", `{"TASK_MODEL":"task-model"}`)
	if forbidden.Code != http.StatusForbidden {
		t.Fatalf("non-admin updated task config: %d %s", forbidden.Code, forbidden.Body.String())
	}
	updated := authenticatedJSON(t, app, adminToken, http.MethodPost, "/api/v1/tasks/config/update", `{"TASK_MODEL":"task-model","ENABLE_TITLE_GENERATION":true}`)
	if updated["TASK_MODEL"] != "task-model" || updated["ENABLE_TITLE_GENERATION"] != true {
		t.Fatalf("task config update was not persisted: %#v", updated)
	}
	reloaded := authenticatedJSON(t, app, userToken, http.MethodGet, "/api/v1/tasks/config", "")
	if reloaded["TASK_MODEL"] != "task-model" {
		t.Fatalf("task config could not be reloaded: %#v", reloaded)
	}
	completion := authenticatedRequest(t, app, userToken, http.MethodPost, "/api/v1/tasks/title/completions", `{"model":"chat-model","messages":[{"role":"user","content":"Discuss Alpha"}]}`)
	if completion.Code != http.StatusOK || !strings.Contains(completion.Body.String(), "Alpha title") {
		t.Fatalf("task completion did not proxy provider response: %d %s", completion.Code, completion.Body.String())
	}
	if upstreamBody["model"] != "task-model" || upstreamBody["stream"] != false {
		t.Fatalf("task provider received the wrong routing payload: %#v", upstreamBody)
	}
	messages, _ := upstreamBody["messages"].([]any)
	if len(messages) < 2 {
		t.Fatalf("task prompt was not appended to conversation context: %#v", upstreamBody)
	}
}

func TestKnowledgeFileIndexQueryAndPermissions(t *testing.T) {
	app := testApp(t)
	app.config.FileMaxSizeBytes = 1 << 20
	adminToken := signupToken(t, app)
	otherToken := signupTokenFor(t, app, "Knowledge User", "knowledge-user@example.com")

	var uploadBody bytes.Buffer
	writer := multipart.NewWriter(&uploadBody)
	part, err := writer.CreateFormFile("file", "alpha-notes.txt")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = part.Write([]byte("Project Alpha deployment checklist and rollback notes"))
	_ = writer.Close()
	uploadRequest := httptest.NewRequest(http.MethodPost, "/api/v1/files/", &uploadBody)
	uploadRequest.Header.Set("Authorization", "Bearer "+adminToken)
	uploadRequest.Header.Set("Content-Type", writer.FormDataContentType())
	uploadResponse := httptest.NewRecorder()
	app.ServeHTTP(uploadResponse, uploadRequest)
	if uploadResponse.Code != http.StatusOK {
		t.Fatalf("knowledge test file upload failed: %d %s", uploadResponse.Code, uploadResponse.Body.String())
	}
	var uploaded map[string]any
	_ = json.NewDecoder(uploadResponse.Body).Decode(&uploaded)
	fileID, _ := uploaded["id"].(string)

	created := authenticatedJSON(t, app, adminToken, http.MethodPost, "/api/v1/knowledge/create", `{"name":"Alpha KB","description":"Deployment docs","access_control":{}}`)
	knowledgeID, _ := created["id"].(string)
	if knowledgeID == "" {
		t.Fatalf("knowledge id missing: %#v", created)
	}
	added := authenticatedJSON(t, app, adminToken, http.MethodPost, "/api/v1/knowledge/"+knowledgeID+"/file/add", `{"file_id":"`+fileID+`"}`)
	files, _ := added["files"].([]any)
	if len(files) != 1 {
		t.Fatalf("knowledge file was not associated: %#v", added)
	}
	query := authenticatedJSON(t, app, adminToken, http.MethodPost, "/api/v1/retrieval/query/collection", `{"collection_names":"`+knowledgeID+`","query":"rollback checklist","k":3}`)
	documents, _ := query["documents"].([]any)
	if len(documents) != 1 || len(documents[0].([]any)) != 1 || !strings.Contains(documents[0].([]any)[0].(string), "rollback notes") {
		t.Fatalf("knowledge content was not indexed and queried: %#v", query)
	}
	fileSearch := authenticatedJSON(t, app, adminToken, http.MethodGet, "/api/v1/knowledge/search/files?query=alpha", "")
	if fileSearch["total"] != float64(1) {
		t.Fatalf("knowledge file search returned the wrong result: %#v", fileSearch)
	}
	forbidden := authenticatedRequest(t, app, otherToken, http.MethodGet, "/api/v1/knowledge/"+knowledgeID, "")
	if forbidden.Code != http.StatusForbidden {
		t.Fatalf("private knowledge leaked to another user: %d %s", forbidden.Code, forbidden.Body.String())
	}
	removed := authenticatedJSON(t, app, adminToken, http.MethodPost, "/api/v1/knowledge/"+knowledgeID+"/file/remove", `{"file_id":"`+fileID+`"}`)
	if len(removed["files"].([]any)) != 0 {
		t.Fatalf("knowledge file was not removed: %#v", removed)
	}
	emptyQuery := authenticatedJSON(t, app, adminToken, http.MethodPost, "/api/v1/retrieval/query/doc", `{"collection_name":"`+knowledgeID+`","query":"rollback","k":3}`)
	emptyDocuments, _ := emptyQuery["documents"].([]any)
	if len(emptyDocuments) != 1 || len(emptyDocuments[0].([]any)) != 0 {
		t.Fatalf("removed knowledge file remained searchable: %#v", emptyQuery)
	}
}

func TestAnthropicChatUsesAccountConnectionAndTransformsResponse(t *testing.T) {
	var upstreamBody map[string]any
	var apiKey, version string
	upstream := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/v1/messages" {
			t.Errorf("unexpected Anthropic path: %s", request.URL.Path)
		}
		apiKey, version = request.Header.Get("x-api-key"), request.Header.Get("anthropic-version")
		_ = json.NewDecoder(request.Body).Decode(&upstreamBody)
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(`{"id":"msg_go","model":"claude-test","content":[{"type":"text","text":"ANTHROPIC_GO_OK"}],"stop_reason":"end_turn","usage":{"input_tokens":7,"output_tokens":3}}`))
	}))
	defer upstream.Close()

	app := testApp(t)
	token := signupToken(t, app)
	authenticatedJSON(t, app, token, http.MethodPost, "/api/v1/users/user/settings/update", `{"ui":{"connections":{"anthropic":{"ANTHROPIC_API_BASE_URLS":["`+upstream.URL+`/v1"],"ANTHROPIC_API_KEYS":["anthropic-user-key"],"ANTHROPIC_API_CONFIGS":{"0":{"enable":true}}}}}}`)
	response := authenticatedRequest(t, app, token, http.MethodPost, "/anthropic/chat/completions", `{"model":"claude-test","messages":[{"role":"system","content":"Be concise"},{"role":"user","content":"hello"}],"max_tokens":32,"stream":false}`)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "ANTHROPIC_GO_OK") || !strings.Contains(response.Body.String(), `"prompt_tokens":7`) {
		t.Fatalf("Anthropic response was not transformed: %d %s", response.Code, response.Body.String())
	}
	if apiKey != "anthropic-user-key" || version == "" || upstreamBody["model"] != "claude-test" || upstreamBody["max_tokens"] != float64(32) {
		t.Fatalf("Anthropic request contract is incomplete: key=%q version=%q body=%#v", apiKey, version, upstreamBody)
	}
	if upstreamBody["system"] != "Be concise" {
		t.Fatalf("Anthropic system instruction was not extracted: %#v", upstreamBody)
	}
}

func TestGeminiStreamingChatTransformsSSE(t *testing.T) {
	var requestPath, apiKey string
	var upstreamBody map[string]any
	upstream := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		requestPath, apiKey = request.URL.RequestURI(), request.Header.Get("x-goog-api-key")
		_ = json.NewDecoder(request.Body).Decode(&upstreamBody)
		response.Header().Set("Content-Type", "text/event-stream")
		_, _ = response.Write([]byte("data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"GEMINI_\"}]}}]}\n\n"))
		_, _ = response.Write([]byte("data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"GO_OK\"}]},\"finishReason\":\"STOP\"}]}\n\n"))
	}))
	defer upstream.Close()

	app := testApp(t)
	token := signupToken(t, app)
	authenticatedJSON(t, app, token, http.MethodPost, "/api/v1/users/user/settings/update", `{"ui":{"connections":{"gemini":{"GEMINI_API_BASE_URLS":["`+upstream.URL+`/v1beta"],"GEMINI_API_KEYS":["gemini-user-key"],"GEMINI_API_CONFIGS":{"0":{"enable":true}}}}}}`)
	response := authenticatedRequest(t, app, token, http.MethodPost, "/gemini/chat/completions", `{"model":"gemini-test","messages":[{"role":"user","content":"hello"}],"stream":true}`)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "GEMINI_") || !strings.Contains(response.Body.String(), "GO_OK") || !strings.Contains(response.Body.String(), "[DONE]") {
		t.Fatalf("Gemini SSE was not transformed: %d %s", response.Code, response.Body.String())
	}
	if requestPath != "/v1beta/models/gemini-test:streamGenerateContent?alt=sse" || apiKey != "gemini-user-key" {
		t.Fatalf("Gemini request routing is wrong: path=%q key=%q", requestPath, apiKey)
	}
	if _, ok := upstreamBody["contents"].([]any); !ok {
		t.Fatalf("Gemini contents were not translated: %#v", upstreamBody)
	}
}

func TestGrokChatUsesOpenAICompatibleAccountConnection(t *testing.T) {
	var requestPath, authorization string
	upstream := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		requestPath, authorization = request.URL.Path, request.Header.Get("Authorization")
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"GROK_GO_OK"}}]}`))
	}))
	defer upstream.Close()

	app := testApp(t)
	token := signupToken(t, app)
	authenticatedJSON(t, app, token, http.MethodPost, "/api/v1/users/user/settings/update", `{"ui":{"connections":{"grok":{"GROK_API_BASE_URLS":["`+upstream.URL+`/v1"],"GROK_API_KEYS":["grok-user-key"],"GROK_API_CONFIGS":{"0":{"enable":true}}}}}}`)
	response := authenticatedRequest(t, app, token, http.MethodPost, "/grok/chat/completions", `{"model":"grok-test","messages":[{"role":"user","content":"hello"}],"stream":false}`)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "GROK_GO_OK") {
		t.Fatalf("Grok response was not proxied: %d %s", response.Code, response.Body.String())
	}
	if requestPath != "/v1/chat/completions" || authorization != "Bearer grok-user-key" {
		t.Fatalf("Grok request routing is wrong: path=%q authorization=%q", requestPath, authorization)
	}
}

func TestRemoteAudioSpeechAndTranscriptionAdapters(t *testing.T) {
	var speechBody map[string]any
	var transcriptionAuthorization string
	upstream := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		transcriptionAuthorization = request.Header.Get("Authorization")
		switch request.URL.Path {
		case "/audio/speech":
			_ = json.NewDecoder(request.Body).Decode(&speechBody)
			response.Header().Set("Content-Type", "audio/mpeg")
			_, _ = response.Write([]byte("GO_AUDIO_BYTES"))
		case "/audio/transcriptions":
			response.Header().Set("Content-Type", "application/json")
			_, _ = response.Write([]byte(`{"text":"GO_TRANSCRIPTION_OK","language":"zh"}`))
		default:
			response.WriteHeader(http.StatusNotFound)
		}
	}))
	defer upstream.Close()

	app := testApp(t)
	token := signupToken(t, app)
	config := `{"tts":{"OPENAI_API_BASE_URL":"` + upstream.URL + `","OPENAI_API_KEY":"tts-key","API_KEY":"","ENGINE":"openai","MODEL":"tts-small","VOICE":"alloy","SPLIT_ON":"punctuation","AZURE_SPEECH_REGION":"","AZURE_SPEECH_OUTPUT_FORMAT":""},"stt":{"OPENAI_API_BASE_URL":"` + upstream.URL + `","OPENAI_API_KEY":"stt-key","ENGINE":"openai","MODEL":"whisper-test","WHISPER_MODEL":"","DEEPGRAM_API_KEY":"","AZURE_API_KEY":"","AZURE_REGION":"","AZURE_LOCALES":""}}`
	response := authenticatedRequest(t, app, token, http.MethodPost, "/api/v1/audio/config/update", config)
	if response.Code != http.StatusOK {
		t.Fatalf("audio config setup failed: %d %s", response.Code, response.Body.String())
	}
	speech := authenticatedRequest(t, app, token, http.MethodPost, "/api/v1/audio/speech", `{"input":"hello"}`)
	if speech.Code != http.StatusOK || speech.Body.String() != "GO_AUDIO_BYTES" || speech.Header().Get("Content-Type") != "audio/mpeg" {
		t.Fatalf("remote speech adapter failed: %d %q", speech.Code, speech.Body.String())
	}
	if speechBody["model"] != "tts-small" || speechBody["voice"] != "alloy" {
		t.Fatalf("speech defaults were not applied: %#v", speechBody)
	}
	var audioBody bytes.Buffer
	writer := multipart.NewWriter(&audioBody)
	part, err := writer.CreateFormFile("file", "sample.wav")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = part.Write([]byte("RIFF-GO-AUDIO"))
	_ = writer.WriteField("language", "zh")
	_ = writer.Close()
	transcriptionRequest := httptest.NewRequest(http.MethodPost, "/api/v1/audio/transcriptions", &audioBody)
	transcriptionRequest.Header.Set("Authorization", "Bearer "+token)
	transcriptionRequest.Header.Set("Content-Type", writer.FormDataContentType())
	transcriptionResponse := httptest.NewRecorder()
	app.ServeHTTP(transcriptionResponse, transcriptionRequest)
	if transcriptionResponse.Code != http.StatusOK || !strings.Contains(transcriptionResponse.Body.String(), "GO_TRANSCRIPTION_OK") || transcriptionAuthorization != "Bearer stt-key" {
		t.Fatalf("remote transcription adapter failed: %d %s auth=%q", transcriptionResponse.Code, transcriptionResponse.Body.String(), transcriptionAuthorization)
	}
}

func TestRemoteImageGenerationAdapter(t *testing.T) {
	var generationBody map[string]any
	upstream := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/v1/models":
			response.Header().Set("Content-Type", "application/json")
			_, _ = response.Write([]byte(`{"data":[{"id":"gpt-image-1","object":"model"},{"id":"text-model"}]}`))
		case "/v1/images/generations":
			_ = json.NewDecoder(request.Body).Decode(&generationBody)
			response.Header().Set("Content-Type", "application/json")
			_, _ = response.Write([]byte(`{"created":1,"data":[{"url":"https://images.example.test/go.png"}]}`))
		default:
			response.WriteHeader(http.StatusNotFound)
		}
	}))
	defer upstream.Close()

	app := testApp(t)
	adminToken := signupToken(t, app)
	config := `{"enabled":true,"engine":"openai","shared_key_enabled":true,"openai":{"OPENAI_API_BASE_URL":"` + upstream.URL + `/v1","OPENAI_API_KEY":"image-key"}}`
	configured := authenticatedRequest(t, app, adminToken, http.MethodPost, "/api/v1/images/config/update", config)
	if configured.Code != http.StatusOK {
		t.Fatalf("image config setup failed: %d %s", configured.Code, configured.Body.String())
	}
	modelResponse := authenticatedRequest(t, app, adminToken, http.MethodGet, "/api/v1/images/models?search=image", "")
	if modelResponse.Code != http.StatusOK {
		t.Fatalf("image model endpoint failed: %d %s", modelResponse.Code, modelResponse.Body.String())
	}
	var modelList []any
	if err := json.NewDecoder(modelResponse.Body).Decode(&modelList); err != nil {
		t.Fatal(err)
	}
	if len(modelList) != 1 || modelList[0].(map[string]any)["id"] != "gpt-image-1" {
		t.Fatalf("image model discovery failed: %#v", modelList)
	}
	generated := authenticatedRequest(t, app, adminToken, http.MethodPost, "/api/v1/images/generations", `{"prompt":"A small Go service","model":"gpt-image-1","image_size":"1024x1024","n":2}`)
	if generated.Code != http.StatusOK || !strings.Contains(generated.Body.String(), "images.example.test/go.png") {
		t.Fatalf("image generation failed: %d %s", generated.Code, generated.Body.String())
	}
	if generationBody["model"] != "gpt-image-1" || generationBody["size"] != "1024x1024" || generationBody["n"] != float64(2) {
		t.Fatalf("image request was not normalized: %#v", generationBody)
	}
	usage := authenticatedJSON(t, app, adminToken, http.MethodGet, "/api/v1/images/usage/config", "")
	if usage["enabled"] != true || usage["engine"] != "openai" {
		t.Fatalf("image usage config is incomplete: %#v", usage)
	}
}

func TestChannelMessagesReactionsAndWebhook(t *testing.T) {
	app := testApp(t)
	token := signupToken(t, app)
	channel := authenticatedJSON(t, app, token, http.MethodPost, "/api/v1/channels/create", `{"name":"Engineering","description":"Go channel"}`)
	channelID, _ := channel["id"].(string)
	if channelID == "" {
		t.Fatalf("channel id missing: %#v", channel)
	}
	message := authenticatedJSON(t, app, token, http.MethodPost, "/api/v1/channels/"+channelID+"/messages/post", `{"content":"Hello channel"}`)
	messageID, _ := message["id"].(string)
	if messageID == "" || message["channel_id"] != channelID {
		t.Fatalf("channel message create failed: %#v", message)
	}
	reply := authenticatedJSON(t, app, token, http.MethodPost, "/api/v1/channels/"+channelID+"/messages/post", `{"parent_id":"`+messageID+`","content":"Thread reply"}`)
	if reply["parent_id"] != messageID {
		t.Fatalf("thread reply was not linked: %#v", reply)
	}
	threadResponse := authenticatedRequest(t, app, token, http.MethodGet, "/api/v1/channels/"+channelID+"/messages/"+messageID+"/thread", "")
	if threadResponse.Code != http.StatusOK || !strings.Contains(threadResponse.Body.String(), "Thread reply") {
		t.Fatalf("thread listing failed: %d %s", threadResponse.Code, threadResponse.Body.String())
	}
	reaction := authenticatedRequest(t, app, token, http.MethodPost, "/api/v1/channels/"+channelID+"/messages/"+messageID+"/reactions/add", `{"name":"thumbsup"}`)
	if reaction.Code != http.StatusOK || strings.TrimSpace(reaction.Body.String()) != "true" {
		t.Fatalf("reaction add failed: %d %s", reaction.Code, reaction.Body.String())
	}
	withReaction := authenticatedRequest(t, app, token, http.MethodGet, "/api/v1/channels/"+channelID+"/messages/"+messageID, "")
	if withReaction.Code != http.StatusOK || !strings.Contains(withReaction.Body.String(), "thumbsup") {
		t.Fatalf("reaction was not persisted: %d %s", withReaction.Code, withReaction.Body.String())
	}
	configuredWebhook := authenticatedJSON(t, app, token, http.MethodPost, "/api/v1/channels/"+channelID+"/webhook", `{"url":"https://hooks.example.test/channel"}`)
	webhookToken, _ := configuredWebhook["token"].(string)
	if webhookToken == "" {
		t.Fatalf("webhook token missing: %#v", configuredWebhook)
	}
	incoming := httptest.NewRequest(http.MethodPost, "/api/v1/channels/"+channelID+"/webhook/incoming", strings.NewReader(`{"content":"Webhook message","username":"CI"}`))
	incoming.Header.Set("Authorization", "Bearer "+webhookToken)
	incomingResponse := httptest.NewRecorder()
	app.ServeHTTP(incomingResponse, incoming)
	if incomingResponse.Code != http.StatusOK || strings.TrimSpace(incomingResponse.Body.String()) != "true" {
		t.Fatalf("incoming webhook failed: %d %s", incomingResponse.Code, incomingResponse.Body.String())
	}
	removed := authenticatedRequest(t, app, token, http.MethodPost, "/api/v1/channels/"+channelID+"/messages/"+messageID+"/reactions/remove", `{"name":"thumbsup"}`)
	if removed.Code != http.StatusOK || strings.TrimSpace(removed.Body.String()) != "true" {
		t.Fatalf("reaction remove failed: %d %s", removed.Code, removed.Body.String())
	}
}

func TestToolVisibilityAndValvePersistence(t *testing.T) {
	app := testApp(t)
	adminToken := signupToken(t, app)
	userToken := signupTokenFor(t, app, "Tool User", "tool-user@example.com")
	publicTool := authenticatedJSON(t, app, adminToken, http.MethodPost, "/api/v1/tools/create", `{"id":"public_tool","name":"Public Tool","content":"go-native-adapter","valves_spec":{"type":"object","properties":{"endpoint":{"type":"string"}}},"user_valves_spec":{"type":"object","properties":{"token":{"type":"string"}}},"access_control":null}`)
	if publicTool["id"] != "public_tool" {
		t.Fatalf("public tool create failed: %#v", publicTool)
	}
	authenticatedJSON(t, app, adminToken, http.MethodPost, "/api/v1/tools/create", `{"id":"private_tool","name":"Private Tool","content":"go-native-adapter","access_control":{}}`)
	listResponse := authenticatedRequest(t, app, userToken, http.MethodGet, "/api/v1/tools/", "")
	if listResponse.Code != http.StatusOK || !strings.Contains(listResponse.Body.String(), "public_tool") || strings.Contains(listResponse.Body.String(), "private_tool") {
		t.Fatalf("tool access control filtering failed: %d %s", listResponse.Code, listResponse.Body.String())
	}
	publicGet := authenticatedRequest(t, app, userToken, http.MethodGet, "/api/v1/tools/id/public_tool", "")
	if publicGet.Code != http.StatusOK {
		t.Fatalf("shared tool could not be read: %d %s", publicGet.Code, publicGet.Body.String())
	}
	privateGet := authenticatedRequest(t, app, userToken, http.MethodGet, "/api/v1/tools/id/private_tool", "")
	if privateGet.Code != http.StatusForbidden {
		t.Fatalf("private tool was readable: %d %s", privateGet.Code, privateGet.Body.String())
	}
	globalValve := authenticatedJSON(t, app, adminToken, http.MethodPost, "/api/v1/tools/id/public_tool/valves/update", `{"endpoint":"https://tools.example.test"}`)
	if globalValve["endpoint"] != "https://tools.example.test" {
		t.Fatalf("global tool valves were not saved: %#v", globalValve)
	}
	userValve := authenticatedJSON(t, app, userToken, http.MethodPost, "/api/v1/tools/id/public_tool/valves/user/update", `{"token":"account-secret"}`)
	if userValve["token"] != "account-secret" {
		t.Fatalf("user tool valves were not saved: %#v", userValve)
	}
	reloaded := authenticatedJSON(t, app, userToken, http.MethodGet, "/api/v1/tools/id/public_tool/valves/user", "")
	if reloaded["token"] != "account-secret" {
		t.Fatalf("user tool valves were not isolated/persisted: %#v", reloaded)
	}
}

func TestFunctionAdminLifecycleAndValves(t *testing.T) {
	app := testApp(t)
	adminToken := signupToken(t, app)
	userToken := signupTokenFor(t, app, "Function User", "function-user@example.com")
	created := authenticatedJSON(t, app, adminToken, http.MethodPost, "/api/v1/functions/create", `{"id":"go_filter","name":"Go Filter","content":"remote-adapter","valves_spec":{"type":"object"}}`)
	if created["id"] != "go_filter" || created["is_active"] != true {
		t.Fatalf("function create failed: %#v", created)
	}
	forbidden := authenticatedRequest(t, app, userToken, http.MethodPost, "/api/v1/functions/create", `{"id":"bad","name":"Bad"}`)
	if forbidden.Code != http.StatusForbidden {
		t.Fatalf("non-admin created a function: %d %s", forbidden.Code, forbidden.Body.String())
	}
	toggled := authenticatedJSON(t, app, adminToken, http.MethodPost, "/api/v1/functions/id/go_filter/toggle/global", "")
	if toggled["is_global"] != true {
		t.Fatalf("function global toggle failed: %#v", toggled)
	}
	valves := authenticatedJSON(t, app, adminToken, http.MethodPost, "/api/v1/functions/id/go_filter/valves/update", `{"mode":"remote"}`)
	if valves["mode"] != "remote" {
		t.Fatalf("function valves failed: %#v", valves)
	}
}

func TestTypedConfigurationOwnershipAndImportExport(t *testing.T) {
	app := testApp(t)
	adminToken := signupToken(t, app)
	userToken := signupTokenFor(t, app, "Config User", "config-user@example.com")
	otherToken := signupTokenFor(t, app, "Other Config User", "other-config@example.com")

	connections := authenticatedJSON(t, app, adminToken, http.MethodPost, "/api/v1/configs/connections", `{"ENABLE_DIRECT_CONNECTIONS":true,"ENABLE_BASE_MODELS_CACHE":false}`)
	if connections["ENABLE_DIRECT_CONNECTIONS"] != true || connections["ENABLE_BASE_MODELS_CACHE"] != false {
		t.Fatalf("global connections config failed: %#v", connections)
	}
	forbidden := authenticatedRequest(t, app, userToken, http.MethodGet, "/api/v1/configs/connections", "")
	if forbidden.Code != http.StatusForbidden {
		t.Fatalf("non-admin read global connection config: %d %s", forbidden.Code, forbidden.Body.String())
	}
	userTools := authenticatedJSON(t, app, userToken, http.MethodPost, "/api/v1/configs/native_tools", `{"TOOL_CALLING_MODE":"native","ENABLE_TIME_TOOLS":false}`)
	if userTools["TOOL_CALLING_MODE"] != "native" || userTools["ENABLE_TIME_TOOLS"] != false {
		t.Fatalf("per-user native tool config failed: %#v", userTools)
	}
	otherTools := authenticatedJSON(t, app, otherToken, http.MethodGet, "/api/v1/configs/native_tools", "")
	if otherTools["TOOL_CALLING_MODE"] != "default" || otherTools["ENABLE_TIME_TOOLS"] != true {
		t.Fatalf("per-user native tool config leaked: %#v", otherTools)
	}
	stdio := authenticatedRequest(t, app, adminToken, http.MethodPost, "/api/v1/configs/mcp_servers/verify", `{"transport_type":"stdio","command":"node"}`)
	if stdio.Code != http.StatusBadRequest {
		t.Fatalf("stdio MCP was enabled in slim profile: %d %s", stdio.Code, stdio.Body.String())
	}
	imported := authenticatedJSON(t, app, adminToken, http.MethodPost, "/api/v1/configs/import", `{"mode":"replace","config":{"models":{"DEFAULT_MODELS":"ignored","MODEL_ORDER_LIST":["go-model"]},"banners":{"banners":[{"id":"go-banner","text":"Go backend"}]}}}`)
	models, _ := imported["models"].(map[string]any)
	if len(models["MODEL_ORDER_LIST"].([]any)) != 1 {
		t.Fatalf("config import/export failed: %#v", imported)
	}
	bannersResponse := authenticatedRequest(t, app, adminToken, http.MethodGet, "/api/v1/configs/banners", "")
	if bannersResponse.Code != http.StatusOK || !strings.Contains(bannersResponse.Body.String(), "go-banner") {
		t.Fatalf("imported banners were not persisted: %d %s", bannersResponse.Code, bannersResponse.Body.String())
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

func TestUtilsContracts(t *testing.T) {
	app := testApp(t)
	token := signupToken(t, app)
	gravatar := authenticatedRequest(t, app, token, http.MethodGet, "/api/v1/utils/gravatar?email=Admin%40Example.com", "")
	if gravatar.Code != http.StatusOK || !strings.Contains(gravatar.Body.String(), "e64c7d89f26bd1972efa854d13d7dd61") {
		t.Fatalf("unexpected gravatar response: %d %s", gravatar.Code, gravatar.Body.String())
	}
	markdown := authenticatedJSON(t, app, token, http.MethodPost, "/api/v1/utils/markdown", `{"md":"# Hello\n\n**world**"}`)
	if !strings.Contains(markdown["html"].(string), "<h1>Hello</h1>") || !strings.Contains(markdown["html"].(string), "<strong>world</strong>") {
		t.Fatalf("markdown conversion failed: %#v", markdown)
	}
	formatted := authenticatedJSON(t, app, token, http.MethodPost, "/api/v1/utils/code/format", `{"code":"x=1"}`)
	if formatted["code"] != "x=1\n" || formatted["formatter_unavailable"] != true {
		t.Fatalf("slim formatter contract failed: %#v", formatted)
	}
	pdf := authenticatedRequest(t, app, token, http.MethodPost, "/api/v1/utils/pdf", `{"title":"Export","messages":[{"role":"user","content":"hello"}]}`)
	if pdf.Code != http.StatusOK || !strings.HasPrefix(pdf.Body.String(), "%PDF-") || pdf.Header().Get("Content-Type") != "application/pdf" {
		t.Fatalf("PDF contract failed: %d %s", pdf.Code, pdf.Body.String()[:minInt(len(pdf.Body.String()), 20)])
	}
	download := authenticatedRequest(t, app, token, http.MethodGet, "/api/v1/utils/db/download?kind=sqlite", "")
	if download.Code != http.StatusOK || !strings.Contains(download.Header().Get("Content-Disposition"), "webui.db") || len(download.Body.Bytes()) < 100 {
		t.Fatalf("database download contract failed: %d %s", download.Code, download.Body.String())
	}
	var multipartBody bytes.Buffer
	writer := multipart.NewWriter(&multipartBody)
	part, err := writer.CreateFormFile("file", "webui.db")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(download.Body.Bytes()); err != nil {
		t.Fatal(err)
	}
	_ = writer.WriteField("expected_kind", "sqlite")
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	inspectRequest := httptest.NewRequest(http.MethodPost, "/api/v1/utils/db/restore/inspect", &multipartBody)
	inspectRequest.Header.Set("Authorization", "Bearer "+token)
	inspectRequest.Header.Set("Content-Type", writer.FormDataContentType())
	inspect := httptest.NewRecorder()
	app.ServeHTTP(inspect, inspectRequest)
	if inspect.Code != http.StatusOK || !strings.Contains(inspect.Body.String(), `"compatible":true`) {
		t.Fatalf("database inspect failed: %d %s", inspect.Code, inspect.Body.String())
	}
}

func TestFullBackupRestoresUploads(t *testing.T) {
	app := testApp(t)
	token := signupToken(t, app)
	uploads := filepath.Join(app.config.DataDir, "uploads")
	if err := os.MkdirAll(uploads, 0o700); err != nil {
		t.Fatal(err)
	}
	proofPath := filepath.Join(uploads, "restore-proof.txt")
	if err := os.WriteFile(proofPath, []byte("UPLOAD_RESTORE_OK"), 0o600); err != nil {
		t.Fatal(err)
	}

	download := authenticatedRequest(t, app, token, http.MethodGet, "/api/v1/utils/db/download?kind=full", "")
	if download.Code != http.StatusOK || len(download.Body.Bytes()) < 100 {
		t.Fatalf("full database download failed: %d %s", download.Code, download.Body.String())
	}
	var multipartBody bytes.Buffer
	writer := multipart.NewWriter(&multipartBody)
	part, err := writer.CreateFormFile("file", "halo-webui-full-backup.hwbk")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(download.Body.Bytes()); err != nil {
		t.Fatal(err)
	}
	_ = writer.WriteField("expected_kind", "full")
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	inspectRequest := httptest.NewRequest(http.MethodPost, "/api/v1/utils/db/restore/inspect", &multipartBody)
	inspectRequest.Header.Set("Authorization", "Bearer "+token)
	inspectRequest.Header.Set("Content-Type", writer.FormDataContentType())
	inspect := httptest.NewRecorder()
	app.ServeHTTP(inspect, inspectRequest)
	if inspect.Code != http.StatusOK {
		t.Fatalf("full backup inspect failed: %d %s", inspect.Code, inspect.Body.String())
	}
	var inspection map[string]any
	if err := json.Unmarshal(inspect.Body.Bytes(), &inspection); err != nil {
		t.Fatal(err)
	}
	restoreToken, _ := inspection["token"].(string)
	if restoreToken == "" {
		t.Fatalf("full backup restore token missing: %#v", inspection)
	}
	restoreBody, _ := json.Marshal(map[string]string{"token": restoreToken, "confirmation": restoreConfirm})
	restored := authenticatedRequest(t, app, token, http.MethodPost, "/api/v1/utils/db/restore", string(restoreBody))
	if restored.Code != http.StatusOK {
		t.Fatalf("full backup restore failed: %d %s", restored.Code, restored.Body.String())
	}
	content, err := os.ReadFile(proofPath)
	if err != nil || string(content) != "UPLOAD_RESTORE_OK" {
		t.Fatalf("restored upload missing or changed: %q %v", content, err)
	}
}

func TestSCIMLifecycle(t *testing.T) {
	t.Setenv("ENABLE_SCIM", "true")
	t.Setenv("SCIM_AUTH_BEARER_TOKEN", "scim-test-token")
	app := testApp(t)
	create := httptest.NewRequest(http.MethodPost, "/scim/v2/Users", bytes.NewBufferString(`{"userName":"scim@example.com","displayName":"SCIM User","externalId":"ext-1","active":true}`))
	create.Header.Set("Authorization", "Bearer scim-test-token")
	create.Header.Set("Content-Type", "application/scim+json")
	created := httptest.NewRecorder()
	app.ServeHTTP(created, create)
	if created.Code != http.StatusCreated {
		t.Fatalf("SCIM user create failed: %d %s", created.Code, created.Body.String())
	}
	var user map[string]any
	if err := json.NewDecoder(created.Body).Decode(&user); err != nil {
		t.Fatal(err)
	}
	id, _ := user["id"].(string)
	if id == "" || user["externalId"] != "ext-1" {
		t.Fatalf("invalid SCIM user: %#v", user)
	}
	list := httptest.NewRequest(http.MethodGet, "/scim/v2/Users?filter=userName%20eq%20%22scim@example.com%22", nil)
	list.Header.Set("Authorization", "Bearer scim-test-token")
	listed := httptest.NewRecorder()
	app.ServeHTTP(listed, list)
	if listed.Code != http.StatusOK || !strings.Contains(listed.Body.String(), id) {
		t.Fatalf("SCIM user list failed: %d %s", listed.Code, listed.Body.String())
	}
	patch := httptest.NewRequest(http.MethodPatch, "/scim/v2/Users/"+id, bytes.NewBufferString(`{"Operations":[{"op":"Replace","path":"active","value":false}]}`))
	patch.Header.Set("Authorization", "Bearer scim-test-token")
	patch.Header.Set("Content-Type", "application/scim+json")
	patched := httptest.NewRecorder()
	app.ServeHTTP(patched, patch)
	if patched.Code != http.StatusOK || strings.Contains(patched.Body.String(), `"active":true`) {
		t.Fatalf("SCIM user patch failed: %d %s", patched.Code, patched.Body.String())
	}
	groupRequest := httptest.NewRequest(http.MethodPost, "/scim/v2/Groups", bytes.NewBufferString(`{"displayName":"SCIM Group","members":[{"value":"`+id+`"}]}`))
	groupRequest.Header.Set("Authorization", "Bearer scim-test-token")
	groupRequest.Header.Set("Content-Type", "application/scim+json")
	groupResponse := httptest.NewRecorder()
	app.ServeHTTP(groupResponse, groupRequest)
	if groupResponse.Code != http.StatusCreated || !strings.Contains(groupResponse.Body.String(), "SCIM Group") {
		t.Fatalf("SCIM group create failed: %d %s", groupResponse.Code, groupResponse.Body.String())
	}
	unauthenticated := httptest.NewRecorder()
	app.ServeHTTP(unauthenticated, httptest.NewRequest(http.MethodGet, "/scim/v2/Users", nil))
	if unauthenticated.Code != http.StatusUnauthorized {
		t.Fatalf("SCIM accepted missing token: %d", unauthenticated.Code)
	}
}

func TestExternalGatewayRoutesByClientKey(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/models" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":[{"id":"allowed-model"},{"id":"blocked-model"}]}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"gateway-chat","choices":[{"message":{"role":"assistant","content":"gateway-ok"},"finish_reason":"stop"}]}`))
	}))
	defer upstream.Close()
	app := testApp(t)
	adminToken := signupToken(t, app)
	app.config.OpenAIBaseURL = upstream.URL
	app.config.OpenAIAPIKey = "owner-provider-key"
	authenticatedJSON(t, app, adminToken, http.MethodPost, "/api/v1/external_api/config", `{"enabled":true,"openai":true,"anthropic":false}`)
	client := authenticatedJSON(t, app, adminToken, http.MethodPost, "/api/v1/external_api/clients", `{"name":"Gateway client","allowed_protocols":["openai"],"allowed_model_ids":["allowed-model"],"allow_tools":false}`)
	key, _ := client["api_key"].(string)
	if !strings.HasPrefix(key, "hwg-") {
		t.Fatalf("external key was not generated: %#v", client)
	}
	listedClients := authenticatedRequest(t, app, adminToken, http.MethodGet, "/api/v1/external_api/clients", "")
	if strings.Contains(listedClients.Body.String(), "api_key_hash") || strings.Contains(listedClients.Body.String(), key) {
		t.Fatalf("external key material leaked: %s", listedClients.Body.String())
	}
	modelsRequest := httptest.NewRequest(http.MethodGet, "/api/v1/external_api/gateway/openai/v1/models", nil)
	modelsRequest.Header.Set("Authorization", "Bearer "+key)
	models := httptest.NewRecorder()
	app.ServeHTTP(models, modelsRequest)
	if models.Code != http.StatusOK || strings.Contains(models.Body.String(), "blocked-model") || !strings.Contains(models.Body.String(), "allowed-model") {
		t.Fatalf("gateway model filtering failed: %d %s", models.Code, models.Body.String())
	}
	chatRequest := httptest.NewRequest(http.MethodPost, "/api/v1/external_api/gateway/openai/v1/chat/completions", bytes.NewBufferString(`{"model":"allowed-model","messages":[{"role":"user","content":"hello"}]}`))
	chatRequest.Header.Set("Authorization", "Bearer "+key)
	chatRequest.Header.Set("Content-Type", "application/json")
	chat := httptest.NewRecorder()
	app.ServeHTTP(chat, chatRequest)
	if chat.Code != http.StatusOK || !strings.Contains(chat.Body.String(), "gateway-ok") {
		t.Fatalf("gateway chat failed: %d %s", chat.Code, chat.Body.String())
	}
	logs := authenticatedRequest(t, app, adminToken, http.MethodGet, "/api/v1/external_api/logs", "")
	if logs.Code != http.StatusOK || !strings.Contains(logs.Body.String(), "allowed-model") || !strings.Contains(logs.Body.String(), "status_code") {
		t.Fatalf("gateway audit log missing: %d %s", logs.Code, logs.Body.String())
	}
}

func TestExternalGatewayEnforcesClientRateLimit(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer upstream.Close()
	app := testApp(t)
	adminToken := signupToken(t, app)
	app.config.OpenAIBaseURL = upstream.URL
	authenticatedJSON(t, app, adminToken, http.MethodPost, "/api/v1/external_api/config", `{"enabled":true,"openai":true}`)
	created := authenticatedJSON(t, app, adminToken, http.MethodPost, "/api/v1/external_api/clients", `{"name":"Limited","allowed_protocols":["openai"],"allowed_model_ids":[],"rpm_limit":1,"enabled":true}`)
	key, _ := created["api_key"].(string)

	request := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/external_api/gateway/openai/v1/models", nil)
		req.Header.Set("Authorization", "Bearer "+key)
		response := httptest.NewRecorder()
		app.ServeHTTP(response, req)
		return response
	}
	first := request()
	if first.Code != http.StatusOK || first.Header().Get("X-RateLimit-Remaining") != "0" {
		t.Fatalf("first limited request failed: %d %#v %s", first.Code, first.Header(), first.Body.String())
	}
	second := request()
	if second.Code != http.StatusTooManyRequests || second.Header().Get("Retry-After") == "" {
		t.Fatalf("rate limit was not enforced: %d %#v %s", second.Code, second.Header(), second.Body.String())
	}
}

func TestTaskRegistryListsAndCancelsOwnedTask(t *testing.T) {
	app := testApp(t)
	token := signupToken(t, app)
	identityRequest := httptest.NewRequest(http.MethodGet, "/", nil)
	identityRequest.Header.Set("Authorization", "Bearer "+token)
	user, ok := app.currentUser(identityRequest)
	if !ok {
		t.Fatal("failed to resolve test user")
	}
	id, taskContext, finish := app.beginTask(identityRequest.Context(), user.ID, "chat-task", true)
	defer finish()
	list := authenticatedJSON(t, app, token, http.MethodGet, "/api/tasks/chat/chat-task", "")
	ids, _ := list["task_ids"].([]any)
	if len(ids) != 1 || ids[0] != id {
		t.Fatalf("task registry did not list active task: %#v", list)
	}
	stopped := authenticatedRequest(t, app, token, http.MethodPost, "/api/tasks/stop/"+id, "")
	if stopped.Code != http.StatusOK {
		t.Fatalf("task cancellation failed: %d %s", stopped.Code, stopped.Body.String())
	}
	select {
	case <-taskContext.Done():
	default:
		t.Fatal("task context was not cancelled")
	}
	missing := authenticatedRequest(t, app, token, http.MethodPost, "/api/tasks/stop/"+id, "")
	if missing.Code != http.StatusNotFound {
		t.Fatalf("missing task returned false success: %d %s", missing.Code, missing.Body.String())
	}
}

func TestAnalyticsAggregatesChatMessages(t *testing.T) {
	app := testApp(t)
	token := signupToken(t, app)
	now := time.Now().Unix()
	authenticatedJSON(t, app, token, http.MethodPost, "/api/v1/chats/new", fmt.Sprintf(`{"chat":{"title":"Usage","messages":[{"id":"u1","role":"user","content":"hello","created_at":%d},{"id":"a1","role":"assistant","content":"hi","model":"usage-model","created_at":%d,"usage":{"prompt_tokens":3,"completion_tokens":5}}]}}`, now, now))
	models := authenticatedRequest(t, app, token, http.MethodGet, "/api/v1/analytics/models?days=1", "")
	if models.Code != http.StatusOK || !strings.Contains(models.Body.String(), "usage-model") || !strings.Contains(models.Body.String(), `"total_tokens":8`) {
		t.Fatalf("model analytics failed: %d %s", models.Code, models.Body.String())
	}
	cleanup := authenticatedJSON(t, app, token, http.MethodPost, "/api/v1/analytics/cleanup", `{"models":["usage-model"],"days":1,"dry_run":false}`)
	if cleanup["deleted_rows"] != float64(1) {
		t.Fatalf("analytics cleanup did not persist exclusion: %#v", cleanup)
	}
	after := authenticatedRequest(t, app, token, http.MethodGet, "/api/v1/analytics/models?days=1", "")
	if strings.Contains(after.Body.String(), "usage-model") {
		t.Fatalf("cleaned analytics remained visible: %s", after.Body.String())
	}
}

func TestTerminalSQLiteReadOnlyContracts(t *testing.T) {
	app := testApp(t)
	app.config.EnableTerminal = true
	token := signupToken(t, app)
	workspace := app.terminalRoot()
	if err := os.MkdirAll(workspace, 0o700); err != nil {
		t.Fatal(err)
	}
	backup, cleanup, err := app.createBackup(backupKindSQLite)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if err := copyFile(backup, filepath.Join(workspace, "sample.db")); err != nil {
		t.Fatal(err)
	}
	tables := authenticatedRequest(t, app, token, http.MethodGet, "/api/v1/terminal/sqlite/tables?path=sample.db", "")
	if tables.Code != http.StatusOK || !strings.Contains(tables.Body.String(), `"name":"user"`) {
		t.Fatalf("sqlite tables failed: %d %s", tables.Code, tables.Body.String())
	}
	query := authenticatedJSON(t, app, token, http.MethodPost, "/api/v1/terminal/sqlite/query", `{"path":"sample.db","query":"SELECT email FROM user","limit":10}`)
	if query["rowCount"] != float64(1) {
		t.Fatalf("sqlite query failed: %#v", query)
	}
	blocked := authenticatedRequest(t, app, token, http.MethodPost, "/api/v1/terminal/sqlite/query", `{"path":"sample.db","query":"DELETE FROM user","limit":10}`)
	if blocked.Code != http.StatusBadRequest {
		t.Fatalf("sqlite mutation was accepted: %d %s", blocked.Code, blocked.Body.String())
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
