// SPDX-License-Identifier: Apache-2.0

package classify

import (
	"fmt"
	"testing"

	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
)

// T-0394 (the 2026-09-25 JSON red team, round 1, A11 and A26). Every value
// here is invented; the numbers are Ofcom's drama ranges or shaped like them.

// rostersDoc is A11's document: a national-format and an international phone
// number used as object keys, each over a person's name.
func rostersDoc(g int) string {
	return fmt.Sprintf(`{"07911 12%04d": "Wren Calloway", "+44 7911 13%04d": "Ysolde Brackenridge"}`, g, g)
}

// A11: transform's keyCategory reads a key's phone number under the run's
// region, so the map must leave the same keys out under the same region, or
// a leaf beneath a masked key would be looked up under a name the map holds.
// With no region the national key is a name the map calls none, as before.
func TestANationalPhoneKeyIsAValueUnderTheRunsPhoneRegion(t *testing.T) {
	p, err := pack()
	if err != nil {
		t.Fatalf("pack: %v", err)
	}
	samples := anyOf(rostersDoc(1), rostersDoc(2), rostersDoc(3))
	for _, c := range []struct {
		region  string
		wantKey bool
	}{
		{"", true},
		{"GB", false},
	} {
		got := jsonKeyCategories(p, samples, c.region)
		_, national := got["07911 120001"]
		if national != c.wantKey {
			t.Errorf("region %q: national key in the map = %v, want %v", c.region, national, c.wantKey)
		}
		if _, ok := got["+44 7911 130001"]; ok {
			t.Errorf("region %q: an international key entered the map; transform masks it under any region", c.region)
		}
	}

	// And through Classify, which hands its own region to the map.
	rosters := ref.TableRef{Schema: "public", Name: "t394_rosters"}
	schema := &pipeline.Schema{Tables: []pipeline.Table{
		tt("public", "t394_rosters", []string{"id"}, tc("id", "integer"), tc("entries", "jsonb")),
	}}
	sampler := mapSampler{
		col(rosters, "id"):      anyOf(int64(1), int64(2), int64(3)),
		col(rosters, "entries"): samples,
	}
	cls, err := New().Classify(schema, sampler, &pipeline.Config{PhoneRegion: "GB"})
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	if _, ok := decision(t, cls, col(rosters, "entries")).LeafKeys["07911 120001"]; ok {
		t.Error("under --phone-region GB the national key entered the decision's map")
	}
}

// A26: a national-format number under a leaf key no name rule names, with no
// --phone-region. The scalar path masks the same values in a column of the
// same name when the table holds a proven personal neighbour; the leaf key
// now gets CatPhone on the same ratio and the same corroboration, or on a
// person-identifying key in the document itself, and nothing without either.
func TestAGuessedRegionPhoneLeafKeyIsPhoneWhenCorroborated(t *testing.T) {
	t.Parallel()
	members := ref.TableRef{Schema: "public", Name: "t394_members"}
	whatsApp := func(g int) string { return fmt.Sprintf("020 7946 6%03d", g) }
	docs := func(extra string) []any {
		var out []any
		for g := 1; g <= 5; g++ {
			out = append(out, fmt.Sprintf(`{"whatsApp": %q, "kind": "standard"%s}`, whatsApp(g), extra))
		}
		return out
	}
	emails := anyOf("wren.calloway1@fixture.test", "wren.calloway2@fixture.test", "wren.calloway3@fixture.test",
		"wren.calloway4@fixture.test", "wren.calloway5@fixture.test")
	ids := anyOf(int64(1), int64(2), int64(3), int64(4), int64(5))

	cases := []struct {
		name      string
		neighbour bool
		extra     string
		region    string
		want      pipeline.Category
	}{
		{"a personal neighbour in the table", true, "", "", pipeline.CatPhone},
		{"a personal key in the document", false, `, "givenName": "Quillon"`, "", pipeline.CatPhone},
		{"neither", false, "", "", pipeline.CatNone},
		// A configured region masks each such leaf by its value already
		// (leafValueCategory); the guessed list is still asked beside it, as
		// the scalar path's is.
		{"a personal neighbour, under GB", true, "", "GB", pipeline.CatPhone},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cols := []pipeline.Column{tc("id", "bigint"), tc("doc", "jsonb")}
			sampler := mapSampler{col(members, "id"): ids, col(members, "doc"): docs(c.extra)}
			if c.neighbour {
				cols = append(cols, tc("email", "text"))
				sampler[col(members, "email")] = emails
			}
			schema := &pipeline.Schema{Tables: []pipeline.Table{tt("public", "t394_members", []string{"id"}, cols...)}}
			var prior *pipeline.Config
			if c.region != "" {
				prior = &pipeline.Config{PhoneRegion: c.region}
			}
			cls, err := New().Classify(schema, sampler, prior)
			if err != nil {
				t.Fatalf("Classify: %v", err)
			}
			d := decision(t, cls, col(members, "doc"))
			got, ok := d.LeafKeys["whatsApp"]
			if !ok {
				t.Fatalf("whatsApp is not in the map %v", d.LeafKeys)
			}
			if got != c.want {
				t.Errorf("LeafKeys[whatsApp] = %q, want %q", got, c.want)
			}
			if d.LeafKeys["kind"] != pipeline.CatNone {
				t.Errorf("LeafKeys[kind] = %q, want none: a key of words is not a phone number", d.LeafKeys["kind"])
			}
		})
	}
}

// The ratio is the scalar path's: at least minSamples string leaves, and
// validatorThreshold of them parsing. A key whose leaves mostly do not parse,
// or that has too few, stays none even beside a personal neighbour.
func TestAGuessedRegionPhoneLeafKeyNeedsTheScalarRatio(t *testing.T) {
	keys := map[string]pipeline.Category{
		"whatsApp": pipeline.CatNone, "mixed": pipeline.CatNone, "short": pipeline.CatNone,
		"named": pipeline.CatEmail, "nested": pipeline.CatNone,
	}
	var samples []any
	for g := 1; g <= 5; g++ {
		mixed := "ask at the desk"
		if g == 1 {
			mixed = "020 7946 6001"
		}
		doc := map[string]any{
			"whatsApp": fmt.Sprintf("020 7946 6%03d", g),
			"mixed":    mixed,
			"named":    fmt.Sprintf("020 7946 6%03d", g),
			// An array's elements count toward the key holding it.
			"nested": []any{fmt.Sprintf("07911 12%04d", g)},
		}
		if g <= 2 {
			doc["short"] = fmt.Sprintf("020 7946 6%03d", g)
		}
		samples = append(samples, doc)
	}
	got := guessedPhoneLeafKeys(samples, keys)
	want := []string{"nested", "whatsApp"}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("guessedPhoneLeafKeys = %v, want %v (mixed is below the ratio, short below minSamples, named already has a category)", got, want)
	}
}
