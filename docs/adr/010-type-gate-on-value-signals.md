# ADR-010: The accepted-types gate silences value signals too

Status: accepted, 2026-09-06. Supersedes one sentence of ARCHITECTURE.md §4 recorded under T-0033.

## Context

T-0033 added the rule that every category declares the PostgreSQL types its maskers accept, and said the gate applies to the name signal only: a column whose sampled values validate for a category is classified on those values whatever it is called. In practice the entropy validator decided `credential` on Pagila's `last_update` timestamp columns and the dictionary validator decided `address` on `film.fulltext`, a tsvector. The maskers for those categories emit text, transform could not write it into a timestamp or a tsvector, and every full run over Pagila refused at exit 7 mid-run.

## Decision

A value signal for a category is silenced on any recognised type family the category's `accepts:` list omits: timestamp, date, time, interval, boolean, bytea, numeric and tsvector families never carry a text-category decision above `low`, and the reason string names the conflict. The families `enum`, `xml` and `other` are not silenced, so recall on unknown or user-defined types is unchanged. A tsvector column is category `derived_text`, decided by type alone, and is always masked to the empty tsvector, because it is derived from text that may itself be masked. Separately, the planner checks at plan time that every chosen masker's output is writable into its column's type and refuses with exit 12 (`plan.refused.unwritable`) otherwise; transform's exit 7 remains as a backstop that a passed plan cannot reach.

## Consequences

A column whose values genuinely hold personal data in a non-text family (a phone number stored as bigint) is still classified, because the phone category accepts integer families. What is lost is only the impossible case: a text masker on a column that cannot hold text. THREAT_MODEL T1 gains a line naming the silenced families so the recall boundary is explicit.

## Reversal condition

A category whose masker can emit a value of a non-text family for a text-family input, at which point that family joins the category's `accepts:` list and nothing here changes.
