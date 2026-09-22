# r/devops draft

Draft only. Nothing here is posted; the maintainer edits and posts it.

## Title

A static binary that turns a production Postgres DB into a safe local/CI
copy — no service, nothing it phones home to

## Post body

The usual ways a team gets realistic data into staging or CI are a
hand-maintained seed script that quietly drifts from the real schema, or a
`pg_dump`/`pg_restore` of production that somebody swears they'll scrub
"before it goes anywhere" and then doesn't, because scrubbing it correctly
by hand is its own project.

lazyslice is a single static Go binary: point it at a source and target
Postgres connection, pick a root table (or let it ask once), and it
subsets by following foreign keys, masks anything that looks like personal
data with a deterministic keyed hash, loads the result, and verifies it —
foreign keys resolve, row counts and sequences match the plan, and a
residual scan confirms no masked column still holds a source value. Same
inputs, same masked outputs, every run, so the emitted `lazyslice.yml` is
something you can commit and rerun in CI unattended, not a one-off
manual step. No account, no hosted service, no telemetry, no runtime call
to anything I operate — the two prior tools in this space (Snaplet, gone
2024; Neosync, archived 2025) both ended up routing a self-hosted-sounding
tool through a control plane, and I built lazyslice specifically not to
need one.

Straight up: **v0.1.0 is PostgreSQL only**, and it **pseudonymises, not
anonymises** — the exact residuals (row identifiers among them) are listed
in the README and in full in THREAT_MODEL.md, not buried. Flags and exit
codes can still change between `0.x` minors, with every change named in
the release notes.

For teams running this in CI today, however you're doing it: what's
actually gating your seed/staging data refresh right now — trust, size,
or just nobody having built the masking step yet?
