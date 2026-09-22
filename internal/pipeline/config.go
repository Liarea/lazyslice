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
	// SourceLabel and TargetLabel are Candidate.Label: the compose service, the
	// container name or the environment variable a candidate came from. They
	// are section 10's `service:` key, and they are identifiers — a developer
	// reading the committed file has to be able to see which database of their
	// project this run was against without a DSN in it.
	// PasswordCommand is the --password-command string, a reference. Never its
	// output.
	PasswordCommand string
	SourceLabel     string
	Target          Provenance
	TargetRef       dsn.Ref
	TargetLabel     string
	Root            TableRef
	Take            int
	// PhoneRegion is --phone-region / the yml's own phone_region: the
	// libphonenumber region (an ISO 3166-1 alpha-2 code such as "GB") a
	// national-format phone column is read under, on top of the
	// international-only reading every run already has (T-0221). Empty means
	// none configured, which is when internal/classify's own short,
	// corroboration-gated guess takes over instead of this field deciding
	// anything (internal/classify/CLAUDE.md, "T-0221").
	PhoneRegion string
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
	// Types is --allow-type-literal TYPE=REASON, recorded under the yml's own
	// types: block on the same shape Columns' Unmask has — reason required,
	// expiring when the type's own fingerprint changes. Keyed by the catalog's
	// plain "schema.type" name (the same string internal/core's resolveType
	// returns and PlanRequest.AllowTypeLiterals uses), never quoted or split:
	// unlike a table or a column, a type opt-out is never decomposed back into
	// a structured ref, so there is nothing here for a second identifier shape
	// to disagree with.
	Types map[string]TypeAllow
	Plan  PlanSummary
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
	// Role is Decision.Role, round-tripped the way TypeFP is (T-0287): written
	// under `role:` for a person_name column and read back so a re-run needs no
	// re-derivation, though internal/classify recomputes it from the column's
	// name every run regardless, the same way it recomputes Category.
	Role mask.Role
	// MappingFile is deliberately absent. ADR-006 named a 1:1 CSV mapping as a
	// unique-index escape hatch; ADR-012 defers the full contract (uniqueness,
	// missing values, FK consistency, secret-file protection) past v1, because
	// nothing consumed it — the config decoder read it, emit round-tripped it,
	// and the unique-domain refusal recommended it, but no stage ever read the
	// CSV or applied a replacement (docs/reviews/2026-09-09/REVIEW.md finding
	// 10). A yml naming mapping_file is refused at read (internal/emit); T-0142
	// is the implementing task.
	Unmask *Unmask
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

// TypeAllow is a per-type opt-out: --allow-type-literal TYPE=REASON, or the
// yml's own types: block carrying it forward. There is no wholesale allow, by
// design — the same loosening ADR-004 already allows for Unmask, applied to
// the one object class that is not a column (ARCHITECTURE.md §11.1's fourth
// arm).
type TypeAllow struct {
	// Reason is never empty: --allow-type-literal TYPE=REASON rejects the bare
	// form with exit 2.
	Reason string
	By     string
	// TypeFP is the type's own fingerprint (its enum labels, in catalog order,
	// or its domain's definition) when the opt-out was taken. The opt-out
	// expires when it changes, so a redefined type is refused again rather
	// than staying exempt.
	TypeFP string
}

// PlanSummary is the plan block of the yml: counts and identifiers only.
type PlanSummary struct {
	Tables      int
	Rows        int64
	Lookups     []TableRef
	Unreachable []TableRef
	// Unreadable is section 3.6's list: tables the source role cannot read that
	// were dropped to SchemaOnly and the run continued past. Section 10's plan
	// block carries it beside `unreachable:`; ARCHITECTURE.md section 2's own
	// PlanSummary does not name it, and internal/emit/CLAUDE.md records the
	// addition.
	Unreadable   []TableRef
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
