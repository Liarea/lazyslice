// SPDX-License-Identifier: Apache-2.0

// Command names regenerates mask/words_corpus.go from the two CSVs this
// directory already carries, tools/names/census2020_first_names_sex_top1000.csv
// and tools/names/census2020_last_names_top1000.csv -- the U.S. Census
// Bureau's 2020 Census top-1000 name files, extracted once by hand.
// tools/names/README.md records the fetch: the source URLs, each file's
// sha256, its column headers and the extraction command used.
//
// Nothing here is downloaded at build or test time: both CSVs are checked
// in, so `go run ./tools/names` and `go run ./tools/names -check` are
// offline, and `make names`/`make names-check` (wired into `make check` and
// into ci.yml's docs job) run them that way.
//
// mask/words_corpus.go is data only, as of this task: no masker reads
// censusGivenWords or censusSurnameWords yet -- see mask/CLAUDE.md's note on
// why, and tracker T-0304, which wires them in.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "names: %s\n", err)
		os.Exit(1)
	}
}

func run() error {
	givenPerSex := flag.Int("given-per-sex", 500,
		"how many top-ranked male given names and top-ranked female given names to take, before the union and dedupe")
	surnameCount := flag.Int("surnames", 1000, "how many top-ranked surnames to take")
	check := flag.Bool("check", false,
		"regenerate into memory and exit 1 if mask/words_corpus.go differs, without writing it")
	flag.Parse()

	root, err := repoRoot()
	if err != nil {
		return err
	}

	firstPath := filepath.Join(root, "tools", "names", "census2020_first_names_sex_top1000.csv")
	lastPath := filepath.Join(root, "tools", "names", "census2020_last_names_top1000.csv")
	outPath := filepath.Join(root, "mask", "words_corpus.go")

	src, stats, err := generate(firstPath, lastPath, *givenPerSex, *surnameCount)
	if err != nil {
		return err
	}

	if *check {
		existing, readErr := os.ReadFile(outPath) // #nosec G304 -- outPath is repoRoot()-derived, not user input
		if readErr != nil {
			return fmt.Errorf("reading %s: %w", outPath, readErr)
		}
		if !bytes.Equal(existing, src) {
			return fmt.Errorf(
				"%s is out of date with tools/names' own CSVs (given=%d words/%d drops, surnames=%d words/%d drops); "+
					"run `make names` and commit the result",
				outPath, len(stats.GivenWords), stats.GivenDrops, len(stats.SurnameWords), stats.SurnameDrops)
		}
		fmt.Printf("names: mask/words_corpus.go matches tools/names (given=%d, surnames=%d)\n",
			len(stats.GivenWords), len(stats.SurnameWords))
		return nil
	}

	if writeErr := os.WriteFile(outPath, src, 0o600); writeErr != nil {
		return fmt.Errorf("writing %s: %w", outPath, writeErr)
	}
	fmt.Printf("names: wrote %s (given=%d, surnames=%d)\n", outPath, len(stats.GivenWords), len(stats.SurnameWords))
	return nil
}

// repoRoot walks up from the working directory to the module root, so this
// program behaves the same whether invoked as `go run ./tools/names` from the
// repository root (`make names`) or from anywhere else a caller finds
// convenient.
func repoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("getwd: %w", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no go.mod found above %s", dir)
		}
		dir = parent
	}
}
