// SPDX-License-Identifier: Apache-2.0

package verify

import (
	"context"
	"errors"
	"reflect"
	"sort"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/Liarea/lazyslice/internal/pg"
	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
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

// noResidual is a filter that answers no to everything.
type noResidual struct{}

func (noResidual) Add(ref.ColumnRef, string, []byte)             {}
func (noResidual) MayContain(ref.ColumnRef, string, []byte) bool { return false }
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
	p, ok := dest[0].(*any)
	if !ok {
		return errors.New("fakeRows: want *any")
	}
	*p = r.vals[r.i-1]
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
// the table, the column and the category. And the same 0.5 ratio over
// minValues values does not fail, because there the ratio is the answer and
// this net is not a per-row scanner.
func TestAColumnBelowMinValuesFailsOnAnyHit(t *testing.T) {
	table := customers()
	col := ref.ColumnRef{Table: table, Column: "note"}

	cases := []struct {
		name     string
		vals     []any
		wantFail bool
	}{
		{
			name:     "two values, one an email address",
			vals:     []any{"ada.lovelace@fixture.test", "nothing to see"},
			wantFail: true,
		},
		{
			name:     "one value, an email address",
			vals:     []any{"ada.lovelace@fixture.test", nil},
			wantFail: true,
		},
		{
			name:     "two values, neither an email address",
			vals:     []any{"nothing to see", "still nothing"},
			wantFail: false,
		},
		{
			// minValues values at the same 0.5 ratio: the threshold is what
			// decides once there are enough values for a ratio to mean
			// anything, and half of four is below it.
			name:     "four values, two email addresses",
			vals:     []any{"ada.lovelace@fixture.test", "grace.hopper@fixture.test", "nothing to see", "still nothing"},
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
		})
	}
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
