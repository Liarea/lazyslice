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

// Supabase's auth.identities.email is `lower((identity_data ->> 'email'))`
// (testdata/torture/supabase-auth). With per-leaf categories the leaf it reads
// is replaced by the email masker, so the generated column holds the masker's
// addresses; the net skips the leaf maskers' own categories for it and runs
// every other validator. Without a map the leaf was filler, and an address
// there is refused as before.
func TestAGeneratedColumnOverAMaskedDocumentsLeafIsTheMaskersOutput(t *testing.T) {
	const supabase = `lower((identity_data ->> 'email'::text))`
	fakes := []any{"glen.manning@example.com", "isaac.murillo@example.net", "sheila.perkins@example.net"}
	for _, tc := range []struct {
		name     string
		keys     map[string]pipeline.Category
		vals     []any
		wantFail string
	}{
		{"the email masker's output passes", map[string]pipeline.Category{"email": pipeline.CatEmail}, fakes, ""},
		{"with no map the same addresses are refused", nil, fakes, "email"},
		{
			"a validator outside the leaf maskers' categories still runs",
			map[string]pipeline.Category{"email": pipeline.CatEmail},
			[]any{"diagnosed with schizophrenia", "HIV positive, CD4 210", "Ahmadiyya Muslim"},
			"special_category",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			table := customers()
			c := func(n string) ref.ColumnRef { return ref.ColumnRef{Table: table, Column: n} }
			s := &state{
				schema: &pipeline.Schema{},
				target: oneColumn{vals: tc.vals},
				steps:  []pipeline.Step{{Table: table, Mode: pipeline.ChildOK}},
				// identity_data is scanned too, over the same plain strings,
				// which are not documents and yield nothing.
				tables: map[ref.TableRef]*pipeline.Table{table: {Ref: table, Columns: []pipeline.Column{
					{Name: "identity_data", TypeName: "jsonb"},
					{Name: "email", TypeName: "text", Generated: supabase},
				}}},
				cls: &pipeline.Classification{Decisions: map[ref.ColumnRef]pipeline.Decision{
					c("identity_data"): {Col: c("identity_data"), Category: pipeline.CatSemiStruct, Masked: true, LeafKeys: tc.keys},
					c("email"):         {Col: c("email"), Category: pipeline.CatNone},
				}},
			}
			if err := s.secondNet(context.Background()); err != nil {
				t.Fatalf("secondNet: %v", err)
			}
			var got []string
			for _, f := range s.failures {
				if f.Column == "email" {
					got = append(got, f.Reason)
				}
			}
			if tc.wantFail == "" && len(got) != 0 {
				t.Errorf("email failures %v, want none: the column holds the email masker's output", got)
			}
			if tc.wantFail != "" && (len(got) != 1 || got[0] != tc.wantFail) {
				t.Errorf("email failures %v, want one %s", got, tc.wantFail)
			}
		})
	}
}
