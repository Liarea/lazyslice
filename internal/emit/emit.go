// SPDX-License-Identifier: Apache-2.0

// Package emit reads and writes lazyslice.yml (ARCHITECTURE.md section 10).
//
// The file is a record of what happened, not a configuration to be filled in:
// it is written on success, and re-running with it asks no questions. Read back
// it can only tighten (ADR-004) — a pattern may add a category or raise a
// confidence, an opt-out expires when the column's type changes, and nothing in
// it can lower a confidence or widen a slice.
//
// It never contains a secret and never contains a row value. A --where
// predicate holding a literal is withheld and recorded as where_fingerprint; a
// later run that finds a fingerprint and no --where stops with exit 2 asking for
// the predicate, rather than silently changing the slice.
//
// go-yaml is used rather than the standard library's because the emitted file
// carries the explanatory header comments that make it readable.
//
// Scaffold status: no-op. Every method returns pipeline.ErrNotImplemented.
package emit

import (
	"fmt"

	"github.com/goccy/go-yaml"

	"github.com/Liarea/lazyslice/internal/pipeline"
)

type emitter struct{}

// New returns the yml emitter.
func New() pipeline.Emitter { return emitter{} }

var _ pipeline.Emitter = emitter{}

func (emitter) Emit(
	_ *pipeline.Plan,
	_ *pipeline.Classification,
	_ *pipeline.Report,
	_ pipeline.PlanRequest,
	_, _ pipeline.Candidate,
	_ string,
) (*pipeline.Config, error) {
	return nil, fmt.Errorf("emit: %w", pipeline.ErrNotImplemented)
}

func (emitter) Write(_ string, _ *pipeline.Config) error {
	return fmt.Errorf("emit: %w", pipeline.ErrNotImplemented)
}

func (emitter) Read(_ string) (*pipeline.Config, error) {
	return nil, fmt.Errorf("emit: %w", pipeline.ErrNotImplemented)
}

// unmarshal is the single decoding entry point, kept here so that every read of
// the yml goes through the same options. Strict mode is deliberate: an unknown
// key in lazyslice.yml is a typo in a safety setting, and silently ignoring it
// is how an --unmask that never applied looks like one that did.
func unmarshal(b []byte, v any) error {
	return yaml.UnmarshalWithOptions(b, v, yaml.Strict())
}

// Decode parses a lazyslice.yml body. It is exported for the reader that
// internal/core uses before the pipeline starts.
//
// Scaffold status: it parses, but into whatever it is given; the Config
// mapping arrives with the emit task.
func Decode(b []byte, v any) error { return unmarshal(b, v) }
