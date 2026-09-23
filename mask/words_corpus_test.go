// SPDX-License-Identifier: Apache-2.0

package mask

import (
	"regexp"
	"strings"
	"testing"
)

// wantCensusGivenWordCount and wantCensusSurnameWordCount pin the counts
// tools/names produced from the 2020 Census top-1000 files at its default
// cut, -given-per-sex 500 -surnames 1000 (tools/names/README.md,
// THIRD_PARTY_NOTICES.md): 958 given names -- the union of the top-500 most
// common male and top-500 most common female first names, deduped -- and all
// 1,000 surnames. Since T-0304 these are the lists every person_name role
// and every email local part draws from (words.go's givenNames and
// surnames), so a silent regeneration with a different cut, a different
// source file or a filtering change would change every masked name; this test
// turns that into a loud failure before role_test.go's Domain counts do.
const (
	wantCensusGivenWordCount   = 958
	wantCensusSurnameWordCount = 1000
)

var censusLowerLetters = regexp.MustCompile(`^[a-z]+$`)

func TestCensusWordListsMatchTheirPinnedCut(t *testing.T) {
	cases := []struct {
		name  string
		words string
		want  int
	}{
		{"censusGivenWords", censusGivenWords, wantCensusGivenWordCount},
		{"censusSurnameWords", censusSurnameWords, wantCensusSurnameWordCount},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			words := strings.Fields(tc.words)
			if len(words) != tc.want {
				t.Fatalf("%s: %d words, want %d -- tools/names' cut changed; if that is intended, update this "+
					"test, THIRD_PARTY_NOTICES.md and tools/names/README.md together (run `make names` first)",
					tc.name, len(words), tc.want)
			}

			seen := make(map[string]bool, len(words))
			for _, w := range words {
				if !censusLowerLetters.MatchString(w) {
					t.Errorf("%s: %q is not all-lowercase-letters (^[a-z]+$)", tc.name, w)
				}
				if len(w) > maxFit {
					t.Errorf("%s: %q is %d bytes, over mask's maxFit of %d", tc.name, w, len(w), maxFit)
				}
				if seen[w] {
					t.Errorf("%s: %q appears more than once", tc.name, w)
				}
				seen[w] = true
			}
		})
	}
}
