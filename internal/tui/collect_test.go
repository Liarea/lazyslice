// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"os"
	"strings"
	"testing"

	"github.com/goccy/go-yaml"

	"github.com/Liarea/lazyslice/internal/core"
	"github.com/Liarea/lazyslice/internal/event"
)

// TestCollectorPassesEverythingOnAndKeepsWhatTheScreensRead: the collector sits
// in front of the line printer rather than instead of it, so asking for the TUI
// never costs a transcript line.
func TestCollectorPassesEverythingOnAndKeepsWhatTheScreensRead(t *testing.T) {
	var seen []event.Code
	c := NewCollector(event.SinkFunc(func(e event.Event) { seen = append(seen, e.Code) }))

	events := append(fixture(), event.Event{
		Stage: event.Load, Kind: event.Info, Code: core.CodeConfigWritten,
	})
	for _, e := range events {
		c.Send(e)
	}

	if len(seen) != len(events) {
		t.Errorf("the sink behind the collector saw %d of %d events", len(seen), len(events))
	}
	kept := c.Events()
	if len(kept) != len(fixture()) {
		t.Errorf("the collector kept %d events, want the %d the screens read", len(kept), len(fixture()))
	}
	for _, e := range kept {
		if !wanted(e.Code) {
			t.Errorf("the collector kept %s, which no screen reads", e.Code)
		}
	}
}

// TestCollectorCountsWhatItRefuses: a screen that silently stopped at its bound
// would be a screen that lies about what the classifier decided, so the bound
// is counted and the header says so.
func TestCollectorCountsWhatItRefuses(t *testing.T) {
	c := NewCollector(nil)
	one := decision(codeColumnMasked, "customer", "email", "name rule: email")
	for range maxCollected + 5 {
		c.Send(one)
	}
	if got := len(c.Events()); got != maxCollected {
		t.Errorf("the collector kept %d events, want the bound %d", got, maxCollected)
	}
	if got := c.Dropped(); got != 5 {
		t.Errorf("Dropped() = %d, want 5", got)
	}
	if !strings.Contains(newModel(core.NewRequest()).setDropped(5).header(), "5 more not shown") {
		t.Error("the header does not say the screen is not the whole classification")
	}
}

// TestEveryCodeTheScreensReadIsInTheCatalogue is this package's drift guard.
//
// Two of the codes are spelled here rather than imported, because importing
// internal/classify would be this package reaching into a stage package. The
// price of that is a string that could go stale, and this is what it is paid
// with: internal/event/catalogue.yml is the source of docs/ERRORS.md, and a
// code that is not a row in it renders as "no row in the catalogue" on every
// line the screens show.
func TestEveryCodeTheScreensReadIsInTheCatalogue(t *testing.T) {
	var rows []struct {
		Code event.Code `yaml:"code"`
	}
	if err := yaml.Unmarshal(event.Catalogue(), &rows); err != nil {
		t.Fatalf("parsing the catalogue: %v", err)
	}
	have := map[event.Code]bool{}
	for _, r := range rows {
		have[r.Code] = true
	}
	for _, c := range screenCodes {
		if !have[c] {
			t.Errorf("the screens read %s, which is not a row in internal/event/catalogue.yml", c)
		}
	}
}

// TestCatalogueLineIsTheCatalogues: the estimate and the polymorphic notes are
// rendered through the same catalogue the line printer uses, so the wording on
// the screen and the wording in docs/ERRORS.md cannot drift apart.
func TestCatalogueLineIsTheCatalogues(t *testing.T) {
	got := catalogueLine(event.Event{
		Stage: event.Plan, Kind: event.Info, Code: core.CodePlanEstimate,
		Args: event.Args{
			event.ArgCount:   "4598",
			event.ArgReason:  "18 KiB of keys",
			event.ArgSeconds: "0.3",
		},
	})
	want := "4598 rows, 18 KiB of keys; the snapshot is held about 0.3s, assuming 20,000 rows/s"
	if got != want {
		t.Errorf("catalogueLine =\n%q\nwant\n%q", got, want)
	}
}

// TestStepModeAndWhyComeBackApart: internal/core packs a step's mode and its Why
// into one argument, and the plan screen needs them apart to know which rows a
// cap and a skip can apply to.
func TestStepModeAndWhyComeBackApart(t *testing.T) {
	for _, tc := range []struct {
		in         string
		mode, why  string
		child, top bool
	}{
		{"child_ok; root", "child_ok", "root", false, true},
		{"child_ok; child of public.customer via public.payment.customer_id",
			"child_ok", "child of public.customer via public.payment.customer_id", true, false},
		{"parent_only; parent of public.customer via public.customer.address_id",
			"parent_only", "parent of public.customer via public.customer.address_id", false, false},
		{"lookup; lookup", "lookup", "lookup", false, false},
		{"schema_only", "schema_only", "", false, false},
	} {
		mode, why := splitStep(tc.in)
		if mode != tc.mode || why != tc.why {
			t.Errorf("splitStep(%q) = %q, %q; want %q, %q", tc.in, mode, why, tc.mode, tc.why)
		}
		row := planRow{Mode: mode, Why: why}
		if row.child() != tc.child {
			t.Errorf("%q: child() = %v, want %v", tc.in, row.child(), tc.child)
		}
		if row.root() != tc.top {
			t.Errorf("%q: root() = %v, want %v", tc.in, row.root(), tc.top)
		}
	}
}

// TestThePlanWordingTheScreenParsesIsStillTheOneThePlanWrites is this package's
// second drift guard, and it is the ugliest thing in it.
//
// planRow.child() and planRow.root() decide whether "c" (--cap) and "x"
// (--skip-table) can fire on a row, and what the cap column prints, by
// prefix-matching prose: "child of " and "root" are internal/plan's own Why
// templates, packed with the mode into one event argument by internal/core.
// pipeline.Step carries Cap, Mode and Depth as fields, and the plan.step event
// simply does not emit them — so a wording change in internal/plan would strike
// a documented action through on every row with every test in this package
// still green.
//
// This package may not import a stage package (internal/tui/CLAUDE.md), and a
// real internal/plan Plan needs a database to build, so the coupling is held
// against the source text of the two files that write those strings. It is a
// blunt instrument: it fails loudly when the wording moves, which is the whole
// point, and it goes away when internal/core emits the step's cap and mode as
// their own event arguments (reported to the orchestrator as a follow-up).
func TestThePlanWordingTheScreenParsesIsStillTheOneThePlanWrites(t *testing.T) {
	for path, wants := range map[string][]string{
		// internal/plan writes the Why the screen reads.
		"../plan/plan.go": {`p.why[root] = "root"`, `p.noteWhy(fk.Child, "child of "`},
		// internal/core packs the mode and the Why into one argument, and names
		// the mode the screen compares against.
		"../core/run.go":   {`modeName(s.Mode) + "; " + s.Why`},
		"../core/names.go": {`return "child_ok"`},
	} {
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("reading %s: %v", path, err)
		}
		for _, want := range wants {
			if !strings.Contains(string(src), want) {
				t.Errorf("%s no longer carries %s, which the plan screen parses out of the "+
					"plan.step event to decide whether --cap and --skip-table apply to a row",
					path, want)
			}
		}
	}
}
