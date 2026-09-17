---
id: T-0231
title: "special_category has no value validator in the row-level second net"
epic: E9
phase: ""
status: open
owner: ""
created: 2026-09-16
started: ""
closed: ""
outcome: ""
---

# T-0231 · special_category has no value validator in the row-level second net

## Goal

T-0198 gave pipeline.CatSpecial a value validator (textsig.SpecialCategoryVocabulary) and wired it into internal/plan/ddlliteral.go's strongValidators and internal/verify/catalog.go's strongCatalogHit -- the task's own rule text scoped it to 'both catalog passes' -- but internal/verify/validators.go's row-scanning second net (ARCHITECTURE.md section 6 item 4) still has no CatSpecial entry, so a digit/name-free special-category sentence in an unmasked column's ROW values (not its DDL) still crosses the second net unseen. Add a validators.go entry (category: pipeline.CatSpecial, name: special_category, text: true, dict: false) calling textsig.SpecialCategoryVocabulary at the ordinary (non-strong) ratio, matching the pattern the other non-parse, vocabulary-backed entries (address, credential) already use; extend TestTheDictionaryRule-style coverage in validators_test.go/secondnet_test.go.

## Acceptance



## Log

- 2026-09-16 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
