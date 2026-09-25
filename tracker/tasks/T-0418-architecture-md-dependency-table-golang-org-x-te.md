---
id: T-0418
title: "ARCHITECTURE.md dependency table: golang.org/x/text is now a direct root-module dependency through internal/textsig"
epic: E9
phase: ""
status: done
owner: ""
created: 2026-09-25
started: ""
closed: 2026-09-25
outcome: done
---

# T-0418 · ARCHITECTURE.md dependency table: golang.org/x/text is now a direct root-module dependency through internal/textsig

## Goal

T-0403 made internal/textsig import golang.org/x/text/unicode/norm for its NFKC candidate spelling, so go mod tidy moved golang.org/x/text from indirect to direct in the root go.mod; ARCHITECTURE.md's dependency table (section 13, the golang.org/x/text row) still gives its reason as the mask module only, and section 2's import-graph line and section 12's layout line still say internal/textsig imports only ref and pipeline when it imports the standard library, phonenumbers and x/text/unicode/norm. ARCHITECTURE.md was outside T-0403's paths; correct the row's reason and the two textsig lines.

## Acceptance

—

## Log

- 2026-09-25 2026-09-25 created
- 2026-09-25 closed: done

## Post-mortem

went well: done with T-0403's landing 4ccfba6: ARCHITECTURE section 13's x/text row names internal/textsig's NFKC folding beside the mask module, and the import-graph and layout lines say textsig imports ref, pipeline and unicode/norm | went badly: nothing | change next time: nothing
