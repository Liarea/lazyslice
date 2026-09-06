// SPDX-License-Identifier: Apache-2.0

package pipeline

import "github.com/Liarea/lazyslice/mask"

// Category is what a column holds. The list is the rule pack's vocabulary; a
// category with no registered masker cannot exist, and a test walks both lists
// to keep that true (ADR-006).
type Category string

// The v1 categories (ARCHITECTURE.md section 4).
const (
	CatNone       Category = "none"
	CatPersonName Category = "person_name"
	CatEmail      Category = "email"
	CatPhone      Category = "phone"
	CatAddress    Category = "address"
	CatGeo        Category = "geo"
	CatPersonDate Category = "person_date"
	CatNationalID Category = "national_id"
	CatFinancial  Category = "financial_account"
	CatNetworkID  Category = "network_id"
	CatOnlineID   Category = "online_id"
	CatCredential Category = "credential"
	CatFreeText   Category = "free_text"
	CatSpecial    Category = "special_category"
	CatBinary     Category = "binary_personal"
	CatSemiStruct Category = "semi_structured"
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
	// Refused is non-empty when a unique column's domain is too small for the
	// planned row count (exit 12).
	Refused string
}

// Classification is every decision for one run.
type Classification struct {
	Decisions map[ColumnRef]Decision
	Drift     []ColumnRef // columns the committed yml had never seen
	Expired   []ColumnRef // opt-outs ignored because TypeFP changed
	// Fingerprint is sha256 over (rule-pack version, and per column: category,
	// masker)[:16]. A change is printed as "classification changed - masked
	// values will differ", because the mapping depends on the category.
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
