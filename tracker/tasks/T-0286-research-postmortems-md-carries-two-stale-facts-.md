---
id: T-0286
title: "research/POSTMORTEMS.md carries two stale facts the blog draft inherited"
epic: E6
phase: 6
status: done
owner: sonnet
created: 2026-09-22
started: ""
closed: 2026-09-22
outcome: done
---

# T-0286 · research/POSTMORTEMS.md carries two stale facts the blog draft inherited

## Goal

T-0280's review (2026-09-22): research/POSTMORTEMS.md line 97 says a Hacker News comment came 'a year after the shutdown' when its date is seven weeks after (17 Oct 2024 against a 31 Aug 2024 shutdown), and line 108 cites an absolute npm weekly download figure (121,478) that no longer holds and whose link did not substantiate it; docs/launch/BLOG_POSTMORTEMS.md was corrected to the ratio (about thirteen to one) with the npm downloads API endpoints as sources. Make the research document say the same, with the same sources, and nothing else in it.

## Acceptance

—

## Log

- 2026-09-22 2026-09-22 created
- 2026-09-22 2026-09-22 2026-09-22 T-0280's post-mortem calls this task T-0285; this is the research correction it means.
- 2026-09-22 2026-09-22 moved to E6 phase 6
- 2026-09-22 closed: done

## Post-mortem

went well: 930c9b1; the dated npm range links were re-fetched by the orchestrator (121,478 vs 9,808 for 23-29 Aug 2026, 12.4x) and the HN date checked by the reviewer (47 days after the shutdown); follow-up 4eaa948 made the copycat cell name its own figure | went badly: the developer changed a sourced number without re-fetching the source, and its summary said 12x while its first draft said 13x; the cell it wrote attached copycat's figure to snapshot | change next time: a research-doc task's brief should require pasting the fetched response for every number it touches
