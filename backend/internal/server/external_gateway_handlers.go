package server

import (
	"bytes"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/iwvw-per/HaloWebUI/backend/internal/auth"
	"github.com/iwvw-per/HaloWebUI/backend/internal/store"
)

type externalGatewayClient struct {
	Resource store.Resource
	Body     map[string]any
	Owner    store.User
}

type gatewayRateWindow struct {
	Started time.Time
	Count   int64
}

func sanitizeExternalClient(raw json.RawMessage) json.RawMessage {
	var body map[string]any
	if json.Unmarshal(raw, &body) != nil {
		return raw
	}
	delete(body, "api_key_hash")
	return mustJSON(body)
}

func (a *App) handleExternalGateway(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/external_api/gateway/")
	protocol := ""
	if strings.HasPrefix(path, "openai/") {
		protocol = "openai"
	}
	if strings.HasPrefix(path, "anthropic/") {
		protocol = "anthropic"
	}
	if protocol == "" {
		writeError(w, http.StatusNotFound, "gateway protocol not found")
		return
	}
	client, err := a.authenticateExternalGateway(r, protocol)
	if err != nil {
		status := http.StatusUnauthorized
		if strings.Contains(err.Error(), "disabled") {
			status = http.StatusNotFound
		}
		if strings.Contains(err.Error(), "not allowed") {
			status = http.StatusForbidden
		}
		writeError(w, status, err.Error())
		return
	}
	if !a.allowExternalGatewayRequest(w, r, client) {
		a.auditExternal(r, client, protocol, path, "", http.StatusTooManyRequests, fmt.Errorf("rate limit exceeded"))
		return
	}
	if protocol == "openai" {
		a.handleOpenAIGateway(w, r, client, path)
		return
	}
	a.handleAnthropicGateway(w, r, client, path)
}

func (a *App) allowExternalGatewayRequest(w http.ResponseWriter, r *http.Request, client externalGatewayClient) bool {
	limit, ok := numberInt64(client.Body["rpm_limit"])
	if !ok || limit <= 0 {
		config := a.loadExternalGatewayConfig(r)
		limit, _ = numberInt64(config["default_rpm_limit"])
	}
	if limit <= 0 {
		return true
	}

	now := time.Now()
	a.gatewayRateMu.Lock()
	window := a.gatewayRates[client.Resource.ID]
	if window.Started.IsZero() || now.Sub(window.Started) >= time.Minute {
		window = gatewayRateWindow{Started: now}
	}
	resetAfter := time.Until(window.Started.Add(time.Minute))
	if resetAfter < time.Second {
		resetAfter = time.Second
	}
	w.Header().Set("X-RateLimit-Limit", strconv.FormatInt(limit, 10))
	w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(window.Started.Add(time.Minute).Unix(), 10))
	if window.Count >= limit {
		a.gatewayRates[client.Resource.ID] = window
		a.gatewayRateMu.Unlock()
		w.Header().Set("X-RateLimit-Remaining", "0")
		w.Header().Set("Retry-After", strconv.FormatInt(int64(resetAfter.Seconds()+0.999), 10))
		writeError(w, http.StatusTooManyRequests, "external client rate limit exceeded")
		return false
	}
	window.Count++
	a.gatewayRates[client.Resource.ID] = window
	a.gatewayRateMu.Unlock()
	w.Header().Set("X-RateLimit-Remaining", strconv.FormatInt(limit-window.Count, 10))
	return true
}

func (a *App) handleExternalConfig(w http.ResponseWriter, r *http.Request) {
	config := a.loadExternalGatewayConfig(r)
	if r.Method == http.MethodPost {
		var patch map[string]any
		if !decodeJSON(w, r, &patch) {
			return
		}
		if value, ok := patch["enabled"].(bool); ok {
			config["enabled"] = value
		}
		protocols, _ := config["protocols"].(map[string]any)
		if protocols == nil {
			protocols = map[string]any{}
		}
		for _, protocol := range []string{"openai", "anthropic"} {
			if value, ok := patch[protocol].(bool); ok {
				protocols[protocol] = value
			}
		}
		config["protocols"] = protocols
		if value, ok := numberInt64(patch["default_rpm_limit"]); ok && value >= 0 {
			config["default_rpm_limit"] = value
		}
		resource, err := a.store.ResourceByKey(r.Context(), "global_setting", externalConfigKey)
		if err != nil {
			resource = store.Resource{Kind: "global_setting", ID: auth.RandomIDForInternalUse(), UserID: "system", Key: externalConfigKey, Active: true}
		}
		resource.Body, _ = json.Marshal(config)
		if _, err := a.store.PutResource(r.Context(), resource); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to save config")
			return
		}
	}
	writeJSON(w, http.StatusOK, config)
}

func (a *App) writeExternalLogs(w http.ResponseWriter, r *http.Request, clientID string) {
	limit := queryInt(r, "limit", 100, 1, 500)
	resources, err := a.store.ListResources(r.Context(), "external_api_log", false)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load audit logs")
		return
	}
	result := make([]json.RawMessage, 0, limit)
	for _, resource := range resources {
		var body map[string]any
		if json.Unmarshal(resource.Body, &body) != nil {
			continue
		}
		if clientID != "" && stringField(body, "client_id") != clientID {
			continue
		}
		result = append(result, resourceResponse(resource))
		if len(result) >= limit {
			break
		}
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *App) loadExternalGatewayConfig(r *http.Request) map[string]any {
	resource, err := a.store.ResourceByKey(r.Context(), "global_setting", externalConfigKey)
	if err != nil {
		return map[string]any{"enabled": false, "protocols": map[string]any{"openai": false, "anthropic": false}, "default_rpm_limit": 60}
	}
	var body map[string]any
	if json.Unmarshal(resource.Body, &body) != nil {
		body = map[string]any{}
	}
	if body["protocols"] == nil {
		body["protocols"] = map[string]any{"openai": false, "anthropic": false}
	}
	return body
}

func (a *App) authenticateExternalGateway(r *http.Request, protocol string) (externalGatewayClient, error) {
	config := a.loadExternalGatewayConfig(r)
	enabled, _ := config["enabled"].(bool)
	if !enabled {
		return externalGatewayClient{}, fmt.Errorf("external client gateway is disabled")
	}
	protocols, _ := config["protocols"].(map[string]any)
	protocolEnabled, _ := protocols[protocol].(bool)
	if !protocolEnabled {
		return externalGatewayClient{}, fmt.Errorf("%s gateway is disabled", protocol)
	}
	raw := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
	if raw == "" {
		return externalGatewayClient{}, fmt.Errorf("missing bearer token")
	}
	if strings.HasPrefix(raw, "sk-") {
		return externalGatewayClient{}, fmt.Errorf("personal API keys are not allowed on the external gateway")
	}
	digest := sha256.Sum256([]byte(raw))
	hexDigest := hex.EncodeToString(digest[:])
	resources, err := a.store.ListResources(r.Context(), "external_api_client", false)
	if err != nil {
		return externalGatewayClient{}, err
	}
	for _, resource := range resources {
		var body map[string]any
		if json.Unmarshal(resource.Body, &body) != nil {
			continue
		}
		if stringField(body, "api_key_hash") == "" || subtle.ConstantTimeCompare([]byte(stringField(body, "api_key_hash")), []byte(hexDigest)) != 1 {
			continue
		}
		enabled, _ := body["enabled"].(bool)
		if !enabled {
			return externalGatewayClient{}, fmt.Errorf("external client is disabled")
		}
		allowed := stringSlice(body["allowed_protocols"])
		found := false
		for _, item := range allowed {
			if strings.EqualFold(item, protocol) {
				found = true
			}
		}
		if !found {
			return externalGatewayClient{}, fmt.Errorf("%s protocol is not allowed for this client", protocol)
		}
		ownerID := stringField(body, "owner_user_id")
		owner, err := a.store.UserByID(r.Context(), ownerID)
		if err != nil {
			return externalGatewayClient{}, fmt.Errorf("gateway owner user not found")
		}
		return externalGatewayClient{Resource: resource, Body: body, Owner: owner}, nil
	}
	return externalGatewayClient{}, fmt.Errorf("invalid external client key")
}

func (a *App) handleOpenAIGateway(w http.ResponseWriter, r *http.Request, client externalGatewayClient, path string) {
	if path == "openai/v1/models" {
		base, key := a.openAIProviderForUser(r, client.Owner)
		payload, err := a.fetchProviderJSON(r, base, key, "/models")
		if err != nil {
			writeError(w, http.StatusBadGateway, "failed to load gateway models")
			return
		}
		var body map[string]any
		_ = json.Unmarshal(payload, &body)
		if data, ok := body["data"].([]any); ok {
			allowed := stringSet(client.Body["allowed_model_ids"])
			filtered := make([]any, 0, len(data))
			for _, item := range data {
				model, _ := item.(map[string]any)
				id, _ := model["id"].(string)
				if allowed[id] {
					filtered = append(filtered, item)
				}
			}
			body["data"] = filtered
		}
		writeJSON(w, http.StatusOK, body)
		return
	}
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if path != "openai/v1/chat/completions" && path != "openai/v1/responses" {
		writeError(w, http.StatusNotFound, "gateway endpoint not found")
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, maxProviderRequestBytes))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid gateway request")
		return
	}
	var payload map[string]any
	if json.Unmarshal(body, &payload) != nil {
		writeError(w, http.StatusBadRequest, "invalid gateway request")
		return
	}
	if path == "openai/v1/responses" {
		payload = responsesToChat(payload)
		body, _ = json.Marshal(payload)
	}
	model, _ := payload["model"].(string)
	if !a.externalModelAllowed(client, model) {
		writeError(w, http.StatusForbidden, "model is not allowed for this external client")
		return
	}
	if tools, ok := payload["tools"].([]any); ok && len(tools) > 0 {
		allow, _ := client.Body["allow_tools"].(bool)
		if !allow {
			writeError(w, http.StatusForbidden, "tool calling is disabled for this external client")
			return
		}
	}
	base, key := a.openAIProviderForUser(r, client.Owner)
	request := r.Clone(r.Context())
	request.Body = io.NopCloser(bytes.NewReader(body))
	request.ContentLength = int64(len(body))
	statusWriter := &externalStatusWriter{ResponseWriter: w}
	a.proxyProvider(statusWriter, request, base, key, "/chat/completions")
	a.auditExternal(r, client, "openai", path, model, statusWriter.status, nil)
}

func responsesToChat(payload map[string]any) map[string]any {
	if _, ok := payload["messages"]; ok {
		return payload
	}
	messages := []any{}
	if input, ok := payload["input"].(string); ok {
		messages = []any{map[string]any{"role": "user", "content": input}}
	} else if input, ok := payload["input"].([]any); ok {
		messages = input
	}
	payload["messages"] = messages
	delete(payload, "input")
	return payload
}
func stringSet(value any) map[string]bool {
	result := map[string]bool{}
	for _, item := range stringSlice(value) {
		result[item] = true
	}
	return result
}
func (a *App) externalModelAllowed(client externalGatewayClient, model string) bool {
	allowed := stringSet(client.Body["allowed_model_ids"])
	return allowed[model]
}

func (a *App) handleAnthropicGateway(w http.ResponseWriter, r *http.Request, client externalGatewayClient, path string) {
	if path == "anthropic/v1/models" {
		base, key, _, found := a.providerConnection(r, client.Owner, "anthropic", -1)
		if !found {
			writeError(w, http.StatusServiceUnavailable, "anthropic provider is not configured")
			return
		}
		payload, err := a.fetchNativeProviderModels(r, "anthropic", base, key)
		if err != nil {
			writeError(w, http.StatusBadGateway, "failed to load gateway models")
			return
		}
		writeJSON(w, http.StatusOK, payload)
		return
	}
	if path != "anthropic/v1/messages" || r.Method != http.MethodPost {
		writeError(w, http.StatusNotFound, "gateway endpoint not found")
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, maxProviderRequestBytes))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid gateway request")
		return
	}
	var payload map[string]any
	if json.Unmarshal(body, &payload) != nil {
		writeError(w, http.StatusBadRequest, "invalid gateway request")
		return
	}
	model, _ := payload["model"].(string)
	if !a.externalModelAllowed(client, model) {
		writeError(w, http.StatusForbidden, "model is not allowed for this external client")
		return
	}
	allow, _ := client.Body["allow_tools"].(bool)
	if _, ok := payload["tools"].([]any); ok && !allow {
		writeError(w, http.StatusForbidden, "tool calling is disabled for this external client")
		return
	}
	base, key, _, found := a.providerConnection(r, client.Owner, "anthropic", -1)
	if !found {
		writeError(w, http.StatusServiceUnavailable, "anthropic provider is not configured")
		return
	}
	target, err := providerTarget(base, "/messages")
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	response, err := providerRequest(r.Context(), "anthropic", http.MethodPost, target, key, bytes.NewReader(body))
	if err != nil {
		writeError(w, http.StatusBadGateway, "anthropic request failed")
		return
	}
	defer response.Body.Close()
	for _, name := range []string{"Content-Type", "Cache-Control"} {
		if value := response.Header.Get(name); value != "" {
			w.Header().Set(name, value)
		}
	}
	w.WriteHeader(response.StatusCode)
	_, _ = io.Copy(w, io.LimitReader(response.Body, maxProviderRequestBytes))
	a.auditExternal(r, client, "anthropic", path, model, response.StatusCode, nil)
}

type externalStatusWriter struct {
	http.ResponseWriter
	status int
}

func (w *externalStatusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}
func (w *externalStatusWriter) Write(body []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.ResponseWriter.Write(body)
}
func (w *externalStatusWriter) Flush() {
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}
func (a *App) auditExternal(r *http.Request, client externalGatewayClient, protocol, endpoint, model string, status int, err error) {
	if status == 0 {
		status = 200
	}
	body, _ := json.Marshal(map[string]any{"client_id": client.Resource.ID, "owner_user_id": client.Owner.ID, "protocol": protocol, "endpoint": endpoint, "model": model, "status_code": status, "error": errorString(err), "created_at": time.Now().UnixMilli()})
	id := "log-" + client.Resource.ID + "-" + time.Now().Format("20060102150405.000000000")
	_, _ = a.store.PutResource(r.Context(), store.Resource{Kind: "external_api_log", ID: id, UserID: "system", Key: id, Body: body, Active: true})
}
func errorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
