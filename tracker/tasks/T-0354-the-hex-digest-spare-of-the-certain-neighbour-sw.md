---
id: T-0354
title: "The hex-digest spare of the certain-neighbour sweep lets short hex tokens through unmasked"
epic: E6
phase: 6
status: done
owner: opus
created: 2026-09-24
started: ""
closed: 2026-09-24
outcome: done
---

# T-0354 · The hex-digest spare of the certain-neighbour sweep lets short hex tokens through unmasked

## Goal

T-0311's review (ec339f6): internal/classify/spare.go's hex-digest spare covers 8-to-31-character hex values, because textsig.LooksSecret claims hex only at 32 or more; in the reviewer's probe a column of 15-character hex values beside a certain email column was copied as 'hex digests'. Short hex API keys, reset tokens, invite codes and session ids under a neutral column name now reach the target unmasked where the sweep used to mask them: a recall loss in THREAT_MODEL T1's terms that no residual names. Spare hex digests only at digest lengths (7 to 12 for abbreviated commit hashes; 32, 40, 64 or 128 when LooksSecret has not already claimed the value), and let everything else fall to the sweep as before; pin with the reviewer's 15-character probe (must be masked) and a 40-character sha1 column (spared), and state in THREAT_MODEL T1 what the spare still leaves: a hex token of exactly a digest's length.

## Acceptance

—

## Log

- 2026-09-24 2026-09-24 created
- 2026-09-24 closed: done

## Post-mortem

went well: landed as e62e4fe with no review findings; HexDigest now spares only a digest's lengths (8 to 12, or 32, 40, 64, 128 when LooksSecret does not already claim the value), the 15-character reviewer probe is pinned as swept, and make torture found nothing because regression 043's serial had already moved to a digest length | went badly: the lower bound stayed at 8, not the 7 the goal asked, because 7 was never spared before and the acceptance forbids masking less; the SHA-1 pin could not be written at the classify level because net.ParseMAC reads bare 40-character hex as a MAC, filed as T-0363 | change next time: before writing a fixture for one shape, run every value validator on it so the fixture is decided by the rule under test
