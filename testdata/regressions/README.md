# testdata/regressions

One file per defect that a real schema found and that a fixture did not.

Every file here started as a failing run against one of the ten schemas in
`testdata/torture/`. Those schemas are between seven and 370 tables; a failure
in one of them is not a test, it is an anecdote, so the rule is: **reduce it to
the smallest schema that still fails, check that in here, and only then change
`internal/`.** Each file names the schema it came from, the run that failed, the
message, and what the correct behaviour is. 010, 011, 012 and 013 are the four exceptions and say so
in their own headers: all four came from the 2026-09-09 review's probe schemas,
which were already the smallest schemas that fail. 014 to 017 are four more of
the same kind, from the 2026-09-15 red team's own probe schemas, and 018 to 020
are three more again, from that red team's round two: none of the ten needed a
new flag once national_id joined the row-scanning nets (docs/TORTURE.md's own
T-0187 note), so what these three reduce is the round's own probe schemas
rather than one of the ten. **021 is a sixth exception of the same kind**, from
the review round that followed T-0187 rather than from one of the ten: the
digits-family national_id entry T-0187 added had no check digit at all, so it
refused an ordinary surrogate id column and an ordinary date column on the
strength of a ratio a mod-free exclusion-range check clears almost regardless
of what the column means; the reviewer's own probe schema is what 021 reduces.
**022 is a seventh**, from the review round that followed that one: the first
fix's key exemption was gated on internal/classify's own surrogate-key
decision, which inherited classify's own miss and exempted a primary key of
real SSNs along with it; a second reviewer's own probe is what 022 reduces.
**023 is an eighth**, from the review round that followed 022's: a sparse
numeric column with a fixed leading prefix is neither dense nor a date, so
neither of 021's two fixes reaches it, and it clears the SSA's exclusion
ranges at essentially 1.0 the same way a genuine leaked identifier column
does; the reviewer's own probe (an account-number column and an
invoice-number column, both ordinary) is what 023 reduces, and the fix is
corroboration rather than a third exclusion rule. **024 is a ninth**, from
the same round two's R2-05/A10: a person's full name in a language
`internal/textsig/names.txt` did not carry crossed a column no rule pack
pattern matches, in a table with no neighbouring personal column, under the
same "no name or value signal" report as 018 to 020's carrier; T-0188
reduces it against ten of the twenty languages it sourced, verified false
before the task's change and true after — A10's own Khmer/Lao/Amharic
instance is not one of the ten, and stays open (tracker T-0197) rather than
closed here, because the dictionary cannot be widened far enough to reach it.
**025 is a tenth**, from the round-3 red team's own kontaktnr/contact
finding: a real, correctly formatted UK phone number, dictated in one column
and written plainly in another, crossed under exit 0 because every phone
validator on the row path parsed under a fixed international-only region
hint; T-0221's `--phone-region` fixes the row path the way `--allow-type-
literal` fixed a different refusal with no escape at all. **026 is not a
reduction of a torture-schema failure and says so in its own header**, on
010's own precedent: it is the false-positive control the fix's own
corroboration requirement needs, written directly against the finished
behaviour rather than reduced from a run that failed. **027 and 028 are an
eleventh and a twelfth**, from the round-3 red team's own two special-category
canaries (`docs/reviews/2026-09-15-redteam/round3-still-leaking.json`, finding
15) rather than from one of the ten: `pipeline.CatSpecial` had no value
validator anywhere, so a health or religion sentence short of a digit and a
dictionary name pair crossed a CHECK constraint's own text under exit 0,
whether the column carrying it was masked (027, exit 13, T-0198's broadened
"whatever it parses as" rule) or not (028, exit 12, the pre-existing unmasked
path -- newly reachable here only because `textsig.SpecialCategoryVocabulary`
now names the category at all). **029 is a thirteenth**, from the review round
that followed T-0198's own landing: the same vocabulary validator had joined
both DDL-literal passes but not `internal/verify/validators.go`'s row-scanning
second net, so the identical sentence crossed unseen when it sat in a ROW
value rather than in a CHECK (tracker T-0231, `expect: exit 9
verify.refused.second_net`, since the column here carries no DDL literal at
all to refuse on). **030 is a fourteenth**, from the round-4 red team's
native-script variant against the A2b rail itself
(`docs/reviews/2026-09-15-redteam/round4-still-leaking.json`): the rail's own
declared-length exclusion, `minUnknownLen`, skipped a `varchar(12)` column
beside a `certain` email column in the same table, on the argument that
free_text's filler does not fit a short column -- which is not true of a
non-unique column (the rail already excludes a unique one, where the domain
rule can refuse a narrow generator); T-0239 lowered the floor from sixteen
characters to two, the shortest length that cannot hold even the rail's own
two-letter-code exception. **031 is a fifteenth**, from the T-0239 fix-round
review rather than from a torture schema: the lowered floor newly reached a
validated foreign key's character-family child (an ISO-style currency code)
beside a `certain` email column, masking the child to `free_text` while its
parent stayed unmasked -- the two ends of one join left in disagreement,
which internal/plan's equality and write-back checks both judge by type and
so cannot catch, and which internal/load turns into a half-loaded target at
`VALIDATE` (T-0132's own failure mode). The fix-round review's own fix
excluded a validated, non-virtual foreign key's character-family columns,
both ends, from `unknownColumnsBesideCertain`'s reach, the same way the rail
already excludes a unique index and an integer or uuid key column -- **and
the round-5 red team found that exclusion itself leaking** (see `035`
below), so `031`'s header now pins the corrected behaviour: both ends are
raised together, under the same category, rather than neither masked at
all -- and because reg031_currencies is a four-row table under a unique
index whose column is only three characters wide, `free_text`'s fixed word
list cannot offer the domain ARCHITECTURE.md §5 asks for, so
`internal/plan`'s existing, unmodified unique-index domain check refuses the
run at exit 12 rather than loading it. Its schema is unchanged; its
`expect` key moved from `ok` to `exit 12 plan.refused.unique_domain`, and
the `equal-masked:`/`not-copied:` keys came back out, since the harness
checks neither on a non-zero exit (T-0253). **Since T-0311 it is `ok`
again**: the child's twenty samples are four ISO codes each seen five times,
an enumeration the sweep now spares, so neither end is raised and both are
copied in agreement — `not-masked:` pins both ends and `equal-masked:` the
join.

**032 and 033 are a sixteenth and a seventeenth**, from the round-4 red
team's three A9b replays against `taxref`/`msisdn`-shaped tables
(`docs/reviews/2026-09-15-redteam/round4-still-leaking.json`): a
person-identifying neighbour the run had already decided to mask -- `msisdn`,
masked as `phone` on its name alone, with no value signal of its own -- was
never counted as corroboration at all, because
`Decision.TableHasLikelyPersonalColumn` only counts a neighbour at
`ConfLikely` or above and a name match with nothing from the values lands one
line below that; and, for the `varchar(9)` half, nothing on the
character-family side of `internal/verify/validators.go` had ever called
`ValidNationalIDDigits`, so the value never reached a validator that could
recognise it regardless. Fixed by a third decision field
(`TableHasMaskedPersonalColumn`), a character-family twin of the
digits-family national_id entry, and the dense-sequence exemption no longer
outranking corroboration once it exists to ask -- `032`'s and `033`'s own
headers have the full account, and both hold a *dense* block on purpose,
since none of `020`, `022` or `023` tests a corroborated column that is also
dense.

**034 is an eighteenth**, from the review round that followed T-0240's own
landing rather than from a torture schema: the fix above read
`Decision.TableHasLikelyPersonalColumn` as one of three signals deciding
whether corroboration overrides the dense-sequence exemption, and that field
counts a neighbour at `ConfLikely` or above under *any* category, with no
`identifiesAPerson` test -- so a `tsvector` column, `derived_text` at
`ConfCertain` by its type alone on every schema that has one, corroborated
an ordinary, non-key, dense business-number column into a false refusal at
exit 9. Fixed by `corroboratedForSequence`, a narrower function read only by
the dense-sequence override, which answers with `NameMatchedNationalID` and
`TableHasMaskedPersonalColumn` alone -- `TableHasMaskedPersonalColumn`'s own
gate does test `identifiesAPerson`, so a tsvector neighbour can never
corroborate through it. `requiresCorroboration`'s own three-signal
`corroborated` (the digits and character national_id entries' own gate) is
untouched, so `032` and `033` -- which corroborate through
`TableHasMaskedPersonalColumn` -- still refuse; `034`'s own header has the
full account.

**035 is a nineteenth**, from the 2026-09-17 round-5 red team's classifier
attacker, the FK variant
(`docs/reviews/2026-09-15-redteam/round5-still-leaking.json`): **a leak,
where `031`'s original shape was only a load-time refusal risk**. `031`'s own
fix (above) excluded a validated foreign key's character-family columns from
`unknownColumnsBesideCertain`'s reach entirely, at either end, on the claim
that a genuinely personal FK-linked column is still reached by every other
pass. `035` replays `030`'s own native-script names -- real Amharic names, in
a column named `ስም` ("name", in the language the column's own name is
written in too) -- as a validated foreign key child of a lookup table
holding the same names, whose own table has no `certain` column for any
other pass to key on; nothing else in `internal/classify` ever reached
either end, and both crossed into the target verbatim beside a real `email`
column that was masked correctly. Fixed by `fkPairs`
(`internal/classify/classify.go`, T-0253): a column this rail would
otherwise raise alone, at either end of a validated foreign key, is raised
together with every column connected to it across such an edge, under the
same category, so the join stays in agreement instead of both ends being
excluded from the rail's reach. reg035_name_dim is a five-row table under a
unique index (its primary key) whose column is twelve characters wide, and
`free_text`'s fixed word list offers only 635 distinct values there --
nowhere near ARCHITECTURE.md §5's `d_required` for five rows -- so
`internal/plan`'s existing, unmodified unique-index domain check refuses the
run at exit 12, naming both `ስም` columns together with `--unmask` for each:
the refusal the round-5 attack's own fix text asked for, reached with no new
code in `internal/plan` at all. `031` is the same mechanism's other control,
also a small unique lookup table, also a refusal now instead of a mask (its
header moved accordingly, above). Where a partner cannot be raised the same
way --
it already carries a decision ARCHITECTURE.md §4 forbids overriding, or a
measured two-letter-code shape -- neither end is raised, which
`internal/classify/redteam_test.go`'s `TestFKPairRefusedWhenPartnerIsTwoLetterCodes`
and `TestFKPairRefusedWhenPartnerHasTypeConflict` pin at the unit level; that
residual copies the pair rather than refusing the run, and tracker **T-0257**
carries the `internal/plan` work to turn it into an exit-12 refusal instead.

**036 is a twentieth**, from the T-0253 review round that followed `035`'s
own fix rather than from a torture schema: bounding `fkPairs` to cref's
*direct* partners closed the medium finding that walking the whole connected
component caused, but reopened the leak `035` closes one hop further out.
`reg036_members.slug_leaf` (cref) and its direct partner
`reg036_name_mid.slug_mid` are raised together, exactly as `035` pins -- but
`slug_mid` is itself the child end of a *further* validated foreign key, to
`reg036_name_root.slug_root`, and nothing but `fkPairs`' own upward walk
ever reaches that further parent: `propagateKeys` (a separate, later pass)
only ever pushes a decision from a masked parent down to its children, never
up from an unmasked one. Before the fix, `reg036_name_root.slug_root` stayed
`CatNone` and carried the same real names verbatim (THREAT_MODEL.md T1), and
the load would `VALIDATE` a foreign key between a masked child and an
unmasked parent (`031`'s own T-0132 half-loaded-target shape, T8). Fixed by
walking `fkParents` -- the child-to-parent half of `fkPartners`
(`internal/classify/classify.go`) -- transitively from cref and from every
column already raised, so a chain of keys is brought into agreement as far
up as it goes; the *downward* bound `035`'s own fix relies on is unchanged,
since walking that direction transitively is the medium finding all over
again. `reg036_name_root` and `reg036_name_mid` are both five-row tables
under a unique index (their own primary keys), so this refuses at exit 12
the same way `035` does, naming the columns `--unmask` can release.
`internal/classify/redteam_test.go`'s
`TestFKPairWalksUpwardThroughAChainedForeignKey` pins the same shape with
three distinct column names (`slug_root`/`slug_mid`/`slug_leaf`), on
purpose: identical names at every hop would let the same-name pass mask the
grandparent for an unrelated reason and hide this bug.

**037 is a twenty-first**, from tracker **T-0257** rather than from a
torture schema: `035` and `036` are both the *raised-together* half of
`fkPairs` -- a partner that qualifies, refused afterwards only because the
lookup table it sits in is too small for `free_text`'s domain. `037` is the
other half, the one `TestFKPairRefusedWhenPartnerIsTwoLetterCodes` and
`TestFKPairRefusedWhenPartnerHasTypeConflict`
(`internal/classify/redteam_test.go`) hold at the unit level: a partner that
does not qualify at all, because it already carries a decision
ARCHITECTURE.md §4 forbids overriding. `reg037_profiles.dob` is `citext`, so
`person_date`'s name pattern matches "dob" but the type does not accept it,
and `internal/classify` records a type conflict at `low` rather than a date.
`reg037_members.linkval`, a validated foreign key child of `dob`, has no
name or value signal of its own, so `unknownColumnsBesideCertain` would
otherwise raise it alone beside `reg037_members.email` (a certain column in
the same table) and leave `dob`'s identical values copied on the other side
of the join -- the leak `fkPairs` exists to close. Before **T-0257**,
nothing read the refusal `fkPairs` already recorded on both columns
(`Decision.Refused`, `Decision.RefusedPartner`), and both loaded copied
verbatim under exit 0; `internal/plan/fkpair.go`'s `checkFKPairRefusal` is
the fix, and this file is its torture-schema guard, refusing at exit 12,
naming both `reg037_members.linkval` and `reg037_profiles.dob`, with
`--unmask` the escape for each. `internal/plan/fkpair_test.go`'s
`TestFKPairIsRefusedAtPlan` pins the same shape at the unit level, driving a
hand-built `Decision` through the planner directly.

**038 is a twenty-second**, from tracker **T-0287** rather than from a
torture schema: `docs/media/first-run.gif`'s own last frame read back
`public.customer.first_name` as `Emma Popescu` and `last_name` as
`Oscar Adler`, because `personNameMasker` (`mask/gen_text.go`) never read
which column it was filling and always emitted "Given Family" — right for a
`full_name` column, wrong for `first_name` (two words instead of one) and
`last_name` (a whole name instead of a surname). Fixed by `mask.Role`
(`RoleGiven`, `RoleFamily`, the zero-value `RoleFull`), decided at classify
time from the column's own name (`internal/classify/classify.go`'s
`roleForColumn`: `first`/`given`/`forename`/`fname` decide `RoleGiven`,
`last`/`family`/`surname`/`lname` decide `RoleFamily`, everything else —
`name`, `full_name` among them — keeps `RoleFull`) and carried on
`Decision.Role` beside `Category` the same route `Decision.UniqueIndex`
already takes to `mask.Constraints.Unique`: `internal/transform` sets
`Constraints.Role` from the Decision, `internal/emit` writes and reads it
back under `role:`, and `personNameMasker.Mask`/`.Domain` branch on it — a
given name only for `RoleGiven`, a surname only for `RoleFamily`. This file's
three columns pin the three roles side by side, and `not-copied:` proves none
of the five source rows' own words survive; the shape of what *did* land —
one word for `first_name`, one for `last_name`, two for `full_name` — is
`mask/role_test.go`'s `TestPersonNameRoleShape` and
`internal/classify/role_test.go`'s `TestPersonNameRoleFromColumnName`, since
`not-copied:` alone cannot tell a correctly shaped fake from a differently
wrong one. **Neither of those two proofs alone covers the path between
them** (the fix round that followed this task's own review): the first calls
`personNameMasker.Mask` directly and the second checks only
`Decision.Role`, so nothing before `internal/transform`'s own
`TestPersonNameRoleReachesTheMasker` asserted that `internal/transform`'s
`plan()` actually carries `Decision.Role` onto `mask.Constraints.Role` —
deleting that one assignment would have passed this file, `make check` and
`make torture` alike, dropping every `person_name` column silently back to
`RoleFull`.

**039 is a twenty-third**, from the fix-round review that followed `038`
rather than from a torture schema, and its premise has since inverted. That
round's `roleGivenWords`/`roleFamilyWords` (`mask/words.go`) were synthetic
consonant-vowel tokens, meant to hold no real name, because the residual
scan confirmed any masked value the source column held anywhere and refused
the run at exit 9; a reviewer measured real names among them (gale, sage,
mari, boris, titus and more), and this file was written seeded with exactly
those so that an overlap would refuse here. ADR-015 (tracker **T-0302**)
then taught the residual scan to explain a coincidence inside the masker's
own vocabulary, and **T-0304** made that vocabulary real names: the 2020
Census top given names and surnames, for every `person_name` role and for
email local parts. The synthetic lists and the three tests that pinned them
(`TestRoleWordsExcludeKnownRealNames`,
`TestRoleWordsDisjointFromSharedNameLists` and `internal/classify`'s
`TestRoleWordsExcludeCurrentNameDictionary`) are gone. What the file proves
now is ADR-015's verify rule end to end over a column of real names masked
to real names: rows 1-12 keep the review's names, rows 13-100 are Census
names, so some masked `first_name` and `last_name` values equal another
row's real value on every run, and it must load at `expect: ok` with
`verify.residual.explained` counting them. Its `not-copied:` names only
`email`, whose source values can never equal a masked one.

**040 is a twenty-fourth**, from ADR-015 (tracker **T-0302**) rather than
from a torture schema, and it is the case `039`'s filtering could only make
rarer: a masked name equal to *another* row's real name in the same column.
The residual scan confirmed a hit by asking whether the source column held
the value anywhere, so any real-name list over any real name column refused a
correct run at exit 9. This file seeds `first_name`/`last_name` with words of
the masker's own lists (since T-0304 the 2020 Census names; before it, the
synthetic role tokens), so such coincidences happen on every run by
construction, in a table with a primary key and in a twin without one.
Since T-0302 the scan explains a hit inside the masker's vocabulary by count
and row identity instead of probing it, and prints
`verify.residual.explained`; both tables must load at exit 0. It carries no
`not-copied:` key on purpose — that key greps the whole target for every
source value, and here a match is the correct outcome.

**041 is a twenty-fifth**, from ADR-015's "Consequences" and **T-0304**
rather than from a torture schema: once masked first and last names are
real Census names, a `full_name GENERATED ALWAYS AS (first_name || ' ' ||
last_name)` column is a real given name followed by a real surname in every
row of the target, which is the shape `internal/verify`'s second net
refuses at exit 9 as a name left in cleartext. The target recomputes the
column from the two masked ones, so the second net skips its dictionary rule
for a generated column whose expression names only masked columns of its own
table (`generatedFromMasked`), and the run must load at `expect: ok`.

**042 is a twenty-sixth**, from dogfood session 1 (**T-0314**) rather than
from a torture schema or a review round: `schema_migrations` had no foreign
key reaching it at all — the ordinary shape of migration bookkeeping, not a
corner case — so `internal/plan`'s own lookup rule ("at least one incoming
edge") left it `SchemaOnly`, zero rows, and a fresh Rails checkout re-ran
every migration against the target. `ar_internal_metadata` went the other
way: copied verbatim, its `environment` row still reading `production`,
which makes the same checkout refuse a destructive rake task against its own
clone. `internal/pipeline.IsFrameworkMetadataTable` (that package's own
CLAUDE.md has the fixed list of names, one per well-known migration tool)
now forces both tables to a `Lookup` step regardless of reachability, and
`internal/classify` never masks a column of one. This file pins the CLI half
— a real run over these two exact table names, with neither column masked —
under `not-masked:`; the row-count and environment-rewrite claims
`unique-masked:`/`equal-masked:`/`not-copied:` have no way to state are
`internal/plan`'s own `TestPlanFrameworkMetadataTablesCopiedAsLookups` and
`internal/load`'s own `TestLoadRewritesArInternalMetadataEnvironment`, both
against a real target.

**043 is a twenty-seventh**, from the same dogfood session (**T-0311**):
the neighbouring-column rule's second arm swept every signal-less character
column beside a `certain` email column into `free_text` — a role, a state
machine, a log level, a timezone, a text uuid, asset paths — and the copy
held word salad where the application expects `admin` or `linux`. A column
whose samples are an enumeration, or all one identifier shape, is now spared
and its reason line says which (internal/classify's own T-0311 section has
the thresholds). The file pins both directions: eleven spared columns under
`not-masked:`, and under `not-copied:` a native-script name column that is
an enumeration by count alone and must still be swept.

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
-- found:  <schema name>         which testdata/torture/ schema found it (or,
--                                for 010, which review did)
-- why:    <one line>            what was wrong
```

`expect: ok` means the run must now succeed: the defect is fixed and this file
is the guard. `expect: exit 13 target.schema.not_recreatable.function` means the
run must fail *in that exact way* — some of these are refusals lazyslice is
right to make, and the regression is that it made them badly (no code, exit 1,
"run with --debug") rather than that it made them at all.

There are two optional keys, and both exist because `expect: ok` is a weak
assertion for a defect that was a *collision* or an *inequality*:

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

The second is the mirror image of it, and **T-0132** added it:

```
-- equal-masked: public.t.child_col = public.t2.parent_col   two columns joined
                                                            by a foreign key
                                                            that must still hold
                                                            the same values
```

Each pair is read out of the loaded target: every non-NULL value of the
left-hand column must also be a value of the right-hand one, and the left-hand
column must hold at least one. That is the relation a foreign key is, asserted
against the rows rather than inferred from the exit code — so it still says
something if a change stops recreating the constraint. It is what 010 asserts,
beside a `unique-masked:` on the parent: the parent's values are masked,
distinct and unusable, and the child holds the same ones. Either key alone would
pass a run that copied both columns verbatim.

**031, 035 and 036 (T-0253) were expected to need this pairing and,
measured, need neither key** (031 has used `equal-masked:` again since
T-0311 spared its enumeration of codes; the rest of this note is about the
T-0253 state, and still true of 035 and 036). All three raised a validated foreign key's ends
together under `free_text`, and every parent involved -- including `036`'s
further one, reached only by `fkPairs`' upward walk -- is a small table
under a unique index whose declared width `free_text`'s fixed word list
cannot fill to ARCHITECTURE.md §5's `d_required` — so `internal/plan`'s
existing unique-index domain check refuses every run at exit 12 before
anything loads, and `expect: exit 12 plan.refused.unique_domain` is the
whole assertion: the harness checks neither optional key on a non-zero exit
(see 020, above), and there are no rows in the target for `equal-masked:` or
`not-copied:` to read. A schema whose FK-paired lookup table held enough
rows, or whose column were wide enough for a bigger word-list domain, would
load rather than refuse, and would need the `equal-masked:`/`not-copied:`
pairing this note first described — none of the three is that schema.

A third, added by **T-0161**:

```
-- masked-default: public.t.col   a masked column whose DEFAULT must hold a
                                 masked address in the target's own catalog
```

Every column it names is read back out of the target's own `pg_attrdef`
(`assertTortureDefaultIsMasked`) and must parse as an address that is not the
source's own literal. `expect: ok` alone proves the object was recreated, not
that ARCHITECTURE.md §11.1 arm 1 ran rather than merely not refusing; this is
what tells the two apart, the way `unique-masked:` tells "masked" from
"copied verbatim" for 004 and 007. It is what 011 asserts.

A fourth, added by **T-0187**:

```
-- not-copied: public.t.col, public.t2.col2   every source value must not
                                             survive anywhere in the target
```

Each column it names has its distinct non-NULL source values read out and
grepped for, byte for byte, over every cell of the whole target
(`assertTortureColumnNotCopied`). It exists because the leak check every
`expect: ok` regression gets automatically — "What each file asserts, beyond
its exit code", below — only ever proves the absence of the two shapes
`scan_test.go`'s detectors recognise, an email and a phone number; a defect
over a third shape needs its own values checked directly rather than trusting
a pattern that was never written to look for it. It is what 018 and 019
assert, for the national identifier the 2026-09-15 red team's round two found
no validator anywhere on the row path; 020, the same round's numeric-family
half, is a refusal rather than a mask and asserts its exit code instead (see
below). **For an array column it reads one element at a time, not the whole
array's text rendering** (the review round that followed T-0187): 019's own
column is `text[]`, and internal/transform masks such a column element-wise,
so the check that would actually catch a partial failure — one element of
several crossing unmasked — has to compare elements and not the array's
rendered literal, which the whole-array form the check used to read could
never reflect.

A fifth, added by **T-0221**, is `not-masked:` and reads the *emitted yml*
rather than the target's rows:

```
-- not-masked: public.t.col   the yml must record no masker: and no unmask:
--                            for the column -- internal/classify decided
--                            none
```

Each column it names is read back out of the run's own `lazyslice.yml`
(`assertTortureColumnNotMasked`) and must carry neither a `masker:` nor an
`unmask:` entry. It exists for the same reason `not-copied:` does, in the
opposite direction: `expect: ok` alone cannot tell "left alone" from "masked,
but the masker's own output happened not to trip anything a rows-based check
would catch" — `phone`'s masker accepts the same text and numeric families a
false-positive guessed-region hit would reach, so a wrongly masked column can
still pass every check that only reads rows. It is what 026 asserts, for the
corroboration gate a guessed-region phone hit needs before it may decide
anything (T-0221; see `internal/classify/CLAUDE.md`'s own note on the point).

`--phone-region` is the sixth optional header, and it is not a check but an
extra flag:

```
-- phone-region: GB   passed to the run as --phone-region GB
```

It exists because a defect can be specific to the region-aware phone reading
rather than to the classifier's ordinary name or value signals, and the
harness's fixed argument list (`--source`, `--target`, `--root`, `--take`,
`--secret-file`, `--config`, `--yes`) had no room for a flag beyond those
until T-0221. It is what 025 sets.

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
| `010-fk-connected-columns-mask-differently.sql` | the 2026-09-09 review, finding 3 | the masker was chosen per column, so a unique column escalated to `credential_unique` while its foreign-key child kept the fixed literal: equal inputs masked to different outputs and the load ended at exit 8 |
| `011-masked-column-default-holds-a-literal.sql` | the 2026-09-09 review, finding 5 | **a leak no row scan could see**: a masked column's `DEFAULT` was recreated in the target verbatim, so the address in it survived under exit 0 and the application's next `INSERT` would put it back into a row |
| `012-single-strong-hit-in-a-mostly-plain-text-column.sql` | the 2026-09-09 review, finding 7 | **a leak the 80% ratio was the wrong question for**: one email address among nineteen ordinary strings was a 5% hit ratio, so the classifier decided `none` and both nets agreed with it, under exit 0 |
| `013-json-object-key-that-parses-as-an-email.sql` | the 2026-09-09 review, finding 8 | **a leak nothing looked at**: a jsonb document keyed by an email address masked its value and kept the address as the key, under exit 0 |
| `014-array-elements-that-dodge-every-validator.sql` | the 2026-09-15 red team, A3 | **a leak both nets agreed was clean**: a `text[]` of addresses written `grace.hopper AT realcorp DOT example` was split correctly and recognised by no validator, under exit 0 |
| `015-printable-bytea-in-a-table-with-no-certain-column.sql` | the 2026-09-15 red team, A4a | **a leak nothing looked at**: a `bytea` holding printable UTF-8 is skipped by the classifier before any validator runs and is outside the second net's family set, under exit 0 |
| `016-enum-label-holds-an-email-address.sql` | the 2026-09-15 red team, A4b and A11 | personal data in the *schema*: an enum label is a DDL string literal and neither the plan-time pass nor the catalog pass read `pg_enum`, so an address and a phone number crossed under exit 0 |
| `017-domain-default-holds-an-email-address.sql` | the 2026-09-15 red team, A12 | the 2026-09-09 finding 5 mechanism one catalog table to the left: a `DOMAIN`'s `DEFAULT` lives in `pg_type.typdefault` and was read by nothing |
| `018-plain-ssn-in-an-unrecognised-column-name.sql` | the 2026-09-15 red team round 2, R2-02/A6 | **a leak with no obfuscation at all**: a plain hyphenated US SSN in a column called `code` — a name no rule pack pattern matches — was reported "no name or value signal" and crossed under exit 0, because national_id had no value validator on the row path |
| `019-national-id-text-array-carrier.sql` | the 2026-09-15 red team round 2, R2-03/A7 | the same missing validator reached through the array carrier: a `text[]` of UK NI numbers reached the target verbatim, split correctly and recognised by nothing |
| `020-ssn-stored-as-bigint.sql` | the 2026-09-15 red team round 2, R2-04/A9b | **the numeric-family half, closed as a refusal and not a mask**: a bigint column can hold no hyphen and drops a leading zero, so a dashed SSN with a leading-zero area renders as an eight-digit number and even a registered national_id validator never saw the same number twice; internal/verify's digits-family entry catches it and refuses at exit 9, `verify.refused.second_net`, since internal/classify's own narrower validator cannot mask what it cannot see. Carries a corroborating `email` column since 023 (below); its own header explains why its five values changed at the same time |
| `021-ordinary-numeric-columns-clear-the-ssn-ratio.sql` | the T-0187 review round | **the other side of 020**: the digits-family entry 020 needed has no check digit, only the SSA's own exclusion ranges, so an ordinary surrogate bigint id column, an ordinary non-key dense business-number column and an ordinary YYYYMMDD date column all cleared its ratio and refused a run holding no personal data at all; fixed by excluding a value that is also a real calendar date directly and by exempting a column whose own values pack into a dense numeric range, read from the values rather than from internal/classify's decision |
| `022-national-id-in-a-surrogate-key-column.sql` | the review round that followed the T-0187 round | **the cost of 021's first-draft fix**: gating the key exemption on internal/classify's own surrogate-key decision meant a primary key of real, non-dense SSNs — a shape classify's own signals find nothing in — was exempted along with the ordinary keys 021 pins, and crossed into the target verbatim; the values-based fix in 021 refuses it because the column is not dense, whatever classify decided about it being a key. Carries a corroborating `email` column since 023 (below) |
| `023-sparse-fixed-prefix-reference-block-is-not-national-id.sql` | the review round that followed 022's | **the cost of 021's fix, the other side of the ratio**: a sparse column with a fixed leading prefix is neither dense (021's own exemption) nor a date (021's own exclusion), so an ordinary account-number or invoice-number column still cleared the ratio at essentially 1.0 and refused a run holding no personal data at all; fixed by requiring corroboration — a rules.yml national_id name-pattern hit on the column, or a certain-or-likely personal column in the same table — before the ratio is asked at all, which is also why 020 and 022 each gained a corroborating column of their own |
| `024-multilingual-name-in-an-unrecognised-column.sql` | the 2026-09-15 red team round 2, R2-05/A10, and T-0188 | a person's full name in a language `internal/textsig/names.txt` did not carry, in a column called `label` no rule matches, in a table with no other personal column — reported "no name or value signal" and crossed verbatim; fixed by sourcing given/family-name stock for twenty languages from Wikidata (CC0), documented in `THIRD_PARTY_NOTICES.md`. Ten rows, one per newly-sourced language (German, French, Spanish, Portuguese, Turkish, Hindi romanised, Arabic romanised, Japanese romanised, Korean romanised, Chinese pinyin), each verified false against a pre-T-0188 checkout and true after. A10's own Khmer/Lao/Amharic names stay an open residual — tracker T-0197 — because Wikidata's CC0 coverage for them is three, twelve and thirteen items total, nowhere near usable |
| `025-national-format-phone-region-kontaktnr.sql` | the 2026-09-15 red team round 3, attack:1:r3 | **a leak with no obfuscation needed for half of it**: a real UK phone number, dictated in words in one column and written plainly with spaces and brackets in another (`kontaktnr`, a name no rule pack pattern matches), crossed verbatim under exit 0 because every phone validator parsed under a fixed international-only region hint; fixed by `--phone-region REGION` (T-0221), which parses a second candidate under the configured region on the same strong footing the international entry has, on both nets |
| `026-ten-digit-account-number-is-not-a-guessed-phone.sql` | the T-0221 review round (not a torture-schema reduction — see this file's own prose above) | the false-positive control T-0221's corroboration gate needs: an ordinary ten-digit account number, in a character column with no name or neighbour signal, must stay unmasked when no `--phone-region` is configured, because a short built-in list of guessed regions clears such a number by chance often enough that masking on the guess alone would cost a real column to no evidence at all; asserts against the emitted yml (`not-masked:`) rather than the target's rows, because a wrongly masked column here would still pass every rows-based check |
| `030-short-declared-length-beside-a-certain-column.sql` | the 2026-09-15 red team round 4, the native-script variant against A2b (T-0239) | **a leak in the rail A2b's own fix built**: `unknownColumnsBesideCertain` masks an unrecognised character column as `free_text` beside a `certain` personal column, but its own declared-length exclusion skipped a `varchar(12)` name column beside a real `email` column in the same table, on the argument that free_text's filler does not fit a short column — untrue of a non-unique column, since the rail already excludes the case (a unique index) where a narrow generator can be refused; fixed by lowering the floor from sixteen characters to two |
| `031-fk-child-code-column-beside-a-certain-column.sql` | the T-0239 fix-round review, corrected by T-0253 | **originally a half-loaded-target risk, not a leak**: the lowered floor above newly swept a validated foreign key's character-family child (an ISO-style currency code) into `free_text` beside a `certain` email column, while its parent's identical values stayed unmasked — the two ends of one join disagreeing, which `internal/plan` cannot catch and `internal/load` turns into an exit-8 `VALIDATE` failure after every row has moved; the fix-round review's own fix excluded a validated, non-virtual foreign key's character-family columns, both ends, from the rail's reach entirely, and the round-5 red team found that exclusion itself leaking on a schema with no ISO code in it (`035`) — now fixed by `fkPairs`, which raises both ends together under the same category, and because the parent is a four-row table under a unique index that `free_text`'s fixed word list cannot fill, `internal/plan`'s existing, unmodified unique-index domain check refuses the run at exit 12 rather than loading it; this file's header moved from both ends unmasked (`ok`) to that refusal, and since **T-0311** back to `ok` for a better reason: the child's four codes, each repeated, are an enumeration the sweep spares, so neither end is masked and `not-masked:`/`equal-masked:` pin both ends copied and in agreement |
| `035-native-script-fk-child-beside-a-certain-column.sql` | the 2026-09-17 round-5 red team, the classifier attacker's FK variant | **a leak `031`'s original fix could not see**: the same native-script names as `030`, made a validated foreign key child of a lookup table holding the same names, whose own table has no `certain` column for any other pass to key on — `031`'s blanket FK exclusion took both ends out of `unknownColumnsBesideCertain`'s reach, and nothing else in `internal/classify` ever reached either one, so both crossed into the target verbatim beside a real, correctly masked `email` column; fixed by `fkPairs`, which raises a column at either end of a validated foreign key together with every column connected to it, under the same category, instead of excluding the edge outright — and because the lookup table here is also a small table under a unique index, `internal/plan`'s existing unique-index domain check refuses the run at exit 12 naming both ends of the pair, exactly as `031` does |
| `036-chained-fk-native-script-names-three-tables-deep.sql` | the T-0253 review round that followed `035`'s own fix | **a leak `035`'s fix reopened one hop further out**: bounding `fkPairs` to cref's direct partners closed the medium finding that walking the whole component caused, but a raised partner that is itself the child end of a *further* validated foreign key had its own parent reached by nothing — `propagateKeys` only ever pushes a decision down from a masked parent, never up from an unmasked one — so a three-table chain of real names left the topmost table copied verbatim beside two masked ones, and the load would `VALIDATE` a foreign key between a masked child and an unmasked parent; fixed by walking `fkParents`, the child-to-parent half of `fkPartners`, transitively from every raised column, so a chain is brought into agreement as far up as it goes while the downward direction stays bounded to `propagateKeys` |
| `032-national-id-in-a-varchar-beside-a-masked-phone.sql` | the 2026-09-15 red team round 4, the A9b varchar(9) replay | **the numeric-family fix moved to a character column and the leak came back**: a dense, zero-padded national identifier stored as `varchar(9)`, beside `msisdn` masked as `phone` on its name alone with no value signal, crossed verbatim under exit 0 — nothing in `internal/verify/validators.go`'s character-family national_id entries recognised the shape at all, and the corroboration gate counted the table as holding no personal neighbour because a name match with nothing from the values lands below `TableHasLikelyPersonalColumn`'s `ConfLikely` floor; fixed by a masked-neighbour corroboration signal at `ConfPossible`, a character-family twin of the digits-family entry, and the dense-sequence exemption no longer outranking corroboration |
| `033-national-id-in-a-bigint-beside-a-name-matched-phone.sql` | the 2026-09-15 red team round 4, the A9b dense bigint replay | **the same corroboration gap, and the dense exemption's own blind spot**: a dense national identifier stored as `bigint`, beside the identical name-matched, value-unconfirmed `msisdn`, crossed verbatim under exit 0 because the same `ConfLikely` floor missed the neighbour and, even once it did not, `digitRange.dense()`'s own exemption fired before corroboration was ever asked — the one combination `020`, `022` and `023` do not test together |
| `034-dense-business-number-beside-a-tsvector-is-not-corroborated.sql` | the T-0240 review round (2026-09-17), high finding | **a false refusal the T-0240 fix itself introduced**: `TableHasLikelyPersonalColumn` counts a neighbour at `ConfLikely` or above under any category, with no `identifiesAPerson` test, and a `tsvector` is `derived_text` at `ConfCertain` by its type alone — so a search-index column beside an ordinary, non-key, dense business-number column cancelled that column's own sequence exemption on the strength of a neighbour that is not personal data, and the run refused at exit 9 with nothing wrong in it; fixed by `corroboratedForSequence`, read only for the dense-sequence override, which answers with `NameMatchedNationalID` and `TableHasMaskedPersonalColumn` alone and never `TableHasLikelyPersonalColumn` — `requiresCorroboration`'s own three-signal `corroborated` is unchanged, so `032` and `033` still refuse |
| `037-fk-pair-partner-carries-a-type-conflict.sql` | tracker T-0257, not a torture-schema reduction — see this file's own prose above | **the residual `035`/`036` left open, now a refusal instead of a copy**: a validated foreign key's parent already carries a decision ARCHITECTURE.md §4 forbids overriding (a `citext` `dob` column, name-matched to `person_date`, type-conflicted at `low`), so `fkPairs` refuses to raise either end rather than mask the child alone and leave the parent copied — before `internal/plan/fkpair.go`'s `checkFKPairRefusal` read that signal (`Decision.Refused`, `Decision.RefusedPartner`), both columns loaded copied verbatim under exit 0; now the run refuses at exit 12, naming both columns with `--unmask` for each |
| `038-person-name-role-from-column-name.sql` | tracker T-0287, not a torture-schema reduction — see this file's own prose above | **the shape a stranger sees in the README's own landing image**: `person_name`'s masker emitted the same "Given Family" pair for every column in the category, so a `first_name` column held two words and a `last_name` column held a stray surname; fixed by `mask.Role`, decided at classify time from the column's own name and carried on `Decision.Role` beside `Category` the way `UniqueIndex` reaches `mask.Constraints.Unique` — `first_name`/`last_name`/`full_name` here pin the three roles side by side. `not-copied:` cannot tell a correctly shaped fake from a differently wrong one, so the real end-to-end proof that a role reaches the masker is `internal/transform`'s own `TestPersonNameRoleReachesTheMasker`, added in the fix round that followed T-0287, which asserts the one/one/two word shape directly against `transformer.plan`'s output |
| `039-person-name-role-word-collides-with-real-names.sql` | the T-0287 fix-round review, inverted by ADR-015 and tracker T-0304 — see this file's own prose above | first a guard that the synthetic role lists held no real name, since a real name there refused a correct run at exit 9; since T-0304 the lists are the 2020 Census names and the file seeds real names, some of them on the list, to prove the residual scan explains the coincidences (`verify.residual.explained`) and the run loads at `expect: ok` |
| `040-person-name-list-coincidence-is-explained.sql` | ADR-015, tracker T-0302, not a torture-schema reduction — see this file's own prose above | a masked name equal to *another* row's real name in the same column was confirmed by the column probe and refused a correct run at exit 9; the residual scan now explains a hit inside the masker's own vocabulary by the count transform emitted and a row check by identity, and reports `verify.residual.explained` — here over a table with a primary key and a twin without one, both at `expect: ok` |
| `041-generated-full-name-over-masked-name-columns.sql` | ADR-015, tracker T-0304, not a torture-schema reduction — see this file's own prose above | a generated `full_name` over masked `first_name` and `last_name` holds a real given name and surname in every target row once the lists are real names, which the second net's dictionary rule would refuse at exit 9; it is skipped for a generated column over masked columns only, and the run loads at `expect: ok` |
| `043-enum-and-identifier-columns-beside-a-certain-email.sql` | dogfood session 1, tracker T-0311, not a torture-schema reduction — see this file's own prose above | **a copy that did not boot**: `unknownColumnsBesideCertain` swept a role, a state, a UI mode, an OS type, a log level, a timezone, a text uuid, a version, a hostname, a hex serial and an asset path into `free_text` beside a `certain` email column; an enumeration or an all-one-identifier-shape column is now spared and copied with a reason line saying why, while a native-script name column that is an enumeration by count alone is still swept and masked |

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

011's header says `ok`: T-0134 landed ARCHITECTURE.md §11.1's literal rule — a
masked column's `DEFAULT` has its literals masked through the column's own
masker — and T-0161 wired the run key it needs, `pipeline.PlanRequest.Key`,
ahead of the plan stage in `internal/core`, so the run this file drives now
reaches arm 1 rather than refusing at exit 13 under arm 2's last clause. The
`masked-default:` key above is what proves it ran rather than merely
stopped refusing. `internal/plan`'s
`TestAMaskedColumnsDefaultIsMaskedThroughItsOwnMasker` is the unit half.

010 is the one file here that did not come from `testdata/torture/`. Its
schema is the reduction the review itself made — `tokens(id PRIMARY KEY, token
TEXT UNIQUE)` and `items(id PRIMARY KEY, token TEXT REFERENCES tokens(token))`,
with rows enough to give the unique-index rule a row count — and its transcript
is `docs/reviews/2026-09-09/evidence/fk_masker.log`. The rule that fixes it is
in `internal/plan/equality.go` and in ARCHITECTURE.md §5's amendment of
2026-09-14: one masker per foreign-key-connected set of masked columns, the
widest any member needs, checked to fit every member.

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
