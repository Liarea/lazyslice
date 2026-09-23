// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"go/format"
	"io"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// maxWordLen mirrors mask.maxFit (mask/words.go): the largest length mask's
// fit table indexes. A census entry over this length would silently fall
// outside every budget mask ever offers a caller, so it is a hard failure
// here rather than a silent narrowing that only some future column would
// ever notice.
const maxWordLen = 32

// lowerLetters is the one shape a word in either list is allowed to have,
// after lowercasing: ^[a-z]+$. A token outside it -- an apostrophe, a
// hyphen, a space in a two-word entry -- is dropped and counted rather than
// rejected outright, because tools/names/README.md's fetch found none in
// either source file and a future re-fetch that does should not itself be
// the failure; corpusStats.GivenDrops/SurnameDrops carry the count and
// TestNoDrops (main_test.go) asserts it is zero against the CSVs actually
// checked in.
var lowerLetters = regexp.MustCompile(`^[a-z]+$`)

// corpusStats is what generate did, for -check's error message and for
// TestNoDrops.
type corpusStats struct {
	GivenWords   []string
	GivenDrops   int
	SurnameWords []string
	SurnameDrops int
}

// generate reads the two checked-in CSVs and renders mask/words_corpus.go's
// full, gofmt-formatted source.
func generate(firstPath, lastPath string, givenPerSex, surnameCount int) ([]byte, corpusStats, error) {
	given, givenDrops, err := selectGivenNames(firstPath, givenPerSex)
	if err != nil {
		return nil, corpusStats{}, err
	}
	surnames, surnameDrops, err := selectSurnames(lastPath, surnameCount)
	if err != nil {
		return nil, corpusStats{}, err
	}

	stats := corpusStats{
		GivenWords:   given,
		GivenDrops:   givenDrops,
		SurnameWords: surnames,
		SurnameDrops: surnameDrops,
	}

	rendered := renderSource(given, surnames)
	src, err := format.Source([]byte(rendered))
	if err != nil {
		return nil, corpusStats{}, fmt.Errorf("gofmt of the generated source: %w", err)
	}
	return src, stats, nil
}

// selectGivenNames reads tools/names/census2020_first_names_sex_top1000.csv
// (columns: name, sex, rank, count -- one row per Census-reported first name
// per sex, ranked by that sex's own count within the file's top 1,000
// overall names; tools/names/README.md), takes every row with rank<=perSex
// for sex "male" and every row with rank<=perSex for sex "female", lowercases
// each name, drops (and counts) any that is not ^[a-z]+$, fails on any word
// over maxWordLen, and returns the union of what is left -- deduped, since a
// name can rank in the top perSex for both sexes -- sorted alphabetically.
func selectGivenNames(path string, perSex int) ([]string, int, error) {
	f, err := os.Open(path) // #nosec G304 -- path is one of the two fixed checked-in CSVs under repoRoot()
	if err != nil {
		return nil, 0, err
	}
	defer f.Close()

	r := csv.NewReader(f)
	header, err := r.Read()
	if err != nil {
		return nil, 0, fmt.Errorf("%s: %w", path, err)
	}
	wantHeader := []string{"name", "sex", "rank", "count"}
	if !equalStrings(header, wantHeader) {
		return nil, 0, fmt.Errorf("%s: header is %v, want %v", path, header, wantHeader)
	}

	set := make(map[string]bool)
	drops := 0
	for {
		rec, readErr := r.Read()
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return nil, 0, fmt.Errorf("%s: %w", path, readErr)
		}
		name, sex, rankField := rec[0], rec[1], rec[2]
		rank, convErr := strconv.Atoi(rankField)
		if convErr != nil {
			return nil, 0, fmt.Errorf("%s: %s: bad rank %q: %w", path, name, rankField, convErr)
		}
		if sex != "male" && sex != "female" {
			return nil, 0, fmt.Errorf("%s: %s: unexpected sex %q, want \"male\" or \"female\"", path, name, sex)
		}
		if rank > perSex {
			continue
		}
		word := strings.ToLower(name)
		if !lowerLetters.MatchString(word) {
			drops++
			continue
		}
		if len(word) > maxWordLen {
			return nil, 0, fmt.Errorf("%s: %q is %d bytes, over mask's maxFit of %d", path, word, len(word), maxWordLen)
		}
		set[word] = true
	}

	return sortedKeys(set), drops, nil
}

// selectSurnames reads tools/names/census2020_last_names_top1000.csv
// (columns: name, rank, count -- tools/names/README.md), takes every row
// with rank<=n, lowercases each name, drops (and counts) any that is not
// ^[a-z]+$, fails on any word over maxWordLen, and returns the result
// deduped and sorted alphabetically.
func selectSurnames(path string, n int) ([]string, int, error) {
	f, err := os.Open(path) // #nosec G304 -- path is one of the two fixed checked-in CSVs under repoRoot()
	if err != nil {
		return nil, 0, err
	}
	defer f.Close()

	r := csv.NewReader(f)
	header, err := r.Read()
	if err != nil {
		return nil, 0, fmt.Errorf("%s: %w", path, err)
	}
	wantHeader := []string{"name", "rank", "count"}
	if !equalStrings(header, wantHeader) {
		return nil, 0, fmt.Errorf("%s: header is %v, want %v", path, header, wantHeader)
	}

	set := make(map[string]bool)
	drops := 0
	for {
		rec, readErr := r.Read()
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return nil, 0, fmt.Errorf("%s: %w", path, readErr)
		}
		name, rankField := rec[0], rec[1]
		rank, convErr := strconv.Atoi(rankField)
		if convErr != nil {
			return nil, 0, fmt.Errorf("%s: %s: bad rank %q: %w", path, name, rankField, convErr)
		}
		if rank > n {
			continue
		}
		word := strings.ToLower(name)
		if !lowerLetters.MatchString(word) {
			drops++
			continue
		}
		if len(word) > maxWordLen {
			return nil, 0, fmt.Errorf("%s: %q is %d bytes, over mask's maxFit of %d", path, word, len(word), maxWordLen)
		}
		set[word] = true
	}

	return sortedKeys(set), drops, nil
}

func sortedKeys(set map[string]bool) []string {
	words := make([]string, 0, len(set))
	for w := range set {
		words = append(words, w)
	}
	sort.Strings(words)
	return words
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// wrapWidth mirrors the line width mask/words.go's own hand-wrapped word
// lists use, so a regenerated file reads the same way a hand-edited one
// would have.
const wrapWidth = 96

// wrapWords joins words with a single space, wrapping to wrapWidth so no
// line (before the leading tab go/format.Source adds) runs long, the same
// shape mask/words.go's givenWords/surnameWords already have.
func wrapWords(words []string) string {
	var b strings.Builder
	lineLen := 0
	for i, w := range words {
		switch {
		case i == 0:
			// no separator before the first word
		case lineLen+1+len(w) > wrapWidth:
			b.WriteByte('\n')
			lineLen = 0
		default:
			b.WriteByte(' ')
			lineLen++
		}
		b.WriteString(w)
		lineLen += len(w)
	}
	return b.String()
}

// renderSource builds mask/words_corpus.go's full source as text; generate
// hands it to go/format.Source for the final, canonical formatting.
func renderSource(given, surnames []string) string {
	const tick = "`"
	var b bytes.Buffer
	fmt.Fprintf(&b, "// SPDX-License-Identifier: Apache-2.0\n\n")
	fmt.Fprintf(&b, "// Code generated by tools/names; DO NOT EDIT.\n\n")
	fmt.Fprintf(&b, "// censusGivenWords and censusSurnameWords are the U.S. Census Bureau's 2020\n")
	fmt.Fprintf(&b, "// Census top-1000 given names and surnames (tools/names/README.md,\n")
	fmt.Fprintf(&b, "// THIRD_PARTY_NOTICES.md), lowercased: %d given names -- the union of the\n", len(given))
	fmt.Fprintf(&b, "// top-ranked male and top-ranked female first names, deduped -- and %d\n", len(surnames))
	fmt.Fprintf(&b, "// surnames. Regenerate with `make names` (go run ./tools/names); never edit by\n")
	fmt.Fprintf(&b, "// hand.\n//\n")
	fmt.Fprintf(&b, "// Nothing in mask reads either constant yet: this task (T-0303) only fetches,\n")
	fmt.Fprintf(&b, "// extracts and generates the corpus, and mask/CLAUDE.md says why no masker is\n")
	fmt.Fprintf(&b, "// wired to it. T-0304 does that, after T-0302 lands.\n")
	fmt.Fprintf(&b, "package mask\n\n")
	fmt.Fprintf(&b, "const (\n")
	fmt.Fprintf(&b, "\tcensusGivenWords = %s%s%s\n\n", tick, wrapWords(given), tick)
	fmt.Fprintf(&b, "\tcensusSurnameWords = %s%s%s\n", tick, wrapWords(surnames), tick)
	fmt.Fprintf(&b, ")\n")
	return b.String()
}
