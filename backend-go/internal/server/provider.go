package server

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const maxProviderRequestBytes = 16 * 1024 * 1024

func (a *App) handleOpenAIChat(response http.ResponseWriter, request *http.Request) {
	if _, ok := a.requireUser(response, request); !ok {
		return
	}
	a.proxyProvider(response, request, a.config.OpenAIBaseURL, a.config.OpenAIAPIKey, "/chat/completions")
}

func (a *App) handleUnifiedChat(response http.ResponseWriter, request *http.Request) {
	if _, ok := a.requireUser(response, request); !ok {
		return
	}
	a.proxyProvider(response, request, a.config.OpenAIBaseURL, a.config.OpenAIAPIKey, "/chat/completions")
}

func (a *App) handleChatCompleted(response http.ResponseWriter, request *http.Request) {
	if _, ok := a.requireUser(response, request); !ok {
		return
	}
	request.Body = http.MaxBytesReader(response, request.Body, maxProviderRequestBytes)
	_, _ = io.Copy(io.Discard, request.Body)
	writeJSON(response, http.StatusOK, map[string]bool{"status": true})
}

func (a *App) handleOpenAIModels(response http.ResponseWriter, request *http.Request) {
	if _, ok := a.requireUser(response, request); !ok {
		return
	}
	a.proxyProvider(response, request, a.config.OpenAIBaseURL, a.config.OpenAIAPIKey, "/models")
}

func (a *App) handleOllamaChat(response http.ResponseWriter, request *http.Request) {
	if _, ok := a.requireUser(response, request); !ok {
		return
	}
	a.proxyProvider(response, request, a.config.OllamaBaseURL, a.config.OllamaAPIKey, "/api/chat")
}

func (a *App) handleOllamaTags(response http.ResponseWriter, request *http.Request) {
	if _, ok := a.requireUser(response, request); !ok {
		return
	}
	a.proxyProvider(response, request, a.config.OllamaBaseURL, a.config.OllamaAPIKey, "/api/tags")
}

func (a *App) handleModels(response http.ResponseWriter, request *http.Request) {
	if _, ok := a.requireUser(response, request); !ok {
		return
	}
	models := make([]map[string]any, 0)
	if payload, err := a.fetchProviderJSON(request, a.config.OpenAIBaseURL, a.config.OpenAIAPIKey, "/models"); err == nil {
		models = append(models, decodeProviderModels(payload)...)
	}
	if payload, err := a.fetchProviderJSON(request, a.config.OllamaBaseURL, a.config.OllamaAPIKey, "/api/tags"); err == nil {
		for _, model := range decodeProviderModels(payload) {
			if name, ok := model["name"].(string); ok {
				model["id"] = name
				model["object"] = "model"
			}
			models = append(models, model)
		}
	}
	writeJSON(response, http.StatusOK, map[string]any{"data": models})
}

func (a *App) fetchProviderJSON(request *http.Request, baseURL, apiKey, suffix string) ([]byte, error) {
	if strings.TrimSpace(baseURL) == "" {
		return nil, errors.New("provider is not configured")
	}
	target, err := url.Parse(strings.TrimRight(baseURL, "/") + suffix)
	if err != nil || target.Scheme == "" || target.Host == "" {
		return nil, errors.New("provider URL is invalid")
	}
	upstreamRequest, err := http.NewRequestWithContext(request.Context(), http.MethodGet, target.String(), nil)
	if err != nil {
		return nil, err
	}
	if apiKey != "" {
		upstreamRequest.Header.Set("Authorization", "Bearer "+apiKey)
	}
	client := &http.Client{Timeout: 10 * time.Second}
	upstreamResponse, err := client.Do(upstreamRequest)
	if err != nil {
		return nil, err
	}
	defer upstreamResponse.Body.Close()
	if upstreamResponse.StatusCode >= 400 {
		return nil, errors.New("provider returned an error")
	}
	return io.ReadAll(io.LimitReader(upstreamResponse.Body, maxProviderRequestBytes))
}

func (a *App) proxyProvider(response http.ResponseWriter, request *http.Request, baseURL, apiKey, suffix string) {
	if strings.TrimSpace(baseURL) == "" {
		writeError(response, http.StatusServiceUnavailable, "provider is not configured")
		return
	}
	target, err := url.Parse(strings.TrimRight(baseURL, "/") + suffix)
	if err != nil || target.Scheme == "" || target.Host == "" || (target.Scheme != "http" && target.Scheme != "https") {
		writeError(response, http.StatusServiceUnavailable, "provider URL is invalid")
		return
	}
	request.Body = http.MaxBytesReader(response, request.Body, maxProviderRequestBytes)
	upstreamRequest, err := http.NewRequestWithContext(request.Context(), request.Method, target.String(), request.Body)
	if err != nil {
		writeError(response, http.StatusBadGateway, "failed to create provider request")
		return
	}
	upstreamRequest.Header.Set("Accept", request.Header.Get("Accept"))
	upstreamRequest.Header.Set("Content-Type", request.Header.Get("Content-Type"))
	if apiKey != "" {
		upstreamRequest.Header.Set("Authorization", "Bearer "+apiKey)
	}
	for _, name := range []string{"X-Request-ID", "OpenAI-Organization", "OpenAI-Project"} {
		if value := request.Header.Get(name); value != "" {
			upstreamRequest.Header.Set(name, value)
		}
	}
	client := &http.Client{
		Timeout: 10 * time.Minute,
		Transport: &http.Transport{
			MaxIdleConns:        8,
			MaxIdleConnsPerHost: 4,
			IdleConnTimeout:     75 * time.Second,
			DisableCompression:  true,
		},
	}
	upstreamResponse, err := client.Do(upstreamRequest)
	if err != nil {
		if errors.Is(err, request.Context().Err()) {
			return
		}
		writeError(response, http.StatusBadGateway, "provider request failed")
		return
	}
	defer upstreamResponse.Body.Close()
	for name, values := range upstreamResponse.Header {
		if strings.EqualFold(name, "Content-Length") || strings.EqualFold(name, "Transfer-Encoding") || strings.EqualFold(name, "Connection") {
			continue
		}
		for _, value := range values {
			response.Header().Add(name, value)
		}
	}
	response.WriteHeader(upstreamResponse.StatusCode)
	if request.Method == http.MethodHead {
		return
	}
	controller := http.NewResponseController(response)
	buffer := make([]byte, 32*1024)
	for {
		read, readErr := upstreamResponse.Body.Read(buffer)
		if read > 0 {
			if _, writeErr := response.Write(buffer[:read]); writeErr != nil {
				return
			}
			_ = controller.Flush()
		}
		if readErr != nil {
			if readErr != io.EOF {
				return
			}
			return
		}
	}
}

func (a *App) handleProviderHealth(response http.ResponseWriter, request *http.Request) {
	if _, ok := a.requireUser(response, request); !ok {
		return
	}
	provider := request.URL.Query().Get("provider")
	baseURL, apiKey, suffix := a.config.OpenAIBaseURL, a.config.OpenAIAPIKey, "/models"
	if provider == "ollama" {
		baseURL, apiKey, suffix = a.config.OllamaBaseURL, a.config.OllamaAPIKey, "/api/tags"
	}
	probe := httptestRequest(request, strings.TrimRight(baseURL, "/")+suffix, apiKey)
	client := &http.Client{Timeout: 5 * time.Second}
	upstream, err := client.Do(probe)
	if err != nil {
		writeError(response, http.StatusBadGateway, "provider is unreachable")
		return
	}
	defer upstream.Body.Close()
	writeJSON(response, http.StatusOK, map[string]any{"status": upstream.StatusCode < 500, "provider": provider})
}

func httptestRequest(original *http.Request, target, apiKey string) *http.Request {
	probe, _ := http.NewRequestWithContext(original.Context(), http.MethodGet, target, nil)
	if apiKey != "" {
		probe.Header.Set("Authorization", "Bearer "+apiKey)
	}
	return probe
}

func decodeProviderModels(body []byte) []map[string]any {
	var payload struct {
		Data   []map[string]any `json:"data"`
		Models []map[string]any `json:"models"`
	}
	if json.Unmarshal(body, &payload) != nil {
		return nil
	}
	if payload.Data != nil {
		return payload.Data
	}
	return payload.Models
}
