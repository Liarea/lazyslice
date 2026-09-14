// SPDX-License-Identifier: Apache-2.0

package verify

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
)

// The catalog pass (catalog.go, T-0134), held without a database.
//
// It is the one check here whose subject is not a row, so none of this file's
// other doubles can carry it: the target has to answer two catalog statements
// with five columns each.

// catalogRows is a five-column result set of catalog objects.
type catalogRows struct {
	rows [][5]string
	i    int
}

func (r *catalogRows) Next() bool { r.i++; return r.i <= len(r.rows) }
func (r *catalogRows) Scan(dest ...any) error {
	if len(dest) != 5 {
		return errors.New("catalogRows: five columns")
	}
	for i, d := range dest {
		p, ok := d.(*string)
		if !ok {
			return errors.New("catalogRows: want *string")
		}
		*p = r.rows[r.i-1][i]
	}
	return nil
}
func (r *catalogRows) Err() error { return nil }
func (r *catalogRows) Close()     {}

// catalogTarget answers each of the three catalog statements with its own rows,
// so a test can say which object class carried the literal.
type catalogTarget struct{ defaults, constraints, indexes [][5]string }

func (c catalogTarget) Query(_ context.Context, sql string, _ ...any) (pipeline.Rows, error) {
	switch {
	case strings.Contains(sql, "pg_attrdef"):
		return &catalogRows{rows: c.defaults}, nil
	case strings.Contains(sql, "pg_index"):
		return &catalogRows{rows: c.indexes}, nil
	}
	return &catalogRows{rows: c.constraints}, nil
}

// maskingItems is the classification finding 5's schema produces: the email
// column of public.items is masked, and nothing else is.
func maskingItems() *pipeline.Classification {
	col := ref.ColumnRef{
		Table:  ref.TableRef{Schema: "public", Name: "items"},
		Column: "email",
	}
	return &pipeline.Classification{Decisions: map[ref.ColumnRef]pipeline.Decision{
		col: {Col: col, Category: pipeline.CatEmail, Masker: "email", Masked: true},
	}}
}

// rewrittenItems is the schema after internal/plan masked the default of
// public.items.email: the planner records the catalog's own text on
// Column.DefaultOriginal and leaves the masked text on Column.Default
// (internal/plan/ddlliteral.go's columnDefault), and core hands this same schema
// to verify. It is what tells this pass that the address now in pg_attrdef is
// the masker's and not the source's.
func rewrittenItems() map[ref.TableRef]*pipeline.Table {
	t := &pipeline.Table{
		Ref: ref.TableRef{Schema: "public", Name: "items"},
		Columns: []pipeline.Column{
			{Name: "id"},
			{
				Name:            "email",
				Default:         "'wxvh7m2q@example.net'::text",
				DefaultOriginal: "'ddl.canary@example.org'::text",
			},
		},
	}
	return map[ref.TableRef]*pipeline.Table{t.Ref: t}
}

// optedOutItems is the same column under an operator's written opt-out.
func optedOutItems() *pipeline.Classification {
	cls := maskingItems()
	for col, d := range cls.Decisions {
		d.Masked = false
		d.Source = pipeline.ByFlagUnmask
		cls.Decisions[col] = d
	}
	return cls
}

func TestTheCatalogPassFindsALiteralNoRowScanCanSee(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		target   catalogTarget
		cls      *pipeline.Classification
		tables   map[ref.TableRef]*pipeline.Table
		wantFail bool
	}{
		{
			name: "the review's own default",
			target: catalogTarget{defaults: [][5]string{
				{"public", "items", "email", "default", "'ddl.canary@example.org'::text"},
			}},
			wantFail: true,
		},
		{
			// The central case of the rule, and the one this pass used to make
			// unreachable: internal/plan masked the default through the
			// column's own masker, and an email masker's output is a valid
			// address, so a pass that judged it on its shape alone failed exit 9
			// on the run that did the right thing (catalogExempt).
			name: "a masked column's masked default is not a hit",
			target: catalogTarget{defaults: [][5]string{
				{"public", "items", "email", "default", "'wxvh7m2q@example.net'::text"},
			}},
			cls:    maskingItems(),
			tables: rewrittenItems(),
		},
		{
			// And the exemption is the *rewrite's*, not the decision's
			// (T-0134's review round). Until internal/core fills
			// pipeline.PlanRequest.Key (tracker T-0161) the planner rewrites no
			// default at all, so a masked column whose pg_attrdef still holds
			// the source's own address must be exit 9 here rather than exempt:
			// an arm keyed on d.Masked alone would stand open over the whole
			// class and exempt an object nothing ever rewrote.
			name: "a masked column's unrewritten default still is",
			target: catalogTarget{defaults: [][5]string{
				{"public", "items", "email", "default", "'ddl.canary@example.org'::text"},
			}},
			cls:      maskingItems(),
			wantFail: true,
		},
		{
			// ARCHITECTURE.md §8's escape has to mean the same thing at both
			// ends: internal/plan does not refuse this literal either.
			name: "an opted-out column's default is not a hit",
			target: catalogTarget{defaults: [][5]string{
				{"public", "items", "email", "default", "'ddl.canary@example.org'::text"},
			}},
			cls: optedOutItems(),
		},
		{
			// The exemption is for the column's own DEFAULT and nothing else: a
			// CHECK on a masked column is never rewritten, so a strong hit in
			// one is still the source's literal.
			name: "a CHECK on the masked column is still a hit",
			target: catalogTarget{constraints: [][5]string{
				{"public", "items", "items_email_check", "constraint",
					"CHECK ((email <> 'ddl.canary@example.org'::text))"},
			}},
			cls:      maskingItems(),
			wantFail: true,
		},
		{
			// The route internal/plan does not walk at all (T-0163): a partial
			// index's predicate has no pg_constraint row, and load/ddl replays
			// the index verbatim.
			name: "a partial index predicate carrying an address is",
			target: catalogTarget{indexes: [][5]string{
				{"public", "items", "items_email_idx", "index predicate",
					"(email = 'ddl.canary@example.org'::text)"},
			}},
			cls:      maskingItems(),
			wantFail: true,
		},
		{
			name: "an ordinary default and an ordinary CHECK are not",
			target: catalogTarget{
				defaults: [][5]string{
					{"public", "items", "status", "default", "'pending'::text"},
					{"public", "items", "id", "default", "nextval('public.items_id_seq'::regclass)"},
				},
				constraints: [][5]string{
					{"public", "items", "items_status_check", "constraint",
						"CHECK ((status = ANY (ARRAY['pending'::text, 'done'::text])))"},
				},
			},
		},
		{
			name: "a CHECK carrying an address is",
			target: catalogTarget{constraints: [][5]string{
				{"public", "items", "items_email_check", "constraint",
					"CHECK ((email <> 'ddl.canary@example.org'::text))"},
			}},
			wantFail: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s := &state{target: tc.target, cls: tc.cls, tables: tc.tables}
			if err := s.catalog(context.Background()); err != nil {
				t.Fatalf("catalog: %v", err)
			}
			failed := s.failed(checkCatalog)
			if failed != tc.wantFail {
				t.Fatalf("catalog failed=%v, want %v (checks %+v)", failed, tc.wantFail, s.checks)
			}
			if !failed {
				return
			}
			r := s.firstFailure()
			if r == nil || r.Code != CodeRefusedCatalogLiteral || r.Exit != exitResidual {
				t.Fatalf("catalog produced %+v, want %s exit %d", r, CodeRefusedCatalogLiteral, exitResidual)
			}
			if strings.Contains(r.Reason, "@") {
				t.Fatalf("the refusal quotes the literal: %q (THREAT_MODEL.md T4)", r.Reason)
			}
			for _, c := range s.checks {
				if c.Name == checkCatalog && c.Passed {
					t.Fatalf("a passing catalog check sits beside the refusal: %+v", s.checks)
				}
			}
		})
	}
}
