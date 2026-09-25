---
id: T-0399
title: "A composite type holding a json, jsonb or hstore field is refused at plan, not classified over its record text"
epic: E6
phase: 6
status: done
owner: sonnet
created: 2026-09-25
started: ""
closed: 2026-09-25
outcome: done
---

# T-0399 · A composite type holding a json, jsonb or hstore field is refused at plan, not classified over its record text

## Goal

JSON red-team round 1 (docs/reviews/2026-09-25-redteam-json/round1.json entry 13): an email inside the jsonb half of a record (composite) column reached the target, because the composite is classified over its record literal, where the document is quoted with doubled quotes, so ValidEmail never sees a bare address; verify's composite path has the same blind spot. THREAT_MODEL T1 says composites fail closed. Fix, fail closed: at plan, a composite column whose type holds a json, jsonb or hstore field (or another composite that does) is refused at exit 12 with a message naming the column and the field and --skip-table as the escape, in internal/classify's composite handling and mirrored in internal/verify so a copy that somehow holds one is refused too; a regression fixture; the T1 sentence on composites names the document case. Paths internal/classify, internal/plan, internal/verify, internal/event/catalogue.yml, testdata/regressions, THREAT_MODEL.md, docs. Nothing may be masked less than before; make check, the classify, plan and verify integration packages and make torture prove it.

## Acceptance

—

## Log

- 2026-09-25 2026-09-25 created
- 2026-09-25 closed: done

## Post-mortem

went well: landed as 9426813 after one review round: a composite column whose type holds a json, jsonb or hstore field (or a nested composite that does, or a domain over one) is refused at plan with exit 12 naming the column and the field, mirrored in verify, with regression 051; the reviewer caught that verify refused a --skip-table run and that a domain over jsonb slipped past, and that the stated root cause (the record's own quoting) was wrong, since the field is unquoted and the real gap is scoring a field's whole text without walking inside | went badly: the harness has no per-fixture flag key, so the --skip-table regression the reviewer asked for is a unit test instead; the wider nested-composite and embedded-value gap is T-0413; two lows filed as a follow-up (the structural branch drops a name-hit category and fragment; the fixture and comments cite round-1 entry 14 where the goal's entry 13 is the composite attempt) | change next time: a brief that states a root cause the developer must verify says so, and a fixture header cites the attempt by its attack text as well as its index
