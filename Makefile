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

GOLANGCI_LINT := $(shell command -v $(TOOLDIR)/golangci-lint 2>/dev/null || command -v golangci-lint 2>/dev/null)
GORELEASER    := $(shell command -v $(TOOLDIR)/goreleaser 2>/dev/null || command -v goreleaser 2>/dev/null)

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

.PHONY: all build test lint integration forbidden spdx fmt check tools clean help

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

## check: lint, the forbidden-name grep, then test. This is what CI runs.
check: lint forbidden test

## tools: install the pinned build tools into bin/tools
tools:
	GOBIN=$(TOOLDIR) go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
	GOBIN=$(TOOLDIR) go install github.com/goreleaser/goreleaser/v2@$(GORELEASER_VERSION)

## snapshot: build the release artifacts locally without publishing or signing
##
## --skip=sign because the cosign signature is keyless and its identity is the
## release workflow's OIDC token (THREAT_MODEL.md T10): there is nothing for a
## laptop to sign with, and a local build should not need cosign installed.
snapshot:
	@if [ -z "$(GORELEASER)" ]; then \
		echo "goreleaser not found; run: make tools"; exit 1; \
	fi
	$(GORELEASER) release --snapshot --clean --skip=sign

## clean: remove build output
clean:
	rm -rf $(BINDIR) dist coverage.out

## help: list the targets
help:
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/^## //'
