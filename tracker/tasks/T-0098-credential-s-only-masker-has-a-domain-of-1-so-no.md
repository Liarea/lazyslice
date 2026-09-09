---
id: T-0098
title: "credential's only masker has a domain of 1, so no unique credential column can be masked at all"
epic: E5
phase: 5
status: open
owner: ""
created: 2026-09-08
started: ""
closed: ""
outcome: ""
---

# T-0098 · credential's only masker has a domain of 1, so no unique credential column can be masked at all

## Goal

mask/register.go registers one generator for CatCredential: fixedMasker with the literal $lazyslice$invalid, Domain() == 1. ARCHITECTURE.md 5's unique-index rule needs d_required = n squared / 2 epsilon, so a column under a unique index that classifies as credential is refused at plan (exit 12) at every row count - MaxRows(1) is zero, so 'lower --take' is not an escape and only --unmask or a mapping_file is. Torture testing says how common that is: 20 of the 33 columns the ten real schemas needed a flag for are unique credential columns, six of them in supabase-auth alone (refresh_tokens.token, users.confirmation_token, recovery_token, email_change_token_new, email_change_token_current, reauthentication_token) and eight in gitlab. An authentication schema is nothing but unique credentials, and today lazyslice cannot mask one. Owed, in mask/ (ADR-006's own module, outside T-TORTURE's paths): a second CatCredential generator with a hash-derived opaque output and a domain above 2^64, registered after the fixed literal so Pick escalates to it only for a unique column - the same shape phone_unique and ip_unique already have for CatPhone and CatNetworkID. mask/writable.go and internal/classify/rules.yml agree already; nothing outside mask/ needs to change.

## Acceptance



## Log

- 2026-09-08 created

- 2026-09-08 moved to E5 phase 5

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
