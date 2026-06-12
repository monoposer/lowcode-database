package authz

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

type stubAuthorizer struct {
	allow bool
	calls int
}

func (s *stubAuthorizer) Allow(_ context.Context, _ Request) (bool, error) {
	s.calls++
	return s.allow, nil
}

func TestMiddlewareAllow(t *testing.T) {
	stub := &stubAuthorizer{allow: true}
	mw := NewMiddleware(stub, true, DefaultSubjectHeaders())
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := SubjectFromContext(r.Context()); !ok {
			t.Fatal("subject missing from context")
		}
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/data/tables/orders/rows", nil)
	req.Header.Set("X-User-Sub", "u1")
	req.Header.Set("X-User-Roles", "viewer")
	w := httptest.NewRecorder()
	mw.Handler(next).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if stub.calls != 1 {
		t.Fatalf("calls=%d", stub.calls)
	}
}

func TestMiddlewareForbidden(t *testing.T) {
	stub := &stubAuthorizer{allow: false}
	mw := NewMiddleware(stub, true, DefaultSubjectHeaders())
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/data/tables/orders/rows", nil)
	req.Header.Set("X-User-Sub", "u1")
	req.Header.Set("X-User-Roles", "viewer")
	w := httptest.NewRecorder()
	mw.Handler(next).ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("status=%d", w.Code)
	}
}

func TestMiddlewareOptionalIdentity(t *testing.T) {
	stub := &stubAuthorizer{allow: true}
	mw := NewMiddleware(stub, false, DefaultSubjectHeaders())
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/admin/tables", nil)
	w := httptest.NewRecorder()
	mw.Handler(next).ServeHTTP(w, req)

	if w.Code != http.StatusOK || !called {
		t.Fatalf("expected pass-through, status=%d called=%v calls=%d", w.Code, called, stub.calls)
	}
	if stub.calls != 0 {
		t.Fatalf("authorizer should not run without identity")
	}
}
