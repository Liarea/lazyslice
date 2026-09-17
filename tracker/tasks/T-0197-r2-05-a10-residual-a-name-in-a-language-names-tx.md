---
id: T-0197
title: "R2-05/A10 residual: a name in a language names.txt does not carry still reports \"no name or value signal\" as a clean bill of health"
epic: E5
phase: 5
status: done
owner: sonnet
created: 2026-09-15
started: 2026-09-17
closed: 2026-09-17
outcome: done
---

# T-0197 · R2-05/A10 residual: a name in a language names.txt does not carry still reports "no name or value signal" as a clean bill of health

## Goal

The 2026-09-15 round-2 red team's A10 (Khmer, Lao and Amharic transliterations in a column called label, no other personal column in the table) still leaks: names.txt cannot be widened to close it (T-0188 measured Wikidata's P407 coverage for Khmer/Lao/Amharic given+family names at 3/12/13 items total, nowhere near usable, and the class is unbounded by language regardless). The reviewer's own two suggestions, neither in T-0188's paths/scope: (1) internal/classify/reasons.go — change the 'no name or value signal' reason string for a character column that was scanned and matched nothing to something that reads as absence-of-evidence rather than a clean bill of health, e.g. 'nothing recognised; not proof the column is impersonal'; (2) internal/classify/classify.go's unknownColumnsBesideCertain rail — currently fires only in a table that already holds a certain person-identifying column, which is exactly the shape A10's table does not have; extending it to raise a short free-text column with no unique index and no type conflict as its table's only unexplained column is an ADR-level recall/precision trade (THREAT_MODEL.md T1), not a patch.

## Acceptance



## Log

- 2026-09-15 created

- 2026-09-15 front matter re-serialised with escaped inner quotes (T-0188 review, low finding)

- 2026-09-17 moved to E5 phase 5

- 2026-09-17 re-homed to E5 and narrowed by the orchestrator, 2026-09-17, after rounds 2, 3 and 4 each asked for the same thing: only the reason string. A sampled character column with no hit renders as absence of evidence ('nothing recognised in N samples; not proof the column is impersonal') and 'no name or value signal' stays for columns nothing could look inside. Widening names.txt stays refused; the certain-neighbour rail's length floor is its own task.

- 2026-09-17 started

- 2026-09-17 closed: done

## Post-mortem

went well: a sampled character column that matched nothing now reads as absence of evidence, and 'no name or value signal' stays for columns nothing could look inside; the Opus reviewer caught a sub-threshold hit reading as nothing recognised and a phone column raised by the guessed-region pass printing a self-contradictory line, both fixed with the existing fragment-blanking idiom; two fix rounds, reverify clean | went badly: filed after round 2 and landed after round 5, three rounds of the same request; no test pins the exact reason string (T-0250, filed by the developer); enum and domain columns the validators did look inside still print the old sentence (filed) | change next time: a reason-string change names the fixture that pins it in the brief, and a request three rounds make is landed in the next one
