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
