// SPDX-License-Identifier: Apache-2.0

package core

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/Liarea/lazyslice/internal/classify"
	"github.com/Liarea/lazyslice/internal/event"
	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
)

// T-0404 (the 2026-09-25 JSON red team, round 1, entry 28): a re-run from a
// committed yml whose source documents grew a key reported "0 drift" and
// copied the new key's leaves, and --strict-schema passed. Every value here
// is invented.

var t404Accounts = ref.TableRef{Schema: "public", Name: "t404_accounts"}

func t404Schema(extra string) *pipeline.Schema {
	var rows [][]any
	for g := 1; g <= 5; g++ {
		rows = append(rows, []any{int64(g), fmt.Sprintf(`{"theme": "dark", "layout": "grid-%d"%s}`, g, extra)})
	}
	return &pipeline.Schema{Tables: []pipeline.Table{{
		Ref: t404Accounts,
		Columns: []pipeline.Column{
			{Name: "id", TypeName: "integer", TypeOID: 23},
			{Name: "prefs", TypeName: "jsonb", Nullable: true, Fingerprint: "0a0b0c0d"},
		},
		PK:      []string{"id"},
		Samples: rows,
	}}}
}

// t404Prior is what internal/emit would have written for cls: each column's
// decision, and the document's copied keys under leaf_keys:.
func t404Prior(cls *pipeline.Classification) *pipeline.Config {
	prior := priorFromDecisions(cls)
	for c, d := range cls.Decisions {
		cc := prior.Columns[c]
		cc.LeafKeys = d.RecordedLeafKeys
		prior.Columns[c] = cc
	}
	return prior
}

func t404Run(extra string, prior *pipeline.Config, strict bool) (*run, *eventCollector) {
	sink := &eventCollector{}
	return &run{
		req:  normalise(Request{StrictSchema: strict, ConfigPath: "./lazyslice.yml"}),
		sink: sink, schema: t404Schema(extra), prior: prior,
	}, sink
}

const t404Grown = `, "guest": "Ysolde Pembrook", "emergency": "Ysolde Pembrook, sister"`

func TestANewDocumentKeyIsReportedAsDriftNamingTheColumnAndTheKey(t *testing.T) {
	t.Parallel()
	first, _ := t404Run("", nil, false)
	if err := first.classifyStage(); err != nil {
		t.Fatalf("first classifyStage: %v", err)
	}
	prefs := ref.ColumnRef{Table: t404Accounts, Column: "prefs"}
	if got := first.cls.Decisions[prefs].RecordedLeafKeys; strings.Join(got, ",") != "layout,theme" {
		t.Fatalf("precondition: run 1 copied keys = %v, want layout and theme", got)
	}

	r, sink := t404Run(t404Grown, t404Prior(first.cls), false)
	if err := r.classifyStage(); err != nil {
		t.Fatalf("second classifyStage: %v", err)
	}
	var columns []string
	for _, e := range sink.events {
		if e.Code != classify.CodeColumnDrift {
			continue
		}
		columns = append(columns, e.Args[event.ArgColumn])
		if e.Column != "prefs" || e.Table != t404Accounts {
			t.Errorf("drift event names %s.%s, want the document column", e.Table, e.Column)
		}
		if !strings.HasPrefix(e.Args[event.ArgVerdict], "masked") {
			t.Errorf("drift verdict = %q, want it to say the key's leaves were masked", e.Args[event.ArgVerdict])
		}
		for _, v := range []string{"Ysolde", "Pembrook", "sister"} {
			for k, a := range e.Args {
				if strings.Contains(a, v) {
					t.Errorf("drift event arg %s = %q carries a leaf value", k, a)
				}
			}
		}
	}
	if got, want := strings.Join(columns, " "), "prefs->'emergency' prefs->'guest'"; got != want {
		t.Errorf("drift lines name %q, want %q", got, want)
	}

	var reused *event.Event
	for i, e := range sink.events {
		if e.Code == CodeClassifyReused {
			reused = &sink.events[i]
		}
		if (e.Code == classify.CodeColumnMasked || e.Code == classify.CodeColumnCopied) && e.Column == "prefs" && e.Settled {
			t.Error("the document column's reason line was folded away as settled, beside a drift line for its keys")
		}
	}
	if reused == nil {
		t.Fatal("no classify.reused line")
	}
	if got := reused.Args[event.ArgDriftCount]; got != "2" {
		t.Errorf("classify.reused drift_count = %s, want 2 (one per key)", got)
	}
	if got := reused.Args[event.ArgChangedCount]; got != "0" {
		t.Errorf("classify.reused changed_count = %s, want 0: no column's own decision moved", got)
	}
	if m := r.cls.Decisions[prefs].LeafMap(); len(m) != 2 {
		t.Errorf("leaf map = %v, want only the two keys the file lists", m)
	}
}

func TestStrictSchemaRefusesANewDocumentKey(t *testing.T) {
	t.Parallel()
	first, _ := t404Run("", nil, false)
	if err := first.classifyStage(); err != nil {
		t.Fatalf("first classifyStage: %v", err)
	}
	prior := t404Prior(first.cls)

	same, _ := t404Run("", prior, true)
	if err := same.classifyStage(); err != nil {
		t.Fatalf("--strict-schema over unchanged documents: %v, want no refusal", err)
	}

	r, _ := t404Run(t404Grown, prior, true)
	err := r.classifyStage()
	var stop *Stop
	if !errors.As(err, &stop) {
		t.Fatalf("--strict-schema with two new keys: %v, want a *Stop", err)
	}
	if stop.Exit != exitDrift || stop.Code != classify.CodeRefusedStrictSchema {
		t.Errorf("stop = exit %d %s, want exit %d %s", stop.Exit, stop.Code, exitDrift, classify.CodeRefusedStrictSchema)
	}
	if stop.Args[event.ArgCount] != "2" {
		t.Errorf("count = %s, want 2", stop.Args[event.ArgCount])
	}
}
