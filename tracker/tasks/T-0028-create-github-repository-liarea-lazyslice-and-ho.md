---
id: T-0028
title: "Create GitHub repository Liarea/lazyslice and homebrew-tap, push main, add HOMEBREW_TAP_TOKEN secret"
epic: E3
phase: 3
status: done
owner: human
created: 2026-09-05
started: 2026-09-15
closed: 2026-09-15
outcome: done
---

# T-0028 · Create GitHub repository Liarea/lazyslice and homebrew-tap, push main, add HOMEBREW_TAP_TOKEN secret

## Goal

Gate 3 item: brew install from the tap prints a version. Needs a GitHub repo, a tap repo, and a token; only the human can create these.

## Acceptance

git remote origin set; CI runs on push; HOMEBREW_TAP_TOKEN set on Liarea/lazyslice (done 2026-09-05); a throwaway pre-release tag produces binaries and a cask

## Log

- 2026-09-05 created

- 2026-09-05 2026-09-05: repo created by the maintainer, origin added (ssh), main pushed. Remaining: Liarea/homebrew-tap repo and HOMEBREW_TAP_TOKEN secret; repo is private, so a public brew install needs it public or the cask will point at inaccessible assets.

- 2026-09-05 2026-09-05: homebrew-tap created (public), HOMEBREW_TAP_TOKEN set by the maintainer. Remaining: make the repo public before the first installable tag; decide git-crypt then.

- 2026-09-15 started

- 2026-09-15 closed: done

## Post-mortem

went well: repository, tap and HOMEBREW_TAP_TOKEN all done by the maintainer on 2026-09-05 and 2026-09-06; closed late by the orchestrator | change next time: close human tasks the day they happen
