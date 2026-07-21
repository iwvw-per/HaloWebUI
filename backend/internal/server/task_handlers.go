package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/iwvw-per/HaloWebUI/backend/internal/auth"
	"github.com/iwvw-per/HaloWebUI/backend/internal/store"
)

const taskConfigKey = "tasks/config"

func defaultTaskConfig() map[string]any {
	return map[string]any{
		"TASK_MODEL":                               nil,
		"TASK_MODEL_EXTERNAL":                      nil,
		"ENABLE_TITLE_GENERATION":                  true,
		"TITLE_GENERATION_PROMPT_TEMPLATE":         "",
		"IMAGE_PROMPT_GENERATION_PROMPT_TEMPLATE":  "",
		"ENABLE_AUTOCOMPLETE_GENERATION":           true,
		"AUTOCOMPLETE_GENERATION_INPUT_MAX_LENGTH": 4096,
		"TAGS_GENERATION_PROMPT_TEMPLATE":          "",
		"ENABLE_TAGS_GENERATION":                   true,
		"ENABLE_SEARCH_QUERY_GENERATION":           true,
		"ENABLE_RETRIEVAL_QUERY_GENERATION":        true,
		"QUERY_GENERATION_PROMPT_TEMPLATE":         "",
		"TOOLS_FUNCTION_CALLING_PROMPT_TEMPLATE":   "",
		"CODE_INTERPRETER_PROMPT_TEMPLATE":         "",
	}
}

func (a *App) loadTaskConfig(r *http.Request) (map[string]any, error) {
	config := defaultTaskConfig()
	resource, err := a.store.ResourceByKey(r.Context(), "global_setting", taskConfigKey)
	if errors.Is(err, store.ErrResourceNotFound) {
		return config, nil
	}
	if err != nil {
		return nil, err
	}
	var saved map[string]any
	if err := json.Unmarshal(resource.Body, &saved); err != nil {
		return nil, err
	}
	for key, value := range saved {
		if _, ok := config[key]; ok {
			config[key] = value
		}
	}
	return config, nil
}

func (a *App) handleTaskConfig(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.requireUser(w, r); !ok {
		return
	}
	config, err := a.loadTaskConfig(r)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load task config")
		return
	}
	writeJSON(w, http.StatusOK, config)
}

func (a *App) handleTaskConfigUpdate(w http.ResponseWriter, r *http.Request) {
	if ok, _ := a.requireAdmin(w, r); !ok {
		return
	}
	config, err := a.loadTaskConfig(r)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load task config")
		return
	}
	var patch map[string]any
	if !decodeJSON(w, r, &patch) {
		return
	}
	for key, value := range patch {
		if _, allowed := config[key]; !allowed {
			writeError(w, http.StatusBadRequest, "unknown task config field: "+key)
			return
		}
		if value == nil && key != "TASK_MODEL" && key != "TASK_MODEL_EXTERNAL" {
			continue
		}
		config[key] = value
	}
	encoded, _ := json.Marshal(config)
	resource, resourceErr := a.store.ResourceByKey(r.Context(), "global_setting", taskConfigKey)
	if errors.Is(resourceErr, store.ErrResourceNotFound) {
		resource = store.Resource{Kind: "global_setting", ID: auth.RandomIDForInternalUse(), UserID: "system", Key: taskConfigKey, Active: true}
	} else if resourceErr != nil {
		writeError(w, http.StatusInternalServerError, "failed to load task config")
		return
	}
	resource.Body = encoded
	if _, err := a.store.PutResource(r.Context(), resource); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save task config")
		return
	}
	writeJSON(w, http.StatusOK, config)
}

func (a *App) handleTaskCompletion(w http.ResponseWriter, r *http.Request) {
	user, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxProviderRequestBytes)
	var form map[string]json.RawMessage
	if !decodeJSON(w, r, &form) {
		return
	}
	if rawTrue(form["skip_text_enhancements"]) && !strings.Contains(r.URL.Path, "/queries/") {
		writeJSON(w, http.StatusOK, map[string]any{"detail": "Task generation skipped for image session.", "skipped": true})
		return
	}
	var model string
	_ = json.Unmarshal(form["model"], &model)
	if strings.TrimSpace(model) == "" {
		writeError(w, http.StatusBadRequest, "model is required")
		return
	}
	config, err := a.loadTaskConfig(r)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load task config")
		return
	}
	if external, _ := config["TASK_MODEL_EXTERNAL"].(string); external != "" {
		model = external
	} else if internal, _ := config["TASK_MODEL"].(string); internal != "" {
		model = internal
	}
	task := taskName(r.URL.Path)
	if task == "queries" {
		var queryType string
		_ = json.Unmarshal(form["type"], &queryType)
		if queryType == "web_search" && !boolConfig(config, "ENABLE_SEARCH_QUERY_GENERATION") {
			writeError(w, http.StatusBadRequest, "Search query generation is disabled")
			return
		}
		if queryType == "retrieval" && !boolConfig(config, "ENABLE_RETRIEVAL_QUERY_GENERATION") {
			writeError(w, http.StatusBadRequest, "Query generation is disabled")
			return
		}
	}
	if task == "title" && !boolConfig(config, "ENABLE_TITLE_GENERATION") {
		writeError(w, http.StatusBadRequest, "Title generation is disabled")
		return
	}
	if task == "tags" && !boolConfig(config, "ENABLE_TAGS_GENERATION") {
		writeError(w, http.StatusBadRequest, "Tags generation is disabled")
		return
	}
	messages := normalizeTaskMessages(form["messages"])
	instruction := taskInstruction(task, form, user.Name, config)
	messages = append(messages, map[string]any{"role": "user", "content": instruction})
	stream := task == "moa" && rawTrue(form["stream"])
	payload := map[string]any{
		"model": model, "messages": messages, "stream": stream,
	}
	if !stream {
		payload["max_completion_tokens"] = 1000
	}
	encoded, _ := json.Marshal(payload)
	request := r.Clone(r.Context())
	request.Method = http.MethodPost
	request.Body = io.NopCloser(bytes.NewReader(encoded))
	request.ContentLength = int64(len(encoded))
	request.Header = r.Header.Clone()
	request.Header.Set("Content-Type", "application/json")
	baseURL, apiKey := a.openAIProviderForUser(r, user)
	chatID := ""
	_ = json.Unmarshal(form["chat_id"], &chatID)
	taskID, taskContext, finish := a.beginTask(r.Context(), user.ID, chatID, true)
	defer finish()
	w.Header().Set("X-Task-ID", taskID)
	request = request.WithContext(taskContext)
	a.proxyProvider(w, request, baseURL, apiKey, "/chat/completions")
}

func taskName(path string) string {
	parts := splitPath(strings.TrimPrefix(path, "/api/v1/tasks/"))
	if len(parts) == 0 {
		return "task"
	}
	return parts[0]
}

func normalizeTaskMessages(raw json.RawMessage) []map[string]any {
	var messages []map[string]any
	if json.Unmarshal(raw, &messages) == nil {
		for index := range messages {
			if _, ok := messages[index]["role"]; !ok {
				messages[index]["role"] = "user"
			}
		}
		return messages
	}
	var stringsOnly []string
	if json.Unmarshal(raw, &stringsOnly) == nil {
		messages = make([]map[string]any, 0, len(stringsOnly))
		for _, content := range stringsOnly {
			messages = append(messages, map[string]any{"role": "user", "content": content})
		}
	}
	return messages
}

func taskInstruction(task string, form map[string]json.RawMessage, userName string, config map[string]any) string {
	var prompt string
	_ = json.Unmarshal(form["prompt"], &prompt)
	if templateKey := map[string]string{
		"title": "TITLE_GENERATION_PROMPT_TEMPLATE", "tags": "TAGS_GENERATION_PROMPT_TEMPLATE",
		"queries": "QUERY_GENERATION_PROMPT_TEMPLATE", "image_prompt": "IMAGE_PROMPT_GENERATION_PROMPT_TEMPLATE",
	}[task]; templateKey != "" {
		if template, _ := config[templateKey].(string); strings.TrimSpace(template) != "" {
			return strings.ReplaceAll(strings.ReplaceAll(template, "{{prompt}}", prompt), "{{USER_NAME}}", userName)
		}
	}
	switch task {
	case "title":
		return "Generate a concise title for this conversation. Return only the title without quotes."
	case "tags":
		return `Generate up to three short tags for this conversation. Return strict JSON: {"tags":["tag"]}.`
	case "follow_ups":
		return `Generate up to three useful follow-up questions for this conversation. Return strict JSON: {"follow_ups":["question"]}.`
	case "emoji":
		return "Return exactly one emoji that best represents this text: " + prompt
	case "queries":
		return `Generate concise search queries for the request. Return strict JSON: {"queries":["query"]}. Request: ` + prompt
	case "auto":
		return `Complete the user's text briefly. Return strict JSON: {"text":"completion"}. Text: ` + prompt
	case "moa":
		return "Synthesize the candidate responses into one accurate answer. User request: " + prompt + "\nCandidates: " + string(form["responses"])
	case "image_prompt":
		return "Rewrite this as a concise image generation prompt: " + prompt
	default:
		return prompt
	}
}

func rawTrue(raw json.RawMessage) bool {
	var value bool
	_ = json.Unmarshal(raw, &value)
	return value
}

func boolConfig(config map[string]any, key string) bool {
	value, ok := config[key].(bool)
	return ok && value
}

func (a *App) handleTaskControl(w http.ResponseWriter, r *http.Request) {
	user, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	switch {
	case r.URL.Path == "/api/tasks":
		writeJSON(w, http.StatusOK, map[string]any{"tasks": a.taskSnapshot(user.ID)})
	case strings.HasPrefix(r.URL.Path, "/api/tasks/chat/"):
		writeJSON(w, http.StatusOK, map[string]any{"task_ids": a.taskIDsForUserChat(user.ID, r.PathValue("chat_id"))})
	case strings.HasPrefix(r.URL.Path, "/api/tasks/stop/"):
		if !a.stopTask(r.PathValue("id"), user.ID) {
			writeError(w, http.StatusNotFound, "Task with ID "+r.PathValue("id")+" not found.")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": true, "message": "Task successfully stopped."})
	case strings.HasPrefix(r.URL.Path, "/api/chat/actions/"):
		writeError(w, http.StatusNotFound, fmt.Sprintf("action %q is not registered", r.PathValue("action_id")))
	default:
		writeError(w, http.StatusNotFound, "Not found")
	}
}
