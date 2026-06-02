package api

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/solat/lowcode-database/internal/service"
	"github.com/solat/lowcode-database/internal/tenant"
)

// NewHandler returns an http.Handler for all /v1/* JSON APIs.
func NewHandler(svc *service.LowcodeService) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/v1/") {
			http.NotFound(w, r)
			return
		}
		ctx := tenant.WithTenantID(r.Context(), r.Header.Get("X-Tenant-Id"))
		h := &handler{svc: svc, w: w, r: r.WithContext(ctx)}
		h.dispatch()
	})
}

type handler struct {
	svc *service.LowcodeService
	w   http.ResponseWriter
	r   *http.Request
}

func (h *handler) dispatch() {
	switch {
	// --- platform ---
	case h.match("GET", "/v1/database/connection"):
		h.handleGetDatabaseConnection()
	case h.match("POST", "/v1/tenants"):
		h.handleCreateTenant()

	// --- types (built-in, internal/columntype) ---
	case h.match("GET", "/v1/types"):
		h.handleListTypes()

	// --- webhooks ---
	case h.match("GET", "/v1/webhooks"):
		h.handleListWebhooks()
	case h.match("POST", "/v1/webhooks"):
		h.handleCreateWebhook()
	case h.match("DELETE", "/v1/webhooks/"):
		h.handleDeleteWebhook()
	case h.match("PATCH", "/v1/webhooks/"):
		h.handleUpdateWebhook()

	// --- tables (schema / rows) ---
	case h.match("GET", "/v1/tables") && h.r.URL.Path == "/v1/tables":
		h.handleListTables()
	case h.match("POST", "/v1/tables") && h.r.URL.Path == "/v1/tables":
		h.handleCreateTable()
	case strings.HasPrefix(h.r.URL.Path, "/v1/tables/"):
		h.handleTablesSubtree()

	// --- columns ---
	case h.match("GET", "/v1/columns") && h.r.URL.Path == "/v1/columns":
		h.handleListColumns()
	case h.match("POST", "/v1/columns") && h.r.URL.Path == "/v1/columns":
		h.handleCreateColumn()
	case strings.HasPrefix(h.r.URL.Path, "/v1/columns/"):
		h.handleColumnsSubtree()

	// --- indexes (PostgreSQL catalog) ---
	case h.match("GET", "/v1/indexes") && h.r.URL.Path == "/v1/indexes":
		h.handleListIndexes()
	case h.match("POST", "/v1/indexes") && h.r.URL.Path == "/v1/indexes":
		h.handleCreateIndex()
	case strings.HasPrefix(h.r.URL.Path, "/v1/indexes/"):
		h.handleIndexesSubtree()

	// --- schema ---
	case h.match("GET", "/v1/schema/er"):
		h.handleGetERDiagram()

	// --- choices ---
	case h.match("GET", "/v1/choices"):
		h.handleListChoices()
	case h.match("POST", "/v1/choices"):
		h.handleCreateChoice()
	case strings.HasPrefix(h.r.URL.Path, "/v1/choices/"):
		h.handleChoicesSubtree()

	// --- relations ---
	case h.match("GET", "/v1/relations"):
		h.handleListRelations()
	case h.match("POST", "/v1/relations"):
		h.handleCreateRelation()
	case h.match("DELETE", "/v1/relations/"):
		h.handleDeleteRelation()

	// --- data sources (list / view definition + query) ---
	case strings.HasPrefix(h.r.URL.Path, "/v1/data-sources/"):
		h.handleDataSourcesSubtree()
	case h.match("GET", "/v1/data-sources") && h.r.URL.Path == "/v1/data-sources":
		h.handleListDataSources()
	case h.match("POST", "/v1/data-sources") && h.r.URL.Path == "/v1/data-sources":
		h.handleCreateDataSource()

	default:
		http.NotFound(h.w, h.r)
	}
}

func (h *handler) match(method, prefix string) bool {
	return h.r.Method == method && strings.HasPrefix(h.r.URL.Path, prefix)
}

func (h *handler) pathID(prefix string) (string, bool) {
	id := strings.TrimPrefix(h.r.URL.Path, prefix)
	if id == "" {
		http.Error(h.w, "id required", http.StatusBadRequest)
		return "", false
	}
	return id, true
}

func (h *handler) readJSON(dst any) bool {
	if h.r.Body == nil {
		return true
	}
	defer h.r.Body.Close()
	dec := json.NewDecoder(h.r.Body)
	dec.UseNumber()
	if err := dec.Decode(dst); err != nil {
		if err == io.EOF {
			return true
		}
		http.Error(h.w, err.Error(), http.StatusBadRequest)
		return false
	}
	return true
}

func (h *handler) writeJSON(v any, err error) {
	if err != nil {
		h.writeErr(err)
		return
	}
	b, err := json.Marshal(v)
	if err != nil {
		http.Error(h.w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.w.Header().Set("Content-Type", "application/json")
	_, _ = h.w.Write(b)
}

func (h *handler) writeErr(err error) {
	msg := err.Error()
	code := http.StatusBadRequest
	if strings.Contains(strings.ToLower(msg), "not found") {
		code = http.StatusNotFound
	}
	http.Error(h.w, msg, code)
}
