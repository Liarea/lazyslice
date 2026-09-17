---
id: T-0239
title: "The certain-neighbour rail no longer skips a column declared shorter than sixteen characters: it masks under a generator that fits or refuses"
epic: E5
phase: 5
status: open
owner: sonnet
created: 2026-09-17
started: ""
closed: ""
outcome: ""
---

# T-0239 · The certain-neighbour rail no longer skips a column declared shorter than sixteen characters: it masks under a generator that fits or refuses

## Goal

Round-4 replay (docs/reviews/2026-09-15-redteam/round4-still-leaking.json, the native-script variant): unknownColumnsBesideCertain masks an unrecognised column as free_text beside a certain personal column, but minUnknownLen = 16 skips varchar(12), so a name column of that length beside a real email column copies verbatim; a given name, a surname, a postcode, a national id and a phone all fit in twelve characters and the schema author picks the length. Keep the two-letter-code exclusion; for a column between that floor and sixteen, mask under a category whose generator fits the declared length (person_name's short values fit) or refuse at exit 12 naming the column and the --unmask escape, the way plan already refuses a unique column free_text cannot fill; never copy silently. Regression fixture: a varchar(12) column beside a certain email column. Files: internal/classify/classify.go and tests, testdata/regressions, docs/TORTURE.md if a torture schema moves, THREAT_MODEL.md T1's A2b bullet.

## Acceptance



## Log

- 2026-09-17 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
