// SPDX-License-Identifier: Apache-2.0

package mask

// hostLen is len("@example.com"); every host in emailHosts is the same length,
// which is what lets one budget serve all three.
const hostLen = 12

// uniqueSuffix is the number of base32 symbols a unique email column carries:
// 13 symbols are 65 bits, which clears the ≥ 2⁶⁴ ARCHITECTURE.md section 5 asks
// of a unique email column.
const uniqueSuffix = 13

// plainSuffix is the base32 length used when even one name pair does not fit.
const plainSuffix = 8

// fitBudget subtracts k bytes from a budget. A budget of 0 is unbounded and
// stays unbounded; a result below one byte is reported as -1, which every
// counting helper reads as "nothing fits".
func fitBudget(b, k int) int {
	if b <= 0 {
		return 0
	}
	if b-k < 1 {
		return -1
	}
	return b - k
}

// emailShape is the branch the email generator takes for a column. Mask and
// Domain both read it, so the number Domain reports is the number of values
// Mask can actually produce.
type emailShape struct {
	ok         bool
	names      bool
	nameBudget int
	suffix     int
}

func emailShapeFor(c Constraints) emailShape {
	local := 0
	if c.MaxLen > 0 {
		local = c.MaxLen - hostLen
		if local < 1 {
			return emailShape{}
		}
	}
	if c.Unique {
		nb := fitBudget(local, 1+uniqueSuffix)
		if pairCount(givenNames, surnames, 1, nb) > 0 {
			return emailShape{ok: true, names: true, nameBudget: nb, suffix: uniqueSuffix}
		}
		s := uniqueSuffix
		if local > 0 {
			s = min(uniqueSuffix, local)
		}
		return emailShape{ok: s > 0, suffix: s}
	}
	if pairCount(givenNames, surnames, 1, local) > 0 {
		return emailShape{ok: true, names: true, nameBudget: local}
	}
	s := plainSuffix
	if local > 0 {
		s = min(plainSuffix, local)
	}
	return emailShape{ok: s > 0, suffix: s}
}

// emailMasker lands every address in one of the RFC 2606 example domains. The
// local part is drawn from the embedded name lists, and a column under a unique
// index gets a hash-derived base32 suffix instead of a retry counter
// (ARCHITECTURE.md section 5). Nothing of the original survives: not the
// domain, not the length, not the first character.
type emailMasker struct{}

func (emailMasker) Domain(c Constraints) int64 {
	if d, ok := labelDomain(c); ok {
		return d
	}
	sh := emailShapeFor(c)
	if !sh.ok {
		return 0
	}
	n := int64(len(emailHosts))
	if sh.names {
		n = satMul(n, pairCount(givenNames, surnames, 1, sh.nameBudget))
	}
	if sh.suffix > 0 {
		n = satMul(n, satPow(int64(len(b32alphabet)), sh.suffix))
	}
	return n
}

func (emailMasker) Mask(h [32]byte, _ Value, c Constraints) (Value, error) {
	if v, ok := labelValue(h, c); ok {
		return v, nil
	}
	sh := emailShapeFor(c)
	if !sh.ok {
		return Value{}, ErrNoRoom
	}
	s := newStream(h)
	host := emailHosts[s.intn(int64(len(emailHosts)))]
	local := ""
	if sh.names {
		n := pairCount(givenNames, surnames, 1, sh.nameBudget)
		g, sn := pairAt(givenNames, surnames, 1, sh.nameBudget, s.intn(n))
		local = g + "." + sn
	}
	if sh.suffix > 0 {
		if local != "" {
			local += "."
		}
		local += s.base32(sh.suffix)
	}
	out := local + "@" + host
	if !fits(out, c) {
		return Value{}, ErrNoRoom
	}
	return Value{Text: out}, nil
}
