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
	// LeafKeys is the per-leaf half of a json or jsonb column's decision
	// (T-0272; the maintainer's arbitrary-JSON decision in T-0143, 2026-09-24).
	// It maps every object key internal/classify saw in the column's sampled
	// documents, spelled exactly as sampled, to the category the rule pack's
	// name rules give that key, or CatNone when no rule names it. A key that
	// itself parses as an email address, a phone number or a Luhn-valid number
	// is never entered: internal/transform masks such a key (json.go's
	// keyCategory), so it is a value and not a name.
	//
	// internal/transform reads it per leaf and internal/verify reads the same
	// map back from the target, which is why it lives here and not in either
	// stage: a leaf under a key the map names with a category is masked by that
	// category's masker; a leaf whose value a validator recognises is masked by
	// the validator's category; a leaf whose every enclosing key is in the map
	// as CatNone and whose value no validator recognises is copied; every other
	// leaf -- a key the samples never showed, or no key at all -- is masked as
	// free_text, as every leaf was before. nil (no samples, a decision nothing
	// sampled, a column that is not json or jsonb) keeps that last rule for
	// every leaf, so a missing map masks more and never less.
	//
	// It holds key names read from production documents, so the map itself
	// is in memory only and never rendered into a reason. What leaves the
	// process is RecordedLeafKeys (T-0404), below: internal/classify's own
	// spelling of the keys the map calls CatNone, a key with an identifier's
	// shape as itself and any other as a fingerprint.
	//
	// Nothing reads the field directly: internal/transform and internal/verify
	// read it through LeafMap, which is what makes the column's own decision
	// the root of every leaf's chain.
	LeafKeys map[string]Category
	// RecordedLeafKeys is what internal/emit writes under the column's
	// `leaf_keys:` (T-0404), sorted: the keys a run from the written file may
	// copy. For a column the committed yml does not carry, the spelling of
	// every key LeafMap called CatNone once every classify pass had run; for
	// one whose entry lists keys, that list as it stands, so a key the run
	// masked as drift (Classification.LeafDrift) is not added and joins the
	// list only by an operator's edit; for one whose entry has no
	// `leaf_keys:` at all (a file from before T-0404), every CatNone key,
	// drift included, once. internal/classify sets it and owns the spelling:
	// the key itself when it has an identifier's shape (an ASCII letter or
	// underscore, then ASCII letters, underscores and hyphens, digits only as
	// a trailing run of one or two, not eight or more letters all
	// hexadecimal, at most 32 bytes) and its column holds no more than 64
	// keys, and "sha256:" plus the first 16 hex characters of its SHA-256
	// otherwise, because a key that is not a name may be a value and the yml
	// holds none (THREAT_MODEL.md T5). nil when LeafMap is nil; empty and
	// non-nil for a per-leaf document none of whose keys is copied.
	RecordedLeafKeys []string
	// NameHit is the category rules.yml's name rules gave this column's own
	// name when the column's type is not one that category accepts, so the
	// name did not decide the category (internal/classify's decide, the
	// `hasName && !nameAccepted` branch; T-0393). It is empty when no rule
	// matched the name or the rule's category accepted the type.
	//
	// A jsonb `full_name`, `home_address`, `passwords`, `emails` or `notes`
	// is such a column: only special_category and semi_structured accept a
	// document, so the name loses to the type and the decision is the plain
	// semi_structured one a column with no name at all gets. Before T-0393
	// that decision carried the per-leaf map and every leaf with no signal of
	// its own was copied (the 2026-09-25 JSON red team, round 1, A11 to A13).
	// This field is what lets LeafMap and LeafNameCategory tell the two
	// apart: a document whose own name marks it personal has every leaf
	// masked, under the name's category.
	//
	// It is in memory only, like LeafKeys: internal/emit writes named fields
	// of a Decision and not this one, and every run re-derives it.
	NameHit Category
	// LogShaped reports that this column's table matched rules.yml's single
	// log_shaped rule (the rule pack's own regex over the normalised --
	// case-split, lower-cased -- table name: an underscore-bounded
	// audit(s)/log(s)/history/histories/event(s)/activity(ies)/trace(s)
	// word), independent of the column's own family or decision (T-0398, the
	// 2026-09-25 JSON red team's round 1, entry 26). ARCHITECTURE.md §4:
	// such a table's json/jsonb document is replaced whole with {}, not
	// walked leaf by leaf.
	//
	// Before this field, internal/transform's maskDocument answered the same
	// question with its own copy, logTableWords -- a fixed eight-word list
	// with no activity or trace, matched by splitting the table name on '_'
	// alone, with no CamelCase normalisation. The reasons line already named
	// the rule pack's own regex ("jsonb in a log-shaped table: the document
	// is replaced whole", appendContext), so a CamelCase table (Prisma's
	// default "AuditLog") or an *_activity or *_trace table was reported
	// replaced whole and walked leaf by leaf instead, copying every leaf
	// with no signal of its own. Before T-0272 that mismatch cost nothing --
	// every walked leaf was masked regardless -- and T-0272's per-leaf rule
	// is what turned it into a leak.
	//
	// internal/transform's maskDocument and internal/verify's own
	// restatement of the per-leaf rule (jsonleaf.go) both read this field
	// now, through their own leafPolicy/policyOf, instead of keeping a
	// second copy of the rule. It is in memory only, like NameHit:
	// internal/emit writes named fields of a Decision and not this one, and
	// every run re-derives it from the same rule pack the reason line reads.
	LogShaped bool
}

// LeafNameCategory is the category every leaf of a document column is masked
// under because the column's own name marked it personal (T-0393): NameHit,
// when the column's decision is the classifier's plain semi_structured verdict
// and NameHit names a category; empty otherwise. internal/transform and
// internal/verify hand it to their leafMasker, so a leaf takes that category's
// own masker where it has one a leaf can use and free_text where it has not.
// A special_category name (`medical_history`) is not this case -- that
// category accepts every type, so its decision is special_category and its
// leaves are free_text through LeafMap's nil -- and neither is a document an
// operator raised (ByYmlRaise), which keeps free_text for every leaf.
func (d Decision) LeafNameCategory() Category {
	if d.Category != CatSemiStruct || d.Source != ByClassifier {
		return ""
	}
	if d.NameHit == "" || d.NameHit == CatNone || d.NameHit == CatSemiStruct {
		return ""
	}
	return d.NameHit
}

// LeafMap is LeafKeys as internal/transform and internal/verify read it
// (T-0272 review round, finding 1): the map only when the column's own
// decision is the classifier's semi_structured verdict, the one a column gets
// for being a document and nothing more (internal/classify's typeSignals), and
// nil otherwise, which masks every leaf. A column whose own name the rule pack
// scores with a personal category (`medical_history`, `diagnosis`: a
// special_category column is masked on its name alone, THREAT_MODEL.md T1)
// holds that category in every leaf, whatever the leaf's own key says; and a
// yml raise, a committed `mask:` block or --mask (ByYmlRaise) is an operator
// saying the document is personal, which per-leaf copying would quietly undo.
// Both come back nil, so every leaf of such a column is masked as it was
// before per-leaf categories existed.
//
// So does a document whose own name matched a personal rule that does not
// accept its type (T-0393: a jsonb `full_name`, `passwords` or `notes`):
// its decision is plain semi_structured because the type decided it, but
// the name still says what every leaf holds, and LeafNameCategory carries
// that category instead of a map.
func (d Decision) LeafMap() map[string]Category {
	if d.Category != CatSemiStruct || d.Source != ByClassifier || d.LeafNameCategory() != "" {
		return nil
	}
	return d.LeafKeys
}

// LeafDrift is one document key a re-run's samples showed that the committed
// lazyslice.yml does not list under the column's `leaf_keys:` (T-0404). The
// classifier takes the key out of the decision's LeafKeys, so every leaf
// beneath it is masked as a key the samples never showed is; internal/core
// reports it as classify.column.drift and --strict-schema turns it into exit
// 10, as it does a column the file has never seen.
type LeafDrift struct {
	Col ColumnRef
	// Key is spelled as RecordedLeafKeys spells it: never the raw key when
	// that is not identifier-shaped.
	Key string
}

// Classification is every decision for one run.
type Classification struct {
	Decisions map[ColumnRef]Decision
	Drift     []ColumnRef // columns the committed yml had never seen
	// LeafDrift lists, sorted by column and then key, the document keys this
	// run would have copied that the committed yml's entry for the column
	// does not list (T-0404). Only a column the yml does carry an entry for
	// is asked: a column it has never seen is in Drift and classified fresh.
	LeafDrift []LeafDrift
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
	// PhoneRegion is the libphonenumber region internal/classify ran under
	// (Config.PhoneRegion: --phone-region, else the yml's phone_region; empty
	// for international-only). internal/transform reads it when it decides
	// whether a JSON leaf's value is a phone number, so a national-format
	// number the second net would read under the same region is masked and
	// never copied (T-0272 review round, finding 2).
	PhoneRegion string
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
