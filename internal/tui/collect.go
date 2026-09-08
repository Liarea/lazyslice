// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"slices"
	"strings"
	"sync"

	"github.com/Liarea/lazyslice/internal/core"
	"github.com/Liarea/lazyslice/internal/event"
	"github.com/Liarea/lazyslice/internal/ref"
	"github.com/Liarea/lazyslice/internal/render"
)

// The codes the two screens are built from.
//
// The plan codes are internal/core's own constants, because core is already
// this package's one dependency for the request it builds. The two classify
// codes are spelled here instead of imported, because importing internal/classify
// would be this package reaching into a stage package for a constant and
// internal/tui/CLAUDE.md forbids reaching a stage at all. The spelling is held
// against the real catalogue by TestEveryCodeTheScreensReadIsInTheCatalogue,
// which is the same arrangement internal/invariants uses for the same two
// prefixes (internal/event/catalogue.yml, the note above classify.masked.column).
const (
	codeColumnMasked event.Code = "classify.masked.column"
	codeColumnCopied event.Code = "classify.copied.column"
)

// screenCodes is every code a screen reads, for the catalogue drift test.
var screenCodes = []event.Code{
	codeColumnMasked,
	codeColumnCopied,
	core.CodePlanStep,
	core.CodePlanEstimate,
	core.CodePlanPolymorphicInferred,
	core.CodePlanPolymorphic,
	core.CodePlanUnmapped,
}

// maxCollected bounds what one run may hand the screens.
//
// A schema with forty tables and two hundred columns produces eight thousand
// decisions, which fits; a pathological one does not, and a TUI that grew
// without bound on a schema the line printer handles would be a new failure
// mode the line printer does not have. Events beyond the bound are counted and
// the header says so, because a table that silently stopped at row 20,000 would
// be a screen that lies about what the classifier decided.
const maxCollected = 20_000

// Collector is the TUI's event.Sink: it passes every event on to the renderer
// behind it and keeps the ones the two screens are built from.
//
// It is what makes internal/tui "one more sink on the same event channel" and
// nothing more (ARCHITECTURE.md section 1). It reads no stage, holds no
// database handle, and keeps only identifiers and counts, because an
// event.Event carries nothing else by construction (THREAT_MODEL.md T4).
type Collector struct {
	mu      sync.Mutex
	next    event.Sink
	events  []event.Event
	dropped int
}

// NewCollector returns a Collector in front of next, which is the line printer
// in every real run: the transcript is printed as it happens and the screens are
// opened afterwards, so nothing is hidden by the TUI having been asked for.
func NewCollector(next event.Sink) *Collector {
	if next == nil {
		next = event.Discard
	}
	return &Collector{next: next}
}

var _ event.Sink = (*Collector)(nil)

// Send implements event.Sink.
func (c *Collector) Send(e event.Event) {
	c.next.Send(e)
	if !wanted(e.Code) {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.events) >= maxCollected {
		c.dropped++
		return
	}
	c.events = append(c.events, e)
}

// Events returns what the screens are built from, in the order the run produced
// it.
func (c *Collector) Events() []event.Event {
	c.mu.Lock()
	defer c.mu.Unlock()
	return slices.Clone(c.events)
}

// Dropped is how many events the bound refused. It is printed in the header
// rather than swallowed.
func (c *Collector) Dropped() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.dropped
}

// wanted reports whether a code reaches a screen.
func wanted(code event.Code) bool { return slices.Contains(screenCodes, code) }

// reasonRow is one classifier decision, as the reasons screen shows it.
//
// Reason is Decision.Reason, which internal/classify renders from a fixed
// template set and TestReasonGrammar parses back, so it holds identifiers,
// category names and counts and never a sampled value. The screen prints it
// unchanged for that reason: a TUI that reworded it would be a second place a
// message is written.
type reasonRow struct {
	Column ref.ColumnRef
	Masked bool
	Reason string
}

// planRow is one table of the plan, as the plan screen shows it.
type planRow struct {
	Table ref.TableRef
	// Rows is the step's row count as the event carried it, already formatted.
	Rows string
	// Mode is the step's pipeline.Mode name: child_ok, parent_only, lookup or
	// schema_only.
	Mode string
	// Why is the step's Why, rendered by internal/plan from its own template
	// set: "root", "child of public.orders via ...", "lookup", "skipped".
	Why string
}

// child reports whether a cap applies to this step.
//
// Only a child edge is capped (--cap is "children per parent key per edge", and
// internal/plan sets Step.Cap on exactly that edge), so the plan screen offers
// "c" on a child step and strikes it through everywhere else. The root is
// reached in ChildOK mode too and is not a child of anything, which is why the
// Why is read and not only the mode.
func (r planRow) child() bool {
	return r.Mode == "child_ok" && strings.HasPrefix(r.Why, "child of ")
}

// root reports whether this row is the plan's root table.
func (r planRow) root() bool { return r.Why == "root" }

// splitStep pulls a plan.step event's mode and why back out of the one argument
// internal/core packs them into ("child_ok; child of public.orders via ...").
func splitStep(reason string) (mode, why string) {
	mode, why, found := strings.Cut(reason, "; ")
	if !found {
		return reason, ""
	}
	return mode, why
}

// catalogueLine renders one event's message through the code catalogue.
//
// It goes through render.Lines rather than formatting the sentence here because
// internal/event/catalogue.yml is "the one place a message a user reads is
// written" (ARCHITECTURE.md section 7) and internal/render does not export the
// lookup on its own. A second copy of the templates in this package would be a
// second place the estimate's wording could drift from docs/ERRORS.md.
func catalogueLine(e event.Event) string {
	var b strings.Builder
	render.NewLines(&b).Send(e)
	line := strings.TrimRight(b.String(), "\n")
	// render.Lines prefixes each line with the marker for its kind, which is
	// that renderer's own vocabulary and not part of the message. The screens
	// carry the kind in their own layout, so the marker is cut rather than
	// printed twice.
	for _, marker := range []string{"  ", "? ", "! ", "✗ "} {
		if rest, found := strings.CutPrefix(line, marker); found {
			return rest
		}
	}
	return line
}
