// Package db owns the Postgres schema: the goose migrations, embedded in the
// binary, and the code that applies them. See docs/adr/0005.
package db

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"log/slog"

	"github.com/pressly/goose/v3"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
)

//go:embed migrations/*.sql
var embedded embed.FS

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

func mustSub(fsys fs.FS, dir string) fs.FS {
	sub, err := fs.Sub(fsys, dir)
	if err != nil {
		panic(err)
	}
	return sub
}
