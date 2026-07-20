package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/iwvw-per/HaloWebUI/backend/internal/auth"
	"github.com/iwvw-per/HaloWebUI/backend/internal/store"
)

type nativeProviderConfig struct {
	Enabled bool
	URLs    []string
	Keys    []string
	Configs map[string]any
}

func providerPrefix(provider string) string {
	return strings.ToUpper(provider)
}

func (a *App) loadNativeProviderConfig(r *http.Request, provider string) (nativeProviderConfig, error) {
	resource, err := a.store.ResourceByKey(r.Context(), "global_setting", "provider/"+provider)
	if errors.Is(err, store.ErrResourceNotFound) {
		return nativeProviderConfig{Configs: map[string]any{}}, nil
	}
	if err != nil {
		return nativeProviderConfig{}, err
	}
	var payload map[string]any
	if json.Unmarshal(resource.Body, &payload) != nil {
		return nativeProviderConfig{}, errors.New("invalid provider config")
	}
	prefix := providerPrefix(provider)
	config := nativeProviderConfig{
		URLs: stringSlice(payload[prefix+"_API_BASE_URLS"]),
		Keys: stringSlice(payload[prefix+"_API_KEYS"]),
	}
	config.Enabled, _ = payload["ENABLE_"+prefix+"_API"].(bool)
	config.Configs, _ = payload[prefix+"_API_CONFIGS"].(map[string]any)
	if config.Configs == nil {
		config.Configs = map[string]any{}
	}
	for len(config.Keys) < len(config.URLs) {
		config.Keys = append(config.Keys, "")
	}
	if len(config.Keys) > len(config.URLs) {
		config.Keys = config.Keys[:len(config.URLs)]
	}
	return config, nil
}

func nativeProviderConfigPayload(provider string, config nativeProviderConfig) map[string]any {
	prefix := providerPrefix(provider)
	return map[string]any{
		"ENABLE_" + prefix + "_API": config.Enabled,
		prefix + "_API_BASE_URLS":   config.URLs,
		prefix + "_API_KEYS":        config.Keys,
		prefix + "_API_CONFIGS":     config.Configs,
	}
}

func (a *App) saveNativeProviderConfig(r *http.Request, provider string, config nativeProviderConfig) error {
	resource, err := a.store.ResourceByKey(r.Context(), "global_setting", "provider/"+provider)
	if errors.Is(err, store.ErrResourceNotFound) {
		resource = store.Resource{Kind: "global_setting", ID: auth.RandomIDForInternalUse(), UserID: "system", Key: "provider/" + provider, Active: true}
	} else if err != nil {
		return err
	}
	resource.Body, _ = json.Marshal(nativeProviderConfigPayload(provider, config))
	_, err = a.store.PutResource(r.Context(), resource)
	return err
}

func (a *App) handleNativeProviderConfig(provider string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if ok, _ := a.requireAdmin(w, r); !ok {
			return
		}
		config, err := a.loadNativeProviderConfig(r, provider)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to load provider config")
			return
		}
		if r.Method == http.MethodPost {
			var payload map[string]any
			if !decodeJSON(w, r, &payload) {
				return
			}
			prefix := providerPrefix(provider)
			if enabled, ok := payload["ENABLE_"+prefix+"_API"].(bool); ok {
				config.Enabled = enabled
			}
			if value, ok := payload[prefix+"_API_BASE_URLS"]; ok {
				config.URLs = stringSlice(value)
			}
			if value, ok := payload[prefix+"_API_KEYS"]; ok {
				config.Keys = stringSlice(value)
			}
			if value, ok := payload[prefix+"_API_CONFIGS"].(map[string]any); ok {
				config.Configs = value
			}
			for index, value := range config.URLs {
				config.URLs[index] = strings.TrimRight(strings.TrimSpace(value), "/")
			}
			for len(config.Keys) < len(config.URLs) {
				config.Keys = append(config.Keys, "")
			}
			if len(config.Keys) > len(config.URLs) {
				config.Keys = config.Keys[:len(config.URLs)]
			}
			if err := a.saveNativeProviderConfig(r, provider, config); err != nil {
				writeError(w, http.StatusInternalServerError, "failed to save provider config")
				return
			}
		}
		writeJSON(w, http.StatusOK, nativeProviderConfigPayload(provider, config))
	}
}

func (a *App) providerConnection(r *http.Request, user store.User, provider string, index int) (string, string, map[string]any, bool) {
	if index < 0 {
		if baseURL, key, config, ok := a.accountProviderForUser(r, user, provider); ok {
			return baseURL, key, config, true
		}
	}
	global, err := a.loadNativeProviderConfig(r, provider)
	if err != nil || !global.Enabled || len(global.URLs) == 0 {
		return "", "", nil, false
	}
	if index < 0 {
		index = 0
	}
	if index >= len(global.URLs) {
		return "", "", nil, false
	}
	key := ""
	if index < len(global.Keys) {
		key = global.Keys[index]
	}
	config, _ := global.Configs[strconv.Itoa(index)].(map[string]any)
	return global.URLs[index], key, config, true
}

func providerRequest(ctx context.Context, provider, method, target, key string, body io.Reader) (*http.Response, error) {
	request, err := http.NewRequestWithContext(ctx, method, target, body)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", "application/json")
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	switch provider {
	case "anthropic":
		request.Header.Set("x-api-key", key)
		request.Header.Set("anthropic-version", "2023-06-01")
	case "gemini":
		request.Header.Set("x-goog-api-key", key)
	default:
		if key != "" {
			request.Header.Set("Authorization", "Bearer "+key)
		}
	}
	client := &http.Client{Timeout: 10 * time.Minute}
	return client.Do(request)
}

func providerTarget(baseURL, suffix string) (string, error) {
	target, err := url.Parse(strings.TrimRight(baseURL, "/") + suffix)
	if err != nil || (target.Scheme != "http" && target.Scheme != "https") || target.Host == "" {
		return "", errors.New("provider URL is invalid")
	}
	return target.String(), nil
}

func (a *App) fetchNativeProviderModels(r *http.Request, provider, baseURL, key string) (map[string]any, error) {
	target, err := providerTarget(baseURL, "/models")
	if err != nil {
		return nil, err
	}
	response, err := providerRequest(r.Context(), provider, http.MethodGet, target, key, nil)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, maxProviderRequestBytes))
	if err != nil {
		return nil, err
	}
	if response.StatusCode >= 400 {
		return nil, errors.New("provider returned " + response.Status)
	}
	var raw map[string]any
	if json.Unmarshal(body, &raw) != nil {
		return nil, errors.New("provider returned invalid JSON")
	}
	if provider == "gemini" {
		models, _ := raw["models"].([]any)
		data := make([]map[string]any, 0, len(models))
		for _, rawModel := range models {
			model, _ := rawModel.(map[string]any)
			name, _ := model["name"].(string)
			id := strings.TrimPrefix(name, "models/")
			if id == "" {
				continue
			}
			entry := map[string]any{"id": id, "name": id, "owned_by": "google"}
			if display, _ := model["displayName"].(string); display != "" {
				entry["name"] = display
			}
			data = append(data, entry)
		}
		return map[string]any{"data": data}, nil
	}
	models, _ := raw["data"].([]any)
	data := make([]map[string]any, 0, len(models))
	for _, rawModel := range models {
		model, _ := rawModel.(map[string]any)
		id, _ := model["id"].(string)
		if id == "" {
			continue
		}
		name := id
		if display, _ := model["display_name"].(string); display != "" {
			name = display
		}
		owner := provider
		if provider == "grok" {
			owner = "xai"
		}
		data = append(data, map[string]any{"id": id, "name": name, "owned_by": owner})
	}
	return map[string]any{"data": data}, nil
}

func (a *App) handleNativeProviderModels(provider string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := a.requireUser(w, r)
		if !ok {
			return
		}
		index := -1
		if value := r.PathValue("index"); value != "" {
			parsed, err := strconv.Atoi(value)
			if err != nil || parsed < 0 {
				writeError(w, http.StatusBadRequest, "invalid connection index")
				return
			}
			index = parsed
		}
		baseURL, key, _, found := a.providerConnection(r, user, provider, index)
		if !found {
			writeJSON(w, http.StatusOK, map[string]any{"data": []any{}})
			return
		}
		models, err := a.fetchNativeProviderModels(r, provider, baseURL, key)
		if err != nil {
			writeError(w, http.StatusBadGateway, provider+" models request failed")
			return
		}
		writeJSON(w, http.StatusOK, models)
	}
}

func (a *App) handleNativeProviderVerify(provider string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := a.requireUser(w, r); !ok {
			return
		}
		var form struct {
			URL string `json:"url"`
			Key string `json:"key"`
		}
		if !decodeJSON(w, r, &form) {
			return
		}
		models, err := a.fetchNativeProviderModels(r, provider, strings.TrimRight(form.URL, "/"), form.Key)
		if err != nil {
			writeError(w, http.StatusBadGateway, provider+" connection failed")
			return
		}
		if provider == "grok" {
			writeJSON(w, http.StatusOK, map[string]any{"models": models["data"]})
			return
		}
		writeJSON(w, http.StatusOK, models)
	}
}

func (a *App) handleNativeProviderChat(provider string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := a.requireUser(w, r)
		if !ok {
			return
		}
		baseURL, key, config, found := a.providerConnection(r, user, provider, -1)
		if !found {
			writeError(w, http.StatusServiceUnavailable, provider+" provider is not configured")
			return
		}
		if provider == "grok" {
			a.proxyProvider(w, r, baseURL, key, "/chat/completions")
			return
		}
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxProviderRequestBytes))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid chat request")
			return
		}
		if provider == "anthropic" {
			a.proxyAnthropicChat(w, r, baseURL, key, config, body)
			return
		}
		a.proxyGeminiChat(w, r, baseURL, key, config, body)
	}
}

func decodeProviderError(response *http.Response) string {
	body, _ := io.ReadAll(io.LimitReader(response.Body, 256*1024))
	var payload map[string]any
	if json.Unmarshal(body, &payload) == nil {
		if detail, _ := payload["detail"].(string); detail != "" {
			return detail
		}
		if value, _ := payload["error"].(map[string]any); value != nil {
			if message, _ := value["message"].(string); message != "" {
				return message
			}
		}
	}
	return strings.TrimSpace(string(body))
}

func postNativeProvider(ctx context.Context, provider, target, key string, payload any) (*http.Response, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return providerRequest(ctx, provider, http.MethodPost, target, key, bytes.NewReader(body))
}
