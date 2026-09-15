// SPDX-License-Identifier: Apache-2.0

package plan

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
)

// The 2026-09-15 red team's A4b, A11, A12 and A20: personal data in the schema
// rather than in a row. Each of these exited 0 with the value in the target's
// catalog, and A12's would have been materialised back into a row by the
// application's next INSERT that omitted the column.

// typeSchema is one ordinary table plus whichever type the case is about.
func typeSchema(enums map[string][]string, domains []pipeline.NamedDef) (ref.TableRef, *pipeline.Schema) {
	t := ref.TableRef{Schema: "public", Name: "tickets"}
	return t, &pipeline.Schema{
		Enums:   enums,
		Domains: domains,
		Tables: []pipeline.Table{{
			Ref: t,
			Columns: []pipeline.Column{
				{Name: "id", TypeName: "bigint", TypeOID: 20},
				{Name: "title", TypeName: "text"},
			},
			PK: []string{"id"},
		}},
	}
}

// planWithRequest is planLiteralsWithRequest for a test that needs the *plan*
// as well as the error: Plan.AllowedTypeLiterals is what internal/verify reads.
func planWithRequest(
	t *testing.T,
	schema *pipeline.Schema,
	cls *pipeline.Classification,
	req pipeline.PlanRequest,
) (*pipeline.Plan, error) {
	t.Helper()
	return New().Plan(context.Background(), &countingReader{}, schema, cls, req)
}

func TestRedTeamTypeLiteralsAreRefused(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		enums   map[string][]string
		domains []pipeline.NamedDef
		secret  string
		object  string
	}{
		{
			// A11: an enum label is a DDL string literal recreated verbatim by
			// internal/load/ddl's CREATE TYPE ... AS ENUM.
			name:   "an email address in an enum label",
			enums:  map[string][]string{"public.assignee": {"unassigned", "enum.canary@bigcorp.com"}},
			secret: "enum.canary@bigcorp.com",
			object: "label 2",
		},
		{
			name:   "a phone number in an enum label",
			enums:  map[string][]string{"public.assignee": {"unassigned", "+1-415-555-0199"}},
			secret: "+1-415-555-0199",
			object: "label 2",
		},
		{
			// A12: a domain's DEFAULT lives in pg_type.typdefault, one catalog
			// table to the left of pg_attrdef.
			name: "an email address in a domain default",
			domains: []pipeline.NamedDef{{
				Name: "public.tenant_d",
				Def:  `CREATE DOMAIN "public"."tenant_d" AS text DEFAULT 'domdefault.canary@bigcorp.com'`,
			}},
			secret: "domdefault.canary@bigcorp.com",
			object: "its definition",
		},
		{
			// A07: a domain's CHECK. internal/verify reads it through
			// pg_constraint; internal/plan read no domain at all.
			name: "an email address in a domain CHECK",
			domains: []pipeline.NamedDef{{
				Name: "public.label_d",
				Def:  `CREATE DOMAIN "public"."label_d" AS text CHECK (VALUE <> 'domain.canary@bigcorp.com')`,
			}},
			secret: "domain.canary@bigcorp.com",
			object: "its definition",
		},
		{
			// A20: the strong set was three validators, and THREAT_MODEL.md T1
			// gave "a CHECK is full of English words" as the reason. That
			// reason does not reach a strict pattern or a mod-97 checksum.
			name: "a national identifier in a domain CHECK",
			domains: []pipeline.NamedDef{{
				Name: "public.code_d",
				Def:  `CREATE DOMAIN "public"."code_d" AS text CHECK (VALUE <> '123-45-6789')`,
			}},
			secret: "123-45-6789",
			object: "its definition",
		},
		{
			name: "an IBAN in a domain CHECK",
			domains: []pipeline.NamedDef{{
				Name: "public.iban_d",
				Def:  `CREATE DOMAIN "public"."iban_d" AS text CHECK (VALUE <> 'GB33BUKB20201555555555')`,
			}},
			secret: "GB33BUKB20201555555555",
			object: "its definition",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tbl, schema := typeSchema(tc.enums, tc.domains)
			cls := masking(tbl, "title", pipeline.CatFreeText, "free_text")
			err := planLiterals(t, schema, cls, literalKey(0x22))

			var refusal *Refusal
			if !errors.As(err, &refusal) {
				t.Fatalf("Plan returned %v, want a *plan.Refusal; the value is in the target's catalog", err)
			}
			if refusal.Code != CodeTypeLiteral || refusal.Exit != exitSchema {
				t.Fatalf("Plan refused with %s exit %d, want %s exit %d",
					refusal.Code, refusal.Exit, CodeTypeLiteral, exitSchema)
			}
			if refusal.Column != tc.object {
				t.Errorf("the refusal names %q, want %q", refusal.Column, tc.object)
			}
			// THREAT_MODEL.md T4: the position, never the text.
			if strings.Contains(refusal.Message, tc.secret) ||
				strings.Contains(refusal.Args["reason"], tc.secret) ||
				strings.Contains(refusal.Args["table"], tc.secret) ||
				strings.Contains(refusal.Args["column"], tc.secret) {
				t.Errorf("the refusal quotes the value: %q / %v", refusal.Message, refusal.Args)
			}
		})
	}
}

// The escape (the T-REDFIX review's fourth finding). Before --allow-type-literal
// existed this refusal had none: it named --skip-table, which drops a table to
// *schema only* and still recreates every type, so a source schema with one
// canary label could not be sliced at all. The opt-out carries a reason, it is
// resolved against the source's own type names in internal/core, and
// internal/verify's catalog pass honours the same list through
// Plan.AllowedTypeLiterals — otherwise the run would load and then refuse at
// exit 9 over the object the operator was told they had allowed.
func TestAllowTypeLiteralClearsTheRefusal(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		enums   map[string][]string
		domains []pipeline.NamedDef
		allow   string
	}{
		{
			name:  "an enum label",
			enums: map[string][]string{"public.assignee": {"unassigned", "enum.canary@bigcorp.com"}},
			allow: "public.assignee",
		},
		{
			name: "a domain definition",
			domains: []pipeline.NamedDef{{
				Name: "public.tenant_d",
				Def:  `CREATE DOMAIN "public"."tenant_d" AS text DEFAULT 'domdefault.canary@bigcorp.com'`,
			}},
			allow: "public.tenant_d",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tbl, schema := typeSchema(tc.enums, tc.domains)
			cls := masking(tbl, "title", pipeline.CatFreeText, "free_text")
			root := schema.Tables[0].Ref
			req := pipeline.PlanRequest{
				Root:              &root,
				Key:               literalKey(0x22),
				AllowTypeLiterals: map[string]string{tc.allow: "a product label, checked by hand"},
			}
			plan, err := planWithRequest(t, schema, cls, req)
			if err != nil {
				t.Fatalf("Plan refused a type the operator opted out of: %v", err)
			}
			if plan == nil {
				t.Fatal("Plan returned no plan and no error")
			}
			if len(plan.AllowedTypeLiterals) != 1 || plan.AllowedTypeLiterals[0] != tc.allow {
				t.Fatalf("Plan.AllowedTypeLiterals = %v, want [%s]: internal/verify's catalog pass "+
					"reads this and refuses the same object at exit 9 without it",
					plan.AllowedTypeLiterals, tc.allow)
			}
		})
	}
}

// An opt-out naming a type this schema does not carry is not written onto the
// plan: internal/core refuses such a name at exit 2, and a plan that told
// internal/verify to exempt an object nobody named would be an exemption with
// no operator behind it.
func TestAllowTypeLiteralForAnAbsentTypeIsNotRecorded(t *testing.T) {
	t.Parallel()
	tbl, schema := typeSchema(map[string][]string{"public.order_status": {"new", "paid"}}, nil)
	cls := masking(tbl, "title", pipeline.CatFreeText, "free_text")
	root := schema.Tables[0].Ref
	plan, err := planWithRequest(t, schema, cls, pipeline.PlanRequest{
		Root:              &root,
		Key:               literalKey(0x22),
		AllowTypeLiterals: map[string]string{"public.nosuchtype": "typo"},
	})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if len(plan.AllowedTypeLiterals) != 0 {
		t.Fatalf("Plan.AllowedTypeLiterals = %v, want none", plan.AllowedTypeLiterals)
	}
}

// The other side: an ordinary enum and an ordinary domain must not refuse a
// run. A status enum and a money domain are what every real schema carries,
// and a check that refused them would be routed around rather than fixed.
func TestOrdinaryTypesAreNotRefused(t *testing.T) {
	t.Parallel()
	tbl, schema := typeSchema(
		map[string][]string{"public.order_status": {"new", "paid", "shipped", "cancelled"}},
		[]pipeline.NamedDef{
			{Name: "public.money_amount", Def: `CREATE DOMAIN "public"."money_amount" AS numeric(10,2)`},
			{Name: "public.postcode", Def: `CREATE DOMAIN "public"."postcode" AS text CHECK (VALUE ~ '^[A-Z]{1,2}[0-9]')`},
			// An all-caps run that clears the mod-97 check but carries no ISO
			// 13616 check digits in positions three and four. Five of pagila's
			// own film titles are this shape, which is why ValidIBAN had to be
			// tightened before it could join the strong set.
			{Name: "public.title_d", Def: `CREATE DOMAIN "public"."title_d" AS text CHECK (VALUE <> 'CHARIOTSCONSPIRACY')`},
		},
	)
	cls := masking(tbl, "title", pipeline.CatFreeText, "free_text")
	if err := planLiterals(t, schema, cls, literalKey(0x22)); err != nil {
		t.Fatalf("Plan refused an ordinary schema: %v", err)
	}
}
