// Package httpapi builds the single router used by both Vercel and the local server.
package httpapi

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/XLap-nattan0n/calitrack-3d/backend/internal/apierr"
	"github.com/XLap-nattan0n/calitrack-3d/backend/internal/auth"
	"github.com/XLap-nattan0n/calitrack-3d/backend/internal/config"
	"github.com/XLap-nattan0n/calitrack-3d/backend/internal/db"
	"github.com/XLap-nattan0n/calitrack-3d/backend/internal/profile"
)

// NewRouter wires config -> db -> repositories -> services -> handlers.
func NewRouter(cfg config.Config) http.Handler {
	pool := db.New(cfg.DatabaseURL)
	verifier := newVerifier(cfg)
	ph := profile.NewHandler(profile.NewService(profile.NewPGRepository(pool)))
	return Build(pool, verifier, ph)
}

func newVerifier(cfg config.Config) *auth.Verifier {
	if err := cfg.ValidateAuth(); err != nil {
		log.Printf("auth not configured: %v (protected routes will return 401)", err)
		return nil
	}
	var jwks *auth.JWKS
	if cfg.JWKSURL != "" {
		jwks = auth.NewJWKS(cfg.JWKSURL)
	}
	v, err := auth.NewVerifier([]byte(cfg.JWTSecret), jwks, cfg.JWTIssuer, cfg.JWTAudience)
	if err != nil {
		log.Printf("auth: %v", err)
		return nil
	}
	return v
}

// Build assembles routes from ready dependencies (also used by tests).
func Build(pool *db.Pool, v *auth.Verifier, ph *profile.Handler) http.Handler {
	r := chi.NewRouter()
	r.NotFound(func(w http.ResponseWriter, _ *http.Request) { apierr.NotFound(w, "route not found") })
	r.MethodNotAllowed(func(w http.ResponseWriter, _ *http.Request) {
		apierr.Write(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed", nil)
	})

	r.Get("/api/v1/health", health(pool))

	r.Group(func(r chi.Router) {
		if v != nil {
			r.Use(v.Middleware)
		} else {
			r.Use(func(http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					apierr.Unauthorized(w, "authentication is not configured")
				})
			})
		}
		r.Get("/api/v1/me", ph.Get)
		r.Patch("/api/v1/me", ph.Patch)
	})
	return r
}

func health(pool *db.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		w.Header().Set("Content-Type", "application/json")
		p, err := pool.Get(ctx)
		if err == nil {
			var one int
			err = p.QueryRow(ctx, "select 1").Scan(&one)
		}
		if err != nil {
			log.Printf("health: database check failed: %v", err)
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "error", "db": "unavailable"})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok", "db": "ok"})
	}
}
