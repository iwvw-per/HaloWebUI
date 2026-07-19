package server

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/iwvw-per/HaloWebUI/backend-go/internal/auth"
	"github.com/iwvw-per/HaloWebUI/backend-go/internal/store"
)

var defaultUserPermissions = json.RawMessage(`{
	"workspace":{"models":false,"knowledge":false,"prompts":false,"tools":false},
	"sharing":{"public_models":true,"public_knowledge":true,"public_prompts":true,"public_tools":true},
	"chat":{"controls":true,"file_upload":true,"delete":true,"edit":true,"stt":true,"tts":true,"call":true,"multiple_models":true,"temporary":true,"temporary_enforced":false},
	"features":{"direct_tool_servers":false,"web_search":true,"image_generation":true,"code_interpreter":true}
}`)

var defaultNewUserSettings = json.RawMessage(`{
	"configured":false,"enabled":false,"roles":["user","pending"],"ui":{},"tools":{"native_tools":{}}
}`)

func (a *App) requireAdmin(response http.ResponseWriter, request *http.Request) (bool, string) {
	user, ok := a.requireUser(response, request)
	if !ok {
		return false, ""
	}
	if user.Role != "admin" {
		writeError(response, http.StatusForbidden, "Access prohibited")
		return false, ""
	}
	return true, user.ID
}

func (a *App) handleUsers(response http.ResponseWriter, request *http.Request) {
	if ok, _ := a.requireAdmin(response, request); !ok {
		return
	}
	limit, _ := strconv.Atoi(request.URL.Query().Get("limit"))
	users, err := a.store.ListUsers(request.Context(), request.URL.Query().Get("query"), limit)
	if err != nil {
		writeError(response, http.StatusInternalServerError, "failed to list users")
		return
	}
	writeJSON(response, http.StatusOK, users)
}

func (a *App) handleUserByID(response http.ResponseWriter, request *http.Request) {
	if ok, _ := a.requireAdmin(response, request); !ok {
		return
	}
	id := request.PathValue("id")
	switch request.Method {
	case http.MethodGet:
		user, err := a.store.UserByID(request.Context(), id)
		if err != nil {
			writeError(response, http.StatusNotFound, "User not found")
			return
		}
		writeJSON(response, http.StatusOK, user)
	case http.MethodDelete:
		if err := a.store.DeleteUser(request.Context(), id); err != nil {
			writeError(response, http.StatusInternalServerError, "failed to delete user")
			return
		}
		writeJSON(response, http.StatusOK, map[string]bool{"status": true})
	case http.MethodPost:
		var form struct {
			Name            string `json:"name"`
			Email           string `json:"email"`
			Role            string `json:"role"`
			ProfileImageURL string `json:"profile_image_url"`
		}
		if !decodeJSON(response, request, &form) {
			return
		}
		user, err := a.store.UpdateUser(request.Context(), id, form.Name, form.Email, form.Role, form.ProfileImageURL)
		if err != nil {
			writeError(response, http.StatusInternalServerError, "failed to update user")
			return
		}
		writeJSON(response, http.StatusOK, user)
	}
}

func (a *App) handleUserRole(response http.ResponseWriter, request *http.Request) {
	if ok, _ := a.requireAdmin(response, request); !ok {
		return
	}
	var form struct {
		ID   string `json:"id"`
		Role string `json:"role"`
	}
	if !decodeJSON(response, request, &form) {
		return
	}
	user, err := a.store.UpdateUser(request.Context(), form.ID, "", "", form.Role, "")
	if err != nil {
		writeError(response, http.StatusInternalServerError, "failed to update role")
		return
	}
	writeJSON(response, http.StatusOK, user)
}

func (a *App) handleDefaultUserPermissions(response http.ResponseWriter, request *http.Request) {
	a.handleGlobalJSONSetting(response, request, "users/default/permissions", defaultUserPermissions, nil)
}

func (a *App) handleDefaultUserSettings(response http.ResponseWriter, request *http.Request) {
	a.handleGlobalJSONSetting(response, request, "users/default/settings", defaultNewUserSettings, sanitizeNewUserDefaults)
}

func (a *App) handleGlobalJSONSetting(response http.ResponseWriter, request *http.Request, key string, fallback json.RawMessage, sanitize func(map[string]any) map[string]any) {
	if ok, _ := a.requireAdmin(response, request); !ok {
		return
	}
	const kind = "global_setting"
	resource, err := a.store.ResourceByKey(request.Context(), kind, key)
	if err != nil && !errors.Is(err, store.ErrResourceNotFound) {
		writeError(response, http.StatusInternalServerError, "failed to load setting")
		return
	}
	if request.Method == http.MethodGet {
		if errors.Is(err, store.ErrResourceNotFound) {
			writeRawJSON(response, http.StatusOK, fallback)
			return
		}
		writeRawJSON(response, http.StatusOK, resource.Body)
		return
	}

	var body map[string]any
	if !decodeJSON(response, request, &body) {
		return
	}
	if body == nil {
		writeError(response, http.StatusBadRequest, "setting must be a JSON object")
		return
	}
	if sanitize != nil {
		body = sanitize(body)
	}
	encoded, err := json.Marshal(body)
	if err != nil {
		writeError(response, http.StatusBadRequest, "invalid setting")
		return
	}
	if resource.ID == "" {
		resource = store.Resource{
			Kind: kind, ID: auth.RandomIDForInternalUse(), UserID: "system", Key: key, Active: true,
		}
	}
	resource.Body = encoded
	if _, err := a.store.PutResource(request.Context(), resource); err != nil {
		writeError(response, http.StatusInternalServerError, "failed to save setting")
		return
	}
	writeRawJSON(response, http.StatusOK, encoded)
}

func sanitizeNewUserDefaults(raw map[string]any) map[string]any {
	ui := map[string]any{}
	rawUI, _ := raw["ui"].(map[string]any)
	for _, key := range []string{
		"showFeaturedAssistantsOnHome", "showChatTitleInTab", "chatBubble", "showUsername",
		"widescreenMode", "notificationSound", "enableAutoScrollOnStreaming", "richTextInput",
		"promptAutocomplete", "showFormattingToolbar", "insertPromptAsRichText", "largeTextAsFile",
		"copyFormatted", "copyFormattedUserSet", "ctrlEnterToSend", "autoTags", "autoFollowUps",
		"detectArtifacts", "svgPreviewAutoOpen", "responseAutoCopy", "temporaryChatByDefault",
		"newChatInheritsPreviousState", "collapseCodeBlocks", "collapseHistoricalLongResponses",
		"showInlineCitations", "showMessageOutline", "expandDetails", "insertSuggestionPrompt",
		"keepFollowUpPrompts", "insertFollowUpPrompt", "displayMultiModelResponsesInTabs",
		"showFloatingActionButtons",
	} {
		if value, ok := rawUI[key].(bool); ok {
			ui[key] = value
		}
	}
	if value, ok := rawUI["system"].(string); ok {
		ui["system"] = truncateRunes(value, 12000)
	}
	if values, ok := rawUI["models"].([]any); ok {
		models := make([]string, 0, min(len(values), 200))
		for _, value := range values[:min(len(values), 200)] {
			if model, ok := value.(string); ok {
				models = append(models, truncateRunes(model, 400))
			}
		}
		ui["models"] = models
	}
	if title, ok := rawUI["title"].(map[string]any); ok {
		if automatic, ok := title["auto"].(bool); ok {
			ui["title"] = map[string]bool{"auto": automatic}
		}
	}
	if buttons, present := rawUI["floatingActionButtons"].([]any); present {
		cleaned := make([]map[string]any, 0, min(len(buttons), 20))
		for _, value := range buttons[:min(len(buttons), 20)] {
			button, ok := value.(map[string]any)
			if !ok {
				continue
			}
			id, idOK := button["id"].(string)
			label, labelOK := button["label"].(string)
			input, inputOK := button["input"].(bool)
			prompt, promptOK := button["prompt"].(string)
			if !idOK || !labelOK || !inputOK || !promptOK || id == "" || label == "" {
				continue
			}
			cleaned = append(cleaned, map[string]any{
				"id": truncateRunes(id, 80), "label": truncateRunes(label, 80),
				"input": input, "prompt": truncateRunes(prompt, 8000),
			})
		}
		ui["floatingActionButtons"] = cleaned
	}
	configured, _ := raw["configured"].(bool)
	return map[string]any{
		"configured": configured || len(ui) > 0,
		"enabled":    len(ui) > 0,
		"roles":      []string{"user", "pending"},
		"ui":         ui,
		"tools":      map[string]any{"native_tools": map[string]any{}},
	}
}

func truncateRunes(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit])
}

func (a *App) applyNewUserDefaults(request *http.Request, user store.User) {
	resource, err := a.store.ResourceByKey(request.Context(), "global_setting", "users/default/settings")
	if err != nil {
		return
	}
	var template struct {
		Enabled bool           `json:"enabled"`
		Roles   []string       `json:"roles"`
		UI      map[string]any `json:"ui"`
	}
	if json.Unmarshal(resource.Body, &template) != nil || !template.Enabled || len(template.UI) == 0 {
		return
	}
	allowed := false
	for _, role := range template.Roles {
		if role == user.Role {
			allowed = true
			break
		}
	}
	if !allowed {
		return
	}
	settings, _ := json.Marshal(map[string]any{"ui": template.UI, "revision": 0})
	_, _ = a.store.SetUserSettings(request.Context(), user.ID, settings)
}

func (a *App) handleCurrentUserSettings(response http.ResponseWriter, request *http.Request) {
	user, ok := a.requireUser(response, request)
	if !ok {
		return
	}
	if request.Method == http.MethodGet {
		settings, err := a.store.UserSettings(request.Context(), user.ID)
		if err != nil {
			writeError(response, http.StatusInternalServerError, "failed to load settings")
			return
		}
		writeRawJSON(response, http.StatusOK, settings)
		return
	}
	var settings json.RawMessage
	if !decodeJSON(response, request, &settings) {
		return
	}
	updated, err := a.store.SetUserSettings(request.Context(), user.ID, settings)
	if err != nil {
		writeError(response, http.StatusInternalServerError, "failed to update settings")
		return
	}
	writeRawJSON(response, http.StatusOK, updated)
}

func (a *App) handleCurrentUserInfo(response http.ResponseWriter, request *http.Request) {
	user, ok := a.requireUser(response, request)
	if !ok {
		return
	}
	if request.Method == http.MethodGet {
		info, err := a.store.UserInfo(request.Context(), user.ID)
		if err != nil {
			writeError(response, http.StatusInternalServerError, "failed to load info")
			return
		}
		writeRawJSON(response, http.StatusOK, info)
		return
	}
	var info json.RawMessage
	if !decodeJSON(response, request, &info) {
		return
	}
	updated, err := a.store.SetUserInfo(request.Context(), user.ID, info)
	if err != nil {
		writeError(response, http.StatusInternalServerError, "failed to update info")
		return
	}
	writeRawJSON(response, http.StatusOK, updated)
}

func (a *App) handleUsersCSV(response http.ResponseWriter, request *http.Request) {
	if ok, _ := a.requireAdmin(response, request); !ok {
		return
	}
	users, err := a.store.ListUsers(request.Context(), "", 500)
	if err != nil {
		writeError(response, http.StatusInternalServerError, "failed to list users")
		return
	}
	response.Header().Set("Content-Type", "text/csv; charset=utf-8")
	response.Header().Set("Content-Disposition", `attachment; filename="users.csv"`)
	writer := csv.NewWriter(response)
	_ = writer.Write([]string{"id", "name", "email", "role", "created_at"})
	for _, user := range users {
		_ = writer.Write([]string{user.ID, user.Name, user.Email, user.Role, strconv.FormatInt(user.CreatedAt, 10)})
	}
	writer.Flush()
}
