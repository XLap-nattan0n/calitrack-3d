package analytics

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/XLap-nattan0n/calitrack-3d/backend/internal/apierr"
	"github.com/XLap-nattan0n/calitrack-3d/backend/internal/auth"
)

type Handler struct{ svc *Service }

func NewHandler(s *Service) *Handler { return &Handler{svc: s} }

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func (h *Handler) fail(w http.ResponseWriter, err error) {
	var ve *ValidationError
	switch {
	case errors.As(err, &ve):
		apierr.BadRequest(w, "invalid request", ve.Details)
	case errors.Is(err, ErrNotFound):
		apierr.NotFound(w, "exercise not found")
	default:
		log.Printf("analytics: %v", err)
		apierr.Internal(w)
	}
}

func user(w http.ResponseWriter, r *http.Request) (string, bool) {
	uid, ok := auth.UserID(r.Context())
	if !ok {
		apierr.Unauthorized(w, "missing user")
	}
	return uid, ok
}

func (h *Handler) Summary(w http.ResponseWriter, r *http.Request) {
	uid, ok := user(w, r)
	if !ok {
		return
	}
	s, err := h.svc.Summary(r.Context(), uid, r.URL.Query().Get("range"))
	if err != nil {
		h.fail(w, err)
		return
	}
	writeJSON(w, s)
}

func (h *Handler) Progress(w http.ResponseWriter, r *http.Request) {
	uid, ok := user(w, r)
	if !ok {
		return
	}
	p, err := h.svc.Progress(r.Context(), uid, chi.URLParam(r, "id"), r.URL.Query().Get("range"))
	if err != nil {
		h.fail(w, err)
		return
	}
	writeJSON(w, p)
}

func (h *Handler) MuscleFrequency(w http.ResponseWriter, r *http.Request) {
	uid, ok := user(w, r)
	if !ok {
		return
	}
	items, err := h.svc.MuscleFrequency(r.Context(), uid, r.URL.Query().Get("range"))
	if err != nil {
		h.fail(w, err)
		return
	}
	writeJSON(w, map[string]any{"items": items})
}
