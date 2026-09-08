// SPDX-License-Identifier: Apache-2.0

package mask

import (
	"fmt"
	"strconv"
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

	wantV4 := wantIPIn("192.0.2.0/24", "198.51.100.0/24", "203.0.113.0/24")
	wantV6 := wantIPIn("2001:db8::/32")

	// v4Sources spans real routable addresses across every octet (i itself,
	// bit-rotated so all four bytes vary rather than only the low ones) and,
	// deliberately, values already inside the three documentation blocks a
	// source column can legitimately hold — network_id must still land those
	// honestly rather than by an identity that only looks that way because the
	// generator never saw anything else.
	v4Sources := func(i int) string {
		switch i % 4 {
		case 0:
			return "192.0.2." + strconv.Itoa(i%256)
		case 1:
			return "198.51.100." + strconv.Itoa(i%256)
		case 2:
			return "203.0.113." + strconv.Itoa(i%256)
		default:
			u := uint32(i)*2654435761 + 1 // Knuth's multiplicative hash, spreads i across all 32 bits
			return fmt.Sprintf("%d.%d.%d.%d", u>>24|1, (u>>16)&0xff, (u>>8)&0xff, u&0xff)
		}
	}

	v4Constraints := Constraints{TypeTag: famInet}
	v4Out := make(map[string]struct{}, n)
	for i := 0; i < n; i++ {
		in := Value{Text: v4Sources(i)}
		r, err := Apply(k, CatNetworkID, MaskerNetworkID, in, v4Constraints)
		if err != nil {
			t.Fatalf("input %d (%q): %v", i, in.Text, err)
		}
		wantV4(t, r.Out.Text)
		v4Out[r.Out.Text] = struct{}{}
	}
	// The default network_id generator's own domain over an inet column is
	// v4Domain = 768 (three /24 blocks), so 10,000 draws cannot produce more
	// distinct outputs than that — the threshold checks that the generator
	// actually varies, not that it is unique.
	if len(v4Out) < 100 {
		t.Fatalf("%d distinct IPv4 outputs from %d distinct inputs; the generator looks constant", len(v4Out), n)
	}

	v6Constraints := Constraints{TypeTag: famInet}
	v6Out := make(map[string]struct{}, n)
	for i := 0; i < n; i++ {
		in := Value{Text: fmt.Sprintf("2a00:1450:4009:%x::%x", i, i+1)}
		r, err := Apply(k, CatNetworkID, MaskerNetworkID, in, v6Constraints)
		if err != nil {
			t.Fatalf("input %d (%q): %v", i, in.Text, err)
		}
		wantV6(t, r.Out.Text)
		v6Out[r.Out.Text] = struct{}{}
	}
	if len(v6Out) < n/2 {
		t.Fatalf("%d distinct IPv6 outputs from %d distinct inputs; the generator looks constant", len(v6Out), n)
	}
}
