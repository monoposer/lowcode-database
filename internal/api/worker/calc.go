package worker

import (
	"net/http"

	"github.com/monoposer/lowcode-database/internal/api/httputil"
	"github.com/monoposer/lowcode-database/internal/service/calc"
)

type Calc struct {
	*httputil.Base
}

type drainRequest struct {
	Batch int `json:"batch,omitempty"`
}

type drainResponse struct {
	Processed int `json:"processed"`
}

func (h *Calc) withStore(r *http.Request) (*http.Request, error) {
	ctx, _, err := h.Svc.Schema.B.Tenants.AttachDataTables(r.Context())
	if err != nil {
		return r, err
	}
	return r.WithContext(ctx), nil
}

func (h *Calc) Drain(w http.ResponseWriter, r *http.Request) {
	var body drainRequest
	_ = h.ReadJSON(w, r, &body)
	r, err := h.withStore(r)
	if err != nil {
		h.WriteJSON(w, nil, err)
		return
	}
	pool, err := h.Svc.Schema.B.Tenants.DataPool(r.Context())
	if err != nil {
		h.WriteJSON(w, nil, err)
		return
	}
	batch := body.Batch
	if batch <= 0 {
		batch = 16
	}
	wk := calc.NewWorker(calc.WorkerConfig{Batch: batch})
	if err := wk.Drain(r.Context(), h.Svc.Schema.B.Tenants.MetaPool(), pool); err != nil {
		h.WriteJSON(w, nil, err)
		return
	}
	h.WriteJSON(w, drainResponse{Processed: batch}, nil)
}

type claimRequest struct {
	Batch int `json:"batch,omitempty"`
}

type claimResponse struct {
	Tasks []calc.Task `json:"tasks"`
}

func (h *Calc) Claim(w http.ResponseWriter, r *http.Request) {
	var body claimRequest
	_ = h.ReadJSON(w, r, &body)
	r, err := h.withStore(r)
	if err != nil {
		h.WriteJSON(w, nil, err)
		return
	}
	pool, err := h.Svc.Schema.B.Tenants.DataPool(r.Context())
	if err != nil {
		h.WriteJSON(w, nil, err)
		return
	}
	batch := body.Batch
	if batch <= 0 {
		batch = 16
	}
	tasks, err := calc.Claim(r.Context(), pool, batch)
	h.WriteJSON(w, claimResponse{Tasks: tasks}, err)
}

type ackRequest struct {
	ID    int64  `json:"id"`
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

func (h *Calc) Ack(w http.ResponseWriter, r *http.Request) {
	var body ackRequest
	if !h.ReadJSON(w, r, &body) {
		return
	}
	r, err := h.withStore(r)
	if err != nil {
		h.WriteJSON(w, nil, err)
		return
	}
	pool, err := h.Svc.Schema.B.Tenants.DataPool(r.Context())
	if err != nil {
		h.WriteJSON(w, nil, err)
		return
	}
	if body.OK {
		err = calc.MarkCompleted(r.Context(), pool, body.ID)
	} else {
		err = calc.MarkRetry(r.Context(), pool, calc.Task{ID: body.ID, MaxRetry: 8}, errOr(body.Error))
	}
	h.WriteJSON(w, map[string]any{"ok": true}, err)
}

func errOr(s string) error {
	if s == "" {
		return nil
	}
	return ackErr(s)
}

type replayRequest struct {
	Limit int `json:"limit,omitempty"`
}

func (h *Calc) Replay(w http.ResponseWriter, r *http.Request) {
	var body replayRequest
	_ = h.ReadJSON(w, r, &body)
	r, err := h.withStore(r)
	if err != nil {
		h.WriteJSON(w, nil, err)
		return
	}
	pool, err := h.Svc.Schema.B.Tenants.DataPool(r.Context())
	if err != nil {
		h.WriteJSON(w, nil, err)
		return
	}
	tid := ""
	if h.Svc.Schema != nil && h.Svc.Schema.B != nil {
		tid, _ = h.Svc.Schema.B.TenantID(r.Context())
	}
	n, err := calc.ReplayDeadLetters(r.Context(), pool, tid, body.Limit)
	h.WriteJSON(w, map[string]any{"replayed": n}, err)
}

func (h *Calc) Stats(w http.ResponseWriter, r *http.Request) {
	r, err := h.withStore(r)
	if err != nil {
		h.WriteJSON(w, nil, err)
		return
	}
	pool, err := h.Svc.Schema.B.Tenants.DataPool(r.Context())
	if err != nil {
		h.WriteJSON(w, nil, err)
		return
	}
	alert := 0
	if h.Svc.Schema != nil && h.Svc.Schema.B != nil {
		alert = h.Svc.Schema.B.CalcAlertQueueLen
	}
	snap, err := calc.Snapshot(r.Context(), pool, alert)
	h.WriteJSON(w, snap, err)
}

type ackErr string

func (e ackErr) Error() string { return string(e) }
