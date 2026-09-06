// SPDX-License-Identifier: Apache-2.0

package mask

import (
	"crypto/sha256"
	"encoding/binary"
	"math"
)

// stream expands h into as many bytes as a generator needs, as SHA-256 over
// h ‖ u32be(counter). It is the only randomness in this module: no global
// seed, no clock, no math/rand. Two calls with the same h draw the same bytes
// in the same order, which is the whole determinism contract.
type stream struct {
	h    [32]byte
	buf  [sha256.Size]byte
	used int
	ctr  uint32
}

func newStream(h [32]byte) *stream {
	s := &stream{h: h, used: sha256.Size}
	return s
}

func (s *stream) refill() {
	var ctr [4]byte
	binary.BigEndian.PutUint32(ctr[:], s.ctr)
	s.ctr++
	sum := sha256.New()
	// hash.Hash never returns an error from Write.
	_, _ = sum.Write(s.h[:])
	_, _ = sum.Write(ctr[:])
	copy(s.buf[:], sum.Sum(nil))
	s.used = 0
}

func (s *stream) read(p []byte) {
	for len(p) > 0 {
		if s.used == sha256.Size {
			s.refill()
		}
		n := copy(p, s.buf[s.used:])
		s.used += n
		p = p[n:]
	}
}

func (s *stream) uint64() uint64 {
	var b [8]byte
	s.read(b[:])
	return binary.BigEndian.Uint64(b[:])
}

// intn returns a value in [0, n) without modulo bias. n must be positive.
func (s *stream) intn(n int64) int64 {
	if n <= 1 {
		return 0
	}
	un := uint64(n)
	limit := math.MaxUint64 - (math.MaxUint64 % un) - 1
	for {
		v := s.uint64()
		if v <= limit {
			return int64(v % un) //nolint:gosec // G115: v % un < n, and n is an int64
		}
	}
}

// decimalDigits is indexed by the stream directly: a string index takes any
// integer type, so there is no int64-to-byte conversion to get wrong.
const decimalDigits = "0123456789"

// digits returns n decimal digits. The first is 1-9 unless leadingZero is set,
// so that a value copied into a numeric column keeps its length.
func (s *stream) digits(n int, leadingZero bool) string {
	if n <= 0 {
		return ""
	}
	out := make([]byte, n)
	for i := range out {
		if i == 0 && !leadingZero {
			out[i] = decimalDigits[1+s.intn(9)]
			continue
		}
		out[i] = decimalDigits[s.intn(10)]
	}
	return string(out)
}

// b32alphabet is RFC 4648 base32 in lower case, chosen so that a local part or
// a URL path is a plain identifier: 32 symbols, so n symbols are exactly 5n
// bits of domain.
const b32alphabet = "abcdefghijklmnopqrstuvwxyz234567"

// base32 returns n symbols from the alphabet above.
func (s *stream) base32(n int) string {
	if n <= 0 {
		return ""
	}
	out := make([]byte, n)
	for i := range out {
		out[i] = b32alphabet[s.intn(int64(len(b32alphabet)))]
	}
	return string(out)
}

// hexdigits returns n lower-case hex symbols.
func (s *stream) hexdigits(n int) string {
	const alphabet = "0123456789abcdef"
	if n <= 0 {
		return ""
	}
	out := make([]byte, n)
	for i := range out {
		out[i] = alphabet[s.intn(int64(len(alphabet)))]
	}
	return string(out)
}
