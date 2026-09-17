// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// readmeFlagsStart and readmeFlagsEnd bracket the table generateReadme
// writes into README.md. T-0260 rewrites everything else in that file
// around them; docgen's contract with that task is that it never touches a
// byte outside this pair.
const (
	readmeFlagsStart = "<!-- docgen:flags:start -->"
	readmeFlagsEnd   = "<!-- docgen:flags:end -->"
)

// firstRunFlags is the curated subset of the registered flag set that a
// first run actually meets, in the order T-0259's goal names them — not
// --help's stage order, which is the right order for the full reference in
// docs/FLAGS.md but not for a reader meeting the tool for the first time.
// The list is intentionally short and hand-picked; renderFirstRunTable
// fails loudly if --help ever stops registering one of these, rather than
// silently dropping a row.
var firstRunFlags = []string{
	"source",
	"target",
	"root",
	"yes",
	"create-target",
	"unmask",
	"skip-table",
	"phone-region",
	"secret-file",
	"require-key",
	"plan",
	"json",
}

// generateReadme reads root/README.md as its own template — the source of
// truth for everything outside the two markers — and returns the file with
// only the text between them replaced by a freshly rendered first-run flag
// table, built the same way generateFlags builds docs/FLAGS.md: from
// `go run ./cmd/lazyslice --help`.
func generateReadme(root string) (string, error) {
	readmePath := filepath.Join(root, "README.md")
	orig, err := os.ReadFile(readmePath)
	if err != nil {
		return "", fmt.Errorf("reading %s: %w", readmePath, err)
	}

	help, err := runHelp(root)
	if err != nil {
		return "", err
	}
	groups, err := parseHelpGroups(help)
	if err != nil {
		return "", err
	}

	table, err := renderFirstRunTable(groups)
	if err != nil {
		return "", err
	}

	out, err := spliceMarkedSection(string(orig), readmeFlagsStart, readmeFlagsEnd, table)
	if err != nil {
		return "", fmt.Errorf("%s: %w", readmePath, err)
	}
	return out, nil
}

// renderFirstRunTable renders the firstRunFlags subset of groups as a
// Markdown table in the exact shape generateFlags uses for docs/FLAGS.md,
// plus a lead-in sentence linking to the full reference there.
func renderFirstRunTable(groups []flagGroup) (string, error) {
	byLong := make(map[string]flagRow)
	for _, g := range groups {
		for _, r := range g.rows {
			byLong[r.long] = r
		}
	}

	var b strings.Builder
	b.WriteString("The flags a first run meets. The full set, one row per registered flag " +
		"grouped by stage, is [docs/FLAGS.md](docs/FLAGS.md).\n\n")
	b.WriteString("| Flag | Type | Default | Description |\n")
	b.WriteString("|---|---|---|---|\n")
	for _, long := range firstRunFlags {
		row, ok := byLong[long]
		if !ok {
			return "", fmt.Errorf(
				"README's first-run table wants --%s but `go run ./cmd/lazyslice --help` registers no such flag "+
					"(update firstRunFlags in tools/docgen/readme.go, or the flag was renamed)", long)
		}
		name := "`--" + row.long + "`"
		if row.short != "" {
			name = "`-" + row.short + ", --" + row.long + "`"
		}
		typ := row.typ
		if typ == "" {
			typ = "bool"
		}
		def := row.def
		if def == "" {
			def = "-"
		}
		fmt.Fprintf(&b, "| %s | %s | %s | %s |\n", name, typ, def, mdEscapeText(row.help))
	}
	return b.String(), nil
}

// spliceMarkedSection replaces the text strictly between the first
// occurrence of start and the first occurrence of end after it with
// newContent, leaving the markers themselves and everything outside them
// byte-for-byte untouched. It fails rather than guessing when either marker
// is missing or out of order, so a hand-edited or half-merged README.md is a
// build error instead of a silent no-op or a corrupted file.
func spliceMarkedSection(doc, start, end, newContent string) (string, error) {
	startIdx := strings.Index(doc, start)
	if startIdx == -1 {
		return "", fmt.Errorf("missing marker %q", start)
	}
	afterStart := startIdx + len(start)

	relEndIdx := strings.Index(doc[afterStart:], end)
	if relEndIdx == -1 {
		return "", fmt.Errorf("missing marker %q (or it appears before %q)", end, start)
	}
	endIdx := afterStart + relEndIdx

	var b strings.Builder
	b.WriteString(doc[:afterStart])
	b.WriteString("\n\n")
	b.WriteString(strings.TrimRight(newContent, "\n"))
	b.WriteString("\n\n")
	b.WriteString(doc[endIdx:])
	return b.String(), nil
}
