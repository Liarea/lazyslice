// SPDX-License-Identifier: Apache-2.0

package transform

import (
	"errors"
	"net/mail"
	"strings"
	"testing"

	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
	"github.com/Liarea/lazyslice/mask"
)

// The array literal half of this package (array.go, tracker T-0118): the
// grammar first, then the masking that rests on it.
//
// The grammar cases are the ones a citext[] on a real schema actually produces —
// an address is quoted the moment it holds a comma, a NULL element is the
// unquoted word, an empty array is "{}" — plus the ones that are only ever seen
// when somebody built the column by hand: a quoted "NULL", a backslash, a
// multidimensional array, and the dimension prefix an array whose subscripts do
// not start at one carries.

// ---------- the grammar ----------

// TestArrayLiteralRoundTripsTheGrammar parses the server's output form and
// writes it back. Everything here is unmasked: the point is that the reader and
// the writer agree, because what the writer emits is what the loader hands the
// server.
func TestArrayLiteralRoundTripsTheGrammar(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		in      string
		want    string   // the re-emitted literal, "" for "the same as in"
		strings []string // the non-NULL elements, in order
		nulls   int
	}{
		{name: "empty", in: "{}"},
		{name: "two plain elements", in: "{a@b.test,c@d.test}", strings: []string{"a@b.test", "c@d.test"}},
		{name: "one element", in: "{ada}", strings: []string{"ada"}},
		{
			name: "a NULL element", in: "{a@b.test,NULL,c@d.test}",
			strings: []string{"a@b.test", "c@d.test"}, nulls: 1,
		},
		{name: "null in any case", in: "{null}", want: "{NULL}", nulls: 1},
		{
			name: "a quoted NULL is the four-letter string",
			in:   `{"NULL"}`, strings: []string{"NULL"},
		},
		{
			name: "a quoted element holding the delimiter",
			in:   `{"Ada Lovelace, Esq. <ada@b.test>"}`,
			// The space alone would already have forced the quotes.
			strings: []string{"Ada Lovelace, Esq. <ada@b.test>"},
		},
		{
			name: "an escaped quote and an escaped backslash",
			in:   `{"a\"b\\c"}`, strings: []string{`a"b\c`},
		},
		{name: "the empty string is quoted", in: `{""}`, strings: []string{""}},
		{
			name: "braces inside an element are quoted",
			in:   `{"{not,an,array}"}`, strings: []string{"{not,an,array}"},
		},
		{
			name: "whitespace around an unquoted element is not part of it",
			in:   "{ a@b.test , c@d.test }", want: "{a@b.test,c@d.test}",
			strings: []string{"a@b.test", "c@d.test"},
		},
		{
			name: "two dimensions keep their shape",
			in:   "{{a,b},{c,d}}", strings: []string{"a", "b", "c", "d"},
		},
		{
			name: "a dimension prefix is written back",
			in:   "[0:1]={a,b}", strings: []string{"a", "b"},
		},
		{
			name: "two dimension prefixes",
			in:   "[0:1][1:2]={{a,b},{c,d}}", strings: []string{"a", "b", "c", "d"},
		},
		{
			name: "a NULL element inside a nested array",
			in:   "{{a,NULL},{NULL,d}}", strings: []string{"a", "d"}, nulls: 2,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			prefix, root, err := parseArrayLiteral(tc.in)
			if err != nil {
				t.Fatalf("parseArrayLiteral(%q): %v", tc.in, err)
			}
			gotStrings, gotNulls := flattenArrayNode(root)
			if strings.Join(gotStrings, "\x00") != strings.Join(tc.strings, "\x00") {
				t.Errorf("parseArrayLiteral(%q) read the elements %q, want %q", tc.in, gotStrings, tc.strings)
			}
			if gotNulls != tc.nulls {
				t.Errorf("parseArrayLiteral(%q) read %d NULL elements, want %d", tc.in, gotNulls, tc.nulls)
			}
			want := tc.want
			if want == "" {
				want = tc.in
			}
			if got := renderArrayLiteral(prefix, root); got != want {
				t.Errorf("renderArrayLiteral over %q wrote %q, want %q", tc.in, got, want)
			}
		})
	}
}

// TestArrayLiteralRefusesWhatIsNotOne is the fail-closed half. A value in an
// array column this package cannot read is a refusal, not a value masked as one
// string and not a value copied: the first is what handed CopyFrom a scalar for
// an _citext column, and the second is THREAT_MODEL.md T12.
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
			if _, _, err := parseArrayLiteral(tc.in); err == nil {
				t.Errorf("parseArrayLiteral(%q) read it as an array; it is not the server's output form", tc.in)
			}
		})
	}
}

// TestArrayElementQuotingMatchesArrayOut states the writer's rule on its own:
// an element is quoted exactly when leaving it bare would change what array_in
// reads back.
func TestArrayElementQuotingMatchesArrayOut(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ in, want string }{
		{"a@b.test", "a@b.test"},
		{"", `""`},
		{"NULL", `"NULL"`},
		{"null", `"null"`},
		{"a b", `"a b"`},
		{"a,b", `"a,b"`},
		{"a{b", `"a{b"`},
		{"a}b", `"a}b"`},
		{`a"b`, `"a\"b"`},
		{`a\b`, `"a\\b"`},
		{"a\tb", "\"a\tb\""},
	} {
		if got := quoteArrayElement(tc.in); got != tc.want {
			t.Errorf("quoteArrayElement(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// flattenArrayNode reads a parsed literal back out in order: the non-NULL
// elements and how many NULL ones there were.
func flattenArrayNode(n arrayNode) (texts []string, nulls int) {
	if n.nested {
		for _, e := range n.elems {
			t, k := flattenArrayNode(e)
			texts, nulls = append(texts, t...), nulls+k
		}
		return texts, nulls
	}
	if n.null {
		return nil, 1
	}
	return []string{n.text}, 0
}

// ---------- masking through the literal ----------

// literalFixture is a table with the column T-0118 is about: a citext[] that
// arrives from the source as one string, because the source pool registers no
// user types and pgx therefore has no codec for _citext.
func literalFixture() (*pipeline.Schema, *pipeline.Classification, ref.ColumnRef) {
	reports := pipeline.Table{
		Ref: tbl("reports"),
		Columns: []pipeline.Column{
			{Name: "report_id", TypeName: "integer", TypeOID: 23},
			{Name: "recipients", TypeName: "citext[]", Nullable: true},
		},
		PK: []string{"report_id"},
	}
	c := ref.ColumnRef{Table: reports.Ref, Column: "recipients"}
	cls := &pipeline.Classification{Decisions: map[ref.ColumnRef]pipeline.Decision{
		c: {Col: c, Category: pipeline.CatEmail, Confidence: pipeline.ConfCertain, Masker: mask.MaskerEmail, Masked: true},
	}}
	return &pipeline.Schema{Tables: []pipeline.Table{reports}}, cls, c
}

func literalBatch(values ...any) pipeline.RowBatch {
	b := pipeline.RowBatch{Table: tbl("reports"), Cols: []string{"report_id", "recipients"}, Last: true}
	for i, v := range values {
		b.Rows = append(b.Rows, []any{int32(i + 1), v})
	}
	return b
}

// maskLiterals masks one batch of citext[] literals and returns the column's
// values, with the filter entries the run made.
func maskLiterals(t *testing.T, k mask.Key, values ...any) ([]any, *recorder) {
	t.Helper()
	schema, cls, _ := literalFixture()
	res := &recorder{inner: NewResidual(1000)}
	out, err := New(schema).Transform(literalBatch(values...), cls, &k, res)
	if err != nil {
		t.Fatalf("Transform: %v", err)
	}
	got := make([]any, len(out.Rows))
	for i, row := range out.Rows {
		got[i] = row[1]
	}
	return got, res
}

// TestAnArrayThatArrivesAsALiteralIsMaskedElementWise is the write half of
// T-0103: the addresses inside the literal are gone, the shape of the literal
// is not, and every element is an address in its own right rather than one
// masked string standing in for the whole array.
func TestAnArrayThatArrivesAsALiteralIsMaskedElementWise(t *testing.T) {
	t.Parallel()
	source := "{ada@fixture.test,grace@fixture.test}"
	got, _ := maskLiterals(t, key(t, 0x21), source)

	out, ok := got[0].(string)
	if !ok {
		t.Fatalf("a citext[] literal masked to %T, want the text form the loader writes back", got[0])
	}
	for _, addr := range []string{"ada@fixture.test", "grace@fixture.test"} {
		if strings.Contains(out, addr) {
			t.Errorf("the masked literal still holds %s", addr)
		}
	}
	_, root, err := parseArrayLiteral(out)
	if err != nil {
		t.Fatalf("the masked literal %q is not an array literal the loader could write: %v", out, err)
	}
	elems, nulls := flattenArrayNode(root)
	if len(elems) != 2 || nulls != 0 {
		t.Fatalf("the masked literal holds %d elements and %d NULLs, want 2 and 0: length is preserved", len(elems), nulls)
	}
	for i, e := range elems {
		if _, err := mail.ParseAddress(e); err != nil {
			t.Errorf("element %d masked to %q, which is not an address: an array masks under its element type", i, e)
		}
	}
	if elems[0] == elems[1] {
		t.Error("two different addresses masked to one value: h is computed per element")
	}
}

// TestALiteralArrayKeepsItsNullsItsEmptinessAndItsDimensions is §5's array rule
// over the literal carrier: a NULL column stays NULL, an empty array stays
// empty, a NULL element stays NULL and a multidimensional array keeps its shape.
func TestALiteralArrayKeepsItsNullsItsEmptinessAndItsDimensions(t *testing.T) {
	t.Parallel()
	got, _ := maskLiterals(t, key(t, 0x22),
		nil,
		"{}",
		"{ada@fixture.test,NULL}",
		"{{ada@fixture.test,grace@fixture.test},{alan@fixture.test,edsger@fixture.test}}",
		"[0:1]={ada@fixture.test,grace@fixture.test}",
	)

	if got[0] != nil {
		t.Errorf("a NULL citext[] masked to %v; NULL stays NULL (§5)", got[0])
	}
	if got[1] != "{}" {
		t.Errorf("an empty array masked to %v; an empty array stays empty", got[1])
	}
	withNull, _ := got[2].(string)
	if !strings.HasSuffix(withNull, ",NULL}") {
		t.Errorf("a NULL element masked to something else in %q; a NULL element stays NULL", withNull)
	}
	nested, _ := got[3].(string)
	_, root, err := parseArrayLiteral(nested)
	if err != nil {
		t.Fatalf("a masked two-dimensional literal %q does not parse back: %v", nested, err)
	}
	if len(root.elems) != 2 || !root.elems[0].nested || len(root.elems[0].elems) != 2 {
		t.Errorf("a two-dimensional array masked to %q, which is not 2x2; array_in rejects a ragged literal", nested)
	}
	prefixed, _ := got[4].(string)
	if !strings.HasPrefix(prefixed, "[0:1]={") {
		t.Errorf("a dimension prefix was lost: %q", prefixed)
	}
}

// TestALiteralArrayMasksTheSameElementAlike is the determinism §5 rests on,
// stated where it is easiest to lose: h is computed per element, so one address
// in two positions of two rows masks to one value, and two runs under one key
// agree.
func TestALiteralArrayMasksTheSameElementAlike(t *testing.T) {
	t.Parallel()
	k := key(t, 0x23)
	first, _ := maskLiterals(t, k, "{ada@fixture.test,grace@fixture.test}", "{grace@fixture.test}")
	second, _ := maskLiterals(t, k, "{ada@fixture.test,grace@fixture.test}", "{grace@fixture.test}")

	if first[0] != second[0] || first[1] != second[1] {
		t.Fatalf("two runs under one key masked the same literal differently: %v then %v", first, second)
	}
	_, row0, err := parseArrayLiteral(first[0].(string))
	if err != nil {
		t.Fatalf("parseArrayLiteral: %v", err)
	}
	_, row1, err := parseArrayLiteral(first[1].(string))
	if err != nil {
		t.Fatalf("parseArrayLiteral: %v", err)
	}
	a, _ := flattenArrayNode(row0)
	b, _ := flattenArrayNode(row1)
	if a[1] != b[0] {
		t.Errorf("grace@fixture.test masked to %q in one row and %q in another; equal elements mask alike", a[1], b[0])
	}
}

// TestEveryLiteralElementEntersTheResidualFilter is §6 item 1 for this path.
// The contract internal/verify reads is one entry per element under the
// column's empty path, keyed by that element's own canonical bytes — the same
// row of internal/transform/CLAUDE.md's table the slice carrier obeys.
func TestEveryLiteralElementEntersTheResidualFilter(t *testing.T) {
	t.Parallel()
	k := key(t, 0x24)
	_, res := maskLiterals(t, k, "{ada@fixture.test,NULL,grace@fixture.test}")

	schema, _, c := literalFixture()
	tr := New(schema).(transformer)
	shape := tr.shapeOf(&schema.Tables[0], schema.Tables[0].Columns[1])
	for _, addr := range []string{"ada@fixture.test", "grace@fixture.test"} {
		r, err := mask.Apply(k, mask.CatEmail, mask.MaskerEmail, mask.Value{Text: addr}, shape.constraints)
		if err != nil {
			t.Fatalf("mask.Apply: %v", err)
		}
		if !res.MayContain(c, "", r.Canonical) {
			t.Errorf("the filter holds no entry for the element %s; a masked element verify cannot test is §6 item 1 failing open", addr)
		}
	}
	if len(res.adds) != 2 {
		t.Errorf("the run made %d filter entries for a three-element array with one NULL, want 2", len(res.adds))
	}
	for _, a := range res.adds {
		if a.path != "" {
			t.Errorf("an element entered the filter under the path %q; an array element uses the column's empty path", a.path)
		}
	}
}

// TestALiteralThatIsNotAnArrayIsARefusal is the exit-7 backstop for this
// carrier: a value in an array column that is not the server's output form
// stops the run rather than being masked as one string or copied through.
func TestALiteralThatIsNotAnArrayIsARefusal(t *testing.T) {
	t.Parallel()
	schema, cls, c := literalFixture()
	k := key(t, 0x25)
	_, err := New(schema).Transform(literalBatch("ada@fixture.test"), cls, &k, NewResidual(10))
	if err == nil {
		t.Fatal("a value in a citext[] column that is not an array literal was masked without a refusal")
	}
	var r *Refusal
	if !errors.As(err, &r) {
		t.Fatalf("Transform returned %T, want a *Refusal core can render", err)
	}
	if r.Code != CodeMasker || r.Exit != exitTransform {
		t.Errorf("the refusal carries %s at exit %d, want %s at exit %d", r.Code, r.Exit, CodeMasker, exitTransform)
	}
	if r.Col != c {
		t.Errorf("the refusal names %s, want %s", r.Col, c)
	}
	if strings.Contains(err.Error(), "ada@fixture.test") {
		t.Error("the refusal quotes the value it could not mask; an error string is a way out of the process (T4)")
	}
}
