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
	// keyFrag is the index in frags of the surrogate-key or FK-column exemption
	// fragment, or -1. Lifting the exemption blanks that fragment, because a
	// line that says both "preserved verbatim" and "propagated through foreign
	// key" describes two different columns.
	keyFrag int
	// propagationRefused records that a masked parent's category was one this
	// column's type family refuses, so that the sweep writes that fragment once.
	propagationRefused bool
	// unmasked is a per-column opt-out that has not expired.
	unmasked bool
	family   string
	table    ref.TableRef
	column   pipeline.Column
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
	st := &state{
		schema:     schema,
		sampler:    s,
		pack:       p,
		dec:        map[ref.ColumnRef]*work{},
		unique:     map[ref.ColumnRef]bool{},
		pkOrUnique: map[ref.ColumnRef]bool{},
	}
	st.indexKeys()
	st.base()
	st.byteaInPersonShapedTable()
	st.neighbouringColumns()
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
// row has moved, and prints none. T-0099 carries the exact rule, along with the
// composite tuple whose columns all repeat.
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
}

// validators is the ordered list ARCHITECTURE.md §4 names. Order is precedence:
// the first one that reaches the threshold decides, so an address that parses as
// an email is an email and a note that mentions a street is prose.
var validators = []struct {
	cat    pipeline.Category
	phrase string
	ok     func(*textsig.Dict, string) bool
}{
	{pipeline.CatEmail, phraseAddresses, func(_ *textsig.Dict, s string) bool { return textsig.ValidEmail(s) }},
	{pipeline.CatFinancial, phraseIBAN, func(_ *textsig.Dict, s string) bool { return textsig.ValidIBAN(s) }},
	{pipeline.CatFinancial, phraseLuhn, func(_ *textsig.Dict, s string) bool { return textsig.ValidLuhn(s) }},
	{pipeline.CatPhone, phraseE164, func(_ *textsig.Dict, s string) bool { return textsig.ValidPhone(s) }},
	{pipeline.CatNetworkID, phraseIP, func(_ *textsig.Dict, s string) bool { return textsig.ValidIP(s) }},
	{pipeline.CatNetworkID, phraseMAC, func(_ *textsig.Dict, s string) bool { return textsig.ValidMAC(s) }},
	{pipeline.CatCredential, phraseSecrets, func(_ *textsig.Dict, s string) bool { return textsig.LooksSecret(s) }},
	{pipeline.CatPersonName, phraseNameDict, func(d *textsig.Dict, s string) bool { return d.LooksLikeName(s) }},
	{pipeline.CatAddress, phraseAddrShape, func(_ *textsig.Dict, s string) bool { return textsig.AddressShape(s) }},
	{pipeline.CatFreeText, phraseProse, func(d *textsig.Dict, s string) bool { return d.Prose(s) }},
}

// signals is what the validators said about one column's samples.
//
// The three are kept apart because ARCHITECTURE.md §4's accepted-types gate
// applies to a value signal exactly as it does to a name signal (T-0054; see
// internal/classify/CLAUDE.md). strong and weak are validators whose category
// the column's type family can hold; refused is one it cannot, recorded so that
// the reason can say why the column was *not* decided on it, and never so that a
// decision can be made from it.
type signals struct {
	strong  *valueSignal
	weak    *valueSignal
	refused *valueSignal
	total   int
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
				d:       pipeline.Decision{Col: cref, Category: pipeline.CatNone, Confidence: pipeline.ConfNone, Source: pipeline.ByClassifier},
				keyFrag: -1,
				family:  ct.Family,
				table:   t.Ref,
				column:  col,
			}
			st.dec[cref] = w
			values := st.samples(cref)
			sig := bestSignal(dict, values, st.pack, ct.Family)
			st.decide(w, col, ct, values, sig)
			st.appendContext(w, t, ct, sig.total)
			st.markNeverMasked(w, t, col, ct)
		}
	}
}

// samples flattens one column's samples into the strings the validators read.
func (st *state) samples(c ref.ColumnRef) []string {
	if st.sampler == nil {
		return nil
	}
	var out []string
	for _, v := range st.sampler.Samples(c) {
		out = append(out, scalars(v)...)
	}
	return out
}

// bestSignal runs the validators over the non-NULL samples and reports the
// first that reaches a threshold on a category the column's type family can
// hold, the first that reaches the strong threshold on one it cannot, and the
// number of values considered.
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
func bestSignal(dict *textsig.Dict, values []string, p *compiledPack, family string) signals {
	sig := signals{total: len(values)}
	if sig.total == 0 {
		return sig
	}
	if isJSONFamily(family) {
		sig.strong = jsonSignal(p, values)
		return sig
	}
	if family == famBytea || family == famTSVector {
		// A bytea sample is arbitrary binary, and asText renders it as a Go
		// string: run through the text validators, a PNG reads as an address
		// and a compressed blob reads as a secret. ARCHITECTURE.md §4 gives
		// bytea its own rule -- "bytea in a person-shaped column is
		// binary_personal and set to NULL" -- and the rule pack's text
		// categories do not accept the family, so a value signal here could
		// only produce a decision the pack itself contradicts. The name rule
		// and byteaInPersonShapedTable are the only routes to a decision on
		// one.
		//
		// A tsvector is here for the same reason and one more: it is decided by
		// its type alone (decide), so no validator's answer about it could
		// change anything.
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
	for _, v := range validators {
		matched := 0
		for _, s := range values {
			if v.ok(dict, s) {
				matched++
			}
		}
		ratio := float64(matched) / float64(sig.total)
		hit := &valueSignal{cat: v.cat, phrase: v.phrase, matched: matched, total: sig.total}
		if silencedByType(p, v.cat, family) {
			// The values look like a category this column cannot hold. It is
			// recorded once, for the reason line, and it decides nothing: the
			// weak threshold is not consulted for a refused category either,
			// because `low` is what the neighbouring-column rule raises and a
			// raise here would put the same unwritable masker on the column by
			// a longer route.
			if ratio >= validatorThreshold && sig.refused == nil {
				sig.refused = hit
			}
			continue
		}
		if ratio >= validatorThreshold {
			sig.strong = hit
			return sig
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
	best := sig.strong
	hit, hasName := st.pack.match(normaliseName(col.Name))
	nameAccepted := hasName && st.pack.accepted(hit.Category, ct.Family)

	switch {
	case hasName && nameAccepted:
		w.d.Category = hit.Category
		w.frags = append(w.frags, render("name_match", quoteIdent(hit.Name)))
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

	case sig.refused != nil:
		// The values validate for a category whose masker emits a value this
		// column's type cannot hold -- a timestamp full of high-entropy text
		// reading as `credential` is the case that took every pagila run down at
		// exit 7 (T-0054). The decision is recorded at `low` with the conflict
		// named, exactly as a type-conflicting *name* hit is, and typeConflict
		// keeps every raising pass off it: `low` is below the mask threshold, so
		// the column is copied and the line says why it was not masked.
		w.d.Category = sig.refused.cat
		w.d.Confidence = pipeline.ConfLow
		w.typeConflict = true
		w.frags = append(w.frags,
			render("samples", sig.refused.matched, sig.refused.total, sig.refused.phrase),
			render("type_conflict", ct.Family, string(sig.refused.cat)),
			render("no_name_signal"))

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
			w.frags = append(w.frags, render("two_letter_codes"), render("no_name_signal"))
			break
		}
		w.frags = append(w.frags, render("no_signal"))
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

// markNeverMasked records the two classes ARCHITECTURE.md §4 never masks and
// always explains. They are excluded from every raising pass below, so that no
// rule can quietly put a generator on a generated column or a join key.
func (st *state) markNeverMasked(w *work, t pipeline.Table, col pipeline.Column, ct columnType) {
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
		for _, col := range t.Columns {
			if w := st.dec[ref.ColumnRef{Table: t.Ref, Column: col.Name}]; w != nil && w.d.Confidence >= pipeline.ConfLikely {
				likely++
			}
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
}

// raisable reports whether a pass may raise a decision. A type-conflicting name
// hit is never raised above low (ARCHITECTURE.md §4), a never-masked column is
// never raised at all, and a decision with no category has nothing to raise.
func (st *state) raisable(w *work) bool {
	return !w.typeConflict && !w.neverMask && w.d.Category != pipeline.CatNone
}

// ---------- pass 4: FK propagation and shared column names ----------

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
			// it is not copied at all, so there is nothing here to mask. The key
			// exemption is lifted, because it was a statement about the column's
			// own signals (markNeverMasked) and the parent has just contradicted
			// it: the child holds the very values the parent is being masked for.
			// Leaving it exempt is the recall hole this sentence of §4 exists to
			// close — the personal value ships in cleartext on the child side
			// under exit 0 (THREAT_MODEL.md T1) — and it breaks the join as well,
			// because the parent's values are replaced and the child's are not
			// (T8).
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
			if cw.keyFrag >= 0 {
				cw.frags[cw.keyFrag] = ""
				cw.keyFrag = -1
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
			// pipeline.Pattern.Name is "regex over column or table name", so a
			// user who writes a pattern for `patients` to raise a whole table
			// gets the table, not silence.
			if w.neverMask || (!re.MatchString(normaliseName(c.Column)) && !re.MatchString(normaliseName(c.Table.Name))) {
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
		if cc.Confidence > w.d.Confidence && !w.neverMask {
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

// finalise applies ARCHITECTURE.md §4's threshold and fills the fields the plan
// and the emitter read.
func (st *state) finalise() {
	for _, c := range st.order {
		w := st.dec[c]
		w.d.Reason = joinReason(w.frags...)
		w.d.TypeFP = w.column.Fingerprint
		w.d.UniqueIndex = st.unique[c]
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
// and whose masking merges two groups at once; T-0099 carries the exact rule.
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

// tableWasSampled reports that at least one column of the table produced a
// sample value. It is how raiseCompositeUnique tells "this column is NULL in
// every row" from "nothing about this table could be read at all", which look
// the same from one column and mean opposite things.
func (st *state) tableWasSampled(t pipeline.Table) bool {
	for _, col := range t.Columns {
		if len(st.samples(ref.ColumnRef{Table: t.Ref, Column: col.Name})) > 0 {
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
	values := st.samples(c)
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
