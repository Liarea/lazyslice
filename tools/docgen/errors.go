// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/goccy/go-yaml"

	"github.com/Liarea/lazyslice/internal/event"
)

// catalogueRow mirrors one entry of internal/event/catalogue.yml. It is a
// separate type from internal/render's private row rather than a shared one:
// that package renders a line at runtime from the fields it needs, this one
// only reads the file to write a table, and the two are independent readers
// of the one file the comment in each names as the source of truth.
type catalogueRow struct {
	Code    string   `yaml:"code"`
	Kind    string   `yaml:"kind"`
	Stage   string   `yaml:"stage"`
	Exit    int      `yaml:"exit"`
	Message string   `yaml:"message"`
	Args    []string `yaml:"args"`
}

// generateErrors renders docs/ERRORS.md from event.Catalogue(): one row per
// code, in the catalogue's own order — its file groups codes by area under
// comments, and that grouping survives here rather than being sorted away.
func generateErrors() (string, error) {
	var rows []catalogueRow
	if err := yaml.Unmarshal(event.Catalogue(), &rows); err != nil {
		return "", fmt.Errorf("parsing internal/event/catalogue.yml: %w", err)
	}
	if len(rows) == 0 {
		return "", fmt.Errorf("internal/event/catalogue.yml parsed to zero rows")
	}

	var b strings.Builder
	b.WriteString(generatedHeader("Errors", "`internal/event.Catalogue()` (internal/event/catalogue.yml)"))
	b.WriteString("One row per code in the catalogue, in the file's own order. Exit is only " +
		"meaningful for a `kind: error` row; every other kind leaves the process exit code to " +
		"whichever stage runs next.\n\n")
	b.WriteString("| Code | Stage | Exit | Message |\n")
	b.WriteString("|---|---|---|---|\n")
	for _, r := range rows {
		exit := "-"
		if r.Kind == "error" {
			exit = strconv.Itoa(r.Exit)
		}
		fmt.Fprintf(&b, "| `%s` | %s | %s | %s |\n", r.Code, r.Stage, exit, mdEscapeText(r.Message))
	}
	return b.String(), nil
}
