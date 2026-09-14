// SPDX-License-Identifier: Apache-2.0

package core

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/Liarea/lazyslice/mask"
)

// T-0161's plan-only requirement — "do not create a secret for a plan-only
// run" — was asserted nowhere before the 2026-09-14 review (finding 3):
// `grep -rn "keyBeforePlan\|resolveKeyIfPresent" --include='*_test.go' .`
// found only comment mentions, and replacing resolveKeyIfPresent's body with
// `return nil` passed every other test in the tree. These three cases are
// the ones that make a stub fail.

// unsetLazysliceSecret clears $LAZYSLICE_SECRET for the duration of the test
// and restores whatever the environment held before, so a developer's own
// shell setting cannot make these tests pass or fail for the wrong reason.
func unsetLazysliceSecret(t *testing.T) {
	t.Helper()
	if orig, ok := os.LookupEnv("LAZYSLICE_SECRET"); ok {
		t.Cleanup(func() { os.Setenv("LAZYSLICE_SECRET", orig) })
	}
	os.Unsetenv("LAZYSLICE_SECRET")
}

// (a) A plan-only run with neither $LAZYSLICE_SECRET nor a committed
// lazyslice.secret leaves the secret file absent and r.keyFP empty — the one
// case a `return nil` stub also satisfies, and the one the review's finding 2
// was about: resolveKeyIfPresent must not touch the filesystem to answer it.
func TestResolveKeyIfPresentWithNoSecretLeavesNothingOnDisk(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	unsetLazysliceSecret(t)

	r := &run{req: normalise(Request{}), sink: &collector{}}
	if err := r.resolveKeyIfPresent(); err != nil {
		t.Fatalf("resolveKeyIfPresent: %v", err)
	}
	if r.keyFP != "" {
		t.Errorf("r.keyFP = %q, want empty: no key exists for a plan-only run to find", r.keyFP)
	}
	if _, err := os.Stat(filepath.Join(dir, r.req.SecretFile)); !os.IsNotExist(err) {
		t.Errorf("%s exists after a plan-only run with no key; it must never be created merely by planning", r.req.SecretFile)
	}
	if _, err := os.Stat(filepath.Join(dir, ".gitignore")); !os.IsNotExist(err) {
		t.Error(".gitignore was created by a read-only plan-only path (2026-09-14 review, finding 2)")
	}
}

// (b) A plan-only run with a secret file already present fills r.key/r.keyFP,
// and the key reaches planRequest().Key — the state ddlliteral.go needs to
// mask a default instead of reporting it pending.
func TestResolveKeyIfPresentWithExistingSecretFillsTheKey(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	unsetLazysliceSecret(t)

	k, err := mask.NewKey()
	if err != nil {
		t.Fatalf("mask.NewKey: %v", err)
	}
	req := normalise(Request{Mode: ModePlan})
	if writeErr := os.WriteFile(filepath.Join(dir, req.SecretFile), []byte(hexKey(k)+"\n"), 0o600); writeErr != nil {
		t.Fatalf("writing the secret file: %v", writeErr)
	}

	r := &run{req: req, sink: &collector{}}
	if resolveErr := r.resolveKeyIfPresent(); resolveErr != nil {
		t.Fatalf("resolveKeyIfPresent: %v", resolveErr)
	}
	if r.keyFP != k.Fingerprint() {
		t.Fatalf("r.keyFP = %q, want %q: the committed key was not read", r.keyFP, k.Fingerprint())
	}

	planReq, err := r.planRequest()
	if err != nil {
		t.Fatalf("planRequest: %v", err)
	}
	if planReq.Key == nil {
		t.Fatal("planRequest().Key is nil although a key was found on disk; ddlliteral.go would report the column pending instead of masking it")
	}
	if !planReq.KeyPending {
		t.Error("planRequest().KeyPending is false for a plan-only run; keyBeforePlan and planRequest must agree it is one")
	}
}

// (c) A writing run with neither the environment variable nor a secret file
// present creates one, through resolveKey rather than resolveKeyIfPresent.
func TestResolveKeyWithNoSecretCreatesOne(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	unsetLazysliceSecret(t)

	r := &run{req: normalise(Request{}), sink: &collector{}}
	if err := r.resolveKey(); err != nil {
		t.Fatalf("resolveKey: %v", err)
	}
	if r.keyFP == "" {
		t.Fatal("r.keyFP is empty after resolveKey with no key anywhere; a writing run must resolve one")
	}
	if _, err := os.Stat(filepath.Join(dir, r.req.SecretFile)); err != nil {
		t.Errorf("%s: %v; a writing run must persist the key it created (section 9, no repository case)", r.req.SecretFile, err)
	}

	planReq, err := r.planRequest()
	if err != nil {
		t.Fatalf("planRequest: %v", err)
	}
	if planReq.KeyPending {
		t.Error("planRequest().KeyPending is true for a writing run; only a plan-only run may leave a masked default's key unresolved")
	}
}

// keyBeforePlan dispatches to resolveKeyIfPresent for ModePlan and for
// Preview's PlanOnly, and to resolveKey for everything else — the same
// condition planOnly() states once, read by planRequest() too. This pins the
// dispatch itself, so a stub of either callee or a drift between the two
// planOnly() call sites shows up here rather than only in end-to-end coverage.
func TestKeyBeforePlanDispatchesOnPlanOnly(t *testing.T) {
	cases := []struct {
		name       string
		mode       Mode
		planOnly   bool
		wantCreate bool
	}{
		{"plan mode", ModePlan, false, false},
		{"preview plan-only", ModeRun, true, false},
		{"writing run", ModeRun, false, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			t.Chdir(dir)
			unsetLazysliceSecret(t)

			req := normalise(Request{})
			req.Mode = tc.mode
			req.PlanOnly = tc.planOnly
			r := &run{req: req, sink: &collector{}}

			if err := r.keyBeforePlan(); err != nil {
				t.Fatalf("keyBeforePlan: %v", err)
			}
			_, statErr := os.Stat(filepath.Join(dir, r.req.SecretFile))
			created := statErr == nil
			if created != tc.wantCreate {
				t.Errorf("secret file created = %v, want %v", created, tc.wantCreate)
			}
		})
	}
}

// The structural pin for T-0161's ordering: execute has to call keyBeforePlan
// before planStage, or a writing run reaches ddlliteral.go with the key that
// keyBeforePlan never resolved. Reached through the AST the way
// TestTheFingerprintIsRecomputedAfterThePlanAndBeforeItIsWritten
// (refingerprint_test.go) and TestTheReviewPinIsWiredIntoTheRunItGuards
// (reviewed_test.go) pin their own orderings, and for the same reason:
// deleting the call, or moving it after planStage, would leave every other
// test in this package green.
func TestKeyBeforePlanRunsBeforePlanStage(t *testing.T) {
	t.Parallel()

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filepath.Join(".", "run.go"), nil, 0)
	if err != nil {
		t.Fatalf("parse run.go: %v", err)
	}

	calls := receiverCalls(f, "execute")
	at := func(name string) int { return slices.Index(calls, name) }

	before, after := at("keyBeforePlan"), at("planStage")
	if before < 0 {
		t.Fatalf("execute never calls keyBeforePlan (order: %v); a writing run would resolve its key "+
			"a stage late again, inside move(), and every masked default whose literal a strong "+
			"validator hits would be refused at exit 13 instead of masked (T-0161)", calls)
	}
	if after < 0 {
		t.Fatalf("execute never calls planStage (order: %v)", calls)
	}
	if before > after {
		t.Errorf("execute calls planStage before keyBeforePlan (order: %v); "+
			"internal/plan/ddlliteral.go needs pipeline.PlanRequest.Key filled before it runs", calls)
	}
}

// The 2026-09-14 review's second finding: keyBeforePlan and planRequest read
// one planOnly() method, but execute's own plan-only early return
// (immediately before r.emitPlanOnly()) still hand-repeated the same
// predicate as a literal `r.req.Mode == ModePlan || r.req.PlanOnly` rather
// than calling planOnly(). That left three copies of one condition instead of
// one, and if the literal ever narrowed relative to planOnly(), a run for
// which planOnly() returned true would skip the early return and reach
// move() with r.key at its zero value — a structurally valid, all-zero
// mask.Key that move() would mask every value under with no error.
//
// This asserts the guard itself is a call to r.planOnly(), not merely that
// keyBeforePlan runs before planStage (TestKeyBeforePlanRunsBeforePlanStage,
// above): replacing the guard with the equivalent inline expression passes
// that test and every other test in the package while reintroducing the
// drift the review found.
func TestExecutePlanOnlyGuardCallsPlanOnly(t *testing.T) {
	t.Parallel()

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filepath.Join(".", "run.go"), nil, 0)
	if err != nil {
		t.Fatalf("parse run.go: %v", err)
	}

	var guard ast.Expr
	ast.Inspect(funcBody(f, "execute"), func(n ast.Node) bool {
		ifStmt, ok := n.(*ast.IfStmt)
		if !ok {
			return true
		}
		// Find the if whose body calls r.emitPlanOnly() — that is the
		// plan-only early return this pins, regardless of how its
		// condition is written.
		for _, stmt := range ifStmt.Body.List {
			ret, ok := stmt.(*ast.ReturnStmt)
			if !ok {
				continue
			}
			for _, res := range ret.Results {
				call, ok := res.(*ast.CallExpr)
				if !ok {
					continue
				}
				if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "emitPlanOnly" {
					guard = ifStmt.Cond
				}
			}
		}
		return true
	})

	if guard == nil {
		t.Fatal("execute has no `if ... { return nil, r.emitPlanOnly() }` guard to inspect")
	}

	call, ok := guard.(*ast.CallExpr)
	if !ok {
		t.Fatalf("execute's plan-only guard is %s, not a call to r.planOnly(); "+
			"this is the same two-hand-kept-copies defect the condition at run.go:362 had "+
			"before the 2026-09-14 review fix, moved back", types.ExprString(guard))
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "planOnly" {
		t.Fatalf("execute's plan-only guard calls %s, not r.planOnly()", types.ExprString(guard))
	}
	if id, ok := sel.X.(*ast.Ident); !ok || id.Name != "r" {
		t.Fatalf("execute's plan-only guard is %s, not r.planOnly()", types.ExprString(guard))
	}
}
