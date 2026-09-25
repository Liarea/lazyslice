// SPDX-License-Identifier: Apache-2.0

package emit

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
)

// T-0404: a document column's copied keys are recorded under leaf_keys:, so a
// re-run can tell a key it has seen from one it has not.

// Emit writes the keys classify recorded for each document column
// (Decision.RecordedLeafKeys, already spelled and sorted), an empty list for a
// document none of whose keys is copied, and nothing at all on any other
// column. The sha256: entry stands in for a key that is not identifier-shaped.
func TestEmitRecordsTheCopiedLeafKeys(t *testing.T) {
	prefs := col("public", "accounts", "prefs")
	allNamed := col("public", "accounts", "contact")
	scalar := col("public", "accounts", "email")
	recorded := []string{"guest", "layout", "sha256:5d41402abc4b2a76", "theme"}
	cls := &pipeline.Classification{
		Decisions: map[ref.ColumnRef]pipeline.Decision{
			prefs: {Col: prefs, Category: pipeline.CatSemiStruct, Confidence: pipeline.ConfPossible,
				Masked: true, RecordedLeafKeys: recorded},
			allNamed: {Col: allNamed, Category: pipeline.CatSemiStruct, Confidence: pipeline.ConfPossible,
				Masked: true, RecordedLeafKeys: []string{}},
			scalar: {Col: scalar, Category: pipeline.CatEmail, Confidence: pipeline.ConfCertain, Masked: true},
		},
	}
	cfg, err := New(Options{}).Emit(&pipeline.Plan{Root: tbl("public", "accounts")}, cls, nil,
		pipeline.PlanRequest{}, pipeline.Candidate{}, pipeline.Candidate{}, "")
	if err != nil {
		t.Fatalf("Emit: %v", err)
	}
	if got := cfg.Columns[prefs].LeafKeys; !reflect.DeepEqual(got, recorded) {
		t.Errorf("prefs leaf_keys = %v, want %v", got, recorded)
	}
	if got := cfg.Columns[allNamed].LeafKeys; got == nil || len(got) != 0 {
		t.Errorf("contact leaf_keys = %#v, want an empty, non-nil list", got)
	}
	if got := cfg.Columns[scalar].LeafKeys; got != nil {
		t.Errorf("email leaf_keys = %v, want none on a scalar column", got)
	}

	// Written and read back: the list, the empty list and the absent key all
	// survive.
	path := filepath.Join(t.TempDir(), "lazyslice.yml")
	cfg.Version = Version
	if err = New(Options{}).Write(path, cfg); err != nil {
		t.Fatalf("Write: %v", err)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(string(body), "leaf_keys:"); n != 2 {
		t.Errorf("the file has %d leaf_keys: lines, want 2 (prefs and contact):\n%s", n, body)
	}
	got, err := New(Options{}).Read(path)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	for _, c := range []ref.ColumnRef{prefs, allNamed, scalar} {
		if !reflect.DeepEqual(got.Columns[c].LeafKeys, cfg.Columns[c].LeafKeys) {
			t.Errorf("%s leaf_keys read back as %#v, want %#v", c, got.Columns[c].LeafKeys, cfg.Columns[c].LeafKeys)
		}
	}
}

// A hand-edited list comes back as a set, and a key YAML would read as
// something other than a string survives as the string.
func TestLeafKeysReadBackAsASetOfStrings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lazyslice.yml")
	c := col("public", "accounts", "prefs")
	cfg := &pipeline.Config{Version: Version, Columns: map[ref.ColumnRef]pipeline.ColumnConfig{
		c: {Category: pipeline.CatSemiStruct, LeafKeys: []string{"theme", "null", "yes", "theme", "on", "a1"}},
	}}
	if err := New(Options{}).Write(path, cfg); err != nil {
		t.Fatalf("Write: %v", err)
	}
	got, err := New(Options{}).Read(path)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	want := []string{"a1", "null", "on", "theme", "yes"}
	if !reflect.DeepEqual(got.Columns[c].LeafKeys, want) {
		t.Errorf("leaf_keys = %#v, want %#v", got.Columns[c].LeafKeys, want)
	}
}
