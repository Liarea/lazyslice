---
id: T-0166
title: "wire dsn param-drop warnings into internal/core and internal/pg's own dsn.Parse call sites"
epic: E9
phase: ""
status: open
owner: ""
created: 2026-09-14
started: ""
closed: ""
outcome: ""
---

# T-0166 · wire dsn param-drop warnings into internal/core and internal/pg's own dsn.Parse call sites

## Goal

T-0135 added dsn.ExtractParams (dropped-key names) and internal/discover/discover.go's warnDroppedParams, which prints on Options.progress wherever this package parses a connection string. internal/core/run.go's dsn.Parse(r.req.Source) / dsn.Parse(r.req.Target), and internal/pg/source.go and internal/pg/target.go's dsn.Parse calls, are the other places a raw operator-supplied connection string is parsed into a dsn.Ref, and none of them warn when a connection parameter is outside dsn.AllowedParams and gets dropped. Both packages were outside T-0135's paths (internal/dsn, internal/emit, internal/discover, internal/config, internal/pipeline, testdata only). Fix: give internal/core (which already threads an event.Sink) a way to print such a warning -- reusing internal/discover's stderr-progress pattern (see internal/discover/CLAUDE.md, both 'the pull and the start print outside the event catalogue' and the new note on warnDroppedParams) rather than a new internal/event/catalogue.yml row, unless a catalogue row is added in the same change. internal/pg has no progress channel today and may need one threaded in, or may reasonably rely on internal/core having already warned once for the same string it hands pg.Connect.

## Acceptance



## Log

- 2026-09-14 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
