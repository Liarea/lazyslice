// SPDX-License-Identifier: Apache-2.0

package plan

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Liarea/lazyslice/internal/event"
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

// R2-07 (2026-09-15 round-2 red team, docs/reviews/2026-09-15-redteam/
// round2-still-leaking.json): a table CHECK carrying a person's name or a
// postal address, neither of which any of the five original strong
// validators reads, crossed into the target under exit 0. T-0189 widened
// strongValidators to run every category the row pipeline masks, and this is
// the regression fixture: a name in a CHECK on a masked column refuses at
// exit 13 (nothing rewrites a CHECK) and an address in one on an unmasked
// column refuses at exit 12 with the --unmask escape, exactly as an email
// already did before this task.
func TestRedTeamR207PersonNameAndAddressInCheckAreRefused(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name       string
		def        string
		maskColumn bool
		wantCode   event.Code
		wantExit   int
	}{
		{
			name:       "a person's full name in a CHECK on a masked column",
			def:        `CHECK (("owner" <> 'Grace Hopper'::text))`,
			maskColumn: true,
			wantCode:   CodeLiteralNotRewritable,
			wantExit:   exitSchema,
		},
		{
			name:     "a postal address in a CHECK on an unmasked column",
			def:      `CHECK (("owner" <> '42 Elm Street'::text))`,
			wantCode: CodeDDLLiteral,
			wantExit: exitPlan,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tbl, schema := literalTable()
			schema.Tables[0].Columns[1].Default = "" // the email column's own default is not this test's subject
			schema.Tables[0].Columns = append(schema.Tables[0].Columns,
				pipeline.Column{Name: "owner", TypeName: "text"})
			schema.Tables[0].Constraints = []pipeline.Constraint{
				{Name: "items_owner_check", Kind: 'c', Def: tc.def},
			}
			cls := masking(tbl, "email", pipeline.CatEmail, mask.MaskerEmail)
			if tc.maskColumn {
				ownerCol := ref.ColumnRef{Table: tbl, Column: "owner"}
				cls.Decisions[ownerCol] = pipeline.Decision{
					Col: ownerCol, Category: pipeline.CatPersonName, Masker: mask.MaskerPersonName, Masked: true,
				}
			}

			err := planLiterals(t, schema, cls, literalKey(0x44))
			var refusal *Refusal
			if !errors.As(err, &refusal) {
				t.Fatalf("Plan returned %v, want a *plan.Refusal: R2-07 crossed under exit 0", err)
			}
			if refusal.Code != tc.wantCode || refusal.Exit != tc.wantExit {
				t.Fatalf("Plan refused with %s exit %d, want %s exit %d",
					refusal.Code, refusal.Exit, tc.wantCode, tc.wantExit)
			}
			if refusal.Column != "items_owner_check" {
				t.Fatalf("the refusal names %q, want the constraint it is about", refusal.Column)
			}
			for _, secret := range []string{"Grace Hopper", "42 Elm Street"} {
				if strings.Contains(refusal.Message, secret) || strings.Contains(refusal.Args["reason"], secret) {
					t.Fatalf("the refusal quotes the literal: %q / %q (THREAT_MODEL.md T4)",
						refusal.Message, refusal.Args["reason"])
				}
			}
		})
	}
}

// R2-08 (the same round): internal/plan read no pg_index at all (tracker
// T-0163), so a partial index's WHERE predicate carrying an address was seen
// only by internal/verify's post-load catalog pass — exit 9 with the target
// already dropped and loaded, rather than exit 12 or 13 before anything was
// touched. tableDDLLiterals now walks t.Indexes through the same
// never-rewritten rule a CHECK gets.
func TestRedTeamR208PartialIndexPredicateIsReadAtPlan(t *testing.T) {
	t.Parallel()
	tbl, schema := literalTable()
	schema.Tables[0].Columns[1].Default = ""
	schema.Tables[0].Columns = append(schema.Tables[0].Columns,
		pipeline.Column{Name: "owner", TypeName: "text"})
	schema.Tables[0].Indexes = []pipeline.Index{{
		Name:    "items_vip_idx",
		Columns: []string{"id"},
		Partial: true,
		Def:     `CREATE INDEX items_vip_idx ON public.items USING btree (id) WHERE ("owner" = '42 Elm Street'::text)`,
	}}
	cls := masking(tbl, "email", pipeline.CatEmail, mask.MaskerEmail)
	ownerCol := ref.ColumnRef{Table: tbl, Column: "owner"}
	cls.Decisions[ownerCol] = pipeline.Decision{
		Col: ownerCol, Category: pipeline.CatPersonName, Masker: mask.MaskerPersonName, Masked: true,
	}

	err := planLiterals(t, schema, cls, literalKey(0x44))
	var refusal *Refusal
	if !errors.As(err, &refusal) {
		t.Fatalf("Plan returned %v, want a *plan.Refusal: an index predicate reached the target unseen (T-0163)", err)
	}
	if refusal.Code != CodeLiteralNotRewritable || refusal.Exit != exitSchema {
		t.Fatalf("Plan refused with %s exit %d, want %s exit %d",
			refusal.Code, refusal.Exit, CodeLiteralNotRewritable, exitSchema)
	}
	if refusal.Column != "items_vip_idx" {
		t.Fatalf("the refusal names %q, want the index it is about", refusal.Column)
	}
	if strings.Contains(refusal.Message, "42 Elm Street") || strings.Contains(refusal.Args["reason"], "42 Elm Street") {
		t.Fatalf("the refusal quotes the literal: %q / %q (THREAT_MODEL.md T4)",
			refusal.Message, refusal.Args["reason"])
	}
}

// R2-10 (the same round): a value on the right-hand side of a pattern
// operator was exempt from detection entirely, not only from rewriting, so
// CHECK (email !~ '^ceo@bigcorp\.example$') crossed under exit 0 while the
// semantically identical CHECK (email <> 'ceo@bigcorp.example') refused. This
// is the red team's own reduction: an anchored, dot-escaped regex carrying an
// exact address must refuse exactly as the equality form does.
func TestRedTeamR210PatternOperandCarryingAValueIsStillDetected(t *testing.T) {
	t.Parallel()
	tbl, schema := literalTable()
	schema.Tables[0].Columns[1].Default = ""
	schema.Tables[0].Constraints = []pipeline.Constraint{
		{Name: "items_email_regex_check", Kind: 'c',
			Def: `CHECK (("email" !~ '^ceo@bigcorp\.example$'::text))`},
	}
	cls := masking(tbl, "email", pipeline.CatEmail, mask.MaskerEmail)

	err := planLiterals(t, schema, cls, literalKey(0x44))
	var refusal *Refusal
	if !errors.As(err, &refusal) {
		t.Fatalf("Plan returned %v, want a *plan.Refusal: the pattern exempted the address from detection (R2-10)", err)
	}
	if refusal.Code != CodeLiteralNotRewritable || refusal.Exit != exitSchema {
		t.Fatalf("Plan refused with %s exit %d, want %s exit %d",
			refusal.Code, refusal.Exit, CodeLiteralNotRewritable, exitSchema)
	}
	if refusal.Column != "items_email_regex_check" {
		t.Fatalf("the refusal names %q, want the constraint it is about", refusal.Column)
	}
	if strings.Contains(refusal.Message, "ceo@bigcorp.example") || strings.Contains(refusal.Args["reason"], "ceo@bigcorp.example") {
		t.Fatalf("the refusal quotes the literal: %q / %q (THREAT_MODEL.md T4)",
			refusal.Message, refusal.Args["reason"])
	}
	// The literal is still marked a pattern operand -- Pattern and Rewritable
	// are orthogonal, and this form happens to be the plain '...' quoting, so
	// Rewritable is true on it -- but a CHECK has no rewrite arm at all
	// regardless of that flag: fixedExpression only ever detects and refuses.
	// The guard that actually matters is StripPatternMeta's own, held by
	// TestPatternMetaStripping below.
	lits := pipeline.Literals(`CHECK (("email" !~ '^ceo@bigcorp\.example$'::text))`)
	if len(lits) != 1 || !lits[0].Pattern {
		t.Fatalf("the literal is not marked a pattern operand: %+v", lits)
	}
}

// StripPatternMeta itself: the reduction R2-10's fix depends on. An escaped
// metacharacter is unescaped to the literal it stands for rather than
// discarded twice over, and an unescaped one is discarded outright -- which is
// the whole difference between a regex that IS a value and one that is only a
// shape.
func TestPatternMetaStripping(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		in, want string
	}{
		{`%@%.%`, `@`},
		{`^ceo@bigcorp\.example$`, `ceo@bigcorp.example`},
		// A removed _ leaves a separator instead of gluing the tokens on
		// either side of it (round-5 red team,
		// docs/reviews/2026-09-15-redteam/round5-still-leaking.json): the
		// LIKE spelling of 'HIV_POSITIVE' must not reduce to one
		// vocabulary-defeating word.
		{`%foo_bar%`, `foo bar`},
		{`%HIV_POSITIVE%`, `HIV POSITIVE`},
	} {
		if got := pipeline.StripPatternMeta(tc.in); got != tc.want {
			t.Errorf("StripPatternMeta(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// StripPatternMetaGlued: the other reduction (T-0254 review, high finding 1).
// It closes the gap `_` sat in instead of leaving a space, which is what lets
// a validator read the value that spacing would have cut in two -- an email
// local part chief among them, since `_` is not a metacharacter at all for
// the tilde operators.
func TestPatternMetaGluedStripping(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		in, want string
	}{
		{`%@%.%`, `@`},
		{`^ceo@bigcorp\.example$`, `ceo@bigcorp.example`},
		{`%foo_bar%`, `foobar`},
		// The gap is closed, not the address kept letter for letter: an
		// underscore is still LIKE's own metacharacter and is dropped like
		// any other, so this reduction is not the literal address -- it is
		// johndoe@bigcorp.example, still an email, still enough to trigger a
		// strongHit and refuse the run over the real one.
		{`^john_doe@bigcorp\.example$`, `johndoe@bigcorp.example`},
	} {
		if got := pipeline.StripPatternMetaGlued(tc.in); got != tc.want {
			t.Errorf("StripPatternMetaGlued(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// TestT0254HighFinding1UnderscoredEmailInARegexOperandIsStillDetected is the
// reviewer's own reduction for the T-0254 review's high finding 1:
// CHECK ("email" !~ '^john_doe@bigcorp\.example$') on a masked email column
// used to cross at exit 0, because StripPatternMeta's spacing of `_` (added
// for the special-category vocabulary's sake) cut the local part in two
// before ValidEmail ever saw it, where `_` is not syntax for the tilde
// operators at all and the address is exactly what the literal carries.
// strongHit now also tries StripPatternMetaGlued's reduction, which keeps the
// address intact.
func TestT0254HighFinding1UnderscoredEmailInARegexOperandIsStillDetected(t *testing.T) {
	t.Parallel()
	tbl, schema := literalTable()
	schema.Tables[0].Columns[1].Default = ""
	schema.Tables[0].Constraints = []pipeline.Constraint{
		{Name: "items_email_regex_check", Kind: 'c',
			Def: `CHECK (("email" !~ '^john_doe@bigcorp\.example$'::text))`},
	}
	cls := masking(tbl, "email", pipeline.CatEmail, mask.MaskerEmail)

	err := planLiterals(t, schema, cls, literalKey(0x44))
	var refusal *Refusal
	if !errors.As(err, &refusal) {
		t.Fatalf("Plan returned %v, want a *plan.Refusal: the underscored address crossed unexamined", err)
	}
	if refusal.Code != CodeLiteralNotRewritable || refusal.Exit != exitSchema {
		t.Fatalf("Plan refused with %s exit %d, want %s exit %d",
			refusal.Code, refusal.Exit, CodeLiteralNotRewritable, exitSchema)
	}
	if refusal.Column != "items_email_regex_check" {
		t.Fatalf("the refusal names %q, want the constraint it is about", refusal.Column)
	}
}

// R2-10 on the DEFAULT path (the T-0189 fix round's finding 1): a pattern
// operand inside a *rewritable, masked column's* DEFAULT was exempt from
// detection as well as from rewriting. columnDefault's RewriteLiterals
// callback declines a Pattern literal outright -- correctly, rewriting one
// changes what the database accepts -- but RewriteLiterals leaves a declined
// literal's text exactly as it stood and still reports ok == true, and the
// success path used to write that text back and return without ever running
// strongHit over what it had declined. internal/verify then exempts the
// column's DEFAULT outright because DefaultOriginal is set (rewroteDefault),
// so nothing looked at it a second time either: exit 0 with the address in
// the target's pg_attrdef.
func TestRedTeamR210FixRoundPatternOperandInARewritableDefaultIsStillDetected(t *testing.T) {
	t.Parallel()
	tbl, schema := literalTable()
	schema.Tables[0].Columns[1].Default = `(("email" !~ '^ceo@bigcorp\.example$'::text))`
	cls := masking(tbl, "email", pipeline.CatEmail, mask.MaskerEmail)

	err := planLiterals(t, schema, cls, literalKey(0x44))
	var refusal *Refusal
	if !errors.As(err, &refusal) {
		t.Fatalf("Plan returned %v, want a *plan.Refusal: the pattern operand in a rewritable DEFAULT "+
			"crossed unexamined (R2-10 fix round)", err)
	}
	if refusal.Code != CodeLiteralNotRewritable || refusal.Exit != exitSchema {
		t.Fatalf("Plan refused with %s exit %d, want %s exit %d",
			refusal.Code, refusal.Exit, CodeLiteralNotRewritable, exitSchema)
	}
	if refusal.Column != "email" {
		t.Fatalf("the refusal names %q, want the column it is about", refusal.Column)
	}
	if strings.Contains(refusal.Message, "ceo@bigcorp.example") || strings.Contains(refusal.Args["reason"], "ceo@bigcorp.example") {
		t.Fatalf("the refusal quotes the literal: %q / %q (THREAT_MODEL.md T4)",
			refusal.Message, refusal.Args["reason"])
	}
	if got := schema.Tables[0].Columns[1].Default; got != `(("email" !~ '^ceo@bigcorp\.example$'::text))` {
		t.Fatalf("the default was rewritten and written back ahead of the refusal: %q", got)
	}
}

// R2-11 (the T-0189 fix round's finding 3): namedColumns was fed
// pg_get_indexdef's whole text, which -- unlike pg_get_constraintdef --
// always opens with `CREATE INDEX name ON schema.table`, so a column sharing
// the table's own name read as though the predicate named it. Measured: table
// public.items with columns {id, items, email}, index `CREATE INDEX
// items_vip_idx ON public.items USING btree (email) WHERE (email =
// '...')` returned named == [items email].
func TestNamedColumnsDoesNotMatchTheTableNameInAnIndexDef(t *testing.T) {
	t.Parallel()
	table := &pipeline.Table{
		Ref: ref.TableRef{Schema: "public", Name: "items"},
		Columns: []pipeline.Column{
			{Name: "id", TypeName: "bigint"},
			{Name: "items", TypeName: "text"},
			{Name: "email", TypeName: "text"},
		},
	}
	def := `CREATE INDEX items_vip_idx ON public.items USING btree (email) ` +
		`WHERE (email = 'ceo@bigcorp.example'::text)`
	got := namedColumns(def, table)
	if len(got) != 1 || got[0] != "email" {
		t.Fatalf("namedColumns(%q) = %v, want [email]: the table's own name was read as a column (R2-11)", def, got)
	}
}

// The end-to-end half of the test above: before the fix, the phantom "items"
// column's own --unmask opt-out silently cleared a genuine hit in a
// *different* column's index predicate -- refuseUnmaskedLiteral returns nil
// on the first named column carrying an opt-out, so a false match fails open
// as well as naming the wrong object.
func TestRedTeamR211FixRoundAnOptOutOnATableNamedColumnDoesNotClearAGenuineHit(t *testing.T) {
	t.Parallel()
	tbl, schema := literalTable()
	schema.Tables[0].Columns[1].Default = ""
	schema.Tables[0].Columns = append(schema.Tables[0].Columns,
		pipeline.Column{Name: "items", TypeName: "text"})
	schema.Tables[0].Indexes = []pipeline.Index{{
		Name:    "items_vip_idx",
		Columns: []string{"email"},
		Partial: true,
		Def: `CREATE INDEX items_vip_idx ON public.items USING btree (email) ` +
			`WHERE ("email" = 'ceo@bigcorp.example'::text)`,
	}}
	itemsCol := ref.ColumnRef{Table: tbl, Column: "items"}
	cls := &pipeline.Classification{Decisions: map[ref.ColumnRef]pipeline.Decision{
		itemsCol: {Col: itemsCol, Source: pipeline.ByFlagUnmask},
	}}

	err := planLiterals(t, schema, cls, literalKey(0x44))
	var refusal *Refusal
	if !errors.As(err, &refusal) {
		t.Fatalf("Plan returned %v, want a *plan.Refusal: an opt-out on the table-named column "+
			"cleared a genuine hit on a different column (R2-11)", err)
	}
	if refusal.Code != CodeDDLLiteral || refusal.Exit != exitPlan {
		t.Fatalf("Plan refused with %s exit %d, want %s exit %d",
			refusal.Code, refusal.Exit, CodeDDLLiteral, exitPlan)
	}
	if !strings.Contains(refusal.Args["reason"], "--unmask public.items.email=REASON") {
		t.Fatalf("the refusal's reason is %q, want the escape naming the email column", refusal.Args["reason"])
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

// TestNationalIDStrongHitIsStructuredOnly is tracker T-0194 (the T-0187
// review round's finding 2). strongValidators' national_id entry used to call
// textsig.ValidNationalID, the twelve-format union: six of those formats are a
// mod-N sum over an otherwise unconstrained digit run, and a mod-N sum clears
// a meaningful fraction of a random string of the right length regardless of
// what it means (measured: 25.7% of random 9-digit strings, 11.0% of
// 11-digit) -- so an ordinary CHECK or DEFAULT literal that happened to be a
// nine- or eleven-digit reference number had a real chance of refusing an
// otherwise clean plan at exit 13 with no ratio to weigh it against. The
// entry now calls textsig.ValidNationalIDStructured, the six formats that also
// constrain the value's *shape* (a dash, a letter, or a fixed length under its
// own mod-97 check), so neither a PESEL-valid eleven-digit literal nor a bare
// nine-digit literal that merely clears a checksum is a hit -- and a dashed US
// SSN, one of the structured six, still is.
func TestNationalIDStrongHitIsStructuredOnly(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name       string
		literal    string
		refused    bool
		wantReason string
	}{
		{
			// nationalid_test.go's own Polish PESEL vector: a real weighted
			// mod-10 checksum with no shape constraint at all. It is not a
			// national_id hit -- textsig.ValidNationalIDStructured must not
			// answer for a checksum-only literal (T-0194) -- and, since the
			// T-0198 fix round's own re-measurement of finding 1, a CHECK no
			// longer carries the broadened "whatever it parses as" net at
			// all (fixedExpression's own comment: that net's only escape can
			// require unmasking a column the flagged literal was never
			// about, which two real schemas hit for real). So this passes.
			name:    "a PESEL-valid eleven-digit literal in a CHECK is not a national_id hit",
			literal: "44050612341",
		},
		{
			// nationalid_test.go's own Canadian SIN vector: nine digits under
			// the same Luhn check ValidLuhn uses, applied directly, again with
			// no shape constraint. Same reasoning as above.
			name:    "a checksum-clearing nine-digit literal in a CHECK is not a national_id hit",
			literal: "123456782",
		},
		{
			name:       "a dashed US SSN literal in a CHECK is still a national_id hit",
			literal:    "078-05-1001",
			refused:    true,
			wantReason: "parses as national_id",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tbl, schema := literalTable()
			schema.Tables[0].Columns[1].Default = "" // the email column's own default is not this test's subject
			schema.Tables[0].Columns = append(schema.Tables[0].Columns,
				pipeline.Column{Name: "ref", TypeName: "text"})
			schema.Tables[0].Constraints = []pipeline.Constraint{
				{Name: "items_ref_check", Kind: 'c', Def: `CHECK (("ref" = '` + tc.literal + `'::text))`},
			}
			cls := masking(tbl, "email", pipeline.CatEmail, mask.MaskerEmail)
			refCol := ref.ColumnRef{Table: tbl, Column: "ref"}
			cls.Decisions[refCol] = pipeline.Decision{
				Col: refCol, Category: pipeline.CatFreeText, Masker: mask.MaskerFreeText, Masked: true,
			}

			err := planLiterals(t, schema, cls, literalKey(0x44))
			if !tc.refused {
				if err != nil {
					t.Fatalf("Plan: %v, want no refusal: a CHECK no longer carries the broadened net "+
						"(T-0198 fix round)", err)
				}
				return
			}
			var refusal *Refusal
			if !errors.As(err, &refusal) {
				t.Fatalf("Plan returned %v, want a *plan.Refusal", err)
			}
			if refusal.Code != CodeLiteralNotRewritable {
				t.Fatalf("Plan refused with %s, want %s", refusal.Code, CodeLiteralNotRewritable)
			}
			if refusal.Column != "items_ref_check" {
				t.Fatalf("the refusal names %q, want the constraint it is about", refusal.Column)
			}
			if !strings.Contains(refusal.Message, tc.wantReason) {
				t.Fatalf("Plan refused with %q, want it to carry %q", refusal.Message, tc.wantReason)
			}
		})
	}
}

// TestAddressStrongHitNeedsAStreetSuffixWord is the T-0189 fix round's finding
// 2. textsig.AddressShape -- "a digit somewhere, and at least two words that
// carry a letter" -- is calibrated for internal/verify's second net, which
// asks it of a whole column and fails only once many rows agree
// (validators.go marks it explicitly not strong for that reason); this
// scanner gets one look at one literal, and had been treating the same loose
// shape as a one-hit refusal since T-0189 first widened strongValidators to
// close R2-07. Measured against ordinary CHECK value-list and enum-label
// text, the bare shape also hit plan pricing tiers and priority labels that
// are nobody's address, refusing a masked column at exit 13 with no escape at
// all and an unmasked one at exit 12 whose only escape says the *column*
// holds nothing personal, not that this one literal does not.
// addressLiteralShape adds the corroboration ValidNationalIDStructured
// already models for national_id: a feature that is actually diagnostic
// (here, a street-type suffix word) rather than merely necessary.
func TestAddressStrongHitNeedsAStreetSuffixWord(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name       string
		literal    string
		refused    bool
		wantReason string
	}{
		// None of these four is an address hit -- addressLiteralShape still
		// needs a street-type suffix word (T-0189 fix round finding 2) -- and,
		// since the T-0198 fix round's own re-measurement of finding 1, a
		// CHECK no longer carries the broadened "whatever it parses as" net
		// either (fixedExpression's own comment), so none of these refuses.
		{name: "a pricing tier label is not an address hit", literal: "Basic 1 user"},
		{name: "a priority label is not an address hit", literal: "P1 High Priority"},
		{name: "a ranking label is not an address hit", literal: "Top 10 sellers"},
		{name: "a room label is not an address hit", literal: "Building 4 Lobby"},
		{
			name:       "a real street address is still an address hit",
			literal:    "42 Elm Street",
			refused:    true,
			wantReason: "parses as address",
		},
		{
			// R2-07's own canary must keep refusing through this change.
			name:       "R2-07's canary address is still an address hit",
			literal:    "1742 Kestrel Hollow Lane, Ashford VT 05024",
			refused:    true,
			wantReason: "parses as address",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tbl, schema := literalTable()
			schema.Tables[0].Columns[1].Default = "" // the email column's own default is not this test's subject
			schema.Tables[0].Columns = append(schema.Tables[0].Columns,
				pipeline.Column{Name: "label", TypeName: "text"})
			schema.Tables[0].Constraints = []pipeline.Constraint{
				{Name: "items_label_check", Kind: 'c', Def: `CHECK (("label" = '` + tc.literal + `'::text))`},
			}
			cls := masking(tbl, "email", pipeline.CatEmail, mask.MaskerEmail)
			labelCol := ref.ColumnRef{Table: tbl, Column: "label"}
			cls.Decisions[labelCol] = pipeline.Decision{
				Col: labelCol, Category: pipeline.CatAddress, Masker: mask.MaskerFreeText, Masked: true,
			}

			err := planLiterals(t, schema, cls, literalKey(0x44))
			if !tc.refused {
				if err != nil {
					t.Fatalf("Plan: %v, want no refusal: a CHECK no longer carries the broadened net "+
						"(T-0198 fix round)", err)
				}
				return
			}
			var refusal *Refusal
			if !errors.As(err, &refusal) {
				t.Fatalf("Plan returned %v, want a *plan.Refusal", err)
			}
			if refusal.Code != CodeLiteralNotRewritable {
				t.Fatalf("Plan refused with %s, want %s", refusal.Code, CodeLiteralNotRewritable)
			}
			if refusal.Column != "items_label_check" {
				t.Fatalf("the refusal names %q, want the constraint it is about", refusal.Column)
			}
			if !strings.Contains(refusal.Message, tc.wantReason) {
				t.Fatalf("Plan refused with %q, want it to carry %q", refusal.Message, tc.wantReason)
			}
		})
	}
}

// TestRedTeamR5SpecialCategoryUnderscoreGluingIsRefused is the round-5 red
// team's still-leaking special-category entry
// (docs/reviews/2026-09-15-redteam/round5-still-leaking.json, T-0254):
// CHECK (note <> 'HIV_POSITIVE') on a masked special-category column used to
// cross at exit 0, because reSpecialCategoryTerm's \b treats `_` as a word
// character and never split "HIV" from "_POSITIVE". Both fixes have to hold
// together for this literal specifically: textsig.SpecialCategoryVocabulary's
// own normalisation reduces the equality operand directly, and this is the
// end-to-end guard that the plan's strongHit path actually reaches it.
func TestRedTeamR5SpecialCategoryUnderscoreGluingIsRefused(t *testing.T) {
	t.Parallel()
	tbl, schema := literalTable()
	schema.Tables[0].Columns[1].Default = "" // the email column's own default is not this test's subject
	schema.Tables[0].Constraints = []pipeline.Constraint{
		{Name: "ck_note", Kind: 'c', Def: `CHECK (("note" <> 'HIV_POSITIVE'::text))`},
	}
	cls := masking(tbl, "email", pipeline.CatEmail, mask.MaskerEmail)
	noteCol := ref.ColumnRef{Table: tbl, Column: "note"}
	cls.Decisions[noteCol] = pipeline.Decision{
		Col: noteCol, Category: pipeline.CatSpecial, Masker: mask.MaskerSpecial, Masked: true,
	}

	err := planLiterals(t, schema, cls, literalKey(0x44))
	var refusal *Refusal
	if !errors.As(err, &refusal) {
		t.Fatalf("Plan returned %v, want a *plan.Refusal", err)
	}
	if refusal.Code != CodeLiteralNotRewritable || refusal.Exit != exitSchema {
		t.Fatalf("Plan refused with %s exit %d, want %s exit %d",
			refusal.Code, refusal.Exit, CodeLiteralNotRewritable, exitSchema)
	}
	if refusal.Column != "ck_note" {
		t.Fatalf("the refusal names %q, want the constraint it is about", refusal.Column)
	}
}

// TestAMaskedNameDefaultIsMaskedUnderTheColumnsRole is T-0294: a first_name
// column's rows are masked to one given name (T-0287), so its default must be
// too, not a full "Given Family" name.
func TestAMaskedNameDefaultIsMaskedUnderTheColumnsRole(t *testing.T) {
	t.Parallel()
	tbl := ref.TableRef{Schema: "public", Name: "people"}
	schema := &pipeline.Schema{Tables: []pipeline.Table{{
		Ref: tbl,
		Columns: []pipeline.Column{
			{Name: "id", TypeName: "bigint", TypeOID: 20},
			{Name: "first_name", TypeName: "text", Default: "'Margaret'::text"},
		},
		PK: []string{"id"},
	}}}
	cls := masking(tbl, "first_name", pipeline.CatPersonName, mask.MaskerPersonName)
	col := ref.ColumnRef{Table: tbl, Column: "first_name"}
	d := cls.Decisions[col]
	d.Role = mask.RoleGiven
	cls.Decisions[col] = d
	key := literalKey(0x22)

	if err := planLiterals(t, schema, cls, key); err != nil {
		t.Fatalf("Plan: %v", err)
	}
	want, err := mask.Apply(*key, mask.CatPersonName, mask.MaskerPersonName,
		mask.Value{Text: "Margaret"}, mask.Constraints{TypeTag: "text", Role: mask.RoleGiven})
	if err != nil {
		t.Fatalf("mask.Apply: %v", err)
	}
	got := schema.Tables[0].Columns[1].Default
	if expect := pipeline.QuoteLiteral(want.Out.Text) + "::text"; got != expect {
		t.Fatalf("the default is %q, want %q: a first_name default must mask as one given name, "+
			"the way the column's own rows do", got, expect)
	}
	if strings.Contains(want.Out.Text, " ") {
		t.Fatalf("mask.Apply under RoleGiven gave %q, two words", want.Out.Text)
	}
}
