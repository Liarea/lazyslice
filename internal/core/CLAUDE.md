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
  `Query` on it. It embeds the `Writer` *interface*, so it forwards
  `RegisterTypes` — `Writer`'s fourth method since T-0093 — to the real writer
  rather than dropping it. That embedding is exactly why the method is on
  `Writer` at all: while registration was an optional `pipeline.TypeRegistrar`,
  this wrapper was not a registrar, and putting it one line earlier in `run.go`
  would have turned every run's type registration off with no compile error. A `Query` method on `internal/pg`'s writer is the proper home
  and is owed there (`internal/verify/CLAUDE.md` records the same deviation).
- **A type name an operator typed is resolved here too** (`resolveType`,
  `names.go`). `--allow-type-literal TYPE=REASON` is the escape from §11.1's
  type-literal refusal (the T-REDFIX review's fourth finding), and it is
  resolved against the source's own `Schema.Enums` and `Schema.Domains` on
  exactly the rules `resolveTable` follows: a qualified name must exist, a bare
  one must be unambiguous, and a name matching nothing is **exit 2** rather than
  a stored opt-out. That is `--unmask`'s own argument applied to a type — an
  opt-out that silently never applied looks exactly like one that did, and this
  one suppresses a refusal whose job is keeping a person's value out of the
  target's catalog. The resolved names travel on
  `pipeline.PlanRequest.AllowTypeLiterals`, and `internal/plan` puts the ones
  the schema actually carries onto `Plan.AllowedTypeLiterals`, which is what
  `internal/verify`'s catalog pass honours.
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

## Decisions made during the 2026-09-14 review of T-0161

- **`resolveKeyIfPresent` no longer shares `resolveKeyState` with
  `resolveKey`.** The first landing of `keyBeforePlan` (T-0161, which fills
  `pipeline.PlanRequest.Key` ahead of `planStage`) ran a plan-only run's key
  lookup through the same `resolveKeyState` a writing run uses, which calls
  `repo.Protect` — and `repo.Protect` is not read-only: it appends
  `lazyslice.secret` and `snapshots/` into `.gitignore`
  (`internal/repo/repo.go`'s `appendMissing`) and can hard-abort with
  `CodeSecretTracked`. `lazyslice plan` and the TUI's Preview pass — commands
  that write nothing — were therefore mutating the operator's `.gitignore`
  and could refuse over a tracked secret file on the one command an operator
  would reach for to inspect that state. `resolveKeyIfPresent` now reads
  `$LAZYSLICE_SECRET` and `r.req.SecretFile` directly and calls neither
  `repo.Protect` nor the tracked-file check: that check exists to stop a
  *write* from trusting a key a clone should not (THREAT_MODEL.md T6), and a
  plan-only run performs no write for it to protect. A writing run still gets
  the full check, unchanged, through `resolveKey`.
- **`keyBeforePlan` and `planRequest` read one `planOnly()` method rather than
  two copies of `r.req.Mode == ModePlan || r.req.PlanOnly`.** The same review
  found `internal/plan/ddlliteral.go` inferring "this is a plan-only run" from
  `PlanRequest.Key == nil` alone, held up only by those two conditions
  agreeing by construction — nothing pinned them together, and a nil key
  reaching that stage from anywhere else (a bug, or a future caller that
  skips `keyBeforePlan`) was treated the same as a deliberate plan-only
  pending state. `PlanRequest` now carries `KeyPending`, set by `planRequest`
  from the one `planOnly()` call `keyBeforePlan` also branches on, and
  `internal/plan/CLAUDE.md`'s T-0161 section records the other half: that
  package's gate now reads `KeyPending`, not a nil `Key` on its own.
  `TestKeyBeforePlanRunsBeforePlanStage` is the AST ordering pin (in the style
  of `refingerprint_test.go`'s own), and `TestResolveKeyIfPresent*` /
  `TestResolveKeyWithNoSecretCreatesOne` / `TestKeyBeforePlanDispatchesOnPlanOnly`
  (`keybeforeplan_test.go`) hold the three states the brief's "do not create a
  secret for a plan-only run" requirement needs: a plan-only run with no key
  anywhere touches no file, a plan-only run with one already present fills
  `PlanRequest.Key`, and a writing run with neither creates one.
- **`execute`'s plan-only early return calls `planOnly()` too.** The first
  landing of the bullet above left `execute`'s own dispatch — the `if
  r.req.Mode == ModePlan || r.req.PlanOnly { return nil, r.emitPlanOnly() }`
  a few lines above the `move()` call — as a third hand-kept copy of the
  condition `planOnly()` exists to collapse to one, found in the same review's
  second pass over this change. The consequence is worse than a stray literal:
  if that copy ever narrowed relative to `planOnly()`, a run for which
  `planOnly()` returns true would skip the early return and reach `move()`
  with `r.key` at its zero value — `mask.Key` is `[KeyLen]byte`, so a
  zero-value key is a *structurally valid* one, and `move()` would mask every
  value under it and write the load, with `SecretFingerprint` empty on the
  marker row and nothing checking that. `execute` now calls `r.planOnly()` at
  that site, so all three sites (`keyBeforePlan`, `planRequest`, `execute`)
  read the one method, and `TestExecutePlanOnlyGuardCallsPlanOnly`
  (`keybeforeplan_test.go`) parses `run.go` and asserts the guard on
  `execute`'s `return nil, r.emitPlanOnly()` is a call to `r.planOnly()`
  rather than any equivalent inline expression — `TestKeyBeforePlanRunsBefore
  PlanStage` only pins that `keyBeforePlan` runs before `planStage`, which a
  drifted literal at the `emitPlanOnly` guard would still pass. `move()` also
  gained a belt: it now refuses with `CodeInternal` at its first line if
  `r.keyFP == ""`, so a plan-only run that reaches it by any future drift
  aborts instead of masking with a zero key.

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
`timestamp_id`, which `testdata/torture/mastodon` carries unedited, and GitLab's
`gen_random_uuid_v7`, real upstream but dropped by that fixture's 43-table
subset — was told "lazyslice failed for a reason it has no code
for; run with --debug" and given exit 1 to branch on, with the correct code and
the whole correct sentence printed *inside* the message
(`testdata/regressions/002-function-default-refusal-uncoded.sql`).

The rule the file already stated is the one that was broken: "an error no stage
claimed is exit 1, never a code a CI job branches on" is only true if every stage
that has a code is claimed. A new refusal type anywhere under `internal/` is a
case here in the same change, and `docs/ERRORS.md` is where to check whether the
code already exists — both of `ddl`'s did, with `stage: plan` and `exit: 13`,
for a refusal that was reaching people as an internal error.

## Decisions made during T-HARD-A (2026-09-08)

- **The classification fingerprint is recomputed after the plan** (`refingerprint`,
  run.go; `classify.Refingerprint`; T-0101). ARCHITECTURE.md §5 makes
  `Classification.Fingerprint` a function of the rule-pack version and, per
  column, its category and its **masker**, and §11.2 prints `classification
  changed — masked values will differ` when a bound marker's recorded one
  differs. But the masker is not final when `Classify` returns: §5's own
  unique-index rule says the *plan* picks the widest registered generator for
  the category, and `internal/plan/unique.go` writes that pick back onto the
  decision — which `internal/transform` masks with and `internal/emit` records.
  So a column that became unique between two runs (a new unique index, or a
  `--take` past `d_required`) changed every masked value in it and changed no
  fingerprint, and the one warning §11.2 has for that case did not print. The
  pick cannot move into `internal/classify`, which has the samples and the
  unique indexes but no row count and no database, so the fingerprint moved
  instead: `execute` calls `refingerprint` immediately after `planStage` and
  before `emitPlanOnly`, so a `--plan` writes the same value a writing run
  would. It is the same structural reason `domain.go` writes `Domain` and
  `SmallDomain` after `Classify` returns. **`r.classFP` is deliberately not
  touched**: that is the review pin's value — the classifier's own verdicts,
  which is what `--tui`'s reasons screen showed somebody — and it is compared
  before the plan on both passes. With no escalation `Refingerprint` returns
  what `Classify` computed, byte for byte
  (`TestRefingerprintIsAnIdentityWhenThePlanChangedNothing`).

  The defect was an **ordering** one, so the order is pinned at the call site
  and not only in the function: `TestTheFingerprintIsRecomputedAfterThePlanAnd`
  `BeforeItIsWritten` parses `run.go` and asserts `execute` calls `planStage`
  before `refingerprint`, and `refingerprint` before `emitPlanOnly` and `move`.
  It is structural for the same reason the review pin's wiring test is
  (`TestTheReviewPinIsWiredIntoTheRunItGuards`, whose `receiverCalls` it
  reuses): reaching `refingerprint` through `execute` needs two Postgres
  servers, and the claim is about two statements' order. Without it, deleting
  the call or moving it above `planStage` left every test in the repo green
  while restoring the bug exactly.

  A consequence worth knowing before reading a warning: because the value is
  computed after the plan, it is a function of **plan inputs** too, not of the
  classification alone. A different root, `--take`, `--depth` or `--skip-table`
  changes the planned row count, which changes what `d_required` escalates,
  which moves the fingerprint — so `classification changed — masked values will
  differ` can print for a table whose rows are not in this target at all. That
  is the conservative direction, but it is why the line fires without the rule
  pack or the schema having moved. `internal/pipeline/classify.go`'s comment on
  the field says the same.

  **Owed:** ARCHITECTURE.md §2's comment on the field and §5's
  determinism-scope paragraph both describe it as computed inside `Classify`;
  tracker **T-0111**.
- **§11.1's not-recreatable refusal is raised here, at plan** (`planStage`,
  T-0097). §11.1 says exit 13 is raised "at plan — before the snapshot is used
  for keys and before anything in the target is dropped", and it was raised by
  `load.Load` as its first statement, because a stage package may not import
  another stage package and the caller §11.1 describes is this one, which did
  not exist when the loader landed. `planStage` now calls `ddl.Recreatable`
  before `planRequest` and before `Plan`, so the refusal costs one introspect
  and no key query. `load.Load` no longer checks: one refusal, raised once.
  That makes it an **unchecked precondition of `Load`** — the caller must have
  run `ddl.Recreatable` over the same `*pipeline.Schema`, and nothing in
  `internal/load` enforces it; `load_integration_test.go` calls
  `ddl.Recreatable` over its fixture, but as a fixture assertion, not a guard.
  `internal/load/CLAUDE.md` records the consequence. `asStop`'s `*ddl.Refusal`
  case is unchanged and now converts a refusal that arrives from the plan rather
  than from the loader
  (`testdata/regressions/002-function-default-refusal-uncoded.sql`). **One** of
  the ten schemas in `testdata/torture/` reaches it as the fixtures stand —
  Mastodon, unedited — which is what made the old ordering measurable: that
  operator paid for a whole extract, holding the source snapshot throughout,
  before being told the target could not be built. GitLab reaches the same
  refusal upstream on two objects its 43-table subset removes, so its fixture
  exits 0 (`docs/TORTURE.md`).
- **The run lease is taken before the gate, and the run id is made here**
  (`acquireLease`, T-0130). ARCHITECTURE.md §9's verdict is acted on several
  stages later — introspect, classify and plan all run between it and the first
  `DROP` — so `openTarget` takes `internal/pg`'s `Lease` on a target connection
  of its own before `Gate`'s first probe, and `close` releases it after
  everything else of the target's business, because §11.2 holds it "until the
  marker is finished" and the marker is closed inside `Load`. That connection
  holds an **open transaction** for the whole run — the lock is
  `pg_try_advisory_xact_lock`, so that it cannot outlive the session on a target
  behind a pooler (`internal/pg/CLAUDE.md`, "The run lease") — so a target
  session sitting idle in transaction for the length of a run is the lease and
  not a leak. A held lease is
  exit 4 and `pg.CodeLeaseHeld` naming the holder; **anything else that goes
  wrong is `unreachableTarget`**, which is the gate's reachability precondition
  arriving one statement earlier than it used to — same code, same exit, same
  `{host}`/`{reason}`, which is what `provenance_test.go`'s `fillCandidates`
  pins. `pg.NewRunID` is called here rather than inside `pg.StartRun` because the
  lease names itself with the id before the marker row exists; `loadRun` passes
  the same id on, so the refusal a second run prints and the row this one writes
  are one story. `loadRun` also carries `gate.MarkerBound/MarkerRunID/MarkerStatus`,
  which is what `internal/load`'s lock-and-recheck re-verifies before each drop.
  `asStop`'s `*load.Refusal` case now fills `{count}` from `Refusal.Rows`, which
  is the row count a changed target is named with.
  `race_integration_test.go` holds four regressions, all built on the same
  instrument — the source held under `ACCESS EXCLUSIVE` by a session of the
  test's own, so the run cannot pass introspect until the test lets it, which is
  a point strictly after the gate and strictly before the first drop:
  `TestARowInsertedAfterTheGateIsNotDropped` (the reviewer's script),
  `TestASecondRunIsRefusedWhileAnotherHoldsTheTarget` (the lease),
  `TestAMarkerDeletedAfterTheGateIsNotTruncated` (the marked-target branch of the
  recheck, which is the production reload path) and
  `TestARefusalOnTheSecondTableLeavesTheFirstDropped`, which pins the **scope**
  of the rollback: the drops are one transaction each, so a refusal on a later
  table leaves the earlier ones dropped, and both amendments say so in those
  terms rather than "nothing was destroyed".

## Decisions made during the 2026-09-14 fix round of T-FAILUX

- **`move`'s two stage goroutines (extract, transform) each recover their own
  panic and send it as a `panicError` on their existing done channel**
  (`extractDone`/`transformDone`, now fields on `run` and not `move`'s own
  locals). `guardedExecute`'s recover (`cmd/lazyslice/main.go`) only wraps
  `root.ExecuteContext` on the main goroutine; without a recover of its own,
  a panic on either — transform is where masking runs, the most panic-prone
  code in the tree — printed Go's raw stack trace and exited 2 whatever
  `--debug` said. `internal/load`'s `CopyFrom` goroutine does the same with
  its own `copyPanicError`, into `copyResult.err`. `eventChannel`'s drain
  goroutine (`channel.go`) recovers a panic in `sink.Send` the same way, kept
  as `sinkPanic` and surfaced by `Run`'s defer as `ch.panicked()` when the run
  otherwise succeeded — a sink is caller-supplied render code, the one call
  into it not already covered by a stage's own done channel.
- **`run.extractDone`/`run.transformDone` exist as struct fields, not `move`'s
  locals, so `close` can join them too.** `move`'s own joins
  (`<-r.extractDone`, `<-r.transformDone`) are ordinary statements a panic
  skips; a panic unwinding out of `move` (in `load.Load`, say) used to leave
  `close` free to release the snapshot and close the target pools while
  extract or transform was still running against them. `close` now joins
  both — a no-op on the ordinary path, since `move` has already read and
  nilled them by the time it returns.
- **`eventChannel.Send` is now safe against a channel already closed**, guarded
  by an `RWMutex` around a `closed` flag rather than relying on every sender
  being joined first (belt to the joins above's suspenders): `close` takes the
  write lock before it closes `ch`, so a `Send` already past the closed check
  cannot land on a closed channel, and one that starts after `close` sees
  `closed` and never touches `ch`.
- **`--debug` was wired into `report()` (`cmd/lazyslice/main.go`), not only
  `guardedExecute`'s panic recover**: an ordinary `*Stop` now prints its
  wrapped cause under `--debug`, and a cause that is a recovered
  stage-goroutine panic (this round's three `panicError`/`copyPanicError`
  types, matched structurally by a `Stack() []byte` method so `report` need
  not import three packages) prints its captured stack too. `docs/FLAGS.md`'s
  other promise for the flag — "the statement trace on error", via
  `pipeline.Source.Trace()` — is still unwired: doing that needs the tracer
  threaded through `Run`'s return path, which this fix round's paths did not
  cover. Filed as **T-0174**; the flag's own description text
  (`cmd/lazyslice/main.go`) was corrected to what `--debug` actually does
  today rather than left promising the untracked half.

## Decisions made during the second 2026-09-14 review round of T-FAILUX

- **`close`'s joins on `extractDone`/`transformDone` needed a cancel to join
  against, not just the joins themselves** (`run.abortStages`, `move`'s
  `pipelineCtx`). The round above joins both channels in `close` so a panic
  out of `move` cannot leave it racing the stage goroutines, but a join is
  only as good as whatever unblocks the other end: `move`'s `defer
  cancelExtract(nil)` still runs during a panic unwind and does unblock
  extract, but transform's masked-send select (`case masked <- out: case
  <-ctx.Done():`) was reading Run's outer `ctx`, which nothing on the unwind
  path ever cancels, so with `batchBuffer` at 2 that goroutine — and `close`
  behind it — blocked forever. `move` now derives one `pipelineCtx` from `ctx`
  ahead of `extractCtx` (which is now `pipelineCtx`'s child, not `ctx`'s) and
  stores its cancel as `r.abortStages`; transform's select reads `pipelineCtx`
  instead of `ctx`; `close` calls `abortStages` before its joins. Ordinary
  Ctrl-C behaviour is unchanged — `pipelineCtx` is still cancelled whenever
  `ctx` is, since it is a child of it — and the ordinary return path is a
  no-op: `move`'s own `defer cancelPipeline(nil)` already ran by the time
  `close` calls `abortStages` again. `TestCloseUnblocksTransformOnAPanicUnwind`
  (`failux_test.go`) reproduces the shape a mid-load panic leaves behind —
  `extractDone` already filled, `transformDone`'s goroutine parked in the same
  select `move`'s blocks in — without driving an actual panic through a live
  pipeline, for the reason `TestTheReviewPinIsWiredIntoTheRunItGuards` gives
  for testing its own wiring instead of driving it end to end.
- **`Introspect` and `Preview` promote a sink panic too, not only `Run`.** The
  round above added `ch.panicked()` to `Run`'s defer alone; `Introspect` and
  `Preview` share `eventChannel` and only called `ch.close()`, so a panicking
  sink during `lazyslice introspect --json` or the TUI's preview pass was
  recovered on the drain goroutine and never reported — nil error, exit 0,
  whatever the panic actually interrupted. Both now carry the same
  `if perr := ch.panicked(); perr != nil && err == nil { err = asStop(perr);
  r.report(err) }` before `ch.close()`, which needed both functions' bare
  `return nil, err` returns turned into named returns (`summary`/`reviewed`,
  `err`) so the defer can see and override the result the same way `Run`'s
  does. `TestIntrospectAndPreviewPromoteASinkPanic` (`failux_test.go`) asserts
  the wiring structurally, the way `TestTheReviewPinIsWiredIntoTheRunItGuards`
  asserts `Preview`'s `PlanOnly` line in the same file — driving an actual
  sink panic through either function needs a live discover/introspect pass
  against a real source.

## Decisions made for the 2026-09-15 red team

- **`checkSecretFile` judges the file, not the path** (THREAT_MODEL.md T6, A15
  and A17). `os.WriteFile` follows a symlink and `repo.Protect` applies both the
  `.gitignore` entry and the `git ls-files --error-unmatch` check to the *link*
  path, so `ln -s Dropbox/leaked.key lazyslice.secret` put the masking key —
  T13's guess-confirmation oracle for every snapshot ever made with it — into a
  cloud-synced folder while the transcript said the file had been added to
  `.gitignore` and written. A secret path that is a symbolic link is exit 5
  before `repo.Protect` opens anything; it is a **refusal** rather than a
  resolve-and-protect, because following a link out of the repository would mean
  writing the key to a path the operator did not name. And a file whose mode
  grants group or other any bit is exit 5 naming the `chmod 600` to run: T6
  promises "created 0600" and nothing re-checked an existing one, so a 0644 key
  in a CI image was readable by every account under exit 0. Not a silent
  `chmod`, because a key that has been world-readable may already have been
  read. The check runs on both key paths and **after** the `$LAZYSLICE_SECRET`
  branch on each, because that branch has no file.
- **`CodeTargetSameCluster` is wired** (ARCHITECTURE.md §9 rule 1,
  THREAT_MODEL.md T2). `internal/pg` has computed `Eligibility.SameCluster`
  since the gate was written and **nothing here or in `internal/render` read
  it**, so a run that wrote into a database on the source's own server said
  nothing about where the write had landed. The gate's own test asserted the
  boolean and never the rendered line, which is how the omission survived —
  `TestASameClusterTargetIsWarnedAbout` asserts the line.
- **`openTarget` hands the target the source's cluster identity** before
  `Gate`, from `Source.ClusterID`. It is rule 1's second disjunct for a role
  that cannot execute `pg_control_system`, which is the role §9 recommends; see
  `internal/pg/CLAUDE.md`. A source that will not answer is not a refusal here —
  the gate decides what an unknown identity means, and it fails closed for a
  target carrying the source's own database name.
- **A recovered panic names the value's type and not the value**
  (`PanicSummary`, THREAT_MODEL.md T4). `panicError.Error` formatted the
  recovered value with `%v`, and so did `cmd/lazyslice`'s `reportPanic`, so a
  masker — or pgx's encoding, the phonenumbers parser, a JSON walker under one —
  that panicked with the offending input in its message wrote a production value
  to stderr **at any verbosity**. T4's stated control ("`event.Event` has no
  free-form string field") is about events and does not reach the error egress,
  which is a free-form string by construction. The value is printed only under
  `--show-row-values-in-errors`, through `panicError.PanicValue`, and **not**
  under `--debug`: a stack frame carries no row value, so the two flags keep
  answering their own questions. `mask.Apply` should recover on its own side
  too — **T-0181**, `mask/` being its own module.