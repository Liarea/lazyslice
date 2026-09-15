---
id: T-0188
title: "Multilingual given-name and surname lists in textsig, sourced under CC0, so a non-English name in a column with no name rule is recognised"
epic: E5
phase: 5
status: open
owner: sonnet
created: 2026-09-15
started: ""
closed: ""
outcome: ""
---

# T-0188 · Multilingual given-name and surname lists in textsig, sourced under CC0, so a non-English name in a column with no name rule is recognised

## Goal

Red team 2026-09-15 R2-05 (A10) and round-1 A1, A2b: names such as Nkechi Okonkwo, Boguslaw Szczepanski, Thorunn Jonsdottir, Wanjiru Kamau, Ayse Yildirim in a column called label cross verbatim because internal/textsig/names.txt is English-only. ARCHITECTURE.md section 14 lists multilingual name dictionaries as a phase-5 item. Source the most frequent given names and surnames for at least these languages: Polish, Italian, Dutch, German, French, Spanish, Portuguese, Turkish, Swedish, Finnish, Icelandic, Yoruba/Igbo/Swahili (Nigeria, Kenya), Hindi, Arabic (romanised), Vietnamese, Japanese and Korean (romanised), Chinese (pinyin), from a CC0 or public-domain source (Wikidata given-name and family-name items are CC0; national statistics offices publish open lists), a few thousand per language at most, with the source, query or URL and licence recorded in THIRD_PARTY_NOTICES.md. Keep the English list; measure the confusion-matrix fixture and the three torture truth sets before and after and record the precision change in docs/TORTURE.md; a regression fixture with the red team names.

## Acceptance



## Log

- 2026-09-15 created

## Post-mortem

_(filled on close: what went well, what went badly, what we change next time)_
