// Package pg is the only package that speaks Postgres. It implements
// pipeline.Source, pipeline.Target, pipeline.Reader, pipeline.Writer and the
// target gate, and it registers the shape allowlist on the source pool.
//
// The read side is defended in three independent ways (ADR-005): every source
// transaction is REPEATABLE READ READ ONLY; all five pgx tracers are registered
// on the source pool, and a statement whose shape is not on the allowlist gets a
// cancelled context from TraceQueryStart and a recorded violation; and
// TestSourceNeverCopiesOrBatches fails if this package calls CopyFrom, SendBatch
// or Prepare on a source connection. Any one of them would do; all three are
// cheap, and the source is production.
//
// The write side is defended by the gate, which runs after discovery and never
// inside the 1 s dial. Verdict is tri-state so that "not probed" can never be
// read as eligible: any error, timeout or unprobed table is Refused.
//
// Scaffold status: no-op apart from RenderError. Every method returns
// pipeline.ErrNotImplemented.
package pg

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Liarea/lazyslice/internal/dsn"
	"github.com/Liarea/lazyslice/internal/pipeline"
)

// Connect opens a pool. The source and the target take different pools with
// different tracers, so this is the one place a connection is made.
//
// Scaffold status: no-op.
func Connect(_ context.Context, _ dsn.DSN) (*pgxpool.Pool, error) {
	return nil, fmt.Errorf("pg: connect: %w", pipeline.ErrNotImplemented)
}

// RenderError turns a Postgres error into something safe to print: Message and
// SQLSTATE. Detail, Where and Hint are dropped unless
// --show-row-values-in-errors, because a unique-violation Detail quotes the
// conflicting row (THREAT_MODEL.md T4).
//
// It is real rather than a no-op because cmd/lazyslice routes every error it
// prints through it: a placeholder here would be a silent hole in the one
// redaction pass the binary has.
func RenderError(e *pgconn.PgError, showValues bool) string {
	if e == nil {
		return ""
	}
	s := fmt.Sprintf("%s (SQLSTATE %s)", e.Message, e.Code)
	if !showValues {
		return s
	}
	// The operator asked for the fields that quote the row, by a flag whose
	// name says so.
	for _, f := range []struct{ label, value string }{
		{"detail", e.Detail},
		{"where", e.Where},
		{"hint", e.Hint},
	} {
		if f.value != "" {
			s += "\n  " + f.label + ": " + f.value
		}
	}
	return s
}

// NewSource returns the read side.
func NewSource(p *pgxpool.Pool) pipeline.Source { return &source{pool: p} }

type source struct{ pool *pgxpool.Pool }

var _ pipeline.Source = (*source)(nil)

func (s *source) Privileges(_ context.Context) (pipeline.RolePrivileges, error) {
	return pipeline.RolePrivileges{}, fmt.Errorf("pg: privileges: %w", pipeline.ErrNotImplemented)
}

func (s *source) Snapshot(_ context.Context) (pipeline.SnapshotID, error) {
	return "", fmt.Errorf("pg: snapshot: %w", pipeline.ErrNotImplemented)
}

func (s *source) Reader(_ context.Context, _ pipeline.SnapshotID) (pipeline.Reader, error) {
	return nil, fmt.Errorf("pg: reader: %w", pipeline.ErrNotImplemented)
}

func (s *source) Short(_ context.Context) (pipeline.Reader, error) {
	return nil, fmt.Errorf("pg: short: %w", pipeline.ErrNotImplemented)
}

func (s *source) Release(_ context.Context) error {
	return fmt.Errorf("pg: release: %w", pipeline.ErrNotImplemented)
}

func (s *source) Trace() []pipeline.TracedStatement { return nil }

// NewTarget returns the write side.
func NewTarget(p *pgxpool.Pool) pipeline.Target { return &target{pool: p} }

type target struct{ pool *pgxpool.Pool }

var _ pipeline.Target = (*target)(nil)

func (t *target) Gate(_ context.Context, _ dsn.Ref, _, _ string) (pipeline.Eligibility, error) {
	// NotProbed is the zero value and it is not Eligible, so a gate that never
	// ran refuses the write rather than allowing it.
	return pipeline.Eligibility{Verdict: pipeline.NotProbed},
		fmt.Errorf("pg: gate: %w", pipeline.ErrNotImplemented)
}

func (t *target) Writer(_ context.Context) (pipeline.Writer, error) {
	return nil, fmt.Errorf("pg: writer: %w", pipeline.ErrNotImplemented)
}

// Shape is one entry in the source statement allowlist. Every statement the run
// sends to the source must match a registered shape by name; anything else is
// refused by the tracer before it reaches the server.
type Shape struct {
	Name string
	// SQL is the statement template with parameters as placeholders. It is what
	// the trace records, so it never holds a value.
	SQL string
	// Added is when the shape was registered, for the trace.
	Added time.Time
}
