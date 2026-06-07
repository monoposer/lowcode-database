package httputil

import (
	"net/http"
	"strings"
)

const ConfirmDangerousHeader = "X-Confirm-Dangerous"

func (b *Base) RequireDangerousConfirm(w http.ResponseWriter, r *http.Request, resourceID string) bool {
	if b == nil || b.Svc == nil || b.Svc.Schema == nil || b.Svc.Schema.B == nil || !b.Svc.Schema.B.DDLConfirmRequired {
		return true
	}
	got := strings.TrimSpace(r.Header.Get(ConfirmDangerousHeader))
	if got == "" {
		got = strings.TrimSpace(r.URL.Query().Get("confirm"))
	}
	if got != resourceID {
		http.Error(w, "dangerous operation requires header X-Confirm-Dangerous matching the resource id", http.StatusConflict)
		return false
	}
	return true
}
