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
type Residual interface {
	Add(col ColumnRef, path string, canonical []byte)
	MayContain(col ColumnRef, path string, canonical []byte) bool
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
