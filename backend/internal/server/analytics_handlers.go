package server

import (
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/iwvw-per/HaloWebUI/backend/internal/auth"
	"github.com/iwvw-per/HaloWebUI/backend/internal/store"
)

type analyticsRecord struct {
	UserID           string
	Model            string
	Role             string
	CreatedAt        int64
	PromptTokens     int64
	CompletionTokens int64
}

type analyticsAggregate struct {
	MessageCount          int64 `json:"message_count"`
	TotalPromptTokens     int64 `json:"total_prompt_tokens"`
	TotalCompletionTokens int64 `json:"total_completion_tokens"`
}

func (value analyticsAggregate) response(extra map[string]any) map[string]any {
	result := map[string]any{
		"message_count":           value.MessageCount,
		"total_prompt_tokens":     value.TotalPromptTokens,
		"total_completion_tokens": value.TotalCompletionTokens,
		"total_tokens":            value.TotalPromptTokens + value.TotalCompletionTokens,
	}
	for key, item := range extra {
		result[key] = item
	}
	return result
}

func (a *App) handleAnalyticsData(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/analytics/")
	if path == "cleanup" {
		a.handleAnalyticsCleanup(w, r)
		return
	}
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	days := queryInt(r, "days", 30, 1, 365)
	cutoff := time.Now().Add(-time.Duration(days) * 24 * time.Hour).Unix()
	records, err := a.analyticsRecords(r, cutoff)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load analytics")
		return
	}
	switch path {
	case "models":
		writeJSON(w, http.StatusOK, aggregateAnalyticsModels(records))
	case "users":
		writeJSON(w, http.StatusOK, aggregateAnalyticsUsers(records))
	case "daily":
		writeJSON(w, http.StatusOK, aggregateAnalyticsDaily(records, r.URL.Query().Get("model"), r.URL.Query().Get("timezone")))
	default:
		writeError(w, http.StatusNotFound, "Not found")
	}
}

func (a *App) analyticsRecords(r *http.Request, cutoff int64) ([]analyticsRecord, error) {
	chats, err := a.store.ListAllChats(r.Context(), 1000)
	if err != nil {
		return nil, err
	}
	exclusions, err := a.store.ListResources(r.Context(), "analytics_exclusion", false)
	if err != nil {
		return nil, err
	}
	result := make([]analyticsRecord, 0)
	for _, chat := range chats {
		var body any
		if json.Unmarshal(chat.Chat, &body) != nil {
			continue
		}
		fallback := normalizeEpoch(chat.CreatedAt)
		walkAnalytics(body, chat.UserID, fallback, &result)
	}
	filtered := result[:0]
	for _, record := range result {
		if record.CreatedAt < cutoff || analyticsExcluded(record, exclusions) {
			continue
		}
		filtered = append(filtered, record)
	}
	return filtered, nil
}

func walkAnalytics(value any, userID string, fallback int64, result *[]analyticsRecord) {
	switch item := value.(type) {
	case map[string]any:
		if role, _ := item["role"].(string); role != "" && item["content"] != nil {
			created := fallback
			if value, ok := numberInt64(item["created_at"]); ok {
				created = normalizeEpoch(value)
			}
			model, _ := item["model"].(string)
			prompt, completion := tokenUsage(item)
			*result = append(*result, analyticsRecord{UserID: userID, Model: model, Role: role, CreatedAt: created, PromptTokens: prompt, CompletionTokens: completion})
			return
		}
		for _, child := range item {
			walkAnalytics(child, userID, fallback, result)
		}
	case []any:
		for _, child := range item {
			walkAnalytics(child, userID, fallback, result)
		}
	}
}

func tokenUsage(message map[string]any) (int64, int64) {
	prompt, _ := numberInt64(message["prompt_tokens"])
	completion, _ := numberInt64(message["completion_tokens"])
	if usage, ok := message["usage"].(map[string]any); ok {
		if value, exists := numberInt64(usage["prompt_tokens"]); exists {
			prompt = value
		}
		if value, exists := numberInt64(usage["completion_tokens"]); exists {
			completion = value
		}
		if details, ok := usage["prompt_tokens_details"].(map[string]any); ok && prompt == 0 {
			prompt, _ = numberInt64(details["cached_tokens"])
		}
	}
	return prompt, completion
}
func numberInt64(value any) (int64, bool) {
	switch number := value.(type) {
	case float64:
		return int64(number), true
	case int64:
		return number, true
	case int:
		return int64(number), true
	case json.Number:
		value, err := number.Int64()
		return value, err == nil
	default:
		return 0, false
	}
}
func normalizeEpoch(value int64) int64 {
	if value > 1000000000000 {
		return value / 1000
	}
	if value > 100000000000 {
		return value / 1000
	}
	return value
}

func analyticsExcluded(record analyticsRecord, resources []store.Resource) bool {
	for _, resource := range resources {
		var body map[string]any
		if json.Unmarshal(resource.Body, &body) != nil {
			continue
		}
		model, _ := body["model"].(string)
		cutoff, _ := numberInt64(body["cutoff"])
		createdAfter, _ := numberInt64(body["created_after"])
		if model == record.Model && (cutoff == 0 || record.CreatedAt <= cutoff) && (createdAfter == 0 || record.CreatedAt >= createdAfter) {
			return true
		}
	}
	return false
}

func aggregateAnalyticsModels(records []analyticsRecord) []map[string]any {
	values := map[string]analyticsAggregate{}
	for _, record := range records {
		if record.Model == "" {
			continue
		}
		item := values[record.Model]
		item.MessageCount++
		item.TotalPromptTokens += record.PromptTokens
		item.TotalCompletionTokens += record.CompletionTokens
		values[record.Model] = item
	}
	result := make([]map[string]any, 0, len(values))
	for model, item := range values {
		result = append(result, item.response(map[string]any{"model": model}))
	}
	sort.Slice(result, func(i, j int) bool {
		left, _ := result[i]["message_count"].(int64)
		right, _ := result[j]["message_count"].(int64)
		if left == right {
			return result[i]["model"].(string) < result[j]["model"].(string)
		}
		return left > right
	})
	return result
}
func aggregateAnalyticsUsers(records []analyticsRecord) []map[string]any {
	values := map[string]analyticsAggregate{}
	for _, record := range records {
		item := values[record.UserID]
		item.MessageCount++
		item.TotalPromptTokens += record.PromptTokens
		item.TotalCompletionTokens += record.CompletionTokens
		values[record.UserID] = item
	}
	result := make([]map[string]any, 0, len(values))
	for user, item := range values {
		result = append(result, item.response(map[string]any{"user_id": user}))
	}
	sort.Slice(result, func(i, j int) bool { return result[i]["message_count"].(int64) > result[j]["message_count"].(int64) })
	return result
}
func aggregateAnalyticsDaily(records []analyticsRecord, model, timezone string) []map[string]any {
	location := time.UTC
	if timezone != "" {
		if loaded, err := time.LoadLocation(timezone); err == nil {
			location = loaded
		}
	}
	values := map[string]analyticsAggregate{}
	for _, record := range records {
		if model != "" && record.Model != model {
			continue
		}
		day := time.Unix(record.CreatedAt, 0).In(location).Format("2006-01-02")
		item := values[day]
		item.MessageCount++
		item.TotalPromptTokens += record.PromptTokens
		item.TotalCompletionTokens += record.CompletionTokens
		values[day] = item
	}
	days := make([]string, 0, len(values))
	for day := range values {
		days = append(days, day)
	}
	sort.Strings(days)
	result := make([]map[string]any, 0, len(days))
	for _, day := range days {
		result = append(result, values[day].response(map[string]any{"date": day}))
	}
	return result
}

func (a *App) handleAnalyticsCleanup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var form struct {
		Models []string `json:"models"`
		Days   *int     `json:"days"`
		DryRun bool     `json:"dry_run"`
	}
	if !decodeJSON(w, r, &form) {
		return
	}
	models := make([]string, 0, len(form.Models))
	seen := map[string]bool{}
	for _, model := range form.Models {
		model = strings.TrimSpace(model)
		if model != "" && !seen[model] {
			models = append(models, model)
			seen[model] = true
		}
	}
	if len(models) == 0 {
		writeError(w, http.StatusBadRequest, "No models provided")
		return
	}
	if len(models) > 200 {
		writeError(w, http.StatusBadRequest, "Too many models provided")
		return
	}
	createdAfter := int64(0)
	if form.Days != nil {
		if *form.Days < 1 || *form.Days > 365 {
			writeError(w, http.StatusBadRequest, "days must be between 1 and 365")
			return
		}
		createdAfter = time.Now().Add(-time.Duration(*form.Days) * 24 * time.Hour).Unix()
	}
	records, err := a.analyticsRecords(r, 0)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load analytics")
		return
	}
	selected := map[string]bool{}
	for _, model := range models {
		selected[model] = true
	}
	values := map[string]analyticsAggregate{}
	for _, record := range records {
		if !selected[record.Model] || record.CreatedAt < createdAfter {
			continue
		}
		item := values[record.Model]
		item.MessageCount++
		item.TotalPromptTokens += record.PromptTokens
		item.TotalCompletionTokens += record.CompletionTokens
		values[record.Model] = item
	}
	perModel := make([]map[string]any, 0, len(models))
	totals := analyticsAggregate{}
	for _, model := range models {
		item := values[model]
		perModel = append(perModel, item.response(map[string]any{"model": model}))
		totals.MessageCount += item.MessageCount
		totals.TotalPromptTokens += item.TotalPromptTokens
		totals.TotalCompletionTokens += item.TotalCompletionTokens
	}
	if !form.DryRun {
		now := time.Now().Unix()
		for _, model := range models {
			body, _ := json.Marshal(map[string]any{"model": model, "cutoff": now, "created_after": createdAfter})
			id := auth.RandomIDForInternalUse()
			_, err = a.store.PutResource(r.Context(), store.Resource{Kind: "analytics_exclusion", ID: id, UserID: "system", Key: model + ":" + strconv.FormatInt(now, 10) + ":" + id, Body: body, Active: true})
			if err != nil {
				writeError(w, http.StatusInternalServerError, "failed to persist analytics cleanup")
				return
			}
		}
	}
	response := map[string]any{"requested_models": models, "days": form.Days, "dry_run": form.DryRun, "deleted_rows": int64(0), "per_model": perModel, "totals": totals.response(nil)}
	if !form.DryRun {
		response["deleted_rows"] = totals.MessageCount
	}
	writeJSON(w, http.StatusOK, response)
}
