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

// A natural key the operator masks takes its foreign-key children with it
// (T-0364): internal/classify propagates across the edge after the raise a
// mask becomes, so the parent alone masks the child under the same category
// and the run goes on. Before, the child stayed in clear and the run stopped
// naming it (review of T-0319), and a key-family child had no --mask that
// could clear the stop.
func TestMaskOnAKeyMasksEveryChild(t *testing.T) {
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

	for _, mask := range []map[string]string{
		{"t319_ext.code": ""},
		{"t319_ext.code": "", "t319_use.ext_code": ""},
	} {
		r, sink := newRun(mask)
		if err := r.classifyStage(); err != nil {
			t.Fatalf("--mask %v: classifyStage = %v, want the run to go on", mask, err)
		}
		for _, e := range sink.events {
			if e.Code == CodeMaskNotApplied {
				t.Errorf("--mask %v: refused %s: %s", mask, e.Column, e.Args[event.ArgReason])
			}
		}
		for _, col := range []ref.ColumnRef{parent, child} {
			if d := r.cls.Decisions[col]; !d.Masked || d.Category != pipeline.CatFreeText {
				t.Errorf("--mask %v: %s is masked=%v as %s, want masked as free_text", mask, col, d.Masked, d.Category)
			}
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

// The backstop T-0364 keeps: a child propagation cannot mask, because its type
// does not accept the parent's category (person_date takes text but not
// citext), is still refused at exit 2 before any write, named for a --mask of
// its own, rather than copied in clear beside its masked parent.
func TestMaskOnAKeyRefusesAChildPropagationCannotMask(t *testing.T) {
	t.Parallel()
	ext := ref.TableRef{Schema: "public", Name: "t364_ext"}
	use := ref.TableRef{Schema: "public", Name: "t364_use"}
	schema := &pipeline.Schema{
		Tables: []pipeline.Table{
			{Ref: ext, Columns: []pipeline.Column{{Name: "code", TypeName: "text"}}, PK: []string{"code"}},
			{Ref: use, Columns: []pipeline.Column{
				{Name: "id", TypeName: "bigint", TypeOID: 20},
				{Name: "ext_code", TypeName: "citext", Nullable: true},
			}, PK: []string{"id"}},
		},
		FKs: []pipeline.ForeignKey{{
			Name: "t364_use_ext_code_fkey", Child: use, ChildCols: []string{"ext_code"},
			Parent: ext, ParentCols: []string{"code"}, Validated: true,
		}},
	}
	parent := ref.ColumnRef{Table: ext, Column: "code"}
	child := ref.ColumnRef{Table: use, Column: "ext_code"}
	sink := &eventCollector{}
	r := &run{req: normalise(Request{Mask: map[string]string{"t364_ext.code": "person_date"}}), sink: sink, schema: schema}

	var stop *Stop
	if err := r.classifyStage(); !errors.As(err, &stop) || stop.Code != CodeMaskNotApplied || stop.Exit != exitUsage {
		t.Fatalf("classifyStage = %v, want %s at exit %d (parent %+v, child %+v)",
			err, CodeMaskNotApplied, exitUsage, r.cls.Decisions[parent], r.cls.Decisions[child])
	}
	if d := r.cls.Decisions[parent]; !d.Masked || d.Category != pipeline.CatPersonDate {
		t.Fatalf("precondition: %s = %+v, want the parent masked as person_date", parent, d)
	}
	if d := r.cls.Decisions[child]; d.Masked {
		t.Fatalf("precondition: %s = %+v is masked, so the backstop proves nothing", child, d)
	}
	var reasons []string
	for _, e := range sink.events {
		if e.Code == CodeMaskNotApplied {
			reasons = append(reasons, e.Args[event.ArgReason])
		}
	}
	if len(reasons) != 1 || !strings.Contains(reasons[0], child.String()+" references it") {
		t.Errorf("refusals %q, want one naming %s", reasons, child)
	}
}
