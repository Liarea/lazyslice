---
id: T-0198
title: "special_category has no value validator; a digit/name-free special-category sentence still crosses unseen"
epic: E5
phase: 5
status: done
owner: sonnet
created: 2026-09-15
started: 2026-09-17
closed: 2026-09-17
outcome: done
---

# T-0198 · special_category has no value validator; a digit/name-free special-category sentence still crosses unseen

## Goal

ARCHITECTURE.md section 11.1's 2026-09-15 amendment (item 1, T-0189) and THREAT_MODEL.md:47 list R2-09's special-category sentence as closed. It is closed only by accident: pipeline.CatSpecial has no value validator anywhere -- internal/plan/ddlliteral.go's strongValidators, internal/verify/catalog.go's strongCatalogHit and internal/verify/validators.go's second-net list all have no CatSpecial entry. R2-09's own canary 'Priya Raghunathan disclosed her HIV diagnosis on 2019-04-02' is caught only because the date supplies textsig.AddressShape's digit; remove the date ('...last spring') and every strongHit call returns "" -- exit 0 on a health/special-category sentence with no name pair and no digit, in a DDL literal or a row. Fix: give special_category its own value validator (vocabulary or shape signal for disease/condition/orientation/religion sentences) in internal/textsig, and wire it into the three call sites above; pin the date-stripped canary in testdata/regressions at expect: refused, both as a DDL literal and as a row. No new flag needed unless the validator proves too broad over ordinary prose, in which case the existing --unmask/--allow-type-literal escapes are the answer.

## Acceptance



## Log

- 2026-09-15 created

- 2026-09-16 moved to E5 phase 5

- 2026-09-16 re-homed to E5 by the orchestrator, 2026-09-16: round 3 confirms it live twice, including a masked column's own CHECK carrying 'HIV positive, CD4 210'. Decision: a column the run masks may not carry a non-rewritable literal in its own constraint, default or index predicate at all, whatever it parses as, unless the existing named opt-out names the column; a special_category validator supplies the category in the message. Measure on the torture suite and report how many schemas the rule newly refuses before landing it as default.

- 2026-09-17 started

- 2026-09-17 closed: done

## Post-mortem

went well: measuring the torture suite before landing caught a large false-positive class (empty jsonb and array defaults), a core-fixture regression and a pre-existing SIMILAR TO gap; the special-category validator closes both round-3 canaries and the row-level second net got the same entry (T-0231); two fix rounds, the second docs-only | went badly: the first landing shipped the broadened rule with make torture red on four schemas although the brief said to land behind the opt-out above two, and the reviewer had to enforce the brief's own conditional; the fix round narrowed the rule to the special-category validator plus the masked-column rule for text literals, so T-0232's curation became moot; the message named an index as masked | change next time: a brief's numeric conditional is an acceptance line the developer checks before returning, and a rule that widens a refusal ships with its torture measurement in the return value
