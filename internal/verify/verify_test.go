// SPDX-License-Identifier: Apache-2.0

package verify

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/Liarea/lazyslice/internal/pg"
	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
	"github.com/Liarea/lazyslice/internal/textsig"
	"github.com/Liarea/lazyslice/mask"
)

// The three things this package promises that can be pinned without a database:
// the statements it sends to the source are the ones its own Shapes() admits;
// the bytes it reproduces from a target value are the bytes mask.Apply put in
// the residual filter; and the leaf spelling is internal/transform's. Everything
// else about this stage is a statement about a real Postgres and lives in
// verify_integration_test.go.

func customers() ref.TableRef { return ref.TableRef{Schema: "public", Name: "customers"} }
func invoices() ref.TableRef  { return ref.TableRef{Schema: "billing", Name: "invoices"} }

// fakeChunk is one unnest argument list, as internal/plan would encode it.
type fakeChunk struct {
	casts []string
	cols  []any
	n     int
}

func (c fakeChunk) Len() int          { return c.n }
func (c fakeChunk) Column(i int) any  { return c.cols[i] }
func (c fakeChunk) Cast(i int) string { return c.casts[i] }

func planAndClassification() (*pipeline.Plan, *pipeline.Classification) {
	p := &pipeline.Plan{Steps: []pipeline.Step{
		{Table: customers(), Mode: pipeline.ChildOK, Keys: keys{n: 3}},
		{Table: invoices(), Mode: pipeline.ChildOK, Keys: keys{n: 1}},
	}}
	cls := &pipeline.Classification{Decisions: map[ref.ColumnRef]pipeline.Decision{
		{Table: customers(), Column: "email"}: {Category: pipeline.CatEmail, Masked: true},
		{Table: customers(), Column: "id"}:    {Category: pipeline.CatNone},
	}}
	return p, cls
}

// keys is the smallest pipeline.KeySet that answers Len.
type keys struct{ n int }

var _ pipeline.KeySet = keys{}

func (k keys) Len() int                    { return k.n }
func (k keys) Bytes() int64                { return int64(k.n) * 8 }
func (k keys) Chunks(int) []pipeline.Chunk { return nil }
func (k keys) FirstChunk(int) pipeline.Chunk {
	return nil
}
func (k keys) EachChunk(int, func(pipeline.Chunk) error) error { return nil }

func tracerFor(t *testing.T, p *pipeline.Plan, cls *pipeline.Classification) *pg.Tracer {
	t.Helper()
	var shapes []pg.Shape
	for _, s := range Shapes(p, cls) {
		shapes = append(shapes, pg.Shape{Name: s.Name, SQL: s.SQL})
	}
	tr, err := pg.NewTracer(shapes...)
	if err != nil {
		t.Fatalf("compiling the verify allowlist: %v", err)
	}
	return tr
}

// admits reports the shape name the allowlist matched, "" for a refusal. It
// asks the tracer the only way a caller can: by tracing the statement.
func admits(t *testing.T, tr *pg.Tracer, sql string) string {
	t.Helper()
	before := len(tr.Trace())
	ctx := tr.TraceQueryStart(context.Background(), nil, pgx.TraceQueryStartData{SQL: sql})
	trace := tr.Trace()
	if len(trace) != before+1 {
		t.Fatalf("the tracer recorded %d statements for one call", len(trace)-before)
	}
	rec := trace[len(trace)-1]
	if (ctx.Err() == nil) == rec.Refused {
		t.Fatalf("the tracer's context and its record disagree about %q", sql)
	}
	return rec.Shape
}

// Every statement this package sends to the source has to match a shape it
// exports, or it never reaches the server (THREAT_MODEL.md T9). A clause added
// in sql.go and not in shapes.go fails here rather than on a production source.
func TestEveryStatementVerifySendsToTheSourceMatchesAShape(t *testing.T) {
	p, cls := planAndClassification()
	tr := tracerFor(t, p, cls)

	cases := []struct {
		name, sql, want string
	}{
		{
			name: "the indexable confirmation probe",
			sql:  probeSQL(customers(), "email"),
			want: "verify.probe.public.customers.email",
		},
		{
			name: "the case-folded confirmation probe",
			sql:  foldedProbeSQL(customers(), "email"),
			want: "verify.probe.folded.public.customers.email",
		},
		{
			name: "a sample fetched by a single int8 key",
			sql: sampleSQL(customers(), []string{"id", "email"}, []string{"id"}, []string{""},
				fakeChunk{casts: []string{"::int8[]"}, cols: []any{[]int64{1, 2}}, n: 2}),
			want: "verify.sample.public.customers",
		},
		{
			name: "a sample fetched by a composite key that is cast back",
			sql: sampleSQL(invoices(), []string{"code", "uid", "Payload"},
				[]string{"code", "uid"}, []string{"", "::timestamp with time zone"},
				fakeChunk{casts: []string{"::text[]", "::text[]"}, cols: []any{[]string{"a"}, []string{"b"}}, n: 1}),
			want: "verify.sample.billing.invoices",
		},
		{
			name: "a probe on a column the run did not mask",
			sql:  probeSQL(customers(), "id"),
			want: "",
		},
		{
			name: "a probe on a table the plan never named",
			sql:  probeSQL(ref.TableRef{Schema: "public", Name: "secrets"}, "email"),
			want: "",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := admits(t, tr, c.sql); got != c.want {
				t.Errorf("admits(%q) = %q, want %q", c.sql, got, c.want)
			}
		})
	}
}

// The residual filter is keyed with mask.Apply's own Result.Canonical over the
// source's value (ARCHITECTURE.md §6 item 1). This stage reproduces those bytes
// from the target's value, and bytes it cannot reproduce are a scan that reports
// no hit for a value that leaked (THREAT_MODEL.md T12). Nothing but this test
// holds the two together across the package boundary.
func TestCanonicalReproducesWhatMaskApplyRecorded(t *testing.T) {
	key, err := mask.NewKey()
	if err != nil {
		t.Fatalf("making a key: %v", err)
	}
	cases := []struct {
		name  string
		cat   pipeline.Category
		id    mask.ID
		value any
	}{
		{"an email as text", pipeline.CatEmail, "email", "Alice.Smith@Example.COM"},
		{"a phone as text", pipeline.CatPhone, "phone", "+44 20 7946 0958"},
		{"a name with odd spacing", pipeline.CatPersonName, "person_name", "  Ada   Lovelace "},
		{"a national id as a bigint", pipeline.CatNationalID, "national_id", int64(4509876543)},
		{"free text", pipeline.CatFreeText, "free_text", "a note about a person"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r, err := mask.Apply(key, mask.Category(c.cat), c.id, valueOf(c.value), mask.Constraints{})
			if err != nil {
				t.Fatalf("mask.Apply: %v", err)
			}
			if !r.Masked {
				t.Fatalf("mask.Apply recorded nothing for this value, so there is nothing to reproduce")
			}
			got, ok, err := canonicalOf(mask.Category(c.cat), c.value)
			if err != nil || !ok {
				t.Fatalf("canonicalOf: ok=%v err=%v", ok, err)
			}
			if string(got) != string(r.Canonical) {
				t.Errorf("canonicalOf gives %d bytes and mask.Apply recorded %d; the residual scan would miss this value",
					len(got), len(r.Canonical))
			}
		})
	}
}

// A NULL and an empty value are the two mask.Apply passes through, and
// internal/transform records neither. Testing the filter for one would be a hit
// on a run that masked correctly.
func TestNullAndEmptyAreNotTested(t *testing.T) {
	for _, v := range []any{nil, "", []byte(nil)} {
		if _, ok, err := canonicalOf(mask.CatEmail, v); ok || err != nil {
			t.Errorf("canonicalOf(%#v) is testable, and internal/transform recorded nothing for it", v)
		}
	}
}

// The leaf spelling is internal/transform's, stated in its CLAUDE.md: a path is
// $.a.b[0], a string and a number leaf are recorded and a boolean and a null are
// not.
func TestLeavesAreSpeltAsTransformSpellsThem(t *testing.T) {
	doc := map[string]any{
		"a":    map[string]any{"b": []any{"x", float64(2), true, nil}},
		"note": "hello",
	}
	var got []string
	for _, l := range leaves(doc) {
		got = append(got, l.path+"="+l.text)
	}
	sort.Strings(got)
	want := []string{"$.a.b[0]=x", "$.a.b[1]=2", "$.note=hello"}
	if len(got) != len(want) {
		t.Fatalf("leaves gave %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("leaves gave %v, want %v", got, want)
		}
	}
}

// A document that arrives as text is parsed with json.Number, so an integer leaf
// is not silently a float — the same decode internal/transform makes.
func TestALeafOfATextDocumentKeepsItsSpelling(t *testing.T) {
	ls := leaves(`{"n": 100000000000000000001}`)
	if len(ls) != 1 || ls[0].text != "100000000000000000001" {
		t.Fatalf("leaves gave %#v; a number leaf must keep the source's spelling", ls)
	}
}

// Verify is handed the target as a pipeline.Writer, which cannot be read from
// (ARCHITECTURE.md §2). Every check in §6 is a read, so a Writer with no Query
// is a refusal and never a report full of checks that did not happen.
func TestVerifyRefusesAWriterItCannotRead(t *testing.T) {
	p, cls := planAndClassification()
	report, err := New(Options{}).Verify(context.Background(), nil, writeOnly{}, &pipeline.Schema{}, p, cls,
		noResidual{}, &pipeline.LoadResult{})
	if report != nil {
		t.Errorf("a verify that could not read the target returned a report")
	}
	if !errors.Is(err, errNoTargetReader) {
		t.Fatalf("err = %v, want the no-target-reader refusal", err)
	}
}

// The four arguments §6 cannot run without are refused rather than skipped. A
// missing residual filter is the one that matters most: it is THREAT_MODEL.md
// T12's only control, and a run without it must not report a green tick.
func TestVerifyRefusesMissingArguments(t *testing.T) {
	p, cls := planAndClassification()
	cases := []struct {
		name   string
		schema *pipeline.Schema
		plan   *pipeline.Plan
		cls    *pipeline.Classification
		res    pipeline.Residual
		lr     *pipeline.LoadResult
	}{
		{"no schema", nil, p, cls, noResidual{}, &pipeline.LoadResult{}},
		{"no plan", &pipeline.Schema{}, nil, cls, noResidual{}, &pipeline.LoadResult{}},
		{"no classification", &pipeline.Schema{}, p, nil, noResidual{}, &pipeline.LoadResult{}},
		{"no residual filter", &pipeline.Schema{}, p, cls, nil, &pipeline.LoadResult{}},
		{"no load result", &pipeline.Schema{}, p, cls, noResidual{}, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := New(Options{}).Verify(context.Background(), nil, writeOnly{},
				c.schema, c.plan, c.cls, c.res, c.lr)
			if err == nil {
				t.Fatal("Verify returned no error")
			}
		})
	}
}

// writeOnly is a pipeline.Writer with no Query, which is what internal/pg's
// writer is today.
type writeOnly struct{}

func (writeOnly) Exec(context.Context, string, ...any) error { return nil }
func (writeOnly) CopyFrom(context.Context, ref.TableRef, []string, <-chan []any) (int64, error) {
	return 0, nil
}
func (writeOnly) Begin(context.Context) (pipeline.Tx, error) { return nil, errors.New("no") }

// RegisterTypes is pipeline.Writer's fourth method:
// the loader registers the source's user-defined types on the target before the
// first CopyFrom. Verify never calls it — by the time verify holds a Writer the
// load is over — so this double answers nil.
func (writeOnly) RegisterTypes(context.Context, *pipeline.Schema) error { return nil }

// noResidual is a filter that answers no to everything.
type noResidual struct{}

func (noResidual) Add(ref.ColumnRef, string, []byte)             {}
func (noResidual) MayContain(ref.ColumnRef, string, []byte) bool { return false }
func (noResidual) AddEmitted(ref.ColumnRef, string, []byte)      {}
func (noResidual) Emitted(ref.ColumnRef, string, []byte) int64   { return 0 }
func (noResidual) Cells() int64                                  { return 0 }
func (noResidual) Bytes() int64                                  { return 0 }

// A domain over an array is masked element-wise by internal/transform, which
// resolves the domain to its base type *before* it strips the "[]"
// (internal/transform/constraints.go, shapeOf). Its residual filter therefore
// carries one entry per element, under the column's empty path, keyed by the
// element's canonical bytes.
//
// A shapeOf here that read only TypeName would call this column a scalar,
// canonicalise the whole []any through textOf and test bytes transform never
// recorded: no hit, a green tick, and every element of the column shipped in
// cleartext (THREAT_MODEL.md T12). The two shapeOf implementations are a
// cross-package contract stated in both CLAUDE.md files and nothing else holds
// them together, so this pins the pair verify has to agree on.
func TestArrayDomainIsScannedElementWise(t *testing.T) {
	s := &state{schema: &pipeline.Schema{Domains: []pipeline.NamedDef{
		{Name: "public.emails", Def: `CREATE DOMAIN public.emails AS text[]`},
		{Name: "public.short", Def: `CREATE DOMAIN public.short AS character varying(8) NOT NULL`},
	}}}
	cases := []struct {
		name       string
		col        pipeline.Column
		wantFamily string
		wantArray  bool
	}{
		{
			name:       "a domain over an array",
			col:        pipeline.Column{Name: "addresses", TypeName: "emails", Domain: "public.emails"},
			wantFamily: famText,
			wantArray:  true,
		},
		{
			name:       "a domain over a scalar",
			col:        pipeline.Column{Name: "code", TypeName: "short", Domain: "public.short"},
			wantFamily: famVarchar,
			wantArray:  false,
		},
		{
			name:       "a plain array",
			col:        pipeline.Column{Name: "tags", TypeName: "text[]"},
			wantFamily: famText,
			wantArray:  true,
		},
		{
			name:       "a plain scalar",
			col:        pipeline.Column{Name: "note", TypeName: "character varying(64)"},
			wantFamily: famVarchar,
			wantArray:  false,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			family, array := s.shapeOf(c.col)
			if family != c.wantFamily || array != c.wantArray {
				t.Errorf("shapeOf = (%q, %v), want (%q, %v); internal/transform's shapeOf gives the second pair",
					family, array, c.wantFamily, c.wantArray)
			}
		})
	}
}

// canonicalOf calls mask.Canonical with an empty mask.Constraints, while
// internal/transform keys the filter with the column's real ones. That is only
// correct because Canonical reads exactly one field, Region, and nothing in the
// tree sets it (value.go). The day it reads a second one, the residual scan
// stops matching for every masked column of that category and fails open,
// silently — which is the failure mode THREAT_MODEL.md T12's only control
// cannot have.
//
// So this walks mask.Constraints by reflection and sets each field in turn to a
// non-zero value, asserting the canonical form does not move. A new field on
// the struct is covered the day it is added, without anyone remembering to come
// back here. Region is the exception and is asserted the other way: it is the
// one field Canonical is known to read, and the whole assumption is that
// nothing in the tree sets it.
func TestCanonicalReadsNoConstraintButRegion(t *testing.T) {
	values := []struct {
		cat pipeline.Category
		v   string
	}{
		{pipeline.CatEmail, "Alice.Smith@Example.COM"},
		{pipeline.CatPhone, "+44 20 7946 0958"},
		{pipeline.CatFinancial, "GB33BUKB20201555555555"},
		{pipeline.CatPersonName, "Ada Lovelace"},
		{pipeline.CatSemiStruct, `{"a": 1}`},
	}
	typ := reflect.TypeOf(mask.Constraints{})
	for i := range typ.NumField() {
		field := typ.Field(i)
		if field.Name == "Region" {
			continue
		}
		t.Run(field.Name, func(t *testing.T) {
			c := reflect.New(typ).Elem()
			if !setNonZero(c.Field(i)) {
				t.Skipf("no non-zero value to put in a %s", field.Type)
			}
			constraints, _ := c.Interface().(mask.Constraints)
			for _, v := range values {
				want, _, err := mask.Canonical(mask.Category(v.cat), mask.Value{Text: v.v}, mask.Constraints{})
				if err != nil {
					t.Fatalf("mask.Canonical: %v", err)
				}
				got, _, err := mask.Canonical(mask.Category(v.cat), mask.Value{Text: v.v}, constraints)
				if err != nil {
					t.Fatalf("mask.Canonical with %s set: %v", field.Name, err)
				}
				if got.Text != want.Text || string(got.Bytes) != string(want.Bytes) {
					t.Fatalf("mask.Canonical now reads Constraints.%s, and value.go's canonicalOf passes none: "+
						"the residual scan would stop matching every masked %s column",
						field.Name, v.cat)
				}
			}
		})
	}

	// Region: the known live wire, and the reason value.go carries the note it
	// does. If this ever stops holding, the empty Constraints in canonicalOf is
	// no longer the same hint transform used and both change in one commit.
	withRegion, _, err := mask.Canonical(mask.CatPhone, mask.Value{Text: "020 7946 0958"}, mask.Constraints{Region: "GB"})
	if err != nil {
		t.Fatalf("mask.Canonical: %v", err)
	}
	without, _, err := mask.Canonical(mask.CatPhone, mask.Value{Text: "020 7946 0958"}, mask.Constraints{})
	if err != nil {
		t.Fatalf("mask.Canonical: %v", err)
	}
	if withRegion.Text == without.Text {
		t.Errorf("Region no longer changes the canonical form; value.go's note about it is stale")
	}
}

// setNonZero puts a non-zero value in one settable field, and reports whether
// it could.
func setNonZero(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.String:
		v.SetString("x")
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		v.SetInt(7)
	case reflect.Bool:
		v.SetBool(true)
	case reflect.Slice:
		v.Set(reflect.MakeSlice(v.Type(), 1, 1))
		return setNonZero(v.Index(0))
	default:
		return false
	}
	return true
}

// yesResidual is a filter that says every value may have been masked, which is
// what a real one does at the far end of its false-positive rate.
type yesResidual struct{ noResidual }

func (yesResidual) MayContain(ref.ColumnRef, string, []byte) bool { return true }

// A number leaf is in internal/transform's filter and is deliberately not
// tested here (residual.go, documentHits). transform redraws it over 10^6
// values, so a match on it is evidence about the size of that domain and not
// about the masker — and because no probe in §6 can ask about a value inside a
// document, every such match would be an unconditional exit 9 on a run that
// masked correctly. A string leaf keeps its entry.
func TestANumberLeafIsNotAResidualHit(t *testing.T) {
	s := &state{res: yesResidual{}}
	hits := s.documentHits(ref.ColumnRef{Table: customers(), Column: "profile"},
		`{"note": "hello", "score": 42, "ok": true, "gone": null}`)

	var leafValues []string
	whole := 0
	for _, h := range hits {
		if !h.leaf {
			whole++
			continue
		}
		v, _ := h.value.(string)
		leafValues = append(leafValues, v)
	}
	if whole != 1 {
		t.Errorf("the collapsed-document entry was tested %d times, want once", whole)
	}
	if len(leafValues) != 1 || leafValues[0] != "hello" {
		t.Errorf("leaf hits are %v; only the string leaf carries residual signal", leafValues)
	}
}

// fakeRows is a target result set of one column.
type fakeRows struct {
	vals []any
	i    int
}

func (r *fakeRows) Next() bool { r.i++; return r.i <= len(r.vals) }
func (r *fakeRows) Scan(dest ...any) error {
	if len(dest) != 1 {
		return errors.New("fakeRows: one column")
	}
	v := r.vals[r.i-1]
	switch p := dest[0].(type) {
	case *any:
		*p = v
		return nil
	case *[]byte:
		// T-0402: scanCell reads a json or jsonb column's raw text through a
		// *[]byte destination, exactly as internal/pg's source reader now
		// hands it back; every fixture that reaches this fake for such a
		// column already stores its target value as the source text a real
		// scan would return.
		return scanFakeBytes(v, p)
	}
	return errors.New("fakeRows: want *any")
}

// scanFakeBytes is the fakes' shared answer to a *[]byte destination
// (T-0402): nil stays nil, a string or []byte is its bytes as they stand, and
// a fixture that stored a document as a Go value directly (map[string]any,
// []any, a bare number or bool -- the convenience most of this package's
// table-driven fixtures use) is marshalled, standing in for the source text a
// real scan of that value would have returned.
func scanFakeBytes(v any, p *[]byte) error {
	switch t := v.(type) {
	case nil:
		*p = nil
		return nil
	case string:
		*p = []byte(t)
		return nil
	case []byte:
		*p = t
		return nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("scanFakeBytes: %T is not a json or jsonb fixture value: %w", v, err)
	}
	*p = b
	return nil
}
func (r *fakeRows) Err() error { return nil }
func (r *fakeRows) Close()     {}

// oneColumn is a target that answers every scan with the same values.
type oneColumn struct{ vals []any }

func (c oneColumn) Query(context.Context, string, ...any) (pipeline.Rows, error) {
	return &fakeRows{vals: c.vals}, nil
}

// A column holding fewer than minValues non-NULL values is unproven, not clean,
// and the second net fails on any hit in it.
//
// The threshold in ARCHITECTURE.md §4 is a ratio, and a ratio over two values
// says nothing; the first version of this net read that as "say nothing" and
// returned before the validators ran. That is a fail-open with nothing above
// it, and it is not hypothetical: public.devices.owned_by in testdata/nasty.sql
// is two email addresses and a NULL in a three-row table, the classifier's own
// value signal was silent for the same reason, and the address reached the
// target in cleartext under exit 0 (THREAT_MODEL.md T1, tracker T-0058).
//
// So the two halves are pinned together here. Two values, one of them an email
// address: 1/2 is below validatorThreshold and the column still fails, naming
// the table, the column and the category.
//
// The fourth case used to be the other half of that pin -- the same 0.5 ratio
// over minValues values did not fail, because there the ratio was the answer
// and this net was not a per-row scanner. **That stopped being true for a
// strong validator** (docs/reviews/2026-09-09/REVIEW.md finding 7,
// evidence/sparse_email.log): email is a precise parse, not a shape guess, so
// two email addresses among four values is still two email addresses in the
// target, whatever the ratio -- and the case now pins `strong` failing on any
// hit once the column is proven, not only below minValues. The fifth case is
// what the fourth case used to pin, restated over a validator that is not
// strong: `address` is a shape guess (mixed digits and words), so the same
// 0.5 ratio over four proven values still does not fail, and stays the
// evidence that the ratio rule was narrowed rather than removed.
func TestAColumnBelowMinValuesFailsOnAnyHit(t *testing.T) {
	table := customers()
	col := ref.ColumnRef{Table: table, Column: "note"}

	cases := []struct {
		name      string
		vals      []any
		wantFail  bool
		wantCount int64
	}{
		{
			name:      "two values, one an email address",
			vals:      []any{"ada.lovelace@fixture.test", "nothing to see"},
			wantFail:  true,
			wantCount: 1,
		},
		{
			name:      "one value, an email address",
			vals:      []any{"ada.lovelace@fixture.test", nil},
			wantFail:  true,
			wantCount: 1,
		},
		{
			name:     "two values, neither an email address",
			vals:     []any{"nothing to see", "still nothing"},
			wantFail: false,
		},
		{
			// email is strong (validators.go): a proven column still fails on
			// any hit, not only a ratio at or over validatorThreshold. The
			// review's finding 3 asked for Refusal.Count to be asserted too
			// -- the brief names it as part of the required message, and a
			// change that miscounted the hits (rather than only whether any
			// existed) previously passed unnoticed.
			name:      "four values, two email addresses (strong: any hit fails)",
			vals:      []any{"ada.lovelace@fixture.test", "grace.hopper@fixture.test", "nothing to see", "still nothing"},
			wantFail:  true,
			wantCount: 2,
		},
		{
			// address is not strong: the ratio rule stays, and half of four
			// is below validatorThreshold.
			name:     "four values, two addresses (heuristic: the ratio rule stays)",
			vals:     []any{"742 Evergreen Terrace", "10 Downing Street", "nothing to see", "still nothing"},
			wantFail: false,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := &state{
				schema: &pipeline.Schema{},
				target: oneColumn{vals: c.vals},
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
			if !c.wantFail {
				if len(s.failures) != 0 {
					t.Fatalf("the net failed %s on %v: %s", col, c.vals, s.failures[0].Reason)
				}
				return
			}
			if len(s.failures) != 1 {
				t.Fatalf("the net recorded %d failures on %v, want one; a value nobody masked "+
					"is in the target and both nets said yes", len(s.failures), c.vals)
			}
			got := s.failures[0]
			if got.Table != table || got.Column != col.Column {
				t.Errorf("the refusal names %s.%s, want %s", got.Table, got.Column, col)
			}
			if got.Reason != "email" {
				t.Errorf("the refusal names the category %q, want %q", got.Reason, "email")
			}
			if got.Exit != exitResidual {
				t.Errorf("the refusal exits %d, want %d", got.Exit, exitResidual)
			}
			if got.Count != c.wantCount {
				t.Errorf("the refusal counts %d hits, want %d", got.Count, c.wantCount)
			}
			// THREAT_MODEL.md T4: no value ever reaches a Refusal. The
			// message is a fixed phrase ("email") and neither of the sample
			// values may appear in it, whatever they are.
			for _, v := range c.vals {
				if s, ok := v.(string); ok && strings.Contains(got.Reason, s) {
					t.Errorf("the refusal reason %q carries a sample value %q", got.Reason, s)
				}
			}
		})
	}
}

// TestLuhnOnANumericColumnKeepsTheRatioRule is the T-0136 review's finding 2:
// the Luhn entry used to be marked strong for both families it runs over, so
// a numeric column with a single Luhn-passing value among ordinary
// identifiers failed on that one hit with no ratio escape. Roughly one in ten
// twelve-to-nineteen-digit identifiers passes the check digit by chance
// (snowflake IDs, epoch-millisecond timestamps, EAN-13 barcodes, order
// numbers), so an ordinary unmasked bigint id column of any realistic size
// held at least one and this net refused an already-loaded target with no
// green path short of --unmask, over a column holding no personal data.
// validators.go now carries two Luhn entries: strong over character columns,
// ratio-scored over integer/bigint/numeric ones -- this pins the numeric
// side both ways, the same shape TestAColumnBelowMinValuesFailsOnAnyHit pins
// for email and address.
func TestLuhnOnANumericColumnKeepsTheRatioRule(t *testing.T) {
	table := customers()
	col := ref.ColumnRef{Table: table, Column: "reference"}

	// luhnValidID(4000000000000000) is a Luhn-valid sixteen-digit number under
	// Visa's prefix, the full shape of a card (textsig.CardShape), so it is a
	// real hit on a column named `reference` (T-0316); the ids from
	// thirteenDigitIDs are ordinary thirteen-digit identifiers that do not
	// pass the check digit.
	cases := []struct {
		name     string
		vals     []any
		wantFail bool
	}{
		{
			name:     "one card-shaped hit among nineteen ordinary thirteen-digit ids (below the ratio, proven column)",
			vals:     append(append([]any{}, thirteenDigitIDs(19)...), luhnValidID(4000000000000000)),
			wantFail: false,
		},
		{
			name:     "sixteen Luhn hits out of twenty (at or over the ratio)",
			vals:     luhnMajority(),
			wantFail: true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := &state{
				schema: &pipeline.Schema{},
				target: oneColumn{vals: c.vals},
				steps:  []pipeline.Step{{Table: table, Mode: pipeline.ChildOK}},
				tables: map[ref.TableRef]*pipeline.Table{
					table: {Ref: table, Columns: []pipeline.Column{{Name: col.Column, TypeName: "bigint"}}},
				},
				cls: &pipeline.Classification{Decisions: map[ref.ColumnRef]pipeline.Decision{
					col: {Col: col, Category: pipeline.CatNone, Source: pipeline.ByClassifier},
				}},
			}
			if err := s.secondNet(context.Background()); err != nil {
				t.Fatalf("secondNet: %v", err)
			}
			if !c.wantFail {
				if len(s.failures) != 0 {
					t.Fatalf("the net failed %s on a numeric column with a minority Luhn hit: %s "+
						"(the digits side of the Luhn entry must not be strong)", col, s.failures[0].Reason)
				}
				return
			}
			if len(s.failures) != 1 {
				t.Fatalf("the net recorded %d failures, want one", len(s.failures))
			}
			if s.failures[0].Reason != "financial_account" {
				t.Errorf("the refusal names the category %q, want %q", s.failures[0].Reason, "financial_account")
			}
		})
	}
}

// thirteenDigitIDs returns n ordinary thirteen-digit identifiers, none of
// which passes the Luhn check digit.
func thirteenDigitIDs(n int) []any {
	out := make([]any, 0, n)
	for i := 0; i < n; i++ {
		v := int64(1300000000000) + int64(i)
		for textsig.ValidLuhn(strconv.FormatInt(v, 10)) {
			v++
		}
		out = append(out, v)
	}
	return out
}

// luhnValidID returns the smallest value at or above base whose decimal form
// passes the Luhn check digit -- the digit-family counterpart of
// thirteenDigitIDs, which walks the other way.
func luhnValidID(base int64) int64 {
	v := base
	for !textsig.ValidLuhn(strconv.FormatInt(v, 10)) {
		v++
	}
	return v
}

// luhnMajority is twenty values, sixteen of which are Luhn-valid -- at or
// over validatorThreshold, where even the ratio-scored digits side of the
// Luhn entry fails. The sixteen are sixteen-digit numbers under Visa's
// prefix, the full shape of a card (textsig.CardShape): since T-0316 a column
// named `reference` is scored on that shape, and the thirteen-digit numbers
// this helper used to return pass only the check digit.
func luhnMajority() []any {
	out := make([]any, 0, 20)
	for i := 0; i < 16; i++ {
		out = append(out, luhnValidID(int64(4000000000000000)+int64(i)*7))
	}
	for i := 0; i < 4; i++ {
		out = append(out, thirteenDigitIDs(1)[0])
	}
	return out
}

// The dictionary rule (tracker T-0055 and its review): person_name and
// free_text are the two validators backed by the name dictionary, and this net
// reads narrower validators than internal/classify does and scores them under a
// threshold of its own.
//
// The cases it exists for are the negative ones below. The dictionary's surname
// section is about two hundred ordinary English words — black, green, hill,
// wood, west, lane, stone, may, price, read, little, long — so under the
// classifier's own two validators a colour column is 100% "person_name", a
// column of street names is too, and any English sentence carrying one of those
// words is "free_text". The classifier's answer to that is to mask the column,
// which costs a lookup table; this net's answer would be exit 9 on a database
// that is already loaded, over a column holding no personal data, with --unmask
// the only way past it. So here a hit has to be a *shape*: a given name
// immediately followed by a surname, over the whole value for person_name and
// anywhere inside the sentence for free_text, and either way the strong ratio
// across at least minValues distinct hitting values.
//
// The last case is the one path where the classifier cannot pre-empt such a
// refusal at all. internal/classify decides a json/jsonb/hstore column on its
// own leaf signal, which asks the key patterns plus email, phone, IP, IBAN and
// Luhn and never consults the dictionary, so the two dictionary validators do
// not run over leaves here either (applies, in secondnet.go): the same values
// that fail as a text column pass as leaves.
func TestTheDictionaryRule(t *testing.T) {
	table := customers()
	col := ref.ColumnRef{Table: table, Column: "note"}

	// The prose that does carry a written name, used twice below: as a text
	// column, where it is the trap-17 case this validator exists for, and
	// inside a document, where it must not fail.
	named := []any{
		"Grace Hopper asked that the renewal be sent to finance.",
		"Escalation contact for this account is Alan Turing.",
		"Raised by Katherine Johnson on the second of March.",
	}

	cases := []struct {
		name     string
		typeName string // "" is a text column
		masked   bool
		vals     []any
		wantFail string // the category the refusal names, empty for no refusal
	}{
		{
			name: "a column of single dictionary words is not a person",
			vals: []any{"black", "brown", "hill", "green", "wood"},
		},
		{
			// Every one of these is two dictionary words, which is all the
			// first version of this rule asked for; none is a given name
			// followed by a surname, because green, lane, west, hill, long,
			// marsh and stone are all in the surname section only.
			name: "and neither is a column of two-word street names",
			vals: []any{"green lane", "west hill", "long lane", "marsh lane", "stone hill"},
		},
		{
			name: "nor a column of compound colours",
			vals: []any{"hunter green", "stone gray", "berry rose", "black cherry", "hunter green"},
		},
		{
			// Every word of every value is in the dictionary, and every value
			// is a given name followed by a surname. "Ada Lovelace" is not,
			// because "lovelace" is not a common surname and the dictionary is
			// deliberately a list of common names (names.txt).
			name:     "a column of written names is",
			vals:     []any{"Grace Hopper", "Alan Turing", "Katherine Johnson", "Mary Taylor"},
			wantFail: "person_name",
		},
		{
			name: "two written names are not enough distinct values",
			vals: []any{"Grace Hopper", "Alan Turing"},
		},
		{
			name: "and neither is one written name repeated",
			vals: []any{"Grace Hopper", "Grace Hopper", "Grace Hopper", "Grace Hopper"},
		},
		{
			name:     "prose carrying other rows' names is free_text",
			vals:     named,
			wantFail: "free_text",
		},
		{
			name: "prose with no name in it is not",
			vals: []any{
				"The renewal was sent to finance on the second.",
				"This account is billed quarterly under the old terms.",
				"Confirmed with the finance team on the second of March.",
			},
		},
		{
			// Ordinary business prose whose only dictionary words are the
			// English ones — may, black, read, little, price. Under the
			// classifier's Prose ("six words with a dictionary word inside")
			// every one of these is a hit and the column is exit 9 on a loaded
			// target holding no personal data.
			name: "nor is business prose whose only dictionary word is an English one",
			vals: []any{
				"The supplier may terminate this agreement on thirty days notice.",
				"Delivery is made in a matte black finish as standard.",
				"Please read the enclosed instructions before first use.",
				"The little pockets on either side hold a passport.",
				"Any change to the price takes effect from the next invoice.",
			},
		},
		{
			// The one path where the classifier cannot have pre-empted this
			// refusal: it decides a jsonb column on a leaf signal that never
			// reads the dictionary, so these validators do not run over leaves.
			// Same sentences as the failing free_text case above, one per
			// document.
			name:     "and prose in a masked jsonb column is outside these two validators",
			typeName: "jsonb",
			masked:   true,
			vals: []any{
				`{"note": "Grace Hopper asked that the renewal be sent to finance."}`,
				`{"note": "Escalation contact for this account is Alan Turing."}`,
				`{"note": "Raised by Katherine Johnson on the second of March."}`,
				`{"note": "Mary Taylor confirmed the renewal on the second of March."}`,
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			typeName := c.typeName
			if typeName == "" {
				typeName = "text"
			}
			dec := pipeline.Decision{Col: col, Category: pipeline.CatNone, Source: pipeline.ByClassifier}
			if c.masked {
				dec.Category, dec.Masked = pipeline.CatSemiStruct, true
			}
			s := &state{
				schema: &pipeline.Schema{},
				target: oneColumn{vals: c.vals},
				steps:  []pipeline.Step{{Table: table, Mode: pipeline.ChildOK}},
				tables: map[ref.TableRef]*pipeline.Table{
					table: {Ref: table, Columns: []pipeline.Column{{Name: col.Column, TypeName: typeName}}},
				},
				cls: &pipeline.Classification{Decisions: map[ref.ColumnRef]pipeline.Decision{col: dec}},
			}
			if err := s.secondNet(context.Background()); err != nil {
				t.Fatalf("secondNet: %v", err)
			}
			if c.wantFail == "" {
				if len(s.failures) != 0 {
					t.Fatalf("the net failed %s on %v as %q; a dictionary word is a word an "+
						"ordinary English column may hold, and this refusal has no green path "+
						"short of --unmask", col, c.vals, s.failures[0].Reason)
				}
				return
			}
			if len(s.failures) != 1 {
				t.Fatalf("the net recorded %d failures on %v, want one naming %s; a value nobody "+
					"masked is in the target and both nets said yes", len(s.failures), c.vals, c.wantFail)
			}
			if got := s.failures[0].Reason; got != c.wantFail {
				t.Errorf("the refusal names the category %q, want %q", got, c.wantFail)
			}
			if got := s.failures[0].Exit; got != exitResidual {
				t.Errorf("the refusal exits %d, want %d", got, exitResidual)
			}
		})
	}
}

// A URL is the shape neither net could see between tracker T-0100 and T-0122.
//
// T-0100 took a URL out of textsig.LooksSecret — a URL clears every guard that
// validator has, so mastodon's accounts.uri read as `credential` on every row —
// and put textsig.ValidURL into internal/classify ahead of the secrets
// validator. internal/verify was outside that task's paths, so this net kept
// the credential entry that no longer matched a URL and gained no online_id
// entry at all: email, phone, ip, mac, luhn, iban, LooksSecret, NameShape,
// AddressShape and ProseName all return false for a profile URI, so one that
// reached the target unmasked was seen by nothing and the run exited 0.
// THREAT_MODEL.md T1 makes this net a blocking control for the column the
// 200-row sample under-represented, and the two packages score independently,
// so classify gaining the validator did not compensate.
//
// The second case is the half that says the entry is in the right place: the
// refusal names online_id and not credential, which is the order of the two in
// validators.go and in internal/classify.
func TestAProfileURLFailsTheSecondNetAsAnOnlineID(t *testing.T) {
	table := customers()
	col := ref.ColumnRef{Table: table, Column: "note"}

	cases := []struct {
		name     string
		vals     []any
		wantFail string // the category the refusal names, empty for no refusal
	}{
		{
			name: "mastodon-shaped profile URIs",
			vals: []any{
				"https://home.social.test/users/bea_donnelly1",
				"https://home.social.test/users/ada_lovelace2",
				"https://home.social.test/users/grace_hopper3",
				"https://home.social.test/users/alan_turing4",
			},
			wantFail: "online_id",
		},
		{
			name: "a column of ordinary words is still not a URL",
			vals: []any{"pending", "settled", "refunded", "settled"},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := &state{
				schema: &pipeline.Schema{},
				target: oneColumn{vals: c.vals},
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
			if c.wantFail == "" {
				if len(s.failures) != 0 {
					t.Fatalf("the net failed %s on %v as %q", col, c.vals, s.failures[0].Reason)
				}
				return
			}
			if len(s.failures) != 1 {
				t.Fatalf("the net recorded %d failures on %v, want one naming %s; a profile URI "+
					"nobody masked is in the target and neither net saw it", len(s.failures), c.vals, c.wantFail)
			}
			got := s.failures[0]
			if got.Reason != c.wantFail {
				t.Errorf("the refusal names the category %q, want %q; online_id is ahead of "+
					"credential in validators.go for the reason T-0100 gives", got.Reason, c.wantFail)
			}
			if got.Table != table || got.Column != col.Column {
				t.Errorf("the refusal names %s.%s, want %s", got.Table, got.Column, col)
			}
			if got.Exit != exitResidual {
				t.Errorf("the refusal exits %d, want %d", got.Exit, exitResidual)
			}
		})
	}
}

// TestTheSecondNetSkipsAFrameworkTablesOwnBookkeepingColumns is T-0348, the
// verify half of T-0314: a Rails migration timestamp that passes the Luhn
// check (dogfood session 1's exact shape) must not refuse the run when it
// sits in schema_migrations.version, a column classify never masks; a
// framework table's identity-bearing column, which is not on the tool's
// bookkeeping allowlist, is still scanned.
func TestTheSecondNetSkipsAFrameworkTablesOwnBookkeepingColumns(t *testing.T) {
	cases := []struct {
		name        string
		table       ref.TableRef
		column      string
		vals        []any
		neverMasked bool
		wantFail    string
	}{
		{
			name:        "schema_migrations.version passing Luhn is copied, not refused",
			table:       ref.TableRef{Schema: "public", Name: "schema_migrations"},
			column:      "version",
			vals:        []any{"20250101050000", "20250101130000", "20250101210000"},
			neverMasked: true,
		},
		{
			name:     "flyway_schema_history.installed_by holding addresses is still scanned",
			table:    ref.TableRef{Schema: "public", Name: "flyway_schema_history"},
			column:   "installed_by",
			vals:     []any{"ana.silva@corp.example", "li.wei@corp.example", "omar.haddad@corp.example", "eva.novak@corp.example"},
			wantFail: "email",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			col := ref.ColumnRef{Table: c.table, Column: c.column}
			s := &state{
				schema: &pipeline.Schema{},
				target: oneColumn{vals: c.vals},
				steps:  []pipeline.Step{{Table: c.table, Mode: pipeline.Lookup}},
				tables: map[ref.TableRef]*pipeline.Table{
					c.table: {Ref: c.table, Columns: []pipeline.Column{{Name: c.column, TypeName: "text"}}},
				},
				cls: &pipeline.Classification{Decisions: map[ref.ColumnRef]pipeline.Decision{
					col: {Col: col, Category: pipeline.CatNone, Source: pipeline.ByClassifier, NeverMasked: c.neverMasked},
				}},
			}
			if err := s.secondNet(context.Background()); err != nil {
				t.Fatalf("secondNet: %v", err)
			}
			if c.wantFail == "" {
				if len(s.failures) != 0 {
					t.Fatalf("the net refused %s on %v as %q; a migration tool's own bookkeeping column is copied by design", col, c.vals, s.failures[0].Reason)
				}
				return
			}
			if len(s.failures) != 1 || s.failures[0].Reason != c.wantFail {
				t.Fatalf("the net recorded %d failures on %v, want one naming %s: a framework table's identity column is not on the bookkeeping allowlist", len(s.failures), c.vals, c.wantFail)
			}
		})
	}
}

// TestSecondNetReadsDocumentKeysAsWellAsValues is the T-0137 review round's
// finding 3: keyHits (residual.go) tests object keys of a *masked* column
// against the filter, but the second net's netValues (secondnet.go) used to
// call leaves(v) alone, which never yields a key. So an email, a phone
// number or a card used as a JSON key inside a column classified `none` or
// carrying `--unmask` was masked by nothing (there is no masker for an
// unmasked column) and seen by neither net, while the identical address as a
// VALUE in the same document was already caught. netValues now folds
// documentKeys(v) in alongside leaves(v) whenever mode.leaves is set, so the
// net's coverage of keys no longer stops at columns internal/transform
// actually masked.
func TestSecondNetReadsDocumentKeysAsWellAsValues(t *testing.T) {
	table := customers()
	col := ref.ColumnRef{Table: table, Column: "profile"}

	cases := []struct {
		name     string
		masked   bool
		vals     []any
		wantFail string
	}{
		{
			name:   "an email used as a key in an unmasked jsonb column",
			masked: false,
			vals: []any{
				`{"ada.lovelace@fixture.test":"ok"}`,
				`{"note":"nothing to see"}`,
				`{"note":"still nothing"}`,
				`{"note":"and nothing here either"}`,
			},
			wantFail: "email",
		},
		{
			name:   "no key of that shape is not a hit",
			masked: false,
			vals: []any{
				`{"note":"nothing to see"}`,
				`{"note":"still nothing"}`,
			},
		},
		// T-0172: a masked document column's key that still matches one of
		// the three strongKeyCategory validators is the masker's own output,
		// not a surviving source value — json.go's maskKey already ran every
		// key through the same three-validator question and replaced every
		// match with that category's own masker, whose output is by
		// construction a value of that category (mask/gen_email.go's
		// address, textsig's own "+44 20 7946 0958" fixture value above,
		// mask/gen_number.go's Luhn-valid digit run). Before this task, the
		// net counted that as a hit and refused a run that masked correctly
		// (testdata/regressions/013). The leaf beside each key is
		// free_text's own filler-word shape (mask/words.go's fillerWords),
		// which is what every leaf of a masked document actually holds
		// (json.go's leafCategory) and matches no validator either — the
		// exposure this task's diagnosis asked about is confirmed absent.
		{
			name:   "a masked document column's key is the email masker's own output",
			masked: true,
			vals:   []any{`{"alice.k7v2x@example.com":"buffer cadence delta"}`},
		},
		{
			name:   "a masked document column's key is the phone masker's own output",
			masked: true,
			vals:   []any{`{"+44 20 7946 0958":"buffer cadence delta"}`},
		},
		{
			name:   "a masked document column's key is the financial masker's own output",
			masked: true,
			vals:   []any{`{"4111111111111111":"buffer cadence delta"}`},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dec := pipeline.Decision{Col: col, Category: pipeline.CatNone, Source: pipeline.ByClassifier}
			if c.masked {
				dec.Category, dec.Masked = pipeline.CatSemiStruct, true
			}
			s := &state{
				schema: &pipeline.Schema{},
				target: oneColumn{vals: c.vals},
				steps:  []pipeline.Step{{Table: table, Mode: pipeline.ChildOK}},
				tables: map[ref.TableRef]*pipeline.Table{
					table: {Ref: table, Columns: []pipeline.Column{{Name: col.Column, TypeName: "jsonb"}}},
				},
				cls: &pipeline.Classification{Decisions: map[ref.ColumnRef]pipeline.Decision{col: dec}},
			}
			if err := s.secondNet(context.Background()); err != nil {
				t.Fatalf("secondNet: %v", err)
			}
			if c.wantFail == "" {
				if len(s.failures) != 0 {
					t.Fatalf("the net failed %s on %v as %q, want no failure", col, c.vals, s.failures[0].Reason)
				}
				return
			}
			if len(s.failures) != 1 {
				t.Fatalf("the net recorded %d failures on %v, want one naming %s; an unmasked "+
					"key of production shape is in the target and neither net saw it",
					len(s.failures), c.vals, c.wantFail)
			}
			if got := s.failures[0].Reason; got != c.wantFail {
				t.Errorf("the refusal names the category %q, want %q", got, c.wantFail)
			}
			if got := s.failures[0].Exit; got != exitResidual {
				t.Errorf("the refusal exits %d, want %d", got, exitResidual)
			}
		})
	}
}

// T-0402 (the 2026-09-25 JSON red team's A11, docs/reviews/2026-09-25-
// redteam-json/round1.json entry 14): leaves (columns.go) keeps a number
// leaf's exact digits now that scanColumn and scanRows (target.go) hand
// decodeDocument the source's own text for a json or jsonb column, through
// rawJSONColumn, instead of whatever Go value the target pool's own *any
// scan would have decoded — a float64, which rounds an integer past 2^53.
//
// This is a decode-correctness guard, not a detection one: neither the second
// net (secondnet.go's netStrings, its `if !l.str` skip) nor the residual scan
// (residual.go's documentHits, the identical skip, and its own comment —
// "a *number* leaf ... is deliberately not tested") ever reads a number
// leaf's *value* at all, by a design predating this task, so nothing here
// makes either net catch a card number it did not catch before. What T-0402
// actually closes is internal/transform's own leafValueCategory, which reads
// the identical text this function now reads exactly rather than rounded —
// see internal/transform/leaf_test.go's own regression. This test is the
// verify-side half the tracker task names anyway: leaves' own contract, that
// a number leaf's text is the source's exact spelling.
func TestLeavesKeepsANumberLeafsExactDigitsPastFloat64Precision(t *testing.T) {
	const pan16 = "4111111111111111"    // Luhn-valid, 16 digits: exact in a float64
	const pan19 = "4000123456789012343" // Luhn-valid, 19 digits: a float64 rounds this

	doc := `{"p0":` + pan16 + `,"p1":` + pan19 + `}`
	got := map[string]string{}
	for _, l := range leaves(doc) {
		got[l.path] = l.text
	}
	for path, want := range map[string]string{"$.p0": pan16, "$.p1": pan19} {
		if got[path] != want {
			t.Errorf("leaves(%q)[%q] = %q, want %q: a float64 would have rounded it", doc, path, got[path], want)
		}
	}
}

// TestRawJSONColumnAnswersFalseForAnArray pins rawJSONColumn (target.go)
// directly, over every shape the fix round after T-0402's first pass found it
// answering wrong for: a review found the function reading only the family
// shapeOf returns and ignoring the array flag beside it, so a json[] or
// jsonb[] column took the same raw-text path a scalar column does. That path
// scans into a *[]byte and hands decodeDocument the exact wire text — correct
// for a scalar json or jsonb column, whose wire text is the document itself,
// but wrong for an array, whose wire text is a Postgres array literal
// (`{"{...}"}`) that decodeDocument's JSON decoder cannot parse at all,
// silently losing every leaf of the array to the residual scan rather than
// merely leaving a number leaf float64-rounded. hstore is document(family)
// too and carries no JSON codec either way, so it stays false regardless of
// the array flag, unchanged from before this task.
func TestRawJSONColumnAnswersFalseForAnArray(t *testing.T) {
	table := customers()
	cases := []struct {
		name string
		typ  string
		want bool
	}{
		{"jsonb scalar", "jsonb", true},
		{"json scalar", "json", true},
		{"jsonb array", "jsonb[]", false},
		{"json array", "json[]", false},
		{"hstore scalar", "hstore", false},
		{"hstore array", "hstore[]", false},
		{"text scalar", "text", false},
		{"text array", "text[]", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := &state{tables: map[ref.TableRef]*pipeline.Table{
				table: {Ref: table, Columns: []pipeline.Column{{Name: "col", TypeName: c.typ}}},
			}}
			if got := s.rawJSONColumn(table, "col"); got != c.want {
				t.Errorf("rawJSONColumn(%q) = %v, want %v", c.typ, got, c.want)
			}
		})
	}
}

// wireJSONRow simulates one row of a json or jsonb target column exactly as a
// real target pool hands it back through whichever destination type scanCell
// chooses (target.go's rawJSONColumn): wireText is what a *[]byte destination
// receives — the column's own bytes on the wire, byte for byte, whether that
// is jsonTextRows's raw source text for a scalar json or jsonb column, or
// (for an array, which no such wrapper covers) the Postgres array literal a
// *[]byte destination would otherwise be left holding; decoded is what pgx's
// own codec hands a *any destination instead — plain encoding/json.Unmarshal
// of a scalar document's text (T-0402's own bug: an integer past 2^53
// already rounded to a float64), or pgx's ArrayCodec's element-wise decode of
// an array's, which is a []any of natively-decoded elements and never wire
// text. Each test below sets only the field the shape under test is supposed
// to reach; production code choosing the other destination type gets the
// zero value of that field — an empty []byte, or a nil any — so a wrong
// choice fails loudly (a parse error, or an empty leaf set) rather than
// quietly returning a stand-in value that happens to look right.
type wireJSONRow struct {
	wireText string
	decoded  any
	scanned  bool
}

func (r *wireJSONRow) Next() bool {
	if r.scanned {
		return false
	}
	r.scanned = true
	return true
}

func (r *wireJSONRow) Scan(dest ...any) error {
	if len(dest) != 1 {
		return errors.New("wireJSONRow: one column")
	}
	switch p := dest[0].(type) {
	case *any:
		*p = r.decoded
		return nil
	case *[]byte:
		if r.wireText == "" {
			*p = nil
			return nil
		}
		*p = []byte(r.wireText)
		return nil
	}
	return errors.New("wireJSONRow: want *any or *[]byte")
}
func (r *wireJSONRow) Err() error { return nil }
func (r *wireJSONRow) Close()     {}

// oneWireJSONColumn is a target that answers scanColumn's one query with a
// single wireJSONRow, whichever destination type scanCell asks it to fill.
type oneWireJSONColumn struct{ row wireJSONRow }

func (c oneWireJSONColumn) Query(context.Context, string, ...any) (pipeline.Rows, error) {
	row := c.row
	return &row, nil
}

// TestScanColumnReadsAScalarJSONBColumnAsRawText drives scanColumn itself
// through rawJSONColumn (target.go, T-0402), and not leaves alone the way
// TestLeavesKeepsANumberLeafsExactDigitsPastFloat64Precision above does — a
// review round on this task found that test, and internal/transform's own
// TestANumberLeafPastFloat64PrecisionIsStillMasked, both pass a string
// straight into the function under test, which exercises decodeDocument's
// pre-existing text branch and pins nothing about which destination type
// scanColumn actually chose. Here wireJSONRow's decoded field holds exactly
// what pgx's own codec would have handed a *any destination for this wire
// text — a plain encoding/json.Unmarshal, carrying the same rounding T-0402
// fixed — so a rawJSONColumn that wrongly answered false for a scalar jsonb
// column would make scanColumn take that path instead, and this test would
// see the rounded spelling.
func TestScanColumnReadsAScalarJSONBColumnAsRawText(t *testing.T) {
	const pan19 = "4000123456789012343" // Luhn-valid, 19 digits: a float64 rounds this
	wire := `{"p1":` + pan19 + `}`

	var decoded any
	if err := json.Unmarshal([]byte(wire), &decoded); err != nil {
		t.Fatalf("json.Unmarshal(%q): %v", wire, err)
	}

	table := customers()
	s := &state{
		target: oneWireJSONColumn{row: wireJSONRow{wireText: wire, decoded: decoded}},
		tables: map[ref.TableRef]*pipeline.Table{
			table: {Ref: table, Columns: []pipeline.Column{{Name: "profile", TypeName: "jsonb"}}},
		},
	}

	var got any
	if err := s.scanColumn(context.Background(), table, "profile", func(v any) error {
		got = v
		return nil
	}); err != nil {
		t.Fatalf("scanColumn: %v", err)
	}

	var found bool
	for _, l := range leaves(got) {
		if l.path != "$.p1" {
			continue
		}
		found = true
		if l.text != pan19 {
			t.Errorf("leaves(scanColumn's value)[%q] = %q, want %q: rawJSONColumn let the *any path "+
				"round it", l.path, l.text, pan19)
		}
	}
	if !found {
		t.Fatalf("leaves(scanColumn's value) has no $.p1 leaf at all: %v", leaves(got))
	}
}

// TestScanColumnDoesNotTakeTheRawTextPathForAJSONBArrayColumn is the fix
// round after T-0402's first pass, a reviewer's finding: rawJSONColumn
// (target.go) must answer false for a json[] or jsonb[] column, because the
// wire text a *[]byte destination would receive for an array is a Postgres
// array literal and decodeDocument cannot parse that as JSON — every leaf of
// the array would be silently invisible to the residual scan, not merely
// float64-rounded. wireText here is set to exactly that shape (matching the
// reviewer's own reproduction) and decoded to what pgx's ArrayCodec actually
// hands a *any destination instead — a []any of natively-decoded elements —
// so a rawJSONColumn that wrongly answered true for an array column would
// make scanColumn take the wire-text path and this test would find no leaf
// at all.
func TestScanColumnDoesNotTakeTheRawTextPathForAJSONBArrayColumn(t *testing.T) {
	wire := `{"{\"email\": \"a@b.test\"}"}`
	decoded := []any{map[string]any{"email": "a@b.test"}}

	table := customers()
	s := &state{
		target: oneWireJSONColumn{row: wireJSONRow{wireText: wire, decoded: decoded}},
		tables: map[ref.TableRef]*pipeline.Table{
			table: {Ref: table, Columns: []pipeline.Column{{Name: "profiles", TypeName: "jsonb[]"}}},
		},
	}

	var got any
	if err := s.scanColumn(context.Background(), table, "profiles", func(v any) error {
		got = v
		return nil
	}); err != nil {
		t.Fatalf("scanColumn: %v", err)
	}

	var found bool
	for _, l := range leaves(got) {
		if l.path == "$[0].email" && l.str && l.text == "a@b.test" {
			found = true
		}
	}
	if !found {
		t.Errorf("leaves(scanColumn's value) = %v, want a hit at $[0].email: rawJSONColumn let the "+
			"raw-text path run on an array column, and decodeDocument cannot parse a Postgres array "+
			"literal", leaves(got))
	}
}
