package db_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/steamedbuns/linked-numbers/services/internal/db"
	"github.com/steamedbuns/linked-numbers/services/internal/db/dbtest"
)

func TestMain(m *testing.M) { dbtest.Main(m) }

// tables lists the base tables in the public schema, except goose's own.
func tables(t *testing.T, pool *pgxpool.Pool) []string {
	t.Helper()
	rows, err := pool.Query(t.Context(), `
		SELECT table_name FROM information_schema.tables
		WHERE table_schema = 'public' AND table_type = 'BASE TABLE' AND table_name <> 'goose_db_version'
		ORDER BY table_name`)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			t.Fatal(err)
		}
		names = append(names, n)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return names
}

var wantTables = []string{
	"report_cell_sum_inputs", "report_cells", "reports", "source_values", "users", "value_changes",
}

func TestMigrationsCreateSixTables(t *testing.T) {
	t.Parallel()
	if got := tables(t, dbtest.New(t)); !slices.Equal(got, wantTables) {
		t.Errorf("tables = %v, want %v", got, wantTables)
	}
}

func TestMigrationsRoundTrip(t *testing.T) {
	t.Parallel()
	pool := dbtest.New(t)
	p, err := db.NewProvider(dbtest.SQLDB(t, pool))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.DownTo(t.Context(), 0); err != nil {
		t.Fatalf("down: %v", err)
	}
	if got := tables(t, pool); len(got) != 0 {
		t.Fatalf("after down, tables = %v, want none", got)
	}
	if _, err := p.Up(t.Context()); err != nil {
		t.Fatalf("up again: %v", err)
	}
	if got := tables(t, pool); !slices.Equal(got, wantTables) {
		t.Errorf("after up, tables = %v, want %v", got, wantTables)
	}
	if pending, err := p.HasPending(t.Context()); err != nil || pending {
		t.Errorf("HasPending = %v, %v; want false, nil", pending, err)
	}
}

// fixture inserts a user, three values, a report with a value_ref cell and a
// sum cell, and history rows for two values. Tests build on it.
const fixture = `
INSERT INTO users (id, display_name) VALUES ('u_ana', 'Ana');
INSERT INTO source_values (id, key, label, amount, unit, period, updated_by) VALUES
  ('00000000-0000-4000-8000-000000000001', 'rev.q3.2026', 'Revenue', 1200000, 'USD', '2026-Q3', 'u_ana'),
  ('00000000-0000-4000-8000-000000000002', 'cogs.q3.2026', 'COGS', 500000, 'USD', '2026-Q3', 'u_ana'),
  ('00000000-0000-4000-8000-000000000003', 'unused.q3.2026', 'Unused', 1, 'count', '2026-Q3', 'u_ana');
INSERT INTO reports (id, title, created_by) VALUES ('00000000-0000-4000-8000-000000000100', 'R', 'u_ana');
INSERT INTO report_cells (id, report_id, position, kind, value_id, label) VALUES
  ('00000000-0000-4000-8000-000000000200', '00000000-0000-4000-8000-000000000100', 0, 'value_ref', '00000000-0000-4000-8000-000000000001', NULL),
  ('00000000-0000-4000-8000-000000000201', '00000000-0000-4000-8000-000000000100', 1, 'sum', NULL, 'Margin');
INSERT INTO report_cell_sum_inputs (sum_cell_id, value_id, sign) VALUES
  ('00000000-0000-4000-8000-000000000201', '00000000-0000-4000-8000-000000000002', -1);
INSERT INTO value_changes (value_id, old_amount, new_amount, version, reason, changed_by) VALUES
  ('00000000-0000-4000-8000-000000000001', 1100000, 1200000, 2, 'restated', 'u_ana'),
  ('00000000-0000-4000-8000-000000000003', 0, 1, 2, 'first count', 'u_ana');
`

func withFixture(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool := dbtest.New(t)
	if _, err := pool.Exec(t.Context(), fixture); err != nil {
		t.Fatalf("fixture: %v", err)
	}
	return pool
}

// sqlState returns err's Postgres error code, or "" if it has none.
func sqlState(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code
	}
	return ""
}

func TestValueChangesIsAppendOnly(t *testing.T) {
	t.Parallel()
	pool := withFixture(t)
	for _, stmt := range []string{
		`UPDATE value_changes SET reason = 'edited'`,
		`DELETE FROM value_changes`,
		`TRUNCATE value_changes`,
	} {
		_, err := pool.Exec(t.Context(), stmt)
		if got := sqlState(err); got != "42501" {
			t.Errorf("%s: error %v (SQLSTATE %q), want insufficient_privilege (42501)", stmt, err, got)
		}
	}
	var n int
	if err := pool.QueryRow(t.Context(), `SELECT count(*) FROM value_changes`).Scan(&n); err != nil || n != 2 {
		t.Errorf("value_changes rows = %d, %v; want 2", n, err)
	}
}

func TestConstraints(t *testing.T) {
	t.Parallel()
	const (
		check      = "23514"
		unique     = "23505"
		foreignKey = "23503"
	)
	tests := []struct {
		name string
		stmt string
		want string
	}{
		{"blank reason", `INSERT INTO value_changes (value_id, old_amount, new_amount, version, reason, changed_by)
			VALUES ('00000000-0000-4000-8000-000000000001', 1, 2, 3, '  ', 'u_ana')`, check},
		{"history version 1", `INSERT INTO value_changes (value_id, old_amount, new_amount, version, reason, changed_by)
			VALUES ('00000000-0000-4000-8000-000000000001', 1, 2, 1, 'why', 'u_ana')`, check},
		{"duplicate history version", `INSERT INTO value_changes (value_id, old_amount, new_amount, version, reason, changed_by)
			VALUES ('00000000-0000-4000-8000-000000000001', 1, 2, 2, 'again', 'u_ana')`, unique},
		{"bad unit", `INSERT INTO source_values (key, label, amount, unit, period, updated_by)
			VALUES ('x.q1.2026', 'X', 1, 'EUR', '2026-Q1', 'u_ana')`, check},
		{"bad key", `INSERT INTO source_values (key, label, amount, unit, period, updated_by)
			VALUES ('Rev Q1', 'X', 1, 'USD', '2026-Q1', 'u_ana')`, check},
		{"duplicate key", `INSERT INTO source_values (key, label, amount, unit, period, updated_by)
			VALUES ('rev.q3.2026', 'X', 1, 'USD', '2026-Q3', 'u_ana')`, unique},
		{"unknown user", `INSERT INTO source_values (key, label, amount, unit, period, updated_by)
			VALUES ('x.q1.2026', 'X', 1, 'USD', '2026-Q1', 'u_nobody')`, foreignKey},
		{"bad user id", `INSERT INTO users (id, display_name) VALUES ('ana', 'Ana')`, check},
		{"text cell with value", `INSERT INTO report_cells (report_id, position, kind, text, value_id)
			VALUES ('00000000-0000-4000-8000-000000000100', 5, 'text', 'hi', '00000000-0000-4000-8000-000000000001')`, check},
		{"sum cell without label", `INSERT INTO report_cells (report_id, position, kind)
			VALUES ('00000000-0000-4000-8000-000000000100', 5, 'sum')`, check},
		{"duplicate position", `INSERT INTO report_cells (report_id, position, kind, text)
			VALUES ('00000000-0000-4000-8000-000000000100', 0, 'text', 'hi')`, unique},
		{"sign 2", `INSERT INTO report_cell_sum_inputs (sum_cell_id, value_id, sign)
			VALUES ('00000000-0000-4000-8000-000000000201', '00000000-0000-4000-8000-000000000001', 2)`, check},
		{"delete value used by a cell", `DELETE FROM source_values WHERE id = '00000000-0000-4000-8000-000000000001'`, foreignKey},
		{"delete value used by a sum", `DELETE FROM source_values WHERE id = '00000000-0000-4000-8000-000000000002'`, foreignKey},
	}
	pool := withFixture(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Each statement runs in a rolled-back transaction, so the
			// fixture stays the same for the next one.
			tx, err := pool.Begin(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = tx.Rollback(context.WithoutCancel(t.Context())) }()
			_, err = tx.Exec(t.Context(), tt.stmt)
			if got := sqlState(err); got != tt.want {
				t.Errorf("error %v (SQLSTATE %q), want SQLSTATE %s", err, got, tt.want)
			}
		})
	}
}

func TestAllowedChanges(t *testing.T) {
	t.Parallel()
	pool := withFixture(t)
	for _, stmt := range []string{
		// An unused value can be deleted, even with history (LN-2.4).
		`DELETE FROM source_values WHERE id = '00000000-0000-4000-8000-000000000003'`,
		// Swapping positions works once the unique check is deferred (LN-3).
		`BEGIN;
		 SET CONSTRAINTS report_cells_position_key DEFERRED;
		 UPDATE report_cells SET position = 1 - position WHERE report_id = '00000000-0000-4000-8000-000000000100';
		 COMMIT`,
		// Deleting a report deletes its cells and sum inputs.
		`DELETE FROM reports WHERE id = '00000000-0000-4000-8000-000000000100'`,
	} {
		if _, err := pool.Exec(t.Context(), stmt); err != nil {
			t.Errorf("%s: %v", stmt, err)
		}
	}
	var n int
	if err := pool.QueryRow(t.Context(), `SELECT count(*) FROM report_cell_sum_inputs`).Scan(&n); err != nil || n != 0 {
		t.Errorf("sum inputs after deleting the report = %d, %v; want 0", n, err)
	}
}
