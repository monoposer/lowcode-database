package authz

import (
	"net/http/httptest"
	"testing"
)

func TestRequestFromHTTPDataQuery(t *testing.T) {
	req := httptest.NewRequest("POST", "/v1/data/tables/orders/rows:query", nil)
	req.Header.Set("X-User-Sub", "u1")
	req.Header.Set("X-User-Roles", "viewer, editor")

	authReq, ok := RequestFromHTTP(req, DefaultSubjectHeaders())
	if !ok {
		t.Fatal("expected ok")
	}
	if authReq.User.Sub != "u1" {
		t.Fatalf("sub=%q", authReq.User.Sub)
	}
	if len(authReq.User.Roles) != 2 {
		t.Fatalf("roles=%v", authReq.User.Roles)
	}
	if authReq.Action != ActionSelect {
		t.Fatalf("action=%q", authReq.Action)
	}
	if authReq.Resource.Type != "db" || authReq.Resource.Table != "orders" {
		t.Fatalf("resource=%+v", authReq.Resource)
	}
}

func TestRequestFromHTTPAdmin(t *testing.T) {
	req := httptest.NewRequest("POST", "/v1/admin/tables", nil)
	authReq, ok := RequestFromHTTP(req, DefaultSubjectHeaders())
	if !ok {
		t.Fatal("expected ok")
	}
	if authReq.Resource.Type != "schema" || authReq.Action != ActionInsert {
		t.Fatalf("got %+v", authReq)
	}
}
