package server

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/iwvw-per/HaloWebUI/backend/internal/auth"
	"github.com/iwvw-per/HaloWebUI/backend/internal/store"
)

type App struct {
	config          Config
	store           *store.Store
	frontend        fs.FS
	index           []byte
	mux             *http.ServeMux
	restoreMu       sync.Mutex
	restoreSessions map[string]restoreSession
	tasksMu         sync.Mutex
	tasks           map[string]*taskEntry
	gatewayRateMu   sync.Mutex
	gatewayRates    map[string]gatewayRateWindow
	ldapAuth        ldapAuthenticator
	youtubeLoader   youtubeTranscriptLoader
}

func New(config Config) (*App, error) {
	dataDir := config.DataDir
	if dataDir == "" {
		dataDir = filepath.Join(os.TempDir(), "halowebui-go")
	}
	dataStore, err := store.Open(context.Background(), dataDir)
	if err != nil {
		return nil, err
	}
	if config.EnableTerminal {
		if err := os.MkdirAll(filepath.Join(dataDir, "workspace"), 0o700); err != nil {
			_ = dataStore.Close()
			return nil, errors.New("failed to initialize terminal workspace")
		}
	}
	frontend := os.DirFS(config.FrontendDir)
	index, err := fs.ReadFile(frontend, "index.html")
	if err != nil {
		_ = dataStore.Close()
		return nil, errors.New("frontend index.html is unavailable in FRONTEND_DIR")
	}
	app := &App{
		config:          config,
		store:           dataStore,
		frontend:        frontend,
		index:           index,
		mux:             http.NewServeMux(),
		restoreSessions: make(map[string]restoreSession),
		tasks:           make(map[string]*taskEntry),
		gatewayRates:    make(map[string]gatewayRateWindow),
		ldapAuth:        goLDAPAuthenticator{},
		youtubeLoader:   goYouTubeTranscriptLoader{},
	}
	app.registerRoutes()
	return app, nil
}

func (a *App) Close() error {
	return a.store.Close()
}

func (a *App) registerRoutes() {
	a.mux.HandleFunc("GET /health", a.handleHealth)
	a.mux.HandleFunc("GET /api/version", a.handleVersion)
	a.mux.HandleFunc("GET /api/version/updates", a.handleRootVersionUpdates)
	a.mux.HandleFunc("GET /api/changelog", a.handleRootChangelog)
	a.mux.HandleFunc("GET /api/config", a.handleConfig)
	a.mux.HandleFunc("GET /api/webhook", a.handleRootWebhook)
	a.mux.HandleFunc("POST /api/webhook", a.handleRootWebhook)
	a.mux.HandleFunc("GET /api/config/model/filter", a.handleRootModelFilter)
	a.mux.HandleFunc("POST /api/config/model/filter", a.handleRootModelFilter)
	a.mux.HandleFunc("GET /api/config/models", a.handleRootModelConfig)
	a.mux.HandleFunc("POST /api/config/models", a.handleRootModelConfig)
	a.mux.HandleFunc("GET /api/community_sharing", a.handleRootCommunitySharing)
	a.mux.HandleFunc("GET /api/community_sharing/toggle", a.handleRootCommunitySharing)
	a.mux.HandleFunc("GET /api/v1/auths/", a.handleSession)
	a.mux.HandleFunc("POST /api/v1/auths/signin", a.handleSignin)
	a.mux.HandleFunc("POST /api/v1/auths/ldap", a.handleLDAPSignin)
	a.mux.HandleFunc("POST /api/v1/auths/signup", a.handleSignup)
	a.mux.HandleFunc("GET /api/v1/auths/signout", a.handleSignout)
	a.mux.HandleFunc("GET /api/v1/auths/admin/details", a.handleAdminDetails)
	a.mux.HandleFunc("GET /api/v1/auths/admin/config", a.handleAdminConfig)
	a.mux.HandleFunc("POST /api/v1/auths/admin/config", a.handleAdminConfig)
	a.mux.HandleFunc("GET /api/v1/auths/admin/config/ldap", a.handleLDAPConfig)
	a.mux.HandleFunc("POST /api/v1/auths/admin/config/ldap", a.handleLDAPConfig)
	a.mux.HandleFunc("GET /api/v1/auths/admin/config/ldap/server", a.handleLDAPServerConfig)
	a.mux.HandleFunc("POST /api/v1/auths/admin/config/ldap/server", a.handleLDAPServerConfig)
	a.mux.HandleFunc("POST /api/v1/auths/add", a.handleAddUser)
	a.mux.HandleFunc("POST /api/v1/auths/update/profile", a.handleProfileUpdate)
	a.mux.HandleFunc("POST /api/v1/auths/update/password", a.handlePasswordUpdate)
	a.mux.HandleFunc("GET /api/v1/auths/signup/enabled", a.handleSignupConfig)
	a.mux.HandleFunc("GET /api/v1/auths/signup/enabled/toggle", a.handleSignupConfig)
	a.mux.HandleFunc("GET /api/v1/auths/signup/user/role", a.handleSignupConfig)
	a.mux.HandleFunc("POST /api/v1/auths/signup/user/role", a.handleSignupConfig)
	a.mux.HandleFunc("GET /api/v1/auths/token/expires", a.handleTokenExpiry)
	a.mux.HandleFunc("POST /api/v1/auths/token/expires/update", a.handleTokenExpiry)
	a.mux.HandleFunc("GET /api/v1/auths/api_key", a.handleAPIKey)
	a.mux.HandleFunc("POST /api/v1/auths/api_key", a.handleAPIKey)
	a.mux.HandleFunc("DELETE /api/v1/auths/api_key", a.handleAPIKey)
	a.mux.HandleFunc("POST /api/v1/chats/new", a.handleChatNew)
	a.mux.HandleFunc("POST /api/v1/chats/import", a.handleChatImport)
	a.mux.HandleFunc("POST /api/v1/chats/import/batch", a.handleChatImportBatch)
	a.mux.HandleFunc("DELETE /api/v1/chats/", a.handleDeleteAllChats)
	a.mux.HandleFunc("GET /api/v1/chats/list/user/{user_id}", a.handleChatListByUser)
	a.mux.HandleFunc("GET /api/v1/chats/search", a.handleChatSearch)
	a.mux.HandleFunc("GET /api/v1/chats/folder/{folder_id}/list", a.handleChatFolderList)
	a.mux.HandleFunc("GET /api/v1/chats/assistant/{assistant_id}/list", a.handleChatAssistantList)
	a.mux.HandleFunc("GET /api/v1/chats/pinned", a.handlePinnedChats)
	a.mux.HandleFunc("GET /api/v1/chats/all/tags", a.handleAllChatTags)
	a.mux.HandleFunc("GET /api/v1/chats/all/db", a.handleAllChatsDB)
	a.mux.HandleFunc("GET /api/v1/chats/shared", a.handleSharedChats)
	a.mux.HandleFunc("POST /api/v1/chats/tags", a.handleChatsByTag)
	a.mux.HandleFunc("POST /api/v1/chats/archive/all", a.handleArchiveAllChats)
	a.mux.HandleFunc("GET /api/v1/chats/archived", a.handleChatList)
	a.mux.HandleFunc("GET /api/v1/chats/all/archived", a.handleChatList)
	a.mux.HandleFunc("GET /api/v1/chats/all", a.handleChatList)
	a.mux.HandleFunc("GET /api/v1/chats/", a.handleChatList)
	a.mux.HandleFunc("GET /api/v1/chats/{id}/context", a.handleChatContext)
	a.mux.HandleFunc("GET /api/v1/chats/{id}/pinned", a.handleChatPinnedStatus)
	a.mux.HandleFunc("POST /api/v1/chats/{id}", a.handleChatByID)
	a.mux.HandleFunc("POST /api/v1/chats/{id}/composer-state", a.handleChatComposerState)
	a.mux.HandleFunc("POST /api/v1/chats/{id}/messages/{message_id}", a.handleChatMessageUpdate)
	a.mux.HandleFunc("POST /api/v1/chats/{id}/messages/{message_id}/event", a.handleChatMessageEvent)
	a.mux.HandleFunc("POST /api/v1/chats/{id}/archive", a.handleChatField)
	a.mux.HandleFunc("POST /api/v1/chats/{id}/pin", a.handleChatField)
	a.mux.HandleFunc("POST /api/v1/chats/{id}/clone", a.handleChatClone)
	a.mux.HandleFunc("POST /api/v1/chats/{id}/branch", a.handleChatBranch)
	a.mux.HandleFunc("POST /api/v1/chats/{id}/clone/shared", a.handleChatCloneShared)
	a.mux.HandleFunc("POST /api/v1/chats/{id}/share", a.handleChatField)
	a.mux.HandleFunc("DELETE /api/v1/chats/{id}/share", a.handleChatShareDelete)
	a.mux.HandleFunc("POST /api/v1/chats/{id}/folder", a.handleChatField)
	a.mux.HandleFunc("POST /api/v1/chats/{id}/title", a.handleChatField)
	a.mux.HandleFunc("GET /api/v1/chats/{id}/tags", a.handleChatTags)
	a.mux.HandleFunc("POST /api/v1/chats/{id}/tags", a.handleChatTags)
	a.mux.HandleFunc("DELETE /api/v1/chats/{id}/tags", a.handleChatTags)
	a.mux.HandleFunc("DELETE /api/v1/chats/{id}/tags/all", a.handleChatTags)
	a.mux.HandleFunc("GET /api/v1/chats/{id}", a.handleChatByID)
	a.mux.HandleFunc("PUT /api/v1/chats/{id}", a.handleChatByID)
	a.mux.HandleFunc("DELETE /api/v1/chats/{id}", a.handleChatByID)
	a.mux.HandleFunc("POST /openai/chat/completions", a.handleOpenAIChat)
	a.mux.HandleFunc("GET /openai/models", a.handleOpenAIModels)
	a.mux.HandleFunc("GET /openai/models/{index}", a.handleOpenAIModels)
	a.mux.HandleFunc("/openai/", a.handleOpenAICompatibility)
	a.mux.HandleFunc("POST /api/chat/completions", a.handleUnifiedChat)
	a.mux.HandleFunc("POST /api/chat/completed", a.handleChatCompleted)
	a.mux.HandleFunc("POST /api/chat/actions/{action_id}", a.handleTaskControl)
	a.mux.HandleFunc("POST /api/tasks/stop/{id}", a.handleTaskControl)
	a.mux.HandleFunc("GET /api/tasks/chat/{chat_id}", a.handleTaskControl)
	a.mux.HandleFunc("GET /api/tasks", a.handleTaskControl)
	a.mux.HandleFunc("GET /api/models", a.handleModels)
	a.mux.HandleFunc("GET /api/models/base", a.handleModels)
	a.mux.HandleFunc("GET /api/v1/models/", a.handleWorkspaceModels)
	a.mux.HandleFunc("GET /api/v1/models/base", a.handleWorkspaceModels)
	a.mux.HandleFunc("POST /api/v1/models/create", a.handleWorkspaceModelCreate)
	a.mux.HandleFunc("GET /api/v1/models/model", a.handleWorkspaceModel)
	a.mux.HandleFunc("POST /api/v1/models/model/update", a.handleWorkspaceModel)
	a.mux.HandleFunc("DELETE /api/v1/models/model/delete", a.handleWorkspaceModel)
	a.mux.HandleFunc("POST /api/v1/models/model/toggle", a.handleWorkspaceModelToggle)
	a.mux.HandleFunc("DELETE /api/v1/models/delete/all", a.handleWorkspaceModelsDeleteAll)
	a.mux.HandleFunc("POST /api/v1/models/bulk/update", a.handleWorkspaceModelsBulkUpdate)
	a.mux.HandleFunc("GET /api/v1/memories/", a.handleMemories)
	a.mux.HandleFunc("POST /api/v1/memories/add", a.handleMemoryAdd)
	a.mux.HandleFunc("POST /api/v1/memories/query", a.handleMemoryQuery)
	a.mux.HandleFunc("POST /api/v1/memories/reset", a.handleMemoryReset)
	a.mux.HandleFunc("DELETE /api/v1/memories/delete/user", a.handleMemoryDeleteAll)
	a.mux.HandleFunc("POST /api/v1/memories/{memory_id}/update", a.handleMemoryByID)
	a.mux.HandleFunc("DELETE /api/v1/memories/{memory_id}", a.handleMemoryByID)
	a.mux.HandleFunc("GET /api/v1/knowledge/", a.handleKnowledgeList)
	a.mux.HandleFunc("GET /api/v1/knowledge/list", a.handleKnowledgeList)
	a.mux.HandleFunc("GET /api/v1/knowledge/search", a.handleKnowledgeSearch)
	a.mux.HandleFunc("GET /api/v1/knowledge/search/files", a.handleKnowledgeFileSearch)
	a.mux.HandleFunc("POST /api/v1/knowledge/create", a.handleKnowledgeCreate)
	a.mux.HandleFunc("POST /api/v1/knowledge/reindex", a.handleKnowledgeReindex)
	a.mux.HandleFunc("GET /api/v1/knowledge/{id}", a.handleKnowledgeByID)
	a.mux.HandleFunc("POST /api/v1/knowledge/{id}/update", a.handleKnowledgeByID)
	a.mux.HandleFunc("DELETE /api/v1/knowledge/{id}/delete", a.handleKnowledgeByID)
	a.mux.HandleFunc("POST /api/v1/knowledge/{id}/file/add", a.handleKnowledgeFile)
	a.mux.HandleFunc("POST /api/v1/knowledge/{id}/file/update", a.handleKnowledgeFile)
	a.mux.HandleFunc("POST /api/v1/knowledge/{id}/file/remove", a.handleKnowledgeFile)
	a.mux.HandleFunc("POST /api/v1/knowledge/{id}/reset", a.handleKnowledgeReset)
	a.mux.HandleFunc("GET /api/v1/knowledge/{id}/export", a.handleKnowledgeExport)
	a.mux.HandleFunc("GET /api/v1/retrieval/", a.handleRetrievalStatus)
	a.mux.HandleFunc("GET /api/v1/retrieval/config", a.handleRetrievalConfig)
	a.mux.HandleFunc("POST /api/v1/retrieval/config/update", a.handleRetrievalConfigUpdate)
	a.mux.HandleFunc("GET /api/v1/retrieval/query/settings", a.handleRetrievalQuerySettings)
	a.mux.HandleFunc("POST /api/v1/retrieval/query/settings/update", a.handleRetrievalQuerySettings)
	a.mux.HandleFunc("GET /api/v1/retrieval/embedding", a.handleRetrievalEmbedding)
	a.mux.HandleFunc("POST /api/v1/retrieval/embedding/update", a.handleRetrievalEmbedding)
	a.mux.HandleFunc("GET /api/v1/retrieval/reranking", a.handleRetrievalReranking)
	a.mux.HandleFunc("POST /api/v1/retrieval/reranking/update", a.handleRetrievalReranking)
	a.mux.HandleFunc("POST /api/v1/retrieval/config/web/verify", a.handleRetrievalVerify)
	a.mux.HandleFunc("POST /api/v1/retrieval/config/web/playwright/verify", a.handleRetrievalVerify)
	a.mux.HandleFunc("POST /api/v1/retrieval/process/file", a.handleRetrievalProcessFile)
	a.mux.HandleFunc("POST /api/v1/retrieval/process/text", a.handleRetrievalProcessText)
	a.mux.HandleFunc("POST /api/v1/retrieval/process/web", a.handleRetrievalProcessWeb)
	a.mux.HandleFunc("POST /api/v1/retrieval/process/youtube", a.handleRetrievalProcessYouTube)
	a.mux.HandleFunc("POST /api/v1/retrieval/process/web/search", a.handleRetrievalProcessWebSearch)
	a.mux.HandleFunc("POST /api/v1/retrieval/process/files/batch", a.handleRetrievalBatch)
	a.mux.HandleFunc("POST /api/v1/retrieval/query/doc", a.handleRetrievalQuery)
	a.mux.HandleFunc("POST /api/v1/retrieval/query/collection", a.handleRetrievalQuery)
	a.mux.HandleFunc("POST /api/v1/retrieval/delete", a.handleRetrievalDelete)
	a.mux.HandleFunc("POST /api/v1/retrieval/reset/db", a.handleRetrievalReset)
	a.mux.HandleFunc("POST /api/v1/retrieval/reset/uploads", a.handleRetrievalReset)
	a.mux.HandleFunc("GET /api/v1/tasks/config", a.handleTaskConfig)
	a.mux.HandleFunc("POST /api/v1/tasks/config/update", a.handleTaskConfigUpdate)
	for _, task := range []string{"title", "tags", "follow_ups", "emoji", "queries", "auto", "moa", "image_prompt"} {
		a.mux.HandleFunc("POST /api/v1/tasks/"+task+"/completions", a.handleTaskCompletion)
	}
	a.mux.HandleFunc("GET /api/v1/users/", a.handleUsers)
	a.mux.HandleFunc("GET /api/v1/users/search", a.handleUsers)
	a.mux.HandleFunc("GET /api/v1/users/export/csv", a.handleUsersCSV)
	a.mux.HandleFunc("POST /api/v1/users/update/role", a.handleUserRole)
	a.mux.HandleFunc("GET /api/v1/users/groups", a.handleUserGroups)
	a.mux.HandleFunc("GET /api/v1/users/default/permissions", a.handleDefaultUserPermissions)
	a.mux.HandleFunc("POST /api/v1/users/default/permissions", a.handleDefaultUserPermissions)
	a.mux.HandleFunc("GET /api/v1/users/default/settings", a.handleDefaultUserSettings)
	a.mux.HandleFunc("POST /api/v1/users/default/settings", a.handleDefaultUserSettings)
	a.mux.HandleFunc("GET /api/v1/users/user/settings", a.handleCurrentUserSettings)
	a.mux.HandleFunc("POST /api/v1/users/user/settings/update", a.handleCurrentUserSettings)
	a.mux.HandleFunc("GET /api/v1/users/user/info", a.handleCurrentUserInfo)
	a.mux.HandleFunc("POST /api/v1/users/user/info/update", a.handleCurrentUserInfo)
	a.mux.HandleFunc("GET /api/v1/users/{id}", a.handleUserByID)
	a.mux.HandleFunc("DELETE /api/v1/users/{id}", a.handleUserByID)
	a.mux.HandleFunc("POST /api/v1/users/{id}/update", a.handleUserByID)
	a.registerResourceRoutes()
	a.mux.HandleFunc("GET /api/v1/skills/", a.handleSkills)
	a.mux.HandleFunc("GET /api/v1/skills/list", a.handleSkills)
	a.mux.HandleFunc("GET /api/v1/skills/catalog", a.handleSkillCatalog)
	a.mux.HandleFunc("GET /api/v1/skills/runtime/capabilities", a.handleSkillRuntimeCapabilities)
	a.mux.HandleFunc("GET /api/v1/skills/legacy-prompts", a.handleLegacySkills)
	a.mux.HandleFunc("POST /api/v1/skills/legacy-prompts/migrate", a.handleLegacySkills)
	a.mux.HandleFunc("POST /api/v1/skills/create", a.handleSkillCreate)
	a.mux.HandleFunc("POST /api/v1/skills/import", a.handleSkillCreate)
	a.mux.HandleFunc("POST /api/v1/skills/import/url", a.handleSkillRemoteImport("url"))
	a.mux.HandleFunc("POST /api/v1/skills/import/github", a.handleSkillRemoteImport("github"))
	a.mux.HandleFunc("POST /api/v1/skills/import/zip", a.handleSkillZipImport)
	a.mux.HandleFunc("GET /api/v1/skills/{skill_id}", a.handleSkillByID)
	a.mux.HandleFunc("POST /api/v1/skills/{skill_id}/update", a.handleSkillByID)
	a.mux.HandleFunc("POST /api/v1/skills/{skill_id}/auto", a.handleSkillByID)
	a.mux.HandleFunc("DELETE /api/v1/skills/{skill_id}/delete", a.handleSkillByID)
	a.mux.HandleFunc("POST /api/v1/skills/{skill_id}/runtime/install", a.handleSkillRuntimeInstall)
	a.mux.HandleFunc("DELETE /api/v1/skills/{skill_id}/runtime/install", a.handleSkillRuntimeInstall)
	a.mux.HandleFunc("GET /api/v1/skills/id/{id}", a.handleSkillByID)
	a.mux.HandleFunc("POST /api/v1/skills/id/{id}/update", a.handleSkillByID)
	a.mux.HandleFunc("POST /api/v1/skills/id/{id}/toggle", a.handleSkillByID)
	a.mux.HandleFunc("DELETE /api/v1/skills/id/{id}/delete", a.handleSkillByID)
	a.mux.HandleFunc("GET /api/v1/tools/export", a.handleToolExport)
	a.mux.HandleFunc("GET /api/v1/prompts/command/{command}", a.handlePromptByCommand("get"))
	a.mux.HandleFunc("POST /api/v1/prompts/command/{command}/update", a.handlePromptByCommand("update"))
	a.mux.HandleFunc("POST /api/v1/prompts/command/{command}/toggle", a.handlePromptByCommand("toggle"))
	a.mux.HandleFunc("DELETE /api/v1/prompts/command/{command}/delete", a.handlePromptByCommand("delete"))
	a.mux.HandleFunc("POST /api/v1/prompts/id/{id}/meta", a.handlePromptMeta)
	for _, suffix := range []string{"valves", "valves/spec", "valves/update", "valves/user", "valves/user/spec", "valves/user/update"} {
		method := "GET "
		if strings.HasSuffix(suffix, "/update") {
			method = "POST "
		}
		a.mux.HandleFunc(method+"/api/v1/tools/id/{id}/"+suffix, a.handlePluginValves("tool"))
	}
	a.mux.HandleFunc("GET /api/v1/functions/", a.handleFunctions)
	a.mux.HandleFunc("GET /api/v1/functions/export", a.handleFunctions)
	a.mux.HandleFunc("POST /api/v1/functions/create", a.handleFunctionCreate)
	a.mux.HandleFunc("GET /api/v1/functions/id/{id}", a.handleFunctionByID)
	a.mux.HandleFunc("POST /api/v1/functions/id/{id}/update", a.handleFunctionByID)
	a.mux.HandleFunc("POST /api/v1/functions/id/{id}/toggle", a.handleFunctionByID)
	a.mux.HandleFunc("POST /api/v1/functions/id/{id}/toggle/global", a.handleFunctionByID)
	a.mux.HandleFunc("DELETE /api/v1/functions/id/{id}/delete", a.handleFunctionByID)
	for _, suffix := range []string{"valves", "valves/spec", "valves/update", "valves/user", "valves/user/spec", "valves/user/update"} {
		method := "GET "
		if strings.HasSuffix(suffix, "/update") {
			method = "POST "
		}
		a.mux.HandleFunc(method+"/api/v1/functions/id/{id}/"+suffix, a.handlePluginValves("function"))
	}
	a.mux.HandleFunc("GET /api/v1/configs/export", a.handleConfigExport)
	a.mux.HandleFunc("POST /api/v1/configs/import", a.handleConfigImport)
	for _, item := range []struct {
		name  string
		admin bool
	}{
		{"direct_connections", true}, {"connections", true}, {"tool_servers", false},
		{"native_tools", false}, {"mcp_servers", false}, {"code_execution", true},
		{"models", true}, {"banners", true}, {"suggestions", true},
	} {
		a.mux.HandleFunc("GET /api/v1/configs/"+item.name, a.handleNamedConfig(item.name, item.admin))
		a.mux.HandleFunc("POST /api/v1/configs/"+item.name, a.handleNamedConfig(item.name, item.admin))
	}
	a.mux.HandleFunc("POST /api/v1/configs/tool_servers/verify", a.handleToolServerVerify)
	a.mux.HandleFunc("POST /api/v1/configs/mcp_servers/verify", a.handleMCPServerVerify)
	a.mux.HandleFunc("POST /api/v1/configs/tool_servers/{index}/share", a.handleConfigShare("openapi"))
	a.mux.HandleFunc("DELETE /api/v1/configs/tool_servers/{index}/share", a.handleConfigShare("openapi"))
	a.mux.HandleFunc("POST /api/v1/configs/mcp_servers/{index}/share", a.handleConfigShare("mcp"))
	a.mux.HandleFunc("DELETE /api/v1/configs/mcp_servers/{index}/share", a.handleConfigShare("mcp"))
	a.mux.HandleFunc("GET /api/v1/folders/", a.handleFolders)
	a.mux.HandleFunc("POST /api/v1/folders/", a.handleFolders)
	a.mux.HandleFunc("GET /api/v1/folders/{id}", a.handleFolderByID)
	a.mux.HandleFunc("POST /api/v1/folders/{id}/update", a.handleFolderByID)
	a.mux.HandleFunc("POST /api/v1/folders/{id}/update/parent", a.handleFolderByID)
	a.mux.HandleFunc("POST /api/v1/folders/{id}/update/expanded", a.handleFolderByID)
	a.mux.HandleFunc("POST /api/v1/folders/{id}/update/items", a.handleFolderByID)
	a.mux.HandleFunc("POST /api/v1/folders/{id}/update/icon", a.handleFolderByID)
	a.mux.HandleFunc("POST /api/v1/folders/{id}/update/system-prompt", a.handleFolderByID)
	a.mux.HandleFunc("DELETE /api/v1/folders/{id}", a.handleFolderByID)
	a.mux.HandleFunc("GET /api/v1/groups/", a.handleGroups)
	a.mux.HandleFunc("POST /api/v1/groups/create", a.handleGroups)
	a.mux.HandleFunc("GET /api/v1/groups/id/{id}", a.handleGroupByID)
	a.mux.HandleFunc("POST /api/v1/groups/id/{id}/update", a.handleGroupByID)
	a.mux.HandleFunc("DELETE /api/v1/groups/id/{id}/delete", a.handleGroupByID)
	a.mux.HandleFunc("GET /api/v1/files/", a.handleFiles)
	a.mux.HandleFunc("POST /api/v1/files/", a.handleFiles)
	a.mux.HandleFunc("GET /api/v1/files/search", a.handleFileSearch)
	a.mux.HandleFunc("POST /api/v1/files/upload/dir", a.handleFileUploadDir)
	a.mux.HandleFunc("DELETE /api/v1/files/all", a.handleAllFilesDelete)
	a.mux.HandleFunc("GET /api/v1/files/{id}", a.handleFileByID)
	a.mux.HandleFunc("DELETE /api/v1/files/{id}", a.handleFileByID)
	a.mux.HandleFunc("GET /api/v1/files/{id}/data/content", a.handleFileDataContent)
	a.mux.HandleFunc("GET /api/v1/files/{id}/content", a.handleFileContent)
	a.mux.HandleFunc("GET /api/v1/files/{id}/content/html", a.handleFileContent)
	a.mux.HandleFunc("GET /api/v1/files/{id}/content/{file_name}", a.handleFileContent)
	a.mux.HandleFunc("POST /api/v1/files/{id}/data/content/update", a.handleFileData)
	a.mux.HandleFunc("POST /ollama/api/chat", a.handleOllamaChat)
	a.mux.HandleFunc("GET /ollama/api/tags", a.handleOllamaTags)
	a.mux.HandleFunc("/ollama/", a.handleOllamaCompatibility)
	for _, provider := range []string{"anthropic", "gemini", "grok"} {
		a.mux.HandleFunc("GET /"+provider+"/config", a.handleNativeProviderConfig(provider))
		a.mux.HandleFunc("POST /"+provider+"/config/update", a.handleNativeProviderConfig(provider))
		a.mux.HandleFunc("GET /"+provider+"/models", a.handleNativeProviderModels(provider))
		a.mux.HandleFunc("GET /"+provider+"/models/{index}", a.handleNativeProviderModels(provider))
		a.mux.HandleFunc("POST /"+provider+"/verify", a.handleNativeProviderVerify(provider))
		a.mux.HandleFunc("POST /"+provider+"/health_check", a.handleNativeProviderVerify(provider))
		a.mux.HandleFunc("POST /"+provider+"/chat/completions", a.handleNativeProviderChat(provider))
	}
	a.mux.HandleFunc("GET /api/v1/providers/health", a.handleProviderHealth)
	a.mux.HandleFunc("GET /api/v1/audio/config", a.handleAudioConfig)
	a.mux.HandleFunc("POST /api/v1/audio/config/update", a.handleAudioConfig)
	a.mux.HandleFunc("GET /api/v1/audio/models", a.handleAudioModels)
	a.mux.HandleFunc("GET /api/v1/audio/voices", a.handleAudioVoices)
	a.mux.HandleFunc("POST /api/v1/audio/speech", a.handleAudioSpeech)
	a.mux.HandleFunc("POST /api/v1/audio/transcriptions", a.handleAudioTranscription)
	a.mux.HandleFunc("GET /api/v1/images/config", a.handleImageConfig)
	a.mux.HandleFunc("POST /api/v1/images/config/update", a.handleImageConfig)
	a.mux.HandleFunc("GET /api/v1/images/usage/config", a.handleImageUsageConfig)
	a.mux.HandleFunc("GET /api/v1/images/image/config", a.handleImageModelConfig)
	a.mux.HandleFunc("POST /api/v1/images/image/config/update", a.handleImageModelConfig)
	a.mux.HandleFunc("GET /api/v1/images/config/url/verify", a.handleImageURLVerify)
	a.mux.HandleFunc("GET /api/v1/images/models", a.handleImageModelList)
	a.mux.HandleFunc("POST /api/v1/images/generations", a.handleImageGeneration)
	a.mux.HandleFunc("GET /api/v1/channels/", a.handleChannels)
	a.mux.HandleFunc("POST /api/v1/channels/create", a.handleChannelCreate)
	a.mux.HandleFunc("GET /api/v1/channels/{id}", a.handleChannelByID)
	a.mux.HandleFunc("POST /api/v1/channels/{id}/update", a.handleChannelByID)
	a.mux.HandleFunc("DELETE /api/v1/channels/{id}/delete", a.handleChannelByID)
	a.mux.HandleFunc("GET /api/v1/channels/{id}/messages", a.handleChannelMessages)
	a.mux.HandleFunc("POST /api/v1/channels/{id}/messages/post", a.handleChannelMessages)
	a.mux.HandleFunc("GET /api/v1/channels/{id}/messages/{message_id}", a.handleChannelMessage)
	a.mux.HandleFunc("GET /api/v1/channels/{id}/messages/{message_id}/thread", a.handleChannelMessage)
	a.mux.HandleFunc("POST /api/v1/channels/{id}/messages/{message_id}/update", a.handleChannelMessage)
	a.mux.HandleFunc("DELETE /api/v1/channels/{id}/messages/{message_id}/delete", a.handleChannelMessage)
	a.mux.HandleFunc("POST /api/v1/channels/{id}/messages/{message_id}/reactions/add", a.handleChannelReaction)
	a.mux.HandleFunc("POST /api/v1/channels/{id}/messages/{message_id}/reactions/remove", a.handleChannelReaction)
	a.mux.HandleFunc("GET /api/v1/channels/{id}/webhook", a.handleChannelWebhook)
	a.mux.HandleFunc("POST /api/v1/channels/{id}/webhook", a.handleChannelWebhook)
	a.mux.HandleFunc("DELETE /api/v1/channels/{id}/webhook", a.handleChannelWebhook)
	a.mux.HandleFunc("POST /api/v1/channels/{id}/webhook/incoming", a.handleIncomingChannelWebhook)
	a.mux.HandleFunc("/api/v1/analytics/", a.handleAnalytics)
	a.mux.HandleFunc("/api/v1/haloclaw/", a.handleHaloClaw)
	a.mux.HandleFunc("/api/v1/external_api/", a.handleExternalAPI)
	a.mux.HandleFunc("GET /api/v1/terminal/config", a.handleTerminalConfig)
	a.mux.HandleFunc("POST /api/v1/terminal/config", a.handleTerminalConfig)
	a.mux.HandleFunc("GET /api/v1/terminal/files", a.handleTerminalFiles)
	a.mux.HandleFunc("DELETE /api/v1/terminal/files", a.handleTerminalFiles)
	a.mux.HandleFunc("GET /api/v1/terminal/files/content", a.handleTerminalFileContent)
	a.mux.HandleFunc("POST /api/v1/terminal/files/content", a.handleTerminalFileContent)
	a.mux.HandleFunc("GET /api/v1/terminal/files/binary", a.handleTerminalFileBinary)
	a.mux.HandleFunc("POST /api/v1/terminal/files/mkdir", a.handleTerminalMkdir)
	a.mux.HandleFunc("POST /api/v1/terminal/files/rename", a.handleTerminalRename)
	a.mux.HandleFunc("POST /api/v1/terminal/files/upload", a.handleTerminalUpload)
	a.mux.HandleFunc("GET /api/v1/terminal/files/raw", a.handleTerminalRaw)
	a.mux.HandleFunc("GET /api/v1/terminal/ports", a.handleTerminalPorts)
	a.mux.HandleFunc("GET /api/v1/terminal/sqlite/tables", a.handleTerminalSQLiteTables)
	a.mux.HandleFunc("POST /api/v1/terminal/sqlite/query", a.handleTerminalSQLiteQuery)
	for _, suffix := range []string{"gravatar", "code/format", "code/execute", "markdown", "pdf", "db/download", "db/restore/inspect", "db/restore", "db/merge/inspect", "db/merge", "litellm/config"} {
		method := "GET "
		if suffix == "code/format" || suffix == "code/execute" || suffix == "markdown" || suffix == "pdf" || suffix == "db/restore/inspect" || suffix == "db/restore" || suffix == "db/merge/inspect" || suffix == "db/merge" {
			method = "POST "
		}
		a.mux.HandleFunc(method+"/api/v1/utils/"+suffix, a.handleUtils)
	}
	a.registerSCIMRoutes()
	// The compatibility handler covers the remaining UI domains while their
	// storage and provider adapters are migrated to typed Go packages. More
	// specific patterns above always win in net/http's ServeMux.
	a.mux.HandleFunc("/api/v1/", a.handleCompatibility)
	a.mux.HandleFunc("/", a.handleFrontend)
}

func (a *App) registerResourceRoutes() {
	for _, resource := range []struct {
		kind   string
		prefix string
	}{
		{kind: "prompt", prefix: "/api/v1/prompts"},
		{kind: "tool", prefix: "/api/v1/tools"},
		{kind: "note", prefix: "/api/v1/notes"},
	} {
		a.mux.HandleFunc("GET "+resource.prefix+"/", a.handleResourceList(resource.kind, false))
		a.mux.HandleFunc("GET "+resource.prefix+"/list", a.handleResourceList(resource.kind, true))
		a.mux.HandleFunc("POST "+resource.prefix+"/create", a.handleResourceCreate(resource.kind))
		a.mux.HandleFunc("GET "+resource.prefix+"/id/{id}", a.handleResourceByID(resource.kind, "get"))
		a.mux.HandleFunc("POST "+resource.prefix+"/id/{id}/update", a.handleResourceByID(resource.kind, "update"))
		a.mux.HandleFunc("POST "+resource.prefix+"/id/{id}/toggle", a.handleResourceByID(resource.kind, "toggle"))
		a.mux.HandleFunc("DELETE "+resource.prefix+"/id/{id}/delete", a.handleResourceByID(resource.kind, "delete"))
		if resource.kind == "note" {
			a.mux.HandleFunc("GET "+resource.prefix+"/{id}", a.handleResourceByID(resource.kind, "get"))
			a.mux.HandleFunc("POST "+resource.prefix+"/{id}/update", a.handleResourceByID(resource.kind, "update"))
			a.mux.HandleFunc("DELETE "+resource.prefix+"/{id}/delete", a.handleResourceByID(resource.kind, "delete"))
		}
	}
}

func (a *App) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	response.Header().Set("X-Content-Type-Options", "nosniff")
	response.Header().Set("X-Frame-Options", "SAMEORIGIN")
	response.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
	a.mux.ServeHTTP(response, request)
}

func (a *App) handleHealth(response http.ResponseWriter, _ *http.Request) {
	writeJSON(response, http.StatusOK, map[string]bool{"status": true})
}

func (a *App) handleVersion(response http.ResponseWriter, _ *http.Request) {
	writeJSON(response, http.StatusOK, map[string]string{"version": a.config.Version})
}

func (a *App) handleConfig(response http.ResponseWriter, request *http.Request) {
	userCount, err := a.store.UserCount(request.Context())
	if err != nil {
		writeError(response, http.StatusInternalServerError, "database unavailable")
		return
	}
	ldapSettings, err := a.loadLDAPSettings(request)
	if err != nil {
		writeError(response, http.StatusInternalServerError, "failed to load authentication config")
		return
	}
	payload := map[string]any{
		"onboarding":                 userCount == 0,
		"status":                     true,
		"name":                       a.config.WebUIName,
		"version":                    a.config.Version,
		"default_locale":             a.config.DefaultLocale,
		"default_models":             "",
		"default_prompt_suggestions": []any{},
		"oauth": map[string]any{
			"providers": map[string]string{},
		},
		"features": map[string]any{
			"auth":                            true,
			"auth_trusted_header":             false,
			"enable_api_key":                  a.config.EnableAPIKey,
			"enable_signup":                   a.config.EnableSignup,
			"enable_login_form":               a.config.EnableLoginForm,
			"enable_ldap":                     ldapSettings.Enabled,
			"enable_code_execution":           false,
			"enable_code_interpreter":         false,
			"enable_websocket":                a.config.EnableWebSocket,
			"enable_web_search":               false,
			"enable_halo_web_search":          false,
			"enable_native_web_search":        false,
			"default_web_search_mode":         "off",
			"enable_google_drive_integration": false,
			"enable_onedrive_integration":     false,
			"enable_image_generation":         false,
			"enable_admin_export":             a.config.EnableAdminExport,
			"enable_admin_chat_access":        a.config.EnableAdminChatAccess,
			"enable_community_sharing":        false,
			"enable_autocomplete_generation":  false,
			"enable_direct_connections":       false,
			"enable_channels":                 false,
			"enable_user_webhooks":            false,
			"database_restore_supported":      true,
			"database_backend":                "sqlite",
			"database_restore_reason":         nil,
			"worker_count":                    1,
		},
	}
	codeConfig, codeErr := a.configResource(request, "system", "code_execution")
	if codeErr != nil {
		writeError(response, http.StatusInternalServerError, "failed to load code execution config")
		return
	}
	codeExecutionEnabled, _ := codeConfig["ENABLE_CODE_EXECUTION"].(bool)
	codeInterpreterEnabled, _ := codeConfig["ENABLE_CODE_INTERPRETER"].(bool)
	features := payload["features"].(map[string]any)
	features["enable_code_execution"] = codeExecutionEnabled
	features["enable_code_interpreter"] = codeInterpreterEnabled
	codeEngine, _ := codeConfig["CODE_EXECUTION_ENGINE"].(string)
	if !codeExecutionEnabled {
		codeEngine = ""
	}
	payload["code"] = map[string]any{"engine": codeEngine}
	if _, ok := a.currentUser(request); ok {
		audio, audioErr := a.loadAudioConfig(request)
		if audioErr != nil {
			writeError(response, http.StatusInternalServerError, "failed to load audio config")
			return
		}
		payload["audio"] = map[string]any{
			"tts": map[string]any{
				"engine":   audio.TTS.Engine,
				"voice":    audio.TTS.Voice,
				"split_on": audio.TTS.SplitOn,
			},
			"stt": map[string]any{"engine": audio.STT.Engine},
		}
	}
	writeJSON(response, http.StatusOK, payload)
}

func (a *App) handleSession(response http.ResponseWriter, request *http.Request) {
	user, ok := a.currentUser(request)
	if !ok {
		writeError(response, http.StatusUnauthorized, "Not authenticated")
		return
	}
	token, claims, err := auth.NewToken(a.secretKey(), user.ID, a.config.JWTExpiresAfter)
	if err != nil {
		writeError(response, http.StatusInternalServerError, "failed to create session")
		return
	}
	a.setSessionCookie(response, token, claims.ExpiresAt)
	_ = a.store.TouchUser(request.Context(), user.ID)
	a.writeSession(response, user, token, claims.ExpiresAt)
}

func (a *App) handleSignin(response http.ResponseWriter, request *http.Request) {
	var form struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !decodeJSON(response, request, &form) {
		return
	}
	user, passwordHash, err := a.store.Authenticate(request.Context(), form.Email)
	if err != nil || !auth.VerifyPassword(passwordHash, form.Password) {
		writeError(response, http.StatusBadRequest, "Invalid email or password")
		return
	}
	a.issueSession(response, request, user)
}

func (a *App) handleSignup(response http.ResponseWriter, request *http.Request) {
	var form struct {
		Name            string `json:"name"`
		Email           string `json:"email"`
		Password        string `json:"password"`
		ProfileImageURL string `json:"profile_image_url"`
	}
	if !decodeJSON(response, request, &form) {
		return
	}
	if !a.config.EnableSignup || !a.config.EnableLoginForm {
		writeError(response, http.StatusForbidden, "Access prohibited")
		return
	}
	if strings.TrimSpace(form.Name) == "" || !strings.Contains(form.Email, "@") {
		writeError(response, http.StatusBadRequest, "Invalid email format")
		return
	}
	if len([]byte(form.Password)) > 72 || len(form.Password) < 1 {
		writeError(response, http.StatusBadRequest, "Invalid password")
		return
	}
	passwordHash, err := auth.HashPassword(form.Password)
	if err != nil {
		writeError(response, http.StatusInternalServerError, "failed to hash password")
		return
	}
	if form.ProfileImageURL == "" {
		form.ProfileImageURL = "/user.png"
	}
	user, err := a.store.CreateUser(
		request.Context(), auth.RandomIDForInternalUse(), form.Name,
		form.Email, passwordHash, form.ProfileImageURL, a.config.DefaultUserRole,
	)
	if errors.Is(err, store.ErrEmailTaken) {
		writeError(response, http.StatusBadRequest, "Email already registered")
		return
	}
	if err != nil {
		writeError(response, http.StatusInternalServerError, "failed to create user")
		return
	}
	a.applyNewUserDefaults(request, user)
	a.issueSession(response, request, user)
}

func (a *App) handleSignout(response http.ResponseWriter, _ *http.Request) {
	http.SetCookie(response, &http.Cookie{
		Name:     "token",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})
	writeJSON(response, http.StatusOK, map[string]bool{"status": true})
}

func (a *App) currentUser(request *http.Request) (store.User, bool) {
	token := ""
	if cookie, err := request.Cookie("token"); err == nil {
		token = cookie.Value
	}
	if token == "" {
		token = strings.TrimSpace(request.Header.Get("X-API-Key"))
	}
	if token == "" {
		const prefix = "Bearer "
		header := request.Header.Get("Authorization")
		if strings.HasPrefix(header, prefix) {
			token = strings.TrimSpace(strings.TrimPrefix(header, prefix))
		}
	}
	if strings.HasPrefix(token, "sk-") {
		user, err := a.store.UserByAPIKey(request.Context(), token)
		return user, err == nil
	}
	claims, err := auth.ParseToken(a.secretKey(), token)
	if err != nil {
		return store.User{}, false
	}
	user, err := a.store.UserByID(request.Context(), claims.ID)
	return user, err == nil
}

func (a *App) issueSession(response http.ResponseWriter, request *http.Request, user store.User) {
	token, claims, err := auth.NewToken(a.secretKey(), user.ID, a.config.JWTExpiresAfter)
	if err != nil {
		writeError(response, http.StatusInternalServerError, "failed to create session")
		return
	}
	a.setSessionCookie(response, token, claims.ExpiresAt)
	_ = a.store.TouchUser(request.Context(), user.ID)
	a.writeSession(response, user, token, claims.ExpiresAt)
}

func (a *App) writeSession(response http.ResponseWriter, user store.User, token string, expiresAt int64) {
	payload := map[string]any{
		"token":             token,
		"token_type":        "Bearer",
		"expires_at":        nil,
		"id":                user.ID,
		"email":             user.Email,
		"name":              user.Name,
		"role":              user.Role,
		"profile_image_url": user.ProfileImageURL,
		"permissions":       map[string]any{},
	}
	if expiresAt > 0 {
		payload["expires_at"] = expiresAt
	}
	writeJSON(response, http.StatusOK, payload)
}

func (a *App) setSessionCookie(response http.ResponseWriter, token string, expiresAt int64) {
	cookie := &http.Cookie{
		Name:     "token",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   a.config.CookieSecure,
		SameSite: parseSameSite(a.config.CookieSameSite),
	}
	if expiresAt > 0 {
		cookie.Expires = time.Unix(expiresAt, 0).UTC()
	}
	http.SetCookie(response, cookie)
}

func (a *App) secretKey() []byte {
	if len(a.config.SecretKey) != 0 {
		return a.config.SecretKey
	}
	return []byte("development-only-secret")
}

func parseSameSite(value string) http.SameSite {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "strict":
		return http.SameSiteStrictMode
	case "none":
		return http.SameSiteNoneMode
	default:
		return http.SameSiteLaxMode
	}
}

func (a *App) handleFrontend(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet && request.Method != http.MethodHead {
		http.NotFound(response, request)
		return
	}
	path := strings.TrimPrefix(filepath.ToSlash(request.URL.Path), "/")
	if path == "" {
		a.writeIndex(response, request)
		return
	}
	if strings.Contains(path, "..") {
		http.NotFound(response, request)
		return
	}
	file, err := fs.ReadFile(a.frontend, path)
	if err != nil {
		a.writeIndex(response, request)
		return
	}
	if extension := filepath.Ext(path); extension != "" {
		if contentType := mime.TypeByExtension(extension); contentType != "" {
			response.Header().Set("Content-Type", contentType)
		}
	}
	if strings.HasPrefix(path, "_app/immutable/") {
		response.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		response.Header().Set("Cache-Control", "public, max-age=86400")
	}
	response.Header().Set("Content-Length", integerString(len(file)))
	if request.Method == http.MethodGet {
		_, _ = response.Write(file)
	}
}

func (a *App) writeIndex(response http.ResponseWriter, request *http.Request) {
	response.Header().Set("Content-Type", "text/html; charset=utf-8")
	response.Header().Set("Cache-Control", "no-cache")
	response.Header().Set("Content-Length", integerString(len(a.index)))
	if request.Method == http.MethodGet {
		_, _ = response.Write(a.index)
	}
}

func writeJSON(response http.ResponseWriter, status int, payload any) {
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(status)
	_ = json.NewEncoder(response).Encode(payload)
}

func writeRawJSON(response http.ResponseWriter, status int, payload json.RawMessage) {
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(status)
	_, _ = response.Write(payload)
}

func writeError(response http.ResponseWriter, status int, detail string) {
	writeJSON(response, status, map[string]string{"detail": detail})
}

func decodeJSON(response http.ResponseWriter, request *http.Request, target any) bool {
	request.Body = http.MaxBytesReader(response, request.Body, 1<<20)
	if err := json.NewDecoder(request.Body).Decode(target); err != nil {
		writeError(response, http.StatusBadRequest, "Invalid JSON body")
		return false
	}
	return true
}

func integerString(value int) string {
	// Avoid fmt in the request path.
	if value == 0 {
		return "0"
	}
	buffer := [20]byte{}
	position := len(buffer)
	for value > 0 {
		position--
		buffer[position] = byte('0' + value%10)
		value /= 10
	}
	return string(buffer[position:])
}

func CheckHealth(url string, timeout time.Duration) error {
	client := &http.Client{Timeout: timeout}
	response, err := client.Get(url)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return errors.New("health endpoint returned a non-200 response")
	}
	return nil
}
