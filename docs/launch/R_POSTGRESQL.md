# r/PostgreSQL draft

Draft only. Nothing here is posted; the maintainer edits and posts it.

## Title

lazyslice: subset a production Postgres database, mask it, load it — one
command, one consistent snapshot

## Post body

I kept needing a small, referentially-complete slice of a production
Postgres database on my laptop and in CI, without customer emails coming
along for the ride. Hand-written seed data drifts from the real schema
within weeks; a raw `pg_dump` of everything is either too big or too
sensitive, usually both.

lazyslice opens the source in a single `REPEATABLE READ READ ONLY`
transaction, picks a root table, walks foreign keys outward — parents to
completeness, children capped, lookup tables kept whole, cycles named
explicitly rather than silently broken — and streams the rows out under
that one snapshot. Anything that looks like personal data is masked deterministically
under a local key before it lands in the target, so the same email masks to
the same fake address in every table and on every run, while surrogate keys
like `customer_id` are copied as-is, so they still join to the same `orders`
rows they did in production. Free text is masked whole rather than scanned
for maybe-PII, because a classifier that is only sometimes right is worse
than one that is boringly conservative; JSON documents are masked leaf by
leaf, each leaf a key rule or a value validator recognises replaced, and a
leaf neither recognises copied so a config document still works. On load it
verifies its own work: foreign keys resolve, sequences are reset past the max
value it loaded, and a residual scan of the target against the source
confirms no masked column still holds a source value, with a second net over
every column it left unmasked. Any failed check is a non-zero exit (7 for a
row count or sequence, 8 for a foreign key, 9 for a residual), and exit 0
means every check passed, not "probably fine."

Two companies shipped close to this exact idea and both are gone —
Snaplet (2024) and Neosync, whose repository was archived in 2025 — so I'm
not pretending this is an unclaimed idea, just one I think is worth having
as a small, boring, dependency-free binary rather than a company.

Honestly, up front: **v0.1.0 is PostgreSQL only**, and it **pseudonymises,
not anonymises** — row identifiers, foreign keys, and a short, named list
of other residuals are copied through and documented rather than hidden
(README's "What a snapshot will not hide", and THREAT_MODEL.md in full).
The `lazyslice.yml` schema and exit codes can still change between `0.x`
minors.

If you've hand-rolled something like this against your own schema: what
broke it — a partition scheme, a cycle in the foreign keys, a composite
type, something else?
