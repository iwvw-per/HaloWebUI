package server

import (
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/iwvw-per/HaloWebUI/backend/internal/auth"
	"github.com/iwvw-per/HaloWebUI/backend/internal/store"
)

const retrievalConfigKey = "retrieval/config"
const retrievalQueryConfigKey = "retrieval/query"

func defaultRetrievalConfig() map[string]any {
	return map[string]any{
		"FILE_PROCESSING_DEFAULT_MODE": "full",
		"CONTENT_EXTRACTION_ENGINE":    "builtin",
		"DOCUMENT_PROVIDER":            "builtin",
		"TEXT_SPLITTER":                "character",
		"CHUNK_SIZE":                   1000,
		"CHUNK_OVERLAP":                100,
		"TOP_K":                        4,
		"TOP_K_RERANKER":               0,
		"RAG_FULL_CONTEXT":             false,
		"ENABLE_RAG_HYBRID_SEARCH":     false,
		"RAG_SYSTEM_CONTEXT":           "",
		"RAG_TEMPLATE":                 "",
		"web_loader_ssl_verification":  true,
	}
}

func (a *App) loadGlobalJSON(r *http.Request, key string, fallback map[string]any) (map[string]any, error) {
	resource, err := a.store.ResourceByKey(r.Context(), "global_setting", key)
	if errors.Is(err, store.ErrResourceNotFound) {
		return fallback, nil
	}
	if err != nil {
		return nil, err
	}
	var value map[string]any
	if err := json.Unmarshal(resource.Body, &value); err != nil {
		return nil, err
	}
	for k, v := range fallback {
		if _, ok := value[k]; !ok {
			value[k] = v
		}
	}
	return value, nil
}

func (a *App) saveGlobalJSON(r *http.Request, key string, value map[string]any) error {
	resource, err := a.store.ResourceByKey(r.Context(), "global_setting", key)
	if errors.Is(err, store.ErrResourceNotFound) {
		resource = store.Resource{Kind: "global_setting", ID: auth.RandomIDForInternalUse(), UserID: "system", Key: key, Active: true}
	} else if err != nil {
		return err
	}
	resource.Body, _ = json.Marshal(value)
	_, err = a.store.PutResource(r.Context(), resource)
	return err
}

func (a *App) handleRetrievalConfig(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.requireUser(w, r); !ok {
		return
	}
	config, err := a.loadGlobalJSON(r, retrievalConfigKey, defaultRetrievalConfig())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load retrieval config")
		return
	}
	writeJSON(w, http.StatusOK, config)
}

func (a *App) handleRetrievalConfigUpdate(w http.ResponseWriter, r *http.Request) {
	if ok, _ := a.requireAdmin(w, r); !ok {
		return
	}
	config, err := a.loadGlobalJSON(r, retrievalConfigKey, defaultRetrievalConfig())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load retrieval config")
		return
	}
	var patch map[string]any
	if !decodeJSON(w, r, &patch) {
		return
	}
	for key, value := range patch {
		config[key] = value
	}
	if err := a.saveGlobalJSON(r, retrievalConfigKey, config); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save retrieval config")
		return
	}
	writeJSON(w, http.StatusOK, config)
}

func (a *App) handleRetrievalStatus(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.requireUser(w, r); !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status": true, "embedding": map[string]any{"engine": "lexical", "available": true},
		"reranking": map[string]any{"engine": "lexical", "available": true},
	})
}

func (a *App) handleRetrievalEmbedding(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.requireUser(w, r); !ok {
		return
	}
	config, err := a.loadGlobalJSON(r, retrievalConfigKey, defaultRetrievalConfig())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load embedding config")
		return
	}
	if r.Method == http.MethodPost {
		if ok, _ := a.requireAdmin(w, r); !ok {
			return
		}
		var patch map[string]any
		if !decodeJSON(w, r, &patch) {
			return
		}
		for key, value := range patch {
			config[key] = value
		}
		if err := a.saveGlobalJSON(r, retrievalConfigKey, config); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to save embedding config")
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"embedding_engine": "lexical", "embedding_model": "", "config": config})
}

func (a *App) handleRetrievalReranking(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.requireUser(w, r); !ok {
		return
	}
	config, err := a.loadGlobalJSON(r, retrievalConfigKey, defaultRetrievalConfig())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load reranking config")
		return
	}
	if r.Method == http.MethodPost {
		if ok, _ := a.requireAdmin(w, r); !ok {
			return
		}
		var patch map[string]any
		if !decodeJSON(w, r, &patch) {
			return
		}
		for key, value := range patch {
			config[key] = value
		}
		if err := a.saveGlobalJSON(r, retrievalConfigKey, config); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to save reranking config")
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"reranking_engine": "lexical", "reranking_model": "", "config": config})
}

func (a *App) handleRetrievalQuerySettings(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.requireUser(w, r); !ok {
		return
	}
	fallback := map[string]any{"k": 4, "r": 0, "template": nil}
	settings, err := a.loadGlobalJSON(r, retrievalQueryConfigKey, fallback)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load query settings")
		return
	}
	if r.Method == http.MethodPost {
		if ok, _ := a.requireAdmin(w, r); !ok {
			return
		}
		var patch map[string]any
		if !decodeJSON(w, r, &patch) {
			return
		}
		for key, value := range patch {
			if key == "k" || key == "r" || key == "template" {
				settings[key] = value
			}
		}
		if err := a.saveGlobalJSON(r, retrievalQueryConfigKey, settings); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to save query settings")
			return
		}
	}
	writeJSON(w, http.StatusOK, settings)
}

func (a *App) indexStoredFile(r *http.Request, user store.User, file store.File, collection string) error {
	if file.Path == nil {
		return errors.New("file has no stored path")
	}
	path, safe := safeDataPath(a.config.DataDir, *file.Path)
	if !safe {
		return errors.New("file path is outside data directory")
	}
	source, err := os.Open(path)
	if err != nil {
		return err
	}
	defer source.Close()
	data, err := io.ReadAll(io.LimitReader(source, 4*1024*1024))
	if err != nil {
		return err
	}
	text := strings.TrimSpace(strings.Map(func(r rune) rune {
		if r == 0 {
			return -1
		}
		return r
	}, string(data)))
	if text == "" {
		return errors.New("file contains no text")
	}
	metadata, _ := json.Marshal(map[string]any{"filename": file.Filename, "file_id": file.ID})
	_, err = a.store.UpsertRetrievalDocument(r.Context(), store.RetrievalDocument{
		ID: file.ID, Collection: collection, UserID: user.ID, Filename: file.Filename, Text: text, MetadataJSON: string(metadata),
	})
	return err
}

func (a *App) retrievalCollectionForRequest(w http.ResponseWriter, r *http.Request, collection string) (store.User, bool) {
	user, ok := a.requireUser(w, r)
	if !ok {
		return store.User{}, false
	}
	if collection == "" {
		writeError(w, http.StatusBadRequest, "collection_name is required")
		return store.User{}, false
	}
	knowledge, err := a.store.KnowledgeByID(r.Context(), collection)
	if err == nil && !a.canReadKnowledge(user, knowledge) {
		writeError(w, http.StatusForbidden, "Access prohibited")
		return store.User{}, false
	}
	if err == nil && user.Role != "admin" && knowledge.UserID != user.ID {
		// Public knowledge is readable but its indexed documents are owned by the
		// knowledge owner; expose only through the collection's owner route.
		user.ID = knowledge.UserID
	}
	return user, true
}

func (a *App) handleRetrievalProcessFile(w http.ResponseWriter, r *http.Request) {
	user, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	var form struct {
		FileID         string `json:"file_id"`
		CollectionName string `json:"collection_name"`
	}
	if !decodeJSON(w, r, &form) {
		return
	}
	file, err := a.store.FileByID(r.Context(), form.FileID)
	if err != nil || (file.UserID != user.ID && user.Role != "admin") {
		writeError(w, http.StatusNotFound, "File not found")
		return
	}
	collection := form.CollectionName
	if collection == "" {
		collection = form.FileID
	}
	if err := a.indexStoredFile(r, user, file, collection); err != nil {
		writeError(w, http.StatusBadRequest, "file could not be indexed: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": true, "collection_name": collection, "filenames": []string{file.Filename}})
}

func (a *App) handleRetrievalProcessText(w http.ResponseWriter, r *http.Request) {
	user, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	var form struct {
		Text           string `json:"text"`
		CollectionName string `json:"collection_name"`
		Filename       string `json:"filename"`
	}
	if !decodeJSON(w, r, &form) {
		return
	}
	if strings.TrimSpace(form.Text) == "" || strings.TrimSpace(form.CollectionName) == "" {
		writeError(w, http.StatusBadRequest, "text and collection_name are required")
		return
	}
	filename := form.Filename
	if filename == "" {
		filename = "text"
	}
	metadata, _ := json.Marshal(map[string]any{"filename": filename})
	_, err := a.store.UpsertRetrievalDocument(r.Context(), store.RetrievalDocument{ID: auth.RandomIDForInternalUse(), Collection: form.CollectionName, UserID: user.ID, Filename: filename, Text: form.Text, MetadataJSON: string(metadata)})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to index text")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": true, "collection_name": form.CollectionName, "filenames": []string{filename}})
}

func (a *App) handleRetrievalProcessWeb(w http.ResponseWriter, r *http.Request) {
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
	parsed, err := url.Parse(strings.TrimSpace(form.URL))
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || blockedRemoteHost(parsed.Hostname()) {
		writeError(w, http.StatusBadRequest, "unsupported or unsafe URL")
		return
	}
	request, _ := http.NewRequestWithContext(r.Context(), http.MethodGet, parsed.String(), nil)
	client := &http.Client{Timeout: 15 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		writeError(w, http.StatusBadGateway, "web loader request failed")
		return
	}
	defer response.Body.Close()
	if response.StatusCode >= 400 {
		writeError(w, http.StatusBadGateway, "web loader returned an error")
		return
	}
	body, _ := io.ReadAll(io.LimitReader(response.Body, 2*1024*1024))
	text := strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(string(body), "<", " <"), ">", "> "))
	if text == "" {
		writeError(w, http.StatusBadRequest, "web page contains no text")
		return
	}
	collection := form.CollectionName
	if collection == "" {
		collection = "web-" + auth.RandomIDForInternalUse()
	}
	metadata, _ := json.Marshal(map[string]any{"url": parsed.String(), "filename": parsed.Hostname()})
	_, err = a.store.UpsertRetrievalDocument(r.Context(), store.RetrievalDocument{ID: auth.RandomIDForInternalUse(), Collection: collection, UserID: user.ID, Filename: parsed.Hostname(), Text: text, MetadataJSON: string(metadata)})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to index web page")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": true, "collection_name": collection, "filenames": []string{parsed.Hostname()}})
}

func blockedRemoteHost(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "localhost" || strings.HasSuffix(host, ".localhost") || host == "metadata.google.internal" {
		return true
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsUnspecified()
}

func (a *App) handleRetrievalProcessUnsupported(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.requireUser(w, r); !ok {
		return
	}
	writeError(w, http.StatusNotImplemented, "remote transcript/document adapter is not configured")
}

func (a *App) retrievalQuery(w http.ResponseWriter, r *http.Request, collections []string, query string, k int) {
	user, ok := a.retrievalCollectionForRequest(w, r, collections[0])
	if !ok {
		return
	}
	docs, distances, err := a.store.RetrievalDocuments(r.Context(), collections, user.ID, query, k)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "retrieval query failed")
		return
	}
	ids := make([]string, 0, len(docs))
	documents := make([]string, 0, len(docs))
	metadatas := make([]any, 0, len(docs))
	for _, doc := range docs {
		ids = append(ids, doc.ID)
		documents = append(documents, doc.Text)
		metadatas = append(metadatas, rawObject(json.RawMessage(doc.MetadataJSON)))
	}
	writeJSON(w, http.StatusOK, map[string]any{"ids": [][]string{ids}, "documents": [][]string{documents}, "metadatas": [][]any{metadatas}, "distances": [][]float64{distances}})
}

func (a *App) handleRetrievalQuery(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.requireUser(w, r); !ok {
		return
	}
	var form struct {
		CollectionName  string          `json:"collection_name"`
		CollectionNames json.RawMessage `json:"collection_names"`
		Query           string          `json:"query"`
		K               int             `json:"k"`
	}
	if !decodeJSON(w, r, &form) {
		return
	}
	var collections []string
	if len(form.CollectionNames) > 0 {
		if json.Unmarshal(form.CollectionNames, &collections) != nil {
			var single string
			if json.Unmarshal(form.CollectionNames, &single) == nil && single != "" {
				collections = []string{single}
			}
		}
	}
	if len(collections) == 0 && form.CollectionName != "" {
		collections = []string{form.CollectionName}
	}
	if len(collections) == 0 {
		writeError(w, http.StatusBadRequest, "collection_name is required")
		return
	}
	a.retrievalQuery(w, r, collections, form.Query, form.K)
}

func (a *App) handleRetrievalDelete(w http.ResponseWriter, r *http.Request) {
	user, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	var form struct {
		CollectionName string `json:"collection_name"`
	}
	if !decodeJSON(w, r, &form) {
		return
	}
	if err := a.store.DeleteRetrievalCollection(r.Context(), form.CollectionName, user.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete collection")
		return
	}
	writeJSON(w, http.StatusOK, true)
}

func (a *App) handleRetrievalReset(w http.ResponseWriter, r *http.Request) {
	user, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	if strings.HasSuffix(r.URL.Path, "/db") {
		if err := a.store.DeleteAllRetrieval(r.Context(), user.ID); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to reset retrieval database")
			return
		}
	}
	writeJSON(w, http.StatusOK, true)
}

func (a *App) handleRetrievalVerify(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.requireUser(w, r); !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"search": map[string]any{"enabled": false, "ok": nil, "message": "remote web search is not configured"},
		"loader": map[string]any{"enabled": false, "ok": nil, "message": "built-in bounded HTTP loader is available"},
	})
}

func (a *App) handleRetrievalBatch(w http.ResponseWriter, r *http.Request) {
	user, ok := a.requireUser(w, r)
	if !ok {
		return
	}
	var form struct {
		FileIDs        []string `json:"file_ids"`
		CollectionName string   `json:"collection_name"`
	}
	if !decodeJSON(w, r, &form) {
		return
	}
	filenames := make([]string, 0, len(form.FileIDs))
	for _, id := range form.FileIDs {
		file, err := a.store.FileByID(r.Context(), id)
		if err != nil || (file.UserID != user.ID && user.Role != "admin") {
			continue
		}
		if a.indexStoredFile(r, user, file, form.CollectionName) == nil {
			filenames = append(filenames, file.Filename)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": true, "collection_name": form.CollectionName, "filenames": filenames})
}

func (a *App) handleRetrievalEmbeddingFunction(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.requireUser(w, r); !ok {
		return
	}
	writeJSON(w, http.StatusNotImplemented, map[string]any{"detail": "embedding worker is not configured; use lexical retrieval or configure a remote adapter"})
}
