// SPDX-License-Identifier: Apache-2.0

package textsig

import "testing"

// A URL is not a credential (tracker T-0100).
//
// LooksSecret matched any 16-to-512-character value with two character classes
// and Shannon entropy at or above 3.2 and no space and no "@", and a URL clears
// every one of those: mastodon's accounts.uri and statuses.uri were
// `credential` on 100% of their rows, masked to the fixed literal, and refused
// at plan under the unique index accounts.uri carries. The two halves of the
// fix are asserted together here, because either alone is wrong — excluding a
// URL from LooksSecret without ValidURL to catch it would drop a
// username-bearing URL to `none` and copy it verbatim (THREAT_MODEL.md T1).
func TestURLIsNotACredential(t *testing.T) {
	t.Parallel()

	urls := []string{
		"https://home.social.test/users/bea_donnelly1",
		"https://home.social.test/users/bea_donnelly1/statuses/109252811051724026",
		"http://example.test/@ada",
		"https://example.test",
		"https://example.test:8443/a/b?c=d#e",
		"ftp://files.example.test/pub/report.csv",
	}
	for _, u := range urls {
		if !ValidURL(u) {
			t.Errorf("ValidURL(%q) = false, want true", u)
		}
		if LooksSecret(u) {
			t.Errorf("LooksSecret(%q) = true: a URL is not a credential (T-0100)", u)
		}
	}

	// A scheme with no host, a host with no scheme, and the shapes the other
	// validators own. None of these is a URL, and the two that are secrets have
	// to stay secrets: the whole risk of this change is a credential that stops
	// being one.
	notURLs := []string{
		"mailto:bea@example.test",
		"a:b",
		"//home.social.test/users/bea",
		"home.social.test/users/bea",
		"bea.donnelly@example.test",
		"2017-02-15T09:34:33Z",
		"",
		"   ",
	}
	for _, s := range notURLs {
		if ValidURL(s) {
			t.Errorf("ValidURL(%q) = true, want false", s)
		}
	}

	secrets := []string{
		"sk_live_4eC39HqLyjWDarjtT1zdp7dc",
		"9f8b1c2d3e4a5b6c7d8e9f0a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c",
	}
	for _, s := range secrets {
		if ValidURL(s) {
			t.Errorf("ValidURL(%q) = true, want false", s)
		}
		if !LooksSecret(s) {
			t.Errorf("LooksSecret(%q) = false: T-0100 must not narrow the secrets validator", s)
		}
	}
}

// SupportedPhoneRegion is exact and case-sensitive against libphonenumber's
// own table: no synonym resolution, so a common non-ISO spelling ("UK") and a
// lower-case one ("gb") both answer false, and only the upper-case ISO code
// answers true. cmd/lazyslice's --phone-region validation (T-0221 review
// round, finding 1) is the caller that normalises to upper case before this
// ever runs; this function itself does no normalising.
func TestSupportedPhoneRegion(t *testing.T) {
	t.Parallel()

	for _, region := range []string{"GB", "US", "DE", "JP"} {
		if !SupportedPhoneRegion(region) {
			t.Errorf("SupportedPhoneRegion(%q) = false, want true", region)
		}
	}

	for _, region := range []string{"gb", "UK", "", "ZZ", "GBR", "notaregion"} {
		if SupportedPhoneRegion(region) {
			t.Errorf("SupportedPhoneRegion(%q) = true, want false", region)
		}
	}
}

// ValidMAC requires a separator (colon, dash or dot groups) or at least one
// hex letter; a bare run of plain digits is not a MAC even though
// net.ParseMAC alone accepts it as unseparated hex (tracker T-0297: Pagila's
// address.phone is 10-to-12 plain digits, which used to parse as a MAC and
// print a misfired reason line on a column already classified as phone by
// name).
func TestValidMACRequiresSeparatorOrHexLetter(t *testing.T) {
	t.Parallel()

	macs := []string{
		"01:23:45:67:89:ab", // colon-separated
		"01-23-45-67-89-AB", // dash-separated
		"0123.4567.89ab",    // dotted Cisco form
		"0123456789AB",      // 12 hex characters, no separator, but a letter
	}
	for _, s := range macs {
		if !ValidMAC(s) {
			t.Errorf("ValidMAC(%q) = false, want true", s)
		}
	}

	notMACs := []string{
		"012345678901", // 12 plain digits, no separator and no hex letter
		"1802001234",   // 10 plain digits (Pagila's address.phone shape)
	}
	for _, s := range notMACs {
		if ValidMAC(s) {
			t.Errorf("ValidMAC(%q) = true, want false: a plain digit run is not a MAC (T-0297)", s)
		}
	}
}
