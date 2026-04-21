package api

import "github.com/solat/lowcode-database/internal/apiv1"

func (h *handler) handleGetERDiagram() {
	resp, err := h.svc.GetERDiagram(h.r.Context(), &apiv1.GetERDiagramRequest{})
	h.writeJSON(resp, err)
}
