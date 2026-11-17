// Package db owns the lazily created pgx pool.
package db

import (
	"context"
	"errors"
	"sync"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Pool lazily connects so the process (and /health) can start without a reachable database.
type Pool struct {
	url  string
	once sync.Once
	pool *pgxpool.Pool
	err  error
}

func New(databaseURL string) *Pool { return &Pool{url: databaseURL} }

func (p *Pool) Get(ctx context.Context) (*pgxpool.Pool, error) {
	p.once.Do(func() {
		if p.url == "" {
			p.err = errors.New("DATABASE_URL is not set")
			return
		}
		cfg, err := pgxpool.ParseConfig(p.url)
		if err != nil {
			p.err = err
			return
		}
		// Supabase pooler on port 6543 runs in transaction mode and cannot hold prepared statements.
		cfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol
		cfg.MaxConns = 2
		p.pool, p.err = pgxpool.NewWithConfig(context.Background(), cfg)
	})
	return p.pool, p.err
}
