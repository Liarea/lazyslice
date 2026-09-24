// SPDX-License-Identifier: Apache-2.0

package textsig

import "testing"

// TestIdentifierShapesRefusePersonalLookalikes holds the edges of T-0311's
// identifier shapes that decide whether a value carrying personal data could
// be spared as an identifier: a slashed date (with a month name or without)
// is not a path, nor is a bare two-segment home directory, a first.last
// username is not a hostname, an all-digit number is not a hex digest, a
// dotted date of birth is not a semantic version, and a URL is ValidURL's
// rather than a path.
func TestIdentifierShapesRefusePersonalLookalikes(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		ok   func(string) bool
		yes  []string
		no   []string
	}{
		{"PathShape", PathShape,
			[]string{"/images/banner.png", "assets/css/app.css", "America/New_York", "/api/v1/users", "Etc/UTC"},
			[]string{"12/03/1985", "1/2", "https://host.example/a", "/home/a b", "a@b/c", "plain",
				"05/Mar/1985", "/12/Nov/1979", "/home/jsmith", "/Users/tbaker", "admin/users", "v1.2/3.4"}},
		{"HostnameShape", HostnameShape,
			[]string{"api.prod.internal", "example.com", "db.local"},
			[]string{"john.smith", "zbigniew.brzezinski", "localhost", "a.b.1", "x@y.com"}},
		{"HexDigest", HexDigest,
			[]string{"3f9a2c7e", "d41d8cd98f00b204e9800998ecf8427e"},
			[]string{"12345678", "deadbeefcafe", "3f9a2c7", "3f9a2c7g"}},
		{"SemanticVersion", SemanticVersion,
			[]string{"1.4.2", "v2.0.0-rc.1", "3.1.0+build.7"},
			[]string{"1.4", "01.2.3", "1.2.3.4", "2024.05", "5.3.1985", "12.11.1979", "1.1.1970", "1985.3.5",
				"v7.8.1990", "12.25.2001"}},
		{"DottedDate", DottedDate,
			[]string{"5.3.1985", "1985-03-05", "12/25/2001", "1985/3/5"},
			[]string{"1.4.2", "2.10.0", "13.13.1985", "5.3.85", "5.3.1985.1", "a.3.1985"}},
	}
	for _, c := range cases {
		for _, v := range c.yes {
			if !c.ok(v) {
				t.Errorf("%s(%q) = false, want true", c.name, v)
			}
		}
		for _, v := range c.no {
			if c.ok(v) {
				t.Errorf("%s(%q) = true, want false", c.name, v)
			}
		}
	}
}
