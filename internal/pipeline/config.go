// SPDX-License-Identifier: Apache-2.0

package pipeline

import (
	"github.com/Liarea/lazyslice/internal/dsn"
	"github.com/Liarea/lazyslice/mask"
)

// Config is the in-memory form of lazyslice.yml (ARCHITECTURE.md section 10).
// The classifier reads it as the prior (opt-outs, raises, extra patterns) and
// the emitter writes it.
//
// It holds references and decisions. A field that could hold a secret is a
// compile error by convention, enforced by TestConfigHasNoSecretField; a
// --where predicate containing a literal is withheld and recorded as
// WhereFingerprint. Read back, the file can only tighten (ADR-004).
type Config struct {
	Version int
	Tool    string
	// PlanOnly is written by --plan --config PATH. Such a file carries no
	// snapshot_id, and a later run takes its defaults and priors, re-classifies,
	// prints "from lazyslice.yml (plan only)", and runs the gate as always.
	PlanOnly  bool
	Source    Provenance
	SourceRef dsn.Ref
	// PasswordCommand is the --password-command string, a reference. Never its
	// output.
	PasswordCommand string
	Target          Provenance
	TargetRef       dsn.Ref
	Root            TableRef
	Take            int
	// Where is empty when the predicate held a literal; see WhereFingerprint.
	Where string
	// WhereFingerprint is sha256(where)[:16] when Where was withheld. A later run
	// that finds a fingerprint and no --where stops with exit 2 asking for the
	// predicate, rather than silently changing the slice.
	WhereFingerprint  string
	Cap               int
	Caps              map[TableRef]int
	Depth             int
	RowBudget         int64
	MemoryBudget      int64
	Keys              map[TableRef][]string // --key, recorded; a default the flag overrides
	Skipped           []TableRef            // --skip-table, recorded
	SchemaFingerprint string
	ClassFingerprint  string     // Classification.Fingerprint
	SnapshotID        SnapshotID // "" when PlanOnly
	SecretFingerprint string     // sha256(K)[:8]; never the key
	ExtraPatterns     []Pattern
	Columns           map[ColumnRef]ColumnConfig
	Plan              PlanSummary
}

// Pattern is a user-supplied classification rule. It may add a category or
// raise a confidence and can never lower or remove one:
// TestConfigCannotLowerConfidence fails otherwise. A pluggable classifier is a
// supported way to see less personal data, which is the mode CONCEPT.md refuses.
type Pattern struct {
	Name       string // regex over column or table name
	Category   Category
	Confidence Confidence // the floor this pattern raises to; never lowers
}

// ColumnConfig is one column's entry in the yml.
type ColumnConfig struct {
	Category   Category
	Confidence Confidence
	Reason     string
	Masker     mask.ID
	Unique     bool
	TypeFP     string
	// MappingFile names a CSV of original,replacement for a column refused under
	// a unique index (ADR-006's escape hatch). It contains original values in
	// plaintext, so it is gitignored, refused when git tracks it, and distributed
	// like a source credential. A value absent from the mapping is exit 12, never
	// a fallback to the refused masker.
	MappingFile string
	Unmask      *Unmask
}

// Unmask is a per-column opt-out. There is no wholesale unmask, by design.
type Unmask struct {
	// Reason is never empty: --unmask TABLE.COL=REASON rejects the bare form
	// with exit 2.
	Reason string
	By     string
	// TypeFP is the column's fingerprint when the opt-out was taken. The opt-out
	// expires when it changes, so a column that became something else is masked
	// again rather than staying exempt.
	TypeFP string
}

// PlanSummary is the plan block of the yml: counts and identifiers only.
type PlanSummary struct {
	Tables       int
	Rows         int64
	Lookups      []TableRef
	Unreachable  []TableRef
	SCCs         [][]TableRef
	VirtualFKs   []ForeignKey
	NotRecreated map[string]int // Object.Kind to count
	// SmallDomain lists masked columns whose substitution is recoverable by
	// frequency. It is printed under "what the green tick does not prove".
	SmallDomain []ColumnRef
}

// Emitter reads and writes lazyslice.yml. internal/emit implements it over
// Config and holds no type of its own.
type Emitter interface {
	Emit(
		plan *Plan,
		cls *Classification,
		report *Report,
		req PlanRequest,
		source, target Candidate,
		keyFP string,
	) (*Config, error)
	Write(path string, cfg *Config) error
	Read(path string) (*Config, error)
}
