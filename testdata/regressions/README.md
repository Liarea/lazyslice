# testdata/regressions

One file per defect that a real schema found and that a fixture did not.

Every file here started as a failing run against one of the ten schemas in
`testdata/torture/`. Those schemas are between seven and 370 tables; a failure
in one of them is not a test, it is an anecdote, so the rule is: **reduce it to
the smallest schema that still fails, check that in here, and only then change
`internal/`.** Each file names the schema it came from, the run that failed, the
message, and what the correct behaviour is.

The files are loaded and run by `make torture` (`internal/invariants`'s
`TestTortureRegressions`, behind the `integration` and `torture` build tags), so
a regression that comes back fails a build rather than being rediscovered by the
next person who tries GitLab.

## Format

A regression is a single `.sql` file that creates its own schema and rows, plus
a header comment block of exactly these keys, which the harness parses:

```
-- root:   public.some_table     the --root the failing run used
-- take:   20                    the --take it used
-- expect: ok                    `ok`, or `exit <n> <event.code>`
-- found:  <schema name>         which testdata/torture/ schema found it
-- why:    <one line>            what was wrong
```

`expect: ok` means the run must now succeed: the defect is fixed and this file
is the guard. `expect: exit 13 target.schema.not_recreatable.function` means the
run must fail *in that exact way* — some of these are refusals lazyslice is
right to make, and the regression is that it made them badly (no code, exit 1,
"run with --debug") rather than that it made them at all.

## Files

| File | From | Defect |
|---|---|---|
| `001-unique-index-masking-collision.sql` | django, rails-activestorage, supabase-auth | a masked column under a unique index collided at load: ARCHITECTURE.md §5's plan-time domain rule was never implemented |
| `002-function-default-refusal-uncoded.sql` | mastodon, gitlab | the exit-13 refusal for a column default calling a user function reached the operator as exit 1, `run.refused.internal`, "run with --debug" |
| `003-composite-unique-index-is-not-a-unique-column.sql` | rails-activestorage, supabase-auth, calcom, gitlab, mastodon | every column of a *composite* unique index was held to §5's `d_required`, so five schemas refused over columns that cannot collide |
| `004-composite-unique-index-all-masked.sql` | django | and the other side of it: a composite unique index with every key column masked collided at load |
| `005-array-of-extension-type-not-registered.sql` | plausible | an array of an extension's base type (`citext[]`) had no codec on the target connection, so the binary `COPY` wrote nonsense; and a scalar `hstore` column could never be loaded at all |
| `006-identity-sequence-renamed-table.sql` | metabase | `setval` named the source's sequence for an identity column whose table had been renamed, and died at `42P01` after every table had been copied |
| `007-partial-unique-index-masked-column.sql` | supabase-auth | a masked column under a *partial* unique index collided when the index was recreated |
| `008-name-hit-on-an-unaccepted-type-drops-the-type-signal.sql` | supabase-auth | **a leak**: a name hit the column's type does not accept removed the masking the type alone would have given, and a jsonb column of names, addresses and phone numbers was copied verbatim under exit 0 |

## What each file asserts, beyond its exit code

Every regression whose header says `expect: ok` is also checked for leaks: no
email address or phone number of the source's may survive anywhere in the target
(the grep half of invariant I2). Some of these defects never changed an exit code
at all — 008 exited 0 before the fix and after it — so a suite that compared only
exit codes would have had nothing to say about the one that mattered most.
