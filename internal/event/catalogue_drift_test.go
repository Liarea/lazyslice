// SPDX-License-Identifier: Apache-2.0

package event

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/goccy/go-yaml"
)

// TestEveryCodeInTheTreeIsInTheCatalogueAndViceVersa is T-FAILUX's drift test
// (tracker T-0089): every event.Code declared anywhere under internal/ or
// cmd/ has a row in catalogue.yml, and every row in catalogue.yml is a Code
// declared somewhere in the tree. Either direction drifting is a failure a
// user would meet with no explanation — a code with no row renders as its own
// bare string (internal/render's fallback), and a row with no code names a
// failure class nothing can ever raise, which is dead documentation
// pretending to be live.
//
// Declaration is found by source text rather than by importing every package
// (which internal/event, a leaf next to ref, must not do — root CLAUDE.md,
// internal/CLAUDE.md's import graph): every Code constant in the tree is
// written `<Name> [event.]Code = "<code>"`, optionally after `const `, one per
// line — codeDeclRE is that shape, and it is the shape every codes.go file in
// the tree already uses (internal/core/codes.go, internal/plan/codes.go,
// internal/load/codes.go, internal/extract/codes.go,
// internal/transform/codes.go, internal/verify/codes.go,
// internal/classify/codes.go, internal/discover/codes.go,
// internal/pg/target.go, internal/pg/lease.go, internal/load/ddl/recreatable.go,
// internal/tui/collect.go, and this package's own event.go for the three
// scaffold codes).
func TestEveryCodeInTheTreeIsInTheCatalogueAndViceVersa(t *testing.T) {
	root := repoRoot(t)

	declared := map[string][]string{} // code -> files it is declared in
	for _, dir := range []string{"internal", "cmd"} {
		walkGoFiles(t, filepath.Join(root, dir), func(path string, src []byte) {
			for _, m := range codeDeclRE.FindAllSubmatch(src, -1) {
				code := string(m[1])
				declared[code] = append(declared[code], relPath(root, path))
			}
		})
	}
	if len(declared) == 0 {
		t.Fatal("found no Code declarations under internal/ or cmd/; codeDeclRE has drifted from the codebase's own style")
	}

	var rows []catalogueRow
	if err := yaml.Unmarshal(Catalogue(), &rows); err != nil {
		t.Fatalf("parsing catalogue.yml: %v", err)
	}
	if len(rows) == 0 {
		t.Fatal("catalogue.yml parsed to zero rows")
	}
	catalogued := map[string]bool{}
	for _, r := range rows {
		if catalogued[r.Code] {
			t.Errorf("%s has more than one row in catalogue.yml", r.Code)
		}
		catalogued[r.Code] = true
	}

	var missingFromCatalogue []string
	for code, files := range declared {
		if !catalogued[code] {
			missingFromCatalogue = append(missingFromCatalogue,
				code+" (declared in "+strings.Join(files, ", ")+")")
		}
	}
	sort.Strings(missingFromCatalogue)
	for _, m := range missingFromCatalogue {
		t.Errorf("%s is declared as an event.Code but has no row in internal/event/catalogue.yml", m)
	}

	var orphanRows []string
	for code := range catalogued {
		if len(declared[code]) == 0 {
			orphanRows = append(orphanRows, code)
		}
	}
	sort.Strings(orphanRows)
	for _, code := range orphanRows {
		t.Errorf("%s has a row in catalogue.yml but is not a Code declared anywhere under internal/ or cmd/: "+
			"either nothing can ever raise it, or its declaration no longer matches codeDeclRE", code)
	}
}

// catalogueRow mirrors the fields of one catalogue.yml entry this test reads.
// It is a copy of internal/render's and tools/docgen's private row type
// rather than a shared one, for the same reason those two are independent of
// each other: this package must stay free of anything but the event model
// (internal/event/CLAUDE.md), and a parsed row shape is a reader's own
// concern.
type catalogueRow struct {
	Code string `yaml:"code"`
}

// codeDeclRE matches one Code constant declaration: an identifier, the type
// spelled either `Code` (inside this package) or `event.Code` (everywhere
// else), `=`, and the quoted string. It requires the value to look like a
// dotted code ("subject.verdict.detail" or "subject.detail") rather than any
// quoted string typed Code, so a struct literal field or an unrelated
// same-named type cannot match by accident.
var codeDeclRE = regexp.MustCompile(
	`(?m)^\s*(?:const\s+)?[A-Za-z_]\w*\s+(?:event\.)?Code\s*=\s*"([a-z][a-z0-9_]*(?:\.[a-z0-9_]+)+)"`)

// repoRoot finds the module root from this test file's own location, so the
// test does not depend on the working directory go test happens to be run
// from.
func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller could not find this file's own path")
	}
	// This file lives at <root>/internal/event/catalogue_drift_test.go.
	return filepath.Dir(filepath.Dir(filepath.Dir(file)))
}

// walkGoFiles calls fn with the contents of every non-test .go file under
// dir. Test files are excluded because a Code is declared once, in a
// package's own codes.go (or equivalent), and a fixture or a mock naming a
// string that happens to look like a code is not a declaration this test
// should hold the catalogue to.
func walkGoFiles(t *testing.T, dir string, fn func(path string, src []byte)) {
	t.Helper()
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		fn(path, src)
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", dir, err)
	}
}

func relPath(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return path
	}
	return rel
}
