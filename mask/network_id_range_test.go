// SPDX-License-Identifier: Apache-2.0

package mask

import (
	"fmt"
	"net/netip"
	"testing"
)

// TestNetworkIDStaysInDocumentationRanges pins ARCHITECTURE.md section 5's
// design: network_id's IPv4 output is always one of the three RFC 5737
// documentation blocks, and its IPv6 output is always under 2001:db8::/32
// (RFC 3849). A source column already holding an address in those ranges can
// therefore receive a masked value equal to its own source value — a known
// false-positive surface for the residual scan, not a leak (T-0059) — which
// is exactly why testdata's fixtures avoid documentation-range IPv4 source
// values. This test does not change the generator; it only pins its output
// space.
func TestNetworkIDStaysInDocumentationRanges(t *testing.T) {
	k := testKey(t)
	const n = 10_000

	v4Blocks := []netip.Prefix{
		netip.MustParsePrefix("192.0.2.0/24"),
		netip.MustParsePrefix("198.51.100.0/24"),
		netip.MustParsePrefix("203.0.113.0/24"),
	}
	v6Block := netip.MustParsePrefix("2001:db8::/32")

	v4Constraints := Constraints{TypeTag: famInet}
	for i := 0; i < n; i++ {
		in := Value{Text: fmt.Sprintf("%d.%d.%d.%d", (i>>24)&0xff|1, (i>>16)&0xff, (i>>8)&0xff, i&0xff)}
		r, err := Apply(k, CatNetworkID, MaskerNetworkID, in, v4Constraints)
		if err != nil {
			t.Fatalf("input %d (%q): %v", i, in.Text, err)
		}
		addr, err := netip.ParseAddr(r.Out.Text)
		if err != nil {
			t.Fatalf("input %d (%q): output %q is not an address: %v", i, in.Text, r.Out.Text, err)
		}
		if !addr.Is4() {
			t.Fatalf("input %d (%q): output %q is not IPv4", i, in.Text, r.Out.Text)
		}
		inAny := false
		for _, b := range v4Blocks {
			if b.Contains(addr) {
				inAny = true
				break
			}
		}
		if !inAny {
			t.Fatalf("input %d (%q): output %q is outside the three RFC 5737 blocks", i, in.Text, r.Out.Text)
		}
	}

	v6Constraints := Constraints{TypeTag: famInet}
	for i := 0; i < n; i++ {
		in := Value{Text: fmt.Sprintf("2a00:1450:4009:%x::%x", i, i+1)}
		r, err := Apply(k, CatNetworkID, MaskerNetworkID, in, v6Constraints)
		if err != nil {
			t.Fatalf("input %d (%q): %v", i, in.Text, err)
		}
		addr, err := netip.ParseAddr(r.Out.Text)
		if err != nil {
			t.Fatalf("input %d (%q): output %q is not an address: %v", i, in.Text, r.Out.Text, err)
		}
		if !addr.Is6() || addr.Is4In6() {
			t.Fatalf("input %d (%q): output %q is not IPv6", i, in.Text, r.Out.Text)
		}
		if !v6Block.Contains(addr) {
			t.Fatalf("input %d (%q): output %q is outside 2001:db8::/32", i, in.Text, r.Out.Text)
		}
	}
}
