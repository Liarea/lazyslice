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

.PHONY: all build test lint integration forbidden unsafe-flags spdx fmt check tools clean help docs docs-check vulncheck

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
## could turn masking off before it is ever wired to a flag: this greps the
## flag-name string literal out of every `*Var`/`*VarP` pflag registration in
## cmd/lazyslice's non-test source — `.BoolVar(&x, "name", ...)`,
## `.StringVar(&x, "name", ...)`, `.IntVarP(&x, "name", "short", ...)` and so
## on — and checks the flag *names*, independently of which `*pflag.FlagSet`
## receives the call: a named group's set (`bindFlags`), `root.PersistentFlags()`
## directly, or a subcommand's own `Flags()`. That includes the one thing
## `forbidden`'s pattern deliberately does not cover, because `--unmask`
## itself is legitimate. "unmask" is safe only as that exact per-column
## `--unmask TABLE.COL=REASON` opt-out (ARCHITECTURE.md section 8); a second,
## wider spelling — `--global-unmask`, `--unmask-all` — would defeat the
## reason-per-column requirement and is refused here even though
## `forbidden`'s grep would not catch it, since it names no masking
## identifier in source.
##
## This used to read *rendered* `--help` text instead. That had a blind spot:
## `groupedUsage` (cmd/lazyslice/main.go) prints only the eight named flag
## groups plus the running command's own LocalNonPersistentFlags, so a flag
## added straight to `root.PersistentFlags()` outside those groups, or to a
## subcommand's own `Flags()` in a way `groupedUsage` does not walk, never
## appeared in any `--help` text and passed regardless of its name (T-CI5
## review). Grepping the registration call sites themselves has no such
## blind spot — every flag reaches the binary through one of these calls,
## however it is grouped — and it also removes the old per-subcommand loop
## over hardcoded command names: there is no subcommand list here to go
## stale when a seventh subcommand is added, because every registration
## lives in cmd/lazyslice/main.go regardless of how many subcommands exist.
## The sound long-term fix is still to walk the real, registered flag set
## with `VisitAll` (cmd/lazyslice/main_test.go's
## TestForbiddenFlagsDoNotExist already does this for the other forbidden
## spellings); that is a cmd/lazyslice change and outside this recipe's
## authority to make.
##
## Before checking the real tree, this proves the check can actually fail:
## it copies cmd/lazyslice's non-test source into a scratch directory,
## appends a fake `--unmask-all` registration to the copy, and confirms the
## same grep catches it there. A rail that has never been seen to fail on
## the case it exists for is not proven to catch anything (T-CI5 review).
UNSAFE_FLAG_GREP := grep -rhoE '\.[A-Za-z0-9]+Var(P)?\(&[A-Za-z0-9_.]+,[[:space:]]*"[^"]+"' \
	--include='*.go' --exclude='*_test.go'

unsafe-flags:
	@scratch=$$(mktemp -d); \
	trap 'rm -rf "$$scratch"' EXIT; \
	for f in cmd/lazyslice/*.go; do case "$$f" in *_test.go) continue;; esac; cp "$$f" "$$scratch/"; done; \
	printf '\nfunc scratchNegativeSelfTest() { fs.BoolVar(&x, "unmask-all", false, "T-CI5 self-test") }\n' >> "$$scratch/main.go"; \
	self_flags=$$($(UNSAFE_FLAG_GREP) "$$scratch" 2>/dev/null | sed -E 's/.*"([^"]+)"$$/\1/' | sort -u); \
	if ! echo "$$self_flags" | grep -Eiq -- '^unmask-all$$'; then \
		echo "unsafe-flags: self-test failed -- a scratch --unmask-all registration was not caught by"; \
		echo "the flag-name grep, so this rail is not proven to fail on the case it exists for."; \
		exit 1; \
	fi; \
	flags=$$($(UNSAFE_FLAG_GREP) cmd/lazyslice 2>/dev/null | sed -E 's/.*"([^"]+)"$$/\1/' | sort -u); \
	bad=$$(echo "$$flags" | grep -Ei -- '^(no[-_]?mask|disable[-_]?mask|skip[-_]?mask)$$' || true); \
	bad="$$bad"$$'\n'"$$(echo "$$flags" | grep -Ei -- 'unmask' | grep -Eiv -- '^unmask$$' || true)"; \
	bad=$$(echo "$$bad" | sed '/^$$/d'); \
	if [ -n "$$bad" ]; then \
		echo "unsafe-flags: the registered flag set includes:"; \
		echo "$$bad"; \
		echo; \
		echo "No flag may disable masking wholesale (CLAUDE.md), and unmask is safe only as the"; \
		echo "exact per-column --unmask TABLE.COL=REASON opt-out (ARCHITECTURE.md section 8)."; \
		exit 1; \
	fi; \
	echo "==> unsafe-flags: no *Var/*VarP registration in cmd/lazyslice turns masking off or widens --unmask (self-test passed)"

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

## docs: regenerate docs/FLAGS.md, docs/KEYBINDINGS.md and docs/ERRORS.md
##
## tools/docgen reads the registered cobra flag set of cmd/lazyslice (via
## `go run ./cmd/lazyslice --help`, grouped as --help groups it), internal/tui's
## Bindings() table, and internal/event's Catalogue() — never hand-edit the
## three files this writes (docs/CLAUDE.md).
docs:
	go run ./tools/docgen -out docs
	@echo "==> docs: wrote docs/FLAGS.md, docs/KEYBINDINGS.md, docs/ERRORS.md"

## docs-check: fail when the committed docs differ from a fresh `make docs`
##
## Regenerates into a temporary directory rather than overwriting the tree, so
## a clean checkout stays clean whether this passes or fails, and prints a
## unified diff plus the exact regeneration command on drift — the developer
## who hits this job is a contributor on their first run, and
## docs/adr/008-first-run.md §8 asks for lazydocker's behaviour here rather
## than lazygit's bare `git diff --quiet`.
docs-check:
	@tmp=$$(mktemp -d); \
	trap 'rm -rf "$$tmp"' EXIT; \
	go run ./tools/docgen -out "$$tmp" >/dev/null; \
	status=0; \
	for f in FLAGS.md KEYBINDINGS.md ERRORS.md; do \
		if ! diff -u "docs/$$f" "$$tmp/$$f" >/dev/null 2>&1; then \
			echo "docs-check: docs/$$f is out of date:"; \
			diff -u "docs/$$f" "$$tmp/$$f" || true; \
			echo; \
			status=1; \
		fi; \
	done; \
	if [ "$$status" -ne 0 ]; then \
		echo "docs-check: docs/FLAGS.md, docs/KEYBINDINGS.md and docs/ERRORS.md must match tools/docgen."; \
		echo "docs-check: run 'make docs' and commit the result."; \
		exit 1; \
	fi; \
	echo "==> docs-check: docs/FLAGS.md, docs/KEYBINDINGS.md and docs/ERRORS.md match tools/docgen"

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
check: lint forbidden unsafe-flags docs-check test

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
