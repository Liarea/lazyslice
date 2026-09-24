# internal/textsig

The value-only half of ARCHITECTURE.md §4's validators, plus the embedded
name dictionary: a pure function over one string, and nothing else.
`internal/classify` scores its signals with these and `internal/verify`'s second
net re-runs them over the loaded target, so this is the one home for both
(tracker T-0055).

**Contract.** ARCHITECTURE.md §4 "Signals", the value half only. A validator is
`func(string) bool` — total, allocation-light, no context, no error. The
dictionary is `Dictionary() *Dict`, parsed once from the embedded `names.txt`
and shared; its set is unexported, so no caller can add a word to a dictionary
every other caller reads.

**Rules.**
- It is a leaf. It imports the standard library and
  `github.com/nyaruka/phonenumbers` and nothing from this repository at all —
  not `ref`, not `pipeline`. internal/CLAUDE.md's import graph allows a leaf
  under `internal/` that imports only `ref` and `pipeline` (that is what `ref`
  and `event` already are); this one is stricter than the rule permits, and it
  should stay that way. Nothing here needs a `ColumnRef` or a `Category`,
  because nothing here knows what a column is.
- **No rule pack lives here.** No name pattern, no category, no confidence, no
  threshold, no scoring. Those are `internal/classify`'s, they are not
  duplicated, and a validator here must never grow a second argument that
  smuggles one in. The dictionary is a word list, which is why it can be here at
  all; the *scoring* over it is the caller's, and the two callers score it
  differently on purpose (see below).
- A validator answers about one value and never about a column. Ratios,
  thresholds and minimum counts belong to the caller.
- A change here changes both callers, which is the point — and it changes what
  `internal/classify` masks, so it is a THREAT_MODEL.md T1 question and not a
  refactor.

**The two dictionary signals, and why there are four validators.** Each signal
has a wide form the classifier scores and a narrow form the second net scores,
because a `true` costs the two callers different things: `internal/classify`
masks a column, which costs a lookup table, and `internal/verify` refuses a
target that is already loaded, at exit 9, with `--unmask` the only way past it.

- **A name.** `Dict.LooksLikeName` is the classifier's: one to three dictionary
  words and nothing else, so a `first_name` column of one word per row is
  caught. `Dict.NameShape` is `internal/verify`'s: two or three dictionary
  words *and* a given name immediately followed by a surname. The middle form —
  "two or three dictionary words", with no given/surname pair — was tried first
  and is not enough, because the surname section holds about two hundred
  ordinary English words: `green lane`, `west hill` and `hunter green` are all
  pairs of dictionary surnames (tracker T-0055's review). That is why `names.txt`'s
  two sections are kept apart in `Dict` rather than merged into one set.
- **Prose.** `Dict.Prose` is the classifier's: six words or more with a
  dictionary word inside (`Dict.ContainsName`). Its word floor excludes short
  values and **nothing more** — an earlier version of this file and of
  `dict.go` claimed the floor also kept a single dictionary word out, and that
  was wrong: "The supplier may terminate this agreement on thirty days notice."
  is prose with a name in it, because *may* is a surname. `Dict.ProseName` is
  `internal/verify`'s: the same word floor with the `NameShape` pair required
  somewhere inside the string, a sentence boundary breaking the adjacency.

The rule verify scores them under is named in internal/verify/CLAUDE.md, "the
dictionary rule". Both narrow forms fail by *missing* a name — a name written
surname-first, a name the dictionary does not carry — which is the direction a
refusal after the target is loaded has to fail in, and neither narrows what the
classifier masks.

**A URL is not a credential** (`ValidURL`, tracker T-0100). `LooksSecret`
matched any 16-to-512-character value with two character classes, Shannon
entropy at or above 3.2, no space and no `@`, and a URL clears every one of
them: mastodon's `accounts.uri` and `statuses.uri` were `credential` on 100% of
their rows, masked to the fixed literal, and refused at plan under the unique
index `accounts.uri` carries. `LooksSecret` now excludes a value that parses as
a URL with **both** a scheme and a host — an opaque scheme (`mailto:a@b`) and a
scheme-relative reference (`//host/path`) are not URLs to it.

The exclusion is only half of the change and must never ship alone: a URL that
carries a username is personal data, so dropping it out of `credential` with
nothing to catch it would leave the column at `none` and copy it verbatim
(THREAT_MODEL.md T1). `internal/classify` runs `ValidURL` **ahead of**
`LooksSecret` in its ordered validator list, under `online_id`, whose generator
has a domain wide enough for a unique column. Do not narrow a validator here
without knowing which category catches what it drops.

**`internal/verify` is the second consumer and it has not been given the other
half — T-0122.** This package has two importers, and T-0100's paths reached only
one of them. `internal/verify/validators.go`'s second net reads `LooksSecret`
and has no `online_id` or URL entry anywhere in it, so a profile URI that
reaches the target unmasked used to fail the run at exit 9 as a residual
credential and is now seen by nothing there. The two packages score
independently — the net is THREAT_MODEL.md T1's control for the column a 200-row
sample under-represented — so this package's own narrowing is a narrowing of a
blocking control until T-0122 lands. That is the concrete form of the rule above:
**a validator narrowed here is narrowed for both nets, and both need an answer
in the same change.**

**The identifier shapes are not validators** (`identifier.go`, T-0311).
`HexDigest`, `SemanticVersion`, `HostnameShape` and `PathShape` (with
`ValidUUID`, which was already here) answer whether a value is a machine
identifier, and nothing masks a column because one matches: `internal/classify`
reads them only to *spare* a signal-less column from the neighbouring-column
sweep, and it owns the minimum count and the dictionary guard over the two
shapes made of words. `internal/verify` does not call them, so they narrow
nothing in the second net. Each errs towards refusing a value that could be
personal — a slashed date (with a month name or without) is not a path, nor
is a path with no application-path marker (a file extension, a leading `/`
with three segments, an IANA `Area/Location` zone), `first.last` is not a
hostname, an all-digit run is not a hex digest, and a version that also reads
as a dotted date with a four-digit year (`5.3.1985`) is not a version
(`DottedDate`, the T-0311 review) — and `TestIdentifierShapesRefusePersonalLookalikes`
holds those edges. `DottedDate` is a guard in the same sense: nothing masks
on it.

**Test.** `go test ./internal/textsig/...`. `textsig_test.go` holds T-0100's
rule in both directions — a URL is a URL and is not a secret, and the two secret
shapes stay secrets. `nationalid_test.go` (T-0187) is the twelve formats' own
suite: one positive and one negative check-rule vector per format
(`TestValidNationalIDTwelveFormats`); a set of fixed ordinary-shape vectors
against the whole union, `ValidNationalID`, including a sequential surrogate
id, a YYYYMMDD date and an order code, each confirmed by direct computation to
fail every checksum-only format a bare digit run of its length can reach
(`TestValidNationalIDRejectsOrdinaryShapes`); a generated-population *rate*
assertion against `ValidNationalIDStructured` — the function
`internal/verify/catalog.go` and `internal/verify/validators.go`'s strong
entry actually call for a single-occurrence refusal — over random digit runs
at NIR's own fifteen-digit length (bounded near its ~1-in-97 mod-97 rate) and
at 8/9/11/12 digits (must never match, no dash or letter present)
(`TestValidNationalIDStructuredPrecisionOverRandomDigitRuns`, T-0187 review
round finding 4: the version of `TestValidNationalIDRejectsOrdinaryShapes`
that shipped with T-0187 asserted a precision claim from one hand-picked
literal, "123456789", annotated "happens to fail every checksum" — true of
that string and untrue of 25.7% of random 9-digit strings, so a stub that
rejected only that literal would have passed); and the digits-only recovery
both ways (`TestValidNationalIDDigitsRecoversTheDroppedLeadingZero` —
`ValidNationalIDDigits` recovers a bigint's dropped leading zero,
`ValidNationalID` itself must not). That and `textsig_test.go` are the whole of
this package's own suite. The rest of its behaviour is pinned where the
decisions are made: the classifier's
precision and recall suite (`TestPagilaPrecisionAndRecall`,
`TestFiftyNamesFromThreeSchemas` in `internal/classify`, which is now 64 names
from four schemas and keeps its name) and the second net's thresholds in
`internal/verify`. A validator changed here that breaks either is supposed to
break it.

## The 2026-09-15 red team (`candidates.go`, `names.txt`)

Three changes landed here, and each one changes what both callers see.

**`Candidates(s) []string` and the four validators that read it.** A parse is
defeated by writing the same value a different way, and the red team wrote a
phone number as "plus four four, two oh, seven nine four six, oh nine five
eight", an address as `grace.hopper AT realcorp DOT example`, a national
insurance number with interleaved spaces and a card with `.` separators — every
one of them into the target verbatim, under exit 0, with both nets agreeing the
column was clean. `Candidates` yields the value as it stands **first**, then the
canonical de-obfuscations: the at/dot words in four languages, a spelled-out
digit run (with `double` and `treble`), a value written as short space-separated
groups, and the address inside an RFC 5322 display name. `ValidEmail`,
`ValidPhone`, `ValidLuhn` and `ValidIBAN` accept a value if **any** candidate
parses, through `anyCandidate`.

The precision argument is in the file and is the part to keep: a candidate is a
*new string offered to the same parser*, so nothing that failed before can pass
now except by a spelling this file produced on purpose — and each rule requires
the whole value to look like the thing. `spelledDigits` needs every field to be
a digit word, a literal digit run, a multiplier or a **short alphanumeric
group** (a substitution wherever "one" appears would make one English sentence
in ten a payment card); `collapseGroups` needs every field to be four
characters or fewer (without it, "Meet me at the dot com office on Tuesday"
normalises and collapses into something net/mail accepts as an address). The
other validators are untouched: an IP, a MAC, a UUID and a URL have no folk
spelling, and the dictionary- and entropy-backed ones are guesses a candidate
list would only multiply.

**`ValidIBAN` requires the two ISO 13616 check digits.** Characters three and
four of an IBAN are always numeric; without that, any fifteen-to-thirty-four
character run of letters clears mod-97 about one time in ninety-seven, which is
why five of pagila's film titles used to. The tightening is what made IBAN
precise enough to join `internal/plan`'s and `internal/verify`'s DDL-literal
strong set (THREAT_MODEL.md T1's 2026-09-15 amendment). It does **not** make
IBAN `strong` in the second net, which is a separate question about a ratio over
a column.

**The letter groups in `spelledDigits` are the T-REDFIX review's third
finding.** The first version required the *whole* value to be digit words, so
the brief's own A2 `ident` value — `AB nine eight seven six five four D`, a UK
National Insurance number with interleaved spaces and spelled digits, listed as
reaching the target verbatim — still validated as nothing, while the all-digit
spelling of the same number (`A B 9 8 7 6 5 4 D`) worked through
`collapseGroups`; `candidates_test.go` covered only the second, so a case the
brief named was standing in for by a test that could not fail on it. A field
that is four characters or fewer and alphanumeric is now written through into
the candidate in its own case, under three guards that keep prose out: at least
one digit actually *spelled*, at least `minMixedDigits` digits, and at least
`minSpelledDigits` alphanumeric characters overall. `TestCandidatesDoNotInventValues`
carries the prose cases that must stay negative, "Room 4 at the end of the hall"
among them — every field of it is short, and it has no spelled digit.

**`ValidNationalID`, and it was exactly two formats.** A US Social Security
number and a UK National Insurance number, both strict patterns with the issuing
authority's own exclusions. They exist for the DDL-literal passes, which run
only the validators that are a parse. A wider "looks like an identifier" rule
would refuse ordinary schemas, which is the failure mode that set of validators
is narrow to avoid.

## T-0187 (2026-09-15, round-2 red team): ten more formats, and the row path finally calls it

`ValidNationalID` was correct and answered `true` for every value the round-2
red team's R2-01 through R2-04 planted (docs/reviews/2026-09-15-redteam/round2-
still-leaking.json, attempts A2, A6, A7, A9b) — the bug was that nothing on the
row-scanning path ever called it at all. `internal/classify`'s ordered
validators list had no `CatNationalID` entry, and `internal/verify`'s second
net had none either, so a plain SSN in a column called `code`, a UK NI number
in a `text[]`, and an SSN stored as `bigint` all crossed into the target
verbatim under exit 0, reported "no name or value signal" — the row-path half
of THREAT_MODEL.md T1's national_id note never having existed, where the note
itself only ever described the two DDL-literal passes. Both entries are wired
now: `internal/classify/classify.go`'s `validators` list gains `CatNationalID`
at the same `strong` footing as email, and `internal/verify/validators.go`
gains two — a `text: true, strong: true` entry mirroring email's, and a
`digits: true` entry (below) mirroring Luhn's own text/digits split (T-0136).
**Owed:** THREAT_MODEL.md T1 still reads as if national_id were a DDL-only
control; correcting it is tracker **T-0193**, filed rather than edited here
because THREAT_MODEL.md is outside this task's paths.

`nationalid.go` is where the other ten formats live, and `ValidNationalID`
dispatches to all twelve: US SSN, UK NINO (both already here), Polish PESEL
(a weighted mod-10 checksum), Italian codice fiscale (the CIN check character,
mod 26 over the first fifteen), Dutch BSN (the "11-proef" weighted sum),
Spanish DNI and NIE (an eight-digit number, or a letter-prefixed one, mod 23
against a fixed letter table), French NIR (the thirteen-digit number mod 97),
Brazilian CPF (two weighted check digits), Canadian SIN (the same Luhn check
digit `ValidLuhn` uses, applied directly because Luhn's own twelve-digit floor
excludes a nine-digit SIN), Indian Aadhaar (the Verhoeff checksum, which
catches every single-digit substitution a mod-11 check would not) and
Australian TFN (a weighted mod-11 sum). Every one is a real check rule and
never a length guess — the same requirement `rules.yml`'s comment on the
national_id name pattern makes of the *name* half, applied to the value half
that was missing for every abbreviation the pattern already carried (`pesel`,
`codice_fiscale`, `bsn`, `dni`, `cpf`, `aadhaar`, `nir` were already there).
`allSameDigit` excludes a run of one repeated digit from every weighted-sum
format (PESEL, BSN, SIN, TFN, and CPF's own pre-existing guard): each of those
checksums is linear in the digits, so an all-zero run clears every one of them
by construction, which is precise about nothing.

**`ValidNationalIDDigits` is the A9b half, and it is deliberately not read by
`Candidates`.** A9b's own SSN, stored as `bigint`, never reached
`ValidNationalID` at all: a numeric column can hold no hyphen and silently
drops a leading zero, so `078-05-1001` renders as the eight-digit `78051001`,
which the dashed regexp does not match regardless of how many candidate
spellings `anyCandidate` offers it. `ValidNationalIDDigits` is a second,
separate function — zero-pad an eight-digit run to nine and apply the SSA's
own exclusions directly, then fall back to `validNationalID` for every other
digit length, which is what lets PESEL, BSN, SIN, TFN, CPF and Aadhaar (none
of which ever had a separator to lose) validate through the same function with
no special case. It is read only by `internal/classify`'s digits-family entry
and, since **T-0240**, both of `internal/verify`'s digits-family and
character-family entries (the character-family twin closes the round-4 red
team's own A9b replay of this shape stored as `varchar(9)` rather than
`bigint` — `internal/verify/CLAUDE.md`'s own T-0240 section has the account) —
never by `Candidates` and never by `ValidNationalID` itself: an SSN carries no
check digit at all, so a bare
nine-digit number is "SSN-shaped" about as often as a random nine-digit number
clears the exclusion ranges (nearly always), which is precise enough for a
*ratio* over a whole numeric column and would refuse an ordinary schema on one
occurrence if the DDL-literal passes' `ValidNationalID` ever gained it —
exactly the failure mode this package's narrow validators exist to avoid.
`TestValidNationalIDDigitsRecoversTheDroppedLeadingZero` pins both halves: the
digits function recovers the padding, and `ValidNationalID` itself does not.

**The review round that followed T-0187 found the eight-digit branch's own
check degenerates on a calendar date, and `looksLikePlausibleDate`
(nationalid.go) is the fix.** The zero-pad recovery above prepends a forced
`'0'` to the first two digits before running `validSSN`'s exclusion ranges,
which means the padded "area" field can never fall in the excluded 666/9xx
bands — the check degenerates to "the year's own two-digit suffix is not
`00`", a rate close to 1.0 over any realistic date range rather than the
~97% the digits-family entry's own comment (`internal/verify/validators.go`)
had assumed from averaging over 1950-2050. `looksLikePlausibleDate` is a
plain calendar check — year in a plausible range, month 01-12, day valid for
that month including leap years — and `ValidNationalIDDigits`'s eight-digit
branch skips its own padded SSA check for a value it accepts, falling back to
`validNationalID` for the six checksum-only formats that do not share SSN's
padding failure. It is a value-shape exclusion like the ones `Candidates`
already has (`spelledDigits`'s field-length guards, `collapseGroups`'s
four-character cap): a fact about the *string*, never a column, a category or
a threshold, so it stays inside this package's own contract.
`TestValidNationalIDDigitsExcludesPlausibleDates` pins it.

**`names.txt` is no longer English only.** ARCHITECTURE.md §14 deferred the
multilingual dictionaries to phase 5; the red team is phase 5, and it showed the
cost in one run. The file now carries the given/surname stock of twenty-one
languages beside the English list, under two rules stated in its own header and
held by `TestDictionaryKeepsItsPrecisionRules`: **nothing shorter than three
characters** (a two-letter value is an ISO code far more often than a person,
and `TwoLetterCode` is the signal for it), and **no name that is also an
ordinary English word** unless the list already carried it — which is why Park,
Can and Long are not there though each is a common surname somewhere.
`ContainsName` splits on `unicode.IsLetter` rather than `[a-zA-Z]` in the same
change, because an ASCII splitter cut "Bogusław" into two fragments in no
section of the file.

**`PrintableText` is here and is not a validator.** It says whether a rendered
value is text somebody wrote rather than bytes a program wrote, and it says
nothing about whether that text is personal data. It is here because both
callers need the same answer to the same question about the same value:
`internal/classify` asks it before running the validators over a `bytea`
column's samples and `internal/verify`'s second net asks it before running them
over a `bytea` column of the loaded target (THREAT_MODEL.md T1's A4a). A second
hand copy of it would be the drift this package exists to end.

**Never:** read a column name, a neighbour, a schema or a database; return
anything but a verdict about the string you were handed; export the dictionary's
set; add a category, a confidence or a threshold to this package.

## Owed elsewhere

- `internal/CLAUDE.md`'s import-graph rule and ARCHITECTURE.md §2's "Import
  graph" both name the packages one by one and do not yet name this one; §12's
  layout does not list it either. Neither file is in T-0055's paths — **tracker
  T-0086** carries the edit.
- The `textOf`/`mask.Canonical` reproduction, the type-family table and the
  identifier quoter are the *other* copies internal/verify/CLAUDE.md owes a
  shared home. T-0055 moved the validators and the dictionary only. If they land
  here later, they arrive with the same rule as above: a value shape, never a
  rule pack — and `mask.Canonical` reproduction may not live here at all if it
  needs `pipeline`, because this package's import list is a feature.
- THREAT_MODEL.md T1's national_id note (its 2026-09-15 amendment, around line
  47) still says the validator "joins the strong set used by both the
  plan-time and the catalog pass" — both DDL passes — which is now stale about
  the row path T-0187 wired it into (`internal/classify`'s validators list,
  `internal/verify`'s second net). THREAT_MODEL.md is not in T-0187's paths —
  **tracker T-0193** carries the edit.

## T-0221 (2026-09-16): `ValidPhoneRegion`, a second phone reading beside the fixed one

`ValidPhone` is unchanged and still reads `PhoneRegionHint` ("ZZ") — the
international-only reading, admitting a number only when it is already
written with a country code. `ValidPhoneRegion(s, region string) bool` is a
sibling, not a replacement: it reads every spelling `Candidates` yields
exactly as `ValidPhone` does (so a number dictated in words under the given
region is the number it spells, `spelledDigits` included), and an empty
`region` falls back to `PhoneRegionHint` so a caller need not special-case
"none configured" itself. It exists because `docs/reviews/2026-09-15-
redteam/round3-still-leaking.json`'s kontaktnr/contact finding is a plain
domestic phone number — `07911 123456`, dictated or not — and a
national-format reading needs a region to parse under at all, which the fixed
hint can never supply.

**Which region to try, whether a caller trusts one hit or asks for
corroboration first, and what the reasons output says about it, are
`internal/classify`'s and `internal/verify`'s questions and not this
package's** — the same rule `PhoneRegionHint`'s own comment already states
for the single fixed hint, extended to a caller-supplied one: this function
takes a region and answers about one value, nothing about a column, a name
pattern or a neighbouring column. `internal/classify/CLAUDE.md`'s T-0221
section has the two callers' own corroboration rules.

**`SupportedPhoneRegion(region string) bool` is a different question about
the same input, added in the review round that followed T-0221's first
landing.** `ValidPhoneRegion` answers "does this number parse under this
region", and an unrecognised region answers that question `false` for every
number — silently, with no way to tell a real region with no matches in a
sample apart from a region libphonenumber cannot even parse under at all.
That silence let a single-character typo on `--phone-region` (`"gb"`, or the
common but non-ISO `"UK"` — libphonenumber's own code for the United Kingdom
is `"GB"`) reach `internal/classify` and `internal/verify` unchecked, where
it silently disabled every control the flag exists to add (the finding is
recorded in `internal/classify/CLAUDE.md`'s own review-round section).
`SupportedPhoneRegion` answers the question a caller needs asked *before*
trusting a region string at all: is this exactly one of the codes
`phonenumbers.GetSupportedRegions()` serves, matched as that table is keyed —
upper case, no synonym resolution, so `"gb"` and `"UK"` both answer `false`
and only `"GB"` answers `true`. `cmd/lazyslice`'s `checkPhoneRegion` is its
one caller: `--phone-region` is normalised to upper case and refused at the
flag surface, exit 2, before a region string ever reaches either net.
