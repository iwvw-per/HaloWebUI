package server

import (
	"bytes"
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

func configDefaults(name string) map[string]any {
	switch name {
	case "direct_connections":
		return map[string]any{"ENABLE_DIRECT_CONNECTIONS": false}
	case "connections":
		return map[string]any{"ENABLE_DIRECT_CONNECTIONS": false, "ENABLE_BASE_MODELS_CACHE": true}
	case "tool_servers":
		return map[string]any{"TOOL_SERVER_CONNECTIONS": []any{}}
	case "mcp_servers":
		return map[string]any{"MCP_SERVER_CONNECTIONS": []any{}, "MCP_RUNTIME_CAPABILITIES": map[string]any{"http": true, "stdio": false}, "MCP_RUNTIME_PROFILE": "go-slim"}
	case "native_tools":
		return map[string]any{
			"TOOL_CALLING_MODE": "default", "ENABLE_INTERLEAVED_THINKING": false, "MAX_TOOL_CALL_ROUNDS": 5,
			"ENABLE_WEB_SEARCH_TOOL": false, "ENABLE_URL_FETCH": true, "ENABLE_URL_FETCH_RENDERED": false,
			"ENABLE_LIST_KNOWLEDGE_BASES": true, "ENABLE_SEARCH_KNOWLEDGE_BASES": true,
			"ENABLE_QUERY_KNOWLEDGE_FILES": true, "ENABLE_VIEW_KNOWLEDGE_FILE": true,
			"ENABLE_IMAGE_GENERATION_TOOL": false, "ENABLE_IMAGE_EDIT": false, "ENABLE_MEMORY_TOOLS": true,
			"ENABLE_NOTES": true, "ENABLE_CHAT_HISTORY_TOOLS": true, "ENABLE_TIME_TOOLS": true,
			"ENABLE_CHANNEL_TOOLS": true, "ENABLE_TERMINAL_TOOL": false,
		}
	case "code_execution":
		return map[string]any{
			"ENABLE_CODE_EXECUTION": false, "CODE_EXECUTION_ENGINE": "jupyter", "CODE_EXECUTION_JUPYTER_URL": nil,
			"CODE_EXECUTION_JUPYTER_AUTH": nil, "CODE_EXECUTION_JUPYTER_AUTH_TOKEN": nil, "CODE_EXECUTION_JUPYTER_AUTH_PASSWORD": nil, "CODE_EXECUTION_JUPYTER_TIMEOUT": 60,
			"ENABLE_CODE_INTERPRETER": false, "CODE_INTERPRETER_ENGINE": "jupyter", "CODE_INTERPRETER_JUPYTER_URL": nil,
			"CODE_INTERPRETER_JUPYTER_AUTH": nil, "CODE_INTERPRETER_JUPYTER_AUTH_TOKEN": nil, "CODE_INTERPRETER_JUPYTER_PASSWORD": nil, "CODE_INTERPRETER_JUPYTER_TIMEOUT": 60,
		}
	case "models":
		return map[string]any{"DEFAULT_MODELS": "", "MODEL_ORDER_LIST": []any{}}
	case "banners":
		return map[string]any{"banners": []any{}}
	case "suggestions":
		return map[string]any{"suggestions": []any{}}
	default:
		return map[string]any{}
	}
}

func (a *App) configResource(r *http.Request, userID, name string) (map[string]any, error) {
	key := userID + ":configs/" + name
	resource, err := a.store.ResourceByKey(r.Context(), "config", key)
	if errors.Is(err, store.ErrResourceNotFound) {
		return configDefaults(name), nil
	}
	if err != nil {
		return nil, err
	}
	var value map[string]any
	if json.Unmarshal(resource.Body, &value) != nil {
		return nil, errors.New("invalid stored config")
	}
	defaults := configDefaults(name)
	mergeJSONMap(defaults, value)
	return defaults, nil
}

func (a *App) saveConfigResource(r *http.Request, userID, name string, value map[string]any) error {
	key := userID + ":configs/" + name
	resource, err := a.store.ResourceByKey(r.Context(), "config", key)
	if errors.Is(err, store.ErrResourceNotFound) {
		resource = store.Resource{Kind: "config", ID: auth.RandomIDForInternalUse(), UserID: userID, Key: key, Active: true}
	} else if err != nil {
		return err
	}
	resource.Body, _ = json.Marshal(value)
	_, err = a.store.PutResource(r.Context(), resource)
	return err
}

func (a *App) handleNamedConfig(name string, adminOnly bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := a.requireUser(w, r)
		if !ok {
			return
		}
		owner := user.ID
		if adminOnly {
			if user.Role != "admin" {
				writeError(w, http.StatusForbidden, "Access prohibited")
				return
			}
			owner = "system"
		}
		value, err := a.configResource(r, owner, name)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to load config")
			return
		}
		if r.Method == http.MethodPost {
			var patch map[string]any
			if !decodeJSON(w, r, &patch) {
				return
			}
			mergeJSONMap(value, patch)
			if name == "native_tools" {
				mode, _ := value["TOOL_CALLING_MODE"].(string)
				if mode != "default" && mode != "native" && mode != "off" {
					writeError(w, http.StatusBadRequest, "Invalid TOOL_CALLING_MODE. Must be 'default', 'native', or 'off'.")
					return
				}
			}
			if err := a.saveConfigResource(r, owner, name, value); err != nil {
				writeError(w, http.StatusInternalServerError, "failed to save config")
				return
			}
		}
		if name == "banners" || name == "suggestions" {
			writeJSON(w, http.StatusOK, value[name])
			return
		}
		writeJSON(w, http.StatusOK, value)
	}
}

func (a *App) handleConfigExport(w http.ResponseWriter, r *http.Request) {
	if ok, _ := a.requireAdmin(w, r); !ok {
		return
	}
	result := map[string]any{}
	for _, name := range []string{"connections", "code_execution", "models", "banners", "suggestions"} {
		value, err := a.configResource(r, "system", name)
		if err == nil {
			result[name] = value
		}
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *App) handleConfigImport(w http.ResponseWriter, r *http.Request) {
	if ok, _ := a.requireAdmin(w, r); !ok {
		return
	}
	var form struct {
		Config map[string]any `json:"config"`
		Mode   string         `json:"mode"`
	}
	if !decodeJSON(w, r, &form) {
		return
	}
	for _, name := range []string{"connections", "code_execution", "models", "banners", "suggestions"} {
		incoming, _ := form.Config[name].(map[string]any)
		if incoming == nil {
			continue
		}
		value := configDefaults(name)
		if form.Mode != "replace" {
			value, _ = a.configResource(r, "system", name)
		}
		mergeJSONMap(value, incoming)
		if err := a.saveConfigResource(r, "system", name, value); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to import config")
			return
		}
	}
	a.handleConfigExport(w, r)
}

func (a *App) handleToolServerVerify(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.requireUser(w, r); !ok {
		return
	}
	var form struct {
		URL      string `json:"url"`
		Path     string `json:"path"`
		AuthType string `json:"auth_type"`
		Key      string `json:"key"`
	}
	if !decodeJSON(w, r, &form) {
		return
	}
	if form.Path == "" {
		form.Path = "openapi.json"
	}
	base, err := url.Parse(strings.TrimRight(form.URL, "/") + "/" + strings.TrimLeft(form.Path, "/"))
	if err != nil || base.Host == "" || blockedRemoteHost(base.Hostname()) {
		writeError(w, http.StatusBadRequest, "unsupported or unsafe tool server URL")
		return
	}
	request, _ := http.NewRequestWithContext(r.Context(), http.MethodGet, base.String(), nil)
	if strings.ToLower(form.AuthType) == "bearer" && form.Key != "" {
		request.Header.Set("Authorization", "Bearer "+form.Key)
	}
	response, err := (&http.Client{Timeout: 15 * time.Second}).Do(request)
	if err != nil {
		writeError(w, http.StatusBadGateway, "failed to connect to tool server")
		return
	}
	defer response.Body.Close()
	if response.StatusCode >= 400 {
		writeError(w, http.StatusBadGateway, "tool server returned an error")
		return
	}
	body, _ := io.ReadAll(io.LimitReader(response.Body, 4*1024*1024))
	var document any
	if json.Unmarshal(body, &document) != nil {
		writeError(w, http.StatusBadGateway, "tool server returned invalid OpenAPI JSON")
		return
	}
	writeJSON(w, http.StatusOK, document)
}

func (a *App) handleMCPServerVerify(w http.ResponseWriter, r *http.Request) {
	user, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	var form map[string]any
	if !decodeJSON(w, r, &form) {
		return
	}
	transport, _ := form["transport_type"].(string)
	if transport == "stdio" {
		writeError(w, http.StatusBadRequest, "stdio MCP is unavailable in the Go slim profile; use an HTTP MCP server")
		return
	}
	target, _ := form["url"].(string)
	parsed, err := url.Parse(strings.TrimSpace(target))
	if err != nil || parsed.Host == "" || blockedRemoteHost(parsed.Hostname()) {
		writeError(w, http.StatusBadRequest, "unsupported or unsafe MCP URL")
		return
	}
	payload := map[string]any{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": map[string]any{"protocolVersion": "2025-03-26", "capabilities": map[string]any{}, "clientInfo": map[string]any{"name": "HaloWebUI Go", "version": a.config.Version}}}
	body, _ := json.Marshal(payload)
	request, _ := http.NewRequestWithContext(r.Context(), http.MethodPost, parsed.String(), bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json, text/event-stream")
	if authType, _ := form["auth_type"].(string); authType == "bearer" {
		if key, _ := form["key"].(string); key != "" {
			request.Header.Set("Authorization", "Bearer "+key)
		}
	}
	response, err := (&http.Client{Timeout: 15 * time.Second}).Do(request)
	if err != nil {
		writeError(w, http.StatusBadGateway, "failed to connect to MCP server")
		return
	}
	defer response.Body.Close()
	if response.StatusCode >= 400 {
		writeError(w, http.StatusBadGateway, "MCP server returned an error")
		return
	}
	var result map[string]any
	if json.NewDecoder(io.LimitReader(response.Body, 2*1024*1024)).Decode(&result) != nil {
		writeError(w, http.StatusBadGateway, "MCP server returned an unsupported response")
		return
	}
	serverInfo := map[string]any{}
	if value, _ := result["result"].(map[string]any); value != nil {
		serverInfo, _ = value["serverInfo"].(map[string]any)
	}
	writeJSON(w, http.StatusOK, map[string]any{"server_info": serverInfo, "tool_count": 0, "verified_at": time.Now().UTC().Format(time.RFC3339), "tools": []any{}, "verified_by": user.ID})
}

func (a *App) handleConfigShare(kind string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ok, userID := a.requireAdmin(w, r)
		if !ok {
			return
		}
		index, err := strconv.Atoi(r.PathValue("index"))
		if err != nil || index < 0 {
			writeError(w, http.StatusBadRequest, "invalid connection index")
			return
		}
		name, field := "tool_servers", "TOOL_SERVER_CONNECTIONS"
		if kind == "mcp" {
			name, field = "mcp_servers", "MCP_SERVER_CONNECTIONS"
		}
		config, err := a.configResource(r, userID, name)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to load connections")
			return
		}
		connections, _ := config[field].([]any)
		if index >= len(connections) {
			writeError(w, http.StatusNotFound, "Connection not found")
			return
		}
		key := userID + ":" + kind + ":" + strconv.Itoa(index)
		resource, resourceErr := a.store.ResourceByKey(r.Context(), "shared_tool_server", key)
		if errors.Is(resourceErr, store.ErrResourceNotFound) {
			resource = store.Resource{Kind: "shared_tool_server", ID: auth.RandomIDForInternalUse(), UserID: userID, Key: key, Active: true}
		}
		var access any
		if r.Method == http.MethodPost {
			var form map[string]any
			if !decodeJSON(w, r, &form) {
				return
			}
			access = form["access_control"]
			resource.Active = true
		} else {
			resource.Active = false
			var current map[string]any
			_ = json.Unmarshal(resource.Body, &current)
			access = current["access_control"]
		}
		resource.Body, _ = json.Marshal(map[string]any{"kind": kind, "connection": connections[index], "access_control": access, "enabled": resource.Active})
		updated, err := a.store.PutResource(r.Context(), resource)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to update shared connection")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"id": updated.ID, "enabled": updated.Active, "access_control": access})
	}
}
