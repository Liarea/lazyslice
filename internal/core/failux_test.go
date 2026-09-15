// SPDX-License-Identifier: Apache-2.0

package core

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

// TestCloseUnblocksTransformOnAPanicUnwind pins the deadlock the 2026-09-14
// review of T-FAILUX found (finding 1). If load.Load panics on the main
// goroutine, the unwind out of move still runs move's own deferred
// cancelExtract, which unblocks extract, but nothing used to cancel the
// context transform's masked-send select blocks on (batchBuffer is 2, so two
// batches fill masked and the third blocks forever) — so close's own join on
// transformDone hung forever, and the process never reported the panic or
// exited.
//
// Driving an actual panic out of load.Load through move needs a live
// extract/transform/load pipeline (a real schema, plan, reader and writer),
// which this package's unit tests cannot build without a database — the same
// reason TestTheReviewPinIsWiredIntoTheRunItGuards and
// TestTheFingerprintIsRecomputedAfterThePlanAndBeforeItIsWritten test their
// wiring rather than driving it end to end. This test instead reproduces the
// exact shape a mid-load panic leaves behind — extractDone already filled (as
// cancelExtract unblocking extract would leave it) and transformDone's
// goroutine still parked in the same select move's transform goroutine blocks
// in — and asserts close still returns instead of hanging.
func TestCloseUnblocksTransformOnAPanicUnwind(t *testing.T) {
	r := &run{}

	pipelineCtx, cancelPipeline := context.WithCancelCause(context.Background())
	r.abortStages = func() { cancelPipeline(nil) }

	// extract's goroutine already sent its result, the same as it does on
	// every path (here, as if cancelExtract's defer had already unblocked it
	// during the unwind).
	r.extractDone = make(chan error, 1)
	r.extractDone <- nil

	// transform's goroutine, blocked exactly where move's own blocks: in the
	// select over `masked <- out` / `<-pipelineCtx.Done()`, with masked full
	// and nobody reading it. Only cancelling pipelineCtx can move this
	// forward.
	r.transformDone = make(chan error, 1)
	go func() {
		<-pipelineCtx.Done()
		r.transformDone <- pipelineCtx.Err()
	}()

	done := make(chan struct{})
	go func() {
		r.close(context.Background())
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("close deadlocked joining transformDone: abortStages did not unblock " +
			"transform's masked-send select on the panic-unwind path")
	}
}

// TestIntrospectAndPreviewPromoteASinkPanic pins the fix for the 2026-09-14
// review of T-FAILUX's finding 2: Run's defer promotes a sink's recovered
// panic (ch.panicked()) into the run's own error before ch.close(), but
// Introspect and Preview shared eventChannel and only called ch.close(),
// so a panicking sink during `lazyslice introspect` or the TUI's preview pass
// was recovered and never reported — nil error, exit 0, with whatever output
// the panic interrupted.
//
// Driving an actual sink panic through Introspect or Preview needs a live
// discover/introspect pass against a real source, which is an integration
// concern; the wiring itself is asserted structurally, the way
// TestTheReviewPinIsWiredIntoTheRunItGuards asserts Preview's own PlanOnly
// wiring in the same file.
func TestIntrospectAndPreviewPromoteASinkPanic(t *testing.T) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filepath.Join(".", "run.go"), nil, 0)
	if err != nil {
		t.Fatalf("parse run.go: %v", err)
	}

	for _, fn := range []string{"Introspect", "Preview"} {
		calls := selectorCalls(funcBody(f, fn), "ch")
		panicked := slices.Index(calls, "panicked")
		closeIdx := slices.Index(calls, "close")
		if panicked < 0 {
			t.Errorf("%s's defer never calls ch.panicked(): a sink panic is recovered on the "+
				"drain goroutine and never surfaced, so %s returns its result and a nil error "+
				"whatever the sink's panic actually was", fn, fn)
			continue
		}
		if closeIdx < 0 {
			t.Fatalf("%s never calls ch.close(): this test no longer guards what it thinks it does", fn)
		}
		if panicked > closeIdx {
			t.Errorf("%s calls ch.panicked() after ch.close() (order: %v): panicked() must run "+
				"before close so a recovered panic can still be reported through r.report while "+
				"the sink is still open to receive it", fn, calls)
		}
	}
}

// selectorCalls is the ordered list of methods a function calls on the named
// receiver: every `recv.Method(...)` in the body, in source order, including
// inside nested function literals (a defer's closure, in particular).
func selectorCalls(n ast.Node, recv string) []string {
	var calls []string
	ast.Inspect(n, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
			if id, ok := sel.X.(*ast.Ident); ok && id.Name == recv {
				calls = append(calls, sel.Sel.Name)
			}
		}
		return true
	})
	return calls
}
