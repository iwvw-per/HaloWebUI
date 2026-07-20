package server

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/iwvw-per/HaloWebUI/backend/internal/auth"
	"github.com/iwvw-per/HaloWebUI/backend/internal/store"
)

func channelAccess(user store.User, channel store.Channel, permission string) bool {
	if user.Role == "admin" || channel.UserID == user.ID || len(channel.AccessControl) == 0 || string(channel.AccessControl) == "null" {
		return true
	}
	var access map[string]any
	if json.Unmarshal(channel.AccessControl, &access) != nil || len(access) == 0 {
		return false
	}
	permissions := []string{permission}
	if permission == "read" {
		permissions = append(permissions, "write")
	}
	for _, name := range permissions {
		entry, _ := access[name].(map[string]any)
		users, _ := entry["user_ids"].([]any)
		for _, id := range users {
			if id == user.ID {
				return true
			}
		}
	}
	return false
}

func (a *App) handleChannels(w http.ResponseWriter, r *http.Request) {
	user, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	channels, err := a.store.ListChannels(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load channels")
		return
	}
	visible := make([]store.Channel, 0, len(channels))
	for _, channel := range channels {
		if channelAccess(user, channel, "read") {
			visible = append(visible, channel)
		}
	}
	writeJSON(w, http.StatusOK, visible)
}

func (a *App) handleChannelCreate(w http.ResponseWriter, r *http.Request) {
	user, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	var form struct {
		Name          string          `json:"name"`
		Description   *string         `json:"description"`
		Data          json.RawMessage `json:"data"`
		Meta          json.RawMessage `json:"meta"`
		AccessControl json.RawMessage `json:"access_control"`
	}
	if !decodeJSON(w, r, &form) {
		return
	}
	form.Name = strings.ToLower(strings.TrimSpace(form.Name))
	if form.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	created, err := a.store.CreateChannel(r.Context(), store.Channel{ID: auth.RandomIDForInternalUse(), UserID: user.ID, Name: form.Name, Description: form.Description, Data: form.Data, Meta: form.Meta, AccessControl: form.AccessControl})
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to create channel")
		return
	}
	writeJSON(w, http.StatusOK, created)
}

func (a *App) channelForRequest(w http.ResponseWriter, r *http.Request, permission string) (store.User, store.Channel, bool) {
	user, ok := a.requireUser(w, r)
	if !ok {
		return store.User{}, store.Channel{}, false
	}
	channel, err := a.store.ChannelByID(r.Context(), r.PathValue("id"))
	if errors.Is(err, store.ErrChannelNotFound) {
		writeError(w, http.StatusNotFound, "Channel not found")
		return store.User{}, store.Channel{}, false
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load channel")
		return store.User{}, store.Channel{}, false
	}
	if !channelAccess(user, channel, permission) {
		writeError(w, http.StatusForbidden, "Access prohibited")
		return store.User{}, store.Channel{}, false
	}
	return user, channel, true
}

func (a *App) handleChannelByID(w http.ResponseWriter, r *http.Request) {
	permission := "read"
	if r.Method != http.MethodGet {
		permission = "write"
	}
	_, channel, ok := a.channelForRequest(w, r, permission)
	if !ok {
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, channel)
	case http.MethodDelete:
		if err := a.store.DeleteChannel(r.Context(), channel.ID); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to delete channel")
			return
		}
		writeJSON(w, http.StatusOK, true)
	case http.MethodPost:
		var patch map[string]json.RawMessage
		if !decodeJSON(w, r, &patch) {
			return
		}
		if value, ok := patch["name"]; ok {
			_ = json.Unmarshal(value, &channel.Name)
		}
		if value, ok := patch["description"]; ok {
			_ = json.Unmarshal(value, &channel.Description)
		}
		if value, ok := patch["data"]; ok {
			channel.Data = value
		}
		if value, ok := patch["meta"]; ok {
			channel.Meta = value
		}
		if value, ok := patch["access_control"]; ok {
			channel.AccessControl = value
		}
		updated, err := a.store.UpdateChannel(r.Context(), channel)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to update channel")
			return
		}
		writeJSON(w, http.StatusOK, updated)
	}
}

func (a *App) channelMessageResponse(r *http.Request, message store.ChannelMessage) map[string]any {
	response := map[string]any{
		"id": message.ID, "user_id": message.UserID, "channel_id": message.ChannelID,
		"parent_id": message.ParentID, "content": message.Content, "data": rawObject(message.Data),
		"meta": rawObject(message.Meta), "created_at": message.CreatedAt, "updated_at": message.UpdatedAt,
	}
	replies, _ := a.store.ListChannelMessages(r.Context(), message.ChannelID, &message.ID, 0, 200)
	response["reply_count"] = len(replies)
	if len(replies) > 0 {
		response["latest_reply_at"] = replies[0].CreatedAt
	} else {
		response["latest_reply_at"] = nil
	}
	reactions, _ := a.store.Reactions(r.Context(), message.ID)
	response["reactions"] = reactions
	if user, err := a.store.UserByID(r.Context(), message.UserID); err == nil {
		response["user"] = map[string]any{"id": user.ID, "name": user.Name, "profile_image_url": user.ProfileImageURL}
	} else if message.UserID == "webhook" {
		response["user"] = map[string]any{"id": "webhook", "name": "Webhook", "profile_image_url": ""}
	}
	return response
}

func (a *App) handleChannelMessages(w http.ResponseWriter, r *http.Request) {
	user, channel, ok := a.channelForRequest(w, r, map[bool]string{true: "write", false: "read"}[r.Method == http.MethodPost])
	if !ok {
		return
	}
	if r.Method == http.MethodGet {
		messages, err := a.store.ListChannelMessages(r.Context(), channel.ID, nil, queryInt(r, "skip", 0, 0, 100000), queryInt(r, "limit", 50, 1, 200))
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to load channel messages")
			return
		}
		result := make([]map[string]any, 0, len(messages))
		for _, message := range messages {
			result = append(result, a.channelMessageResponse(r, message))
		}
		writeJSON(w, http.StatusOK, result)
		return
	}
	var form struct {
		ParentID *string         `json:"parent_id"`
		Content  string          `json:"content"`
		Data     json.RawMessage `json:"data"`
		Meta     json.RawMessage `json:"meta"`
	}
	if !decodeJSON(w, r, &form) {
		return
	}
	if strings.TrimSpace(form.Content) == "" {
		writeError(w, http.StatusBadRequest, "content is required")
		return
	}
	created, err := a.store.CreateChannelMessage(r.Context(), store.ChannelMessage{ID: auth.RandomIDForInternalUse(), UserID: user.ID, ChannelID: channel.ID, ParentID: form.ParentID, Content: form.Content, Data: form.Data, Meta: form.Meta})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create channel message")
		return
	}
	writeJSON(w, http.StatusOK, created)
}

func (a *App) handleChannelMessage(w http.ResponseWriter, r *http.Request) {
	user, channel, ok := a.channelForRequest(w, r, "read")
	if !ok {
		return
	}
	message, err := a.store.ChannelMessageByID(r.Context(), r.PathValue("message_id"))
	if errors.Is(err, store.ErrChannelMessageNotFound) || message.ChannelID != channel.ID {
		writeError(w, http.StatusNotFound, "Message not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load message")
		return
	}
	if strings.HasSuffix(r.URL.Path, "/thread") {
		messages, err := a.store.ListChannelMessages(r.Context(), channel.ID, &message.ID, queryInt(r, "skip", 0, 0, 100000), queryInt(r, "limit", 50, 1, 200))
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to load thread")
			return
		}
		result := make([]map[string]any, 0, len(messages))
		for _, candidate := range messages {
			result = append(result, a.channelMessageResponse(r, candidate))
		}
		writeJSON(w, http.StatusOK, result)
		return
	}
	if r.Method == http.MethodGet {
		writeJSON(w, http.StatusOK, a.channelMessageResponse(r, message))
		return
	}
	if message.UserID != user.ID && user.Role != "admin" {
		writeError(w, http.StatusForbidden, "Access prohibited")
		return
	}
	if r.Method == http.MethodDelete {
		if err := a.store.DeleteChannelMessage(r.Context(), message.ID); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to delete message")
			return
		}
		writeJSON(w, http.StatusOK, true)
		return
	}
	var patch map[string]json.RawMessage
	if !decodeJSON(w, r, &patch) {
		return
	}
	if value, ok := patch["content"]; ok {
		_ = json.Unmarshal(value, &message.Content)
	}
	if value, ok := patch["data"]; ok {
		message.Data = value
	}
	if value, ok := patch["meta"]; ok {
		message.Meta = value
	}
	updated, err := a.store.UpdateChannelMessage(r.Context(), message)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update message")
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (a *App) handleChannelReaction(w http.ResponseWriter, r *http.Request) {
	user, channel, ok := a.channelForRequest(w, r, "read")
	if !ok {
		return
	}
	message, err := a.store.ChannelMessageByID(r.Context(), r.PathValue("message_id"))
	if err != nil || message.ChannelID != channel.ID {
		writeError(w, http.StatusNotFound, "Message not found")
		return
	}
	var form struct {
		Name string `json:"name"`
	}
	if !decodeJSON(w, r, &form) {
		return
	}
	if strings.TrimSpace(form.Name) == "" {
		writeError(w, http.StatusBadRequest, "reaction name is required")
		return
	}
	if strings.HasSuffix(r.URL.Path, "/add") {
		err = a.store.AddReaction(r.Context(), user.ID, message.ID, form.Name, auth.RandomIDForInternalUse())
	} else {
		err = a.store.RemoveReaction(r.Context(), user.ID, message.ID, form.Name)
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update reaction")
		return
	}
	writeJSON(w, http.StatusOK, true)
}

func (a *App) handleChannelWebhook(w http.ResponseWriter, r *http.Request) {
	if ok, _ := a.requireAdmin(w, r); !ok {
		return
	}
	channel, err := a.store.ChannelByID(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, "Channel not found")
		return
	}
	var data map[string]any
	_ = json.Unmarshal(channel.Data, &data)
	if data == nil {
		data = map[string]any{}
	}
	if r.Method == http.MethodGet {
		writeJSON(w, http.StatusOK, map[string]any{"url": data["webhook_url"], "token": data["webhook_token"]})
		return
	}
	if r.Method == http.MethodDelete {
		delete(data, "webhook_url")
		delete(data, "webhook_token")
		channel.Data, _ = json.Marshal(data)
		_, _ = a.store.UpdateChannel(r.Context(), channel)
		writeJSON(w, http.StatusOK, true)
		return
	}
	var form struct {
		URL string `json:"url"`
	}
	if !decodeJSON(w, r, &form) {
		return
	}
	token := auth.RandomIDForInternalUse() + auth.RandomIDForInternalUse()
	data["webhook_url"], data["webhook_token"] = form.URL, token
	channel.Data, _ = json.Marshal(data)
	if _, err := a.store.UpdateChannel(r.Context(), channel); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save webhook")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"url": form.URL, "token": token})
}

func (a *App) handleIncomingChannelWebhook(w http.ResponseWriter, r *http.Request) {
	channel, err := a.store.ChannelByID(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, "Channel not found")
		return
	}
	var data map[string]any
	_ = json.Unmarshal(channel.Data, &data)
	stored, _ := data["webhook_token"].(string)
	provided := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
	if stored == "" || provided == "" || subtle.ConstantTimeCompare([]byte(stored), []byte(provided)) != 1 {
		writeError(w, http.StatusForbidden, "Invalid webhook token")
		return
	}
	var form struct {
		Content  string `json:"content"`
		Username string `json:"username"`
	}
	if !decodeJSON(w, r, &form) {
		return
	}
	if strings.TrimSpace(form.Content) == "" {
		writeError(w, http.StatusBadRequest, "content is required")
		return
	}
	_, err = a.store.CreateChannelMessage(r.Context(), store.ChannelMessage{ID: auth.RandomIDForInternalUse(), UserID: "webhook", ChannelID: channel.ID, Content: form.Content})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create webhook message")
		return
	}
	writeJSON(w, http.StatusOK, true)
}
