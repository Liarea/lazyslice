// SPDX-License-Identifier: Apache-2.0

package main

import "testing"

// TestNoDrops runs the real filter over the two checked-in CSVs at their
// default cut (-given-per-sex 500 -surnames 1000) and asserts it drops
// nothing: tools/names/README.md records that the fetch found every name in
// both source files already lowercase-letters-only, and this is the guard
// that a future re-fetch which is not stays loud rather than silently
// shrinking the corpus mask/words_corpus_test.go pins.
func TestNoDrops(t *testing.T) {
	_, stats, err := generate(
		"census2020_first_names_sex_top1000.csv",
		"census2020_last_names_top1000.csv",
		500, 1000,
	)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if stats.GivenDrops != 0 {
		t.Errorf("given names: %d dropped for not matching ^[a-z]+$ after lowercasing, want 0", stats.GivenDrops)
	}
	if stats.SurnameDrops != 0 {
		t.Errorf("surnames: %d dropped for not matching ^[a-z]+$ after lowercasing, want 0", stats.SurnameDrops)
	}
}
