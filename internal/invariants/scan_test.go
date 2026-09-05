//go:build integration

package invariants

import (
	"context"
	"regexp"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

// The detectors this suite greps with.
//
// They are deliberately not the classifier's: I2 has to be able to fail when
// the classifier is wrong, so it cannot be written in terms of the classifier's
// own opinion of what a personal value is. What it can do is take every
// address and every number that is a phone in the source and assert that none
// of them appears in the target, which is a statement about the masker that
// holds whatever the classifier decided.
var (
	// emailPattern is deliberately narrow: local@label.tld with a real TLD
	// shape. A looser pattern would collect Postgres array literals and JSON
	// fragments and turn I2 into a coin toss.
	emailPattern = regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9](?:[A-Za-z0-9.\-]*[A-Za-z0-9])?\.[A-Za-z]{2,}`)

	// internationalPhonePattern matches a number written with a country code,
	// which is the shape nasty.sql's notes and JSON documents carry
	// ("+44 20 7946 0958"). It requires the leading +, because a bare run of
	// digits is indistinguishable from the surrogate keys the snapshot
	// preserves on purpose (§6 item 6) and would make I2 fail on a correct run.
	internationalPhonePattern = regexp.MustCompile(`\+\d[\d ().\-]{5,}\d`)

	// phoneColumnPattern names the columns whose whole value is a phone number
	// however it is written. pagila's address.phone holds "14033335568", which
	// no safe pattern finds in free text but which is unambiguous here.
	phoneColumnPattern = regexp.MustCompile(`(?i)phone|mobile|msisdn|fax|telephone`)
)

// minPhoneDigits is the shortest run of digits this suite will treat as a
// phone number from a phone-named column. Below it the literal is short enough
// to occur inside an unrelated identifier and the grep would cry wolf.
const minPhoneDigits = 7

// cell is one value of one column of one row, as text.
type cell struct {
	Table  tableRef
	Column string
	Value  string
}

// scanCells reads every non-null value of every column of every ordinary table
// in a database, as text.
//
// It is a full scan on purpose. The target is small by construction, and the
// two fixtures together are about 50,000 rows, so the honest thing is to look
// at all of them rather than to sample and call the result an invariant.
func scanCells(ctx context.Context, t *testing.T, conn *pgx.Conn) []cell {
	t.Helper()

	var out []cell
	for _, ref := range dataTables(ctx, t, conn) {
		cols := columnsOf(ctx, t, conn, ref)
		if len(cols) == 0 {
			continue
		}
		out = append(out, scanTableCells(ctx, t, conn, ref, cols)...)
	}
	return out
}

// scanTableCells reads one table, every column cast to text so that arrays,
// jsonb, inet and enums arrive in the form they are written in.
func scanTableCells(ctx context.Context, t *testing.T, conn *pgx.Conn, ref tableRef, cols []string) []cell {
	t.Helper()

	projected := make([]string, len(cols))
	for i, c := range cols {
		projected[i] = pgx.Identifier{c}.Sanitize() + "::text"
	}
	sql := "SELECT " + strings.Join(projected, ", ") + " FROM " + ref.quoted()

	rows, err := conn.Query(ctx, sql)
	if err != nil {
		t.Fatalf("invariants: reading %s: %v", ref, err)
	}
	defer rows.Close()

	var out []cell
	for rows.Next() {
		values := make([]*string, len(cols))
		dest := make([]any, len(cols))
		for i := range values {
			dest[i] = &values[i]
		}
		if scanErr := rows.Scan(dest...); scanErr != nil {
			t.Fatalf("invariants: reading %s: %v", ref, scanErr)
		}
		for i, v := range values {
			if v == nil || *v == "" {
				continue
			}
			out = append(out, cell{Table: ref, Column: cols[i], Value: *v})
		}
	}
	if rows.Err() != nil {
		t.Fatalf("invariants: reading %s: %v", ref, rows.Err())
	}
	return out
}

// personalLiterals is every email address and phone number found in a
// database, each mapped to the column it was found in so that a hit in the
// target can say where the value came from.
type personalLiterals map[string]string

// collectPersonalLiterals extracts the addresses and numbers I2 greps for.
func collectPersonalLiterals(cells []cell) personalLiterals {
	found := personalLiterals{}
	for _, c := range cells {
		for _, m := range emailPattern.FindAllString(c.Value, -1) {
			found.add(m, c)
		}
		for _, m := range internationalPhonePattern.FindAllString(c.Value, -1) {
			found.add(strings.TrimSpace(m), c)
		}
		if phoneColumnPattern.MatchString(c.Column) && digitCount(c.Value) >= minPhoneDigits {
			found.add(strings.TrimSpace(c.Value), c)
		}
	}
	return found
}

func (p personalLiterals) add(literal string, from cell) {
	if len(literal) < 4 {
		return
	}
	if _, seen := p[literal]; seen {
		return
	}
	p[literal] = from.Table.String() + "." + from.Column
}

func digitCount(s string) int {
	n := 0
	for _, r := range s {
		if r >= '0' && r <= '9' {
			n++
		}
	}
	return n
}

// leak is one source literal found again in the target.
type leak struct {
	Table   tableRef
	Column  string
	Source  string // "schema.table.column" the literal was found in on the source
	Literal string
}

// findLeaks greps the target's cells for the source's literals.
//
// It is a substring search, not equality, because §5's promise is that nothing
// survives: an address embedded in a free-text note or in a JSON document is
// the case testdata/README.md traps 16 and 17 exist for, and it would pass an
// equality test.
func findLeaks(targetCells []cell, source personalLiterals) []leak {
	if len(source) == 0 {
		return nil
	}
	var out []leak
	for _, c := range targetCells {
		for literal, from := range source {
			if strings.Contains(c.Value, literal) {
				out = append(out, leak{Table: c.Table, Column: c.Column, Source: from, Literal: literal})
			}
		}
	}
	return out
}
