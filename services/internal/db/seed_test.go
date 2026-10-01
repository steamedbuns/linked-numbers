package db_test

import (
	"log/slog"
	"maps"
	"slices"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/steamedbuns/linked-numbers/services/internal/db"
	"github.com/steamedbuns/linked-numbers/services/internal/db/dbtest"
	"github.com/steamedbuns/linked-numbers/services/internal/store"
)

// wantValues is the seed's 12 values, key → amount, in key order.
var wantValues = []struct{ key, amount string }{
	{"cogs.q1.2026", "430500.0000"},
	{"cogs.q2.2026", "459200.0000"},
	{"cogs.q3.2026", "492000.0000"},
	{"cogs.q4.2026", "524800.0000"},
	{"opex.q1.2026", "315000.0000"},
	{"opex.q2.2026", "324800.0000"},
	{"opex.q3.2026", "336000.0000"},
	{"opex.q4.2026", "345600.0000"},
	{"rev.q1.2026", "1050000.0000"},
	{"rev.q2.2026", "1120000.0000"},
	{"rev.q3.2026", "1200000.0000"},
	{"rev.q4.2026", "1280000.0000"},
}

func TestSeedValues(t *testing.T) {
	t.Parallel()
	values, err := store.New(dbtest.NewSeeded(t)).ListValues(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(values) != len(wantValues) {
		t.Fatalf("got %d values, want %d", len(values), len(wantValues))
	}
	for i, v := range values {
		want := wantValues[i]
		// Amounts are decimals end to end; StringFixed is how they go on the wire.
		if v.Key != want.key || v.Amount.StringFixed(4) != want.amount || v.Unit != "USD" || v.Version != 1 {
			t.Errorf("value %d = %s %s %s v%d, want %s %s USD v1",
				i, v.Key, v.Amount.StringFixed(4), v.Unit, v.Version, want.key, want.amount)
		}
	}
}

func TestSeedPeriodFilter(t *testing.T) {
	t.Parallel()
	period := "2026-Q3"
	values, err := store.New(dbtest.NewSeeded(t)).ListValues(t.Context(), &period)
	if err != nil {
		t.Fatal(err)
	}
	var keys []string
	for _, v := range values {
		keys = append(keys, v.Key)
	}
	if want := []string{"cogs.q3.2026", "opex.q3.2026", "rev.q3.2026"}; !slices.Equal(keys, want) {
		t.Errorf("ListValues(%q) keys = %v, want %v", period, keys, want)
	}
}

func TestSeedReportsShareFourValues(t *testing.T) {
	t.Parallel()
	pool := dbtest.NewSeeded(t)
	q := store.New(pool)
	reports, err := q.ListReports(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	var titles []string
	used := map[uuid.UUID]int{} // value ID → number of reports using it
	for _, r := range reports {
		titles = append(titles, r.Title)
		ids, err := q.ListReportValueIDs(t.Context(), r.ID)
		if err != nil {
			t.Fatal(err)
		}
		for _, id := range ids {
			used[id]++
		}
	}
	slices.Sort(titles)
	if want := []string{"Board deck: financial summary", "Q3 earnings release"}; !slices.Equal(titles, want) {
		t.Fatalf("report titles = %q, want %q", titles, want)
	}

	keyOf := valueKeys(t, pool)
	var shared []string
	for id, n := range used {
		if n == len(reports) {
			shared = append(shared, keyOf[id])
		}
	}
	slices.Sort(shared)
	if want := []string{"cogs.q3.2026", "opex.q3.2026", "rev.q2.2026", "rev.q3.2026"}; !slices.Equal(shared, want) {
		t.Errorf("values used by both reports = %v, want %v", shared, want)
	}
}

func valueKeys(t *testing.T, pool *pgxpool.Pool) map[uuid.UUID]string {
	t.Helper()
	values, err := store.New(pool).ListValues(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	keys := map[uuid.UUID]string{}
	for _, v := range values {
		keys[v.ID] = v.Key
	}
	return keys
}

// The schema leaves these two sum-cell rules to values-api (docs/adr/0005),
// so check that the seed follows them.
func TestSeedSumCells(t *testing.T) {
	t.Parallel()
	pool := dbtest.NewSeeded(t)
	var sums, withoutInputs, inputsOnNonSums int
	err := pool.QueryRow(t.Context(), `
		SELECT
		  (SELECT count(*) FROM report_cells WHERE kind = 'sum'),
		  (SELECT count(*) FROM report_cells c WHERE c.kind = 'sum'
		     AND NOT EXISTS (SELECT 1 FROM report_cell_sum_inputs i WHERE i.sum_cell_id = c.id)),
		  (SELECT count(*) FROM report_cell_sum_inputs i JOIN report_cells c ON c.id = i.sum_cell_id
		     WHERE c.kind <> 'sum')`).Scan(&sums, &withoutInputs, &inputsOnNonSums)
	if err != nil {
		t.Fatal(err)
	}
	if sums != 3 || withoutInputs != 0 || inputsOnNonSums != 0 {
		t.Errorf("sum cells = %d, without inputs = %d, inputs on non-sum cells = %d; want 3, 0, 0",
			sums, withoutInputs, inputsOnNonSums)
	}
}

func TestSeedIsIdempotent(t *testing.T) {
	t.Parallel()
	pool := dbtest.NewSeeded(t)
	counts := func() map[string]int {
		t.Helper()
		got := map[string]int{}
		for _, table := range wantTables {
			var n int
			if err := pool.QueryRow(t.Context(), "SELECT count(*) FROM "+table).Scan(&n); err != nil {
				t.Fatal(err)
			}
			got[table] = n
		}
		return got
	}
	before := counts()
	if err := db.Seed(t.Context(), pool, slog.New(slog.DiscardHandler)); err != nil {
		t.Fatalf("second seed: %v", err)
	}
	if after := counts(); !maps.Equal(before, after) {
		t.Errorf("row counts changed on reseeding: before %v, after %v", before, after)
	}
	want := map[string]int{
		"users": 3, "source_values": 12, "reports": 2, "report_cells": 17,
		"report_cell_sum_inputs": 9, "value_changes": 0,
	}
	if !maps.Equal(before, want) {
		t.Errorf("row counts = %v, want %v", before, want)
	}
}
