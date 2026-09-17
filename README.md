# lazyslice

Snapshot a production SQL database into a safe local copy: subset by a root
table, follow foreign keys, mask personal data, load.

## Status: pre-release, PostgreSQL only

The pipeline runs end to end against PostgreSQL 14 to 18: it discovers a
source and a target, refuses a target that is not empty or not its own,
subsets from a root table across foreign keys, masks personal data
deterministically, loads, and verifies the copy (foreign keys, row counts, a
residual scan of the target against the source). It is in hardening: an
independent review on 2026-09-09 found leak-class defects that are being fixed
in the open ([docs/reviews/](docs/reviews/), tracked in [tracker/](tracker/)),
and there is no supported version until `v0.1.0` is tagged.

Until then, point it only at data you are already allowed to hold on the
machine that runs it. What a snapshot does not hide is listed below and in
[THREAT_MODEL.md](THREAT_MODEL.md).

- What it will do, and what it refuses to do: [CONCEPT.md](CONCEPT.md)
- How it is built: [ARCHITECTURE.md](ARCHITECTURE.md) and
  [docs/adr/](docs/adr/)
- What it protects against, and what it does not:
  [THREAT_MODEL.md](THREAT_MODEL.md)
- Reporting a masking miss, privately: [SECURITY.md](SECURITY.md)
- Working on it: [CONTRIBUTING.md](CONTRIBUTING.md)
- The ten third-party schemas under `testdata/torture/` and their licences:
  [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md)

## What a snapshot will not hide

When lazyslice does work, a green tick will mean the checks passed, not that the
snapshot is anonymous. The full list of stated false negatives is in
[SECURITY.md](SECURITY.md); the first one matters most and is worth reading
before anything else:

> The snapshot preserves production row identifiers, so anyone holding any other
> reference to a production record — an admin URL, a ticket, a log line, a
> payment or support system — can re-identify every row exactly.

## Flags

<!-- docgen:flags:start -->

The flags a first run meets. The full set, one row per registered flag grouped by stage, is [docs/FLAGS.md](docs/FLAGS.md).

| Flag | Type | Default | Description |
|---|---|---|---|
| `--source` | string | - | Names the source; a non-Postgres scheme or unsupported major exits 2 |
| `--target` | string | - | Names the target; never bypasses the gate |
| `--root` | string | - | Root table (default: computed from the foreign-key graph) |
| `--yes` | bool | - | Headless: ask nothing; questions with no safe default become hard failures naming their flag |
| `--create-target` | bool | - | Start postgres:<source major> as lazyslice-target-<project> instead of asking |
| `--unmask` | stringArray | - | Per-column opt-out, as TABLE.COL=REASON; the bare form is exit 2; repeatable |
| `--skip-table` | stringArray | - | Drop a child-only table to schema-only; repeatable |
| `--phone-region` | string | - | ISO 3166-1 alpha-2 region libphonenumber recognises (e.g. GB; anything else is exit 2) a national-format phone column is read under, alongside the guessed regions every run already tries; recorded as phone_region and shown in the reasons output |
| `--secret-file` | string | "./lazyslice.secret" | Masking key file; LAZYSLICE_SECRET overrides it |
| `--require-key` | bool | - | Exit 5 instead of using an ephemeral key |
| `--plan` | bool | - | Stop after printing the plan; touch nothing |
| `--json` | bool | - | NDJSON events on stdout |

<!-- docgen:flags:end -->

## Building

```sh
make check        # lint and unit tests; this is what CI runs
make build        # bin/lazyslice
make integration  # container-backed tests; needs a Docker endpoint
```

The masker is a nested Go module, `github.com/Liarea/lazyslice/mask`, so it can
be imported by a program that has never heard of lazyslice (ADR-006).

## Licence

Apache License 2.0. See [LICENSE](LICENSE). The name lazyslice is a trademark of the project; the licence does not grant permission to use it for a derived product (Apache-2.0 §6).
