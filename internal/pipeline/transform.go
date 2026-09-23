// SPDX-License-Identifier: Apache-2.0

package pipeline

import "github.com/Liarea/lazyslice/mask"

// Residual is the Bloom filter of HMAC(runKey, Encode(column, path,
// canonical(source))) for every masked cell and, for JSON documents, every
// masked leaf keyed by its path. It is built during transform and read by
// verify (ARCHITECTURE.md section 6).
//
// The run key is random per run and never leaves the process, so the filter
// cannot be used to test a guess against the source after the run. The filter is
// sized for 10^-6 false positives, about 29 bits per entry, and its size is
// printed by the plan and counted against the memory budget.
//
// Implemented in internal/transform/bloom.go, stdlib only: it is about a hundred
// lines, which is less than a dependency costs (ARCHITECTURE.md section 13).
//
// AddEmitted and Emitted are ADR-015's count (ARCHITECTURE.md section 2, "Type
// additions recorded after implementation"): for every masked cell of a column
// whose masker has a vocabulary (mask.Emitting) — scalars and array elements,
// never a JSON leaf — transform records one count against HMAC(runKey,
// Encode("emitted", column, path, canonical(output))), and verify reads the
// count back for a residual hit inside that vocabulary: a value the target
// holds more often than the masker produced it is probed as any other hit.
// The key is truncated to 64 bits and nothing stores a value.
type Residual interface {
	Add(col ColumnRef, path string, canonical []byte)
	MayContain(col ColumnRef, path string, canonical []byte) bool
	AddEmitted(col ColumnRef, path string, canonical []byte)
	Emitted(col ColumnRef, path string, canonical []byte) int64
	Cells() int64
	Bytes() int64
}

// Transformer applies the classification to a batch.
type Transformer interface {
	// Transform is pure apart from Residual.Add: it masks in place and returns
	// the batch. Being pure is what lets the same batch be replayed in a test
	// against the same key and compared byte for byte.
	Transform(b RowBatch, cls *Classification, key *mask.Key, res Residual) (RowBatch, error)
}
