// SPDX-License-Identifier: Apache-2.0

package textsig

import (
	"regexp"
	"strings"
)

// SpecialCategoryVocabulary is the value half of pipeline.CatSpecial (tracker
// T-0198). internal/classify masks a special-category column on its NAME
// alone (rules.yml's own pattern, ARCHITECTURE.md §4: "special categories mask
// on name alone"), and until this file existed nothing anywhere had ever
// asked what a special-category VALUE looks like — so a DDL literal or a row
// value that happened to also carry a digit or a dictionary name pair was
// caught by accident (AddressShape's digit, Dict.ProseName's name adjacency),
// and one that carried neither was not. The round-3 red team's own canaries
// are the reason this file exists: `CHECK (diagnosis <> 'HIV positive, CD4
// 210')` and `CHECK (religion <> 'Ahmadiyya Muslim')`
// (docs/reviews/2026-09-15-redteam/round3-still-leaking.json, finding 15) are
// both short of ProseName's six-word floor and shaped like neither a name nor
// an address, so every validator internal/plan and internal/verify already
// carried returned nothing, and both crossed into a masked column's own CHECK
// constraint under exit 0.
//
// This is a vocabulary, not a parse: the special categories GDPR Art. 9 and
// rules.yml's own pattern name — health, religion or belief, sexual
// orientation and gender identity, ethnicity or race, trade-union membership,
// political opinion — have no shape a checksum or a regular parse can decide,
// only words. A word list scored over free English prose is exactly the kind
// of signal this package's two dictionary validators (NameShape, ProseName)
// are already deliberately narrow about, for the identical reason: a hit here
// is a one-occurrence refusal, at plan with no rewrite arm at all or at
// verify on an already-loaded target, so it has to be precision over recall
// or it refuses an ordinary schema over an ordinary word. Every term below
// was chosen because it is rarely if ever a business label or an everyday
// English word standing alone — a disease or condition name, a named religion
// or its adjective, a term for sexual orientation or gender identity, a
// multi-word trade-union phrase (never the bare word "union": a credit union,
// a students' union and a company literally named Union carry it too), a
// named political party or "political affiliation" itself, and a small set
// of ethnicity/descent terms. It is deliberately not wide enough to reach
// every value a person could write about themselves in one of these
// categories — recall is the row pipeline's job, already covered by masking
// on the column's name — and this validator's only job is telling a DDL
// literal or a row value that plainly carries one of these terms from one
// that does not.
var specialCategoryTerms = []string{
	// health condition, diagnosis or disability
	"hiv", "aids", "cancer", "diabetes", "epilepsy", "schizophrenia",
	"bipolar disorder", "clinical depression", "chronic illness",
	"terminal illness", "mental illness", "psychiatric",
	"diagnosed", "diagnosis",

	// religion or belief
	"muslim", "christian", "catholic", "protestant", "jewish", "hindu",
	"buddhist", "sikh", "atheist", "agnostic", "islam", "christianity",
	"judaism", "hinduism", "buddhism",

	// sexual orientation and gender identity
	"gay", "lesbian", "bisexual", "transgender", "heterosexual",
	"homosexual", "queer", "lgbtq", "lgbt",

	// ethnicity or race
	"hispanic", "latino", "latina", "caucasian", "african american",
	"asian american", "indigenous", "aboriginal",

	// trade-union membership
	"trade union", "labor union", "labour union", "union member",
	"unionized", "collective bargaining",

	// political opinion
	"republican", "democrat", "conservative party", "labour party",
	"socialist", "communist", "libertarian", "political affiliation",
	"political party",
}

// reSpecialCategoryTerm matches any term above as a whole word or phrase,
// case-insensitively. \b on both sides of the alternation is what keeps a
// phrase like "trade union" from matching inside a longer identifier, and
// what keeps a short term like "gay" from matching inside one ("Uruguayan").
var reSpecialCategoryTerm = regexp.MustCompile(`(?i)\b(?:` + strings.Join(quoteTerms(specialCategoryTerms), "|") + `)\b`)

func quoteTerms(terms []string) []string {
	out := make([]string, len(terms))
	for i, t := range terms {
		out[i] = regexp.QuoteMeta(t)
	}
	return out
}

// SpecialCategoryVocabulary reports whether s carries one of the terms above.
// It is a value shape in the sense the rest of this package uses the word —
// func(string) bool, no column, no category, no threshold — and a caller
// attaches pipeline.CatSpecial to a hit the way every other validator's
// caller attaches its own category.
func SpecialCategoryVocabulary(s string) bool {
	return reSpecialCategoryTerm.MatchString(s)
}
