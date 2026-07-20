package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/iwvw-per/HaloWebUI/backend/internal/auth"
	"github.com/iwvw-per/HaloWebUI/backend/internal/store"
)

const audioConfigKey = "audio/config"

type audioConfig struct {
	TTS audioTTSConfig `json:"tts"`
	STT audioSTTConfig `json:"stt"`
}

type audioTTSConfig struct {
	OpenAIBaseURL           string `json:"OPENAI_API_BASE_URL"`
	OpenAIAPIKey            string `json:"OPENAI_API_KEY"`
	APIKey                  string `json:"API_KEY"`
	Engine                  string `json:"ENGINE"`
	Model                   string `json:"MODEL"`
	Voice                   string `json:"VOICE"`
	SplitOn                 string `json:"SPLIT_ON"`
	AzureSpeechRegion       string `json:"AZURE_SPEECH_REGION"`
	AzureSpeechOutputFormat string `json:"AZURE_SPEECH_OUTPUT_FORMAT"`
}

type audioSTTConfig struct {
	OpenAIBaseURL  string `json:"OPENAI_API_BASE_URL"`
	OpenAIAPIKey   string `json:"OPENAI_API_KEY"`
	Engine         string `json:"ENGINE"`
	Model          string `json:"MODEL"`
	WhisperModel   string `json:"WHISPER_MODEL"`
	DeepgramAPIKey string `json:"DEEPGRAM_API_KEY"`
	AzureAPIKey    string `json:"AZURE_API_KEY"`
	AzureRegion    string `json:"AZURE_REGION"`
	AzureLocales   string `json:"AZURE_LOCALES"`
}

func (a *App) defaultAudioConfig() audioConfig {
	return audioConfig{
		TTS: audioTTSConfig{
			OpenAIBaseURL: a.config.OpenAIBaseURL,
			OpenAIAPIKey:  a.config.OpenAIAPIKey,
			SplitOn:       "punctuation",
		},
		STT: audioSTTConfig{
			OpenAIBaseURL: a.config.OpenAIBaseURL,
			OpenAIAPIKey:  a.config.OpenAIAPIKey,
			Engine:        "openai",
			Model:         "whisper-1",
		},
	}
}

func (a *App) loadAudioConfig(r *http.Request) (audioConfig, error) {
	config := a.defaultAudioConfig()
	resource, err := a.store.ResourceByKey(r.Context(), haloClawConfigKind, audioConfigKey)
	if errors.Is(err, store.ErrResourceNotFound) {
		return config, nil
	}
	if err != nil {
		return audioConfig{}, err
	}
	if err := json.Unmarshal(resource.Body, &config); err != nil {
		return audioConfig{}, err
	}
	// Local Python, Deepgram, and Azure engines are not part of the Go slim
	// image. Normalize old persisted values to the supported remote adapter.
	if config.STT.Engine != "openai" {
		config.STT.Engine = "openai"
	}
	if config.TTS.Engine != "" && config.TTS.Engine != "openai" {
		config.TTS.Engine = ""
	}
	return config, nil
}

func (a *App) handleAudioConfig(w http.ResponseWriter, r *http.Request) {
	if ok, _ := a.requireAdmin(w, r); !ok {
		return
	}
	if r.Method == http.MethodGet {
		config, err := a.loadAudioConfig(r)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to load audio config")
			return
		}
		writeJSON(w, http.StatusOK, config)
		return
	}

	var payload map[string]any
	if !decodeJSON(w, r, &payload) {
		return
	}
	config, err := a.loadAudioConfig(r)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load audio config")
		return
	}
	if _, nested := payload["tts"]; !nested {
		if value, ok := payload["url"].(string); ok {
			config.TTS.OpenAIBaseURL = value
		}
		if value, ok := payload["key"].(string); ok {
			config.TTS.OpenAIAPIKey = value
		}
		if value, ok := payload["model"].(string); ok {
			config.TTS.Model = value
		}
		if value, ok := payload["speaker"].(string); ok {
			config.TTS.Voice = value
		}
		config.TTS.Engine = "openai"
	} else {
		encoded, _ := json.Marshal(payload)
		if err := json.Unmarshal(encoded, &config); err != nil {
			writeError(w, http.StatusBadRequest, "invalid audio config")
			return
		}
	}
	if config.STT.Engine != "openai" {
		writeError(w, http.StatusBadRequest, "Go slim supports only the remote OpenAI-compatible STT engine")
		return
	}
	if config.TTS.Engine != "" && config.TTS.Engine != "openai" {
		writeError(w, http.StatusBadRequest, "Go slim supports browser TTS or the remote OpenAI-compatible TTS engine")
		return
	}
	if config.TTS.SplitOn == "" {
		config.TTS.SplitOn = "punctuation"
	}
	encoded, err := json.Marshal(config)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid audio config")
		return
	}
	resource, err := a.store.ResourceByKey(r.Context(), haloClawConfigKind, audioConfigKey)
	if errors.Is(err, store.ErrResourceNotFound) {
		resource = store.Resource{
			Kind: haloClawConfigKind, ID: auth.RandomIDForInternalUse(), UserID: "system",
			Key: audioConfigKey, Active: true,
		}
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load audio config")
		return
	}
	resource.Body = encoded
	if _, err := a.store.PutResource(r.Context(), resource); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save audio config")
		return
	}
	writeJSON(w, http.StatusOK, config)
}

func (a *App) handleAudioModels(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.requireUser(w, r); !ok {
		return
	}
	config, err := a.loadAudioConfig(r)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load audio config")
		return
	}
	models := make([]map[string]string, 0)
	if config.TTS.Engine == "openai" {
		models = append(models,
			map[string]string{"id": "tts-1", "name": "tts-1"},
			map[string]string{"id": "tts-1-hd", "name": "tts-1-hd"},
		)
	}
	writeJSON(w, http.StatusOK, map[string]any{"models": models})
}

func (a *App) handleAudioVoices(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.requireUser(w, r); !ok {
		return
	}
	config, err := a.loadAudioConfig(r)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load audio config")
		return
	}
	voices := make([]map[string]string, 0)
	if config.TTS.Engine == "openai" {
		for _, voice := range []string{"alloy", "echo", "fable", "onyx", "nova", "shimmer"} {
			voices = append(voices, map[string]string{"id": voice, "name": voice})
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"voices": voices})
}

func audioTarget(baseURL, suffix string) (string, error) {
	target, err := url.Parse(strings.TrimRight(strings.TrimSpace(baseURL), "/") + suffix)
	if err != nil || target.Host == "" || (target.Scheme != "http" && target.Scheme != "https") {
		return "", errors.New("audio provider URL is invalid")
	}
	return target.String(), nil
}

func (a *App) handleAudioSpeech(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.requireUser(w, r); !ok {
		return
	}
	config, err := a.loadAudioConfig(r)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load audio config")
		return
	}
	if strings.ToLower(config.TTS.Engine) != "openai" {
		writeError(w, http.StatusServiceUnavailable, "remote OpenAI-compatible TTS is not configured")
		return
	}
	var payload map[string]any
	if !decodeJSON(w, r, &payload) {
		return
	}
	if _, ok := payload["model"]; !ok && config.TTS.Model != "" {
		payload["model"] = config.TTS.Model
	}
	if _, ok := payload["voice"]; !ok && config.TTS.Voice != "" {
		payload["voice"] = config.TTS.Voice
	}
	target, err := audioTarget(config.TTS.OpenAIBaseURL, "/audio/speech")
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	body, _ := json.Marshal(payload)
	request, _ := http.NewRequestWithContext(r.Context(), http.MethodPost, target, bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "audio/mpeg, audio/*")
	if config.TTS.OpenAIAPIKey != "" {
		request.Header.Set("Authorization", "Bearer "+config.TTS.OpenAIAPIKey)
	}
	upstream, err := (&http.Client{Timeout: 5 * time.Minute}).Do(request)
	if err != nil {
		writeError(w, http.StatusBadGateway, "audio provider request failed")
		return
	}
	defer upstream.Body.Close()
	if upstream.StatusCode >= 400 {
		writeError(w, upstream.StatusCode, "audio provider returned an error")
		return
	}
	if contentType := upstream.Header.Get("Content-Type"); contentType != "" {
		w.Header().Set("Content-Type", contentType)
	} else {
		w.Header().Set("Content-Type", "audio/mpeg")
	}
	_, _ = io.Copy(w, io.LimitReader(upstream.Body, 25*1024*1024))
}

func (a *App) handleAudioTranscription(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.requireUser(w, r); !ok {
		return
	}
	config, err := a.loadAudioConfig(r)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load audio config")
		return
	}
	if strings.ToLower(config.STT.Engine) != "openai" {
		writeError(w, http.StatusServiceUnavailable, "remote OpenAI-compatible STT is not configured")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 25*1024*1024)
	if err := r.ParseMultipartForm(25 * 1024 * 1024); err != nil {
		writeError(w, http.StatusBadRequest, "invalid audio upload")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "file is required")
		return
	}
	defer file.Close()
	var formBody bytes.Buffer
	writer := multipart.NewWriter(&formBody)
	part, err := writer.CreateFormFile("file", header.Filename)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to prepare audio upload")
		return
	}
	if _, err := io.CopyN(part, file, 25*1024*1024); err != nil && err != io.EOF {
		writeError(w, http.StatusBadRequest, "audio upload is too large")
		return
	}
	if language := r.FormValue("language"); language != "" {
		_ = writer.WriteField("language", language)
	}
	model := config.STT.Model
	if model == "" {
		model = "whisper-1"
	}
	_ = writer.WriteField("model", model)
	_ = writer.Close()
	target, err := audioTarget(config.STT.OpenAIBaseURL, "/audio/transcriptions")
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	request, _ := http.NewRequestWithContext(r.Context(), http.MethodPost, target, &formBody)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request.Header.Set("Accept", "application/json")
	if config.STT.OpenAIAPIKey != "" {
		request.Header.Set("Authorization", "Bearer "+config.STT.OpenAIAPIKey)
	}
	upstream, err := (&http.Client{Timeout: 5 * time.Minute}).Do(request)
	if err != nil {
		writeError(w, http.StatusBadGateway, "transcription provider request failed")
		return
	}
	defer upstream.Body.Close()
	if upstream.StatusCode >= 400 {
		writeError(w, upstream.StatusCode, "transcription provider returned an error")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = io.Copy(w, io.LimitReader(upstream.Body, 2*1024*1024))
}
