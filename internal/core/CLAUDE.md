# internal/core

`Run`, `Request`, `Report`. The only place the nine stages are wired together
in order. No stage logic, no flag parsing (that's `cmd/`), no rendering (that's
`internal/render`) — `Run` calls each stage's interface method and sends
events; it never contains a SQL statement or a masking rule itself.

**Contract.** ARCHITECTURE.md §1: `core.Run(ctx, Request, event.Sink)
(*Report, error)` drives discover → introspect → classify → plan → extract →
transform → load → verify → emit, in that order, with no other entry point.
Two exported functions stand beside it and are not a second pipeline — both run
the same stages through the same `run` struct and stop where a mode stops,
because what they return is a document and events are all `Run` returns:
`Introspect` (a `SchemaSummary`) and `Preview` (a `Reviewed`). Both are
recorded below; ARCHITECTURE.md §1 is owed the correction (tracker T-0079).
`Request` mirrors the CLI flags in §8 field for field — `cmd/lazyslice` builds
one from flags, `internal/tui` builds the same struct from keystrokes — with
one field that is not a flag and must not become one (`Reviewed`, below).

**Rules.**
- `Run` is the only place stages are sequenced; a stage package must never
  call another stage package directly.
- The `Default*` constants here are the single source of the v1 flag defaults
  (ARCHITECTURE.md §8) — the CLI, the yml reader and the TUI all read these
  constants rather than repeating the literal.
- A run holds the source snapshot from the start of introspect to the end of
  extract and releases it before load and verify (ARCHITECTURE.md §1) — that
  ordering is `Run`'s responsibility and must not be reordered for
  convenience.
- The line printer, NDJSON writer and TUI are three sinks on one channel; `Run`
  must never special-case which sink is attached.

**Test.** `go test ./internal/core/...`. Full pipeline behaviour needs
`make integration` once the stages are real.

**Never:** reach into a stage's internals instead of its `pipeline` interface;
change stage order without an ARCHITECTURE.md §1 update first; let `Report` or
`Request` carry a value-bearing field (THREAT_MODEL.md T4).

## Decisions made during implementation

Recorded here because ARCHITECTURE.md is silent on them (root CLAUDE.md), and
each one is a deviation a reviewer should see rather than discover.

- **`Request.Mode`.** §8 lists five stage subcommands and §2's `Request` has no
  field that tells them apart; `Mode` is that field, and `Mode.needsTarget()`
  is why `lazyslice classify --source URL` needs no second database
  (T-0020's log).
- **`Request.Explicit`.** §10 makes the yml "a default the flag overrides,
  never a way to widen", and an int flag bound to its own default is
  indistinguishable from one the operator typed. `cmd/lazyslice` records which
  flags were passed and `planRequest` reads it; without it the file could never
  supply a take without also overriding one.
- **Defaults live here.** §3's `--take 500`, `--cap 100` and `--depth 3` are
  substituted by `normalise` before `Plan` is called. The planner takes the
  request as given and the CLI refuses `--take 0`, `--cap 0` and `--depth 0`
  with exit 2, because an int cannot mean both "unset" and "none" (T-CORE).
- **Privileges are read once.** `Source.Privileges` is called in `discover` and
  passed to the planner on `PlanRequest.Priv`; `internal/plan` no longer asks
  the catalog itself. The role the header prints and the role a refusal names
  are one read. **Narrowing:** `internal/pg`'s privilege query excludes
  partition leaves (`NOT c.relispartition`), and the planner's own query did
  not, so an unreadable *leaf* of a readable partitioned root is no longer
  attributed to its root. Owed to `internal/pg`.
- **`Schema.Fingerprint` is filled here** from `load.SchemaFingerprint`, after
  introspection, which is ADR-009's placement: the caller that holds both ends
  of §11.2's binding.
- **`dropMarkerTable`.** `lazyslice_meta` is removed from the catalog before
  any stage sees it. `internal/load/ddl` already refuses to recreate or
  fingerprint a source table of that name; leaving it in the catalog makes the
  classifier score `secret_fingerprint` as a credential (so `lazyslice classify`
  pointed at a target lazyslice wrote reports lazyslice's own table as personal
  data), gives the planner a step for it, and makes the row-count check compare
  a table nothing loaded. Refusing such a source at the gate is the better home
  and `internal/load/CLAUDE.md` already records it as owed.
- **`markSmallDomains` (domain.go).** §5's small-domain rule — `d < 2 ×
  distinct(samples)` — is not applied anywhere in the tree: `Decision.Domain`
  and `Decision.SmallDomain` were dead fields. `internal/emit` writes
  `small_domain:` from them and `internal/invariants` I2 exempts exactly those
  columns from its value-overlap check, so testdata trap 24's
  `people.marital_status` cannot pass without it. Computed here from the column
  half of `d` only (an enum's labels, a boolean, `char(n)`/`varchar(n)` at n ≤
  2); the generator half needs `mask.Constraints`, which `internal/transform`
  builds and does not export. Its proper home is `internal/classify` (which has
  the samples) or `internal/plan` (which has the row count the unique half of
  the same rule needs).
- **`smallDomainAware` (run.go).** The residual filter is wrapped so that a
  small-domain column's cells are never added to it. §5 collapses a
  small-domain `special_category` to one fixed label, so every value in the
  target equals a real source value by construction, every one is a filter hit
  and every one confirms: the run would refuse itself with exit 9 for doing
  what §5 tells it to do. The same holds for any masked enum, because §5
  requires a masked enum value to be a valid label. §6 item 6 already names
  those columns as a stated false negative, and `internal/transform` made the
  same argument when it kept boolean and number JSON leaves out of the filter.
  Its proper home is `internal/transform`, which builds the filter.
  **The exemption is bounded** (`smallDomainCeiling`, 128, domain.go): §5's
  ratio has no upper bound on `d`, so a three-hundred-label enum with two
  hundred distinct labels in the sample satisfied it and dropped out of both
  the residual filter and I2 — three hundred clinic or city names checked by
  nothing. 128 covers every domain the ratio can reach for a reason (boolean,
  a human-written enum, `character(1)` at 95). The cost is stated in
  `domain.go` and is a *false exit 9*, not a silent gap: a masked enum with
  more than 128 labels reaches exit 9 on a true statement about its domain.
  ARCHITECTURE.md §6 item 6 should carry the ceiling; that file is outside
  T-CORE's paths.
- **`readableWriter` (run.go).** §2's `pipeline.Writer` has no read method and
  every check `internal/verify` makes is a read of the target, so core opens a
  second pool on the target through `pg.Connect` and hands verify a writer with
  `Query` on it. A `Query` method on `internal/pg`'s writer is the proper home
  and is owed there (`internal/verify/CLAUDE.md` records the same deviation).
- **The first-run ladder runs here** (`resolveEndpoints`, T-0061). `cmd/`
  reaches no stage package, so §9's ladder is walked by the discover stage:
  `discover.Resolve` for `lazyslice` with no arguments, and for the five stage
  subcommands only `Discoverer.Discover`, which prints the ladder and chooses
  nothing before exit 3 names `--source`. A run that names both endpoints walks
  no rung and makes no Docker call (ADR-008 §1), which is every run in CI and in
  `internal/invariants`. `discover.Refusal` becomes a `Stop` (`refusalStop`)
  because `core` imports `discover` and the dependency cannot go the other way;
  the `Stop` is marked `sent`, since the ladder already put its own `Error`
  event on the sink and one refusal is one line.
- **The provenance of each endpoint is kept** (`sourceProv`, `targetProv` and
  their labels). §10's `source:` block records the rung an endpoint came from,
  and the run is the only place that knows it: an endpoint named on the command
  line is `FromFlag`, one from the committed yml keeps that file's own
  provenance and label, and one the ladder chose keeps the winning candidate's.
  Building both candidates as `FromFlag` — which is what this did before
  T-0060 — rewrote a committed `from: compose` / `service: db` as `from: flag`
  on every argument-free re-run. They are fields on `run` and not on `Request`
  because `Request` mirrors the §8 flag surface and there is no flag for either.
- **A gate refusal ends the run.** `internal/discover` ranks the target-shaped
  candidates and hands over one; when `Target.Gate` refuses it, the run stops at
  exit 4 with the gate's own refusal and never tries the runner-up, because
  `Request` carries one target and not a list. That is the behaviour ADR-008 §5
  gets in this build and `TestAGateRefusalEndsTheRunInsteadOfTryingTheRunnerUp`
  is what pins it (T-0062). ADR-008 §5 and ARCHITECTURE.md §9 still describe the
  tie-break as running "among eligible targets" and are owed the correction;
  both files were outside the paths of the tasks that built this.
- **`ParseMemoryBudget` is exported.** §1 gives this package `Run`, `Request`
  and `Report`, and `Introspect` and this are the two entry points beyond them.
  It exists because `cmd/lazyslice` refuses a misspelled `--memory-budget` at the
  flag surface and did that by importing `internal/emit` — the one stage package
  a file that may reach none still reached (cmd/CLAUDE.md). It is a one-line call
  of `emit.ParseSize`, the same function `planRequest` uses, so the flag and the
  planner cannot disagree about a size (T-0060's review round).
- **The plan print is partial.** §3.5 lists eleven things the plan prints; this
  build emits one line per step, the polymorphic pairs, the unmapped values and
  the estimate. The SCCs, the unindexed edges, the not-recreated counts and the
  small-domain list are in the yml and not yet on the transcript.
- **`ModeVerify` runs the whole pipeline.** A standalone `lazyslice verify`
  needs the run's residual filter, which is random per run and never leaves the
  process (§6 item 1), so it cannot be reconstructed from a target. Until §6
  says how, `verify` re-runs everything rather than reporting a green tick over
  a residual scan that tested nothing. The subcommand's own `Short` text now
  says so — "Re-run the whole slice into the target and re-check it (drops and
  reloads the target)" — because the argument belongs where a user reads it and
  not only here. §8's row still describes it as a re-check and is owed the same
  correction.

## Decisions made during the T-CORE review round (2026-09-06)

- **The bounded channel is wired** (`channel.go`). §7 requires `core.Run` to
  send into a channel of 256 with `Progress` dropped when full and the drop
  counted; `run.send` called the sink synchronously, so a slow renderer
  back-pressured the pipeline and nothing counted anything. `eventChannel` is
  that channel: one drain goroutine, `Progress` dropped on a full buffer,
  everything else blocking, and `run.progress.dropped` printed once at the end
  of a run that dropped anything. Every event still reaches the sink from one
  goroutine, so a sink needs no locking of its own.
- **`markerWarnings` moved from `openTarget` to `move`.** All three of §11.2's
  bound-marker warnings were guarded on `r.keyFP` and `r.cls`, and neither is
  filled until `resolveKey` and `classifyStage`, which run *after* discover: the
  warnings could never print. The gate's `Eligibility` is kept on the run
  (`r.gate`) and the comparison happens in `move`, after the key and the
  classification exist and before the loader's first write.
  `target.marker.tool_changed` is emitted from the same place now, from
  `pipeline.Eligibility.PrevToolVersion` — a field this change adds and
  `internal/pg` still has to fill from the marker row. Until it does, the third
  warning has nothing to compare and stays silent; that is owed to `internal/pg`
  and is the last of §11.2's three.
- **`repo.Protect` runs on the read path too** (`resolveKey`). §9 protects the
  secret "before writing or using it", and the check was inside the
  `fs.ErrNotExist` branch, so a `lazyslice.secret` already in the worktree — the
  state of a clone whose key was committed — was read and used with no tracked
  check and no `.gitignore` entry. The check now runs before the file is read;
  `$LAZYSLICE_SECRET` is still exempt, because that branch has no file.
- **A masking refusal is no longer reported as an interrupt.** `transform`
  cancels `extract` when a masker refuses a value, and extract then returned a
  bare `context.Canceled` that the failure switch preferred over the refusal:
  the run printed "interrupted" and exited 130 for a `*transform.Refusal`.
  `context.WithCancelCause` records that the cancellation was this code's own,
  and `move` drops `extractErr` when the cause is the refusal. `asStop` also
  gains a `CodeInterrupted`/130 case, so a genuine Ctrl-C makes the Error event
  and the process exit carry the same number — `cmd/lazyslice` used to print
  exit 1 into the stream and return 130 from the process.

## Decisions made during T-PIN (2026-09-08)

- **`Preview` is a third entry point** (run.go). §1 gives this package `Run` and
  "no other entry point"; `Introspect` was already the exception, for the reason
  restated here — a document is not an event, and events are all `Run` returns.
  `Preview` runs the same stages through the same `run` struct with `PlanOnly`
  forced and returns a `Reviewed`: the schema fingerprint (ADR-009), the
  classifier's own verdicts and the two endpoints one pass resolved. It exists
  because `--tui` runs the pipeline twice, and before it nothing tied the two
  passes together — the operator
  reviewed a classification and a plan built over one snapshot and one walk of
  the discovery ladder, and a second pass took its own of each and wrote the
  target. **Owed:** ARCHITECTURE.md §1 should name both exceptions, and
  cmd/CLAUDE.md should carry a `core.Preview` bullet beside its `core.Introspect`
  one; both files were outside T-PIN's paths (tracker T-0079).
- **`Request.Reviewed` is the one field that is not a flag.** §8 has no flag for
  it and must not gain one: it carries no operator intent, only the identity of
  what was reviewed, and a `--reviewed-fingerprint` on the command line would be
  a way to assert a review that never happened. `cmd/lazyslice`'s `runTUI` is the
  only filler in the tree. It is compared in two places, each the first point at
  which the thing it pins exists: `checkReviewed` (the two endpoints and the
  schema) immediately after `introspectStage`, and
  `checkReviewedClassification` immediately after `classifyStage`. Both are
  before the plan, before extract and before every write.
- **The classification is pinned as well as the schema, and the two are not the
  same fact.** The reasons screen is the masking review, and a classification is
  not a function of the DDL: `internal/classify` reads `Table.Samples`, drawn
  with `TABLESAMPLE SYSTEM ... REPEATABLE`, which is identical between two passes
  only while the data is. A column that reaches the mask threshold on a value
  signal alone (THREAT_MODEL.md T1's `ref` column holding emails) can fall below
  it when the second pass draws different blocks and be copied in clear by the
  run that writes, with the schema fingerprint unchanged throughout. What is
  compared is `classifierFingerprint`: `Classification.Fingerprint` recomputed
  from the committed yml alone, with this run's `--unmask` flags left out,
  because the reasons screen writes those and pinning them would refuse the
  operator for using the screen. With no `--unmask` flag the run's own
  classification already is that classification and no second call is made.
- **The pin's exit is ADR-005's 12.** That table calls 12 "plan refused" and
  lists four cases, none of them this one, and this refusal happens *before* the
  planner runs. It is the nearest fit — the operator's approved plan is what is
  being refused — and T-PIN's brief named it. ADR-005 is accepted and frozen, so
  widening 12 or giving the review pin an exit of its own needs a superseding
  ADR (tracker T-0079).
- **The pin does not cover the plan, and `afterThePlan` still reprints it.** An
  earlier round of this task dropped the plan lines from an unchanged second
  pass on the argument that the pin made them redundant. It does not: a plan is
  not a function of the schema. Each step's `Mode` — `child_ok`, `parent_only`,
  `lookup`, `schema_only`, and so whether a table's children are followed and
  whether it is copied at all — comes from the rows the discovery walk popped,
  and `Plan.Virtual` comes from `internal/plan`'s bounded data sample of the
  source, so two passes with one fingerprint over one pair of endpoints can
  follow different virtual edges and copy a different set of tables. §3.5 wants
  every step and every virtual FK stated before the extraction, and the second
  pass is the one that extracts. `plan.estimate` is likewise the hold estimate
  for the snapshot that pass holds (THREAT_MODEL.md A9). So the plan is printed
  by both passes and the classification, which the pin does cover, is printed
  once.
- **The wiring is pinned structurally.** `checkReviewed`, its classification
  half and `Preview`'s forced `PlanOnly` are each one statement, and a unit test
  of the comparison functions alone stayed green with all three deleted — a
  safety rail nothing noticed the absence of.
  `TestTheReviewPinIsWiredIntoTheRunItGuards` parses `run.go` with `go/ast`, in
  the manner of `TestEveryLadderOptionTheRequestCarriesIsCopied`, and asserts
  both the presence and the position of the two checks inside `execute`.
  `cmd/lazyslice` does the same for `runTUI` over its `pinned` helper. Driving
  either through `Run` needs two Postgres servers and a schema that changes
  between them, which is an integration test and not this one.
- **The catalogue's dangling back-references.** Two comments in
  `internal/event/catalogue.yml` pointed at a comment on
  `target.refused.start_timeout` that T-0071 deleted when it gave that code its
  own `ArgContainer`; both clauses are gone. The third,
  `internal/load/ddl/recreatable.go:53`, was outside T-PIN's paths and is filed
  as tracker T-0078.
- **A root `lazyslice` binary is not gitignored.** T-PIN's review found a ~32MB
  Mach-O left at the repo root by a bare `go build ./cmd/lazyslice`; that
  binary is deleted. `make build` sends its `-o` to `bin/`, which `.gitignore`
  covers, but nothing in that file's build section matches the root artefact a
  bare `go build` writes, so `git status` reports it as untracked and
  `implement.js`'s `git add -A` commit step would put it in history
  permanently. The fix is a `/lazyslice` line in `.gitignore` — anchored, or it
  would also ignore the tracked source directory `cmd/lazyslice/` — and that
  file was outside T-PIN's paths: tracker T-0080.

## `asStop` is exhaustive or it is nothing (T-TORTURE)

There are **six** refusal types, not five: `internal/load/ddl`'s is returned by
`load.Load` unwrapped and needs its own case. Without it an operator whose schema
has a column default calling one of the application's own functions — Mastodon's
`timestamp_id`, GitLab's `gen_random_uuid_v7`, both real, both in
`testdata/torture/` — was told "lazyslice failed for a reason it has no code
for; run with --debug" and given exit 1 to branch on, with the correct code and
the whole correct sentence printed *inside* the message
(`testdata/regressions/002-function-default-refusal-uncoded.sql`).

The rule the file already stated is the one that was broken: "an error no stage
claimed is exit 1, never a code a CI job branches on" is only true if every stage
that has a code is claimed. A new refusal type anywhere under `internal/` is a
case here in the same change, and `docs/ERRORS.md` is where to check whether the
code already exists — both of `ddl`'s did, with `stage: plan` and `exit: 13`,
for a refusal that was reaching people as an internal error.
