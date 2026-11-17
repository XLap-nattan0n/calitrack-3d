// Package profile implements GET/PATCH /me as Handler -> Service -> Repository.
package profile

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"

	"github.com/XLap-nattan0n/calitrack-3d/backend/internal/apierr"
	"github.com/XLap-nattan0n/calitrack-3d/backend/internal/auth"
	"github.com/XLap-nattan0n/calitrack-3d/backend/internal/db"
)

var (
	ErrNotFound   = errors.New("profile not found")
	ErrValidation = errors.New("validation failed")
)

type Profile struct {
	ID          string    `json:"id"`
	DisplayName *string   `json:"display_name"`
	Locale      string    `json:"locale"`
	CreatedAt   time.Time `json:"created_at"`
}

type Update struct {
	DisplayName *string `json:"display_name"`
	Locale      *string `json:"locale"`
}

// Repository: every method is scoped by userID (ownership).
type Repository interface {
	Get(ctx context.Context, userID string) (Profile, error)
	Update(ctx context.Context, userID string, u Update) (Profile, error)
}

// ---- repository (pgx)

type PGRepository struct{ pool *db.Pool }

func NewPGRepository(p *db.Pool) *PGRepository { return &PGRepository{pool: p} }

const cols = `id::text, display_name, locale, created_at`

func (r *PGRepository) Get(ctx context.Context, userID string) (Profile, error) {
	pool, err := r.pool.Get(ctx)
	if err != nil {
		return Profile{}, err
	}
	return scan(pool.QueryRow(ctx, `select `+cols+` from public.profiles where id = $1`, userID))
}

func (r *PGRepository) Update(ctx context.Context, userID string, u Update) (Profile, error) {
	pool, err := r.pool.Get(ctx)
	if err != nil {
		return Profile{}, err
	}
	return scan(pool.QueryRow(ctx,
		`update public.profiles
		    set display_name = coalesce($2, display_name),
		        locale       = coalesce($3, locale)
		  where id = $1
		  returning `+cols, userID, u.DisplayName, u.Locale))
}

type row interface{ Scan(...any) error }

func scan(r row) (Profile, error) {
	var p Profile
	err := r.Scan(&p.ID, &p.DisplayName, &p.Locale, &p.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Profile{}, ErrNotFound
	}
	return p, err
}

// ---- service

type Service struct{ repo Repository }

func NewService(r Repository) *Service { return &Service{repo: r} }

func (s *Service) Get(ctx context.Context, userID string) (Profile, error) {
	return s.repo.Get(ctx, userID)
}

func (s *Service) Update(ctx context.Context, userID string, u Update) (Profile, error) {
	if u.DisplayName != nil {
		n := strings.TrimSpace(*u.DisplayName)
		if utf8.RuneCountInString(n) > 80 {
			return Profile{}, ErrValidation
		}
		u.DisplayName = &n
	}
	if u.Locale != nil {
		l := strings.TrimSpace(*u.Locale)
		if l == "" || len(l) > 35 {
			return Profile{}, ErrValidation
		}
		u.Locale = &l
	}
	if u.DisplayName == nil && u.Locale == nil {
		return s.repo.Get(ctx, userID)
	}
	return s.repo.Update(ctx, userID, u)
}

// ---- handler

type Handler struct{ svc *Service }

func NewHandler(s *Service) *Handler { return &Handler{svc: s} }

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	uid, ok := auth.UserID(r.Context())
	if !ok {
		apierr.Unauthorized(w, "missing user")
		return
	}
	p, err := h.svc.Get(r.Context(), uid)
	h.respond(w, p, err)
}

func (h *Handler) Patch(w http.ResponseWriter, r *http.Request) {
	uid, ok := auth.UserID(r.Context())
	if !ok {
		apierr.Unauthorized(w, "missing user")
		return
	}
	var u Update
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&u); err != nil {
		apierr.BadRequest(w, "invalid JSON body", nil)
		return
	}
	p, err := h.svc.Update(r.Context(), uid, u)
	h.respond(w, p, err)
}

func (h *Handler) respond(w http.ResponseWriter, p Profile, err error) {
	switch {
	case err == nil:
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(p)
	case errors.Is(err, ErrNotFound):
		apierr.NotFound(w, "profile not found")
	case errors.Is(err, ErrValidation):
		apierr.BadRequest(w, "invalid display_name or locale", map[string]any{"display_name": "max 80 characters", "locale": "non-empty, max 35 characters"})
	default:
		log.Printf("profile: %v", err)
		apierr.Internal(w)
	}
}
