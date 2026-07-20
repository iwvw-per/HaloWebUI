package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/iwvw-per/HaloWebUI/backend/internal/auth"
	"github.com/iwvw-per/HaloWebUI/backend/internal/store"
)

func (a *App) handleDeleteAllChats(w http.ResponseWriter, r *http.Request) {
	user, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	if err := a.store.DeleteAllChats(r.Context(), user.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete chats")
		return
	}
	writeJSON(w, http.StatusOK, true)
}

func (a *App) handleChatImportBatch(w http.ResponseWriter, r *http.Request) {
	user, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	var form struct {
		Mode  string `json:"mode"`
		Items []struct {
			Chat        json.RawMessage `json:"chat"`
			Meta        json.RawMessage `json:"meta"`
			Pinned      bool            `json:"pinned"`
			FolderID    *string         `json:"folder_id"`
			AssistantID *string         `json:"assistant_id"`
		} `json:"items"`
	}
	if !decodeJSON(w, r, &form) {
		return
	}
	if form.Mode == "replace" {
		if err := a.store.DeleteAllChats(r.Context(), user.ID); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to replace chats")
			return
		}
	}
	imported := 0
	failures := make([]map[string]any, 0)
	for index, item := range form.Items {
		chat, err := a.store.CreateChat(r.Context(), store.Chat{
			ID: auth.RandomIDForInternalUse(), UserID: user.ID, Chat: item.Chat,
			Meta: item.Meta, Title: titleFromChat(item.Chat), Pinned: item.Pinned, FolderID: item.FolderID, AssistantID: item.AssistantID,
		})
		if err != nil {
			failures = append(failures, map[string]any{"index": index, "detail": err.Error()})
			continue
		}
		imported++
		_ = chat
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"mode": form.Mode, "total": len(form.Items), "imported": imported,
		"failed": len(failures), "failures": failures,
	})
}

func (a *App) handleChatListByUser(w http.ResponseWriter, r *http.Request) {
	if ok, _ := a.requireAdmin(w, r); !ok {
		return
	}
	if !a.config.EnableAdminChatAccess {
		writeError(w, http.StatusForbidden, "Access prohibited")
		return
	}
	userID := r.PathValue("user_id")
	chats, err := a.store.ListChatsWithFilter(r.Context(), userID, store.ChatFilter{}, 1, 200)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list chats")
		return
	}
	writeJSON(w, http.StatusOK, chats)
}

func pageQuery(r *http.Request, fallback int) int {
	page, err := strconv.Atoi(r.URL.Query().Get("page"))
	if err != nil || page < 1 {
		return fallback
	}
	return page
}

func (a *App) handleChatSearch(w http.ResponseWriter, r *http.Request) {
	user, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	chats, err := a.store.ListChatsWithFilter(r.Context(), user.ID, store.ChatFilter{Search: r.URL.Query().Get("text")}, pageQuery(r, 1), 60)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to search chats")
		return
	}
	writeJSON(w, http.StatusOK, chats)
}

func (a *App) handleChatFolderList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	user, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	folder := r.PathValue("folder_id")
	if folder == "" {
		folder = strings.TrimPrefix(strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/v1/chats/folder/"), "/list"), "/")
	}
	chats, err := a.store.ListChatsWithFilter(r.Context(), user.ID, store.ChatFilter{FolderID: &folder}, pageQuery(r, 1), 60)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list folder chats")
		return
	}
	writeJSON(w, http.StatusOK, chats)
}

func (a *App) handleChatAssistantList(w http.ResponseWriter, r *http.Request) {
	user, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	assistant := r.PathValue("assistant_id")
	if assistant == "" {
		assistant = strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/v1/chats/assistant/"), "/list")
	}
	chats, err := a.store.ListChatsWithFilter(r.Context(), user.ID, store.ChatFilter{Assistant: &assistant}, pageQuery(r, 1), 60)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list assistant chats")
		return
	}
	writeJSON(w, http.StatusOK, chats)
}

func (a *App) handlePinnedChats(w http.ResponseWriter, r *http.Request) {
	user, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	pinned := true
	chats, err := a.store.ListChatsWithFilter(r.Context(), user.ID, store.ChatFilter{Pinned: &pinned}, 1, 200)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list pinned chats")
		return
	}
	writeJSON(w, http.StatusOK, chats)
}

func (a *App) handleAllChatTags(w http.ResponseWriter, r *http.Request) {
	user, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	chats, err := a.store.ListChatsWithFilter(r.Context(), user.ID, store.ChatFilter{}, 1, 200)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list tags")
		return
	}
	writeJSON(w, http.StatusOK, chatTags(chats, user.ID))
}

func (a *App) handleAllChatsDB(w http.ResponseWriter, r *http.Request) {
	if ok, _ := a.requireAdmin(w, r); !ok {
		return
	}
	chats, err := a.store.ListAllChats(r.Context(), 1000)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to export chats")
		return
	}
	writeJSON(w, http.StatusOK, chats)
}

func (a *App) handleSharedChats(w http.ResponseWriter, r *http.Request) {
	user, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	chats, err := a.store.ListSharedChats(r.Context(), user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list shared chats")
		return
	}
	writeJSON(w, http.StatusOK, chats)
}

func (a *App) handleSharedChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if _, ok := a.requireUser(w, r); !ok {
		return
	}
	shareID := r.PathValue("share_id")
	if shareID == "" {
		shareID = strings.TrimPrefix(r.URL.Path, "/api/v1/chats/share/")
	}
	chat, err := a.store.ChatByShareID(r.Context(), shareID)
	if errors.Is(err, store.ErrChatNotFound) {
		writeError(w, http.StatusNotFound, "Chat not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load shared chat")
		return
	}
	writeJSON(w, http.StatusOK, chat)
}

func (a *App) handleChatsByTag(w http.ResponseWriter, r *http.Request) {
	user, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	var form struct {
		Name string `json:"name"`
	}
	if !decodeJSON(w, r, &form) {
		return
	}
	chats, err := a.store.ListChatsWithFilter(r.Context(), user.ID, store.ChatFilter{}, 1, 200)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list tagged chats")
		return
	}
	result := make([]store.Chat, 0)
	for _, chat := range chats {
		if chatHasTag(chat, form.Name) {
			result = append(result, chat)
		}
	}
	writeJSON(w, http.StatusOK, result)
}

func (a *App) handleArchiveAllChats(w http.ResponseWriter, r *http.Request) {
	user, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	chats, err := a.store.ListChatsWithFilter(r.Context(), user.ID, store.ChatFilter{}, 1, 200)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to archive chats")
		return
	}
	for _, chat := range chats {
		if _, err := a.store.SetChatField(r.Context(), chat.ID, "archived", true); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to archive chats")
			return
		}
	}
	writeJSON(w, http.StatusOK, true)
}

func (a *App) chatForUser(w http.ResponseWriter, r *http.Request) (store.User, store.Chat, bool) {
	user, ok := a.requireUser(w, r)
	if !ok {
		return store.User{}, store.Chat{}, false
	}
	id := r.PathValue("id")
	if id == "" {
		parts := splitPath(strings.TrimPrefix(r.URL.Path, "/api/v1/chats/"))
		if len(parts) > 0 {
			id = parts[0]
		}
	}
	chat, err := a.store.ChatByID(r.Context(), id)
	if errors.Is(err, store.ErrChatNotFound) {
		writeError(w, http.StatusNotFound, "Chat not found")
		return store.User{}, store.Chat{}, false
	}
	if err != nil || (chat.UserID != user.ID && user.Role != "admin") {
		writeError(w, http.StatusForbidden, "Access prohibited")
		return store.User{}, store.Chat{}, false
	}
	return user, chat, true
}

func (a *App) handleChatPinnedStatus(w http.ResponseWriter, r *http.Request) {
	_, chat, ok := a.chatForUser(w, r)
	if ok {
		writeJSON(w, http.StatusOK, chat.Pinned)
	}
}

func (a *App) handleChatComposerState(w http.ResponseWriter, r *http.Request) {
	_, chat, ok := a.chatForUser(w, r)
	if !ok {
		return
	}
	var form struct {
		ComposerState map[string]any `json:"composer_state"`
	}
	if !decodeJSON(w, r, &form) {
		return
	}
	var payload map[string]any
	_ = json.Unmarshal(chat.Chat, &payload)
	payload["composer_state"] = form.ComposerState
	chat.Chat, _ = json.Marshal(payload)
	updated, err := a.store.UpdateChat(r.Context(), chat)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update composer state")
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (a *App) handleChatMessageUpdate(w http.ResponseWriter, r *http.Request) {
	_, chat, ok := a.chatForUser(w, r)
	if !ok {
		return
	}
	var form struct {
		Content string `json:"content"`
	}
	if !decodeJSON(w, r, &form) {
		return
	}
	var payload map[string]any
	_ = json.Unmarshal(chat.Chat, &payload)
	if messages, ok := payload["messages"].([]any); ok {
		for _, raw := range messages {
			if message, ok := raw.(map[string]any); ok && stringField(message, "id") == r.PathValue("message_id") {
				message["content"] = form.Content
			}
		}
	}
	chat.Chat, _ = json.Marshal(payload)
	updated, err := a.store.UpdateChat(r.Context(), chat)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update chat message")
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (a *App) handleChatMessageEvent(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := a.chatForUser(w, r); ok {
		writeJSON(w, http.StatusOK, true)
	}
}

func (a *App) handleChatClone(w http.ResponseWriter, r *http.Request) {
	user, chat, ok := a.chatForUser(w, r)
	if !ok {
		return
	}
	var form struct {
		Title string `json:"title"`
	}
	if r.ContentLength != 0 && !decodeJSON(w, r, &form) {
		return
	}
	var payload map[string]any
	_ = json.Unmarshal(chat.Chat, &payload)
	payload["originalChatId"] = chat.ID
	title := firstNonEmpty(form.Title, "Clone of "+chat.Title)
	payload["title"] = title
	cloned, err := a.store.CreateChat(r.Context(), store.Chat{ID: auth.RandomIDForInternalUse(), UserID: user.ID, Title: title, Chat: mustJSON(payload), Meta: chat.Meta, AssistantID: chat.AssistantID})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to clone chat")
		return
	}
	writeJSON(w, http.StatusOK, cloned)
}

func (a *App) handleChatCloneShared(w http.ResponseWriter, r *http.Request) {
	user, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	chat, err := a.store.ChatByShareID(r.Context(), r.PathValue("id"))
	if user.Role == "admin" && errors.Is(err, store.ErrChatNotFound) {
		chat, err = a.store.ChatByID(r.Context(), r.PathValue("id"))
	}
	if err != nil {
		writeError(w, http.StatusNotFound, "Chat not found")
		return
	}
	var payload map[string]any
	_ = json.Unmarshal(chat.Chat, &payload)
	payload["originalChatId"] = chat.ID
	title := "Clone of " + chat.Title
	payload["title"] = title
	cloned, err := a.store.CreateChat(r.Context(), store.Chat{ID: auth.RandomIDForInternalUse(), UserID: user.ID, Title: title, Chat: mustJSON(payload)})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to clone shared chat")
		return
	}
	writeJSON(w, http.StatusOK, cloned)
}

func (a *App) handleChatBranch(w http.ResponseWriter, r *http.Request) {
	user, chat, ok := a.chatForUser(w, r)
	if !ok {
		return
	}
	var form struct {
		BranchPointMessageID string `json:"branch_point_message_id"`
		Title                string `json:"title"`
	}
	if !decodeJSON(w, r, &form) {
		return
	}
	var payload map[string]any
	_ = json.Unmarshal(chat.Chat, &payload)
	payload["originalChatId"] = chat.ID
	payload["branchPointMessageId"] = form.BranchPointMessageID
	title := firstNonEmpty(form.Title, chat.Title+" · 分支")
	payload["title"] = title
	branched, err := a.store.CreateChat(r.Context(), store.Chat{ID: auth.RandomIDForInternalUse(), UserID: user.ID, Title: title, Chat: mustJSON(payload), Meta: chat.Meta, FolderID: chat.FolderID, AssistantID: chat.AssistantID})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to branch chat")
		return
	}
	writeJSON(w, http.StatusOK, branched)
}

func (a *App) handleChatShareDelete(w http.ResponseWriter, r *http.Request) {
	_, chat, ok := a.chatForUser(w, r)
	if !ok {
		return
	}
	_, err := a.store.SetChatField(r.Context(), chat.ID, "share_id", nil)
	writeJSON(w, http.StatusOK, err == nil)
}

func (a *App) handleChatTags(w http.ResponseWriter, r *http.Request) {
	_, chat, ok := a.chatForUser(w, r)
	if !ok {
		return
	}
	var meta map[string]any
	_ = json.Unmarshal(chat.Meta, &meta)
	tags := stringSlice(meta["tags"])
	if r.Method == http.MethodGet {
		writeJSON(w, http.StatusOK, tagModels(tags, chat.UserID))
		return
	}
	if strings.HasSuffix(r.URL.Path, "/all") {
		tags = nil
	} else {
		var form struct {
			Name string `json:"name"`
		}
		if !decodeJSON(w, r, &form) {
			return
		}
		id := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(form.Name), " ", "_"))
		if r.Method == http.MethodPost {
			if id != "" && !containsString(tags, id) {
				tags = append(tags, id)
			}
		} else {
			filtered := tags[:0]
			for _, tag := range tags {
				if tag != id {
					filtered = append(filtered, tag)
				}
			}
			tags = filtered
		}
	}
	meta["tags"] = tags
	chat.Meta = mustJSON(meta)
	updated, err := a.store.UpdateChat(r.Context(), chat)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update chat tags")
		return
	}
	_ = updated
	writeJSON(w, http.StatusOK, tagModels(tags, chat.UserID))
}

func chatTags(chats []store.Chat, userID string) []map[string]any {
	seen := map[string]bool{}
	result := make([]map[string]any, 0)
	for _, chat := range chats {
		var meta map[string]any
		_ = json.Unmarshal(chat.Meta, &meta)
		for _, tag := range stringSlice(meta["tags"]) {
			if !seen[tag] {
				seen[tag] = true
				result = append(result, map[string]any{"id": tag, "name": tag, "user_id": userID})
			}
		}
	}
	return result
}

func tagModels(tags []string, userID string) []map[string]any {
	result := make([]map[string]any, 0, len(tags))
	for _, tag := range tags {
		result = append(result, map[string]any{"id": tag, "name": tag, "user_id": userID})
	}
	return result
}

func chatHasTag(chat store.Chat, name string) bool {
	var meta map[string]any
	_ = json.Unmarshal(chat.Meta, &meta)
	id := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(name), " ", "_"))
	return containsString(stringSlice(meta["tags"]), id)
}

func stringSlice(value any) []string {
	values, _ := value.([]any)
	result := make([]string, 0, len(values))
	for _, item := range values {
		if value, ok := item.(string); ok {
			result = append(result, value)
		}
	}
	return result
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func mustJSON(value any) json.RawMessage {
	encoded, _ := json.Marshal(value)
	return encoded
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
