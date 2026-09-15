// SPDX-License-Identifier: Apache-2.0

package load

import (
	"context"
	"strings"
	"testing"

	"github.com/Liarea/lazyslice/internal/pipeline"
)

// The 2026-09-15 red team's A07. THREAT_MODEL.md T8's amendment claimed that
// after a content-class verify failure the target ends the run "either empty or
// holding nothing this run wrote". DropLoaded iterated ddl.DropTables and
// nothing else, so a domain whose CHECK carried an address — the very object
// internal/verify's catalog pass refused the run over — was still in the target
// afterwards, and still there on every rerun.
func TestDropLoadedDropsTheTypesTheRunCreated(t *testing.T) {
	schema := &pipeline.Schema{
		Enums:   map[string][]string{"public.assignee": {"unassigned", "enum.canary@bigcorp.com"}},
		Domains: []pipeline.NamedDef{{Name: "public.label_d", Def: `CREATE DOMAIN "public"."label_d" AS text`}},
		Tables: []pipeline.Table{{
			Ref: tref("public", "accounts"),
			Columns: []pipeline.Column{
				{Name: "id", TypeName: "bigint"},
				{Name: "label", TypeName: "public.label_d", Domain: "public.label_d"},
			},
			PK: []string{"id"},
		}},
	}

	w := &fakeWriter{}
	if err := DropLoaded(context.Background(), w, schema, nil); err != nil {
		t.Fatalf("DropLoaded: %v", err)
	}

	log := strings.Join(w.log(), "\n")
	for _, want := range []string{
		`DROP TABLE IF EXISTS "public"."accounts" CASCADE`,
		`DROP DOMAIN IF EXISTS "public"."label_d"`,
		`DROP TYPE IF EXISTS "public"."assignee"`,
	} {
		if !strings.Contains(log, want) {
			t.Errorf("the quarantine never ran %q; it left an object this run created in the target:\n%s",
				want, log)
		}
	}
	// Order matters: a type is dropped after the tables that depend on it,
	// because there is no CASCADE on the object drops.
	table := strings.Index(log, `DROP TABLE IF EXISTS "public"."accounts"`)
	domain := strings.Index(log, `DROP DOMAIN IF EXISTS "public"."label_d"`)
	if table < 0 || domain < 0 || domain < table {
		t.Errorf("the domain was dropped before the table that uses it:\n%s", log)
	}
}
