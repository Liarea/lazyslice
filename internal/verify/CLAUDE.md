# internal/verify

Proves what the run did: FK validation, the residual scan and its capped
source confirmation, the second net, sequence/row-count checks, and the
unmasked-column sample compare. The only stage that tests the *masker* rather
than the classifier (THREAT_MODEL.md T12). Never uses the run snapshot — it is
released by the time this stage starts.

**Contract.** ARCHITECTURE.md §2 "verify, emit" and §6 (the whole algorithm):
`Verifier.Verify(ctx, Source, Writer, *Schema, *Plan, *Classification,
Residual, *LoadResult) (*Report, error)`.

**Rules.**
- Fails closed, always: a residual hit that cannot be confirmed (source won't
  open, a probe errors, the role lacks `SELECT`, the cap is reached with hits
  untested) is exit 9 naming the reason — never a printed note
  (ARCHITECTURE.md §6 item 3, THREAT_MODEL.md T12).
- Confirmation uses `Source.Short` (a fresh short `REPEATABLE READ READ ONLY`
  transaction), never the run's own snapshot — the indexable probe first, the
  case-folded probe only if that's false, capped at
  `--residual-probe-cap` (THREAT_MODEL.md T4's value-egress note: a probe
  parameter can land in the source's own log).
- The second net re-runs the classifier's own validators at full coverage over
  every unmasked, non-opted-out column, and the string leaves of every masked
  JSON column (ARCHITECTURE.md §6 item 4) — it catches what the 200-row
  sample missed, never a category outside the rule pack (THREAT_MODEL.md T1).
  A column carrying a `--unmask TABLE.COL=REASON` opt-out (ARCHITECTURE.md §8)
  is deliberately outside the second net's coverage — that is why the opt-out
  requires a reason and expires on `TypeFP` change, not a scan the opt-out
  would otherwise fail every time.
- A source-changed mismatch (row absent, or a byte differs since the snapshot)
  is reported and counted, not failed — only a *confirmed* residual hit or a
  `possible`-or-above second-net hit is a failing exit.
- `schema` supplies column types for canonicalisation and identity columns for
  the sample fetch — verify must never reclassify from scratch.

**Test.** `go test ./internal/verify/...`; the confirmation/second-net/FK
behaviour needs `go test -tags integration ./internal/verify/...` against both
fixtures.

**Never:** use the run's own (released) snapshot; treat an unconfirmable
residual hit as anything but exit 9; skip the cap on confirmation probes; treat
a source-changed mismatch as a pass.
