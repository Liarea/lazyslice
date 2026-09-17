// SPDX-License-Identifier: Apache-2.0

package textsig

import (
	"regexp"
	"strings"
	"unicode"
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

// normalizeSpecialCategoryCandidate reduces s to a lowercase, space-separated
// run of its alphanumeric content before reSpecialCategoryTerm ever sees it
// (round-5 red team, docs/reviews/2026-09-15-redteam/round5-still-leaking.json:
// the secrets attacker's catalog variant): \b on both sides of the
// alternation is a *word*-boundary in Go's regexp, as in PCRE, and `_` is a
// word character there, so `HIV_POSITIVE` -- the spelling an application
// status code or enum label actually takes, not prose -- has no boundary
// between "HIV" and "_POSITIVE" and never matched, alongside HIV_STATUS and
// TRADE_UNION_MEMBER. Every run of a non-alphanumeric character (underscore,
// hyphen, dot, ...) collapses to one space, and a camel-case boundary opens
// one too -- both the ordinary lower-to-Upper one ("hivPositive") and the
// acronym one, an upper rune immediately followed by an upper-then-lower run
// ("HIVPositive", "AIDSDiagnosis", "LGBTQMember": T-0254 review, medium
// finding 3, the immediate next spelling the round-5 canary's own reasoning
// covers but its first landing did not implement) -- so "HIV_POSITIVE"
// reduces to "hiv positive" (matches "hiv") and "TRADE_UNION_MEMBER" reduces
// to "trade union member" (matches "trade union" and "union member"). An
// all-caps glued spelling such as HIVSTATUS opens no boundary at all and
// stays unsplittable; that is a residual, not a claim this function makes.
//
// This reduction is NOT a strict widening of what s itself exposes, and
// SpecialCategoryVocabulary below does not treat it as one (T-0254 review,
// high finding 2): a case difference turning into lowercase is a widening,
// but a camel-case boundary opening a space is not -- it can turn a
// previously-matching glued spelling into two words neither of which
// matches, which is exactly what happened to "TransGender", "BiSexual",
// "HomoSexual", "HeteroSexual" and "UnionIzed" once this function started
// inserting the ordinary lower-to-Upper boundary: each of those already
// matched reSpecialCategoryTerm as raw, unreduced text (no separator, no
// case difference reSpecialCategoryTerm's own \b does not already tolerate),
// and reducing them split "trans" from "gender" and so on. The two
// boundary rules stay, because HIV_POSITIVE's family needs them; the
// narrowing they can cause is corrected by testing the raw string as well,
// below.
func normalizeSpecialCategoryCandidate(s string) string {
	var b strings.Builder
	runes := []rune(s)
	for i, r := range runes {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			if i > 0 {
				prev := runes[i-1]
				switch {
				case unicode.IsLower(prev) && unicode.IsUpper(r):
					b.WriteByte(' ')
				case unicode.IsUpper(prev) && unicode.IsUpper(r) &&
					i+1 < len(runes) && unicode.IsLower(runes[i+1]):
					b.WriteByte(' ')
				}
			}
			b.WriteRune(unicode.ToLower(r))
		default:
			b.WriteByte(' ')
		}
	}
	return b.String()
}

// SpecialCategoryVocabulary reports whether s carries one of the terms above.
// It is a value shape in the sense the rest of this package uses the word —
// func(string) bool, no column, no category, no threshold — and a caller
// attaches pipeline.CatSpecial to a hit the way every other validator's
// caller attaches its own category.
//
// The vocabulary is matched over s directly as well as over its reduction
// (T-0254 review, high finding 2): normalizeSpecialCategoryCandidate's own
// comment explains why the reduction is not a strict widening of s, and
// testing both is what keeps a term the raw text already carried un-narrowed
// by a camel-case split the reduction introduces for an entirely different
// spelling's sake.
func SpecialCategoryVocabulary(s string) bool {
	return reSpecialCategoryTerm.MatchString(s) || reSpecialCategoryTerm.MatchString(normalizeSpecialCategoryCandidate(s))
}
