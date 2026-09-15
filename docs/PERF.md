# Performance: the extract/transform/load pipeline (T-PERF)

This is T-PERF's record: where extract, transform and load spend time and
memory on a 5,000-root-row / 2,000,000-child-row snapshot, what
`docs/reviews/2026-09-09/REVIEW.md` finding 9 cost before it was fixed, the
two hotspots profiling found that this task's paths (`internal/extract/`,
`internal/load/`, `internal/transform/`) could actually fix, and the
commands to reproduce every number here. It does not attempt the
20,000,000-row run — that is T-0157, a 0.2 item, on purpose.

**Target: under 60 seconds against local containers.** Measured: the
extract → transform → load pipeline over 2,005,000 rows took about 12
seconds of stage time (below), inside a 26-second `go test` run that also
pays for starting two Postgres containers and seeding 2,000,000 rows with a
set-based `INSERT`. Comfortably inside budget.

## Environment

- MacBook, Apple M4, darwin/arm64, Go 1.27.1.
- Docker Desktop, `postgres:16` containers via `testcontainers-go`
  (`internal/testutil.Postgres`) — one for the source, one for the target.
- No other load on the machine during measurement. Every number below is
  from a real run on this machine on 2026-09-14; none is estimated.

## The scenario, and why it is not `testdata/nasty.sql`

The task asked for "5,000 root rows from a source whose child table has
2,000,000 rows," extending `testdata/nasty.sql`'s generator with a size
parameter. `testdata/nasty.sql` and `internal/testutil` (which loads it) are
**not** under this task's authorized paths (`internal/extract/`,
`internal/load/`, `internal/transform/`, `docs/PERF.md`, `Makefile`,
`.github/`), so neither was touched. `fill_stream_rows(n bigint)` and
`fill_stream_docs(n bigint)` already take `n` as a SQL parameter — only the
`psql -v big=1` gate and `testutil.LoadNasty`'s `big bool` hardcode
2,000,000 and 1,000,000 — but extending the gate itself needed a write
outside this task's paths. **Filed as tracker T-0175** rather than done
here or worked around by widening the paths.

What actually produced every number below is
`internal/extract/perf_profile_test.go` (`//go:build integration`, skipped
unless `LAZYSLICE_PERF=1` — `make integration` does not run it): it seeds
its own throwaway schema —

```sql
perf_root(id bigint primary key, name text)                          -- 5,000 rows
perf_child(id bigint primary key, root_id bigint references perf_root,
           email text, body text)                                    -- 2,000,000 rows, 400/root
perf_wide(id bigint primary key, payload text)                       -- 2,000 rows, 1 MiB payload each
perf_wide_json(id bigint primary key, payload jsonb)                 -- 2,000 rows, ~1 MiB payload each
```

with two set-based `INSERT ... SELECT ... FROM generate_series(...)`
statements, then runs the real introspector, planner, extractor,
transformer and loader over it through `internal/pg`, exactly as
`internal/extract` and `internal/load`'s own integration suites already do
against `testdata/nasty.sql` and `testdata/pagila` — just against a schema
sized for this question instead. `perf_root` is the plan's `Root`, `Take:
5000`.

## Reproducing these numbers

```
# Stage timings and the CPU profile (the flame summary below):
LAZYSLICE_PERF=1 LAZYSLICE_PERF_CPUPROFILE=/tmp/lazyslice_cpu.prof \
  go test -tags integration -run TestPerfProfile -v -timeout 20m ./internal/extract/...
go tool pprof -top -nodecount=25 /tmp/lazyslice_cpu.prof

# Wide-row batch count and peak RSS (finding 9), text and jsonb columns:
go test -tags integration -c -o /tmp/extract.test ./internal/extract/...
LAZYSLICE_PERF=1 /usr/bin/time -l /tmp/extract.test \
  -test.run 'TestPerfWideRowRSS$' -test.v -test.timeout 10m
LAZYSLICE_PERF=1 /usr/bin/time -l /tmp/extract.test \
  -test.run 'TestPerfWideRowRSSJSON$' -test.v -test.timeout 10m
# (Linux: /usr/bin/time -v, and read "Maximum resident set size (kbytes)")

# The CI throughput gate, reproduced locally:
make bench
```

## Stage timings: 5,000 root rows, 2,000,000 child rows

`streamRows` in `perf_profile_test.go` wires `extract.New(...).Extract` →
`transform.New(...).Transform` (masking `perf_child.email`) →
`load.New(...).Load` on three goroutines over channels, and times each
stage with its own `time.Now()`/`time.Since()` around its own work — the
three run concurrently on the channel, so "stage sum" below is not the
wall clock of the whole pipeline (12.2s), it is each stage's own busy time.

| stage | before (this task's start) | after (both fixes below) |
|---|---:|---:|
| extract | 3.94 s | 3.79 s |
| transform | 3.97 s | 3.82 s |
| load | 4.73 s | 4.58 s |
| stage sum | 12.63 s | 12.19 s |
| rows | 2,005,000 | 2,005,000 |

"Before" here already carries the byte-cap batching fix (below) — it is
isolating the bloom-filter fix's effect, not a from-scratch baseline; see
"The two hotspots this task optimised" and "The byte-cap batching fix"
below for each fix measured on its own against a clean before/after.

## Flame summary: where the CPU actually goes

`go tool pprof -top -nodecount=25` over the whole three-stage run (the
profile spans all three goroutines: extract's `Query`/`Scan`, transform's
masking, load's `CopyFrom`):

```
      flat  flat%   sum%        cum   cum%
     2.13s 35.50% 35.50%      2.14s 35.67%  runtime.pthread_cond_signal
     0.37s  6.17% 41.67%      0.37s  6.17%  syscall.rawsyscalln
     0.30s  5.00% 46.67%      0.30s  5.00%  runtime.pthread_cond_wait
     0.28s  4.67% 51.33%      0.28s  4.67%  runtime.usleep
     0.27s  4.50% 55.83%      0.27s  4.50%  crypto/internal/fips140/sha256.blockSHA2
     0.20s  3.33% 59.17%      0.20s  3.33%  runtime.kevent
     0.13s  2.17% 61.33%      0.57s  9.50%  crypto/internal/fips140/hmac.New[...]
     0.09s  1.50% 68.50%      0.34s  5.67%  internal/transform.(*bloom).Add
     0.05s  0.83% 73.33%      1.90s 31.67%  internal/transform.transformer.maskScalar
```

Reading it stage by stage:

- **Extract and load are I/O-bound, not CPU-bound.** Neither stage's own
  code appears above ~1% of samples; `extractor.read`'s CPU (7.6% cum) is
  almost entirely inside `pgx`'s `Rows.Next`/`Scan` waiting on the network,
  and `loader.copy`'s CPU (0.8% cum) likewise waits on `CopyFrom`. Over half
  the whole profile — `pthread_cond_signal`, `pthread_cond_wait`,
  `usleep`, `kevent`, `syscall.rawsyscalln` — is Go's runtime scheduler and
  netpoller doing exactly what they are for: coordinating three goroutines
  across two Postgres connections. There is no algorithmic hotspot to fix
  in either stage's own code at this row shape; the ceiling is round-trip
  latency to Postgres, and both stages already avoid the two things that
  would make that worse (extract's chunked `unnest` join instead of N
  single-row reads, load's `CopyFrom` instead of row-at-a-time `INSERT`).
- **Transform is where the CPU-bound work is** (`maskScalar` cum 31.67%),
  and almost all of it is `mask.Apply` — the HMAC key schedule and
  generator dispatch in the `mask` module (ADR-006's own module,
  **not** among this task's authorized paths). That is the single largest
  hotspot in the whole pipeline and it is filed rather than fixed here:
  **tracker T-0176**.
- **The next-largest hotspot this task *could* fix** is
  `internal/transform.(*bloom).Add` → `positions` (5.67% cum): every masked
  cell calls it once, and profiling before this task's changes showed
  `hmac.New` and `mac.Sum(nil)` — a fresh HMAC-SHA256 state and a fresh
  32-byte result, allocated on **every one of 2,000,000 calls** — as
  `positions`'s whole cost. Fixed below.

### The two hotspots this task optimised

Both are in `internal/transform/bloom.go`, because that is the largest
CPU cost these authorized paths actually control — `mask.Apply` (the
larger cost, filed as T-0176) and the extract/load stages' own code (both
I/O-bound, nothing to fix) are not.

1. **`hmac.New(sha256.New, ...)` allocated a fresh HMAC-SHA256 state on
   every `Add`/`MayContain` call.** Fixed with a `sync.Pool` of reusable
   `hash.Hash` values, `Reset()` before each use — a pool rather than one
   shared instance under `bloom.mu`, because `Add` and `MayContain` run in
   different goroutines in a streaming pipeline and a single shared
   instance would have serialised them behind the mutex for the whole hash
   rather than only the word write that needs it.
2. **`[]byte(col.Table.Schema)`, `[]byte(col.Table.Name)`,
   `[]byte(col.Column)` were re-converted from the same three strings on
   every call for a column**, though they never change for that column.
   Fixed with a small `sync.Map` cache keyed by `pipeline.ColumnRef`,
   populated once per column and read back on every later call.

Isolated measurement (transform stage alone, same 2,000,000-row run,
everything else held fixed — the byte-cap batching fix present in both):

| | transform stage time |
|---|---:|
| before (`hmac.New`/`Sum(nil)` per call) | 4.19 s |
| after (pooled HMAC, cached column bytes) | 3.82 s |

About 9% faster. `mac.Sum(buf[:0])` (a stack array instead of `Sum(nil)`'s
fresh heap slice) did not show a clean win at this profile's sample
resolution — `mac` is a `hash.Hash` interface value, and the call through
it likely still escapes `buf` to the heap; the `hmac.New` allocation this
was mainly aimed at did go away (`go tool pprof -list` on `positions`
before this change shows a `120ms` flat cost on the `hmac.New(...)` line
alone; after, that line is `b.macPool.Get().(hash.Hash)` and carries no
flat cost of its own). `TestBatchesAreAlsoCutByBytes` and the rest of
`go test ./internal/transform/...` (below) hold correctness; `-race`
passes, which is what actually matters for the `sync.Pool`/`sync.Map`
change.

## The byte-cap batching fix (finding 9)

> "Extraction batches 2,000 rows regardless of byte size. Even one batch of
> 2,000 one-MiB values can require roughly two GiB before transformation
> overhead." — `docs/reviews/2026-09-09/REVIEW.md`, finding 9

`internal/extract`'s batcher now flushes a batch at whichever of
`batchRows` (2,000, unchanged) or `batchBytes` (8 MiB, new) it reaches
first (`sql.go`, `extract.go`'s `rowEstimate`). `rowEstimate` is a cheap
lower bound — a `string`/`[]byte` value counts its own length; a jsonb,
json or array column (which pgx decodes into `map[string]any`/`[]any`/a
typed slice when scanned into `*any`, per this package's CLAUDE.md) is
walked recursively up to a bounded depth so its string content counts too;
everything else a small fixed constant — not a claim about a batch's true
heap footprint, only enough to stop 2,000 one-MiB values from ever forming
one batch again, on a jsonb or array column exactly as much as on a `text`
one. Recursing into the decoded shape rather than counting it as the fixed
constant was a fix during T-PERF review: the first cut of `rowEstimate`
counted every non-string/[]byte value, jsonb and arrays included, at 16
bytes regardless of size, so a 1 MiB jsonb document still formed an
effectively-unbounded batch.

Measured on exactly finding 9's own example — `perf_wide`, 2,000 rows of
one ~1 MiB text value each, extracted through a properly-sized 8-slot
channel (`ARCHITECTURE.md` §1's "2,000 rows × 8") so a streaming consumer
is what is being measured, not an unread channel buffer — and again on
`perf_wide_json`, the same 2,000 rows but each value a jsonb array of 1,024
1 KiB strings (~1 MiB of string content, decoded to `[]any` of `string`):

| | batches | largest batch | peak RSS | peak footprint |
|---|---:|---:|---:|---:|
| text, before | 2 | 2,000 rows (~2 GiB) | 1,757,495,296 B (1.64 GiB) | 2,333,804,728 B (2.17 GiB) |
| text, after | 251 | 8 rows (~8 MiB) | 480,100,352 B (458 MiB) | 467,305,336 B (446 MiB) |
| jsonb, after (recursive `rowEstimate`) | 251 | 8 rows (~8 MiB) | 504,643,584 B (481 MiB) | 491,963,232 B (469 MiB) |

The jsonb column cuts into the same 251 batches of at most 8 rows as the
text column — the fix that mattered for finding 9 was extending
`rowEstimate` to walk the decoded shape at all; before that fix, this
jsonb column batched as if 16 bytes wide regardless of its ~1 MiB content,
the same unbounded-batch failure mode finding 9 named for `text`, just
unfixed by the first cut of the fix. About 3.7× lower peak RSS on `text`
than before the byte cap existed at all, and the batch itself is now
bounded by `batchBytes` regardless of row width or column type — the
property finding 9 asked for. `TestBatchesAreAlsoCutByBytes`
(`internal/extract/extract_test.go`) is the unit-level guard, run on every
`make test`/`make check`; the table above
is what running it against a real 2,000-row, ~2 GiB table actually costs,
which no unit test can measure.

## `make bench` and the CI regression gate

`BenchmarkExtractThroughput` (`internal/extract/throughput_bench_test.go`)
drives the real extractor — statement building, `Rows.Scan` into `[]any`,
batching by rows and by bytes — against an in-memory fake `Reader`, no
database and no network, so it runs in about two seconds and needs no
Docker. It is not a substitute for the real-Postgres numbers above (a fake
reader cannot measure network-bound extract/load, which is most of this
pipeline's wall time); it is the number cheap enough to run on every push.

`make bench` runs it (best of five one-second `-count=5` runs, to absorb
ordinary noise) and compares the best run against the recorded baseline in
`internal/extract/testdata/bench/baseline.json`, failing if throughput
dropped more than 20% (`BENCH_REGRESSION_PCT` in the `Makefile`).
`.github/workflows/ci.yml`'s `bench` job runs it on every push and pull
request, no Docker service required.

Measured on this machine: ~22,000,000 rows/sec, stable across five runs
(21.5M–22.2M). The recorded baseline is deliberately lower — see Concerns.

```
$ make bench
==> bench: BenchmarkExtractThroughput, best of 5 one-second runs
BenchmarkExtractThroughput-10   130   9037463 ns/op   22130103 rows/sec
...
bench: best of 5 run(s) = 22,139,957 rows/sec, baseline (internal/extract/testdata/bench/baseline.json) = 15,000,000 rows/sec (+47.6%)
==> bench: within 20% of the recorded baseline
```

## Checks

```
$ go test -race -count=1 ./internal/extract/... ./internal/transform/... ./internal/load/...
ok  	github.com/Liarea/lazyslice/internal/extract	1.3s
ok  	github.com/Liarea/lazyslice/internal/transform	3.6s
ok  	github.com/Liarea/lazyslice/internal/load	(no test files run outside -tags integration)
```

`make check`'s full output is in this task's return value.

## Concerns

- **`testdata/nasty.sql` was not extended with a size parameter**, as this
  task's brief asked, because `testdata/` and `internal/testutil` are
  outside this task's authorized paths. `internal/extract/perf_profile_test.go`
  seeds its own schema instead (above). **Filed as T-0175.**
- **`internal/extract/testdata/bench/baseline.json`, not top-level
  `testdata/bench/baseline.json`.** The task named the top-level path;
  `testdata/` is outside this task's authorized paths, and
  `internal/extract/testdata/` is inside `internal/extract/`. Whoever picks
  up T-0175 should also move this file (or make `internal/extract/`'s copy
  the one the CI job reads from a shared location) so both fixtures live in
  one place.
- **The recorded baseline (15,000,000 rows/sec) is well below the ~22,000,000
  rows/sec measured locally**, on purpose: `BenchmarkExtractThroughput` has
  never run on the actual CI runner, whose hardware is unknown to this task,
  and a baseline that only just clears a laptop's number would fail the
  very first real CI run on ordinary machine-to-machine variance. But a
  baseline this far under the laptop's own number does not by itself say
  the CI runner will clear it: `ubuntu-latest` is a shared x86 GitHub-hosted
  runner, plausibly 2-3x slower single-thread than the Apple M4 this
  benchmark was recorded on, which could put the runner's real throughput
  under even this deliberately-lowered floor. T-PERF review flagged that a
  never-measured baseline should not gate every push, so
  `.github/workflows/ci.yml`'s `bench` job now runs with
  `continue-on-error: true` until **T-0177** (tracker) pushes a branch,
  reads the job's own best-of-5 rows/sec from a real `ubuntu-latest` run,
  replaces `baseline.json` with that number, and removes the
  `continue-on-error` line so the job gates for real.
- **The top CPU hotspot in the whole pipeline (`mask.Apply`'s HMAC
  derivation, ~26–33% of samples) is in the `mask` module**, a separate Go
  module (ADR-006) outside this task's authorized paths, so it was profiled
  and reported, not fixed. **Filed as T-0176.**
- **Extract and load showed no CPU hotspot to fix at this row shape** —
  both are network-round-trip-bound against the containers on this
  machine, not CPU-bound (Flame summary, above). A wider or deeper subset,
  a slower network path to a real production source, or a target with many
  more indexes could change this; nothing in this run's profile suggested
  either stage's own code was the constraint.
- **Stage times (12.2s total) are on one laptop against local containers**,
  as the task asked; they say nothing about a production source over a
  real network, which is the case THREAT_MODEL.md T9's snapshot-hold
  estimate exists for and which this task did not attempt to reproduce.
- **T-PERF review found a data race in `perf_profile_test.go`'s stage-timing
  goroutine**: the extract goroutine sent its error on the buffered
  `extractErr` channel before assigning `extractElapsed`, so the main
  goroutine's read of `extractElapsed` after `<-extractErr` was
  unsynchronised with that write, and the printed extract-stage figure could
  read as 0. Fixed by assigning `extractElapsed` before the send (the
  transform goroutine already had the correct order). The stage timings
  table above is from a re-run after the fix; the numbers did not change.
