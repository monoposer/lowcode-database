package authz

import (
	"net/http"
	"strings"
)

// -------- HTTP entry --------
// -------- HTTP entry --------

const (
	adminPrefix = "/v1/admin"
	dataPrefix  = "/v1/data"
)

const (
	userSubHeader   = "X-User-Sub"
	userRolesHeader = "X-User-Roles"
)

// RequestFromHTTP maps an incoming API request to an authorization Request.
// Returns ok=false for paths that skip authorization (public / unknown).
func RequestFromHTTP(r *http.Request) (Request, bool) {
	path := r.URL.Path
	if !strings.HasPrefix(path, adminPrefix) && !strings.HasPrefix(path, dataPrefix) {
		return Request{}, false
	}

	subject := Subject{
		Sub:   strings.TrimSpace(r.Header.Get(userSubHeader)),
		Roles: ParseRoles(r.Header.Get(userRolesHeader)),
	}

	method := r.Method
	switch {
	case strings.HasPrefix(path, adminPrefix):
		return adminPathAuth(subject, method, path)
	case strings.HasPrefix(path, dataPrefix):
		return dataPathAuth(subject, method, path, r.URL.Query().Get("table_id"))
	}
	return Request{}, false
}

// -------- Helpers --------

func schemaReq(subject Subject, method string) Request {
	switch method {
	case http.MethodGet:
		return req(subject, ActionSelect, "schema", "")
	case http.MethodPost:
		return req(subject, ActionInsert, "schema", "")
	case http.MethodPatch, http.MethodPut:
		return req(subject, ActionUpdate, "schema", "")
	case http.MethodDelete:
		return req(subject, ActionDelete, "schema", "")
	default:
		return req(subject, ActionSelect, "schema", "")
	}
}

func platformReq(subject Subject, method, _ string) Request {
	switch method {
	case http.MethodGet:
		return req(subject, ActionSelect, "platform", "")
	case http.MethodPost:
		return req(subject, ActionInsert, "platform", "")
	case http.MethodPatch, http.MethodPut:
		return req(subject, ActionUpdate, "platform", "")
	case http.MethodDelete:
		return req(subject, ActionDelete, "platform", "")
	default:
		return req(subject, ActionSelect, "platform", "")
	}
}

func dbReq(subject Subject, action Action, table string) Request {
	return req(subject, action, "db", table)
}

func req(subject Subject, action Action, resType, table string) Request {
	return Request{
		User:   subject,
		Action: action,
		Resource: Resource{
			Type:   resType,
			Schema: "public",
			Table:  table,
		},
	}
}

// -------- Admin paths --------

func adminPathAuth(subject Subject, method, path string) (Request, bool) {
	switch {
	case path == adminPrefix+"/database/connection" && method == http.MethodGet:
		return req(subject, ActionSelect, "platform", ""), true
	case path == adminPrefix+"/tenants" && method == http.MethodPost:
		return req(subject, ActionInsert, "platform", ""), true
	case path == adminPrefix+"/api-keys" || strings.HasPrefix(path, adminPrefix+"/api-keys/"):
		return platformReq(subject, method, "api-keys"), true
	case path == adminPrefix+"/event-sinks" || strings.HasPrefix(path, adminPrefix+"/event-sinks/"):
		return platformReq(subject, method, "event-sinks"), true
	case path == adminPrefix+"/types" && method == http.MethodGet:
		return req(subject, ActionSelect, "schema", ""), true
	case path == adminPrefix+"/tables" && method == http.MethodGet:
		return req(subject, ActionSelect, "schema", ""), true
	case path == adminPrefix+"/tables" && method == http.MethodPost:
		return req(subject, ActionInsert, "schema", ""), true
	case strings.HasPrefix(path, adminPrefix+"/tables/"):
		return adminTablePathAuth(subject, method, path)
	case path == adminPrefix+"/columns" || strings.HasPrefix(path, adminPrefix+"/columns/"):
		return schemaReq(subject, method), true
	case path == adminPrefix+"/indexes" || strings.HasPrefix(path, adminPrefix+"/indexes/"):
		return schemaReq(subject, method), true
	case path == adminPrefix+"/schema/er" && method == http.MethodGet:
		return req(subject, ActionSelect, "schema", ""), true
	case path == adminPrefix+"/choices" || strings.HasPrefix(path, adminPrefix+"/choices/"):
		return schemaReq(subject, method), true
	case path == adminPrefix+"/relations" || strings.HasPrefix(path, adminPrefix+"/relations/"):
		return schemaReq(subject, method), true
	case path == adminPrefix+"/data-sources" || strings.HasPrefix(path, adminPrefix+"/data-sources/"):
		return schemaReq(subject, method), true
	}
	return Request{}, false
}

func adminTablePathAuth(subject Subject, method, path string) (Request, bool) {
	rest := strings.TrimPrefix(path, adminPrefix+"/tables/")
	if rest == "" {
		return Request{}, false
	}
	if method == http.MethodPost && strings.HasSuffix(rest, ":rename") {
		return req(subject, ActionUpdate, "schema", ""), true
	}
	if method == http.MethodDelete && !strings.Contains(rest, "/") {
		return req(subject, ActionDelete, "schema", ""), true
	}
	parts := strings.SplitN(rest, "/", 2)
	if len(parts) == 2 && parts[1] == "schema" && method == http.MethodGet {
		return dbReq(subject, ActionSelect, parts[0]), true
	}
	return Request{}, false
}

// -------- Data paths --------

func dataPathAuth(subject Subject, method, path, tableID string) (Request, bool) {
	switch {
	case strings.HasPrefix(path, dataPrefix+"/tables/"):
		return dataTablePathAuth(subject, method, path)
	case strings.HasPrefix(path, dataPrefix+"/data-sources/"):
		if method == http.MethodPost && strings.Contains(path, ":query") {
			return dbReq(subject, ActionSelect, tableID), true
		}
	}
	return Request{}, false
}

func dataTablePathAuth(subject Subject, method, path string) (Request, bool) {
	rest := strings.TrimPrefix(path, dataPrefix+"/tables/")
	if rest == "" {
		return Request{}, false
	}
	parts := strings.SplitN(rest, "/", 2)
	tableID := parts[0]
	if tableID == "" || len(parts) < 2 {
		return Request{}, false
	}
	tail := parts[1]

	switch {
	case tail == "rows:query", tail == "rows:export", tail == "rows:search":
		return dbReq(subject, ActionSelect, tableID), true
	case tail == "rows" && method == http.MethodGet:
		return dbReq(subject, ActionSelect, tableID), true
	case tail == "rows" && method == http.MethodPost:
		return dbReq(subject, ActionInsert, tableID), true
	case tail == "rows:bulkUpsert", tail == "rows:saveGraph", tail == "rows:import":
		return dbReq(subject, ActionInsert, tableID), true
	case tail == "rows:bulkDelete" && method == http.MethodPost:
		return dbReq(subject, ActionDelete, tableID), true
	case strings.HasPrefix(tail, "rows/"):
		rowTail := strings.TrimPrefix(tail, "rows/")
		if strings.Contains(rowTail, "/") {
			return Request{}, false
		}
		switch method {
		case http.MethodPatch:
			return dbReq(subject, ActionUpdate, tableID), true
		case http.MethodDelete:
			return dbReq(subject, ActionDelete, tableID), true
		}
	}
	return Request{}, false
}
