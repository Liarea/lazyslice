// SPDX-License-Identifier: Apache-2.0

package classify

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
)

// T-0272: the per-leaf half of a json or jsonb decision. Every object key the
// samples show, at any depth and inside arrays of objects, is named through the
// rule pack, and a key that is itself an address, a phone number or a card
// number never enters the map. A sample arrives the way pgx hands it back — a
// decoded map — as well as as text.
func TestJSONKeyCategoriesNamesEveryKeyTheSamplesShow(t *testing.T) {
	p, err := pack()
	if err != nil {
		t.Fatalf("pack: %v", err)
	}
	samples := []any{
		map[string]any{
			"theme":   "dark",
			"profile": map[string]any{"email": "ada.lovelace@fixture.test"},
			"items":   []any{map[string]any{"sku": "A-1"}},
		},
		`{"theme":"light","grace.hopper@fixture.test":{"note":"x"}}`,
		nil,
		"not a document",
	}
	got := jsonKeyCategories(p, samples)

	want := map[string]pipeline.Category{
		"theme":   pipeline.CatNone,
		"profile": pipeline.CatNone,
		"email":   pipeline.CatEmail,
		"items":   pipeline.CatNone,
		"sku":     pipeline.CatNone,
		"note":    pipeline.CatFreeText,
	}
	for k, cat := range want {
		g, ok := got[k]
		if !ok {
			t.Errorf("key %q is not in the map; a leaf beneath it would be masked for want of a name", k)
			continue
		}
		if g != cat {
			t.Errorf("key %q = %q, want %q (the rule pack's own answer for the name)", k, g, cat)
		}
	}
	if _, ok := got["grace.hopper@fixture.test"]; ok {
		t.Error("an email address used as a key entered the map; it is a value transform masks, not a name")
	}
	if jsonKeyCategories(p, []any{nil, `[1,2]`, `"x"`}) != nil {
		t.Error("samples with no object key produced a map; nil is what says nothing is known")
	}
}

// The decision a json or jsonb column comes out of Classify with carries the
// map, and a text column's does not.
func TestAJSONColumnsDecisionCarriesItsLeafKeys(t *testing.T) {
	table := ref.TableRef{Schema: "public", Name: "t0272_settings"}
	schema := &pipeline.Schema{Tables: []pipeline.Table{
		tt("public", "t0272_settings", []string{"id"},
			tc("id", "bigint"), tc("prefs", "jsonb"), tc("label", "text")),
	}}
	samples := mapSampler{
		col(table, "id"):    anyOf(int64(1), int64(2)),
		col(table, "prefs"): anyOf(map[string]any{"theme": "dark"}, map[string]any{"theme": "light", "email": "ada@fixture.test"}),
		col(table, "label"): anyOf(`{"theme":"dark"}`, "plain"),
	}
	cls, err := New().Classify(schema, samples, nil)
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	prefs := decision(t, cls, col(table, "prefs"))
	if prefs.LeafKeys["theme"] != pipeline.CatNone || prefs.LeafKeys["email"] != pipeline.CatEmail {
		t.Errorf("prefs.LeafKeys = %v, want theme none and email named", prefs.LeafKeys)
	}
	if label := decision(t, cls, col(table, "label")); label.LeafKeys != nil {
		t.Errorf("a text column's decision carries LeafKeys %v", label.LeafKeys)
	}
}

// T-0272 review round, finding 3: past jsonKeyLimit, which keys enter the map
// is decided over the sorted set of every key the samples showed, so the same
// samples in any order, ranged over in any map order, give the same map.
func TestJSONKeyCategoriesPastTheLimitIsTheSameEveryTime(t *testing.T) {
	p, err := pack()
	if err != nil {
		t.Fatalf("pack: %v", err)
	}
	doc := func(prefix string) map[string]any {
		m := map[string]any{}
		for i := 0; i < jsonKeyLimit; i++ {
			m[fmt.Sprintf("%s%05d", prefix, i)] = "x"
		}
		return m
	}
	a, b := doc("k"), doc("j")
	first := jsonKeyCategories(p, []any{a, b})
	if len(first) != jsonKeyLimit {
		t.Fatalf("map holds %d keys, want the limit %d", len(first), jsonKeyLimit)
	}
	for i := 0; i < 5; i++ {
		if got := jsonKeyCategories(p, []any{b, a}); !reflect.DeepEqual(got, first) {
			t.Fatal("the same samples in another order gave a different map")
		}
	}
	if _, ok := first["j00000"]; !ok {
		t.Error("the sorted-first key is not in the map")
	}
}

// The region internal/classify ran under is on the classification, where
// internal/transform reads it for a JSON leaf's phone question.
func TestTheClassificationCarriesItsPhoneRegion(t *testing.T) {
	schema := &pipeline.Schema{Tables: []pipeline.Table{
		tt("public", "t0272_region", []string{"id"}, tc("id", "bigint")),
	}}
	cls, err := New().Classify(schema, mapSampler{}, &pipeline.Config{PhoneRegion: "GB"})
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	if cls.PhoneRegion != "GB" {
		t.Errorf("Classification.PhoneRegion = %q, want GB", cls.PhoneRegion)
	}
}
