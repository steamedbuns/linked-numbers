// Package db owns the Postgres schema and demo data: the goose migrations and
// seed.sql, embedded in the binary, and the code that applies them. See
// docs/adr/0005.
package db

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"log/slog"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/pressly/goose/v3"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
)

//go:embed migrations/*.sql
var embedded embed.FS

//go:embed seed.sql
var seedSQL string

// Migrations is the migrations folder, as goose reads it.
var Migrations = mustSub(embedded, "migrations")

var tracer = otel.Tracer("github.com/steamedbuns/linked-numbers/services/internal/db")

// NewProvider returns a goose provider for the embedded migrations on conn.
func NewProvider(conn *sql.DB) (*goose.Provider, error) {
	return goose.NewProvider(goose.DialectPostgres, conn, Migrations)
}

// Migrate applies every pending migration and logs each one it applies.
func Migrate(ctx context.Context, conn *sql.DB, logger *slog.Logger) (err error) {
	ctx, span := tracer.Start(ctx, "db.migrate")
	defer func() {
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
		span.End()
	}()

	p, err := NewProvider(conn)
	if err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	results, err := p.Up(ctx)
	for _, r := range results {
		logger.InfoContext(ctx, "migration applied",
			"version", r.Source.Version, "file", r.Source.Path, "duration_ms", r.Duration.Milliseconds())
	}
	if err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	version, err := p.GetDBVersion(ctx)
	if err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	logger.InfoContext(ctx, "schema up to date", "version", version, "applied", len(results))
	return nil
}

// Execer runs SQL. *pgx.Conn, *pgxpool.Pool and pgx.Tx all implement it.
type Execer interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// Seed inserts the demo data: 3 users, 12 values and 2 reports. It runs as one
// statement batch, so it applies fully or not at all, and rows that already
// exist are skipped, so seeding twice changes nothing.
func Seed(ctx context.Context, conn Execer, logger *slog.Logger) (err error) {
	ctx, span := tracer.Start(ctx, "db.seed")
	defer func() {
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
		span.End()
	}()

	// With no arguments, pgx sends the file as one simple-protocol query,
	// which Postgres runs in a single implicit transaction.
	if _, err := conn.Exec(ctx, seedSQL); err != nil {
		return fmt.Errorf("seed: %w", err)
	}
	logger.InfoContext(ctx, "seed data applied")
	return nil
}

func mustSub(fsys fs.FS, dir string) fs.FS {
	sub, err := fs.Sub(fsys, dir)
	if err != nil {
		panic(err)
	}
	return sub
}
