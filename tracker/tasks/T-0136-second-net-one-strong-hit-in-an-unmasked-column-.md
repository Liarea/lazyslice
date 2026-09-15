---
id: T-0136
title: "Second net: one strong hit in an unmasked column is a finding; classify masks a mixed column that carries a strong hit"
epic: E5
phase: 5
status: done
owner: sonnet
created: 2026-09-14
started: 2026-09-14
closed: 2026-09-14
outcome: done
---

# T-0136 · Second net: one strong hit in an unmasked column is a finding; classify masks a mixed column that carries a strong hit

## Goal

The second net fails a column only on the 80 percent ratio across at least three distinct hits (internal/verify/validators.go:34 and :44, internal/verify/secondnet.go:64), so one email among nineteen plain strings loads unchanged at exit 0: docs/reviews/2026-09-09/REVIEW.md finding 7, evidence/sparse_email.log. Separate category inference from residual detection. Verify: a strong validator (email, phone, credit card, IP, URL, as the strong set in validators.go defines it) hitting even one distinct value in an unmasked column is exit 9 naming the column and the hit count, never the value; the ratio rule stays for dictionary and heuristic validators. Classify: a column with any strong hit among its samples and a ratio below the category threshold is decided free_text, masked, with the explanation naming n of m samples that parse and the --unmask escape, so the run masks the column instead of failing at verify. Both fixtures and make torture must still pass; a torture schema that now needs a flag gets it recorded in docs/TORTURE.md with the reason.

## Acceptance



## Log

- 2026-09-14 created

- 2026-09-14 started

- 2026-09-14 closed: done

## Post-mortem

went well: one strong hit in an unmasked column is exit 9 naming column and count; classify decides free_text for a mixed column; regression 012 (0c34738, one fix round) | went badly: the reviewer caught two real defects in the first landing: the strong-hit gate checked the hit's category rather than the free_text family the column is assigned to (bigint column refused at 12), and Luhn was strong on numeric families so one chance hit in ~10 of 16-digit ids would have refused a correct target; both fixed with regression tests and Luhn values verified against the algorithm | change next time: a validator that is strong on text is not automatically strong on digits
