package server

import (
	"encoding/json"
	"net/http"
	"strings"
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
			writeJSON(w, http.StatusOK, body)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"ENABLE_OPENAI_API":    baseURL != "",
			"OPENAI_API_BASE_URLS": []string{baseURL},
			"OPENAI_API_KEYS":      []string{apiKey},
			"OPENAI_API_CONFIGS":   map[string]any{},
		})
	case "/urls":
		writeJSON(w, http.StatusOK, map[string]any{"OPENAI_API_BASE_URLS": []string{baseURL}})
	case "/keys":
		writeJSON(w, http.StatusOK, map[string]any{"OPENAI_API_KEYS": []string{apiKey}})
	case "/urls/update", "/keys/update":
		var body map[string]any
		if !decodeJSON(w, r, &body) {
			return
		}
		if path == "/urls/update" {
			writeJSON(w, http.StatusOK, map[string]any{"OPENAI_API_BASE_URLS": body["urls"]})
		} else {
			writeJSON(w, http.StatusOK, map[string]any{"OPENAI_API_KEYS": body["keys"]})
		}
	case "/verify", "/health_check":
		a.verifyProviderFromBody(w, r, "/models")
	default:
		a.proxyProvider(w, r, baseURL, apiKey, path)
	}
}

func (a *App) handleOllamaCompatibility(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.requireUser(w, r); !ok {
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/ollama")
	switch path {
	case "/config", "/config/update":
		if r.Method == http.MethodPost {
			var body map[string]any
			if !decodeJSON(w, r, &body) {
				return
			}
			writeJSON(w, http.StatusOK, body)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"ENABLE_OLLAMA_API":  a.config.OllamaBaseURL != "",
			"OLLAMA_BASE_URLS":   []string{a.config.OllamaBaseURL},
			"OLLAMA_API_CONFIGS": map[string]any{},
		})
	case "/urls":
		writeJSON(w, http.StatusOK, map[string]any{"OLLAMA_BASE_URLS": []string{a.config.OllamaBaseURL}})
	case "/urls/update":
		var body map[string]any
		if !decodeJSON(w, r, &body) {
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"OLLAMA_BASE_URLS": body["urls"]})
	case "/verify", "/health_check":
		a.verifyProviderFromBody(w, r, "/api/tags")
	default:
		a.proxyProvider(w, r, a.config.OllamaBaseURL, a.config.OllamaAPIKey, path)
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
