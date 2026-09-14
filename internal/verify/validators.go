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
	// strong is true for the five entries that are a precise parse rather
	// than a shape guess: email, phone, network_id (IP, MAC), the Luhn half
	// of financial_account, and online_id (URL). A hit from one of these is
	// one value that IS the thing it parses as, so secondnet.go fails the
	// column on any hit at all, whatever the ratio and whatever the column's
	// size (docs/reviews/2026-09-09/REVIEW.md finding 7,
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
	ok     func(string) bool
}

// validators is the set: all eleven of internal/classify's value validators, in
// its own precedence order, folded into eleven entries here (its two network
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
// numeric columns is not, so this file's own entry count is eleven where the
// classifier's stays at eleven distinct validators -- the two Luhn rows here
// answer for the classifier's one.
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
// Every one of the eleven now has an entry here, and none is missing; two are
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
}
