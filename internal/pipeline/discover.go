// SPDX-License-Identifier: Apache-2.0

package pipeline

import (
	"context"

	"github.com/Liarea/lazyslice/internal/dsn"
	"github.com/Liarea/lazyslice/internal/event"
)

// Provenance records which rung of the discovery ladder produced a candidate
// (ARCHITECTURE.md section 9). It is printed beside every candidate, because a
// developer must be able to see why lazyslice thinks a database is theirs.
type Provenance int

// The ladder, in order. A lower rung wins.
const (
	FromYml              Provenance = iota // ./lazyslice.yml (rung 0)
	FromEnvVar                             // DATABASE_URL, .env* (rung 1)
	FromLibpq                              // PGHOST/PGSERVICE (rung 2)
	FromContainer                          // running Postgres container (rung 3)
	FromStoppedContainer                   // exited container (rung 4)
	FromCompose                            // compose service name only, never a DSN (rung 5)
	FromFlag                               // --source / --target
)

// Candidate is one database discovery found. Everything in it is either an
// identifier or a count; Ref is redacted by construction.
type Candidate struct {
	Ref        dsn.Ref // host, port, database, user, allowlisted params; String never prints a password
	Provenance Provenance
	Label      string // compose service, container name, env var name
	Local      bool   // loopback, or a container whose compose working_dir matches cwd
	Reachable  bool
	ConnectErr string // sanitised; empty when reachable
	Version    int    // server major, 0 when unreachable
	Tables     int    // user tables, from pg_class; the gate's cap is checked against this
	// Empty is nil at discovery, always. Only Target.Gate fills it, because
	// emptiness is a per-table probe and discovery has a 1 s budget.
	Empty *bool
	// EmptyHint comes from one batched pg_class query: "no user table has
	// relpages > 0". It renders as "probably empty" and never gates a write.
	EmptyHint  bool
	Marked     bool   // lazyslice_meta present
	MetaSchema int    // lazyslice_meta.schema_version, 0 when absent
	CanCreate  bool   // has_schema_privilege(current_user, 'public', 'CREATE')
	SystemID   string // pg_control_system().system_identifier when callable
}

// IsZero reports whether c carries no data at all — every field at its zero
// value, Ref included.
//
// Candidate embeds dsn.Ref by value, and Ref stopped being comparable with ==
// once Params (T-0135) made it hold a map: a struct is comparable only when
// every field is, so Candidate lost == right along with it. This is what the
// one place inside internal/pipeline's own paths that used to write
// `== (Candidate{})` needed instead; internal/core/provenance_test.go is the
// same comparison outside them, filed as T-0165 and fixed alongside this.
func (c Candidate) IsZero() bool {
	return c.Ref.IsZero() &&
		c.Provenance == FromYml && // Provenance's zero value
		c.Label == "" &&
		!c.Local &&
		!c.Reachable &&
		c.ConnectErr == "" &&
		c.Version == 0 &&
		c.Tables == 0 &&
		c.Empty == nil &&
		!c.EmptyHint &&
		!c.Marked &&
		c.MetaSchema == 0 &&
		!c.CanCreate &&
		c.SystemID == ""
}

// Discoverer walks the discovery ladder.
type Discoverer interface {
	// Discover walks the ladder in ARCHITECTURE.md section 9 with a 2 s listing
	// budget and a 1 s per-candidate dial timeout, emitting each candidate as it
	// resolves. Inside the dial it runs at most three statements per candidate:
	// version, the pg_class count and hint, and to_regclass('lazyslice_meta').
	// It never probes emptiness table by table; that is the gate's job, after
	// discovery, and the decision header prints "checking..." until the gate
	// answers.
	Discover(ctx context.Context, workdir string, sink event.Sink) ([]Candidate, error)
}
