// SPDX-License-Identifier: Apache-2.0

package core

import (
	"strconv"
	"testing"

	"github.com/Liarea/lazyslice/internal/discover"
	"github.com/Liarea/lazyslice/internal/event"
	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
)

// rootQuestionSchema is three tables with a clear score order (§3.1):
// public.customers (1 inbound, 0 outbound) ranks above public.orders (1
// inbound, 1 outbound), which ranks above public.order_items (0 inbound, 1
// outbound). None is lookup-shaped: every ApproxRows is well past the
// thousand-row discard and none matches the lookup name pattern.
func rootQuestionSchema() *pipeline.Schema {
	customers := ref.TableRef{Schema: "public", Name: "customers"}
	orders := ref.TableRef{Schema: "public", Name: "orders"}
	items := ref.TableRef{Schema: "public", Name: "order_items"}
	return &pipeline.Schema{
		Tables: []pipeline.Table{
			{Ref: customers, ApproxRows: 5000},
			{Ref: orders, ApproxRows: 5000},
			{Ref: items, ApproxRows: 5000},
		},
		FKs: []pipeline.ForeignKey{
			{Name: "orders_customer_fk", Child: orders, Parent: customers},
			{Name: "items_order_fk", Child: items, Parent: orders},
		},
	}
}

// fakeQ2Prompter is a discover.Prompter whose Ask answers come off a queue,
// one per call: "" is a bare Enter (Ask's own contract: an empty answer means
// take def), and an exhausted queue is the terminal going away mid-question,
// which Ask itself reports as discover.ErrNoTerminal.
type fakeQ2Prompter struct {
	answers []string
}

func (f *fakeQ2Prompter) Confirm(_ string, def bool) (bool, error) { return def, nil }

func (f *fakeQ2Prompter) Ask(_ string, def string) (string, error) {
	if len(f.answers) == 0 {
		return def, discover.ErrNoTerminal
	}
	a := f.answers[0]
	f.answers = f.answers[1:]
	if a == "" {
		return def, nil
	}
	return a, nil
}

func (f *fakeQ2Prompter) Close() error { return nil }

// rootDecision finds the one plan.root.decided event a case should have sent
// exactly once, and fails the test if it is missing or duplicated.
func rootDecision(t *testing.T, c *eventCollector) event.Event {
	t.Helper()
	var found []event.Event
	for _, e := range c.events {
		if e.Code == CodeRootChosen {
			found = append(found, e)
		}
	}
	if len(found) != 1 {
		t.Fatalf("%s sent %d time(s), want exactly 1", CodeRootChosen, len(found))
	}
	return found[0]
}

func countEvents(c *eventCollector, code event.Code) int {
	n := 0
	for _, e := range c.events {
		if e.Code == code {
			n++
		}
	}
	return n
}

// TestRootQuestionDefaultOnEnter is ADR-008 §6 Q2: a bare Enter at the prompt
// takes the top-scoring table, and the decision line names it with the
// score-components reason.
func TestRootQuestionDefaultOnEnter(t *testing.T) {
	c := &eventCollector{}
	r := &run{
		req:    normalise(Request{prompter: &fakeQ2Prompter{answers: []string{""}}}),
		sink:   c,
		schema: rootQuestionSchema(),
	}
	if err := r.rootQuestion(); err != nil {
		t.Fatalf("rootQuestion: %v", err)
	}
	d := rootDecision(t, c)
	if got := d.Args[event.ArgTable]; got != "public.customers" {
		t.Errorf("root = %q, want public.customers (the top-scoring table)", got)
	}
	if got := d.Args[event.ArgReason]; got != "1 inbound - 0 outbound FKs" {
		t.Errorf("reason = %q, want the score components", got)
	}
	if got := d.Args[event.ArgFlag]; got != "--root" {
		t.Errorf("flag = %q, want --root", got)
	}
	if r.req.Root != "" {
		t.Errorf("r.req.Root = %q, want empty: Enter takes the default, it does not name one", r.req.Root)
	}
}

// TestRootQuestionNamedTable is a table name typed at the prompt, other than
// the default, taking that table.
func TestRootQuestionNamedTable(t *testing.T) {
	c := &eventCollector{}
	r := &run{
		req:    normalise(Request{prompter: &fakeQ2Prompter{answers: []string{"public.orders"}}}),
		sink:   c,
		schema: rootQuestionSchema(),
	}
	if err := r.rootQuestion(); err != nil {
		t.Fatalf("rootQuestion: %v", err)
	}
	d := rootDecision(t, c)
	if got := d.Args[event.ArgTable]; got != "public.orders" {
		t.Errorf("root = %q, want public.orders", got)
	}
	if got := d.Args[event.ArgReason]; got != "named at the prompt" {
		t.Errorf("reason = %q, want %q", got, "named at the prompt")
	}
	// planRequest's own --root case is what validates this answer, exactly as
	// it validates --root itself: the answer must have been carried forward,
	// as a resolved ref.TableRef (r.qRoot) rather than round-tripped through
	// r.req.Root's unquoted string, which planRequest would parse a second
	// time and could disagree with (T-0271 review).
	if r.qRoot == nil || *r.qRoot != (ref.TableRef{Schema: "public", Name: "orders"}) {
		t.Errorf("r.qRoot = %v, want public.orders fed forward for planRequest to use", r.qRoot)
	}
	if r.req.Root != "" {
		t.Errorf("r.req.Root = %q, want empty: the answer travels on r.qRoot, not a re-rendered string", r.req.Root)
	}
}

// TestRootQuestionQuestionMark is "?": it prints the ranked top candidates
// with their score components and re-asks, rather than taking anything.
func TestRootQuestionQuestionMark(t *testing.T) {
	c := &eventCollector{}
	r := &run{
		req:    normalise(Request{prompter: &fakeQ2Prompter{answers: []string{"?", "public.order_items"}}}),
		sink:   c,
		schema: rootQuestionSchema(),
	}
	if err := r.rootQuestion(); err != nil {
		t.Fatalf("rootQuestion: %v", err)
	}

	wantRanked := []struct{ table, reason string }{
		{"public.customers", "1 inbound - 0 outbound FKs"},
		{"public.orders", "1 inbound - 1 outbound FKs"},
		{"public.order_items", "0 inbound - 1 outbound FKs"},
	}
	var got []event.Event
	for _, e := range c.events {
		if e.Code == CodeRootCandidate {
			got = append(got, e)
		}
	}
	if len(got) != len(wantRanked) {
		t.Fatalf("%s sent %d time(s), want %d", CodeRootCandidate, len(got), len(wantRanked))
	}
	for i, want := range wantRanked {
		if rank := got[i].Args[event.ArgCount]; rank != strconv.Itoa(i+1) {
			t.Errorf("candidate %d: rank arg = %q, want %d", i, rank, i+1)
		}
		if table := got[i].Args[event.ArgTable]; table != want.table {
			t.Errorf("candidate %d: table = %q, want %q", i, table, want.table)
		}
		if reason := got[i].Args[event.ArgReason]; reason != want.reason {
			t.Errorf("candidate %d: reason = %q, want %q", i, reason, want.reason)
		}
	}

	d := rootDecision(t, c)
	if got := d.Args[event.ArgTable]; got != "public.order_items" {
		t.Errorf("root = %q, want public.order_items (the answer after '?')", got)
	}
}

// TestRootQuestionUnknownName re-asks, with the reason, when the typed answer
// names no table — and re-asking is what lets a later, valid answer still
// take the run.
func TestRootQuestionUnknownName(t *testing.T) {
	c := &eventCollector{}
	r := &run{
		req:    normalise(Request{prompter: &fakeQ2Prompter{answers: []string{"nosuchtable", "public.order_items"}}}),
		sink:   c,
		schema: rootQuestionSchema(),
	}
	if err := r.rootQuestion(); err != nil {
		t.Fatalf("rootQuestion: %v", err)
	}

	if n := countEvents(c, CodeRootUnknown); n != 1 {
		t.Fatalf("%s sent %d time(s), want 1", CodeRootUnknown, n)
	}
	var warned event.Event
	for _, e := range c.events {
		if e.Code == CodeRootUnknown {
			warned = e
		}
	}
	if reason := warned.Args[event.ArgReason]; reason == "" {
		t.Error("plan.root.unknown carries no reason")
	}

	d := rootDecision(t, c)
	if got := d.Args[event.ArgTable]; got != "public.order_items" {
		t.Errorf("root = %q, want public.order_items (the valid answer after the unknown one)", got)
	}
}

// TestRootQuestionNoControllingTerminal is ADR-008's own
// TestNoControllingTerminalBehavesAsYes for Q2: with neither --yes nor a
// controlling terminal, Q2 asks nothing and takes the default.
//
// noTerminal forces the "no controlling terminal" branch deterministically
// (discover.Options.NoControllingTerminal, via the run's own noTerminal
// field) rather than relying on the process running this test to have none.
// The original version of this test passed a bare Request{} and depended on
// go test's own ambient environment: that holds under CI, which starts with
// no controlling terminal, but not for a developer running `go test
// ./internal/core` from an interactive shell, where a real /dev/tty opens,
// rootQuestion takes the "somebody is here" branch, askRoot's discover.Prompter
// writes "root table? [public.customers]" to that terminal and blocks in
// bufio.Reader.ReadString waiting for an answer nobody is there to type —
// hanging the whole suite rather than testing the no-terminal path at all
// (T-0271 review).
func TestRootQuestionNoControllingTerminal(t *testing.T) {
	c := &eventCollector{}
	r := &run{
		req:    normalise(Request{noTerminal: true}),
		sink:   c,
		schema: rootQuestionSchema(),
	}
	if err := r.rootQuestion(); err != nil {
		t.Fatalf("rootQuestion: %v", err)
	}
	if n := countEvents(c, CodeRootCandidate); n != 0 {
		t.Errorf("%s sent %d time(s), want 0: no terminal, nothing was asked", CodeRootCandidate, n)
	}
	if n := countEvents(c, CodeRootUnknown); n != 0 {
		t.Errorf("%s sent %d time(s), want 0: no terminal, nothing was asked", CodeRootUnknown, n)
	}
	d := rootDecision(t, c)
	if got := d.Args[event.ArgTable]; got != "public.customers" {
		t.Errorf("root = %q, want public.customers (the default, taken silently)", got)
	}
}

// TestRootQuestionYes is --yes: headless by discover's own definition, so Q2
// takes the default the same way as with no controlling terminal.
func TestRootQuestionYes(t *testing.T) {
	c := &eventCollector{}
	r := &run{
		req:    normalise(Request{Yes: true, prompter: &fakeQ2Prompter{answers: []string{"public.orders"}}}),
		sink:   c,
		schema: rootQuestionSchema(),
	}
	if err := r.rootQuestion(); err != nil {
		t.Fatalf("rootQuestion: %v", err)
	}
	// The prompter is supplied only to prove --yes is what stops it being
	// asked, not the absence of a Prompter to ask through.
	if n := countEvents(c, CodeRootCandidate); n != 0 {
		t.Errorf("%s sent %d time(s), want 0: --yes takes the default", CodeRootCandidate, n)
	}
	d := rootDecision(t, c)
	if got := d.Args[event.ArgTable]; got != "public.customers" {
		t.Errorf("root = %q, want public.customers (the default; --yes never reads the prompter)", got)
	}
}

// TestRootQuestionRootFlagGiven is --root: Q2 never opens a prompter at all,
// and the decision names the flag.
func TestRootQuestionRootFlagGiven(t *testing.T) {
	c := &eventCollector{}
	r := &run{
		req:    normalise(Request{Root: "public.orders", prompter: &fakeQ2Prompter{}}),
		sink:   c,
		schema: rootQuestionSchema(),
	}
	if err := r.rootQuestion(); err != nil {
		t.Fatalf("rootQuestion: %v", err)
	}
	if n := countEvents(c, CodeRootCandidate) + countEvents(c, CodeRootUnknown); n != 0 {
		t.Errorf("Q2 sent %d prompt-only event(s) with --root given, want 0", n)
	}
	d := rootDecision(t, c)
	if got := d.Args[event.ArgTable]; got != "public.orders" {
		t.Errorf("root = %q, want public.orders", got)
	}
	if got := d.Args[event.ArgReason]; got != "named by --root" {
		t.Errorf("reason = %q, want %q", got, "named by --root")
	}
}

// TestRootQuestionReviewedRoot is the T-0271 review's finding 5: a --tui
// second pass whose Request carries the preview pass's Reviewed root asks Q2
// nothing at all and takes that root, the same way --root and a committed yml
// do, rather than asking the operator the same question a second time.
func TestRootQuestionReviewedRoot(t *testing.T) {
	c := &eventCollector{}
	root := ref.TableRef{Schema: "public", Name: "orders"}
	r := &run{
		req:    normalise(Request{Reviewed: &Reviewed{Root: root}, prompter: &fakeQ2Prompter{}}),
		sink:   c,
		schema: rootQuestionSchema(),
	}
	if err := r.rootQuestion(); err != nil {
		t.Fatalf("rootQuestion: %v", err)
	}
	if n := countEvents(c, CodeRootCandidate) + countEvents(c, CodeRootUnknown); n != 0 {
		t.Errorf("Q2 sent %d prompt-only event(s) with a reviewed root, want 0", n)
	}
	d := rootDecision(t, c)
	if got := d.Args[event.ArgTable]; got != "public.orders" {
		t.Errorf("root = %q, want public.orders (the reviewed root)", got)
	}
	if r.qRoot != nil {
		t.Errorf("r.qRoot = %v, want nil: planRequest reads Request.Reviewed directly, not r.qRoot", r.qRoot)
	}
}

// TestRootQuestionYmlRoot is a committed lazyslice.yml naming a root: Q2
// takes it and never opens a prompter, same as --root.
func TestRootQuestionYmlRoot(t *testing.T) {
	c := &eventCollector{}
	r := &run{
		req:    normalise(Request{ConfigPath: "lazyslice.yml", prompter: &fakeQ2Prompter{}}),
		sink:   c,
		schema: rootQuestionSchema(),
		prior:  &pipeline.Config{Root: ref.TableRef{Schema: "public", Name: "order_items"}},
	}
	if err := r.rootQuestion(); err != nil {
		t.Fatalf("rootQuestion: %v", err)
	}
	if n := countEvents(c, CodeRootCandidate) + countEvents(c, CodeRootUnknown); n != 0 {
		t.Errorf("Q2 sent %d prompt-only event(s) with a yml root, want 0", n)
	}
	d := rootDecision(t, c)
	if got := d.Args[event.ArgTable]; got != "public.order_items" {
		t.Errorf("root = %q, want public.order_items (the yml's own root)", got)
	}
}

// partitionedRootSchema reproduces the T-0271 review's finding 2: a
// partitioned root (public.events) whose own leaf (public.events_2024_01,
// Table.Parent set) carries a huge row count and the only inbound foreign
// key in the schema. Ranking the *raw* schema (as rootQuestion did before
// CollapseForRanking) scores the leaf 1 inbound - 0 outbound, ahead of
// public.events at 0 - 0, and picks the leaf as the default root — a table
// build() drops before the planner ever sees it, so chooseRoot's own
// defaultRoot, over the collapsed set, could never agree. Collapsing first
// drops the leaf and the foreign key that names it (its parent is the leaf,
// which is no longer in scope), leaving public.events and public.audit tied
// at 0 - 0 and public.events winning the tie on row count.
func partitionedRootSchema() *pipeline.Schema {
	events := ref.TableRef{Schema: "public", Name: "events"}
	leaf := ref.TableRef{Schema: "public", Name: "events_2024_01"}
	audit := ref.TableRef{Schema: "public", Name: "audit"}
	return &pipeline.Schema{
		Tables: []pipeline.Table{
			{Ref: events, ApproxRows: 10000},
			{Ref: leaf, ApproxRows: 900000, Parent: &events},
			{Ref: audit, ApproxRows: 5000},
		},
		FKs: []pipeline.ForeignKey{
			// The leaf's own inbound edge: raw RankRoots scores the leaf 1 - 0,
			// ahead of every non-partition table in this schema.
			{Name: "audit_events_leaf_fk", Child: audit, Parent: leaf},
		},
	}
}

// TestRootQuestionMatchesThePlannerOverAPartition is the T-0271 review's
// finding 2: Q2's default must be the table chooseRoot would actually plan
// from, over the identical collapsed set — not a partition leaf ranked from
// the raw, uncollapsed schema.
func TestRootQuestionMatchesThePlannerOverAPartition(t *testing.T) {
	c := &eventCollector{}
	r := &run{
		req:    normalise(Request{noTerminal: true}),
		sink:   c,
		schema: partitionedRootSchema(),
	}
	if err := r.rootQuestion(); err != nil {
		t.Fatalf("rootQuestion: %v", err)
	}
	d := rootDecision(t, c)
	if got := d.Args[event.ArgTable]; got != "public.events" {
		t.Errorf("root = %q, want public.events (chooseRoot's own default over the collapsed schema); "+
			"public.events_2024_01 is a partition leaf build() drops and chooseRoot can never pick", got)
	}
	if got := d.Args[event.ArgReason]; got != "0 inbound - 0 outbound FKs" {
		t.Errorf("reason = %q, want the collapsed score (the leaf's own inbound edge must not count here)", got)
	}
}

// TestRootQuestionUnknownQualifiedName is the T-0271 review's finding 3: a
// schema-qualified answer that names no table in the source re-asks exactly
// as an unqualified one does. resolveTable's qualified branch takes a name
// as given, with no catalog lookup at all (names.go), so before this a typo
// like "public.nope" resolved to a TableRef with no error, and Q2 printed a
// decision naming a table that does not exist, refused minutes later by
// planStage instead of re-asked here.
func TestRootQuestionUnknownQualifiedName(t *testing.T) {
	c := &eventCollector{}
	r := &run{
		req:    normalise(Request{prompter: &fakeQ2Prompter{answers: []string{"public.nope", "public.orders"}}}),
		sink:   c,
		schema: rootQuestionSchema(),
	}
	if err := r.rootQuestion(); err != nil {
		t.Fatalf("rootQuestion: %v", err)
	}
	if n := countEvents(c, CodeRootUnknown); n != 1 {
		t.Fatalf("%s sent %d time(s), want 1: a schema-qualified name matching nothing must re-ask", CodeRootUnknown, n)
	}
	d := rootDecision(t, c)
	if got := d.Args[event.ArgTable]; got != "public.orders" {
		t.Errorf("root = %q, want public.orders (the valid answer after the unknown qualified one)", got)
	}
}

// TestPlanRequestReviewedRootWithADot is the T-0271 review's finding 5's own
// fix. A table (or schema) name containing a dot is a legal Postgres
// identifier (CREATE TABLE "my.table"), and before this fix a --tui second
// pass whose Reviewed.Root named one broke: cmd/lazyslice's pinned() rendered
// it onto Request.Root as ref.TableRef.String()'s unquoted "schema.name" —
// indistinguishable from a *three*-part qualified name — and planRequest's own
// resolveTable split "public.my.table" on the wrong dot and refused with
// plan.CodeNoRoot, after the operator had already reviewed and approved the
// plan on the screens. planRequest must instead read the resolved
// ref.TableRef straight off Request.Reviewed and plan from it untouched.
func TestPlanRequestReviewedRootWithADot(t *testing.T) {
	root := ref.TableRef{Schema: "public", Name: "my.table"}
	r := &run{
		req:    normalise(Request{Reviewed: &Reviewed{Root: root}}),
		schema: &pipeline.Schema{Tables: []pipeline.Table{{Ref: root}}},
	}
	req, err := r.planRequest()
	if err != nil {
		t.Fatalf("planRequest = %v, want the reviewed root to plan cleanly, not refuse", err)
	}
	if req.Root == nil || *req.Root != root {
		t.Errorf("planRequest's Root = %v, want %v (the reviewed root, not re-parsed from a rendered string)",
			req.Root, root)
	}
}
