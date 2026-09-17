# lazyslice
#
# Every claim about this repository is made by one of these targets. "Should
# work" is not a status (CLAUDE.md): run `make check` and paste the output.
#
# The build tools are pinned here rather than in go.mod, because they are tools
# and not dependencies (ARCHITECTURE.md section 13). `make tools` installs them
# into $(TOOLDIR) at the pinned versions, and every target prefers a pinned
# binary over whatever is on PATH.

SHELL := /usr/bin/env bash
.SHELLFLAGS := -eu -o pipefail -c
.DEFAULT_GOAL := check

MODULE  := github.com/Liarea/lazyslice
BINARY  := lazyslice
BINDIR  := bin
TOOLDIR := $(CURDIR)/$(BINDIR)/tools

# Pinned build tools. A bump is a pull request that says why.
GOLANGCI_LINT_VERSION := v2.13.2
GORELEASER_VERSION    := v2.18.0
GOVULNCHECK_VERSION   := v1.7.0

GOLANGCI_LINT := $(shell command -v $(TOOLDIR)/golangci-lint 2>/dev/null || command -v golangci-lint 2>/dev/null)
GORELEASER    := $(shell command -v $(TOOLDIR)/goreleaser 2>/dev/null || command -v goreleaser 2>/dev/null)
GOVULNCHECK   := $(shell command -v $(TOOLDIR)/govulncheck 2>/dev/null || command -v govulncheck 2>/dev/null)

# The masker is a nested module (ADR-006) with its own go.mod, so every target
# that walks the tree walks both.
MODULES := . ./mask

VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  := $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE    := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -s -w \
	-X main.version=$(VERSION) \
	-X main.commit=$(COMMIT) \
	-X main.date=$(DATE)

.PHONY: all build test lint integration torture vet-tagged forbidden unsafe-flags spdx fmt check tools clean help docs docs-check vulncheck bench relnotes bench-compare

## build: compile the binary into bin/
build:
	CGO_ENABLED=0 go build -trimpath -ldflags '$(LDFLAGS)' -o $(BINDIR)/$(BINARY) ./cmd/$(BINARY)

## test: unit tests, both modules, with the race detector
test:
	@set -e; for m in $(MODULES); do \
		echo "==> go test $$m"; \
		( cd $$m && go test -race -count=1 ./... ); \
	done

## integration: tests behind the integration build tag; needs a Docker endpoint
##
## These are the tests that matter: the planner, the gate, the loader and the
## residual scan are statements about a real Postgres. They start containers
## through internal/testutil, so they are slow and they are not in `check`.
##
## GOTESTFLAGS is for a caller that needs to read the run rather than only its
## exit code: CI passes -v, because `go test` prints no `--- SKIP` or
## `--- PASS` line without it and a grep for one over the default output can
## never match. It is the whole of the knob; the command stays defined here so
## that CI and a laptop run the same one.
GOTESTFLAGS ?=

integration:
	go test -tags integration -count=1 -timeout 30m $(GOTESTFLAGS) ./...

## bench: BenchmarkExtractThroughput (internal/extract), compared against the
## recorded baseline
##
## docs/PERF.md profiled extract, transform and load together against a real
## Postgres (5,000 root rows, a 2,000,000-row child table) and recorded those
## numbers by hand, because they need Docker and take tens of seconds — the
## wrong cost for every push. This target is the number that can run on every
## push: BenchmarkExtractThroughput drives this package's own extractor —
## statement building, scanning into []any, batching by rows and by bytes —
## against an in-memory fake reader, no database and no network, so the
## number it reports is about this package's code on the runner it ran on and
## not about a container's disk that day.
##
## BENCH_BASELINE is internal/extract/testdata/bench/baseline.json and not
## top-level testdata/bench/baseline.json: testdata/ was outside T-PERF's
## authorized paths (internal/extract/, internal/load/, internal/transform/,
## docs/PERF.md, Makefile, .github/), and internal/extract/testdata/ is
## inside internal/extract/. docs/PERF.md's concerns section names this
## deviation; T-0175 (tracker) is the sibling task for nasty.sql's own size
## parameter, which has the same problem for a different reason.
##
## baseline.json is a catastrophic-regression floor (3,000,000 rows/sec),
## not a number this target is expected to come close to: two identical-code
## CI runs on GitHub-hosted ubuntu-latest measured 10,865,118 rows/sec and
## 6,741,072 rows/sec (see baseline.json's own note for the run ids), a 38%
## swing that made a tight baseline here fail on ordinary runner noise. The
## real regression gate is `bench-compare`, below, which measures against a
## base commit on this same machine instead of against a number recorded
## somewhere else on some other day. BENCH_REGRESSION_PCT (20%) is shared by
## both targets: here it is measured against the deliberately-low floor, so
## in practice only a collapse trips it.
BENCH_BASELINE := internal/extract/testdata/bench/baseline.json
BENCH_REGRESSION_PCT := 20

# define/endef and export, rather than a tools/ program: tools/ was outside
# T-PERF's authorized paths, the same reason BENCH_BASELINE above lives under
# internal/extract/testdata/ and not top-level testdata/bench/.
#
# BENCH_PARSE_PY holds best_rate(), the "go test -bench output -> sorted
# rows/sec numbers" parsing both BENCH_CHECK_PY (bench) and BENCH_COMPARE_PY
# (bench-compare) need; each exec()s it from the BENCH_PARSE_PY environment
# variable rather than repeating the regex, so there is one parser for one
# bench.out line format.
define BENCH_PARSE_PY
import re

def best_rate(path):
    rates = []
    with open(path) as f:
        for line in f:
            if not line.startswith("BenchmarkExtractThroughput"):
                continue
            m = re.search(r"([0-9.]+)\s+rows/sec", line)
            if m:
                rates.append(float(m.group(1)))
    return rates
endef
export BENCH_PARSE_PY

define BENCH_CHECK_PY
import json, os, sys

exec(os.environ["BENCH_PARSE_PY"])

bench_out, baseline_path, ceiling_pct = sys.argv[1], sys.argv[2], float(sys.argv[3])

rates = best_rate(bench_out)
if not rates:
    print(f"bench: no 'rows/sec' line from BenchmarkExtractThroughput in {bench_out}")
    sys.exit(1)

with open(baseline_path) as f:
    baseline = json.load(f)["BenchmarkExtractThroughput"]["rows_per_sec"]

best = max(rates)
pct = (best - baseline) / baseline * 100
print(f"bench: best of {len(rates)} run(s) = {best:,.0f} rows/sec, "
      f"baseline ({baseline_path}) = {baseline:,.0f} rows/sec ({pct:+.1f}%)")
if pct < -ceiling_pct:
    print(f"bench: throughput dropped more than {ceiling_pct:.0f}% from the recorded (floor) baseline")
    sys.exit(1)
endef
export BENCH_CHECK_PY

bench:
	@echo "==> bench: BenchmarkExtractThroughput, best of 5 one-second runs, catastrophic-floor check"
	@mkdir -p $(BINDIR) && go test -run '^$$' -bench BenchmarkExtractThroughput -benchtime=1s -count=5 ./internal/extract/... | tee $(BINDIR)/bench.out
	@python3 -c "$$BENCH_CHECK_PY" $(BINDIR)/bench.out $(BENCH_BASELINE) $(BENCH_REGRESSION_PCT)

## bench-compare: BenchmarkExtractThroughput, BENCH_BASE vs the working tree,
## measured back-to-back on this machine in this invocation
##
## The regression gate `bench` cannot be: an absolute floor compared against
## a number recorded on some other run, on some other day, on a shared
## runner. ae51123 recorded one such number from the CI job's own runner and
## made the job blocking; the very next run, with identical code, measured
## 6,741,072 rows/sec against that run's 10,865,118 (CI runs 34928141791 and
## 34927703066) — a 38% swing on the same commit, comfortably past
## BENCH_REGRESSION_PCT in both directions. No single recorded baseline
## survives that; the question has to be answered on the machine that is
## asking it.
##
## So this target benchmarks BENCH_BASE (default HEAD~1: "did this commit
## regress it") and the working tree on the *same* runner, in the *same*
## make invocation, and compares their own best-of-five to each other rather
## than either to a stored number. BENCH_BASE is checked out into a
## temporary `git worktree` under a scratch directory (mktemp -d) rather than
## a second clone or a `git stash`/checkout-and-back dance in this tree,
## because the working tree must stay exactly as the caller left it —
## including uncommitted changes — for its own runs. The worktree is removed
## in a `trap ... EXIT`, so it is cleaned up whether the comparison passes,
## fails, or the script errors out early.
##
## The five runs per side are interleaved (base, head, base, head, ...)
## rather than five-then-five: a shared runner's noisy neighbour or thermal
## throttling tends to drift over a run's lifetime rather than jump, and
## five-then-five would let that drift land entirely on whichever side ran
## second. Interleaving means every "run i" pairs a base sample and a head
## sample from the same moment on the runner, so drift affects both sides
## equally instead of biasing the comparison. Best-of-five is kept per side
## for the same reason `bench` takes a best-of-five: a single run is noisy,
## and the code should not fail a comparison because one sample was slow —
## regressing should.
##
## BENCH_COMPARE_PY reuses BENCH_PARSE_PY's best_rate(), the same
## bench.out-line parser `bench`'s BENCH_CHECK_PY uses, so the two targets
## share one parser rather than each carrying its own copy of the regex.
BENCH_BASE ?= HEAD~1

define BENCH_COMPARE_PY
import os, sys

exec(os.environ["BENCH_PARSE_PY"])

base_out, head_out, ceiling_pct = sys.argv[1], sys.argv[2], float(sys.argv[3])

base_rates = best_rate(base_out)
if not base_rates:
    print(f"bench-compare: no 'rows/sec' line from BenchmarkExtractThroughput in {base_out}")
    sys.exit(1)
head_rates = best_rate(head_out)
if not head_rates:
    print(f"bench-compare: no 'rows/sec' line from BenchmarkExtractThroughput in {head_out}")
    sys.exit(1)

base_best = max(base_rates)
head_best = max(head_rates)
pct = (head_best - base_best) / base_best * 100
print(f"bench-compare: base best of {len(base_rates)} run(s) = {base_best:,.0f} rows/sec, "
      f"head best of {len(head_rates)} run(s) = {head_best:,.0f} rows/sec ({pct:+.1f}%)")
if pct < -ceiling_pct:
    print(f"bench-compare: head throughput dropped more than {ceiling_pct:.0f}% from base")
    sys.exit(1)
endef
export BENCH_COMPARE_PY

bench-compare:
	@set -eu; \
	scratch=$$(mktemp -d); \
	worktree="$$scratch/base"; \
	base_out="$$scratch/base.out"; \
	head_out="$$scratch/head.out"; \
	: >"$$base_out"; : >"$$head_out"; \
	cleanup() { \
		git worktree remove --force "$$worktree" >/dev/null 2>&1 || true; \
		git worktree prune >/dev/null 2>&1 || true; \
		rm -rf "$$scratch"; \
	}; \
	trap cleanup EXIT; \
	base_sha=$$(git rev-parse "$(BENCH_BASE)"); \
	echo "==> bench-compare: BENCH_BASE=$(BENCH_BASE) ($$base_sha) vs the working tree, interleaved, best of 5 one-second runs each"; \
	git worktree add --quiet --detach "$$worktree" "$$base_sha"; \
	for i in 1 2 3 4 5; do \
		echo "-- run $$i/5 (base $$base_sha)"; \
		( cd "$$worktree" && go test -run '^$$' -bench BenchmarkExtractThroughput -benchtime=1s -count=1 ./internal/extract/... ) | tee -a "$$base_out"; \
		echo "-- run $$i/5 (head, working tree)"; \
		go test -run '^$$' -bench BenchmarkExtractThroughput -benchtime=1s -count=1 ./internal/extract/... | tee -a "$$head_out"; \
	done; \
	python3 -c "$$BENCH_COMPARE_PY" "$$base_out" "$$head_out" $(BENCH_REGRESSION_PCT)

## torture: the ten real schemas of testdata/torture/, plus testdata/regressions/
##
## Phase 5's gate: ten open-source schemas nobody wrote with us in mind, each
## loaded into a container with generated rows, sliced from its most-connected
## table into a second container, and put through the invariants. Nine snapshot
## cleanly; the tenth (mastodon) refuses at exit 13 for a reason ARCHITECTURE.md
## §11.1 states, and the suite asserts that refusal as tightly as it asserts the
## nine successes. docs/TORTURE.md is the table: schema, tables, time, and every
## defect these fixtures found.
##
## It is a target of its own rather than part of `integration` because it is a
## different unit of work — twenty containers and about 2.5 GB of images, one of
## them pgvector, against `integration`'s twelve — and because a change to the
## planner wants the six invariants back in seconds, not in twenty minutes. The
## build tag is the mechanism: every file behind `integration && torture` is
## invisible to `make integration`, and `internal/invariants` is one package
## either way, so the torture suite runs the *same* I1, I4 and I6 helpers rather
## than a second implementation of them.
##
## The timeout is 60m, not `integration`'s 30m: discourse alone is 370 tables and
## the four largest schemas each pull an image before they start.
##
## **This target is a manual gate for a pull request.** It wants twenty
## containers, about 2.5 GB of images and up to an hour, against a pull
## request's minutes, so no `pull_request` job runs it. What CI does run over
## these files on every pull request is `vet-tagged` (below), which
## type-checks and vets them under the same two build tags, so a compile
## error or a vet finding in the suite fails `make check` rather than waiting
## for somebody to run this by hand. Since T-0140, ci.yml's `torture` job runs
## this target for real on every push to `main` — rare enough, and important
## enough (docs/RUNBOOK.md gates a release tag on it), to carry the cost a
## pull request should not. The behaviour of the ten schemas is asserted here
## and recorded in docs/TORTURE.md; the reductions in `testdata/regressions/`
## are what stop a defect coming back, and `internal/classify`'s and
## `internal/plan`'s own unit tests are what run on every change.
##
## `-run 'TestTorture'` needs the same guard `unsafe-flags` explains at length:
## `go test -run <pattern>` exits 0 and prints plain `ok` when the pattern matches
## zero tests, so a rename of these four functions would make this target go
## silently vacuous. It runs with `-v` and greps the output for each one's own
## `--- PASS:` line, which is why the output is teed rather than buffered — an
## hour of silence is not a run anybody would trust.
TORTURE_TESTS := TestTortureSchemas TestTortureRegressions TestTortureCatalogueMatchesTheFixtures TestTortureImagesAreReachable TestTortureNegativeControl

torture:
	@log=$$(mktemp); \
	trap 'rm -f "$$log"' EXIT; \
	go test -tags 'integration torture' -count=1 -timeout 60m -v $(GOTESTFLAGS) -run 'TestTorture' ./internal/invariants/... | tee "$$log"; \
	ok=1; \
	for name in $(TORTURE_TESTS); do \
		grep -q -- "--- PASS: $$name " "$$log" || { echo "torture: no '--- PASS: $$name' line"; ok=0; }; \
	done; \
	if [ "$$ok" != 1 ]; then \
		echo; \
		echo "torture: a go test -run that matches zero tests exits 0, so this target checks for the"; \
		echo "explicit --- PASS lines of every TestTorture* function rather than trusting the exit code."; \
		exit 1; \
	fi
	@echo "==> torture: the ten schemas, the catalogue and testdata/regressions/ all reported --- PASS"

## vet-tagged: type-check and vet the code behind the `integration` and
## `integration && torture` build tags
##
## `go build` and `go test` see neither: every file in `internal/invariants` is
## behind a tag, and the torture suite is behind two. So `make check` — the
## release gate, and what ci.yml runs — compiled and vetted none of it, and a
## compile error in an 800-line test suite would ship. This runs `go vet`, which
## type-checks the package including its test files, once per tag set. It needs
## no Docker and no database: nothing here is executed.
##
## .golangci.yml's `run.build-tags` lists `integration` alone, so `make lint`
## still does not lint the torture files; that file was outside T-TORTURE's paths
## and **T-0105** is the task that adds the tag there.
vet-tagged:
	go vet -tags integration ./...
	go vet -tags 'integration torture' ./internal/invariants/...
	@echo "==> vet-tagged: the integration and torture build tags compile and vet clean"

## forbidden: fail on any name that would turn masking off
##
## CLAUDE.md's hardest rule is that no flag disables masking wholesale, and
## ARCHITECTURE.md section 8 lists the spellings. TestForbiddenFlagsDoNotExist
## is the second layer and it is strictly narrower: it walks the registered
## cobra flags of one command tree, so it cannot see an environment variable, a
## lazyslice.yml key, a struct field or a flag registered somewhere else. This
## grep reads every non-test Go file in both modules instead. The remaining
## forbidden names in section 8 (--replace, --rules) are ordinary words in
## prose, so they stay with the exact-match test rather than becoming a grep
## that cries wolf.
FORBIDDEN := no-?_?mask|nomask|disable[-_]?mask|skip[-_]?mask|unsafe|allow-nonempty-target|allow-ctid

forbidden:
	@if grep -rnEi --include='*.go' --exclude='*_test.go' '$(FORBIDDEN)' cmd internal mask tools; then \
		echo; \
		echo "forbidden: the lines above match /$(FORBIDDEN)/i."; \
		echo "Masking is not configurable away (CLAUDE.md, ARCHITECTURE.md section 8)."; \
		exit 1; \
	fi
	@echo "==> forbidden: no name that turns masking off"

## unsafe-flags: fail if any registered flag could turn masking off
##
## Distinct from `forbidden`, which greps source identifiers for a name that
## could turn masking off before it is ever wired to a flag: this walks the
## real, registered `pflag.FlagSet`s of the whole command tree, recursively —
## root persistent flags, root local flags, and, at every depth, each
## subcommand's own persistent *and* local flags (cobra does not merge a
## command's PersistentFlags() into its Flags() until ParseFlags/LocalFlags
## runs, so visiting only Flags() on an unparsed tree would miss a
## subcommand's persistent flags), VisitAll so hidden flags are seen too —
## and checks the flag *names* themselves, independently of how or where each
## was registered.
##
## This used to grep cmd/lazyslice's non-test source for the flag-name string
## literal in every `*Var`/`*VarP` pflag registration
## (`.BoolVar(&x, "name", ...)` and so on). T-CI5's review found that grep's
## blind spot: it cannot see a pointer-returning registration
## (`fs.Bool("unmask-all", ...)`), a `Var(&v, "unmask-all", ...)`
## registration, or a flag-name literal that wraps onto a second source line
## — none of which is a `*Var`/`*VarP` call with the name on the same line,
## so none of which the grep's regular expression ever matched, regardless of
## the flag's name. `cmd/lazyslice/main_test.go`'s
## TestForbiddenFlagsDoNotExist already walked the registered `pflag.FlagSet`
## with `VisitAll` for the other forbidden spellings and has no such blind
## spot: every flag reaches the binary through cobra's registration, however
## it was made, and VisitAll sees it. `checkForbiddenFlagName` there now also
## carries this rule: the exact name `unmask` is the legitimate per-column
## `--unmask TABLE.COL=REASON` opt-out (ARCHITECTURE.md section 8) and must
## pass, while any other flag name that merely *contains* "unmask" —
## `--unmask-all`, `--global-unmask` — would widen that opt-out past the
## reason-per-column requirement and must fail.
##
## `TestForbiddenFlagsDoNotExist_SelfTestUnmaskAll` is the negative
## self-test this recipe used to run itself, moved into the same test binary:
## it registers a fake `--unmask-all` flag on a scratch `pflag.FlagSet` and
## asserts `checkForbiddenFlagName` actually rejects it, so the rail is
## proven to fail on the case it exists for rather than merely asserted to
## (T-CI5 review). `-run TestForbiddenFlagsDoNotExist` picks up both tests,
## because `go test -run` is an unanchored regexp match against the test name
## and the self-test's name carries this one as a prefix.
## `go test -run <pattern>` exits 0 and prints plain `ok` when the pattern
## matches zero tests (verified: a typo'd -run prints "ok ... [no tests to
## run]" and still exits 0), so a bare `go test -run` here would go silently
## vacuous the moment either test is renamed or deleted and stop asserting
## anything. Passing -v and grepping the output for both tests' own `---
## PASS:` lines means a missing test fails this target instead of passing it
## by accident.
unsafe-flags:
	@out="$$(go test ./cmd/lazyslice -run 'TestForbiddenFlagsDoNotExist' -count=1 -v)"; \
	echo "$$out"; \
	ok=1; \
	echo "$$out" | grep -q -- '--- PASS: TestForbiddenFlagsDoNotExist ' || ok=0; \
	echo "$$out" | grep -q -- '--- PASS: TestForbiddenFlagsDoNotExist_SelfTestUnmaskAll ' || ok=0; \
	if [ "$$ok" != 1 ]; then \
		echo; \
		echo "unsafe-flags: TestForbiddenFlagsDoNotExist and/or its self-test did not report --- PASS above."; \
		echo "A go test -run that matches zero tests exits 0, so this target checks for the"; \
		echo "explicit --- PASS lines rather than trusting the exit code alone."; \
		exit 1; \
	fi
	@echo "==> unsafe-flags: no registered flag in cmd/lazyslice's command tree turns masking off or widens --unmask (self-test passed)"

## spdx: fail on any Go file whose first line is not the SPDX identifier
##
## research/LICENSE_DECISION.md recommendation 1: the licence is carried
## per-file by "an SPDX-License-Identifier: Apache-2.0 header in every source
## file", which is the machine-readable form a compliance scanner reads. The
## check is a first-line comparison rather than a grep of the whole file, so a
## header buried halfway down — where no scanner looks — fails.
##
## `git ls-files --cached --others --exclude-standard` rather than `find`, so
## generated output under bin/ or dist/ is not a licence question -- it is
## gitignored -- while a new file the author has not staged yet still is. A
## check that only looked at the index would pass locally and fail in CI on the
## commit that adds the file.
SPDX := // SPDX-License-Identifier: Apache-2.0

spdx:
	@missing=$$(for f in $$(git ls-files --cached --others --exclude-standard '*.go'); do \
		if [ "$$(head -n 1 "$$f")" != '$(SPDX)' ]; then echo "$$f"; fi; \
	done); \
	if [ -n "$$missing" ]; then \
		echo "$$missing"; \
		echo; \
		echo "spdx: the files above do not start with '$(SPDX)'."; \
		echo "Every Go file carries the licence per-file (research/LICENSE_DECISION.md recommendation 1)."; \
		exit 1; \
	fi
	@echo "==> spdx: every Go file carries the licence identifier"

## docs: regenerate docs/FLAGS.md, docs/KEYBINDINGS.md, docs/ERRORS.md and
## README.md's first-run flag table
##
## tools/docgen reads the registered cobra flag set of cmd/lazyslice (via
## `go run ./cmd/lazyslice --help`, grouped as --help groups it), internal/tui's
## Bindings() table, and internal/event's Catalogue() — never hand-edit the
## three files this writes into docs/ (docs/CLAUDE.md), or the table it
## writes into README.md between the `<!-- docgen:flags:start -->` and
## `<!-- docgen:flags:end -->` markers.
docs:
	go run ./tools/docgen -out docs -readme README.md
	@echo "==> docs: wrote docs/FLAGS.md, docs/KEYBINDINGS.md, docs/ERRORS.md and README.md's flag table"

## docs-check: fail when the committed docs differ from a fresh `make docs`
##
## Regenerates into a temporary directory rather than overwriting the tree, so
## a clean checkout stays clean whether this passes or fails, and prints a
## unified diff plus the exact regeneration command on drift — the developer
## who hits this job is a contributor on their first run, and
## docs/adr/008-first-run.md §8 asks for lazydocker's behaviour here rather
## than lazygit's bare `git diff --quiet`. README.md's flag table is checked
## the same way: -readme points the regenerated README.md at a scratch copy
## in the same temporary directory instead of the tree's own README.md, so
## docgen still reads the committed README.md as its template but the
## comparison is a diff, never a write to the working tree.
docs-check:
	@tmp=$$(mktemp -d); \
	trap 'rm -rf "$$tmp"' EXIT; \
	if ! go run ./tools/docgen -out "$$tmp" -readme "$$tmp/README.md" >/dev/null; then \
		echo "docs-check: tools/docgen failed; fix the generator or README.md's docgen markers (this is not documentation drift, and 'make docs' will not cure it)"; \
		exit 1; \
	fi; \
	status=0; \
	for f in FLAGS.md KEYBINDINGS.md ERRORS.md; do \
		if ! diff -u "docs/$$f" "$$tmp/$$f" >/dev/null 2>&1; then \
			echo "docs-check: docs/$$f is out of date:"; \
			diff -u "docs/$$f" "$$tmp/$$f" || true; \
			echo; \
			status=1; \
		fi; \
	done; \
	if ! diff -u "README.md" "$$tmp/README.md" >/dev/null 2>&1; then \
		echo "docs-check: README.md's flag table (between the docgen:flags markers) is out of date:"; \
		diff -u "README.md" "$$tmp/README.md" || true; \
		echo; \
		status=1; \
	fi; \
	if [ "$$status" -ne 0 ]; then \
		echo "docs-check: docs/FLAGS.md, docs/KEYBINDINGS.md, docs/ERRORS.md and README.md's flag table must match tools/docgen."; \
		echo "docs-check: run 'make docs' and commit the result."; \
		exit 1; \
	fi; \
	echo "==> docs-check: docs/FLAGS.md, docs/KEYBINDINGS.md, docs/ERRORS.md and README.md's flag table match tools/docgen"

## vulncheck: govulncheck over both modules (THREAT_MODEL.md T10)
vulncheck:
	@if [ -z "$(GOVULNCHECK)" ]; then \
		echo "govulncheck not found; run: make tools"; exit 1; \
	fi
	@set -e; for m in $(MODULES); do \
		echo "==> vulncheck $$m"; \
		( cd $$m && $(GOVULNCHECK) ./... ); \
	done

## lint: the SPDX header check, then golangci-lint over both modules
lint: spdx
	@if [ -z "$(GOLANGCI_LINT)" ]; then \
		echo "golangci-lint not found; run: make tools"; exit 1; \
	fi
	@set -e; for m in $(MODULES); do \
		echo "==> lint $$m"; \
		( cd $$m && $(GOLANGCI_LINT) run --config $(CURDIR)/.golangci.yml ); \
	done

## fmt: gofmt -w over both modules, and tidy go.mod
fmt:
	gofmt -w -s $$(git ls-files '*.go')
	@set -e; for m in $(MODULES); do ( cd $$m && go mod tidy ); done

## check: lint, the forbidden-name grep, the unsafe-flags check, the docs-drift
## check, then test. release.yml runs this target — and only this target —
## before a tag publishes, so anything CLAUDE.md's hardest rule depends on has
## to be a prerequisite here, not only a ci.yml job: a tag is not required to
## point at a commit ci.yml ever ran. vulncheck stays out on purpose, because
## it is a network call and this target is also the local default; run it
## separately (`make vulncheck`) or add it to release.yml if the release path
## should block on it too.
check: lint forbidden unsafe-flags docs-check vet-tagged test

## tools: install the pinned build tools into bin/tools
tools:
	GOBIN=$(TOOLDIR) go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
	GOBIN=$(TOOLDIR) go install github.com/goreleaser/goreleaser/v2@$(GORELEASER_VERSION)
	GOBIN=$(TOOLDIR) go install golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION)

## snapshot: build the release artifacts locally without publishing or signing
##
## --skip=sign because the cosign signature is keyless and its identity is the
## release workflow's OIDC token (THREAT_MODEL.md T10): there is nothing for a
## laptop to sign with, and a local build should not need cosign installed.
##
## SNAPSHOT_SKIP defaults to also skipping sbom, one level down, for the same
## reason: the sboms pipe shells out to syft, and a laptop should not need it
## installed just to prove the config is valid. ci.yml's release-config job
## installs syft and overrides this to `sign` alone, so that job — the one
## place a config error must be caught before a tag, not at one
## (.goreleaser.yaml's own comment) — actually runs the sboms pipe rather than
## only validating its schema.
SNAPSHOT_SKIP ?= sign,sbom

snapshot:
	@if [ -z "$(GORELEASER)" ]; then \
		echo "goreleaser not found; run: make tools"; exit 1; \
	fi
	$(GORELEASER) release --snapshot --clean --skip=$(SNAPSHOT_SKIP)

## clean: remove build output
clean:
	rm -rf $(BINDIR) dist coverage.out

## help: list the targets
help:
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/^## //'

## relnotes: release notes from the commit bodies between two refs (FROM exclusive, TO
## inclusive), grouped by the headline's stage prefix. Every task commit carries
## bullets written for this (docs/OPERATING_MODEL.md "Commit discipline"), so the
## release workflow hands goreleaser this file instead of a list of subjects.
FROM ?= $(shell git describe --tags --abbrev=0 HEAD^ 2>/dev/null || git rev-list --max-parents=0 HEAD)
TO ?= HEAD
relnotes:
	@python3 tools/relnotes/relnotes.py $(FROM) $(TO)
