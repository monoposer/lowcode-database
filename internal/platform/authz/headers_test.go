package authz

import (
	"net/http/httptest"
	"testing"

	"github.com/monoposer/lowcode-database/internal/config"
)

func TestSubjectFromHTTPLegacyRoleHeader(t *testing.T) {
	req := httptest.NewRequest("GET", "/v1/admin/tables", nil)
	req.Header.Set("X-User-Sub", "u1")
	req.Header.Set("X-User-Role", "editor")

	sub := subjectFromHTTP(req, DefaultSubjectHeaders())
	if sub.Sub != "u1" {
		t.Fatalf("sub=%q", sub.Sub)
	}
	if len(sub.Roles) != 1 || sub.Roles[0] != "editor" {
		t.Fatalf("roles=%v", sub.Roles)
	}
}

func TestSubjectFromHTTPRolesHeaderWinsOverLegacy(t *testing.T) {
	req := httptest.NewRequest("GET", "/v1/admin/tables", nil)
	req.Header.Set("X-User-Roles", "viewer")
	req.Header.Set("X-User-Role", "editor")

	sub := subjectFromHTTP(req, DefaultSubjectHeaders())
	if len(sub.Roles) != 1 || sub.Roles[0] != "viewer" {
		t.Fatalf("roles=%v", sub.Roles)
	}
}

func TestSubjectHeadersFromConfigCustomList(t *testing.T) {
	h := SubjectHeadersFromConfig(&config.Config{
		AuthzUserSubHeader:    "X-Subject",
		AuthzUserRolesHeaders: []string{"X-Roles", "X-Role"},
	})
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Subject", "u2")
	req.Header.Set("X-Role", "admin")

	sub := subjectFromHTTP(req, h)
	if sub.Sub != "u2" || len(sub.Roles) != 1 || sub.Roles[0] != "admin" {
		t.Fatalf("sub=%+v", sub)
	}
}

func TestRequestFromHTTPLegacyRoleHeader(t *testing.T) {
	req := httptest.NewRequest("POST", "/v1/data/tables/orders/rows:query", nil)
	req.Header.Set("X-User-Sub", "u1")
	req.Header.Set("X-User-Role", "viewer")

	authReq, ok := RequestFromHTTP(req, DefaultSubjectHeaders())
	if !ok {
		t.Fatal("expected ok")
	}
	if authReq.User.Roles[0] != "viewer" {
		t.Fatalf("roles=%v", authReq.User.Roles)
	}
}
