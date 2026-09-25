// SPDX-License-Identifier: Apache-2.0

package classify

import (
	"encoding/json"
	"testing"
)

// T-0402 (the 2026-09-25 JSON red team's A11, docs/reviews/2026-09-25-
// redteam-json/round1.json entry 14): decodeSampleDocument now decodes a
// []byte or string sample with encoding/json's Decoder and UseNumber, the
// same way internal/transform's and internal/verify's own copies do, instead
// of plain json.Unmarshal. A json or jsonb column now reaches this package as
// the source's own raw text (internal/pg's jsonTextRows), the same text a
// domain over jsonb has always arrived as, so this is the one place both are
// decoded, and it is the same place the per-leaf key map (jsonKeyCategories)
// and the phone-guessing leaf walk (guessedPhoneLeafKeys) both read through.
// Before this, a number leaf decoded to a float64, which rounds an integer
// past 2^53 -- a 19-digit Luhn-valid card number's trailing digits -- before
// anything downstream of this function could read its exact text; a 16-digit
// card fits inside a float64's exact range and was never rounded either way.
func TestDecodeSampleDocumentKeepsEveryDigitOfALargeNumber(t *testing.T) {
	const pan16 = "4111111111111111"    // Luhn-valid, 16 digits: exact in a float64
	const pan19 = "4000123456789012343" // Luhn-valid, 19 digits: a float64 rounds this

	for _, sample := range []any{
		`{"pan16":` + pan16 + `,"pan19":` + pan19 + `}`,
		[]byte(`{"pan16":` + pan16 + `,"pan19":` + pan19 + `}`),
	} {
		doc, ok := decodeSampleDocument(sample)
		if !ok {
			t.Fatalf("decodeSampleDocument(%T): not ok", sample)
		}
		m, ok := doc.(map[string]any)
		if !ok {
			t.Fatalf("decodeSampleDocument(%T) = %T, want map[string]any", sample, doc)
		}
		for key, want := range map[string]string{"pan16": pan16, "pan19": pan19} {
			n, ok := m[key].(json.Number)
			if !ok {
				t.Fatalf("decodeSampleDocument(%T)[%q] = %T, want json.Number", sample, key, m[key])
			}
			if n.String() != want {
				t.Errorf("decodeSampleDocument(%T)[%q] = %q, want %q: a float64 would have rounded it",
					sample, key, n.String(), want)
			}
		}
	}

	// A value pgx had already decoded (the default arm, not a []byte or
	// string) passes through unchanged; this function has no text to decode
	// and nothing to fix, which is what internal/pg's jsonTextRows now
	// changes upstream instead.
	if doc, ok := decodeSampleDocument(map[string]any{"n": float64(1)}); !ok {
		t.Errorf("decodeSampleDocument(map[string]any): not ok")
	} else if _, ok := doc.(map[string]any); !ok {
		t.Errorf("decodeSampleDocument(map[string]any) = %T, want map[string]any unchanged", doc)
	}

	if _, ok := decodeSampleDocument(nil); ok {
		t.Errorf("decodeSampleDocument(nil): ok, want false")
	}
	if _, ok := decodeSampleDocument("not json"); ok {
		t.Errorf("decodeSampleDocument(%q): ok, want false", "not json")
	}
}
