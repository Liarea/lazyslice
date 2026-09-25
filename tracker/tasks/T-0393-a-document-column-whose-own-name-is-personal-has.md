---
id: T-0393
title: "A document column whose own name is personal has every leaf masked, whatever category the name matched"
epic: E6
phase: 6
status: done
owner: opus
created: 2026-09-25
started: ""
closed: 2026-09-25
outcome: done
---

# T-0393 · A document column whose own name is personal has every leaf masked, whatever category the name matched

## Goal

JSON red-team round 1 (docs/reviews/2026-09-25-redteam-json/round1.json, attempts_list entries 0, 1 and 2; attacker 1's A12, A13 and A11): a jsonb column named full_name, home_address, passwords, date_of_birth, national_id, emails, notes or by_phone matches a personal category whose accepts list in internal/classify/rules.yml does not admit json, so classify falls back to the plain semi_structured decision with Source ByClassifier ('jsonb is not an accepted type for person_name; type jsonb is a semi_structured type'), pipeline.Decision.LeafMap (internal/pipeline/classify.go lines 216 to 221) returns the leaf map, and every leaf is copied: names, addresses, passwords and dates of birth reached the target verbatim at exit 0. SECURITY.md item 8 and THREAT_MODEL T1's T-0272 amendment promise that every leaf of a column whose own name marks it personal is masked; today only special_category (accepts *) reaches that path. Fix: the fallback decision must carry the name hit so LeafMap returns nil, and each leaf is then masked under the name's category through the leaf masker where that category has a string masker (person_name, email, phone, address, person_date, national_id, credential, free_text) and as free_text otherwise; a raised or yml-masked document keeps its current path. Mirror the rule in internal/verify/jsonleaf.go so the second net agrees. Regression tests in internal/classify, internal/transform and internal/verify for each of the eight names, plus a testdata/regressions fixture; restate SECURITY item 8 and the T1 amendment so they describe the landed rule. Paths internal/pipeline, internal/classify, internal/transform, internal/verify, testdata/regressions, THREAT_MODEL.md, SECURITY.md, README.md, ARCHITECTURE.md, docs. Nothing may be masked less than before; make check, the transform, verify and classify integration packages and make torture prove it.

## Acceptance

—

## Log

- 2026-09-25 2026-09-25 created
- 2026-09-25 closed: done

## Post-mortem

went well: landed as e49df22 by Opus with two reviewers and no fix round: a document column whose own name is personal carries Decision.NameHit, LeafMap returns nil for it and every leaf is masked under the name's category through the leaf masker, mirrored in verify; the developer mutation-tested each layer and measured T1 over the ten torture schemas (44 of 190 document columns now mask every leaf, none the other way) | went badly: precision: 18 odoo translation labels named name are now filler (T-0396, a maintainer call); three lows filed as a follow-up (an hstore column with a NameHit narrows the second net's reading of it; SECURITY item 8 leaves the reader to infer that person_name and person_date names mask as filler; fixture 046 pins whole documents rather than every leaf); the developer filed T-0395 for the key half, a duplicate of T-0394, cancelled | change next time: a fix brief that names the categories says which masker each gets, and points at the sibling task already filed for the adjacent finding
