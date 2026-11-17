package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	poolOnce sync.Once
	pool     *pgxpool.Pool
	poolErr  error
)

func NewRouter() http.Handler {
	r := chi.NewRouter()
	r.Get("/api/v1/health", health)
	return r
}

func getPool() (*pgxpool.Pool, error) {
	poolOnce.Do(func() {
		url := strings.TrimSpace(os.Getenv("DATABASE_URL"))
		if url == "" {
			poolErr = errors.New("DATABASE_URL is not set")
			return
		}
		cfg, err := pgxpool.ParseConfig(url)
		if err != nil {
			poolErr = err
			return
		}
		// Supabase pooler on port 6543 runs in transaction mode and cannot hold prepared statements.
		cfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol
		cfg.MaxConns = 2
		pool, poolErr = pgxpool.NewWithConfig(context.Background(), cfg)
	})
	return pool, poolErr
}

func health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	w.Header().Set("Content-Type", "application/json")
	p, err := getPool()
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
