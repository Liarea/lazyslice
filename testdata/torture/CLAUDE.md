# testdata/torture/

Ten real open-source PostgreSQL schemas, pinned, plus the row generator that
fills them. Phase 5's gate (docs/BUILD_PLAN.md); docs/TORTURE.md is the result.
No Go code lives here — the loader and the catalogue are in
`internal/invariants` behind `integration && torture`.

**Debt this directory records:** `testdata/CLAUDE.md` still opens with "Two
fixtures and nothing else" and `testdata/README.md` with "Two PostgreSQL
fixtures". Both were outside T-TORTURE's paths; **T-0096** is the task that
corrects them. Until it lands, this file and `README.md` here are the spec for
everything under `testdata/torture/`, and `testdata/regressions/README.md` for
everything under `testdata/regressions/`.

**Contract.** `README.md` in this directory is the spec: what each schema is in
the set for, the rule that decides its root, and the six rules a fixture here
obeys. Each schema's own `README.md` pins it — upstream URL, commit or image
digest, artifact SHA-256, how `schema.sql` was derived, and every deviation from
upstream with a count. `internal/invariants/torture_catalogue_test.go` is the
other half: the root, the `--take`, the flags, the expected exit and the tables
the slice must reach. Neither can be added without the other —
`TestTortureCatalogueMatchesTheFixtures` fails on a directory with no entry and
on an entry with no directory.

**Rules.** The six that matter are in `README.md` and are not restated here.
The ones about *changing* this directory are:

- **A fixture change is a `make torture` run, not a reasoned argument.** These
  ten exist because reading a schema does not tell you what a pipeline does to
  it. Changing a row count in a `generate.sql` can move which table is
  most-connected, which moves the root, which is why
  `assertRootIsMostConnected` recomputes it at run time rather than trusting the
  catalogue.
- **A new defect is a file in `testdata/regressions/` before it is a fix in
  `internal/`.** All eight of the current ones came from these schemas, and each
  is the smallest schema that still shows the failure. A defect fixed without one
  is a defect that comes back the next time somebody touches `internal/classify`.
- **`_common/fill.sql` is a library.** A change to it changes all ten fixtures
  and can move every root. Per-schema peculiarities go in that schema's
  `generate.sql`, and a table it fills by hand carries a comment saying which of
  the generator's stated limits it ran into — a `CHECK` over nullable columns, a
  trigger with a hard-coded foreign key, a value the catalogue cannot describe.
- **An `--unmask` in the catalogue is a claim about the column, not about the
  run.** It says either "this is not personal data, and here is why" or "this is
  personal data and lazyslice cannot mask it yet, tracker task N". Eighteen of
  the forty-five flags the ten schemas used to need were the second kind and
  every one of them named **T-0098**; `mask/gen_credential.go` landed the masker
  and **T-0112** stripped all eighteen and re-ran `make torture`, so the ten
  schemas need twenty-seven flags now — nineteen `--unmask`, seven
  `--skip-table`, one `--key` — and those eighteen columns are masked rather
  than copied. The twenty-seventh, `auth.mfa_factors.friendly_name`, is a third
  kind and its reason has to carry the whole argument: the column is under a
  partial composite unique index that admits none of its rows, and `d_required`
  is computed over the whole table regardless — the over-estimate **ADR-011**
  clause (b) states as the rule (ARCHITECTURE.md §5).
- **A masking failure that happens sometimes is a fixture bug, not flakiness.**
  The suite fixes the masking key (`fixedSecret`) and the generators keep their
  name pools disjoint from `mask/words.go`'s word lists, for the reason
  `README.md` states at length. If a schema starts failing intermittently at
  exit 9 under `verify.refused.residual`, the question is which generated value
  is inside a masker's own output space — not how many times to re-run.
- **Nine clean and one failing is the gate, and the one that fails is asserted.**
  `mastodon` refuses at exit 13 under `target.schema.not_recreatable.function`,
  and the suite requires that exact code. Making it pass by editing its schema
  would delete the only place in the set where §11.1's refusal is exercised
  end to end; `TestTortureCatalogueMatchesTheFixtures` refuses a second failing
  schema for the mirror-image reason.

**Test.**

```
make torture
go test -tags 'integration torture' -run TestTortureSchemas/plausible ./internal/invariants/
./build.sh plausible          # rebuild one schema.sql from its pin (Docker + network)
```

**Never:** hand-edit a `schema.sql`; add a schema without a `README.md` that
pins it and a catalogue entry that runs it; put `random()`, `now()` or
`clock_timestamp()` in a generator (invariant I3); generate an email address
under `example.com`/`.net`/`.org`, an IPv4 in an RFC 5737 documentation range, a
phone number in `555-01XX`, or a person's name, street or suffix that is a word
in `mask/words.go` — those are the maskers' own output spaces
(ARCHITECTURE.md §5, T-0059, T-0073); add an `--unmask` whose reason would not
survive a stranger reading it.
