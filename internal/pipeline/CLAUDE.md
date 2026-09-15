# internal/pipeline

Every stage interface and every shared type from ARCHITECTURE.md §2, one file
per stage (`discover.go`, `introspect.go`, `classify.go`, `plan.go`,
`extract.go`, `transform.go`, `load.go`, `verify.go`, `config.go`,
`source.go`, `pipeline.go`). No implementations — a `New()` constructor, a
SQL query, a masker, a renderer all belong in a stage package or in
`internal/pg`, never here.

**Contract.** This package *is* ARCHITECTURE.md §2. `Config` and every type in
that section live here and nowhere else; `internal/emit` implements `Emitter`
over `pipeline.Config` and holds no type of its own — that pattern (types
here, behaviour in the stage package) is the one to follow for any new type.

**Rules.**
- Imports only `ref`, `event`, `dsn` and `mask` (ARCHITECTURE.md §2 "Import
  graph"). It must never import a stage package — that would be the cycle
  `TestImportGraph` exists to catch.
- `Config` may hold no field that could carry a secret; a `secret:"true"` tag
  and `TestConfigHasNoSecretField` are the enforcement (THREAT_MODEL.md T5).
- `Decision.Reason` and `Step.Why` are rendered from fixed template sets
  (`internal/classify/reasons.go` and the equivalent for `plan`), never
  free-form — a sample value must never be constructible into one of these
  fields.
- Changing a stage's interface signature here is an ARCHITECTURE.md change
  first: update §2, then this file, in the same commit.

§2's `KeySet` block matches `plan.go` (Len, Bytes, Chunks, FirstChunk, EachChunk) as of 2026-09-08; a change here is a change to §2 in the same commit.

**`Writer` has four methods and `TypeRegistrar` is gone (T-0093).**
`RegisterTypes(ctx, *Schema) error` is `Writer`'s fourth method, in `source.go`,
and ARCHITECTURE.md §2 prints all four (reconciled 2026-09-09). It was an optional second
interface, `TypeRegistrar`, from T-0083 until T-0093: separate because the schema
is the loader's argument and registration can only happen once the DDL has
created the types in the target, halfway through the load. That justification did
not survive review. **"Optional" was the wrong word for it and it was not how the
loader treated it**: `internal/load` refused a `Writer` that was not a
`TypeRegistrar`, by name, at run time — and a type assertion that misses is a
load step that vanishes with no compile error. `internal/core` already wraps that
same writer in a `readableWriter` (which embeds the `Writer` *interface*), so one
refactor stood between the assertion and a run that registers nothing and fails
mid-copy on a composite column. The method is the compiler-checked shape:
`internal/pg`'s `writer` implements it, the doubles in `internal/load` and
`internal/verify` answer nil, and the by-name refusal in `internal/load` is gone
with the assertion that needed it.

**Test.** `go build ./internal/pipeline/...` (no logic to unit test on its
own); `TestNoValueBearingFieldSerialised`, `TestReasonGrammar` and
`TestConfigHasNoSecretField` live in this package or one that imports it.

**Never:** add an implementation; import a stage package; add a field that
`TestNoValueBearingFieldSerialised` would need to special-case rather than walk.

**`Tx` has a `Query` (T-0130, 2026-09-14).** ARCHITECTURE.md §2 prints it, and
this file records it in the same commit, as the rule above requires. `Writer`
still has no read and should not get one: verify reads the target through a
reader `internal/core` opens for it. The exception is the lock-and-recheck of
§11.2 — `LOCK TABLE ... NOWAIT`, re-verify what the gate approved, `DROP` — whose
read has to happen inside the transaction the drop commits in. A read made
anywhere else is an answer about a moment that has already passed, which is the
defect the recheck exists to close. `Eligibility` grew `MarkerRunID` and
`MarkerStatus` for the same reason: they are what the loader re-verifies, and
both are identifiers rather than values.

**One implementation lives here, under protest, and has a task against it
(T-0134).** `ddlliteral.go` holds `Literal`, `Literals`, `RewriteLiterals` and
`QuoteLiteral`: the reader that pulls the string constants out of a deparsed SQL
expression, and the rewrite of them. It breaks the "no implementation" rule
above and it is here because `internal/plan` and `internal/verify` both need
exactly one answer to "what is a literal in this expression" — the plan refuses
or masks them before anything is dropped, the verify catalog pass reads the
target's `pg_attrdef` and `pg_constraint` back afterwards — and a stage package
may not import another (internal/CLAUDE.md). Two copies of a scanner that
decides what is and is not inside the data boundary is the failure
`internal/verify/validators.go` records from its own hand copy of the
classifier's validators. It imports `strings` and nothing else, touches no type
in §2, and reads no row. **T-0162** moves it to a leaf package beside
`internal/textsig`, which is where it belongs; do not add a second
implementation here in the meantime, and do not treat this as a precedent for
one.

**`Column` has a `DefaultOriginal` (T-0134's review round, 2026-09-14).**
ARCHITECTURE.md §2 prints it and §11.1's amendment says what it is for. It holds
the catalog's own text of `Default` from before `internal/plan` masked the
literals in it, and is `""` on every column of every schema nothing has
rewritten — `internal/introspect` never sets it, and `internal/plan` is the only
stage that writes it.

`internal/verify` **reads** it (`catalog.go`'s `rewroteDefault`, the same review
round), and that read is a contract rather than a convenience: the catalog pass
exempts a masked column's `DEFAULT` from its literal scan only where the planner
masked it, and this field is the only record that it did. A stage that sets
`DefaultOriginal` without having rewritten the default, or rewrites one without
setting it, moves an object in or out of that exemption — so the pair is written
together in `internal/plan/ddlliteral.go` and read together here.

It exists because that rewrite mutates the `*Schema` the caller handed in (the
one deliberate mutation `internal/plan/CLAUDE.md` records, because
`internal/load/ddl` generates the target's DDL from that schema), and a mutation
that is not idempotent is a bug the second time anyone plans: `internal/tui`
re-plans against a cached schema and `internal/plan`'s own integration suite
plans twice over one snapshot to compare two plans, so without this the second
plan masked the first plan's output and the default became `mask(mask(x))`.
`internal/plan` reads the default through `originalDefault` everywhere, so a
re-plan re-masks the catalog's text — including under a different key, where the
answer is that key's value rather than a composition of both.

It is a text the source wrote and not a value from a row: a `DEFAULT` expression
is catalog text that `internal/introspect` already carries on `Default` and that
`internal/emit` does not write. Nothing here weakens `TestNoValueBearingField\
Serialised`'s rule; a field that carried a row value would.

**`Candidate` has an `IsZero()` (T-0165, 2026-09-14).** `Candidate` embeds
`dsn.Ref` by value, and T-0135 gave `Ref` a `Params map[string]string` field,
which makes a struct incomparable with `==` — and `Candidate` with it, since
Go's comparability is transitive over fields. `internal/core/provenance_test.go`
compared `r.sourceCand`/`r.targetCand` against `(pipeline.Candidate{})` with
`==`, which stopped compiling and took the rest of that package's suite (the
target gate and provenance safety tests included) down with it, since a test
binary that does not compile runs nothing. `IsZero()` is field-wise, matching
`dsn.Ref.IsZero()`'s own shape (`Params` counted as zero when empty via
`c.Ref.IsZero()`), and `provenance_test.go` calls it instead of comparing with
`==`. This is not the "no implementation" rule's exception clause and does not
need to be: `IsZero()` is a value method on a type this package already owns —
field comparisons only, no query, no masker, no renderer — the same footing
`Provenance`'s own constants stand on.
