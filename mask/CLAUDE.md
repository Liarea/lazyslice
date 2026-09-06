# mask/

The deterministic masking module: `github.com/Liarea/lazyslice/mask`, a nested
Go module with its own `go.mod`, its own tests and its own release tags
(`mask/v0.x`). ADR-006 keeps it separate so that a program which has never
heard of lazyslice can import it, its tests run without a database, and it
outlives the tool the way copycat outlived Snaplet. It depends on the standard
library, `golang.org/x/text` and `github.com/nyaruka/phonenumbers`, and on
nothing else — never on `internal/`, never on `pgx`.

## Contract

ARCHITECTURE.md §5 in full. The scheme is **part of this module's public
contract**, so it is fixed now rather than after the first release, and
changing any line of it is a major version of the module (ADR-006
"Consequences").

```
K        = 32 random bytes                              // Key; ./lazyslice.secret | $LAZYSLICE_SECRET | ephemeral
K_cat    = HKDF-SHA256(K, salt=nil, info="lazyslice/v1/"+category, 32)
h        = HMAC-SHA256(K_cat, Encode(typeTag, canonical(value)))
fake     = registry[id].Mask(h, value, constraints)

Encode(f1, ..., fn) = u32be(len(f1)) ‖ f1 ‖ ... ‖ u32be(len(fn)) ‖ fn
```

A masker is a pure function pair, registered by name at build time by
`mask.Register`:

```go
Mask(h [32]byte, in Value, c Constraints) (Value, error)
Domain(c Constraints) int64
```

- **`h` is the only source of variation.** Every choice a generator makes —
  which word from an embedded list, the local part, the digits, the length of
  a free-text filler — is derived from `h`. No global seed, no clock, no
  `math/rand`, no `gofakeit`, and never the input's length.
- **`Domain(c)` is the number of distinct outputs** the generator can emit
  under `c`. The planner compares it against the column's own admissible
  domain and refuses a unique column the generator cannot carry (exit 12);
  there is never a retry counter, here or anywhere.
- **`Encode` is length-prefixed** so `("email", "a@b.com")` and
  `("emai", "la@b.com")` hash differently, and so a column `b.c` in schema `a`
  never aliases column `c` in schema `a.b`. Those colliding pairs are test
  fixtures in this module, not a comment.

## Rules

- **Nothing survives a mask**: no prefix, no length, no first character, no
  real domain. Emails land in `example.com`/`.net`/`.org`; phones in the
  fictional `555-01XX` range (see "Phones are always North American" below);
  IPs in the documentation ranges; URLs under `example.invalid`. Free-text filler takes
  its length from `h` within `[1, min(atttypmod, 4096)]`, so a 1,247-character
  bio and a two-word note are indistinguishable —
  `TestFreeTextLengthUncorrelated` asserts that for a fixed key.
- **The two exceptions are stated, not incidental**: `NULL` stays `NULL` and
  `''` stays `''`, and both are listed as false negatives in ARCHITECTURE.md §6
  item 6. Do not add a third.
- **Canonicalise before hashing, per category**: emails trimmed and
  case-folded, phones to E.164, names NFKC-normalised and case-folded, numeric
  identifiers decimal-normalised. `typeTag` names the canonical form, so a
  `bigint` and a `text` copy of one identifier hash alike — and `Canonical`
  **returns** that tag rather than reading it off the column (see "Decisions
  made during implementation").
- **Preserve what the application checks**: `varchar(n)` length, parseable
  `CHECK` shapes, enum membership — a masked enum emits a member label, never
  an arbitrary string, which would fail the load with `22P02`. libphonenumber
  validity is preserved except under `phone_unique`, which says so.
- **Credentials become the fixed unusable value `$lazyslice$invalid`**, never
  a plausible-looking one.
- **Nothing is loaded at runtime** (ADR-006): no `.so`, no subprocess, no
  expression language, no `--masker-exec`, no file path or plugin accepted as
  a masker. Extension is a pull request to this module or a build calling
  `mask.Register`.
- **This module never touches a database, a file or the network.** It takes
  values and returns values; the caller (`internal/transform`) does the I/O
  and owns the residual filter. A `Value` holds production data, so nothing
  here serialises, logs or errors with one.
- **Determinism scope**: same value → same fake within a run and across runs
  given the same `K`, the same category and the same lazyslice version. The
  embedded word lists are part of the mapping, because `K_cat` is derived from
  the category — changing a list changes every masked value in that category,
  which is a release note and a `classification changed` line, not a tidy-up.

## Test

`cd mask && go test -race ./...`, or `make test`, which walks both modules.
No database, no container, no build tag: if a test here needs one, it belongs
in `internal/transform`.

Every generator needs the §5 test vectors that make it a contract rather than
a guess: the colliding-pair encoding fixtures, `Domain()` against its own
output space, and the length-uncorrelated free-text check.

`corpus_test.go` is one row per registered masker, and
`TestCorpusCoversTheRegistry` fails on a generator with no row: the
determinism, format, pass-through and NULL/`''` tests all walk it, so a masker
added without a row is a masker nothing asserts anything about.
`TestDomainMatchesWhatTheGeneratorEmits` counts the distinct outputs of every
narrow generator and fails if `Domain()` is above **or noticeably below** what
it emits, which is the only way a fictional number is caught.
`TestSameKeySameFakeAcrossProcesses` re-executes this test binary, so "same key
same fake" is a claim about two processes and not about one loop.

## Decisions made during implementation

ARCHITECTURE.md §5 is silent on each of these; the choice is here so that the
next change to this module argues with a decision rather than rediscovering it.

- **`Canonical` returns the type tag** — the resolution of the scaffold review
  note (tracker T-0020). `Canonical(cat, in, c)` hands back
  `(canonical, typeTag)`, where `typeTag` names the *canonical form*
  (`e164`, `decimal`, `ip`, `uuid`, …). `Constraints.TypeTag` is the column's
  PostgreSQL **type family**, in the names `internal/classify/types.go` uses,
  and it only shapes the output — a phone in a `bigint` column emits digits,
  one in `text` emits E.164. They cannot be the same field: if the digest's tag
  were the column's type, a `bigint` and a `text` copy of one identifier would
  hash apart and §5's sentence would be false.
- **`Constraints.Distinct` was added to the scaffold's struct.** §5's
  small-domain rule (`d < 2 × distinct(samples)`) and the `special_category`
  collapse are both functions of the distinct sampled values, and a masker that
  is a pure function of `(h, value, constraints)` has nowhere else to read it.
- **An unknown `Distinct` is not "not small".** `Distinct` is 0 whenever nobody
  counted, and §4 makes that path normal (a special category recognised by name
  alone, a partitioned table with no leaves). A rule that switched itself off
  at that zero value would be a control defaulting to *off*: an `hiv_status`
  enum would be substituted 1:1 over its own two labels, roughly half the rows
  would carry their true value, and `small_domain:` would not mention the
  column either (THREAT_MODEL.md T1, T12). So `smallDomain(d, c)` is
  `d < 2 × Distinct` when there are samples and, when there are none, true for
  a closed column (an enum or a `CHECK` list — substitution inside a fixed set
  of labels preserves the frequencies that recover it) or a domain at or below
  `smallDomainCeiling` (64; §5's worked example is eight diagnoses). `Small`
  and `specialCollapses` are both that one function, so a collapsed column is
  always a reported one.
- **Phones are always North American.** Every masked number is
  `+1 <area> 555 01NN`, valid per libphonenumber and inside the range reserved
  for fiction. §5's other reading — the reserved range of the *detected* region
  — is not shipped: the only non-NANP block this module could verify against
  libphonenumber is the Ofcom drama range, which is a thousand numbers wide and
  whose mobile half libphonenumber rejects outright, and inventing per-region
  ranges we cannot check would put numbers into a column the application
  validates. The region does not survive a mask, which is the safer half of §5
  anyway. `Constraints.Region` is still used, for **canonicalisation**.
- **The 447 NANP area codes are embedded, not probed.** Probing
  libphonenumber at start-up would make every masked phone number depend on the
  library version, so a dependency bump would silently remap a column.
  `TestNANPAreaCodesAreStillValid` checks the list against the library instead,
  so a bump that invalidates one is loud. `phone`'s domain is therefore 44,700
  rather than §5's approximate 8 × 10⁴; the refusal arithmetic is unchanged and
  §5's worked example (a unique `varchar(15)` phone column) still resolves to
  `phone_unique` or a refusal, never to a collision at load.
- **`Domain()` is a lower bound where it cannot be exact**, never an upper one.
  `free_text` counts one output per admissible length plus the words that open
  it; `semi_structured` reports one string leaf's worth. Under-reporting
  refuses more columns than strictly necessary. Over-reporting is a collision
  at load under a green tick, so it is the one direction that is forbidden.
- **A generator that branches on the value reports the narrowest branch the
  column admits**, because `Domain` is handed the column and not the value.
  `network_id` reports 256 rather than 768 for a `text`, `varchar` or `citext`
  column, since a MAC-shaped value in one takes the MAC branch (an `inet` or
  `cidr` column cannot hold a MAC, so it keeps 768 and `netShapeFor` does not
  offer the MAC branch there). `online_id` takes the minimum over the handle,
  URL and UUID branches: in a `varchar(25)` the URL branch has one base32
  symbol left and emits 32 values while the handle branch emits billions, so
  reporting the handle's figure would let a unique column through and collide
  at load.
- **Every generator honours a closed column first.** An enum or a `CHECK` value
  list bounds the column whatever category it was classified under, so
  `labelDomain` opens `Domain` and `labelValue` opens `Mask` in every
  generator: a street address in a column constrained to `'GB'` and `'US'`
  fails the load with 23514, and an arbitrary string in an enum with 22P02.
  The three that do not call them already answer for a closed column —
  `special_category` (through `generic` and `collapsed`), `null` (NULL, or
  `zeroValue`, which is the first label) and `fixed` (whose literal is the
  point, and which a closed credential column would break; that column shape
  is not one this module has seen).
- **A numeric column's width is a digit count, not a length.** `phone` asks
  `digitBudget` for its eleven digits and drops to the ten-digit national shape
  or to `Domain() == 0` when they do not fit, and `phone_unique` fits its
  twelve digits and the leading `1` the same way. An `integer` phone column
  holds nine digits, so the default generator refuses it at plan rather than
  emitting `12315550139` and failing the load with 22003.
- **A column too small for any masked value has `Domain() == 0` and `Mask`
  returns `ErrNoRoom`.** Nothing is ever truncated into a narrow column,
  because a truncated value keeps a prefix. §5's domain check runs on every
  masked column, not only the unique ones, so the refusal happens at plan.
  `Pick` refuses it with a `*NoRoomError` (which unwraps to `ErrNoRoom`) and
  not with a `*DomainError`: there is no `d_required` and no largest `--take`
  to print when no row count would make the column fit, and §5 asks the exit-12
  refusal to print all three numbers truthfully.
- **A unique column with no row count is refused, not accepted.** `n` is the
  whole of `d_required`, so `Rows <= 0` under a unique index is
  `ErrRowCountUnknown` rather than a comparison against `Required(0) == 0`,
  which every generator passes — including `fixed:$lazyslice$invalid`, whose
  domain is 1. Failing open there is the load-time unique violation whose
  `PgError.Detail` we drop (THREAT_MODEL.md T4).
- **`Apply` has no runtime "output must differ from input" guard.** Two
  generators are *meant* to be able to return the input — a masked enum emits a
  member label, and a collapsed special category emits the first one — so a
  global guard would refuse the correct answer. THREAT_MODEL.md T12 names this
  as a **test** (`TestNoGeneratorReturnsItsInput`,
  `TestUnexpectedTypesDoNotPassThrough`) and a runtime **residual scan**, and
  those are where it stays.
- **Arrays are the caller's loop.** §5 masks an array element-wise with `h`
  computed per element; `Value` has no array form, and dimensions and lower
  bounds are a pgx concern, so `internal/transform` maps `Apply` over the
  elements (ARCHITECTURE.md §12 gives it the walking). Adding an array shape to
  `Value` would put encoding in a module that has no driver.
- **JSON leaves default to free text here.** `semi_structured` keeps structure
  and key names and replaces every scalar leaf, but choosing a leaf's category
  from its key name needs the rule pack, which lives in `internal/classify`;
  §12 puts JSON leaf walking in `internal/transform` for that reason. This
  generator is the value-only floor beneath it. Object keys are walked in
  sorted order, never Go's map order, or the same document would draw different
  values in two runs.
- **hstore is emptied rather than walked.** No hstore parser here; an empty map
  is the one answer that cannot leak.
- **Two `CHECK` shapes are parsed**: a value list (`IN (...)`,
  `= ANY (ARRAY[...])`), which really bounds a domain, and an exact length
  (`char_length(col) = n`). Every other CHECK is ignored, which can make a load
  fail loudly — a worse first run, not a leak.
- **`address` picks its shape by column width, not by column name.** The rule
  pack's address pattern matches street, city and postcode columns alike and
  `Constraints` carries no name, so a column wide enough gets a street address
  and a narrow one gets an alphanumeric code, which is the shape a
  `postal_code varchar(10)` has.
- **`Key.String()` is the fingerprint**, so a `%v` in a log line or an error
  cannot print key material (THREAT_MODEL.md A4). `ParseKey` reads 64 hex
  characters, the form `./lazyslice.secret` and `$LAZYSLICE_SECRET` carry, and
  its error never quotes the input.
- **`Register(id, category, masker)`**, and order within a category is
  meaningful: the first registration is the category's default and the rest are
  the alternates `Pick` considers for a unique column. `Get` understands the
  one parametric id, `fixed:LITERAL`.

## Never

Import anything under `internal/`; add a dependency outside the three ADR-006
names without amending that ADR; make a generator read the clock, a global
seed or the input's length; add a masker that can be selected by a file path
or an expression; let a masked value keep any part of the original; add a
`Domain()` that reports more than the generator can actually emit — the
planner's refusal is only as honest as that number.
