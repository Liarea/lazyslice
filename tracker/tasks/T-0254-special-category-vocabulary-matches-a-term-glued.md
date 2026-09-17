---
id: T-0254
title: "Special-category vocabulary matches a term glued to the next token by an underscore, and stripping a LIKE metacharacter leaves a separator"
epic: E5
phase: 5
status: done
owner: sonnet
created: 2026-09-17
started: 2026-09-17
closed: 2026-09-17
outcome: done
---

# T-0254 · Special-category vocabulary matches a term glued to the next token by an underscore, and stripping a LIKE metacharacter leaves a separator

## Goal

Round-5 replay (docs/reviews/2026-09-15-redteam/round5-still-leaking.json, the secrets attacker's catalog variant): CHECK (note <> 'HIV_POSITIVE') on a masked column survives into the target because the vocabulary regexp uses word boundaries and _ is a word character; HIV_STATUS and TRADE_UNION_MEMBER are how a status code is spelled. Normalise before matching: map every non-alphanumeric run and camel-case boundary to one space, lowercase, then match; and make StripPatternMeta leave a separator where it removes LIKE's _ instead of gluing the tokens. Add the three canaries to the vocabulary test and a plan-level case asserting exit 13. Files: internal/textsig/special.go, internal/pipeline/ddlliteral.go, internal/plan/ddlliteral_test.go.

## Acceptance



## Log

- 2026-09-17 created

- 2026-09-17 started

- 2026-09-17 closed: done

## Post-mortem

went well: special-category terms now match through underscores, hyphens, dots and camel case, including the acronym boundary (HIVPositive), and a stripped LIKE metacharacter no longer glues tokens; the Opus reviewer caught the first cut re-opening a round-2 leak, an underscored email in a regex operand split before the email validator saw it, fixed by trying both reductions, and caught the normalisation narrowing several terms that matched raw, fixed by matching raw and reduced; one fix round, reverify clean | went badly: an all-caps glued spelling (HIVSTATUS) stays unsplittable and is recorded as a residual in the comment; the LIKE spelling has no plan-level pin (filed) | change next time: a normalisation placed in front of validators is tested as not narrowing anything that matched before, with the prior canaries re-run through it
