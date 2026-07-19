package server

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/iwvw-per/HaloWebUI/backend-go/internal/auth"
)

const mebibyte = int64(1024 * 1024)

type Config struct {
	Host                  string
	Port                  int
	Version               string
	WebUIName             string
	DefaultLocale         string
	FrontendDir           string
	DataDir               string
	OpenAIBaseURL         string
	OpenAIAPIKey          string
	OllamaBaseURL         string
	OllamaAPIKey          string
	SecretKey             []byte
	JWTExpiresAfter       time.Duration
	GoMemoryLimitBytes    int64
	EnableSignup          bool
	EnableLoginForm       bool
	EnableWebSocket       bool
	EnableAPIKey          bool
	EnableAdminChatAccess bool
	EnableTerminal        bool
	DefaultUserRole       string
	CookieSecure          bool
	CookieSameSite        string
	FileMaxSizeBytes      int64
}

func LoadConfig(version string) (Config, error) {
	port, err := envInt("PORT", 8080)
	if err != nil || port < 1 || port > 65535 {
		return Config{}, fmt.Errorf("invalid PORT: %q", os.Getenv("PORT"))
	}
	memoryMiB, err := envInt("HALO_GO_MEMORY_LIMIT_MIB", 48)
	if err != nil || memoryMiB < 16 || memoryMiB > 160 {
		return Config{}, fmt.Errorf(
			"invalid HALO_GO_MEMORY_LIMIT_MIB: %q; expected 16..160",
			os.Getenv("HALO_GO_MEMORY_LIMIT_MIB"),
		)
	}
	fileMaxMiB, err := envInt("FILE_MAX_SIZE_MB", 25)
	if err != nil || fileMaxMiB < 1 || fileMaxMiB > 250 {
		return Config{}, fmt.Errorf("invalid FILE_MAX_SIZE_MB: %q", os.Getenv("FILE_MAX_SIZE_MB"))
	}

	frontendDir := envString("FRONTEND_DIR", "/app/build")
	if absolute, err := filepath.Abs(frontendDir); err == nil {
		frontendDir = absolute
	}
	dataDir := envString("DATA_DIR", "/app/data")
	if absolute, err := filepath.Abs(dataDir); err == nil {
		dataDir = absolute
	}
	secret, err := auth.LoadOrCreateSecret(dataDir)
	if err != nil {
		return Config{}, err
	}
	expiresAfter, err := parseExpiry(envString("JWT_EXPIRES_IN", "-1"))
	if err != nil {
		return Config{}, err
	}

	return Config{
		Host:                  envString("HOST", "0.0.0.0"),
		Port:                  port,
		Version:               version,
		WebUIName:             envString("WEBUI_NAME", "HaloWebUI"),
		DefaultLocale:         envString("DEFAULT_LOCALE", "zh-CN"),
		FrontendDir:           frontendDir,
		DataDir:               dataDir,
		OpenAIBaseURL:         strings.TrimRight(envString("OPENAI_API_BASE_URL", "https://api.openai.com/v1"), "/"),
		OpenAIAPIKey:          os.Getenv("OPENAI_API_KEY"),
		OllamaBaseURL:         strings.TrimRight(envString("OLLAMA_BASE_URL", "http://127.0.0.1:11434"), "/"),
		OllamaAPIKey:          os.Getenv("OLLAMA_API_KEY"),
		SecretKey:             secret,
		JWTExpiresAfter:       expiresAfter,
		GoMemoryLimitBytes:    int64(memoryMiB) * mebibyte,
		EnableSignup:          envBool("ENABLE_SIGNUP", true),
		EnableLoginForm:       envBool("ENABLE_LOGIN_FORM", true),
		EnableWebSocket:       envBool("ENABLE_WEBSOCKET_SUPPORT", false),
		EnableAPIKey:          envBool("ENABLE_API_KEY", true),
		EnableAdminChatAccess: envBool("ENABLE_ADMIN_CHAT_ACCESS", false),
		EnableTerminal:        envBool("ENABLE_TERMINAL", false),
		DefaultUserRole:       envString("DEFAULT_USER_ROLE", "pending"),
		CookieSecure:          envBool("WEBUI_AUTH_COOKIE_SECURE", false),
		CookieSameSite:        envString("WEBUI_AUTH_COOKIE_SAME_SITE", "lax"),
		FileMaxSizeBytes:      int64(fileMaxMiB) * mebibyte,
	}, nil
}

func (c Config) ListenAddress() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

func (c Config) HealthURL() string {
	host := c.Host
	if host == "0.0.0.0" || host == "::" || host == "" {
		host = "127.0.0.1"
	}
	return fmt.Sprintf("http://%s:%d/health", host, c.Port)
}

func envString(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

func envInt(name string, fallback int) (int, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback, nil
	}
	return strconv.Atoi(value)
}

func envBool(name string, fallback bool) bool {
	value := strings.TrimSpace(strings.ToLower(os.Getenv(name)))
	switch value {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}

func parseExpiry(value string) (time.Duration, error) {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" || value == "-1" || value == "none" {
		return 0, nil
	}
	if len(value) > 1 {
		unit := value[len(value)-1]
		amount, err := strconv.Atoi(value[:len(value)-1])
		if err == nil && amount >= 0 {
			switch unit {
			case 'd':
				return time.Duration(amount) * 24 * time.Hour, nil
			case 'w':
				return time.Duration(amount) * 7 * 24 * time.Hour, nil
			}
		}
	}
	duration, err := time.ParseDuration(value)
	if err != nil || duration < 0 {
		return 0, fmt.Errorf("invalid JWT_EXPIRES_IN: %q", value)
	}
	return duration, nil
}
