---
id: T-0103
title: "An array of an extension type is sampled as one opaque string, so the classifier never sees the values inside it"
epic: E5
phase: 5
status: done
owner: ""
created: 2026-09-08
started: ""
closed: 2026-09-09
outcome: "done: in T-HARD-B (78530ef)"
---

# T-0103 · An array of an extension type is sampled as one opaque string, so the classifier never sees the values inside it

## Goal

internal/classify's scalars() flattens an array sample element-wise, which is what lets a text[] of addresses be decided email. It only works when pgx hands back a slice, and pgx only does that for an array type its map knows: the source pool runs in QueryExecModeExec and registers no user types (internal/pg forbids an AfterConnect hook on the source pool, T-0076), so a citext[] arrives as the single string {a@b.test,c@d.test}, no validator matches it, and the column is decided none and copied. Plausible's monthly_reports.recipients is exactly that column - the list of addresses a site's report is emailed to - so this is a real leak surface on a real schema. Found through testdata/regressions/005, which had to have its citext[] values changed away from addresses to keep the regression about the load failure it was written for. Owed: decide the fix - a Postgres array-literal splitter in internal/classify for a sample that is a string and a column that is an array, or source-side client type registration, which needs T-0076's rule to be read carefully first (that rule is about session state on the server, and a pgx type map is client-side).

## Acceptance



## Log

- 2026-09-08 created

- 2026-09-08 Second half, same root cause: because the array is not flattened, the classifier decides a category on the whole literal {a,b} and internal/transform then masks it as a *scalar* - so CopyFrom is handed the plain string $lazyslice$invalid for an _citext column and fails with 'cannot find encode plan' (SQLSTATE 57014), exit 7, mid-load. So the gap is not only 'the values inside are not seen': a masked array-of-extension-type column cannot be loaded at all. testdata/regressions/005 steers around both by keeping its citext[] values short and dull, and says so.

- 2026-09-08 moved to E5 phase 5

- 2026-09-09 closed: done: in T-HARD-B (78530ef)

## Post-mortem

Went well: landed. Went badly: nothing. Change: none.
