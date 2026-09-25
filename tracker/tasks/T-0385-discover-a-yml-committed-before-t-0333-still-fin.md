---
id: T-0385
title: "discover: a yml committed before T-0333 still finds its lazyslice-target-lazyslice-* container; ARCHITECTURE section 9 records the name rule; the Q1 and create path is tested end to end"
epic: E6
phase: 6
status: done
owner: sonnet
created: 2026-09-25
started: ""
closed: 2026-09-25
outcome: done
---

# T-0385 · discover: a yml committed before T-0333 still finds its lazyslice-target-lazyslice-* container; ARCHITECTURE section 9 records the name rule; the Q1 and create path is tested end to end

## Goal

Three low findings from T-0333's review (80a93ce), the first a regression for an existing user. (1) internal/discover/discover.go line 968: rung0Target compares the committed TargetLabel with provision.Name(projectName(workdir)); a lazyslice.yml committed before 80a93ce for a project named lazyslice-foo records service lazyslice-target-lazyslice-foo, and Name now returns lazyslice-target-foo, so the labels no longer match and the container and the remembered password for it are not found (the maintainer's own dogfood directories are named lazyslice-dogfood*). Recognise both the legacy name (prefix plus the full project name) and the new name when matching a recorded TargetLabel and when looking up the remembered password, with a test that drives a legacy record. (2) internal/discover/provision/provision.go line 153: ARCHITECTURE.md section 9 (lines 120, 1008, 1108, 1118) and ADR-008's Q1 text say the container is named lazyslice-target-<project>; the code now strips a lazyslice- prefix and neither the ARCHITECTURE amendment nor internal/discover/CLAUDE.md records it, and the namePrefix comment still says the compose project name is what matters. Add the section 9 amendment stating the rule and that Name is not injective (shop and lazyslice-shop share a name, which --create-target and Q1 refuse when the container is another directory's). (3) internal/discover/discover_test.go line 120: the end-to-end test only composes projectName and provision.Name; assert through askQ1's prompt text or the fake provisioner's recorded create name for a workdir named lazyslice-dogfood3, and that the LabelProject label stays lazyslice-dogfood3. Paths internal/discover, docs, ARCHITECTURE.md, README.md; make check and go test -tags integration ./internal/discover/... prove it.

## Acceptance

—

## Log

- 2026-09-25 2026-09-25 created
- 2026-09-25 closed: done

## Post-mortem

went well: landed as 6a0e687 with no review finding above low: a yml committed before T-0333 finds its lazyslice-target-lazyslice-* container and the remembered password again through provision.LegacyName, ARCHITECTURE section 9 records the stripping rule and that Name is not injective, and two tests drive a legacy record and a lazyslice-dogfood3 directory end to end | went badly: four lows filed as a follow-up (the section 9 amendment's sentence about service:/target_label contradicts the fix it describes; the namePrefix comment names documentation rather than ownContainer as what keeps two checkouts apart; the LabelProject assertion stops at the fake provisioner's request; a trailing blank line) | change next time: a docs amendment that explains a fix is read against the fix's own test before review
