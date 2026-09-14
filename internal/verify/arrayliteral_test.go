// SPDX-License-Identifier: Apache-2.0

package verify

import (
	"context"
	"strings"
	"testing"

	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
	"github.com/Liarea/lazyslice/mask"
)

// The array-literal half of this stage (arrayliteral.go, tracker T-0129): the
// grammar first, then what the residual scan and the second net do with a
// column that arrives in it.
//
// The fixture is a citext[] of email addresses, because that is the column
// class this exists for: the pool registers no user types, so pgx hands a
// citext[] back as the single string "{a@b.test,c@d.test}" and not as a []any
// (testdata/regressions/009-citext-array-of-addresses-masked-as-one-string.sql,
// tracker T-0118).

// reports is the fixture's table, spelt as regression 009 spells it.
func reports() ref.TableRef { return ref.TableRef{Schema: "public", Name: "reg9_report"} }

// recipients is the fixture's column: citext[] NOT NULL, masked as email.
func recipients() ref.ColumnRef { return ref.ColumnRef{Table: reports(), Column: "recipients"} }

// ---------- the grammar ----------

// The grammar is internal/transform's, copied because a stage package may not
// import another one (internal/CLAUDE.md), and a copy that drifts is a residual
// scan testing bytes the masker never recorded. These are
// TestArrayLiteralRoundTripsTheGrammar's own cases from
// internal/transform/array_test.go, read for their elements: what transform
// masked one by one is what this splits one by one, in the same order and with
// the same count.
func TestArrayLiteralElementsAreSpeltAsTransformSpellsThem(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		in   string
		want []string
	}{
		{name: "empty", in: "{}"},
		{name: "two plain elements", in: "{a@b.test,c@d.test}", want: []string{"a@b.test", "c@d.test"}},
		{name: "one element", in: "{ada}", want: []string{"ada"}},
		{
			name: "a NULL element is not an element to test",
			in:   "{a@b.test,NULL,c@d.test}", want: []string{"a@b.test", "c@d.test"},
		},
		{name: "null in any case", in: "{null}"},
		{name: "a quoted NULL is the four-letter string", in: `{"NULL"}`, want: []string{"NULL"}},
		{
			name: "a quoted element holding the delimiter",
			in:   `{"Ada Lovelace, Esq. <ada@b.test>"}`,
			want: []string{"Ada Lovelace, Esq. <ada@b.test>"},
		},
		{name: "an escaped quote and an escaped backslash", in: `{"a\"b\\c"}`, want: []string{`a"b\c`}},
		{name: "the empty string is quoted", in: `{""}`, want: []string{""}},
		{name: "braces inside an element are quoted", in: `{"{not,an,array}"}`, want: []string{"{not,an,array}"}},
		{
			name: "whitespace around an unquoted element is not part of it",
			in:   "{ a@b.test , c@d.test }", want: []string{"a@b.test", "c@d.test"},
		},
		{name: "two dimensions flatten", in: "{{a,b},{c,d}}", want: []string{"a", "b", "c", "d"}},
		{name: "a dimension prefix is dropped", in: "[0:1]={a,b}", want: []string{"a", "b"}},
		{name: "two dimension prefixes", in: "[0:1][1:2]={{a,b},{c,d}}", want: []string{"a", "b", "c", "d"}},
		{name: "a NULL element inside a nested array", in: "{{a,NULL},{NULL,d}}", want: []string{"a", "d"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := arrayLiteralElements(tc.in)
			if err != nil {
				t.Fatalf("arrayLiteralElements(%q): %v", tc.in, err)
			}
			if strings.Join(got, "\x00") != strings.Join(tc.want, "\x00") {
				t.Errorf("arrayLiteralElements(%q) = %q, want %q; internal/transform masked the second list",
					tc.in, got, tc.want)
			}
		})
	}
}

// The fail-closed half, and the same case list internal/transform refuses. A
// value this cannot split is not a value to test as one string: the filter
// holds one entry per element, so the whole literal matches nothing and the
// scan would pass green over a column nobody has looked inside (T-0129).
func TestArrayLiteralRefusesWhatIsNotOne(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, in string }{
		{"not a literal at all", "a@b.test"},
		{"no braces", "a,b"},
		{"unclosed brace", "{a,b"},
		{"unopened brace", "a,b}"},
		{"an unclosed quote", `{"a@b.test}`},
		{"a trailing backslash inside quotes", `{"a\`},
		{"a bare backslash", `{a\b}`},
		{"an empty unquoted element", "{a,}"},
		{"nothing between two delimiters", "{,}"},
		{"trailing text after the array", "{a,b} junk"},
		{"a dimension prefix with no braces", "[0:1]=a"},
		{"an unterminated dimension prefix", "[0:1{a}"},
		{"empty", ""},
		{"a quoted element with no separator after it", `{"a""b"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if _, err := arrayLiteralElements(tc.in); err == nil {
				t.Errorf("arrayLiteralElements(%q) read it as an array; it is not the server's output form", tc.in)
			}
		})
	}
}

// ---------- the residual scan ----------

// askingResidual records every (path, canonical) the scan tests and answers yes
// for the canonical forms it was given. A real filter is a Bloom filter and
// cannot be asked what it holds; this one can, which is the only way to pin
// *which bytes* the scan looked for.
type askingResidual struct {
	asked []string
	yes   map[string]bool
}

func (r *askingResidual) Add(ref.ColumnRef, string, []byte) {}
func (r *askingResidual) MayContain(_ ref.ColumnRef, path string, canonical []byte) bool {
	r.asked = append(r.asked, path+"|"+string(canonical))
	return r.yes[string(canonical)]
}
func (r *askingResidual) Cells() int64 { return 0 }
func (r *askingResidual) Bytes() int64 { return 0 }

// entry is one residual-filter entry as internal/transform spells it for an
// array column: the column's empty path, and the element's canonical bytes.
func entry(t *testing.T, cat pipeline.Category, v any) string {
	t.Helper()
	canon, ok, err := canonicalOf(mask.Category(cat), v)
	if err != nil || !ok {
		t.Fatalf("canonicalOf(%v): ok=%v err=%v", cat, ok, err)
	}
	return "|" + string(canon)
}

// citextArrayState is the fixture: one loaded step over public.reg9_report,
// whose recipients column is a citext[] masked as email, and a target that
// hands every value back the way pgx hands an array of an unregistered element
// type back — as one string.
func citextArrayState(target targetReader, res pipeline.Residual) *state {
	col := recipients()
	return &state{
		schema: &pipeline.Schema{},
		target: target,
		res:    res,
		steps:  []pipeline.Step{{Table: reports(), Mode: pipeline.ChildOK}},
		tables: map[ref.TableRef]*pipeline.Table{
			reports(): {Ref: reports(), Columns: []pipeline.Column{
				{Name: "id", TypeName: "integer"},
				{Name: col.Column, TypeName: "citext[]"},
			}},
		},
		cls: &pipeline.Classification{Decisions: map[ref.ColumnRef]pipeline.Decision{
			col: {Col: col, Category: pipeline.CatEmail, Masked: true, Masker: "email",
				Source: pipeline.ByClassifier},
		}},
	}
}

// A masked citext[] arrives as one text literal, and internal/transform masked
// it *element-wise*: one filter entry per element, under the column's empty
// path, keyed by that element's canonical bytes (internal/transform/array.go,
// tracker T-0118). So the scan has to ask the filter the same question per
// element.
//
// Before T-0129 arrayHits fell back to scalarHits on the whole value when it was
// not a []any, so the one thing tested was the canonical form of the string
// "{a@b.test,c@d.test}" — bytes transform never added, no hit, a green tick, and
// every address inside the braces shipped in cleartext. That is the masker
// failing open, and ARCHITECTURE.md §6 item 1 is the only control
// THREAT_MODEL.md T12 has against it.
//
// This asserts the bytes, not just the verdict: a scan that asks the wrong
// question passes for the wrong reason.
func TestAMaskedArrayLiteralIsScannedElementWise(t *testing.T) {
	res := &askingResidual{}
	target := oneColumn{vals: []any{
		"{ada.lovelace1@fixture.test,grace.hopper1@reports.fixture.test}",
		// The two other shapes regression 009 loads: one element, and the
		// empty array the column defaults to.
		"{ada.lovelace3@fixture.test}",
		"{}",
		nil,
	}}
	s := citextArrayState(target, res)
	if err := s.residualScan(context.Background()); err != nil {
		t.Fatalf("residualScan: %v", err)
	}
	if len(s.failures) != 0 {
		t.Fatalf("the scan failed on a target with nothing in the filter: %s", s.failures[0].Reason)
	}

	want := []string{
		entry(t, pipeline.CatEmail, "ada.lovelace1@fixture.test"),
		entry(t, pipeline.CatEmail, "grace.hopper1@reports.fixture.test"),
		entry(t, pipeline.CatEmail, "ada.lovelace3@fixture.test"),
	}
	if strings.Join(res.asked, "\n") != strings.Join(want, "\n") {
		t.Errorf("the scan tested %d entries, want the %d elements transform recorded\n got %q\nwant %q",
			len(res.asked), len(want), res.asked, want)
	}
	// The whole literal is what the pre-T-0129 scan tested, and the filter has
	// no entry for it: asking that question is the green tick this test exists
	// to stop.
	whole := entry(t, pipeline.CatEmail, "{ada.lovelace1@fixture.test,grace.hopper1@reports.fixture.test}")
	for _, a := range res.asked {
		if a == whole {
			t.Errorf("the scan canonicalised the whole literal; the filter holds one entry per element")
		}
	}
}

// The other half: an element the filter says may have leaked is found, and the
// run does not end with a green residual tick over it.
//
// The refusal here is *unconfirmable* rather than *residual* only because this
// state has no source to confirm against, which is the shortest way to prove
// the hit was produced at all; what matters is that the element inside the
// braces reached section 6 item 3's confirmation, which before T-0129 no
// element of such a column ever did.
func TestAnElementOfAMaskedArrayLiteralIsFoundByTheScan(t *testing.T) {
	leaked := "grace.hopper1@reports.fixture.test"
	canon, ok, err := canonicalOf(mask.CatEmail, leaked)
	if err != nil || !ok {
		t.Fatalf("canonicalOf: ok=%v err=%v", ok, err)
	}
	res := &askingResidual{yes: map[string]bool{string(canon): true}}
	target := oneColumn{vals: []any{"{ada.lovelace1@fixture.test," + leaked + "}"}}
	s := citextArrayState(target, res)
	if err := s.residualScan(context.Background()); err != nil {
		t.Fatalf("residualScan: %v", err)
	}
	if len(s.failures) != 1 {
		t.Fatalf("the scan recorded %d failures, want one: an element the filter holds is in the target",
			len(s.failures))
	}
	got := s.failures[0]
	if got.Table != reports() || got.Column != "recipients" {
		t.Errorf("the refusal names %s.%s, want %s", got.Table, got.Column, recipients())
	}
	if got.Exit != exitResidual {
		t.Errorf("the refusal exits %d, want %d", got.Exit, exitResidual)
	}
	if got.Reason != reasonSourceClosed {
		t.Errorf("the refusal reads %q; the hit reached confirmation and this state has no source", got.Reason)
	}
	assertNoGreenTick(t, s)
}

// A masked array column whose value this stage cannot split into elements is a
// column it cannot scan at all, and that is exit 9 naming the column — never a
// fall back to the scalar path, which would test the whole value against
// per-element entries and pass for the wrong reason (T-0129, the decision
// recorded in internal/verify/CLAUDE.md).
func TestAMaskedArrayValueThatIsNotALiteralIsRefused(t *testing.T) {
	res := &askingResidual{}
	s := citextArrayState(oneColumn{vals: []any{"ada.lovelace1@fixture.test"}}, res)
	if err := s.residualScan(context.Background()); err != nil {
		t.Fatalf("residualScan: %v", err)
	}
	if len(s.failures) != 1 {
		t.Fatalf("the scan recorded %d failures, want one: a masked array column it cannot read", len(s.failures))
	}
	got := s.failures[0]
	if got.Code != CodeRefusedUnconfirmable || got.Check != checkUnconfirmable {
		t.Errorf("the refusal is %s on check %q, want %s on %q",
			got.Code, got.Check, CodeRefusedUnconfirmable, checkUnconfirmable)
	}
	if got.Exit != exitResidual {
		t.Errorf("the refusal exits %d, want %d", got.Exit, exitResidual)
	}
	if got.Table != reports() || got.Column != "recipients" {
		t.Errorf("the refusal names %s.%s, want %s", got.Table, got.Column, recipients())
	}
	if got.Reason != reasonArrayLiteral {
		t.Errorf("the refusal reads %q, want %q", got.Reason, reasonArrayLiteral)
	}
	if len(res.asked) != 0 {
		t.Errorf("the scan tested %d entries on a value it could not split; the scalar fall back is the green tick", len(res.asked))
	}
	assertNoGreenTick(t, s)
}

// assertNoGreenTick is the rule residual.go shares with fk.go, counts.go and
// secondnet.go: a pass is never appended beside a failure, or the report
// carries a green tick for the check that produced the exit code.
func assertNoGreenTick(t *testing.T, s *state) {
	t.Helper()
	for _, c := range s.checks {
		if c.Passed {
			t.Errorf("the report carries a passing %q check beside the refusal", c.Name)
		}
	}
}

// ---------- the second net ----------

// The second net reads an array column on its elements, because ARCHITECTURE.md
// §4 classifies an array on its element type. An array that arrives as a
// literal has to be split the same way: unsplit, a column of addresses reaches
// the validators as one string per row that no validator recognises, and the
// net THREAT_MODEL.md T1 names as one of two controls on classifier recall sees
// nothing in the column class §4 classifies element-wise.
//
// The column here is *unmasked* — that is the only kind this net reads — so the
// literal it cannot parse is read whole rather than refused, which is the
// asymmetry with the residual scan and is recorded in internal/verify/CLAUDE.md.
func TestTheSecondNetReadsAnArrayLiteralElementWise(t *testing.T) {
	table := reports()
	col := ref.ColumnRef{Table: table, Column: "recipients"}
	cases := []struct {
		name     string
		vals     []any
		wantFail bool
	}{
		{
			name: "three literals of addresses",
			vals: []any{
				"{ada.lovelace1@fixture.test,grace.hopper1@reports.fixture.test}",
				"{alan.turing2@fixture.test}",
				"{ada.lovelace3@fixture.test}",
			},
			wantFail: true,
		},
		{
			name: "three literals of nothing in particular",
			vals: []any{"{wk,mo}", "{wk}", "{mo}"},
		},
		{
			name: "a value that is not a literal is read whole and recognised by nobody",
			vals: []any{"wk,mo", "wk", "mo"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := &state{
				schema: &pipeline.Schema{},
				target: oneColumn{vals: c.vals},
				steps:  []pipeline.Step{{Table: table, Mode: pipeline.ChildOK}},
				tables: map[ref.TableRef]*pipeline.Table{
					table: {Ref: table, Columns: []pipeline.Column{{Name: col.Column, TypeName: "citext[]"}}},
				},
				cls: &pipeline.Classification{Decisions: map[ref.ColumnRef]pipeline.Decision{
					col: {Col: col, Category: pipeline.CatNone, Source: pipeline.ByClassifier},
				}},
			}
			if err := s.secondNet(context.Background()); err != nil {
				t.Fatalf("secondNet: %v", err)
			}
			switch {
			case c.wantFail && len(s.failures) != 1:
				t.Fatalf("the net recorded %d failures, want one: every element is an address "+
					"and nothing masked the column", len(s.failures))
			case !c.wantFail && len(s.failures) != 0:
				t.Fatalf("the net failed %s on %v: %s", col, c.vals, s.failures[0].Reason)
			}
			if !c.wantFail {
				return
			}
			if got := s.failures[0]; got.Reason != "email" || got.Column != col.Column {
				t.Errorf("the refusal names %s as %q, want %s as %q", got.Column, got.Reason, col.Column, "email")
			}
		})
	}
}
