# Security policy

lazyslice exists to move data out of a production database. The whole point of
the tool is that the copy is safe to hold on a laptop, so a bug that leaves
personal data unmasked is not a cosmetic defect: it is the failure the tool was
built to prevent.

**lazyslice is pre-release.** The pipeline runs end to end against PostgreSQL;
`v0.1.0` was the first version a stranger may install; `v0.3.0` is the
current release and the one a report is triaged against, and its `lazyslice.yml` schema, flags and exit
codes may still change between `0.x` minors. The findings of
the independent review of 2026-09-09
([docs/reviews/2026-09-09/REVIEW.md](docs/reviews/2026-09-09/REVIEW.md)) and
of six red-team rounds have landed; what is still open is an
[issue](https://github.com/Liarea/lazyslice/issues). Report anything you find
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

These are documented in [THREAT_MODEL.md](THREAT_MODEL.md), and the first five
are on the README's front page. They are the honest edges of the design, and
reporting one tells us nothing new —
though an argument that one of them should be fixed is very welcome as a public
issue.

1. **Production row identifiers are preserved.** Surrogate keys and the foreign
   keys that reference them are copied verbatim, so anyone with any other
   reference to a production record — an admin URL, a ticket, a log line, a
   payment or support system — can re-identify every row exactly. A keyed remap
   is deferred past v1.
2. **A shape none of the validators recognise, in a column no name rule
   names.** The classifier and the second net between them parse or guess at
   about a dozen shapes (email, phone, national ID, IBAN, card number, IP or
   MAC address, a credential's entropy, a name, an address, ordinary prose, a
   special-category term). A card number is only recognised inside a known
   issuer's range and, under an id/number/version/reference column name, at
   that issuer's own length; one from a range the table lacks, or of an
   unlisted length under such a name, is copied. Under a version, build or
   release column name, IP or MAC addresses that are only a minority of
   the column's values are copied too; under a key, code, license, serial
   or token column name, a guessed-region phone number is copied too,
   when the table's best personal neighbour is only likely, not certain,
   personal. A value outside that list, in a column no name
   rule matches and whose table holds no other column already decided
   personal, is copied. This is the general case; the next two are the two
   specific instances of it that an adversarial red team found worth naming
   on their own. The entropy check itself now passes four more shapes
   through, in a column no credential name rule matches: a secret that is a
   hex run of exactly 32, 40 or 64 characters, a secret column of one to
   four non-NULL rows, a secret in a column named `type`, `klass` or
   `component_name`, and a file named after a person in a column where
   fewer than a fifth of the file names carry a word the dictionary holds.
3. **A name — or any other value — in a script the built-in dictionaries do
   not carry**, in a column also named in that script. The name, address,
   phone and email name-patterns and the name dictionary are Latin-script
   only, covering the given/surname stock of roughly twenty languages; a
   column named — and holding values — in a non-Latin script (Cyrillic, CJK,
   Arabic, Thai, Devanagari, Amharic, and more) defeats both the name-pattern
   match on the column and the value match on its contents at once.
   Extending the rule pack to non-Latin scripts is filed, not shipped
   (docs/reviews/2026-09-15-redteam/round5.json). A column called just
   `name` (or `display_name`, or `plan_name`) reaches the same place in any
   script: unless its table or its own name has a word for people in it
   (`users`, `customer_name`), it is copied when no column beside it is
   decided personal at `likely` or above (a neighbour masked on its name
   alone, such as a `phone` column, does not count), three or more of its
   values are sampled, and fewer than a fifth of them carry a name the
   dictionary holds — so a list of names the dictionary cannot carry, in
   such a column, is copied. A name whose own qualifier says it is a
   person's (`legal_name`, `billing_name`, `name_on_card`) is masked on
   the name alone, as before.
4. **A bare national identifier with nothing to corroborate it** — for
   example a nine-digit number with no dashes — in a column whose name
   matches no rule, when no other column of its table has already been
   decided personal (masked or not). One exception inside that: a
   contiguously issued block of national identifiers that is itself a table's
   own primary key reads as a surrogate key and stays exempt even beside a
   column that is masked, on the same reasoning as item 1. A key of scattered
   identifiers is scored like any other column, and one whose name matches a
   rule is refused at plan — see THREAT_MODEL.md T1 for the boundary and the
   reasoning against it.
5. **The marker-bound reload window.** A target lazyslice writes to for the
   first time gets a whole-target check, under the run's lease, for a table
   that was not part of the plan the gate approved. A *reload* of a target
   lazyslice has already marked as its own does not repeat that check: a
   table an application creates in the target strictly after one run
   finishes and strictly before the next run's first drop is left alone,
   untouched and unmentioned, rather than refused. See THREAT_MODEL.md T2.
6. A leaked value that was truncated, reformatted, or embedded in a longer
   string: the residual scan tests canonical equality only.
7. Values inside `bytea` that are not printable UTF-8 text — a genuine binary
   blob such as an image or a PDF. `bytea` holding printable text is read
   like a text column on both nets.
8. JSON key names, unless a key itself parses as an email address, a phone
   number or a credit-card number, in which case it is masked through that
   category's own masker (T-0137). An arbitrary identifier used as a key — a
   UUID, a slug, a customer number — is not named by any of the three and
   still survives. A leaf *value* is copied when nothing marks it personal:
   every key above it appeared in the sampled documents, no name rule names
   any of them, and no value validator recognises the value (T-0272). That
   keeps configuration documents working; the cost is a personal value no
   rule and no validator knows, inside a document, which is copied too. A key
   the samples never showed, or a document with no key, is still masked, and
   so is every leaf of a column whose own name marks it personal (a `jsonb`
   `medical_history`) or that was raised with `--mask
   TABLE.COL=semi_structured` or the yml.
9. `NULL` and the empty string, which survive and reveal that much, and the
   approximate length of a value masked as free text: its filler is fitted to
   the input's length, so an application's short values (`admin`, `linux`)
   stay short. Every character of it is still replaced.
10. Masked columns with a small admissible domain, where the substitution is
    recoverable by frequency. The tool lists these under `small_domain:`
    rather than hiding them.
11. Quasi-identifier combinations, and frequency or prefix leaks in unmasked
    columns.
12. A masked value that happens to coincide with another row's real value.
    For a person's name this is expected rather than rare, because masked
    names are real, common names — the 2020 U.S. Census top given names and
    surnames — so a masked "Mary" is often some other customer's real
    "Mary": the residual scan explains such a match (the list contains the
    value, the masker produced every copy, and, on a table with a primary or
    unique key, no row kept its own) and reports it as a count. A masked
    person name never equals its own source value, which reveals less than
    1/500 of a bit per value in a column wide enough for the whole list.
13. Values that were already fake in the source.

## Disclosure

We will work with you on a fix and credit you in the advisory unless you ask us
not to. We ask for the usual courtesy of not disclosing publicly until a fix is
available or 90 days have passed, whichever comes first.
