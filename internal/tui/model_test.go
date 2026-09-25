// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/Liarea/lazyslice/internal/core"
	"github.com/Liarea/lazyslice/internal/event"
	"github.com/Liarea/lazyslice/internal/ref"
)

// The model is driven the way the framework drives it: a recorded sequence of
// tea.Msg values through Update, with the request read off the model afterwards
// (internal/tui/CLAUDE.md, "model update logic against recorded tea.Msg
// sequences"). Nothing here starts a program, because a screen's decisions are
// Update's, not the renderer's.

func tbl(name string) ref.TableRef { return ref.TableRef{Schema: "public", Name: name} }

func decision(code event.Code, name, column, reason string) event.Event {
	return event.Event{
		Stage:  event.Classify,
		Kind:   event.Decision,
		Code:   code,
		Table:  tbl(name),
		Column: column,
		Args: event.Args{
			event.ArgTable:  tbl(name).String(),
			event.ArgColumn: column,
			event.ArgReason: reason,
		},
	}
}

func step(name, rows, reason string) event.Event {
	return event.Event{
		Stage: event.Plan,
		Kind:  event.Info,
		Code:  core.CodePlanStep,
		Table: tbl(name),
		Args: event.Args{
			event.ArgTable:  tbl(name).String(),
			event.ArgCount:  rows,
			event.ArgReason: reason,
		},
	}
}

// fixture is one small Pagila-shaped run: two classified columns and three plan
// steps, one of them the root and one of them a child.
func fixture() []event.Event {
	return []event.Event{
		decision(codeColumnMasked, "customer", "email", "name rule: email; validator: rfc5322 on 200/200"),
		decision(codeColumnCopied, "customer", "store_id", "type rule: integer; no name match"),
		decision(codeColumnMasked, "staff", "last_name", "name rule: person_name"),
		step("customer", "200", "child_ok; root"),
		step("address", "198", "parent_only; parent of public.customer via public.customer.address_id"),
		step("payment", "4200", "child_ok; child of public.customer via public.payment.customer_id"),
		{
			Stage: event.Plan, Kind: event.Info, Code: core.CodePlanEstimate,
			Args: event.Args{
				event.ArgCount:   "4598",
				event.ArgReason:  "18 KiB of keys, 2 KiB of residual filter",
				event.ArgSeconds: "0.3",
			},
		},
	}
}

func fixtureModel(t *testing.T) model {
	t.Helper()
	return newModel(core.NewRequest()).seed(fixture())
}

// press sends one key press per name and returns the model that came back.
func press(t *testing.T, m model, names ...string) model {
	t.Helper()
	for _, name := range names {
		next, _ := m.Update(keyMsg(name))
		got, ok := next.(model)
		if !ok {
			t.Fatalf("Update returned a %T, not a model", next)
		}
		m = got
	}
	return m
}

// typeText presses one key per rune, which is what a terminal sends.
func typeText(t *testing.T, m model, s string) model {
	t.Helper()
	for _, r := range s {
		m = press(t, m, string(r))
	}
	return m
}

// onPlan switches to the plan screen.
func onPlan(t *testing.T, m model) model {
	t.Helper()
	m = press(t, m, "tab")
	if m.screen != screenPlan {
		t.Fatalf("tab left the model on %v", m.screen)
	}
	return m
}

// TestReasonsScreenBuildsTheUnmaskFlag is the reasons screen's whole point: the
// per-column opt-out builds the same --unmask TABLE.COL=REASON the flag builds,
// keyed the same way, so internal/core cannot tell which one made it.
func TestReasonsScreenBuildsTheUnmaskFlag(t *testing.T) {
	m := fixtureModel(t)
	m = press(t, m, "u")
	if !m.prompt.active {
		t.Fatal("u did not ask for a reason")
	}
	m = typeText(t, m, "support reads it in the ticket")
	m = press(t, m, "enter")

	want := "support reads it in the ticket"
	if got := m.request().Unmask["public.customer.email"]; got != want {
		t.Errorf("Unmask[public.customer.email] = %q, want %q", got, want)
	}
	if !m.request().Explicit["unmask"] {
		t.Error("the opt-out was not recorded as an explicit flag, so lazyslice.yml would override it")
	}
	flags := m.flags()
	wantFlag := `--unmask "public.customer.email=support reads it in the ticket"`
	if !slices.Contains(flags, wantFlag) {
		t.Errorf("flags() = %q, want it to contain %q", flags, wantFlag)
	}
}

// TestUnmaskRefusesAReasonlessOptOut holds ARCHITECTURE.md section 8's rule at
// the screen: the bare --unmask form is exit 2 on the command line, so the
// screen must not have a way to make one either. An opt-out with no reason is
// an opt-out nobody can review later.
func TestUnmaskRefusesAReasonlessOptOut(t *testing.T) {
	m := fixtureModel(t)
	m = press(t, m, "u", "enter")

	if len(m.request().Unmask) != 0 {
		t.Fatalf("an empty reason made an opt-out: %v", m.request().Unmask)
	}
	if !m.prompt.active {
		t.Error("the prompt closed on an empty reason rather than saying why")
	}
	if m.prompt.err == "" {
		t.Error("the prompt gave no reason for refusing")
	}
}

// TestUnmaskTogglesBackToMasked: removing an opt-out asks nothing, because the
// absence of the flag is the absence of the opt-out.
func TestUnmaskTogglesBackToMasked(t *testing.T) {
	m := fixtureModel(t)
	m = press(t, m, "u")
	m = typeText(t, m, "reviewed")
	m = press(t, m, "enter", "u")

	if _, still := m.request().Unmask["public.customer.email"]; still {
		t.Error("the second u did not remove the opt-out")
	}
	if m.prompt.active {
		t.Error("removing an opt-out asked for a reason")
	}
	if len(m.flags()) != 0 {
		t.Errorf("flags() = %q, want none once the opt-out is gone", m.flags())
	}
}

// TestPlanScreenBuildsThePlanFlags walks the plan screen's four value actions
// and the one that needs no value, and reads the request back.
func TestPlanScreenBuildsThePlanFlags(t *testing.T) {
	m := onPlan(t, fixtureModel(t))

	// --take on the root row.
	m = press(t, m, "n")
	m = typeText(t, m, "50")
	m = press(t, m, "enter")

	// --depth.
	m = press(t, m, "d")
	m = typeText(t, m, "2")
	m = press(t, m, "enter")

	// The third row is the child step, which is the only one a cap applies to.
	m = press(t, m, "down", "down", "c")
	m = typeText(t, m, "10")
	m = press(t, m, "enter")

	// --skip-table on the same row.
	m = press(t, m, "x")

	// The row the opt-out was made on has to say so, not only the request: a
	// screen that recorded the skip and went on printing "child of ..." is a
	// screen the operator cannot check their own work against.
	if got := m.plan.Rows()[2][3]; !strings.HasPrefix(got, "skipped") {
		t.Errorf("the skipped row's why is %q, want it to start with \"skipped\"", got)
	}
	if !strings.Contains(m.transcript(), "skipped") {
		t.Errorf("the transcript does not carry the skip:\n%s", m.transcript())
	}

	req := m.request()
	if req.Take != 50 {
		t.Errorf("Take = %d, want 50", req.Take)
	}
	if req.Depth != 2 {
		t.Errorf("Depth = %d, want 2", req.Depth)
	}
	if got := req.TableCaps["public.payment"]; got != 10 {
		t.Errorf("TableCaps[public.payment] = %d, want 10", got)
	}
	if !slices.Contains(req.SkipTables, "public.payment") {
		t.Errorf("SkipTables = %q, want public.payment", req.SkipTables)
	}
	if !req.Explicit["take"] || !req.Explicit["depth"] {
		t.Error("--take and --depth were not recorded as explicit, so lazyslice.yml would override them")
	}
	// A per-table cap is not the global one; recording it as one would make the
	// committed cap: silently ignored for every other table (cmd/lazyslice's
	// parseCaps makes the same distinction).
	if req.Explicit["cap"] {
		t.Error("a per-table --cap was recorded as the global cap")
	}

	want := []string{"--take 50", "--depth 2", "--cap public.payment=10", "--skip-table public.payment"}
	if got := m.flags(); !reflect.DeepEqual(got, want) {
		t.Errorf("flags() = %q, want %q", got, want)
	}
}

// TestRootActionNamesTheSelectedTable: --root takes the row rather than a
// prompt, because the table the cursor is on is the answer.
func TestRootActionNamesTheSelectedTable(t *testing.T) {
	m := onPlan(t, fixtureModel(t))
	m = press(t, m, "down", "R")

	if got := m.request().Root; got != "public.address" {
		t.Errorf("Root = %q, want public.address", got)
	}
	if !slices.Contains(m.flags(), "--root public.address") {
		t.Errorf("flags() = %q, want --root public.address", m.flags())
	}
}

// TestCountPromptRefusesZero mirrors cmd/lazyslice's checkCounts: a zero take,
// cap or depth is a refusal and not a silent default, because an int field
// cannot carry the difference between "unset" and "none".
func TestCountPromptRefusesZero(t *testing.T) {
	m := onPlan(t, fixtureModel(t))
	m = press(t, m, "n")
	m = typeText(t, m, "0")
	m = press(t, m, "enter")

	if m.request().Take != core.DefaultTake {
		t.Errorf("Take = %d, want the default %d left alone", m.request().Take, core.DefaultTake)
	}
	if !m.prompt.active || m.prompt.err == "" {
		t.Error("a zero count was accepted, or was refused without saying why")
	}
}

// TestUnavailableActionIsStruckThroughNotHidden is ADR-002's footer rule,
// sourced from research/SQLIT_STUDY.md section 2.4: a binding that cannot fire
// on this row stays in the footer struck through, because a key that disappears
// reads as a key that never existed.
func TestUnavailableActionIsStruckThroughNotHidden(t *testing.T) {
	// The second reasons row is a copied column with no opt-out, so there is
	// nothing for --unmask to do on it.
	m := press(t, fixtureModel(t), "down")

	var found bool
	for _, p := range m.footerParts() {
		if p.Key != "u" {
			continue
		}
		found = true
		if p.Available {
			t.Error("the opt-out reads as available on a column that is not masked")
		}
	}
	if !found {
		t.Fatal("the opt-out vanished from the footer instead of being struck through")
	}
	if !strings.Contains(m.footer(), "\x1b[9m") {
		t.Error("the rendered footer carries no strikethrough")
	}

	// Pressing it anyway changes nothing and says so.
	after := press(t, m, "u")
	if len(after.request().Unmask) != 0 {
		t.Error("an unavailable action fired")
	}
	if after.status == "" {
		t.Error("an unavailable action fired nothing and said nothing")
	}
}

// TestNavigationBuildsNoRequest is the runtime half of the flag rule: a binding
// that carries no flag must change no field of the request, or
// TestEveryTUIActionHasFlag would be checking the wrong set.
func TestNavigationBuildsNoRequest(t *testing.T) {
	start := fixtureModel(t)
	before := start.request()

	for _, b := range Bindings() {
		if !b.Nav {
			continue
		}
		for _, name := range b.Bind.Keys() {
			m := press(t, start, name)
			if !reflect.DeepEqual(m.request(), before) {
				t.Errorf("the navigation key %q changed the request", name)
			}
		}
	}
}

// TestTheScreensDoNotWriteThroughTheCallersRequest: the model is handed the
// request the flags built, and leaving without running must leave it alone —
// including its maps, which a shallow copy would share.
func TestTheScreensDoNotWriteThroughTheCallersRequest(t *testing.T) {
	req := core.NewRequest()
	m := newModel(req).seed(fixture())
	m = press(t, m, "u")
	m = typeText(t, m, "reviewed")
	m = press(t, m, "enter")

	if len(req.Unmask) != 0 {
		t.Errorf("the caller's request gained %v", req.Unmask)
	}
	if len(m.request().Unmask) != 1 {
		t.Errorf("the model's own request did not gain the opt-out: %v", m.request().Unmask)
	}
}

// TestCapCellIsTheCapInForce: the plan.step event does not carry Step.Cap, so
// the screen computes it the way internal/plan does — the per-table cap when
// there is one, the global cap otherwise, and only on a child edge.
func TestCapCellIsTheCapInForce(t *testing.T) {
	m := onPlan(t, fixtureModel(t))

	cells := map[string]string{}
	for i, row := range m.plan.Rows() {
		cells[m.steps[i].Table.String()] = row[2]
	}
	if got := cells["public.payment"]; got != "100" {
		t.Errorf("the child step shows cap %q, want the default 100", got)
	}
	if got := cells["public.customer"]; got != "-" {
		t.Errorf("the root shows cap %q, want none: the root is nobody's child", got)
	}
	if got := cells["public.address"]; got != "-" {
		t.Errorf("a parent step shows cap %q, want none: --cap is per child edge", got)
	}
}

// TestCancelAndQuitLeaveWithoutConfirming. Cancel is not rebindable (ADR-002)
// and both of its keys leave, whatever the screen is showing; so does "q". Only
// ctrl+c is the terminal's own interrupt (T-0345).
func TestCancelAndQuitLeaveWithoutConfirming(t *testing.T) {
	for _, tc := range []struct {
		key         string
		interrupted bool
	}{
		{"q", false},
		{"esc", false},
		{"ctrl+c", true},
	} {
		m := fixtureModel(t)
		next, cmd := m.Update(keyMsg(tc.key))
		got, ok := next.(model)
		if !ok {
			t.Fatalf("%s: Update returned a %T", tc.key, next)
		}
		if cmd == nil {
			t.Errorf("%s did not leave", tc.key)
		}
		if got.accepted {
			t.Errorf("%s: accepted = true, want false", tc.key)
		}
		if got.interrupted != tc.interrupted {
			t.Errorf("%s: interrupted = %v, want %v", tc.key, got.interrupted, tc.interrupted)
		}
	}
}

// TestAcceptOnAWritingModeConfirmsBeforeRunning. ModeRun and ModeVerify would
// drop and rewrite the target, so the first Accept must only open the
// confirmation (T-0345): a stranger pressing enter to open a row must not be
// able to start that run. The second Accept is what actually leaves.
func TestAcceptOnAWritingModeConfirmsBeforeRunning(t *testing.T) {
	m := fixtureModel(t) // core.NewRequest()'s zero Mode is ModeRun.

	next, cmd := m.Update(keyMsg("enter"))
	got, ok := next.(model)
	if !ok {
		t.Fatalf("Update returned a %T", next)
	}
	if cmd != nil {
		t.Error("the first enter left the TUI instead of opening the confirmation")
	}
	if got.accepted {
		t.Error("the first enter accepted the run")
	}
	if !got.confirming {
		t.Fatal("the first enter did not open the confirmation")
	}

	next, cmd = got.Update(keyMsg("enter"))
	got, ok = next.(model)
	if !ok {
		t.Fatalf("Update returned a %T", next)
	}
	if cmd == nil {
		t.Error("the second enter did not leave")
	}
	if !got.accepted {
		t.Error("the second enter did not accept the run")
	}
	if got.interrupted {
		t.Error("confirming the run reported an interrupt")
	}
}

// TestConfirmationNamesTheTargetTablesAndRows is the confirmation's whole
// point (T-0345): "run: drop and rewrite <target>, N tables, M rows", read
// once and acted on, not a screen the operator has to already understand the
// plan table to interpret.
func TestConfirmationNamesTheTargetTablesAndRows(t *testing.T) {
	m := fixtureModel(t).setTarget("nobody@127.0.0.1:5432/target")
	m = press(t, m, "enter")
	if !m.confirming {
		t.Fatal("enter did not open the confirmation")
	}
	got := m.confirmText()
	for _, want := range []string{"drop and rewrite nobody@127.0.0.1:5432/target", "3 tables", "4598 rows"} {
		if !strings.Contains(got, want) {
			t.Errorf("confirmText() = %q, want it to contain %q", got, want)
		}
	}
}

// TestAnyOtherKeyBacksOutOfTheConfirmation: only Accept runs it; esc, "q" and
// everything else return to the screen with nothing changed and nothing left.
func TestAnyOtherKeyBacksOutOfTheConfirmation(t *testing.T) {
	for _, key := range []string{"esc", "q"} {
		m := press(t, fixtureModel(t), "enter")
		if !m.confirming {
			t.Fatalf("%s: enter did not open the confirmation", key)
		}
		next, cmd := m.Update(keyMsg(key))
		got, ok := next.(model)
		if !ok {
			t.Fatalf("%s: Update returned a %T", key, next)
		}
		if cmd != nil {
			t.Errorf("%s left the TUI instead of backing out of the confirmation", key)
		}
		if got.confirming {
			t.Errorf("%s did not close the confirmation", key)
		}
		if got.accepted {
			t.Errorf("%s accepted the run", key)
		}
	}
}

// TestCtrlCLeavesEvenFromInsideTheConfirmation: the interrupt reaches every
// screen this package has, the confirmation included (T-0345).
func TestCtrlCLeavesEvenFromInsideTheConfirmation(t *testing.T) {
	m := press(t, fixtureModel(t), "enter")
	if !m.confirming {
		t.Fatal("enter did not open the confirmation")
	}
	next, cmd := m.Update(keyMsg("ctrl+c"))
	got, ok := next.(model)
	if !ok {
		t.Fatalf("Update returned a %T", next)
	}
	if cmd == nil {
		t.Error("ctrl+c did not leave")
	}
	if !got.interrupted {
		t.Error("ctrl+c from the confirmation did not report an interrupt")
	}
	if got.accepted {
		t.Error("ctrl+c accepted the run")
	}
}

// TestAcceptOnAModeThatWritesNoTargetSkipsTheConfirmation: classify and plan
// never open a target, so there is nothing for a second Accept to confirm.
func TestAcceptOnAModeThatWritesNoTargetSkipsTheConfirmation(t *testing.T) {
	req := core.NewRequest()
	req.Mode = core.ModePlan
	m := newModel(req).seed(fixture())

	next, cmd := m.Update(keyMsg("enter"))
	got, ok := next.(model)
	if !ok {
		t.Fatalf("Update returned a %T", next)
	}
	if cmd == nil {
		t.Error("enter on a plan-only mode did not leave")
	}
	if !got.accepted {
		t.Error("enter on a plan-only mode did not accept the run")
	}
	if got.confirming {
		t.Error("enter on a plan-only mode opened a confirmation")
	}
}

// TestEscClosesHelpInsteadOfQuitting is T-0345's first finding: Cancel was
// matched before Help in pressed(), so esc while the overlay was open quit the
// whole program. ctrl+c is left to still leave, because it is the terminal's
// interrupt and not a way to dismiss a screen.
func TestEscClosesHelpInsteadOfQuitting(t *testing.T) {
	m := press(t, fixtureModel(t), "?")
	if !m.showHelp {
		t.Fatal("? did not open the help")
	}

	next, cmd := m.Update(keyMsg("esc"))
	got, ok := next.(model)
	if !ok {
		t.Fatalf("Update returned a %T", next)
	}
	if cmd != nil {
		t.Error("esc quit the program instead of closing the help overlay")
	}
	if got.showHelp {
		t.Error("esc did not close the help overlay")
	}
	if got.accepted {
		t.Error("closing help accepted the run")
	}

	m = press(t, fixtureModel(t), "?")
	next, cmd = m.Update(keyMsg("ctrl+c"))
	got, ok = next.(model)
	if !ok {
		t.Fatalf("Update returned a %T", next)
	}
	if cmd == nil {
		t.Error("ctrl+c did not leave while help was open")
	}
	if !got.interrupted {
		t.Error("ctrl+c while help was open did not report an interrupt")
	}
}

// TestEscDiscardsAnAnswerRatherThanLeaving: while the prompt is open, cancel
// means "discard this answer". "q" is a character the reason needs, which is
// why it is a binding of its own.
func TestEscDiscardsAnAnswerRatherThanLeaving(t *testing.T) {
	m := fixtureModel(t)
	m = press(t, m, "u")
	m = typeText(t, m, "quick q")

	next, cmd := m.Update(keyMsg("esc"))
	got, ok := next.(model)
	if !ok {
		t.Fatalf("Update returned a %T", next)
	}
	if cmd != nil {
		t.Error("esc left the TUI instead of discarding the answer")
	}
	if got.prompt.active {
		t.Error("esc did not close the prompt")
	}
	if len(got.request().Unmask) != 0 {
		t.Error("a discarded answer was applied anyway")
	}
	if got.accepted {
		t.Error("discarding an answer accepted the run")
	}
}

// TestCtrlCInterruptsThePromptToo: promptKey matched Cancel before ctrl+c
// could be told apart from esc, so ctrl+c with the prompt open only discarded
// the answer instead of leaving the program (T-0345 review). ctrl+c must
// leave whatever screen or overlay is in front, the prompt included.
func TestCtrlCInterruptsThePromptToo(t *testing.T) {
	m := fixtureModel(t)
	m = press(t, m, "u")
	if !m.prompt.active {
		t.Fatal("u did not open the prompt")
	}

	next, cmd := m.Update(keyMsg("ctrl+c"))
	got, ok := next.(model)
	if !ok {
		t.Fatalf("Update returned a %T", next)
	}
	if cmd == nil {
		t.Error("ctrl+c did not leave while the prompt was open")
	}
	if !got.interrupted {
		t.Error("ctrl+c while the prompt was open did not report an interrupt")
	}
	if got.accepted {
		t.Error("ctrl+c accepted the run")
	}
}

// TestPlanOnlyRunModeWritesNoTarget: `lazyslice --plan --tui` builds a
// request with Mode == ModeRun and PlanOnly == true. core stops before the
// target is touched on PlanOnly alone (main.go's previewIsTheRun, core.run's
// `Mode == ModePlan || PlanOnly`), so writesTarget must say so too: the first
// enter must accept immediately, not open a confirmation that names a write
// that was never going to happen (T-0345 review).
func TestPlanOnlyRunModeWritesNoTarget(t *testing.T) {
	req := core.NewRequest() // zero Mode is ModeRun.
	req.PlanOnly = true
	m := newModel(req).seed(fixture())

	next, cmd := m.Update(keyMsg("enter"))
	got, ok := next.(model)
	if !ok {
		t.Fatalf("Update returned a %T", next)
	}
	if cmd == nil {
		t.Error("enter on a PlanOnly run did not leave")
	}
	if !got.accepted {
		t.Error("enter on a PlanOnly run did not accept")
	}
	if got.confirming {
		t.Error("enter on a PlanOnly run opened a confirmation naming a write that will not happen")
	}
}

// TestLiveEventsReachTheScreens: an event arriving as a tea.Msg is the same
// path seed takes, which is what makes internal/tui one more sink on the event
// channel rather than a reader of its own.
func TestLiveEventsReachTheScreens(t *testing.T) {
	m := newModel(core.NewRequest())
	if len(m.rows) != 0 {
		t.Fatalf("a fresh model already has %d rows", len(m.rows))
	}
	next, _ := m.Update(decision(codeColumnMasked, "customer", "email", "name rule: email"))
	got, ok := next.(model)
	if !ok {
		t.Fatalf("Update returned a %T", next)
	}
	if len(got.rows) != 1 || len(got.reasons.Rows()) != 1 {
		t.Fatalf("the event did not reach the reasons screen: %d rows, %d table rows",
			len(got.rows), len(got.reasons.Rows()))
	}
}

// TestTranscriptCarriesTheScreenAndTheFlags: ADR-002's reason for the line
// printer being the default is that the transcript survives the run, so the two
// screens must not be the one part of a run that leaves nothing behind.
func TestTranscriptCarriesTheScreenAndTheFlags(t *testing.T) {
	m := onPlan(t, fixtureModel(t))
	m = press(t, m, "n")
	m = typeText(t, m, "50")
	m = press(t, m, "enter")

	got := m.transcript()
	for _, want := range []string{"public.customer", "public.payment", "flags these screens set: --take 50", "0.3"} {
		if !strings.Contains(got, want) {
			t.Errorf("the transcript does not carry %q:\n%s", want, got)
		}
	}
}

// TestHelpNamesTheFlagOfEveryAction. "?" is where an operator learns that what
// they just did in the screen is a flag they can put in a script.
func TestHelpNamesTheFlagOfEveryAction(t *testing.T) {
	m := press(t, fixtureModel(t), "?")
	if !m.showHelp {
		t.Fatal("? did not open the help")
	}
	view := m.helpView()
	for _, b := range Bindings() {
		if b.Nav {
			continue
		}
		if !strings.Contains(view, "--"+b.Flag) {
			t.Errorf("the help does not name --%s", b.Flag)
		}
	}
}

// TestWindowSizeIsLaidOut: the model gets its size from the framework, and a
// terminal too small to lay out must still show a row rather than a negative
// height.
func TestWindowSizeIsLaidOut(t *testing.T) {
	m := fixtureModel(t)
	next, _ := m.Update(tea.WindowSizeMsg{Width: 40, Height: 6})
	got, ok := next.(model)
	if !ok {
		t.Fatalf("Update returned a %T", next)
	}
	if got.reasons.Height() < 1 {
		t.Errorf("the table is %d rows high on a short terminal", got.reasons.Height())
	}
	if got.render() == "" {
		t.Error("the model rendered nothing at 40x6")
	}
}

// TestAnEmptyCountKeepsTheValue: the count prompts start empty with the value
// in force in the label, so "enter" alone is "leave it as it is". An empty
// reason is not the same thing — --unmask has no bare form — and
// TestUnmaskRefusesAReasonlessOptOut holds that end.
func TestAnEmptyCountKeepsTheValue(t *testing.T) {
	m := onPlan(t, fixtureModel(t))
	m = press(t, m, "n", "enter")

	if got := m.request().Take; got != core.DefaultTake {
		t.Errorf("Take = %d, want the default %d", got, core.DefaultTake)
	}
	if m.prompt.active {
		t.Error("the prompt stayed open on an empty count")
	}
	if len(m.flags()) != 0 {
		t.Errorf("flags() = %q, want none", m.flags())
	}
}

// TestStrictSchemaTogglesFromTheReasonsScreen: --strict-schema is a classify
// flag and the reasons screen is the classify screen, so it is reachable there
// and it is the same field the flag sets.
func TestStrictSchemaTogglesFromTheReasonsScreen(t *testing.T) {
	m := press(t, fixtureModel(t), "S")
	if !m.request().StrictSchema {
		t.Fatal("S did not set --strict-schema")
	}
	if !slices.Contains(m.flags(), "--strict-schema") {
		t.Errorf("flags() = %q, want --strict-schema", m.flags())
	}
	if m = press(t, m, "S"); m.request().StrictSchema {
		t.Error("S did not toggle back off")
	}
}

// TestSkippingBackAgainRedrawsTheRow is the other half of the same rule: "x" on
// a table that is already skipped puts it back in the slice, and the row has to
// stop saying it was skipped.
func TestSkippingBackAgainRedrawsTheRow(t *testing.T) {
	m := onPlan(t, fixtureModel(t))
	m = press(t, m, "down", "down", "x", "x")

	if got := m.plan.Rows()[2][3]; strings.HasPrefix(got, "skipped") {
		t.Errorf("the row still reads %q after the skip was taken off", got)
	}
	if len(m.request().SkipTables) != 0 {
		t.Errorf("SkipTables = %q, want none", m.request().SkipTables)
	}
}

// TestAnOptOutIsTheSameOptOutHoweverItWasSpelled: --unmask takes TABLE.COL as
// well as SCHEMA.TABLE.COL, and cmd/lazyslice stores the string the operator
// typed. Both spellings are one opt-out to internal/core, so both have to be
// one opt-out to the screen — otherwise `--tui --unmask customer.email=...`
// shows the column as copied with "u" struck through, and the operator cannot
// restore masking on a column that is about to be copied in clear.
func TestAnOptOutIsTheSameOptOutHoweverItWasSpelled(t *testing.T) {
	req := core.NewRequest()
	req.Unmask["customer.email"] = "support reads it in the ticket"
	m := newModel(req).seed(fixture())

	if got := m.reasons.Rows()[0][1]; got != "opt-out" {
		t.Errorf("the decision cell reads %q, want opt-out", got)
	}
	// A column the classifier copied, opted out under the bare spelling, is the
	// case the qualified-key lookup got wrong: Masked is false, so the only
	// thing that can make "u" available is finding the opt-out.
	copied := newModel(bareOptOut("customer.store_id")).seed(fixture())
	copied = press(t, copied, "down")
	if !copied.available(copied.keys.Unmask) {
		t.Error("u is struck through on a copied column that this run opted out, so masking cannot be restored")
	}
	if got := copied.reasons.Rows()[1][1]; got != "opt-out" {
		t.Errorf("the copied column's decision cell reads %q, want opt-out", got)
	}

	m = press(t, m, "u")
	if m.prompt.active {
		t.Fatal("removing an opt-out asked for a reason")
	}
	if len(m.request().Unmask) != 0 {
		t.Errorf("Unmask = %v, want the bare-spelled opt-out gone", m.request().Unmask)
	}
	if got := m.reasons.Rows()[0][1]; got != "masked" {
		t.Errorf("the decision cell reads %q after the opt-out was removed, want masked", got)
	}
}

// bareOptOut is a request carrying one --unmask spelled TABLE.COL, which is
// what cmd/lazyslice stores when the operator spells it that way.
func bareOptOut(name string) core.Request {
	req := core.NewRequest()
	req.Unmask[name] = "support reads it in the ticket"
	return req
}

// TestAQuotedOptOutIsRecognisedToo: internal/core's resolveColumn splits on the
// dots outside double quotes, so a column whose table needs quoting is still
// the same opt-out. The screen splits it the same way.
func TestAQuotedOptOutIsRecognisedToo(t *testing.T) {
	req := core.NewRequest()
	req.Unmask[`"public"."customer".email`] = "reviewed"
	m := newModel(req).seed(fixture())

	if got := m.reasons.Rows()[0][1]; got != "opt-out" {
		t.Errorf("the decision cell reads %q, want opt-out", got)
	}
}
