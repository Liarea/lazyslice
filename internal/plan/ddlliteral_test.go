// SPDX-License-Identifier: Apache-2.0

package plan

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
	"github.com/Liarea/lazyslice/mask"
)

// The recreated-DDL literal rule, held without a database (ddlliteral.go,
// T-0134; ARCHITECTURE.md §11.1's 2026-09-14 amendment).
//
// testdata/regressions/011-masked-column-default-holds-a-literal.sql is the
// end-to-end half and asserts one exit code. These are the four arms of the
// rule, including the one that needed pipeline.PlanRequest.Key filled before
// the CLI could reach it at all (T-0161): a masked column's default really
// being rewritten, to the value mask.Apply gives for that literal.

// literalTable is finding 5's schema: a masked email column carrying an address
// in its DEFAULT, beside a surrogate key and a note.
func literalTable() (ref.TableRef, *pipeline.Schema) {
	t := ref.TableRef{Schema: "public", Name: "items"}
	return t, &pipeline.Schema{
		Tables: []pipeline.Table{{
			Ref: t,
			Columns: []pipeline.Column{
				{Name: "id", TypeName: "bigint", TypeOID: 20},
				{Name: "email", TypeName: "text", Default: "'ddl.canary@example.org'::text"},
				{Name: "note", TypeName: "text"},
			},
			PK: []string{"id"},
		}},
	}
}

// literalNextvalTable is literalTable's shape-unrewritable twin: the same
// column, still text (so checkWriteBack, which only judges the column's type
// against the category, has nothing to say about it), but its DEFAULT calls
// nextval — one of the three shapes defaultIsRewritable declines whatever key
// this run holds, because its literal is a relation name and a masked one is
// a CREATE TABLE that fails after every table has been dropped.
func literalNextvalTable() (ref.TableRef, *pipeline.Schema) {
	t := ref.TableRef{Schema: "public", Name: "items"}
	return t, &pipeline.Schema{
		Tables: []pipeline.Table{{
			Ref: t,
			Columns: []pipeline.Column{
				{Name: "id", TypeName: "bigint", TypeOID: 20},
				{
					Name:     "email",
					TypeName: "text",
					Default:  "COALESCE(nextval('legacy_email_seq'::regclass)::text, 'ddl.canary@example.org')",
				},
				{Name: "note", TypeName: "text"},
			},
			PK: []string{"id"},
		}},
	}
}

func literalKey(b byte) *mask.Key {
	var k mask.Key
	for i := range k {
		k[i] = b
	}
	return &k
}

func planLiterals(t *testing.T, schema *pipeline.Schema, cls *pipeline.Classification, key *mask.Key) error {
	t.Helper()
	root := schema.Tables[0].Ref
	_, err := New().Plan(context.Background(), &countingReader{}, schema, cls,
		pipeline.PlanRequest{Root: &root, Key: key})
	return err
}

// planLiteralsWithRequest is planLiterals for a caller that needs to set a
// field planLiterals does not expose, such as KeyPending — the drift-guard
// test needs a PlanRequest with both Key and KeyPending at their zero value.
func planLiteralsWithRequest(t *testing.T, schema *pipeline.Schema, cls *pipeline.Classification, req pipeline.PlanRequest) error {
	t.Helper()
	_, err := New().Plan(context.Background(), &countingReader{}, schema, cls, req)
	return err
}

func TestAMaskedColumnsDefaultIsMaskedThroughItsOwnMasker(t *testing.T) {
	t.Parallel()
	tbl, schema := literalTable()
	cls := masking(tbl, "email", pipeline.CatEmail, mask.MaskerEmail)
	key := literalKey(0x11)

	if err := planLiterals(t, schema, cls, key); err != nil {
		t.Fatalf("Plan: %v", err)
	}

	got := schema.Tables[0].Columns[1].Default
	if strings.Contains(got, "ddl.canary@example.org") {
		t.Fatalf("the default is still %q: the address the review planted survived into the recreated DDL", got)
	}
	want, err := mask.Apply(*key, mask.Category(pipeline.CatEmail), mask.MaskerEmail,
		mask.Value{Text: "ddl.canary@example.org"}, mask.Constraints{TypeTag: "text"})
	if err != nil {
		t.Fatalf("mask.Apply: %v", err)
	}
	if expect := pipeline.QuoteLiteral(want.Out.Text) + "::text"; got != expect {
		t.Fatalf("the default is %q, want %q: a default masked with anything but the column's own masker "+
			"is not the value a row holding that literal would have", got, expect)
	}
}

func TestAMaskedColumnsDefaultThatCannotBeRewrittenIsRefused(t *testing.T) {
	t.Parallel()
	tbl, schema := literalNextvalTable()
	cls := masking(tbl, "email", pipeline.CatEmail, mask.MaskerEmail)

	// A key exists; the shape (a default calling nextval) is what refuses this
	// one — the literal is a relation name, and a masked one is a CREATE TABLE
	// that fails after every table has been dropped.
	err := planLiterals(t, schema, cls, literalKey(0x99))
	var refusal *Refusal
	if !errors.As(err, &refusal) {
		t.Fatalf("Plan returned %v, want a *plan.Refusal", err)
	}
	if refusal.Code != CodeLiteralNotRewritable || refusal.Exit != exitSchema {
		t.Fatalf("Plan refused with %s exit %d, want %s exit %d",
			refusal.Code, refusal.Exit, CodeLiteralNotRewritable, exitSchema)
	}
	if refusal.Column != "email" {
		t.Fatalf("the refusal names %q, want the column it is about", refusal.Column)
	}
	if strings.Contains(refusal.Message, "ddl.canary@example.org") ||
		strings.Contains(refusal.Args["reason"], "ddl.canary@example.org") {
		t.Fatalf("the refusal quotes the literal: %q / %q (THREAT_MODEL.md T4)",
			refusal.Message, refusal.Args["reason"])
	}
}

// T-0161: a masked default with no key *yet* is not the same finding as one
// this stage can never rewrite. Before this task wired
// pipeline.PlanRequest.Key ahead of the plan stage, a nil key reached here
// only from a direct caller — there was no other way to get one, and the old
// version of this test (over literalTable, the rewritable shape) asserted the
// same exit-13 refusal as the case above. Now a nil key with KeyPending set is
// exactly what internal/core's keyBeforePlan leaves a plan-only run with when
// neither $LAZYSLICE_SECRET nor lazyslice.secret exists, and the CLI must
// still be able to plan such a run: this is what it reports instead of
// refusing. KeyPending is what internal/core sets for that state (2026-09-14
// review, finding 1) — see TestAMaskedDefaultWithNoKeyAndNoKeyPendingIsRefused
// for the case where a nil key is *not* accompanied by it.
func TestAMaskedDefaultWithNoKeyIsPendingNotRefused(t *testing.T) {
	t.Parallel()
	tbl, schema := literalTable()
	cls := masking(tbl, "email", pipeline.CatEmail, mask.MaskerEmail)
	root := schema.Tables[0].Ref

	p, err := New().Plan(context.Background(), &countingReader{}, schema, cls,
		pipeline.PlanRequest{Root: &root, Key: nil, KeyPending: true})
	if err != nil {
		t.Fatalf("Plan with no key: %v", err)
	}
	if want := []string{"public.items.email"}; len(p.PendingKeyDefaults) != 1 || p.PendingKeyDefaults[0] != want[0] {
		t.Fatalf("Plan.PendingKeyDefaults = %v, want %v", p.PendingKeyDefaults, want)
	}
	if got := schema.Tables[0].Columns[1].Default; got != "'ddl.canary@example.org'::text" {
		t.Fatalf("the default changed to %q with no key to mask it with", got)
	}
}

// The drift guard (2026-09-14 review of T-0161, finding 1): a nil Key with
// KeyPending left false is not the plan-only state PendingKeyDefaults exists
// for — internal/core's keyBeforePlan sets KeyPending only for a run it is
// deliberately leaving without a key, and a writing run always resolves one
// before planStage runs. A caller that reaches this stage with neither (a
// bug in internal/core, or a caller that never went through keyBeforePlan at
// all) must not silently recreate the source's literal in the target's DDL;
// it refuses exactly as the "cannot be rewritten" shapes do.
func TestAMaskedDefaultWithNoKeyAndNoKeyPendingIsRefused(t *testing.T) {
	t.Parallel()
	tbl, schema := literalTable()
	cls := masking(tbl, "email", pipeline.CatEmail, mask.MaskerEmail)
	root := schema.Tables[0].Ref

	err := planLiteralsWithRequest(t, schema, cls,
		pipeline.PlanRequest{Root: &root, Key: nil, KeyPending: false})
	var refusal *Refusal
	if !errors.As(err, &refusal) {
		t.Fatalf("Plan returned %v, want a *plan.Refusal", err)
	}
	if refusal.Code != CodeLiteralNotRewritable || refusal.Exit != exitSchema {
		t.Fatalf("Plan refused with %s exit %d, want %s exit %d",
			refusal.Code, refusal.Exit, CodeLiteralNotRewritable, exitSchema)
	}
	if refusal.Column != "email" {
		t.Fatalf("the refusal names %q, want the column it is about", refusal.Column)
	}
}

func TestALiteralInAnUnmaskedColumnsDDLIsRefusedWithTheUnmaskEscape(t *testing.T) {
	t.Parallel()
	_, schema := literalTable()
	// Nothing is masked: the classifier decided `none` on every column, and the
	// address in the DEFAULT is then a literal nobody has looked at.
	cls := &pipeline.Classification{Decisions: map[ref.ColumnRef]pipeline.Decision{}}

	err := planLiterals(t, schema, cls, literalKey(0x22))
	var refusal *Refusal
	if !errors.As(err, &refusal) {
		t.Fatalf("Plan returned %v, want a *plan.Refusal", err)
	}
	if refusal.Code != CodeDDLLiteral || refusal.Exit != exitPlan {
		t.Fatalf("Plan refused with %s exit %d, want %s exit %d",
			refusal.Code, refusal.Exit, CodeDDLLiteral, exitPlan)
	}
	if !strings.Contains(refusal.Args["reason"], "--unmask public.items.email=REASON") {
		t.Fatalf("the refusal's reason is %q and does not print the escape", refusal.Args["reason"])
	}
}

func TestAnUnmaskedColumnThatCarriesAnOptOutIsNotRefused(t *testing.T) {
	t.Parallel()
	tbl, schema := literalTable()
	col := ref.ColumnRef{Table: tbl, Column: "email"}
	cls := &pipeline.Classification{Decisions: map[ref.ColumnRef]pipeline.Decision{
		col: {Col: col, Category: pipeline.CatEmail, Source: pipeline.ByFlagUnmask},
	}}

	if err := planLiterals(t, schema, cls, literalKey(0x33)); err != nil {
		t.Fatalf("Plan: %v: an operator who has said in writing that this column is not a person's "+
			"is not asked again", err)
	}
}

func TestACheckOnAMaskedColumnCarryingAnAddressIsRefusedAndALabelListIsNot(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		def     string
		refused bool
	}{
		{
			name:    "a closed value list is not personal data",
			def:     "CHECK ((status = ANY (ARRAY['active'::text, 'banned'::text])))",
			refused: false,
		},
		{
			name:    "an address in the predicate is",
			def:     "CHECK ((email <> 'ddl.canary@example.org'::text))",
			refused: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tbl, schema := literalTable()
			schema.Tables[0].Columns[1].Default = ""
			schema.Tables[0].Columns = append(schema.Tables[0].Columns,
				pipeline.Column{Name: "status", TypeName: "text"})
			schema.Tables[0].Constraints = []pipeline.Constraint{
				{Name: "items_check", Kind: 'c', Def: tc.def},
			}
			cls := masking(tbl, "email", pipeline.CatEmail, mask.MaskerEmail)
			cls.Decisions[ref.ColumnRef{Table: tbl, Column: "status"}] = pipeline.Decision{
				Col:      ref.ColumnRef{Table: tbl, Column: "status"},
				Category: pipeline.CatFreeText, Masker: mask.MaskerFreeText, Masked: true,
			}

			err := planLiterals(t, schema, cls, literalKey(0x44))
			var refusal *Refusal
			switch {
			case tc.refused && !errors.As(err, &refusal):
				t.Fatalf("Plan returned %v, want a *plan.Refusal", err)
			case tc.refused && refusal.Code != CodeLiteralNotRewritable:
				t.Fatalf("Plan refused with %s, want %s", refusal.Code, CodeLiteralNotRewritable)
			case tc.refused && refusal.Column != "items_check":
				t.Fatalf("the refusal names %q, want the constraint it is about", refusal.Column)
			case !tc.refused && err != nil:
				t.Fatalf("Plan: %v: a CHECK whose literals are labels the masker already honours is not "+
					"a refusal, or most real schemas stop here", err)
			}
		})
	}
}

func TestLiteralsReadsTheFormsTheDeparserWrites(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		expr       string
		want       []string
		rewritable []bool
	}{
		{expr: "'a@b.test'::text", want: []string{"a@b.test"}, rewritable: []bool{true}},
		{expr: "'it''s'::text", want: []string{"it's"}, rewritable: []bool{true}},
		{expr: "nextval('public.items_id_seq'::regclass)", want: []string{"public.items_id_seq"}, rewritable: []bool{true}},
		{expr: `("email" = 'x'::text)`, want: []string{"x"}, rewritable: []bool{true}},
		// A column actually named with a quote in it must not open a literal.
		{expr: `("we''re" = 'x'::text)`, want: []string{"x"}, rewritable: []bool{true}},
		{expr: `E'a\'b'`, want: []string{`a\'b`}, rewritable: []bool{false}},
		{expr: "$q$a@b.test$q$", want: []string{"a@b.test"}, rewritable: []bool{false}},
		{expr: "(length(value) < 100)", want: nil, rewritable: nil},
		// A trailing e on an identifier is not an escape-string prefix.
		{expr: "(value = 'x'::text)", want: []string{"x"}, rewritable: []bool{true}},
	} {
		t.Run(tc.expr, func(t *testing.T) {
			t.Parallel()
			got := pipeline.Literals(tc.expr)
			if len(got) != len(tc.want) {
				t.Fatalf("Literals(%q) found %d literals, want %d: %+v", tc.expr, len(got), len(tc.want), got)
			}
			for i, l := range got {
				if l.Text != tc.want[i] {
					t.Errorf("literal %d is %q, want %q", i, l.Text, tc.want[i])
				}
				if l.Rewritable != tc.rewritable[i] {
					t.Errorf("literal %d rewritable=%v, want %v", i, l.Rewritable, tc.rewritable[i])
				}
				if tc.expr[l.Start:l.End] == "" {
					t.Errorf("literal %d spans nothing in %q", i, tc.expr)
				}
			}
		})
	}
}

// A pattern operand is a shape and not a value, and testdata/nasty.sql is the
// reason this is a rule: CHECK ("EmailAddress" LIKE '%@%.%') carries a literal
// net/mail parses as a valid address (% is an ordinary atext character), so the
// first version of this check refused the whole fixture at exit 12.
func TestAPatternOperandIsNotAValue(t *testing.T) {
	t.Parallel()
	for _, def := range []string{
		`CHECK (("EmailAddress" LIKE '%@%.%'))`,
		`CHECK (("EmailAddress" ~~ '%@%.%'::text))`,
		`CHECK (("EmailAddress" !~~ '%@%.%'::text))`,
		`CHECK (("EmailAddress" ~ '%@%.%'::text))`,
		`CHECK (("EmailAddress" SIMILAR TO '%@%.%'))`,
	} {
		t.Run(def, func(t *testing.T) {
			t.Parallel()
			lits := pipeline.Literals(def)
			if len(lits) != 1 {
				t.Fatalf("Literals(%q) found %d literals, want 1", def, len(lits))
			}
			if !lits[0].Pattern {
				t.Fatalf("the literal in %q is not marked a pattern operand", def)
			}
			if hit := strongHit(lits[0]); hit != "" {
				t.Fatalf("the pattern in %q is read as %s", def, hit)
			}
		})
	}
	// The other direction: an equality against the same text is a value.
	lits := pipeline.Literals(`CHECK (("EmailAddress" <> 'ddl.canary@example.org'::text))`)
	if len(lits) != 1 || lits[0].Pattern {
		t.Fatalf("an equality operand is marked a pattern: %+v", lits)
	}
	if hit := strongHit(lits[0]); hit != string(pipeline.CatEmail) {
		t.Fatalf("the address is read as %q, want email", hit)
	}
}

// The Constraints a default is masked with must be the Constraints the *rows*
// are masked with, and the one that changes a generator's output on an ordinary
// schema is Unique: internal/transform derives it from the table
// (constraints.go's uniqueColumn) and ORs Decision.UniqueIndex on top, and this
// file's uniqueColumn is the copy of that rule. Before it existed, a masked
// unique column with a default took its value from the *non-unique* generator
// while every row took one from the unique generator.
func TestAMaskedDefaultUnderAUniqueIndexIsMaskedAsAUniqueColumn(t *testing.T) {
	t.Parallel()
	tbl, schema := literalTable()
	schema.Tables[0].Indexes = []pipeline.Index{{
		Name: "items_email_key", Columns: []string{"email"}, Unique: true, Immediate: true,
	}}
	cls := masking(tbl, "email", pipeline.CatEmail, mask.MaskerEmail)
	key := literalKey(0x55)

	if err := planLiterals(t, schema, cls, key); err != nil {
		t.Fatalf("Plan: %v", err)
	}

	apply := func(c mask.Constraints) string {
		t.Helper()
		res, err := mask.Apply(*key, mask.Category(pipeline.CatEmail), mask.MaskerEmail,
			mask.Value{Text: "ddl.canary@example.org"}, c)
		if err != nil {
			t.Fatalf("mask.Apply: %v", err)
		}
		return pipeline.QuoteLiteral(res.Out.Text) + "::text"
	}
	unique := apply(mask.Constraints{TypeTag: "text", Unique: true})
	plain := apply(mask.Constraints{TypeTag: "text"})
	if unique == plain {
		t.Fatal("the unique and non-unique generators agree on this literal, so this test proves nothing")
	}
	if got := schema.Tables[0].Columns[1].Default; got != unique {
		t.Fatalf("the default is %q, want %q: a default masked under different constraints than the "+
			"column's rows is a value no row of that column can hold", got, unique)
	}
}

// uniqueColumn is a copy of internal/transform's, which this package may not
// import (internal/CLAUDE.md), so the case list is the contract. Every case here
// is one internal/transform/constraints.go answers the same way.
func TestUniqueColumnIsSpeltAsTransformSpellsIt(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		table pipeline.Table
		want  bool
	}{
		{
			name:  "the whole primary key",
			table: pipeline.Table{PK: []string{"email"}},
			want:  true,
		},
		{
			name:  "half of a composite primary key",
			table: pipeline.Table{PK: []string{"email", "tenant"}},
		},
		{
			name: "alone under a unique index",
			table: pipeline.Table{Indexes: []pipeline.Index{
				{Name: "u", Columns: []string{"email"}, Unique: true},
			}},
			want: true,
		},
		{
			name: "one column of a composite unique index",
			table: pipeline.Table{Indexes: []pipeline.Index{
				{Name: "u", Columns: []string{"email", "tenant"}, Unique: true},
			}},
		},
		{
			name: "alone under a partial unique index",
			table: pipeline.Table{Indexes: []pipeline.Index{
				{Name: "u", Columns: []string{"email"}, Unique: true, Partial: true},
			}},
		},
		{
			name: "alone under an expression index",
			table: pipeline.Table{Indexes: []pipeline.Index{
				{Name: "u", Columns: []string{"email"}, Unique: true, Expression: true},
			}},
		},
		{
			name: "alone under a non-unique index",
			table: pipeline.Table{Indexes: []pipeline.Index{
				{Name: "i", Columns: []string{"email"}},
			}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := uniqueColumn(&tc.table, "email"); got != tc.want {
				t.Fatalf("uniqueColumn = %v, want %v", got, tc.want)
			}
		})
	}
	if uniqueColumn(nil, "email") {
		t.Fatal("uniqueColumn(nil) is true")
	}
}

// The rewrite mutates the *pipeline.Schema it is given, so it has to be
// idempotent on it: internal/tui re-plans against a cached schema and
// plan_integration_test.go plans twice over one snapshot to compare two plans.
// Without Column.DefaultOriginal the second plan masked the first plan's output
// and the default became mask(mask(x)).
func TestPlanningTwiceOverOneSchemaProducesOneDefault(t *testing.T) {
	t.Parallel()
	tbl, schema := literalTable()
	cls := masking(tbl, "email", pipeline.CatEmail, mask.MaskerEmail)
	key := literalKey(0x66)

	if err := planLiterals(t, schema, cls, key); err != nil {
		t.Fatalf("Plan: %v", err)
	}
	first := schema.Tables[0].Columns[1].Default
	if err := planLiterals(t, schema, cls, key); err != nil {
		t.Fatalf("Plan (second): %v", err)
	}
	if got := schema.Tables[0].Columns[1].Default; got != first {
		t.Fatalf("a second plan over the same schema produced %q, want %q: the rewrite masked its own "+
			"output", got, first)
	}
	if got := schema.Tables[0].Columns[1].DefaultOriginal; got != "'ddl.canary@example.org'::text" {
		t.Fatalf("DefaultOriginal is %q, want the catalog's own text", got)
	}
	// And a re-plan under a different key is that key's value, not a
	// composition of the two.
	other := literalKey(0x77)
	if err := planLiterals(t, schema, cls, other); err != nil {
		t.Fatalf("Plan (rekeyed): %v", err)
	}
	want, err := mask.Apply(*other, mask.Category(pipeline.CatEmail), mask.MaskerEmail,
		mask.Value{Text: "ddl.canary@example.org"}, mask.Constraints{TypeTag: "text"})
	if err != nil {
		t.Fatalf("mask.Apply: %v", err)
	}
	if got, expect := schema.Tables[0].Columns[1].Default,
		pipeline.QuoteLiteral(want.Out.Text)+"::text"; got != expect {
		t.Fatalf("the rekeyed default is %q, want %q", got, expect)
	}
}
