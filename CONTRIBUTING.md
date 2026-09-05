# Contributing to lazyslice

These are the rules the repository actually enforces. They are the same rules
[CLAUDE.md](CLAUDE.md) gives an agent working here, written out for a person.
Where the two disagree, CLAUDE.md wins and this file is wrong.

## Before you claim anything works

```sh
make check        # lint and unit tests, both modules
make integration  # container-backed tests; needs a Docker endpoint
```

Paste the output. **"Should work" is not a status.** A pull request that says a
change is fine without the result of the checks will be asked for the result of
the checks.

## Scope

**Work on one thing.** A change touches one pipeline stage unless an ADR says
otherwise. If you notice something else wrong while you are in there, say so in
the pull request description and open an issue; do not fix it in the same
change. Nearby-code cleanups, extra tests the change did not need and behaviour
the issue did not ask for all make a change harder to review and harder to
revert.

The pipeline stages are: **discover, introspect, classify, plan, extract,
transform, load, verify, emit** (ADR-005). They are wired together in exactly
one place, `internal/core`.

## Decisions

Decisions live in [docs/adr/](docs/adr/). An ADR is proposed until its phase
gate closes, then accepted and frozen.

**Never edit an accepted ADR.** To change an accepted decision, add a new ADR
that supersedes it, with the reversal condition that would send it back.

`ARCHITECTURE.md` is the reference the code is built against. Where it and an
ADR disagree, the ADR wins and ARCHITECTURE.md is corrected.

## The rules that are not up for discussion

These come from CONCEPT.md and THREAT_MODEL.md, and a pull request that breaks
one of them will be closed rather than reviewed.

- **Never add a flag that disables masking wholesale.** The CI grep rejects
  anything matching `no-mask|disable-mask|skip-mask|unsafe`, and
  `TestForbiddenFlagsDoNotExist` rejects it again. A per-column `--unmask
  TABLE.COL=REASON` with a mandatory reason is the only opt-out, by design.
- **When in doubt, mask it.** The classifier is biased to recall. The failure
  mode is mask more, never less.
- **A safety rail is not configurable away** without a flag whose name says what
  it does, and a row in THREAT_MODEL.md.
- **Nothing writes to the source.** Every source transaction is
  `REPEATABLE READ READ ONLY` and every statement passes a shape allowlist.
- **No value leaves the process.** A row value may not reach `lazyslice.yml`, an
  event, an error message, a log line, `lazyslice_meta`, a `--json` stream or a
  subprocess. Several tests exist only to enforce this; if one of them is in
  your way, that is the test doing its job.
- **The TUI is a thin layer.** Every TUI action must be reachable by a CLI flag
  first (ADR-002).
- **Documentation is never the fix for a confusing first run.** Change the
  default or change the question.

## Style

- `gofmt -s`, and `goimports` grouping with `github.com/Liarea/lazyslice` local.
  `make fmt` does both.
- Ordering is `(schema, name)`, then column, everywhere. Nothing iterates a Go
  map where the order can be observed: determinism is what makes
  `lazyslice.yml` a record of what happened.
- Comments say **why**, not what. A comment that restates the line above it will
  be asked to earn its place.
- Errors are wrapped with `%w` and the exit code is computed with `errors.Is`.
  Exit codes are part of the interface (ADR-005) and never change meaning.

## Tests

- Unit tests for anything pure: the classifier, the planner's ordering, the
  masker, the encoding.
- Container-backed tests, behind the `integration` build tag, for anything that
  is a statement about a real Postgres. `internal/testutil` starts them.
- The invariant suite is the definition of correct. If a change makes an
  invariant fail, the change is wrong until an ADR says the invariant is.

## The masker is a separate module

`mask/` is a nested Go module, `github.com/Liarea/lazyslice/mask` (ADR-006), so
it can be imported by a program that has never heard of lazyslice. It depends
only on the standard library, `golang.org/x/text` and
`github.com/nyaruka/phonenumbers`. Its determinism scheme — the length-prefixed
HMAC input encoding and each generator's declared `Domain()` — is part of its
public contract, and changing it is a major version of the module.

Adding a masker is a pull request to that module. **The classifier is not
pluggable**, by design and not by omission.

## Security

Do not open a public issue for a masking miss or any other vulnerability. See
[SECURITY.md](SECURITY.md) for the private route, and read the rule about not
sending us personal data before you write the report.
