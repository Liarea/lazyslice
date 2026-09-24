---
id: T-0313
title: "A bare 'name' column and a '*_file_name' column are not a person's name without corroboration"
epic: E6
phase: 6
status: done
owner: opus
created: 2026-09-23
started: ""
closed: 2026-09-24
outcome: done
---

# T-0313 · A bare 'name' column and a '*_file_name' column are not a person's name without corroboration

## Goal

Dogfood session 1: 38 columns named 'name' (tags, folders, playlists, widgets, roles, languages, AI models, triggers...) and every column ending in _file_name (file_name, logo_file_name, transcode_file_file_name) were masked as person_name by the rule pack's (^|_)names?(_|$) token; two under unique indexes refused the plan. The classifier should need corroboration for the bare token: a name-dictionary hit rate in the samples, or a person-ish table (users, people, customers, contacts, employees, members, staff, authors), before reaching possible; *_file_name and plan_name/folder_name/display_name-of-an-object are labels. A person's name in a bare name column of a non-person table with samples the dictionary cannot carry is then residual 3 (THREAT_MODEL T1), which it already is for non-Latin names; say so. Measure recall on the truth sets; pin both directions.

## Acceptance

—

## Log

- 2026-09-23 2026-09-23 created
- 2026-09-24 closed: done

## Post-mortem

went well: 637305e; the bare name token and *_file_name need corroboration (dictionary hit rate or a person-ish table word) to reach possible, the person qualifiers (legal, preferred, nick, birth, married, holder, cardholder, billing, shipping _name, name_on_card, name_given/family/first/last/middle) still mask on the name alone, and a script over every torture schema showed which columns moved before the tests ran, catching nick_?names? which would have masked mastodon's nickname newly | went badly: the first pass moved every <thing>_name into the gated rule; birth_name and cardholder_name are masked under person_date and financial_account by higher-priority rules, which may be the wrong category; run-together spellings (nickname, legalname) match no rule before or since (T-0353); fixture 044's corroboration claim is not independent of its email neighbour | change next time: a rule-pack change lists the columns that move in each direction over the whole torture corpus in its return value, which this task did and which should be the template
