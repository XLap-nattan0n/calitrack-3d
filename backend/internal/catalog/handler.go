package catalog

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
		log.Printf("catalog: %v", err)
		apierr.Internal(w)
	}
}

func (h *Handler) Equipment(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.Equipment(r.Context())
	if err != nil {
		h.fail(w, err)
		return
	}
	writeJSON(w, map[string]any{"equipment": items})
}

func (h *Handler) MuscleGroups(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.MuscleGroups(r.Context())
	if err != nil {
		h.fail(w, err)
		return
	}
	writeJSON(w, map[string]any{"items": items})
}

func (h *Handler) Muscles(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.Muscles(r.Context())
	if err != nil {
		h.fail(w, err)
		return
	}
	writeJSON(w, map[string]any{"items": items})
}

func (h *Handler) MyEquipment(w http.ResponseWriter, r *http.Request) {
	uid, ok := auth.UserID(r.Context())
	if !ok {
		apierr.Unauthorized(w, "missing user")
		return
	}
	items, err := h.svc.MyEquipment(r.Context(), uid)
	if err != nil {
		h.fail(w, err)
		return
	}
	writeJSON(w, map[string]any{"equipment": items})
}

func (h *Handler) SetMyEquipment(w http.ResponseWriter, r *http.Request) {
	uid, ok := auth.UserID(r.Context())
	if !ok {
		apierr.Unauthorized(w, "missing user")
		return
	}
	var body struct {
		EquipmentCodes []string `json:"equipment_codes"`
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&body); err != nil {
		apierr.BadRequest(w, "invalid JSON body", nil)
		return
	}
	items, err := h.svc.SetMyEquipment(r.Context(), uid, body.EquipmentCodes)
	if err != nil {
		h.fail(w, err)
		return
	}
	writeJSON(w, map[string]any{"equipment": items})
}

func (h *Handler) ListExercises(w http.ResponseWriter, r *http.Request) {
	uid, ok := auth.UserID(r.Context())
	if !ok {
		apierr.Unauthorized(w, "missing user")
		return
	}
	q := r.URL.Query()
	p := SearchParams{
		Q: q.Get("q"), Category: q.Get("category"), Difficulty: q.Get("difficulty"),
		Muscle: q.Get("muscle"), Role: q.Get("role"), Limit: q.Get("limit"), Cursor: q.Get("cursor"),
	}
	if _, present := q["equipment"]; present {
		p.Equipment = append([]string{}, q["equipment"]...)
		// tolerate comma-separated form too: equipment=a,b
		split := []string{}
		for _, v := range p.Equipment {
			split = append(split, splitComma(v)...)
		}
		p.Equipment = split
	}
	page, err := h.svc.SearchExercises(r.Context(), uid, p)
	if err != nil {
		h.fail(w, err)
		return
	}
	writeJSON(w, page)
}

func (h *Handler) GetExercise(w http.ResponseWriter, r *http.Request) {
	d, err := h.svc.GetExercise(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		h.fail(w, err)
		return
	}
	writeJSON(w, d)
}

func splitComma(s string) []string {
	out := []string{}
	start := 0
	for i := 0; i <= len(s); i++ {
		if i == len(s) || s[i] == ',' {
			if i > start {
				out = append(out, s[start:i])
			}
			start = i + 1
		}
	}
	return out
}
