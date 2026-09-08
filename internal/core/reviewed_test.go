// SPDX-License-Identifier: Apache-2.0

package core

import (
	"bytes"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Liarea/lazyslice/internal/dsn"
	"github.com/Liarea/lazyslice/internal/event"
	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/render"
)

// pass is one pass of the pipeline as the review pin sees it: the schema
// fingerprint introspect computed, the two endpoints discover resolved and (set
// by the caller that needs it) the fingerprint classify decided.
//
// A run is built by hand rather than driven, because the two comparisons read
// exactly these fields and reaching them through Run needs two Postgres servers
// and a schema that changes between them. That the comparisons are *called*, in
// the right place, is asserted separately and structurally
// (TestTheReviewPinIsWiredIntoTheRunItGuards).
func pass(fingerprint string, source, target dsn.Ref) *run {
	return &run{
		req:        normalise(Request{Mode: ModeRun}),
		sink:       event.Discard,
		schema:     &pipeline.Schema{Fingerprint: fingerprint},
		sourceCand: pipeline.Candidate{Ref: source},
		targetCand: pipeline.Candidate{Ref: target},
	}
}

var (
	prod  = dsn.Ref{User: "app", Host: "db.internal", Port: 5432, Database: "app"}
	local = dsn.Ref{User: "app", Host: "127.0.0.1", Port: 5433, Database: "app_dev"}
	other = dsn.Ref{User: "app", Host: "127.0.0.1", Port: 5434, Database: "app_scratch"}
)

// The pin passes the run it was taken from.
//
// --tui shows the operator a classification and a plan built by one pass and
// writes the target with a second, and the second must not be refused for
// merely being a second: a source nobody migrated, resolved to the same two
// databases, is the run that was reviewed.
func TestTheReviewPinPassesAnIdenticalSnapshot(t *testing.T) {
	preview := pass("ddl-fingerprint-1", prod, local)
	reviewed := preview.reviewed()

	if reviewed.SchemaFingerprint != "ddl-fingerprint-1" {
		t.Errorf("reviewed fingerprint = %q, want the schema's own (ADR-009)", reviewed.SchemaFingerprint)
	}
	if reviewed.Source != prod.String() || reviewed.Target != local.String() {
		t.Errorf("reviewed endpoints = %q and %q, want %q and %q",
			reviewed.Source, reviewed.Target, prod, local)
	}
	if strings.Contains(reviewed.Source+reviewed.Target, "password") {
		t.Errorf("an endpoint carries a credential: %q, %q", reviewed.Source, reviewed.Target)
	}

	second := pass("ddl-fingerprint-1", prod, local)
	second.req.Reviewed = reviewed
	if err := second.checkReviewed(); err != nil {
		t.Fatalf("checkReviewed = %v, want the run that was reviewed to proceed", err)
	}

	// No pin at all is the whole of the CLI's behaviour without --tui, and it
	// must stay a run and not a refusal.
	unpinned := pass("ddl-fingerprint-2", prod, local)
	if err := unpinned.checkReviewed(); err != nil {
		t.Errorf("checkReviewed with no pin = %v, want nil", err)
	}
}

// A source that changed between the two passes is refused before the target is
// written.
//
// The second pass takes its own snapshot, so a migration between the two gives
// the operator a plan and a set of masking decisions that were made over a
// schema this run is not reading. ADR-009's fingerprint is what makes the two
// comparable.
func TestTheReviewPinRefusesASchemaThatChangedBetweenThePasses(t *testing.T) {
	second := pass("ddl-fingerprint-2", prod, local)
	second.req.Reviewed = &Reviewed{
		SchemaFingerprint: "ddl-fingerprint-1",
		Source:            prod.String(),
		Target:            local.String(),
	}

	err := second.checkReviewed()
	stop := wantReviewedStop(t, err)
	if !strings.Contains(stop.Args[event.ArgReason], "schema") {
		t.Errorf("reason = %q, want it to name the schema as the thing that changed",
			stop.Args[event.ArgReason])
	}
	if strings.Contains(stop.Args[event.ArgReason], "target is") {
		t.Errorf("reason = %q, want only the schema named: the endpoints did not change",
			stop.Args[event.ArgReason])
	}
}

// A target the ladder resolved differently the second time is refused.
//
// The two passes walk the discovery ladder independently, so a container that
// stopped, started or was outranked between them gives a second pass that
// writes a database the operator never approved. This is the case the pin
// exists for: everything else about the run can be identical.
func TestTheReviewPinRefusesADifferentResolvedTarget(t *testing.T) {
	second := pass("ddl-fingerprint-1", prod, other)
	second.req.Reviewed = &Reviewed{
		SchemaFingerprint: "ddl-fingerprint-1",
		Source:            prod.String(),
		Target:            local.String(),
	}

	err := second.checkReviewed()
	stop := wantReviewedStop(t, err)
	reason := stop.Args[event.ArgReason]
	if !strings.Contains(reason, other.String()) || !strings.Contains(reason, local.String()) {
		t.Errorf("reason = %q, want both the target this run resolved (%s) and the reviewed one (%s)",
			reason, other, local)
	}

	// A mode that opens no target compares none: an empty reviewed target is a
	// fact about `lazyslice classify`, not a review that never happened.
	readOnly := pass("ddl-fingerprint-1", prod, dsn.Ref{})
	readOnly.req.Mode = ModeClassify
	readOnly.req.Reviewed = &Reviewed{SchemaFingerprint: "ddl-fingerprint-1", Source: prod.String()}
	if err := readOnly.checkReviewed(); err != nil {
		t.Errorf("checkReviewed for %v = %v, want nil: that mode opens no target", ModeClassify, err)
	}
}

// wantReviewedStop asserts the refusal's code, its exit and that its catalogue
// row renders with every placeholder filled.
func wantReviewedStop(t *testing.T, err error) *Stop {
	t.Helper()
	var stop *Stop
	if !errors.As(err, &stop) {
		t.Fatalf("checkReviewed = %v, want a Stop", err)
	}
	if stop.Code != CodeReviewedChanged || stop.Exit != exitReviewed {
		t.Fatalf("stop = %s/exit %d, want %s/exit %d",
			stop.Code, stop.Exit, CodeReviewedChanged, exitReviewed)
	}

	var buf bytes.Buffer
	render.NewLines(&buf).Send(event.Event{Kind: event.Error, Code: stop.Code, Args: stop.Args})
	if line := buf.String(); strings.ContainsAny(line, "{}") {
		t.Errorf("rendered %q, want no unfilled placeholder", line)
	}
	return stop
}

// A classification the operator did not see is refused before the plan.
//
// The reasons screen is the masking review and the classifier reads samples,
// not DDL: a second snapshot of a live source can move a column that reached
// the mask threshold on a value signal alone below it, and the schema
// fingerprint above is identical while it happens (THREAT_MODEL.md T1).
func TestTheReviewPinRefusesAClassificationThatChangedBetweenThePasses(t *testing.T) {
	second := pass("ddl-fingerprint-1", prod, local)
	second.classFP = "class-fingerprint-2"
	second.req.Reviewed = &Reviewed{
		SchemaFingerprint: "ddl-fingerprint-1",
		ClassFingerprint:  "class-fingerprint-1",
		Source:            prod.String(),
		Target:            local.String(),
	}

	// The first half of the pin passes: nothing about the schema or the
	// endpoints moved, which is the whole point of the second half.
	if err := second.checkReviewed(); err != nil {
		t.Fatalf("checkReviewed = %v, want nil: only the classification changed", err)
	}

	stop := wantReviewedStop(t, second.checkReviewedClassification())
	if !strings.Contains(stop.Args[event.ArgReason], "classification") {
		t.Errorf("reason = %q, want it to name the classification as the thing that changed",
			stop.Args[event.ArgReason])
	}

	second.classFP = "class-fingerprint-1"
	if err := second.checkReviewedClassification(); err != nil {
		t.Fatalf("checkReviewedClassification = %v, want the reviewed classification to proceed", err)
	}

	// No pin at all is every run without --tui, and it must stay a run.
	unpinned := pass("ddl-fingerprint-1", prod, local)
	unpinned.classFP = "class-fingerprint-9"
	if err := unpinned.checkReviewedClassification(); err != nil {
		t.Errorf("checkReviewedClassification with no pin = %v, want nil", err)
	}
}

// The pin is wired into the pipeline, and not merely written.
//
// The three tests above build a run by hand and call the comparison directly,
// because reaching it through Run needs two Postgres servers and a schema that
// changes between them. That leaves the wiring itself unpinned: deleting the
// `if err := r.checkReviewed()` block from execute, or the line in Preview that
// forces PlanOnly, left the whole suite green — a safety rail nothing noticed
// the absence of, which is what root CLAUDE.md says a rail must never be.
//
// So the wiring is asserted structurally, the way TestEveryLadderOptionTheRequestCarriesIsCopied
// asserts the ladder's own. Position matters as much as presence: a pin checked
// after the plan, or after the loader's first write, is a refusal that arrives
// too late to be one.
func TestTheReviewPinIsWiredIntoTheRunItGuards(t *testing.T) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filepath.Join(".", "run.go"), nil, 0)
	if err != nil {
		t.Fatalf("parse run.go: %v", err)
	}

	calls := receiverCalls(f, "execute")
	for _, want := range [][2]string{
		{"introspectStage", "checkReviewed"},
		{"checkReviewed", "classifyStage"},
		{"classifyStage", "checkReviewedClassification"},
		{"checkReviewedClassification", "planStage"},
	} {
		before, after := slices.Index(calls, want[0]), slices.Index(calls, want[1])
		if before < 0 {
			t.Fatalf("execute does not call %s; the pipeline in %v is not the one this test reads", want[0], calls)
		}
		if after < 0 {
			t.Fatalf("execute does not call %s: the review pin is not wired into the run, "+
				"so a run is never compared against what the operator approved", want[1])
		}
		if before > after {
			t.Errorf("execute calls %s before %s (order: %v), and the pin has to sit between "+
				"the stage that fills it and the first stage that acts on it", want[1], want[0], calls)
		}
	}

	// Preview forces --plan itself. A Preview that ran the request as given
	// would be the pass that writes the target, silently, whatever it was
	// handed.
	if !assignsTrue(f, "Preview", "r.req.PlanOnly") {
		t.Error("Preview does not set r.req.PlanOnly = true: the pass that fills the review screens " +
			"would write the target it is reviewing")
	}
}

// receiverCalls is the ordered list of methods a function calls on its
// receiver: every `r.name(...)` in the body, in source order.
func receiverCalls(f *ast.File, fn string) []string {
	var calls []string
	ast.Inspect(funcBody(f, fn), func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
			if id, ok := sel.X.(*ast.Ident); ok && id.Name == "r" {
				calls = append(calls, sel.Sel.Name)
			}
		}
		return true
	})
	return calls
}

// assignsTrue reports whether fn's body assigns the literal true to target.
func assignsTrue(f *ast.File, fn, target string) bool {
	found := false
	ast.Inspect(funcBody(f, fn), func(n ast.Node) bool {
		assign, ok := n.(*ast.AssignStmt)
		if !ok || len(assign.Lhs) != 1 || len(assign.Rhs) != 1 {
			return true
		}
		if types.ExprString(assign.Lhs[0]) == target && types.ExprString(assign.Rhs[0]) == "true" {
			found = true
		}
		return true
	})
	return found
}

// funcBody is the body of the named function or method in f.
func funcBody(f *ast.File, name string) ast.Node {
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if ok && fn.Name.Name == name && fn.Body != nil {
			return fn.Body
		}
	}
	panic("no function " + name + " in run.go: this test no longer guards anything")
}
