# internal/transform

Applies each column's `Decision` to every batch between extract and load: mask
invocation, JSON-leaf walking, per-category canonicalisation, FK-decision
propagation, and the residual Bloom filter (`bloom.go`). The masking
*algorithms* (generators, `Domain()`, the HMAC scheme) live in the top-level
`mask` module (ADR-006); this package calls them, never reimplements them.

**Contract.** ARCHITECTURE.md §2 "extract, transform, load":
`Transformer.Transform(b RowBatch, *Classification, *mask.Key, Residual)
(RowBatch, error)`; `Residual` (`Add`/`MayContain`/`Cells`/`Bytes`) is
implemented in `bloom.go` per ARCHITECTURE.md §6.

The masker contract this package calls into is `mask`'s, not its own
(ARCHITECTURE.md §5, ADR-006), and it is fixed:

```go
Mask(h [32]byte, in mask.Value, c mask.Constraints) (mask.Value, error)
Domain(c mask.Constraints) int64
```

`h = HMAC-SHA256(K_cat, mask.Encode(typeTag, canonical(value)))` with
`K_cat = HKDF-SHA256(K, nil, "lazyslice/v1/"+category, 32)`. This package
derives nothing: it canonicalises per category, hands `h` and the column's
`Constraints` to the registered masker, and takes the `Value` back. `h` is the
only source of variation a generator gets — no clock, no global seed, no input
length — which is why `Transform` can be pure. `NULL` stays `NULL` and `''`
stays `''` (§5); a masked enum comes back a valid label; a `Domain()` too
small for a unique column was already refused at plan, so this package never
sees that case and must not paper over it with a retry.

**Rules.**
- `Transform` is pure apart from `Residual.Add` — same key, same
  classification, same input, same output, every time. This is what invariant
  I3 (two runs, byte-identical targets) rests on; do not introduce
  non-determinism (a clock read, a random source outside `mask.Key`) here.
- Every masked cell, and every masked JSON leaf keyed by its path, must be
  added to the residual filter (ARCHITECTURE.md §6 item 1) — a code path that
  masks a value without calling `Residual.Add` defeats THREAT_MODEL.md T12's
  only control.
- JSON leaf rules are exact (ARCHITECTURE.md §4 "Free text and JSON"): string
  leaves get the `free_text` masker (see the decision below), numbers/booleans
  are re-derived from `h`, `null` stays `null`, structure and key names survive,
  wildly varying keys or an audit/log/history/event table's `jsonb` collapses to
  `{}`.
- **Every `Residual.Add` follows the contract below**, and a leaf's goes through
  `addLeaf`. `internal/verify` is a different stage package that cannot import
  this one, so a convention only the call sites know is a control that fails
  open: verify canonicalises the *target's* value the same way and tests the
  filter, and bytes it cannot reproduce are a residual scan that reports no hit
  for a leaf that shipped in cleartext (THREAT_MODEL.md T12, PII-4/PII-5).
- **A value the target will also hold is never added.** `mask.Apply` records
  nothing for a `NULL` or an empty value; the same rule extends to a document
  that already *is* the collapse output and to a boolean leaf, whose two-valued
  domain leaves it equal to the source under half the run keys. An entry the
  target's own value matches is a residual hit on a run that masked correctly,
  which §6 item 3 turns into exit 9.
- No masker may be loaded at runtime — only the compiled registry in `mask`
  (ADR-006, THREAT_MODEL.md T7); never accept a file path, plugin or
  expression as a masker.
- A masker that refuses a value is a refusal, never a copy: `Transform` returns
  a `*Refusal` and the run stops. Copying cleartext through under a column the
  report calls masked is exactly T12.

**Decisions made during implementation.**
- **This package now imports `internal/textsig`** (`json.go`'s `keyCategory`,
  T-0137). ARCHITECTURE.md §2 and §12 document `textsig` as shared by
  `classify` and `verify` only ("so classify and verify share one
  implementation without importing each other"); this is a third importer, not
  named there. It is not a new edge in the *cycle* `TestImportGraph` polices —
  `textsig` imports nothing from this repository, so it stays a leaf and this
  package already imports `pipeline` and `ref`, its documented siblings — but
  the prose undersells it now, and ARCHITECTURE.md §2/§12 are owed the third
  name. Reported here rather than silently widened.
- **`New` takes the `*pipeline.Schema`.** §2's `Transform` is given the
  classification, and a `Decision` carries a category and a masker id but not
  the column's type, length, enum labels, `CHECK` constraints or nullability —
  which are exactly `mask.Constraints` and exactly what keeps a masked value
  inside the shape the application checks (§5). `Verify` is given the schema
  for the same reason; §2's `Transformer` comment is owed the same line.
- **The type-name table moved into `mask`** (`mask.TypeTag`, `mask.MaxLen`,
  T-0054). An earlier version of this file recorded the table here as "a third
  copy of one vocabulary" and said the fix was a shared home rather than a
  fourth copy; `internal/plan`'s write-back check needed the same reduction and
  is what forced it. It is not a tidy-up: the plan check asks whether a column
  can be written into and this package builds the `Constraints` the generator
  reads, and if the two named different families the check would be about a
  different column than the one masked. The family *names* are still spelled out
  here, because `isDocument` and the shape switch branch on three of them and
  `mask`'s constants are unexported; `TestFamilyNamesMatchMask` walks the two
  vocabularies against each other. What is still written twice is the domain and
  enum resolution, which needs the schema and so cannot live in `mask`;
  `internal/classify/types.go` keeps its own table for the reason its CLAUDE.md
  gives.
- **A `tsvector` is emptied, never copied** (`derived_text`, T-0054). It is a
  family this package now names (`famTSVector`), and the write path is the
  ordinary one: `mask`'s `derived_text` generator returns the empty string, the
  column's value arrives as the text form pgx gives an unknown OID, `coerce`
  keeps it as text, and the loader writes the empty tsvector. There is nothing
  special in this package for it, which is the point — the decision is
  `internal/classify`'s (a tsvector holds the lexemes of text that may itself be
  masked) and the value is `mask`'s.
- **The exit-7 refusal is now a backstop, not a discovery**
  (`TestTransformNeverRefusesWhatThePlanCheckAdmits`, T-0054). This package can
  only notice a type mismatch per value: it masks, tries to parse the masker's
  text back into the Go kind the column arrived as, fails, and stops the run
  after rows have moved. That is how pagila's `credential`-on-a-timestamp
  reached anyone. `internal/plan` now refuses such a column at exit 12 before a
  key is fetched, and the test masks a probe column under every category on
  every type family and asserts one direction: anything this package refuses is
  something `mask.Writable` — which is what the plan check reads — had already
  said no to. The other direction is deliberately not asserted; a combination
  the plan refuses may still survive here, and a refusal already made is not a
  bug in this package. Do not delete the refusal: a backstop that has never
  fired is what it is for.
- **Every JSON string leaf is masked as `free_text`** (`json.go`,
  `leafCategory`). §4 sends a leaf's key name "through the name rules", which are
  the embedded rule pack in `internal/classify`; the same import ban applies, and
  a hand-written table of about 60 name→category entries stood in for it here
  until this review. It was two things at once and wrong as both: a **second
  rule pack** with its own vocabulary, its own matching (exact lower-cased
  lookup against `rules.yml`'s priority-ordered regexes) and no place in
  `Classification.Fingerprint`, so editing it changed masked output with none of
  the "classification changed" warning `rules.yml` carries; and an **unexported**
  one, so `internal/verify` could not compute the canonical bytes of a leaf it
  classified — the residual control failing open, which is the T12 failure it
  exists to catch. §4's own default for an unrecognised key is `free_text`, so
  that is now every string leaf. What is lost is the *shape* of the fake — an
  `email` leaf becomes free text rather than an address — and nothing about
  whether a leaf is masked. This is a deviation from §4's "through the name
  rules" and is reported as one; §14's one-level JSON key collection in
  `internal/classify` is what should end it, by putting the category on the
  decision where verify can see it.
- **"Wildly varying keys" is not implemented here.** §4 names two triggers for
  collapsing a document to `{}`; the table-name one (`audit|log|history|event`,
  plus plurals, matched on underscore-separated words) is deterministic per
  column and is implemented. "Wildly varying keys" is a property across a
  column's rows, which a pure per-batch `Transform` cannot see and a classifier
  with 200 samples can: it belongs in `internal/classify`, which would set
  `Decision.Masker` to `fixed:{}` — a masker id `mask.Get` already understands.
  Reported as an open task.
- **`json` is collapsed in a log-shaped table as well as `jsonb`.** §4 says
  `jsonb`; collapsing is strictly more masking than walking, and CLAUDE.md's
  rule is to mask more when in doubt.
- **`hstore` is replaced with the empty map**, never walked: nothing here parses
  hstore, and `{}` is not an empty hstore, so the value is `''`. That is the
  answer `mask`'s own `semi_structured` generator gives.
- **A masked cell comes back as the Go kind it arrived as** (`value.go`). A
  masker's output is always text or bytes, and the loader has to encode it back
  into the same column, so a masked `bigint` is parsed back to an `int64`, a
  `uuid` to `[16]byte`, and anything implementing `sql.Scanner`
  (`pgtype.Numeric` and friends) through one reflective path — which is why this
  package does not import pgtype. A kind with no parse back keeps the text,
  which is what an unregistered type arrives as anyway.
- **`Constraints.Unique` is taken from the schema**, `OR`ed with
  `Decision.UniqueIndex`. §2 has the field and `internal/classify` does not set
  it — it sets a category, a confidence and the rule pack's default masker and
  nothing else — and a generator reads it (a unique email column gets a
  hash-derived suffix, §5). When §5's plan-time domain check lands and fills
  `Decision.UniqueIndex`, the two have to agree.
- **`Constraints.MaxLen` is set only for `varchar`, `bpchar` and `citext`.**
  `numeric`'s `atttypmod` packs a precision and a scale, not a byte count, and
  `mask`'s `digitBudget` reads `MaxLen` as a digit bound; a `numeric(5,2)`'s own
  precision therefore does not reach the masker at all. That is a gap in
  `mask.Constraints`, reported rather than papered over here. The rule itself
  moved to `mask.MaxLen` with T-0054, so `internal/plan`'s write-back check
  reads the same length this package hands the generator.
- **FK propagation is honoured, not re-decided.** §12 lists it under this
  package and §4 states it as a classification rule, which
  `internal/classify`'s pass 4 implements. What this package owes it is
  determinism: one category, one canonical value and one key give one fake, so a
  masked parent key and every column referencing it come out equal
  (`TestEqualValuesMaskAlikeAcrossColumnsOfOneCategory`).
- **`bloom.go` is real now**, so `MayContain` answers rather than failing
  closed. It keys on `HMAC(runKey, Encode(schema, table, column, path,
  canonical))` — the column's three parts encoded separately, for the reason §5
  gives its own encoding — at 29 bits per cell and k = 20 by double hashing,
  with a 64 KiB floor and a 512 MiB cap. The run key is `crypto/rand` per run
  and never leaves the process. A JSON path is `$.a.b[0]`; two leaves whose key
  names collide under that spelling merge into one entry, which costs a false
  positive and never a missed hit.
- **A refusal is `transform.refused.masker`, exit 7.** ADR-005's table names
  "7 extract or load" and does not name transform; a masker refusal stops the
  same movement of rows between the two, so it takes the same exit.

**The residual-filter contract.** `internal/verify` reproduces these bytes from
the *target* and tests the filter (§6 item 3), and it cannot import this
package, so this is the whole of what it may assume. A JSON path is spelled
`$.a.b[0]`.

| what | path | bytes |
|---|---|---|
| a masked scalar cell | `""` | `mask.Canonical(category, source value, constraints)` under the decision's category and the column's constraints — that is, `mask.Apply`'s own `Result.Canonical` |
| each masked array element | `""` | the same, per element |
| a collapsed document (log-shaped table, `hstore`, a document nothing could parse) | `""` | `mask.Canonical(semi_structured, source text)`, **omitted** when it equals what the target will hold (`{}`, or `''` for `hstore`) or is empty |
| a masked string leaf | the leaf's path | `mask.Canonical(free_text, the string)` |
| a masked number leaf | the leaf's path | `mask.Canonical(free_text, the number's JSON spelling)` |
| a boolean leaf | — | **not recorded** (a two-valued domain) |
| a `null` leaf | — | **not recorded** (not masked) |
| a leaf of a collapsed document | — | **not recorded** (no per-leaf masker ran) |
| a masked object key (email, phone or credit-card shaped, T-0137) | the key's own path in the *target* (`path+"."+maskedName`) | `mask.Canonical(the matched category, the key text)` — `mask.Apply`'s own `Result.Canonical`, not `free_text`: the category is whichever of the three strong validators the key matched |

Every leaf entry goes through one function, `addLeaf`, so the table above has
one implementation and not five call sites. Changing a row of it changes a
contract another package is written against: change both in one commit.

**A masked object key is keyed at its own path in the target, and `walk`
recurses under the masked spelling, not the source one** (`maskKey`, `walk`,
T-0137, **2026-09-14 review round, finding 1**). The first version of this
keyed a masked key's entry at the document's empty path and built every
child's path from the *source* key: `walk` still had the masked value to
write into `out`, but the path it recursed with was the one the target would
never spell that way again. `internal/verify`'s `leaves` and `documentKeys`
rebuild every path from the *target* alone (they cannot import this
package), so for `{"a@b.com":{"note":"Jane Smith"}}` this package used to
record the leaf under `$.a@b.com.note` while verify queried a masked spelling
at `$.<maskedkey>.note` — a bucket that was always empty. Every string leaf
beneath a masked key was therefore untested by the residual scan; a leaf
masker that failed open under such a key would have exited 0 with the leak in
the target. The fix makes `walk` mask the key first and recurse with
`path+"."+maskedName`, so every path beneath a masked key — the key's own
entry and every leaf nested under it — is spelled exactly as the target
spells it, and `internal/verify` can rebuild every one of them by reading the
target and nothing else. The masked key's own entry now sits at that real
path instead of the empty-path special case the first version needed; the
empty path is still where a scalar cell and a collapsed document record,
because neither of those has a path of its own to use.

**Two source keys that canonicalise alike are a refusal, not a silent
overwrite** (`walk`, T-0137, **2026-09-14 review round, finding 2**).
`mask.Apply` is a pure function of the category and the key's own canonical
text, so two distinct source keys that canonicalise to the same value mask to
the same fake — verified directly against `mask`: `"+1 415 555 2671"` and
`"+14155552671"` both mask to the same phone number, two case spellings of
one email address both mask to the same address, and `"4111 1111 1111 1111"`
and `"4111111111111111"` both mask to the same card number. A document keyed
by the same phone number in two formats, or the same address in two cases, is
ordinary in real payloads. The first version of `walk` wrote `out[maskedName]
= child` with no check, so the second key to arrive silently dropped the
first key's whole subtree — deterministically, since `names` is sorted before
the loop runs — with no error, no event and no counter; the target then held
fewer facts than the source claimed and nothing in the report said so. `walk`
now checks `out[maskedName]` before writing and refuses the cell
(`transform.refused.masker`, exit 7, `Masker: "json_key"`) the moment a
second source key would land on a name already taken, the same way any other
masker refusal stops the run — never naming either source key in the
`Refusal` (THREAT_MODEL.md T4).

**An array whose value arrives as a text literal is masked element-wise through
its literal (`array.go`, T-0118).** `maskArray` fires on a `[]any`, which is what
pgx hands back for an array type its map knows. The source pool registers no user
types (T-0076), so an array of an *extension* type — a `citext[]` — arrives as
the single string `{a@b.test,c@d.test}`. It used to fall through to the scalar
path, be masked as one string, and `CopyFrom` then failed with "cannot find
encode plan" at exit 7 with rows already moving. `internal/classify` reads inside
such a literal (T-0103, `literal.go` there) so that a `citext[]` of real
addresses is *decided* `email` rather than copied silently; `array.go` is the
write half of that, and the two have to agree about what an element is.

The rule is `maskArray`'s over a different carrier and not a second one: the
literal is parsed, each non-NULL element goes through the same element masker
with `h` computed per element, a NULL element stays NULL, an empty array stays
empty, a NULL column stays NULL, the dimensions and the `[0:1]=` prefix are
unchanged, and the result is written back in the form `array_in` reads. Every
element enters the filter under the column's empty path, which is the row the
residual table above already gives an array element.

**The filter entries a literal array makes are reachable by the residual scan**
(`internal/verify`, T-0129, landed). The entries are per element, and before
T-0129, `internal/verify` read the target column back as one Go `string` — the
literal — and `residual.go`'s `arrayHits` fell back to `scalarHits` on the whole
value when it was not a `[]any`: the whole literal canonicalised to something no
per-element entry matched, so every entry was untestable and the scan reported a
green pass. Measured by writing one element through unmasked while still
recording it: exit 0, residual check passed, and only the torture harness's
external I2 grep saw the addresses in the target. `internal/verify`'s
`arrayHits` now splits a `string` or `[]byte` carrier with the same grammar
`array.go` uses (`literalHits`) and tests each element; a literal it cannot
parse is `verify.refused.residual_unconfirmable` at exit 9 naming the column
rather than a silent fall-through, which is the same rule this package applies
at load time below. `internal/verify` was outside T-0118's paths, so it was filed rather
than worked around, and it was fixed and landed ahead of T-0127 below.

Two things separate it from `internal/classify`'s reader of the same grammar,
and both come from this being a parser rather than a signal. It **keeps the
nesting**, because a multidimensional literal is only valid when every inner
array has the same length and a flattened one is a literal `array_in` rejects.
And a literal it cannot read is a **refusal** (`transform.refused.masker`, exit
7) rather than one opaque value: masking it whole is what handed `CopyFrom` a
scalar for an `_citext` column, and copying it through is T12. classify may be
liberal because a literal it cannot read costs a signal; nothing here may be,
because what it produces is written into the target.

One thing about this is still owed outside this package and is filed rather
than worked around; two more that used to sit here have landed.

- **T-0127, landed.** `arrayArrivesAsLiteral` in `internal/plan/writeback.go`
  used to refuse such a column at exit 12 — a stand-in for the masker above,
  written so that landing the classify half alone would not turn a silent leak
  into a target half-loaded behind exit 7. It refused a column this package can
  mask, which is why `testdata/regressions/009` headered
  `exit 12 plan.refused.unwritable` rather than `ok`, and why nothing here had
  end-to-end coverage: the CLI stopped at plan before a row moved, so
  `array_test.go` was the whole of it. Removing the refusal was ordered behind
  T-0129 on purpose — while it stood, no *masked* array column arriving as a
  literal reached the target at all, so verify's blindness (below, now closed)
  cost nothing; landing it after T-0129 is what makes the residual scan's second
  net live over this column class instead of inert. `testdata/regressions/009`
  now headers `ok`, and its leak assertion is what the torture harness runs to
  confirm the whole path — plan admits the column, transform masks it
  element-wise, load writes it back, and the residual scan (T-0129) actually
  tests the per-element entries — end to end. Removing the refusal also moves
  the failure for an *unparseable* array literal from plan time (exit 12,
  before a key is fetched) to load time (`transform.refused.masker`, exit 7,
  mid-stream with earlier tables already committed) — the task carries that
  constraint, and no plan-time parse check was added to keep the failure where
  it was. What T-0127 established instead, in place of that check: the two
  grammars are not the same, and the divergence is fail-closed. classify's
  `splitLiteral` is deliberately liberal — a trailing empty field after a
  comma (`{a,}`) is silently dropped rather than flushed, and a doubled quote
  (`{"a""b"}`) is read as an escaped quote — because every caller there is a
  signal and a literal it cannot read falls back to one opaque value. This
  package's `array.go` refuses both forms (`bareElement` on the empty field,
  `quoted`/`element` on the doubled quote) rather than guess, because guessing
  can change the element count or the text, and what it produces is written
  into the target. So classify can decide a column that this package then
  refuses at load time. The reason it does not happen in practice: a value
  this package ever sees for an array column already passed through Postgres's
  own `array_out` — pgx hands back that exact server output, not user input —
  and `array_out` never writes a trailing empty field or a doubled quote; both
  forms above are things `array_in` would accept from a human but `array_out`
  itself does not produce (see the comments on `bareElement` and the
  backslash case in `element`). The residual risk is therefore a value
  classify's reader treats as parseable that no real array column can ever
  contain, not a real column this package will refuse. (`internal/classify/CLAUDE.md`'s
  own array-literal section still describes the refusal as standing and T-0118
  as owed; it is outside this package's paths, so it is not fixed here —
  **T-0147** tracks repointing it.)
- **T-0129, landed.** See the paragraph above: `internal/verify`'s `arrayHits`
  now splits the literal with this package's own grammar and tests each
  element, so the second net holds for this carrier too.
- **T-0128**: the load flattens a multidimensional array that arrives as a
  literal. pgx's `encodeCopyValue` falls back to scanning the literal as text and
  re-encoding it in binary (`values.go`), which is what makes a `string` writable
  into an `_citext` column at all — measured on postgres:16 — and that round trip
  drops the nesting and the dimension prefix. This package preserves both; the
  loss is entirely in the load, and it is there for an unmasked column too.

`testdata/regressions/005` keeps its `citext[]` values short and dull to steer
around the classify half and says so; `testdata/regressions/009` is the same
column with the addresses it really holds.

**Test.** `go test ./internal/transform/...`: determinism across two runs
under one key, difference across two keys, distinct values staying distinct
under a unique column, every JSON leaf replaced with its kind and its key name
intact, the log-shaped table collapsed with no per-leaf entries, an
already-empty document recorded nowhere, a boolean leaf redrawn across 32 keys
and recorded nowhere, arrays masked element-wise as a slice and as a literal,
the array grammar read and written back over quoting, escaping, NULL elements,
the empty array, nesting and the dimension prefix, a value in an array column
that is not a literal refused, `NULL` and `''` surviving,
every masked cell and leaf found in the filter afterwards, and a masker refusal
carrying `transform.refused.masker` at exit 7 without quoting the value.
`TestEveryJSONLeafIsReplaced` asserts nothing about a boolean leaf's value: it
is drawn from a two-element domain, so under half the run keys it comes back as
it went in, and an inequality there would pass or fail on the fixture's key
rather than on the code. `internal/extract`'s
`TestMemoryStaysBoundedOnTwoMillionRows` masks 2,000,000 rows through this
package under a `runtime.MemStats` ceiling. `TestAMaskedKeysResidualEntryIsAtTheMaskedPath`
and `TestTwoKeysThatMaskAlikeAreRefused` (2026-09-14 review round) pin the two
findings above: every residual entry under a masked key — the key's own and
every leaf beneath it — is spelled with the masked key and not the source
one, and two source keys that canonicalise alike are a refusal naming no
source value rather than a silently dropped subtree.

**Never:** mask a cell without adding it to the residual filter; accept a
runtime-loaded masker; let `Transform` depend on anything but its arguments
and `mask.Key`; copy a value through because a masker refused it.
