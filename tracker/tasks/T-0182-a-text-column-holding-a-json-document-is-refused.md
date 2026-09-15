---
id: T-0182
title: "A text column holding a JSON document is refused, not masked"
epic: E9
phase: ""
status: open
owner: ""
created: 2026-09-15
started: ""
closed: ""
outcome: ""
---

# T-0182 · A text column holding a JSON document is refused, not masked

## Goal

The 2026-09-15 red team's A5b put a three-level JSON document in a column declared text. internal/verify's second net now walks its leaves and refuses at exit 9 (T-REDFIX), so the leak is closed fail-closed, but internal/classify still cannot mask such a column: semi_structured's accepts: list in rules.yml is [json, jsonb, hstore] and is held against mask.WritableTypes by TestRulePackAgreesWithMaskAboutTypes, so adding the character families needs mask/writable.go to declare CatSemiStruct writable on text/varchar/bpchar/citext, and internal/transform's isDocument (transform.go) to take the document path for a character column whose decision is semi_structured (decodeDocument already carries the fromText flag for the round trip). mask/ is its own module and was outside T-REDFIX's paths.

## Acceptance



## Log

- 2026-09-15 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
