// SPDX-License-Identifier: Apache-2.0

package core

import (
	"errors"
	"strings"
	"testing"

	"github.com/Liarea/lazyslice/internal/event"
	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
	"github.com/Liarea/lazyslice/internal/verify"
)

// T-0319: --mask TABLE.COL[=CATEGORY] is how an operator answers the second
// net without declaring anything safe, and every second-net column is named in
// one run. Both are held here without a database: classifyStage over a
// hand-built schema, and reportVerifyRefusals over hand-built refusals.

var maskFeeds = ref.TableRef{Schema: "public", Name: "t319_feeds"}

func maskSchema() *pipeline.Schema {
	return &pipeline.Schema{Tables: []pipeline.Table{{
		Ref: maskFeeds,
		Columns: []pipeline.Column{
			{Name: "id", TypeName: "bigint", TypeOID: 20},
			{Name: "status_code", TypeName: "text", Nullable: true},
			{Name: "attempts", TypeName: "integer", TypeOID: 23, Nullable: true},
		},
		PK: []string{"id"},
	}}}
}

func maskRun(req Request, prior *pipeline.Config) (*run, *eventCollector) {
	sink := &eventCollector{}
	return &run{req: normalise(req), sink: sink, schema: maskSchema(), prior: prior}, sink
}

func TestMaskMasksAColumnTheClassifierCopied(t *testing.T) {
	t.Parallel()
	col := ref.ColumnRef{Table: maskFeeds, Column: "status_code"}

	plain, _ := maskRun(Request{}, nil)
	if err := plain.classifyStage(); err != nil {
		t.Fatalf("classifyStage with no flag: %v", err)
	}
	if plain.cls.Decisions[col].Masked {
		t.Fatalf("precondition: %s is masked with no flag, so the flag proves nothing", col)
	}

	for _, c := range []struct {
		flag string
		want pipeline.Category
	}{
		{"", pipeline.CatFreeText},
		{"online_id", pipeline.CatOnlineID},
	} {
		r, _ := maskRun(Request{Mask: map[string]string{"t319_feeds.status_code": c.flag}}, nil)
		if err := r.classifyStage(); err != nil {
			t.Fatalf("--mask t319_feeds.status_code=%s: %v", c.flag, err)
		}
		d := r.cls.Decisions[col]
		if !d.Masked || d.Category != c.want {
			t.Errorf("--mask =%q left %s as masked=%v category=%s, want masked as %s",
				c.flag, col, d.Masked, d.Category, c.want)
		}
		if got := r.flagMasks()[col]; got != c.want {
			t.Errorf("the flag's record for the yml is %q, want %q", got, c.want)
		}
	}
}

func TestAMaskThatCannotApplyIsExit2NamingEveryColumn(t *testing.T) {
	t.Parallel()
	// free_text on an integer column, and a surrogate key: neither can be
	// masked, and both must be named in the one run.
	r, sink := maskRun(Request{Mask: map[string]string{
		"t319_feeds.attempts": "",
		"t319_feeds.id":       "",
	}}, nil)
	err := r.classifyStage()

	var stop *Stop
	if !errors.As(err, &stop) || stop.Code != CodeMaskNotApplied || stop.Exit != exitUsage {
		t.Fatalf("classifyStage = %v, want %s at exit %d", err, CodeMaskNotApplied, exitUsage)
	}
	if !stop.sent {
		t.Error("the Stop is not marked sent, so run.report would print the first column twice")
	}
	var named []string
	for _, e := range sink.events {
		if e.Code == CodeMaskNotApplied {
			named = append(named, e.Column)
			if e.Args[event.ArgFlag] != "--mask" {
				t.Errorf("the refusal for %s names %q, want --mask", e.Column, e.Args[event.ArgFlag])
			}
		}
	}
	if len(named) != 2 || named[0] != "attempts" || named[1] != "id" {
		t.Errorf("refusals named %v, want [attempts id]", named)
	}
}

func TestAMaskInTheYmlIsHonouredAndBeatsTheFilesOptOut(t *testing.T) {
	t.Parallel()
	col := ref.ColumnRef{Table: maskFeeds, Column: "status_code"}
	prior := &pipeline.Config{Columns: map[ref.ColumnRef]pipeline.ColumnConfig{
		col: {
			Category: pipeline.CatNone, Confidence: pipeline.ConfNone,
			Mask:   &pipeline.Mask{Category: pipeline.CatFreeText, By: "flag"},
			Unmask: &pipeline.Unmask{Reason: "hand-written", By: "someone", TypeFP: "x"},
		},
	}}
	r, _ := maskRun(Request{}, prior)
	if err := r.classifyStage(); err != nil {
		t.Fatalf("classifyStage: %v", err)
	}
	if d := r.cls.Decisions[col]; !d.Masked {
		t.Errorf("%s carries a mask: block in the yml and was copied (%+v)", col, d)
	}
	if len(r.flagMasks()) != 0 {
		t.Errorf("a mask read from the yml was recorded as the flag's: %v", r.flagMasks())
	}
}

func TestMaskAndUnmaskOnOneColumnIsExit2(t *testing.T) {
	t.Parallel()
	r, _ := maskRun(Request{
		Mask:   map[string]string{"t319_feeds.status_code": ""},
		Unmask: map[string]string{"public.t319_feeds.status_code": "not personal"},
	}, nil)
	var stop *Stop
	if err := r.classifyStage(); !errors.As(err, &stop) || stop.Exit != exitUsage {
		t.Fatalf("classifyStage = %v, want exit %d", err, exitUsage)
	}
}

func TestReportVerifyRefusalsSendsEveryColumn(t *testing.T) {
	t.Parallel()
	refusals := verify.Refusals{
		{Code: verify.CodeRefusedSecondNet, Exit: 9, Check: "second_net", Table: maskFeeds, Column: "status_code", Count: 3, Reason: "online_id"},
		{Code: verify.CodeRefusedSecondNet, Exit: 9, Check: "second_net", Table: maskFeeds, Column: "attempts", Count: 1, Reason: "phone"},
		{Code: verify.CodeRefusedFK, Exit: 8, Check: "fk", Table: maskFeeds},
	}
	sink := &eventCollector{}
	r := &run{sink: sink}
	err := r.reportVerifyRefusals(refusals)

	if len(sink.events) != len(refusals) {
		t.Fatalf("sent %d events, want one per refusal (%d)", len(sink.events), len(refusals))
	}
	for i, e := range sink.events {
		if e.Kind != event.Error || e.Code != refusals[i].Code || e.Column != refusals[i].Column {
			t.Errorf("event %d is %v %s on %q, want an Error %s on %q",
				i, e.Kind, e.Code, e.Column, refusals[i].Code, refusals[i].Column)
		}
	}
	var stop *Stop
	if !errors.As(err, &stop) || stop.Exit != 9 || stop.Column != "status_code" || !stop.sent {
		t.Fatalf("reportVerifyRefusals = %v, want the first refusal's Stop at exit 9, marked sent", err)
	}
	before := len(sink.events)
	r.report(err)
	if len(sink.events) != before {
		t.Error("run.report printed the first refusal a second time")
	}
}

// A natural key the operator masks takes its foreign-key children with it, or
// the run stops naming each one: internal/classify propagates across an edge
// before a mask is applied, so without this the child ships the parent's
// values in clear (review of T-0319).
func TestMaskOnAKeyNamesEveryUnmaskedChild(t *testing.T) {
	t.Parallel()
	ext := ref.TableRef{Schema: "public", Name: "t319_ext"}
	use := ref.TableRef{Schema: "public", Name: "t319_use"}
	schema := &pipeline.Schema{
		Tables: []pipeline.Table{
			{Ref: ext, Columns: []pipeline.Column{{Name: "code", TypeName: "text"}}, PK: []string{"code"}},
			{Ref: use, Columns: []pipeline.Column{
				{Name: "id", TypeName: "bigint", TypeOID: 20},
				{Name: "ext_code", TypeName: "text", Nullable: true},
			}, PK: []string{"id"}},
		},
		FKs: []pipeline.ForeignKey{{
			Name: "t319_use_ext_code_fkey", Child: use, ChildCols: []string{"ext_code"},
			Parent: ext, ParentCols: []string{"code"}, Validated: true,
		}},
	}
	parent := ref.ColumnRef{Table: ext, Column: "code"}
	child := ref.ColumnRef{Table: use, Column: "ext_code"}
	newRun := func(mask map[string]string) (*run, *eventCollector) {
		sink := &eventCollector{}
		return &run{req: normalise(Request{Mask: mask}), sink: sink, schema: schema}, sink
	}

	r, sink := newRun(map[string]string{"t319_ext.code": ""})
	var stop *Stop
	if err := r.classifyStage(); !errors.As(err, &stop) || stop.Code != CodeMaskNotApplied || stop.Exit != exitUsage {
		t.Fatalf("--mask on the parent alone: classifyStage = %v, want %s at exit %d (child %+v)",
			err, CodeMaskNotApplied, exitUsage, r.cls.Decisions[child])
	}
	var reasons []string
	for _, e := range sink.events {
		if e.Code == CodeMaskNotApplied {
			reasons = append(reasons, e.Args[event.ArgReason])
		}
	}
	if len(reasons) != 1 || !strings.Contains(reasons[0], "--mask "+child.String()) {
		t.Errorf("refusals %q, want one naming --mask %s", reasons, child)
	}

	both, _ := newRun(map[string]string{"t319_ext.code": "", "t319_use.ext_code": ""})
	if err := both.classifyStage(); err != nil {
		t.Fatalf("--mask on both ends: %v", err)
	}
	for _, col := range []ref.ColumnRef{parent, child} {
		if d := both.cls.Decisions[col]; !d.Masked || d.Category != pipeline.CatFreeText {
			t.Errorf("%s is masked=%v as %s, want masked as free_text", col, d.Masked, d.Category)
		}
	}
}

// --mask never swaps the masker of a column the classifier already masks, and
// never records a category the column was not masked as (review of T-0319).
func TestMaskCannotChangeAnExistingMasker(t *testing.T) {
	t.Parallel()
	people := ref.TableRef{Schema: "public", Name: "t319_people"}
	col := ref.ColumnRef{Table: people, Column: "email"}
	schema := &pipeline.Schema{Tables: []pipeline.Table{{
		Ref: people,
		Columns: []pipeline.Column{
			{Name: "id", TypeName: "bigint", TypeOID: 20},
			{Name: "email", TypeName: "text", Nullable: true},
		},
		PK: []string{"id"},
	}}}
	newRun := func(mask map[string]string) *run {
		return &run{req: normalise(Request{Mask: mask}), sink: &eventCollector{}, schema: schema}
	}

	plain := newRun(nil)
	if err := plain.classifyStage(); err != nil {
		t.Fatalf("classifyStage with no flag: %v", err)
	}
	was := plain.cls.Decisions[col]
	if !was.Masked || was.Category == pipeline.CatOnlineID {
		t.Fatalf("precondition: %s is masked=%v as %s with no flag", col, was.Masked, was.Category)
	}

	var stop *Stop
	if err := newRun(map[string]string{"t319_people.email": "online_id"}).classifyStage(); !errors.As(err, &stop) || stop.Exit != exitUsage {
		t.Fatalf("--mask =online_id over a column masked as %s: classifyStage = %v, want exit %d",
			was.Category, err, exitUsage)
	}
	same := newRun(map[string]string{"t319_people.email": string(was.Category)})
	if err := same.classifyStage(); err != nil {
		t.Errorf("--mask naming the category the column is already masked as: %v", err)
	}
}
