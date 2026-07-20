package server

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-ldap/ldap/v3"
	"github.com/iwvw-per/HaloWebUI/backend/internal/auth"
	"github.com/iwvw-per/HaloWebUI/backend/internal/store"
)

const ldapConfigOwner = "system"

var ldapAttributePattern = regexp.MustCompile(`^(?:[A-Za-z][A-Za-z0-9-]*|[0-9]+(?:\.[0-9]+)+)$`)

type ldapServerConfig struct {
	Label                string  `json:"label"`
	Host                 string  `json:"host"`
	Port                 int     `json:"port"`
	AttributeForMail     string  `json:"attribute_for_mail"`
	AttributeForUsername string  `json:"attribute_for_username"`
	AppDN                string  `json:"app_dn"`
	AppDNPassword        string  `json:"app_dn_password"`
	SearchBase           string  `json:"search_base"`
	SearchFilters        string  `json:"search_filters"`
	UseTLS               bool    `json:"use_tls"`
	CertificatePath      *string `json:"certificate_path"`
	Ciphers              string  `json:"ciphers"`
}

type ldapSettings struct {
	Enabled bool
	Server  ldapServerConfig
}

type ldapIdentity struct {
	Username string
	Email    string
	Name     string
}

type ldapAuthenticator interface {
	Authenticate(context.Context, ldapServerConfig, string, string) (ldapIdentity, error)
}

type goLDAPAuthenticator struct{}

func (goLDAPAuthenticator) Authenticate(ctx context.Context, config ldapServerConfig, username, password string) (ldapIdentity, error) {
	if err := validateLDAPServerConfig(config); err != nil {
		return ldapIdentity{}, err
	}
	if strings.TrimSpace(username) == "" || password == "" {
		return ldapIdentity{}, errors.New("LDAP username and password are required")
	}
	dial := func() (*ldap.Conn, error) {
		port := config.Port
		if port == 0 {
			if config.UseTLS {
				port = 636
			} else {
				port = 389
			}
		}
		scheme := "ldap"
		options := []ldap.DialOpt{ldap.DialWithDialer(&net.Dialer{Timeout: 10 * time.Second})}
		if config.UseTLS {
			scheme = "ldaps"
			tlsConfig, err := ldapTLSConfig(config)
			if err != nil {
				return nil, err
			}
			options = append(options, ldap.DialWithTLSConfig(tlsConfig))
		}
		connection, err := ldap.DialURL(fmt.Sprintf("%s://%s", scheme, net.JoinHostPort(config.Host, strconv.Itoa(port))), options...)
		if err != nil {
			return nil, fmt.Errorf("connect to LDAP server: %w", err)
		}
		connection.SetTimeout(10 * time.Second)
		return connection, nil
	}

	if err := ctx.Err(); err != nil {
		return ldapIdentity{}, err
	}
	appConnection, err := dial()
	if err != nil {
		return ldapIdentity{}, err
	}
	defer appConnection.Close()
	if config.AppDN == "" {
		err = appConnection.UnauthenticatedBind("")
	} else {
		err = appConnection.Bind(config.AppDN, config.AppDNPassword)
	}
	if err != nil {
		return ldapIdentity{}, errors.New("LDAP application account bind failed")
	}
	filter := fmt.Sprintf("(&(%s=%s)%s)", config.AttributeForUsername, ldap.EscapeFilter(strings.ToLower(strings.TrimSpace(username))), strings.TrimSpace(config.SearchFilters))
	result, err := appConnection.Search(ldap.NewSearchRequest(
		config.SearchBase, ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 2, 10, false,
		filter, []string{config.AttributeForUsername, config.AttributeForMail, "cn"}, nil,
	))
	if err != nil {
		return ldapIdentity{}, errors.New("LDAP user search failed")
	}
	if len(result.Entries) != 1 {
		return ldapIdentity{}, errors.New("LDAP user was not found or is ambiguous")
	}
	entry := result.Entries[0]
	resolvedUsername := strings.ToLower(strings.TrimSpace(entry.GetAttributeValue(config.AttributeForUsername)))
	if resolvedUsername != strings.ToLower(strings.TrimSpace(username)) {
		return ldapIdentity{}, errors.New("LDAP user record mismatch")
	}
	email := strings.ToLower(strings.TrimSpace(entry.GetAttributeValue(config.AttributeForMail)))
	if !validLDAPEmail(email) {
		return ldapIdentity{}, errors.New("LDAP user does not have a valid email address")
	}

	userConnection, err := dial()
	if err != nil {
		return ldapIdentity{}, err
	}
	defer userConnection.Close()
	if err := userConnection.Bind(entry.DN, password); err != nil {
		return ldapIdentity{}, errors.New("LDAP authentication failed")
	}
	name := strings.TrimSpace(entry.GetAttributeValue("cn"))
	if name == "" {
		name = resolvedUsername
	}
	return ldapIdentity{Username: resolvedUsername, Email: email, Name: name}, nil
}

func ldapTLSConfig(config ldapServerConfig) (*tls.Config, error) {
	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12, ServerName: config.Host}
	if config.CertificatePath == nil || strings.TrimSpace(*config.CertificatePath) == "" {
		return tlsConfig, nil
	}
	pem, err := os.ReadFile(strings.TrimSpace(*config.CertificatePath))
	if err != nil {
		return nil, errors.New("failed to read LDAP CA certificate")
	}
	pool, err := x509.SystemCertPool()
	if err != nil || pool == nil {
		pool = x509.NewCertPool()
	}
	if !pool.AppendCertsFromPEM(pem) {
		return nil, errors.New("LDAP CA certificate is invalid")
	}
	tlsConfig.RootCAs = pool
	return tlsConfig, nil
}

func validateLDAPServerConfig(config ldapServerConfig) error {
	if strings.TrimSpace(config.Label) == "" || strings.TrimSpace(config.Host) == "" || strings.TrimSpace(config.SearchBase) == "" {
		return errors.New("LDAP label, host, and search base are required")
	}
	if config.Port < 0 || config.Port > 65535 {
		return errors.New("LDAP port must be between 1 and 65535")
	}
	if !ldapAttributePattern.MatchString(config.AttributeForMail) || !ldapAttributePattern.MatchString(config.AttributeForUsername) {
		return errors.New("LDAP attribute name is invalid")
	}
	return nil
}

func validLDAPEmail(value string) bool {
	at := strings.LastIndexByte(value, '@')
	return at > 0 && at < len(value)-1 && !strings.ContainsAny(value, "\r\n")
}

func (a *App) loadLDAPSettings(r *http.Request) (ldapSettings, error) {
	value, err := a.configResource(r, ldapConfigOwner, "ldap")
	if err != nil {
		return ldapSettings{}, err
	}
	enabled, _ := value["ENABLE_LDAP"].(bool)
	serverValue, _ := value["server"].(map[string]any)
	encoded, _ := json.Marshal(serverValue)
	var server ldapServerConfig
	if err := json.Unmarshal(encoded, &server); err != nil {
		return ldapSettings{}, errors.New("invalid stored LDAP config")
	}
	return ldapSettings{Enabled: enabled, Server: server}, nil
}

func (a *App) saveLDAPSettings(r *http.Request, settings ldapSettings) error {
	server, _ := json.Marshal(settings.Server)
	var serverValue map[string]any
	_ = json.Unmarshal(server, &serverValue)
	return a.saveConfigResource(r, ldapConfigOwner, "ldap", map[string]any{"ENABLE_LDAP": settings.Enabled, "server": serverValue})
}

func (a *App) handleLDAPConfig(w http.ResponseWriter, r *http.Request) {
	if ok, _ := a.requireAdmin(w, r); !ok {
		return
	}
	settings, err := a.loadLDAPSettings(r)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load LDAP config")
		return
	}
	if r.Method == http.MethodPost {
		var form struct {
			EnableLDAP *bool `json:"enable_ldap"`
		}
		if !decodeJSON(w, r, &form) {
			return
		}
		if form.EnableLDAP == nil {
			writeError(w, http.StatusBadRequest, "enable_ldap is required")
			return
		}
		if *form.EnableLDAP {
			if err := validateLDAPServerConfig(settings.Server); err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
		}
		settings.Enabled = *form.EnableLDAP
		if err := a.saveLDAPSettings(r, settings); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to save LDAP config")
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ENABLE_LDAP": settings.Enabled})
}

func (a *App) handleLDAPServerConfig(w http.ResponseWriter, r *http.Request) {
	if ok, _ := a.requireAdmin(w, r); !ok {
		return
	}
	settings, err := a.loadLDAPSettings(r)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load LDAP config")
		return
	}
	if r.Method == http.MethodPost {
		var server ldapServerConfig
		if !decodeJSON(w, r, &server) {
			return
		}
		if err := validateLDAPServerConfig(server); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		settings.Server = server
		if err := a.saveLDAPSettings(r, settings); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to save LDAP server config")
			return
		}
	}
	writeJSON(w, http.StatusOK, settings.Server)
}

func (a *App) handleLDAPSignin(w http.ResponseWriter, r *http.Request) {
	settings, err := a.loadLDAPSettings(r)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load LDAP config")
		return
	}
	if !settings.Enabled {
		writeError(w, http.StatusBadRequest, "LDAP authentication is not enabled")
		return
	}
	var form struct {
		User     string `json:"user"`
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &form) {
		return
	}
	if strings.TrimSpace(form.User) == "" || form.Password == "" {
		writeError(w, http.StatusBadRequest, "LDAP username and password are required")
		return
	}
	identity, err := a.ldapAuth.Authenticate(r.Context(), settings.Server, form.User, form.Password)
	if err != nil {
		writeError(w, http.StatusBadRequest, "LDAP authentication failed")
		return
	}
	user, err := a.store.UserByEmail(r.Context(), identity.Email)
	if errors.Is(err, store.ErrUserNotFound) {
		randomPassword, hashErr := auth.HashPassword(auth.RandomIDForInternalUse())
		if hashErr != nil {
			writeError(w, http.StatusInternalServerError, "failed to create LDAP user")
			return
		}
		user, err = a.store.CreateUser(r.Context(), auth.RandomIDForInternalUse(), identity.Name, identity.Email, randomPassword, "/user.png", a.config.DefaultUserRole)
		if err == nil {
			a.applyNewUserDefaults(r, user)
		}
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load LDAP user")
		return
	}
	a.issueSession(w, r, user)
}
