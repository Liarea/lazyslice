// SPDX-License-Identifier: Apache-2.0

package verify

import (
	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/textsig"
)

// The second net's validators (ARCHITECTURE.md section 6 item 4).
//
// They are internal/textsig's, which is where internal/classify's are too: one
// leaf package holding the value-only half of section 4's signals, imported by
// both (tracker T-0055). Until that package existed this file carried a hand
// copy of eight of the ten, and the copy is what made the net *narrower* than
// the classifier -- the two it could not copy were the two that read the name
// dictionary, which is exactly the largest personal-data category. A copy that
// has to be kept in step and is not is a second net that has quietly stopped
// agreeing with the classifier it is a second look at.
//
// What the shared package holds is a value shape and nothing else: no name
// pattern, no category, no scoring. This file is where the categories are
// attached and secondnet.go is where the scoring happens, and the scoring is
// deliberately not the classifier's -- see "the dictionary rule" below and in
// internal/verify/CLAUDE.md.
//
// One consequence is unchanged by T-0055 and is stated rather than hidden: the
// net can only find what a validator recognises, which is what THREAT_MODEL.md
// T1 already says about the classifier.

// validatorThreshold is section 4's "at least 80% of non-null samples
// validating", applied here over the whole column rather than over 200 samples:
// the target is small, so this is a scan (section 6 item 4).
const validatorThreshold = 0.8

// minValues is how many non-NULL values a column needs before a *ratio* over it
// means anything. One row that parses as an address is evidence about a row.
//
// It is not a floor on the net: a column below it is unproven, not clean, so
// secondnet.go runs the validators over what there is and fails on any hit
// (tracker T-0058). The number bounds what the threshold below is applied to,
// and nothing else. The two dictionary-backed validators are the exception in
// both directions -- see the dictionary rule below.
const minValues = 3

// nationalIDDigitsThreshold overrides validatorThreshold for the national_id
// digits entry alone (T-0187 review round, finding 1). ValidNationalIDDigits
// has no check digit at all — it is the SSA's own exclusion ranges applied to
// whatever 8 or 9 digits it is handed — so it is "SSN-shaped" about as often
// as a random number of that length clears those ranges: the review measured
// ~91% for a random 9-digit run.
//
// **It is not, on its own, a meaningful bar for an 8-digit YYYYMMDD date or a
// dense sequence of assigned numbers, and it is not asked to be any more**
// (T-0187 second review round, finding 1). The entry's own first version
// justified this threshold with "~97% of YYYYMMDD dates", a figure averaged
// over 1950-2050 where the only rejects are the two xx00 years whose
// two-digit year suffix collides with the SSA's excluded group 00; over any
// realistic booking range the clear rate is 1.0, which this threshold cannot
// sit above. `textsig.ValidNationalIDDigits` now excludes a value that is
// also a real calendar date directly (looksLikePlausibleDate,
// internal/textsig/nationalid.go) rather than leaving it to this ratio, and
// `digitRange` in secondnet.go excludes a column whose values pack into a
// dense numeric range — a generated sequence, key or not — independently of
// what internal/classify decided about the column. Those two are what
// actually answer the two shapes the review measured (1000/1000 on YYYYMMDD
// dates across 2022-2024, 10000/10000 on a sequential 9-digit non-key
// business number); this threshold is what is left over for a column that is
// neither a date nor a dense sequence and still clears the SSA ranges by
// chance more often than an ordinary heuristic should refuse a loaded target
// on, which the measured ~91%/9-digit and ~9%/8-digit random rates say can
// happen. Set above those, a random non-key, non-date, non-sequence numeric
// column mostly falls under it while a genuine leaked-identifier column —
// assigned numbers, which by construction almost never fall in an issuing
// authority's own excluded ranges — stays at or near 100% and still fails.
// testdata/regressions/021-ordinary-numeric-columns-clear-the-ssn-ratio.sql
// pins a dense surrogate id column, a dense non-key business-number column
// and an all-2024-dated booking column all at expect: ok with none of the
// three tuned to sit just under this threshold, 022-national-id-in-a-
// surrogate-key-column.sql pins a primary key of real, non-dense SSNs still
// refusing (finding 3), and 020-ssn-stored-as-bigint.sql (a genuine SSN,
// ratio 1.0, neither a date nor dense) still refuses at exit 9.
const nationalIDDigitsThreshold = 0.97

// validator is one category's value signal.
type validator struct {
	category pipeline.Category
	// name is what the report says the column validated as. It is a fixed
	// identifier, never a value.
	name string
	// text is true when the validator runs over character columns.
	text bool
	// digits is true when it runs over integer, bigint and numeric columns.
	digits bool
	// dict is true for the two validators backed by the name dictionary. They
	// are scored under the dictionary rule in secondnet.go rather than under
	// the plain ratio: a dictionary word is weaker evidence than a parse, and
	// this net's verdict is exit 9 on a target that is already loaded. It is
	// also what keeps them out of a document's leaves (applies, in
	// secondnet.go), because internal/classify runs no dictionary signal over a
	// document and this net may not refuse on evidence the classifier is
	// structurally unable to see.
	dict bool
	// strong is true for the six entries that are a precise parse rather
	// than a shape guess: email, phone, network_id (IP, MAC), the Luhn half
	// of financial_account, online_id (URL), and the structured half of
	// national_id (below). A hit from one of these is one value that IS the
	// thing it parses as, so secondnet.go fails the column on any hit at
	// all, whatever the ratio and whatever the column's size
	// (docs/reviews/2026-09-09/REVIEW.md finding 7,
	// docs/reviews/2026-09-09/evidence/sparse_email.log): one email address
	// among nineteen ordinary strings is still one email address in the
	// target. The IBAN half of financial_account is deliberately not strong
	// (see below), and neither are credential (looksSecret, an entropy
	// guess) and address (mixed digits and words, a shape guess): a
	// heuristic that fires on one occurrence in an ordinary
	// column would be exit 9 on a slug or a room number, which is the
	// direction §4's "when in doubt, mask it" does not require here because
	// the evidence is not precise enough to name a single value as personal
	// data. The two dictionary-backed validators are their own rule (dict,
	// above) and are never strong.
	strong bool
	// minRatio overrides validatorThreshold for one entry's ratio branch
	// (scoreHits's default case) when non-zero. It exists for the
	// national_id digits entry alone (T-0187 review round, finding 1):
	// ValidNationalIDDigits has no check digit at all, only the SSA's own
	// exclusion ranges, so it clears a meaningful fraction of a random 8- or
	// 9-digit number regardless of what it means (measured; see
	// nationalIDDigitsThreshold below), which is not what the ordinary
	// validatorThreshold of 0.8 is precise enough to decide alone.
	minRatio float64
	// sequenceExempt is true for the one entry a column's own *values* —
	// read by this net during the scan, never by re-deriving a decision
	// internal/classify already made — form a dense, order-independent
	// numeric range: again the national_id digits entry alone (T-0187
	// second review round, findings 1 and 3). Until the second review round
	// this was gated on internal/classify's surrogate-key decision instead
	// (netMode.surrogateKey, since removed), and that was two bugs at once —
	// finding 1's own probe planted an ordinary *non-key* dense business
	// number (400100000+i, no primary key or FK at all) that the
	// decision-based gate could never reach, and finding 3's planted a
	// primary key of *real* SSNs, which the decision-based gate exempted
	// outright because classify's own signals found nothing in it, turning
	// the net that exists to catch what classify missed into a no-op for
	// exactly that shape. Reading the range instead of the decision answers
	// both: a surrogate key's own values (id, id+1, id+2, ...) and an
	// ordinary dense business-number block are both dense whether or not
	// classify called either one a key, and a primary key of independently
	// assigned SSNs is not dense whatever classify decided about the column
	// being a key. See applies and digitRange in secondnet.go.
	sequenceExempt bool
	// requiresCorroboration is true for the one entry whose ratio, even after
	// nationalIDDigitsThreshold, looksLikePlausibleDate and sequenceExempt
	// above, is still not precise enough to refuse on alone: the national_id
	// digits entry (T-0187 third review round, finding 1). A sparse numeric
	// column with a fixed leading prefix and no check digit -- an
	// account_no or invoice_no column, neither dense nor a date -- clears
	// the SSA's exclusion ranges at or near 1.0 by construction, the same
	// way a genuine leaked SSN column does, and nothing above tells the two
	// apart.
	//
	// So this entry may refuse only when the column carries corroboration
	// beyond its own ratio, read off pipeline.Decision rather than
	// re-derived: a rules.yml national_id name-pattern hit on the column
	// (Decision.NameMatchedNationalID) or a certain-or-likely personal
	// column in the same table (Decision.TableHasLikelyPersonalColumn, the
	// neighbouring-column rule's own count). Without either, the ratio is
	// never asked at all -- see corroborated in secondnet.go.
	requiresCorroboration bool
	ok                    func(string) bool
}

// validators is the set: internal/classify's twelve value validators, in its
// own precedence order, folded into fourteen entries here (its two network
// validators, IP and MAC, share one). Its two financial validators used
// to share one too, until T-0136 split them back apart: Luhn and IBAN are
// both checksums over an arbitrary string, but Luhn's only ever matches a run
// of digits, where IBAN's matches fifteen to thirty-four letters-and-digits
// with the first two required to be letters -- pagila's own film titles carry
// five IBAN-shaped false positives ("CHARIOTS CONSPIRACY" passes the mod-97
// check) and zero Luhn-shaped ones, because nothing in an ordinary English
// title is a run of digits. IBAN is not strong.
//
// Luhn itself is then split a second way, by family rather than by strength
// (T-0136's review round, finding 2): the entry over character columns is
// `strong` (see its own comment below), and the entry over integer/bigint/
// numeric columns is not, so this file's own entry count runs two ahead of
// the classifier's for financial_account -- the two Luhn rows here answer
// for the classifier's one.
//
// national_id is split three ways (below, T-0187 review round, finding 2),
// which is why the classifier's twelve validators fold into fourteen entries
// and not thirteen: a structured, strong, text-family entry; a
// checksum-only, non-strong, text-family entry; and a digits-family entry
// that is neither strong nor at the ordinary threshold (nationalIDDigitsThreshold,
// above). The classifier's own single national_id entry answers for all
// three, the same asymmetry Luhn already has for two.
//
// Eleven, not the ten this comment said until tracker T-0122: internal/classify
// gained textsig.ValidURL ahead of its secrets validator when T-0100 stopped
// textsig.LooksSecret reading a URL as a credential, and this package was
// outside that task's paths. The online_id entry below is the counterpart, in
// the same position for the same reason. Between T-0100 and T-0122 a URL was
// the one value shape *neither* net could see: LooksSecret had stopped matching
// it and nothing here had replaced it, so a profile URI that reached the target
// unmasked passed email, phone, ip, mac, luhn, iban, LooksSecret, NameShape,
// AddressShape and ProseName alike. THREAT_MODEL.md T1 makes this net a
// blocking control for the column the 200-row sample under-represented, and the
// two packages score independently, so classify gaining the validator did not
// compensate.
//
// Every one of the twelve now has an entry here, and none is missing; two are
// answered by a *deliberately different* validator rather than copied, which is
// the dictionary rule and is the next paragraph.
//
// person_name and free_text were missing until tracker T-0055, because both
// read internal/classify's embedded name dictionary and this package may not
// import a stage package (internal/CLAUDE.md) and would not carry a second copy
// of a rule pack. The dictionary now lives in internal/textsig with the
// validators, so both are here -- and they read *different validators* from the
// classifier's and are *scored* differently, because the classifier's own
// validators and thresholds would fail a target on ordinary English words. The
// classifier asks LooksLikeName and Prose ("a dictionary word"); this net asks
// NameShape and ProseName ("a given name followed by a surname"), which a
// street name, a compound colour and a contract clause cannot carry. See the
// dictionary rule in secondnet.go.
var validators = []validator{
	{category: pipeline.CatEmail, name: "email", text: true, strong: true, ok: textsig.ValidEmail},
	// national_id joined at T-0187 (red team round 2, R2-01/A2, R2-02/A6,
	// R2-03/A7, R2-04/A9b: docs/reviews/2026-09-15-redteam/round2-still-
	// leaking.json). textsig.ValidNationalID was correct and recognised
	// every attack value; nothing on this net's own table ever called it, so
	// a plain SSN or NI number in a column whose name missed rules.yml's
	// pattern -- ref_code, ni_ref, code, a text[] of NI numbers, a JSON
	// document's only personal datum -- crossed into the target verbatim,
	// reported clean.
	//
	// It is now three entries, not one (T-0187's own review round, finding
	// 2), because "every one of the twelve formats is a checksum" turned out
	// to be true and not sufficient: six of them (PESEL, BSN, SIN, TFN,
	// Aadhaar's Verhoeff check, and CPF, whose own two check digits are
	// closer to these five's precision than to the other six's but still
	// carry no shape constraint -- textsig.ValidNationalIDChecksumOnly's own
	// comment has the reasoning) are a mod-N sum over an otherwise
	// unconstrained digit run, and a mod-N sum answers "yes" to a random
	// string of the right length far too often for a strong, any-hit-fails
	// entry: measured at 25.7% for a random 9-digit string, 11.0% for
	// 11-digit, 9.1% for 8-digit and 3.8% for an 8-digit-plus-letter code.
	// Marking the whole character-column entry strong meant one ordinary
	// nine-digit reference code in a varchar column -- a batch number, an
	// invoice code, nothing personal -- had roughly a one-in-four chance of
	// failing an already-loaded target at exit 9 with no ratio escape, the
	// same "an operator cannot act on" outcome the dictionary rule and the
	// Luhn split both argue against. So the character-column entry is split
	// the way T-0136 split Luhn, by evidence quality rather than only by
	// family: structured and strong (the six formats
	// textsig.ValidNationalIDStructured answers for -- a US SSN, a UK NINO,
	// an Italian codice fiscale, a Spanish DNI or NIE, a French NIR -- every
	// one of which also constrains the value's *shape*, so none of them
	// matches a bare digit run at all), and checksum-only and not strong
	// (the other six, scored under the ordinary ratio rule below like IBAN
	// and credential).
	{category: pipeline.CatNationalID, name: "national_id", text: true, strong: true, ok: textsig.ValidNationalIDStructured},
	{category: pipeline.CatNationalID, name: "national_id", text: true, ok: textsig.ValidNationalIDChecksumOnly},
	// The digits entry is the A9b half, and it mirrors Luhn's own text/digits
	// split (T-0136, this file's own comment below) for the same reason: an
	// SSN stored as bigint loses both its hyphens and (when the area starts
	// with 0) its leading digit, so textsig.ValidNationalIDStructured's
	// dashed regex never sees the same number twice -- 078-05-1001 renders
	// as the eight-digit 78051001, and ValidNationalIDDigits is the function
	// that recovers it. It is deliberately not strong: an SSN carries no
	// check digit at all, so a bare nine-digit number is "SSN-shaped" about
	// as often as a random nine-digit number clears the SSA's exclusion
	// ranges. ADR-010's numeric-family silencing does not reach national_id
	// in the first place: rules.yml's accepts: list for the category already
	// names bigint, integer and numeric (mask/gen_number.go's masker already
	// writes digits into them), so there is nothing to exempt.
	//
	// **It was not, on its own, precise enough for the ordinary ratio rule
	// either** (T-0187's own review round, finding 1): 91% of random 9-digit
	// numbers clear it (measured), which is well over validatorThreshold's
	// 0.8 -- so an ordinary unmasked bigint id column, or a booking-date
	// integer column, had no green path short of --unmask, the same outcome
	// the entry's own original comment said it was narrow enough to avoid
	// and was not.
	//
	// **The first fix (minRatio and surrogateExempt, a threshold raised
	// above the measured rate plus a skip keyed to internal/classify's own
	// surrogate-key decision) was not enough either, and the second review
	// round found both halves of why** (T-0187 second review round, findings
	// 1 and 3). The 97% figure the threshold was set above was itself wrong
	// -- an artefact of averaging over 1950-2050, where over any realistic
	// booking range the true clear rate is 1.0 -- so an ordinary YYYYMMDD
	// date column still refused with no green path (finding 1). And gating
	// the skip on classify's decision inherited classify's own miss: a
	// primary key of real SSNs that classify's name and value signals found
	// nothing in was exempted from the net that exists to catch exactly what
	// classify missed (finding 3) -- before this file's surrogate-key
	// exemption existed, that same column correctly refused at exit 9.
	//
	// The fix is now in three parts, and none of them is this file's ratio
	// alone. `textsig.ValidNationalIDDigits` excludes a value that is also a
	// real calendar date directly (nationalid.go's
	// looksLikePlausibleDate), which is what actually answers the date case
	// -- a column of real dates now scores zero hits regardless of this
	// threshold. `digitRange` in secondnet.go reads the column's own values,
	// never internal/classify's decision, and exempts a column whose values
	// pack into a dense numeric range: a surrogate key's own values and an
	// ordinary dense business-number block (no key at all) are both dense,
	// and a primary key of independently assigned SSNs is not, which closes
	// finding 3 by construction rather than by asking classify a second
	// time. `nationalIDDigitsThreshold` is what is left for this validator's
	// ratio to decide once those two run first: a column that is neither a
	// date nor a dense sequence and still clears the SSA exclusion ranges on
	// more than 97% of its values.
	//
	// **That was still not the whole answer (the T-0187 third review round,
	// finding 1).** A *sparse* numeric column with a fixed leading prefix and
	// no check digit clears the exclusion ranges at essentially 1.0 whatever
	// the threshold is set to, and it is neither a date nor dense: 500
	// account numbers of the form 100000000+rand(1e8) measured 494/500
	// (ratio 0.988, over nationalIDDigitsThreshold's 0.97), and 500 invoice
	// numbers of the form 202600000+7*rand(50000) measured 500/500. Neither
	// shape is personal data, and an ordinary invoice_no or account_no
	// column of either shape refused an already-loaded run at exit 9 with no
	// green path short of --unmask -- the same outcome minRatio was raised
	// to avoid and, for a sparse fixed-prefix column, cannot: there is no
	// threshold under 1.0 that admits it and still refuses a real leaked
	// identifier column, because assigned identifiers clear the same ranges
	// at the same rate.
	//
	// So a ratio, however tuned, is not this entry's whole answer, and it
	// stops being asked at all without requiresCorroboration's own two
	// signals (below): a genuine leak is corroborated by rules.yml's
	// national_id name pattern or by a proven personal neighbour in the same
	// table, and an ordinary reference-number column, sparse or dense, named
	// so neither pattern matches and sitting in a table with no personal
	// column beside it, is not. testdata/regressions/023-sparse-fixed-prefix-
	// reference-block-is-not-national-id.sql is the shape this finding is
	// named for; 020 and 022 both needed a corroborating column added for the
	// same reason (020's own header says why `taxref` alone could not be
	// it), and 021 needs none, because its three columns are already
	// excluded by the date and dense-range checks before corroboration is
	// ever asked.
	{
		category: pipeline.CatNationalID, name: "national_id", digits: true,
		minRatio: nationalIDDigitsThreshold, sequenceExempt: true, requiresCorroboration: true,
		ok: textsig.ValidNationalIDDigits,
	},
	// ok stays textsig.ValidPhone, the international-only ("ZZ") reading:
	// count (secondnet.go) is where the region-aware widening lives, because
	// this entry's ok is a plain func(string) bool with no room for a
	// per-run region and the second net's own Options carries one now
	// (T-0221). When Options.PhoneRegion is set, count also asks
	// textsig.ValidPhoneRegion under it, on this entry's same strong,
	// any-hit-fails footing -- never internal/classify's own guessed-region
	// list, which stays corroboration-gated on that side only (Options.
	// PhoneRegion's own comment has the reason).
	{category: pipeline.CatPhone, name: "phone", text: true, strong: true, ok: textsig.ValidPhone},
	{category: pipeline.CatNetworkID, name: "network_id", text: true, strong: true, ok: func(s string) bool {
		return textsig.ValidIP(s) || textsig.ValidMAC(s)
	}},
	// Luhn is split by family, not just by strength (T-0136 review finding 2).
	// A 12-to-19-digit run inside a *character* column that passes the check
	// digit is a precise parse -- a payment card number typed into a text
	// field -- so that side stays strong: any hit at all fails the column,
	// as it did before this split. An *integer/bigint/numeric* column is a
	// different claim: roughly one in ten 12-to-19-digit identifiers passes
	// Luhn by chance (snowflake IDs, epoch-millisecond timestamps, EAN-13
	// barcodes, order numbers), so an ordinary unmasked bigint id column of
	// any realistic size contains at least one hit, and a `strong` digits
	// entry would fail nearly every such column with no ratio escape and no
	// green path short of --unmask on a column that holds no personal data
	// -- the outcome the dictionary-rule paragraph above and
	// internal/verify/CLAUDE.md's "an operator cannot act on" sentence both
	// argue against, and the state a column lands in once internal/classify
	// stops routing it to an unwritable free_text (T-0136 review finding 1).
	// So the digits side keeps the ratio rule instead: strong stays false,
	// and validatorThreshold (with the any-hit-below-minValues floor,
	// T-0058) is what decides it, same as IBAN and every other non-strong
	// entry.
	{category: pipeline.CatFinancial, name: "financial_account", text: true, strong: true, ok: textsig.ValidLuhn},
	{category: pipeline.CatFinancial, name: "financial_account", digits: true, ok: textsig.ValidLuhn},
	// IBAN is a checksum over letters and digits, not a run of digits, so it
	// keeps the ratio rule rather than joining Luhn as strong: an ordinary
	// all-caps title or slug is about as likely to be fifteen-to-thirty-four
	// letters-and-digits passing a mod-97 check as any other string of that
	// shape is, and one occurrence of that is not the same claim as one
	// occurrence of a value that parses as a payment card number.
	{category: pipeline.CatFinancial, name: "financial_account", text: true, ok: textsig.ValidIBAN},
	// Ahead of the credential entry, which is internal/classify's order and, as
	// there, the whole of tracker T-0100: a URL clears every guard in
	// textsig.LooksSecret, so before that task mastodon's accounts.uri read as
	// `credential` on every row. A URL that names a person is an online_id.
	{category: pipeline.CatOnlineID, name: "online_id", text: true, strong: true, ok: textsig.ValidURL},
	// credential is deliberately not strong: textsig.LooksSecret is an entropy
	// guess (16+ characters, two character classes, no space, no "@"), not a
	// parse, so one occurrence in an ordinary column is not evidence that the
	// value is a credential the way one occurrence of a valid email address is
	// evidence that it is an email address.
	{category: pipeline.CatCredential, name: "credential", text: true, ok: textsig.LooksSecret},
	// The two dictionary-backed ones keep internal/classify's precedence:
	// person_name before address, free_text last, so a note that mentions a
	// street is prose and an address that parses as one is an address. Both ask
	// internal/textsig's narrow shapes, never the classifier's own two: a value
	// of two dictionary words is not a person here unless one of them is a
	// given name followed by a surname, and a sentence is not prose here unless
	// it carries that same pair (tracker T-0055 and its review).
	{category: pipeline.CatPersonName, name: "person_name", text: true, dict: true, ok: func(s string) bool {
		return textsig.Dictionary().NameShape(s)
	}},
	{category: pipeline.CatAddress, name: "address", text: true, ok: textsig.AddressShape},
	{category: pipeline.CatFreeText, name: "free_text", text: true, dict: true, ok: func(s string) bool {
		return textsig.Dictionary().ProseName(s)
	}},
	// special_category joined at T-0231: T-0198 gave pipeline.CatSpecial a
	// value validator (textsig.SpecialCategoryVocabulary) and wired it into
	// both DDL-literal passes (internal/plan/ddlliteral.go's
	// strongValidators, internal/verify/catalog.go's strongCatalogHit), but
	// this file -- the row-scanning second net, ARCHITECTURE.md section 6
	// item 4 -- carried no entry for it, so a digit/name-free
	// special-category sentence in an unmasked column's ROW value still
	// crossed this net unseen: internal/classify masks the category on the
	// column's *name* alone (section 4, "special categories mask on name
	// alone"), which is exactly the miss THREAT_MODEL.md T1 names this net
	// as the control for. It is a vocabulary match, not a checksum or a
	// dictionary shape, so it is scored at the ordinary ratio rather than as
	// strong (one hit in an otherwise-ordinary column is not a certain
	// diagnosis the way one valid email address is certainly an email
	// address) and never as dict (a term list is not the name dictionary,
	// and this validator is precise enough over a document's leaves the way
	// address and credential already are — see applies, secondnet.go).
	{category: pipeline.CatSpecial, name: "special_category", text: true, ok: textsig.SpecialCategoryVocabulary},
}
