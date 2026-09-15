---
id: T-0168
title: "dsn.Ref.Params misses rung-2 (env/PGSERVICE) settings, so a first run through libpq env alone reruns with no sslmode at rung 0"
epic: E9
phase: ""
status: open
owner: ""
created: 2026-09-14
started: ""
closed: ""
outcome: ""
---

# T-0168 · dsn.Ref.Params misses rung-2 (env/PGSERVICE) settings, so a first run through libpq env alone reruns with no sslmode at rung 0

## Goal

internal/dsn/dsn.go: Ref.Params comes from a second, literal reading of the connection string (ExtractParams), never from what pgconn merged in from PGSSLMODE/PGSSLROOTCERT/a PGSERVICE entry. Host/Port/Database/User do include those (they're read off pgconn's resolved Config), so a source or target found through discovery rung 2 (libpq env / service file, ARCHITECTURE.md §9) records params: {} in lazyslice.yml even when it connected with sslmode=verify-full, and internal/discover's rung-0 rerun then dials with no sslmode at all -- pgx falls back to prefer, same failure mode as docs/reviews/2026-09-09/REVIEW.md finding 6 for one whole rung. Fix by recovering the effective settings from pgconn (e.g. sslmode from cfg.TLSConfig/cfg.Fallbacks, connect_timeout from cfg.ConnectTimeout) so rung 2 is covered too. Flagged in docs/reviews, 2026-09-14, finding 4.

## Acceptance



## Log

- 2026-09-14 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
