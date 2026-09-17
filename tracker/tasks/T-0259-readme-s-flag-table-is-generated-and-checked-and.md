---
id: T-0259
title: "README's flag table is generated and checked, and a release tag refuses a README that does not name it"
epic: E5
phase: 5
status: open
owner: sonnet
created: 2026-09-17
started: ""
closed: ""
outcome: ""
---

# T-0259 · README's flag table is generated and checked, and a release tag refuses a README that does not name it

## Goal

The maintainer, 2026-09-17: the README is very out of date. It was last touched 81 commits ago, names none of the 33 flags, and nothing fails when it rots, while docs/FLAGS.md and docs/ERRORS.md stay true because make docs-check compares them with tools/docgen. Give README.md the same footing: tools/docgen writes the first-run flag table (the flags a first run meets: --source, --target, --root, --yes, --create-target, --unmask, --skip-table, --phone-region, --secret-file, --require-key, --plan, --json, with the full list linked to docs/FLAGS.md) between two HTML comment markers in README.md, make docs regenerates it and make docs-check fails when it differs; the release workflow refuses a tag whose README Status section does not name that version. Files: tools/docgen, Makefile's docs and docs-check targets, README.md markers, .github/workflows/release.yml, CONTRIBUTING.md's docs paragraph.

## Acceptance



## Log

- 2026-09-17 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
