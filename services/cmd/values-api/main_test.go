package main

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"github.com/steamedbuns/linked-numbers/services/internal/db/dbtest"
	"github.com/steamedbuns/linked-numbers/services/internal/logging"
	"github.com/steamedbuns/linked-numbers/services/internal/service"
)

func TestMain(m *testing.M) { dbtest.Main(m) }

// downURL points at a port nothing listens on. Its password lets
// TestSetupErrors check that errors don't leak it.
const downURL = "postgres://u:secret@127.0.0.1:1/ln?connect_timeout=1" //nolint:gosec // not a real credential

// checks runs setup with cfg and returns each readiness check's error by
// name ("" when it passes).
func checks(t *testing.T, cfg service.Config) map[string]string {
	t.Helper()
	deps, err := setup(t.Context(), cfg, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatalf("setup() = %v", err)
	}
	t.Cleanup(deps.Close)
	got := map[string]string{}
	for _, c := range deps.Ready {
		got[c.Name] = ""
		if err := c.Fn(t.Context()); err != nil {
			got[c.Name] = err.Error()
		}
	}
	return got
}

func TestReadyOnMigratedDatabase(t *testing.T) {
	t.Parallel()
	url := dbtest.New(t).Config().ConnString()
	got := checks(t, service.Config{DatabaseURL: url})
	if got["postgres"] != "" || got["schema"] != "" || len(got) != 2 {
		t.Errorf("checks = %q, want postgres and schema passing", got)
	}
}

func TestNotReadyBeforeMigrating(t *testing.T) {
	t.Parallel()
	pool := dbtest.NewEmpty(t)
	got := checks(t, service.Config{DatabaseURL: pool.Config().ConnString()})
	if got["postgres"] != "" || got["schema"] != "not migrated, want version 1" {
		t.Errorf("checks = %q, want postgres passing and schema not migrated", got)
	}
	// The probe only reads: it must not create goose's version table.
	var exists bool
	if err := pool.QueryRow(t.Context(), `SELECT to_regclass('goose_db_version') IS NOT NULL`).Scan(&exists); err != nil || exists {
		t.Errorf("goose_db_version exists = %v (%v) after the schema check, want false", exists, err)
	}
}

func TestMigrateOnStart(t *testing.T) {
	t.Parallel()
	pool := dbtest.NewEmpty(t)
	var logs bytes.Buffer
	deps, err := setup(t.Context(), service.Config{DatabaseURL: pool.Config().ConnString(), MigrateOnStart: true},
		logging.New(&logs, slog.LevelInfo).With("version", "test"))
	if err != nil {
		t.Fatalf("setup() = %v", err)
	}
	t.Cleanup(deps.Close)
	if err := deps.Ready[1].Fn(t.Context()); err != nil {
		t.Errorf("schema check = %v after MIGRATE_ON_START, want passing", err)
	}
	var n int
	if err := pool.QueryRow(t.Context(), `SELECT count(*) FROM source_values`).Scan(&n); err != nil {
		t.Errorf("source_values after migrating: %v", err)
	}

	// The migration is logged with its own key, not the build's "version".
	var applied map[string]any
	for line := range strings.Lines(logs.String()) {
		var rec map[string]any
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			t.Fatalf("log line is not JSON: %q", line)
		}
		if rec["msg"] == "migration applied" {
			applied = rec
		}
	}
	if applied["schema_version"] != float64(1) || applied["version"] != "test" {
		t.Errorf("migration log = %v, want schema_version 1 and the build version kept", applied)
	}
}

func TestNotReadyWhenPostgresIsDown(t *testing.T) {
	t.Parallel()
	// Nothing listens on port 1, and pgxpool connects lazily, so setup
	// succeeds and the checks report the outage.
	got := checks(t, service.Config{DatabaseURL: downURL})
	if got["postgres"] == "" || got["schema"] == "" {
		t.Errorf("checks = %q, want postgres and schema failing", got)
	}
}

func TestSetupErrors(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		cfg  service.Config
		want string
	}{
		{"no DATABASE_URL", service.Config{}, "DATABASE_URL is required"},
		{"bad DATABASE_URL", service.Config{DatabaseURL: strings.Replace(downURL, ":1/", ":notaport/", 1)}, "DATABASE_URL"},
		{"migrate with Postgres down", service.Config{DatabaseURL: downURL, MigrateOnStart: true}, "migrate"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := setup(t.Context(), tt.cfg, slog.New(slog.DiscardHandler))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("setup() = %v, want an error mentioning %q", err, tt.want)
			}
			if err != nil && strings.Contains(err.Error(), "secret") {
				t.Errorf("setup() error %q leaks the password", err)
			}
		})
	}
}
