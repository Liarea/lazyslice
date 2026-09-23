// SPDX-License-Identifier: Apache-2.0

package verify

import (
	"context"
	"strings"
	"testing"

	"github.com/Liarea/lazyslice/internal/event"
	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
)

// The 2026-09-15 red team's second-net findings. Both attacks put a production
// value in a column this net was structurally unable to look inside, and both
// runs exited 0.
func TestRedTeamSecondNetReadsWhatItUsedToSkip(t *testing.T) {
	cases := []struct {
		name     string
		typeName string
		masked   bool
		vals     []any
		wantFail string
	}{
		// A4a: famBytea was not in netText, so nothing looked at the loaded
		// value either. `assets.blob_doc bytea` held printable UTF-8 carrying
		// an address.
		{
			name:     "a bytea holding printable text with an address in it",
			typeName: "bytea",
			vals: []any{
				[]byte("Grace Hopper <grace.hopper1@realcorp.example>"),
				[]byte("nothing to see"),
				[]byte("still nothing"),
				[]byte("and nothing here either"),
			},
			wantFail: "email",
		},
		// The other half of the same guard: a bytea holding actual bytes is
		// still read by nothing, because a PNG renders as a string with digits
		// and words in it and every shape validator would fire on it.
		{
			name:     "a bytea holding binary is left alone",
			typeName: "bytea",
			vals: []any{
				[]byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a, 0xff, 0xfe, 0x00, 0x01},
				[]byte{0x1f, 0x8b, 0x08, 0x00, 0xff, 0xfe, 0x00, 0x01, 0x02, 0x03},
			},
		},
		// A5b: a JSON document inside a column declared `text`, with leaf
		// values chosen so that no validator fires on the document read as one
		// string. document() covers json, jsonb and hstore only.
		{
			name:     "a JSON document inside a text column",
			typeName: "text",
			vals: []any{
				`{"profile":{"contact":{"email":"ada.lovelace@fixture.test"}}}`,
				`{"profile":{"contact":{"note":"nothing to see"}}}`,
				`{"profile":{"contact":{"note":"still nothing"}}}`,
				`{"profile":{"contact":{"note":"and nothing here either"}}}`,
			},
			wantFail: "email",
		},
		{
			name:     "an email used as a key inside a text column's document",
			typeName: "text",
			vals: []any{
				`{"ada.lovelace@fixture.test":"ok"}`,
				`{"note":"nothing to see"}`,
				`{"note":"still nothing"}`,
				`{"note":"and nothing here either"}`,
			},
			wantFail: "email",
		},
		// The fix round's own regression (the T-REDFIX review's high
		// finding): a document in a column must not dilute the column's own
		// values. These two rows are below minValues, so the address is "any
		// hit on an unproven column" and exit 9 — and it stayed exit 9 when the
		// second row became a four-key object, which used to make the column
		// "proven" with a denominator of five and let 1/5 pass under
		// validatorThreshold.
		{
			name:     "an address in a text column that also holds a document",
			typeName: "text",
			vals: []any{
				"221 Baker Street, London",
				"ok",
			},
			wantFail: "address",
		},
		{
			name:     "the same address with a document beside it",
			typeName: "text",
			vals: []any{
				"221 Baker Street, London",
				`{"a":"x","b":"y","c":"z","d":"w"}`,
			},
			wantFail: "address",
		},
		// A bare JSON scalar is not a document. Reading "12345" as one would
		// double-count every numeric-looking text column in the target.
		{
			name:     "a text column of bare numbers is not a document",
			typeName: "text",
			vals:     []any{`12345`, `67890`, `13579`},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			table := customers()
			col := ref.ColumnRef{Table: table, Column: "payload"}
			dec := pipeline.Decision{Col: col, Category: pipeline.CatNone, Source: pipeline.ByClassifier}
			if c.masked {
				dec.Masked = true
			}
			s := &state{
				schema: &pipeline.Schema{},
				target: oneColumn{vals: c.vals},
				steps:  []pipeline.Step{{Table: table, Mode: pipeline.ChildOK}},
				tables: map[ref.TableRef]*pipeline.Table{
					table: {Ref: table, Columns: []pipeline.Column{{Name: col.Column, TypeName: c.typeName}}},
				},
				cls: &pipeline.Classification{Decisions: map[ref.ColumnRef]pipeline.Decision{col: dec}},
			}
			if err := s.secondNet(context.Background()); err != nil {
				t.Fatalf("secondNet: %v", err)
			}
			if c.wantFail == "" {
				if len(s.failures) != 0 {
					t.Fatalf("the net failed %s as %q, want no failure", col, s.failures[0].Reason)
				}
				return
			}
			if len(s.failures) != 1 {
				t.Fatalf("the net recorded %d failures, want one naming %s; a production value "+
					"is in the target and neither net saw it", len(s.failures), c.wantFail)
			}
			if got := s.failures[0].Reason; got != c.wantFail {
				t.Errorf("the refusal names the category %q, want %q", got, c.wantFail)
			}
		})
	}
}

// The dictionary-backed validators must not see a document's leaves, whether
// the document arrived in a jsonb column or in a text one: internal/classify
// runs no dictionary signal over a document's leaves, and this net may not
// refuse an already-loaded target on evidence the classifier is structurally
// unable to have seen. A text column whose leaves are written names is the
// A5b case with the exclusion still applied.
func TestRedTeamTextDocumentLeavesAreOutsideTheDictionaryRule(t *testing.T) {
	table := customers()
	col := ref.ColumnRef{Table: table, Column: "payload"}
	vals := make([]any, 0, 8)
	for _, name := range []string{
		"Grace Hopper", "Katherine Johnson", "Ada Lovelace",
		"Alan Turing", "Dorothy Vaughan", "Mary Jackson",
		"Annie Easley", "Melba Roy",
	} {
		vals = append(vals, `{"who":"`+name+`"}`)
	}
	s := &state{
		schema: &pipeline.Schema{},
		target: oneColumn{vals: vals},
		steps:  []pipeline.Step{{Table: table, Mode: pipeline.ChildOK}},
		tables: map[ref.TableRef]*pipeline.Table{
			table: {Ref: table, Columns: []pipeline.Column{{Name: col.Column, TypeName: "text"}}},
		},
		cls: &pipeline.Classification{Decisions: map[ref.ColumnRef]pipeline.Decision{
			col: {Col: col, Category: pipeline.CatNone, Source: pipeline.ByClassifier},
		}},
	}
	if err := s.secondNet(context.Background()); err != nil {
		t.Fatalf("secondNet: %v", err)
	}
	for _, f := range s.failures {
		if f.Reason == "person_name" || f.Reason == "free_text" {
			t.Fatalf("the net refused the target as %q over a document's leaves; "+
				"internal/classify cannot see that evidence and there is no green path", f.Reason)
		}
	}
}

// ADR-015's two new refusals and its explained line are value-free by the same
// rule every other line of this stage is (THREAT_MODEL.md T4): a refusal
// carries a fixed phrase from reasons.go, the explained line carries a count,
// and the catalogue template each renders through substitutes identifiers and
// that count and nothing else. The fixture's values are real-looking list
// names, the very thing a leak here would print.
func TestRedTeamExplainedLineAndItsRefusalsAreValueFree(t *testing.T) {
	fixed := map[string]bool{
		reasonStillHolds: true, reasonOverCount: true, reasonSameRow: true,
		reasonProbeCap: true, reasonProbeFailed: true, reasonSourceClosed: true,
	}
	names := givenWords(t, 6)

	scenes := map[string]func() (*scene, *fakeTable){
		"explained": func() (*scene, *fakeTable) {
			ft, plan, cls := people(t, names)
			return newScene(ft, plan, cls), ft
		},
		"same row": func() (*scene, *fakeTable) {
			ft, plan, cls := people(t, names)
			ft.target[2][1] = names[2]
			ft.target[1][1] = names[0]
			return newScene(ft, plan, cls), ft
		},
		"over count": func() (*scene, *fakeTable) {
			ft, plan, cls := people(t, names)
			sc := newScene(ft, plan, cls)
			ft.target[4][1] = names[0]
			return sc, ft
		},
	}
	for name, build := range scenes {
		t.Run(name, func(t *testing.T) {
			sc, ft := build()
			s := sc.run(t)
			assertValueFree(t, s, ft)
			for _, f := range s.failures {
				if !fixed[f.Reason] {
					t.Errorf("a refusal carries a reason outside reasons.go's fixed set: %q", f.Reason)
				}
			}
			for _, c := range s.checks {
				if c.Code == CodeResidualExplained && (c.Column == "" || c.Count == 0) {
					t.Errorf("the explained line %+v names no column or counts nothing", c)
				}
			}
		})
	}

	// The catalogue rows: the explained line and the residual refusal render
	// only identifiers, a count and a fixed reason.
	cat := string(event.Catalogue())
	cat = strings.ReplaceAll(cat, "\r\n", "\n") // a Windows checkout may carry CRLF (T-0302 follow-up)
	for code, args := range map[event.Code]string{
		CodeResidualExplained: "args: [table, column, count]",
		CodeRefusedResidual:   "args: [table, column, reason]",
	} {
		i := strings.Index(cat, "- code: "+string(code)+"\n")
		if i < 0 {
			t.Fatalf("the catalogue has no row for %s", code)
		}
		row := cat[i:]
		if j := strings.Index(row[1:], "\n- code: "); j >= 0 {
			row = row[:j+1]
		}
		if !strings.Contains(row, args) {
			t.Errorf("%s renders %q, want exactly %s", code, row, args)
		}
	}
}
