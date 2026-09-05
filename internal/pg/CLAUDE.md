# internal/pg

The only package that speaks Postgres. Implements `pipeline.Source`,
`pipeline.Target` (including the gate), `pipeline.Reader`, `pipeline.Writer`,
and registers the shape-allowlist tracers on the source pool. No other
package opens a `pgxpool.Pool`.

**Contract.** ARCHITECTURE.md §2 "source and target handles" in full —
`Source.Snapshot`/`.Reader`/`.Short`/`.Release`/`.Trace`,
`Target.Gate`/`.Writer` — and §9 for the gate's exact order (reachable+CREATE
→ identity → locality → marker → emptiness).

**Rules.**
- Every source transaction is `REPEATABLE READ READ ONLY`. All five pgx
  tracers are registered on the source pool; a statement whose shape is not
  registered gets a cancelled context from `TraceQueryStart` and a recorded
  violation that fails the run (THREAT_MODEL.md T9) — no first-keyword
  allowlist (`WITH x AS (DELETE ...)` defeats that).
- `TestSourceNeverCopiesOrBatches` must keep passing: no `CopyFrom`,
  `SendBatch` or `Prepare` on a `Source` connection, ever.
- The gate's `Eligibility.Verdict` is tri-state; `NotProbed` must never be
  treated as eligible. Any probe error, timeout, or a table above the 2,000
  cap is `Refused`, never skipped-as-ok (THREAT_MODEL.md T2). RLS-enabled
  tables and probe errors count as **not empty**, never `pg_stat_user_tables`.
- Target identity is a disjunction over sameness (same `host:port/database`,
  or same `system_identifier` + `current_database()`), not a conjunction over
  difference — refuse on any match, exit 2 before any write-side connection is
  used.
- A marker is **bound** only when `source_fingerprint` (and `source_system_id`
  when both sides have it) and `schema_fingerprint` all match; an unbound
  marker authorises nothing and falls through to the emptiness check (§11.2).

**Test.** `go test ./internal/pg/...`; the gate, tracer and identity behaviour
need `go test -tags integration ./internal/pg/...` (the `TestGate*` suite in
THREAT_MODEL.md T2).

**Never:** open a `pgxpool.Pool` for the source or target anywhere but here;
let the gate return `Eligible` for a `NotProbed` table; add a rung to the
identity ladder that treats "not probed" as safe.
