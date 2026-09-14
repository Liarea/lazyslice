---
id: T-0156
title: "Go public: git-crypt decision, tap initial commit, flip visibility, branch protection"
epic: E5
phase: 5
status: done
owner: human
created: 2026-09-14
started: ""
closed: 2026-09-14
outcome: done
---

# T-0156 · Go public: git-crypt decision, tap initial commit, flip visibility, branch protection

## Goal

ROADMAP.md "Go-public checklist". Public repositories get GitHub-hosted runner minutes free, which is what gate 5 needs. Decide T-0029 (recommendation: cancel), give the tap repository an initial commit, flip Liarea/lazyslice to public, protect main (ci and DCO required, no force pushes). The orchestrator can run the gh commands for the tap commit, the flip and the protection on a go from the maintainer.

## Acceptance



## Log

- 2026-09-14 created

- 2026-09-14 closed: done

## Post-mortem

went well: every step ran from the CLI in one sitting once the maintainer gave the go; the third-party notices were the one thing the checklist had missed and an agent verified all ten licences at their pins in five minutes | went badly: gitleaks was not in the toolchain until today; four fixture hits needed an allowlist | change next time: gitleaks in make check from the start, and a notices file the day a third-party fixture lands
