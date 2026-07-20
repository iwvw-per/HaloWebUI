package server

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/iwvw-per/HaloWebUI/backend/internal/store"
)

type userConnectionSettings struct {
	UI struct {
		Connections userConnections `json:"connections"`
	} `json:"ui"`
	Connections userConnections `json:"connections"`
}

type userConnections struct {
	OpenAI struct {
		Enabled  *bool                               `json:"ENABLE_OPENAI_API"`
		BaseURLs []string                            `json:"OPENAI_API_BASE_URLS"`
		APIKeys  []string                            `json:"OPENAI_API_KEYS"`
		Configs  map[string]openAIConnectionSettings `json:"OPENAI_API_CONFIGS"`
	} `json:"openai"`
	Ollama struct {
		Enabled  *bool                               `json:"ENABLE_OLLAMA_API"`
		BaseURLs []string                            `json:"OLLAMA_BASE_URLS"`
		APIKeys  []string                            `json:"OLLAMA_API_KEYS"`
		Configs  map[string]openAIConnectionSettings `json:"OLLAMA_API_CONFIGS"`
	} `json:"ollama"`
	Gemini    genericUserProvider `json:"gemini"`
	Grok      genericUserProvider `json:"grok"`
	Anthropic genericUserProvider `json:"anthropic"`
}

type genericUserProvider struct {
	GeminiBaseURLs    []string                            `json:"GEMINI_API_BASE_URLS"`
	GeminiAPIKeys     []string                            `json:"GEMINI_API_KEYS"`
	GeminiConfigs     map[string]openAIConnectionSettings `json:"GEMINI_API_CONFIGS"`
	GrokBaseURLs      []string                            `json:"GROK_API_BASE_URLS"`
	GrokAPIKeys       []string                            `json:"GROK_API_KEYS"`
	GrokConfigs       map[string]openAIConnectionSettings `json:"GROK_API_CONFIGS"`
	AnthropicBaseURLs []string                            `json:"ANTHROPIC_API_BASE_URLS"`
	AnthropicAPIKeys  []string                            `json:"ANTHROPIC_API_KEYS"`
	AnthropicConfigs  map[string]openAIConnectionSettings `json:"ANTHROPIC_API_CONFIGS"`
}

type openAIConnectionSettings struct {
	Enable     *bool `json:"enable"`
	APIKeyPool struct {
		Keys []struct {
			Key     string `json:"key"`
			Enabled *bool  `json:"enabled"`
		} `json:"keys"`
	} `json:"api_key_pool"`
}

// openAIProviderForUser makes the per-account connection page authoritative.
// Environment values remain a deployment-level fallback for fresh accounts.
func (a *App) openAIProviderForUser(request *http.Request, user store.User) (string, string) {
	baseURL, apiKey := a.config.OpenAIBaseURL, a.config.OpenAIAPIKey
	raw, err := a.store.UserSettings(request.Context(), user.ID)
	if err != nil {
		return baseURL, apiKey
	}
	var settings userConnectionSettings
	if json.Unmarshal(raw, &settings) != nil {
		return baseURL, apiKey
	}
	connections := settings.UI.Connections
	if len(connections.OpenAI.BaseURLs) == 0 {
		connections = settings.Connections
	}
	if connections.OpenAI.Enabled != nil && !*connections.OpenAI.Enabled {
		return "", ""
	}
	for index, candidate := range connections.OpenAI.BaseURLs {
		candidate = strings.TrimRight(strings.TrimSpace(candidate), "/")
		if candidate == "" {
			continue
		}
		connection := connections.OpenAI.Configs[strconv.Itoa(index)]
		if connection.Enable != nil && !*connection.Enable {
			continue
		}
		candidateKey := ""
		if index < len(connections.OpenAI.APIKeys) {
			candidateKey = strings.TrimSpace(connections.OpenAI.APIKeys[index])
		}
		for _, pooled := range connection.APIKeyPool.Keys {
			if pooled.Enabled != nil && !*pooled.Enabled {
				continue
			}
			if key := strings.TrimSpace(pooled.Key); key != "" {
				candidateKey = key
				break
			}
		}
		return candidate, candidateKey
	}
	return baseURL, apiKey
}

func (a *App) ollamaProviderForUser(request *http.Request, user store.User, requestedIndex int) (string, string) {
	fallbackURL, fallbackKey := a.config.OllamaBaseURL, a.config.OllamaAPIKey
	raw, err := a.store.UserSettings(request.Context(), user.ID)
	if err != nil {
		return fallbackURL, fallbackKey
	}
	var settings userConnectionSettings
	if json.Unmarshal(raw, &settings) != nil {
		return fallbackURL, fallbackKey
	}
	connections := settings.UI.Connections
	if len(connections.Ollama.BaseURLs) == 0 {
		connections = settings.Connections
	}
	if len(connections.Ollama.BaseURLs) == 0 {
		return fallbackURL, fallbackKey
	}
	if connections.Ollama.Enabled != nil && !*connections.Ollama.Enabled {
		return "", ""
	}
	for index, candidate := range connections.Ollama.BaseURLs {
		if requestedIndex >= 0 && index != requestedIndex {
			continue
		}
		candidate = strings.TrimRight(strings.TrimSpace(candidate), "/")
		if candidate == "" {
			continue
		}
		connection := connections.Ollama.Configs[strconv.Itoa(index)]
		if connection.Enable != nil && !*connection.Enable {
			continue
		}
		key := ""
		if index < len(connections.Ollama.APIKeys) {
			key = strings.TrimSpace(connections.Ollama.APIKeys[index])
		}
		return candidate, key
	}
	return "", ""
}

func (a *App) accountProviderForUser(request *http.Request, user store.User, provider string) (string, string, map[string]any, bool) {
	raw, err := a.store.UserSettings(request.Context(), user.ID)
	if err != nil {
		return "", "", nil, false
	}
	var root map[string]any
	if json.Unmarshal(raw, &root) != nil {
		return "", "", nil, false
	}
	connections, _ := nestedMap(root, "ui", "connections")
	if len(connections) == 0 {
		connections, _ = nestedMap(root, "connections")
	}
	providerValue, _ := connections[provider].(map[string]any)
	if len(providerValue) == 0 {
		return "", "", nil, false
	}
	prefix := strings.ToUpper(provider)
	urls := stringSlice(providerValue[prefix+"_API_BASE_URLS"])
	keys := stringSlice(providerValue[prefix+"_API_KEYS"])
	configs, _ := providerValue[prefix+"_API_CONFIGS"].(map[string]any)
	for index, candidate := range urls {
		candidate = strings.TrimRight(strings.TrimSpace(candidate), "/")
		if candidate == "" {
			continue
		}
		config, _ := configs[strconv.Itoa(index)].(map[string]any)
		if enabled, ok := config["enable"].(bool); ok && !enabled {
			continue
		}
		key := ""
		if index < len(keys) {
			key = strings.TrimSpace(keys[index])
		}
		if pool, _ := config["api_key_pool"].(map[string]any); pool != nil {
			if entries, _ := pool["keys"].([]any); entries != nil {
				for _, rawEntry := range entries {
					entry, _ := rawEntry.(map[string]any)
					if enabled, ok := entry["enabled"].(bool); ok && !enabled {
						continue
					}
					if pooled, _ := entry["key"].(string); strings.TrimSpace(pooled) != "" {
						key = strings.TrimSpace(pooled)
						break
					}
				}
			}
		}
		return candidate, key, config, true
	}
	return "", "", nil, false
}

func nestedMap(root map[string]any, keys ...string) (map[string]any, bool) {
	current := root
	for _, key := range keys {
		next, ok := current[key].(map[string]any)
		if !ok {
			return nil, false
		}
		current = next
	}
	return current, true
}
