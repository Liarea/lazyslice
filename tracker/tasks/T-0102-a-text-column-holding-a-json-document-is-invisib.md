---
id: T-0102
title: "A text column holding a JSON document is invisible to ARCHITECTURE.md 4's JSON rule"
epic: E9
phase: ""
status: open
owner: ""
created: 2026-09-08
started: ""
closed: ""
outcome: ""
---

# T-0102 · A text column holding a JSON document is invisible to ARCHITECTURE.md 4's JSON rule

## Goal

4 gives json and jsonb columns a signal from their type and walks their leaves (jsonSignal); a *text* column holding the same document gets the scalar validators, and none of them matches - ValidEmail over {"uploader":"a@b.test"} is false and Prose over JSON is false - so the column is decided none and copied verbatim. Found while writing testdata/torture/rails-activestorage/generate.sql, where an ActiveStorage blob's metadata column carried an uploader address; the fixture was changed to real ActiveStorage metadata (which has no address in it) rather than left asserting a leak nobody had decided to fix, so the torture set no longer exercises this. Storing JSON in a text column is common enough - anything written before jsonb, anything portable across engines - that the gap is worth a decision. Owed: decide whether internal/classify should try to parse a text column's samples as JSON and run jsonSignal over the leaves when they parse (and at what fraction of samples), or state in ARCHITECTURE.md 4 that it deliberately does not.

## Acceptance



## Log

- 2026-09-08 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
