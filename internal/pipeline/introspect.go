// SPDX-License-Identifier: Apache-2.0

package pipeline

import "context"

// Schema and the types below carry everything schema recreation needs
// (ARCHITECTURE.md section 11.1) and count what it does not. Definition text
// comes from the catalog's own deparser (pg_get_expr, pg_get_constraintdef,
// pg_get_indexdef), never from a printer of ours, so the target receives the
// source's expression verbatim.

// Column is one column of one table.
type Column struct {
	Name      string
	TypeName  string // as pg_catalog.format_type
	TypeOID   uint32
	TypMod    int32
	Collation string // "" when the type's default collation
	Nullable  bool
	Default   string // pg_get_expr(adbin, adrelid); "" when none
	Generated string // pg_get_expr of a generated column's expression; "" when not generated
	Identity  string // "", "a" (always), "d" (by default)
	// IdentitySeq holds the parameters of the identity sequence when Identity is
	// not empty.
	IdentitySeq *SequenceDef
	Domain      string   // CREATE DOMAIN name, if any
	Checks      []string // CHECK expressions naming this column, for classification and masking constraints
	// Fingerprint is sha256 over (TypeOID, TypMod, Nullable, Domain)[:8]. An
	// --unmask opt-out expires when it changes, so a column that was renamed to
	// a new type does not silently keep its exemption.
	Fingerprint string
}

// Index is one index on one table.
type Index struct {
	Name       string
	Columns    []string // empty for an expression index
	Unique     bool
	Partial    bool
	Expression bool
	// Immediate is pg_index.indimmediate: false for the index backing a
	// DEFERRABLE primary key or unique constraint. Postgres refuses such a key
	// as the referenced side of a foreign key ("cannot use a deferrable unique
	// constraint for referenced table"), and nothing else on this struct tells
	// one apart from a usable key.
	Immediate bool
	Def       string // pg_get_indexdef, recreated verbatim in the target
}

// Constraint is one table-level constraint.
type Constraint struct {
	Name string
	Kind byte   // 'p' primary, 'u' unique, 'c' check, 'f' foreign, 'x' exclusion
	Def  string // pg_get_constraintdef
}

// SequenceDef is a sequence and its parameters.
type SequenceDef struct {
	Name      string // schema-qualified
	Column    string // owned column via pg_get_serial_sequence; "" when unowned
	Start     int64
	Increment int64
	Min, Max  int64
	Cache     int64
	Cycle     bool
}

// Table is one source table as introspect found it.
type Table struct {
	Ref         TableRef
	Columns     []Column
	PK          []string
	Indexes     []Index      // every index; the planner filters Unique && !Partial && !Expression
	Constraints []Constraint // table-level, in creation order; FKs also appear in Schema.FKs
	Sequences   []SequenceDef
	// Partitioned is relkind 'p'. A partitioned source table is recreated as one
	// plain table in the target (ARCHITECTURE.md section 11.1), which removes the
	// masked-partition-key trap as a side effect.
	Partitioned  bool
	PartitionKey []string
	Partitions   []TableRef // leaf partitions via pg_inherits, recursively; empty for a plain table
	Parent       *TableRef  // for a partition, its root
	RLS          bool       // relrowsecurity
	ForceRLS     bool       // relforcerowsecurity
	Triggers     int        // user triggers on the source; counted in Schema.NotRecreated
	ApproxRows   int64      // pg_class.reltuples, planning only, never a gate
	// Samples holds up to 200 rows by TABLESAMPLE SYSTEM ... REPEATABLE (seed).
	//
	// This is the only field in this package that holds production values, and
	// it is never serialised: no renderer, subcommand, --json output or --debug
	// path writes it, and TestNoValueBearingFieldSerialised asserts that the
	// types reachable from event.Event, Config, Decision, Plan and Report
	// exclude it (THREAT_MODEL.md T4).
	Samples [][]any
	// SampledFrom names the leaf partition the samples came from, nil for a
	// plain table. TABLESAMPLE is not accepted on a partitioned table.
	SampledFrom *TableRef
}

// Object names a source object v1 does not recreate. Kinds are "view",
// "matview", "function", "procedure", "trigger", "policy", "rule", "comment",
// "privilege", "publication", "foreign_table", "operator", "collation". The
// count per kind is printed by the plan and recorded in the yml; none of them
// is a refusal on its own.
type Object struct{ Kind, Name string }

// Extension is one installed extension a recreated object depends on.
type Extension struct{ Name, Schema string }

// NamedDef is a CREATE statement for an enum, domain or composite type,
// rendered from the catalog at introspect.
type NamedDef struct{ Name, Def string }

// ForeignKey is one edge of the graph the planner walks.
type ForeignKey struct {
	Name       string
	Child      TableRef
	ChildCols  []string
	Parent     TableRef
	ParentCols []string
	MatchFull  bool
	Validated  bool // convalidated; unvalidated FKs are hints
	Virtual    bool // inferred polymorphic edge, parent-direction only
	Indexed    bool // child columns covered by an index
	// NotRecreatable marks an edge ARCHITECTURE.md section 11.1 item 6 cannot
	// replay in the target, so the planner refuses at plan (exit 13,
	// target.schema.not_recreatable) before anything is dropped rather than
	// failing the ADD CONSTRAINT after every user table is gone.
	//
	// It is set by introspect for one case today: an edge that references a
	// leaf partition whose root carries no unique or primary-key constraint
	// over the referenced columns. A partitioned table's unique constraint must
	// include the partition key and a leaf's need not, and section 11.1
	// recreates a leaf's own indexes and constraints nowhere, so there is no key
	// in the target for such an edge to reference.
	//
	// This field is not in ARCHITECTURE.md section 2's ForeignKey; the
	// deviation is recorded in internal/introspect/CLAUDE.md and section 2 is
	// owed the same line.
	NotRecreatable bool
}

// Schema is the whole source catalog, as much of it as v1 understands.
type Schema struct {
	ServerVersion int
	Schemas       []string    // user schemas, ordered
	Extensions    []Extension // every installed extension a recreated column, index or default depends on
	Enums         map[string][]string
	Domains       []NamedDef
	Composites    []NamedDef
	Tables        []Table
	FKs           []ForeignKey
	// NotRecreated lists the object classes v1 leaves behind. It is never a
	// refusal and it is always printed.
	NotRecreated []Object
	// Fingerprint is sha256 over the DDL text internal/load/ddl generates for
	// this Schema — the recreated object classes of section 11.1, in the order
	// it writes them — which is ADR-009's definition and the only one. It is
	// therefore the same hash whether computed on the source or on a target we
	// wrote, which is what the marker's binding needs (section 11.2).
	//
	// Introspector.Introspect does not fill this field: it comes back empty.
	// internal/core fills it after introspection by calling
	// load.SchemaFingerprint, which is the caller that has both halves of
	// section 11.2's binding — the loader writes the marker with that same
	// function and the gate recomputes it through load.GateFingerprint.
	// Nothing may fingerprint a Schema some other way:
	// a marker written with one definition and checked with another binds
	// nothing, and section 11.2 then refuses a target lazyslice itself wrote.
	Fingerprint string
}

// SchemaSummary is what `lazyslice introspect --json` emits. Schema itself is
// never serialised, because it reaches Table.Samples.
type SchemaSummary struct {
	ServerVersion int            `json:"server_version"`
	Schemas       []string       `json:"schemas"`
	Tables        int            `json:"tables"`
	Columns       int            `json:"columns"`
	ForeignKeys   int            `json:"foreign_keys"`
	NotRecreated  map[string]int `json:"not_recreated"`
	Fingerprint   string         `json:"fingerprint"`
}

// Introspector reads the source catalog.
type Introspector interface {
	Introspect(ctx context.Context, r Reader) (*Schema, error)
}

// SchemaOnlyIntrospector is an Introspector that can also read the catalog
// without the samples: the same Schema with Table.Samples and
// Table.SampledFrom left empty.
//
// It is an interface of its own rather than a second method on Introspector so
// that Introspector stays the one ARCHITECTURE.md section 2 declares. A caller
// that wants the cheaper read asserts for this one and falls back to
// Introspect, which returns a superset and is therefore always a correct
// answer to the question — never the reverse, because a caller that needs the
// samples must not silently get a Schema without them.
//
// The one caller is load.GateFingerprint, the target end of section 11.2's
// binding. That fingerprint is sha256 over generated DDL, which reads no
// sample, so sampling there is a TABLESAMPLE per table whose result is
// discarded — inside the REPEATABLE READ transaction internal/pg holds open
// around the call, over rows in a database the gate has not yet agreed to
// touch (THREAT_MODEL.md T4).
type SchemaOnlyIntrospector interface {
	Introspector
	IntrospectSchema(ctx context.Context, r Reader) (*Schema, error)
}
