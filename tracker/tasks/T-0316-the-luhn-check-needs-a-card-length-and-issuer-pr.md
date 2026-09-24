---
id: T-0316
title: "The Luhn check needs a card length and issuer prefix before it masks an id, number or version column"
epic: E6
phase: 6
status: done
owner: sonnet
created: 2026-09-23
started: ""
closed: 2026-09-24
outcome: done
---

# T-0316 · The Luhn check needs a card length and issuer prefix before it masks an id, number or version column

## Goal

Dogfood session 1: eight columns whose digit strings pass the Luhn checksum by chance were masked as free_text with 'a strong validator hit below the category threshold': CRM and billing customer ids, subscription ids, invoice and estimate numbers, and schema_migrations.version. A bare mod-10 pass over any digit run is one chance in ten; textsig's card validator should require a card length (12 to 19 digits) and a known issuer prefix range, and a column whose name says id, number, version or reference should need the full shape, not the checksum alone. Pin with a 14-digit timestamp-shaped version and a real test card number.

## Acceptance

—

## Log

- 2026-09-23 2026-09-23 created
- 2026-09-24 closed: done

## Post-mortem

went well: b800355: the Luhn check needs a card length (12 to 19) and a known issuer prefix, and a column whose name says id, number, version or reference needs the full card shape; each test was checked against a temporary break in the code and restored byte for byte | went badly: three lows: JSON leaves are now checked with ValidCard where the docs still say bare Luhn; UATP's single-digit prefix makes the issuer requirement almost meaningless for a value starting with 1 (a millisecond timestamp passes); card_no and cc_no are not caught by the name pattern and are scored under the stricter shape; filed | change next time: nothing beyond the lows
