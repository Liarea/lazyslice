// SPDX-License-Identifier: Apache-2.0

package classify

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
	"github.com/Liarea/lazyslice/internal/textsig"
)

// Dogfood session 1 masked eight identifier columns as free_text, "a strong
// validator hit below the category threshold", on one value each that passed
// the bare Luhn check (tracker T-0316). The card entry now wants an issuer
// prefix, and on a column named for an identifier the issuer's own length as
// well; a real card number is masked in either kind of column.
func TestCardEntryNeedsTheShapeOfACard(t *testing.T) {
	t.Parallel()

	tbl := ref.TableRef{Schema: "public", Name: "t316_billing"}
	schema := &pipeline.Schema{
		Tables: []pipeline.Table{
			tt("public", "t316_billing", []string{"id"},
				tc("id", "integer"),
				tc("version", "character varying(255)"),
				tc("customer_id", "character varying(255)"),
				tc("batch_code", "text"),
				tc("invoice_number", "text"),
				tc("batch_label", "text"),
			),
		},
		Fingerprint: "t316",
	}

	// filler is n distinct thirteen-digit numbers none of which passes the
	// bare Luhn check, so the one planted value is the column's only hit.
	filler := func(seed, n int) []any {
		out := make([]any, 0, n)
		for x := 3000000000000 + seed*7919; len(out) < n; x += 104729 {
			if s := fmt.Sprint(x); !textsig.ValidLuhn(s) {
				out = append(out, s)
			}
		}
		return out
	}
	with := func(seed int, planted string) []any { return append(filler(seed, 19), planted) }

	const (
		migration = "20230415123453"   // a Rails migration timestamp; no issuer starts 2023
		uatpShort = "1234567890003"    // UATP's prefix at thirteen digits; UATP issues fifteen
		testVisa  = "4111111111111111" // the Visa test card
		visa14    = "40001234567899"   // Visa's prefix at a length Visa does not issue
	)
	for _, v := range []string{migration, uatpShort, testVisa, visa14} {
		if !textsig.ValidLuhn(v) {
			t.Fatalf("precondition: %q must pass the bare Luhn check, or it pins nothing", v)
		}
	}
	versions := make([]any, 0, 20)
	for i := 0; i < 19; i++ {
		versions = append(versions, fmt.Sprintf("202304%02d%06d", i%28+1, 120000+i))
	}
	versions = append(versions, migration)

	samples := mapSampler{
		col(tbl, "id"):             filler(0, 20),
		col(tbl, "version"):        versions,
		col(tbl, "customer_id"):    with(1, uatpShort),
		col(tbl, "batch_code"):     with(2, migration),
		col(tbl, "invoice_number"): with(3, testVisa),
		col(tbl, "batch_label"):    with(4, visa14),
	}
	cls, err := New().Classify(schema, samples, nil)
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	for _, name := range []string{"version", "customer_id", "batch_code"} {
		if d := decision(t, cls, col(tbl, name)); d.Masked {
			t.Errorf("t316_billing.%s = %s/%v masked (%s): a digit run that passes only the check digit is not a card (T-0316)",
				name, d.Category, d.Confidence, d.Reason)
		}
	}
	// batch_label matches no rule-pack name, so ValidCard alone decides it;
	// invoice_number is identifier-named, so CardShape does. Either way the
	// card entry itself must be what masks the column: an undotted
	// sixteen-digit run also reads as a MAC address, and a name rule would
	// mask a column like memo whatever its values.
	for _, name := range []string{"invoice_number", "batch_label"} {
		d := decision(t, cls, col(tbl, name))
		if !d.Masked {
			t.Errorf("t316_billing.%s = %s/%v copied (%s): a card-shaped value must still mask its column (T-0316)",
				name, d.Category, d.Confidence, d.Reason)
			continue
		}
		if !strings.Contains(d.Reason, phraseLuhn) {
			t.Errorf("t316_billing.%s masked for %q, want the card entry's %q (T-0316)", name, d.Reason, phraseLuhn)
		}
	}
}
