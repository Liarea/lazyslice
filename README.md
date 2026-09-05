# lazyslice

Snapshot a production SQL database into a safe local copy: subset by a root
table, follow foreign keys, mask personal data, load.

## Status: pre-release, and not yet usable

This repository is a scaffold. The command-line surface is real and the
architecture is written down; **every pipeline stage is a documented no-op**. A
run announces the nine stages and exits non-zero. Nothing connects to a
database, nothing is masked, and nothing is written.

Do not point this at a production database expecting a snapshot. There is not
yet anything to point it at.

- What it will do, and what it refuses to do: [CONCEPT.md](CONCEPT.md)
- How it is built: [ARCHITECTURE.md](ARCHITECTURE.md) and
  [docs/adr/](docs/adr/)
- What it protects against, and what it does not:
  [THREAT_MODEL.md](THREAT_MODEL.md)
- Reporting a masking miss, privately: [SECURITY.md](SECURITY.md)
- Working on it: [CONTRIBUTING.md](CONTRIBUTING.md)

## What a snapshot will not hide

When lazyslice does work, a green tick will mean the checks passed, not that the
snapshot is anonymous. The full list of stated false negatives is in
[SECURITY.md](SECURITY.md); the first one matters most and is worth reading
before anything else:

> The snapshot preserves production row identifiers, so anyone holding any other
> reference to a production record — an admin URL, a ticket, a log line, a
> payment or support system — can re-identify every row exactly.

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
