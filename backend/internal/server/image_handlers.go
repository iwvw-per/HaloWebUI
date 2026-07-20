package server

import (
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"strings"
)

const imageConfigKey = "images/config"

func defaultImageConfig() map[string]any {
	return map[string]any{
		"enabled":            false,
		"engine":             "openai",
		"prompt_generation":  false,
		"shared_key_enabled": false,
		"image_model_filter": "",
		"openai": map[string]any{
			"OPENAI_API_BASE_URL":   "https://api.openai.com/v1",
			"OPENAI_API_FORCE_MODE": false,
			"OPENAI_API_KEY":        "",
		},
		"gemini": map[string]any{"GEMINI_API_BASE_URL": "", "GEMINI_API_FORCE_MODE": false, "GEMINI_API_KEY": ""},
		"grok":   map[string]any{"GROK_API_BASE_URL": "", "GROK_API_KEY": ""},
	}
}

func (a *App) loadImageConfig(r *http.Request) (map[string]any, error) {
	return a.loadGlobalJSON(r, imageConfigKey, defaultImageConfig())
}

func (a *App) handleImageConfig(w http.ResponseWriter, r *http.Request) {
	if ok, _ := a.requireAdmin(w, r); !ok {
		return
	}
	config, err := a.loadImageConfig(r)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load image config")
		return
	}
	if r.Method == http.MethodPost {
		var patch map[string]any
		if !decodeJSON(w, r, &patch) {
			return
		}
		mergeJSONMap(config, patch)
		if err := a.saveGlobalJSON(r, imageConfigKey, config); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to save image config")
			return
		}
	}
	writeJSON(w, http.StatusOK, config)
}

func (a *App) handleImageUsageConfig(w http.ResponseWriter, r *http.Request) {
	user, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	config, err := a.loadImageConfig(r)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load image config")
		return
	}
	_ = user
	engine, _ := config["engine"].(string)
	writeJSON(w, http.StatusOK, map[string]any{
		"enabled": config["enabled"], "engine": engine,
		"defaults":     map[string]any{"model": "", "size": "auto", "aspect_ratio": "", "resolution": "", "steps": 0},
		"shared_key":   map[string]any{"enabled": config["shared_key_enabled"], "available": imageProviderKey(config, engine) != ""},
		"personal_key": map[string]any{"supported": engine == "openai" || engine == "gemini" || engine == "grok", "provider": engine},
	})
}

func (a *App) handleImageModelConfig(w http.ResponseWriter, r *http.Request) {
	if ok, _ := a.requireAdmin(w, r); !ok {
		return
	}
	config, err := a.loadImageConfig(r)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load image config")
		return
	}
	if r.Method == http.MethodPost {
		var patch struct {
			Filter *string `json:"IMAGE_MODEL_FILTER_REGEX"`
		}
		if !decodeJSON(w, r, &patch) {
			return
		}
		if patch.Filter != nil {
			if *patch.Filter != "" {
				if _, err := regexp.Compile(*patch.Filter); err != nil {
					writeError(w, http.StatusBadRequest, "invalid image model filter regex")
					return
				}
			}
			config["image_model_filter"] = *patch.Filter
			if err := a.saveGlobalJSON(r, imageConfigKey, config); err != nil {
				writeError(w, http.StatusInternalServerError, "failed to save image model config")
				return
			}
		}
	}
	filter, _ := config["image_model_filter"].(string)
	writeJSON(w, http.StatusOK, map[string]any{"IMAGE_MODEL_FILTER_REGEX": filter})
}

func imageProviderConfig(config map[string]any, provider string) map[string]any {
	value, _ := config[provider].(map[string]any)
	return value
}

func imageProviderURL(config map[string]any, provider string) string {
	value, _ := imageProviderConfig(config, provider)[strings.ToUpper(provider)+"_API_BASE_URL"].(string)
	if provider == "openai" {
		value, _ = imageProviderConfig(config, provider)["OPENAI_API_BASE_URL"].(string)
	}
	return strings.TrimRight(strings.TrimSpace(value), "/")
}

func imageProviderKey(config map[string]any, provider string) string {
	value, _ := imageProviderConfig(config, provider)[strings.ToUpper(provider)+"_API_KEY"].(string)
	if provider == "openai" {
		value, _ = imageProviderConfig(config, provider)["OPENAI_API_KEY"].(string)
	}
	return strings.TrimSpace(value)
}

func (a *App) handleImageModelList(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.requireUser(w, r); !ok {
		return
	}
	config, err := a.loadImageConfig(r)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load image config")
		return
	}
	if enabled, _ := config["enabled"].(bool); !enabled {
		writeJSON(w, http.StatusOK, []any{})
		return
	}
	provider := "openai"
	if value := r.URL.Query().Get("provider"); value == "gemini" || value == "grok" {
		provider = value
	}
	baseURL, key := imageProviderURL(config, provider), imageProviderKey(config, provider)
	if baseURL == "" || key == "" {
		writeJSON(w, http.StatusOK, []any{})
		return
	}
	target, err := providerTarget(baseURL, "/models")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	response, err := providerRequest(r.Context(), provider, http.MethodGet, target, key, nil)
	if err != nil {
		writeError(w, http.StatusBadGateway, "image model discovery failed")
		return
	}
	defer response.Body.Close()
	if response.StatusCode >= 400 {
		writeError(w, http.StatusBadGateway, "image model discovery failed")
		return
	}
	body, _ := io.ReadAll(io.LimitReader(response.Body, 2*1024*1024))
	var raw map[string]any
	_ = json.Unmarshal(body, &raw)
	models := make([]map[string]any, 0)
	data, _ := raw["data"].([]any)
	for _, candidate := range data {
		model, _ := candidate.(map[string]any)
		id, _ := model["id"].(string)
		if id == "" {
			continue
		}
		models = append(models, map[string]any{"id": id, "name": id, "provider": provider, "generation_mode": "generations", "source": "settings"})
	}
	filter, _ := config["image_model_filter"].(string)
	if filter != "" {
		pattern, patternErr := regexp.Compile(filter)
		if patternErr == nil {
			filtered := models[:0]
			for _, model := range models {
				if pattern.MatchString(model["id"].(string)) {
					filtered = append(filtered, model)
				}
			}
			models = filtered
		}
	}
	search := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("search")))
	if search != "" {
		filtered := models[:0]
		for _, model := range models {
			if strings.Contains(strings.ToLower(model["id"].(string)), search) {
				filtered = append(filtered, model)
			}
		}
		models = filtered
	}
	writeJSON(w, http.StatusOK, models)
}

func (a *App) handleImageURLVerify(w http.ResponseWriter, r *http.Request) {
	if ok, _ := a.requireAdmin(w, r); !ok {
		return
	}
	config, err := a.loadImageConfig(r)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load image config")
		return
	}
	provider := "openai"
	if engine, _ := config["engine"].(string); engine == "gemini" || engine == "grok" {
		provider = engine
	}
	baseURL, key := imageProviderURL(config, provider), imageProviderKey(config, provider)
	if baseURL == "" || key == "" {
		writeJSON(w, http.StatusOK, true)
		return
	}
	target, err := providerTarget(baseURL, "/models")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid image provider URL")
		return
	}
	response, err := providerRequest(r.Context(), provider, http.MethodGet, target, key, nil)
	if err != nil {
		writeError(w, http.StatusBadGateway, "image provider is unreachable")
		return
	}
	defer response.Body.Close()
	if response.StatusCode >= 400 {
		writeError(w, http.StatusBadGateway, "image provider is unreachable")
		return
	}
	writeJSON(w, http.StatusOK, true)
}

func (a *App) handleImageGeneration(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.requireUser(w, r); !ok {
		return
	}
	config, err := a.loadImageConfig(r)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load image config")
		return
	}
	if enabled, _ := config["enabled"].(bool); !enabled {
		writeError(w, http.StatusForbidden, "image generation is disabled")
		return
	}
	provider := "openai"
	engine, _ := config["engine"].(string)
	if engine == "gemini" || engine == "grok" {
		provider = engine
	}
	baseURL, key := imageProviderURL(config, provider), imageProviderKey(config, provider)
	if baseURL == "" || key == "" {
		writeError(w, http.StatusServiceUnavailable, "image provider is not configured")
		return
	}
	var form struct {
		Prompt      string `json:"prompt"`
		Model       string `json:"model"`
		Size        string `json:"size"`
		ImageSize   string `json:"image_size"`
		N           int    `json:"n"`
		ResponseFmt string `json:"response_format"`
	}
	if !decodeJSON(w, r, &form) {
		return
	}
	form.Prompt = strings.TrimSpace(form.Prompt)
	if form.Prompt == "" || len(form.Prompt) > 10000 {
		writeError(w, http.StatusBadRequest, "prompt is required and must be at most 10000 bytes")
		return
	}
	if form.N < 1 {
		form.N = 1
	}
	if form.N > 4 {
		form.N = 4
	}
	if form.Model == "" {
		form.Model = "gpt-image-1"
	}
	if form.Size == "" && regexp.MustCompile(`^\d+x\d+$`).MatchString(form.ImageSize) {
		form.Size = form.ImageSize
	}
	payload := map[string]any{"prompt": form.Prompt, "model": form.Model, "n": form.N}
	if form.Size != "" {
		payload["size"] = form.Size
	}
	if form.ResponseFmt != "" {
		payload["response_format"] = form.ResponseFmt
	}
	target, err := providerTarget(baseURL, "/images/generations")
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	response, err := postNativeProvider(r.Context(), provider, target, key, payload)
	if err != nil {
		writeError(w, http.StatusBadGateway, "image generation request failed")
		return
	}
	defer response.Body.Close()
	if response.StatusCode >= 400 {
		writeError(w, response.StatusCode, "image provider returned an error")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = io.Copy(w, io.LimitReader(response.Body, 32*1024*1024))
}

func mergeJSONMap(target, patch map[string]any) {
	for key, value := range patch {
		if targetMap, ok := target[key].(map[string]any); ok {
			if patchMap, ok := value.(map[string]any); ok {
				mergeJSONMap(targetMap, patchMap)
				continue
			}
		}
		target[key] = value
	}
}
