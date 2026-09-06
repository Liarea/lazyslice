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
  fictional `555-01XX` range or the region's reserved equivalent; IPs in the
  documentation ranges; URLs under `example.invalid`. Free-text filler takes
  its length from `h` within `[1, min(atttypmod, 4096)]`, so a 1,247-character
  bio and a two-word note are indistinguishable —
  `TestFreeTextLengthUncorrelated` asserts that for a fixed key.
- **The two exceptions are stated, not incidental**: `NULL` stays `NULL` and
  `''` stays `''`, and both are listed as false negatives in ARCHITECTURE.md §6
  item 6. Do not add a third.
- **Canonicalise before hashing, per category**: emails trimmed and
  case-folded, phones to E.164, names NFKC-normalised and case-folded, numeric
  identifiers decimal-normalised. `typeTag` names the canonical form, so a
  `bigint` and a `text` copy of one identifier hash alike.
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

## Never

Import anything under `internal/`; add a dependency outside the three ADR-006
names without amending that ADR; make a generator read the clock, a global
seed or the input's length; add a masker that can be selected by a file path
or an expression; let a masked value keep any part of the original; add a
`Domain()` that reports more than the generator can actually emit — the
planner's refusal is only as honest as that number.
