// SPDX-License-Identifier: Apache-2.0

package core

import (
	"go/parser"
	"go/token"
	"path/filepath"
	"slices"
	"testing"

	"github.com/Liarea/lazyslice/internal/classify"
	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
	"github.com/Liarea/lazyslice/mask"
)

// T-0101, end to end over the two things that make it a defect rather than a
// tidiness complaint: the fingerprint has to move when the plan's pick moves,
// and §11.2's "classification changed" line has to print when it does.
//
// The shape is one column that becomes unique between two runs — somebody added
// a unique index to auth.users.confirmation_token, or raised --take past
// d_required. internal/classify decides the same category and writes the same
// default masker on both passes, so its own fingerprint is identical; it is
// internal/plan that escalates the second one to credential_unique and rewrites
// the decision, and internal/transform then masks with what the plan left.
// Recomputing after the plan is what makes the fingerprint describe that.
func TestAColumnBecomingUniqueChangesTheClassificationFingerprint(t *testing.T) {
	t.Parallel()

	before := classifyTokenColumn(t, false)
	after := classifyTokenColumn(t, true)

	// The defect, stated as an assertion so that a change which "fixes" it by
	// moving the pick back into classify still has to keep this true.
	if before.Fingerprint != after.Fingerprint {
		t.Fatalf("the classifier's own fingerprints differ (%q, %q); this test is about the case where "+
			"they do not, because the category and the default masker are the same on both passes",
			before.Fingerprint, after.Fingerprint)
	}

	col := ref.ColumnRef{
		Table:  ref.TableRef{Schema: "auth", Name: "users"},
		Column: "confirmation_token",
	}
	d := after.Decisions[col]
	if !d.Masked || d.Category != pipeline.CatCredential {
		t.Fatalf("confirmation_token is %+v; the fixture needs it masked as a credential", d)
	}
	if !d.UniqueIndex {
		t.Fatal("the second pass's column is not under a unique index, so nothing would escalate")
	}

	// What internal/plan/unique.go does, with the planned row count it is the
	// first stage to have. It is reproduced rather than driven because Plan
	// needs a reader and this claim needs none; internal/plan's own
	// TestUniqueCredentialColumnEscalates holds the pick itself.
	id, err := mask.Pick(mask.Category(d.Category), mask.Constraints{
		TypeTag: "text", Unique: true, Rows: 500,
	})
	if err != nil {
		t.Fatalf("the plan would have refused this column: %v", err)
	}
	if id == d.Masker {
		t.Fatalf("the plan picked %q, the masker classify already wrote; this test needs an escalation", id)
	}
	d.Masker = id
	after.Decisions[col] = d

	// core.refingerprint, over the decisions the plan left behind.
	r := &run{req: normalise(Request{}), sink: &collector{}, cls: after}
	if err := r.refingerprint(); err != nil {
		t.Fatalf("refingerprint: %v", err)
	}

	if after.Fingerprint == before.Fingerprint {
		t.Fatalf("the fingerprint is still %q after the plan escalated the masker to %q: "+
			"every value in the column will differ from the last run's and §11.2's "+
			"\"classification changed\" line would not print",
			after.Fingerprint, id)
	}
	if len(after.Fingerprint) != 16 {
		t.Errorf("fingerprint = %q, want 16 hex characters (ARCHITECTURE.md §5)", after.Fingerprint)
	}

	// And the warning §11.2 owes the operator, from the marker the previous run
	// left: the target holds values masked under `before` and this run is about
	// to write values masked under `after`.
	sink := &collector{}
	warned := &run{
		req:  normalise(Request{}),
		sink: sink,
		gate: pipeline.Eligibility{MarkerBound: true, PrevClassFP: before.Fingerprint},
		cls:  after,
	}
	warned.markerWarnings()
	if !sink.has(CodeClassChanged) {
		t.Errorf("%s was not emitted; a column that became unique between two runs changes every "+
			"masked value in it, and §11.2 says the operator is told before the target is truncated",
			CodeClassChanged)
	}

	// The same run against its own fingerprint says nothing, so the warning is
	// a comparison and not an unconditional line.
	quiet := &collector{}
	same := &run{
		req:  normalise(Request{}),
		sink: quiet,
		gate: pipeline.Eligibility{MarkerBound: true, PrevClassFP: after.Fingerprint},
		cls:  after,
	}
	same.markerWarnings()
	if quiet.has(CodeClassChanged) {
		t.Error("an unchanged classification printed \"classification changed\"")
	}
}

// A run whose plan escalated nothing keeps the fingerprint Classify computed,
// byte for byte. Without this the recompute would be a second, silently
// different definition of the same value, and every existing marker would read
// as changed on the next run.
func TestRefingerprintIsAnIdentityWhenThePlanChangedNothing(t *testing.T) {
	t.Parallel()

	cls := classifyTokenColumn(t, true)
	want := cls.Fingerprint

	r := &run{req: normalise(Request{}), sink: &collector{}, cls: cls}
	if err := r.refingerprint(); err != nil {
		t.Fatalf("refingerprint: %v", err)
	}
	if cls.Fingerprint != want {
		t.Errorf("fingerprint = %q, want the classifier's own %q", cls.Fingerprint, want)
	}

	// And a run that never classified is not a nil dereference: the two modes
	// that stop before classify (introspect, doctor) never reach the plan, but
	// the guard is what makes that a property of this function rather than of
	// where it is called from.
	empty := &run{req: normalise(Request{}), sink: &collector{}}
	if err := empty.refingerprint(); err != nil {
		t.Errorf("refingerprint with no classification: %v", err)
	}
}

// T-0101's defect was an *ordering* one, and the two tests above cannot see it.
// Both call r.refingerprint() directly, so deleting the call from execute, or
// moving it above planStage, leaves them green while restoring the bug exactly:
// the fingerprint would again be computed over the maskers classify wrote, and
// the plan's escalation would again change every masked value and no
// fingerprint.
//
// So the call site is pinned here, structurally, the way the review pin's own
// wiring is (TestTheReviewPinIsWiredIntoTheRunItGuards in reviewed_test.go,
// whose receiverCalls this reuses). It is structural rather than driven because
// reaching refingerprint through execute means discover, a snapshot, an
// introspect and a real Plan -- two Postgres servers -- and the claim is about
// two statements' order, not about what either of them computes. What they
// compute is asserted above and in internal/plan's own
// TestUniqueCredentialColumnEscalates; the integration coverage is
// gate_integration_test.go's.
func TestTheFingerprintIsRecomputedAfterThePlanAndBeforeItIsWritten(t *testing.T) {
	t.Parallel()

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filepath.Join(".", "run.go"), nil, 0)
	if err != nil {
		t.Fatalf("parse run.go: %v", err)
	}

	calls := receiverCalls(f, "execute")
	at := func(name string) int { return slices.Index(calls, name) }

	if at("refingerprint") < 0 {
		t.Fatalf("execute never calls refingerprint (order: %v). ARCHITECTURE.md §5 makes "+
			"Classification.Fingerprint a function of each column's masker, and internal/plan "+
			"rewrites Decision.Masker for a unique column, so without this call a column that "+
			"became unique between two runs changes every masked value in it and changes no "+
			"fingerprint -- and §11.2's \"classification changed\" line does not print (T-0101)", calls)
	}

	// The whole of T-0101: after the plan, because the plan is what moves the
	// masker.
	for _, want := range [][2]string{
		{"planStage", "refingerprint"},
		// ...and before either consumer of the value. emitPlanOnly writes the
		// yml a --plan produces and move writes the target, so a refingerprint
		// after either is a fingerprint that describes a different run.
		{"refingerprint", "emitPlanOnly"},
		{"refingerprint", "move"},
	} {
		before, after := at(want[0]), at(want[1])
		if before < 0 || after < 0 {
			t.Fatalf("execute does not call both %s and %s (order: %v); the pipeline this test "+
				"reads is not the one in run.go", want[0], want[1], calls)
		}
		if before > after {
			t.Errorf("execute calls %s before %s (order: %v). %s", want[1], want[0], calls,
				map[string]string{
					"planStage": "the fingerprint has to be computed over the decisions the " +
						"plan left behind, not the ones classify wrote: internal/plan/unique.go " +
						"escalates a unique column's masker and internal/transform masks with it",
					"refingerprint": "the recomputed fingerprint has to exist before anything " +
						"records it, or the yml and the marker row carry the pre-plan value",
				}[want[0]])
		}
	}
}

// classifyTokenColumn is one table with a token column, optionally under a
// single-column unique index, run through the real classifier. Nothing here is
// stubbed: the category, the default masker and Decision.UniqueIndex are all
// the rule pack's own answers, which is what makes the fingerprint comparison
// above a statement about the pipeline.
func classifyTokenColumn(t *testing.T, unique bool) *pipeline.Classification {
	t.Helper()
	tbl := ref.TableRef{Schema: "auth", Name: "users"}
	table := pipeline.Table{
		Ref: tbl,
		Columns: []pipeline.Column{
			{Name: "id", TypeName: "bigint", TypeOID: 20},
			{Name: "confirmation_token", TypeName: "text", Nullable: true},
		},
		PK: []string{"id"},
	}
	if unique {
		table.Indexes = []pipeline.Index{{
			Name:      "users_confirmation_token_key",
			Columns:   []string{"confirmation_token"},
			Unique:    true,
			Immediate: true,
			Def:       "CREATE UNIQUE INDEX users_confirmation_token_key ON auth.users USING btree (confirmation_token)",
		}}
	}
	schema := &pipeline.Schema{Tables: []pipeline.Table{table}}
	cls, err := classify.New().Classify(schema, schemaSampler{schema: schema}, nil)
	if err != nil {
		t.Fatalf("classify: %v", err)
	}
	return cls
}
