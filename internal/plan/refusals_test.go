// SPDX-License-Identifier: Apache-2.0

package plan

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
	"github.com/Liarea/lazyslice/mask"
)

// Collecting refusals, held without a database (T-0318).
//
// Dogfood session 1 took nine runs to reach a green verify, each one refused
// on the next single cause in isolation: one join table with no identity (a
// second, identical one was implied by the schema but not named until the
// first was cleared), then ten masked columns under unique indexes named one
// per run. This file is the fixture that reproduces the shape of both halves
// at once and pins that a single Plan call now finds all of it: two Rails-
// shaped join tables with no derivable identity, and two masked columns under
// a unique index too narrow for any registered generator to fill.
//
// It is held without a database the way writeback_test.go and unique_test.go
// hold their own checks: neither half needs one. A table with no primary key,
// no usable unique index and no NOT NULL foreign-key or discriminator column
// has nothing for resolveIdentity's pseudo-key rung to probe, so it refuses
// before a statement is ever built; a masked column's planned row count is
// faked the way uniqueRun (unique_test.go) fakes it, through Lookup/LookupRows,
// which is the shortest honest way to give checkUniqueDomain an n without
// walking a schema that needs one.

// noIdentityJoinTable is a Rails habtm join table: two NOT NULL foreign-key
// columns, no primary key, no unique index. Both columns are pseudo-key
// candidates (§3.4), but nothing here probes them -- the fixture never gives
// this table a parent whose identity can be joined against under a real
// database, and neither test below runs the walk that would need one; both
// call resolveIdentities directly, which only reaches the probe if the PK and
// unique-index rungs above it were empty, exactly as they are here.
func noIdentityJoinTable(name string, leftCol, rightCol string) pipeline.Table {
	t := ref.TableRef{Schema: "public", Name: name}
	return pipeline.Table{
		Ref: t,
		Columns: []pipeline.Column{
			{Name: leftCol, TypeName: "bigint", TypeOID: 20, Nullable: false},
			{Name: rightCol, TypeName: "bigint", TypeOID: 20, Nullable: false},
		},
	}
}

// narrowUniqueColumnTable is a table with one masked column under a unique
// index whose declared width has no room for credential_unique's prefix and a
// distinguishing suffix -- unique_test.go's TestUniqueDomainRefusesWhenNoMaskerFits
// builds the identical shape as a single table; this is the same column type
// twice over, under two different table names, so the fixture below can
// collect two independent unique_domain refusals in one run.
func narrowUniqueColumnTable(name string) (pipeline.Table, ref.ColumnRef) {
	t := ref.TableRef{Schema: "public", Name: name}
	col := ref.ColumnRef{Table: t, Column: "handle"}
	return pipeline.Table{
		Ref: t,
		Columns: []pipeline.Column{
			{Name: "id", TypeName: "bigint", TypeOID: 20},
			// typmod 22 is varchar(18): len("lazyslice-invalid-") with nothing
			// after it -- the narrowest column credential_unique cannot widen
			// into, the same reduction unique_test.go's uniqueRun uses.
			{Name: "handle", TypeName: "character varying(18)", TypMod: 22},
		},
		PK: []string{"id"},
	}, col
}

// twoNoIdentityAndTwoUniqueDomainRun builds a *run carrying both halves of the
// fixture, indexed the way build() would have left it (p.tables, p.byRef,
// p.inScope), so resolveIdentities and checkUniqueDomain can be called
// exactly as plan() calls them.
func twoNoIdentityAndTwoUniqueDomainRun(t *testing.T) (p *run, joins []ref.TableRef, uniques []ref.ColumnRef) {
	t.Helper()

	j1 := noIdentityJoinTable("assemblies_parts", "assembly_id", "part_id")
	j2 := noIdentityJoinTable("assemblies_kits", "assembly_id", "kit_id")
	u1, c1 := narrowUniqueColumnTable("api_tokens")
	u2, c2 := narrowUniqueColumnTable("session_tokens")

	tables := []pipeline.Table{j1, j2, u1, u2}
	decisions := map[ref.ColumnRef]pipeline.Decision{
		c1: {Col: c1, Category: pipeline.CatCredential, Masker: mask.CredentialMasker, Masked: true, UniqueIndex: true},
		c2: {Col: c2, Category: pipeline.CatCredential, Masker: mask.CredentialMasker, Masked: true, UniqueIndex: true},
	}

	p = &run{
		schema:   &pipeline.Schema{Tables: tables},
		tables:   tables,
		byRef:    map[ref.TableRef]*pipeline.Table{},
		inScope:  map[ref.TableRef]bool{},
		outgoing: map[ref.TableRef][]pipeline.ForeignKey{},
		incoming: map[ref.TableRef][]pipeline.ForeignKey{},
		ids:      map[ref.TableRef]identity{},
		why:      map[ref.TableRef]string{},
		cls:      &pipeline.Classification{Decisions: decisions},
		// checkUniqueDomain reads n from p.plannedRows, which for a
		// non-lookup table is p.selected[t].Len(): a lookup table's planned
		// row count is measured once, before the walk, and never depends on
		// keys the walk would otherwise have to read (unique_test.go's own
		// uniqueRun does the same).
		lookups:    map[ref.TableRef]bool{u1.Ref: true, u2.Ref: true},
		lookupRows: map[ref.TableRef]int64{u1.Ref: 200, u2.Ref: 200},
		selected:   map[ref.TableRef]*keys{},
	}
	for i := range p.tables {
		p.byRef[p.tables[i].Ref] = &p.tables[i]
		p.inScope[p.tables[i].Ref] = true
	}
	return p, []ref.TableRef{j1.Ref, j2.Ref}, []ref.ColumnRef{c1, c2}
}

// TestCollectsTwoNoIdentityAndTwoUniqueDomainRefusalsInOneRun is T-0318's own
// pin: resolveIdentities does not stop at the first join table with no
// identity, and checkUniqueDomain does not stop at the first narrow unique
// column, so a schema with both defects finds all four causes the moment
// either check runs over it -- not over nine runs, one cause at a time.
func TestCollectsTwoNoIdentityAndTwoUniqueDomainRefusalsInOneRun(t *testing.T) {
	t.Parallel()
	p, joins, uniques := twoNoIdentityAndTwoUniqueDomainRun(t)

	// plan() calls these two, in this order, before it ever reaches assemble;
	// this test drives them the same way rather than through Plan itself,
	// because nothing else this fixture would need (a privilege read, a seed
	// read) has anything to do with what T-0318 changed.
	if err := p.resolveIdentities(context.Background()); err != nil {
		t.Fatalf("resolveIdentities returned an error instead of collecting: %v", err)
	}
	if err := p.checkUniqueDomain(); err != nil {
		t.Fatalf("checkUniqueDomain returned an error instead of collecting: %v", err)
	}

	if len(p.refusals) != 4 {
		t.Fatalf("collected %d refusals, want 4 (two no_identity, two unique_domain): %v",
			len(p.refusals), p.refusals)
	}

	var gotNoIdentity, gotUniqueDomain []ref.TableRef
	for _, r := range p.refusals {
		switch r.Code {
		case CodeNoIdentity:
			gotNoIdentity = append(gotNoIdentity, r.Table)
			if r.Exit != exitPlan {
				t.Errorf("%s: Exit = %d, want %d", r.Table, r.Exit, exitPlan)
			}
		case CodeUniqueDomain:
			gotUniqueDomain = append(gotUniqueDomain, r.Table)
			if r.Exit != exitPlan {
				t.Errorf("%s: Exit = %d, want %d", r.Table, r.Exit, exitPlan)
			}
		default:
			t.Errorf("unexpected refusal code %s for %s", r.Code, r.Table)
		}
	}
	if len(gotNoIdentity) != 2 || len(gotUniqueDomain) != 2 {
		t.Fatalf("got %d no_identity and %d unique_domain refusals, want 2 and 2",
			len(gotNoIdentity), len(gotUniqueDomain))
	}
	for _, want := range joins {
		found := false
		for _, got := range gotNoIdentity {
			found = found || got == want
		}
		if !found {
			t.Errorf("no_identity refusals do not name %s", want)
		}
	}
	for _, want := range uniques {
		found := false
		for _, got := range gotUniqueDomain {
			found = found || got == want.Table
		}
		if !found {
			t.Errorf("unique_domain refusals do not name %s", want.Table)
		}
	}

	// resolveIdentities ran first, so the first refusal collected -- the one
	// core.reportPlanRefusals treats as the run's own Code and Exit -- is a
	// no_identity refusal, and both dropped join tables left the plan's
	// scope rather than blocking the rest of the pass.
	if p.refusals[0].Code != CodeNoIdentity {
		t.Errorf("first collected refusal = %s, want %s (resolveIdentities runs first)",
			p.refusals[0].Code, CodeNoIdentity)
	}
	for _, j := range joins {
		if p.inScope[j] {
			t.Errorf("%s is still in scope after its no_identity refusal was collected", j)
		}
	}

	// Refusals.Error() is what a bare %v -- --debug's trail, a test failure --
	// renders, and it has to carry every member or a caller reading only that
	// string still sees one cause.
	msg := p.refusals.Error()
	for _, want := range joins {
		if !strings.Contains(msg, want.String()) {
			t.Errorf("Refusals.Error() does not mention %s: %s", want, msg)
		}
	}
	for _, want := range uniques {
		if !strings.Contains(msg, want.Table.String()) {
			t.Errorf("Refusals.Error() does not mention %s: %s", want.Table, msg)
		}
	}
}

// TestNoIdentityHintNamesTheColumnsOfASmallTable is the other half of T-0318:
// a table of three columns or fewer that reaches CodeNoIdentity gets its own
// columns in the --key hint, not the generic col,col placeholder -- a Rails
// join table's composite key is exactly its own columns, and this fixture's
// two habtm tables are the shape the hint exists for.
func TestNoIdentityHintNamesTheColumnsOfASmallTable(t *testing.T) {
	t.Parallel()
	p, _, _ := twoNoIdentityAndTwoUniqueDomainRun(t)

	if err := p.resolveIdentities(context.Background()); err != nil {
		t.Fatalf("resolveIdentities returned an error instead of collecting: %v", err)
	}
	if len(p.refusals) != 2 {
		t.Fatalf("collected %d refusals, want 2 (only the two no-identity join tables)", len(p.refusals))
	}

	want := map[string]string{
		"public.assemblies_parts": "--key public.assemblies_parts=assembly_id,part_id or --skip-table public.assemblies_parts",
		"public.assemblies_kits":  "--key public.assemblies_kits=assembly_id,kit_id or --skip-table public.assemblies_kits",
	}
	for _, r := range p.refusals {
		hint, ok := want[r.Table.String()]
		if !ok {
			t.Fatalf("unexpected refusal for %s", r.Table)
		}
		if !strings.Contains(r.Message, hint) {
			t.Errorf("%s message = %q, want it to carry %q", r.Table, r.Message, hint)
		}
		if got := r.Args["flag"]; got != hint {
			t.Errorf("%s args[flag] = %q, want %q", r.Table, got, hint)
		}
		if strings.Contains(r.Message, "col,col") {
			t.Errorf("%s message still carries the generic col,col placeholder: %q", r.Table, r.Message)
		}
	}
}

// TestKeyHintFallsBackToThePlaceholderPastThreeColumns is the guard against
// the hint over-promising: a table past three columns gives no reliable
// composite-key guess, so the generic placeholder stays.
func TestKeyHintFallsBackToThePlaceholderPastThreeColumns(t *testing.T) {
	t.Parallel()
	tbl := &pipeline.Table{
		Ref: ref.TableRef{Schema: "public", Name: "wide"},
		Columns: []pipeline.Column{
			{Name: "a", TypeName: "bigint"},
			{Name: "b", TypeName: "bigint"},
			{Name: "c", TypeName: "bigint"},
			{Name: "d", TypeName: "bigint"},
		},
	}
	got := keyHint(tbl, nil, nil)
	want := "--key public.wide=col,col or --skip-table public.wide"
	if got != want {
		t.Errorf("keyHint = %q, want %q", got, want)
	}
}

// TestKeyHintFallsBackToThePlaceholderWhenAColumnIsNullable is the T-0318
// review's high finding: smallTableKeyColumns must never suggest a nullable
// column, foreign key or not. `create_table :a_b, id: false { t.belongs_to
// :a; t.belongs_to :b }` is the ordinary Rails shape that produces exactly
// this -- two nullable foreign-key columns and no primary key -- and pasting
// a --key naming both is testdata/README.md trap 12 without the exit 12:
// count(DISTINCT (a, b)) counts (NULL, 1) and (NULL, 2) as distinct, the
// explicit key "passes", and readKeys then drops every row with a NULL in a
// key column with no refusal and no warning.
func TestKeyHintFallsBackToThePlaceholderWhenAColumnIsNullable(t *testing.T) {
	t.Parallel()
	tbl := &pipeline.Table{
		Ref: ref.TableRef{Schema: "public", Name: "habtm_nullable"},
		Columns: []pipeline.Column{
			{Name: "a_id", TypeName: "bigint", TypeOID: 20},
			{Name: "b_id", TypeName: "bigint", TypeOID: 20, Nullable: true},
		},
	}
	got := keyHint(tbl, nil, nil)
	want := "--key public.habtm_nullable=col,col or --skip-table public.habtm_nullable"
	if got != want {
		t.Errorf("keyHint = %q, want %q: a nullable column must never enter the suggested key", got, want)
	}
}

// TestKeyHintFallsBackToThePlaceholderWhenAColumnIsIncomparable is the other
// half of the same finding: a NOT NULL column of a type the key encoding has
// no default btree opclass for -- json, xml, a geometric type -- does not
// fail with a refusal if it is suggested, it dies inside pgx with "could not
// identify a comparison function", which carries none of §3.4's remedies.
func TestKeyHintFallsBackToThePlaceholderWhenAColumnIsIncomparable(t *testing.T) {
	t.Parallel()
	tbl := &pipeline.Table{
		Ref: ref.TableRef{Schema: "public", Name: "habtm_json"},
		Columns: []pipeline.Column{
			{Name: "a_id", TypeName: "bigint", TypeOID: 20},
			{Name: "payload", TypeName: "json"},
		},
	}
	got := keyHint(tbl, nil, nil)
	want := "--key public.habtm_json=col,col or --skip-table public.habtm_json"
	if got != want {
		t.Errorf("keyHint = %q, want %q: a json column has no default btree opclass and must never "+
			"enter the suggested key", got, want)
	}
}

// TestKeyHintFallsBackWhenThePseudoKeyProbeAlreadyFailedOnTheSameColumns is
// the T-0318 review's other finding: a join table whose FK columns are
// already the pseudo-key rung's own candidate reaches this hint only after
// those exact columns failed a uniqueness probe, so suggesting them back
// spends a whole run confirming what this run already found. keyHint takes
// the rung's own probed-and-failed set and, when its own suggestion would
// name the same columns (or a subset), offers --skip-table alone rather than
// a --key that would fail identically on the next run.
func TestKeyHintFallsBackWhenThePseudoKeyProbeAlreadyFailedOnTheSameColumns(t *testing.T) {
	t.Parallel()
	tbl := &pipeline.Table{
		Ref: ref.TableRef{Schema: "public", Name: "narrow"},
		Columns: []pipeline.Column{
			{Name: "a_id", TypeName: "bigint", TypeOID: 20},
			{Name: "b_id", TypeName: "bigint", TypeOID: 20},
		},
	}
	got := keyHint(tbl, nil, []string{"a_id", "b_id"})
	want := "--skip-table public.narrow"
	if got != want {
		t.Errorf("keyHint = %q, want %q: the probe already refused this exact column set", got, want)
	}
}

// TestAFailFastRefusalJoinsWhatWasAlreadyCollected is T-0318's other corner:
// a rare combination of one collected skip_parent refusal and one fail-fast
// unreadable-parent refusal in the same run must still name both, not only
// the fail-fast one. orders has two parents; --skip-table names one of them
// (warehouses, collected: the slice needs it), and the role cannot read the
// other (customers, fail-fast: §3.6 never collects an unreadable parent,
// because it is not one of the four types T-0318 named). Without p.fail
// wrapping applySkipAndPrivileges's own return, plan() would return the bare
// unreadable refusal and the already-collected skip_parent one would never
// reach core at all.
func TestAFailFastRefusalJoinsWhatWasAlreadyCollected(t *testing.T) {
	t.Parallel()
	root := ref.TableRef{Schema: "public", Name: "orders"}
	customers := ref.TableRef{Schema: "public", Name: "customers"}
	warehouses := ref.TableRef{Schema: "public", Name: "warehouses"}
	schema := &pipeline.Schema{
		Tables: []pipeline.Table{
			{
				Ref: root,
				Columns: []pipeline.Column{
					{Name: "id", TypeName: "bigint", TypeOID: 20},
					{Name: "customer_id", TypeName: "bigint", TypeOID: 20},
					{Name: "warehouse_id", TypeName: "bigint", TypeOID: 20},
				},
				PK: []string{"id"},
			},
			{Ref: customers, Columns: []pipeline.Column{{Name: "id", TypeName: "bigint", TypeOID: 20}}, PK: []string{"id"}},
			{Ref: warehouses, Columns: []pipeline.Column{{Name: "id", TypeName: "bigint", TypeOID: 20}}, PK: []string{"id"}},
		},
		FKs: []pipeline.ForeignKey{
			{
				Name: "orders_customer_id_fkey", Child: root, ChildCols: []string{"customer_id"},
				Parent: customers, ParentCols: []string{"id"}, Validated: true,
			},
			{
				Name: "orders_warehouse_id_fkey", Child: root, ChildCols: []string{"warehouse_id"},
				Parent: warehouses, ParentCols: []string{"id"}, Validated: true,
			},
		},
	}
	req := pipeline.PlanRequest{
		Root: &root,
		Skip: []ref.TableRef{warehouses},
		Priv: pipeline.RolePrivileges{Role: "lazyslice_ro", Unreadable: []ref.TableRef{customers}},
	}

	_, err := New().Plan(context.Background(), &countingReader{}, schema, nil, req)

	var refusals Refusals
	if !errors.As(err, &refusals) {
		t.Fatalf("Plan returned %v, want plan.Refusals carrying both causes", err)
	}
	if len(refusals) != 2 {
		t.Fatalf("collected %d refusals, want 2 (skip_parent then unreadable): %v", len(refusals), refusals)
	}
	if refusals[0].Code != CodeSkipParent {
		t.Errorf("first refusal = %s, want %s (applySkipAndPrivileges collects it before it reaches the unreadable loop)",
			refusals[0].Code, CodeSkipParent)
	}
	if refusals[0].Table != warehouses {
		t.Errorf("first refusal names %s, want %s", refusals[0].Table, warehouses)
	}
	if refusals[1].Code != CodeUnreadable {
		t.Errorf("second refusal = %s, want %s (the fail-fast cause, joined rather than replacing the first)",
			refusals[1].Code, CodeUnreadable)
	}
	if refusals[1].Table != customers {
		t.Errorf("second refusal names %s, want %s", refusals[1].Table, customers)
	}
}

// TestKeyHintSkipsAGeneratedColumn: the target recomputes a generated column,
// so it can never be part of a --key an operator passes on the source, the
// same exclusion §3.4's pseudo-key rung already makes.
func TestKeyHintSkipsAGeneratedColumn(t *testing.T) {
	t.Parallel()
	tbl := &pipeline.Table{
		Ref: ref.TableRef{Schema: "public", Name: "narrow"},
		Columns: []pipeline.Column{
			{Name: "a_id", TypeName: "bigint", TypeOID: 20},
			{Name: "b_id", TypeName: "bigint", TypeOID: 20},
			{Name: "full_label", TypeName: "text", Generated: "a_id::text || b_id::text"},
		},
	}
	got := keyHint(tbl, nil, nil)
	want := "--key public.narrow=a_id,b_id or --skip-table public.narrow"
	if got != want {
		t.Errorf("keyHint = %q, want %q (the generated column left out)", got, want)
	}
}
