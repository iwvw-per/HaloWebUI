package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/iwvw-per/HaloWebUI/backend/internal/auth"
	"github.com/iwvw-per/HaloWebUI/backend/internal/store"
)

type webSearchResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Content string `json:"content"`
}

func retrievalWebConfig(config map[string]any) map[string]any {
	web, _ := config["web"].(map[string]any)
	if web == nil {
		web = map[string]any{}
	}
	return web
}

func configuredResultCount(config map[string]any) int {
	value := 3
	switch raw := config["WEB_SEARCH_RESULT_COUNT"].(type) {
	case float64:
		value = int(raw)
	case int:
		value = raw
	case string:
		if parsed, err := strconv.Atoi(raw); err == nil {
			value = parsed
		}
	}
	if value < 1 {
		return 1
	}
	if value > 10 {
		return 10
	}
	return value
}

func tavilyAPIURL(baseURL, endpoint string, force bool) (string, error) {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		baseURL = "https://api.tavily.com"
	}
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", errors.New("Tavily API URL is invalid")
	}
	if force {
		return strings.TrimRight(parsed.String(), "/"), nil
	}
	wanted := "/" + strings.TrimLeft(endpoint, "/")
	path := strings.TrimRight(parsed.Path, "/")
	if !strings.HasSuffix(path, wanted) {
		path += wanted
	}
	parsed.Path = path
	return parsed.String(), nil
}

func (a *App) searchTavily(r *http.Request, config map[string]any, query string) ([]webSearchResult, error) {
	key, _ := config["TAVILY_API_KEY"].(string)
	if strings.TrimSpace(key) == "" {
		return nil, errors.New("Tavily API key is not configured")
	}
	baseURL, _ := config["TAVILY_SEARCH_API_BASE_URL"].(string)
	force, _ := config["TAVILY_SEARCH_API_FORCE_MODE"].(bool)
	target, err := tavilyAPIURL(baseURL, "search", force)
	if err != nil {
		return nil, err
	}
	payload := map[string]any{
		"api_key": key, "query": query, "max_results": configuredResultCount(config),
		"search_depth": "basic", "include_answer": false, "include_raw_content": false,
	}
	if domains := stringSlice(config["WEB_SEARCH_DOMAIN_FILTER_LIST"]); len(domains) > 0 {
		payload["include_domains"] = domains
	}
	body, _ := json.Marshal(payload)
	request, _ := http.NewRequestWithContext(r.Context(), http.MethodPost, target, bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+key)
	response, err := (&http.Client{Timeout: 20 * time.Second}).Do(request)
	if err != nil {
		return nil, errors.New("Tavily search request failed")
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, 2*1024*1024))
	if err != nil {
		return nil, errors.New("Tavily search response could not be read")
	}
	if response.StatusCode >= 400 {
		return nil, fmt.Errorf("Tavily search returned HTTP %d", response.StatusCode)
	}
	var result struct {
		Results []webSearchResult `json:"results"`
	}
	if json.Unmarshal(data, &result) != nil {
		return nil, errors.New("Tavily search returned invalid JSON")
	}
	return result.Results, nil
}

func (a *App) searchSearXNG(r *http.Request, config map[string]any, query string) ([]webSearchResult, error) {
	template, _ := config["SEARXNG_QUERY_URL"].(string)
	if strings.TrimSpace(template) == "" {
		return nil, errors.New("SearXNG query URL is not configured")
	}
	var target string
	if strings.Contains(template, "<query>") {
		target = strings.ReplaceAll(template, "<query>", url.QueryEscape(query))
	} else {
		parsed, err := url.Parse(template)
		if err != nil {
			return nil, errors.New("SearXNG query URL is invalid")
		}
		values := parsed.Query()
		values.Set("q", query)
		parsed.RawQuery = values.Encode()
		target = parsed.String()
	}
	parsed, err := url.Parse(target)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil, errors.New("SearXNG query URL is invalid")
	}
	values := parsed.Query()
	values.Set("format", "json")
	parsed.RawQuery = values.Encode()
	request, _ := http.NewRequestWithContext(r.Context(), http.MethodGet, parsed.String(), nil)
	request.Header.Set("Accept", "application/json")
	response, err := (&http.Client{Timeout: 20 * time.Second}).Do(request)
	if err != nil {
		return nil, errors.New("SearXNG search request failed")
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, 2*1024*1024))
	if err != nil {
		return nil, errors.New("SearXNG search response could not be read")
	}
	if response.StatusCode >= 400 {
		return nil, fmt.Errorf("SearXNG search returned HTTP %d", response.StatusCode)
	}
	var raw struct {
		Results []struct {
			Title   string `json:"title"`
			URL     string `json:"url"`
			Content string `json:"content"`
		} `json:"results"`
	}
	if json.Unmarshal(data, &raw) != nil {
		return nil, errors.New("SearXNG search returned invalid JSON")
	}
	limit := configuredResultCount(config)
	results := make([]webSearchResult, 0, min(limit, len(raw.Results)))
	for _, item := range raw.Results {
		if len(results) >= limit {
			break
		}
		results = append(results, webSearchResult{Title: item.Title, URL: item.URL, Content: item.Content})
	}
	return results, nil
}

func (a *App) handleRetrievalProcessWebSearch(w http.ResponseWriter, r *http.Request) {
	user, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	var form struct {
		Query          string `json:"query"`
		CollectionName string `json:"collection_name"`
	}
	if !decodeJSON(w, r, &form) {
		return
	}
	form.Query = strings.TrimSpace(form.Query)
	if form.Query == "" {
		writeError(w, http.StatusBadRequest, "query is required")
		return
	}
	config, err := a.loadGlobalJSON(r, retrievalConfigKey, defaultRetrievalConfig())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load web search config")
		return
	}
	web := retrievalWebConfig(config)
	engine, _ := web["WEB_SEARCH_ENGINE"].(string)
	var results []webSearchResult
	switch engine {
	case "tavily":
		results, err = a.searchTavily(r, web, form.Query)
	case "searxng":
		results, err = a.searchSearXNG(r, web, form.Query)
	case "":
		err = errors.New("web search engine is not configured")
	default:
		err = errors.New("configured web search engine is unsupported by the Go backend")
	}
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	if len(results) == 0 {
		writeError(w, http.StatusNotFound, "web search returned no results")
		return
	}
	collection := strings.TrimSpace(form.CollectionName)
	if collection == "" {
		collection = "web-search-" + auth.RandomIDForInternalUse()
	}
	filenames := make([]string, 0, len(results))
	var content strings.Builder
	for _, result := range results {
		if strings.TrimSpace(result.URL) != "" {
			filenames = append(filenames, result.URL)
		}
		content.WriteString(result.Title)
		content.WriteByte('\n')
		content.WriteString(result.Content)
		content.WriteByte('\n')
		content.WriteString(result.URL)
		content.WriteString("\n\n")
	}
	metadata, _ := json.Marshal(map[string]any{"query": form.Query, "engine": engine, "results": results})
	_, err = a.store.UpsertRetrievalDocument(r.Context(), store.RetrievalDocument{
		ID: auth.RandomIDForInternalUse(), Collection: collection, UserID: user.ID,
		Filename: "web-search:" + form.Query, Text: strings.TrimSpace(content.String()), MetadataJSON: string(metadata),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to index web search results")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status": true, "collection_name": collection, "filenames": filenames, "docs": results,
		"loaded_count": len(results), "failed_count": 0, "direct_content_only": true,
	})
}
