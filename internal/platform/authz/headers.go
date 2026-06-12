package authz

import (
	"net/http"
	"strings"

	"github.com/solat/lowcode-database/internal/config"
)

const (
	defaultUserSubHeader = "X-User-Sub"
)

// DefaultRolesHeaders is the built-in role header precedence (plural first, singular legacy).
var DefaultRolesHeaders = []string{"X-User-Roles", "X-User-Role"}

// SubjectHeaders names HTTP headers that carry the authenticated subject for RBAC.
type SubjectHeaders struct {
	SubHeader    string
	RolesHeaders []string
}

// SubjectHeadersFromConfig builds header names from service config.
func SubjectHeadersFromConfig(cfg *config.Config) SubjectHeaders {
	if cfg == nil {
		return DefaultSubjectHeaders()
	}
	h := SubjectHeaders{
		SubHeader:    strings.TrimSpace(cfg.AuthzUserSubHeader),
		RolesHeaders: append([]string(nil), cfg.AuthzUserRolesHeaders...),
	}
	if h.SubHeader == "" {
		h.SubHeader = defaultUserSubHeader
	}
	if len(h.RolesHeaders) == 0 {
		h.RolesHeaders = append([]string(nil), DefaultRolesHeaders...)
	}
	return h
}

// DefaultSubjectHeaders returns the default subject header mapping.
func DefaultSubjectHeaders() SubjectHeaders {
	return SubjectHeaders{
		SubHeader:    defaultUserSubHeader,
		RolesHeaders: append([]string(nil), DefaultRolesHeaders...),
	}
}

// IdentityHint describes which headers satisfy AuthzRequired (for error messages).
func (h SubjectHeaders) IdentityHint() string {
	if len(h.RolesHeaders) == 0 {
		return h.SubHeader
	}
	return h.SubHeader + " / " + strings.Join(h.RolesHeaders, " / ")
}

func subjectFromHTTP(r *http.Request, h SubjectHeaders) Subject {
	sub := strings.TrimSpace(r.Header.Get(h.SubHeader))
	var roles []string
	for _, name := range h.RolesHeaders {
		if raw := strings.TrimSpace(r.Header.Get(name)); raw != "" {
			roles = ParseRoles(raw)
			break
		}
	}
	return Subject{Sub: sub, Roles: roles}
}
