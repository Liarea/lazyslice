// SPDX-License-Identifier: Apache-2.0

package verify

import (
	"context"
	"testing"

	"github.com/Liarea/lazyslice/internal/event"
	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
)

// T-0369: verify.refused.second_net's hint used to always spell
// "--mask TABLE.COL={reason}", the reason being whatever validator matched —
// a dead end for an unmasked json/jsonb/hstore column (no category but
// semi_structured accepts the family), for an already-masked one (--mask
// changes nothing) and for a category the column's own family does not
// accept (classify.refused.mask, exit 2, is the flag's own answer). Each case
// now carries its own code, and secondNetCode is what chooses among them.
//
// secondNetCode is a pure function of (validator, netMode), so the four
// routes are pinned directly rather than only through a scan: a scan proves
// the wiring (the two tests below it), and this proves the choice itself
// covers exactly the cases T-0369 named, including one no validator in this
// file's own table reaches today (a category outside categoryAcceptedFamilies
// entirely) — nothing may fall through to the ordinary code by accident.
func TestSecondNetCodeChoosesTheHintThatWorks(t *testing.T) {
	cases := []struct {
		name string
		val  validator
		mode netMode
		want event.Code
	}{
		{
			name: "an unmasked json family column",
			val:  validator{category: pipeline.CatEmail, name: "email"},
			mode: netMode{family: famJSON},
			want: CodeRefusedSecondNetDocument,
		},
		{
			name: "an unmasked jsonb family column",
			val:  validator{category: pipeline.CatNationalID, name: "national_id"},
			mode: netMode{family: famJSONB},
			want: CodeRefusedSecondNetDocument,
		},
		{
			name: "an unmasked hstore family column",
			val:  validator{category: pipeline.CatFinancial, name: "financial_account"},
			mode: netMode{family: famHstore},
			want: CodeRefusedSecondNetDocument,
		},
		{
			name: "an already-masked json family column",
			val:  validator{category: pipeline.CatEmail, name: "email"},
			mode: netMode{family: famJSON, docMasked: true},
			want: CodeRefusedSecondNetDocumentMasked,
		},
		{
			name: "a category this column's family does not accept",
			val:  validator{category: pipeline.CatCredential, name: "credential"},
			mode: netMode{family: famUUID},
			want: CodeRefusedSecondNetTypeConflict,
		},
		{
			name: "a category no entry in categoryAcceptedFamilies names at all",
			val:  validator{category: pipeline.CatGeo, name: "geo"},
			mode: netMode{family: famText},
			want: CodeRefusedSecondNetTypeConflict,
		},
		{
			name: "an ordinary accepted category on a character family",
			val:  validator{category: pipeline.CatEmail, name: "email"},
			mode: netMode{family: famText},
			want: CodeRefusedSecondNet,
		},
		{
			name: "a digits-family category rules.yml accepts on that family",
			val:  validator{category: pipeline.CatNationalID, name: "national_id", digits: true},
			mode: netMode{family: famBigint},
			want: CodeRefusedSecondNet,
		},
		{
			name: "special_category on a family with no entry of its own",
			val:  validator{category: pipeline.CatSpecial, name: "special_category"},
			mode: netMode{family: famInet},
			want: CodeRefusedSecondNet,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := secondNetCode(c.val, c.mode); got != c.want {
				t.Errorf("secondNetCode(%+v, %+v) = %s, want %s", c.val, c.mode, got, c.want)
			}
		})
	}
}

// categoryAcceptsFamily is secondNetCode's own gate, pinned on its own so a
// future entry added to categoryAcceptedFamilies is checked against
// rules.yml's accepts: lists rather than only through secondNetCode's cases
// above.
func TestCategoryAcceptsFamily(t *testing.T) {
	cases := []struct {
		cat    pipeline.Category
		family string
		want   bool
	}{
		{pipeline.CatEmail, famText, true},
		{pipeline.CatEmail, famUUID, false},
		{pipeline.CatNationalID, famBigint, true},
		{pipeline.CatFinancial, famNumeric, true},
		{pipeline.CatNetworkID, famInet, true},
		{pipeline.CatNetworkID, famUUID, false},
		{pipeline.CatOnlineID, famUUID, true},
		{pipeline.CatCredential, famBytea, true},
		{pipeline.CatCredential, famUUID, false},
		// special_category's rules.yml row is accepts: ["*"]: every family,
		// including one none of this file's own validators ever scans.
		{pipeline.CatSpecial, famTime, true},
	}
	for _, c := range cases {
		if got := categoryAcceptsFamily(c.cat, c.family); got != c.want {
			t.Errorf("categoryAcceptsFamily(%s, %s) = %v, want %v", c.cat, c.family, got, c.want)
		}
	}
}

// TestSecondNetTypeConflictHintNamesAWorkingCategory pins the fix for
// T-0369's own review round 1, medium finding: the bare `--mask TABLE.COL`
// this code used to hint at is not a working flag on any family it can
// actually fire on (T-0369's review comment on this file) -- it always
// records DefaultMaskCategory, free_text, and free_text's rules.yml row
// accepts only the four character families, never the bytea/uuid/inet/cidr/
// macaddr families a type conflict is reached on. internal/event/catalogue.yml
// now hardcodes the hint's suggested category as the literal
// "special_category" rather than templating one in, so this pins the
// invariant that literal choice relies on: rules.yml's special_category row
// is accepts: ["*"], so categoryAcceptsFamily(pipeline.CatSpecial, family)
// must answer true for every family a type conflict can be reached on -- and,
// as a floor, for every family this file names at all.
func TestSecondNetTypeConflictHintNamesAWorkingCategory(t *testing.T) {
	families := []string{
		famBytea, famUUID, famInet, famCIDR, famMacaddr,
		famText, famVarchar, famBpchar, famCitext,
		famBigint, famInteger, famNumeric, famFloat,
		famDate, famTimestamp, famTime, famOther,
		famJSON, famJSONB, famHstore,
	}
	for _, family := range families {
		if !categoryAcceptsFamily(pipeline.CatSpecial, family) {
			t.Errorf("categoryAcceptsFamily(special_category, %s) = false, want true -- "+
				"the type-conflict hint names special_category as a flag that always works", family)
		}
	}
}

// The scan-level guard: an unmasked jsonb column whose leaf is a production
// email address is exit 9 under CodeRefusedSecondNetDocument, not the
// ordinary code — the hint it carries names --mask TABLE.COL=semi_structured,
// which classify.refused.mask accepts, rather than =email, which it refuses
// (email's own accepts: list is text/varchar/bpchar/citext, never json).
func TestSecondNetDocumentColumnCarriesTheDocumentCode(t *testing.T) {
	table := customers()
	col := ref.ColumnRef{Table: table, Column: "profile"}
	s := &state{
		schema: &pipeline.Schema{},
		target: oneColumn{vals: []any{`{"contact":"ada.lovelace@fixture.test"}`}},
		steps:  []pipeline.Step{{Table: table, Mode: pipeline.ChildOK}},
		tables: map[ref.TableRef]*pipeline.Table{
			table: {Ref: table, Columns: []pipeline.Column{{Name: col.Column, TypeName: "jsonb"}}},
		},
		cls: &pipeline.Classification{Decisions: map[ref.ColumnRef]pipeline.Decision{
			col: {Col: col, Category: pipeline.CatNone, Source: pipeline.ByClassifier},
		}},
	}
	if err := s.secondNet(context.Background()); err != nil {
		t.Fatalf("secondNet: %v", err)
	}
	if len(s.failures) != 1 {
		t.Fatalf("the net recorded %d failures, want 1", len(s.failures))
	}
	f := s.failures[0]
	if f.Code != CodeRefusedSecondNetDocument {
		t.Errorf("code %s, want %s (a json/jsonb/hstore column's hint must name semi_structured)",
			f.Code, CodeRefusedSecondNetDocument)
	}
	if f.Reason != "email" {
		t.Errorf("reason %q, want %q — the code changes, not what validated", f.Reason, "email")
	}
}

// The scan-level guard for the already-masked half: json.go's maskKey only
// rewrites a key matching email, phone or the Luhn half of financial_account
// (T-0172); a national-id-shaped key is untouched by the masker even on a
// masked document column, so the net still refuses it, and the hint must not
// suggest --mask again — it would change nothing and refuse the same way
// next run.
func TestSecondNetMaskedDocumentColumnCarriesTheMaskedDocumentCode(t *testing.T) {
	table := customers()
	col := ref.ColumnRef{Table: table, Column: "profile"}
	s := &state{
		schema: &pipeline.Schema{},
		target: oneColumn{vals: []any{`{"078-05-1120":"buffer cadence delta"}`}},
		steps:  []pipeline.Step{{Table: table, Mode: pipeline.ChildOK}},
		tables: map[ref.TableRef]*pipeline.Table{
			table: {Ref: table, Columns: []pipeline.Column{{Name: col.Column, TypeName: "jsonb"}}},
		},
		cls: &pipeline.Classification{Decisions: map[ref.ColumnRef]pipeline.Decision{
			col: {Col: col, Category: pipeline.CatSemiStruct, Masked: true, Source: pipeline.ByClassifier},
		}},
	}
	if err := s.secondNet(context.Background()); err != nil {
		t.Fatalf("secondNet: %v", err)
	}
	if len(s.failures) != 1 {
		t.Fatalf("the net recorded %d failures, want 1 (a national-id-shaped key is not one "+
			"maskKey rewrites, masked column or not)", len(s.failures))
	}
	f := s.failures[0]
	if f.Code != CodeRefusedSecondNetDocumentMasked {
		t.Errorf("code %s, want %s (the column is already masked; --mask cannot change its category)",
			f.Code, CodeRefusedSecondNetDocumentMasked)
	}
	if f.Reason != "national_id" {
		t.Errorf("reason %q, want %q", f.Reason, "national_id")
	}
}
