// SPDX-License-Identifier: Apache-2.0

package core

import (
	"bytes"
	"strconv"
	"strings"
	"testing"

	"github.com/Liarea/lazyslice/internal/event"
	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
	"github.com/Liarea/lazyslice/internal/render"
)

// ARCHITECTURE.md section 3's defaults are substituted here and nowhere else:
// the planner takes the request as given, so a zero reaching it would mean
// "take no rows" rather than "take 500" (T-CORE).
func TestNormaliseSubstitutesTheDefaults(t *testing.T) {
	got := normalise(Request{})

	if got.Take != DefaultTake || got.Cap != DefaultCap || got.Depth != DefaultDepth {
		t.Errorf("take/cap/depth = %d/%d/%d, want %d/%d/%d",
			got.Take, got.Cap, got.Depth, DefaultTake, DefaultCap, DefaultDepth)
	}
	if got.RowBudget != DefaultRowBudget || got.MemoryBudget != DefaultMemoryBudget {
		t.Errorf("budgets = %d/%q, want %d/%q",
			got.RowBudget, got.MemoryBudget, DefaultRowBudget, DefaultMemoryBudget)
	}
	if got.ConfigPath != DefaultConfigPath || got.SecretFile != DefaultSecretFile {
		t.Errorf("paths = %q/%q", got.ConfigPath, got.SecretFile)
	}
	if got.Explicit == nil || got.TableCaps == nil || got.Keys == nil || got.Unmask == nil {
		t.Error("normalise left a map nil, so a flag parser would panic on it")
	}

	// A value the caller set survives.
	set := normalise(Request{Take: 7, Cap: 1, Depth: 1})
	if set.Take != 7 || set.Cap != 1 || set.Depth != 1 {
		t.Errorf("normalise overwrote a value the caller set: %d/%d/%d", set.Take, set.Cap, set.Depth)
	}
}

// The five subcommands stop at five different places, and only two of them
// need a target at all — which is what lets `lazyslice classify --source URL`
// answer with no second database.
func TestModeNeedsTarget(t *testing.T) {
	for mode, want := range map[Mode]bool{
		ModeRun:        true,
		ModeVerify:     true,
		ModeIntrospect: false,
		ModeClassify:   false,
		ModePlan:       false,
		ModeDoctor:     false,
	} {
		if got := mode.needsTarget(); got != want {
			t.Errorf("%s.needsTarget() = %v, want %v", mode, got, want)
		}
	}
}

// A flag or a yml key that names nothing must be refused rather than stored: an
// --unmask that silently never applied looks exactly like one that did.
func TestResolveNames(t *testing.T) {
	schema := &pipeline.Schema{Tables: []pipeline.Table{
		{
			Ref:     ref.TableRef{Schema: "public", Name: "people"},
			Columns: []pipeline.Column{{Name: "email"}, {Name: "notes"}},
		},
		{
			Ref:     ref.TableRef{Schema: "billing", Name: "people"},
			Columns: []pipeline.Column{{Name: "email"}},
		},
		{
			Ref:     ref.TableRef{Schema: "public", Name: "LegacyCustomer"},
			Columns: []pipeline.Column{{Name: "EmailAddress"}},
		},
	}}

	if got, err := resolveTable("public.people", schema); err != nil || got.Name != "people" {
		t.Errorf("resolveTable(public.people) = %v, %v", got, err)
	}
	if _, err := resolveTable("people", schema); err == nil {
		t.Error("a bare name in two schemas must be refused, not silently resolved to one of them")
	}
	if got, err := resolveTable("LegacyCustomer", schema); err != nil || got.Schema != "public" {
		t.Errorf("resolveTable(LegacyCustomer) = %v, %v; a bare name in one schema resolves", got, err)
	}

	col, err := resolveColumn("public.people.email", schema)
	if err != nil || col.Column != "email" {
		t.Errorf("resolveColumn(public.people.email) = %v, %v", col, err)
	}
	quoted, err := resolveColumn(`public."LegacyCustomer"."EmailAddress"`, schema)
	if err != nil || quoted.Column != "EmailAddress" {
		t.Errorf("resolveColumn of a quoted name = %v, %v", quoted, err)
	}
	if _, err := resolveColumn("public.people.nope", schema); err == nil {
		t.Error("a column the source does not have must be refused")
	}
}

// ARCHITECTURE.md section 5's small-domain rule, which nothing else in the tree
// applies: an enum with six labels over five distinct sampled values is a
// substitution recoverable by frequency, and both the yml and the residual
// filter depend on the run saying so (domain.go).
func TestSmallDomainIsMarked(t *testing.T) {
	table := ref.TableRef{Schema: "public", Name: "people"}
	marital := ref.ColumnRef{Table: table, Column: "marital_status"}
	notes := ref.ColumnRef{Table: table, Column: "notes"}

	schema := &pipeline.Schema{
		Enums: map[string][]string{
			"public.marital_status": {"single", "married", "civil_partnership", "divorced", "widowed", "undisclosed"},
		},
		Tables: []pipeline.Table{{
			Ref: table,
			Columns: []pipeline.Column{
				{Name: "marital_status", TypeName: "public.marital_status"},
				{Name: "notes", TypeName: "text"},
			},
			Samples: [][]any{
				{"single", "a"},
				{"married", "b"},
				{"divorced", "c"},
				{"widowed", "d"},
				{"undisclosed", "e"},
			},
		}},
	}
	cls := &pipeline.Classification{Decisions: map[ref.ColumnRef]pipeline.Decision{
		marital: {Col: marital, Masked: true},
		notes:   {Col: notes, Masked: true},
	}}

	markSmallDomains(schema, cls)

	if d := cls.Decisions[marital]; d.Domain != 6 || !d.SmallDomain {
		t.Errorf("marital_status: domain = %d, small = %v, want 6 and true", d.Domain, d.SmallDomain)
	}
	if d := cls.Decisions[notes]; d.Domain != 0 || d.SmallDomain {
		t.Errorf("notes: domain = %d, small = %v, want unknown and false", d.Domain, d.SmallDomain)
	}

	// And the filter leaves the small-domain column out, because a match on it
	// is evidence about the domain and not about the masker (section 6 item 6).
	f := smallDomainAware{Residual: recorder{}, exempt: smallDomainColumns(cls)}
	if f.exempt[marital] != true || f.exempt[notes] {
		t.Errorf("exempt = %v, want only marital_status", f.exempt)
	}
}

// recorder is a pipeline.Residual that does nothing. The wrapper's behaviour is
// which columns it forwards, not what the filter does with them.
type recorder struct{}

func (recorder) Add(ref.ColumnRef, string, []byte)             {}
func (recorder) MayContain(ref.ColumnRef, string, []byte) bool { return false }
func (recorder) Cells() int64                                  { return 0 }
func (recorder) Bytes() int64                                  { return 0 }

// lazyslice_meta is lazyslice's own bookkeeping, and no stage may see it: the
// classifier scores secret_fingerprint as a credential, and the planner would
// give it a step (run.go, dropMarkerTable).
func TestMarkerTableLeavesTheCatalog(t *testing.T) {
	marker := ref.TableRef{Schema: "public", Name: "lazyslice_meta"}
	people := ref.TableRef{Schema: "public", Name: "people"}
	schema := &pipeline.Schema{
		Tables: []pipeline.Table{{Ref: marker}, {Ref: people}},
		FKs:    []pipeline.ForeignKey{{Name: "fk", Child: marker, Parent: people}},
	}

	dropMarkerTable(schema)

	if len(schema.Tables) != 1 || schema.Tables[0].Ref != people {
		t.Errorf("tables = %v, want only public.people", schema.Tables)
	}
	if len(schema.FKs) != 0 {
		t.Errorf("FKs = %v, want the edge into the marker dropped with it", schema.FKs)
	}
}

// A domain that is "small" by section 5's ratio and large in absolute terms is
// not exempt from the residual filter (smallDomainCeiling, domain.go). The
// ratio alone had no upper bound, so a three-hundred-label enum with two hundred
// distinct labels in the sample dropped out of both the filter and invariant
// I2 — a column nothing checks.
func TestALargeDomainIsNeverSmall(t *testing.T) {
	table := ref.TableRef{Schema: "public", Name: "visits"}
	clinic := ref.ColumnRef{Table: table, Column: "clinic"}

	labels := make([]string, 0, 300)
	samples := make([][]any, 0, 200)
	for i := range 300 {
		labels = append(labels, "clinic_"+strconv.Itoa(i))
	}
	for i := range 200 {
		samples = append(samples, []any{"clinic_" + strconv.Itoa(i)})
	}

	schema := &pipeline.Schema{
		Enums:  map[string][]string{"public.clinic": labels},
		Tables: []pipeline.Table{{Ref: table, Columns: []pipeline.Column{{Name: "clinic", TypeName: "public.clinic"}}, Samples: samples}},
	}
	cls := &pipeline.Classification{Decisions: map[ref.ColumnRef]pipeline.Decision{
		clinic: {Col: clinic, Masked: true},
	}}

	markSmallDomains(schema, cls)

	d := cls.Decisions[clinic]
	if d.Domain != 300 {
		t.Errorf("domain = %d, want 300", d.Domain)
	}
	if d.SmallDomain {
		t.Errorf("300 labels is small by the ratio (300 < 2×200) and not small in absolute terms: "+
			"marking it small takes it out of the residual filter and out of I2, and %d is the ceiling",
			smallDomainCeiling)
	}
}

// Section 11.2 requires three warnings on a bound marker written by another
// run. Every one of them compares something that does not exist until after
// discover — the secret fingerprint and the classification — so calling
// markerWarnings from openTarget, as this did, could never print any of them.
func TestMarkerWarningsFireAfterTheKeyAndTheClassification(t *testing.T) {
	sink := &collector{}
	r := &run{
		req:  normalise(Request{}),
		sink: sink,
		gate: pipeline.Eligibility{
			MarkerBound:     true,
			PrevKeyFP:       "old-key",
			PrevClassFP:     "old-class",
			PrevToolVersion: "0.0.1-old",
		},
		keyFP: "new-key",
		cls:   &pipeline.Classification{Fingerprint: "new-class"},
	}

	r.markerWarnings()

	for _, want := range []event.Code{CodeSecretChanged, CodeClassChanged, CodeVersionChanged} {
		if !sink.has(want) {
			t.Errorf("%s was not emitted; section 11.2 requires all three", want)
		}
	}

	// An unbound marker authorises nothing and says nothing.
	quiet := &collector{}
	(&run{req: normalise(Request{}), sink: quiet}).markerWarnings()
	if len(quiet.codes) != 0 {
		t.Errorf("an unbound marker emitted %v", quiet.codes)
	}
}

// A followed virtual edge is printed under plan.polymorphic.inferred (§3.2
// amended 2026-09-08, T-POLY): virtualEvents is the loop planStage calls for
// every entry of Plan.Virtual, and this pins its one event, its args and that
// the rendered line carries no unfilled placeholder.
func TestVirtualEdgeIsPrintedAsInferred(t *testing.T) {
	events := &eventCollector{}
	r := &run{sink: events}
	child := ref.TableRef{Schema: "public", Name: "attachments"}
	parent := ref.TableRef{Schema: "public", Name: "people"}
	p := &pipeline.Plan{
		Virtual: []pipeline.ForeignKey{
			{
				Name:       "public.attachments.owner_type",
				Child:      child,
				ChildCols:  []string{"owner_id"},
				Parent:     parent,
				ParentCols: []string{"person_id"},
				Virtual:    true,
			},
		},
	}

	r.virtualEvents(p)

	var got []event.Event
	for _, e := range events.events {
		if e.Code == CodePlanPolymorphicInferred {
			got = append(got, e)
		}
	}
	if len(got) != 1 {
		t.Fatalf("virtualEvents emitted %d plan.polymorphic.inferred events, want exactly 1: %v", len(got), got)
	}
	e := got[0]
	if e.Args[event.ArgColumn] != "owner_type" {
		t.Errorf("column arg = %q, want the bare discriminator column %q", e.Args[event.ArgColumn], "owner_type")
	}
	if e.Args[event.ArgTable] != "public.attachments (owner_id)" {
		t.Errorf("table arg = %q, want the child table and column", e.Args[event.ArgTable])
	}
	if e.Args[event.ArgReason] != "public.people" {
		t.Errorf("reason arg = %q, want the parent table %q", e.Args[event.ArgReason], "public.people")
	}

	var buf bytes.Buffer
	render.NewLines(&buf).Send(e)
	line := buf.String()
	if strings.ContainsAny(line, "{}") {
		t.Errorf("rendered %q, want no unfilled placeholder", line)
	}
	if !strings.Contains(line, "owner_type") || !strings.Contains(line, "public.attachments") ||
		!strings.Contains(line, "public.people") {
		t.Errorf("rendered %q, want the column, the child and the parent all named", line)
	}
}

// eventCollector is an event.Sink that keeps every event it was sent, for
// tests that need the args and not only the codes.
type eventCollector struct{ events []event.Event }

func (c *eventCollector) Send(e event.Event) { c.events = append(c.events, e) }

// collector is an event.Sink that keeps the codes it was sent.
type collector struct{ codes []event.Code }

func (c *collector) Send(e event.Event) { c.codes = append(c.codes, e.Code) }

func (c *collector) has(code event.Code) bool {
	for _, got := range c.codes {
		if got == code {
			return true
		}
	}
	return false
}
