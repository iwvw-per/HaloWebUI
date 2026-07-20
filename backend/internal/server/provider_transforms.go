package server

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type openAIChatEnvelope struct {
	Model               string            `json:"model"`
	Messages            []json.RawMessage `json:"messages"`
	Stream              bool              `json:"stream"`
	Temperature         *float64          `json:"temperature,omitempty"`
	TopP                *float64          `json:"top_p,omitempty"`
	MaxTokens           int               `json:"max_tokens,omitempty"`
	MaxCompletionTokens int               `json:"max_completion_tokens,omitempty"`
	Stop                any               `json:"stop,omitempty"`
	Tools               []map[string]any  `json:"tools,omitempty"`
}

func decodeOpenAIChat(body []byte) (openAIChatEnvelope, error) {
	var envelope openAIChatEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		return envelope, err
	}
	envelope.Model = strings.TrimSpace(envelope.Model)
	if envelope.Model == "" || len(envelope.Messages) == 0 {
		return envelope, fmt.Errorf("model and messages are required")
	}
	return envelope, nil
}

func decodeOpenAIMessage(raw json.RawMessage) (string, any, []map[string]any, string) {
	var message struct {
		Role       string           `json:"role"`
		Content    json.RawMessage  `json:"content"`
		ToolCalls  []map[string]any `json:"tool_calls"`
		ToolCallID string           `json:"tool_call_id"`
	}
	_ = json.Unmarshal(raw, &message)
	var content any
	if len(message.Content) > 0 {
		_ = json.Unmarshal(message.Content, &content)
	}
	return message.Role, content, message.ToolCalls, message.ToolCallID
}

func plainContent(content any) string {
	switch value := content.(type) {
	case string:
		return value
	case []any:
		var builder strings.Builder
		for _, rawPart := range value {
			part, _ := rawPart.(map[string]any)
			if text, _ := part["text"].(string); text != "" {
				if builder.Len() > 0 {
					builder.WriteByte('\n')
				}
				builder.WriteString(text)
			}
		}
		return builder.String()
	default:
		return ""
	}
}

func anthropicRequest(envelope openAIChatEnvelope) map[string]any {
	messages := make([]map[string]any, 0, len(envelope.Messages))
	system := make([]string, 0)
	for _, raw := range envelope.Messages {
		role, content, toolCalls, toolCallID := decodeOpenAIMessage(raw)
		if role == "system" || role == "developer" {
			if text := plainContent(content); text != "" {
				system = append(system, text)
			}
			continue
		}
		if role == "tool" {
			messages = append(messages, map[string]any{"role": "user", "content": []any{map[string]any{"type": "tool_result", "tool_use_id": toolCallID, "content": plainContent(content)}}})
			continue
		}
		anthropicRole := "user"
		if role == "assistant" {
			anthropicRole = "assistant"
		}
		blocks := make([]any, 0)
		if text := plainContent(content); text != "" {
			blocks = append(blocks, map[string]any{"type": "text", "text": text})
		}
		for _, call := range toolCalls {
			function, _ := call["function"].(map[string]any)
			arguments, _ := function["arguments"].(string)
			var input any = map[string]any{}
			_ = json.Unmarshal([]byte(arguments), &input)
			blocks = append(blocks, map[string]any{"type": "tool_use", "id": call["id"], "name": function["name"], "input": input})
		}
		if len(blocks) == 0 {
			blocks = append(blocks, map[string]any{"type": "text", "text": ""})
		}
		messages = append(messages, map[string]any{"role": anthropicRole, "content": blocks})
	}
	maxTokens := envelope.MaxCompletionTokens
	if maxTokens == 0 {
		maxTokens = envelope.MaxTokens
	}
	if maxTokens == 0 {
		maxTokens = 4096
	}
	payload := map[string]any{"model": envelope.Model, "messages": messages, "max_tokens": maxTokens, "stream": envelope.Stream}
	if len(system) > 0 {
		payload["system"] = strings.Join(system, "\n\n")
	}
	if envelope.Temperature != nil {
		payload["temperature"] = *envelope.Temperature
	}
	if envelope.TopP != nil {
		payload["top_p"] = *envelope.TopP
	}
	if len(envelope.Tools) > 0 {
		tools := make([]map[string]any, 0, len(envelope.Tools))
		for _, tool := range envelope.Tools {
			function, _ := tool["function"].(map[string]any)
			if function == nil {
				continue
			}
			tools = append(tools, map[string]any{"name": function["name"], "description": function["description"], "input_schema": function["parameters"]})
		}
		if len(tools) > 0 {
			payload["tools"] = tools
		}
	}
	return payload
}

func (a *App) proxyAnthropicChat(w http.ResponseWriter, r *http.Request, baseURL, key string, _ map[string]any, body []byte) {
	envelope, err := decodeOpenAIChat(body)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	target, err := providerTarget(baseURL, "/messages")
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	response, err := postNativeProvider(r.Context(), "anthropic", target, key, anthropicRequest(envelope))
	if err != nil {
		writeError(w, http.StatusBadGateway, "anthropic request failed")
		return
	}
	defer response.Body.Close()
	if response.StatusCode >= 400 {
		writeError(w, response.StatusCode, decodeProviderError(response))
		return
	}
	if envelope.Stream {
		a.streamAnthropicAsOpenAI(w, response, envelope.Model)
		return
	}
	var payload struct {
		ID         string           `json:"id"`
		Model      string           `json:"model"`
		Content    []map[string]any `json:"content"`
		StopReason string           `json:"stop_reason"`
		Usage      struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}
	if json.NewDecoder(response.Body).Decode(&payload) != nil {
		writeError(w, http.StatusBadGateway, "anthropic returned invalid JSON")
		return
	}
	text := strings.Builder{}
	toolCalls := make([]map[string]any, 0)
	for _, block := range payload.Content {
		switch block["type"] {
		case "text":
			if value, _ := block["text"].(string); value != "" {
				text.WriteString(value)
			}
		case "tool_use":
			arguments, _ := json.Marshal(block["input"])
			toolCalls = append(toolCalls, map[string]any{"id": block["id"], "type": "function", "function": map[string]any{"name": block["name"], "arguments": string(arguments)}})
		}
	}
	message := map[string]any{"role": "assistant", "content": text.String()}
	if len(toolCalls) > 0 {
		message["tool_calls"] = toolCalls
	}
	finish := "stop"
	if payload.StopReason == "max_tokens" {
		finish = "length"
	} else if payload.StopReason == "tool_use" {
		finish = "tool_calls"
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id": payload.ID, "object": "chat.completion", "created": time.Now().Unix(), "model": payload.Model,
		"choices": []any{map[string]any{"index": 0, "message": message, "finish_reason": finish}},
		"usage":   map[string]any{"prompt_tokens": payload.Usage.InputTokens, "completion_tokens": payload.Usage.OutputTokens, "total_tokens": payload.Usage.InputTokens + payload.Usage.OutputTokens},
	})
}

func (a *App) streamAnthropicAsOpenAI(w http.ResponseWriter, response *http.Response, model string) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	flusher, _ := w.(http.Flusher)
	scanner := bufio.NewScanner(response.Body)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	id := "chatcmpl-anthropic"
	toolIndex := -1
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" || data == "[DONE]" {
			continue
		}
		var event map[string]any
		if json.Unmarshal([]byte(data), &event) != nil {
			continue
		}
		if message, _ := event["message"].(map[string]any); message != nil {
			if value, _ := message["id"].(string); value != "" {
				id = value
			}
		}
		delta := map[string]any{}
		finish := any(nil)
		switch event["type"] {
		case "message_start":
			delta["role"] = "assistant"
		case "content_block_start":
			block, _ := event["content_block"].(map[string]any)
			if block["type"] == "tool_use" {
				toolIndex++
				delta["tool_calls"] = []any{map[string]any{"index": toolIndex, "id": block["id"], "type": "function", "function": map[string]any{"name": block["name"], "arguments": ""}}}
			}
		case "content_block_delta":
			value, _ := event["delta"].(map[string]any)
			if value["type"] == "text_delta" {
				delta["content"] = value["text"]
			} else if value["type"] == "input_json_delta" {
				delta["tool_calls"] = []any{map[string]any{"index": toolIndex, "function": map[string]any{"arguments": value["partial_json"]}}}
			}
		case "message_delta":
			value, _ := event["delta"].(map[string]any)
			finish = anthropicFinish(value["stop_reason"])
		case "message_stop":
			writeSSE(w, "[DONE]")
			if flusher != nil {
				flusher.Flush()
			}
			return
		}
		if len(delta) == 0 && finish == nil {
			continue
		}
		writeOpenAIStreamChunk(w, id, model, delta, finish)
		if flusher != nil {
			flusher.Flush()
		}
	}
	writeSSE(w, "[DONE]")
}

func anthropicFinish(value any) any {
	switch value {
	case "max_tokens":
		return "length"
	case "tool_use":
		return "tool_calls"
	case nil:
		return nil
	default:
		return "stop"
	}
}

func geminiRequest(envelope openAIChatEnvelope) map[string]any {
	contents := make([]map[string]any, 0, len(envelope.Messages))
	system := make([]string, 0)
	for _, raw := range envelope.Messages {
		role, content, _, _ := decodeOpenAIMessage(raw)
		text := plainContent(content)
		if role == "system" || role == "developer" {
			system = append(system, text)
			continue
		}
		geminiRole := "user"
		if role == "assistant" {
			geminiRole = "model"
		}
		contents = append(contents, map[string]any{"role": geminiRole, "parts": []any{map[string]any{"text": text}}})
	}
	generation := map[string]any{}
	if envelope.Temperature != nil {
		generation["temperature"] = *envelope.Temperature
	}
	if envelope.TopP != nil {
		generation["topP"] = *envelope.TopP
	}
	maxTokens := envelope.MaxCompletionTokens
	if maxTokens == 0 {
		maxTokens = envelope.MaxTokens
	}
	if maxTokens > 0 {
		generation["maxOutputTokens"] = maxTokens
	}
	payload := map[string]any{"contents": contents}
	if len(system) > 0 {
		payload["systemInstruction"] = map[string]any{"parts": []any{map[string]any{"text": strings.Join(system, "\n\n")}}}
	}
	if len(generation) > 0 {
		payload["generationConfig"] = generation
	}
	return payload
}

func (a *App) proxyGeminiChat(w http.ResponseWriter, r *http.Request, baseURL, key string, _ map[string]any, body []byte) {
	envelope, err := decodeOpenAIChat(body)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	model := strings.TrimPrefix(envelope.Model, "models/")
	suffix := "/models/" + url.PathEscape(model) + ":generateContent"
	if envelope.Stream {
		suffix = "/models/" + url.PathEscape(model) + ":streamGenerateContent?alt=sse"
	}
	target, err := providerTarget(baseURL, suffix)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	response, err := postNativeProvider(r.Context(), "gemini", target, key, geminiRequest(envelope))
	if err != nil {
		writeError(w, http.StatusBadGateway, "gemini request failed")
		return
	}
	defer response.Body.Close()
	if response.StatusCode >= 400 {
		writeError(w, response.StatusCode, decodeProviderError(response))
		return
	}
	if envelope.Stream {
		a.streamGeminiAsOpenAI(w, response, model)
		return
	}
	var payload map[string]any
	if json.NewDecoder(response.Body).Decode(&payload) != nil {
		writeError(w, http.StatusBadGateway, "gemini returned invalid JSON")
		return
	}
	text, finish := geminiCandidate(payload)
	usage, _ := payload["usageMetadata"].(map[string]any)
	writeJSON(w, http.StatusOK, map[string]any{
		"id": "chatcmpl-gemini", "object": "chat.completion", "created": time.Now().Unix(), "model": model,
		"choices": []any{map[string]any{"index": 0, "message": map[string]any{"role": "assistant", "content": text}, "finish_reason": finish}},
		"usage":   map[string]any{"prompt_tokens": usage["promptTokenCount"], "completion_tokens": usage["candidatesTokenCount"], "total_tokens": usage["totalTokenCount"]},
	})
}

func geminiCandidate(payload map[string]any) (string, string) {
	candidates, _ := payload["candidates"].([]any)
	if len(candidates) == 0 {
		return "", "stop"
	}
	candidate, _ := candidates[0].(map[string]any)
	content, _ := candidate["content"].(map[string]any)
	parts, _ := content["parts"].([]any)
	var text strings.Builder
	for _, rawPart := range parts {
		part, _ := rawPart.(map[string]any)
		if value, _ := part["text"].(string); value != "" {
			text.WriteString(value)
		}
	}
	finish := "stop"
	if reason, _ := candidate["finishReason"].(string); reason == "MAX_TOKENS" {
		finish = "length"
	}
	return text.String(), finish
}

func (a *App) streamGeminiAsOpenAI(w http.ResponseWriter, response *http.Response, model string) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	flusher, _ := w.(http.Flusher)
	scanner := bufio.NewScanner(response.Body)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	started := false
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		var payload map[string]any
		if json.Unmarshal([]byte(data), &payload) != nil {
			continue
		}
		text, finish := geminiCandidate(payload)
		delta := map[string]any{"content": text}
		if !started {
			delta["role"] = "assistant"
			started = true
		}
		var finishValue any
		candidates, _ := payload["candidates"].([]any)
		if len(candidates) > 0 {
			candidate, _ := candidates[0].(map[string]any)
			if candidate["finishReason"] != nil {
				finishValue = finish
			}
		}
		writeOpenAIStreamChunk(w, "chatcmpl-gemini", model, delta, finishValue)
		if flusher != nil {
			flusher.Flush()
		}
	}
	writeSSE(w, "[DONE]")
}

func writeOpenAIStreamChunk(w http.ResponseWriter, id, model string, delta map[string]any, finish any) {
	payload, _ := json.Marshal(map[string]any{
		"id": id, "object": "chat.completion.chunk", "created": time.Now().Unix(), "model": model,
		"choices": []any{map[string]any{"index": 0, "delta": delta, "finish_reason": finish}},
	})
	writeSSE(w, string(payload))
}

func writeSSE(w http.ResponseWriter, data string) {
	_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
}
