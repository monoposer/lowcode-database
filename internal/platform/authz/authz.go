package authz

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/monoposer/lowcode-database/internal/config"
	"io"
	"net/http"
	"strings"
	"time"
)

// Action is a data-level CRUD verb (compatible with lowcode-role).
type Action string

const (
	ActionSelect Action = "select"
	ActionInsert Action = "insert"
	ActionUpdate Action = "update"
	ActionDelete Action = "delete"
)

// Subject is the caller identity passed in by upstream (gateway / BFF).
type Subject struct {
	Sub   string
	Roles []string
}

// Resource describes what is being accessed.
type Resource struct {
	// Type is "db" for row/table data, "schema" for DDL/metadata, "platform" for admin APIs.
	Type   string
	Schema string
	Table  string
}

// Request is the authorization input for one API operation.
type Request struct {
	User     Subject
	Action   Action
	Resource Resource
}

// Authorizer decides whether a subject may perform an operation.
type Authorizer interface {
	Allow(ctx context.Context, req Request) (bool, error)
}

// NewFromConfig builds the configured authorizer (noop when driver is empty/disabled).
func NewFromConfig(cfg *config.Config) (Authorizer, error) {
	driver := strings.ToLower(strings.TrimSpace(cfg.AuthzDriver))
	switch driver {
	case "", "noop", "none", "off":
		return Noop{}, nil
	case "file":
		return NewFileAuthorizer(cfg.AuthzFile)
	case "http":
		return NewHTTPAuthorizer(cfg.AuthzHTTPURL)
	default:
		return nil, fmt.Errorf("unknown AUTHZ_DRIVER %q (want noop|file|http)", cfg.AuthzDriver)
	}
}

// Noop allows every request.
type Noop struct{}

func (Noop) Allow(context.Context, Request) (bool, error) { return true, nil }

// ParseRoles splits a comma-separated role header value.
func ParseRoles(raw string) []string {
	if raw == "" {
		return nil
	}
	var roles []string
	for _, part := range strings.Split(raw, ",") {
		if r := strings.TrimSpace(part); r != "" {
			roles = append(roles, r)
		}
	}
	return roles
}

// HTTPAuthorizer delegates allow/deny to an external authorize endpoint
// (e.g. lowcode-role POST /v1/authorize).
type HTTPAuthorizer struct {
	url    string
	client *http.Client
}

func NewHTTPAuthorizer(url string) (*HTTPAuthorizer, error) {
	url = strings.TrimSpace(url)
	if url == "" {
		return nil, fmt.Errorf("AUTHZ_HTTP_URL is required when AUTHZ_DRIVER=http")
	}
	return &HTTPAuthorizer{
		url: url,
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
	}, nil
}

type httpAuthorizeBody struct {
	User struct {
		Sub   string   `json:"sub"`
		Roles []string `json:"roles"`
	} `json:"user"`
	Request struct {
		Action   string `json:"action"`
		Resource struct {
			Type   string         `json:"type"`
			Schema string         `json:"schema"`
			Table  string         `json:"table"`
			Row    map[string]any `json:"row,omitempty"`
			Fields []string       `json:"fields,omitempty"`
		} `json:"resource"`
	} `json:"request"`
}

type httpAuthorizeResp struct {
	Allow bool `json:"allow"`
}

func (a *HTTPAuthorizer) Allow(ctx context.Context, req Request) (bool, error) {
	body := httpAuthorizeBody{}
	body.User.Sub = req.User.Sub
	body.User.Roles = req.User.Roles
	body.Request.Action = string(req.Action)
	body.Request.Resource.Type = req.Resource.Type
	body.Request.Resource.Schema = req.Resource.Schema
	body.Request.Resource.Table = req.Resource.Table
	if body.Request.Resource.Schema == "" {
		body.Request.Resource.Schema = "public"
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return false, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, a.url, bytes.NewReader(payload))
	if err != nil {
		return false, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := a.client.Do(httpReq)
	if err != nil {
		return false, fmt.Errorf("authz http: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return false, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return false, fmt.Errorf("authz http status %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}

	var out httpAuthorizeResp
	if err := json.Unmarshal(raw, &out); err != nil {
		return false, fmt.Errorf("authz http decode: %w", err)
	}
	return out.Allow, nil
}
