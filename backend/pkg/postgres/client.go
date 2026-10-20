package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Client owns the pgx connection pool used by generated queries.
type Client struct {
	pg *pgxpool.Pool
}

// New creates a new Postgres client and verifies its connection using ctx.
func New(ctx context.Context, pgconfigURL string, minConns int, maxConns int) (*Client, error) {
	cfg, err := pgxpool.ParseConfig(pgconfigURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse postgres config: %w", err)
	}

	if cfg.ConnConfig.ConnectTimeout == 0 {
		cfg.ConnConfig.ConnectTimeout = 10 * time.Second
	}
	cfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeExec
	cfg.MaxConns = int32(maxConns)
	cfg.MinConns = int32(minConns)
	cfg.MaxConnLifetime = time.Minute * 60
	cfg.MaxConnIdleTime = time.Minute * 5
	cfg.MaxConnLifetimeJitter = time.Millisecond * 450

	pg, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to postgres: %w", err)
	}
	if err := pg.Ping(ctx); err != nil {
		pg.Close()
		return nil, fmt.Errorf("failed to ping postgres: %w", err)
	}

	return &Client{pg: pg}, nil
}

func (c *Client) Close() {
	c.pg.Close()
}

func (c *Client) Pool() *pgxpool.Pool {
	return c.pg
}
