---
id: T-0080
title: "Anchor /lazyslice in .gitignore so a bare go build cannot commit a 32MB binary"
epic: E9
phase: ""
status: open
owner: ""
created: 2026-09-08
started: ""
closed: ""
outcome: ""
---

# T-0080 · Anchor /lazyslice in .gitignore so a bare go build cannot commit a 32MB binary

## Goal

make build sends its -o to bin/, which .gitignore covers, but a bare 'go build ./cmd/lazyslice' drops a ~32MB Mach-O at the repo root as ./lazyslice. The build section of .gitignore lists /bin/, /dist/, coverage.out, *.test, /docgen and /tools/docgen/docgen, and none of them match it, so git status reports it as an untracked file and .claude/workflows' implement.js commit step (git add -A) would sweep it into history permanently, where removing it needs a history rewrite rather than a follow-up commit. T-PIN's review caught one such binary and deleted it, but nothing stops the next one. Add '/lazyslice' to the build section of .gitignore, beside /bin/ and /dist/. The leading slash is load-bearing: an unanchored 'lazyslice' line would also ignore the tracked source directory cmd/lazyslice/. .gitignore was outside T-PIN's paths.

## Acceptance

'.gitignore' has a '/lazyslice' line in its build section; 'go build ./cmd/lazyslice' at the repo root followed by 'git status --porcelain' reports no untracked lazyslice; 'git ls-files cmd/lazyslice' still lists main.go and main_test.go, proving the pattern did not swallow the source directory.

## Log

- 2026-09-08 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
