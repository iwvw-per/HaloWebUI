package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/iwvw-per/HaloWebUI/backend/internal/auth"
	"github.com/iwvw-per/HaloWebUI/backend/internal/store"
)

const rootSettingKind = "root_setting"

func (a *App) handleRootChangelog(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	// Keep the release endpoint self-contained so the final distroless image does
	// not need to ship repository documentation just to render the changelog.
	writeJSON(w, http.StatusOK, map[string]any{
		"0.0.1": map[string]any{
			"date": "2026-03-22",
			"Highlights": map[string]any{
				"go-backend": map[string]string{
					"title":   "Go 后端与轻量部署",
					"content": "核心控制面已迁移到 Go，默认运行不加载 Python 或本地模型运行时。",
				},
			},
		},
	})
}

func (a *App) handleRootVersionUpdates(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.requireUser(w, r); !ok {
		return
	}
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"current": a.config.Version, "latest": a.config.Version})
}

func (a *App) rootSetting(r *http.Request, key string) (store.Resource, map[string]any, error) {
	resource, err := a.store.ResourceByKey(r.Context(), rootSettingKind, key)
	if errors.Is(err, store.ErrResourceNotFound) {
		return store.Resource{}, map[string]any{}, nil
	}
	if err != nil {
		return store.Resource{}, nil, err
	}
	var value map[string]any
	if json.Unmarshal(resource.Body, &value) != nil || value == nil {
		value = map[string]any{}
	}
	return resource, value, nil
}

func (a *App) saveRootSetting(r *http.Request, key string, resource store.Resource, value map[string]any) error {
	if resource.ID == "" {
		resource = store.Resource{Kind: rootSettingKind, ID: auth.RandomIDForInternalUse(), UserID: "system", Key: key, Active: true}
	}
	resource.Kind = rootSettingKind
	resource.Key = key
	resource.UserID = "system"
	resource.Body, _ = json.Marshal(value)
	_, err := a.store.PutResource(r.Context(), resource)
	return err
}

func (a *App) handleRootWebhook(w http.ResponseWriter, r *http.Request) {
	if ok, _ := a.requireAdmin(w, r); !ok {
		return
	}
	resource, value, err := a.rootSetting(r, "webhook")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load webhook")
		return
	}
	if r.Method == http.MethodPost {
		var patch struct {
			URL string `json:"url"`
		}
		if !decodeJSON(w, r, &patch) {
			return
		}
		value["url"] = strings.TrimSpace(patch.URL)
		if err := a.saveRootSetting(r, "webhook", resource, value); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to save webhook")
			return
		}
	} else if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"url": stringField(value, "url")})
}

func (a *App) handleRootModelFilter(w http.ResponseWriter, r *http.Request) {
	if ok, _ := a.requireAdmin(w, r); !ok {
		return
	}
	resource, value, err := a.rootSetting(r, "model_filter")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load model filter")
		return
	}
	if r.Method == http.MethodPost {
		var patch struct {
			Enabled bool     `json:"enabled"`
			Models  []string `json:"models"`
		}
		if !decodeJSON(w, r, &patch) {
			return
		}
		value["enabled"], value["models"] = patch.Enabled, patch.Models
		if err := a.saveRootSetting(r, "model_filter", resource, value); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to save model filter")
			return
		}
	} else if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if _, ok := value["enabled"].(bool); !ok {
		value["enabled"] = false
	}
	if _, ok := value["models"].([]any); !ok {
		if _, stringsOK := value["models"].([]string); !stringsOK {
			value["models"] = []any{}
		}
	}
	writeJSON(w, http.StatusOK, value)
}

func (a *App) handleRootModelConfig(w http.ResponseWriter, r *http.Request) {
	if ok, _ := a.requireAdmin(w, r); !ok {
		return
	}
	resource, value, err := a.rootSetting(r, "models")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load global model config")
		return
	}
	if r.Method == http.MethodPost {
		var patch struct {
			Models []any `json:"models"`
		}
		if !decodeJSON(w, r, &patch) {
			return
		}
		value["models"] = patch.Models
		if err := a.saveRootSetting(r, "models", resource, value); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to save global model config")
			return
		}
	} else if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if value["models"] == nil {
		value["models"] = []any{}
	}
	writeJSON(w, http.StatusOK, value)
}

func (a *App) handleRootCommunitySharing(w http.ResponseWriter, r *http.Request) {
	if ok, _ := a.requireAdmin(w, r); !ok {
		return
	}
	resource, value, err := a.rootSetting(r, "community_sharing")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load community sharing")
		return
	}
	enabled, _ := value["enabled"].(bool)
	if r.URL.Path == "/api/community_sharing/toggle" {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		enabled = !enabled
		value["enabled"] = enabled
		if err := a.saveRootSetting(r, "community_sharing", resource, value); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to save community sharing")
			return
		}
	} else if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"enabled": enabled})
}
