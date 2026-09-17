---
id: T-0259
title: "README's flag table is generated and checked, and a release tag refuses a README that does not name it"
epic: E5
phase: 5
status: done
owner: sonnet
created: 2026-09-17
started: ""
closed: 2026-09-17
outcome: done
---

# T-0259 · README's flag table is generated and checked, and a release tag refuses a README that does not name it

## Goal

The maintainer, 2026-09-17: the README is very out of date. It was last touched 81 commits ago, names none of the 33 flags, and nothing fails when it rots, while docs/FLAGS.md and docs/ERRORS.md stay true because make docs-check compares them with tools/docgen. Give README.md the same footing: tools/docgen writes the first-run flag table (the flags a first run meets: --source, --target, --root, --yes, --create-target, --unmask, --skip-table, --phone-region, --secret-file, --require-key, --plan, --json, with the full list linked to docs/FLAGS.md) between two HTML comment markers in README.md, make docs regenerates it and make docs-check fails when it differs; the release workflow refuses a tag whose README Status section does not name that version. Files: tools/docgen, Makefile's docs and docs-check targets, README.md markers, .github/workflows/release.yml, CONTRIBUTING.md's docs paragraph.

## Acceptance



## Log

- 2026-09-17 created

- 2026-09-17 closed: done

## Post-mortem

went well: README's first-run flag table is now generated between docgen markers from the same --help the binary prints, make docs-check diffs it without writing to the tree, and the release workflow refuses a tag the README's Status line does not name; review caught that make docs wrote a stray docs/README.md instead of the root file, fixed in the Makefile | went badly: four low findings were real and cheap (a generator crash reported as drift, an angle-bracket placeholder GitHub drops from the rendered table, the tag spliced into the release script, the generated-files rule not naming README), and they sat in the return value until the orchestrator fixed them at the gate-5 read | change next time: a low finding that is a one-line fix inside the task's own paths is fixed in the fix round, not filed (commit 165b24e, follow-ups in the gate-5 docs commit)
