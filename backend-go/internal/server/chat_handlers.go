package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/iwvw-per/HaloWebUI/backend-go/internal/auth"
	"github.com/iwvw-per/HaloWebUI/backend-go/internal/store"
)

func (a *App) requireUser(response http.ResponseWriter, request *http.Request) (store.User, bool) {
	user, ok := a.currentUser(request)
	if !ok {
		writeError(response, http.StatusUnauthorized, "Not authenticated")
	}
	return user, ok
}

func (a *App) handleChatNew(response http.ResponseWriter, request *http.Request) {
	user, ok := a.requireUser(response, request)
	if !ok {
		return
	}
	var form struct {
		Chat        json.RawMessage `json:"chat"`
		FolderID    *string         `json:"folder_id"`
		AssistantID *string         `json:"assistant_id"`
	}
	if !decodeJSON(response, request, &form) {
		return
	}
	chat := store.Chat{
		ID:          auth.RandomIDForInternalUse(),
		UserID:      user.ID,
		Chat:        form.Chat,
		FolderID:    form.FolderID,
		AssistantID: form.AssistantID,
		Title:       titleFromChat(form.Chat),
	}
	created, err := a.store.CreateChat(request.Context(), chat)
	if err != nil {
		writeError(response, http.StatusInternalServerError, "failed to create chat")
		return
	}
	writeJSON(response, http.StatusOK, created)
}

func (a *App) handleChatImport(response http.ResponseWriter, request *http.Request) {
	user, ok := a.requireUser(response, request)
	if !ok {
		return
	}
	var form struct {
		Chat        json.RawMessage `json:"chat"`
		Meta        json.RawMessage `json:"meta"`
		Pinned      bool            `json:"pinned"`
		FolderID    *string         `json:"folder_id"`
		AssistantID *string         `json:"assistant_id"`
	}
	if !decodeJSON(response, request, &form) {
		return
	}
	created, err := a.store.CreateChat(request.Context(), store.Chat{
		ID:          auth.RandomIDForInternalUse(),
		UserID:      user.ID,
		Chat:        form.Chat,
		Meta:        form.Meta,
		Pinned:      form.Pinned,
		FolderID:    form.FolderID,
		AssistantID: form.AssistantID,
		Title:       titleFromChat(form.Chat),
	})
	if err != nil {
		writeError(response, http.StatusInternalServerError, "failed to import chat")
		return
	}
	writeJSON(response, http.StatusOK, created)
}

func (a *App) handleChatList(response http.ResponseWriter, request *http.Request) {
	if strings.HasPrefix(request.URL.Path, "/api/v1/chats/folder/") && !strings.HasSuffix(request.URL.Path, "/list") {
		a.handleChatFolderList(response, request)
		return
	}
	if strings.HasPrefix(request.URL.Path, "/api/v1/chats/assistant/") {
		a.handleChatAssistantList(response, request)
		return
	}
	if strings.HasPrefix(request.URL.Path, "/api/v1/chats/list/user/") {
		a.handleChatListByUser(response, request)
		return
	}
	if strings.HasPrefix(request.URL.Path, "/api/v1/chats/share/") {
		a.handleSharedChat(response, request)
		return
	}
	user, ok := a.requireUser(response, request)
	if !ok {
		return
	}
	page, _ := strconv.Atoi(request.URL.Query().Get("page"))
	var archived *bool
	if request.URL.Path == "/api/v1/chats/archived" || request.URL.Path == "/api/v1/chats/all/archived" {
		value := true
		archived = &value
	}
	chats, err := a.store.ListChats(request.Context(), user.ID, archived, page, 60)
	if err != nil {
		writeError(response, http.StatusInternalServerError, "failed to list chats")
		return
	}
	writeJSON(response, http.StatusOK, chats)
}

func (a *App) handleChatByID(response http.ResponseWriter, request *http.Request) {
	user, ok := a.requireUser(response, request)
	if !ok {
		return
	}
	id := request.PathValue("id")
	chat, err := a.store.ChatByID(request.Context(), id)
	if errors.Is(err, store.ErrChatNotFound) {
		writeError(response, http.StatusNotFound, "Chat not found")
		return
	}
	if err != nil {
		writeError(response, http.StatusInternalServerError, "failed to load chat")
		return
	}
	if chat.UserID != user.ID && user.Role != "admin" {
		writeError(response, http.StatusForbidden, "Access prohibited")
		return
	}
	switch request.Method {
	case http.MethodGet:
		writeJSON(response, http.StatusOK, chat)
	case http.MethodDelete:
		if err := a.store.DeleteChat(request.Context(), id); err != nil {
			writeError(response, http.StatusInternalServerError, "failed to delete chat")
			return
		}
		writeJSON(response, http.StatusOK, map[string]bool{"status": true})
	case http.MethodPut, http.MethodPost:
		a.updateChat(response, request, chat)
	default:
		http.NotFound(response, request)
	}
}

func (a *App) updateChat(response http.ResponseWriter, request *http.Request, chat store.Chat) {
	var payload map[string]json.RawMessage
	if !decodeJSON(response, request, &payload) {
		return
	}
	if value, exists := payload["chat"]; exists {
		chat.Chat = value
	}
	if value, exists := payload["meta"]; exists {
		chat.Meta = value
	}
	if value, exists := payload["title"]; exists {
		_ = json.Unmarshal(value, &chat.Title)
	}
	if value, exists := payload["pinned"]; exists {
		_ = json.Unmarshal(value, &chat.Pinned)
	}
	if value, exists := payload["archived"]; exists {
		_ = json.Unmarshal(value, &chat.Archived)
	}
	updated, err := a.store.UpdateChat(request.Context(), chat)
	if err != nil {
		writeError(response, http.StatusInternalServerError, "failed to update chat")
		return
	}
	writeJSON(response, http.StatusOK, updated)
}

func (a *App) handleChatField(response http.ResponseWriter, request *http.Request) {
	user, ok := a.requireUser(response, request)
	if !ok {
		return
	}
	id := request.PathValue("id")
	chat, err := a.store.ChatByID(request.Context(), id)
	if err != nil {
		writeError(response, http.StatusNotFound, "Chat not found")
		return
	}
	if chat.UserID != user.ID && user.Role != "admin" {
		writeError(response, http.StatusForbidden, "Access prohibited")
		return
	}
	field := ""
	var value any
	switch {
	case strings.HasSuffix(request.URL.Path, "/archive"):
		field, value = "archived", !chat.Archived
	case strings.HasSuffix(request.URL.Path, "/pin"):
		field, value = "pinned", !chat.Pinned
	case strings.HasSuffix(request.URL.Path, "/share"):
		field = "share_id"
		if chat.ShareID != nil && *chat.ShareID != "" {
			value = *chat.ShareID
		} else {
			value = auth.RandomIDForInternalUse()
		}
	case strings.HasSuffix(request.URL.Path, "/folder"):
		var form struct {
			FolderID *string `json:"folder_id"`
		}
		if !decodeJSON(response, request, &form) {
			return
		}
		field, value = "folder_id", form.FolderID
	case strings.HasSuffix(request.URL.Path, "/title"):
		var form struct {
			Title string `json:"title"`
		}
		if !decodeJSON(response, request, &form) {
			return
		}
		field, value = "title", form.Title
	default:
		http.NotFound(response, request)
		return
	}
	updated, err := a.store.SetChatField(request.Context(), id, field, value)
	if err != nil {
		writeError(response, http.StatusInternalServerError, "failed to update chat")
		return
	}
	writeJSON(response, http.StatusOK, updated)
}

func (a *App) handleChatContext(response http.ResponseWriter, request *http.Request) {
	if _, ok := a.requireUser(response, request); !ok {
		return
	}
	writeJSON(response, http.StatusOK, map[string]any{
		"tags":     []any{},
		"task_ids": []string{},
	})
}

func titleFromChat(raw json.RawMessage) string {
	var payload struct {
		Title string `json:"title"`
	}
	if json.Unmarshal(raw, &payload) == nil && strings.TrimSpace(payload.Title) != "" {
		return payload.Title
	}
	return "New Chat"
}
