// SPDX-License-Identifier: Apache-2.0

package verify

import (
	"context"
	"crypto/sha256"
	"math/big"
	"strings"

	"github.com/Liarea/lazyslice/internal/event"
	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
	"github.com/Liarea/lazyslice/internal/textsig"
)

// The second net (ARCHITECTURE.md section 6 item 4).
//
// All twelve of the classifier's value validators, folded into the fifteen
// entries in validators.go (its two network validators, IP and MAC, share one
// entry here; the financial and national_id ones do not, and national_id
// answers for four -- see validators.go's own comment, T-0136, the T-0187
// review round and T-0240), run over the full contents of
// every column of the loaded target that is not fully masked by a category
// masker: every unmasked, non-opted-out column of a family this package can
// name and render, and the string leaves of every JSON column, masked or not. A
// column reaching the strong ratio is exit 9, and so is any hit at all on a
// column holding fewer than minValues non-NULL values, which has no ratio to
// reach it with (netColumn) — except under the dictionary rule below. The
// target is small, so this is a scan and not a sample, which is what makes it
// catch a column the 200-row sample under-represented.
//
// A string that came out of a *document* — a json column's leaf, or a document
// inside a character column (withDocuments) — is scored against its own
// denominator and never against the column's (netTally, netColumn). Counting
// both into one number let a document dilute the column's own values out of
// every threshold it is judged by, which is the fail-open the T-REDFIX review
// found and netTally's comment records.
//
// Two things it does not catch, stated here and in internal/verify/CLAUDE.md
// rather than implied:
//   - a category outside the rule pack (THREAT_MODEL.md T1 says so);
//   - a column of a family this package cannot name — famOther, and so a
//     tsvector or an enum (columns.go, netText).
//
// It also implements only the strong branch of ARCHITECTURE.md section 4's
// scoring: a weak ratio raised to `possible` by the neighbouring-column rule is
// not reached here (see CLAUDE.md).
//
// **The dictionary rule.** person_name and free_text arrived with the shared
// leaf package that holds the name dictionary (internal/textsig, tracker
// T-0055), and they are scored under a threshold of their own, because the
// classifier's would fail a target on an ordinary English word. Black, Brown,
// Hill, Green and Wood are all surnames, so a `product.colour` column is 100%
// "person_name" to the validator the classifier uses — and the two packages do
// different things with that answer. The classifier masks the column, which
// costs a lookup table; this net refuses a target that is already loaded, at
// exit 9, with no green path short of --unmask on a column that holds no
// personal data. So on this side a dictionary hit has to be a *shape* and not a
// word, and the shape is the same one for both validators — a given name
// immediately followed by a surname:
//
//	person_name counts textsig.Dict.NameShape — a whole value of two or three
//	dictionary words carrying that pair — never LooksLikeName, and never the
//	plain multi-token shape either, because green lane, west hill and hunter
//	green are all pairs of dictionary surnames (tracker T-0055 review);
//
//	free_text counts textsig.Dict.ProseName — six words or more carrying that
//	same pair — never Prose, which fires on one dictionary word and therefore
//	on "The supplier may terminate this agreement on thirty days notice.",
//	may being a surname like about two hundred other ordinary English words
//	in names.txt;
//
//	neither runs over a document's leaves at all (applies, below), because
//	internal/classify's leaf signal never consults the dictionary;
//
//	and either way the column fails only on the strong ratio across at least
//	minValues *distinct* hitting values, so neither the T-0058 "any hit below
//	minValues" branch nor one literal repeated down a column can carry it.
//
// What that costs is a real name column of one word per row — a `forename`
// column the classifier did not mask — which this net now passes, and a name
// column written surname-first, and a note naming a person the dictionary does
// not carry. That is the direction the asymmetry has to fail in: the
// alternative is exit 9 on a colour column, a street-name column or a column
// of contract clauses, which is a refusal an operator cannot act on and would
// learn to route around with --unmask.
//
// A column carrying an --unmask opt-out is deliberately outside this net: that
// is why the opt-out requires a reason and expires when the column's type
// changes (ARCHITECTURE.md section 8), rather than being a scan it would fail
// on every run.

// secondNet is item 4.
func (s *state) secondNet(ctx context.Context) error {
	scanned := int64(0)
	for _, step := range s.steps {
		table := s.tables[step.Table]
		for _, c := range table.Columns {
			col := ref.ColumnRef{Table: step.Table, Column: c.Name}
			mode, ok := s.netMode(col, c)
			if !ok {
				continue
			}
			scanned++
			if err := s.netColumn(ctx, col, mode); err != nil {
				return err
			}
		}
	}
	// Never beside a failure of the same name: Report.Checks is what the run
	// prints, and a passing "second_net" line under a failing one is a green
	// tick on the very check that produced exit 9. fk.go, counts.go,
	// residual.go and sample.go all guard the same way.
	if !s.failed(checkSecondNet) {
		s.pass(checkSecondNet, CodeSecondNetPassed, scanned)
	}
	return nil
}

// netMode is how one column is read by the net, and whether it is read at all.
type netMode struct {
	// leaves is true for a masked JSON column, whose string leaves are the net's
	// subject rather than the column's own value.
	leaves bool
	// array is true for an array column, whose elements are what the validators
	// run over: ARCHITECTURE.md section 4 classifies an array on its element
	// type, so the net has to ask the same question of the same values
	// (netValues).
	array bool
	// text and digits say which validators apply, from the column's family.
	text   bool
	digits bool
	// docMasked is true for a masked document column (T-0172). json.go's
	// maskKey already replaces every key strongKeyCategory names — email,
	// phone, the Luhn half of financial_account — through that category's own
	// masker, and a category masker's output is, by construction, still a
	// value of that category (mask/gen_email.go's address, mask/gen_number.go's
	// Luhn-valid digit run): the masked key still matches the validator that
	// made transform mask it in the first place. netValues reads this to drop
	// exactly those keys from the net's input, so the net never counts the
	// masker's own correct output as a hit. A key none of the three names is
	// untouched by transform whether or not the column is masked — that is
	// keyCategory's own stated limitation (json.go) — so it stays in the net's
	// coverage either way; only the three the masker actually rewrites are
	// excluded, and only when this column was masked at all.
	docMasked bool
	// bytea is true for a bytea column read as text (the 2026-09-15 red team's
	// A4a). famBytea used to be outside netText altogether, on the same
	// argument internal/classify's bestSignal made about its samples — a PNG
	// reads as an address — so a bytea holding printable UTF-8 was read by
	// nothing on either side and `assets.blob_doc` crossed into the target
	// with an email address and a national identifier in it, under exit 0.
	// netValues drops a value this flag is set on unless
	// textsig.PrintableText accepts it, so a column of images is still read by
	// nothing and a column of documents is read like text.
	bytea bool
	// maybeDocument is true for a character column, whose value may itself be
	// a JSON document (the 2026-09-15 red team's A5b). `blobs.payload2 text`
	// held a three-level document whose leaves were chosen so that no
	// validator fires on the document read as one string; document() covers
	// json, jsonb and hstore only, so neither this net nor
	// internal/classify's leaf signal ever looked inside it. A `payload text`
	// holding JSON is one of the commonest shapes in a real schema.
	maybeDocument bool
	// derivedFromMasked is true for a generated column whose expression names
	// at least one column of its own table and only masked ones (ADR-015's
	// second-net clause, generatedFromMasked). Its value is computed by the
	// target from values the run already masked, so a dictionary shape in it
	// is the masker's own vocabulary — `full_name GENERATED ALWAYS AS
	// (first_name || ' ' || last_name)` over masked names is a given name and
	// a surname by construction — and the dictionary rule is skipped for it.
	// Every validator that carries a parse still runs over it.
	derivedFromMasked bool
	// family is the column's own type family (columns.go's fam* constants),
	// carried alongside the flags above so netColumn can choose the right
	// verify.refused.second_net* code and hint without asking shapeOf a
	// second time (T-0369).
	family string
}

func (s *state) netMode(col ref.ColumnRef, c pipeline.Column) (netMode, bool) {
	family, array := s.shapeOf(c)
	d, has := s.decision(col)
	switch {
	case has && optedOut(d):
		// The one column deliberately outside this net (ARCHITECTURE.md section
		// 8): the opt-out carries a reason and expires on a type change instead.
		return netMode{}, false
	case has && d.NeverMasked && pipeline.IsFrameworkMetadataColumn(col.Table.Name, col.Column):
		// A migration tool's own bookkeeping column (T-0314: schema_migrations.
		// version, ar_internal_metadata.key, flyway_schema_history.checksum and
		// their kin, listed per tool in internal/pipeline/framework.go) is
		// copied whole by design and never masked, so a hit here can only
		// refuse a correct run: a Rails migration timestamp that happens to
		// pass the Luhn check is the shape dogfood session 1 met (T-0348). The
		// exemption is the allowlist's, not the table's: a framework table's
		// identity-bearing column (Flyway's installed_by, Liquibase's AUTHOR)
		// is not on it, is classified like any other column, and is scanned
		// here when it is copied. Both names are matched case-insensitively.
		return netMode{}, false
	case document(family):
		// A masked document's masker was chosen per key by name, so the net
		// checks the leaves; an unmasked one had no masker at all, which is a
		// stronger reason to read its leaves and not a reason to skip it.
		return netMode{leaves: true, text: true, docMasked: has && d.Masked, family: family}, true
	case has && d.Masked:
		return netMode{}, false
	case netText(family):
		return netMode{
			array: array, text: true, maybeDocument: character(family),
			derivedFromMasked: s.generatedFromMasked(col.Table, c.Generated),
			family:            family,
		}, true
	case family == famBytea:
		return netMode{array: array, text: true, bytea: true, maybeDocument: true, family: family}, true
	case numeric(family):
		return netMode{array: array, digits: true, family: family}, true
	}
	return netMode{}, false
}

// netTally is one denominator and the hits counted against it: either the
// column's own non-NULL values, or the strings that came out of a document one
// of those values held.
//
// **They are counted apart, and that is the whole of this type.** When
// withDocuments grew its maybeDocument arm (the 2026-09-15 red team's A5b),
// every leaf and key a document yielded was counted into the *column's*
// nonNull — and nonNull is both the ratio's denominator and the `proven` gate
// below, so a document could dilute the column's own values out of a refusal. A
// two-row text column holding "221 Baker Street, London" and "ok" is unproven
// and fails at exit 9 on the address (T-0058); the identical column with the
// second row replaced by a four-key JSON object had a denominator of five,
// which is `proven`, and 1/5 is under validatorThreshold, so the net recorded
// nothing and a production address reached the target under exit 0. The same
// dilution weakened the dictionary rule on any text column that also held a
// document. A document is evidence about the document; it is not evidence
// about the column's own values, in either direction, and it may not move the
// number the column's own values are judged by.
type netTally struct {
	nonNull int64
	hits    []int64
}

// fileTally counts one column's direct values that are file names
// (textsig.FileNameStem): how many carry a word from the name dictionary in
// their stem, and how many textsig.LooksSecret spared. It is the second net's
// half of internal/classify's namedFileNames (T-0315): when at least
// namedFileShare of a column's values are file names carrying a name, the
// spared ones are counted as the credential entry's hits again, so a column
// of documents named after people is scored as it was before LooksSecret
// stopped reading a file name as a secret, and a column of screenshots is not.
type fileTally struct {
	named, spared int64
}

func (f *fileTally) observe(text string) {
	stem, ok := textsig.FileNameStem(text)
	if !ok {
		return
	}
	if textsig.Dictionary().ContainsName(stem) {
		f.named++
	}
	if !textsig.LooksSecret(text) {
		f.spared++
	}
}

// credentialHits is hits for every entry but the credential one, and for that
// one hits plus the file names LooksSecret spared, when the column's file
// names carry a name often enough (fileTally).
func (f fileTally) credentialHits(val validator, hits, nonNull int64) int64 {
	if val.category != pipeline.CatCredential || f.named == 0 || nonNull == 0 ||
		float64(f.named)/float64(nonNull) < namedFileShare {
		return hits
	}
	return hits + f.spared
}

// digitRange tracks one column's numeric span, order-independent on purpose
// (T-0187 second review round, findings 1 and 3): scanSQL's "SELECT column
// FROM table" (sql.go) carries no ORDER BY, so the order this net sees a
// column's values in is whatever Postgres's own heap scan gives, not a
// signal it may read anything into. A generated sequence — a surrogate key's
// own values (id, id+1, id+2, ...) or an ordinary dense business-number
// block with no key at all (400100000+i) — packs n values into a numeric
// range about n wide, however they arrive; n independently assigned
// identifiers, real SSNs among them, are drawn from a space many orders of
// magnitude wider than the sample and do not. Reading the values this way,
// rather than asking internal/classify whether it called the column a
// surrogate key, is what closes finding 3: a primary key of real SSNs is
// exactly as dense-or-not as the same values in a plain column, so classify's
// own miss (nothing in the column's name or values said "personal" to it)
// cannot suppress this net's answer any more.
//
// Despite the name, it is fed every direct value regardless of family
// (T-0240, netColumn's own comment on the point): the observation is a fact
// about the column's own digit shape, not about its Postgres type, and the
// character-family national_id entry (validators.go) needs the identical
// answer a digits-family column's own dense sequence already gives — a
// national identifier issued in a contiguous block reads the same whether it
// sits in a bigint or in a varchar(9). A genuine character column of
// ordinary strings breaks the range on its first value (observe's own
// comment) and dense() answers false for it, the same as a broken
// digits-family range.
type digitRange struct {
	min, max *big.Int
	n        int64
	broken   bool
}

// observe folds one digits-family value into the range. A value that will not
// parse as a plain non-negative integer — a numeric column can carry a sign
// or a decimal point, which a digits-family national identifier never does —
// breaks the range rather than being skipped: an unparseable value is
// evidence this net has no basis for calling the column a generated sequence,
// and the safe default is to keep the ordinary ratio in charge.
func (r *digitRange) observe(s string) {
	if r.broken {
		return
	}
	v, ok := new(big.Int).SetString(s, 10)
	if !ok || v.Sign() < 0 {
		r.broken = true
		return
	}
	r.n++
	if r.min == nil || v.Cmp(r.min) < 0 {
		r.min = v
	}
	if r.max == nil || v.Cmp(r.max) > 0 {
		r.max = v
	}
}

// dense reports a range packed to within a factor of two of its own count —
// wide enough that a handful of gaps (deleted rows, or a --take slice that
// thinned a contiguous block) still reads as a sequence, and many orders of
// magnitude short of what an issuing authority's own independently assigned
// numbers would need to clear by chance. minValues floors it the same way
// every other ratio in this net is floored (T-0058): two values a range apart
// says nothing about whether the column is a sequence.
func (r *digitRange) dense() bool {
	if r.broken || r.n < minValues || r.min == nil {
		return false
	}
	span := new(big.Int).Sub(r.max, r.min)
	span.Add(span, big.NewInt(1))
	n := big.NewInt(r.n)
	if span.Cmp(n) < 0 {
		return false // impossible unless values repeat; not proven dense
	}
	limit := new(big.Int).Mul(n, big.NewInt(2))
	return span.Cmp(limit) <= 0
}

// corroborated is the national_id digits-family and character-family
// entries' own gate (T-0187 third review round, finding 1; T-0240): true
// when this column's pipeline.Decision carries any of the three signals
// internal/classify already computed and this package may not re-derive -- a
// rules.yml national_id name-pattern hit on the column itself, a
// certain-or-likely personal column elsewhere in the same table (the
// neighbouring-column rule's own `likely` count), or a masked
// person-identifying column elsewhere in the same table at ConfPossible or
// above (the same rule's `maskedPersonal` count). A column with none of the
// three is read by nothing but its own ratio, which a sparse, fixed-prefix
// reference-number column clears about as often as a real leaked identifier
// column does -- see requiresCorroboration's own comment in validators.go.
//
// **The third signal is the round-4 red team's own finding, and it is not
// the second signal's floor lowered.** TableHasLikelyPersonalColumn's own
// ConfLikely floor missed a column the run had already decided to mask: a
// name-only match with no samples decides ConfPossible, not ConfLikely
// (§4's own threshold), so `msisdn numeric`, masked as `phone` on its name
// alone, beside `taxref bigint` or `taxref varchar(9)` with no name or value
// signal of its own, counted the table as having no personal neighbour at
// all -- `taxref` read "no name or value signal" and crossed verbatim in all
// three of the round's A9b replays (docs/reviews/2026-09-15-redteam/
// round4-still-leaking.json). A column the run itself is about to mask is
// evidence about the table whatever confidence line it landed on.
// Decision.TableHasMaskedPersonalColumn (internal/pipeline/classify.go) is
// that lower floor, kept as its own field rather than folded into
// TableHasLikelyPersonalColumn because that field also corroborates
// internal/classify's own guessed-region phone pass (guessedPhoneColumns,
// T-0221), which this task's brief did not ask to widen.
//
// It reads the decision internal/classify already recorded rather than
// asking rules.yml or the schema a second question: internal/verify may not
// import internal/classify (internal/CLAUDE.md's import graph), and a second
// copy of the rule pack's name-matching logic here would be exactly the drift
// internal/verify/CLAUDE.md already records this file's validators as a risk
// of. A column with no decision at all -- s.decision's own "has" -- is
// uncorroborated, the same as one classify decided `none` with neither
// signal set.
func (s *state) corroborated(col ref.ColumnRef) bool {
	d, has := s.decision(col)
	return has && (d.NameMatchedNationalID || d.TableHasLikelyPersonalColumn || d.TableHasMaskedPersonalColumn)
}

// corroboratedForSequence is corroborated's narrower sibling, read only by
// the dense-sequence override below (T-0240 review round, high finding). It
// answers the same "is there evidence beyond this column's own ratio"
// question with two of corroborated's three signals -- NameMatchedNationalID
// and TableHasMaskedPersonalColumn -- and never
// Decision.TableHasLikelyPersonalColumn.
//
// That field is neighbouringColumns' own `likely` count
// (internal/classify/classify.go), and it counts a neighbour at ConfLikely
// or above under *any* category, with no identifiesAPerson test at all --
// unlike TableHasMaskedPersonalColumn, whose own gate (maskedPersonalNeighbour)
// checks identifiesAPerson explicitly. free_text, semi_structured,
// binary_personal and derived_text are categories a column reaches by its
// *type* alone (identifiesAPerson's own comment), and a tsvector is
// derived_text at ConfCertain on every schema that has one. Reading
// TableHasLikelyPersonalColumn here let a search-index column beside a
// dense, non-key business number cancel that column's own dense-sequence
// exemption: a `business_ref bigint` packed into a dense range beside a
// `search tsvector` column has neverMasked false (it is not a key or an FK
// child) and corroborated true (via TableHasLikelyPersonalColumn, on the
// strength of a neighbour that is not personal data at all and never will
// be, however this table's schema grows) -- so the exemption was cancelled,
// the ratio was scored, and a column requiresCorroboration exists to protect
// from exactly this shape of numeric coincidence refused the run instead
// (T-0240 review round, high finding). corroborated itself is unchanged --
// requiresCorroboration's own three-signal scoring is the pre-existing
// behaviour this task's brief did not ask to narrow -- and
// testdata/regressions/032 and 033 both still refuse under this narrower
// check: each corroborates through TableHasMaskedPersonalColumn, which this
// function reads too.
func (s *state) corroboratedForSequence(col ref.ColumnRef) bool {
	d, has := s.decision(col)
	return has && (d.NameMatchedNationalID || d.TableHasMaskedPersonalColumn)
}

// neverMasked reads Decision.NeverMasked (T-0240 review round, high
// finding): internal/classify's own key exemption for this column -- a
// surrogate key, or a validated foreign-key child whose parent stayed
// unmasked (ARCHITECTURE.md §4; internal/pipeline/classify.go's own comment
// on the field has the full account). It is the dense-sequence exemption's
// unconditional half, below: a column classify has already decided *is* a
// surrogate key's own values, or a mirror of one, is dense by construction
// -- `order_id`'s values ARE `id`'s, copied verbatim across the foreign key
// -- and that fact does not depend on, and must not be cancelled by,
// whatever category an unrelated column elsewhere in the table was decided
// under. A column with no decision at all is not exempt this way, the same
// as corroborated's own "has" branch.
func (s *state) neverMasked(col ref.ColumnRef) bool {
	d, has := s.decision(col)
	return has && d.NeverMasked
}

// netColumn runs every applicable validator over one column's whole contents.
func (s *state) netColumn(ctx context.Context, col ref.ColumnRef, mode netMode) error {
	own := netTally{hits: make([]int64, len(validators))}
	leaf := netTally{hits: make([]int64, len(validators))}
	var seq digitRange
	var files fileTally
	// distinct holds, for the dictionary-backed validators only, the digests of
	// up to minValues distinct values that hit. A digest rather than the value
	// because a free_text value can be a whole document and this set outlives
	// the row: at most minValues×32 bytes per validator, and no production value
	// held any longer than the scan of the row it came from. It is the column's
	// own values only, because those are the only strings a dictionary-backed
	// validator ever sees (applies, and count's fromLeaf).
	distinct := make([]map[[sha256.Size]byte]struct{}, len(validators))
	for i, val := range validators {
		if val.dict && applies(val, mode) {
			distinct[i] = make(map[[sha256.Size]byte]struct{}, minValues)
		}
	}

	err := s.scanColumn(ctx, col.Table, col.Column, func(v any) error {
		direct, fromLeaves := s.netStrings(v, mode)
		for _, text := range direct {
			own.nonNull++
			s.count(text, false, mode, own.hits, distinct)
			files.observe(text)
			// seq reads every direct value regardless of family, not only a
			// digits-family column's (T-0240): the character-family twin of
			// the national_id digits entry (validators.go) is sequenceExempt
			// too, over a varchar(9) or text column of zero-padded
			// identifiers, and it needs the identical dense-range answer a
			// digits-family column already gets. digitRange.observe breaks
			// the range on the first value that will not parse as a plain
			// non-negative integer, which an ordinary text column of email
			// addresses or names does on its very first value -- so this
			// costs nothing on a column no sequenceExempt entry ever applies
			// to, and dense() answers false for it the same way it already
			// does for a broken digits-family range.
			seq.observe(text)
		}
		for _, text := range fromLeaves {
			leaf.nonNull++
			s.count(text, true, mode, leaf.hits, nil)
		}
		return nil
	})
	if err != nil {
		return err
	}
	if own.nonNull == 0 && leaf.nonNull == 0 {
		return nil
	}
	dense := seq.dense()
	corroborated := s.corroborated(col)
	corroboratedForSeq := s.corroboratedForSequence(col)
	neverMasked := s.neverMasked(col)
	for i, val := range validators {
		if !applies(val, mode) {
			continue
		}
		if val.dict && mode.derivedFromMasked {
			// ADR-015: a generated column over masked columns only holds the
			// masker's own words, and a dictionary shape there is not evidence
			// about the source (netMode.derivedFromMasked).
			continue
		}
		if val.sequenceExempt && dense && (neverMasked || !corroboratedForSeq) {
			// T-0187 second review round, findings 1 and 3, narrowed by
			// T-0240 and by the two review rounds that followed it: a
			// column whose own values pack into a dense numeric range is a
			// generated sequence, key or not, and this validator has no
			// check digit to tell that apart from a real identifier by
			// ratio alone -- *when nothing corroborates it, or when
			// internal/classify has already decided the column IS a
			// surrogate key or a mirror of one*. The round-4 red team's own
			// dense A9b replay is what `&& !corroboratedForSeq` closes: a
			// contiguously issued identifier block -- a payroll or benefits
			// import -- has exactly this dense shape, so once a masked
			// personal neighbour or a name match on this column corroborates
			// it, density alone stops being a reason to skip it and the
			// ratio is scored anyway, below.
			//
			// `neverMasked` is checked first and independently, and it is
			// not "corroborated, but weaker" -- it is a different question
			// with the opposite answer for the shape it covers.
			// `corroboratedForSeq` asks whether something *else* in the
			// table suggests this column might be personal; `neverMasked`
			// asks whether internal/classify has already established that
			// this column's own values are a surrogate key's, or a
			// foreign-key child's copy of one. The second is not defeated
			// by the first, because it is not evidence about the same
			// thing: `order_id`'s values are `id`'s, dense because they
			// are copied, whatever category an unrelated `email` column
			// elsewhere in the table was decided under. Reading
			// `corroboratedForSeq` alone here would let exactly that
			// unrelated column cancel the exemption for `order_id`
			// (testdata/regressions/021's own control, which pins this).
			// See digitRange's own comment above and neverMasked's own,
			// above.
			//
			// `corroboratedForSeq`, not `corroborated`, is deliberate, and
			// it is corroboratedForSequence's own comment (above) that
			// explains why: `corroborated`'s TableHasLikelyPersonalColumn
			// signal counts a neighbour at ConfLikely or above under *any*
			// category, including derived_text (a tsvector, ConfCertain by
			// type alone) -- so a search-index column beside a dense,
			// non-key business number used to cancel that column's own
			// exemption on the strength of a neighbour that is not, and
			// never will be, personal data (T-0240 review round, high
			// finding; testdata/regressions/034's own control).
			continue
		}
		if val.columns != nil && !val.columns(snakeColumnName(col.Column)) {
			// T-0316: the card entry this column's name does not select.
			continue
		}
		if val.requiresCorroboration && !corroborated {
			// T-0187 third review round, finding 1: this validator's ratio,
			// however tuned, cannot tell a sparse column of assigned
			// identifiers from a sparse column of ordinary fixed-prefix
			// reference numbers -- both clear the SSA's exclusion ranges at
			// essentially 1.0. See corroborated, above, and
			// requiresCorroboration's own comment in validators.go.
			continue
		}
		distinctHits := 0
		if d := distinct[i]; d != nil {
			distinctHits = len(d)
		}
		// Each denominator is scored on its own, and either one failing is the
		// refusal. A json column yields no direct values at all (own.nonNull is
		// zero and scoreHits says nothing about it), and a text column holding
		// a document is judged twice over two sets of values that have nothing
		// to do with each other.
		//
		// exemptColumns (T-0315, narrowed T-0361) is asked of the column's own
		// values only, never of leaf.hits: internal/classify's entropyExemptNames
		// exempts a `type`/`klass`/`component_name` *column's own scalar value*
		// -- a Rails STI class name the application constantizes -- and says
		// nothing about a document a same-named jsonb column happens to hold.
		// classify's jsonSignal never reads the exemption at all (it has no
		// column-name context at a leaf), so a jsonb `type` column whose leaves
		// carry a credential-shaped string is a leak neither net may wave
		// through: skipping leaf.hits here on the strength of the column's own
		// name would be looser than the classifier, which is exactly what
		// T-0315's review found and T-0361 closes.
		var ownHits int64
		ownFails := false
		if !val.exemptColumns[snakeColumnName(col.Column)] {
			ownHits = files.credentialHits(val, own.hits[i], own.nonNull)
			ownFails = scoreHits(val, ownHits, own.nonNull, distinctHits)
		}
		if !ownFails && !scoreHits(val, leaf.hits[i], leaf.nonNull, 0) {
			continue
		}
		s.fail(&Refusal{
			Code: secondNetCode(val, mode), Exit: exitResidual, Check: checkSecondNet,
			Table: col.Table, Column: col.Column, Count: ownHits + leaf.hits[i],
			Reason: val.name,
		})
		// One category per column: the column is already exit 9, and a second
		// line naming a second validator over the same values says nothing more
		// about what to do next.
		return nil
	}
	return nil
}

// secondNetCode chooses which verify.refused.second_net* code names this
// refusal, which is what internal/event/catalogue.yml's hint for that code
// prints (T-0369): the ordinary code's hint, "--mask TABLE.COL={reason}", is
// a dead end in three cases this net can produce, each named after the
// column's own family and masked state rather than after the validator, so a
// wrong hint is never printed even when it names the same column two
// different validators fail it for.
func secondNetCode(val validator, mode netMode) event.Code {
	switch {
	case document(mode.family):
		// The second net reads a json, jsonb or hstore column's leaves
		// whatever category matched them, masked or not
		// (internal/verify/CLAUDE.md's own "Rules" section) -- but no
		// category but semi_structured accepts the json family
		// (internal/classify/rules.yml), and an already-masked column's
		// category cannot be changed by --mask at all (internal/core's
		// checkMasks). mode.docMasked is netMode's own "has && d.Masked"
		// for exactly this family, read once there rather than re-asked.
		if mode.docMasked {
			return CodeRefusedSecondNetDocumentMasked
		}
		return CodeRefusedSecondNetDocument
	case !categoryAcceptsFamily(val.category, mode.family):
		// A scalar column is never scanned here once it is masked (netMode's
		// own `case has && d.Masked: return netMode{}, false`, above), so
		// the only two states a non-document refusal reaches are "the
		// category accepts this family" (the ordinary code) and this one.
		return CodeRefusedSecondNetTypeConflict
	default:
		return CodeRefusedSecondNet
	}
}

// scoreHits is section 4's scoring for one validator over one denominator: true
// when those hits fail the column.
//
// A column with fewer than minValues non-NULL values is *unproven*, not clean.
// Returning early there was a fail-open with nothing above it: a three-row
// table yields two values, the ratio over them means nothing, and
// public.devices.owned_by in testdata/nasty.sql — two email addresses and a
// NULL — reached the target in cleartext under exit 0, with the classifier
// silent for the same reason and this net silent after it (THREAT_MODEL.md
// T1, tracker T-0058). So the validators run over whatever there is and *any*
// hit fails: the threshold is what a ratio buys, and below minValues there is
// no ratio to buy it with. One value that parses as an email is still a
// production email address in the target, which is the thing this net exists
// to refuse.
//
// The two dictionary-backed validators are outside that branch: a dictionary
// word is not a parse, so "any hit" there would be exit 9 on a two-row lookup
// table of English nouns (the dictionary rule, in the package comment above).
//
// nonNull is counted over the *target*, so the unproven branch is slice-size
// dependent: a table --take reduces to two rows is unproven here and the same
// table at --take 500 is not, which makes the verdict non-monotone in the slice
// size for the two validators that carry no parse (textsig.AddressShape,
// textsig.LooksSecret). That cost is weighed against the leak above in
// internal/verify/CLAUDE.md, which also records the narrowing to revisit if the
// refusal proves noisy.
func scoreHits(val validator, hits, nonNull int64, distinctHits int) bool {
	if hits == 0 || nonNull == 0 {
		return false
	}
	if nonNull < val.minNonNull {
		// T-0315: the credential entry's own floor, below which it is not
		// scored at all -- see validator.minNonNull.
		return false
	}
	ratio := float64(hits) / float64(nonNull)
	// threshold is validatorThreshold unless the entry overrides it
	// (validator.minRatio, T-0187 review round, finding 1): the national_id
	// digits entry alone, whose ordinary 0.8 is not a meaningful bar for a
	// validator with no check digit — see nationalIDDigitsThreshold's own
	// comment in validators.go.
	threshold := validatorThreshold
	if val.minRatio > 0 {
		threshold = val.minRatio
	}
	switch {
	case val.dict:
		// The dictionary rule (see the package comment above and
		// internal/verify/CLAUDE.md). Two differences from the validators
		// that carry a parse, both narrowing, and both because a
		// dictionary word is a word an ordinary English column may hold:
		// the strong ratio is required whatever the column's size, so the
		// unproven branch below does not extend to these two; and the hits
		// must be at least minValues *distinct* values, so a single
		// dictionary literal repeated down a column cannot reach exit 9.
		// The validators themselves are already the narrow ones —
		// NameShape, not LooksLikeName; ProseName, not Prose.
		return ratio >= threshold && distinctHits >= minValues
	case val.strong:
		// Second net, second bug (docs/reviews/2026-09-09/REVIEW.md
		// finding 7): a strong validator is a precise parse, so any hit
		// at all is one production value of that shape sitting in the
		// target, whatever the ratio and whether or not the column is
		// "proven". Before this case existed, a strong hit fell into the
		// `proven && ratio < validatorThreshold` branch below like every
		// other non-dict validator, so one email address among nineteen
		// ordinary strings had a ratio of 5% and passed at exit 0
		// (evidence/sparse_email.log) — the 80% column ratio is a good
		// question for "what category is this column", and a poor one
		// for "does this already-loaded target hold a recognisable
		// source value". No ratio gate here: hits == 0 was already
		// filtered above, so reaching this case is the refusal.
		return true
	default:
		// The ratio rule, for the validators that are a shape guess rather
		// than a parse (credential, address), the checksum-only half of
		// national_id, and the digits half of Luhn: below minValues the
		// column is unproven and any hit still fails (T-0058, above); at or
		// above it, only a ratio at or over threshold does. Weakening this
		// to "any hit" for these would be exit 9 on an ordinary slug or a
		// room number, which is not what a heuristic's occasional false
		// positive should cost on a target that is already loaded.
		return nonNull < minValues || ratio >= threshold
	}
}

// applies reports whether one validator runs over the values this mode yields.
//
// The two dictionary-backed validators are excluded from a document's leaves,
// and that is a deliberate hole with a task against it. internal/classify
// short-circuits every json/jsonb/hstore column into its own leaf signal
// (jsonSignal), which asks the key patterns plus email, phone, IP, IBAN and
// Luhn about a leaf and never consults the name dictionary at all — so a
// person_name or free_text refusal over leaves would be this net refusing a
// loaded target on evidence the classifier is structurally unable to have seen,
// with no green path short of --unmask on a column the classifier had no reason
// to mask. The other seven entries stay — email, phone, network_id,
// financial_account, online_id (T-0122), credential and address — and five of
// the shapes they ask about are the classifier's own leaf questions exactly:
// email, phone, IP, IBAN and Luhn. Four are not — MAC, URL, LooksSecret and
// AddressShape — and they stay because the argument above does not reach them:
// every one is a *parse* rather than a dictionary word, so a leaf that hits one
// is a value of that shape and not an ordinary English sentence, and a document
// is not a safer place to keep an address than the scalar column of the same
// table the classifier would have masked. When classify's leaf signal reads the
// dictionary, the dict exclusion comes off with it (tracker T-0087).
func applies(v validator, mode netMode) bool {
	if v.dict && mode.leaves {
		return false
	}
	// v.sequenceExempt is not read here: whether the exemption fires depends
	// on the column's own values (digitRange, above netColumn), which are not
	// known until the scan finishes, so netColumn checks it itself once
	// scoring starts rather than gating the tally here. v.requiresCorroboration
	// is not read here either, for a simpler reason: corroborated (above) is a
	// property of the column's decision, not of its values, and is cheap
	// enough to read once in netColumn rather than being threaded through
	// this function's signature.
	return (mode.text && v.text) || (mode.digits && v.digits)
}

// netValues reduces one scanned value to the strings the validators run over:
// an array yields its elements, because section 4 classifies an array on its
// element type; a masked document yields its string leaves; a NULL yields
// nothing, because the ratio is over the non-NULL values.
func (s *state) netStrings(v any, mode netMode) (direct, fromLeaves []string) {
	if v == nil {
		return nil, nil
	}
	if mode.leaves {
		ls := leaves(v)
		keys := documentKeys(v)
		out := make([]string, 0, len(ls)+len(keys))
		for _, l := range ls {
			if l.str && l.text != "" {
				out = append(out, l.text)
			}
		}
		// Object keys too, not only values (T-0137 review round, finding 3):
		// an email, a phone number or a card used as a JSON key is exactly as
		// unmasked as the same value used as a value, in a column this mode
		// already reads because it is document-shaped — a masked jsonb column
		// whose masker (json.go's maskKey) only fires on the three strong
		// validators, or an unmasked/--unmask one with no masker at all. The
		// dictionary-backed validators still never see these strings, because
		// applies excludes mode.leaves for both — a key is never a sentence.
		for _, occ := range keys {
			if occ.name == "" {
				continue
			}
			if mode.docMasked {
				// T-0172: on a *masked* document column, a key that still
				// matches one of the three strongKeyCategory validators is
				// never a surviving source value — json.go's maskKey ran
				// every key through keyCategory, the same three-validator
				// question, and replaced every match with that category's own
				// masker output; a category masker's output is by
				// construction a value of that category (mask.CLAUDE.md,
				// financialAccountMasker's own comment), so the masked key
				// still matches the validator that made transform mask it.
				// Counting that match here would be the net refusing the
				// masker's own correct output (the bug this task fixes,
				// testdata/regressions/013). residual.go's keyHits already
				// proves no *source* key survived at this path, by testing
				// canonical equality against the source rather than asking
				// "does this look like the category" — that is the check
				// this key needs, not this one. A key none of the three
				// names was never touched by transform, so it is kept: it
				// still needs the net's other validators (network_id,
				// online_id, credential, address, the IBAN half of
				// financial_account), none of which json.go's keyCategory
				// ever masks, whether or not the column is masked.
				if _, ok := strongKeyCategory(occ.name); ok {
					continue
				}
			}
			out = append(out, occ.name)
		}
		return nil, out
	}
	if elems, ok := v.([]any); ok {
		out := make([]string, 0, len(elems))
		for _, e := range elems {
			if e == nil {
				continue
			}
			out = append(out, textOf(e))
		}
		return s.withDocuments(out, mode)
	}
	text := textOf(v)
	if mode.array {
		// The other carrier an array arrives in: the server's own text output
		// form, which is what an array of an element type the pool's map does
		// not know comes back as — a citext[] is the single string
		// "{a@b.test,c@d.test}" (arrayliteral.go, tracker T-0118 and T-0129).
		// Unsplit, that value reaches the validators as one string holding two
		// addresses and no validator recognises it, so the net that
		// THREAT_MODEL.md T1 names as one of two controls on classifier recall
		// sees nothing in exactly the column class §4 now classifies
		// element-wise.
		//
		// A literal that will not parse falls through to the whole value
		// rather than failing the run. This net's subject is a column nothing
		// masked, so there is no masker to have failed open: reading the whole
		// value is a recall hole of the kind famOther already is (columns.go),
		// and exit 9 on a column the classifier had no reason to mask would be
		// a refusal with no action behind it. The masked column, where the
		// argument is the opposite one, is refused in residual.go.
		if elems, err := arrayLiteralElements(text); err == nil {
			return s.withDocuments(elems, mode)
		}
	}
	return s.withDocuments([]string{text}, mode)
}

// withDocuments applies the two content gates this net grew from the
// 2026-09-15 red team, and returns the strings split into the ones the
// validators read as themselves and the ones that came out of a document.
//
// A bytea value is dropped unless it is readable text (mode.bytea, A4a): the
// classifier's own argument for never running the text validators over a bytea
// — a PNG reads as an address — is sound about bytes and says nothing about a
// bytea holding a UTF-8 document, and the printability guard is what separates
// the two. textsig.PrintableText is the same function internal/classify's
// bestSignal asks, and since the T-REDFIX review's second finding it is the
// whole of the question on both sides: that package also required 95% of the
// sample set to be readable, so a half-printable column was unmasked there and
// exit 9 here, which is a run with no green path. Neither side has a column
// ratio now.
//
// A character (or readable bytea) value that parses as a JSON object or array
// yields its leaves and keys as well as itself (mode.maybeDocument, A5b). They
// are returned separately because the dictionary-backed validators must not
// see them: internal/classify runs no dictionary signal over a document's
// leaves, and this net may not refuse an already-loaded target on evidence the
// classifier is structurally unable to have seen — the same rule `applies`
// states for a json column, applied to the text column holding the same bytes.
func (s *state) withDocuments(values []string, mode netMode) (direct, fromLeaves []string) {
	if mode.bytea {
		kept := values[:0:0]
		for _, v := range values {
			if textsig.PrintableText(v) {
				kept = append(kept, v)
			}
		}
		values = kept
	}
	if !mode.maybeDocument {
		return values, nil
	}
	for _, v := range values {
		doc, ok := decodeDocument(v)
		if !ok || !isDocumentShape(doc) {
			continue
		}
		for _, l := range leaves(v) {
			if l.str && l.text != "" {
				fromLeaves = append(fromLeaves, l.text)
			}
		}
		for _, occ := range documentKeys(v) {
			if occ.name != "" {
				fromLeaves = append(fromLeaves, occ.name)
			}
		}
	}
	return values, fromLeaves
}

// isDocumentShape reports an object or an array, and nothing else. A bare
// number, string or boolean is valid JSON and is not a document: reading
// "12345" as one would double-count every numeric column in the target.
func isDocumentShape(doc any) bool {
	switch doc.(type) {
	case map[string]any, []any:
		return true
	}
	return false
}

// count runs the applicable validators over one string and records the hits.
// fromLeaf marks a string that came out of a document rather than out of the
// column: the dictionary-backed validators never see one, for the reason
// `applies` gives, and such a string is counted into its own netTally so that a
// document cannot move the denominator the column's own values are judged by.
// distinct is nil on the leaf pass for the same reason — nothing there ever
// reaches a dictionary-backed validator to record a digest for.
func (s *state) count(text string, fromLeaf bool, mode netMode, hits []int64, distinct []map[[sha256.Size]byte]struct{}) {
	for i, val := range validators {
		if !applies(val, mode) || (fromLeaf && val.dict) {
			continue
		}
		hit := val.ok(text)
		if !hit && val.category == pipeline.CatPhone && s.opts.PhoneRegion != "" {
			// T-0221: the configured region only, on the same strong footing
			// as val.ok's own international-only reading -- never the guessed
			// list internal/classify's row path uses, which is corroboration-
			// gated there for a reason that does not survive being asked of an
			// already-loaded target (Options.PhoneRegion's own comment).
			hit = textsig.ValidPhoneRegion(text, s.opts.PhoneRegion)
		}
		if !hit {
			continue
		}
		hits[i]++
		if distinct == nil {
			continue
		}
		if d := distinct[i]; d != nil && len(d) < minValues {
			d[sha256.Sum256([]byte(text))] = struct{}{}
		}
	}
}

// generatedFromMasked reports a generated column whose expression names at
// least one column of its own table and only columns the run masked (ADR-015).
// Anything else — an ordinary column, an expression over an unmasked column, an
// expression that names no column of its table at all, a table this stage does
// not know — is scanned as it always was.
//
// The expression is pg_get_expr's own deparse, so an identifier in it is
// either quoted (and exact) or bare (and folded to lower case, as the server
// folded it), and a string literal is skipped whole. A bare word that names no
// column of the table — a type name after `::`, a function name — is ignored;
// one that does name a column counts, so a table with a column named `text`
// beside an expression casting `::text` reads that column as referenced, and
// the direction that mistake fails in is the strict one.
func (s *state) generatedFromMasked(t ref.TableRef, expr string) bool {
	if strings.TrimSpace(expr) == "" {
		return false
	}
	table := s.tables[t]
	if table == nil {
		return false
	}
	names := make(map[string]bool, len(table.Columns))
	for _, c := range table.Columns {
		names[c.Name] = true
	}
	refs := 0
	for _, id := range exprIdentifiers(expr) {
		if !names[id] {
			continue
		}
		refs++
		d, ok := s.decision(ref.ColumnRef{Table: t, Column: id})
		if !ok || !d.Masked {
			return false
		}
	}
	return refs > 0
}

// exprIdentifiers lists the identifiers of a deparsed SQL expression: every
// double-quoted identifier as written ("" read as one quote), and every bare
// word folded to lower case, with string literals (a doubled single quote
// read as one) skipped.
func exprIdentifiers(expr string) []string {
	var out []string
	for i := 0; i < len(expr); {
		c := expr[i]
		switch {
		case c == '\'':
			i++
			for i < len(expr) {
				if expr[i] == '\'' {
					if i+1 < len(expr) && expr[i+1] == '\'' {
						i += 2
						continue
					}
					i++
					break
				}
				i++
			}
		case c == '"':
			i++
			var b strings.Builder
			for i < len(expr) {
				if expr[i] == '"' {
					if i+1 < len(expr) && expr[i+1] == '"' {
						b.WriteByte('"')
						i += 2
						continue
					}
					i++
					break
				}
				b.WriteByte(expr[i])
				i++
			}
			out = append(out, b.String())
		case isIdentStart(c):
			j := i
			for j < len(expr) && isIdentPart(expr[j]) {
				j++
			}
			out = append(out, strings.ToLower(expr[i:j]))
			i = j
		default:
			i++
		}
	}
	return out
}

func isIdentStart(c byte) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c >= 0x80
}

func isIdentPart(c byte) bool {
	return isIdentStart(c) || (c >= '0' && c <= '9') || c == '$'
}
