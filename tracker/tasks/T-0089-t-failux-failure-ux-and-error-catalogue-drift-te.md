---
id: T-0089
title: "T-FAILUX: failure UX and error catalogue drift test"
epic: E5
phase: 5
status: done
owner: opus
created: 2026-09-08
started: 2026-09-14
closed: 2026-09-14
outcome: done
---

# T-0089 · T-FAILUX: failure UX and error catalogue drift test

## Goal



## Acceptance



## Log

- 2026-09-08 created

- 2026-09-14 started

- 2026-09-14 closed: done

## Post-mortem

went well: every error path audited, a catalogue drift test, --debug prints the wrapped chain and a recovered panic's stack, extract and transform goroutines recover panics, one pipeline context so a panic unwinding move cannot leave transform parked, kill-mid-load test | went badly: blocked after two rounds on a deadlock the fix round introduced (the transform recover returned without cancelling extract); landed by hand with a four-line fix verified by make check, torture and the core and load integration packages; Docker had died on the machine meanwhile, so one earlier integration verification may have passed by skipping (guard added to implement.js) | change next time: a recover that leaves a channel loop must cancel its producer; verify steps check Docker first
