package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/iwvw-per/HaloWebUI/backend/internal/store"
)

type chatCompletedForm struct {
	Model    string           `json:"model"`
	Messages []map[string]any `json:"messages"`
	ChatID   string           `json:"chat_id"`
	ID       string           `json:"id"`
	Tasks    struct {
		Title     bool `json:"title_generation"`
		Tags      bool `json:"tags_generation"`
		FollowUps bool `json:"follow_up_generation"`
	} `json:"background_tasks"`
}

func (a *App) runChatEnhancements(r *http.Request, user store.User, form chatCompletedForm) (map[string]any, error) {
	result := map[string]any{"status": true, "messages": form.Messages}
	if strings.TrimSpace(form.ChatID) == "" {
		return result, nil
	}
	chat, err := a.store.ChatByID(r.Context(), form.ChatID)
	if err != nil || chat.UserID != user.ID {
		return nil, errors.New("chat not found")
	}
	config, err := a.loadTaskConfig(r)
	if err != nil {
		return nil, errors.New("failed to load task config")
	}
	model := taskModelID(form.Model)
	if !form.Tasks.Title && !form.Tasks.Tags && !form.Tasks.FollowUps {
		var settings struct {
			UI struct {
				Title struct {
					Auto *bool `json:"auto"`
				} `json:"title"`
				AutoTags      *bool `json:"autoTags"`
				AutoFollowUps *bool `json:"autoFollowUps"`
			} `json:"ui"`
		}
		if raw, settingsErr := a.store.UserSettings(r.Context(), user.ID); settingsErr == nil && json.Unmarshal(raw, &settings) == nil {
			form.Tasks.Title = settings.UI.Title.Auto == nil || *settings.UI.Title.Auto
			form.Tasks.Tags = settings.UI.AutoTags == nil || *settings.UI.AutoTags
			form.Tasks.FollowUps = settings.UI.AutoFollowUps == nil || *settings.UI.AutoFollowUps
		} else {
			form.Tasks.Title, form.Tasks.Tags, form.Tasks.FollowUps = true, true, true
		}
	}
	if configured, _ := config["TASK_MODEL"].(string); strings.TrimSpace(configured) != "" {
		model = taskModelID(configured)
	}
	if model == "" {
		return nil, errors.New("task model is required")
	}

	if form.Tasks.Title && boolConfig(config, "ENABLE_TITLE_GENERATION") && isDefaultChatTitle(chat.Title) {
		if title, taskErr := a.runTaskCompletion(r, user, "title", model, form.Messages, config); taskErr == nil {
			title = strings.Trim(strings.TrimSpace(title), "\"'` ")
			if title != "" {
				chat, err = a.store.SetChatField(r.Context(), chat.ID, "title", title)
				if err != nil {
					return nil, errors.New("failed to save generated title")
				}
				result["title"] = title
			}
		} else {
			result["title_error"] = taskErr.Error()
		}
	}
	if form.Tasks.Tags && boolConfig(config, "ENABLE_TAGS_GENERATION") {
		if content, taskErr := a.runTaskCompletion(r, user, "tags", model, form.Messages, config); taskErr == nil {
			for _, tag := range taskStringArray(content, "tags") {
				chat = addGeneratedChatTag(chat, tag)
			}
			if updated, updateErr := a.store.UpdateChat(r.Context(), chat); updateErr == nil {
				chat = updated
			} else {
				return nil, errors.New("failed to save generated tags")
			}
			result["tags"] = chatMetaTags(chat)
		} else {
			result["tags_error"] = taskErr.Error()
		}
	}
	if form.Tasks.FollowUps {
		if content, taskErr := a.runTaskCompletion(r, user, "follow_ups", model, form.Messages, config); taskErr == nil {
			followUps := taskStringArray(content, "follow_ups")
			for index := range form.Messages {
				if fmt.Sprint(form.Messages[index]["id"]) == form.ID {
					form.Messages[index]["followUps"] = followUps
				}
			}
			result["messages"] = form.Messages
			result["follow_ups"] = followUps
		} else {
			result["follow_ups_error"] = taskErr.Error()
		}
	}
	return result, nil
}

func (a *App) runTaskCompletion(r *http.Request, user store.User, task, model string, messages []map[string]any, config map[string]any) (string, error) {
	cleanMessages := make([]map[string]any, 0, len(messages)+1)
	for _, message := range messages {
		role, _ := message["role"].(string)
		content := message["content"]
		if role != "user" && role != "assistant" && role != "system" {
			continue
		}
		cleanMessages = append(cleanMessages, map[string]any{"role": role, "content": content})
	}
	cleanMessages = append(cleanMessages, map[string]any{"role": "user", "content": taskInstruction(task, nil, user.Name, config)})
	payload, _ := json.Marshal(map[string]any{"model": model, "messages": cleanMessages, "stream": false, "max_completion_tokens": 1000})
	baseURL, apiKey := a.openAIProviderForUser(r, user)
	target := strings.TrimRight(baseURL, "/") + "/chat/completions"
	request, err := http.NewRequestWithContext(r.Context(), http.MethodPost, target, bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	request.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		request.Header.Set("Authorization", "Bearer "+apiKey)
	}
	upstream, err := (&http.Client{}).Do(request)
	if err != nil {
		return "", errors.New("task provider request failed")
	}
	defer upstream.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(upstream.Body, maxProviderRequestBytes))
	if upstream.StatusCode >= 400 {
		return "", fmt.Errorf("task provider returned %s", upstream.Status)
	}
	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if json.Unmarshal(body, &result) != nil || len(result.Choices) == 0 {
		return "", errors.New("task provider returned invalid response")
	}
	return result.Choices[0].Message.Content, nil
}

func isDefaultChatTitle(title string) bool {
	title = strings.TrimSpace(title)
	return title == "" || title == "New Chat" || title == "新对话"
}
func taskStringArray(content, key string) []string {
	start, end := strings.Index(content, "{"), strings.LastIndex(content, "}")
	if start < 0 || end < start {
		return nil
	}
	var payload map[string]any
	if json.Unmarshal([]byte(content[start:end+1]), &payload) != nil {
		return nil
	}
	return stringSlice(payload[key])
}
func chatMetaTags(chat store.Chat) []string {
	var meta map[string]any
	_ = json.Unmarshal(chat.Meta, &meta)
	return stringSlice(meta["tags"])
}
func addGeneratedChatTag(chat store.Chat, name string) store.Chat {
	var meta map[string]any
	_ = json.Unmarshal(chat.Meta, &meta)
	if meta == nil {
		meta = map[string]any{}
	}
	tags := stringSlice(meta["tags"])
	id := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(name), " ", "_"))
	if id != "" && !containsString(tags, id) {
		tags = append(tags, id)
	}
	meta["tags"] = tags
	chat.Meta = mustJSON(meta)
	return chat
}
