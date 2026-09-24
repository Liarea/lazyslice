// SPDX-License-Identifier: Apache-2.0

package verify

import (
	"context"
	"testing"

	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
	"github.com/Liarea/lazyslice/internal/textsig"
)

// TestTheSecondNetWantsTheShapeOfACard is T-0316's verify half. The card entry
// over a character column is strong -- one hit refuses the run -- so once
// internal/classify stops masking a column for a value that passes only the
// check digit, this net must not refuse it for the same value; a card-shaped
// value still refuses, under an identifier's name or any other.
func TestTheSecondNetWantsTheShapeOfACard(t *testing.T) {
	const (
		migration = "20230415123453"   // a Rails migration timestamp; no issuer starts 2023
		uatpShort = "1234567890003"    // UATP's prefix at thirteen digits; UATP issues fifteen
		testVisa  = "4111111111111111" // the Visa test card
		// testVisaDashed is the same card as a person types it; unlike the
		// undotted run it does not also read as a hardware address, so a
		// refusal for it can only be the card entry's.
		testVisaDashed = "4111-1111-1111-1111"
		visa14         = "40001234567899" // Visa's prefix at a length Visa does not issue
	)
	for _, v := range []string{migration, uatpShort, testVisa, visa14} {
		if !textsig.ValidLuhn(v) {
			t.Fatalf("precondition: %q must pass the bare Luhn check, or it pins nothing", v)
		}
	}
	ordinary := []any{"3000000000001", "3000000000019", "3000000000027"}
	for _, v := range ordinary {
		if textsig.ValidLuhn(v.(string)) {
			t.Fatalf("precondition: filler %q passes the Luhn check", v)
		}
	}
	cases := []struct {
		column   string
		planted  string
		wantFail bool
	}{
		{"version", migration, false},
		{"customer_id", uatpShort, false},
		{"billingCustomerId", uatpShort, false},
		{"batch_code", migration, false},
		{"invoice_number", testVisaDashed, true},
		{"memo", visa14, true},
	}
	table := ref.TableRef{Schema: "public", Name: "t316_billing"}
	for _, c := range cases {
		t.Run(c.column, func(t *testing.T) {
			col := ref.ColumnRef{Table: table, Column: c.column}
			s := &state{
				schema: &pipeline.Schema{},
				target: oneColumn{vals: append(append([]any{}, ordinary...), c.planted)},
				steps:  []pipeline.Step{{Table: table, Mode: pipeline.ChildOK}},
				tables: map[ref.TableRef]*pipeline.Table{
					table: {Ref: table, Columns: []pipeline.Column{{Name: c.column, TypeName: "text"}}},
				},
				cls: &pipeline.Classification{Decisions: map[ref.ColumnRef]pipeline.Decision{
					col: {Col: col, Category: pipeline.CatNone, Source: pipeline.ByClassifier},
				}},
			}
			if err := s.secondNet(context.Background()); err != nil {
				t.Fatalf("secondNet: %v", err)
			}
			if !c.wantFail {
				if len(s.failures) != 0 {
					t.Fatalf("the net refused %s as %q on a value that passes only the check digit (T-0316)", col, s.failures[0].Reason)
				}
				return
			}
			if len(s.failures) != 1 {
				t.Fatalf("the net recorded %v for %s holding a card-shaped value, want one refusal", s.failures, col)
			}
			if s.failures[0].Reason != "financial_account" {
				t.Errorf("the net refused %s as %q, want the card entry's %q", col, s.failures[0].Reason, "financial_account")
			}
		})
	}
}
