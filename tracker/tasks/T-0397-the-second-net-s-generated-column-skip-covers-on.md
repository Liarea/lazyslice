---
id: T-0397
title: "The second net's generated-column skip covers only the leaf the expression reads"
epic: E6
phase: 6
status: done
owner: opus
created: 2026-09-25
started: ""
closed: 2026-09-25
outcome: done
---

# T-0397 · The second net's generated-column skip covers only the leaf the expression reads

## Goal

JSON red-team round 1 (docs/reviews/2026-09-25-redteam-json/round1.json entry 24) and T-0272's review (low): internal/verify/secondnet.go's derivedFromMaskedLeaves skips every validator whose category a leaf masker emits for any generated column whose identifiers are all masked columns of its table when one of them is a document with LeafKeys, whichever leaf the expression reads, so a generated column that assembles an email, a phone number and a card number from copied leaves of a masked jsonb (the auth.identities shape) passes the net at exit 0. Fix: in netColumn, for such a column, skip a hit only when the generated value equals (after lower and btrim) a string leaf of the same row's document that leafRule says a category masker replaced; every other hit counts. At plan time internal/classify raises the keys a generated column's expression reads (exprIdentifiers plus the ->> and -> literals) to that column's own category whenever the generated column's samples validate, so the leaves are masked rather than merely caught. State the rule in ARCHITECTURE section 6 item 6 and the T1 amendment; regression fixture with the auth.identities shape and the round's assembled-value shape. Paths internal/verify, internal/classify, testdata/regressions, ARCHITECTURE.md, THREAT_MODEL.md, docs. Nothing may be masked less than before; make check, the verify and classify integration packages and make torture prove it.

## Acceptance

—

## Log

- 2026-09-25 2026-09-25 created
- 2026-09-25 closed: done

## Post-mortem

went well: landed as 39587db by Opus with no fix round: the second net scans a generated column over a masked document together with that row's documents and skips a hit only when the value equals, after lower and btrim, a leaf a category masker replaced in that row; classify raises every key a validating generated column reads through -> or ->> to the column's category so transform masks the leaf; regression 049 (the red team's assembled email, phone and card) fails before the fix on not-copied and I2 and passes after | went badly: keys read through #>, #>> or jsonb_extract_path are not raised at plan (T-0405, the net still refuses the value); a second-net refusal on a generated column prints a --mask hint that --mask refuses (T-0406); three lows filed as a follow-up (intermediate arrow operands are raised too, moving a nested leaf's coverage to residual-only; a generated text column copying a masked subdocument is now refused at exit 9 with no doc saying so; the btrim half, NULL rows and two-document expressions are untested) | change next time: a brief that names an operator's arrow syntax lists the other accessors so the developer covers or files them in one pass
