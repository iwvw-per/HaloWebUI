package server

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/iwvw-per/HaloWebUI/backend/internal/auth"
	"github.com/iwvw-per/HaloWebUI/backend/internal/store"
)

const (
	scimUserSchema     = "urn:ietf:params:scim:schemas:core:2.0:User"
	scimGroupSchema    = "urn:ietf:params:scim:schemas:core:2.0:Group"
	scimListSchema     = "urn:ietf:params:scim:api:messages:2.0:ListResponse"
	scimErrorSchema    = "urn:ietf:params:scim:api:messages:2.0:Error"
	scimProviderSchema = "urn:ietf:params:scim:schemas:core:2.0:ServiceProviderConfig"
	scimTypeSchema     = "urn:ietf:params:scim:schemas:core:2.0:ResourceType"
	scimSchemaSchema   = "urn:ietf:params:scim:schemas:core:2.0:Schema"
)

func (a *App) registerSCIMRoutes() {
	a.mux.HandleFunc("GET /scim/v2/ServiceProviderConfig", a.handleSCIMDiscovery)
	a.mux.HandleFunc("GET /scim/v2/ResourceTypes", a.handleSCIMDiscovery)
	a.mux.HandleFunc("GET /scim/v2/Schemas", a.handleSCIMDiscovery)
	a.mux.HandleFunc("GET /scim/v2/Users", a.handleSCIMUsers)
	a.mux.HandleFunc("POST /scim/v2/Users", a.handleSCIMUsers)
	a.mux.HandleFunc("GET /scim/v2/Users/{id}", a.handleSCIMUser)
	a.mux.HandleFunc("PUT /scim/v2/Users/{id}", a.handleSCIMUser)
	a.mux.HandleFunc("PATCH /scim/v2/Users/{id}", a.handleSCIMUser)
	a.mux.HandleFunc("DELETE /scim/v2/Users/{id}", a.handleSCIMUser)
	a.mux.HandleFunc("GET /scim/v2/Groups", a.handleSCIMGroups)
	a.mux.HandleFunc("POST /scim/v2/Groups", a.handleSCIMGroups)
	a.mux.HandleFunc("GET /scim/v2/Groups/{id}", a.handleSCIMGroup)
	a.mux.HandleFunc("PUT /scim/v2/Groups/{id}", a.handleSCIMGroup)
	a.mux.HandleFunc("PATCH /scim/v2/Groups/{id}", a.handleSCIMGroup)
	a.mux.HandleFunc("DELETE /scim/v2/Groups/{id}", a.handleSCIMGroup)
}

func scimEnabled() bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv("ENABLE_SCIM")))
	return value == "true" || value == "1" || value == "yes" || value == "on"
}

func requireSCIM(w http.ResponseWriter, r *http.Request) bool {
	if !scimEnabled() {
		writeSCIMError(w, http.StatusNotFound, "SCIM provisioning is not enabled")
		return false
	}
	expected := strings.TrimSpace(os.Getenv("SCIM_AUTH_BEARER_TOKEN"))
	if expected == "" {
		writeSCIMError(w, http.StatusInternalServerError, "SCIM bearer token not configured")
		return false
	}
	provided := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
	if len(provided) != len(expected) || subtle.ConstantTimeCompare([]byte(provided), []byte(expected)) != 1 {
		writeSCIMError(w, http.StatusUnauthorized, "Invalid SCIM bearer token")
		return false
	}
	return true
}

func writeSCIMError(w http.ResponseWriter, status int, detail string) {
	w.Header().Set("Content-Type", "application/scim+json")
	writeJSON(w, status, map[string]any{"schemas": []string{scimErrorSchema}, "status": strconv.Itoa(status), "detail": detail})
}

func (a *App) handleSCIMDiscovery(w http.ResponseWriter, r *http.Request) {
	if !requireSCIM(w, r) {
		return
	}
	w.Header().Set("Content-Type", "application/scim+json")
	switch r.URL.Path {
	case "/scim/v2/ServiceProviderConfig":
		writeJSON(w, http.StatusOK, map[string]any{"schemas": []string{scimProviderSchema}, "documentationUri": "https://github.com/ztx888/HaloWebUI", "patch": map[string]bool{"supported": true}, "bulk": map[string]any{"supported": false, "maxOperations": 0, "maxPayloadSize": 0}, "filter": map[string]any{"supported": true, "maxResults": 200}, "changePassword": map[string]bool{"supported": false}, "sort": map[string]bool{"supported": false}, "etag": map[string]bool{"supported": false}, "authenticationSchemes": []map[string]string{{"type": "oauthbearertoken", "name": "Bearer Token", "description": "Authentication via static bearer token"}}})
	case "/scim/v2/ResourceTypes":
		resources := []map[string]any{{"schemas": []string{scimTypeSchema}, "id": "User", "name": "User", "endpoint": "/Users", "schema": scimUserSchema}, {"schemas": []string{scimTypeSchema}, "id": "Group", "name": "Group", "endpoint": "/Groups", "schema": scimGroupSchema}}
		writeJSON(w, http.StatusOK, scimList(resources, 1, len(resources)))
	case "/scim/v2/Schemas":
		resources := []map[string]any{{"schemas": []string{scimSchemaSchema}, "id": scimUserSchema, "name": "User", "description": "User Account", "attributes": []map[string]any{{"name": "userName", "type": "string", "required": true, "uniqueness": "server"}, {"name": "displayName", "type": "string"}, {"name": "emails", "type": "complex", "multiValued": true}, {"name": "active", "type": "boolean"}, {"name": "externalId", "type": "string"}}}, {"schemas": []string{scimSchemaSchema}, "id": scimGroupSchema, "name": "Group", "description": "Group", "attributes": []map[string]any{{"name": "displayName", "type": "string", "required": true}, {"name": "members", "type": "complex", "multiValued": true}, {"name": "externalId", "type": "string"}}}}
		writeJSON(w, http.StatusOK, scimList(resources, 1, len(resources)))
	}
}

func scimList[T any](resources []T, start, count int) map[string]any {
	return map[string]any{"schemas": []string{scimListSchema}, "totalResults": len(resources), "startIndex": start, "itemsPerPage": count, "Resources": resources}
}

func (a *App) handleSCIMUsers(w http.ResponseWriter, r *http.Request) {
	if !requireSCIM(w, r) {
		return
	}
	w.Header().Set("Content-Type", "application/scim+json")
	if r.Method == http.MethodPost {
		a.createSCIMUser(w, r)
		return
	}
	start := queryInt(r, "startIndex", 1, 1, 1000000)
	count := queryInt(r, "count", 100, 0, 200)
	users, err := a.store.ListUsers(r.Context(), "", 500)
	if err != nil {
		writeSCIMError(w, http.StatusInternalServerError, "Failed to list users")
		return
	}
	attribute, value := parseSCIMFilter(r.URL.Query().Get("filter"))
	resources := make([]map[string]any, 0)
	for _, user := range users {
		if scimUserMatches(user, attribute, value) {
			resources = append(resources, a.userToSCIM(r, user))
		}
	}
	total := len(resources)
	if attribute == "" {
		from := start - 1
		if from > len(resources) {
			from = len(resources)
		}
		to := from + count
		if to > len(resources) {
			to = len(resources)
		}
		resources = resources[from:to]
	}
	payload := scimList(resources, start, count)
	payload["totalResults"] = total
	writeJSON(w, http.StatusOK, payload)
}

func (a *App) createSCIMUser(w http.ResponseWriter, r *http.Request) {
	var body map[string]any
	if !decodeJSON(w, r, &body) {
		return
	}
	email, name, external, active := scimUserFields(body)
	if email == "" {
		writeSCIMError(w, http.StatusBadRequest, "userName or email is required")
		return
	}
	if existing, _ := a.findUserByEmail(r, email); existing.ID != "" {
		writeSCIMError(w, http.StatusConflict, "User with email '"+email+"' already exists")
		return
	}
	if external != "" {
		if existing, _ := a.findUserByExternalID(r, external); existing.ID != "" {
			writeSCIMError(w, http.StatusConflict, "externalId '"+external+"' already in use")
			return
		}
	}
	passwordHash, err := auth.HashPassword(auth.RandomIDForInternalUse() + auth.RandomIDForInternalUse())
	if err != nil {
		writeSCIMError(w, http.StatusInternalServerError, "Failed to create user")
		return
	}
	role := "user"
	if !active {
		role = "pending"
	}
	user, err := a.store.CreateUser(r.Context(), auth.RandomIDForInternalUse(), name, email, passwordHash, "/user.png", role)
	if err != nil {
		writeSCIMError(w, http.StatusConflict, "Failed to create user")
		return
	}
	if user.Role != role {
		user, _ = a.store.UpdateUser(r.Context(), user.ID, "", "", role, "")
	}
	if external != "" {
		_ = a.setSCIMUserExternalID(r, user.ID, external)
		user, _ = a.store.UserByID(r.Context(), user.ID)
	}
	w.WriteHeader(http.StatusCreated)
	_, _ = w.Write(mustJSON(a.userToSCIM(r, user)))
}

func (a *App) handleSCIMUser(w http.ResponseWriter, r *http.Request) {
	if !requireSCIM(w, r) {
		return
	}
	w.Header().Set("Content-Type", "application/scim+json")
	user, err := a.store.UserByID(r.Context(), r.PathValue("id"))
	if errors.Is(err, store.ErrUserNotFound) {
		writeSCIMError(w, http.StatusNotFound, "User not found")
		return
	}
	if err != nil {
		writeSCIMError(w, http.StatusInternalServerError, "Failed to load user")
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, a.userToSCIM(r, user))
	case http.MethodDelete:
		if _, err := a.store.UpdateUser(r.Context(), user.ID, "", "", "pending", ""); err != nil {
			writeSCIMError(w, http.StatusInternalServerError, "Failed to deactivate user")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	case http.MethodPut:
		var body map[string]any
		if !decodeJSON(w, r, &body) {
			return
		}
		email, name, external, active := scimUserFields(body)
		role := user.Role
		if active && role == "pending" {
			role = "user"
		}
		if !active {
			role = "pending"
		}
		updated, err := a.store.UpdateUser(r.Context(), user.ID, name, email, role, "")
		if err != nil {
			writeSCIMError(w, http.StatusConflict, "Failed to update user")
			return
		}
		if external != "" {
			_ = a.setSCIMUserExternalID(r, user.ID, external)
			updated, _ = a.store.UserByID(r.Context(), user.ID)
		}
		writeJSON(w, http.StatusOK, a.userToSCIM(r, updated))
	case http.MethodPatch:
		updated, err := a.patchSCIMUser(r, user)
		if err != nil {
			writeSCIMError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, a.userToSCIM(r, updated))
	}
}

func scimUserFields(body map[string]any) (email, name, external string, active bool) {
	email, _ = body["userName"].(string)
	name, _ = body["displayName"].(string)
	external, _ = body["externalId"].(string)
	active = true
	if value, ok := body["active"].(bool); ok {
		active = value
	}
	if object, ok := body["name"].(map[string]any); ok && name == "" {
		name, _ = object["formatted"].(string)
		if name == "" {
			name, _ = object["givenName"].(string)
		}
	}
	if emails, ok := body["emails"].([]any); ok {
		for _, item := range emails {
			entry, _ := item.(map[string]any)
			value, _ := entry["value"].(string)
			if value != "" {
				email = value
			}
			if primary, _ := entry["primary"].(bool); primary && value != "" {
				email = value
				break
			}
		}
	}
	email = strings.ToLower(strings.TrimSpace(email))
	if name == "" {
		name = strings.Split(email, "@")[0]
	}
	return
}

func (a *App) patchSCIMUser(r *http.Request, user store.User) (store.User, error) {
	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return store.User{}, err
	}
	operations, _ := body["Operations"].([]any)
	if operations == nil {
		operations, _ = body["operations"].([]any)
	}
	name, email, role := user.Name, user.Email, user.Role
	external := ""
	for _, raw := range operations {
		op, _ := raw.(map[string]any)
		if strings.ToLower(stringField(op, "op")) != "replace" {
			continue
		}
		path := stringField(op, "path")
		value := op["value"]
		if path == "active" {
			if active, ok := value.(bool); ok {
				if active && role == "pending" {
					role = "user"
				}
				if !active {
					role = "pending"
				}
			}
		}
		if path == "displayName" || path == "name.formatted" {
			if v, ok := value.(string); ok {
				name = v
			}
		}
		if path == "userName" || strings.HasPrefix(path, "emails[") {
			if v, ok := value.(string); ok {
				email = v
			}
		}
		if path == "externalId" {
			external, _ = value.(string)
		}
		if path == "" {
			if values, ok := value.(map[string]any); ok {
				if v, ok := values["displayName"].(string); ok {
					name = v
				}
				if v, ok := values["userName"].(string); ok {
					email = v
				}
				if active, ok := values["active"].(bool); ok {
					if active && role == "pending" {
						role = "user"
					}
					if !active {
						role = "pending"
					}
				}
			}
		}
	}
	updated, err := a.store.UpdateUser(r.Context(), user.ID, name, email, role, "")
	if err != nil {
		return store.User{}, err
	}
	if external != "" {
		if err := a.setSCIMUserExternalID(r, user.ID, external); err != nil {
			return store.User{}, err
		}
		updated, _ = a.store.UserByID(r.Context(), user.ID)
	}
	return updated, nil
}

func (a *App) userToSCIM(r *http.Request, user store.User) map[string]any {
	external := ""
	if len(user.Info.String) > 0 {
		var info map[string]any
		_ = json.Unmarshal([]byte(user.Info.String), &info)
		external, _ = info["scim_external_id"].(string)
	}
	resource := map[string]any{"schemas": []string{scimUserSchema}, "id": user.ID, "userName": user.Email, "name": map[string]string{"formatted": user.Name}, "displayName": user.Name, "emails": []map[string]any{{"value": user.Email, "type": "work", "primary": true}}, "active": user.Role != "pending", "meta": map[string]any{"resourceType": "User", "created": epochISO(user.CreatedAt), "lastModified": epochISO(user.UpdatedAt), "location": "/scim/v2/Users/" + user.ID}}
	if external != "" {
		resource["externalId"] = external
	}
	return resource
}
func epochISO(epoch int64) string { return time.Unix(epoch, 0).UTC().Format(time.RFC3339) }
func (a *App) findUserByEmail(r *http.Request, email string) (store.User, error) {
	users, err := a.store.ListUsers(r.Context(), email, 20)
	if err != nil {
		return store.User{}, err
	}
	for _, user := range users {
		if strings.EqualFold(user.Email, email) {
			return user, nil
		}
	}
	return store.User{}, store.ErrUserNotFound
}
func (a *App) findUserByExternalID(r *http.Request, external string) (store.User, error) {
	users, err := a.store.ListUsers(r.Context(), "", 500)
	if err != nil {
		return store.User{}, err
	}
	for _, user := range users {
		var info map[string]any
		_ = json.Unmarshal([]byte(user.Info.String), &info)
		if info["scim_external_id"] == external {
			return user, nil
		}
	}
	return store.User{}, store.ErrUserNotFound
}
func (a *App) setSCIMUserExternalID(r *http.Request, id, external string) error {
	raw, err := a.store.UserInfo(r.Context(), id)
	if err != nil {
		return err
	}
	var info map[string]any
	if json.Unmarshal(raw, &info) != nil || info == nil {
		info = map[string]any{}
	}
	info["scim_external_id"] = external
	encoded, _ := json.Marshal(info)
	_, err = a.store.SetUserInfo(r.Context(), id, encoded)
	return err
}
func scimUserMatches(user store.User, attribute, value string) bool {
	if attribute == "" {
		return true
	}
	if attribute == "userName" || attribute == "emails.value" {
		return strings.EqualFold(user.Email, value)
	}
	if attribute == "externalId" {
		var info map[string]any
		_ = json.Unmarshal([]byte(user.Info.String), &info)
		return info["scim_external_id"] == value
	}
	return false
}

func (a *App) handleSCIMGroups(w http.ResponseWriter, r *http.Request) {
	if !requireSCIM(w, r) {
		return
	}
	w.Header().Set("Content-Type", "application/scim+json")
	if r.Method == http.MethodPost {
		a.createSCIMGroup(w, r)
		return
	}
	resources, err := a.store.ListResources(r.Context(), "group", false)
	if err != nil {
		writeSCIMError(w, http.StatusInternalServerError, "Failed to list groups")
		return
	}
	attribute, value := parseSCIMFilter(r.URL.Query().Get("filter"))
	start := queryInt(r, "startIndex", 1, 1, 1000000)
	count := queryInt(r, "count", 100, 0, 200)
	result := []map[string]any{}
	for _, resource := range resources {
		payload := a.groupToSCIM(r, resource)
		if attribute == "displayName" && payload["displayName"] != value {
			continue
		}
		if attribute == "externalId" && payload["externalId"] != value {
			continue
		}
		if attribute != "" && attribute != "displayName" && attribute != "externalId" {
			continue
		}
		result = append(result, payload)
	}
	total := len(result)
	from := start - 1
	if from > len(result) {
		from = len(result)
	}
	to := from + count
	if to > len(result) {
		to = len(result)
	}
	payload := scimList(result[from:to], start, count)
	payload["totalResults"] = total
	writeJSON(w, http.StatusOK, payload)
}

func (a *App) createSCIMGroup(w http.ResponseWriter, r *http.Request) {
	var body map[string]any
	if !decodeJSON(w, r, &body) {
		return
	}
	name := stringField(body, "displayName")
	if name == "" {
		writeSCIMError(w, http.StatusBadRequest, "displayName is required")
		return
	}
	external := stringField(body, "externalId")
	resources, _ := a.store.ListResources(r.Context(), "group", false)
	for _, resource := range resources {
		payload := a.groupToSCIM(r, resource)
		if external != "" && payload["externalId"] == external {
			writeSCIMError(w, http.StatusConflict, "externalId '"+external+"' already in use")
			return
		}
	}
	id := auth.RandomIDForInternalUse()
	data := map[string]any{"id": id, "user_id": "scim", "name": name, "description": "SCIM provisioned group", "user_ids": a.validSCIMMembers(r, body["members"]), "data": map[string]any{"scim_external_id": external}}
	encoded, _ := json.Marshal(data)
	resource, err := a.store.PutResource(r.Context(), store.Resource{Kind: "group", ID: id, UserID: "scim", Key: id, Body: encoded, Active: true})
	if err != nil {
		writeSCIMError(w, http.StatusInternalServerError, "Failed to create group")
		return
	}
	w.WriteHeader(http.StatusCreated)
	_, _ = w.Write(mustJSON(a.groupToSCIM(r, resource)))
}

func (a *App) handleSCIMGroup(w http.ResponseWriter, r *http.Request) {
	if !requireSCIM(w, r) {
		return
	}
	w.Header().Set("Content-Type", "application/scim+json")
	resource, err := a.store.ResourceByID(r.Context(), "group", r.PathValue("id"))
	if errors.Is(err, store.ErrResourceNotFound) {
		writeSCIMError(w, http.StatusNotFound, "Group not found")
		return
	}
	if err != nil {
		writeSCIMError(w, http.StatusInternalServerError, "Failed to load group")
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, a.groupToSCIM(r, resource))
	case http.MethodDelete:
		if err := a.store.DeleteResource(r.Context(), "group", resource.ID); err != nil {
			writeSCIMError(w, http.StatusInternalServerError, "Failed to delete group")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	case http.MethodPut, http.MethodPatch:
		updated, err := a.updateSCIMGroup(r, resource)
		if err != nil {
			writeSCIMError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, a.groupToSCIM(r, updated))
	}
}

func (a *App) updateSCIMGroup(r *http.Request, resource store.Resource) (store.Resource, error) {
	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return store.Resource{}, err
	}
	var current map[string]any
	_ = json.Unmarshal(resource.Body, &current)
	if r.Method == http.MethodPut {
		if name := stringField(body, "displayName"); name != "" {
			current["name"] = name
		}
		current["user_ids"] = a.validSCIMMembers(r, body["members"])
		setGroupExternal(current, stringField(body, "externalId"))
	} else {
		operations, _ := body["Operations"].([]any)
		if operations == nil {
			operations, _ = body["operations"].([]any)
		}
		members := stringSlice(current["user_ids"])
		for _, raw := range operations {
			op, _ := raw.(map[string]any)
			kind := strings.ToLower(stringField(op, "op"))
			path := stringField(op, "path")
			value := op["value"]
			if kind == "replace" && path == "displayName" {
				if name, ok := value.(string); ok {
					current["name"] = name
				}
			}
			if kind == "replace" && path == "externalId" {
				if external, ok := value.(string); ok {
					setGroupExternal(current, external)
				}
			}
			if path == "members" {
				ids := a.validSCIMMembers(r, value)
				if kind == "replace" {
					members = ids
				}
				if kind == "add" {
					members = appendUnique(members, ids...)
				}
				if kind == "remove" {
					members = removeStrings(members, ids)
				}
			}
			if kind == "remove" && strings.HasPrefix(path, "members[value eq") {
				if _, after, ok := strings.Cut(path, `"`); ok {
					if id, _, ok := strings.Cut(after, `"`); ok {
						members = removeStrings(members, []string{id})
					}
				}
			}
		}
		current["user_ids"] = members
	}
	encoded, _ := json.Marshal(current)
	resource.Body = encoded
	return a.store.PutResource(r.Context(), resource)
}

func setGroupExternal(body map[string]any, external string) {
	data, _ := body["data"].(map[string]any)
	if data == nil {
		data = map[string]any{}
	}
	if external != "" {
		data["scim_external_id"] = external
	}
	body["data"] = data
}
func (a *App) groupToSCIM(r *http.Request, resource store.Resource) map[string]any {
	var body map[string]any
	_ = json.Unmarshal(resource.Body, &body)
	members := []map[string]string{}
	for _, id := range stringSlice(body["user_ids"]) {
		if user, err := a.store.UserByID(r.Context(), id); err == nil {
			members = append(members, map[string]string{"value": id, "display": user.Name})
		}
	}
	payload := map[string]any{"schemas": []string{scimGroupSchema}, "id": resource.ID, "displayName": stringField(body, "name"), "members": members, "meta": map[string]any{"resourceType": "Group", "created": epochISO(resource.CreatedAt), "lastModified": epochISO(resource.UpdatedAt), "location": "/scim/v2/Groups/" + resource.ID}}
	if data, ok := body["data"].(map[string]any); ok {
		if external, _ := data["scim_external_id"].(string); external != "" {
			payload["externalId"] = external
		}
	}
	return payload
}
func (a *App) validSCIMMembers(r *http.Request, value any) []string {
	raw, ok := value.([]any)
	if !ok {
		return []string{}
	}
	result := []string{}
	for _, item := range raw {
		entry, _ := item.(map[string]any)
		id := stringField(entry, "value")
		if id == "" {
			continue
		}
		if _, err := a.store.UserByID(r.Context(), id); err == nil {
			result = appendUnique(result, id)
		}
	}
	return result
}
func appendUnique(values []string, more ...string) []string {
	seen := map[string]bool{}
	for _, value := range values {
		seen[value] = true
	}
	for _, value := range more {
		if value != "" && !seen[value] {
			values = append(values, value)
			seen[value] = true
		}
	}
	return values
}
func removeStrings(values, remove []string) []string {
	blocked := map[string]bool{}
	for _, value := range remove {
		blocked[value] = true
	}
	result := values[:0]
	for _, value := range values {
		if !blocked[value] {
			result = append(result, value)
		}
	}
	return result
}
func parseSCIMFilter(value string) (string, string) {
	parts := strings.SplitN(strings.TrimSpace(value), " ", 3)
	if len(parts) != 3 || !strings.EqualFold(parts[1], "eq") {
		return "", ""
	}
	return parts[0], strings.Trim(strings.TrimSpace(parts[2]), `"'`)
}
