---
id: T-0162
title: "move the SQL literal scanner out of internal/pipeline into a leaf package beside internal/textsig"
epic: E9
phase: ""
status: open
owner: ""
created: 2026-09-14
started: ""
closed: ""
outcome: ""
---

# T-0162 · move the SQL literal scanner out of internal/pipeline into a leaf package beside internal/textsig

## Goal

T-0134 put Literals, RewriteLiterals and QuoteLiteral in internal/pipeline/ddlliteral.go because internal/plan and internal/verify both need them and a stage package may not import another (internal/CLAUDE.md). internal/pipeline/CLAUDE.md's 'Never: add an implementation' rule says that is the wrong home: the package is ARCHITECTURE.md section 2's contract and nothing else. The right home is a leaf package beside internal/textsig (internal/ddlsig, importing only strings), which both stage packages may import; move the three functions and the Literal type there, update ARCHITECTURE.md section 2's import graph and internal/CLAUDE.md's rule list, and remove the exception note from internal/pipeline/CLAUDE.md.

## Acceptance



## Log

- 2026-09-14 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
