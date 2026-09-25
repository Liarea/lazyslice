---
id: T-0369
title: "verify: the second_net hint names a --mask category the column's type accepts, or semi_structured for a json column"
epic: E6
phase: 6
status: done
owner: sonnet
created: 2026-09-24
started: ""
closed: 2026-09-24
outcome: done
---

# T-0369 · verify: the second_net hint names a --mask category the column's type accepts, or semi_structured for a json column

## Goal

T-0319's review (low, 2026-09-24): internal/event/catalogue.yml line 639's verify.refused.second_net message always suggests --mask TABLE.COL=CATEGORY with the validator's category, and that can be a dead end. The net scans the string leaves of every json/jsonb column, masked or not (internal/verify/secondnet.go header), so for an unmasked jsonb column with email leaves the suggested --mask t.c=email is refused at exit 2 (classify.refused.mask), because email does not accept the json family and semi_structured is the only category that does; for an already-masked json column --mask changes nothing and the net fails again next run; a digits-family hit on a type the category does not accept is refused the same way. The hint is the advertised way out of exit 9, so it must name a flag that works: choose the suggested category per column family (semi_structured for json, jsonb and hstore; the validator's category only when rules.yml accepts it for the column's family; otherwise the bare --mask TABLE.COL), or, for a column that is already masked, say so and offer --skip-table alone. Pin each case with a verify test; make docs so docs/ERRORS.md follows; paths internal/verify, internal/event/catalogue.yml, docs, README.md. Nothing may be masked less than before.

## Acceptance

—

## Log

- 2026-09-24 2026-09-24 created
- 2026-09-24 closed: done

## Post-mortem

went well: landed as 02c918f after one review round (one medium, three low): the second_net hint now names a category the column's type accepts: semi_structured for a json document, --skip-table alone for a masked document, and =special_category for a type conflict, since the bare --mask always records free_text, which never accepts bytea, uuid, inet, cidr or macaddr | went badly: three lows filed as a follow-up (the CLAUDE.md claim that the scalar type-conflict route is unreachable is wrong for bytea; categoryAcceptedFamilies is a hand copy of rules.yml with no test against the source; the new codes' catalogue messages are rendered in no test) | change next time: a task that copies a table from another package's rules gets a test that reads the source file
