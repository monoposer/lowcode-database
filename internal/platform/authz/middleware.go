package authz

import (
	"context"
	"fmt"
	"net/http"
)

type ctxKey struct{}

// Middleware enforces authorization for /v1/* API routes.
type Middleware struct {
	auth     Authorizer
	required bool
	headers  SubjectHeaders
}

func NewMiddleware(auth Authorizer, required bool, headers SubjectHeaders) *Middleware {
	if headers.SubHeader == "" && len(headers.RolesHeaders) == 0 {
		headers = DefaultSubjectHeaders()
	}
	return &Middleware{auth: auth, required: required, headers: headers}
}

func (m *Middleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if m == nil || m.auth == nil {
			next.ServeHTTP(w, r)
			return
		}
		if isPublicPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		authReq, ok := RequestFromHTTP(r, m.headers)
		if !ok {
			next.ServeHTTP(w, r)
			return
		}

		if authReq.User.Sub == "" && len(authReq.User.Roles) == 0 {
			if m.required {
				http.Error(w, fmt.Sprintf("user identity required (%s)", m.headers.IdentityHint()), http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
			return
		}

		allow, err := m.auth.Allow(r.Context(), authReq)
		if err != nil {
			http.Error(w, "authorization failed", http.StatusBadGateway)
			return
		}
		if !allow {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}

		ctx := WithSubject(r.Context(), authReq.User)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func isPublicPath(path string) bool {
	switch {
	case path == adminPrefix+"/events/schemas", path == adminPrefix+"/events/envelope-schema":
		return true
	default:
		return false
	}
}

// WithSubject stores the authorized subject on the request context.
func WithSubject(ctx context.Context, sub Subject) context.Context {
	return context.WithValue(ctx, ctxKey{}, sub)
}

// SubjectFromContext returns the subject set by authz middleware.
func SubjectFromContext(ctx context.Context) (Subject, bool) {
	v, ok := ctx.Value(ctxKey{}).(Subject)
	return v, ok
}
