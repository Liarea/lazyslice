// SPDX-License-Identifier: Apache-2.0

package emit

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Liarea/lazyslice/internal/dsn"
	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
	"github.com/Liarea/lazyslice/mask"
)

func tbl(schema, name string) ref.TableRef { return ref.TableRef{Schema: schema, Name: name} }

func col(schema, name, column string) ref.ColumnRef {
	return ref.ColumnRef{Table: tbl(schema, name), Column: column}
}

// sample is a config with one of everything a re-run reads back.
func sample() *pipeline.Config {
	return &pipeline.Config{
		Version: Version,
		Tool:    "0.1.0",
		Source:  pipeline.FromCompose,
		SourceRef: dsn.Ref{Host: "127.0.0.1", Port: 5432, Database: "pagila", User: "app_ro", Params: map[string]string{
			"sslmode":     "verify-full",
			"sslrootcert": "/etc/ssl/certs/pagila-ca.pem",
		}},
		SourceLabel:  "db",
		Target:       pipeline.FromFlag,
		TargetRef:    dsn.Ref{Host: "127.0.0.1", Port: 5433, Database: "pagila_test"},
		Root:         tbl("public", "customer"),
		Take:         200,
		Cap:          100,
		Caps:         map[ref.TableRef]int{tbl("public", "payment"): 50},
		Depth:        3,
		RowBudget:    1_000_000,
		MemoryBudget: 256 << 20,
		Keys: map[ref.TableRef][]string{
			tbl("public", "film_actor"): {"actor_id", "film_id"},
		},
		Skipped:           []ref.TableRef{tbl("public", "click_stream")},
		SchemaFingerprint: "9f1c3a7e2b0d4c55",
		ClassFingerprint:  "2d7e0b91a3c4f608",
		SnapshotID:        "00000004-0000004B-1",
		SecretFingerprint: "5c0a91be",
		ExtraPatterns: []pipeline.Pattern{
			{Name: "^nino$", Category: pipeline.CatNationalID, Confidence: pipeline.ConfCertain},
		},
		Columns: map[ref.ColumnRef]pipeline.ColumnConfig{
			col("public", "customer", "email"): {
				Category:   pipeline.CatEmail,
				Confidence: pipeline.ConfCertain,
				Reason:     "name matches email",
				Masker:     mask.MaskerEmail,
				Unique:     true,
				TypeFP:     "3e51a0c2",
			},
			// A quoted identifier: testdata/nasty.sql's trap 9. It has to
			// survive being written and read back as the same column.
			col("public", "LegacyCustomer", "EmailAddress"): {
				Category:   pipeline.CatEmail,
				Confidence: pipeline.ConfLikely,
				Reason:     "values look like addresses",
				Masker:     mask.MaskerEmail,
				TypeFP:     "3e51a0c2",
			},
			col("public", "film", "description"): {
				Category:   pipeline.CatFreeText,
				Confidence: pipeline.ConfPossible,
				Reason:     "name matches description",
				Masker:     mask.MaskerFreeText,
				TypeFP:     "3e51a0c2",
				Unmask: &pipeline.Unmask{
					Reason: "product catalogue text, no personal data",
					By:     "gareth",
					TypeFP: "3e51a0c2",
				},
			},
		},
		Plan: pipeline.PlanSummary{
			Tables:       15,
			Rows:         6412,
			Lookups:      []ref.TableRef{tbl("public", "country")},
			NotRecreated: map[string]int{"view": 7},
			SmallDomain:  []ref.ColumnRef{col("public", "people", "marital_status")},
		},
	}
}

// The file is a record of what happened and the next run's only input, so
// everything a re-run reads has to come back the same.
func TestWriteReadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lazyslice.yml")
	e := New(Options{})
	want := sample()

	if err := e.Write(path, want); err != nil {
		t.Fatalf("Write: %v", err)
	}
	got, err := e.Read(path)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}

	if got.Root != want.Root {
		t.Errorf("root = %v, want %v", got.Root, want.Root)
	}
	// The rung each endpoint came from, and the name that goes with it. They
	// are what an argument-free re-run reads back at rung 0 and writes out
	// again, so a round trip that loses them turns a committed
	// `from: compose` / `service: db` into `from: flag` on the next run
	// (T-0060).
	if got.Source != want.Source || got.SourceLabel != want.SourceLabel {
		t.Errorf("source provenance/label = %v/%q, want %v/%q",
			got.Source, got.SourceLabel, want.Source, want.SourceLabel)
	}
	if got.Target != want.Target || got.TargetLabel != want.TargetLabel {
		t.Errorf("target provenance/label = %v/%q, want %v/%q",
			got.Target, got.TargetLabel, want.Target, want.TargetLabel)
	}
	if !reflect.DeepEqual(got.SourceRef, want.SourceRef) || !reflect.DeepEqual(got.TargetRef, want.TargetRef) {
		t.Errorf("refs = %v/%v, want %v/%v", got.SourceRef, got.TargetRef, want.SourceRef, want.TargetRef)
	}
	// docs/reviews/2026-09-09/REVIEW.md finding 6: sslmode=verify-full and
	// sslrootcert have to survive the write-read round trip, not just the
	// four identity fields.
	if got.SourceRef.Params["sslmode"] != "verify-full" || got.SourceRef.Params["sslrootcert"] != "/etc/ssl/certs/pagila-ca.pem" {
		t.Errorf("source params = %v, want sslmode=verify-full and sslrootcert to survive", got.SourceRef.Params)
	}
	if got.Take != want.Take || got.Cap != want.Cap || got.Depth != want.Depth {
		t.Errorf("take/cap/depth = %d/%d/%d, want %d/%d/%d",
			got.Take, got.Cap, got.Depth, want.Take, want.Cap, want.Depth)
	}
	if got.MemoryBudget != want.MemoryBudget {
		t.Errorf("memory_budget = %d, want %d", got.MemoryBudget, want.MemoryBudget)
	}
	if got.Caps[tbl("public", "payment")] != 50 {
		t.Errorf("caps = %v, want public.payment: 50", got.Caps)
	}
	if len(got.Keys[tbl("public", "film_actor")]) != 2 {
		t.Errorf("keys = %v, want public.film_actor: [actor_id film_id]", got.Keys)
	}
	if len(got.Skipped) != 1 || got.Skipped[0] != tbl("public", "click_stream") {
		t.Errorf("skipped = %v, want [public.click_stream]", got.Skipped)
	}
	if got.SnapshotID != want.SnapshotID || got.SecretFingerprint != want.SecretFingerprint {
		t.Errorf("snapshot/secret = %q/%q, want %q/%q",
			got.SnapshotID, got.SecretFingerprint, want.SnapshotID, want.SecretFingerprint)
	}
	if len(got.ExtraPatterns) != 1 || got.ExtraPatterns[0].Confidence != pipeline.ConfCertain {
		t.Errorf("extra_patterns = %v, want the one pattern at certain", got.ExtraPatterns)
	}

	// The quoted identifier, and the opt-out, are the two entries a re-run
	// changes its behaviour over.
	legacy, ok := got.Columns[col("public", "LegacyCustomer", "EmailAddress")]
	if !ok {
		t.Fatalf(`public."LegacyCustomer"."EmailAddress" did not survive the round trip: %v`, keysOf(got))
	}
	if legacy.Category != pipeline.CatEmail || legacy.Confidence != pipeline.ConfLikely {
		t.Errorf("the quoted column came back as %s/%v", legacy.Category, legacy.Confidence)
	}
	film := got.Columns[col("public", "film", "description")]
	if film.Unmask == nil || film.Unmask.By != "gareth" || film.Unmask.TypeFP != "3e51a0c2" {
		t.Errorf("the opt-out came back as %+v, want reason, by and type", film.Unmask)
	}
	if got.Columns[col("public", "customer", "email")].Masker != mask.MaskerEmail {
		t.Error("the masker did not survive the round trip")
	}
}

// The file is committed and read by a person: the header says what it is, and
// the identifiers are spelled the way section 10 spells them.
func TestWrittenFileIsReadable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lazyslice.yml")
	if err := New(Options{}).Write(path, sample()); err != nil {
		t.Fatalf("Write: %v", err)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading it back: %v", err)
	}
	text := string(body)

	for _, want := range []string{
		"# lazyslice.yml — written by lazyslice 0.1.0 on ",
		"root: public.customer",
		"take: 200",
		"memory_budget: 256MiB",
		`public."LegacyCustomer"."EmailAddress"`,
		"small_domain:",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("the emitted file does not contain %q:\n%s", want, text)
		}
	}
}

// A followed virtual edge (§3.2 amended 2026-09-08, T-POLY) is rendered into
// `virtual_fks:` as text, never replayed as an input: Name Child (cols) ->
// Parent (cols).
func TestVirtualFKsLineForAnInferredEdge(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lazyslice.yml")
	cfg := sample()
	cfg.Plan.VirtualFKs = []pipeline.ForeignKey{
		{
			Name:       "public.attachments.owner_type",
			Child:      tbl("public", "attachments"),
			ChildCols:  []string{"owner_id"},
			Parent:     tbl("public", "people"),
			ParentCols: []string{"person_id"},
			Virtual:    true,
		},
	}
	if err := New(Options{}).Write(path, cfg); err != nil {
		t.Fatalf("Write: %v", err)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading it back: %v", err)
	}
	text := string(body)

	if !strings.Contains(text, "virtual_fks:") {
		t.Fatalf("the emitted file does not contain a virtual_fks: key:\n%s", text)
	}
	want := "public.attachments.owner_type public.attachments (owner_id) -> public.people (person_id)"
	if !strings.Contains(text, want) {
		t.Errorf("the emitted file does not contain the virtual_fks line %q:\n%s", want, text)
	}
}

// A --where predicate holding a literal is withheld and recorded only as a
// fingerprint (THREAT_MODEL.md T5). The writer refuses a config that carries
// one anyway, because the file is committed.
func TestWherePredicateIsWithheld(t *testing.T) {
	e := New(Options{})
	plan := &pipeline.Plan{Root: tbl("public", "customer"), Take: 10}
	cls := &pipeline.Classification{Decisions: map[ref.ColumnRef]pipeline.Decision{}}

	withLiteral, err := e.Emit(plan, cls, nil,
		pipeline.PlanRequest{Where: "created_at > '2024-01-01'"},
		pipeline.Candidate{}, pipeline.Candidate{}, "5c0a91be")
	if err != nil {
		t.Fatalf("Emit: %v", err)
	}
	if withLiteral.Where != "" {
		t.Errorf("where = %q, want it withheld", withLiteral.Where)
	}
	if withLiteral.WhereFingerprint != WhereFingerprint("created_at > '2024-01-01'") {
		t.Errorf("where_fingerprint = %q, want sha256(where)[:16]", withLiteral.WhereFingerprint)
	}

	// A predicate with no literal is recorded, because a later run can then
	// reproduce the slice with no flag at all.
	plain, err := e.Emit(plan, cls, nil,
		pipeline.PlanRequest{Where: "active"},
		pipeline.Candidate{}, pipeline.Candidate{}, "5c0a91be")
	if err != nil {
		t.Fatalf("Emit: %v", err)
	}
	if plain.Where != "active" {
		t.Errorf("where = %q, want it recorded", plain.Where)
	}

	path := filepath.Join(t.TempDir(), "lazyslice.yml")
	cfg := sample()
	cfg.Where = "created_at > '2024-01-01'"
	if err := e.Write(path, cfg); err == nil {
		t.Error("Write accepted a config carrying an unwithheld literal predicate")
	}
}

// The merge is what makes a committed opt-out survive the run that honours it:
// a Decision carries no reason and no author, so without the prior the next run
// would mask the column again.
func TestMergeCarriesTheOptOutForward(t *testing.T) {
	target := col("public", "film", "description")
	prior := &pipeline.Config{
		Columns: map[ref.ColumnRef]pipeline.ColumnConfig{
			target: {Unmask: &pipeline.Unmask{
				Reason: "product catalogue text, no personal data",
				By:     "gareth",
				TypeFP: "3e51a0c2",
			}},
		},
		ExtraPatterns: []pipeline.Pattern{{Name: "^nino$", Category: pipeline.CatNationalID}},
	}
	cls := &pipeline.Classification{Decisions: map[ref.ColumnRef]pipeline.Decision{
		target: {Col: target, Category: pipeline.CatFreeText, Source: pipeline.ByYmlUnmask, TypeFP: "3e51a0c2"},
	}}

	cfg, err := New(Options{Prior: prior}).Emit(
		&pipeline.Plan{Root: tbl("public", "film")}, cls, nil, pipeline.PlanRequest{},
		pipeline.Candidate{}, pipeline.Candidate{}, "")
	if err != nil {
		t.Fatalf("Emit: %v", err)
	}
	got := cfg.Columns[target]
	if got.Unmask == nil || got.Unmask.By != "gareth" {
		t.Fatalf("the opt-out was not carried forward: %+v", got.Unmask)
	}
	if len(cfg.ExtraPatterns) != 1 {
		t.Errorf("extra_patterns = %v, want the prior's one pattern", cfg.ExtraPatterns)
	}

	// An opt-out the classifier stopped honouring must not come back. classify
	// puts it in Expired and the decision is no longer ByYmlUnmask.
	cls.Decisions[target] = pipeline.Decision{Col: target, Source: pipeline.ByClassifier, Masked: true}
	expired, err := New(Options{Prior: prior}).Emit(
		&pipeline.Plan{Root: tbl("public", "film")}, cls, nil, pipeline.PlanRequest{},
		pipeline.Candidate{}, pipeline.Candidate{}, "")
	if err != nil {
		t.Fatalf("Emit: %v", err)
	}
	if expired.Columns[target].Unmask != nil {
		t.Error("an expired opt-out was written back, so it would never expire")
	}
}

// --unmask's own opt-out is recorded with `by: flag` and this run's type
// fingerprint, so the next run needs no flag and the opt-out still expires when
// the column changes.
func TestFlagOptOutIsRecorded(t *testing.T) {
	target := col("public", "film", "description")
	cls := &pipeline.Classification{Decisions: map[ref.ColumnRef]pipeline.Decision{
		target: {Col: target, Source: pipeline.ByFlagUnmask, TypeFP: "7b20e9d1"},
	}}

	cfg, err := New(Options{Unmask: map[ref.ColumnRef]string{target: "ticket 42"}}).Emit(
		&pipeline.Plan{Root: tbl("public", "film")}, cls, nil, pipeline.PlanRequest{},
		pipeline.Candidate{}, pipeline.Candidate{}, "")
	if err != nil {
		t.Fatalf("Emit: %v", err)
	}
	got := cfg.Columns[target].Unmask
	if got == nil || got.Reason != "ticket 42" || got.By != "flag" || got.TypeFP != "7b20e9d1" {
		t.Errorf("the flag's opt-out was recorded as %+v", got)
	}
}

func TestParseAndFormatSize(t *testing.T) {
	cases := []struct {
		text string
		n    int64
	}{
		{"256MiB", 256 << 20},
		{"1GiB", 1 << 30},
		{"512KiB", 512 << 10},
		{"1024", 1024},
	}
	for _, c := range cases {
		got, err := ParseSize(c.text)
		if err != nil || got != c.n {
			t.Errorf("ParseSize(%q) = %d, %v, want %d", c.text, got, err, c.n)
		}
	}
	if got := FormatSize(256 << 20); got != "256MiB" {
		t.Errorf("FormatSize(256MiB) = %q", got)
	}
	if _, err := ParseSize("lots"); err == nil {
		t.Error("ParseSize accepted a size that is not one")
	}
}

func keysOf(c *pipeline.Config) []string {
	out := make([]string, 0, len(c.Columns))
	for k := range c.Columns {
		out = append(out, k.String())
	}
	return out
}
