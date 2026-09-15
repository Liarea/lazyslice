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
- Uses `goccy/go-yaml` (ARCHITECTURE.md §13), because the emitted header
  comments must survive a write-read-write round trip and the alternatives do
  not preserve them. Go has no YAML in its standard library, so the choice is
  between third-party parsers, not between one and the standard library.

**Test.** `go test ./internal/emit/...`, including round-trip
(write-then-read) fixtures and the withheld-`where` case.

**Never:** write a secret, a row value, or an unwithheld literal predicate to
the file; let a re-read narrow a prior decision instead of only tightening it.

## Decisions made during implementation

- **`emit.New(Options)`.** §2 fixes `Emit`'s parameters and four of §10's
  top-level keys are in none of them: `tool:`, `schema_fingerprint:`, the
  committed file this run read (without which an opt-out cannot survive the run
  that honours it) and the `--unmask` flag's own reasons. They arrive in the
  constructor, the way `internal/transform`'s schema and `internal/verify`'s
  options do.
- **`document` is a second struct** (document.go). `pipeline.Config` is keyed by
  `ref.TableRef` and `ref.ColumnRef`, which have no YAML spelling and must not
  grow one: a struct key would put `{schema: public, name: customer}` where §10
  shows `public.customer`.
- **Identifier quoting.** A part is written bare when it is already lower-case
  and needs no quoting, and double-quoted otherwise, so §10's examples stay
  bare and testdata trap 9's `public."LegacyCustomer"."EmailAddress"` survives a
  round trip. `internal/invariants` parses both.
- **`HoldsLiteral` is generous.** A quote of any kind, a dollar sign or a digit
  anywhere makes a `--where` predicate a literal. A predicate wrongly withheld
  costs the operator the flag on the next run and says so; one wrongly recorded
  puts a production value in a committed file (THREAT_MODEL.md T5).
- **The merge carries two things forward** and nothing else: `extra_patterns`
  and the `unmask:` blocks of columns the classifier still honours (a decision
  whose `Source` is `ByYmlUnmask`; an expired one is not, so it is dropped
  exactly when the classifier stopped honouring it). Everything else in a prior
  file is a record of a run that is over. The tighten-only property itself is
  enforced on **read**, in `internal/classify`, which is the only place a prior
  can change a decision.
- **`mapping_file:` is no longer carried forward, or written at all (ADR-012,
  T-0138).** ADR-006 named it a unique-index escape hatch, but nothing ever
  consumed it — no stage read the CSV or applied a replacement
  (docs/reviews/2026-09-09/REVIEW.md finding 10) — so a committed file that
  round-tripped it was false confidence. `document.config()` now refuses a
  `mapping_file:` entry by name (`*MappingFileError`, exit 2 via
  `internal/core`), `toDocument` never writes the key, and
  `pipeline.ColumnConfig` carries no field for it. T-0142 is the task that
  implements the full contract and, if it lands, restores the field and the
  merge.
- **`plan:`, `small_domain:` and `virtual_fks:` do not come back.** They are a
  record; nothing reads them as an input. `virtual_fks:` is written as text
  rather than a structured form because the one thing it exists to say is the
  rendered edge itself, not a shape a reader parses back.
- **`pipeline.PlanSummary.Unreadable` was added.** §10's plan block carries
  `unreadable:` and §2's `PlanSummary` did not name it.
- **`pipeline.Config.SourceLabel`/`TargetLabel` were added**, for §10's
  `service:` key: `Candidate.Label` had nowhere to land.
- **`ParseSize`/`FormatSize` live here** because the yml is where a byte budget
  is written and read; `internal/core` uses them for `--memory-budget` rather
  than keeping a second parser.
- **Write is atomic** (temp file plus rename): a half-written yml is a file that
  asks every question the run exists to avoid.
- **`endpointDoc` gained `params:` (T-0135, docs/reviews/2026-09-09/REVIEW.md
  finding 6).** `dsn.Ref` gained an allowlisted `Params` map (sslmode,
  sslrootcert, sslcert, sslkey, connect_timeout, application_name, options —
  see internal/dsn/CLAUDE.md) because the four identity fields alone lost a
  first run's `sslmode=verify-full` on rerun. `endpoint()` writes it, never
  omitted (an empty map, like every other field here); `document.config()`
  reads it back through `refOf`, which now takes a fifth argument and passes
  it through `dsn.FilterAllowedParams` before it reaches `Ref` — the committed
  file is hand-editable text, and "never a secret in this file" (this file's
  own Rules, THREAT_MODEL.md T5) has to hold against a `params:` block someone
  added a `password:` key to, not just against what this package itself
  writes.
  - `r == (dsn.Ref{})` in `endpoint()` became `r.IsZero()`: `Ref` is no longer
    comparable with `==` once one of its fields is a map. See
    internal/dsn/CLAUDE.md's note on `Ref.Params` for what else that broke
    outside this package's paths (`internal/core/provenance_test.go`, filed as
    T-0165 and since landed) and what it broke inside them, fixed alongside
    this (`emit_test.go`'s round-trip assertion, now `reflect.DeepEqual`).
  - ARCHITECTURE.md §10's file spec does not show a `params:` key under
    `source:`/`target:` yet (ARCHITECTURE.md is outside this task's paths, so
    it is filed rather than edited here — **T-0167**).
