package server

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/iwvw-per/HaloWebUI/backend/internal/auth"
	"github.com/iwvw-per/HaloWebUI/backend/internal/store"
)

type youtubeCaptionTrack struct {
	BaseURL      string `json:"baseUrl"`
	LanguageCode string `json:"languageCode"`
	Kind         string `json:"kind"`
}

type youtubeTranscriptLoader interface {
	Load(context.Context, string, map[string]any) (string, string, error)
}

type goYouTubeTranscriptLoader struct{}

func youtubeVideoID(rawURL string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", errors.New("YouTube URL is invalid")
	}
	host := strings.ToLower(strings.TrimPrefix(parsed.Hostname(), "www."))
	var id string
	switch host {
	case "youtu.be":
		id = strings.Trim(strings.Split(strings.Trim(parsed.Path, "/"), "/")[0], " ")
	case "youtube.com", "m.youtube.com", "music.youtube.com":
		id = parsed.Query().Get("v")
		if id == "" {
			parts := splitPath(parsed.Path)
			if len(parts) == 2 && (parts[0] == "shorts" || parts[0] == "embed" || parts[0] == "live") {
				id = parts[1]
			}
		}
	default:
		return "", errors.New("URL is not a supported YouTube host")
	}
	if len(id) != 11 {
		return "", errors.New("YouTube video ID is invalid")
	}
	for _, char := range id {
		if !(char == '-' || char == '_' || char >= '0' && char <= '9' || char >= 'A' && char <= 'Z' || char >= 'a' && char <= 'z') {
			return "", errors.New("YouTube video ID is invalid")
		}
	}
	return id, nil
}

func extractJSONArray(source []byte, marker string) ([]byte, bool) {
	index := strings.Index(string(source), marker)
	if index < 0 {
		return nil, false
	}
	start := index + len(marker)
	for start < len(source) && source[start] != '[' {
		start++
	}
	if start >= len(source) {
		return nil, false
	}
	depth := 0
	inString := false
	escaped := false
	for cursor := start; cursor < len(source); cursor++ {
		char := source[cursor]
		if inString {
			if escaped {
				escaped = false
				continue
			}
			if char == '\\' {
				escaped = true
			} else if char == '"' {
				inString = false
			}
			continue
		}
		if char == '"' {
			inString = true
			continue
		}
		switch char {
		case '[':
			depth++
		case ']':
			depth--
			if depth == 0 {
				return source[start : cursor+1], true
			}
		}
	}
	return nil, false
}

func selectCaptionTrack(tracks []youtubeCaptionTrack, languages []string) (youtubeCaptionTrack, bool) {
	for _, language := range languages {
		language = strings.ToLower(strings.TrimSpace(language))
		for _, track := range tracks {
			if strings.EqualFold(track.LanguageCode, language) {
				return track, true
			}
		}
	}
	for _, track := range tracks {
		if track.Kind != "asr" {
			return track, true
		}
	}
	if len(tracks) > 0 {
		return tracks[0], true
	}
	return youtubeCaptionTrack{}, false
}

func parseYouTubeTranscript(data []byte) (string, error) {
	var json3 struct {
		Events []struct {
			Segments []struct {
				Text string `json:"utf8"`
			} `json:"segs"`
		} `json:"events"`
	}
	if json.Unmarshal(data, &json3) == nil && len(json3.Events) > 0 {
		var text strings.Builder
		for _, event := range json3.Events {
			for _, segment := range event.Segments {
				value := strings.TrimSpace(segment.Text)
				if value != "" && value != "\n" {
					text.WriteString(value)
					text.WriteByte(' ')
				}
			}
		}
		if result := strings.TrimSpace(text.String()); result != "" {
			return result, nil
		}
	}
	var transcript struct {
		Texts []struct {
			Value string `xml:",chardata"`
		} `xml:"text"`
	}
	if xml.Unmarshal(data, &transcript) == nil {
		values := make([]string, 0, len(transcript.Texts))
		for _, item := range transcript.Texts {
			if value := strings.TrimSpace(item.Value); value != "" {
				values = append(values, value)
			}
		}
		if len(values) > 0 {
			return strings.Join(values, " "), nil
		}
	}
	return "", errors.New("YouTube transcript is empty")
}

func (goYouTubeTranscriptLoader) Load(ctx context.Context, videoID string, config map[string]any) (string, string, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if proxyValue, _ := config["YOUTUBE_LOADER_PROXY_URL"].(string); strings.TrimSpace(proxyValue) != "" {
		proxyURL, err := url.Parse(strings.TrimSpace(proxyValue))
		if err != nil || proxyURL.Host == "" || (proxyURL.Scheme != "http" && proxyURL.Scheme != "https") {
			return "", "", errors.New("YouTube proxy URL is invalid")
		}
		transport.Proxy = http.ProxyURL(proxyURL)
	}
	client := &http.Client{Timeout: 20 * time.Second, Transport: transport}
	watchURL := "https://www.youtube.com/watch?v=" + videoID + "&hl=en"
	watchRequest, _ := http.NewRequestWithContext(ctx, http.MethodGet, watchURL, nil)
	watchRequest.Header.Set("User-Agent", "Mozilla/5.0 (compatible; HaloWebUI-Go/1.0)")
	watchRequest.Header.Set("Accept-Language", "en-US,en;q=0.8")
	watchResponse, err := client.Do(watchRequest)
	if err != nil {
		return "", "", errors.New("failed to load YouTube video metadata")
	}
	defer watchResponse.Body.Close()
	if watchResponse.StatusCode >= 400 {
		return "", "", errors.New("YouTube returned an error")
	}
	page, _ := io.ReadAll(io.LimitReader(watchResponse.Body, 4*1024*1024))
	tracksJSON, found := extractJSONArray(page, `"captionTracks":`)
	if !found {
		return "", "", errors.New("YouTube video has no accessible captions")
	}
	var tracks []youtubeCaptionTrack
	if json.Unmarshal(tracksJSON, &tracks) != nil {
		return "", "", errors.New("YouTube caption metadata is invalid")
	}
	track, found := selectCaptionTrack(tracks, stringSlice(config["YOUTUBE_LOADER_LANGUAGE"]))
	if !found || track.BaseURL == "" {
		return "", "", errors.New("YouTube video has no accessible captions")
	}
	captionURL, err := url.Parse(track.BaseURL)
	if err != nil || !strings.HasSuffix(strings.ToLower(captionURL.Hostname()), "youtube.com") {
		return "", "", errors.New("YouTube caption URL is invalid")
	}
	values := captionURL.Query()
	values.Set("fmt", "json3")
	if translation, _ := config["YOUTUBE_LOADER_TRANSLATION"].(string); strings.TrimSpace(translation) != "" {
		values.Set("tlang", strings.TrimSpace(translation))
	}
	captionURL.RawQuery = values.Encode()
	captionRequest, _ := http.NewRequestWithContext(ctx, http.MethodGet, captionURL.String(), nil)
	captionRequest.Header.Set("User-Agent", "Mozilla/5.0 (compatible; HaloWebUI-Go/1.0)")
	captionResponse, err := client.Do(captionRequest)
	if err != nil {
		return "", "", errors.New("failed to load YouTube captions")
	}
	defer captionResponse.Body.Close()
	captionData, _ := io.ReadAll(io.LimitReader(captionResponse.Body, 4*1024*1024))
	if captionResponse.StatusCode >= 400 {
		return "", "", errors.New("YouTube captions returned an error")
	}
	transcript, err := parseYouTubeTranscript(captionData)
	if err != nil {
		return "", "", err
	}
	return transcript, track.LanguageCode, nil
}

func (a *App) handleRetrievalProcessYouTube(w http.ResponseWriter, r *http.Request) {
	user, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	var form struct {
		URL            string `json:"url"`
		CollectionName string `json:"collection_name"`
	}
	if !decodeJSON(w, r, &form) {
		return
	}
	videoID, err := youtubeVideoID(form.URL)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	config, err := a.loadGlobalJSON(r, retrievalConfigKey, defaultRetrievalConfig())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load YouTube config")
		return
	}
	web := retrievalWebConfig(config)
	transcript, language, err := a.youtubeLoader.Load(r.Context(), videoID, web)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	collection := strings.TrimSpace(form.CollectionName)
	if collection == "" {
		collection = "youtube-" + videoID
	}
	metadata, _ := json.Marshal(map[string]any{"url": form.URL, "video_id": videoID, "language": language})
	_, err = a.store.UpsertRetrievalDocument(r.Context(), store.RetrievalDocument{
		ID: auth.RandomIDForInternalUse(), Collection: collection, UserID: user.ID,
		Filename: form.URL, Text: transcript, MetadataJSON: string(metadata),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to index YouTube captions")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status": true, "collection_name": collection, "filename": form.URL,
		"file": map[string]any{"data": map[string]string{"content": transcript}, "meta": map[string]string{"name": form.URL, "source": form.URL}},
	})
}
