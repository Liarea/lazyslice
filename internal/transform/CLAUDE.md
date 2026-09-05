# internal/transform

Applies each column's `Decision` to every batch between extract and load: mask
invocation, JSON-leaf walking, per-category canonicalisation, FK-decision
propagation, and the residual Bloom filter (`bloom.go`). The masking
*algorithms* (generators, `Domain()`, the HMAC scheme) live in the top-level
`mask` module (ADR-006); this package calls them, never reimplements them.

**Contract.** ARCHITECTURE.md §2 "extract, transform, load":
`Transformer.Transform(b RowBatch, *Classification, *mask.Key, Residual)
(RowBatch, error)`; `Residual` (`Add`/`MayContain`/`Cells`/`Bytes`) is
implemented in `bloom.go` per ARCHITECTURE.md §6.

**Rules.**
- `Transform` is pure apart from `Residual.Add` — same key, same
  classification, same input, same output, every time. This is what invariant
  I3 (two runs, byte-identical targets) rests on; do not introduce
  non-determinism (a clock read, a random source outside `mask.Key`) here.
- Every masked cell, and every masked JSON leaf keyed by its path, must be
  added to the residual filter (ARCHITECTURE.md §6 item 1) — a code path that
  masks a value without calling `Residual.Add` defeats THREAT_MODEL.md T12's
  only control.
- JSON leaf rules are exact (ARCHITECTURE.md §4 "Free text and JSON"): string
  leaves get the category masker by key name, numbers/booleans are re-derived
  from `h`, `null` stays `null`, structure and key names survive, wildly
  varying keys or an audit/log/history table's `jsonb` collapses to `{}`.
- No masker may be loaded at runtime — only the compiled registry in `mask`
  (ADR-006, THREAT_MODEL.md T7); never accept a file path, plugin or
  expression as a masker.
- `bloom.go`'s `MayContain` fails closed while unimplemented (`true` for
  everything) — do not "fix" that with `false` to make a test pass; it stays
  closed until the real filter lands.

**Test.** `go test ./internal/transform/...`, including
`TestFreeTextLengthUncorrelated` and the colliding-pair encoding fixtures.

**Never:** mask a cell without adding it to the residual filter; accept a
runtime-loaded masker; let `Transform` depend on anything but its arguments
and `mask.Key`.
