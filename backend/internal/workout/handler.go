package workout

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/XLap-nattan0n/calitrack-3d/backend/internal/apierr"
	"github.com/XLap-nattan0n/calitrack-3d/backend/internal/auth"
)

const maxBody = 1 << 17

type Handler struct{ svc *Service }

func NewHandler(s *Service) *Handler { return &Handler{svc: s} }

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func (h *Handler) fail(w http.ResponseWriter, err error) {
	var ve *ValidationError
	var ur *ErrUnknownReference
	switch {
	case errors.As(err, &ve):
		apierr.BadRequest(w, "invalid request", ve.Details)
	case errors.As(err, &ur):
		apierr.BadRequest(w, "invalid request", map[string]any{ur.Field: "does not exist"})
	case errors.Is(err, ErrSessionClosed):
		apierr.BadRequest(w, "session is not in progress", nil)
	case errors.Is(err, ErrNotFound):
		apierr.NotFound(w, "not found")
	default:
		log.Printf("workout: %v", err)
		apierr.Internal(w)
	}
}

// user returns the authenticated user or writes 401.
func user(w http.ResponseWriter, r *http.Request) (string, bool) {
	uid, ok := auth.UserID(r.Context())
	if !ok {
		apierr.Unauthorized(w, "missing user")
	}
	return uid, ok
}

// decode reads a single JSON value. When optional is true an empty body is accepted (dst untouched).
func decode(w http.ResponseWriter, r *http.Request, dst any, optional bool) bool {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBody))
	if err != nil {
		apierr.BadRequest(w, "invalid JSON body", nil)
		return false
	}
	if optional && len(bytes.TrimSpace(body)) == 0 {
		return true
	}
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil || dec.More() {
		apierr.BadRequest(w, "invalid JSON body", nil)
		return false
	}
	return true
}

func (h *Handler) ListPlans(w http.ResponseWriter, r *http.Request) {
	uid, ok := user(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	items, err := h.svc.ListPlans(r.Context(), uid, q.Get("from"), q.Get("to"))
	if err != nil {
		h.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) CreatePlan(w http.ResponseWriter, r *http.Request) {
	uid, ok := user(w, r)
	if !ok {
		return
	}
	var in PlanInput
	if !decode(w, r, &in, false) {
		return
	}
	p, err := h.svc.CreatePlan(r.Context(), uid, in)
	if err != nil {
		h.fail(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, p)
}

func (h *Handler) GetPlan(w http.ResponseWriter, r *http.Request) {
	uid, ok := user(w, r)
	if !ok {
		return
	}
	p, err := h.svc.GetPlan(r.Context(), uid, chi.URLParam(r, "id"))
	if err != nil {
		h.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (h *Handler) ReplacePlan(w http.ResponseWriter, r *http.Request) {
	uid, ok := user(w, r)
	if !ok {
		return
	}
	var in PlanInput
	if !decode(w, r, &in, false) {
		return
	}
	p, err := h.svc.ReplacePlan(r.Context(), uid, chi.URLParam(r, "id"), in)
	if err != nil {
		h.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (h *Handler) DeletePlan(w http.ResponseWriter, r *http.Request) {
	uid, ok := user(w, r)
	if !ok {
		return
	}
	if err := h.svc.DeletePlan(r.Context(), uid, chi.URLParam(r, "id")); err != nil {
		h.fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) StartSession(w http.ResponseWriter, r *http.Request) {
	uid, ok := user(w, r)
	if !ok {
		return
	}
	var body struct {
		PlanID *string `json:"plan_id"`
	}
	if !decode(w, r, &body, true) {
		return
	}
	s, err := h.svc.StartSession(r.Context(), uid, body.PlanID)
	if err != nil {
		h.fail(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, s)
}

func (h *Handler) ListSessions(w http.ResponseWriter, r *http.Request) {
	uid, ok := user(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	page, err := h.svc.ListSessions(r.Context(), uid, SessionParams{
		From: q.Get("from"), To: q.Get("to"), Status: q.Get("status"), Limit: q.Get("limit"), Cursor: q.Get("cursor"),
	})
	if err != nil {
		h.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, page)
}

func (h *Handler) ActiveSession(w http.ResponseWriter, r *http.Request) {
	uid, ok := user(w, r)
	if !ok {
		return
	}
	s, err := h.svc.ActiveSession(r.Context(), uid)
	if err != nil {
		h.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, s)
}

func (h *Handler) GetSession(w http.ResponseWriter, r *http.Request) {
	uid, ok := user(w, r)
	if !ok {
		return
	}
	s, err := h.svc.GetSession(r.Context(), uid, chi.URLParam(r, "id"))
	if err != nil {
		h.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, s)
}

func (h *Handler) UpdateSet(w http.ResponseWriter, r *http.Request) {
	uid, ok := user(w, r)
	if !ok {
		return
	}
	var raw map[string]json.RawMessage
	if !decode(w, r, &raw, false) {
		return
	}
	p, err := parsePatch(raw)
	if err != nil {
		h.fail(w, err)
		return
	}
	res, err := h.svc.UpdateSet(r.Context(), uid, chi.URLParam(r, "id"), chi.URLParam(r, "setId"), p)
	if err != nil {
		h.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (h *Handler) FinishSession(w http.ResponseWriter, r *http.Request) {
	uid, ok := user(w, r)
	if !ok {
		return
	}
	var body struct {
		Notes *string `json:"notes"`
	}
	if !decode(w, r, &body, true) {
		return
	}
	s, err := h.svc.FinishSession(r.Context(), uid, chi.URLParam(r, "id"), body.Notes)
	if err != nil {
		h.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, s)
}

func (h *Handler) Records(w http.ResponseWriter, r *http.Request) {
	uid, ok := user(w, r)
	if !ok {
		return
	}
	items, err := h.svc.Records(r.Context(), uid)
	if err != nil {
		h.fail(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

// parsePatch keeps the difference between an absent field and an explicit null.
func parsePatch(raw map[string]json.RawMessage) (SetPatch, error) {
	var p SetPatch
	for k, v := range raw {
		var err error
		switch k {
		case "actual_reps":
			err = setOpt(&p.ActualReps, v)
		case "duration_seconds":
			err = setOpt(&p.DurationSeconds, v)
		case "completed":
			err = setOpt(&p.Completed, v)
		case "assist_band":
			err = setOpt(&p.AssistBand, v)
		case "notes":
			err = setOpt(&p.Notes, v)
		default:
			return p, invalid(k, "unknown field")
		}
		if err != nil {
			return p, invalid(k, "has an invalid type")
		}
	}
	return p, nil
}

func setOpt[T any](o *Optional[T], raw json.RawMessage) error {
	o.Set = true
	return json.Unmarshal(raw, &o.Value) // null -> nil pointer
}
