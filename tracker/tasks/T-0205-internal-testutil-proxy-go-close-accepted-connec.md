---
id: T-0205
title: "internal/testutil/proxy.go: close accepted connections in the same Cleanup that closes the listener"
epic: E9
phase: ""
status: open
owner: sonnet
created: 2026-09-15
started: ""
closed: ""
outcome: ""
---

# T-0205 · internal/testutil/proxy.go: close accepted connections in the same Cleanup that closes the listener

## Goal

T-0190's reviewer (low): splice returns when one direction ends and leaves the other io.Copy goroutine on a closed connection; connections accepted before Cleanup keep their goroutines and backend dials alive for the rest of the test binary. Track accepted connections under a mutex and close them in the Cleanup that closes the listener, or derive a context from the test and close both ends when it is done.

## Acceptance



## Log

- 2026-09-15 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
