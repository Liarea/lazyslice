---
id: T-0279
title: "README as the landing page: GIF first, install, the run, why, a verified comparison table, how PII is decided, the rails"
epic: E6
phase: 6
status: done
owner: sonnet
created: 2026-09-22
started: ""
closed: 2026-09-22
outcome: done
---

# T-0279 · README as the landing page: GIF first, install, the run, why, a verified comparison table, how PII is decided, the rails

## Goal

docs/BUILD_PLAN.md PROMPT 6.1, superseded where ROADMAP.md says so. Restructure README.md in this order: the GIF (docs/media/first-run.gif, T-0065); one sentence on what it does; install for brew, go install github.com/Liarea/lazyslice/cmd/lazyslice@latest (prove it works from an empty module cache), and the release archives with the signed checksum; the one-command example with real output (the quickstart already there, kept verbatim); why, in three short paragraphs that match CONCEPT.md's three principles (zero config, safe by default, terminal first); a comparison table against Greenmask, PostgreSQL Anonymizer and Tonic on time to first snapshot, config required, databases, masking determinism and licence, where every cell about another tool links to that tool's own current documentation checked on the day (CLAUDE.md: recognising a name is not knowing its state) and a cell that cannot be verified says so; how it decides what is personal data and how to override it (--unmask with a reason, lazyslice.yml, the reasons output); the safety rails in six sentences (already there); a link to ROADMAP.md. Keep 'What a snapshot will not hide' and the exit codes. Do NOT write the 'why I built this' paragraph: it is the maintainer's own voice (T-0281). Everything the README claims must be true of v0.1.0 as installed from the tap; make docs-check must pass; the flag table stays generated. Study what the sqlit, lazygit and Bruno READMEs put on the first screen before writing, and say in concerns what you took from each.

## Acceptance

—

## Log

- 2026-09-22 2026-09-22 created
- 2026-09-22 2026-09-22 2026-09-22 correction: the maintainer's 'why I built this' paragraph is T-0282 (the goal says T-0281, which is the issue-templates task); the README leaves that slot out until T-0282 is written.
- 2026-09-22 closed: done

## Post-mortem

went well: the README is a landing page now: GIF, one sentence, install, the real transcript, why in three principle-shaped paragraphs, a comparison table whose every cell about another tool cites that tool's docs fetched the same day, how decisions are made and overridden, the rails, the residuals; review caught the personal-data section claiming a sweep the classifier only applies to character columns, and the developer proved go install does not work (the mask module's local replace) and wrote that honestly instead of a hopeful install line (d3fbf9a) | went badly: three sentences contradicted the transcript beside them (a timing the run never printed, 'asks one question and proceeds', 'shows all three signals') and reached main as lows; the orchestrator fixed them at the end-to-end read (533e446) | change next time: a landing-page task's reviewer checks every sentence that cites 'the run above' against the transcript, sentence by sentence, before anything else (commits d3fbf9a, 533e446; go install filed as T-0284)
