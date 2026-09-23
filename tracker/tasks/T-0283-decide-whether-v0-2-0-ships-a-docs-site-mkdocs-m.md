---
id: T-0283
title: "Decide whether v0.2.0 ships a docs site (mkdocs-material on GitHub Pages) or the in-repo docs stay the docs"
epic: E6
phase: 6
status: done
owner: human
created: 2026-09-22
started: ""
closed: 2026-09-23
outcome: done
---

# T-0283 · Decide whether v0.2.0 ships a docs site (mkdocs-material on GitHub Pages) or the in-repo docs stay the docs

## Goal

docs/BUILD_PLAN.md PROMPT 6.2 asks for a minimal mkdocs-material site (install, first run, CI usage with a GitHub Actions example, overrides, custom maskers, every flag, the error catalogue) with a test that nothing on it contradicts --help. Today docs/FLAGS.md and docs/ERRORS.md are generated and checked, and the README is the landing page, so the open question is whether a site earns its toolchain (Python, a Pages deployment, a second place for every flag) before launch or after the first ten issues. If yes, the orchestrator files and runs the build task; if no, README links to docs/ and the item moves to Later with that reasoning.

## Acceptance

—

## Log

- 2026-09-22 2026-09-22 created
- 2026-09-23 closed: done

## Post-mortem

went well: decided by the maintainer 2026-09-22: no docs site for now; the README is the landing page, docs/FLAGS.md and docs/ERRORS.md are generated and checked by make docs-check, and a site would add a Python toolchain, a Pages deployment and a second copy of every flag before any demand exists | went badly: nothing | change next time: revisit after the first ten issues (the build task is filed in Later with this reasoning)
