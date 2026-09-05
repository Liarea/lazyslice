# internal/emit

Reads and writes `lazyslice.yml`, and the tighten-only merge over
`pipeline.Config` on re-run. No decisions are made here — classification,
planning and masking choices arrive already made; this package only records
and re-reads them.

**Contract.** ARCHITECTURE.md §2 "verify, emit" and §10 (the full file spec):
`Emitter.Emit(*Plan, *Classification, *Report, PlanRequest, source, target
Candidate, keyFP) (*Config, error)`, plus `Write(path, *Config) error` and
`Read(path) (*Config, error)`.

**Rules.**
- Never a secret, never a row value, in the written file — `Config`'s
  `secret:"true"` fields are refused by the writer, checked by
  `TestConfigHasNoSecretField` (THREAT_MODEL.md T5).
- A `--where` predicate containing a literal is withheld and recorded only as
  `where_fingerprint`; a later run finding a fingerprint with no `--where` is
  exit 2, never a silent re-derivation of the old predicate (§10,
  THREAT_MODEL.md T5).
- Read-back can only tighten (ADR-004): a pattern in the file may add a
  category or raise a confidence, never lower or remove one
  (`TestConfigCannotLowerConfidence` lives against this contract even though
  the check itself runs in classify).
- Written on success; on `--plan` only when `--config PATH` is given
  explicitly, and then with `plan_only: true` and no `snapshot_id`.
- Uses `goccy/go-yaml`, not the standard library, specifically because the
  emitted header comments must survive round-trips (ARCHITECTURE.md §13).

**Test.** `go test ./internal/emit/...`, including round-trip
(write-then-read) fixtures and the withheld-`where` case.

**Never:** write a secret, a row value, or an unwithheld literal predicate to
the file; let a re-read narrow a prior decision instead of only tightening it.
