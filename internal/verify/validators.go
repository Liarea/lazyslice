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
	ok   func(string) bool
}

// validators is the set: all ten of internal/classify's value validators, in
// its own precedence order, folded into eight entries (its two financial
// validators share one here, and so do its two network ones).
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
	{category: pipeline.CatEmail, name: "email", text: true, ok: textsig.ValidEmail},
	{category: pipeline.CatPhone, name: "phone", text: true, ok: textsig.ValidPhone},
	{category: pipeline.CatNetworkID, name: "network_id", text: true, ok: func(s string) bool {
		return textsig.ValidIP(s) || textsig.ValidMAC(s)
	}},
	{category: pipeline.CatFinancial, name: "financial_account", text: true, digits: true, ok: func(s string) bool {
		return textsig.ValidLuhn(s) || textsig.ValidIBAN(s)
	}},
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
