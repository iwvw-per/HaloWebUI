package server

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/iwvw-per/HaloWebUI/backend/internal/store"
)

func (a *App) handleOpenAICompatibility(w http.ResponseWriter, r *http.Request) {
	user, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	baseURL, apiKey := a.openAIProviderForUser(r, user)
	path := strings.TrimPrefix(r.URL.Path, "/openai")
	switch path {
	case "/config", "/config/update":
		if r.Method == http.MethodPost {
			var body map[string]any
			if !decodeJSON(w, r, &body) {
				return
			}
			if err := a.saveUserOpenAIConfig(r, user, body); err != nil {
				writeError(w, http.StatusInternalServerError, "failed to save OpenAI connection")
				return
			}
			writeJSON(w, http.StatusOK, body)
			return
		}
		writeJSON(w, http.StatusOK, a.userOpenAIConfig(r, user, baseURL, apiKey))
	case "/urls":
		config := a.userOpenAIConfig(r, user, baseURL, apiKey)
		writeJSON(w, http.StatusOK, map[string]any{"OPENAI_API_BASE_URLS": config["OPENAI_API_BASE_URLS"]})
	case "/keys":
		config := a.userOpenAIConfig(r, user, baseURL, apiKey)
		writeJSON(w, http.StatusOK, map[string]any{"OPENAI_API_KEYS": config["OPENAI_API_KEYS"]})
	case "/urls/update", "/keys/update":
		var body map[string]any
		if !decodeJSON(w, r, &body) {
			return
		}
		config := a.userOpenAIConfig(r, user, baseURL, apiKey)
		if path == "/urls/update" {
			config["OPENAI_API_BASE_URLS"] = body["urls"]
			if err := a.saveUserOpenAIConfig(r, user, config); err != nil {
				writeError(w, http.StatusInternalServerError, "failed to save OpenAI URLs")
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"OPENAI_API_BASE_URLS": body["urls"]})
		} else {
			config["OPENAI_API_KEYS"] = body["keys"]
			if err := a.saveUserOpenAIConfig(r, user, config); err != nil {
				writeError(w, http.StatusInternalServerError, "failed to save OpenAI keys")
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"OPENAI_API_KEYS": body["keys"]})
		}
	case "/verify", "/health_check":
		a.verifyProviderFromBody(w, r, "/models")
	default:
		a.proxyProvider(w, r, baseURL, apiKey, path)
	}
}

func (a *App) userOpenAIConfig(r *http.Request, user store.User, fallbackURL, fallbackKey string) map[string]any {
	defaults := map[string]any{
		"ENABLE_OPENAI_API":    fallbackURL != "",
		"OPENAI_API_BASE_URLS": []string{fallbackURL},
		"OPENAI_API_KEYS":      []string{fallbackKey},
		"OPENAI_API_CONFIGS":   map[string]any{},
	}
	return a.userProviderConfig(r, user, "openai", defaults)
}

func (a *App) userProviderConfig(r *http.Request, user store.User, provider string, defaults map[string]any) map[string]any {
	raw, err := a.store.UserSettings(r.Context(), user.ID)
	if err != nil {
		return defaults
	}
	var root map[string]any
	if json.Unmarshal(raw, &root) != nil {
		return defaults
	}
	connections, ok := nestedMap(root, "ui", "connections")
	if !ok {
		connections, _ = nestedMap(root, "connections")
	}
	stored, _ := connections[provider].(map[string]any)
	if len(stored) == 0 {
		return defaults
	}
	for key, value := range stored {
		defaults[key] = value
	}
	return defaults
}

func (a *App) saveUserOpenAIConfig(r *http.Request, user store.User, config map[string]any) error {
	return a.saveUserProviderConfig(r, user, "openai", config)
}

func (a *App) saveUserProviderConfig(r *http.Request, user store.User, provider string, config map[string]any) error {
	raw, err := a.store.UserSettings(r.Context(), user.ID)
	if err != nil {
		return err
	}
	root := map[string]any{}
	_ = json.Unmarshal(raw, &root)
	ui, ok := root["ui"].(map[string]any)
	if !ok {
		ui = map[string]any{}
		root["ui"] = ui
	}
	connections, ok := ui["connections"].(map[string]any)
	if !ok {
		connections = map[string]any{}
		ui["connections"] = connections
	}
	connections[provider] = config
	revision := int64(0)
	if value, ok := root["revision"].(float64); ok {
		revision = int64(value)
	}
	root["revision"] = revision + 1
	encoded, err := json.Marshal(root)
	if err != nil {
		return err
	}
	_, err = a.store.SetUserSettings(r.Context(), user.ID, encoded)
	return err
}

func (a *App) handleOllamaCompatibility(w http.ResponseWriter, r *http.Request) {
	user, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	baseURL, apiKey := a.ollamaProviderForUser(r, user, -1)
	path := strings.TrimPrefix(r.URL.Path, "/ollama")
	switch path {
	case "/config", "/config/update":
		if r.Method == http.MethodPost {
			var body map[string]any
			if !decodeJSON(w, r, &body) {
				return
			}
			if err := a.saveUserProviderConfig(r, user, "ollama", body); err != nil {
				writeError(w, http.StatusInternalServerError, "failed to save Ollama connection")
				return
			}
			writeJSON(w, http.StatusOK, body)
			return
		}
		writeJSON(w, http.StatusOK, a.userProviderConfig(r, user, "ollama", map[string]any{
			"ENABLE_OLLAMA_API":  baseURL != "",
			"OLLAMA_BASE_URLS":   []string{baseURL},
			"OLLAMA_API_CONFIGS": map[string]any{},
		}))
	case "/urls":
		config := a.userProviderConfig(r, user, "ollama", map[string]any{"OLLAMA_BASE_URLS": []string{baseURL}})
		writeJSON(w, http.StatusOK, map[string]any{"OLLAMA_BASE_URLS": config["OLLAMA_BASE_URLS"]})
	case "/urls/update":
		var body map[string]any
		if !decodeJSON(w, r, &body) {
			return
		}
		config := a.userProviderConfig(r, user, "ollama", map[string]any{"OLLAMA_BASE_URLS": []string{baseURL}})
		config["OLLAMA_BASE_URLS"] = body["urls"]
		if err := a.saveUserProviderConfig(r, user, "ollama", config); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to save Ollama URLs")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"OLLAMA_BASE_URLS": body["urls"]})
	case "/verify", "/health_check":
		a.verifyProviderFromBody(w, r, "/api/tags")
	default:
		index := -1
		upstreamPath := path
		parts := splitPath(path)
		if len(parts) == 3 && parts[0] == "api" && (parts[1] == "tags" || parts[1] == "version") {
			parsed, err := strconv.Atoi(parts[2])
			if err != nil || parsed < 0 {
				writeError(w, http.StatusBadRequest, "invalid connection index")
				return
			}
			index = parsed
			upstreamPath = "/api/" + parts[1]
		}
		if index >= 0 {
			baseURL, apiKey = a.ollamaProviderForUser(r, user, index)
		}
		a.proxyProvider(w, r, baseURL, apiKey, upstreamPath)
	}
}

func (a *App) verifyProviderFromBody(w http.ResponseWriter, r *http.Request, suffix string) {
	var body struct {
		URL string `json:"url"`
		Key string `json:"key"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	body.URL = strings.TrimRight(strings.TrimSpace(body.URL), "/")
	if body.URL == "" {
		writeError(w, http.StatusBadRequest, "provider URL is required")
		return
	}
	payload, err := a.fetchProviderJSON(r, body.URL, body.Key, suffix)
	if err != nil {
		writeError(w, http.StatusBadGateway, "provider connection failed")
		return
	}
	var result any
	if json.Unmarshal(payload, &result) != nil {
		result = map[string]bool{"status": true}
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *App) handleDisabledProvider(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := a.requireUser(w, r); !ok {
			return
		}
		path := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/"+name), "/")
		if r.Method == http.MethodGet && (path == "/config" || path == "") {
			writeJSON(w, http.StatusOK, map[string]any{
				"enabled":  false,
				"provider": name,
				"detail":   "provider adapter is not configured",
			})
			return
		}
		writeError(w, http.StatusServiceUnavailable, name+" provider is not configured")
	}
}
