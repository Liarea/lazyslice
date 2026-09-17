// SPDX-License-Identifier: Apache-2.0

package textsig

import "testing"

// The round-3 red team's own canaries (tracker T-0198,
// docs/reviews/2026-09-15-redteam/round3-still-leaking.json, finding 15):
// neither is long enough for Dict.ProseName's six-word floor or shaped like a
// name or an address, so both crossed a masked column's own CHECK under exit
// 0 until this validator existed.
func TestSpecialCategoryVocabularyCatchesTheRedTeamCanaries(t *testing.T) {
	t.Parallel()

	cases := []string{
		"HIV positive, CD4 210",
		"Ahmadiyya Muslim",
		"Priya Raghunathan disclosed her HIV diagnosis on 2019-04-02",
		"diagnosed with schizophrenia last spring",
		"a practising Catholic",
		"identifies as bisexual",
		"a member of the trade union",
		"registered Republican",
		// Round-5 red team canaries
		// (docs/reviews/2026-09-15-redteam/round5-still-leaking.json): the
		// spelling a status code or enum label actually takes, glued by
		// underscores rather than spaced out as prose -- \b treats `_` as a
		// word character, so these never matched before normalisation.
		"HIV_POSITIVE",
		"HIV_STATUS",
		"TRADE_UNION_MEMBER",
		// T-0254 review, high finding 2: camel-case space insertion narrowed
		// SpecialCategoryVocabulary for these -- each already matched as raw,
		// unreduced text (no separator, only a case difference) before the
		// normalisation added a lower-to-Upper boundary that split it apart.
		"TransGender",
		"BiSexual",
		"HomoSexual",
		"HeteroSexual",
		"UnionIzed",
		"unionIzed",
		"aIDS",
		// T-0254 review, medium finding 3: the acronym camel-case boundary
		// (an uppercase run followed by Upper+lower), the immediate next
		// spelling of the round-5 canary the first landing missed half of.
		"HIVPositive",
		"HIVStatus",
		"AIDSDiagnosis",
		"LGBTQMember",
		"isHIVPositive",
	}
	for _, s := range cases {
		if !SpecialCategoryVocabulary(s) {
			t.Errorf("SpecialCategoryVocabulary(%q) = false, want true", s)
		}
	}
}

// Precision over recall: ordinary business prose, status labels and adjacent
// English words that are not this category must not hit, or the two catalog
// passes that call this refuse an ordinary schema over a word that was never
// personal data.
func TestSpecialCategoryVocabularyLeavesOrdinaryProseAlone(t *testing.T) {
	t.Parallel()

	cases := []string{
		"active",
		"a conservative estimate of Q4 revenue",
		"child labour laws in this jurisdiction",
		"the European Union trade agreement",
		"a credit union branch",
		"the Great Depression of 1929",
		"a bipolar transistor amplifier",
		"Basic 1 user",
		"tier 2 plus",
		"1742 Kestrel Hollow Lane, Ashford VT 05024",
	}
	for _, s := range cases {
		if SpecialCategoryVocabulary(s) {
			t.Errorf("SpecialCategoryVocabulary(%q) = true, want false", s)
		}
	}
}
