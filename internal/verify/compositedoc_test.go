// SPDX-License-Identifier: Apache-2.0

package verify

import (
	"strings"
	"testing"

	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
)

// compositeDocuments (compositedoc.go, T-0399) is verify's mirror of
// internal/classify's own structural check: a composite column reaching the
// target whose type holds a json, jsonb or hstore field -- or a nested
// composite that does -- is refused here too, in case a copy somehow reached
// the target with one despite internal/plan's own exit-12 refusal
// (THREAT_MODEL.md T1, the 2026-09-25 JSON red team round 1's entry 14). It
// needs no database: the schema Schema.Composites already carries is the same
// catalog read internal/plan used, since ARCHITECTURE.md §11.1 recreates a
// composite verbatim.

func documentCompositeSchema() *pipeline.Schema {
	return &pipeline.Schema{
		Composites: []pipeline.NamedDef{
			{Name: "public.wrap", Def: "CREATE TYPE public.wrap AS (tag text, doc jsonb)"},
			{Name: "public.plain", Def: "CREATE TYPE public.plain AS (a text, b text)"},
		},
	}
}

func documentCompositeTable(cols ...pipeline.Column) (ref.TableRef, map[ref.TableRef]*pipeline.Table) {
	t := ref.TableRef{Schema: "public", Name: "widgets"}
	tab := &pipeline.Table{Ref: t, Columns: cols}
	return t, map[ref.TableRef]*pipeline.Table{t: tab}
}

func TestCompositeDocumentsRefusesAColumnHoldingADocumentField(t *testing.T) {
	t.Parallel()
	tbl, tables := documentCompositeTable(
		pipeline.Column{Name: "id", TypeName: "bigint"},
		pipeline.Column{Name: "w", TypeName: "public.wrap"},
	)
	s := &state{
		schema: documentCompositeSchema(),
		tables: tables,
		steps:  []pipeline.Step{{Table: tbl}},
		cls:    &pipeline.Classification{Decisions: map[ref.ColumnRef]pipeline.Decision{}},
	}
	s.compositeDocuments()

	if !s.failed(checkCatalog) {
		t.Fatalf("compositeDocuments did not fail %s", checkCatalog)
	}
	r := s.firstFailure()
	if r == nil {
		t.Fatalf("no failure recorded")
	}
	if r.Code != CodeRefusedCompositeDocument || r.Exit != exitResidual {
		t.Errorf("got code %s exit %d, want %s exit %d", r.Code, r.Exit, CodeRefusedCompositeDocument, exitResidual)
	}
	if r.Table != tbl || r.Column != "w" {
		t.Errorf("Table/Column = %s/%s, want %s/w", r.Table, r.Column, tbl)
	}
	for _, want := range []string{"public.wrap", "jsonb", "doc"} {
		if !strings.Contains(r.Reason, want) {
			t.Errorf("reason %q does not name %q", r.Reason, want)
		}
	}
}

func TestCompositeDocumentsLeavesAPlainCompositeAlone(t *testing.T) {
	t.Parallel()
	tbl, tables := documentCompositeTable(
		pipeline.Column{Name: "id", TypeName: "bigint"},
		pipeline.Column{Name: "p", TypeName: "public.plain"},
	)
	s := &state{
		schema: documentCompositeSchema(),
		tables: tables,
		steps:  []pipeline.Step{{Table: tbl}},
		cls:    &pipeline.Classification{Decisions: map[ref.ColumnRef]pipeline.Decision{}},
	}
	s.compositeDocuments()
	if s.failed(checkCatalog) {
		t.Fatalf("a composite with no document field was refused: %+v", s.firstFailure())
	}
}

// TestCompositeDocumentsHonoursUnmask: the operator's own --unmask for the
// column is the accepted risk the escape exists for and must not be
// second-guessed here.
func TestCompositeDocumentsHonoursUnmask(t *testing.T) {
	t.Parallel()
	tbl, tables := documentCompositeTable(
		pipeline.Column{Name: "id", TypeName: "bigint"},
		pipeline.Column{Name: "w", TypeName: "public.wrap"},
	)
	col := ref.ColumnRef{Table: tbl, Column: "w"}
	s := &state{
		schema: documentCompositeSchema(),
		tables: tables,
		steps:  []pipeline.Step{{Table: tbl}},
		cls: &pipeline.Classification{Decisions: map[ref.ColumnRef]pipeline.Decision{
			col: {
				Col:      col,
				Category: pipeline.CatSemiStruct,
				Masked:   false,
				Source:   pipeline.ByFlagUnmask,
			},
		}},
	}
	s.compositeDocuments()
	if s.failed(checkCatalog) {
		t.Fatalf("an explicitly --unmask'd composite was refused anyway: %+v", s.firstFailure())
	}
}

// TestCompositeDocumentsSkipsASchemaOnlyStep is the fix-round finding: a
// SchemaOnly step -- --skip-table gives Why "skipped", and an unreachable or
// unreadable table also lands here (plan.go:1291) -- recreates the composite
// type but copies no row of it, the same assumption shapes.go and counts.go
// already make. Before this fix compositeDocuments walked every step
// regardless of Mode, so an operator who followed the plan's own advice to
// skip the table got refused anyway.
func TestCompositeDocumentsSkipsASchemaOnlyStep(t *testing.T) {
	t.Parallel()
	tbl, tables := documentCompositeTable(
		pipeline.Column{Name: "id", TypeName: "bigint"},
		pipeline.Column{Name: "w", TypeName: "public.wrap"},
	)
	s := &state{
		schema: documentCompositeSchema(),
		tables: tables,
		steps:  []pipeline.Step{{Table: tbl, Mode: pipeline.SchemaOnly, Why: "skipped"}},
		cls:    &pipeline.Classification{Decisions: map[ref.ColumnRef]pipeline.Decision{}},
	}
	s.compositeDocuments()
	if s.failed(checkCatalog) {
		t.Fatalf("a SchemaOnly step was refused for a column whose row it never copied: %+v", s.firstFailure())
	}
}

// TestCompositeDocumentsResolvesADomainOverAJSONBField is the fix-round
// finding's other half: a field typed as a domain over jsonb, not jsonb
// directly, must still be caught -- documentField resolves a field's type
// through Schema.Domains before the family check, the way s.domainBase
// already does for a column's own declared type.
func TestCompositeDocumentsResolvesADomainOverAJSONBField(t *testing.T) {
	t.Parallel()
	schema := documentCompositeSchema()
	schema.Domains = append(schema.Domains, pipeline.NamedDef{
		Name: "public.docdom", Def: "CREATE DOMAIN public.docdom AS jsonb",
	})
	schema.Composites = append(schema.Composites, pipeline.NamedDef{
		Name: "public.domwrap", Def: "CREATE TYPE public.domwrap AS (tag text, doc public.docdom)",
	})
	tbl, tables := documentCompositeTable(
		pipeline.Column{Name: "id", TypeName: "bigint"},
		pipeline.Column{Name: "w", TypeName: "public.domwrap"},
	)
	s := &state{
		schema: schema,
		tables: tables,
		steps:  []pipeline.Step{{Table: tbl}},
		cls:    &pipeline.Classification{Decisions: map[ref.ColumnRef]pipeline.Decision{}},
	}
	s.compositeDocuments()
	r := s.firstFailure()
	if r == nil {
		t.Fatalf("a domain-over-jsonb field was not refused")
	}
	for _, want := range []string{"public.domwrap", "jsonb", "doc"} {
		if !strings.Contains(r.Reason, want) {
			t.Errorf("reason %q does not name %q", r.Reason, want)
		}
	}
}

// TestCompositeDocumentsResolvesADomainOverANestedComposite: the field
// holding the document is not a composite directly but a domain over one.
func TestCompositeDocumentsResolvesADomainOverANestedComposite(t *testing.T) {
	t.Parallel()
	schema := documentCompositeSchema()
	schema.Domains = append(schema.Domains, pipeline.NamedDef{
		Name: "public.wrapdom", Def: "CREATE DOMAIN public.wrapdom AS public.wrap",
	})
	schema.Composites = append(schema.Composites, pipeline.NamedDef{
		Name: "public.outer_wrap", Def: "CREATE TYPE public.outer_wrap AS (label text, inner public.wrapdom)",
	})
	tbl, tables := documentCompositeTable(
		pipeline.Column{Name: "id", TypeName: "bigint"},
		pipeline.Column{Name: "o", TypeName: "public.outer_wrap"},
	)
	s := &state{
		schema: schema,
		tables: tables,
		steps:  []pipeline.Step{{Table: tbl}},
		cls:    &pipeline.Classification{Decisions: map[ref.ColumnRef]pipeline.Decision{}},
	}
	s.compositeDocuments()
	r := s.firstFailure()
	if r == nil {
		t.Fatalf("a domain-over-composite field was not refused")
	}
	if !strings.Contains(r.Reason, "public.wrap") {
		t.Errorf("reason %q does not name the nested type public.wrap", r.Reason)
	}
	if strings.Contains(r.Reason, "outer_wrap") {
		t.Errorf("reason %q names the outer wrapper instead of the field's own type", r.Reason)
	}
}

// TestCompositeDocumentsWalksANestedComposite names the field's own type, not
// the outer wrapper, the same way internal/classify's own message does.
func TestCompositeDocumentsWalksANestedComposite(t *testing.T) {
	t.Parallel()
	schema := documentCompositeSchema()
	schema.Composites = append(schema.Composites, pipeline.NamedDef{
		Name: "public.outer_wrap", Def: "CREATE TYPE public.outer_wrap AS (label text, inner public.wrap)",
	})
	tbl, tables := documentCompositeTable(
		pipeline.Column{Name: "id", TypeName: "bigint"},
		pipeline.Column{Name: "o", TypeName: "public.outer_wrap"},
	)
	s := &state{
		schema: schema,
		tables: tables,
		steps:  []pipeline.Step{{Table: tbl}},
		cls:    &pipeline.Classification{Decisions: map[ref.ColumnRef]pipeline.Decision{}},
	}
	s.compositeDocuments()
	r := s.firstFailure()
	if r == nil {
		t.Fatalf("a nested document field was not refused")
	}
	if !strings.Contains(r.Reason, "public.wrap") {
		t.Errorf("reason %q does not name the nested type public.wrap", r.Reason)
	}
	if strings.Contains(r.Reason, "outer_wrap") {
		t.Errorf("reason %q names the outer wrapper instead of the field's own type", r.Reason)
	}
}
