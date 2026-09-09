// SPDX-License-Identifier: Apache-2.0

package plan

import (
	"errors"
	"strings"
	"testing"

	"github.com/Liarea/lazyslice/internal/event"
	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
	"github.com/Liarea/lazyslice/mask"
)

// The unique-index domain rule, held without a database.
//
// checkUniqueDomain has exactly two outcomes and both of them matter to somebody
// reading a failed run: either a wider generator for the category clears
// d_required and the decision is rewritten to name it, or none does and the run
// stops at exit 12 with d, d_required and the escapes. Before this check landed
// the first anyone heard of either was a 23505 in the loader with every row
// already moved (unique.go's header lists the three torture schemas it happened
// on), so a revert has to fail something that runs on every change, not only
// `make torture`.
//
// It is called directly rather than through Plan because n is the *planned* row
// count: a Plan over a reader with no rows selects nothing, plannedRows is 0 and
// the check correctly skips every table. `lookups`/`lookupRows` is the shortest
// honest way to give a table a planned row count — a lookup is copied whole, so
// its n is the measured count — and it exercises the same plannedRows branch a
// real lookup table takes.
func uniqueRun(cat pipeline.Category, id mask.ID, typeName string, typmod int32, rows int64) (*run, ref.TableRef) {
	t := ref.TableRef{Schema: "public", Name: "contact"}
	col := ref.ColumnRef{Table: t, Column: "handle"}
	table := pipeline.Table{
		Ref: t,
		Columns: []pipeline.Column{
			{Name: "id", TypeName: "bigint", TypeOID: 20},
			{Name: "handle", TypeName: typeName, TypMod: typmod},
		},
		PK: []string{"id"},
	}
	return &run{
		schema: &pipeline.Schema{Tables: []pipeline.Table{table}},
		tables: []pipeline.Table{table},
		cls: &pipeline.Classification{Decisions: map[ref.ColumnRef]pipeline.Decision{
			col: {Col: col, Category: cat, Masker: id, Masked: true, UniqueIndex: true},
		}},
		lookups:    map[ref.TableRef]bool{t: true},
		lookupRows: map[ref.TableRef]int64{t: rows},
		selected:   map[ref.TableRef]*keys{},
	}, t
}

// TestUniqueDomainPicksTheWiderMasker is the first outcome: `phone` on a unique
// column becomes `phone_unique`, which is the whole reason mask registers two
// maskers for the category.
func TestUniqueDomainPicksTheWiderMasker(t *testing.T) {
	t.Parallel()
	p, tbl := uniqueRun(pipeline.CatPhone, mask.MaskerPhone, "text", -1, 200)

	if err := p.checkUniqueDomain(); err != nil {
		t.Fatalf("checkUniqueDomain refused a phone column mask can widen: %v", err)
	}
	d := p.cls.Decisions[ref.ColumnRef{Table: tbl, Column: "handle"}]
	if d.Masker == mask.MaskerPhone {
		t.Fatalf("the decision still names %q; a column under a unique index must be given the widest masker for its category", d.Masker)
	}
	if d.Masker != mask.MaskerPhoneUnique {
		t.Errorf("Masker = %q, want %q", d.Masker, mask.MaskerPhoneUnique)
	}
	if d.Category != pipeline.CatPhone {
		t.Errorf("Category = %q; the check chooses within the category and never changes it", d.Category)
	}
}

// TestUniqueCredentialColumnEscalates is T-0098's half of the first outcome, and
// it is the one an authentication schema is made of. `credential` used to have
// one masker — the fixed literal `$lazyslice$invalid`, domain 1 — so every
// column under a unique index that classified as a credential was refused here
// at every row count, and eighteen of the thirty-seven `--unmask` flags
// internal/invariants/torture_catalogue_test.go carries were that (they are
// tagged `(T-0098)`; stripping them and re-measuring docs/TORTURE.md's split is
// tracker T-0112). `mask` now registers `credential_unique`
// as the category's alternate and this check reaches it.
func TestUniqueCredentialColumnEscalates(t *testing.T) {
	t.Parallel()
	p, tbl := uniqueRun(pipeline.CatCredential, mask.CredentialMasker, "text", -1, 200)

	if err := p.checkUniqueDomain(); err != nil {
		t.Fatalf("checkUniqueDomain still refuses a unique credential column: %v", err)
	}
	d := p.cls.Decisions[ref.ColumnRef{Table: tbl, Column: "handle"}]
	if d.Masker != mask.MaskerCredentialUnique {
		t.Errorf("Masker = %q, want %q", d.Masker, mask.MaskerCredentialUnique)
	}
	if d.Category != pipeline.CatCredential {
		t.Errorf("Category = %q; the check chooses within the category and never changes it", d.Category)
	}
}

// TestUniqueDomainRefusesWhenNoMaskerFits is the second outcome, and it is the
// one an operator reads. A `varchar(18)` credential column has no room for
// `credential_unique`'s prefix and a single suffix symbol, so the widest
// generator the category has left is the fixed literal, whose domain is 1: no
// row count and no generator makes that column unique, and the refusal has to
// say so rather than print a `--take` that would not help.
func TestUniqueDomainRefusesWhenNoMaskerFits(t *testing.T) {
	t.Parallel()
	// typmod 22 is varchar(18): len("lazyslice-invalid-") with nothing after it.
	p, tbl := uniqueRun(pipeline.CatCredential, mask.CredentialMasker, "character varying(18)", 22, 200)

	err := p.checkUniqueDomain()
	var refusal *Refusal
	if !errors.As(err, &refusal) {
		t.Fatalf("checkUniqueDomain returned %v, want a *plan.Refusal", err)
	}
	if refusal.Code != CodeUniqueDomain {
		t.Errorf("Code = %q, want %q", refusal.Code, CodeUniqueDomain)
	}
	if refusal.Exit != 12 {
		t.Errorf("Exit = %d, want 12", refusal.Exit)
	}
	// §5's sentence: d, d_required, and the escapes. A refusal that named none
	// of them leaves the operator with a stopped run and no next step.
	msg := refusal.Error()
	for _, want := range []string{
		"public.contact.handle",
		"credential",
		"1 distinct values",
		"200 row(s)",
		"no row count is small enough",
		"--unmask public.contact.handle=REASON",
		"mapping_file",
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("message %q does not carry %q", msg, want)
		}
	}
	if got := refusal.Args[event.ArgTable]; got != tbl.String() {
		t.Errorf("args[table] = %q, want %q", got, tbl)
	}
	if got := refusal.Args[event.ArgColumn]; got != "handle" {
		t.Errorf("args[column] = %q, want %q", got, "handle")
	}
	if got := refusal.Args[event.ArgCount]; got != "200" {
		t.Errorf("args[count] = %q, want the planned row count", got)
	}
}

// TestUniqueDomainSkipsATableThisRunWillNotLoad is the guard against refusing on
// the source's shape rather than on the plan's: a table with no planned rows has
// nothing in the target to collide, and refusing on it would stop a run over a
// column the run never writes.
func TestUniqueDomainSkipsATableThisRunWillNotLoad(t *testing.T) {
	t.Parallel()
	p, _ := uniqueRun(pipeline.CatCredential, mask.CredentialMasker, "text", -1, 0)
	p.lookups = map[ref.TableRef]bool{}
	p.lookupRows = map[ref.TableRef]int64{}

	if err := p.checkUniqueDomain(); err != nil {
		t.Fatalf("checkUniqueDomain refused a table this run puts no rows in: %v", err)
	}
}
