---
id: T-0348
title: "verify's second net has no exemption for a framework metadata column that validates strongly"
epic: E9
phase: ""
status: done
owner: sonnet
created: 2026-09-24
started: ""
closed: 2026-09-24
outcome: done
---

# T-0348 · verify's second net has no exemption for a framework metadata column that validates strongly

## Goal

internal/verify/secondnet.go's netMode scans every column with Decision.Masked=false, including one T-0314's markNeverMasked (internal/classify/classify.go) deliberately never masks (a framework metadata table by internal/pipeline.IsFrameworkMetadataTable). Decision.NeverMasked only gates the dense-sequence exemption inside netColumn (the sequenceExempt branch, secondnet.go ~line 463) for the two requiresCorroboration-gated national_id-digits validator entries; every other entry -- email, phone, URL, financial_account/Luhn (strong, unconditional), national_id structured/checksum-only, credential, address, free_text -- runs unconditionally regardless of NeverMasked. A real Rails schema_migrations table whose migration-timestamp values happen to pass the Luhn check (the exact shape dogfood session 1 hit, per internal/classify/CLAUDE.md's T-0314 entry) therefore refuses the whole run at exit 9 (verify.refused.second_net, naming the column financial_account) even though classify correctly, deliberately left it unmasked. Verified 2026-09-24: testdata/regressions/042-framework-metadata-tables-copied-whole-and-unmasked.sql exits 0 with the fixture's ordinary (non-Luhn) migration timestamps, and exits 9 when the same fixture's schema_migrations.version values are replaced with Luhn-valid ones (20250101050000, 20250101130000, 20250101210000), confirmed against a real target via 'go test -tags "integration torture" -run TestTortureRegressions/042 ./internal/invariants/...'. Fix: either teach netMode/netColumn to read pipeline.IsFrameworkMetadataTable (or a broader NeverMasked signal) and skip the whole-column scan for such a table the way T-0314 already does in internal/plan and internal/classify, or decide explicitly that the second net is a deliberate backstop framework tables do not escape and document that in internal/verify/CLAUDE.md and testdata/regressions' own README so the next reduction knows why 042 cannot use a masking-worthy value. Whichever way it is decided, testdata/regressions/ is owed a new fixture pinning the chosen behaviour (a Luhn-valid schema_migrations table, expect: ok if exempted or expect: exit 9 verify.refused.second_net if not).

## Acceptance

—

## Log

- 2026-09-24 2026-09-24 created
- 2026-09-24 closed: done

## Post-mortem

went well: landed with T-0314 in effdee3: netMode skips a framework table's own bookkeeping column when classify marked it NeverMasked and internal/pipeline.IsFrameworkMetadataColumn lists it; the exemption is the allowlist's, so an identity-bearing column of a framework table is still scanned (both directions pinned in verify_test.go; the test fails without the case) | went badly: nothing | change next time: nothing
