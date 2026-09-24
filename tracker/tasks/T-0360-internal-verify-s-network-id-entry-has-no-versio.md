---
id: T-0360
title: "internal/verify's network_id entry has no version/build/release veto to match T-0317's classifier-side one"
epic: E9
phase: ""
status: done
owner: ""
created: 2026-09-24
started: ""
closed: 2026-09-24
outcome: done
---

# T-0360 · internal/verify's network_id entry has no version/build/release veto to match T-0317's classifier-side one

## Goal

internal/verify/validators.go's network_id entry (ValidIP || ValidMAC, text: true, strong: true) and secondnet.go's val.strong single-hit-fails path have no gate matching classify's networkIDVetoed (T-0317, internal/classify/validators.go). So a column like last_player_version that T-0317 now leaves unmasked (no certain neighbour: masked=false, cat=none) still carries its coincidental IP-parsing minority into the target, and verify refuses the whole run at exit 9 after the load -- the only way past is --unmask. Not a data leak (fails closed), but the dogfood run goes from over-masked to a post-load refusal, and classify/verify now disagree in a way internal/verify/CLAUDE.md argues against. Fix: add a columns-based gate (networkIDVetoed on snakeColumnName, or an equivalent mirrored check) to verify's network_id entry, the same way T-0315 and T-0316 each mirrored their vetoes into internal/verify's exemptColumns/columns func in the same landing. Add an end-to-end or verify-level test showing an unmasked app_version column of dotted versions does not exit 9. Source: T-0317 review round, finding 1 (high).

## Acceptance

—

## Log

- 2026-09-24 2026-09-24 created
- 2026-09-24 closed: done

## Post-mortem

went well: landed by hand with T-0317 in 44412d1: the second net's network_id validator no longer fires on a column whose name carries version, build or release, pinned by internal/verify/network_id_veto_test.go | went badly: filed by the T-0317 developer into E9 because it lay outside its paths, so the fix that made the classify change safe waited on a hand landing | change next time: the same change as T-0317: put the verify twin in the classify task's paths
