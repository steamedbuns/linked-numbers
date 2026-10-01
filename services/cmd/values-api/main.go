// Command values-api serves the REST API for values and reports. It is the
// only service that reads or writes the database.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"

	"github.com/steamedbuns/linked-numbers/services/internal/db"
	"github.com/steamedbuns/linked-numbers/services/internal/service"
)

func main() {
	service.Service{Name: "values-api", DefaultAddr: ":8081", Init: setup}.Main()
}

// setup opens the database pool, applies migrations if MIGRATE_ON_START is
// set, and registers two readiness checks: "postgres" (a ping) and "schema"
// (every migration applied).
func setup(ctx context.Context, cfg service.Config, logger *slog.Logger) (service.Deps, error) {
	if cfg.DatabaseURL == "" {
		return service.Deps{}, errors.New("DATABASE_URL is required")
	}
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		// pgx errors here don't include the password.
		return service.Deps{}, fmt.Errorf("DATABASE_URL: %w", err)
	}
	pc := pool.Config().ConnConfig
	logger.InfoContext(ctx, "database configured", "host", pc.Host, "port", pc.Port, "database", pc.Database)

	if cfg.MigrateOnStart {
		// goose needs database/sql; this handle borrows the pool's connections.
		sqlDB := stdlib.OpenDBFromPool(pool)
		err := db.Migrate(ctx, sqlDB, logger)
		_ = sqlDB.Close()
		if err != nil {
			pool.Close()
			return service.Deps{}, err
		}
	}
	schema, err := db.SchemaCheck(pool)
	if err != nil {
		pool.Close()
		return service.Deps{}, err
	}
	return service.Deps{
		Ready: []service.Check{{Name: "postgres", Fn: pool.Ping}, {Name: "schema", Fn: schema}},
		Close: pool.Close,
	}, nil
}
