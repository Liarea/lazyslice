package transform

import "github.com/Liarea/lazyslice/internal/pipeline"

// The residual filter is the only check that tests the masker rather than the
// classifier: it answers "did any value we meant to mask reach the target
// unchanged?". It is a Bloom filter of HMAC(runKey, Encode(column, path,
// canonical(source))) over every masked cell, sized for 10^-6 false positives
// (about 29 bits per entry, k = 20 hashes derived from one HMAC by double
// hashing).
//
// It is about a hundred lines of standard library, which is why lazyslice has no
// Bloom filter dependency (ARCHITECTURE.md section 13).
//
// The run key is random per run and never leaves the process, so the filter
// cannot be used afterwards to test a guess against the source.
//
// Scaffold status: no-op. Every method is present so that the interface is
// satisfied and verify can be written against it; the filter itself arrives with
// the residual-scan task.

type bloom struct{}

// NewResidual returns the residual filter, sized for the number of masked cells
// the plan estimated. The plan prints its size and counts it against the memory
// budget, so a run that would need a gigabyte of filter says so before it starts.
func NewResidual(_ int64) pipeline.Residual { return &bloom{} }

var _ pipeline.Residual = (*bloom)(nil)

func (*bloom) Add(_ pipeline.ColumnRef, _ string, _ []byte) {}

func (*bloom) MayContain(_ pipeline.ColumnRef, _ string, _ []byte) bool { return false }

func (*bloom) Cells() int64 { return 0 }

func (*bloom) Bytes() int64 { return 0 }
