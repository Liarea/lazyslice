// SPDX-License-Identifier: Apache-2.0

package pipeline

import (
	"context"

	"github.com/Liarea/lazyslice/mask"
)

// Mode is how a table was reached, which decides whether its children are
// followed. A row's mode is decided once, when it is first popped, and never
// changes: that is both the size control and the privacy control
// (ARCHITECTURE.md section 3).
type Mode int

// The four modes.
const (
	ChildOK    Mode = iota // root, or reached via a child edge; children are followed
	ParentOnly             // reached via a parent edge; a leaf for the child step
	Lookup                 // copied whole
	SchemaOnly             // unreachable; DDL only
)

// IdentityKind records which rung of the row-identity ladder supplied the key,
// because a pseudo-key was only ever probed on a sample and the plan says so.
type IdentityKind int

// The identity rungs (ARCHITECTURE.md section 3.4). There is no ctid rung.
const (
	IdentityPK IdentityKind = iota
	IdentityUnique
	IdentityPseudo
)

// Identity is a table's row identity.
type Identity struct {
	Kind    IdentityKind
	Columns []string
	Types   []uint32 // TypeOID per column, drives Chunk encoding
}

// KeySet is a sorted set of key tuples, ordered by the identity columns in
// order. Single int8 keys are stored as []int64; composite or non-integer keys
// as sorted slabs of encoded tuples.
//
// Bytes is the single source of truth for the memory estimate: the []int64
// implementation returns len x 8 x 2 (the slice, doubled for overhead, which is
// the figure ADR-005 quotes); the slab implementation returns its slab size x 2.
// Iteration order is key order, never Go map order.
//
// There are three ways to read a set, and which one a caller picks is a memory
// decision rather than a matter of taste, because a Chunk holds its own copy of
// the keys it carries:
//
//	Chunks(n)      every chunk at once — a second copy of the whole set
//	EachChunk(n,f) one chunk at a time — the set's peak cost is one chunk
//	FirstChunk(n)  the first chunk alone, nil when the set is empty
//
// Chunks is what a caller that wants the whole set materialised takes;
// internal/extract takes EachChunk, so a table of any size costs it one chunk
// and one batch, and internal/verify takes FirstChunk, because a bounded sample
// of a step at the --row-budget ceiling must not allocate a second copy of that
// step's key set at verify time — after --memory-budget (section 8, exit 11)
// has been checked at plan and can no longer refuse anything.
//
// EachChunk stops at the first error f returns and returns it.
type KeySet interface {
	Len() int
	Bytes() int64
	Chunks(n int) []Chunk // consecutive runs of at most n tuples, in key order
	FirstChunk(n int) Chunk
	EachChunk(n int, f func(Chunk) error) error
}

// Chunk is one unnest argument list: one typed array per identity column, so a
// composite key of k columns is unnest($1, ..., $k) with k parallel arrays.
//
// Column(i) is []int64 for int2/int4/int8, []string for text, varchar, bpchar
// and citext, []pgtype.UUID for uuid, and []string holding the text form for
// every other type. Cast(i) is the matching SQL cast. pgx encodes these without
// registration; []any is never passed to Query, because pgx cannot infer an
// array OID for it.
type Chunk interface {
	Len() int
	Column(i int) any
	Cast(i int) string
}

// Step is one table's place in the plan.
type Step struct {
	Table    TableRef
	Mode     Mode
	Identity Identity
	Keys     KeySet // nil for Lookup and SchemaOnly
	Cap      int    // per parent key per edge, 0 when unused
	Depth    int
	// Why is rendered from a template set like Decision.Reason: "child of orders
	// via order_items.order_id", "parent of ...", "lookup", "unreachable",
	// "unreadable", "skipped". Never free-form.
	Why string
}

// AssumedRowsPerSec is the extract throughput the snapshot-hold estimate
// assumes. It is a constant, printed beside the estimate ("assuming 20,000
// rows/s"), so that the estimate is falsifiable rather than circular; the phase
// 5 performance task recalibrates it from measurement.
const AssumedRowsPerSec = 20_000

// Estimate is what the plan predicts, printed before anything is extracted with
// the flag that changes each number.
type Estimate struct {
	Rows      int64
	Bytes     int64
	KeyMemory int64 // sum of KeySet.Bytes over every step with keys
	// FilterMemory is the residual Bloom filter at 29 bits per masked cell; JSON
	// leaves count individually.
	FilterMemory int64
	// HoldSeconds is measured plan time + Rows / AssumedRowsPerSec. It is the
	// number that tells a developer whether this run will hold a snapshot open
	// on production for a minute or an hour.
	HoldSeconds float64
}

// PlanRequest is the planner's input, built from the flags in
// ARCHITECTURE.md section 8 and from lazyslice.yml.
type PlanRequest struct {
	Root         *TableRef // nil: computed default
	Take         int
	Where        string
	Cap          int
	TableCaps    map[TableRef]int
	Depth        int
	RowBudget    int64
	MemoryBudget int64
	Keys         map[TableRef][]string // --key table=col,col, or the yml's keys: block
	Skip         []TableRef            // --skip-table, or the yml's skipped: block
	// AllowTypeLiterals is --allow-type-literal TYPE=REASON: the qualified name of an enum
	// or a domain whose recreated definition the operator has said in writing
	// does not carry a person's value, to the reason they gave. internal/core
	// fills this from the flag and, for a type not named on this run's command
	// line, from the committed yml's own types: block (Config.Types) — the same
	// shape Keys and Skip above take their yml defaults from — expiring an
	// entry whose recorded fingerprint no longer matches the type's current
	// one, exactly as a column's Unmask does.
	//
	// It is ARCHITECTURE.md §8's per-column escape for the one object class
	// that is not a column. §11.1's type-literal rule (internal/plan's
	// checkTypeLiterals) refuses an enum label or a domain definition carrying
	// a literal a strong validator hits, at exit 13, and nothing can rewrite
	// either — a label is referenced by value by every row of every column of
	// the type, and a domain's DEFAULT belongs to the type rather than to any
	// one column's masker. Without this the refusal had no escape at all:
	// --skip-table drops a table to *schema only*, which still recreates its
	// DDL and still needs the type, so a source schema with one such label was
	// permanently unrunnable (the T-REDFIX review's fourth finding).
	//
	// A reason is required for the same purpose --unmask requires one: an
	// opt-out nobody can review later is not an opt-out. internal/verify's
	// catalog pass honours it too, through Plan.AllowedTypeLiterals, or the run would
	// pass the planner and then fail at exit 9 on the object it was told to
	// allow.
	AllowTypeLiterals map[string]string
	// Priv is what the source role can do, read once by internal/core through
	// Source.Privileges before the snapshot is opened. Section 3.6 is decided
	// from it: the planner consults Priv.Unreadable before the snapshot is used
	// for a key, and names Priv.Role in the GRANT statement a refusal carries.
	// The planner does not read privileges itself — one run, one answer, and
	// the header prints the same role the refusal names.
	Priv RolePrivileges

	// Key is the run key, for the one thing the planner masks: the string
	// literals inside a masked column's DEFAULT (ARCHITECTURE.md section 11.1,
	// amended 2026-09-14). Those literals are recreated in the target verbatim
	// today, so a masked email column with DEFAULT 'ddl.canary@example.org'
	// ships that address into the target's catalog and a later INSERT
	// materialises it again (docs/reviews/2026-09-09 finding 5). Masking them
	// through the column's own masker is what keeps the default a *working*
	// default: the same key, category and generator the rows went through, so
	// the default's value is the one a row holding that literal would have.
	//
	// It is a pointer because a nil key is a real state and not an error: the
	// planner then cannot rewrite anything, and a literal a strong validator
	// hits is refused at exit 13 instead of masked. Nothing else in this stage
	// reads it, no row reaches it, and it never leaves the process.
	Key *mask.Key

	// KeyPending is true when internal/core's keyBeforePlan deliberately did
	// not create a key for this run — a plan-only run with neither
	// $LAZYSLICE_SECRET nor a committed secret file — and is false for every
	// other run, including a writing run.
	//
	// internal/plan/ddlliteral.go reads this, not a nil Key alone, to decide
	// whether a masked default with no key yet is §11.1 arm 1's "report and
	// carry on" case (T-0161) or a defect: a writing run's key is always
	// resolved in full before planStage runs, so a nil Key on a run with
	// KeyPending false is not the plan-only state this field names, and that
	// stage refuses instead of silently skipping the column (2026-09-14
	// review of T-0161, finding 1 — two hand-kept copies of "is this a
	// plan-only run" agreeing only by construction, with nothing pinning
	// them together).
	KeyPending bool

	// Take, Cap and Depth are taken as given: an int cannot tell an unset field
	// from an explicit zero, so internal/core substitutes section 3's defaults
	// before it calls Plan and cmd/lazyslice refuses --take 0, --cap 0 and
	// --depth 0 with exit 2 (T-CORE, 2026-09-06).
}

// Plan is what will happen, in enough detail to say why every row is present.
// It is printed in full before extraction starts.
type Plan struct {
	Root       TableRef
	RootReason string // "23 inbound - 0 outbound FKs; name preference"
	Take       int
	Steps      []Step       // in load order (SCC condensation, topologically sorted, ties by (schema, name))
	SCCs       [][]TableRef // every SCC with more than one table, or a self-cycle
	Virtual    []ForeignKey // inferred polymorphic edges followed
	// Polymorphic names the <x>_type/<x>_id pairs detected but not followed
	// (section 3.2), as "schema.table (type_col, id_col)". It is separate from
	// Unmapped because the two are different findings and a renderer that
	// printed one under the other's heading would say a pair is a value: v1
	// detects pairs and follows none of them, so this list is normally
	// populated and Unmapped is normally empty.
	Polymorphic []string
	// Unmapped names the sampled _type values that map to no table (section
	// 3.2). It stays empty until the mapping half of section 3.2 ships.
	Unmapped   []string
	Unindexed  []ForeignKey
	Skipped    []TableRef // dropped to SchemaOnly by --skip-table
	Unreadable []TableRef // unreachable tables the role cannot read, dropped to SchemaOnly
	Estimate   Estimate
	SnapshotID SnapshotID
	// PendingKeyDefaults names, as "schema.table.column", every masked
	// column whose DEFAULT arm 1 of section 11.1's literal rule would mask
	// through the column's own masker if this run held a key. It is filled
	// only when PlanRequest.Key was nil and the column's default was
	// otherwise rewritable (T-0161): a plan-only run resolves a key only
	// when one already exists, so a run with none plans anyway rather than
	// refusing, and this is what it reports instead of masking. Empty on
	// every run that carries a key, since PlanRequest.Key is nil only for
	// such a run.
	PendingKeyDefaults []string
	// AllowedTypeLiterals names, in name order, every type the operator opted out of
	// with --allow-type-literal (PlanRequest.AllowTypeLiterals) *and* whose name the source
	// schema actually carries. internal/verify's catalog pass reads it so that
	// the two ends of §8's escape mean the same thing, exactly as they do for a
	// column's --unmask: an opt-out the planner honours and the verifier does
	// not is a run that loads and then refuses at exit 9 over the object the
	// operator was told they had allowed.
	AllowedTypeLiterals []string
}

// Planner computes the subset. It is a client-side monotone worklist with
// provenance tags, never SQL pushdown, because the plan must say why each row
// is present and the source must never see a temp table.
type Planner interface {
	Plan(ctx context.Context, r Reader, schema *Schema, cls *Classification, req PlanRequest) (*Plan, error)
}
