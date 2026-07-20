package server

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/iwvw-per/HaloWebUI/backend/internal/auth"
	"github.com/iwvw-per/HaloWebUI/backend/internal/store"
)

const (
	haloClawConfigKind       = "global_setting"
	haloClawConfigKey        = "haloclaw/config"
	haloClawGatewayKind      = "haloclaw_gateway"
	haloClawExternalUserKind = "haloclaw_external_user"
	haloClawMessageLogKind   = "haloclaw_message_log"
	externalConfigKey        = "external_api/config"
)

func (a *App) handleAnalytics(w http.ResponseWriter, r *http.Request) {
	if ok, _ := a.requireAdmin(w, r); !ok {
		return
	}
	a.handleAnalyticsData(w, r)
}

func (a *App) handleHaloClaw(w http.ResponseWriter, r *http.Request) {
	parts := splitPath(strings.TrimPrefix(r.URL.Path, "/api/v1/haloclaw/"))
	if len(parts) > 0 && parts[0] == "webhook" {
		a.handleHaloClawWebhook(w, r, parts[1:])
		return
	}
	if ok, _ := a.requireAdmin(w, r); !ok {
		return
	}
	if len(parts) == 0 {
		writeError(w, http.StatusNotFound, "Not found")
		return
	}
	switch parts[0] {
	case "config":
		a.handleOptionalConfig(w, r, haloClawConfigKey, map[string]any{
			"enabled": false, "default_model": "", "max_history": 20, "rate_limit": 60,
		})
	case "gateways":
		a.handleHaloClawGateways(w, r, parts[1:])
	case "users":
		a.handleHaloClawUser(w, r, parts[1:])
	default:
		writeError(w, http.StatusNotFound, "Not found")
	}
}

func (a *App) handleHaloClawWebhook(w http.ResponseWriter, r *http.Request, parts []string) {
	if len(parts) != 2 {
		writeError(w, http.StatusNotFound, "Gateway not found")
		return
	}
	platform, gatewayID := parts[0], parts[1]
	gateway, err := a.store.ResourceByID(r.Context(), haloClawGatewayKind, gatewayID)
	if err != nil {
		writeError(w, http.StatusNotFound, "Gateway not found")
		return
	}
	var body map[string]any
	if json.Unmarshal(gateway.Body, &body) != nil || stringField(body, "platform") != platform {
		writeError(w, http.StatusNotFound, "Gateway not found")
		return
	}
	config, _ := body["config"].(map[string]any)
	switch platform {
	case "feishu":
		a.handleFeishuWebhook(w, r, config)
	case "wechat_work":
		a.handleWeChatWorkWebhook(w, r, config)
	default:
		writeError(w, http.StatusNotFound, "Gateway not found")
	}
}

func (a *App) handleFeishuWebhook(w http.ResponseWriter, r *http.Request, config map[string]any) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var event map[string]any
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		writeError(w, http.StatusBadRequest, "invalid webhook body")
		return
	}
	if stringField(event, "type") == "url_verification" {
		if expected := stringField(config, "verification_token"); expected != "" && stringField(event, "token") != expected {
			writeError(w, http.StatusForbidden, "Invalid token")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"challenge": stringField(event, "challenge")})
		return
	}
	if header, ok := event["header"].(map[string]any); ok {
		if expected := stringField(config, "verification_token"); expected != "" && stringField(header, "token") != expected {
			writeJSON(w, http.StatusOK, map[string]any{})
			return
		}
	}
	// Event delivery is acknowledged immediately. Remote messaging workers can
	// consume the persisted gateway contract without blocking the control plane.
	writeJSON(w, http.StatusOK, map[string]any{})
}

func (a *App) handleWeChatWorkWebhook(w http.ResponseWriter, r *http.Request, config map[string]any) {
	token := stringField(config, "token")
	aesKey := stringField(config, "aes_key")
	timestamp := r.URL.Query().Get("timestamp")
	nonce := r.URL.Query().Get("nonce")
	signature := r.URL.Query().Get("msg_signature")
	if token == "" || aesKey == "" {
		writeError(w, http.StatusInternalServerError, "Gateway missing token or aes_key")
		return
	}
	if r.Method == http.MethodGet {
		echo := r.URL.Query().Get("echostr")
		if !validWeChatSignature(token, timestamp, nonce, echo, signature) {
			writeError(w, http.StatusForbidden, "Invalid signature")
			return
		}
		decrypted, err := decryptWeChatWork(echo, aesKey)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Decrypt failed")
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, decrypted)
		return
	}
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var envelope struct {
		Encrypt string `xml:"Encrypt"`
	}
	if err := xml.NewDecoder(r.Body).Decode(&envelope); err != nil || envelope.Encrypt == "" {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = io.WriteString(w, "success")
		return
	}
	if validWeChatSignature(token, timestamp, nonce, envelope.Encrypt, signature) {
		_, _ = decryptWeChatWork(envelope.Encrypt, aesKey)
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = io.WriteString(w, "success")
}

func validWeChatSignature(token, timestamp, nonce, encrypted, signature string) bool {
	values := []string{token, timestamp, nonce, encrypted}
	sort.Strings(values)
	digest := sha1.Sum([]byte(strings.Join(values, "")))
	return strings.EqualFold(hex.EncodeToString(digest[:]), signature)
}

func decryptWeChatWork(value, encodingAESKey string) (string, error) {
	key, err := base64.StdEncoding.DecodeString(strings.TrimRight(encodingAESKey, "=") + "=")
	if err != nil || len(key) != 32 {
		return "", errors.New("invalid EncodingAESKey")
	}
	ciphertext, err := base64.StdEncoding.DecodeString(value)
	if err != nil || len(ciphertext) == 0 || len(ciphertext)%aes.BlockSize != 0 {
		return "", errors.New("invalid encrypted payload")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	plain := make([]byte, len(ciphertext))
	cipher.NewCBCDecrypter(block, key[:aes.BlockSize]).CryptBlocks(plain, ciphertext)
	padding := int(plain[len(plain)-1])
	if padding < 1 || padding > 32 || padding > len(plain) {
		return "", errors.New("invalid encrypted padding")
	}
	for _, value := range plain[len(plain)-padding:] {
		if int(value) != padding {
			return "", errors.New("invalid encrypted padding")
		}
	}
	plain = plain[:len(plain)-padding]
	if len(plain) < 20 {
		return "", errors.New("invalid encrypted payload")
	}
	length := int(binary.BigEndian.Uint32(plain[16:20]))
	if length < 0 || 20+length > len(plain) {
		return "", errors.New("invalid encrypted payload")
	}
	return string(plain[20 : 20+length]), nil
}

func (a *App) handleHaloClawGateways(w http.ResponseWriter, r *http.Request, rest []string) {
	const kind = haloClawGatewayKind
	if len(rest) == 0 {
		if r.Method == http.MethodGet {
			a.writeOptionalResourceList(w, r, kind)
			return
		}
		if r.Method == http.MethodPost {
			a.writeOptionalResourceCreate(w, r, kind, false)
			return
		}
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	id := rest[0]
	if len(rest) > 1 && (rest[1] == "users" || rest[1] == "logs") {
		gatewayID := rest[0]
		if rest[1] == "users" && len(rest) == 2 {
			a.writeHaloClawUsers(w, r, gatewayID)
			return
		}
		if rest[1] == "logs" && len(rest) == 2 {
			a.writeHaloClawLogs(w, r, gatewayID, "")
			return
		}
		if rest[1] == "users" && len(rest) == 4 && rest[3] == "logs" {
			a.writeHaloClawLogs(w, r, gatewayID, rest[2])
			return
		}
		writeError(w, http.StatusNotFound, "Not found")
		return
	}
	resource, err := a.store.ResourceByID(r.Context(), kind, id)
	if errors.Is(err, store.ErrResourceNotFound) {
		if r.Method == http.MethodPost {
			a.writeOptionalResourceCreate(w, r, kind, false)
			return
		}
		writeError(w, http.StatusNotFound, "resource not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load gateway")
		return
	}
	if len(rest) > 1 && rest[1] == "toggle" {
		var patch map[string]any
		if !decodeJSON(w, r, &patch) {
			return
		}
		if enabled, ok := patch["enabled"].(bool); ok {
			var body map[string]any
			_ = json.Unmarshal(resource.Body, &body)
			body["enabled"] = enabled
			resource.Body, _ = json.Marshal(body)
			resource, err = a.store.PutResource(r.Context(), resource)
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to toggle gateway")
			return
		}
		writeRawJSON(w, http.StatusOK, resourceResponse(resource))
		return
	}
	if r.Method == http.MethodGet {
		writeRawJSON(w, http.StatusOK, resourceResponse(resource))
		return
	}
	if r.Method == http.MethodDelete {
		a.deleteHaloClawChildren(r.Context(), id)
		_ = a.store.DeleteResource(r.Context(), kind, id)
		writeJSON(w, http.StatusOK, true)
		return
	}
	if r.Method == http.MethodPost {
		var patch map[string]any
		if !decodeJSON(w, r, &patch) {
			return
		}
		var body map[string]any
		_ = json.Unmarshal(resource.Body, &body)
		for key, value := range patch {
			body[key] = value
		}
		resource.Body, _ = json.Marshal(body)
		updated, updateErr := a.store.PutResource(r.Context(), resource)
		if updateErr != nil {
			writeError(w, http.StatusInternalServerError, "failed to update gateway")
			return
		}
		writeRawJSON(w, http.StatusOK, resourceResponse(updated))
		return
	}
	writeError(w, http.StatusMethodNotAllowed, "method not allowed")
}

func (a *App) handleHaloClawUser(w http.ResponseWriter, r *http.Request, rest []string) {
	if len(rest) != 2 || r.Method != http.MethodPost {
		writeError(w, http.StatusNotFound, "Not found")
		return
	}
	userID, action := rest[0], rest[1]
	resource, err := a.store.ResourceByID(r.Context(), haloClawExternalUserKind, userID)
	if errors.Is(err, store.ErrResourceNotFound) {
		writeError(w, http.StatusNotFound, "resource not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load external user")
		return
	}
	var patch map[string]any
	if !decodeJSON(w, r, &patch) {
		return
	}
	var body map[string]any
	if json.Unmarshal(resource.Body, &body) != nil {
		body = map[string]any{}
	}
	switch action {
	case "model-override":
		value, exists := patch["model_override"]
		if !exists || value == nil {
			body["model_override"] = nil
		} else if model, ok := value.(string); ok {
			body["model_override"] = strings.TrimSpace(model)
		} else {
			writeError(w, http.StatusBadRequest, "model_override must be a string or null")
			return
		}
	case "block":
		blocked, ok := patch["is_blocked"].(bool)
		if !ok {
			writeError(w, http.StatusBadRequest, "is_blocked must be boolean")
			return
		}
		body["is_blocked"] = blocked
	case "link":
		value, exists := patch["halo_user_id"]
		if !exists || value == nil {
			body["halo_user_id"] = nil
		} else if linked, ok := value.(string); ok {
			body["halo_user_id"] = strings.TrimSpace(linked)
		} else {
			writeError(w, http.StatusBadRequest, "halo_user_id must be a string or null")
			return
		}
	default:
		writeError(w, http.StatusNotFound, "Not found")
		return
	}
	body["updated_at"] = time.Now().UnixNano()
	encoded, _ := json.Marshal(body)
	resource.Body = encoded
	updated, err := a.store.PutResource(r.Context(), resource)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update external user")
		return
	}
	writeRawJSON(w, http.StatusOK, haloClawResourceResponse(updated))
}

func (a *App) writeHaloClawUsers(w http.ResponseWriter, r *http.Request, gatewayID string) {
	resources, err := a.store.ListResources(r.Context(), haloClawExternalUserKind, false)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list external users")
		return
	}
	result := make([]json.RawMessage, 0, len(resources))
	for _, resource := range resources {
		if stringFieldJSON(resource.Body, "gateway_id") == gatewayID {
			result = append(result, haloClawResourceResponse(resource))
		}
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *App) writeHaloClawLogs(w http.ResponseWriter, r *http.Request, gatewayID, externalUserID string) {
	limit := parseLimit(r.URL.Query().Get("limit"), 100, 200)
	resources, err := a.store.ListResources(r.Context(), haloClawMessageLogKind, false)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list message logs")
		return
	}
	result := make([]json.RawMessage, 0, haloClawMin(limit, len(resources)))
	for _, resource := range resources {
		if stringFieldJSON(resource.Body, "gateway_id") != gatewayID {
			continue
		}
		if externalUserID != "" && stringFieldJSON(resource.Body, "external_user_id") != externalUserID {
			continue
		}
		result = append(result, haloClawResourceResponse(resource))
		if len(result) == limit {
			break
		}
	}
	for left, right := 0, len(result)-1; left < right; left, right = left+1, right-1 {
		result[left], result[right] = result[right], result[left]
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *App) deleteHaloClawChildren(ctx context.Context, gatewayID string) {
	for _, kind := range []string{haloClawExternalUserKind, haloClawMessageLogKind} {
		resources, err := a.store.ListResources(ctx, kind, false)
		if err != nil {
			continue
		}
		for _, resource := range resources {
			if stringFieldJSON(resource.Body, "gateway_id") == gatewayID {
				_ = a.store.DeleteResource(ctx, kind, resource.ID)
			}
		}
	}
}

func haloClawResourceResponse(resource store.Resource) json.RawMessage {
	var body map[string]any
	if json.Unmarshal(resource.Body, &body) != nil {
		body = map[string]any{}
	}
	body["id"] = resource.ID
	if _, ok := body["created_at"]; !ok {
		body["created_at"] = resource.CreatedAt * int64(time.Second)
	}
	if _, ok := body["updated_at"]; !ok {
		body["updated_at"] = resource.UpdatedAt * int64(time.Second)
	}
	encoded, _ := json.Marshal(body)
	return encoded
}

func stringFieldJSON(raw json.RawMessage, key string) string {
	var body map[string]any
	if json.Unmarshal(raw, &body) != nil {
		return ""
	}
	value, _ := body[key].(string)
	return value
}

func parseLimit(value string, fallback, maximum int) int {
	limit, err := strconv.Atoi(value)
	if err != nil || limit < 1 {
		return fallback
	}
	if limit > maximum {
		return maximum
	}
	return limit
}

func haloClawMin(left, right int) int {
	if left < right {
		return left
	}
	return right
}

func (a *App) handleExternalAPI(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/v1/external_api/gateway/") {
		a.handleExternalGateway(w, r)
		return
	}
	ok, userID := a.requireAdmin(w, r)
	if !ok {
		return
	}
	parts := splitPath(strings.TrimPrefix(r.URL.Path, "/api/v1/external_api/"))
	if len(parts) == 0 {
		writeError(w, http.StatusNotFound, "Not found")
		return
	}
	switch parts[0] {
	case "config":
		a.handleExternalConfig(w, r)
	case "logs":
		a.writeExternalLogs(w, r, "")
	case "clients":
		a.handleExternalClients(w, r, parts[1:], userID)
	default:
		writeError(w, http.StatusNotFound, "Not found")
	}
}

func (a *App) handleExternalClients(w http.ResponseWriter, r *http.Request, rest []string, ownerID string) {
	const kind = "external_api_client"
	if len(rest) == 0 {
		if r.Method == http.MethodGet {
			a.writeOptionalResourceList(w, r, kind)
			return
		}
		if r.Method == http.MethodPost {
			a.writeOptionalResourceCreate(w, r, kind, true, ownerID)
			return
		}
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if len(rest) > 1 && rest[1] == "logs" {
		a.writeExternalLogs(w, r, rest[0])
		return
	}
	id := rest[0]
	resource, err := a.store.ResourceByID(r.Context(), kind, id)
	if errors.Is(err, store.ErrResourceNotFound) {
		writeError(w, http.StatusNotFound, "resource not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load client")
		return
	}
	if r.Method == http.MethodGet {
		writeRawJSON(w, http.StatusOK, sanitizeExternalClient(resourceResponse(resource)))
		return
	}
	if r.Method == http.MethodDelete {
		_ = a.store.DeleteResource(r.Context(), kind, id)
		writeJSON(w, http.StatusOK, true)
		return
	}
	if r.Method == http.MethodPost {
		var patch map[string]any
		if !decodeJSON(w, r, &patch) {
			return
		}
		var body map[string]any
		_ = json.Unmarshal(resource.Body, &body)
		for key, value := range patch {
			body[key] = value
		}
		resource.Body, _ = json.Marshal(body)
		updated, updateErr := a.store.PutResource(r.Context(), resource)
		if updateErr != nil {
			writeError(w, http.StatusInternalServerError, "failed to update client")
			return
		}
		writeRawJSON(w, http.StatusOK, sanitizeExternalClient(resourceResponse(updated)))
		return
	}
	writeError(w, http.StatusMethodNotAllowed, "method not allowed")
}

func (a *App) handleOptionalConfig(w http.ResponseWriter, r *http.Request, key string, fallback map[string]any) {
	resource, err := a.store.ResourceByKey(r.Context(), haloClawConfigKind, key)
	if err != nil && !errors.Is(err, store.ErrResourceNotFound) {
		writeError(w, http.StatusInternalServerError, "failed to load config")
		return
	}
	if r.Method == http.MethodGet {
		if errors.Is(err, store.ErrResourceNotFound) {
			writeJSON(w, http.StatusOK, fallback)
			return
		}
		writeRawJSON(w, http.StatusOK, resource.Body)
		return
	}
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var body map[string]any
	if !decodeJSON(w, r, &body) {
		return
	}
	if body == nil {
		body = map[string]any{}
	}
	if resource.ID == "" {
		resource = store.Resource{Kind: haloClawConfigKind, ID: auth.RandomIDForInternalUse(), UserID: "system", Key: key, Active: true}
	}
	resource.Body, _ = json.Marshal(body)
	updated, err := a.store.PutResource(r.Context(), resource)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save config")
		return
	}
	writeRawJSON(w, http.StatusOK, updated.Body)
}

func (a *App) writeOptionalResourceList(w http.ResponseWriter, r *http.Request, kind string) {
	resources, err := a.store.ListResources(r.Context(), kind, false)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list resources")
		return
	}
	result := make([]json.RawMessage, 0, len(resources))
	for _, resource := range resources {
		payload := resourceResponse(resource)
		if kind == "external_api_client" {
			payload = sanitizeExternalClient(payload)
		}
		result = append(result, payload)
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *App) writeOptionalResourceCreate(w http.ResponseWriter, r *http.Request, kind string, withClientEnvelope bool, ownerID ...string) {
	var body map[string]any
	if !decodeJSON(w, r, &body) {
		return
	}
	if body == nil {
		body = map[string]any{}
	}
	if kind == haloClawGatewayKind {
		platform := stringField(body, "platform")
		if platform != "telegram" && platform != "wechat_work" && platform != "feishu" {
			writeError(w, http.StatusBadRequest, "Unsupported platform. Must be: telegram, wechat_work, feishu")
			return
		}
		if strings.TrimSpace(stringField(body, "name")) == "" {
			writeError(w, http.StatusBadRequest, "name is required")
			return
		}
	}
	id := stringField(body, "id")
	if id == "" {
		id = auth.RandomIDForInternalUse()
	}
	body["id"], body["user_id"] = id, "system"
	if kind == "external_api_client" {
		if body["owner_user_id"] == nil || stringField(body, "owner_user_id") == "" {
			if len(ownerID) > 0 {
				body["owner_user_id"] = ownerID[0]
			}
		}
		if body["allowed_protocols"] == nil {
			body["allowed_protocols"] = []string{"openai"}
		}
		if body["enabled"] == nil {
			body["enabled"] = true
		}
		if body["allow_tools"] == nil {
			body["allow_tools"] = false
		}
	}
	var rawKey string
	if kind == "external_api_client" {
		rawKey = "hwg-" + auth.RandomIDForInternalUse() + auth.RandomIDForInternalUse()
		digest := sha256.Sum256([]byte(rawKey))
		body["api_key_hash"] = hex.EncodeToString(digest[:])
		body["key_prefix"] = rawKey[:12]
	}
	encoded, _ := json.Marshal(body)
	resource, err := a.store.PutResource(r.Context(), store.Resource{Kind: kind, ID: id, UserID: "system", Key: id, Body: encoded, Active: true})
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to create resource")
		return
	}
	response := resourceResponse(resource)
	if withClientEnvelope {
		response = sanitizeExternalClient(response)
		var client any
		_ = json.Unmarshal(response, &client)
		writeJSON(w, http.StatusOK, map[string]any{"client": client, "api_key": rawKey})
		return
	}
	writeRawJSON(w, http.StatusOK, response)
}
