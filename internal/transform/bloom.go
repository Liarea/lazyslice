// SPDX-License-Identifier: Apache-2.0

package transform

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"sync"

	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/mask"
)

// The residual filter is the only check that tests the masker rather than the
// classifier: it answers "did any value we meant to mask reach the target
// unchanged?". It is a Bloom filter of HMAC(runKey, Encode(column, path,
// canonical(source))) over every masked cell, sized for 10^-6 false positives
// (about 29 bits per entry, k = 20 hashes derived from one HMAC by double
// hashing) — ARCHITECTURE.md §6 item 1, THREAT_MODEL.md T12.
//
// It is about a hundred lines of standard library, which is why lazyslice has
// no Bloom filter dependency (ARCHITECTURE.md §13).
//
// The run key is random per run and never leaves the process, so the filter
// cannot be used afterwards to test a guess against the source. Nothing here
// stores a value: an entry is 20 bit positions and the source bytes are gone
// the moment Add returns.

const (
	// bitsPerCell is §6's sizing: about 29 bits per entry at a 10^-6
	// false-positive rate. internal/plan counts the same 29 against the memory
	// budget (its residualBitsPerCell), so the figure the plan prints and the
	// filter this builds are the same filter.
	bitsPerCell = 29
	// hashes is k, the number of bit positions one entry sets. It is the k that
	// minimises the false-positive rate at 29 bits per entry.
	hashes = 20
	// minBits keeps a filter built for a tiny or unknown estimate from being
	// one word long, where every query would collide. It is 64 KiB, which is
	// noise beside a 256 MiB memory budget.
	minBits = 1 << 19
	// maxBits caps the filter at 512 MiB of bits so that a wrong estimate
	// cannot allocate the machine away before the plan's budget check has
	// anything to check. The plan refuses past --memory-budget long before
	// this; this is the backstop for a caller that did not plan.
	maxBits = 1 << 32
)

// bloom is the residual filter. Add is called from transform and MayContain
// from verify, which run in different goroutines in a streaming pipeline, so
// the word array is guarded.
type bloom struct {
	key [32]byte

	mu    sync.Mutex
	words []uint64
	bits  uint64
	cells int64
}

// NewResidual returns the residual filter, sized for the number of masked cells
// the plan estimated. The plan prints its size and counts it against the memory
// budget, so a run that would need a gigabyte of filter says so before it
// starts.
//
// cells is an estimate and not a bound: a filter given too small an estimate
// degrades to a higher false-positive rate, which verify reports as unconfirmed
// hits, and never to a missed one. Under-reporting is impossible by
// construction; that asymmetry is the whole reason a Bloom filter is the right
// structure here.
func NewResidual(cells int64) pipeline.Residual {
	bits := uint64(0)
	if cells > 0 {
		bits = uint64(cells) * bitsPerCell
	}
	bits = max(bits, minBits)
	bits = min(bits, maxBits)
	// Round up to a whole word, so no bit position is unreachable.
	words := (bits + 63) / 64
	b := &bloom{words: make([]uint64, words), bits: words * 64}
	// crypto/rand.Read does not fail: since Go 1.24 it panics rather than
	// returning an error, and a filter without a key would be a filter anyone
	// holding a snapshot could test a guess against (T13's oracle, one level
	// down).
	_, _ = rand.Read(b.key[:])
	return b
}

var _ pipeline.Residual = (*bloom)(nil)

// Add records one masked cell, or one masked JSON leaf under its path.
func (b *bloom) Add(col pipeline.ColumnRef, path string, canonical []byte) {
	h1, h2 := b.positions(col, path, canonical)
	b.mu.Lock()
	defer b.mu.Unlock()
	for i := range uint64(hashes) {
		p := (h1 + i*h2) % b.bits
		b.words[p/64] |= 1 << (p % 64)
	}
	b.cells++
}

// MayContain reports whether this column, path and canonical value may have
// been masked. A false answer is certain; a true answer is confirmed against
// the source by verify (§6 item 3), and an unconfirmable hit is exit 9.
func (b *bloom) MayContain(col pipeline.ColumnRef, path string, canonical []byte) bool {
	h1, h2 := b.positions(col, path, canonical)
	b.mu.Lock()
	defer b.mu.Unlock()
	for i := range uint64(hashes) {
		p := (h1 + i*h2) % b.bits
		if b.words[p/64]&(1<<(p%64)) == 0 {
			return false
		}
	}
	return true
}

// Cells is how many entries were added, which the plan's estimate is checked
// against and the report prints.
func (b *bloom) Cells() int64 {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.cells
}

// Bytes is the filter's resident size, which counts against the memory budget.
func (b *bloom) Bytes() int64 {
	b.mu.Lock()
	defer b.mu.Unlock()
	return int64(len(b.words)) * 8
}

// positions derives the two hashes the k bit positions are built from, by
// double hashing one HMAC (§6 item 1). The column is encoded as its three
// parts and not as "schema.table.column", because the encoding has to be
// unambiguous for the same reason §5 gives: column b.c in schema a must not
// alias column c in schema a.b.
//
// h2 is forced odd so that it is coprime with the (even) word-aligned bit
// count, which keeps the k positions from collapsing onto a short cycle.
func (b *bloom) positions(col pipeline.ColumnRef, path string, canonical []byte) (uint64, uint64) {
	mac := hmac.New(sha256.New, b.key[:])
	// hash.Hash never returns an error from Write.
	_, _ = mac.Write(mask.Encode(
		[]byte(col.Table.Schema),
		[]byte(col.Table.Name),
		[]byte(col.Column),
		[]byte(path),
		canonical,
	))
	sum := mac.Sum(nil)
	return binary.BigEndian.Uint64(sum[0:8]), binary.BigEndian.Uint64(sum[8:16]) | 1
}
