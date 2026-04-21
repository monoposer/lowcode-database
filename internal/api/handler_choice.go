package api

import (
	"net/http"
	"strings"

	"github.com/solat/lowcode-database/internal/apiv1"
)

func (h *handler) handleListChoices() {
	resp, err := h.svc.ListChoices(h.r.Context(), &apiv1.ListChoicesRequest{})
	h.writeJSON(resp, err)
}

func (h *handler) handleCreateChoice() {
	var req apiv1.CreateChoiceRequest
	if !h.readJSON(&req) {
		return
	}
	resp, err := h.svc.CreateChoice(h.r.Context(), &req)
	h.writeJSON(resp, err)
}

func (h *handler) handleChoicesSubtree() {
	id := strings.TrimPrefix(h.r.URL.Path, "/v1/choices/")
	if id == "" {
		http.NotFound(h.w, h.r)
		return
	}
	switch h.r.Method {
	case http.MethodGet:
		resp, err := h.svc.GetChoice(h.r.Context(), &apiv1.GetChoiceRequest{Id: id})
		h.writeJSON(resp, err)
	case http.MethodPatch:
		var body apiv1.UpdateChoiceRequest
		if !h.readJSON(&body) {
			return
		}
		body.Id = id
		resp, err := h.svc.UpdateChoice(h.r.Context(), &body)
		h.writeJSON(resp, err)
	case http.MethodDelete:
		resp, err := h.svc.DeleteChoice(h.r.Context(), &apiv1.DeleteChoiceRequest{Id: id})
		h.writeJSON(resp, err)
	default:
		http.NotFound(h.w, h.r)
	}
}
