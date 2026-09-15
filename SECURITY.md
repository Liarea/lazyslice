# Security policy

lazyslice exists to move data out of a production database. The whole point of
the tool is that the copy is safe to hold on a laptop, so a bug that leaves
personal data unmasked is not a cosmetic defect: it is the failure the tool was
built to prevent.

**lazyslice is pre-release.** The pipeline runs end to end against PostgreSQL,
but there is no supported version until `v0.1.0` is tagged, and the findings of
the independent review of 2026-09-09
([docs/reviews/2026-09-09/REVIEW.md](docs/reviews/2026-09-09/REVIEW.md)) are
tracked in [tracker/](tracker/) until each lands. Report anything you find
anyway — a hole in the design is cheaper to fix than a hole in the code.

## Reporting a masking miss, or any other vulnerability

**Report privately. Do not open a public issue.**

Use GitHub's private vulnerability reporting on this repository:

> **Security** tab → **Report a vulnerability**

That opens a private advisory visible only to you and the maintainers. If the
Security tab is unavailable to you, open a public issue containing nothing but
the words "requesting a private security contact" and no detail at all, and a
maintainer will open a private channel.

We aim to acknowledge a report within three working days.

### Do not send us personal data

This is the one rule we ask you to hold to, because a report is a copy.

- **Never paste real values.** Not an email address, not a name, not a phone
  number, not a row, not a `pg_dump` fragment, not a screenshot of one.
- Send the **shape** instead: the column name, the column type, what the
  classifier decided, what you expected it to decide, and a value you invented
  that has the same shape.
- If a redacted reproduction is genuinely impossible, say so in the report and
  we will agree on a channel before you send anything.

A report that arrives carrying production data has reproduced the bug rather
than described it.

### What is in scope

- A column holding personal data that the classifier leaves unmasked (a **PII
  miss**). This is the report we most want.
- A masked value that is reversible, or a masking that is a stable substitution
  over a domain small enough to invert by frequency, where the tool did not say
  so.
- Anything that writes to the source database, or that a read-only role should
  have prevented.
- The target gate accepting a database it should have refused: a populated
  target, the source itself, a remote host without `--allow-remote-target`.
- A secret, a DSN, a password or a row value reaching a place it should never
  reach: `lazyslice.yml`, an event, an error message, a log line, the
  `lazyslice_meta` table, a `--json` stream, or a subprocess.
- Dependency vulnerabilities that are reachable from the shipped binary.

### What is a known limitation, not a vulnerability

These are documented in ARCHITECTURE.md and printed by `lazyslice doctor`. They
are the honest edges of the design, and reporting one tells us nothing new —
though an argument that one of them should be fixed is very welcome as a public
issue.

1. **Production row identifiers are preserved.** Surrogate keys and the foreign
   keys that reference them are copied verbatim, so anyone with any other
   reference to a production record — an admin URL, a ticket, a log line, a
   payment or support system — can re-identify every row exactly. A keyed remap
   is deferred past v1.
2. Personal data in a column classified `none` that no rule and no validator
   recognises.
3. A leaked value that was truncated, reformatted, or embedded in a longer
   string: the residual scan tests canonical equality only.
4. Values inside `bytea`.
5. JSON key names, unless a key itself parses as an email address, a phone
   number or a credit-card number, in which case it is masked through that
   category's own masker (T-0137). An arbitrary identifier used as a key — a
   UUID, a slug, a customer number — is not named by any of the three and
   still survives. Values never survive: every leaf value is masked.
6. `NULL` and the empty string, which survive and reveal that much.
7. Masked columns with a small admissible domain, where the substitution is
   recoverable by frequency. The tool lists these under `small_domain:` rather
   than hiding them.
8. Quasi-identifier combinations, and frequency or prefix leaks in unmasked
   columns.
9. A masked value that happens to coincide with another row's real value.
10. Values that were already fake in the source.

## Disclosure

We will work with you on a fix and credit you in the advisory unless you ask us
not to. We ask for the usual courtesy of not disclosing publicly until a fix is
available or 90 days have passed, whichever comes first.
