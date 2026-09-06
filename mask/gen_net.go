// SPDX-License-Identifier: Apache-2.0

package mask

import (
	"net/netip"
	"strconv"
	"strings"
)

// Masked addresses land in the ranges reserved for documentation: the three
// IPv4 blocks of RFC 5737, the IPv6 block 2001:db8::/32 of RFC 3849, and the
// MAC block 00-00-5E-00-53-xx of RFC 7042. None of them routes, so a masked
// address cannot be dialled back to the original host.

const (
	v6Prefix  = "2001:0db8:"
	v6Groups  = 6  // 96 bits from h
	v6Len     = 39 // len(v6Prefix) + v6Groups*4 + 5 separators
	macPrefix = "00:00:5e:00:53:"
	macLen    = 17
	maxV4Len  = 14 // "198.51.100.255"
	v4Domain  = 3 * 256
	macDomain = 256
	cidrV4Len = maxV4Len + 3 // "/32"
	cidrV6Len = v6Len + 4    // "/128"
)

// netShape is the address family a network_id column gets.
type netShape int

const (
	netNone netShape = iota
	netV4
	netV6
	netMAC
)

// netShapeFor keeps the *shape* of the source — an IPv6 column stays IPv6, a
// MAC column stays a MAC — because a column the application parses must still
// parse. The address itself keeps nothing.
func netShapeFor(in Value, c Constraints) netShape {
	shape := netV4
	switch c.TypeTag {
	case famMacaddr:
		shape = netMAC
	default:
		t := strings.TrimSpace(in.Text)
		if a, err := netip.ParseAddr(t); err == nil && a.Is6() && !a.Is4In6() {
			shape = netV6
		} else if p, err := netip.ParsePrefix(t); err == nil && p.Addr().Is6() && !p.Addr().Is4In6() {
			shape = netV6
		} else if macAdmissible(c) && reMAC.MatchString(t) {
			shape = netMAC
		}
	}
	if !fits(strings.Repeat("x", netShapeLen(shape, c)), c) {
		return netNone
	}
	return shape
}

// macAdmissible reports whether the column could hold a MAC address at all.
// An inet or cidr column cannot — a MAC in one fails the load with 22P02 — so
// a MAC-shaped value there is masked as an address, and Domain says 768 for
// those families without having to consider the MAC branch.
func macAdmissible(c Constraints) bool {
	switch c.TypeTag {
	case famInet, famCIDR:
		return false
	default:
		return fits(strings.Repeat("x", macLen), c)
	}
}

func netShapeLen(shape netShape, c Constraints) int {
	cidr := c.TypeTag == famCIDR
	switch shape {
	case netV4:
		if cidr {
			return cidrV4Len
		}
		return maxV4Len
	case netV6:
		if cidr {
			return cidrV6Len
		}
		return v6Len
	case netMAC:
		return macLen
	case netNone:
		return 0
	}
	return 0
}

// networkIDMasker replaces an address with one from the documentation range of
// its own family.
type networkIDMasker struct{}

func (networkIDMasker) Domain(c Constraints) int64 {
	if d, ok := labelDomain(c); ok {
		return d
	}
	// Domain is a property of the column, and a network_id column can hold a
	// mixture of shapes — netShapeFor picks the branch from the value, not from
	// the type — so the honest figure is the narrowest branch the column
	// admits. A text, varchar or citext column admits all three (the rule pack
	// accepts those for network_id), and the MAC's 256 is narrower than the
	// IPv4 block's 768: reporting 768 for a column of MAC addresses is the
	// over-reporting mask/CLAUDE.md calls the one forbidden direction.
	if c.TypeTag == famMacaddr {
		if !fits(strings.Repeat("x", macLen), c) {
			return 0
		}
		return macDomain
	}
	if !fits(strings.Repeat("x", netShapeLen(netV4, c)), c) {
		return 0
	}
	if macAdmissible(c) {
		return min(int64(v4Domain), int64(macDomain))
	}
	return v4Domain
}

func (networkIDMasker) Mask(h [32]byte, in Value, c Constraints) (Value, error) {
	if v, ok := labelValue(h, c); ok {
		return v, nil
	}
	shape := netShapeFor(in, c)
	if shape == netNone {
		return Value{}, ErrNoRoom
	}
	s := newStream(h)
	switch shape {
	case netMAC:
		return Value{Text: macPrefix + s.hexdigits(2)}, nil
	case netV6:
		return Value{Text: withPrefixLen(v6Address(s), c, "/128")}, nil
	case netV4, netNone:
		host := docPrefixes[s.intn(int64(len(docPrefixes)))] + strconv.FormatInt(s.intn(256), 10)
		return Value{Text: withPrefixLen(host, c, "/32")}, nil
	}
	return Value{}, ErrNoRoom
}

func withPrefixLen(addr string, c Constraints, suffix string) string {
	if c.TypeTag == famCIDR {
		return addr + suffix
	}
	return addr
}

func v6Address(s *stream) string {
	var b strings.Builder
	b.WriteString(v6Prefix)
	for i := 0; i < v6Groups; i++ {
		if i > 0 {
			b.WriteByte(':')
		}
		b.WriteString(s.hexdigits(4))
	}
	return b.String()
}

// ipUniqueMasker is the generator a network_id column under a unique index
// gets: 96 bits of h inside 2001:db8::/32, a domain of 2⁹⁶ (reported as the
// largest int64, which is all the planner's comparison needs). It needs an
// inet or cidr column, or a text column of at least 39 characters, which is
// exactly what ARCHITECTURE.md section 5 says.
type ipUniqueMasker struct{}

func (ipUniqueMasker) Domain(c Constraints) int64 {
	if d, ok := labelDomain(c); ok {
		return d
	}
	switch c.TypeTag {
	case famInet, famCIDR:
	default:
		if !fits(strings.Repeat("x", v6Len), c) {
			return 0
		}
	}
	if c.TypeTag == famMacaddr {
		return 0
	}
	return satPow(2, 96)
}

func (m ipUniqueMasker) Mask(h [32]byte, _ Value, c Constraints) (Value, error) {
	if v, ok := labelValue(h, c); ok {
		return v, nil
	}
	if m.Domain(c) == 0 {
		return Value{}, ErrNoRoom
	}
	s := newStream(h)
	return Value{Text: withPrefixLen(v6Address(s), c, "/128")}, nil
}
