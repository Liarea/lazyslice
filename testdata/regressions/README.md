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

There is one optional sixth key, and it exists because `expect: ok` is a weak
assertion for a defect that was a *collision*:

```
-- unique-masked: public.t.col, public.t.other   columns the target must hold
                                                masked, distinct and unusable
```

Every column it names is read out of the loaded target and must be non-empty,
carry `lazyslice-invalid-` (`mask.CredentialUniquePrefix`) on every non-NULL
value, and hold no two rows alike. It is what 004 and 007 assert since **T-0113**
made their runs exit 0: a run that copied the tokens verbatim also exits 0, and
the prefix half is what tells the two apart, while the distinctness half is the
original collision restated against the target instead of against an exit code.

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
| `009-citext-array-of-addresses-masked-as-one-string.sql` | plausible | **a leak, then a half-loaded target**: a `citext[]` of addresses arrives as one text literal, so the classifier saw one opaque value and copied it, and once it read inside the literal the transformer still masked it as one scalar and `CopyFrom` refused the result mid-load |

009's header now says `ok`. It did not always: `arrayArrivesAsLiteral` in
`internal/plan/writeback.go` was written as a stand-in for the element-wise
masker T-0118 hadn't yet landed (`internal/transform/array.go`), and while it
stood the file headered `exit 12 plan.refused.unwritable` — the CLI stopped at
plan, before a row moved, so the leak check below never ran. **T-0127** removed
that stand-in refusal (ordered behind **T-0129**, so the residual scan below
would not stay blind to the column class the refusal was newly letting
through) and flipped the header to `ok` in the same change. With the refusal
gone, the run now reaches the leak check, which is the half of 009 that
matters: plan admits the column, transform masks it element-wise, load writes
it back, and the residual scan tests the per-element entries — end to end,
with `internal/verify/arrayliteral.go` (T-0129) splitting the literal the same
way transform does so the scan sees inside it instead of past it.

## What each file asserts, beyond its exit code

Every regression whose header says `expect: ok` is also checked for leaks: no
email address or phone number of the source's may survive anywhere in the target
(the grep half of invariant I2). Some of these defects never changed an exit code
at all — 008 exited 0 before the fix and after it — so a suite that compared only
exit codes would have had nothing to say about the one that mattered most.

004 and 007 add the `unique-masked:` check described above. Both reduce a
`credential` column under a unique index, and both said `expect: exit 12
plan.refused.unique_domain` until T-0113: with the fixed literal as
`credential`'s only masker, `Domain()` was 1, ARCHITECTURE.md §5's `d_required`
could not be met at any row count, and the plan refused. `credential_unique`
(`mask/gen_credential.go`, T-0098) gave the category a generator wide enough for
a unique column, so both runs load and the header moved with the behaviour. The
classifier rules they pin — `raiseCompositeUnique` and the single-column
partial-index raise — are unchanged, which is why the schemas did not move with
the headers. One thing was lost and is recorded rather than papered over:
`plan.refused.unique_domain` is a real refusal that nothing here exercises any
more, and tracker **T-0124** owes a reduction that does.
