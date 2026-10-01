// Package dbtest gives tests a real, migrated Postgres database.
//
// A package's TestMain calls Main, which starts one Postgres container for
// the test binary and builds two template databases in it: one migrated, one
// migrated and seeded. Each New or NewSeeded call then clones a template into
// a fresh database (NewEmpty clones template0, for code that migrates), so
// tests are isolated and can run in parallel. It needs Docker
// (testcontainers-go).
package dbtest

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/steamedbuns/linked-numbers/services/internal/db"
)

// Image is the Postgres image tests run against. Keep it in sync with
// deploy/compose/compose.yaml.
const Image = "postgres:16.15-alpine"

const (
	migratedDB = "ln_migrated"
	seededDB   = "ln_seeded"
)

var (
	adminURL string     // the container's maintenance database
	createMu sync.Mutex // serializes CREATE DATABASE ... TEMPLATE
	seq      atomic.Int64
)

// Main starts Postgres, migrates the template database, runs m and exits.
// Call it from TestMain.
func Main(m *testing.M) {
	os.Exit(run(m))
}

func run(m *testing.M) int {
	ctx := context.Background()
	ctr, err := postgres.Run(ctx, Image,
		postgres.WithDatabase("postgres"),
		postgres.WithUsername("postgres"),
		postgres.WithPassword("postgres"),
		postgres.BasicWaitStrategies(),
	)
	defer func() {
		if err := testcontainers.TerminateContainer(ctr); err != nil {
			fmt.Fprintln(os.Stderr, "dbtest: terminate postgres:", err)
		}
	}()
	if err != nil {
		fmt.Fprintln(os.Stderr, "dbtest: start postgres (is Docker running?):", err)
		return 1
	}
	adminURL, err = ctr.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		fmt.Fprintln(os.Stderr, "dbtest:", err)
		return 1
	}
	if err := createTemplates(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "dbtest: create templates:", err)
		return 1
	}
	return m.Run()
}

func createTemplates(ctx context.Context) error {
	logger := slog.New(slog.DiscardHandler)
	if err := exec(ctx, "CREATE DATABASE "+migratedDB); err != nil {
		return err
	}
	cfg, err := configFor(migratedDB)
	if err != nil {
		return err
	}
	// Close every connection before cloning: Postgres refuses to copy a
	// template that has other sessions.
	sqlDB := stdlib.OpenDB(*cfg.ConnConfig)
	err = db.Migrate(ctx, sqlDB, logger)
	_ = sqlDB.Close()
	if err != nil {
		return err
	}

	if err := exec(ctx, "CREATE DATABASE "+seededDB+" TEMPLATE "+migratedDB); err != nil {
		return err
	}
	if cfg, err = configFor(seededDB); err != nil {
		return err
	}
	conn, err := pgx.ConnectConfig(ctx, cfg.ConnConfig)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close(ctx) }()
	return db.Seed(ctx, conn, logger)
}

// New returns a pool on a new, migrated database with no rows. The database
// is dropped when the test ends.
func New(t testing.TB) *pgxpool.Pool {
	t.Helper()
	return clone(t, migratedDB)
}

// NewSeeded is New with the seed data (db.Seed) already applied.
func NewSeeded(t testing.TB) *pgxpool.Pool {
	t.Helper()
	return clone(t, seededDB)
}

// NewEmpty returns a pool on a new database with no migrations applied.
func NewEmpty(t testing.TB) *pgxpool.Pool {
	t.Helper()
	return clone(t, "template0")
}

func clone(t testing.TB, template string) *pgxpool.Pool {
	t.Helper()
	ctx := t.Context()
	name := fmt.Sprintf("t_%d_%d", os.Getpid(), seq.Add(1))

	createMu.Lock()
	err := exec(ctx, "CREATE DATABASE "+name+" TEMPLATE "+template)
	createMu.Unlock()
	if err != nil {
		t.Fatalf("dbtest: %v", err)
	}
	cfg, err := configFor(name)
	if err != nil {
		t.Fatalf("dbtest: %v", err)
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatalf("dbtest: %v", err)
	}
	t.Cleanup(func() {
		pool.Close()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := exec(ctx, "DROP DATABASE "+name+" WITH (FORCE)"); err != nil {
			t.Errorf("dbtest: %v", err)
		}
	})
	return pool
}

// SQLDB returns a database/sql handle on pool, for goose. It is closed when
// the test ends.
func SQLDB(t testing.TB, pool *pgxpool.Pool) *sql.DB {
	t.Helper()
	conn := stdlib.OpenDBFromPool(pool)
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

// configFor parses a URL for database, so the pool's ConnString() names it
// too: tests pass that to code that takes DATABASE_URL.
func configFor(database string) (*pgxpool.Config, error) {
	u, err := url.Parse(adminURL)
	if err != nil {
		return nil, err
	}
	u.Path = "/" + database
	return pgxpool.ParseConfig(u.String())
}

// exec runs one statement on the maintenance database. Names are generated
// here, never taken from input.
func exec(ctx context.Context, stmt string) error {
	conn, err := pgx.Connect(ctx, adminURL)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close(context.WithoutCancel(ctx)) }()
	if _, err := conn.Exec(ctx, stmt); err != nil {
		return fmt.Errorf("%s: %w", stmt, err)
	}
	return nil
}
