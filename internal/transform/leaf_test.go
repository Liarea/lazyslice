// SPDX-License-Identifier: Apache-2.0

package transform

import (
	"reflect"
	"testing"

	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/textsig"
	"github.com/Liarea/lazyslice/mask"
)

// Per-leaf categories (T-0272, the maintainer's T-0143 decision of
// 2026-09-24): a leaf with a personal signal on a key or its value is masked,
// under its category's masker where that masker is safe for a leaf; a leaf
// with none, under keys the samples showed, is copied; everything else is
// masked as every leaf was before.

// pinnedLeafValues is the value half of the rule, pinned. The identical table
// is in internal/verify/jsonleaf_test.go: the two restatements of
// leafValueCategory have to answer every row the same way, and a change to
// either list that is not made to both fails one of the two copies of this
// test. No row imitates a real provider's secret format.
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

func TestLeafRule(t *testing.T) {
	keys := map[string]pipeline.Category{
		"settings": pipeline.CatNone,
		"theme":    pipeline.CatNone,
		"model":    pipeline.CatNone,
		"email":    pipeline.CatEmail,
		"address":  pipeline.CatAddress,
		"line1":    pipeline.CatNone,
		"name":     pipeline.CatPersonName,
		"dob":      pipeline.CatPersonDate,
		"notes":    pipeline.CatFreeText,
	}
	cases := []struct {
		name  string
		keys  map[string]pipeline.Category
		chain []string
		text  string
		want  leafVerdict
	}{
		{"no map masks every leaf as before", nil, []string{"settings", "theme"}, "dark", leafVerdict{cat: pipeline.CatFreeText}},
		{"no map ignores the value's shape too", nil, []string{"settings"}, "ada@example.org", leafVerdict{cat: pipeline.CatFreeText}},
		{"a signal-free leaf under seen keys is copied", keys, []string{"settings", "theme"}, "dark", leafVerdict{copy: true}},
		{"the leaf's own key names the category", keys, []string{"settings", "email"}, "n/a", leafVerdict{cat: pipeline.CatEmail}},
		{"an enclosing key names the category", keys, []string{"address", "line1"}, "Flat 2", leafVerdict{cat: pipeline.CatAddress}},
		{"a value a validator recognises is masked under its category", keys, []string{"settings", "theme"}, "ada@example.org", leafVerdict{cat: pipeline.CatEmail}},
		{"a key the samples never showed is masked", keys, []string{"settings", "unseen"}, "dark", leafVerdict{cat: pipeline.CatFreeText}},
		{"an unseen enclosing key is masked too", keys, []string{"unseen", "theme"}, "dark", leafVerdict{cat: pipeline.CatFreeText}},
		{"a leaf with no key is masked", keys, nil, "dark", leafVerdict{cat: pipeline.CatFreeText}},
		{"person_name is masked as free_text in a leaf", keys, []string{"name"}, "Ada Lovelace", leafVerdict{cat: pipeline.CatFreeText}},
		{"person_date is masked as free_text in a leaf", keys, []string{"dob"}, "1815-12-10", leafVerdict{cat: pipeline.CatFreeText}},
		{"a free_text key is masked", keys, []string{"notes"}, "call after six", leafVerdict{cat: pipeline.CatFreeText}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := leafRule(leafPolicy{keys: c.keys}, c.chain, c.text, true); got != c.want {
				t.Errorf("leafRule = %+v, want %+v", got, c.want)
			}
		})
	}
}

// configDocument is the shape dogfood session 1 found destroyed (T-0143's
// log, 2026-09-23): a configuration document with no personal leaf beside
// three that are.
func configDocument() map[string]any {
	return map[string]any{
		"model": map[string]any{
			"engine":      "gpt-4o",
			"temperature": 0.7,
			"max_tokens":  float64(4096),
			"stream":      true,
		},
		"owner":    map[string]any{"email": "ada.lovelace@example.org"},
		"notify":   "grace.hopper@example.org",
		"endpoint": "https://api.example.test/v1/chat",
		"extra":    map[string]any{"note": "call after six"},
		"tags":     []any{"beta", "internal"},
		"retired":  nil,
	}
}

// configKeys is what internal/classify would put on the decision for samples
// of configDocument, less "extra", which the samples never showed.
func configKeys() map[string]pipeline.Category {
	return map[string]pipeline.Category{
		"model": pipeline.CatNone, "engine": pipeline.CatNone, "temperature": pipeline.CatNone,
		"max_tokens": pipeline.CatNone, "stream": pipeline.CatNone,
		"owner": pipeline.CatNone, "email": pipeline.CatEmail,
		"notify": pipeline.CatNone, "endpoint": pipeline.CatNone,
		"note": pipeline.CatNone, "tags": pipeline.CatNone, "retired": pipeline.CatNone,
	}
}

func TestASignalFreeLeafIsCopiedAndAPersonalLeafIsMasked(t *testing.T) {
	k := key(t, 0x72)
	cls := classification()
	d := cls.Decisions[col("people", "contact")]
	d.LeafKeys = configKeys()
	cls.Decisions[col("people", "contact")] = d

	b := peopleBatch()
	b.Rows = b.Rows[:1]
	b.Rows[0][4] = configDocument()
	res := &recorder{inner: NewResidual(100)}
	out, err := New(fixture()).Transform(b, cls, &k, res)
	if err != nil {
		t.Fatalf("Transform: %v", err)
	}
	doc, ok := out.Rows[0][4].(map[string]any)
	if !ok {
		t.Fatalf("people.contact masked to %T", out.Rows[0][4])
	}
	got := leaves(t, doc)
	src := leaves(t, configDocument())

	for _, path := range []string{
		"$.model.engine", "$.model.temperature", "$.model.max_tokens", "$.model.stream",
		"$.tags[0]", "$.tags[1]", "$.retired",
	} {
		if !reflect.DeepEqual(got[path], src[path]) {
			t.Errorf("%s = %v, want %v copied: no key or value signal, under keys the samples showed", path, got[path], src[path])
		}
	}
	for _, path := range []string{"$.owner.email", "$.notify"} {
		s, _ := got[path].(string)
		if s == src[path] || !textsig.ValidEmail(s) {
			t.Errorf("%s = %q, want a different email address from the email masker", path, s)
		}
	}
	for _, path := range []string{"$.endpoint", "$.extra.note"} {
		if got[path] == src[path] {
			t.Errorf("%s survived masking as %v", path, got[path])
		}
	}

	// The filter holds the four masked leaves, each under free_text's
	// canonical form of its source whatever masker replaced it, and nothing
	// for a copied leaf, which the target holds unchanged.
	want := map[string]string{}
	for _, path := range []string{"$.owner.email", "$.notify", "$.endpoint", "$.extra.note"} {
		canon, _, err := mask.Canonical(mask.Category(pipeline.CatFreeText), mask.Value{Text: src[path].(string)}, leafConstraints())
		if err != nil {
			t.Fatalf("canonical of %s: %v", path, err)
		}
		want[path] = canon.Text
	}
	recorded := map[string]string{}
	for _, a := range res.adds {
		if a.col != col("people", "contact") {
			continue
		}
		recorded[a.path] = a.canonical
	}
	if !reflect.DeepEqual(recorded, want) {
		t.Errorf("filter entries for people.contact = %v, want %v", recorded, want)
	}
}

// An email leaf copied under a map that says nothing about its key is the
// failure the value half exists for: without it, "notify" would be copied.
func TestAPersonalValueUnderAnImpersonalKeyIsMasked(t *testing.T) {
	k := key(t, 0x73)
	cls := classification()
	d := cls.Decisions[col("people", "contact")]
	d.LeafKeys = map[string]pipeline.Category{"notify": pipeline.CatNone, "count": pipeline.CatNone}
	cls.Decisions[col("people", "contact")] = d

	b := peopleBatch()
	b.Rows = b.Rows[:1]
	b.Rows[0][4] = map[string]any{"notify": "grace.hopper@example.org", "count": float64(4111111111111111)}
	out, err := New(fixture()).Transform(b, cls, &k, NewResidual(100))
	if err != nil {
		t.Fatalf("Transform: %v", err)
	}
	doc := out.Rows[0][4].(map[string]any)
	if doc["notify"] == "grace.hopper@example.org" {
		t.Error("an email address under a key no rule names was copied")
	}
	if doc["count"] == float64(4111111111111111) {
		// A float64 leaf is read in its plain decimal spelling: 'g' would
		// hand the validators 4.111111111111111e+15.
		t.Error("a card number stored as a JSON number under a key no rule names was copied")
	}
}

// T-0272 review round, finding 1: the column's own decision is the root of
// every leaf's chain. A jsonb column whose own name the rule pack scores
// special_category (`medical_history`: masked on its name alone,
// THREAT_MODEL.md T1), or one an operator raised through the yml or --mask,
// has every leaf masked whatever its map says, so a signal-free leaf under a
// key the samples showed is not copied.
func TestAColumnsOwnDecisionMasksEveryLeafWhateverItsMapSays(t *testing.T) {
	src := map[string]any{"status": "positive", "smoker": true, "blood": "O+", "visits": float64(3)}
	leafKeys := map[string]pipeline.Category{
		"status": pipeline.CatNone, "smoker": pipeline.CatNone,
		"blood": pipeline.CatNone, "visits": pipeline.CatNone,
	}
	for _, c := range []struct {
		name string
		cat  pipeline.Category
		src  pipeline.DecisionSource
	}{
		{"a special_category column", pipeline.CatSpecial, pipeline.ByClassifier},
		{"a yml-raised semi_structured column", pipeline.CatSemiStruct, pipeline.ByYmlRaise},
	} {
		t.Run(c.name, func(t *testing.T) {
			k := key(t, 0x74)
			cls := classification()
			d := cls.Decisions[col("people", "contact")]
			d.Category, d.Source, d.LeafKeys = c.cat, c.src, leafKeys
			cls.Decisions[col("people", "contact")] = d

			b := peopleBatch()
			b.Rows = b.Rows[:1]
			b.Rows[0][4] = src
			res := &recorder{inner: NewResidual(100)}
			out, err := New(fixture()).Transform(b, cls, &k, res)
			if err != nil {
				t.Fatalf("Transform: %v", err)
			}
			doc := out.Rows[0][4].(map[string]any)
			for _, name := range []string{"status", "blood", "visits"} {
				if reflect.DeepEqual(doc[name], src[name]) {
					t.Errorf("%s was copied as %v: the column's own decision says every leaf is personal", name, doc[name])
				}
			}
			// The boolean is redrawn and may land on its own value, so it is
			// checked through the filter instead: every masked string and
			// number leaf is recorded, which a copied leaf never is.
			recorded := map[string]bool{}
			for _, a := range res.adds {
				recorded[a.path] = true
			}
			for _, path := range []string{"$.status", "$.blood", "$.visits"} {
				if !recorded[path] {
					t.Errorf("%s has no filter entry: it was copied, not masked", path)
				}
			}
		})
	}
}

// T-0272 review round, finding 2: the value half reads a phone number under
// the region the run classified under, which is the region the second net
// reads it under, so a national-format number under a key no rule names is
// masked rather than copied and refused at exit 9.
func TestANationalNumberIsMaskedUnderTheRunsPhoneRegion(t *testing.T) {
	const national = "020 7946 0958"
	for _, c := range []struct {
		region   string
		wantCopy bool
	}{
		{"", true},
		{"GB", false},
	} {
		k := key(t, 0x75)
		cls := classification()
		cls.PhoneRegion = c.region
		d := cls.Decisions[col("people", "contact")]
		d.LeafKeys = map[string]pipeline.Category{"reception": pipeline.CatNone}
		cls.Decisions[col("people", "contact")] = d

		b := peopleBatch()
		b.Rows = b.Rows[:1]
		b.Rows[0][4] = map[string]any{"reception": national}
		out, err := New(fixture()).Transform(b, cls, &k, NewResidual(100))
		if err != nil {
			t.Fatalf("Transform: %v", err)
		}
		got := out.Rows[0][4].(map[string]any)["reception"]
		if copied := got == national; copied != c.wantCopy {
			t.Errorf("region %q: reception = %v, copied %v, want copied %v", c.region, got, copied, c.wantCopy)
		}
	}
}
