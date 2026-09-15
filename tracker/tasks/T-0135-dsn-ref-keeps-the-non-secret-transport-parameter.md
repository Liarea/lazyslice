---
id: T-0135
title: "dsn.Ref keeps the non-secret transport parameters so a rerun preserves sslmode and certificate paths"
epic: E5
phase: 5
status: done
owner: sonnet
created: 2026-09-14
started: 2026-09-14
closed: 2026-09-14
outcome: done
---

# T-0135 · dsn.Ref keeps the non-secret transport parameters so a rerun preserves sslmode and certificate paths

## Goal

dsn.Ref holds host, port, database and user only (internal/dsn/dsn.go:70); emit writes those (internal/emit/document.go:459) and discover rebuilds a URI from them (internal/discover/discover.go:530), so a first run with sslmode=verify-full reruns at the pgx default of prefer and loses sslrootcert, sslcert, sslkey, connect_timeout, application_name and options: docs/reviews/2026-09-09/REVIEW.md finding 6. Add an allowlisted Params map to Ref holding exactly those keys (never password, never a whole DSN), carry it through the emitted endpoint block and the discover reconstruction, and have Ref.String show it only as a count. A key outside the allowlist is dropped with a warning naming it. Tests in dsn, emit and discover round-trip sslmode=verify-full with sslrootcert.

## Acceptance



## Log

- 2026-09-14 created

- 2026-09-14 started

- 2026-09-14 closed: done

## Post-mortem

went well: Params allowlist on dsn.Ref round-trips sslmode and certificate paths through emit and rung 0; the reviewer caught that the rung-0 retry lived only inside the ladder walk and not in Resolve's short-circuit, and then that the factored helper swallowed a parse failure silently; both fixed, the second by refusing at exit 2 naming the field and parameter | went badly: blocked after two rounds on a core test that compared a struct now carrying a map, outside the paths; landed by hand with one Sonnet agent (290k tokens) | change next time: a change to a shared value type gets internal/core's tests in its paths, or an IsZero from the start
