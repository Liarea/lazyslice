// SPDX-License-Identifier: Apache-2.0

//go:build integration

package pg

import (
	"context"
	"testing"

	"github.com/Liarea/lazyslice/internal/dsn"
	"github.com/Liarea/lazyslice/internal/testutil"
)

// T-0402 (the 2026-09-25 JSON red team's A11, docs/reviews/2026-09-25-
// redteam-json/round1.json entry 14): a real server, not a fake, is what pgx's
// own JSON codec actually does with a *any destination — measured once here,
// directly, rather than trusted from reading pgx's source. Before jsonTextRows
// existed, this same query returned a Go float64 for p1, rounded to
// 4.0001234567890125e+18; the exact digits were already gone by the time any
// caller saw the value.
func TestSourceReaderHandsBackJSONAsRawText(t *testing.T) {
	ctx := context.Background()
	testutil.SkipWithoutDocker(ctx, t)

	url := testutil.Postgres(ctx, t, "")
	const pan19 = "4000123456789012343" // Luhn-valid, 19 digits: a float64 rounds this
	execOn(ctx, t, url,
		`CREATE TABLE public.t0402 (id int PRIMARY KEY, w jsonb, n jsonb)`,
		`INSERT INTO public.t0402 VALUES (1, '{"p1":`+pan19+`}', '3'), (2, NULL, NULL)`,
	)

	src, err := OpenSource(ctx, dsn.DSN(url),
		Shape{Name: "test.t0402_select", SQL: `SELECT w, n FROM public.t0402 ORDER BY id`},
	)
	if err != nil {
		t.Fatalf("opening the source: %v", err)
	}
	defer src.Close()

	r, err := src.Short(ctx)
	if err != nil {
		t.Fatalf("Short: %v", err)
	}
	defer r.Close(ctx)

	rows, err := r.Query(ctx, `SELECT w, n FROM public.t0402 ORDER BY id`)
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	defer rows.Close()

	var got []any
	for rows.Next() {
		var w, n any
		if err := rows.Scan(&w, &n); err != nil {
			t.Fatalf("Scan: %v", err)
		}
		got = append(got, w, n)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows.Err: %v", err)
	}
	if len(got) != 4 {
		t.Fatalf("scanned %d values, want 4 (two columns, two rows)", len(got))
	}

	w1, ok := got[0].(string)
	if !ok {
		t.Fatalf("row 1's w = %T, want string (the source's own raw text)", got[0])
	}
	want := `{"p1": ` + pan19 + `}`
	if w1 != want {
		t.Errorf("row 1's w = %q, want %q: a rounded spelling means jsonTextRows did not run", w1, want)
	}
	n1, ok := got[1].(string)
	if !ok || n1 != "3" {
		t.Errorf("row 1's n = %#v, want the raw text \"3\"", got[1])
	}
	if got[2] != nil {
		t.Errorf("row 2's w = %#v, want nil: a NULL json value must stay nil through a *[]byte scan", got[2])
	}
	if got[3] != nil {
		t.Errorf("row 2's n = %#v, want nil", got[3])
	}
}
