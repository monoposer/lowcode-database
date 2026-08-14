package ratelimit

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/monoposer/lowcode-database/pkg/tenant"
)

func TestGlobalLimit(t *testing.T) {
	l := New(1, 0)
	h := l.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	ok := httptest.NewRecorder()
	h.ServeHTTP(ok, httptest.NewRequest(http.MethodGet, "/", nil))
	if ok.Code != http.StatusOK {
		t.Fatalf("first: %d", ok.Code)
	}
	blocked := httptest.NewRecorder()
	h.ServeHTTP(blocked, httptest.NewRequest(http.MethodGet, "/", nil))
	if blocked.Code != http.StatusTooManyRequests {
		t.Fatalf("second: %d", blocked.Code)
	}
}

func TestTenantLimit(t *testing.T) {
	l := New(0, 1)
	h := l.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(tenant.WithTenantID(req.Context(), "t1"))
	ok := httptest.NewRecorder()
	h.ServeHTTP(ok, req)
	if ok.Code != http.StatusOK {
		t.Fatalf("first: %d", ok.Code)
	}
	blocked := httptest.NewRecorder()
	h.ServeHTTP(blocked, req)
	if blocked.Code != http.StatusTooManyRequests {
		t.Fatalf("second: %d", blocked.Code)
	}
}
