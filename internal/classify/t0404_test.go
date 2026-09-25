// SPDX-License-Identifier: Apache-2.0

package classify

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
)

// T-0404 (the 2026-09-25 JSON red team, round 1, entry 28). Every value here
// is invented.

var t404Accounts = ref.TableRef{Schema: "public", Name: "t404_accounts"}

func t404Schema() *pipeline.Schema {
	return &pipeline.Schema{Tables: []pipeline.Table{
		tt("public", "t404_accounts", []string{"id"}, tc("id", "integer"), tc("prefs", "jsonb")),
	}}
}

// t404Docs is five documents with the given extra members appended.
func t404Docs(extra string) []any {
	var out []any
	for g := 1; g <= 5; g++ {
		out = append(out, fmt.Sprintf(`{"theme": "dark", "layout": "grid-%d", "email": "w%d@example.invalid"%s}`, g, g, extra))
	}
	return out
}

func t404Classify(t *testing.T, docs []any, prior *pipeline.Config) *pipeline.Classification {
	t.Helper()
	sampler := mapSampler{
		col(t404Accounts, "id"):    anyOf(int64(1), int64(2), int64(3), int64(4), int64(5)),
		col(t404Accounts, "prefs"): docs,
	}
	cls, err := New().Classify(t404Schema(), sampler, prior)
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	return cls
}

// t404PriorFrom is the prior a committed file written from cls reads back as:
// every column's entry, with the document's copied keys under leaf_keys.
func t404PriorFrom(cls *pipeline.Classification) *pipeline.Config {
	cols := map[ref.ColumnRef]pipeline.ColumnConfig{}
	for c, d := range cls.Decisions {
		cols[c] = pipeline.ColumnConfig{
			Category: d.Category, Confidence: d.Confidence, TypeFP: d.TypeFP,
			LeafKeys: d.RecordedLeafKeys,
		}
	}
	return &pipeline.Config{Columns: cols}
}

// The red team's own shape: run 1 records the copied keys, the source grows
// two keys, and run 2 from the committed file masks both and reports each as
// drift, while the keys the file lists keep their decisions.
func TestAKeyTheCommittedFileDoesNotListIsMaskedAndIsDrift(t *testing.T) {
	t.Parallel()
	prefs := col(t404Accounts, "prefs")

	first := t404Classify(t, t404Docs(""), nil)
	if got, want := decision(t, first, prefs).RecordedLeafKeys, []string{"layout", "theme"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("run 1 copied keys = %v, want %v (email has a category and is not copied)", got, want)
	}
	if len(first.LeafDrift) != 0 {
		t.Fatalf("run 1 with no committed file reported leaf drift: %v", first.LeafDrift)
	}
	prior := t404PriorFrom(first)

	grown := t404Docs(`, "guest": "Ysolde Pembrook", "emergency": "Ysolde Pembrook, sister"`)

	// Without a committed file the new keys are copied, as before T-0404.
	fresh := t404Classify(t, grown, nil)
	if m := decision(t, fresh, prefs).LeafMap(); m["guest"] != pipeline.CatNone || m["emergency"] != pipeline.CatNone {
		t.Fatalf("precondition: a fresh run does not call guest and emergency none: %v", m)
	}

	second := t404Classify(t, grown, prior)
	d := decision(t, second, prefs)
	m := d.LeafMap()
	for _, k := range []string{"guest", "emergency"} {
		if _, ok := m[k]; ok {
			t.Errorf("%s is still in the leaf map; a key the committed file does not list must be masked like one the samples never showed", k)
		}
	}
	if m["theme"] != pipeline.CatNone || m["layout"] != pipeline.CatNone {
		t.Errorf("a key the committed file lists lost its decision: %v", m)
	}
	if m["email"] != pipeline.CatEmail {
		t.Errorf("email = %q in the leaf map, want email", m["email"])
	}
	want := []pipeline.LeafDrift{{Col: prefs, Key: "emergency"}, {Col: prefs, Key: "guest"}}
	if !reflect.DeepEqual(second.LeafDrift, want) {
		t.Errorf("LeafDrift = %v, want %v", second.LeafDrift, want)
	}
	if got, want := d.RecordedLeafKeys, []string{"layout", "theme"}; !reflect.DeepEqual(got, want) {
		t.Errorf("recorded keys = %v, want %v: the file this run writes lists only the keys it copied", got, want)
	}
	if len(second.Drift) != 0 {
		t.Errorf("column drift = %v, want none: every column is in the file", second.Drift)
	}
	if !d.Masked || d.Category != pipeline.CatSemiStruct {
		t.Errorf("the column's own decision moved: masked %v, category %s", d.Masked, d.Category)
	}

	// The same samples against the same file give the same answer.
	again := t404Classify(t, grown, prior)
	if !reflect.DeepEqual(again.LeafDrift, second.LeafDrift) {
		t.Errorf("LeafDrift is not deterministic: %v then %v", second.LeafDrift, again.LeafDrift)
	}

	// A third run from the file the second one wrote -- every non-strict run
	// rewrites --config in place -- still masks both keys and reports them:
	// a run never approves a key on its own (T-0404 review round, finding 1).
	third := t404Classify(t, grown, t404PriorFrom(second))
	if !reflect.DeepEqual(third.LeafDrift, want) {
		t.Errorf("run 3 from run 2's file: LeafDrift = %v, want %v", third.LeafDrift, want)
	}
	if m3 := decision(t, third, prefs).LeafMap(); m3["guest"] == pipeline.CatNone || m3["emergency"] == pipeline.CatNone {
		t.Errorf("run 3 from run 2's file copies a drifted key: %v", m3)
	}

	// Listed by hand, the key is copied and is no longer drift.
	approved := t404PriorFrom(second)
	cc := approved.Columns[prefs]
	cc.LeafKeys = append(cc.LeafKeys, "guest")
	approved.Columns[prefs] = cc
	fourth := t404Classify(t, grown, approved)
	if got, want := fourth.LeafDrift, []pipeline.LeafDrift{{Col: prefs, Key: "emergency"}}; !reflect.DeepEqual(got, want) {
		t.Errorf("with guest listed by hand: LeafDrift = %v, want %v", got, want)
	}
	if m4 := decision(t, fourth, prefs).LeafMap(); m4["guest"] != pipeline.CatNone {
		t.Errorf("guest = %q with guest listed by hand, want none", m4["guest"])
	}
}

// A listed key the samples happen not to show keeps its place in the list,
// so sampling variance does not turn an approved key back into drift.
func TestAListedKeyTheSamplesDoNotShowStaysListed(t *testing.T) {
	t.Parallel()
	prefs := col(t404Accounts, "prefs")
	prior := t404PriorFrom(t404Classify(t, t404Docs(`, "guest": "on"`), nil))
	if got := prior.Columns[prefs].LeafKeys; !reflect.DeepEqual(got, []string{"guest", "layout", "theme"}) {
		t.Fatalf("precondition: run 1 recorded %v", got)
	}
	cls := t404Classify(t, t404Docs(""), prior)
	if got, want := decision(t, cls, prefs).RecordedLeafKeys, []string{"guest", "layout", "theme"}; !reflect.DeepEqual(got, want) {
		t.Errorf("recorded keys = %v, want %v", got, want)
	}
}

// A file written before T-0404 lists no keys, so every key the column copies
// is drift once, and masked: the safe direction for a format change. That run
// records the keys, so a run from the file it writes copies them; an entry
// whose list is present and empty approves nothing, run after run.
func TestAFileWithNoLeafKeysMakesEveryCopiedKeyDrift(t *testing.T) {
	t.Parallel()
	prefs := col(t404Accounts, "prefs")
	first := t404Classify(t, t404Docs(""), nil)
	prior := t404PriorFrom(first)
	cc := prior.Columns[prefs]
	cc.LeafKeys = nil
	prior.Columns[prefs] = cc

	cls := t404Classify(t, t404Docs(""), prior)
	want := []pipeline.LeafDrift{{Col: prefs, Key: "layout"}, {Col: prefs, Key: "theme"}}
	if !reflect.DeepEqual(cls.LeafDrift, want) {
		t.Errorf("LeafDrift = %v, want %v", cls.LeafDrift, want)
	}
	for k, c := range decision(t, cls, prefs).LeafMap() {
		if c == pipeline.CatNone {
			t.Errorf("%s is still copied from a file that lists no keys", k)
		}
	}
	if got, want := decision(t, cls, prefs).RecordedLeafKeys, []string{"layout", "theme"}; !reflect.DeepEqual(got, want) {
		t.Errorf("recorded keys = %v, want %v", got, want)
	}
	if next := t404Classify(t, t404Docs(""), t404PriorFrom(cls)); len(next.LeafDrift) != 0 {
		t.Errorf("run from the file the first re-run wrote: LeafDrift = %v, want none", next.LeafDrift)
	}

	cc.LeafKeys = []string{}
	prior.Columns[prefs] = cc
	empty := t404Classify(t, t404Docs(""), prior)
	if !reflect.DeepEqual(empty.LeafDrift, want) {
		t.Errorf("leaf_keys: []: LeafDrift = %v, want %v", empty.LeafDrift, want)
	}
	if got := decision(t, empty, prefs).RecordedLeafKeys; got == nil || len(got) != 0 {
		t.Errorf("leaf_keys: []: recorded keys = %#v, want an empty list", got)
	}
}

// Listing a key never copies it: a key the file lists that this run's
// evidence gives a category keeps the category.
func TestListingAKeyNeverCopiesOneThisRunMasks(t *testing.T) {
	t.Parallel()
	prefs := col(t404Accounts, "prefs")
	first := t404Classify(t, t404Docs(""), nil)
	prior := t404PriorFrom(first)
	cc := prior.Columns[prefs]
	cc.LeafKeys = append(cc.LeafKeys, "email")
	prior.Columns[prefs] = cc

	cls := t404Classify(t, t404Docs(""), prior)
	if got := decision(t, cls, prefs).LeafMap()["email"]; got != pipeline.CatEmail {
		t.Errorf("email = %q under a file that lists it, want email", got)
	}
	if len(cls.LeafDrift) != 0 {
		t.Errorf("LeafDrift = %v, want none", cls.LeafDrift)
	}
}

// A key that is not identifier-shaped -- a person's name used as a key -- is
// recorded and compared as a fingerprint, never as itself, and a column the
// file has never seen is column drift, classified fresh, with no key drift.
func TestANonIdentifierKeyIsComparedByFingerprint(t *testing.T) {
	t.Parallel()
	prefs := col(t404Accounts, "prefs")
	first := t404Classify(t, t404Docs(""), nil)
	prior := t404PriorFrom(first)

	grown := t404Docs(`, "Wren Calloway": "on"`)
	cls := t404Classify(t, grown, prior)
	if len(cls.LeafDrift) != 1 {
		t.Fatalf("LeafDrift = %v, want one key", cls.LeafDrift)
	}
	got := cls.LeafDrift[0].Key
	if strings.Contains(got, "Wren") || !strings.HasPrefix(got, "sha256:") || len(got) != len("sha256:")+16 {
		t.Errorf("drift key = %q, want a sha256: fingerprint and never the key itself", got)
	}
	if got != leafKeySpelling("Wren Calloway", false) {
		t.Errorf("drift key = %q, want leafKeySpelling's %q", got, leafKeySpelling("Wren Calloway", false))
	}

	// Listed by its fingerprint, the key is no longer drift.
	cc := prior.Columns[prefs]
	cc.LeafKeys = append(cc.LeafKeys, got)
	prior.Columns[prefs] = cc
	if again := t404Classify(t, grown, prior); len(again.LeafDrift) != 0 {
		t.Errorf("LeafDrift = %v once the file lists the fingerprint, want none", again.LeafDrift)
	}

	// A file that has never seen the column: column drift, keys copied fresh.
	delete(prior.Columns, prefs)
	unseen := t404Classify(t, grown, prior)
	if len(unseen.LeafDrift) != 0 {
		t.Errorf("LeafDrift = %v for a column the file has never seen, want none (it is column drift)", unseen.LeafDrift)
	}
	if !reflect.DeepEqual(unseen.Drift, []ref.ColumnRef{prefs}) {
		t.Errorf("Drift = %v, want [%s]", unseen.Drift, prefs)
	}
}

func TestLeafKeySpellingWritesOnlyIdentifierShapedKeysAsThemselves(t *testing.T) {
	t.Parallel()
	for _, k := range []string{"theme", "firstName", "contact_person", "ui-dark-mode", "_id", "a1", "address_12", "facade"} {
		if got := leafKeySpelling(k, false); got != k {
			t.Errorf("leafKeySpelling(%q) = %q, want the key itself", k, got)
		}
		if got := leafKeySpelling(k, true); got != leafKeyFingerprint(k) {
			t.Errorf("leafKeySpelling(%q) in a column of many keys = %q, want its fingerprint", k, got)
		}
	}
	for _, k := range []string{
		"", "Wren Calloway", "1st", "@type", "$schema", "key:value", "é", strings.Repeat("a", 33),
		// T-0404 review round, finding 2: a UUID whose first character is a
		// letter, a dotted handle, an id with a digit run, a synthetic token
		// with digits mid-string, and an all-hex key with no digit at all.
		"a3f1c2e4-5b6d-4e7f-8a9b-0c1d2e3f4a5b", "alice.smith", "cust_48213", "line123", "Qm7xRt2vLp9sWk4n", "deadbeefcafe",
	} {
		got := leafKeySpelling(k, false)
		if !strings.HasPrefix(got, "sha256:") || len(got) != len("sha256:")+16 {
			t.Errorf("leafKeySpelling(%q) = %q, want a sha256: fingerprint", k, got)
		}
	}
}

// A column whose documents show more than leafKeyNameLimit keys is keyed by
// data, so every key is recorded and reported as a fingerprint, word-shaped
// ones included, and a file that listed a key by name still matches it.
func TestAColumnOfManyKeysIsRecordedByFingerprint(t *testing.T) {
	t.Parallel()
	prefs := col(t404Accounts, "prefs")
	var members []string
	for i := 0; i <= leafKeyNameLimit; i++ {
		members = append(members, fmt.Sprintf(`"user%c%c": "on"`, 'a'+i/26, 'a'+i%26))
	}
	docs := t404Docs(", " + strings.Join(members, ", "))
	cls := t404Classify(t, docs, nil)
	recorded := decision(t, cls, prefs).RecordedLeafKeys
	if len(recorded) != leafKeyNameLimit+3 {
		t.Fatalf("recorded %d keys, want %d", len(recorded), leafKeyNameLimit+3)
	}
	for _, k := range recorded {
		if !strings.HasPrefix(k, "sha256:") {
			t.Errorf("recorded key %q in a column of many keys, want a fingerprint", k)
		}
	}

	// A file that lists the keys by name (written when the column held
	// fewer) still matches them by fingerprint.
	prior := t404PriorFrom(cls)
	cc := prior.Columns[prefs]
	cc.LeafKeys = []string{"layout", "theme"}
	for i := 0; i <= leafKeyNameLimit; i++ {
		cc.LeafKeys = append(cc.LeafKeys, fmt.Sprintf("user%c%c", 'a'+i/26, 'a'+i%26))
	}
	prior.Columns[prefs] = cc
	if again := t404Classify(t, docs, prior); len(again.LeafDrift) != 0 {
		t.Errorf("LeafDrift = %v from a file that lists every key by name, want none", again.LeafDrift)
	}
}
