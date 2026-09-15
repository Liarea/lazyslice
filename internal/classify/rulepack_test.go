// SPDX-License-Identifier: Apache-2.0

package classify

import (
	"fmt"
	"testing"
)

// TestMatchColumnIsTableScoped exercises matchColumn directly, rather than
// only through Classify (supabase_misses_test.go), against the embedded
// pack's refresh_token_parent rule (T-0119): a table-scoped rule must fire in
// a matching table, must not fire in a non-matching one, and must not change
// how an ordinary, unscoped patterns: rule behaves in either table. This is
// the reviewer-requested direct pin: deleting the `pat.tableRe != nil &&
// !pat.tableRe.MatchString(normalisedTable)` gate in matchColumn (making
// refresh_token_parent a global credential rule) fails this test even though
// it left every other test in the package green.
func TestMatchColumnIsTableScoped(t *testing.T) {
	t.Parallel()
	p, err := pack()
	if err != nil {
		t.Fatalf("rule pack: %v", err)
	}

	t.Run("table-scoped rule fires in a matching table", func(t *testing.T) {
		got, ok := p.matchColumn(normaliseName("refresh_tokens"), normaliseName("parent"))
		if !ok {
			t.Fatalf("matchColumn(refresh_tokens, parent) = no match, want a hit from refresh_token_parent")
		}
		if got.Name != "refresh_token_parent" {
			t.Errorf("matchColumn(refresh_tokens, parent) matched %q, want refresh_token_parent", got.Name)
		}
	})

	t.Run("table-scoped rule does not fire in a non-matching table", func(t *testing.T) {
		for _, table := range []string{"categories", "comments", "widgets"} {
			_, ok := p.matchColumn(normaliseName(table), normaliseName("parent"))
			if ok {
				got, _ := p.matchColumn(normaliseName(table), normaliseName("parent"))
				t.Errorf("matchColumn(%s, parent) matched %q, want no match: the table: scope should have excluded this table", table, got.Name)
			}
		}
	})

	t.Run("an ordinary patterns: rule still fires in both tables", func(t *testing.T) {
		// "email" has no table_patterns: entry, so it must match identically
		// regardless of which table it is asked about — a table-scoped rule
		// merged into ColumnPatterns must not change an unscoped rule's reach.
		for _, table := range []string{"refresh_tokens", "categories", "users"} {
			got, ok := p.matchColumn(normaliseName(table), normaliseName("email"))
			if !ok {
				t.Fatalf("matchColumn(%s, email) = no match, want the email name rule to fire", table)
			}
			if got.Category != "email" {
				t.Errorf("matchColumn(%s, email) matched category %q, want email", table, got.Category)
			}
		}
	})
}

// TestLoadPackRejectsEmptyTableScopeRegexps is the medium review finding for
// T-0119: regexp.Compile("") succeeds and matches every string, so an omitted
// `table:` or `match:` field in a table_patterns: row must be rejected by
// name rather than silently compiling into a rule that applies everywhere (an
// empty table:) or to every column of a matching table (an empty match:).
func TestLoadPackRejectsEmptyTableScopeRegexps(t *testing.T) {
	t.Parallel()

	const base = `
version: "test"
categories:
  - category: credential
    masker: "fixed:redacted"
    accepts: ["*"]
patterns:
  - name: some_pattern
    category: credential
    priority: 10
    match: "^unrelated$"
table_patterns:
  - name: bad_rule
    category: credential
    priority: 80
    table: %q
    match: %q
`

	t.Run("empty table:", func(t *testing.T) {
		_, err := decodePack([]byte(fmt.Sprintf(base, "", "^parents?$")))
		if err == nil {
			t.Fatalf("decodePack accepted a table_patterns row with an empty table: regexp, which would compile and match every table")
		}
	})

	t.Run("empty match:", func(t *testing.T) {
		_, err := decodePack([]byte(fmt.Sprintf(base, "^refresh_tokens$", "")))
		if err == nil {
			t.Fatalf("decodePack accepted a table_patterns row with an empty match: regexp, which would compile and match every column")
		}
	})

	t.Run("both set compiles cleanly, as a control", func(t *testing.T) {
		_, err := decodePack([]byte(fmt.Sprintf(base, "^refresh_tokens$", "^parents?$")))
		if err != nil {
			t.Fatalf("decodePack rejected a well-formed table_patterns row: %v", err)
		}
	})
}
