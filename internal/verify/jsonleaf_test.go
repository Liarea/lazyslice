// SPDX-License-Identifier: Apache-2.0

package verify

import (
	"context"
	"testing"

	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
)

// pinnedLeafValues is internal/transform/leaf_test.go's table, row for row:
// the two restatements of leafValueCategory have to answer every row the same
// way (jsonleaf.go). No row imitates a real provider's secret format.
var pinnedLeafValues = []struct {
	in   string
	want pipeline.Category
}{
	{"ada.lovelace@example.org", pipeline.CatEmail},
	{"+44 20 7946 0958", pipeline.CatPhone},
	{"4111111111111111", pipeline.CatFinancial},
	{"GB82WEST12345698765432", pipeline.CatFinancial},
	{"203.0.113.7", pipeline.CatNetworkID},
	{"00:1a:2b:3c:4d:5e", pipeline.CatNetworkID},
	{"123-45-6789", pipeline.CatNationalID},
	{"123456789", pipeline.CatNationalID},
	{"https://api.example.test/v1/chat", pipeline.CatOnlineID},
	{"Qx7vR2mK9pL4tZ8wN3bH6cJ1yF5dS0aE", pipeline.CatCredential},
	{"221 Baker Street", pipeline.CatAddress},
	{"diagnosed with schizophrenia last spring", pipeline.CatSpecial},
	// The JSON red team's round 1 spellings (entries 16 and 21, T-0403): a
	// known shape in another code point or a reversible encoding, read through
	// textsig.Candidates on both sides.
	{"ana.fake\uff20example.org", pipeline.CatEmail},
	{"\uff0b\uff14\uff14 \uff12\uff10 \uff17\uff19\uff14\uff16 \uff10\uff19\uff15\uff18", pipeline.CatPhone},
	{"4111\u20131111\u20131111\u20131111", pipeline.CatFinancial},
	{"4111_1111_1111_1111", pipeline.CatFinancial},
	{"4111\u200b1111\u200b1111\u200b1111", pipeline.CatFinancial},
	{"ANA.FAKE@EXAMPLE.ORG.", pipeline.CatEmail},
	{`ana.fake\u0040example.org`, pipeline.CatEmail},
	{"mailto:ana.fake@example.org", pipeline.CatEmail},
	{"ana.fake%40example.org", pipeline.CatEmail},
	{"%2B442079460958", pipeline.CatPhone},
	{"YW5hLmZha2VAZXhhbXBsZS5vcmc=", pipeline.CatEmail},
	{"078\u201305\u20131120", pipeline.CatNationalID},
	{"", ""},
	{"dark", ""},
	{"en-GB", ""},
	{"gpt-4o", ""},
	{"4096", ""},
	{"0.7", ""},
	{"deploy on merge to main", ""},
	{"2026-09-24", ""},
	{"us-east-1", ""},
	{"Europe/London", ""},
	{"ad-slot-728x90", ""},
	{"2026/09/24", ""},
	{"24/09/2026", ""},
}

// pinnedRegionLeafValues is the phone question under a configured region
// (--phone-region, T-0221), pinned the same way in both packages: the net
// reads a leaf's number under the region, so the rule must too, or a copied
// national-format number is a refusal on a correct run (T-0272 review round,
// finding 2). Invented numbers in the ranges each country reserves for drama.
var pinnedRegionLeafValues = []struct {
	in, region string
	want       pipeline.Category
}{
	{"020 7946 0958", "GB", pipeline.CatPhone},
	{"020 7946 0958", "", ""},
	{"020 7946 0958", "US", ""},
	{"(415) 555-2671", "US", pipeline.CatPhone},
	{"(415) 555-2671", "", ""},
	{"+44 20 7946 0958", "US", pipeline.CatPhone},
	{"dark", "GB", ""},
}

func TestLeafValueCategoryIsPinned(t *testing.T) {
	for _, c := range pinnedLeafValues {
		got, ok := leafValueCategory(c.in, "")
		if ok != (c.want != "") || got != c.want {
			t.Errorf("leafValueCategory(%q) = %q, %v; want %q", c.in, got, ok, c.want)
		}
	}
	for _, c := range pinnedRegionLeafValues {
		got, ok := leafValueCategory(c.in, c.region)
		if ok != (c.want != "") || got != c.want {
			t.Errorf("leafValueCategory(%q, region %q) = %q, %v; want %q", c.in, c.region, got, ok, c.want)
		}
	}
}

// The net's skip is safe only while every validator the net runs over a leaf
// is also a question leafValueCategory asks: a leaf the rule calls copied must
// be a leaf no net validator recognises (jsonleaf.go, "Why that skip cannot
// hide a copied source value"). The net is asked through its own count, under
// each region --phone-region could name, because count also reads a phone
// number under the configured region (T-0221): a region the rule did not read
// was a national-format number copied by transform and refused here at exit 9
// (T-0272 review round, finding 2). A validator added to validators.go and not
// to leafValueCategory — in both packages — fails here on the corpus.
func TestLeafValueCategoryCoversTheNetsLeafValidators(t *testing.T) {
	corpus := []string{
		"078-05-1120", "AB123456C", "https://example.com/u/ada", "aa:bb:cc:dd:ee:ff",
		"2001:db8::1", "5500 0000 0000 0004", "DE89370400440532013000",
		"10 Downing Street", "Flat 4, 12 Mill Lane", "Ahmadiyya Muslim", "HIV positive, CD4 210",
		"+1 415 555 0132", "grace.hopper@example.com", "Zb4nQ8rT1vW6yK3mP7sD2fH9",
		"020 7946 0958", "07700 900123", "(415) 555-2671", "0412 345 678",
	}
	for _, c := range pinnedLeafValues {
		corpus = append(corpus, c.in)
	}
	for _, c := range pinnedRegionLeafValues {
		corpus = append(corpus, c.in)
	}
	leafMode := netMode{leaves: true, text: true}
	for _, region := range []string{"", "GB", "US", "AU"} {
		s := &state{opts: Options{PhoneRegion: region}}
		for _, text := range corpus {
			if text == "" {
				continue
			}
			hits := make([]int64, len(validators))
			s.count(text, true, leafMode, hits, nil)
			for i, n := range hits {
				if n == 0 {
					continue
				}
				if _, ok := leafValueCategory(text, region); !ok {
					t.Errorf("under region %q the net's %s validator recognises %q at a leaf and leafValueCategory does not: "+
						"transform would copy it and the net would skip nothing, so a correct run refuses", region, validators[i].name, text)
				}
			}
		}
	}
}

// The second net over a masked document with a per-leaf map (T-0272) leaves a
// leaf a category masker replaced to the residual scan — a fake address is
// still an address — and still reads every other leaf. Without a map, every
// leaf was free_text filler, and an email among them is a masker that failed
// open, which the net refuses exactly as before.
func TestTheSecondNetLeavesACategoryMaskedLeafToTheResidualScan(t *testing.T) {
	table := customers()
	col := ref.ColumnRef{Table: table, Column: "profile"}
	keys := map[string]pipeline.Category{
		"owner": pipeline.CatNone, "email": pipeline.CatEmail,
		"notify": pipeline.CatNone, "theme": pipeline.CatNone,
	}
	// What transform writes: the email masker's output at owner.email (its key
	// names the category) and at notify (its value did), and the copied theme.
	vals := []any{
		`{"owner":{"email":"glen.manning@example.com"},"notify":"isaac.murillo@example.net","theme":"dark"}`,
		`{"owner":{"email":"andre.valentine@example.net"},"notify":"sheila.perkins@example.net","theme":"light"}`,
	}
	cases := []struct {
		name     string
		keys     map[string]pipeline.Category
		vals     []any
		wantFail string
		// cat and src are the column's own decision, semi_structured by the
		// classifier when empty.
		cat pipeline.Category
		src pipeline.DecisionSource
	}{
		{name: "a masked document with a map passes on the masker's own output", keys: keys, vals: vals},
		{name: "the same target with no map is a masker that failed open", keys: nil, vals: vals, wantFail: "email"},
		{
			// T-0272 review round, finding 1: a column whose own name the
			// rule pack scores special_category had every leaf masked with
			// filler by transform (pipeline.Decision.LeafMap), so the map it
			// still carries says nothing here, and an address is a leak.
			name: "a special_category column's map is not read", keys: keys, vals: vals,
			cat: pipeline.CatSpecial, wantFail: "email",
		},
		{
			name: "a yml-raised column's map is not read", keys: keys, vals: vals,
			src: pipeline.ByYmlRaise, wantFail: "email",
		},
		{
			// A leaf whose key names free_text got filler, so the net still
			// reads it: an address there is a masker that failed open, and
			// the rule, which reads the key first, does not call it the
			// email masker's output.
			name:     "a free_text leaf is still read",
			keys:     map[string]pipeline.Category{"notes": pipeline.CatFreeText, "theme": pipeline.CatNone},
			vals:     []any{`{"notes":"ada.lovelace@fixture.test","theme":"dark"}`},
			wantFail: "email",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cat := c.cat
			if cat == "" {
				cat = pipeline.CatSemiStruct
			}
			dec := pipeline.Decision{
				Col: col, Category: cat, Masked: true,
				Source: c.src, LeafKeys: c.keys,
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
					t.Fatalf("the net failed %s as %q on the masker's own output", col, s.failures[0].Reason)
				}
				return
			}
			if len(s.failures) != 1 || s.failures[0].Reason != c.wantFail {
				t.Fatalf("the net recorded %d failures, want one naming %s", len(s.failures), c.wantFail)
			}
		})
	}
}

// replacedByCategoryMasker reads the target's own key chain, which leaves()
// now carries per leaf.
func TestALeafCarriesItsKeyChain(t *testing.T) {
	ls := leaves(`{"a":{"b":[{"c":"x"}]},"d":"y"}`)
	want := map[string][]string{"$.a.b[0].c": {"a", "b", "c"}, "$.d": {"d"}}
	if len(ls) != len(want) {
		t.Fatalf("leaves = %+v, want %d", ls, len(want))
	}
	for _, l := range ls {
		w := want[l.path]
		if len(w) != len(l.keys) {
			t.Errorf("%s keys = %v, want %v", l.path, l.keys, w)
			continue
		}
		for i := range w {
			if w[i] != l.keys[i] {
				t.Errorf("%s keys = %v, want %v", l.path, l.keys, w)
			}
		}
	}
}

// generatedNet runs the second net over one table holding a masked jsonb
// document and generated text columns over it, row by row through the
// multi-column fake target (explain_test.go's fakeWorld), because the net
// now reads a generated value beside its own row's document (T-0397). rows
// hold the document first and each generated column's value after it, in
// gen's order. It returns each generated column's failure reasons.
func generatedNet(t *testing.T, doc string, dec pipeline.Decision, gen [][2]string, rows [][]any) map[string][]string {
	t.Helper()
	table := customers()
	c := func(n string) ref.ColumnRef { return ref.ColumnRef{Table: table, Column: n} }
	cols := []pipeline.Column{{Name: doc, TypeName: "jsonb"}}
	decisions := map[ref.ColumnRef]pipeline.Decision{}
	dec.Col = c(doc)
	decisions[c(doc)] = dec
	for _, g := range gen {
		cols = append(cols, pipeline.Column{Name: g[0], TypeName: "text", Generated: g[1]})
		decisions[c(g[0])] = pipeline.Decision{Col: c(g[0]), Category: pipeline.CatNone}
	}
	w := newWorld(&fakeTable{ref: table, cols: cols, target: rows})
	s := &state{
		schema: &pipeline.Schema{},
		target: targetSide{w},
		steps:  []pipeline.Step{{Table: table, Mode: pipeline.ChildOK}},
		tables: map[ref.TableRef]*pipeline.Table{table: {Ref: table, Columns: cols}},
		cls:    &pipeline.Classification{Decisions: decisions},
	}
	if err := s.secondNet(context.Background()); err != nil {
		t.Fatalf("secondNet: %v", err)
	}
	got := map[string][]string{}
	for _, f := range s.failures {
		got[f.Column] = append(got[f.Column], f.Reason)
	}
	return got
}

// Supabase's auth.identities.email is `lower((identity_data ->> 'email'))`
// (testdata/torture/supabase-auth). With per-leaf categories the leaf it reads
// is replaced by the email masker, so the generated column holds the masker's
// addresses, each equal to its own row's masked leaf; the net leaves those
// values' email hits to the residual scan and runs every other validator.
// Without a map the leaf was filler, and an address there is refused as
// before. Since T-0397 the skip is per value: an address that is not its own
// row's masked leaf -- another row's, or one assembled from copied leaves --
// is refused.
func TestAGeneratedColumnOverAMaskedDocumentsLeafIsTheMaskersOutput(t *testing.T) {
	const supabase = `lower((identity_data ->> 'email'::text))`
	gen := [][2]string{{"email", supabase}}
	emailKey := map[string]pipeline.Category{"email": pipeline.CatEmail, "kind": pipeline.CatNone}
	fakes := []string{"Glen.Manning@example.com", "isaac.murillo@example.net", "sheila.perkins@example.net"}
	doc := func(email string) string { return `{"email":"` + email + `","kind":"standard"}` }
	own := func(vals []string) [][]any {
		var rows [][]any
		for i, f := range fakes {
			rows = append(rows, []any{doc(f), vals[i]})
		}
		return rows
	}
	lowered := []string{"glen.manning@example.com", "isaac.murillo@example.net", "sheila.perkins@example.net"}
	for _, tc := range []struct {
		name     string
		keys     map[string]pipeline.Category
		rows     [][]any
		wantFail string
	}{
		{"the email masker's output passes", emailKey, own(lowered), ""},
		{"with no map the same addresses are refused", nil, own(lowered), "email"},
		{
			"a validator outside the leaf maskers' categories still runs",
			emailKey,
			own([]string{"diagnosed with schizophrenia", "HIV positive, CD4 210", "Ahmadiyya Muslim"}),
			"special_category",
		},
		{
			// The value is a masked leaf, but another row's: the skip reads
			// the row the value is in.
			"another row's masked leaf is refused",
			emailKey,
			own([]string{"isaac.murillo@example.net", "sheila.perkins@example.net", "glen.manning@example.com"}),
			"email",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dec := pipeline.Decision{Category: pipeline.CatSemiStruct, Masked: true, LeafKeys: tc.keys}
			got := generatedNet(t, "identity_data", dec, gen, tc.rows)["email"]
			if tc.wantFail == "" && len(got) != 0 {
				t.Errorf("email failures %v, want none: the column holds the email masker's output", got)
			}
			if tc.wantFail != "" && (len(got) != 1 || got[0] != tc.wantFail) {
				t.Errorf("email failures %v, want one %s", got, tc.wantFail)
			}
		})
	}
}

// T-0398: a log-shaped document is never "categorised" for the
// generated-column skip, whatever LeafKeys or NameHit say -- internal/
// transform's maskDocument collapses such a column to {} ahead of leafRule,
// so nothing under it was ever replaced through a category's own masker, and
// a generated column that reads it must be judged like any other unmasked
// value. LeafKeys is set here anyway (internal/classify computes it without
// asking whether the table is log-shaped) to prove the skip reads
// Decision.LogShaped and not merely an absent map.
func TestALogShapedDocumentIsNeverCategorisedForTheGeneratedColumnSkip(t *testing.T) {
	gen := [][2]string{{"email", `lower((identity_data ->> 'email'::text))`}}
	emailKey := map[string]pipeline.Category{"email": pipeline.CatEmail, "kind": pipeline.CatNone}
	doc := `{"email":"glen.manning@example.com","kind":"standard"}`
	dec := pipeline.Decision{
		Category: pipeline.CatSemiStruct, Masked: true, LeafKeys: emailKey, LogShaped: true,
	}
	got := generatedNet(t, "identity_data", dec, gen, [][]any{{doc, "glen.manning@example.com"}})["email"]
	if len(got) != 1 || got[0] != "email" {
		t.Errorf("email failures %v, want one email: LogShaped must stop the skip from reading it as replaced", got)
	}
}

// T-0397 (the 2026-09-25 JSON red team, round 1, entry 24): generated columns
// that assemble an email, a phone number and a card number out of copied
// leaves of a masked jsonb, beside the Supabase column over the email leaf.
// The first version of the rail skipped the email, phone and card validators
// for every such column, whichever leaf it read, and all three crossed at
// exit 0; each is refused now, and the Supabase column still passes. The
// values are what the red team's own run left in the target: the documents
// hold the email masker's output under `email` and the copied u, h, cc, nsn,
// bin and tail.
func TestAGeneratedColumnAssembledFromCopiedLeavesIsRefused(t *testing.T) {
	gen := [][2]string{
		{"email", `lower((identity_data ->> 'email'::text))`},
		{"joined", `(((identity_data ->> 'u'::text) || '@'::text) || (identity_data ->> 'h'::text))`},
		{"dial", `(('+'::text || (identity_data ->> 'cc'::text)) || (identity_data ->> 'nsn'::text))`},
		{"pan", `((identity_data ->> 'bin'::text) || (identity_data ->> 'tail'::text))`},
	}
	keys := map[string]pipeline.Category{
		"email": pipeline.CatEmail, "u": pipeline.CatNone, "h": pipeline.CatNone, "cc": pipeline.CatNone,
		"nsn": pipeline.CatNone, "bin": pipeline.CatNone, "tail": pipeline.CatNone,
	}
	fakes := []string{"celia.camacho@example.com", "isaac.murillo@example.net", "sheila.perkins@example.net"}
	var rows [][]any
	for i, f := range fakes {
		n := string(rune('1' + i))
		rows = append(rows, []any{
			`{"email":"` + f + `","u":"quillon.varda` + n + `","h":"fictionmail.example","cc":"44","nsn":"207946000` + n +
				`","bin":"411111","tail":"1111111111"}`,
			f, "quillon.varda" + n + "@fictionmail.example", "+44207946000" + n, "4111111111111111",
		})
	}
	dec := pipeline.Decision{Category: pipeline.CatSemiStruct, Masked: true, LeafKeys: keys}
	got := generatedNet(t, "identity_data", dec, gen, rows)
	if len(got["email"]) != 0 {
		t.Errorf("email failures %v, want none: it holds the email masker's output", got["email"])
	}
	for col, want := range map[string]string{"joined": "email", "dial": "phone"} {
		if len(got[col]) != 1 || got[col][0] != want {
			t.Errorf("%s failures %v, want one %s: its value was assembled from copied leaves", col, got[col], want)
		}
	}
	// A sixteen-digit run is also a bare-hex hardware address to the net,
	// which asks network_id first; either way the column is refused.
	if len(got["pan"]) != 1 {
		t.Errorf("pan failures %v, want one: its value is a Luhn-valid card number assembled from copied leaves", got["pan"])
	}
}

// T-0393: the second net agrees with transform about a document column whose
// own name matched a personal rule its type did not accept (a jsonb `emails`,
// `full_name`, `passwords`). transform masks every leaf of such a column
// under the name's category through leafMasker, whatever the per-leaf map
// says, so the rule here does too: a leaf is never called copied, and a leaf
// the name's own masker replaced (an email, a phone, an address, a national
// id, a credential) is the masker's output and left to the residual scan,
// while a leaf of a name whose leafMasker is free_text (person_name,
// person_date, free_text) is filler and still read.
var nameHitColumns = []struct {
	column string
	cat    pipeline.Category
	// fake is what transform's leaf masker for cat writes, or, where that
	// masker is free_text, an address standing in for one that failed open.
	fake string
	// wantFail is the net's verdict on a target holding fake.
	wantFail string
}{
	{"full_name", pipeline.CatPersonName, "wren.calloway@example.test", "email"},
	{"home_address", pipeline.CatAddress, "221 Baker Street", ""},
	{"passwords", pipeline.CatCredential, "Qx7vR2mK9pL4tZ8wN3bH6cJ1yF5dS0aE", ""},
	{"date_of_birth", pipeline.CatPersonDate, "wren.calloway@example.test", "email"},
	{"national_id", pipeline.CatNationalID, "123-45-6789", ""},
	{"emails", pipeline.CatEmail, "glen.manning@example.com", ""},
	{"notes", pipeline.CatFreeText, "wren.calloway@example.test", "email"},
	{"by_phone", pipeline.CatPhone, "+44 20 7946 0958", ""},
}

func TestADocumentWhoseOwnNameIsPersonalHasNoCopiedLeaf(t *testing.T) {
	keys := map[string]pipeline.Category{"given": pipeline.CatNone, "family": pipeline.CatNone, "d": pipeline.CatNone}
	for _, c := range nameHitColumns {
		t.Run(c.column, func(t *testing.T) {
			d := pipeline.Decision{Category: pipeline.CatSemiStruct, Masked: true, NameHit: c.cat, LeafKeys: keys}
			p := policyOf(d, "")
			for _, l := range []struct{ key, text string }{{"given", "Wren"}, {"family", "Calloway"}, {"d", "1987-03-14"}} {
				v := leafRule(p, []string{l.key}, l.text, true)
				if v.copy || v.cat != leafMasker(c.cat) {
					t.Errorf("leafRule(%s) = %+v, want masked under %s as transform masks it", l.key, v, leafMasker(c.cat))
				}
			}
		})
	}
}

func TestTheSecondNetReadsANameHitDocumentAsTransformMaskedIt(t *testing.T) {
	for _, c := range nameHitColumns {
		for _, keys := range []map[string]pipeline.Category{nil, {"owner": pipeline.CatNone}} {
			name := c.column + "/with a map"
			if keys == nil {
				name = c.column + "/with no map"
			}
			t.Run(name, func(t *testing.T) {
				table := customers()
				col := ref.ColumnRef{Table: table, Column: c.column}
				dec := pipeline.Decision{Col: col, Category: pipeline.CatSemiStruct, Masked: true, NameHit: c.cat, LeafKeys: keys}
				s := &state{
					schema: &pipeline.Schema{},
					target: oneColumn{vals: []any{`{"owner":"` + c.fake + `"}`}},
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
						t.Fatalf("the net failed %s as %q on the %s masker's own output", col, s.failures[0].Reason, c.cat)
					}
					return
				}
				if len(s.failures) != 1 || s.failures[0].Reason != c.wantFail {
					t.Fatalf("the net recorded %d failures, want one naming %s: a %s leaf is filler, so an address there is a leak",
						len(s.failures), c.wantFail, c.cat)
				}
			})
		}
	}

	// A raised document keeps free_text for every leaf, so the net reads it
	// whatever the column is called.
	t.Run("a raised emails column is still read", func(t *testing.T) {
		table := customers()
		col := ref.ColumnRef{Table: table, Column: "emails"}
		dec := pipeline.Decision{Col: col, Category: pipeline.CatSemiStruct, Masked: true, Source: pipeline.ByYmlRaise, NameHit: pipeline.CatEmail}
		s := &state{
			schema: &pipeline.Schema{},
			target: oneColumn{vals: []any{`{"owner":"glen.manning@example.com"}`}},
			steps:  []pipeline.Step{{Table: table, Mode: pipeline.ChildOK}},
			tables: map[ref.TableRef]*pipeline.Table{
				table: {Ref: table, Columns: []pipeline.Column{{Name: col.Column, TypeName: "jsonb"}}},
			},
			cls: &pipeline.Classification{Decisions: map[ref.ColumnRef]pipeline.Decision{col: dec}},
		}
		if err := s.secondNet(context.Background()); err != nil {
			t.Fatalf("secondNet: %v", err)
		}
		if len(s.failures) != 1 || s.failures[0].Reason != "email" {
			t.Fatalf("the net recorded %d failures, want one naming email", len(s.failures))
		}
	})
}

// A generated column over a name-hit document's leaf holds that name's
// masker's output, as one over a per-leaf map's leaf does (ADR-015's rail,
// generatedFromMaskedLeaves).
func TestAGeneratedColumnOverANameHitDocumentsLeafIsTheMaskersOutput(t *testing.T) {
	gen := [][2]string{{"owner_email", `lower((emails ->> 'owner'::text))`}}
	rows := [][]any{
		{`{"owner":"glen.manning@example.com"}`, "glen.manning@example.com"},
		{`{"owner":"isaac.murillo@example.net"}`, "isaac.murillo@example.net"},
	}
	for _, tc := range []struct {
		name     string
		hit      pipeline.Category
		wantFail string
	}{
		{"under an emails column the email masker's output passes", pipeline.CatEmail, ""},
		{"under a notes column the leaf was filler and an address is refused", pipeline.CatFreeText, "email"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dec := pipeline.Decision{Category: pipeline.CatSemiStruct, Masked: true, NameHit: tc.hit}
			got := generatedNet(t, "emails", dec, gen, rows)["owner_email"]
			if tc.wantFail == "" && len(got) != 0 {
				t.Errorf("owner_email failures %v, want none: the column holds the email masker's output", got)
			}
			if tc.wantFail != "" && (len(got) != 1 || got[0] != tc.wantFail) {
				t.Errorf("owner_email failures %v, want one %s", got, tc.wantFail)
			}
		})
	}
}

// T-0394 (the 2026-09-25 JSON red team, round 1, A11): under --phone-region
// GB the net reads a masked document's keys under the region, and transform's
// keyCategory now masks a national-format key under the same region, so what
// the target holds is the phone masker's international output, which the
// net's international-only key skip leaves to the residual scan -- the run
// that refused at exit 9 with --skip-table as the only remedy now passes.
// The skip is not widened to the region: a national key that did survive can
// only be one transform failed to mask, and the net still refuses it.
func TestTheSecondNetPassesANationalPhoneKeyTransformMaskedUnderTheRegion(t *testing.T) {
	for _, c := range []struct {
		name, doc, wantFail string
	}{
		{"the keys transform writes", `{"+12015550142": "ledger matrix", "+13055550117": "node profile", "whatsApp": "+14155550123"}`, ""},
		{"a national key that survived", `{"07911 120001": "ledger matrix", "+13055550117": "node profile", "whatsApp": "+14155550123"}`, "phone"},
	} {
		t.Run(c.name, func(t *testing.T) {
			table := customers()
			col := ref.ColumnRef{Table: table, Column: "entries"}
			// The GB map internal/classify builds: no phone key is in it, and
			// whatsApp is phone from its sampled leaves.
			dec := pipeline.Decision{
				Col: col, Category: pipeline.CatSemiStruct, Masked: true,
				LeafKeys: map[string]pipeline.Category{"whatsApp": pipeline.CatPhone},
			}
			s := &state{
				opts:   Options{PhoneRegion: "GB"},
				schema: &pipeline.Schema{},
				target: oneColumn{vals: []any{c.doc}},
				steps:  []pipeline.Step{{Table: table, Mode: pipeline.ChildOK}},
				tables: map[ref.TableRef]*pipeline.Table{
					table: {Ref: table, Columns: []pipeline.Column{{Name: col.Column, TypeName: "jsonb"}}},
				},
				cls: &pipeline.Classification{PhoneRegion: "GB", Decisions: map[ref.ColumnRef]pipeline.Decision{col: dec}},
			}
			if err := s.secondNet(context.Background()); err != nil {
				t.Fatalf("secondNet: %v", err)
			}
			if c.wantFail == "" {
				if len(s.failures) != 0 {
					t.Fatalf("the net failed %s as %q on the phone masker's own output", col, s.failures[0].Reason)
				}
				return
			}
			if len(s.failures) != 1 || s.failures[0].Reason != c.wantFail {
				t.Fatalf("the net recorded %d failures, want one naming %s: a national key in the target is a copy", len(s.failures), c.wantFail)
			}
		})
	}
}
