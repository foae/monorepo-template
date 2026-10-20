package postgres

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/golang-migrate/migrate/v4"
	migratepg "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"

	sharedpg "monorepo-template/backend/pkg/postgres"
	"monorepo-template/backend/services/example/storage/postgres/sqlc"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

// Client provides database access for the service.
type Client struct {
	cl  *sharedpg.Client
	sql *sqlc.Queries
	pgx *pgxpool.Pool
}

func runMigrations(ctx context.Context, cfgURL string) (err error) {
	defer func() {
		if ctxErr := ctx.Err(); ctxErr != nil {
			err = errors.Join(err, ctxErr)
		}
	}()

	d, err := iofs.New(migrationFiles, "migrations")
	if err != nil {
		return fmt.Errorf("pg: unable to create migration source: %w", err)
	}
	var m *migrate.Migrate
	defer func() {
		if m != nil {
			return // m.Close owns the initialized source.
		}
		if err := d.Close(); err != nil {
			slog.WarnContext(ctx, "pg: migration source cleanup failed")
		}
	}()

	cfg, err := pgx.ParseConfig(cfgURL)
	if err != nil {
		return fmt.Errorf("pg: unable to parse migration config: %w", err)
	}
	db := stdlib.OpenDB(*cfg)
	db.SetMaxOpenConns(1)
	defer func() {
		if err := db.Close(); err != nil {
			slog.WarnContext(ctx, "pg: migration database cleanup failed")
		}
	}()

	conn, err := db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("pg: unable to connect for migrations: %w", err)
	}
	defer func() {
		if m != nil {
			sourceErr, databaseErr := m.Close()
			if sourceErr != nil || databaseErr != nil {
				slog.WarnContext(ctx, "pg: migration resource cleanup failed")
			}
			return
		}
		if err := conn.Close(); err != nil {
			slog.WarnContext(ctx, "pg: migration connection cleanup failed")
		}
	}()

	var migrationConn *pgconn.PgConn
	if err := conn.Raw(func(driverConn any) error {
		migrationConn = driverConn.(*stdlib.Conn).Conn().PgConn()
		return nil
	}); err != nil {
		return fmt.Errorf("pg: unable to access migration connection: %w", err)
	}

	// The migration driver uses Background for locks and metadata queries.
	// Cancel server-side work too: socket closure alone can leave a backend
	// waiting on a lock until PostgreSQL next checks the client connection.
	interrupt := func() {
		cancelCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := migrationConn.CancelRequest(cancelCtx); err != nil {
			slog.WarnContext(ctx, "pg: migration query cancellation failed")
		}
		_ = migrationConn.Conn().Close()
	}
	canceled := make(chan struct{})
	stopCancel := context.AfterFunc(ctx, func() {
		interrupt()
		close(canceled)
	})
	defer func() {
		if !stopCancel() {
			<-canceled
		} else if err != nil {
			// A library lock timeout can leave its Lock goroutine active.
			interrupt()
		}
	}()

	driver, err := migratepg.WithConnection(ctx, conn, &migratepg.Config{})
	if err != nil {
		return fmt.Errorf("pg: unable to create migration database driver: %w", err)
	}
	m, err = migrate.NewWithInstance("iofs", d, "postgres", driver)
	if err != nil {
		return fmt.Errorf("pg: unable to create migration instance: %w", err)
	}

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("pg: unable to run migrations: %w", err)
	}

	return nil
}

// New creates a new database client with optional migration execution.
func New(ctx context.Context, cfgURL string, minConns int, maxConns int, shouldRunMigrations bool) (*Client, error) {
	p, err := sharedpg.New(ctx, cfgURL, minConns, maxConns)
	if err != nil {
		return nil, fmt.Errorf("pg: unable to create postgres client: %w", err)
	}

	if shouldRunMigrations {
		slog.InfoContext(ctx, "pg: running database migrations")
		if err := runMigrations(ctx, cfgURL); err != nil {
			p.Close()
			return nil, fmt.Errorf("pg: migration failed: %w", err)
		}
		slog.InfoContext(ctx, "pg: migrations completed successfully")
	}

	scWrap := &SqlcWrapper{pgx: p.Pool()}
	sc := sqlc.New(scWrap)

	return &Client{
		cl:  p,
		sql: sc,
		pgx: scWrap.pgx,
	}, nil
}

func (c *Client) DB() *pgxpool.Pool {
	return c.pgx
}

func (c *Client) Queries() *sqlc.Queries {
	return c.sql
}

func (c *Client) WithTx(tx pgx.Tx) *Client {
	return &Client{
		cl:  c.cl,
		pgx: c.pgx,
		sql: c.sql.WithTx(tx),
	}
}

func (c *Client) Close() {
	c.cl.Close()
}

// SqlcWrapper wraps pgxpool.Pool to implement sqlc's DBTX interface.
type SqlcWrapper struct {
	pgx *pgxpool.Pool
}

func (sw *SqlcWrapper) Exec(ctx context.Context, query string, args ...interface{}) (pgconn.CommandTag, error) {
	return sw.pgx.Exec(ctx, query, args...)
}

func (sw *SqlcWrapper) Query(ctx context.Context, query string, args ...interface{}) (pgx.Rows, error) {
	return sw.pgx.Query(ctx, query, args...)
}

func (sw *SqlcWrapper) QueryRow(ctx context.Context, query string, args ...interface{}) pgx.Row {
	return sw.pgx.QueryRow(ctx, query, args...)
}
