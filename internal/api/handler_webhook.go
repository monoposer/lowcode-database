package api

import "github.com/solat/lowcode-database/internal/apiv1"

func (h *handler) handleListWebhooks() {
	resp, err := h.svc.ListWebhooks(h.r.Context(), &apiv1.ListWebhooksRequest{})
	h.writeJSON(resp, err)
}

func (h *handler) handleCreateWebhook() {
	var req apiv1.CreateWebhookRequest
	if !h.readJSON(&req) {
		return
	}
	resp, err := h.svc.CreateWebhook(h.r.Context(), &req)
	h.writeJSON(resp, err)
}

func (h *handler) handleDeleteWebhook() {
	id, ok := h.pathID("/v1/webhooks/")
	if !ok {
		return
	}
	resp, err := h.svc.DeleteWebhook(h.r.Context(), &apiv1.DeleteWebhookRequest{Id: id})
	h.writeJSON(resp, err)
}

func (h *handler) handleUpdateWebhook() {
	id, ok := h.pathID("/v1/webhooks/")
	if !ok {
		return
	}
	var body apiv1.UpdateWebhookRequest
	if !h.readJSON(&body) {
		return
	}
	body.Id = id
	resp, err := h.svc.UpdateWebhook(h.r.Context(), &body)
	h.writeJSON(resp, err)
}
