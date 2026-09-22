---
id: T-0281
title: "Issue templates that keep personal data out of reports, and CHANGELOG.md as the pointer at releases"
epic: E6
phase: 6
status: done
owner: sonnet
created: 2026-09-22
started: ""
closed: 2026-09-22
outcome: done
---

# T-0281 · Issue templates that keep personal data out of reports, and CHANGELOG.md as the pointer at releases

## Goal

docs/BUILD_PLAN.md PROMPT 6.4's second half (v0.1.0 is cut, T-0155). Under .github/ISSUE_TEMPLATE/: a bug report that asks for the schema's shape (table and column names and types, the reasons output, the exit code and message) and says in its first line never to paste a row, a dump or a screenshot of one (SECURITY.md 'Do not send us personal data'); a PII-miss template whose whole body redirects to SECURITY.md's private route and refuses to be the place for the details; a feature request that must name which of CONCEPT.md's three principles it serves and what it refuses to add; and config.yml with blank issues off and the security advisory link. Labels: the repo already has good first issue, type:*, area:*; make the templates apply type:bug and type:feature. CHANGELOG.md: ROADMAP.md 'Versioning and releases' says it is a pointer at GitHub releases until 1.0; write that file (the releases URL, how notes are generated from commit bodies, make relnotes). Prove the templates with gh api on the repo's issue-template endpoint or by rendering them locally with a YAML parser; open no issue.

## Acceptance

—

## Log

- 2026-09-22 2026-09-22 created
- 2026-09-22 closed: done

## Post-mortem

went well: three issue forms and a config that turns blank issues off and points at the security advisory route; the PII-miss form is one paragraph that refuses to be the place for details; CHANGELOG.md points at the releases page (4d13f27) | went badly: the bug form invented an exit message no catalogue entry produces and required an exit code from bugs that exit 0; the orchestrator fixed both (533e446). The PII form applies type:security where the brief said bug or feature, which is the better label and stays | change next time: an example message in any form or doc is copied from docs/ERRORS.md, never composed (commits 4d13f27, 533e446)
