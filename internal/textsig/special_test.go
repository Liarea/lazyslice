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
