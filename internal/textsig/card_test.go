// SPDX-License-Identifier: Apache-2.0

package textsig

import "testing"

// TestCardNeedsAnIssuerAndALength is T-0316's pin. Each value is first shown
// to pass the bare Luhn check, so the test cannot pass on a value that was
// never a false positive.
func TestCardNeedsAnIssuerAndALength(t *testing.T) {
	cases := []struct {
		value     string
		card      bool // ValidCard
		cardShape bool // CardShape
		why       string
	}{
		{"20230415123453", false, false, "a 14-digit Rails migration version: no issuer starts 2023"},
		{"4111111111111111", true, true, "the Visa test card"},
		{"4111 1111 1111 1111", true, true, "the Visa test card, grouped"},
		{"4111.1111.1111.1111", true, true, "the Visa test card, dotted (the 2026-09-15 red team)"},
		{"5555555555554444", true, true, "the Mastercard test card"},
		{"378282246310005", true, true, "the American Express test card"},
		{"40001234567899", true, false, "Visa's prefix at fourteen digits, a length Visa does not issue"},
		{"1234567890003", true, false, "UATP's prefix at thirteen digits; UATP issues fifteen"},
		{"7000123456785", false, false, "no issuer starts 7"},
	}
	for _, c := range cases {
		if !ValidLuhn(c.value) {
			t.Fatalf("%q (%s) must pass the bare Luhn check to be a pin at all", c.value, c.why)
		}
		if got := ValidCard(c.value); got != c.card {
			t.Errorf("ValidCard(%q) = %v, want %v: %s", c.value, got, c.card, c.why)
		}
		if got := CardShape(c.value); got != c.cardShape {
			t.Errorf("CardShape(%q) = %v, want %v: %s", c.value, got, c.cardShape, c.why)
		}
	}
	if ValidCard("4111111111111112") || CardShape("4111111111111112") {
		t.Error("a Visa-prefixed number that fails the check digit is not a card")
	}
}
