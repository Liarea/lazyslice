---
id: T-0368
title: "Reconcile ARCHITECTURE.md and docgen fixtures with T-0326's removal of the positional DSN"
epic: E9
phase: ""
status: done
owner: ""
created: 2026-09-24
started: ""
closed: 2026-09-24
outcome: done
---

# T-0368 · Reconcile ARCHITECTURE.md and docgen fixtures with T-0326's removal of the positional DSN

## Goal

T-0326 (noArgs review) removed root's positional DSN acceptance, but ARCHITECTURE.md §8 (lazyslice [DSN] [flags]) and §11's remediation line still show 'lazyslice postgres://...'; internal/core/core.go:52's ModeRun comment and tools/docgen/flags_test.go fixtures still say the same. Either update those to say --source only, or change the fix to accept a positional DSN only when --source is absent (refusing when both are given), per the maintainer's preference. Subcommands still document [DSN], so decide and make the surface consistent.

## Acceptance

—

## Log

- 2026-09-24 2026-09-24 created
- 2026-09-24 closed: done

## Post-mortem

went well: closed by T-0326's landing 4ba7027: ARCHITECTURE section 8's usage line and section 11's remediation show --source, core.go's ModeRun and Source comments say --source, the docgen fixtures follow; the decision was --source only on the root command (the dogfood log's own call), and a subcommand keeps its optional positional DSN but refuses at exit 2 when --source is also given | went badly: filed by a developer as a question for the maintainer when the task's goal had already decided it | change next time: nothing
