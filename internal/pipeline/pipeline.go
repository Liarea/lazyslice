// Package pipeline holds the stage interfaces and the types that cross between
// stages (ARCHITECTURE.md section 2). It contains no implementations: the
// engine-facing ones live in internal/pg, and classify, plan and transform are
// engine-agnostic packages of their own.
//
// Every type in section 2 lives here and nowhere else, so that there is one
// definition of a Plan, a Decision and a Config to change. internal/emit
// implements Emitter over pipeline.Config and holds no type of its own.
//
// Import graph (ARCHITECTURE.md section 2): pipeline imports ref, event, dsn
// and mask. The stage packages import pipeline. event never imports pipeline,
// so the graph is acyclic and TestImportGraph fails on any edge added the other
// way.
//
// Ordering is by (Schema, Name), then Column, everywhere. Nothing iterates a Go
// map. That is what makes two runs over one snapshot produce byte-identical
// output, which is what makes lazyslice.yml a record of what happened (ADR-004).
package pipeline

import (
	"errors"

	"github.com/Liarea/lazyslice/internal/ref"
)

// ErrNotImplemented is returned by every no-op stage in the scaffold. It is the
// scaffold's single sentinel so that a partially built pipeline says which
// stage is missing rather than returning a zero value that reads as success.
var ErrNotImplemented = errors.New("lazyslice: stage not implemented")

// TableRef and ColumnRef are declared in internal/ref, which imports nothing.
// They are aliased here so the rest of this package reads without the prefix.
type (
	TableRef  = ref.TableRef
	ColumnRef = ref.ColumnRef
)
