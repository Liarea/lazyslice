// SPDX-License-Identifier: Apache-2.0

// Package classify decides what every column holds: names, types, validated
// samples, dictionaries, and a reason string for each decision
// (ARCHITECTURE.md section 4).
//
// It is pure. It never issues SQL: samples arrive through pipeline.Sampler,
// taken at introspect. That is what lets the whole classifier be tested against
// a fixture without a database, and it is why the rule pack can be changed with
// confidence.
//
// It is biased to recall. After the neighbouring-column rule and FK propagation
// have run, "possible" and above is masked; the failure mode is mask more, never
// less. There is no ML gate and no cloud model over values.
//
// The classifier is not pluggable, by design and not by omission (ADR-006): a
// pluggable classifier is a supported way to see less personal data. The rule
// pack is an embedded YAML file in this package and contributions to it are pull
// requests. lazyslice.yml may add a pattern or raise a confidence; it can never
// lower or remove one.
package classify

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
	"github.com/Liarea/lazyslice/internal/textsig"
	"github.com/Liarea/lazyslice/mask"
)

type classifier struct{}

// New returns the rule-pack classifier.
func New() pipeline.Classifier { return classifier{} }

var _ pipeline.Classifier = classifier{}

// work is one column's decision while the passes are still running, with the
// flags that decide which pass may touch it.
type work struct {
	d pipeline.Decision
	// frags are the reason fragments, in the order they were produced.
	frags []string
	// typeConflict records a name hit on a type the category does not accept.
	// ARCHITECTURE.md §4: such a decision stays at low, and the
	// neighbouring-column rule never raises it.
	typeConflict bool
	// neverMask is a generated column or a surrogate key: never masked, always
	// explained (ARCHITECTURE.md §4).
	neverMask bool
	// generated is the half of neverMask that no pass may lift. A generated
	// column is not copied at all — the target recomputes it — so there is
	// nothing for FK propagation to mask. The key exemption is the other half,
	// and it is lifted when the key it references turns out to be masked.
	generated bool
	// frameworkMetadata is the T-0314 half of neverMask, and — unlike
	// generated — a pass is allowed to lift it (T-0314 review round, finding
	// 2). pipeline.IsFrameworkMetadataTable matches a bare table name in any
	// schema, so the exemption's premise ("bookkeeping the framework itself
	// wrote and reads back, never end-user data") is not guaranteed the way
	// it is for a real generated column: an application table that happens to
	// share one of the list's names, and that carries a validated foreign key
	// into a masked parent, is exactly the shape THREAT_MODEL.md T1's recall
	// floor and T8's join-integrity requirement are about, and a real
	// migration table never carries a foreign key at all — the ordinary shape
	// dogfood session 1 found — so lifting the exemption in that case costs a
	// genuine framework table nothing. propagateKeys therefore treats this
	// exactly as it treats the ordinary key exemption: cleared when a
	// validated FK parent turns out to be masked. applyPrior lifts it too, on
	// an explicit lazyslice.yml pattern or column entry naming this column —
	// ADR-004 lets a committed file only tighten, and silently refusing an
	// operator's own "mask this" for a column real values are copied into
	// would be a loosening in disguise. Nothing here widens
	// pipeline.IsFrameworkMetadataTable's list or narrows it by schema; both
	// lifts are about a *specific column* an outside signal — a validated
	// edge, an explicit file entry — has spoken about.
	//
	// markNeverMasked only ever sets this field true for a column that also
	// clears pipeline.IsFrameworkMetadataColumn (T-0314 review round, finding
	// 2, second pass): the table-name match alone used to exempt every column
	// of the table, which reached Liquibase's DATABASECHANGELOG.AUTHOR and
	// Flyway's flyway_schema_history.INSTALLED_BY — a developer's or a
	// database role's identity, not bookkeeping the tool reads back — the same
	// way it reached the columns that actually are. A column that clears
	// IsFrameworkMetadataTable but not IsFrameworkMetadataColumn falls straight
	// through this function to the ordinary passes below, exactly as if the
	// table were not on the list at all.
	frameworkMetadata bool
	// frameworkMetadataFrag is the index in frags of the "framework metadata
	// table" exemption fragment, or -1, the same bookkeeping keyFrag does for
	// the surrogate-key fragment: lifting the exemption blanks it, because a
	// line that says both "copied whole, never masked" and "raised by
	// lazyslice.yml" describes two different columns.
	frameworkMetadataFrag int
	// keyFrag is the index in frags of the surrogate-key or FK-column exemption
	// fragment, or -1. Lifting the exemption blanks that fragment, because a
	// line that says both "preserved verbatim" and "propagated through foreign
	// key" describes two different columns.
	keyFrag int
	// noSignalFrag is the index in frags of decide()'s "nothing_recognised" or
	// "sub_threshold_signal" fragment, or -1. Both say a character column's
	// samples were looked at and (not) found wanting, before guessedPhoneHit's
	// own region guess is folded in (T-0221) or raised by guessedPhoneColumns
	// (T-0197 review finding 2): a column guessedPhoneColumns goes on to mask
	// as phone had its samples re-described by that pass's own "samples"
	// fragment, so the earlier claim that nothing was recognised, or only a
	// sub-threshold amount was, is blanked exactly as keyFrag's exemption line
	// is when a later pass changes the column's story -- otherwise the reason
	// line asserts both "nothing recognised in N samples" and "N/N samples
	// parse as phone numbers" about the same column in the same breath.
	noSignalFrag int
	// propagationRefused records that a masked parent's category was one this
	// column's type family refuses, so that the sweep writes that fragment once.
	propagationRefused bool
	// unmasked is a per-column opt-out that has not expired.
	unmasked bool
	// twoLetterCodes records that every sample was a two-letter code -- an ISO
	// country or language column. It is read by the unknown-column arm of the
	// neighbouring-column rule, which must not mask a lookup column of codes
	// whose whole domain is two characters.
	twoLetterCodes bool
	family         string
	// array is columnType.Array: family is the *element* family for an array
	// column, so the two questions "is this a uuid" and "is this a uuid[]" need
	// both fields. The key exemption is scoped to a scalar key (markNeverMasked),
	// and keyChildren has to ask the same question of the same column later.
	array  bool
	table  ref.TableRef
	column pipeline.Column
	// guessedPhone is set in base(), whatever --phone-region says, when the
	// column's values clear validatorThreshold under some region in
	// phoneGuessRegions but decided nothing else -- see guessedPhoneHit and
	// guessedPhoneColumns (T-0221, widened by its own review round: a
	// configured region is the one region trusted without corroboration,
	// and this fallback still covers every other one). It is never acted on
	// without corroboration.
	guessedPhone *valueSignal
	// spare is what the samples of a signal-less character column say it is
	// -- one identifier shape throughout, or an enumeration -- set in base()
	// and read only by unknownColumnsBesideCertain, which spares such a column
	// from its sweep into free_text and prints why (T-0311; spare.go).
	spare spareShape
	// sweptNoSignal is set by unknownColumnsBesideCertain, on the column it
	// raises and on every fkPairs partner it raises alongside it (T-0312,
	// dogfood session 1): the whole claim that pass makes about this column
	// is "a certain neighbour sits beside it and nothing is known about its
	// own contents" -- no name rule, no value validator, no type signal, not
	// even the sub-threshold kind sameColumnName's own silenced-type-conflict
	// gate already excludes. sameColumnName's masked map is evidence for a
	// same-named column in another table entirely, so a decision with this
	// bit set must never enter it: the neighbour that justified raising this
	// column has nothing to say about a column in a table it is not even
	// beside, and letting the raise re-export as if it were an ordinary name
	// or value hit is exactly how one guess became the "25 propagations"
	// dogfood session 1 counted. It is never read outside sameColumnName; no
	// other pass treats a decision differently for carrying it.
	sweptNoSignal bool
	// nameUncorroborated is set by decide on a column the rule pack's
	// bare_name rule matched and nothing corroborated as a person's name
	// (T-0313, bareNameVerdict): no word for people in its table or column
	// name, and enough samples to say the name dictionary does not carry
	// them. Such a column sits at low, and sameColumnName must not raise it
	// from a same-named column in another table: that `users.name` holds
	// people is no evidence about `tags.name`, and exporting it was the rest
	// of dogfood session 1's 38 masked `name` columns. The neighbouring-column
	// rule still raises it, because a likely personal column beside it is
	// evidence about its own table.
	nameUncorroborated bool
}

// state is one Classify call.
type state struct {
	schema  *pipeline.Schema
	sampler pipeline.Sampler
	pack    *compiledPack
	order   []ref.ColumnRef
	dec     map[ref.ColumnRef]*work
	// unique is every column under a unique index, for Decision.UniqueIndex.
	unique map[ref.ColumnRef]bool
	// pkOrUnique is every column that can be the referenced side of an edge,
	// which is the condition ARCHITECTURE.md §4 puts on FK propagation.
	pkOrUnique map[ref.ColumnRef]bool
	// fkPartners maps every column at either end of a validated, non-virtual
	// foreign key edge to the column(s) at its other end: read by
	// unknownColumnsBesideCertain's own join-safety pairing, below (T-0253).
	// A column with no entry here is not part of any such edge.
	fkPartners map[ref.ColumnRef][]ref.ColumnRef
	// fkParents is fkPartners' directional half: a validated, non-virtual
	// foreign key's child column maps to the parent column(s) it references,
	// and a parent has no entry here. fkPairs' upward walk (T-0253's second
	// review round) reads this to find the parent of a column it has just
	// raised, a direction `propagateKeys` (below) never covers — it only ever
	// pushes a decision from a masked parent down to its children.
	fkParents map[ref.ColumnRef][]ref.ColumnRef
	// region is --phone-region / the yml's phone_region, or "" when neither
	// is set. It is resolved once, here, and read by buildValidators (which
	// this state's own validators list is built from) and by decide (to name
	// the region in the reason). base's own guessed-region pass runs whatever
	// this holds (T-0221's own review round, finding 2): a configured region
	// is the one region trusted without corroboration, not a reason to skip
	// the corroboration-gated fallback for every other one.
	region string
	// validators is this call's ordered validator list: baseValidators, with
	// a region-aware phone entry spliced in right after the international-
	// only one when region is set (buildValidators). It replaces the package
	// var every prior version of this file read directly, because the phone
	// entry's ok func now closes over a per-call value.
	validators []validatorEntry
}

// Classify decides every column of every table. It is pure: it issues no SQL,
// reads no file and consults no clock, so two runs over one schema and one
// sample set produce byte-identical decisions.
func (classifier) Classify(schema *pipeline.Schema, s pipeline.Sampler, prior *pipeline.Config) (*pipeline.Classification, error) {
	if schema == nil {
		return nil, fmt.Errorf("classify: no schema")
	}
	p, err := pack()
	if err != nil {
		return nil, err
	}
	region := ""
	if prior != nil {
		region = prior.PhoneRegion
	}
	st := &state{
		schema:     schema,
		sampler:    s,
		pack:       p,
		dec:        map[ref.ColumnRef]*work{},
		unique:     map[ref.ColumnRef]bool{},
		pkOrUnique: map[ref.ColumnRef]bool{},
		fkPartners: map[ref.ColumnRef][]ref.ColumnRef{},
		fkParents:  map[ref.ColumnRef][]ref.ColumnRef{},
		region:     region,
		validators: buildValidators(region),
	}
	st.indexKeys()
	st.indexFKColumns()
	st.base()
	st.byteaInPersonShapedTable()
	// T-0221's own pass (guessedPhoneColumns) runs inside neighbouringColumns,
	// after Decision.TableHasLikelyPersonalColumn is filled and before its
	// unknownColumnsBesideCertain arm, which would otherwise sweep the same
	// character column into free_text first -- see neighbouringColumns' own
	// comment.
	st.neighbouringColumns()
	st.keyChildren()
	st.foreignKeys()
	st.sameColumnName()
	cls, err := st.applyPrior(prior)
	if err != nil {
		return nil, err
	}
	st.finalise()
	cls.Decisions = make(map[ref.ColumnRef]pipeline.Decision, len(st.dec))
	for col, w := range st.dec {
		cls.Decisions[col] = w.d
	}
	cls.Fingerprint = fingerprintOf(st.pack.Version, cls.Decisions)
	return cls, nil
}

// indexKeys records which columns are under a unique index and which can be the
// referenced side of a foreign key.
//
// The two are different questions and the sets are built by different rules.
//
// pkOrUnique is "may this column be the referenced side of an edge", which is
// what ARCHITECTURE.md §4's foreign-key propagation asks. Postgres will accept a
// reference to any column list a non-partial, non-expression unique constraint
// covers, so every member column of a composite one is in it.
//
// unique is Decision.UniqueIndex, and §5 asks something narrower: whether *this
// column on its own* has to hold a distinct value in every row, because that is
// the column whose masker has to emit d_required = n²/2ε distinct values. One
// column of `CREATE UNIQUE INDEX ... (record_type, record_id, name, blob_id)` is
// not that column — the other three are what make the row distinct — and §5 says
// nothing about the composite case at all. Marking all four raised
// Decision.UniqueIndex on ActiveStorage's `name` (the literal 'cover'), on
// GitLab's `events.target_type` and on three more of the ten schemas in
// testdata/torture/, and the moment internal/plan's checkUniqueDomain started
// reading the field those runs refused at exit 12 over columns that cannot
// collide (testdata/regressions/003-composite-unique-index-is-not-a-unique-column.sql).
//
// So a column is in `unique` when it is a single-column primary key, or the only
// key column of a non-partial unique index, or the only column an expression
// unique index is over — §5 names `lower(email)` as exactly the case that drives
// the generator choice. That is the predicate internal/transform's uniqueColumn
// already applied to the schema, which internal/transform/CLAUDE.md says the two
// have to agree on; this is the agreement, and it adds the expression case that
// uniqueColumn drops.
//
// A **partial** single-column unique index is in, which is where this parts
// company with internal/transform's uniqueColumn. A partial index says something
// weaker than a total one — every row *the predicate admits* holds a distinct
// value, not every row of the table — but it still says something, and Supabase
// is the case that proves it has to be honoured: `auth.users` carries
// `CREATE UNIQUE INDEX confirmation_token_idx ON auth.users (confirmation_token)
// WHERE confirmation_token::text !~ '^[0-9 ]*$'`, the column classifies as
// credential, and `$lazyslice$invalid` is inside the predicate for every row, so
// the load put the rows in and the index would not build over them
// (testdata/regressions/007-partial-unique-index-masked-column.sql).
//
// d_required is then computed over the whole table's row count, which
// over-estimates: the predicate admits at most that many rows and usually far
// fewer. Over-estimating refuses a plan that might have loaded, and the refusal
// prints three escapes; under-estimating is a 23505 one statement after every
// row has moved, and prints none. **ADR-011 clause (b) is the accepted decision
// for this rule** (2026-09-08, from T-0099's text), and ARCHITECTURE.md §5
// states it: the over-estimate is the rule, not a placeholder, and its reversal
// condition is introspect collecting the statistic that would make it exact.
func (st *state) indexKeys() {
	for _, t := range st.schema.Tables {
		for _, name := range t.PK {
			st.pkOrUnique[ref.ColumnRef{Table: t.Ref, Column: name}] = true
		}
		if len(t.PK) == 1 {
			st.unique[ref.ColumnRef{Table: t.Ref, Column: t.PK[0]}] = true
		}
		for _, idx := range t.Indexes {
			if !idx.Unique {
				continue
			}
			cols := idx.Columns
			if idx.Expression {
				// introspect leaves Index.Columns empty for an expression
				// index, so the columns are recovered from the definition here.
				cols = expressionColumns(t, idx)
			}
			if len(cols) == 1 {
				st.unique[ref.ColumnRef{Table: t.Ref, Column: cols[0]}] = true
			}
			// An expression index is never the referenced side of a foreign
			// key, and neither is a partial one.
			if idx.Partial || idx.Expression {
				continue
			}
			for _, name := range cols {
				st.pkOrUnique[ref.ColumnRef{Table: t.Ref, Column: name}] = true
			}
		}
	}
}

// indexFKColumns pairs every column at either end of a validated, non-virtual
// foreign key edge with the column at its other end -- the ones
// `internal/load` recreates and enforces after every row has been committed
// (NOT VALID, then VALIDATE), so a decision that leaves the two ends
// disagreeing is not a masking gap but a half-loaded target at load time
// (`reconcileKeyChildren`'s own comment makes the identical argument for the
// integer/uuid key case). It is read by unknownColumnsBesideCertain's own
// join-safety pairing, below (T-0253): a rail with no evidence about a
// column of its own must not be what puts the two ends of a join out of
// step, and this package has no other pass that would catch it doing so --
// `keyChildren` only reconciles a key-family (integer/bigint/uuid) child
// (`isKeyFamily`), and `propagateKeys` only ever propagates a masked
// **parent**'s decision forward, never a child's back to an unmasked parent,
// which is exactly the direction a rail that fires on the child end needs.
//
// An unvalidated constraint is a hint over rows Postgres never checked and a
// virtual one is a line in the yml, so a child of either can already hold a
// value its "parent" does not -- the same distinction `reconcileKeyChildren`
// draws for the same reason.
//
// The pairing is position-matched across a composite key, the same way
// `propagateKeys` walks one: `fkPartners[parent[i]]` holds `child[i]` and
// `fkPartners[child[i]]` holds `parent[i]`, never the whole column list
// against itself. `fkParents[child[i]]` holds the same `parent[i]` alone —
// the directional half `fkPairs`' upward walk needs, below.
func (st *state) indexFKColumns() {
	for _, fk := range st.schema.FKs {
		if !fk.Validated || fk.Virtual {
			continue
		}
		for i, childName := range fk.ChildCols {
			if i >= len(fk.ParentCols) {
				break
			}
			parent := ref.ColumnRef{Table: fk.Parent, Column: fk.ParentCols[i]}
			child := ref.ColumnRef{Table: fk.Child, Column: childName}
			st.fkPartners[parent] = append(st.fkPartners[parent], child)
			st.fkPartners[child] = append(st.fkPartners[child], parent)
			st.fkParents[child] = append(st.fkParents[child], parent)
		}
	}
}

// identRE is every bare or quoted identifier in an index expression.
var identRE = regexp.MustCompile(`"(?:[^"]|"")*"|[A-Za-z_][A-Za-z0-9_$]*`)

// indexKeyList cuts the parenthesised key list out of a pg_get_indexdef line —
// "CREATE UNIQUE INDEX x ON s.t USING btree (lower((email)::text)) WHERE ..." —
// by counting brackets from the one that opens it, so that a partial index's
// WHERE clause is not read as part of the key.
func indexKeyList(def string) string {
	i := strings.Index(def, " USING ")
	if i < 0 {
		return ""
	}
	open := strings.IndexByte(def[i:], '(')
	if open < 0 {
		return ""
	}
	open += i
	depth := 0
	for j := open; j < len(def); j++ {
		switch def[j] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return def[open+1 : j]
			}
		}
	}
	return ""
}

// expressionColumns is the columns an expression index covers, read out of its
// definition by matching identifiers against the table's own column names.
//
// It over-reports rather than under-reports: a function whose name happens to
// equal a column name marks that column unique, which makes the planner pick a
// wider generator for it. The other direction would be a load that fails on a
// unique violation, which is the failure ARCHITECTURE.md §5's domain check
// exists to prevent.
func expressionColumns(t pipeline.Table, idx pipeline.Index) []string {
	keys := indexKeyList(idx.Def)
	if keys == "" {
		return nil
	}
	seen := map[string]bool{}
	var out []string
	for _, tok := range identRE.FindAllString(keys, -1) {
		if strings.HasPrefix(tok, `"`) {
			tok = strings.ReplaceAll(strings.Trim(tok, `"`), `""`, `"`)
		}
		for _, col := range t.Columns {
			if col.Name == tok && !seen[tok] {
				seen[tok] = true
				out = append(out, col.Name)
			}
		}
	}
	return out
}

// ---------- pass 1: name, type and value signals ----------

// valueSignal is what the validators said about one column's samples.
type valueSignal struct {
	cat     pipeline.Category
	phrase  string
	matched int
	total   int
	// digitsOnly is true when every sample that matched is plain 0-9 after
	// trimming (T-0297: a 12-digit phone number is a bare-hex MAC to
	// net.ParseMAC). Set only by bestSignal's validator loop.
	digitsOnly bool
}

// validatorEntry is one row of the ordered validator list. It used to be an
// anonymous struct literal (baseValidators' own element type still is one, by
// composite literal); it is named here only so that bestSignal, byteaTextSignal
// and compositeSignal can take a built list as a parameter (buildValidators,
// T-0221) instead of reading the package var they used to share.
type validatorEntry = struct {
	cat    pipeline.Category
	phrase string
	// strong marks the validators internal/verify's second net also marks
	// strong: a precise parse rather than a shape guess. It is read only by
	// bestSignal's strongHit tracking below (docs/reviews/2026-09-09/
	// REVIEW.md finding 7) -- it does not change which validator decides a
	// column outright, only what happens when none of them reaches
	// validatorThreshold. See internal/verify/validators.go's own `strong`
	// field, which this mirrors validator for validator; keep the two in
	// step. IBAN is the one exception among the six parse-shaped categories:
	// it is a checksum over letters and digits rather than over a run of
	// digits, so an ordinary all-caps string is about as likely to pass its
	// mod-97 check as any other string of the right length is -- five of
	// pagila's own film titles do. Luhn does not share that problem, because
	// nothing in ordinary text is a run of digits, so it stays strong.
	strong bool
	ok     func(*textsig.Dict, string) bool
}

// baseValidators is the ordered list ARCHITECTURE.md §4 names, before
// buildValidators splices in the region-aware phone entry T-0221 adds when a
// region is configured. Order is precedence: the first one that reaches the
// threshold decides, so an address that parses as an email is an email and a
// note that mentions a street is prose.
var baseValidators = []validatorEntry{
	{pipeline.CatEmail, phraseAddresses, true, func(_ *textsig.Dict, s string) bool { return textsig.ValidEmail(s) }},
	// national_id joined the list at T-0187 (the 2026-09-15 round-2 red team,
	// R2-01/A2, R2-02/A6, R2-03/A7): textsig.ValidNationalID was correct and
	// recognised every attack value, but nothing on this list ever called it,
	// so a plain SSN or NI number in a column whose name missed rules.yml's
	// pattern crossed into the target verbatim, reported as "no name or value
	// signal".
	//
	// **It was one entry at `strong`, calling the twelve-format union, and
	// the T-0187 review round's finding 2 is why it is two now — but this
	// split closes only part of that finding, and the CLAUDE.md note beside
	// this list says which part.** Six of the twelve (PESEL, BSN, SIN, TFN,
	// Aadhaar's Verhoeff check and CPF) are a mod-N sum over an otherwise
	// unconstrained digit run and clear a meaningful fraction of a random
	// string of the right length regardless of what it means (measured:
	// 25.7% for a random 9-digit string, 11.0% for 11-digit) —
	// internal/verify/validators.go's own comment has the full table.
	// Marking the whole union `strong` meant `bestSignal`'s minority-hit
	// tracking (`strongHit`, the T-0136 note above) masked an unrelated
	// proven column outright — a business key, a reference-code column — as
	// `free_text` on a *single* coincidental checksum hit anywhere in the
	// sample, `ConfPossible`, with no neighbouring column needed at all
	// (`decide`'s `case sig.strongHit != nil`, below). That is the exposure
	// this split closes: the checksum-only six can no longer set
	// `sig.strongHit`, only `sig.weak` at the ordinary ratio, so a lone hit
	// no longer masks anything by itself. The other six (a US SSN, a UK
	// NINO, an Italian codice fiscale, a Spanish DNI or NIE, a French NIR)
	// also constrain the value's *shape* — a dash, a letter, or a fixed
	// length under its own mod-97 check — so none of them matches a bare
	// digit run at all, and `textsig.ValidNationalIDStructured` is precise
	// enough to stay on email's own `strong` footing.
	//
	// **What this split does not close, and why that is tracker T-0195 and
	// not a second bug here.** `sig.weak` (`bestSignal`, below) is set by
	// ratio alone — `proven && ratio >= weakThreshold` — with no read of
	// `v.strong` at all, so a business key whose values clear one of the six
	// checksum-only formats at or above `weakThreshold` (0.5) still records
	// `national_id` at `low`, and the neighbouring-column rule can still
	// raise that to `possible`/masked beside a `likely` personal column in
	// the same table — finding 2's own "consequence (1)". Closing that needs
	// either a per-validator type-family gate this package does not have
	// (the way `rules.yml`'s `accepts:` gates a *name* hit, not a value hit)
	// or a materially higher within-column threshold scoped to this one
	// entry, and either is a recall-affecting scoring change that
	// internal/classify/CLAUDE.md's own rule says needs a T1 review and a
	// `TestPagilaPrecisionAndRecall` measurement, not a quiet edit bundled
	// into this task — T-0195 already carries it, filed at the same
	// specificity as this paragraph. This also mirrors
	// internal/verify/validators.go's own three-way national_id split
	// (T-0187 review round, finding 2) on the row-scanning side only in
	// part: this package still has no `text`/`digits` split the way that
	// file does (internal/textsig/CLAUDE.md's own note on the point), which
	// is why an SSN stored as `bigint` with no name hit is still `none` here
	// and is what internal/verify's digits-family entry exists to catch
	// instead. **Owed:** the `text`/`digits` split and the type-family gate
	// T-0195 describes.
	{pipeline.CatNationalID, phraseNationalID, true, func(_ *textsig.Dict, s string) bool { return textsig.ValidNationalIDStructured(s) }},
	{pipeline.CatNationalID, phraseNationalID, false, func(_ *textsig.Dict, s string) bool { return textsig.ValidNationalIDChecksumOnly(s) }},
	{pipeline.CatFinancial, phraseIBAN, false, func(_ *textsig.Dict, s string) bool { return textsig.ValidIBAN(s) }},
	// The card entry asks for a known issuer prefix as well as the Luhn check
	// digit (T-0316): a bare check digit is one digit run in ten, and dogfood
	// session 1 masked eight identifier columns on it. A column whose name
	// says it holds an identifier asks textsig.CardShape instead, in base()
	// (identifierNamed, validators.go).
	{pipeline.CatFinancial, phraseLuhn, true, func(_ *textsig.Dict, s string) bool { return textsig.ValidCard(s) }},
	// ValidPhone parses under textsig.PhoneRegionHint ("ZZ"), which only ever
	// admits an already-international number -- a national-format column
	// (07911 123456, 020 7946 0958) scores zero here whatever the ratio
	// (T-0221, the 2026-09-15 round-3 red team's kontaktnr/contact finding:
	// docs/reviews/2026-09-15-redteam/round3-still-leaking.json). It stays
	// exactly this narrow on purpose: buildValidators (below) is what adds
	// the region-aware companion entry, and only when a region is configured.
	{pipeline.CatPhone, phraseE164, true, func(_ *textsig.Dict, s string) bool { return textsig.ValidPhone(s) }},
	{pipeline.CatNetworkID, phraseIP, true, func(_ *textsig.Dict, s string) bool { return textsig.ValidIP(s) }},
	{pipeline.CatNetworkID, phraseMAC, true, func(_ *textsig.Dict, s string) bool { return textsig.ValidMAC(s) }},
	// Ahead of the secrets one, and that order is the whole of tracker T-0100:
	// a URL clears every guard in textsig.LooksSecret, so mastodon's
	// accounts.uri was `credential` on every row -- masked to the fixed
	// literal, and refused at plan under the unique index it carries. A URL
	// that names a person is an online_id, whose generator has a domain large
	// enough for a unique column.
	{pipeline.CatOnlineID, phraseURL, true, func(_ *textsig.Dict, s string) bool { return textsig.ValidURL(s) }},
	// credential and address are deliberately not strong: LooksSecret is an
	// entropy guess and AddressShape is a mixed-digits-and-words guess,
	// neither a parse, so one occurrence of either is not the same claim as
	// one occurrence of a valid email address.
	{pipeline.CatCredential, phraseSecrets, false, func(_ *textsig.Dict, s string) bool { return textsig.LooksSecret(s) }},
	{pipeline.CatPersonName, phraseNameDict, false, func(d *textsig.Dict, s string) bool { return d.LooksLikeName(s) }},
	{pipeline.CatAddress, phraseAddrShape, false, func(_ *textsig.Dict, s string) bool { return textsig.AddressShape(s) }},
	{pipeline.CatFreeText, phraseProse, false, func(d *textsig.Dict, s string) bool { return d.Prose(s) }},
}

// buildValidators returns this call's ordered validator list (T-0221): a copy
// of baseValidators with a second phone entry spliced in immediately after the
// international-only one, when and only when region is non-empty.
//
// It has to be built per call and not once at init, because the entry's ok
// func closes over region -- the operator's --phone-region flag or the
// committed yml's phone_region, resolved once in Classify. Splicing it in
// right after the existing phone entry, rather than appending it, is what
// keeps the two on one footing in the precedence order §4 states: neither
// moves ahead of national_id/financial (which come first) or behind
// network_id/online_id/credential/person_name/address/free_text (which come
// after), so an address-shaped or IBAN-shaped value that also happens to
// parse under the configured region is still decided by the earlier entry,
// exactly as the two phone entries decide before address and free_text do.
//
// A region an operator configured is trusted evidence on the same footing as
// the international-only entry: parsing under a specific libphonenumber
// region is as precise a claim as parsing under none at all, so this entry is
// `strong` too and is decided through the same generic ratio loop bestSignal
// already runs -- no corroboration gate. With no region configured, the
// national-format path is guessedPhoneHit/guessedPhoneColumns instead, a
// separate corroboration-gated pass and never a validators-list entry: an
// ordinary ratio entry here would mask a column outright the moment a guessed
// region's numbering plan happened to fit, which is exactly the false
// positive a ten-digit account or order-number column risks (see
// guessedPhoneColumns's own comment).
func buildValidators(region string) []validatorEntry {
	if region == "" {
		return baseValidators
	}
	out := make([]validatorEntry, 0, len(baseValidators)+1)
	for _, v := range baseValidators {
		out = append(out, v)
		if v.cat == pipeline.CatPhone {
			out = append(out, validatorEntry{
				cat: pipeline.CatPhone, phrase: phraseE164Region, strong: true,
				ok: func(_ *textsig.Dict, s string) bool { return textsig.ValidPhoneRegion(s, region) },
			})
		}
	}
	return out
}

// phoneGuessRegions is the short, fixed list of libphonenumber regions
// guessedPhoneHit tries when no --phone-region / phone_region is configured
// (T-0221). It is short on purpose and not phonenumbers.GetSupportedRegions()'s
// full two hundred and forty-odd: every region added here is one more chance
// an ordinary ten-digit account number or order code clears somebody's
// numbering plan by accident, which is the false positive guessedPhoneColumns'
// corroboration gate exists to catch rather than avoid by emptying the list.
// The fifteen chosen are large calling-code populations spanning distinct
// numbering-plan shapes (length, trunk prefixes), which is what "look like a
// phone number to somebody's dial plan" can mean without an operator naming
// one.
var phoneGuessRegions = []string{
	"US", "CN", "IN", "ID", "BR", "PK", "NG", "GB", "DE", "FR", "JP", "MX", "RU", "ES", "IT",
}

// guessedPhoneHit reports the ratio of values that parse as a phone number
// under any one of phoneGuessRegions -- an OR across the list, because which
// region (if any) is right is exactly what nobody has said, and "some region
// somewhere accepts it" is the whole of the claim this makes. It answers nil
// below minSamples, for bestSignal's own reason: a coincidental hit in one or
// two values is not evidence about a column.
//
// Its result is never acted on directly (base, below): it decides nothing on
// its own, only offers a candidate to guessedPhoneColumns' corroboration
// gate.
func guessedPhoneHit(values []string) *valueSignal {
	if len(values) < minSamples {
		return nil
	}
	matched := 0
	for _, v := range values {
		if matchesAnyGuessRegion(v) {
			matched++
		}
	}
	if matched == 0 {
		return nil
	}
	return &valueSignal{cat: pipeline.CatPhone, phrase: phraseGuessedPhone, matched: matched, total: len(values)}
}

func matchesAnyGuessRegion(s string) bool {
	for _, region := range phoneGuessRegions {
		if textsig.ValidPhoneRegion(s, region) {
			return true
		}
	}
	return false
}

// byteaTextSignal is ARCHITECTURE.md §4's bytea rule extended to the case the
// 2026-09-15 red team's A4a found: a bytea column whose contents are printable
// UTF-8 carrying personal data, in a table with no `certain` column for
// byteaInPersonShapedTable to key on.
//
// It answers binary_personal or nothing. The category is fixed rather than
// taken from the validator that hit, because binary_personal is the one
// category rules.yml accepts on a bytea column and the only one whose masker
// can write there — everything else would be the T-0054 failure by a longer
// route.
//
// The gate is per value and not per column, and it is textsig.PrintableText:
// a sample is considered when it is valid UTF-8 and at least 95% printable
// runes, and the validators then run over the samples that are. A hit is either
// a *strong* validator (a precise parse: an address, a phone number, an IP, a
// MAC, a card, a URL) on any considered sample, or any validator at all
// reaching validatorThreshold across them.
//
// **It used to be per column as well** — at least 95% of the whole sample set
// had to be readable before any validator ran — and that made this signal and
// internal/verify's second net two different rules over the same bytes, which
// both files claimed they were not (the T-REDFIX review's second finding). A
// bytea column that is half printable documents and half images passed that
// gate nowhere: this package left it unmasked, and the second net — which asks
// textsig.PrintableText per *value* and has no column ratio — refused the
// loaded target at exit 9, with no green path, because semi_structured and
// binary_personal cannot be reached for such a column by any other route.
// --unmask on a column that really does hold documents was the only way
// through, which is the refusal-an-operator-routes-around outcome this tree
// argues against everywhere else. So the column ratio is gone and the two sides
// ask one question per value. What it costs is the case the ratio was written
// for — a column of images with one readable blob in it that a validator hits
// is now masked as binary_personal rather than left alone — and that is the
// direction it has to fail in: the same column is exit 9 in internal/verify
// today, so masking it is strictly the kinder of the two answers, and
// CLAUDE.md's "when in doubt, mask it" is the rule that decides it.
func byteaTextSignal(dict *textsig.Dict, values []string, p *compiledPack, vs []validatorEntry) *valueSignal {
	if len(values) == 0 {
		return nil
	}
	readable := make([]string, 0, len(values))
	for _, v := range values {
		if textsig.PrintableText(v) {
			readable = append(readable, v)
		}
	}
	if len(readable) == 0 {
		return nil
	}
	for _, v := range vs {
		if silencedByType(p, v.cat, famText) {
			continue
		}
		matched := 0
		for _, s := range readable {
			if v.ok(dict, s) {
				matched++
			}
		}
		if matched == 0 {
			continue
		}
		if v.strong || float64(matched)/float64(len(readable)) >= validatorThreshold {
			return &valueSignal{
				cat:     pipeline.CatBinary,
				phrase:  phraseByteaText,
				matched: matched,
				total:   len(values),
			}
		}
	}
	return nil
}

// signals is what the validators said about one column's samples.
//
// They are kept apart because ARCHITECTURE.md §4's accepted-types gate applies
// to a value signal exactly as it does to a name signal (T-0054; see
// internal/classify/CLAUDE.md). strong and weak are validators whose category
// the column's type family can hold. A validator whose category the family
// cannot hold is never run at all as a candidate decision (T-0269; see
// bestSignal's silencedByType branch): its category, phrase and sample count
// are never kept, because keeping them only for a reason line is exactly what
// T-0269 removed (a timestamp column truthfully and uselessly "looks like a
// secret"). silencedStrong is the one trace such a validator can still leave
// (T-0269 fix round) -- see its own comment below.
type signals struct {
	strong *valueSignal
	weak   *valueSignal
	// strongHit is the first *strong* validator (see the validators list
	// above) that matched at least one proven sample without reaching
	// validatorThreshold -- so neither strong nor weak was set from it.
	// decide() reads this as "mask this column as free_text rather than let a
	// minority strong hit through as `none`" (docs/reviews/2026-09-09/
	// REVIEW.md finding 7): a strong validator is a precise parse, so one
	// email address among nineteen ordinary strings is still one email
	// address, and a column ratio is the wrong question to ask about whether
	// it should reach the target unmasked. It is never set below minSamples --
	// a minority hit there is already covered by the strong branch above (a
	// ratio over one or two values is either both of them or none), and
	// internal/verify's own minValues floor already fails any hit on an
	// unproven column, so there is no gap at that size for this to close.
	strongHit *valueSignal
	// silencedStrong is true when some validator whose category this column's
	// type family cannot hold (silencedByType) matched at or above
	// validatorThreshold -- the shape the pre-T-0269 sig.refused field held,
	// with none of what made its reason an alarm: no category, no phrase, no
	// sample count. decide()'s own sig.silencedStrong case (T-0269 fix round,
	// a reviewer finding on T-0269) is the only reader, and it exists so that
	// a column this shape describes keeps sameColumnName, guessedPhoneColumns
	// and fkPairs off it exactly as sig.refused's typeConflict used to --
	// dropping the field entirely (T-0269's first landing) let those raising
	// passes reach a column no category was ever going to be decided under,
	// because nothing on the column recorded that a validator had even been
	// silenced. This field is not a decision and is read only by that one
	// case, which sets w.typeConflict and nothing else about the column's
	// category or reason.
	silencedStrong bool
	total          int
	// anyMatched is true the moment any validator matches at least one
	// sample, independent of minSamples, weakThreshold or silencedByType --
	// it is the only field in this struct answering "did anything recognise
	// anything at all", which decide()'s default branch needs to tell a
	// column nothing looked like anything (T-0197's "nothing recognised")
	// from a column something *did* match, just not enough of it, or too few
	// samples, to decide on (T-0197 review finding 1).
	anyMatched bool
}

// base gives every column its name, type and value decision.
func (st *state) base() {
	dict := textsig.Dictionary()
	for _, t := range st.schema.Tables {
		for _, col := range t.Columns {
			cref := ref.ColumnRef{Table: t.Ref, Column: col.Name}
			st.order = append(st.order, cref)
			ct := typeOf(st.schema, col)
			w := &work{
				d:                     pipeline.Decision{Col: cref, Category: pipeline.CatNone, Confidence: pipeline.ConfNone, Source: pipeline.ByClassifier},
				keyFrag:               -1,
				frameworkMetadataFrag: -1,
				noSignalFrag:          -1,
				family:                ct.Family,
				array:                 ct.Array,
				table:                 t.Ref,
				column:                col,
			}
			st.dec[cref] = w
			values := st.samples(cref, ct)
			vs := st.validators
			if entropyExemptNames[normaliseName(col.Name)] {
				// T-0315: a class or component name is not asked whether it
				// looks like a secret; see entropyExemptNames.
				vs = withoutSecrets(vs)
			}
			if identifierNamed(normaliseName(col.Name)) {
				// T-0316: an id, number, version or reference column needs a
				// value in the full shape of a card; see identifierNamed.
				vs = withCardShape(vs)
			}
			sig := bestSignal(dict, values, st.pack, ct.Family, vs)
			st.decide(w, col, ct, values, sig)
			st.appendContext(w, t, ct, sig.total)
			st.markNeverMasked(w, t, col, ct)
			// T-0221, widened by its own review round (finding 2): computed
			// whenever nothing above already decided the column, whatever
			// --phone-region says. The first landing skipped this block
			// entirely once st.region was non-empty, on the reasoning that
			// the configured region's entry in st.validators had already
			// had its say inside bestSignal, above -- true only for numbers
			// *in* that region: a column of US-format numbers under
			// --phone-region GB masked with no flag at all and was left
			// unmasked, "no name or value signal", the moment GB was named,
			// because a configured region replaced the fifteen-region
			// fallback instead of adding to it. A configured region is not
			// evidence about every *other* region a real multi-country
			// database also holds, so this pass now runs unconditionally:
			// the configured region is the one region trusted without
			// corroboration, and this pass is still the corroboration-gated
			// fallback for every other region. Running unconditionally also
			// closes the same review round's first finding's belt-and-
			// braces half -- an unusable --phone-region value (a typo, a
			// non-ISO spelling) can no longer disable this fallback,
			// because nothing here reads st.region at all any more; a bad
			// value is refused earlier, at the flag surface (cmd/lazyslice),
			// and even one that reached here regardless would leave this
			// pass exactly as active as no flag at all.
			//
			// Only on a character family. A digits-family column (bigint,
			// integer, numeric) is exactly the family internal/verify's own
			// national_id digits entry exists to scan unmasked, ratio-scored
			// and itself corroboration-gated (T-0187 third review round):
			// masking one here on a guessed-region coincidence would decide
			// it phone before that entry ever saw it, over a column whose
			// real shape may be a national identifier this package's own
			// (digits-family-blind) national_id entry cannot recognise --
			// testdata/regressions/020-ssn-stored-as-bigint.sql is exactly
			// that column, corroborated by the same neighbour signal for the
			// same coincidental reason its own header already explains for
			// national_id's checksum-only formats. Restricting the guess to
			// text/varchar/bpchar/citext leaves that column for verify's own
			// entry to decide, exactly as it does today.
			if w.d.Confidence < pipeline.ConfPossible &&
				isCharacterFamily(ct.Family) && !silencedByType(st.pack, pipeline.CatPhone, ct.Family) {
				if hit := guessedPhoneHit(values); hit != nil &&
					float64(hit.matched)/float64(hit.total) >= validatorThreshold {
					w.guessedPhone = hit
				}
			}
			// T-0311: only a column the sweep could reach is described --
			// character family, no decision of its own. Whether it is then
			// spared is unknownColumnsBesideCertain's call, which alone knows
			// whether the table has a certain neighbour at all.
			if isCharacterFamily(ct.Family) && w.d.Category == pipeline.CatNone && w.d.Confidence == pipeline.ConfNone {
				w.spare = sparedBy(dict, values)
			}
		}
	}
}

// samples flattens one column's samples into the strings the validators read.
// The column's type is part of the flattening: an array whose sample arrived as
// one string is read back as an array literal (tracker T-0103, scalarsOf).
func (st *state) samples(c ref.ColumnRef, ct columnType) []string {
	if st.sampler == nil {
		return nil
	}
	var out []string
	for _, v := range st.sampler.Samples(c) {
		out = append(out, scalarsOf(ct, v)...)
	}
	return out
}

// bestSignal runs the validators over the non-NULL samples and reports the
// first that reaches a threshold on a category the column's type family can
// hold, and the number of values considered. A validator whose category the
// family cannot hold never decides the column, strong or weak, and never
// leaves its category, phrase or sample count behind for a reason to name
// (T-0269) -- the one trace it can leave is sig.silencedStrong, a bare flag
// with none of that (T-0269 fix round; see signals.silencedStrong).
//
// The type gate is the whole of the difference from an earlier version of this
// function, and it is T-0054's fix: a validator whose category the family
// refuses can no longer decide the column, because the masker chosen from that
// category emits a value the column cannot hold. Two live examples, both on
// pagila: every `last_update timestamptz` renders as "2017-02-15T09:45:30Z",
// which has no space and no "@" and mixes two character classes, so looksSecret
// called it a credential on 100% of its rows -- and `film.fulltext tsvector`
// renders as "'academi':1 'battl':15", which carries digits and words, so
// addressShape called it an address. Both were masked, and internal/transform
// then could not write "$lazyslice$invalid" into a timestamp: exit 7, mid-run,
// on every whole-pipeline run over pagila.
//
// The gate is narrower than the rule pack's accepts: lists, because those lists
// answer a slightly different question. See silencedByType.
func bestSignal(dict *textsig.Dict, values []string, p *compiledPack, family string, vs []validatorEntry) signals {
	sig := signals{total: len(values)}
	if sig.total == 0 {
		return sig
	}
	if isJSONFamily(family) {
		sig.strong = jsonSignal(p, values)
		return sig
	}
	if family == famComposite {
		sig.strong = compositeSignal(dict, values, vs)
		return sig
	}
	if family == famBytea {
		// A bytea sample is arbitrary binary, and asText renders it as a Go
		// string: run through the text validators, a PNG reads as an address
		// and a compressed blob reads as a secret. ARCHITECTURE.md §4 gives
		// bytea its own rule -- "bytea in a person-shaped column is
		// binary_personal and set to NULL" -- and the rule pack's text
		// categories do not accept the family, so a *text category* decided
		// here could only produce a decision the pack itself contradicts.
		//
		// That argument is sound and it does not cover the case the
		// 2026-09-15 red team's A4a leaked through: a bytea holding printable
		// UTF-8. `assets.blob_doc` held "Grace Hopper
		// <grace.hopper1@realcorp.example> 078-05-1120" in a table with no
		// `certain` column, so byteaInPersonShapedTable could not fire either,
		// and the address crossed into the target verbatim under exit 0.
		//
		// So the guard is on the *content* and not on the family. A sample set
		// that is valid UTF-8 and overwhelmingly printable is text somebody put
		// in a bytea column, and the validators are asked about it; the
		// category the hit produces is **binary_personal**, never the hit's
		// own, because binary_personal is the one category the pack accepts on
		// this family and its masker (NULL) is already registered — no new
		// masker, no new writability question, and no route back to the
		// contradiction above. A PNG fails the printability guard and is
		// unaffected, which is what byteaTextSignal's own test pins.
		sig.strong = byteaTextSignal(dict, values, p, vs)
		return sig
	}
	if family == famTSVector {
		// A tsvector is decided by its type alone (decide), so no validator's
		// answer about it could change anything. Its text form
		// ("'academi':1 'battl':15") reads as an address to AddressShape,
		// which is the other half of the reason not to ask.
		return sig
	}
	// Below minSamples the column is *unproven*, not clean, and returning here
	// is a fail-open: a three-row table yields two non-NULL values, and
	// public.devices.owned_by in testdata/nasty.sql is two email addresses and
	// a NULL under a column name that says nothing. An early return decided it
	// `none`, internal/transform copied it, and the target held a production
	// email address in cleartext under exit 0 (THREAT_MODEL.md T1) — found by
	// TestI2NothingFlaggedSurvives/nasty, tracker T-0058.
	//
	// So the validators run over whatever there is, and only the *strong*
	// branch may decide: at two values a strong ratio is both of them, which is
	// the fail-closed reading of "when in doubt, mask it" (CLAUDE.md). The weak
	// branch stays silent below minSamples, because `low` is what the
	// neighbouring-column rule raises to `possible`, and one of two values
	// matching is the noise minSamples was written about — raising it would
	// mask a column on the strength of a single row.
	proven := sig.total >= minSamples
	namedFiles := namedFileNames(dict, values)
	for _, v := range vs {
		ok, phrase := v.ok, v.phrase
		if v.cat == pipeline.CatCredential && namedFiles {
			// T-0315: see namedFileNames. Every file name counts, whatever
			// its own stem carries, as every one did before LooksSecret
			// spared a file name.
			ok = func(d *textsig.Dict, s string) bool {
				_, file := textsig.FileNameStem(s)
				return file || v.ok(d, s)
			}
			phrase = phraseNamedFiles
		}
		matched := 0
		digitsOnly := true
		for _, s := range values {
			if ok(dict, s) {
				matched++
				digitsOnly = digitsOnly && allDigits(s)
			}
		}
		ratio := float64(matched) / float64(sig.total)
		if matched > 0 {
			sig.anyMatched = true
		}
		if silencedByType(p, v.cat, family) {
			// T-0269: a category whose accepted types this column's type is
			// not in decides nothing and is never named in a reason -- scoring
			// it and keeping the *result* only for the reason line (the
			// pre-T-0269 behaviour) is what produced "200/200 samples look
			// like secrets" on a timestamp column: technically true of the
			// entropy check and irrelevant, since credential was never an
			// option for that type.
			//
			// T-0269 fix round: dropping the result entirely, rather than just
			// its category and phrase, was itself a T1 regression -- a
			// silenced validator at or above validatorThreshold used to set
			// typeConflict (sig.refused, before T-0269) and keep
			// sameColumnName, guessedPhoneColumns and fkPairs off the column;
			// with nothing recorded at all, a same-named column elsewhere in
			// the schema, or a corroborating neighbour, could raise this one
			// on evidence that was never actually about it (see decide()'s own
			// sig.silencedStrong case). silencedStrong is the flag that keeps
			// those raising passes off, and it is deliberately bare: no
			// category, no phrase, no count, so decide() cannot render the
			// alarm T-0269 removed from it.
			if ratio >= validatorThreshold {
				sig.silencedStrong = true
			}
			continue
		}
		hit := &valueSignal{cat: v.cat, phrase: phrase, matched: matched, total: sig.total, digitsOnly: matched > 0 && digitsOnly}
		if ratio >= validatorThreshold && (v.cat != pipeline.CatCredential || sig.total >= minSecretSamples) {
			// T-0315: the entropy validator decides only over
			// minSecretSamples or more; below that it falls through to the
			// weak branch, and the validators after it are still asked.
			sig.strong = hit
			return sig
		}
		if proven && v.strong && matched > 0 && sig.strongHit == nil &&
			!silencedByType(p, pipeline.CatFreeText, family) {
			// finding 7: a strong validator below validatorThreshold is still
			// a precise parse over at least one sample, which is stronger
			// evidence than the weak branch below asks for and is scored
			// separately from it in decide() -- see signals.strongHit.
			//
			// decide() always assigns this hit's *column* the category
			// free_text (not the hit's own category), because the column is
			// only proven mixed, not reliably the hit's category -- so the
			// gate here has to ask whether free_text itself can be written
			// into this family, not whether the hit's own category can
			// (T-0136 review finding 1). A bigint column with one Luhn hit
			// passes silencedByType(p, financial_account, bigint) == false
			// (financial_account accepts bigint) but free_text accepts only
			// text/varchar/bpchar/citext (rules.yml), so deciding it here
			// produced Category=free_text, Masked=true on a family
			// mask.Writable refuses, and internal/plan/writeback.go refused
			// the whole run at exit 12. A column this excludes is not
			// silenced outright: it is left for sig.weak/none as
			// before T-0136, and internal/verify's second net (T-0136's
			// matching fix on validators.go) is what catches the loaded
			// value on the family this branch cannot reach.
			sig.strongHit = hit
		}
		if proven && sig.weak == nil && ratio >= weakThreshold {
			sig.weak = hit
		}
	}
	return sig
}

// silencedByType reports whether a value signal for this category on a column
// of this family must be discarded because no masker for the category could
// write into the column.
//
// It is not the same question as `accepted:`. The rule pack's list is the set
// of families a *name* hit may decide a column on, and ARCHITECTURE.md §4 keeps
// it deliberately tight: a name is weak evidence, so a name on a family the
// list omits is recorded at `low` and copied. Sampled values are evidence about
// this column, and CLAUDE.md's rule is "when in doubt, mask it" — so silencing
// one needs the stronger claim that the masking could not have been performed
// at all, not merely that the pack does not list the family.
//
// Three families are outside the gate for that reason, and each of them was
// masked before T-0054 and is masked again:
//
//   - famEnum. A labelled column is writable under *every* category:
//     mask.Writable answers `len(labels(c)) > 0` before it looks at the family
//     at all, and every generator begins with labelValue, so a masked enum is
//     one of its own labels (ARCHITECTURE.md §5). internal/plan leaves an enum
//     unjudged for the same reason.
//   - famXML and famOther. These are not "a family that refuses the category",
//     they are "a type this package has no family for" — an xml, an ltree, a
//     PostGIS geometry, an extension type, a domain whose base introspect could
//     not render. Nothing downstream is behind a silence here: internal/plan
//     does not refuse a type mask has no tag for, and internal/verify's second
//     net (columns.go, netText) runs over the character, uuid, inet, cidr and
//     macaddr families only. Copying such a column on the strength of a type
//     nobody recognised is exactly THREAT_MODEL.md T1's fail-open direction.
//
// What is left is the set the fix was opened for and nothing else: a family
// this package does recognise, whose maskers for the category emit a value it
// cannot hold — timestamp, date, time, interval, boolean, bytea, numeric,
// tsvector and the rest of the non-text families, per category.
func silencedByType(p *compiledPack, cat pipeline.Category, family string) bool {
	switch family {
	case famEnum, famXML, famOther:
		return false
	}
	return !p.accepted(cat, family)
}

// compositeSignal asks the one question that can be asked of a composite: do
// its fields carry personal data at all (tracker T-0094).
//
// It is jsonSignal's shape rather than the scalar validators' ratio, for
// jsonSignal's reason. A record is a small document with heterogeneous fields,
// so scoring the fields as if they were the column's values dilutes the
// evidence -- a (street, city, email) record is one third addresses and one
// third email, both under the weak threshold, and the column would be decided
// `none` and copied with the address in it. Any field of any sample that
// validates is a hit, because the answer here is not which masker to use: no
// masker can write a record, so what follows a hit is internal/plan's exit-12
// refusal naming --skip-table and --unmask, and what follows no hit is a copy
// whose reason says the fields were read.
//
// **Each sample is scored whole as well as field by field**, and either is a
// hit. Splitting alone was a fail-open, because a record can hold personal data
// that exists only as the concatenation of its fields: `(9,"Rue de
// Rivoli",Paris)` is an address, and no field of it is one — AddressShape wants
// a digit and two words in a single value, and the digit lives in its own
// field. Scored field-wise that record was `none` and copied verbatim, under a
// reason that said the fields had been read and none was personal data, which
// is worse than a silence. The whole literal is what the address shape reads.
//
// The category is the first validator in precedence order that matched, which
// is what the refusal names. A composite the splitter cannot read is scored as
// one opaque value, which is what happened to every composite before this
// existed.
func compositeSignal(dict *textsig.Dict, values []string, vs []validatorEntry) *valueSignal {
	fields := make([][]string, 0, len(values))
	for _, v := range values {
		// The raw sample first: precedence inside one record is the validator
		// order, not the order the strings are listed in, because the loop
		// below asks each validator about every string before moving on.
		record := []string{v}
		if f, ok := splitCompositeLiteral(v); ok {
			record = append(record, f...)
		}
		fields = append(fields, record)
	}
	for _, v := range vs {
		matched := 0
		for _, record := range fields {
			for _, f := range record {
				if v.ok(dict, f) {
					matched++
					break
				}
			}
		}
		if matched > 0 {
			return &valueSignal{cat: v.cat, phrase: v.phrase, matched: matched, total: len(values)}
		}
	}
	return nil
}

// jsonSignal walks the sampled documents. ARCHITECTURE.md §4 masks a json or
// jsonb column whole, so the question here is only whether the documents carry
// personal data at a leaf — which is what raises the column above the type
// signal every json column already has.
func jsonSignal(p *compiledPack, values []string) *valueSignal {
	matched := 0
	for _, doc := range values {
		for _, leaf := range jsonLeaves(doc) {
			if jsonLeafIsPersonal(p, leaf) {
				matched++
				break
			}
		}
	}
	if matched == 0 {
		return nil
	}
	return &valueSignal{cat: pipeline.CatSemiStruct, phrase: phraseJSONLeaf, matched: matched, total: len(values)}
}

// decide applies ARCHITECTURE.md §4's scoring rule to one column.
func (st *state) decide(w *work, col pipeline.Column, ct columnType, values []string, sig signals) {
	if ct.Family == famTSVector {
		// A tsvector is decided by its type and by nothing else. It is built
		// from other columns' text -- by a trigger, as pagila's film.fulltext
		// is, or by a generated expression -- so it holds the lexemes of a
		// column that may itself be masked, and copying it would ship those
		// words in cleartext beside the masked original (THREAT_MODEL.md T12).
		// There is no fake worth generating either: a tsvector of invented
		// lexemes is a search index that matches nothing, which is what the
		// empty one honestly is. So: always masked, always to ''::tsvector.
		w.d.Category = pipeline.CatDerivedText
		w.d.Confidence = pipeline.ConfCertain
		w.frags = append(w.frags, render("derived_text"))
		return
	}
	if ct.Family == famComposite {
		st.decideComposite(w, col, sig)
		return
	}
	best := sig.strong
	normTable, normCol := normaliseName(w.table.Name), normaliseName(col.Name)
	hit, hasName := st.pack.matchColumn(normTable, normCol)
	nameAccepted := hasName && st.pack.accepted(hit.Category, ct.Family)
	// T-0313: the bare name word needs corroboration before it decides
	// anything above low -- see bareNameVerdict and rules.yml's bare_name.
	bareFrag := ""
	if hasName && nameAccepted && hit.needsCorroboration() {
		verdict, frag := bareNameVerdict(hit, normTable, normCol, values)
		switch {
		case verdict == bareUnproven && best != nil:
			// Too few samples for the dictionary, and yet enough for a
			// value signal to decide the column outright: the values
			// decide it, below, and "masked on the name alone" would not
			// be true of the line.
		case verdict != bareUncorroborated:
			bareFrag = frag
		case best != nil:
			// The values decide the column on their own, at likely, exactly
			// as they do for any name and values that disagree (the
			// hasName && nameAccepted branch's best != nil case below), so
			// the uncorroborated name changes nothing here.
		case sig.strongHit != nil:
			// A strong validator matched a proven sample: one precise parse
			// of a personal value is corroboration enough to mask on the
			// name, as the column always was, and the line names the hit.
			bareFrag = render("samples", sig.strongHit.matched, sig.strongHit.total, sig.strongHit.phrase)
		default:
			// A lower rule may still name the column (`content_name` is
			// free_text as well as a bare name); only when none does is the
			// bare name recorded, at low.
			next, ok := st.pack.matchColumnAfter(normTable, normCol, hit.Name)
			if !ok {
				w.frags = append(w.frags, render("name_match", quoteIdent(hit.Name)))
				w.d.Category = pipeline.CatPersonName
				if sig.weak != nil {
					w.d.Category = sig.weak.cat
					w.frags = append(w.frags, render("samples", sig.weak.matched, sig.weak.total, sig.weak.phrase))
				}
				w.d.Confidence = pipeline.ConfLow
				w.nameUncorroborated = true
				w.frags = append(w.frags, frag)
				return
			}
			hit = next
			nameAccepted = st.pack.accepted(hit.Category, ct.Family)
		}
	}
	// Carried for internal/verify's national_id digits-family entry (T-0187
	// third review round, finding 1), independent of nameAccepted and of which
	// category the decision below actually records: see
	// pipeline.Decision.NameMatchedNationalID's own comment.
	if hasName && hit.Category == pipeline.CatNationalID {
		w.d.NameMatchedNationalID = true
	}

	switch {
	case hasName && nameAccepted:
		w.d.Category = hit.Category
		w.frags = append(w.frags, render("name_match", quoteIdent(hit.Name)))
		if bareFrag != "" {
			w.frags = append(w.frags, bareFrag)
		}
		switch {
		case hit.Category == pipeline.CatSpecial:
			// Special categories mask on name alone (ARCHITECTURE.md §4), which
			// is what puts testdata/README.md trap 24's enum above the
			// threshold with no value signal at all.
			w.d.Confidence = pipeline.ConfCertain
			w.frags = append(w.frags, render("special_by_name"))
		case best != nil && best.cat == hit.Category:
			w.d.Confidence = pipeline.ConfCertain
			w.frags = append(w.frags, render("samples", best.matched, best.total, best.phrase))
		case best != nil && hit.Category == pipeline.CatPhone && digitsOnlyMAC(best):
			// A column named phone whose only value evidence is plain digit runs
			// parsing as bare-hex MAC addresses (Pagila's address.phone, T-0297)
			// is a phone column: net.ParseMAC accepts 12 unseparated decimal
			// digits. It stays masked at the confidence the values would have
			// given it, and the reason does not name a category that did not
			// decide it. textsig.ValidMAC itself is unchanged, because the
			// second net and the DDL-literal passes read it too and must not
			// lose a digit run with no name to vouch for it.
			w.d.Confidence = pipeline.ConfLikely
		case best != nil:
			// The name and the values disagree about which category. The values
			// win, because they are evidence about this column rather than
			// about the person who named it, and both readings are above the
			// mask threshold either way.
			w.d.Category = best.cat
			w.d.Confidence = pipeline.ConfLikely
			w.frags = append(w.frags, render("samples", best.matched, best.total, best.phrase))
		default:
			w.d.Confidence = pipeline.ConfPossible
		}

	case hasName && !nameAccepted:
		// ARCHITECTURE.md §4 "Accepted types per category": the name signal is
		// gated by type, and only the name signal. testdata/README.md trap 19's
		// people.email_verified lands here.
		w.frags = append(w.frags,
			render("name_match", quoteIdent(hit.Name)),
			render("type_conflict", ct.Family, string(hit.Category)))
		switch cat, hasType := typeSignals[ct.Family]; {
		case best != nil:
			w.d.Category = best.cat
			w.d.Confidence = pipeline.ConfLikely
			w.frags = append(w.frags, render("samples", best.matched, best.total, best.phrase))

		case hasType:
			// **A rejected name hit must not leave the column worse off than no
			// name at all.** §4 gives json, jsonb, hstore and bytea a signal
			// from their type alone, and the default branch below is where that
			// signal used to be read — so a jsonb column whose *name* happened
			// to match a rule that does not accept jsonb landed on `low`, below
			// the mask threshold, and was copied verbatim.
			//
			// Supabase's auth schema is where that showed up, with the two
			// columns side by side: `identities.identity_data` (jsonb, no name
			// hit) was `semi_structured` and masked, and
			// `users.raw_user_meta_data` (jsonb, name matching the free_text
			// rule on its leading `raw_`) was `free_text` at `low` and copied —
			// with the person's name, email address, phone number and postal
			// address inside it. Two hundred of the source's own addresses
			// arrived in the target in cleartext, under exit 0
			// (THREAT_MODEL.md T1; testdata/regressions/008-name-hit-on-an-
			// unaccepted-type-drops-the-type-signal.sql).
			//
			// So the type decides here exactly as it would have with no name:
			// same category, same `possible`, same reason fragment. The name and
			// the conflict are still printed above, because why the name did not
			// decide is worth reading.
			//
			// trap 19's `people.email_verified boolean` is unaffected and still
			// lands on `low`: boolean has no entry in typeSignals, so there is
			// no type evidence to fall back to and §4's "recorded at low and
			// copied" is the whole answer for it.
			w.d.Category = cat
			w.d.Confidence = pipeline.ConfPossible
			w.frags = append(w.frags, render("type_signal", ct.Family, string(cat)))

		default:
			w.d.Category = hit.Category
			w.d.Confidence = pipeline.ConfLow
			w.typeConflict = true
		}

	case best != nil:
		// "values look like X" (ARCHITECTURE.md §4). testdata/README.md trap 20's
		// people.ref is this branch and nothing else.
		w.d.Category = best.cat
		w.d.Confidence = pipeline.ConfLikely
		w.frags = append(w.frags,
			render("samples", best.matched, best.total, best.phrase),
			render("no_name_signal"))

	case sig.strongHit != nil:
		// docs/reviews/2026-09-09/REVIEW.md finding 7,
		// evidence/sparse_email.log: one email address among nineteen
		// ordinary strings is a ratio of 5%, which reaches neither
		// validatorThreshold (best above) nor weakThreshold (sig.weak
		// below), so before this case existed the column fell all the way
		// to `none` and internal/transform copied the email verbatim.
		//
		// A *strong* validator (see the validators list above) is a precise
		// parse rather than a shape guess, so any hit at all among proven
		// samples is one production value of that category sitting in an
		// otherwise ordinary column -- category inference asks "what is this
		// column", which the 80% ratio answers well; residual detection asks
		// "does this column hold a recognisable value", which it answers
		// badly. Deciding the column `free_text` rather than the hit's own
		// category (email, phone, ...) is deliberate: the column is not
		// reliably that category, only mixed, and free_text's masker
		// replaces the whole value, so the minority of rows that do carry
		// personal data are covered without claiming the majority are
		// something they are not. This is also what keeps
		// internal/verify's second net from having to refuse an
		// already-loaded target over the same column: the fix on that side
		// (validators.go's strong field) fails any strong hit outright, with
		// no green path short of --unmask, so a column this case reaches
		// first is one refusal fewer.
		w.d.Category = pipeline.CatFreeText
		w.d.Confidence = pipeline.ConfPossible
		w.frags = append(w.frags,
			render("samples", sig.strongHit.matched, sig.strongHit.total, sig.strongHit.phrase),
			render("strong_hit_free_text"),
			render("no_name_signal"))

	case sig.weak != nil:
		// The samples point at a category without reaching the threshold that
		// decides on values alone. ARCHITECTURE.md §4 is silent on a partial
		// hit; recording it at `low` is what gives the neighbouring-column rule
		// something to raise once another column in the table is `likely`, and
		// `low` is below the mask threshold on its own.
		w.d.Category = sig.weak.cat
		w.d.Confidence = pipeline.ConfLow
		w.frags = append(w.frags,
			render("samples", sig.weak.matched, sig.weak.total, sig.weak.phrase),
			render("no_name_signal"))

	case sig.silencedStrong:
		// T-0269 fix round (a reviewer finding on T-0269 itself): the samples
		// validate, at validatorThreshold, for a category this column's type
		// family cannot hold -- exactly the shape the pre-T-0269 sig.refused
		// case existed for. That case set Category to the refused category and
		// named it in the reason ("timestamp is not an accepted type for
		// credential"), which is the alarm T-0269 removed; this case keeps only
		// the half of the old behaviour that matters operationally and none of
		// the noise. Category and Confidence are left at their zero values
		// (CatNone, ConfNone) -- not the refused category, and not `low` --
		// so the column reads exactly as any other column with no name or
		// value signal does (TestTimestampCredentialEntropyIsNeverScored), and
		// bypassing `default` below means a family with its own typeSignals
		// entry (inet, cidr, macaddr) is not masked on that entry either: the
		// silenced hit pre-empts it exactly as sig.refused used to.
		//
		// w.typeConflict is the one thing this case sets, and it is the fix:
		// sameColumnName, guessedPhoneColumns and fkPairs all gate on it, so a
		// same-named column elsewhere in the schema or a corroborating
		// neighbour can no longer raise this column to `possible` on evidence
		// that was never actually about it.
		// TestSameColumnNameDoesNotRaiseASilencedTypeConflict pins this.
		w.typeConflict = true
		w.frags = append(w.frags, render("no_signal"))

	default:
		if cat, ok := typeSignals[ct.Family]; ok {
			w.d.Category = cat
			w.d.Confidence = pipeline.ConfPossible
			w.frags = append(w.frags,
				render("type_signal", ct.Family, string(cat)),
				render("no_name_signal"))
			break
		}
		if sig.total >= minSamples && allTwoLetterCodes(values) {
			w.twoLetterCodes = true
			w.frags = append(w.frags, render("two_letter_codes"), render("no_name_signal"))
			break
		}
		// T-0197: a character column the validators actually looked inside
		// and found nothing in is evidence of absence, not evidence of
		// nothing -- "no name or value signal" reads as a clean bill of
		// health, which is exactly wrong for the unbounded class of names
		// and values no rule pack or dictionary can ever finish covering
		// (see this file's multilingual-dictionary note above). A column
		// nothing could be sampled from, or one outside the family the
		// validators run over, keeps the old phrase: there was nothing here
		// to look inside in the first place.
		//
		// "nothing recognised" is only honest when nothing did: sig.anyMatched
		// folds in every validator hit this branch's own guards discarded --
		// a strong hit below minSamples (a two-sample column can't reach
		// proven), a strong hit below validatorThreshold that missed
		// strongHit's own proven gate, a weak hit below weakThreshold, and a
		// silenced hit that never reached validatorThreshold -- so a column
		// with any of those still gets a phrase that admits something was
		// seen (T-0197 review finding 1).
		if sig.total > 0 && isCharacterFamily(ct.Family) {
			w.noSignalFrag = len(w.frags)
			if sig.anyMatched {
				w.frags = append(w.frags, render("sub_threshold_signal", sig.total))
			} else {
				w.frags = append(w.frags, render("nothing_recognised", sig.total))
			}
			break
		}
		w.frags = append(w.frags, render("no_signal"))
	}
	// T-0221: names the region a national-format phone hit was read under,
	// whichever branch above used it -- sig.strong decided the column outright
	// (best, above) or sig.strongHit masked it as free_text on a minority hit.
	// Both draw from st.validators' single region-aware entry, so at most one
	// of the two can ever carry the phrase.
	if regionAssumed(sig) {
		w.frags = append(w.frags, render("phone_region_configured", quoteIdent(st.region)))
	}
}

// bareVerdict is what bareNameVerdict found about a column the rule pack's
// bare_name rule matched (T-0313).
type bareVerdict int

const (
	// bareCorroborated: a word for people in the table or column name, or
	// enough samples carrying a word from the name dictionary.
	bareCorroborated bareVerdict = iota
	// bareUnproven: fewer than minSamples samples, so the dictionary cannot
	// answer and the name decides alone, as it always did.
	bareUnproven
	// bareUncorroborated: enough samples, and too few of them carry a
	// dictionary word.
	bareUncorroborated
)

// bareNameVerdict asks whether a column the bare_name rule matched holds a
// person's name, and returns the reason fragment that says what answered
// (T-0313, dogfood session 1).
//
// rules.yml's bare_name is the word `name` -- and `display_name`, and every
// `<thing>_name` -- which says a column holds *a* name. On a production Rails
// schema that was 38 `name` columns of tags, folders, playlists, widgets,
// roles, languages, AI models and triggers, each masked as a person's name,
// two of them under unique indexes that refused the plan. So the word decides
// `possible` only with one of three things behind it, tried in this order:
//
//  1. A word for people in the table name or the column name (the rule's
//     corroborated_by regexp): `users.name`, `staff.display_name`,
//     `orders.customer_name`. The table or the qualifier says whose name it
//     is, and no sample is needed.
//  2. Fewer than minSamples samples. An unproven column is not a clean one
//     (bestSignal's own argument), so the name decides alone exactly as it
//     did before this rule existed: an empty table, or a two-row one, is
//     masked.
//  3. At least nameCorroborationThreshold of the samples carrying a word from
//     the name dictionary (textsig.Dict.ContainsName, the word-level question,
//     not LooksLikeName's whole-value one: "Dr. Jane Smith" and "Jan Kowalski"
//     carry a name and are not one to LooksLikeName).
//
// With none of the three, decide records the column at low: the
// neighbouring-column rule still raises it beside a likely column, and
// sameColumnName does not (work.nameUncorroborated). A person's name in such a
// column that the dictionary cannot carry -- a script it does not hold, a name
// it does not list -- is then copied, which is THREAT_MODEL.md T1's stated
// residual for a name the dictionary cannot carry, as it already was for a
// non-Latin name in a column named in that script.
func bareNameVerdict(hit compiledPattern, normTable, normCol string, values []string) (bareVerdict, string) {
	if word, ok := hit.corroboratedByName(normTable, normCol); ok {
		return bareCorroborated, render("bare_name_by_word", quoteIdent(word))
	}
	if len(values) < minSamples {
		return bareUnproven, render("bare_name_unproven", len(values))
	}
	dict := textsig.Dictionary()
	matched := 0
	for _, v := range values {
		if dict.ContainsName(v) {
			matched++
		}
	}
	if float64(matched)/float64(len(values)) >= nameCorroborationThreshold {
		return bareCorroborated, render("bare_name_by_samples", matched, len(values))
	}
	return bareUncorroborated, render("bare_name_uncorroborated", matched, len(values))
}

// regionAssumed reports whether sig's strong or strongHit signal is the
// region-aware phone entry buildValidators adds (T-0221) -- the one entry
// whose phrase is phraseE164Region, never set unless a region was configured.
func regionAssumed(sig signals) bool {
	for _, v := range [2]*valueSignal{sig.strong, sig.strongHit} {
		if v != nil && v.cat == pipeline.CatPhone && v.phrase == phraseE164Region {
			return true
		}
	}
	return false
}

// decideComposite is the fail-closed answer for a column of a composite type
// (tracker T-0094, THREAT_MODEL.md T1).
//
// No category in the rule pack accepts a composite and none can: mask.TypeTag
// has no tag for one, every generator emits a scalar, and there is no shape a
// record could be masked into field-wise without a masker per field. Before
// this branch a composite took the ordinary path, where a name hit on a type no
// category accepts is recorded at `low` and copied and a value hit could decide
// a category whose masker cannot be written -- so a composite carrying an
// address was copied into the target verbatim under exit 0, and one the
// validators did decide died at load.
//
// So the decision is only ever one of two. A hit -- from the name or from any
// field of any sample -- is `possible`, which is above ARCHITECTURE.md §4's
// mask threshold and is what internal/plan turns into an exit-12 refusal naming
// the column, --skip-table and a reasoned --unmask (writeback.go). No hit is a
// copy, and the reason says the fields were read and said nothing, so that a
// green run over a composite is a claim somebody made rather than a silence --
// and when there was nothing to read at all, it says that instead, because a
// claim about a check that did not run is worse than no claim.
//
// `possible` and not `likely`: the confidence is a claim about how much is
// known, and what is known is that something in the record looked personal.
// Nothing downstream reads it above the threshold -- the refusal is on
// Decision.Masked -- and the operator's escape is a flag either way.
func (st *state) decideComposite(w *work, col pipeline.Column, sig signals) {
	hit, hasName := st.pack.matchColumn(normaliseName(w.table.Name), normaliseName(col.Name))
	best := sig.strong
	switch {
	case best != nil:
		w.d.Category = best.cat
		w.d.Confidence = pipeline.ConfPossible
		if hasName {
			w.frags = append(w.frags, render("name_match", quoteIdent(hit.Name)))
		}
		w.frags = append(w.frags,
			render("samples", best.matched, best.total, best.phrase),
			render("composite_refused"))
	case hasName:
		w.d.Category = hit.Category
		w.d.Confidence = pipeline.ConfPossible
		w.frags = append(w.frags,
			render("name_match", quoteIdent(hit.Name)),
			render("composite_refused"))
	case sig.total == 0:
		// No name, and nothing to read: the copy is decided on the name alone
		// and the reason has to say so. Claiming the fields were read here
		// would be a claim about a check that did not run, on precisely the
		// branch that copies -- and it is the common case, since a composite
		// column in an empty table is exactly this.
		w.frags = append(w.frags, render("composite_no_sample"))
	default:
		w.frags = append(w.frags, render("composite_no_signal"))
	}
}

func allTwoLetterCodes(values []string) bool {
	if len(values) == 0 {
		return false
	}
	for _, v := range values {
		if !textsig.TwoLetterCode(v) {
			return false
		}
	}
	return true
}

// appendContext adds the fragments that explain where the evidence came from
// rather than what it said.
func (st *state) appendContext(w *work, t pipeline.Table, ct columnType, total int) {
	if ct.Array {
		w.frags = append(w.frags, render("array_element", ct.Family))
	}
	switch {
	case t.SampledFrom != nil && total > 0:
		w.frags = append(w.frags, render("partition_samples", quoteTable(*t.SampledFrom)))
	case t.Partitioned && len(t.Partitions) == 0 && total == 0:
		w.frags = append(w.frags, render("no_leaf_samples"))
	case total == 0:
		w.frags = append(w.frags, render("no_samples"))
	}
	if isJSONFamily(w.family) && st.pack.logShapedTable(t.Ref.Name) {
		w.frags = append(w.frags, render("json_log_shaped"))
	}
}

func isJSONFamily(family string) bool {
	return family == famJSON || family == famJSONB || family == famHstore
}

// markNeverMasked records the classes ARCHITECTURE.md §4 never masks and
// always explains. They are excluded from every raising pass below, so that no
// rule can quietly put a generator on a generated column or a join key.
func (st *state) markNeverMasked(w *work, t pipeline.Table, col pipeline.Column, ct columnType) {
	// A framework metadata table (T-0314): schema_migrations, ar_internal_metadata
	// and the rest of pipeline.IsFrameworkMetadataTable's list. A column of one
	// that is also on pipeline.IsFrameworkMetadataColumn's own allowlist for
	// that table is bookkeeping the framework itself wrote and reads back,
	// never end-user data, so root CLAUDE.md's "when in doubt, mask it" does
	// not apply to it by default -- there is no doubt to resolve, the way
	// there is for an ordinary column this package has never seen a signal
	// from. internal/plan's own T-0314 entry is the other half: it copies the
	// table whole as a lookup regardless of what this function decides, so a
	// masked column here would still cost nothing to the row count and
	// everything to what Rails, Django or Flyway reads back from it. This is
	// checked first, ahead of the generated-column and surrogate-key checks
	// below, because it is a property of the table (and, since the T-0314
	// review round's second finding, of the specific column) rather than of
	// the column's own signals, and it must win over whatever either of those
	// would have said about the same column.
	//
	// The table match is on the bare name alone, in any schema, so the "never
	// end-user data" premise can be wrong about a same-named application
	// table -- w.frameworkMetadata's own doc comment is where the two passes
	// that may still find doubt and mask anyway (a validated FK to a masked
	// parent, an explicit lazyslice.yml raise) are recorded. The column
	// allowlist is the same premise applied one level narrower: even a
	// genuine instance of the tool's own table carries columns -- Liquibase's
	// AUTHOR, Flyway's INSTALLED_BY -- that are a developer's or a database
	// role's identity rather than the tool's bookkeeping, and a column not on
	// the allowlist falls through to the ordinary passes below exactly as if
	// its table were not on pipeline.IsFrameworkMetadataTable's list at all.
	if pipeline.IsFrameworkMetadataTable(t.Ref.Name) && pipeline.IsFrameworkMetadataColumn(t.Ref.Name, col.Name) {
		w.neverMask = true
		w.frameworkMetadata = true
		w.frameworkMetadataFrag = len(w.frags)
		w.frags = append(w.frags, render("framework_metadata"))
		return
	}
	if col.Generated != "" {
		w.neverMask = true
		w.generated = true
		w.frags = append(w.frags, render("generated"))
		return
	}
	if !isKeyFamily(ct.Family) || ct.Array {
		return
	}
	// A key column is a surrogate key only when the classifier found nothing
	// personal in it. ARCHITECTURE.md §4 scopes the exemption to "surrogate keys
	// (id bigint and the FK columns that reference them)"; a natural key is not
	// one, and exempting every integer or uuid key would copy
	// subscribers(msisdn bigint primary key) and people(nhs_number bigint)
	// verbatim under a green tick (THREAT_MODEL.md T1). It would also make §4's
	// own propagation sentence -- "a masked PK or unique column's decision
	// overrides the decision on every column referencing it" -- unreachable for
	// the common integer and uuid case, because such a key could never be
	// masked.
	//
	// The gate is the mask threshold rather than "has a category", so a key
	// whose name hit was on a type its category refuses (address_id integer)
	// keeps the exemption and its verbatim reason; only a key the classifier
	// would already have masked loses it.
	//
	// The exemption recorded here is provisional in one direction: a column
	// whose own signals say nothing can still be the child of a key that is
	// masked, and foreignKeys() lifts it there. It is what a column *knows about
	// itself* before propagation has run.
	if w.d.Confidence >= pipeline.ConfPossible {
		return
	}
	for _, name := range t.PK {
		if name == col.Name {
			w.neverMask = true
			w.keyFrag = len(w.frags)
			w.frags = append(w.frags, render("surrogate_key"))
			return
		}
	}
	for _, fk := range st.schema.FKs {
		if fk.Child != t.Ref {
			continue
		}
		for _, name := range fk.ChildCols {
			if name == col.Name {
				w.neverMask = true
				w.keyFrag = len(w.frags)
				w.frags = append(w.frags, render("fk_column", quoteTable(fk.Parent)))
				return
			}
		}
	}
}

// isKeyFamily is the type side of "surrogate keys (id bigint and the FK columns
// that reference them)". A text primary key is not a surrogate key and is
// classified like any other column.
func isKeyFamily(family string) bool {
	return family == famInteger || family == famBigint || family == famUUID
}

// ---------- pass 2: bytea in a person-shaped table ----------

func (st *state) byteaInPersonShapedTable() {
	for _, t := range st.schema.Tables {
		certain := 0
		for _, col := range t.Columns {
			if w := st.dec[ref.ColumnRef{Table: t.Ref, Column: col.Name}]; w != nil && w.d.Confidence == pipeline.ConfCertain {
				certain++
			}
		}
		if certain == 0 {
			continue
		}
		for _, col := range t.Columns {
			w := st.dec[ref.ColumnRef{Table: t.Ref, Column: col.Name}]
			if w == nil || w.family != famBytea || w.neverMask || w.d.Confidence >= pipeline.ConfPossible {
				continue
			}
			w.d.Category = pipeline.CatBinary
			w.d.Confidence = pipeline.ConfPossible
			w.frags = append(w.frags, render("bytea_person_shaped", quoteTable(t.Ref), certain))
		}
	}
}

// ---------- pass 3: the neighbouring-column rule ----------

func (st *state) neighbouringColumns() {
	for _, t := range st.schema.Tables {
		likely := 0
		maskedPersonal := 0
		for _, col := range t.Columns {
			w := st.dec[ref.ColumnRef{Table: t.Ref, Column: col.Name}]
			if w == nil {
				continue
			}
			if w.d.Confidence >= pipeline.ConfLikely {
				likely++
			}
			if maskedPersonalNeighbour(w) {
				maskedPersonal++
			}
		}
		// Decision.TableHasLikelyPersonalColumn and
		// Decision.TableHasMaskedPersonalColumn are both carried on every
		// column of the table, not only the ones this pass goes on to raise
		// (T-0187 third review round, finding 1, and T-0240): internal/
		// verify's second net reads them off a column this rule never
		// touches -- a numeric or character column at `none` or `low` whose
		// own confidence never reaches ConfLow's exact match below.
		// "Another" excludes the column's own confidence, which is what a
		// neighbour has to mean; a column already at ConfLikely or above is
		// masked and never reaches that net regardless, and the same is true
		// of maskedPersonalNeighbour's own ConfPossible floor.
		for _, col := range t.Columns {
			w := st.dec[ref.ColumnRef{Table: t.Ref, Column: col.Name}]
			if w == nil {
				continue
			}
			others := likely
			if w.d.Confidence >= pipeline.ConfLikely {
				others--
			}
			w.d.TableHasLikelyPersonalColumn = others > 0

			othersMasked := maskedPersonal
			if maskedPersonalNeighbour(w) {
				othersMasked--
			}
			w.d.TableHasMaskedPersonalColumn = othersMasked > 0
		}
		if likely == 0 {
			continue
		}
		for _, col := range t.Columns {
			w := st.dec[ref.ColumnRef{Table: t.Ref, Column: col.Name}]
			if w == nil || !st.raisable(w) || w.d.Confidence != pipeline.ConfLow {
				continue
			}
			w.d.Confidence = pipeline.ConfPossible
			w.d.Source = pipeline.ByNeighbour
			w.frags = append(w.frags, render("neighbour", quoteTable(t.Ref), likely))
		}
	}
	// T-0221: after TableHasLikelyPersonalColumn is filled (above, so the
	// corroboration signal exists) and before unknownColumnsBesideCertain
	// (below), which would otherwise sweep the same column into free_text
	// first -- a character column with no signal at all, beside a `certain`
	// person-identifying neighbour, is exactly the shape both this pass and
	// that one can reach, and a real phone hit deserves its own category
	// rather than the generic catch-all one. unknownColumnsBesideCertain's
	// own raisableUnknown skips a column this pass has already decided
	// (Category != CatNone), so the two never fight over one column.
	st.guessedPhoneColumns()
	st.unknownColumnsBesideCertain()
}

// unknownColumnsBesideCertain is the second arm of the neighbouring-column
// rule, and it is the 2026-09-15 red team's A2b.
//
// The arm above raises `low` to `possible`: a column that had *some* signal,
// under the threshold, in a table with a `likely` column. What it cannot reach
// is a free-text column with no signal at all — no name rule, no validator, no
// type signal — sitting at `none` beside a column the classifier is certain
// about. That column is copied verbatim, and the report says "no name or value
// signal", which reads as a clean bill of health for a column nobody looked
// inside. `entries.label`, `records.ident` and `blobs.payload2` were all that
// column. CLAUDE.md's rule for that state is "when in doubt, mask it".
//
// So: a character column with no decision at all, in a table that already
// holds a column at `certain`, is masked as free_text. The strength required of
// the neighbour is `certain` and not `likely`, one step above the arm that
// raises a column with evidence of its own, because this arm has no evidence
// about *this* column to weigh against the false positive.
//
// The exclusions, each a run this rule must not break rather than a
// softening of it:
//
//   - A never-masked column (a generated column, a surrogate key, a FK
//     column) and a type-conflicting decision: the same exclusions raisable
//     applies to the arm above, for the same reasons.
//
//   - A column under a unique index. free_text's generator would then have to
//     emit d_required = n²/2ε distinct values (ARCHITECTURE.md §5), and
//     internal/plan refuses at exit 12 when it cannot — over a column this
//     rule masked on no evidence at all.
//
//   - A column of two-letter codes. An ISO country or language column has a
//     domain of two characters and reads as an unknown text column to every
//     signal in §4; masking one is a plan refusal for the same reason, and it
//     is not personal data.
//
//   - A column whose samples are an enumeration or all one identifier shape
//     (T-0311, dogfood session 1: a production Rails copy that did not boot
//     because `role`, `state` and a text uuid held free_text filler). This
//     and the unique-index exclusion above are skipped in the sweep loop, and each
//     prints why on the column's line; spare.go has the thresholds and the
//     guard that keeps a name out of both.
//
//   - A column at either end of a validated foreign key used to be excluded
//     outright here (`indexFKColumns`, T-0239's fix-round review), on the
//     claim that "a genuinely personal FK-linked column is still reached by
//     every other pass — a name hit, a value validator, or propagation once
//     one end is masked on real evidence." **The 2026-09-17 round-5 red team
//     disproved that claim**
//     (`docs/reviews/2026-09-15-redteam/round5-still-leaking.json`, the
//     classifier attacker's FK variant): a validated foreign key's
//     character-family child can carry the exact shape this rail exists for —
//     a native-script name, no name rule, no value hit — beside a `certain`
//     email neighbour, with a parent that has no `certain` column of its own
//     for any other pass to key on, and nothing in this package ever reaches
//     the child either: `keyChildren` only reconciles the integer/uuid key
//     case (`isKeyFamily`), and `propagateKeys` only ever propagates a masked
//     **parent** forward, never a masked child back. The exclusion copied real
//     personal data verbatim on both ends under exit 0 (THREAT_MODEL.md T1) —
//     worse than the half-loaded target (T-0132's failure mode, exit 8) it
//     was written to avoid, which is a refusal that costs a rerun rather than
//     a leak that costs nothing at all.
//
//     `fkPairs` (below) replaces the exclusion: a column this rail would
//     otherwise raise alone, at either end of a validated foreign key, is
//     raised together with every column paired to it (`indexFKColumns`), so
//     the join stays in agreement and the same values are not copied on one
//     side and replaced with free_text on the other — the same argument
//     `propagateKeys` already makes for a parent-first decision, run in the
//     direction that pass cannot reach. Where a partner cannot be raised the
//     same way — it already carries a decision of its own that
//     ARCHITECTURE.md §4 does not let this rail override, or a value shape
//     (`twoLetterCodes`) that says the column is a code lookup and not
//     personal data — neither end is raised: raising one alone would still
//     copy the pair, which is the one outcome this rule must never produce.
//     `testdata/regressions/031-fk-child-code-column-beside-a-certain-column.sql`
//     now pins the safe direction reached by raising both ends together
//     rather than by masking neither; the round-5 attack's own schema is
//     `testdata/regressions/035-native-script-fk-child-beside-a-certain-column.sql`.
//     A validated foreign key's partner that is itself under a unique index
//     is not excluded from the pairing the way the rail's own column is
//     (just above): it is a partner *because* the certain neighbour supplies
//     the evidence the standalone exclusion says it has none of, and
//     `internal/plan`'s own unique-index domain check — unchanged — is what
//     admits or refuses the masked result on its own existing terms once
//     both ends carry a decision. Measured against both regressions above, a
//     small lookup table refuses: `free_text`'s generator draws from a fixed
//     word list, so a narrow unique column's domain is nowhere near
//     ARCHITECTURE.md §5's `d_required` at any but a handful of rows, and
//     `internal/plan` refuses at exit 12 naming both ends of the pair
//     together with `--unmask` for each — the same message T-0132's
//     equality-group mechanism already prints, unmodified.
//
//     **T-0253's review round found two more things to say.** First,
//     `fkPairs` originally walked the whole connected component `fkPartners`
//     reaches, not only `cref`'s own direct partners — a partner two hops
//     away, in a table with nothing to do with the `certain` neighbour that
//     justified raising `cref` at all, could veto the pairing on its own
//     shape (a two-letter code, a type conflict) and leave `cref` itself
//     copied verbatim, the very leak this rail exists to close. `fkPairs` now
//     walks only `cref`'s direct partners; a masked parent still reaches
//     every other table that references it, because `propagateKeys` (below,
//     a separate pass that runs after this one in `Classify`'s ordering)
//     already does that unconditionally, per ARCHITECTURE.md §4's own
//     propagation sentence, and does not veto the parent's own masking when
//     one further-out child cannot accept the category — it records
//     `type_conflict` on that one child and moves on. Second, when a direct
//     partner cannot be raised, neither end is raised, exactly as before, but
//     both ends now carry `Decision.Refused` naming the other
//     (`internal/pipeline/classify.go`), the signal `internal/plan` reads
//     (`checkFKPairRefusal`, `internal/plan/fkpair.go`, tracker T-0257) to
//     turn this into the exit-12 refusal the paragraph above already
//     describes, in place of the pre-T-0253 copy under exit 0.
//
// A sixth exclusion — a declared length under sixteen characters — used to
// stand here too, and the 2026-09-15 round-4 red team's native-script variant
// (T-0239) found it wrong. The comment that shipped with it claimed
// "free_text's filler does not fit" in a short column; `mask.freeTextMasker`
// does not agree with its own former defence — its filler is drawn to *fit*
// whatever `Constraints.MaxLen` says down to a single byte, and the domain
// rule above already excludes the one case where a narrow column is refused
// rather than masked (a unique index, where §5's `d_required` might exceed
// what a narrow domain can offer). What a short declared length is not is
// evidence the column is impersonal: a given name, a surname, a postcode, a
// national ID and a phone number all fit in `varchar(12)`, and an attacker —
// or an ordinary schema author — picks the length, not this package. So the
// floor is now `minUnknownLen`, two characters: nothing shorter can hold even
// a two-letter code, and everything from there to any width is raised, the
// same as it always was above sixteen. **This is a floor of two, not three**
// — it excludes only a declared length of one, which cannot hold even a
// two-letter code, and it is not a backstop for the two-letter-code shape
// itself: the two-letter-code exclusion just above is a check over the
// column's *values* (`w.twoLetterCodes`, set only once `decide`'s own
// `minSamples`, three, is met), so a genuine ISO code column sampled fewer
// than three times has no value-level check to fall back on and is masked by
// this rail like any other unrecognised column — correctly, on this
// project's own "when in doubt, mask it" rule (CLAUDE.md), not by accident of
// a floor that happens to still exclude it. If a generator ever cannot write
// into a column this rail raises, `internal/plan`'s write-back check
// (`checkWriteBack`, `mask.Writable`) is what refuses the run at exit 12
// naming the column, with `--skip-table` and `--unmask` — the same backstop a
// unique column already relies on above, and the reason there is no narrower
// "does this fit" gate written here: that question belongs to the generator
// and the planner, not to a guess made from `pg_attribute.atttypmod` before
// any masker has been asked.
//
// It is not the whole answer to A2b, and the attack itself says so: its own
// table had no `certain` column, so nothing here reaches it. What reaches
// that one is the multilingual dictionary and textsig.Candidates. This is the
// rail under both of them, for the next value shape neither recognises.
func (st *state) unknownColumnsBesideCertain() {
	for _, t := range st.schema.Tables {
		certain := 0
		for _, col := range t.Columns {
			if w := st.dec[ref.ColumnRef{Table: t.Ref, Column: col.Name}]; w != nil &&
				w.d.Confidence == pipeline.ConfCertain && identifiesAPerson(w.d.Category) {
				certain++
			}
		}
		if certain == 0 {
			continue
		}
		for _, col := range t.Columns {
			cref := ref.ColumnRef{Table: t.Ref, Column: col.Name}
			w := st.dec[cref]
			if !st.raisableUnknown(cref, w) {
				continue
			}
			if st.unique[cref] {
				// Skipped before fkPairs, exactly as raisableUnknown used to
				// skip it; T-0311 only makes the line say so.
				w.frags = append(w.frags, render("spared_unique", quoteTable(t.Ref), certain))
				continue
			}
			partners, blocked, ok := st.fkPairs(cref)
			if ok {
				if frag := st.spared(w, t.Ref, certain); frag != "" {
					// T-0311: the column's own samples, or its unique index,
					// say the sweep would be wrong here; the line says which.
					// A spared column is not raised and its foreign-key
					// partners are not touched on its account, so no pair is
					// split either. It is asked only once fkPairs has agreed:
					// a partner carrying a decision of its own (a name hit, a
					// type conflict -- testdata/regressions/037's `dob`) is
					// evidence about the values on both ends of the join, and
					// the child's enumeration-shaped samples must not copy
					// the pair past the T-0257 refusal below.
					w.frags = append(w.frags, frag)
					continue
				}
			}
			if !ok {
				// T-0253: cref is at either end of a validated foreign key
				// and its direct partner cannot be raised the same way
				// (fkPairs' own comment has the reasons). Raising cref alone
				// would copy the pair exactly as the blanket exclusion this
				// replaces did, so neither end is touched here — but both now
				// carry Decision.Refused naming the other, which
				// internal/plan reads to refuse the run at exit 12 instead of
				// copying the pair under exit 0 (tracker T-0257;
				// internal/plan/fkpair.go; internal/pipeline/classify.go's
				// own comment on the field).
				reason := render("fk_pair_refused", quoteColumn(blocked))
				w.d.Refused = reason
				w.d.RefusedPartner = blocked
				w.frags = append(w.frags, reason)
				if bw := st.dec[blocked]; bw != nil {
					bReason := render("fk_pair_refused", quoteColumn(cref))
					bw.d.Refused = bReason
					bw.d.RefusedPartner = cref
					bw.frags = append(bw.frags, bReason)
				}
				continue
			}
			w.d.Category = pipeline.CatFreeText
			w.d.Confidence = pipeline.ConfPossible
			w.d.Source = pipeline.ByNeighbour
			w.sweptNoSignal = true
			w.frags = append(w.frags, render("neighbour_unknown", quoteTable(t.Ref), certain))
			for _, p := range partners {
				w.frags = append(w.frags, render("fk_pair", quoteColumn(p)))
				pw := st.dec[p]
				pw.d.Category = pipeline.CatFreeText
				pw.d.Confidence = pipeline.ConfPossible
				pw.d.Source = pipeline.ByNeighbour
				pw.sweptNoSignal = true
				pw.frags = append(pw.frags, render("fk_pair", quoteColumn(cref)))
			}
		}
	}
}

// fkPairs is unknownColumnsBesideCertain's join-safety pairing (T-0253; see
// that function's own comment on the exclusion it replaces). cref has
// already passed raisableUnknown on its own signals; this asks whether every
// column *directly* paired to it across a validated foreign key
// (`st.fkPartners[cref]`) can be raised the same way, so that masking cref
// never leaves its partner's identical values unmasked on the other side of
// the join.
//
// It returns every column that has to be raised alongside cref and true when
// all of them qualify — the caller raises cref and all of them together — or
// nil, the first disqualifying column, and false when at least one does not,
// in which case the caller raises nothing: a partner this rail cannot bring
// into agreement must not be worked around by masking only the column that
// happens to have a certain neighbour of its own, because that copies the
// pair exactly as leaving both alone did.
//
// A column with no partner at all (not part of any validated foreign key)
// returns an empty set and true, which is unknownColumnsBesideCertain's
// ordinary single-column raise.
//
// It stops at cref's *direct* partners in the downward direction — a parent
// raised here reaches every other child through `propagateKeys` (below), a
// separate pass that runs after this one in `Classify`'s ordering and already
// propagates a masked parent to *every* column referencing it, unconditionally,
// per ARCHITECTURE.md §4 — but it walks upward without that bound (T-0253's
// second review round, high finding). `propagateKeys` only ever pushes a
// decision from a masked parent down to its children; nothing else in this
// package ever raises an unmasked parent because one of *its* children just
// got masked. So when a raised partner is itself the child end of a further
// validated foreign key, that further parent is never reached by anything —
// the same asymmetry T-0253 exists to fix in the first place, one hop
// further out: `name_root.slug_root <- name_mid.slug_mid <- members.slug_leaf`
// (regression 036) raised `members.slug_leaf` and `name_mid.slug_mid`
// together and left `name_root.slug_root`, holding the identical values,
// copied verbatim (THREAT_MODEL.md T1), and also left the load with a masked
// child and an unmasked parent across the `name_mid.slug_mid ->
// name_root.slug_root` edge (the T-0132 half-loaded-target shape, T8). So
// this walks `fkParents` — the child-to-parent half of `fkPartners`
// — transitively from cref and from every column it has already collected,
// gathering and gating each further parent the same way `fkPartnerRaisable`
// gates a direct one, until nothing new is found. A disqualified ancestor,
// however many hops up, still refuses the whole set: masking the columns
// below it and leaving it copied is exactly the leak above.
//
// The bound that survives from the review before this one is the *downward*
// direction only: this never walks from a raised column to a further child
// of *its own* (only to its own parents), because that is the direction
// `propagateKeys`'s unconditional, per-column sweep already covers, and
// walking it here would resurrect the medium finding that bounded this
// function to direct partners in the first place — an unrelated column many
// hops down, in a table with no relationship to the `certain` neighbour that
// justified raising cref at all, vetoing cref's own masking.
// testdata/regressions/031 and 035 (both a single direct edge, no chain) are
// unaffected; regression 036 pins the chain.
func (st *state) fkPairs(cref ref.ColumnRef) (partners []ref.ColumnRef, blocked ref.ColumnRef, ok bool) {
	seen := map[ref.ColumnRef]bool{cref: true}
	visit := func(c ref.ColumnRef) bool {
		if seen[c] {
			// A redundant duplicate FK declaration between the same two
			// columns, a composite key visiting the same partner twice, or a
			// column reached both as a direct partner and as an ancestor of
			// another one — nothing to add the second time.
			return true
		}
		seen[c] = true
		if !st.fkPartnerRaisable(c) {
			blocked = c
			return false
		}
		partners = append(partners, c)
		return true
	}

	for _, p := range st.fkPartners[cref] {
		if !visit(p) {
			return nil, blocked, false
		}
	}

	queue := append([]ref.ColumnRef{cref}, partners...)
	for len(queue) > 0 {
		c := queue[0]
		queue = queue[1:]
		for _, parent := range st.fkParents[c] {
			if seen[parent] {
				continue
			}
			if !visit(parent) {
				return nil, blocked, false
			}
			queue = append(queue, parent)
		}
	}

	return partners, ref.ColumnRef{}, true
}

// fkPartnerRaisable is fkPairs' own gate on one partner. It asks the same
// question raisableUnknown asks of cref itself, with one deliberate
// difference: a partner under a unique index is not excluded here (see
// unknownColumnsBesideCertain's own comment on why) — `internal/plan`'s
// unique-index domain check, unchanged, is what judges that once the column
// carries a decision.
//
// A partner that already carries any decision of its own — a name hit, a
// value signal, a type conflict recorded at `low` — is refused rather than
// overridden: ARCHITECTURE.md §4 never lets a raising pass move a
// type-conflicting decision, and a category this rail did not choose is not
// this rail's to replace. `twoLetterCodes` is refused for a different
// reason: it is real, measured evidence that the column's whole domain is a
// short code, which is not personal data whatever its neighbour holds.
func (st *state) fkPartnerRaisable(pref ref.ColumnRef) bool {
	pw := st.dec[pref]
	if pw == nil || pw.neverMask || pw.typeConflict || pw.twoLetterCodes {
		return false
	}
	if pw.d.Category != pipeline.CatNone || pw.d.Confidence != pipeline.ConfNone {
		return false
	}
	if !isCharacterFamily(pw.family) {
		return false
	}
	if n, ok := declaredLength(pw.column); ok && n < minUnknownLen {
		return false
	}
	return true
}

// identifiesAPerson is the set of categories that make a table person-shaped
// for the rule above: the ones that name, locate or contact an individual.
//
// The four it leaves out are the reason it exists. free_text, semi_structured,
// binary_personal and derived_text are categories a column reaches **by its
// type alone** — a tsvector is derived_text at `certain` on every schema that
// has one, a jsonb is semi_structured at `certain` — so counting them made
// pagila's `film` table person-shaped on the strength of its own search index
// and masked `film.title` and `film.special_features` with it. A table is
// person-shaped because it holds a person, not because it holds a document.
func identifiesAPerson(cat pipeline.Category) bool {
	switch cat {
	case pipeline.CatEmail, pipeline.CatPersonName, pipeline.CatPhone,
		pipeline.CatAddress, pipeline.CatGeo, pipeline.CatPersonDate,
		pipeline.CatNationalID, pipeline.CatFinancial, pipeline.CatNetworkID,
		pipeline.CatOnlineID, pipeline.CatCredential, pipeline.CatSpecial:
		return true
	case pipeline.CatNone, pipeline.CatFreeText, pipeline.CatBinary,
		pipeline.CatSemiStruct, pipeline.CatDerivedText:
		return false
	}
	return false
}

// maskedPersonalNeighbour is Decision.TableHasMaskedPersonalColumn's own gate
// (T-0240, the 2026-09-15 round-4 red team's A9b replays): a column counts as
// a masked person-identifying neighbour once its own decision has reached
// ConfPossible -- the mask threshold -- under a category that
// identifiesAPerson, and is not exempt from masking as a surrogate key or an
// FK column. It is deliberately a lower floor than the `likely` count above
// (TableHasLikelyPersonalColumn's own ConfLikely), because a name-only match
// with no samples decides a column at ConfPossible and no higher (§4's own
// threshold), and a column the run itself is about to mask on its name alone
// -- `msisdn numeric`, masked as `phone` -- is evidence about the table
// whatever confidence line it landed on. It reads w.d.Confidence and
// w.d.Category rather than w.d.Masked, because this pass runs before
// `finalise` sets that field for every column (the six-pass order in this
// package's own CLAUDE.md); !w.neverMask is what the mask verdict actually
// depends on at this point in the run, since a per-column --unmask opt-out
// (w.unmasked) is not resolved until applyPrior, later still, and carries the
// same limitation TableHasLikelyPersonalColumn's own count already accepts.
//
// **It is evaluated inside neighbouringColumns' own first loop, which is
// earlier in the six-pass order than several passes that can still raise a
// column to ConfPossible or above** (T-0240 review round, medium finding):
// guessedPhoneColumns and unknownColumnsBesideCertain, both called from the
// end of neighbouringColumns itself (Classify's own ordering comment), and
// sameColumnName, keyChildren and foreignKeys, which run as separate passes
// afterwards. A column whose only person-identifying signal is decided by one
// of those -- a guessed-region phone hit with no name of its own, the
// free_text catch-all beside a `certain` neighbour, a category shared by
// column name, or FK propagation from a masked parent -- does not set this
// field, because the loop that reads w.d.Confidence and w.d.Category here has
// already run by the time any of them would move it. THREAT_MODEL.md's own
// T1, T-0240 amendment states the same limitation in its own terms: the
// residual it closes is narrower, in classification order, than "no other
// column decided possible or above under a person-identifying category"
// reads on its own.
func maskedPersonalNeighbour(w *work) bool {
	return w.d.Confidence >= pipeline.ConfPossible && !w.neverMask && identifiesAPerson(w.d.Category)
}

// minUnknownLen is the shortest declared length unknownColumnsBesideCertain
// will mask. Below it (a declared length of one) a character column cannot
// hold even a two-letter code, let alone a name, a postcode, a national ID or
// a phone number — T-0239 lowered this from sixteen, which excluded exactly
// the shape the round-4 red team's native-script variant used to defeat this
// rail: see unknownColumnsBesideCertain's own comment for why sixteen was
// wrong and why this is not the same claim as "free_text does not fit".
const minUnknownLen = 2

// raisableUnknown is unknownColumnsBesideCertain's gate, kept apart from
// raisable because the two ask different questions: raisable is about a
// decision that exists, and this is about the absence of one.
func (st *state) raisableUnknown(cref ref.ColumnRef, w *work) bool {
	if w == nil || w.neverMask || w.typeConflict || w.twoLetterCodes {
		return false
	}
	if w.d.Category != pipeline.CatNone || w.d.Confidence != pipeline.ConfNone {
		return false
	}
	if !isCharacterFamily(w.family) {
		return false
	}
	// The unique-index exclusion that stood here is the sweep loop's now (T-0311), so
	// that a column the sweep skips for it says so on its line.
	//
	// T-0253: a validated foreign key no longer excludes cref outright here.
	// unknownColumnsBesideCertain's own fkPairs is what a column at either
	// end of one is checked against, after this gate — raised together with
	// every column paired to it, or not raised at all (see that function's
	// comment, and the exclusion list above it, for why a blanket exclusion
	// leaked instead).
	if n, ok := declaredLength(w.column); ok && n < minUnknownLen {
		return false
	}
	return true
}

// spared is the two skips dogfood session 1 asked unknownColumnsBesideCertain
// for (T-0311), returning the reason fragment that says why the column was not
// swept, or "" when nothing spares it. The column has already passed
// raisableUnknown, is not under a unique index (the third skip, older than
// T-0311 and silent until it, which the caller prints itself ahead of
// fkPairs, where it always ran), and fkPairs has agreed its partners could be
// raised with it:
//
//   - Every sample one identifier shape (spare.go's identifierShapes: uuid,
//     hex digest, semantic version, hostname, path), the hostname and path
//     shapes only when no value carries a dictionary name or a
//     special-category term.
//   - An enumeration: at least enumMinSamples non-NULL samples, at most
//     enumMaxDistinct distinct values, each seen at least twice, every value
//     an ASCII token with no whitespace, and none a dictionary name, a
//     special-category term or a gender term.
//
// Neither moves the decision: the column stays at CatNone and is
// copied, exactly as a column in a table with no certain neighbour is, and the
// line it gets beside "nothing recognised in N samples" is the account of why
// the neighbour did not change that. THREAT_MODEL.md T1's T-0311 amendment is
// the measurement and the residual.
func (st *state) spared(w *work, table ref.TableRef, certain int) string {
	switch {
	case w.spare.identifier != shapeNothing:
		return render("spared_identifier", quoteTable(table), certain, w.spare.total, w.spare.identifier)
	case w.spare.enum:
		return render("spared_enum", quoteTable(table), certain,
			w.spare.distinct, w.spare.total, enumMaxDistinct, enumMinSamples)
	}
	return ""
}

// isCharacterFamily is the set free_text's masker can write into: rules.yml's
// own accepts: list for the category.
func isCharacterFamily(family string) bool {
	switch family {
	case famText, famVarchar, famBpchar, famCitext:
		return true
	}
	return false
}

// declaredLength is the length of a varchar(n) or char(n), from the type
// modifier the catalog recorded. A type with no modifier reports false.
func declaredLength(col pipeline.Column) (int, bool) {
	if col.TypMod <= 4 {
		return 0, false
	}
	return int(col.TypMod) - 4, true
}

// raisable reports whether a pass may raise a decision. A type-conflicting name
// hit is never raised above low (ARCHITECTURE.md §4), a never-masked column is
// never raised at all, and a decision with no category has nothing to raise.
func (st *state) raisable(w *work) bool {
	return !w.typeConflict && !w.neverMask && w.d.Category != pipeline.CatNone
}

// ---------- pass 3b: guessed-region phone corroboration (T-0221) ----------

// guessedPhoneColumns is the "no --phone-region configured" half of T-0221.
// base (above) already asked, per column, whether its values parse as a phone
// number under some region drawn from phoneGuessRegions (guessedPhoneHit);
// this pass decides whether that guess is trusted enough to mask on, and it
// is trusted only with corroboration: a proven personal neighbour sits in
// the same table (Decision.TableHasLikelyPersonalColumn, which
// neighbouringColumns just filled for every column -- the same signal
// internal/verify's national_id digits entry reads for its own
// requiresCorroboration gate, T-0187's third review round, and the pattern
// this task's brief names).
//
// An earlier landing also tried to corroborate off the column's own name
// matching rules.yml's phone pattern, but that arm could never fire: this
// pass only ever sees a column whose confidence is still below ConfPossible
// (base only sets w.guessedPhone under that same guard), and a name that
// matches rules.yml's phone pattern on a character family is always
// nameAccepted (rules.yml's phone entry accepts exactly
// text/varchar/bpchar/citext, the family isCharacterFamily restricts this
// whole guessed-region feature to) -- so decide's hasName && nameAccepted
// branch had already raised the column to ConfPossible or above before this
// pass could ever see it with guessedPhone set. A named safety gate no input
// can reach is worse than no gate, so the name-match arm and
// work.nameMatchedPhone were removed (T-0221 review round, finding 3)
// instead of kept as documentation of a corroboration path that does not
// exist. Without a personal neighbour, the guess decides nothing and the
// column is left exactly as base's earlier passes left it. A ten-digit
// account number or an order code
// clears one of phoneGuessRegions' fifteen numbering plans often enough by
// chance -- the same shape of false positive internal/verify/validators.go's
// own requiresCorroboration comment measures for a sparse, fixed-prefix
// national_id digits column -- so masking on the guess alone here would be
// the row-path version of the leak that gate exists to prevent on the loaded
// target.
//
// It is called from inside neighbouringColumns, after the loop that fills
// Decision.TableHasLikelyPersonalColumn for every column (not only the ones
// that pass goes on to raise) and before unknownColumnsBesideCertain, which
// would otherwise sweep the same unsignalled character column into the
// generic free_text catch-all first — see neighbouringColumns' own comment
// on the ordering. Running before keyChildren/foreignKeys/sameColumnName
// makes a column this pass masks visible to every later pass exactly as one
// neighbouringColumns masked would be.
func (st *state) guessedPhoneColumns() {
	for _, col := range st.order {
		w := st.dec[col]
		if w == nil || w.guessedPhone == nil || !st.phoneGuessRaisable(w) {
			continue
		}
		if !w.d.TableHasLikelyPersonalColumn {
			continue
		}
		hit := w.guessedPhone
		w.d.Category = pipeline.CatPhone
		w.d.Confidence = pipeline.ConfPossible
		// T-0197 review finding 2: this pass raises a column decide() already
		// described as "nothing recognised" or "sub-threshold" -- that claim
		// was true when decide() wrote it (guessedPhoneHit is computed after
		// decide runs, in base()) but is false now that the column is being
		// masked on a 5/5 (or better) region-guessed hit. Blank it exactly as
		// keyFrag's exemption line is blanked when a later pass changes a
		// column's story, so the reason line does not assert both claims.
		if w.noSignalFrag >= 0 {
			w.frags[w.noSignalFrag] = ""
			w.noSignalFrag = -1
		}
		w.frags = append(w.frags,
			render("samples", hit.matched, hit.total, hit.phrase),
			render("phone_region_guessed"))
	}
}

// phoneGuessRaisable is guessedPhoneColumns' own gate, kept apart from
// raisable above because that one requires an existing category
// (w.d.Category != CatNone) -- a guessed-region hit routinely starts from
// CatNone, exactly as unknownColumnsBesideCertain's column does, so this
// checks the never-mask and type-conflict guards on their own instead.
func (st *state) phoneGuessRaisable(w *work) bool {
	return !w.neverMask && !w.typeConflict && w.d.Confidence < pipeline.ConfPossible
}

// ---------- pass 4: FK propagation and shared column names ----------

// keyChildren is the child-to-parent half of the key exemption, and it is
// tracker T-0120's reconciliation.
//
// markNeverMasked scopes the exemption to a key column whose *own* signals
// stayed below the mask threshold, so an integer or uuid FK child that reaches
// `possible` on a name hit alone loses it while the primary key it references —
// the same values, no name hit — keeps it. That is the one shape where the two
// ends of one edge disagree and propagation cannot see it: the child is masked,
// the parent is copied verbatim, the load adds the edge NOT VALID and
// internal/verify/fk.go counts the orphans and fails the run at exit 8
// (THREAT_MODEL.md T8). It protects nothing either — a FK child's values are a
// subset of the parent key's by definition, and the parent shipped them in the
// clear — so masking this end alone costs the join and buys no confidentiality.
// ARCHITECTURE.md §4's own wording is the child's side here: the exemption is
// for "surrogate keys (id bigint and the FK columns that reference them)", with
// no condition on what the child column happens to be *called*.
//
// So a key-family child whose parent end is an exempt surrogate key takes the
// exemption back, and says which column vouched for it. The direction §4 states
// is untouched and still wins: a **masked** parent overrides the child, which is
// why this runs before propagateKeys rather than after — a column with two
// parents, one exempt and one masked, ends masked, and propagateKeys lifts and
// blanks what this grants through the same keyFrag it sets.
//
// The gate on the parent is its own key fragment (keyFrag), not "the parent is
// unmasked". A generated parent is also neverMask and is not a surrogate key:
// its column is recomputed by the target from columns that may themselves be
// masked, so its values are not evidence that the child's are copied anywhere.
// The edge has to be a checked one for the same reason (see the sweep below).
//
// It sweeps for the same reason foreignKeys does: a chain of exempt keys is one
// lift per edge and Schema.FKs is in no order relative to the chain.
func (st *state) keyChildren() {
	for round := 0; round <= len(st.schema.FKs); round++ {
		if !st.reconcileKeyChildren() {
			return
		}
	}
}

// reconcileKeyChildren is one sweep. It reports whether it changed anything.
func (st *state) reconcileKeyChildren() bool {
	changed := false
	for _, fk := range st.schema.FKs {
		// The whole argument is "the child's values are a subset of the
		// parent's", and only Postgres saying so makes that true. An
		// unvalidated constraint is a hint over rows it never checked
		// (pipeline.ForeignKey.Validated) and a virtual one is a line in the
		// yml, so a child of either can hold a value the parent does not, and
		// internal/plan does not even follow it to fetch the parent row
		// (followsAsParent). Propagation runs over every edge because it only
		// ever masks more; this pass masks less, so it takes the narrow set.
		if !fk.Validated || fk.Virtual {
			continue
		}
		for i, childName := range fk.ChildCols {
			if i >= len(fk.ParentCols) {
				break
			}
			parent := ref.ColumnRef{Table: fk.Parent, Column: fk.ParentCols[i]}
			child := ref.ColumnRef{Table: fk.Child, Column: childName}
			pw, cw := st.dec[parent], st.dec[child]
			if pw == nil || cw == nil {
				continue
			}
			// keyFrag is written on the surrogate-key and fk-column paths of
			// markNeverMasked and on no other, so it is the exact question
			// "is the referenced column an exempt key".
			if !pw.neverMask || pw.keyFrag < 0 {
				continue
			}
			// A child that is already exempt has nothing to take back, and one
			// below the threshold would not have been masked anyway.
			if cw.neverMask || cw.d.Confidence < pipeline.ConfPossible {
				continue
			}
			// The same type gate markNeverMasked applies: a text key is not a
			// surrogate key, and this may not exempt one.
			if !isKeyFamily(cw.family) || cw.array {
				continue
			}
			cw.neverMask = true
			cw.keyFrag = len(cw.frags)
			cw.frags = append(cw.frags, render("key_child_exempt", quoteColumn(parent)))
			changed = true
		}
	}
	return changed
}

// foreignKeys is ARCHITECTURE.md §4's propagation sentence: "a masked PK or
// unique column's decision overrides the decision on every column referencing
// it".
//
// It sweeps until nothing moves. A chain of keys — a natural key, a child that
// references it, a grandchild that references the child — is one propagation per
// edge, and Schema.FKs is in no order relative to that chain, so a single sweep
// would leave the grandchild verbatim whenever the edges arrived in the wrong
// one. Every sweep either changes a decision or is the last, so the number of
// edges bounds the work; the cap also terminates the one shape that could
// otherwise oscillate, a cycle of masked keys that disagree about category.
func (st *state) foreignKeys() {
	for round := 0; round <= len(st.schema.FKs); round++ {
		if !st.propagateKeys() {
			return
		}
	}
}

// propagateKeys is one sweep. It reports whether it changed anything.
func (st *state) propagateKeys() bool {
	changed := false
	for _, fk := range st.schema.FKs {
		for i, childName := range fk.ChildCols {
			if i >= len(fk.ParentCols) {
				break
			}
			parent := ref.ColumnRef{Table: fk.Parent, Column: fk.ParentCols[i]}
			child := ref.ColumnRef{Table: fk.Child, Column: childName}
			pw, cw := st.dec[parent], st.dec[child]
			if pw == nil || cw == nil || !st.pkOrUnique[parent] {
				continue
			}
			if pw.d.Confidence < pipeline.ConfPossible || pw.neverMask || pw.d.Category == pipeline.CatNone {
				continue
			}
			// A generated column is the one exemption propagation may not lift:
			// it is not copied at all, so there is nothing here to mask.
			// Everything else — the ordinary surrogate/FK key exemption and, as
			// of the T-0314 review round, the framework-metadata one — is a
			// statement about the column's own signals (markNeverMasked) that
			// the parent has just contradicted: the child holds the very values
			// the parent is being masked for. Leaving either exempt is the
			// recall hole this sentence of §4 exists to close — the personal
			// value ships in cleartext on the child side under exit 0
			// (THREAT_MODEL.md T1) — and it breaks the join as well, because
			// the parent's values are replaced and the child's are not (T8). A
			// framework metadata table is still copied whole as a lookup
			// regardless of what this function decides (internal/plan's own
			// T-0314 entry), and a real one — Rails, Django, Flyway and the
			// rest of pipeline.IsFrameworkMetadataTable's list — never carries
			// a foreign key at all, so lifting the exemption here never costs a
			// genuine framework table anything; what it closes is a same-named
			// application table that does (w.frameworkMetadata's own doc
			// comment).
			if cw.generated {
				continue
			}
			if !st.pack.accepted(pw.d.Category, cw.family) {
				// The two ends of the key are different enough that the parent's
				// masker cannot run on the child. Say so on the child's line
				// rather than propagating a category that would produce a value
				// its column cannot hold.
				if !cw.propagationRefused {
					cw.propagationRefused = true
					cw.frags = append(cw.frags, render("type_conflict", cw.family, string(pw.d.Category)))
					changed = true
				}
				continue
			}
			// The two ends of one key converge on the parent's decision, not on
			// whichever end scored higher: two categories across one edge are two
			// maskers over one set of values, which is a broken join at load as
			// surely as a missing one. The confidence only ever rises.
			if cw.d.Category == pw.d.Category && cw.d.Confidence >= pw.d.Confidence && !cw.neverMask {
				continue
			}
			cw.neverMask = false
			cw.typeConflict = false
			cw.frameworkMetadata = false
			// T-0312 review round: this branch is about to give cw a category
			// backed by real evidence -- the parent's own decision, propagated
			// across a validated edge -- which is exactly the kind of signal
			// sweptNoSignal exists to distinguish a sweep's guess from. Leaving
			// the bit set here would keep sameColumnName from treating this
			// now-evidenced decision as a source, which is a T1 recall loss the
			// goal never asked for: a same-named column elsewhere loses the
			// propagation this decision should now be allowed to make.
			cw.sweptNoSignal = false
			if cw.keyFrag >= 0 {
				cw.frags[cw.keyFrag] = ""
				cw.keyFrag = -1
			}
			if cw.frameworkMetadataFrag >= 0 {
				cw.frags[cw.frameworkMetadataFrag] = ""
				cw.frameworkMetadataFrag = -1
			}
			cw.d.Category = pw.d.Category
			if cw.d.Confidence < pw.d.Confidence {
				cw.d.Confidence = pw.d.Confidence
			}
			cw.d.Source = pipeline.ByFKPropagation
			cw.frags = append(cw.frags, render("fk_propagation", quoteIdent(fk.Name), quoteColumn(parent)))
			changed = true
		}
	}
	return changed
}

// sameColumnName is the second half of ARCHITECTURE.md §4's propagation
// sentence: "identical column names across tables share a category".
func (st *state) sameColumnName() {
	type source struct {
		cat   pipeline.Category
		table ref.TableRef
		col   string
	}
	masked := map[string]source{}
	for _, c := range st.order {
		w := st.dec[c]
		if w.d.Confidence < pipeline.ConfPossible || w.neverMask || w.d.Category == pipeline.CatNone {
			continue
		}
		// T-0312: a decision unknownColumnsBesideCertain reached on no name
		// or value signal of its own -- only a certain neighbour sitting
		// beside it in *this* table -- carries nothing that speaks to a
		// same-named column in a different table, so it is not a source
		// here. Skipping it here, rather than at the consuming loop below,
		// also keeps it out of shadowing an earlier, evidenced source: two
		// columns sharing a name where the first is swept and the second has
		// a real hit must still record the second.
		if w.sweptNoSignal {
			continue
		}
		key := normaliseName(c.Column)
		if _, seen := masked[key]; !seen {
			masked[key] = source{cat: w.d.Category, table: c.Table, col: c.Column}
		}
	}
	for _, c := range st.order {
		w := st.dec[c]
		if w.neverMask || w.typeConflict || w.d.Confidence >= pipeline.ConfPossible {
			continue
		}
		// T-0313: a bare name its own samples did not corroborate is not
		// raised by a same-named column elsewhere (work.nameUncorroborated).
		if w.nameUncorroborated {
			continue
		}
		s, ok := masked[normaliseName(c.Column)]
		if !ok || s.table == c.Table {
			continue
		}
		if w.d.Category != pipeline.CatNone && w.d.Category != s.cat {
			continue
		}
		if !st.pack.accepted(s.cat, w.family) {
			continue
		}
		w.d.Category = s.cat
		w.d.Confidence = pipeline.ConfPossible
		w.d.Source = pipeline.ByFKPropagation
		w.frags = append(w.frags, render("same_name", quoteIdent(s.col), string(s.cat), quoteTable(s.table)))
	}
}

// ---------- pass 5: the committed lazyslice.yml ----------

// applyPrior folds in the yml. It can only tighten: a pattern or a column entry
// may add a category or raise a confidence and can never lower or remove one
// (ADR-004, THREAT_MODEL.md T3), and only when the raise leaves a category the
// column's type family accepts (raiseFromConfig). The one thing it may loosen
// is a per-column opt-out, which is recorded with a reason and with the
// column's type fingerprint, and is ignored when either is missing or the
// fingerprint has moved (honourOptOut).
//
// A generated column or an ordinary surrogate/FK key is still off limits to
// both loops below — neverMask stands as markNeverMasked or keyChildren left
// it, because there is nothing a generated column's raise could mask and an
// unlifted key exemption is a statement about the column's own signals a
// caller's file has not contradicted. A framework-metadata exemption
// (w.frameworkMetadata) is the one w.neverMask a raise here may still lift
// (T-0314 review round, finding 3): an operator naming this exact column in a
// pattern or a Columns entry has spoken about it more specifically than
// pipeline.IsFrameworkMetadataTable's bare-name match did, and a committed yml
// that already recorded the column as masked — from a run before this
// exemption existed, or naming a real column a same-named application table
// carries — must not read back unmasked, which ADR-004's "read-back only
// tightens" forbids. Both loops leave the actual lifting to raiseFromConfig
// (liftFrameworkMetadataExemption, called from its success branch only), so a
// raise this package is about to refuse for lack of a usable category never
// unmasks the column's default exemption for nothing.
func (st *state) applyPrior(prior *pipeline.Config) (*pipeline.Classification, error) {
	cls := &pipeline.Classification{}
	if prior == nil {
		return cls, nil
	}
	for _, pat := range prior.ExtraPatterns {
		re, err := regexp.Compile(pat.Name)
		if err != nil {
			return nil, fmt.Errorf("classify: lazyslice.yml pattern: %w", err)
		}
		for _, c := range st.order {
			w := st.dec[c]
			if w.neverMask && !w.frameworkMetadata {
				continue
			}
			// pipeline.Pattern.Name is "regex over column or table name", so a
			// user who writes a pattern for `patients` to raise a whole table
			// gets the table, not silence.
			if !re.MatchString(normaliseName(c.Column)) && !re.MatchString(normaliseName(c.Table.Name)) {
				continue
			}
			if pat.Confidence <= w.d.Confidence {
				continue
			}
			st.raiseFromConfig(w, pat.Category, pat.Confidence, "yml_raise")
		}
	}
	for _, c := range st.order {
		w := st.dec[c]
		cc, ok := prior.Columns[c]
		if !ok {
			cls.Drift = append(cls.Drift, c)
			continue
		}
		if cc.Confidence > w.d.Confidence && (!w.neverMask || w.frameworkMetadata) {
			st.raiseFromConfig(w, cc.Category, cc.Confidence, "yml_column")
		}
		if cc.Unmask == nil {
			continue
		}
		if !honourOptOut(cc.Unmask, w.column.Fingerprint) {
			cls.Expired = append(cls.Expired, c)
			continue
		}
		w.unmasked = true
		if cc.Unmask.By == "flag" {
			w.d.Source = pipeline.ByFlagUnmask
			w.frags = append(w.frags, render("unmask_flag"))
		} else {
			w.d.Source = pipeline.ByYmlUnmask
			w.frags = append(w.frags, render("unmask_yml"))
		}
	}
	sort.Slice(cls.Drift, func(i, j int) bool { return cls.Drift[i].Less(cls.Drift[j]) })
	sort.Slice(cls.Expired, func(i, j int) bool { return cls.Expired[i].Less(cls.Expired[j]) })
	return cls, nil
}

// liftFrameworkMetadataExemption clears the one w.neverMask a yml raise is
// allowed to lift (applyPrior's own doc comment) and blanks the "framework
// metadata table" fragment it recorded, the same bookkeeping propagateKeys
// does for the identical field over an FK edge instead of a yml entry.
// raiseFromConfig is the one caller, and only from its success branch, so a
// raise that is about to be refused (yml_no_category) never unmasks the
// column's default exemption for nothing. A no-op on any column that is not
// currently exempt for this reason.
func liftFrameworkMetadataExemption(w *work) {
	if !w.frameworkMetadata {
		return
	}
	w.neverMask = false
	w.frameworkMetadata = false
	if w.frameworkMetadataFrag >= 0 {
		w.frags[w.frameworkMetadataFrag] = ""
		w.frameworkMetadataFrag = -1
	}
}

// raiseFromConfig applies one yml raise, which is the only place a caller's
// file can move a decision up.
//
// Two things it will not do. It will not take a category the column's type
// family refuses — st.pack.accepted is the same gate the name signal passes
// through (ARCHITECTURE.md §4 "Accepted types per category"), so a yml entry
// naming semi_structured on a text column cannot put a JSON masker on it — and
// it will not raise a decision that would end with no category at all: Masker
// is chosen from the category at finalise, so "masked, category none" is a
// column transform is told to mask with nothing to mask it with (ADR-006).
// Both refusals are written into the column's reason rather than dropped, so
// the §4 explanation line says that the file did not take effect here.
//
// The gate is on the category the raise would *leave*, not only on the one the
// yml named. A file that supplies a confidence and no category at all raises
// whatever the classifier already recorded, and that can be a name hit its type
// refuses — people.email_verified is `email` at low on a boolean — which §4 pins
// at low and raisable() keeps every other pass off. Checking only the incoming
// category would make "confidence: certain" with no category the one route to
// the email masker on a boolean column.
//
// It is also where a framework-metadata exemption is lifted (T-0314 review
// round, finding 3): liftFrameworkMetadataExemption runs only once the checks
// above have confirmed the raise leaves a usable category, so a raise about to
// be refused (yml_no_category) never clears the column's default exemption for
// nothing — a caller in applyPrior may call this whether or not the column is
// currently framework-metadata-exempt, and only a raise that actually applies
// changes that.
func (st *state) raiseFromConfig(w *work, cat pipeline.Category, conf pipeline.Confidence, frag string) {
	if cat != "" && cat != pipeline.CatNone {
		if st.pack.accepted(cat, w.family) {
			w.d.Category = cat
			// The standing conflict, if there was one, is about the category
			// that has just been replaced.
			w.typeConflict = false
		} else {
			w.frags = append(w.frags, render("type_conflict", w.family, string(cat)))
		}
	}
	if w.d.Category == pipeline.CatNone || w.typeConflict || !st.pack.accepted(w.d.Category, w.family) {
		w.frags = append(w.frags, render("yml_no_category"))
		return
	}
	liftFrameworkMetadataExemption(w)
	w.d.Confidence = conf
	w.d.Source = pipeline.ByYmlRaise
	w.frags = append(w.frags, render(frag))
}

// honourOptOut reports whether a per-column opt-out still stands.
//
// It fails closed. THREAT_MODEL.md T3's v1-blocking control is that "an opt-out
// records the column's type fingerprint and is ignored when it changes", so an
// opt-out that records no fingerprint at all is the fail-open version of that
// control: nothing could ever revoke it. A hand-written or older lazyslice.yml
// carrying `unmask: {}` is therefore ignored, and so is one with no reason,
// which ARCHITECTURE.md §2 says can never happen and §8 makes exit 2 at the
// flag. The one opt-out with no fingerprint that is honoured is the --unmask
// flag's, which is made for this run and dies with it; that is a branch on By
// rather than on the fingerprint being absent.
func honourOptOut(u *pipeline.Unmask, fingerprint string) bool {
	switch {
	case u.Reason == "":
		return false
	case u.TypeFP == "":
		return u.By == "flag"
	default:
		return u.TypeFP == fingerprint
	}
}

// ---------- pass 6: the threshold ----------

// reRoleGiven and reRoleFamily are Decision.Role's source for a person_name
// column (T-0287, mask.Role): the column's own name, matched the way
// rules.yml's patterns are -- against normaliseName's output, which lower-cases
// and splits camelCase and digit runs onto underscore boundaries. They are
// deliberately narrower than rules.yml's own person_name pattern, which is
// multilingual and matches many more words than these two care about: a role is
// a bare English word away from the masker's original "Given Family" behaviour
// (mask.RoleFull), never a reason to mask a column this pattern set misses, so
// guessing wrong here costs nothing worse than the full name every person_name
// column already emitted before this field existed.
// The optional, non-capturing "(_?names?)?" lets a bare root match the same
// way whether or not "name" is spelled onto it -- "first", "first_name" and
// "firstname" all decide RoleGiven, the way rules.yml's own person_name
// pattern already treats an underscore before "name" as optional.
var (
	reRoleGiven  = regexp.MustCompile(`(^|_)(first|given|forename|fname)(_?names?)?(_|$)`)
	reRoleFamily = regexp.MustCompile(`(^|_)(last|family|surname|lname)(_?names?)?(_|$)`)
)

// roleForColumn is Decision.Role's whole implementation: first/given/forename/
// fname decide RoleGiven, last/family/surname/lname decide RoleFamily, and
// everything else -- "name", "full_name", a column no pattern here recognises
// -- decides mask.RoleFull, the masker's unchanged default.
func roleForColumn(name string) mask.Role {
	n := normaliseName(name)
	switch {
	case reRoleGiven.MatchString(n):
		return mask.RoleGiven
	case reRoleFamily.MatchString(n):
		return mask.RoleFamily
	default:
		return mask.RoleFull
	}
}

// finalise applies ARCHITECTURE.md §4's threshold and fills the fields the plan
// and the emitter read.
func (st *state) finalise() {
	for _, c := range st.order {
		w := st.dec[c]
		w.d.Reason = joinReason(w.frags...)
		w.d.TypeFP = w.column.Fingerprint
		w.d.UniqueIndex = st.unique[c]
		if w.d.Category == pipeline.CatPersonName {
			// Read from the column's name at classify time, the way every other
			// name-pattern decision is, and carried on the Decision beside
			// Category whether or not the column ends up masked below -- the
			// same footing TypeFP and UniqueIndex are already on.
			w.d.Role = roleForColumn(w.column.Name)
		}
		// NeverMasked is w.neverMask's final value, after keyChildren and
		// foreignKeys have both run (T-0240 review round, high finding):
		// internal/verify's second net reads it as the unconditional half of
		// its dense-sequence exemption, so it has to be the same answer
		// w.neverMask settles on once the exemption can be lifted (a masked
		// parent) as well as granted or handed back -- never an earlier,
		// mid-pass snapshot.
		w.d.NeverMasked = w.neverMask
		w.d.Masked = w.d.Confidence >= pipeline.ConfPossible && !w.neverMask && !w.unmasked
		if w.d.Masked && w.d.Category != pipeline.CatNone {
			w.d.Masker = st.pack.Masker[w.d.Category]
		}
	}
	st.raiseCompositeUnique()
}

// raiseCompositeUnique is the composite half of Decision.UniqueIndex, and it
// runs after the threshold because it is the only part of the answer that
// depends on which columns ended up masked.
//
// indexKeys deliberately raises UniqueIndex only on a column that carries the
// uniqueness alone (see there). That is right for a composite index with an
// unmasked column in it — ActiveStorage's (record_type, record_id, name,
// blob_id), where the two ids are surrogate keys copied verbatim and hold the
// tuple apart whatever `name` masks to. It is wrong when *every* key column of
// the index is masked, because then nothing is left to hold the tuple apart:
// Django's `django_content_type` is UNIQUE (app_label, model) with both columns
// masked, and the load died on
// `django_content_type_app_label_model_76bd3d3b_uniq`
// (testdata/regressions/004-composite-unique-index-all-masked.sql).
//
// A **partial** or **expression** composite index is in, for the same reason
// indexKeys keeps a partial single-column one: it says something weaker than a
// total index — every row *the predicate admits* holds a distinct tuple — but it
// still says something, and nothing else raises it. Excluding the two shapes was
// a strict loss against the code this replaced, which raised every column of
// every unique index: the torture schemas alone carry 85 composite partial
// unique indexes, several of them over a masked column (calcom's
// `Watchlist_type_value_global_key ON public."Watchlist" (type, value) WHERE
// "organizationId" IS NULL`, discourse's `topic_custom_fields (topic_id, value)`
// partials), and dropping them puts the collision back in the loader after every
// row has moved. An expression index's columns are recovered from its definition
// the way indexKeys recovers them, because introspect leaves Index.Columns empty
// for one. d_required is then computed over the whole table's row count, which
// over-estimates a partial index exactly as it does in indexKeys, and for the
// same reason: over-estimating refuses a plan with three printed escapes, and
// under-estimating is a 23505 with none.
//
// The rule it applies is about one question: which of its key columns is what
// makes a row distinct?
//
// Take `distinct` to mean "no value repeats in this column's sample". Then:
//
//  1. If an **unmasked** key column is distinct, nothing is raised. That column
//     is copied verbatim and it holds every row apart on its own, so whatever
//     the masked columns become the tuple stays unique. ActiveStorage's
//     (record_type, record_id, name, blob_id) is this case: `record_id` and
//     `blob_id` are surrogate keys, `name` is the literal 'cover' in every row,
//     and raising `name` refused a run that could not have collided
//     (testdata/regressions/003-composite-unique-index-is-not-a-unique-column.sql).
//
//  2. Otherwise the masked key columns that are distinct are raised, because one
//     of them has to be the discriminator. Django's `auth_permission`
//     (content_type_id, codename) is this case: `content_type_id` is a foreign
//     key with forty values across two hundred permissions, so `codename` is
//     what makes the row unique, it is masked, and not raising it loaded two
//     hundred rows and died on
//     `auth_permission_content_type_id_codename_01ab375a_uniq`.
//
//  3. If no key column is distinct at all, every masked one is raised. Nothing
//     in the sample says which column carries the uniqueness, and the direction
//     that fails safe is to hold each of them to d_required.
//
// Rule 1 has a condition on it, and it is the difference between two absences of
// evidence that look identical in the sample. An unmasked key column with **no
// scalar samples** is either (a) NULL in every sampled row, or (b) in a table
// nothing could be sampled from — introspect tolerates a relation the role may
// not read, and a partitioned table with no leaves has nothing to read. In case
// (a) the tuple genuinely cannot collide: a NULL key component is never equal to
// anything under a plain unique index, so `Role_name_teamId_key` over calcom's
// three roles, whose `teamId` is NULL in all of them, holds however `name` is
// masked. In case (b) nothing at all is known, and counting the column as
// distinct suppresses the raise on no evidence — which is the fail-open this
// used to be. The two are told apart by whether **any** column of the table
// produced a sample (`tableWasSampled`), and case (a) is withdrawn for an index
// declared `NULLS NOT DISTINCT`, where a NULL does collide with a NULL
// (`nullsAreDistinct`, PostgreSQL 15 and later).
//
// All three are approximations of a rule ARCHITECTURE.md §5 does not state —
// §5's d_required = n²/2ε is written about a column, never about a tuple — and
// each errs towards raising, because raising wrongly costs a plan refusal with
// three printed escapes and not raising costs a loader that dies with rows
// already moved. What none of them covers is a tuple whose columns all repeat
// and whose masking merges two groups at once. **ADR-011 clause (a) is the
// accepted decision for this rule** (2026-09-08, from T-0099's text), and
// ARCHITECTURE.md §5 states it; the exact rule needs the largest-agreeing-group
// statistic nothing collects, which is that ADR's reversal condition.
//
// The sample is the classifier's own (`state.samples`, the same flattening the
// validators read), so this asks the source nothing it was not already asked.
func (st *state) raiseCompositeUnique() {
	for _, t := range st.schema.Tables {
		for _, idx := range t.Indexes {
			if !idx.Unique {
				continue
			}
			cols := idx.Columns
			if idx.Expression {
				cols = expressionColumns(t, idx)
			}
			if len(cols) < 2 {
				continue
			}
			// Whether the *table* produced any samples at all is what tells the
			// two absences of evidence apart on an unmasked key column. See
			// `nullsAreDistinct` and the rule 1 paragraph above.
			sampledTable := st.tableWasSampled(t)
			nullsDistinct := nullsAreDistinct(idx)
			var maskedDistinct, maskedRepeating []*work
			heldApart := false
			for _, name := range cols {
				c := ref.ColumnRef{Table: t.Ref, Column: name}
				w, ok := st.dec[c]
				distinct, sampled := st.sampleDistinct(c)
				switch {
				case !ok || !w.d.Masked:
					heldApart = heldApart || (distinct && (sampled || (sampledTable && nullsDistinct)))
				case distinct:
					maskedDistinct = append(maskedDistinct, w)
				default:
					maskedRepeating = append(maskedRepeating, w)
				}
			}
			raise := maskedDistinct
			if len(raise) == 0 {
				raise = maskedRepeating
			}
			if heldApart {
				continue
			}
			for _, w := range raise {
				w.d.UniqueIndex = true
			}
		}
	}
}

// rawSamples flattens a column's samples without the array-literal split
// scalarsOf performs for the validators (tracker T-0103). The two callers below
// ask about the *value* in the column -- does it repeat, was there one at all --
// and a unique index is over the whole array, not over the strings inside it.
func (st *state) rawSamples(c ref.ColumnRef) []string {
	return st.samples(c, columnType{})
}

// tableWasSampled reports that at least one column of the table produced a
// sample value. It is how raiseCompositeUnique tells "this column is NULL in
// every row" from "nothing about this table could be read at all", which look
// the same from one column and mean opposite things.
func (st *state) tableWasSampled(t pipeline.Table) bool {
	for _, col := range t.Columns {
		if len(st.rawSamples(ref.ColumnRef{Table: t.Ref, Column: col.Name})) > 0 {
			return true
		}
	}
	return false
}

// nullsAreDistinct reports PostgreSQL's default for a unique index: a NULL is
// never equal to another NULL, so a tuple with a NULL key component conflicts
// with nothing. PostgreSQL 15 added the opposite, spelled in the definition the
// target recreates the index from, and there a NULL does collide.
func nullsAreDistinct(idx pipeline.Index) bool {
	return !strings.Contains(strings.ToUpper(idx.Def), "NULLS NOT DISTINCT")
}

// sampleDistinct reports whether every sampled value of the column is different
// from every other, and — separately — whether there were any samples at all.
//
// The two answers are separate because the caller reads them in opposite
// directions. A column with no samples counts as distinct, which is the
// fail-closed answer for a *masked* key column: no evidence that it repeats is
// not evidence that it does not, and being wrong that way costs a refusal the
// operator can lift with --unmask rather than a collision in the loader. For an
// *unmasked* key column the fail-closed answer is the other one — an unsampled
// column must not be taken to hold the tuple apart — so raiseCompositeUnique
// requires `sampled` there. Returning one boolean made the second case read as
// the first, which is why this returns both.
func (st *state) sampleDistinct(c ref.ColumnRef) (distinct, sampled bool) {
	values := st.rawSamples(c)
	seen := make(map[string]bool, len(values))
	for _, v := range values {
		if seen[v] {
			return false, true
		}
		seen[v] = true
	}
	return true, len(values) > 0
}

// fingerprint is sha256 over the rule-pack version and, per column, the
// category and the masker, hex-truncated to 16 characters (ARCHITECTURE.md §5
// "Determinism scope"). A column that moves category through drift, the
// neighbouring-column rule or an extra_patterns raise changes it, which is what
// makes "classification changed — masked values will differ" printable rather
// than silent.
func fingerprintOf(version string, decisions map[ref.ColumnRef]pipeline.Decision) string {
	cols := make([]ref.ColumnRef, 0, len(decisions))
	for c := range decisions {
		cols = append(cols, c)
	}
	sort.Slice(cols, func(i, j int) bool { return cols[i].Less(cols[j]) })
	var buf []byte
	buf = appendField(buf, "rulepack")
	buf = appendField(buf, version)
	for _, c := range cols {
		d := decisions[c]
		buf = appendField(buf, c.String())
		buf = appendField(buf, string(d.Category))
		buf = appendField(buf, string(d.Masker))
	}
	sum := sha256.Sum256(buf)
	return hex.EncodeToString(sum[:])[:16]
}

// Refingerprint recomputes Classification.Fingerprint over the decisions as
// they stand now, rather than as Classify left them (T-0101).
//
// ARCHITECTURE.md §5 defines the fingerprint as sha256 over the rule-pack
// version and, per column, its category and its masker; §11.2 prints
// "classification changed -- masked values will differ" when a marked target's
// recorded one differs from this run's. But the masker on a column under a
// unique index is not settled until the plan has run: §5's own domain rule says
// "the plan picks, within the column's category, the registered generator with
// the largest Domain()", and internal/plan/unique.go writes that pick back onto
// the decision -- which internal/transform then masks with and internal/emit
// writes into the yml. Computed inside Classify alone, the fingerprint covered
// the category and the *default* masker, so a column that became unique between
// two runs -- a new unique index, or a --take that pushed it past d_required --
// changed every masked value in it and changed no fingerprint, and the warning
// §11.2 exists for did not print.
//
// The pick needs the planned row count, which internal/classify does not have
// and cannot get (it reads samples, not a database), so it cannot move here.
// The fingerprint moves instead: internal/core calls this after the plan, and
// the fingerprint it records is the classification *plus* the plan's picks.
// This is the same reason internal/core/domain.go writes Domain and SmallDomain
// after Classify returns.
//
// Nothing else changes. With no escalation the decisions are the ones Classify
// hashed and the value is byte-identical, so a run over a schema with no unique
// masked column has exactly the fingerprint it had before.
func Refingerprint(cls *pipeline.Classification) (string, error) {
	if cls == nil {
		return "", nil
	}
	p, err := pack()
	if err != nil {
		return "", err
	}
	return fingerprintOf(p.Version, cls.Decisions), nil
}

// appendField is a length-prefixed encoding, for the same reason mask.Encode is
// one: without it a column b.c in schema a would hash like column c in schema
// a.b (ARCHITECTURE.md §5). The length is written in decimal followed by a
// colon rather than as four bytes, which is the same claim with no integer
// narrowing in it.
func appendField(dst []byte, s string) []byte {
	dst = strconv.AppendInt(dst, int64(len(s)), 10)
	dst = append(dst, ':')
	return append(dst, s...)
}

// digitsOnlyMAC reports whether a value signal is the MAC validator firing on
// samples with no separator and no hex letter, which is how a plain 12-digit
// number reads to net.ParseMAC (T-0297). The samples themselves are not kept on
// the signal, so the phrase and the category stand for them: the MAC entry is
// the only network_id validator with that phrase.
func digitsOnlyMAC(v *valueSignal) bool {
	return v.cat == pipeline.CatNetworkID && v.phrase == phraseMAC && v.digitsOnly
}

// allDigits reports whether s, trimmed, is one or more ASCII digits.
func allDigits(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}
