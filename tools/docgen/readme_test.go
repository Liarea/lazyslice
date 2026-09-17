// SPDX-License-Identifier: Apache-2.0

package main

import (
	"strings"
	"testing"
)

// TestSpliceMarkedSection pins the byte-for-byte contract spliceMarkedSection
// promises: everything outside the marker pair is untouched, and only the
// text strictly between the markers is replaced. Nothing but `make
// docs-check` exercised this before — a regeneration diffed against its own
// template — so a splice that quietly degraded to returning doc unchanged
// would have passed forever.
func TestSpliceMarkedSection(t *testing.T) {
	doc := "# Title\n\nSome intro.\n\n<!-- start -->\nold content\nmore old content\n<!-- end -->\n\nTrailer text.\n"

	got, err := spliceMarkedSection(doc, "<!-- start -->", "<!-- end -->", "new content")
	if err != nil {
		t.Fatalf("spliceMarkedSection(): %v", err)
	}

	const wantPrefix = "# Title\n\nSome intro.\n\n<!-- start -->\n\nnew content\n\n<!-- end -->"
	if !strings.HasPrefix(got, wantPrefix) {
		t.Errorf("output does not start with the untouched prefix + replacement:\ngot:  %q\nwant prefix: %q", got, wantPrefix)
	}
	const wantSuffix = "<!-- end -->\n\nTrailer text.\n"
	if !strings.HasSuffix(got, wantSuffix) {
		t.Errorf("output does not end with the untouched suffix:\ngot: %q\nwant suffix: %q", got, wantSuffix)
	}
	if strings.Contains(got, "old content") {
		t.Errorf("old content between the markers survived the splice: %q", got)
	}
}

// TestSpliceMarkedSectionMissingMarkers asserts a missing or reversed marker
// pair is a build error, not a silent no-op: spliceMarkedSection must never
// return doc unchanged when it cannot find both markers in order.
func TestSpliceMarkedSectionMissingMarkers(t *testing.T) {
	tests := []struct {
		name string
		doc  string
	}{
		{"start marker missing", "no markers here at all\n"},
		{"end marker missing", "<!-- start -->\ncontent\n"},
		{"markers reversed", "<!-- end -->\ncontent\n<!-- start -->\n"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := spliceMarkedSection(tc.doc, "<!-- start -->", "<!-- end -->", "new content")
			if err == nil {
				t.Fatalf("spliceMarkedSection(): got no error and output %q, want an error", got)
			}
		})
	}
}

// TestRenderFirstRunTable pins the happy path: every entry in firstRunFlags
// must appear as a row, in firstRunFlags's own order, not --help's group
// order.
func TestRenderFirstRunTable(t *testing.T) {
	groups := []flagGroup{
		{
			title: "discover",
			rows: []flagRow{
				{long: "target", typ: "string", help: "Names the target"},
				{long: "source", short: "s", typ: "string", def: "postgres", help: "Names the source"},
			},
		},
	}

	// Use a scoped subset so the test does not have to track every entry in
	// the real firstRunFlags list.
	orig := firstRunFlags
	firstRunFlags = []string{"source", "target"}
	defer func() { firstRunFlags = orig }()

	table, err := renderFirstRunTable(groups)
	if err != nil {
		t.Fatalf("renderFirstRunTable(): %v", err)
	}

	sourceIdx := strings.Index(table, "--source")
	targetIdx := strings.Index(table, "--target")
	if sourceIdx == -1 || targetIdx == -1 {
		t.Fatalf("table missing an expected flag: %q", table)
	}
	if sourceIdx > targetIdx {
		t.Errorf("table orders --target before --source; want firstRunFlags order (source, target): %q", table)
	}
	if !strings.Contains(table, "-s, --source") {
		t.Errorf("table missing the short flag form for source: %q", table)
	}
	if !strings.Contains(table, "postgres") {
		t.Errorf("table missing the default for source: %q", table)
	}
}

// TestRenderFirstRunTableMissingFlag asserts renderFirstRunTable errors,
// rather than silently dropping a row, when a firstRunFlags entry is absent
// from the parsed groups — the case the doc comment above firstRunFlags
// promises is a loud failure, not a stub.
func TestRenderFirstRunTableMissingFlag(t *testing.T) {
	groups := []flagGroup{
		{
			title: "discover",
			rows: []flagRow{
				{long: "target", typ: "string", help: "Names the target"},
			},
		},
	}

	orig := firstRunFlags
	firstRunFlags = []string{"source", "target"}
	defer func() { firstRunFlags = orig }()

	_, err := renderFirstRunTable(groups)
	if err == nil {
		t.Fatal("renderFirstRunTable(): got no error, want one (source is not in groups)")
	}
}
