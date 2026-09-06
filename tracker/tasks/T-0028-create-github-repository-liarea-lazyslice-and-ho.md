---
id: T-0028
title: "Create GitHub repository Liarea/lazyslice and homebrew-tap, push main, add HOMEBREW_TAP_TOKEN secret"
epic: E3
phase: 3
status: open
owner: human
created: 2026-09-05
started: ""
closed: ""
outcome: ""
---

# T-0028 · Create GitHub repository Liarea/lazyslice and homebrew-tap, push main, add HOMEBREW_TAP_TOKEN secret

## Goal

Gate 3 item: brew install from the tap prints a version. Needs a GitHub repo, a tap repo, and a token; only the human can create these.

## Acceptance

git remote origin set; CI runs on push; HOMEBREW_TAP_TOKEN set on Liarea/lazyslice (done 2026-09-05); a throwaway pre-release tag produces binaries and a cask

## Log

- 2026-09-05 created

- 2026-09-05 2026-09-05: repo created by Gareth, origin added (ssh), main pushed. Remaining: Liarea/homebrew-tap repo and HOMEBREW_TAP_TOKEN secret; repo is private, so a public brew install needs it public or the cask will point at inaccessible assets.

- 2026-09-05 2026-09-05: homebrew-tap created (public), HOMEBREW_TAP_TOKEN set by Gareth. Remaining: make the repo public before the first installable tag; decide git-crypt then.

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
