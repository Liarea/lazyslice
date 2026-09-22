// SPDX-License-Identifier: Apache-2.0

package pipeline

import "github.com/Liarea/lazyslice/mask"

// Category is what a column holds. The list is the rule pack's vocabulary; a
// category with no registered masker cannot exist, and a test walks both lists
// to keep that true (ADR-006).
type Category string

// The v1 categories (ARCHITECTURE.md section 4).
const (
	CatNone        Category = "none"
	CatPersonName  Category = "person_name"
	CatEmail       Category = "email"
	CatPhone       Category = "phone"
	CatAddress     Category = "address"
	CatGeo         Category = "geo"
	CatPersonDate  Category = "person_date"
	CatNationalID  Category = "national_id"
	CatFinancial   Category = "financial_account"
	CatNetworkID   Category = "network_id"
	CatOnlineID    Category = "online_id"
	CatCredential  Category = "credential"
	CatFreeText    Category = "free_text"
	CatSpecial     Category = "special_category"
	CatBinary      Category = "binary_personal"
	CatSemiStruct  Category = "semi_structured"
	CatDerivedText Category = "derived_text"
)

// Confidence is how sure the classifier is. The scale is biased to recall: the
// failure mode is mask more, never less.
type Confidence int

// The confidence levels. ConfPossible is the mask threshold.
const (
	ConfNone Confidence = iota
	ConfLow
	ConfPossible // mask threshold
	ConfLikely
	ConfCertain
)

// DecisionSource records what produced a decision, so that the explanation can
// say whether a human, a rule or a propagation rule decided.
type DecisionSource int

// Where a decision came from.
const (
	ByClassifier DecisionSource = iota
	ByYmlUnmask
	ByFlagUnmask
	ByYmlRaise
	ByFKPropagation
	ByNeighbour // neighbouring-column rule
)

// Decision is the classifier's verdict on one column.
type Decision struct {
	Col        ColumnRef
	Category   Category
	Confidence Confidence
	// Reason is not free-form. It is rendered from the fixed template set in
	// internal/classify/reasons.go, whose placeholders admit only identifiers
	// (column, table, pattern name, dictionary name, validator name) and counts.
	// TestReasonGrammar parses every emitted reason back against the template set
	// and fails on any string a template did not produce, so a sample value can
	// never appear in a reason, in the yml or in an event.
	Reason string
	Masker mask.ID
	Masked bool
	// Role is a person_name column's sub-category (T-0287): mask.RoleGiven or
	// mask.RoleFamily when the column's name says so, mask.RoleFull (the zero
	// value) otherwise. internal/classify sets it at decision time for every
	// CatPersonName column, masked or not; internal/transform carries it onto
	// mask.Constraints.Role, the way UniqueIndex below reaches Constraints.Unique,
	// and internal/emit writes and reads it back under `columns:`'s `role:` --
	// the same route Config.PhoneRegion follows to reach a masker, except
	// per-column rather than per-run because the role a name column plays is a
	// property of the column, not of the source database.
	Role   mask.Role
	Source DecisionSource
	TypeFP string // Column.Fingerprint at decision time
	// UniqueIndex reports that the column sits under a unique index, so the
	// generator must fit d_required.
	UniqueIndex bool
	// Domain is the admissible output domain, min(column domain, generator
	// Domain()); 0 when unknown.
	Domain int64
	// SmallDomain reports Domain < 2 x distinct sampled values, which makes the
	// masking a substitution recoverable by frequency. It is printed, listed in
	// the yml and named under "what the green tick does not prove".
	SmallDomain bool
	// Refused is non-empty when classify has decided the run must refuse
	// this column rather than mask or copy it -- internal/plan turns a
	// non-empty Refused into an exit-12 refusal naming the column (and
	// RefusedPartner, when set), with --unmask the escape for each
	// (internal/plan/fkpair.go, T-0257), the way it already does for a
	// masked column under a unique index whose domain is too small for the
	// planned row count. The one source that sets it today is
	// unknownColumnsBesideCertain's own fkPairs (T-0253,
	// internal/classify/classify.go): a validated foreign key's partner
	// cannot be raised the same way as this column -- it already carries a
	// decision ARCHITECTURE.md §4 forbids overriding, or measured
	// two-letter-code evidence -- so raising this column alone would copy
	// the pair, and refusing the run is the only outcome that never does.
	Refused string
	// RefusedPartner is the foreign-key partner named in Refused's message,
	// set alongside it by the same source. It is the zero ColumnRef when
	// Refused is empty.
	RefusedPartner ColumnRef
	// NameMatchedNationalID reports that rules.yml's national_id name pattern
	// (priority 75) matched this column's name -- internal/classify's own
	// pack.matchColumn answer, carried here independent of whether the type
	// was accepted or which category the decision went on to record (T-0187
	// third review round, finding 1). internal/verify may not import
	// internal/classify (internal/CLAUDE.md's import graph) and so cannot
	// re-run the rule pack itself; this is the one bit its second net's
	// national_id digits-family entry needs, as one of its two corroboration
	// signals, to ask the same question about a numeric column classify
	// decided `none` or `low`.
	NameMatchedNationalID bool
	// TableHasLikelyPersonalColumn reports that another column of this
	// column's table was decided at ConfLikely or ConfCertain --
	// internal/classify's neighbouring-column rule already counts this
	// (neighbouringColumns's own `likely`), carried onto every decision in
	// the table so a caller that does not re-run that rule can still ask the
	// same question about a column decided `none` or `low` (T-0187 third
	// review round, finding 1). It is internal/verify's second corroboration signal
	// for the national_id digits-family entry, alongside
	// NameMatchedNationalID above.
	TableHasLikelyPersonalColumn bool
	// TableHasMaskedPersonalColumn reports that another column of this
	// column's table was decided at ConfPossible or above, is going to be
	// masked (not exempt as a surrogate key or an FK column), and holds a
	// category that identifies a person -- internal/classify's own
	// identifiesAPerson (T-0240, the 2026-09-15 round-4 red team's three A9b
	// replays: docs/reviews/2026-09-15-redteam/round4-still-leaking.json). It
	// is TableHasLikelyPersonalColumn's own question asked at a lower floor:
	// a name-only match with no samples decides a column at ConfPossible, not
	// ConfLikely, so a `msisdn numeric` column the run itself masked as a
	// phone on its name alone was never counted as a personal neighbour by
	// the field above, and a `taxref` column beside it -- bigint or
	// varchar(9), a national identifier with no name or value signal of its
	// own -- read "no name or value signal" and crossed verbatim. A column
	// the run itself masked is evidence about the table whatever confidence
	// line it landed on. It is internal/verify's third corroboration signal
	// for the national_id digits-family and character-family entries,
	// alongside NameMatchedNationalID and TableHasLikelyPersonalColumn above
	// -- kept apart from that field rather than lowering its own floor,
	// because TableHasLikelyPersonalColumn also corroborates
	// internal/classify's own guessed-region phone pass (guessedPhoneColumns,
	// T-0221), which this task's brief did not ask to widen.
	TableHasMaskedPersonalColumn bool
	// NeverMasked reports that internal/classify exempted this column from
	// masking outright, independent of Category or Confidence: a surrogate
	// key, a validated foreign-key child whose parent stayed unmasked, or a
	// generated column (ARCHITECTURE.md §4's key exemption; work.neverMask in
	// internal/classify/classify.go's markNeverMasked, keyChildren and
	// foreignKeys). internal/verify's second net reads it as an unconditional
	// half of its dense-sequence exemption (T-0240 review round, high
	// finding): a column classify has already decided is a surrogate key's
	// own values, or an FK child that mirrors one, is dense by construction
	// -- `order_id`'s values ARE `id`'s, copied verbatim -- and that has
	// nothing to do with what category an unrelated column of the same table
	// was decided under. Gating the exemption on corroboration (the three
	// fields above) let a genuinely personal neighbour of any category
	// cancel it for such a column, which is the shape
	// testdata/regressions/021 exists to keep passing. See
	// internal/verify/secondnet.go's own comment on the point.
	NeverMasked bool
}

// Classification is every decision for one run.
type Classification struct {
	Decisions map[ColumnRef]Decision
	Drift     []ColumnRef // columns the committed yml had never seen
	// Expired lists the columns whose opt-out was not honoured: its TypeFP no
	// longer matches the column's, or it records no TypeFP and no reason at
	// all. THREAT_MODEL.md T3's control is that an opt-out records the
	// column's type fingerprint and is ignored when it changes, so one that
	// records nothing is the fail-open version of it and expires the same way
	// (internal/classify's honourOptOut). The --unmask flag's own opt-out is
	// the single exception: it carries no fingerprint because it is made for
	// this run and dies with it.
	Expired []ColumnRef
	// Fingerprint is sha256 over (rule-pack version, and per column: category,
	// masker)[:16]. A change is printed as "classification changed - masked
	// values will differ", because the mapping depends on the category.
	//
	// Classify computes it, but it is not final when Classify returns: the value
	// that reaches the yml is recomputed *after the plan*, by internal/core's
	// refingerprint, because §5's unique-index rule lets the plan overwrite
	// Decision.Masker and internal/transform masks with what the plan left
	// (T-0101). So this covers the classification plus the plan's unique-index
	// picks, and it is a function of plan inputs too, not of the classification
	// alone: --take, --depth, --skip-table or a different root change the
	// planned row count, which changes what d_required escalates, which moves
	// this value. A table that is SchemaOnly in one run and selected in another
	// can therefore print "classification changed - masked values will differ"
	// for rows that are not in the target at all. That is the conservative
	// direction -- the warning is about values that may differ -- but it is the
	// reason the line can fire without the rule pack or the schema moving.
	Fingerprint string
}

// Sampler hands the classifier the samples introspect already took. The
// classifier never issues SQL: it is pure, so it can be tested without a
// database and re-run over a fixture.
type Sampler interface {
	Samples(col ColumnRef) []any
}

// Classifier decides what every column holds. Classify is pure.
type Classifier interface {
	Classify(schema *Schema, s Sampler, prior *Config) (*Classification, error)
}
