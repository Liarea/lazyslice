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
  a plausible-looking one. The one exception is a credential column *under a
  unique index*, which escalates to `credential_unique` and gets
  `lazyslice-invalid-` followed by thirteen base32 symbols of `h` — still not a
  credential any application would accept, still announcing the tool that wrote
  it, but wide enough to carry `d_required` (`gen_credential.go`, T-0098).
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

## Workspace and release (T-0285)

The repo root's `go.work` (`use ./ ./mask`) is how a change here reaches the
binary during development: edit `mask/`, and `go build`/`go test` in the root
module pick up this directory directly, with no `replace` directive and no
tag needed, because a workspace member always wins over the root module's own
`require github.com/Liarea/lazyslice/mask v0.x.y`. That requirement is what
everyone *without* this workspace resolves — `go install`, a CI job with
`GOWORK=off`, anyone who has cloned only the root module — so it has to be
moved forward by hand once a change here is meant to ship:

1. Edit and test the change here, in the workspace (`go test ./...` from
   `mask/`, or `make test` from the root, which walks both modules).
2. Tag this module on its own: `make tag TAG=mask/vX.Y.Z` (docs/RUNBOOK.md).
   A mask tag is not a version of the tool — `.goreleaser.yaml`'s
   `git.ignore_tags` keeps goreleaser from reading it as one — and carries no
   release run of its own to watch.
3. Bump the root `go.mod`'s `require github.com/Liarea/lazyslice/mask` to
   that tag and run `go mod tidy` **outside the workspace**
   (`GOWORK=off go mod tidy`, or just `go mod tidy` from a checkout with no
   `go.work`): inside the workspace, `go mod tidy` still resolves this
   directory locally and the tagged version's checksum can end up unwritten.
   Land this bump before, or in the same change as, the tool's own next tag —
   never after — so the tag that ships always requires a `mask` version the
   proxy can actually serve.

`make install-proof` (root `Makefile`) is the check that a local override can
never come back unnoticed: it builds a copy of the working tree with `mask/`
and `go.work` deleted, from a fresh module cache, so it fails the moment
`go.mod` stops naming a real, tagged `mask` version — `GOWORK=off` alone
proved nothing about a `replace` directive still sitting next to `go.mod`,
since a relative `replace ... => ./mask` resolves whether or not workspace
mode is on, and the target's own negative control now asserts that a
reintroduced `replace` fails the build. `install-proof` is scoped to the
`cmd/lazyslice` build; `.github/workflows/ci.yml`'s `install-proof` job also
runs the test suite once with `GOWORK=off` and no other change, as a cheaper
stand-in for what a release build would resolve — it does not reach
goreleaser's own build, which still compiles inside this tree with `go.work`
present, so a tag cut before this module's version is bumped in `go.mod` can
still ship a binary built against a different `mask` than `go install`
resolves for the same tag. Landing order (item 3, above) is what actually
prevents that today; making the release build itself GOWORK=off, and
checking a release tag's `go.mod` requirement against `mask/`'s own tag, is
tracked in the tracker (T-0290) and not yet done.

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
- **`Apply` has a post-condition, and it is not `out != in`** (`mask.go`,
  T-0191). Until the 2026-09-15 red team registered a masker that returns its
  input, "output never equals input" lived only in this module's per-generator
  unit tests, and `Apply` handed the source value back with `Masked: true`, so
  the caller fed the residual filter a digest for a cell nothing had masked
  (THREAT_MODEL.md T12). What the post-condition tests is the property the red
  team actually broke — **that the generator is a function of `h` and not of its
  input** — and a hit is `ErrPassthrough`, naming the category and the masker id
  and never the value.
  - **A single cell whose masked value equals its source is not a passthrough.**
    Every generator here ignores `in` except to read its shape, so at a domain
    of *d* the output coincides with the source once in *d* distinct values:
    `national_id` in a `varchar(4)` (d = 9,000) hits it about once in 9,000
    distinct ids, `person_date` on a `date` about once in 25,567. A bare
    comparison made that a hard stop at exit 7, deterministic under the key, so
    an ordinary birthdate or short-id column could not be snapshotted at all.
    `looksLikeItsInput` is therefore only the **trigger**: when it fires,
    `tracksItsInput` calls the generator a second time under the same `h` with a
    sentinel input (`probeValue` steps every ASCII letter and digit on by one,
    so an email stays an email and a JSON document keeps its structure), and
    only an output that follows *that* input too is refused. A generator that
    refuses the sentinel has not answered the question, and an unanswered
    question is not evidence: the cell passes and the residual scan keeps its
    role.
  - **A document with nothing in it to mask is exempt, and the module decides
    that itself.** `semi_structured` keeps structure and key names and replaces
    scalar leaves, so a document whose leaves are all empty objects, empty
    arrays or JSON nulls — `{"tags":[]}`, `{"prefs":{}}`, `{"a":{"b":{}}}` — is
    returned identical to its input, and so is the sentinel, whose keys
    `probeValue` merely steps by one letter. Both halves of the post-condition
    therefore fired on a cell that proves nothing about the generator, and
    `{"tags":[]}`-shaped rows are ordinary production data: the refusal was
    deterministic under the key, so an ordinary `jsonb` column — and with it the
    database — could not be snapshotted at all. `nothingToMask` parses the input
    and passes such a document before the verdict is reached. It is computed
    here and never asked of the masker, for the same reason `Domain()` is not
    asked; the exemption is for a document with no leaf, not for `jsonb`, and a
    masker that hands back `{"email":"…","tags":[]}` is refused like any other.
  - **`Domain()` is not asked, and neither is the closed-column exemption.**
    Both were self-declared by the object the guard exists to contain: a
    masker that returns its input and reports `Domain(c) == 1` was exempted by
    its own answer. The second call settles the same cases without trusting
    anyone — a constant generator (`fixed:`, `null`, `derived_text`, a collapsed
    `special_category`) and a closed column's `labelValue` both return the same
    value for the sentinel as for the row, which is exactly not following the
    input.
  - **Nothing on the per-cell path canonicalises.** `looksLikeItsInput` compares
    the output to the input and to the input's canonical form byte for byte and
    then over letters and digits only (`foldEqual`), which catches a masker that
    folds the case of an address, re-spaces or re-punctuates it and hands it
    back. Canonicalising the *output* instead would put the phonenumbers parser
    on every masked cell — measured at roughly 85% of the cost of masking a
    phone — for the sake of the narrower case where a generator returns the
    input in a form whose letters and digits differ from the source's (a local
    number handed back in E.164, a date in another layout). That case is a
    stated limit of this guard and the residual scan is what sees it.
  - The residual scan and `TestNoGeneratorReturnsItsInput` are unchanged: the
    post-condition is the module's own control, not a replacement for the
    pipeline's.
- **`Apply` recovers a panicking generator** (`mask.go`, T-0191). A masker that
  panics with the offending row in its message used to cross the module
  boundary with the value in it, and the redaction that caught it lived in the
  parent binary (`core.PanicSummary`) — which a program that imports only this
  module never runs (ADR-006). `maskCell` recovers and returns `ErrMaskerPanic`
  naming the category, the masker id and the panic value's *type*. `panicKind`
  is `core.PanicSummary`'s rule restated here because this module may not import
  `internal/`; it is blunter on purpose, since there is no flag here to offer.
- **`Apply` wraps a *generator's own* returned error the same way it wraps a
  panic** (`mask.go`, T-0223, round-3 replay R2-13; round-4 replay R3-1).
  Before this, `maskCell` returned a third-party masker's error straight out
  of the public module: a masker's message is free-form text it can write
  however it likes, and
  `'cannot mask "victim.canary@bigcorp.example": unsupported shape'` reached
  any caller of `Apply`, `mask/` being importable on its own under ADR-006 and
  its contract not able to rely on the binary's redaction (`core.PanicSummary`
  never runs there). `maskCell` wraps that free-form residue in
  `ErrMaskerFailed`, naming the category, the masker id and the returned
  error's *type* via `panicKind` — the same helper the panic path uses, and
  for the same reason: the original error is not kept. The first fix wrapped
  *every* error `Mask` returned, including this module's own documented,
  value-free sentinels and typed errors (`ErrNoRoom` foremost — every
  built-in generator's "the column is too short to hold a masked value"), so
  `errors.Is(err, ErrNoRoom)` on an `Apply` error went from true to false and
  the module's contract broke silently. `internal/transform/codes.go`'s
  `maskReason` names every other mask sentinel by hand so its exit-code text
  is the sentinel's own safe words rather than the generic "an error of type
  %T" fallback; `ErrMaskerFailed` itself is still not named there (T-0229),
  which only affects a *third-party* masker's own error, since `ErrNoRoom` and
  the module's other sentinels reach `maskReason` unwrapped as before.
- **`maskCell` never returns a masker's error object, matched sentinel or not**
  (`mask.go`, T-0238, round-4 replay: a new variant of finding 16, against the
  T-0223 fix itself). The fix above added `isModuleError`, gated on
  `errors.Is`/`errors.As`, and when it matched, `maskCell` returned the
  masker's own error value whole — so
  `fmt.Errorf("cannot fit %q: %w", in.Text, mask.ErrNoRoom)` bought the
  canary a pass straight through the redaction the same fix had just added,
  because declaring `ErrNoRoom` inside the wrapper was a test the masker
  controlled. A masker can go further still: `*NoRoomError` and `*DomainError`
  are exported struct types, so a masker can build or rewrap one itself and
  set `TypeTag` — a plain string — to anything it likes. `wrapMaskerError`
  replaced `isModuleError` and never hands back the object it matched against.
  It always constructs a new error: `%w` on the matched sentinel
  (`ErrMaskerFailed` when none of them match), the category, the masker id and
  the returned value's *type* via `panicKind` — never its text. A match on
  `*NoRoomError` or `*DomainError` rebuilds that same type instead of a plain
  wrap, so `errors.As` still reaches it, but from fields this call already
  trusts: `cat` and `id`, the arguments `maskCell` was called with, and for
  `NoRoomError`, `TypeTag`/`MaxLen` off the `Constraints` it was given — never
  off the fields on the masker's error. Nothing on this path may read a
  masker's error fields *or call a masker method*: `DomainError`'s four
  fields are recomputed from the caller's own `Constraints` alone —
  `ColumnDomain(c)`, `Required(c.Rows)`, `c.Rows` and `MaxRows(domain)` —
  never `Admissible(id, c)`. `Admissible` (`registry.go`) is
  `min(ColumnDomain(c), m.Domain(c))`: it calls back into the *registered*
  masker's own `Domain` method, and on this path that masker is the same
  object whose `Mask` just ran on the cell. A masker that stashes the cell's
  value in `Mask` and hands it back from `Domain` on its very next call
  smuggles it out through `Domain`/`MaxRows` on the error path with a real,
  registered id — no forged struct field needed (T-0238 round 2: the
  round-1 fix used `Admissible(id, c)` here and reopened exactly the leak it
  closed). No field of a masker's error is trusted, numeric or not, and no
  masker method runs on this path either: an int64 a masker controls (or
  computes on demand) carries a value as well as a string does; four of them
  are 32 bytes, enough to move a canary a chunk per refused row.
  `errors.Is`/`errors.As` still answer every sentinel and typed error this
  module documents. Test: a masker whose `Mask` returns
  `fmt.Errorf("cannot fit %q: %w", victim, ErrNoRoom)`, a second masker that
  returns `&NoRoomError{TypeTag: victim}` directly, a third that returns
  `&DomainError{Domain, Required, Rows, MaxRows}` set to canary integers, and
  a fourth — *registered* under a real id and driven through the public
  `Apply`, not a helper that skips registration — whose `Mask` stashes the
  cell's value and returns a zero-valued `&DomainError{}`, and whose `Domain`
  returns a canary integer on the next call: all four assert
  `errors.Is`/`errors.As` reaches the module's own rebuilt error, the
  canary's absence from `err.Error()`, and (the fourth) that `Domain`/
  `MaxRows` on the rebuilt error are the `Constraints`-derived values, not
  the masker's canary.
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
- **`derived_text` empties a derived column; it never fakes one** (`gen_derived.go`,
  T-0054). A `tsvector` is built by a trigger or a generated expression from
  other columns' text, so it carries the lexemes of a column that may itself be
  masked: copying one ships the words of a masked value in cleartext beside it
  (THREAT_MODEL.md T12), and a tsvector of invented lexemes is a search index
  that matches nothing — which is what the empty one already is, honestly. So
  `Mask` returns `''`, the empty tsvector, for every row, `Domain()` is 1, and
  `Domain()` is **0** for any other type tag, so a category the rule pack
  accepts only on `tsvector` refuses through `Pick`'s ordinary `*NoRoomError`
  rather than by emitting a value some other type could not hold.
  - **It is the one id `Small` exempts.** `Small` reports a *substitution* that
    frequency can undo — a mapping onto a domain narrow enough that the
    commonest fake is the commonest original. Emptying is not a substitution:
    no ordering, frequency or partition of the source survives it, and listing a
    column with nothing left in it under "what the green tick does not prove"
    trains the reader to skim the list that matters. The other domain-1
    generators (`fixed:`, `null`) are **not** exempt, because they can land on a
    column the reader would want reported.
- **`writableTags` is derived from `internal/classify/rules.yml`, not
  independent of it** (`writable.go`, T-0054). `rules.yml`'s `accepts:` lists
  are the source; this table is the copy, and it is a copy on purpose: the plan
  check has to ask `mask` — where the generators are — whether a generator's
  output fits a column, and this module cannot import the rule pack.
  `TestRulePackAgreesWithMaskAboutTypes` (in `internal/classify`) walks the two
  and fails on a disagreement. What that test does **not** cover: the other four
  places a new category has to be added (`mask/register.go`,
  `internal/pipeline/classify.go`'s `Category` list,
  `internal/transform/writeback_test.go`'s `everyCategory`, and
  `internal/classify/classify_test.go`'s `all`), and whether a family listed
  here is one the generator can *really* emit into — only
  `TestTransformNeverRefusesWhatThePlanCheckAdmits` exercises that, and only in
  one direction. The single-declaration fix is to carry the compiled
  `accepts:` map on `pipeline.Classification` so `internal/plan` reads the rule
  pack itself; that needs `internal/pipeline`, which T-0054's paths did not
  include, and it would delete `writableTags` and `WritableTypes`.
  - **`special_category` is the one row that is deliberately narrower than the
    rule pack.** The pack spells its `accepts:` `["*"]`, because §4 scores it
    `certain` by name alone and silencing it by type would copy an `hiv_status`
    column in cleartext. The generator is narrower: `Mask` calls `generic`,
    whose switch covers boolean, the numeric families, date, timestamp, uuid,
    bytea and the document families and falls through to the free-text filler
    for `time`, `interval`, `inet`, `cidr`, `macaddr` and `tsvector` — and
    filler is none of those. So `health_check_ip inet` ("health" matches the
    pattern) and `medication_time time` are refused at plan with exit 12 rather
    than written and failing in the loader mid-run (THREAT_MODEL.md T8). The
    labelled-column bypass still applies first, so `testdata/README.md` trap 24
    — `special_category` on an enum — is unaffected.
- **The three quoting helpers are exported** (`StripTypmod`, `UnquoteType`,
  `BareTypeName`, `typetag.go`). Their only job is to produce `TypeTag`'s
  argument, and each caller having its own copy is each caller asking about a
  slightly different column. `internal/classify`, `internal/plan` and
  `internal/transform` all read these; `internal/verify` still has a copy and
  is owed the same change.
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
- **A unique credential column escalates rather than being refused**
  (`gen_credential.go`, T-0098). `CatCredential` shipped with only the fixed
  literal, whose `Domain()` is 1, so every column under a unique index that
  classified as a credential was refused at plan with exit 12 at *every* row
  count — `MaxRows(1)` is zero, so the refusal could not even name a `--take`
  that would work, and the operator's only escapes were `--unmask`, which copies
  the credential into the target verbatim, and `mapping_file:`. Of the
  thirty-seven `--unmask` flags the ten schemas carry in
  `internal/invariants/torture_catalogue_test.go`, **eighteen** are tagged
  `(T-0098)` and are that one defect — six in supabase-auth, eight in gitlab,
  two in mastodon, one each in calcom and discourse.
  `credential_unique` is the alternate, registered after the
  literal so `Pick` reaches it only for a unique column, and it is *not* a
  plausible token: the first eighteen characters are the constant
  `lazyslice-invalid-` and only the suffix varies. Thirteen base32 symbols are
  65 bits, which clears the 2⁶⁴ §5 asks of a generator a unique column escalates
  to; a narrower column shortens the suffix and `Domain()` shrinks with it, and
  a column with no room for the prefix and one symbol has `Domain() == 0` and
  returns `ErrNoRoom` like every other generator. Nothing is truncated.
  **`docs/TORTURE.md`'s counts were re-measured** (T-0112, and again in
  T-HARD-C's run): the eighteen flags are out of the catalogue, the split is
  **nineteen `--unmask`, seven `--skip-table`, one `--key`** over twenty-seven
  flags, and `docs/TORTURE.md`, ROADMAP.md's gate-5 line, the per-schema table
  and the flags-by-kind assertion in `internal/invariants/torture_test.go` all
  carry that. `make torture` no longer copies eighteen credential columns into a
  target in clear, and it exits 0.
  **`credential_unique` has end-to-end coverage now**, and it is worth stating
  precisely rather than generously. Of the eighteen columns whose flag went,
  **four** hold masked values in a target — `auth.refresh_tokens.token` 136/136,
  `auth.users.confirmation_token` 100/100, `auth.users.recovery_token` 100/100
  and `public.user_security_keys.credential_id` 100/100, every value prefixed
  `lazyslice-invalid-` with a distinct count equal to the row count. Two are the
  empty string `mask.Apply` passes through, ten are NULL in their fixture, and
  mastodon's two are in the run that refuses at exit 13 before the plan. Beside
  those, `testdata/regressions/004` and `007` reduce a unique credential column
  each and assert the same thing directly: T-0113 re-cut them from the exit-12
  refusal this generator removed to `expect: ok` plus a `unique-masked:` header
  the harness checks against the loaded target.
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
