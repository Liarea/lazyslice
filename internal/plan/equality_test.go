// SPDX-License-Identifier: Apache-2.0

package plan

import (
	"errors"
	"strings"
	"testing"

	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
	"github.com/Liarea/lazyslice/mask"
)

// The foreign-key equality group, held without a database (T-0132).
//
// This is finding 3 of docs/reviews/2026-09-09/REVIEW.md as a unit test. The
// evidence for the rule is otherwise one Docker-gated regression
// (testdata/regressions/010-fk-connected-columns-mask-differently.sql), which
// nothing on the path a change actually takes runs, and the failure it guards
// against is silent at plan: the run exits 8 in the loader with the foreign key
// unvalidatable, one stage after the mistake was made.
//
// checkUniqueDomain is called directly for the reason unique_test.go states: n
// is the *planned* row count, and a Plan over a reader with no rows plans none.

// fkRun builds the review's two tables: a parent whose masked column is under a
// unique index, a child that references it and is not, and one declared
// foreign key between them. Both columns are classified `credential`, which is
// what the review's probe produced from the column name `token`.
func fkRun(parentType string, parentTypmod int32, childType string, childTypmod int32, parentUnique bool, rows int64) (*run, ref.ColumnRef, ref.ColumnRef) {
	parent := ref.TableRef{Schema: "public", Name: "tokens"}
	child := ref.TableRef{Schema: "public", Name: "items"}
	pcol := ref.ColumnRef{Table: parent, Column: "token"}
	ccol := ref.ColumnRef{Table: child, Column: "token"}

	tables := []pipeline.Table{
		{
			Ref: child,
			Columns: []pipeline.Column{
				{Name: "id", TypeName: "bigint", TypeOID: 20},
				{Name: "token", TypeName: childType, TypMod: childTypmod},
			},
			PK: []string{"id"},
		},
		{
			Ref: parent,
			Columns: []pipeline.Column{
				{Name: "id", TypeName: "bigint", TypeOID: 20},
				{Name: "token", TypeName: parentType, TypMod: parentTypmod},
			},
			PK: []string{"id"},
		},
	}
	fk := pipeline.ForeignKey{
		Name:       "items_token_fkey",
		Child:      child,
		ChildCols:  []string{"token"},
		Parent:     parent,
		ParentCols: []string{"token"},
		Validated:  true,
	}
	return &run{
		schema: &pipeline.Schema{Tables: tables, FKs: []pipeline.ForeignKey{fk}},
		tables: tables,
		fks:    []pipeline.ForeignKey{fk},
		cls: &pipeline.Classification{Decisions: map[ref.ColumnRef]pipeline.Decision{
			pcol: {
				Col: pcol, Category: pipeline.CatCredential,
				Masker: mask.CredentialMasker, Masked: true, UniqueIndex: parentUnique,
			},
			ccol: {
				Col: ccol, Category: pipeline.CatCredential,
				Masker: mask.CredentialMasker, Masked: true,
			},
		}},
		lookups:    map[ref.TableRef]bool{parent: true, child: true},
		lookupRows: map[ref.TableRef]int64{parent: rows, child: rows},
		selected:   map[ref.TableRef]*keys{},
	}, pcol, ccol
}

// TestEqualityGroupEscalatesBothEndsOfAKey is the defect. The parent is under a
// unique index, so §5 escalates it to credential_unique; the child is not, and
// before T-0132 it kept the category default, the fixed literal. One value, two
// outputs, and the load ended at exit 8 (evidence/fk_masker.log). Both ends must
// now name the same generator.
func TestEqualityGroupEscalatesBothEndsOfAKey(t *testing.T) {
	t.Parallel()
	p, pcol, ccol := fkRun("text", -1, "text", -1, true, 200)

	if err := p.checkUniqueDomain(); err != nil {
		t.Fatalf("checkUniqueDomain refused a group both ends of which credential_unique fits: %v", err)
	}
	parent := p.cls.Decisions[pcol]
	childD := p.cls.Decisions[ccol]
	if parent.Masker != mask.MaskerCredentialUnique {
		t.Errorf("%s masker = %q, want %q", pcol, parent.Masker, mask.MaskerCredentialUnique)
	}
	if childD.Masker != parent.Masker {
		t.Errorf("%s masker = %q and %s masker = %q; the two ends of a foreign key must mask alike or the "+
			"key does not validate", ccol, childD.Masker, pcol, parent.Masker)
	}
}

// withChecks puts a CHECK expression on each end's masked column. The two ends
// of the group are `public.items.token` (the child) and `public.tokens.token`
// (the parent), in the order fkRun builds them.
func withChecks(p *run, childCheck, parentCheck string) {
	for i := range p.tables {
		for j := range p.tables[i].Columns {
			if p.tables[i].Columns[j].Name != "token" {
				continue
			}
			if p.tables[i].Ref.Name == "items" {
				p.tables[i].Columns[j].Checks = []string{childCheck}
			} else {
				p.tables[i].Columns[j].Checks = []string{parentCheck}
			}
		}
	}
}

// TestEqualityGroupRefusesTwoClosedColumnsWithDifferentLabels is finding 1 of
// the T-0132 review. Every generator answers a closed column — an enum, a CHECK
// with a value list — with one of *its own* labels (mask/domain.go's
// labelValue), so two members whose lists differ mask one input to two different
// values. Both lists here have two members, so Domain() is 2 at each end and the
// count check that was written to catch exactly this failure mode passes it
// through; the group has to be refused on the labels themselves.
func TestEqualityGroupRefusesTwoClosedColumnsWithDifferentLabels(t *testing.T) {
	t.Parallel()
	p, pcol, ccol := fkRun("text", -1, "text", -1, false, 10)
	withChecks(p,
		`CHECK ((token = ANY (ARRAY['a'::text, 'b'::text])))`,
		`CHECK ((token = ANY (ARRAY['c'::text, 'd'::text])))`)

	err := p.checkUniqueDomain()
	var refusal *Refusal
	if !errors.As(err, &refusal) {
		t.Fatalf("checkUniqueDomain returned %v, want a *plan.Refusal: two CHECK lists of "+
			"equal length and different contents mask one value to two different labels", err)
	}
	if refusal.Code != CodeEqualityGroup {
		t.Errorf("Code = %q, want %q", refusal.Code, CodeEqualityGroup)
	}
	msg := refusal.Error()
	for _, want := range []string{pcol.String(), ccol.String(), "labels"} {
		if !strings.Contains(msg, want) {
			t.Errorf("message %q does not carry %q", msg, want)
		}
	}
	if strings.Contains(msg, "unique index") {
		t.Errorf("message %q claims a unique index; no column in this group is under one", msg)
	}
}

// TestEqualityGroupAdmitsTwoClosedColumnsWithTheSameLabels is the other half:
// the check must not refuse a group whose members accept the same declared
// values, or every enum and CHECK-listed column at either end of a key becomes a
// plan refusal.
func TestEqualityGroupAdmitsTwoClosedColumnsWithTheSameLabels(t *testing.T) {
	t.Parallel()
	p, pcol, ccol := fkRun("text", -1, "text", -1, false, 10)
	same := `CHECK ((token = ANY (ARRAY['a'::text, 'b'::text])))`
	withChecks(p, same, same)

	if err := p.checkUniqueDomain(); err != nil {
		t.Fatalf("checkUniqueDomain refused a group whose members accept the same labels: %v", err)
	}
	for _, c := range []ref.ColumnRef{pcol, ccol} {
		if got := p.cls.Decisions[c].Masker; got != mask.CredentialMasker {
			t.Errorf("%s masker = %q, want the category default %q", c, got, mask.CredentialMasker)
		}
	}
}

// TestEqualityGroupWithNoUniqueMemberKeepsTheDefault is the other side, and it
// is what stops the rule escalating the whole schema: the widest generator *any
// member needs* is the category default when no member is under a unique index,
// not the widest generator the category has.
func TestEqualityGroupWithNoUniqueMemberKeepsTheDefault(t *testing.T) {
	t.Parallel()
	p, pcol, ccol := fkRun("text", -1, "text", -1, false, 200)

	if err := p.checkUniqueDomain(); err != nil {
		t.Fatalf("checkUniqueDomain refused a group nothing needed widening: %v", err)
	}
	for _, c := range []ref.ColumnRef{pcol, ccol} {
		if got := p.cls.Decisions[c].Masker; got != mask.CredentialMasker {
			t.Errorf("%s masker = %q, want the category default %q", c, got, mask.CredentialMasker)
		}
	}
}

// TestEqualityGroupRefusesWhenNoMaskerFitsEveryMember is the refusal. The
// parent needs credential_unique; the child is a varchar(20), which has room for
// the prefix and two suffix symbols and no more, so the same generator emits a
// different number of values at each end and equal inputs would still mask
// differently. §5's amendment says that is exit 12 naming the group, at plan,
// rather than exit 8 in the loader.
func TestEqualityGroupRefusesWhenNoMaskerFitsEveryMember(t *testing.T) {
	t.Parallel()
	// typmod 24 is varchar(20): len("lazyslice-invalid-") plus two symbols.
	p, pcol, ccol := fkRun("text", -1, "character varying(20)", 24, true, 200)

	err := p.checkUniqueDomain()
	var refusal *Refusal
	if !errors.As(err, &refusal) {
		t.Fatalf("checkUniqueDomain returned %v, want a *plan.Refusal", err)
	}
	// The code is the group's own and not plan.refused.unique_domain, whose
	// template asserts a unique index: this branch also fires with no member
	// under one (T-0132 review, finding 3).
	if refusal.Code != CodeEqualityGroup {
		t.Errorf("Code = %q, want %q", refusal.Code, CodeEqualityGroup)
	}
	if refusal.Exit != exitPlan {
		t.Errorf("Exit = %d, want %d", refusal.Exit, exitPlan)
	}
	// The refusal has to name the whole group, and the escape has to say that
	// --unmask covers every column of it: unmasking one end of the key leaves
	// that end's real values in the target and the key still unvalidatable
	// (T-0132 review, finding 2).
	msg := refusal.Error()
	for _, want := range []string{
		pcol.String(), ccol.String(), "joined by foreign keys",
		"--unmask every column of the group", "all of them or none",
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("message %q does not carry %q", msg, want)
		}
	}
	// And it must not send the operator to change a number that is not the
	// cause: this group is refused for a length mismatch, not for its row count.
	if strings.Contains(msg, "lower the row count") {
		t.Errorf("message %q offers the row-count escape for a refusal rows did not cause", msg)
	}
	// And nothing may have been written back: a refused group leaves the
	// classification as it found it.
	for _, c := range []ref.ColumnRef{pcol, ccol} {
		if got := p.cls.Decisions[c].Masker; got != mask.CredentialMasker {
			t.Errorf("%s masker = %q after a refusal; a refused group is not rewritten", c, got)
		}
	}
}
