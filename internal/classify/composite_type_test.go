// SPDX-License-Identifier: Apache-2.0

package classify

import (
	"strings"
	"testing"

	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
)

// A column of a composite type, and an array of a type pgx has no codec for.
// Both arrive from the source as one string, because the source pool runs in
// QueryExecModeExec and registers no user types (T-0076), and both were a
// cleartext copy before this: nothing in the masking chain covered a composite
// (T-0094) and no validator could see inside an extension-type array (T-0103).

var tComposite = ref.TableRef{Schema: "public", Name: "settlements"}

// compositeSchema is one table with the composite columns named, plus the
// CREATE TYPE lines internal/introspect reads out of the catalog. A composite
// is told from an ltree or a geometry by Schema.Composites and by nothing else.
func compositeSchema(cols ...pipeline.Column) *pipeline.Schema {
	return &pipeline.Schema{
		Composites: []pipeline.NamedDef{
			{Name: "public.money_amount", Def: "CREATE TYPE public.money_amount AS (amount numeric(12,2), currency text)"},
			{Name: "public.postal_address", Def: "CREATE TYPE public.postal_address AS (line1 text, city text, email text)"},
		},
		Tables:      []pipeline.Table{tt("public", "settlements", []string{"settlement_id"}, cols...)},
		Fingerprint: "composite",
	}
}

func classifyComposite(t *testing.T, s mapSampler, cols ...pipeline.Column) *pipeline.Classification {
	t.Helper()
	cls, err := New().Classify(compositeSchema(cols...), s, nil)
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	return cls
}

// TestCompositeWithPersonalDataFailsClosed is T-0094's decision: a composite
// whose fields carry personal data reaches `possible`, which is above the mask
// threshold, so internal/plan refuses the run rather than copying the record.
// It is never `none` and never a silent copy (THREAT_MODEL.md T1).
func TestCompositeWithPersonalDataFailsClosed(t *testing.T) {
	t.Parallel()

	col := tc("billing", "public.postal_address")
	s := mapSampler{}
	s[ref.ColumnRef{Table: tComposite, Column: "billing"}] = anyOf(
		`("12 Bridge Street",Manchester,bea.donnelly@example.test)`,
		`("4 Marsh Lane",Leeds,cai.osei@example.test)`,
		`("221b Baker Street",London,dee.abara@example.test)`,
	)
	cls := classifyComposite(t, s, tc("settlement_id", "bigint"), col)
	d := decision(t, cls, ref.ColumnRef{Table: tComposite, Column: "billing"})

	if !d.Masked {
		t.Fatalf("a composite holding email addresses is not masked: %+v", d)
	}
	if d.Confidence < pipeline.ConfPossible {
		t.Errorf("Confidence = %v, want possible or above", d.Confidence)
	}
	if d.Category != pipeline.CatEmail {
		t.Errorf("Category = %q, want %q: email is the first validator that hits a field",
			d.Category, pipeline.CatEmail)
	}
	if bad, ok := ParseReason(d.Reason); !ok {
		t.Errorf("reason %q does not parse: %q", d.Reason, bad)
	}
	if want := render("composite_refused"); !strings.Contains(d.Reason, want) {
		t.Errorf("reason = %q, want it to carry %q", d.Reason, want)
	}
}

// TestCompositeWithNoSignalIsCopiedAndSaysSo is the other half of T-0094's
// decision, and it is testdata/nasty.sql trap 27: money_amount is not personal
// data, is copied, and the line says its fields were read. A composite that
// were simply silent here would be the same green tick a leak has.
func TestCompositeWithNoSignalIsCopiedAndSaysSo(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"booked", "reversed"} {
		s := mapSampler{}
		s[ref.ColumnRef{Table: tComposite, Column: name}] = anyOf(
			`(1234.50,GBP)`, `(-99.99,USD)`, nil, `(99.99,USD)`,
		)
		cls := classifyComposite(t, s,
			tc("settlement_id", "bigint"), tc(name, "public.money_amount"))
		d := decision(t, cls, ref.ColumnRef{Table: tComposite, Column: name})

		if d.Masked {
			t.Errorf("%s is masked: %+v", name, d)
		}
		if d.Category != pipeline.CatNone {
			t.Errorf("%s Category = %q, want %q", name, d.Category, pipeline.CatNone)
		}
		if want := render("composite_no_signal"); !strings.Contains(d.Reason, want) {
			t.Errorf("%s reason = %q, want it to carry %q", name, d.Reason, want)
		}
		if bad, ok := ParseReason(d.Reason); !ok {
			t.Errorf("%s reason %q does not parse: %q", name, d.Reason, bad)
		}
	}
}

// TestCompositeAddressAcrossFieldsFailsClosed is the half of T-0094 that
// field-wise scoring alone missed: an address that exists only as the
// concatenation of the record's fields. AddressShape wants a digit and two
// words in one value, and the house number lives in a field of its own, so no
// field of `(9,"Rue de Rivoli",Paris)` validates and the column was decided
// `none` and copied — under a reason claiming its fields had been read and none
// was personal data. compositeSignal scores the whole literal as well.
func TestCompositeAddressAcrossFieldsFailsClosed(t *testing.T) {
	t.Parallel()

	s := mapSampler{}
	s[ref.ColumnRef{Table: tComposite, Column: "billing"}] = anyOf(
		`(221,"Baker Street",London)`,
		`(19,"Calle Mayor",Madrid)`,
		`(3,"Via Roma",Milano)`,
	)
	cls := classifyComposite(t, s,
		tc("settlement_id", "bigint"), tc("billing", "public.postal_address"))
	d := decision(t, cls, ref.ColumnRef{Table: tComposite, Column: "billing"})

	if !d.Masked {
		t.Fatalf("a composite whose fields spell an address is not masked: %+v", d)
	}
	if d.Category != pipeline.CatAddress {
		t.Errorf("Category = %q, want %q", d.Category, pipeline.CatAddress)
	}
	if want := render("composite_refused"); !strings.Contains(d.Reason, want) {
		t.Errorf("reason = %q, want it to carry %q", d.Reason, want)
	}
	if bad, ok := ParseReason(d.Reason); !ok {
		t.Errorf("reason %q does not parse: %q", d.Reason, bad)
	}
}

// TestCompositeHoldingADocumentFieldFailsClosedWithNoHit is T-0399 (the
// 2026-09-25 JSON red team, round 1, entry 14): a composite whose type holds a
// jsonb field carried an email inside that field's document, and
// compositeSignal never saw it. Not because of the record's own quoting --
// splitCompositeLiteral already unquotes the field and undoes its doubled
// quotes, so `row('a','{"k":"x@y.test"}')` reaches a validator as the bare
// text `{"k": "x@y.test"}` -- but because compositeSignal asks whether the
// *whole* field matches ValidEmail's shape, and a JSON document is never
// itself a bare address however deep one is nested inside it. The fix does
// not try to walk into the document: any composite whose type holds a json,
// jsonb or hstore field is refused on the type alone, the same as one
// compositeSignal did find a hit on -- so this must refuse even over a
// sample with no value hit at all, and even with no sample to read.
func TestCompositeHoldingADocumentFieldFailsClosedWithNoHit(t *testing.T) {
	t.Parallel()

	schema := &pipeline.Schema{
		Composites: []pipeline.NamedDef{
			{Name: "public.wrap", Def: "CREATE TYPE public.wrap AS (tag text, doc jsonb)"},
		},
		Tables: []pipeline.Table{tt("public", "widgets", []string{"id"},
			tc("id", "bigint"), tc("w", "public.wrap"), tc("w2", "public.wrap"))},
		Fingerprint: "wrap",
	}
	s := mapSampler{}
	tWidgets := ref.TableRef{Schema: "public", Name: "widgets"}
	// w: an ordinary field carries the email, which compositeSignal already
	// caught before this fix.
	s[ref.ColumnRef{Table: tWidgets, Column: "w"}] = anyOf(
		`(ana.fake@example.org,"{""k"": ""v""}")`)
	// w2: the email is only inside the jsonb field's own quoted text -- no
	// field of the record is itself a bare address -- so before this fix
	// compositeSignal found nothing and the column was copied.
	s[ref.ColumnRef{Table: tWidgets, Column: "w2"}] = anyOf(
		`(a,"{""k"": ""ana.fake@example.org""}")`)

	cls, err := New().Classify(schema, s, nil)
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	for _, col := range []string{"w", "w2"} {
		d := decision(t, cls, ref.ColumnRef{Table: tWidgets, Column: col})
		if !d.Masked {
			t.Errorf("%s: not masked: %+v", col, d)
		}
		if d.Confidence < pipeline.ConfPossible {
			t.Errorf("%s: Confidence = %v, want possible or above", col, d.Confidence)
		}
		if want := render("composite_document_field", quoteIdent("public.wrap"), "jsonb", quoteIdent("doc")); !strings.Contains(d.Reason, want) {
			t.Errorf("%s: reason = %q, want it to carry %q", col, d.Reason, want)
		}
		if bad, ok := ParseReason(d.Reason); !ok {
			t.Errorf("%s: reason %q does not parse: %q", col, d.Reason, bad)
		}
	}
}

// TestCompositeHoldingADocumentFieldFailsClosedWithNoSample is the same rule
// with nothing sampled at all: a composite structurally carrying a document
// field is refused whatever the table holds, never a "no samples" copy.
func TestCompositeHoldingADocumentFieldFailsClosedWithNoSample(t *testing.T) {
	t.Parallel()

	schema := &pipeline.Schema{
		Composites: []pipeline.NamedDef{
			{Name: "public.wrap", Def: "CREATE TYPE public.wrap AS (tag text, doc jsonb)"},
		},
		Tables: []pipeline.Table{tt("public", "widgets", []string{"id"},
			tc("id", "bigint"), tc("w", "public.wrap"))},
		Fingerprint: "wrap-empty",
	}
	cls, err := New().Classify(schema, mapSampler{}, nil)
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	d := decision(t, cls, ref.ColumnRef{Table: ref.TableRef{Schema: "public", Name: "widgets"}, Column: "w"})
	if !d.Masked {
		t.Fatalf("no samples: not masked: %+v", d)
	}
	if unwanted := render("composite_no_sample"); strings.Contains(d.Reason, unwanted) {
		t.Errorf("reason = %q claims %q for a structural refusal", d.Reason, unwanted)
	}
	if bad, ok := ParseReason(d.Reason); !ok {
		t.Errorf("reason %q does not parse: %q", d.Reason, bad)
	}
}

// TestCompositeHoldingANestedDocumentFieldFailsClosed: the field itself may be
// another composite that holds the document, one level down.
func TestCompositeHoldingANestedDocumentFieldFailsClosed(t *testing.T) {
	t.Parallel()

	schema := &pipeline.Schema{
		Composites: []pipeline.NamedDef{
			{Name: "public.wrap", Def: "CREATE TYPE public.wrap AS (tag text, doc jsonb)"},
			{Name: "public.outer_wrap", Def: "CREATE TYPE public.outer_wrap AS (label text, inner public.wrap)"},
		},
		Tables: []pipeline.Table{tt("public", "widgets", []string{"id"},
			tc("id", "bigint"), tc("o", "public.outer_wrap"))},
		Fingerprint: "wrap-nested",
	}
	cls, err := New().Classify(schema, mapSampler{}, nil)
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	d := decision(t, cls, ref.ColumnRef{Table: ref.TableRef{Schema: "public", Name: "widgets"}, Column: "o"})
	if !d.Masked {
		t.Fatalf("nested document field: not masked: %+v", d)
	}
	if want := render("composite_document_field", quoteIdent("public.wrap"), "jsonb", quoteIdent("doc")); !strings.Contains(d.Reason, want) {
		t.Errorf("reason = %q, want it to carry %q", d.Reason, want)
	}
	if bad, ok := ParseReason(d.Reason); !ok {
		t.Errorf("reason %q does not parse: %q", d.Reason, bad)
	}
}

// TestCompositeHoldingADomainOverAJSONBFieldFailsClosed is the fix-round
// finding on T-0399: the document field is not itself jsonb, but a domain
// declared `AS jsonb`. documentFieldWalk must resolve a field's type through
// Schema.Domains before the family check, the way typeOf already does for a
// column's own declared type, or a domain one level down defeats the whole
// structural refusal.
func TestCompositeHoldingADomainOverAJSONBFieldFailsClosed(t *testing.T) {
	t.Parallel()

	schema := &pipeline.Schema{
		Domains: []pipeline.NamedDef{
			{Name: "public.docdom", Def: "CREATE DOMAIN public.docdom AS jsonb"},
		},
		Composites: []pipeline.NamedDef{
			{Name: "public.domwrap", Def: "CREATE TYPE public.domwrap AS (tag text, doc public.docdom)"},
		},
		Tables: []pipeline.Table{tt("public", "widgets", []string{"id"},
			tc("id", "bigint"), tc("w", "public.domwrap"))},
		Fingerprint: "domain-wrap",
	}
	cls, err := New().Classify(schema, mapSampler{}, nil)
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	d := decision(t, cls, ref.ColumnRef{Table: ref.TableRef{Schema: "public", Name: "widgets"}, Column: "w"})
	if !d.Masked {
		t.Fatalf("domain over jsonb field: not masked: %+v", d)
	}
	if want := render("composite_document_field", quoteIdent("public.domwrap"), "jsonb", quoteIdent("doc")); !strings.Contains(d.Reason, want) {
		t.Errorf("reason = %q, want it to carry %q", d.Reason, want)
	}
}

// TestCompositeHoldingADomainOverANestedCompositeFailsClosed: the field
// holding the document is not a composite directly, but a domain over one.
// documentFieldWalk must resolve the domain before asking whether the
// resolved name is itself a composite to walk into.
func TestCompositeHoldingADomainOverANestedCompositeFailsClosed(t *testing.T) {
	t.Parallel()

	schema := &pipeline.Schema{
		Domains: []pipeline.NamedDef{
			{Name: "public.wrapdom", Def: "CREATE DOMAIN public.wrapdom AS public.wrap"},
		},
		Composites: []pipeline.NamedDef{
			{Name: "public.wrap", Def: "CREATE TYPE public.wrap AS (tag text, doc jsonb)"},
			{Name: "public.outer_wrap", Def: "CREATE TYPE public.outer_wrap AS (label text, inner public.wrapdom)"},
		},
		Tables: []pipeline.Table{tt("public", "widgets", []string{"id"},
			tc("id", "bigint"), tc("o", "public.outer_wrap"))},
		Fingerprint: "domain-wrap-nested",
	}
	cls, err := New().Classify(schema, mapSampler{}, nil)
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	d := decision(t, cls, ref.ColumnRef{Table: ref.TableRef{Schema: "public", Name: "widgets"}, Column: "o"})
	if !d.Masked {
		t.Fatalf("domain over nested composite: not masked: %+v", d)
	}
	if want := render("composite_document_field", quoteIdent("public.wrap"), "jsonb", quoteIdent("doc")); !strings.Contains(d.Reason, want) {
		t.Errorf("reason = %q, want it to carry %q", d.Reason, want)
	}
}

// TestCompositeWithNoDocumentFieldIsUnaffected: money_amount and
// postal_address, T-0094's own fixtures, hold no json/jsonb/hstore field, so
// TestCompositeWithNoSignalIsCopiedAndSaysSo above must still pass -- this
// pins compositeDocumentField itself returning false for them, so a
// regression there fails here directly rather than only through that test's
// unrelated assertions.
func TestCompositeWithNoDocumentFieldIsUnaffected(t *testing.T) {
	t.Parallel()

	schema := compositeSchema()
	if _, _, _, ok := compositeDocumentField(schema, "public.money_amount"); ok {
		t.Errorf("money_amount has no document field")
	}
	if _, _, _, ok := compositeDocumentField(schema, "public.postal_address"); ok {
		t.Errorf("postal_address has no document field")
	}
}

// TestCompositeWithNoSampleSaysSo: the copy reason must not claim a check that
// did not run. A composite in a table nothing could be read from takes the same
// copy branch, and "its fields were read and none is personal data; no samples"
// is a line that contradicts itself on precisely the branch that leaks.
func TestCompositeWithNoSampleSaysSo(t *testing.T) {
	t.Parallel()

	cls := classifyComposite(t, mapSampler{},
		tc("settlement_id", "bigint"), tc("booked", "public.money_amount"))
	d := decision(t, cls, ref.ColumnRef{Table: tComposite, Column: "booked"})

	if d.Masked {
		t.Errorf("booked is masked: %+v", d)
	}
	if want := render("composite_no_sample"); !strings.Contains(d.Reason, want) {
		t.Errorf("reason = %q, want it to carry %q", d.Reason, want)
	}
	if unwanted := render("composite_no_signal"); strings.Contains(d.Reason, unwanted) {
		t.Errorf("reason = %q claims %q with no value read", d.Reason, unwanted)
	}
	if bad, ok := ParseReason(d.Reason); !ok {
		t.Errorf("reason %q does not parse: %q", d.Reason, bad)
	}
}

// TestCompositeNameHitReachesTheThreshold: seven of supabase-auth's ten misses
// were columns with no rows at all, and a composite is the same case. A name
// that says personal data is enough on its own here, because the alternative is
// the `low` that a type no category accepts used to produce — copied verbatim.
func TestCompositeNameHitReachesTheThreshold(t *testing.T) {
	t.Parallel()

	cls := classifyComposite(t, mapSampler{},
		tc("settlement_id", "bigint"), tc("home_address", "public.postal_address"))
	d := decision(t, cls, ref.ColumnRef{Table: tComposite, Column: "home_address"})
	if !d.Masked {
		t.Fatalf("a composite named home_address is not masked: %+v", d)
	}
	if d.Category != pipeline.CatAddress {
		t.Errorf("Category = %q, want %q", d.Category, pipeline.CatAddress)
	}
	if bad, ok := ParseReason(d.Reason); !ok {
		t.Errorf("reason %q does not parse: %q", d.Reason, bad)
	}
}

// TestExtensionTypeArrayOfAddressesIsSeen is T-0103: a citext[] of email
// addresses arrives as the single string "{a@b.test,c@d.test}" because pgx has
// no codec for the array of an extension type, and before the array-literal
// splitter no validator matched it and the column was decided `none` and
// copied. Plausible's monthly_reports.recipients is this column.
func TestExtensionTypeArrayOfAddressesIsSeen(t *testing.T) {
	t.Parallel()

	tbl := ref.TableRef{Schema: "public", Name: "monthly_reports"}
	c := ref.ColumnRef{Table: tbl, Column: "recipients"}
	s := mapSampler{}
	s[c] = anyOf(
		`{bea.donnelly@example.test,cai.osei@example.test}`,
		`{"dee.abara@example.test"}`,
		`{eli.novak@example.test,NULL,fay.mbeki@example.test}`,
	)
	cls, err := New().Classify(&pipeline.Schema{
		Tables: []pipeline.Table{tt("public", "monthly_reports", []string{"id"},
			tc("id", "bigint"),
			tc("recipients", "extensions.citext[]"),
		)},
		Fingerprint: "citext-array",
	}, s, nil)
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}

	d := decision(t, cls, c)
	if !d.Masked {
		t.Fatalf("a citext[] of email addresses is not masked: %+v", d)
	}
	if d.Category != pipeline.CatEmail {
		t.Errorf("Category = %q, want %q", d.Category, pipeline.CatEmail)
	}
	if bad, ok := ParseReason(d.Reason); !ok {
		t.Errorf("reason %q does not parse: %q", d.Reason, bad)
	}
}

// TestArrayLiteralSplitter holds the reading of the server's own output forms.
// It is a reader and not a parser: what it cannot read falls back to one opaque
// value, which is the behaviour before it existed.
func TestArrayLiteralSplitter(t *testing.T) {
	t.Parallel()

	cases := []struct {
		in   string
		want []string
		ok   bool
	}{
		{`{a@b.test,c@d.test}`, []string{"a@b.test", "c@d.test"}, true},
		{`{}`, nil, true},
		{`{NULL,a}`, []string{"a"}, true},
		{`{"NULL"}`, []string{"NULL"}, true},
		{`{"a,b","c\"d"}`, []string{"a,b", `c"d`}, true},
		{`{{a,b},{c,d}}`, []string{"a", "b", "c", "d"}, true},
		{`[0:1]={a,b}`, []string{"a", "b"}, true},
		{`(1,2)`, nil, false},
		{`not an array`, nil, false},
		{`{unclosed`, nil, false},
		{`{"unclosed}`, nil, false},
	}
	for _, c := range cases {
		got, ok := splitArrayLiteral(c.in)
		if ok != c.ok {
			t.Errorf("splitArrayLiteral(%q) ok = %v, want %v", c.in, ok, c.ok)
			continue
		}
		if !ok {
			continue
		}
		if len(got) != len(c.want) {
			t.Errorf("splitArrayLiteral(%q) = %q, want %q", c.in, got, c.want)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("splitArrayLiteral(%q) = %q, want %q", c.in, got, c.want)
				break
			}
		}
	}
}

// TestCompositeLiteralSplitter is the record form: an empty field is NULL, a
// doubled quote is a quote, and a field that carries a comma is quoted.
func TestCompositeLiteralSplitter(t *testing.T) {
	t.Parallel()

	cases := []struct {
		in   string
		want []string
		ok   bool
	}{
		{`(1234.50,GBP)`, []string{"1234.50", "GBP"}, true},
		{`(,GBP)`, []string{"GBP"}, true},
		{`("12 Bridge Street, Flat 2",Manchester)`, []string{"12 Bridge Street, Flat 2", "Manchester"}, true},
		{`("he said ""hi""")`, []string{`he said "hi"`}, true},
		{`{a,b}`, nil, false},
		{`(unclosed`, nil, false},
	}
	for _, c := range cases {
		got, ok := splitCompositeLiteral(c.in)
		if ok != c.ok {
			t.Errorf("splitCompositeLiteral(%q) ok = %v, want %v", c.in, ok, c.ok)
			continue
		}
		if !ok {
			continue
		}
		if len(got) != len(c.want) {
			t.Errorf("splitCompositeLiteral(%q) = %q, want %q", c.in, got, c.want)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("splitCompositeLiteral(%q) = %q, want %q", c.in, got, c.want)
				break
			}
		}
	}
}
