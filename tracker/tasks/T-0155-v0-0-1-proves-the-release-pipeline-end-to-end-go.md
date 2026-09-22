---
id: T-0155
title: "v0.0.1 proves the release pipeline end to end: goreleaser, the tap cask, brew install prints a version"
epic: E5
phase: 5
status: done
owner: sonnet
created: 2026-09-15
started: ""
closed: 2026-09-22
outcome: done
---

# T-0155 · v0.0.1 proves the release pipeline end to end: goreleaser, the tap cask, brew install prints a version

## Goal

The tap is empty and no tag has ever been cut, so gate 3 item "brew install from your tap installs a binary that prints its version" was never actually run. After the repository is public (T-0154): check .goreleaser.yaml against goreleaser v2 with goreleaser check, confirm release.yml refuses a tag whose commit has no green ci run (T-0140) and passes one that has, tag v0.0.1 on a green commit, watch the release workflow, and prove on a machine or container that did not build it that brew install Liarea/tap/lazyslice prints the version. Record what broke and what was changed in docs/RUNBOOK.md "Cutting a release". Note that prerelease: true with a tap set to skip pre-releases would leave the tap empty; decide and record which setting v0.0.x uses.

## Acceptance

—

## Log

- 2026-09-22 2026-09-22 2026-09-22 v0.0.1 was tagged on 13a12ed by the maintainer and its release run (35736762240) failed at Set up job: sigstore/cosign-installer@v4 does not resolve (the action has exact v4.x.y tags and floating v3/v2 only). Nothing was published. Pinned to v4.1.2, every uses: in release.yml checked against its real tags, README Status now names v0.0.2, RUNBOOK records it (1924a43). v0.0.2 goes on 1924a43 once its CI run is green; the maintainer tags and pushes, the classifier blocks git tag here.
- 2026-09-22 closed: done

## Post-mortem

went well: three throwaway tags did exactly what v0.0.x exists for, each failure one step further along than the last, and v0.0.3 published everything the pipeline promises: six archives, six SBOMs, checksums.txt with a keyless Sigstore bundle that verifies, and the cask; brew install on a Mac that did not build it printed lazyslice 0.0.3 and Gatekeeper let it run; make tag (tools/release/tag.sh) now encodes every precondition and cut v0.0.3 itself under the maintainer's one-line permission rule | went badly: two workflow defects were invisible to every push to main because nothing but a tag runs release.yml: a floating cosign-installer major that the action never published (v0.0.1, run 35736762240) and cosign v3 ignoring the v2-era sign-blob flags (v0.0.2, run 35739188409); the first make tag run had two macOS-only bugs (BSD sed has no \s; gh --jq takes no --arg), one of which fired after the tag was pushed; the maintainer had to tag v0.0.1 and v0.0.2 by hand because the classifier blocks git tag for the orchestrator | change next time: any workflow step that only a tag exercises gets checked by make tag before the tag (the action-ref check is the first); a script that talks to gh is exercised on macOS before it is trusted with a tag; and v0.0.4 is a run of make tag with nothing else changed, to prove the tool end to end before v0.1.0 depends on it (commits 1924a43, b9195bf, 6da7920, 8627e2d, abf532d)
