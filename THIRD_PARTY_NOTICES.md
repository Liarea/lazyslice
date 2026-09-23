# Third-party notices

The files under `testdata/torture/` are database schemas taken or generated
from ten open-source projects, used here only as read-only test fixtures
(sample schemas that lazyslice's test suite plans and loads against). They
are **not** covered by this repository's Apache-2.0 licence, they retain
their own upstream licences as set out below, and they are not linked into,
built into, or distributed with the lazyslice binary.

Each directory's own `README.md` records the exact pin (commit, tag,
version, or image digest) and how `schema.sql` was derived; this file
records the licence that applies to the upstream source at that pin, as
verified against the upstream repository itself (not from memory), and the
URL of the licence text checked.

| Directory | Upstream | Pin | Upstream file(s) used | Licence (SPDX) | Licence text verified at | Caveat |
|---|---|---|---|---|---|---|
| `calcom` | https://github.com/calcom/cal.com | commit `1251ba5be567d26a7f922452fe7797642376476e` | `packages/prisma/migrations/*/migration.sql` (all 595) | MIT | https://github.com/calcom/cal.diy/blob/1251ba5be567d26a7f922452fe7797642376476e/LICENSE | The repository has since been renamed `calcom/cal.com` → `calcom/cal.diy` (same repository id, GitHub 301-redirects the old path). Licence text fetched at the pinned commit is unchanged and confirmed MIT regardless of the name used to reach it. |
| `discourse` | https://github.com/discourse/discourse | commit `7b13572c76fa2c32e86267c91c238b0a3d8084f9` | `db/structure.sql` | GPL-2.0-or-later | https://github.com/discourse/discourse/blob/7b13572c76fa2c32e86267c91c238b0a3d8084f9/LICENSE.txt | `LICENSE.txt` itself carries the bare GPLv2 template; the repository's `README.md` at the same commit states the code is "Licensed under the GNU General Public License Version 2.0 (or later)", which is the basis for the `-or-later` suffix. The fixture is an unmodified schema dump: `db/structure.sql` loaded into `pgvector/pgvector:pg16` and re-dumped with `pg_dump --schema-only --no-owner --no-privileges`, with only `pg_dump`'s own per-session `\restrict`/`\unrestrict` lines stripped; no table or column definitions were changed. |
| `django` | https://github.com/django/django | version `5.2.6` (tag `5.2.6`) | Migration files under `django/contrib/{admin,auth,contenttypes,sessions}/migrations/` — the packages `startproject`'s default `INSTALLED_APPS` pulls in | BSD-3-Clause | https://github.com/django/django/blob/5.2.6/LICENSE | GitHub's license API returned NOASSERTION for this file (its detector does not auto-classify Django's 3-clause BSD text with certainty); the fetched text was read directly and matches the standard BSD-3-Clause template. |
| `gitlab` | https://github.com/gitlabhq/gitlabhq (GitHub mirror of `gitlab-org/gitlab`) | commit `f49b990568b44b24710efabf287200733f6fa930` | `db/structure.sql`, reduced by `../build.sh gitlab` to a 43-table computed subset (seeded from `users`, `namespaces`, `projects`, `issues`, `merge_requests`, `notes`, `members`, `milestones`, `emails`, `user_details`, `user_preferences`, `personal_access_tokens`, `identities`, `project_authorizations`, `issue_assignees`, `todos`, `events`, `abuse_reports`, `award_emoji`, `project_settings`, `namespace_settings`, closed over required parents and trigger dependencies) | MIT | https://github.com/gitlabhq/gitlabhq/blob/f49b990568b44b24710efabf287200733f6fa930/LICENSE | GitLab's own `LICENSE` file dual-licenses the repository: content under `ee/` is proprietary (GitLab Enterprise Edition License, in `ee/LICENSE`), content under `doc/` is CC BY-SA 4.0, and everything else — including `db/structure.sql`, which lives outside `ee/` — is MIT ("MIT Expat"). All 21 seed tables and their closure are long-standing GitLab Community Edition tables (accounts, projects, issues, and their support tables); none of the 43 kept tables are EE-only features. The fixture is also a trimmed and lightly edited subset, not the full 1,448-table file: two upstream objects (`organizations.uuid`'s `DEFAULT gen_random_uuid_v7()` and the index `index_todos_coalesced_snoozed_until_created_at`) are dropped because they depend on GitLab-defined functions lazyslice v1 does not recreate — the only departure from upstream's text, and it is documented in `gitlab/README.md`. |
| `mastodon` | https://github.com/mastodon/mastodon | commit `26fce0f2f9e36ea0e0c2b03b7f57d1b1ea58ed1c` | `db/schema.rb`, `lib/mastodon/snowflake.rb` | AGPL-3.0 | https://github.com/mastodon/mastodon/blob/26fce0f2f9e36ea0e0c2b03b7f57d1b1ea58ed1c/LICENSE | The fixture is a generated dump: `schema.rb` is loaded via ActiveRecord + Scenic into an empty PostgreSQL database and the result is `pg_dump`'d. The one intentional edit is the salt installed by `lib/mastodon/snowflake.rb`'s `timestamp_id` function — fixed at thirty-two zeroes by the build script for reproducibility, in place of upstream's random `SecureRandom.hex(16)` salt on every install; the function's logic is otherwise upstream's, character for character, per `mastodon/README.md`. |
| `metabase` | https://github.com/metabase/metabase | image `metabase/metabase:v0.56.10` (digest `sha256:4b2bdce29288b8e94d73e44862644e84c940ab68e551ba9aefb1a8dc1738e9d8`), matching upstream tag `v0.56.10` | `resources/migrations/*.yaml` (Liquibase changelog), applied by running the image | AGPL-3.0 | https://github.com/metabase/metabase/blob/v0.56.10/LICENSE.txt | Metabase dual-licenses by directory: files under the top-level `enterprise/` directory are Metabase Commercial License, everything else is AGPL. `resources/migrations/` sits outside `enterprise/`, and the pinned image is `metabase/metabase` (the AGPL binary) rather than `metabase/metabase-enterprise` (the commercial binary), per the same `LICENSE.txt`. The fixture is a generated dump: the official image is started against an empty database and run to "Metabase Initialization COMPLETE" (applying the changelog itself), then `pg_dump`'d — no upstream file content is copied verbatim, but the resulting schema is entirely the deterministic output of upstream's own migration definitions. |
| `odoo` | https://github.com/odoo/odoo | image `odoo:18` (digest `sha256:259fa933bf3ee7f3e375bd74d1e0bc28bd75955159723be477359e0fdb8acf67`), modules `base,mail,contacts` | Schema emitted by Odoo's ORM from the `base`, `mail`, and `contacts` addon model definitions (`odoo/addons/base/`, `addons/mail/`, `addons/contacts/`) | LGPL-3.0 | https://github.com/odoo/odoo/blob/18.0/LICENSE | Odoo has no schema file and no per-release git tag for Docker image versions; the image's own Dockerfile (`odoo/docker`'s `18.0/Dockerfile`) sets `ENV ODOO_VERSION=18.0` and installs a dated nightly `.deb` build off that branch, so the closest verifiable upstream reference is the `18.0` branch tip rather than one fixed commit — lower-precision provenance than the commit pins used for the other nine schemas. All three modules used live in the community `odoo/odoo` repository (not the proprietary `odoo/enterprise` repository), confirmed by directory listing at the `18.0` ref. The fixture is schema-only and excludes Odoo's own seed/reference data (currencies, countries, admin user), which `generate.sql` supplies separately per `odoo/README.md`. |
| `plausible` | https://github.com/plausible/analytics | commit `e74d6fb214b76664442d6a6bb8d96e74807ccfbf` | `priv/repo/structure.sql` | AGPL-3.0 | https://github.com/plausible/analytics/blob/e74d6fb214b76664442d6a6bb8d96e74807ccfbf/LICENSE.md | The fixture is an unmodified schema dump: `structure.sql` (Ecto's own `mix ecto.dump` output) loaded and re-dumped with `pg_dump --schema-only --no-owner --no-privileges`, with only `pg_dump`'s `\restrict` lines removed; nothing else changed, per `plausible/README.md`. |
| `rails-activestorage` | https://github.com/rails/rails | version `8.0.2` (tag `v8.0.2`) | ActiveStorage's bundled migration template, `activestorage/db/migrate/20170806125915_create_active_storage_tables.rb`, installed via `bin/rails active_storage:install` | MIT | https://github.com/rails/rails/blob/v8.0.2/MIT-LICENSE | The `posts` and `comments` tables in the fixture are not upstream content — they come from `rails generate scaffold`/`generate model` commands run by the build script against locally-authored, trivial column lists, added only so ActiveStorage's polymorphic attachment columns have something to point at. |
| `supabase-auth` | https://github.com/supabase/auth | commit `0907af9bd6be3c76f472c40a7dcc0dc34abeffaf` | `migrations/*.sql` (all 75) | MIT | https://github.com/supabase/auth/blob/0907af9bd6be3c76f472c40a7dcc0dc34abeffaf/LICENSE | The build script performs two mechanical substitutions before concatenation: GoTrue's Go-template namespace placeholder (`{{ index .Options "Namespace" }}`) is filled in as `auth`, and a missing trailing `;` is added between two migration files so concatenation doesn't run one file's last statement into the next file's first. Neither changes any identifier, type, or constraint upstream declared. |

## `internal/textsig/names.txt` — multilingual given/family name stock (T-0188)

The given-name and family-name entries `internal/textsig/names.txt` gained for
Polish, Italian, Dutch, German, French, Spanish, Portuguese, Turkish, Swedish,
Finnish, Icelandic, Yoruba, Igbo, Swahili, Hindi (romanised), Arabic
(romanised), Vietnamese, Japanese (romanised), Korean (romanised) and Chinese
(romanised via pinyin) are **not** derived from the ten `testdata/torture/`
projects above; they are data, not code, taken from Wikidata, which dedicates
its structured data to the public domain under **CC0 1.0** — confirmed at
<https://www.wikidata.org/wiki/Wikidata:Licensing> ("Wikidata … structured
data available under the Creative Commons CC0 License") on 2026-09-15. CC0 is
a public-domain dedication, not an attribution licence, so nothing here is
owed a byline; this section exists so the query, the date and the exact
`wikibase:sitelinks`-ordered cut are reproducible rather than merely asserted.

**The query, one language at a time.** For each language, two runs — `given`
over `wd:Q12308941` (male given name), `wd:Q11879590` (female given name) and
`wd:Q202444` (given name) unioned together, and `family` over `wd:Q101352`
(family name) alone — both filtered to items carrying `wdt:P407` ("language of
work or name") equal to that language's own Wikidata item, grouped by label and
ordered by `MAX(?sitelinks)` descending (a proxy for how well-attested the name
is, so `LIMIT` keeps the most frequent names rather than an arbitrary cut), run
against `https://query.wikidata.org/sparql`:

```sparql
SELECT ?name (MAX(?sl) AS ?sitelinks) WHERE {
  { ?item wdt:P31 wd:Q12308941 } UNION { ?item wdt:P31 wd:Q11879590 } UNION { ?item wdt:P31 wd:Q202444 }
  ?item wdt:P407 wd:<language Q-id> .
  ?item rdfs:label ?name .
  ?item wikibase:sitelinks ?sl .
  FILTER(LANG(?name) = "<label language>")
}
GROUP BY ?name
ORDER BY DESC(?sitelinks)
LIMIT 1500
```

(the `family` run is the same query with the given-name union replaced by
`?item wdt:P31 wd:Q101352`). For the five languages whose own script is not
Latin — Hindi, Arabic, Japanese, Korean and Chinese — `<label language>` is
`en` rather than the language's own code, deliberately: an `auth.users`-shaped
column in an English-schema
database holds a romanised name (`Muhammad`, `Priya`, `Wei`), not the native
script, and most Wikidata name items carry an English label that already is
that romanisation. Run on 2026-09-15.

| Language | `<language Q-id>` | Label read | Given fetched | Family fetched |
|---|---|---|---:|---:|
| Polish | `wd:Q809` | `pl` | 1,500 | 1,500 |
| Italian | `wd:Q652` | `it` | 1,500 | 1,500 |
| Dutch | `wd:Q7411` | `nl` | 1,500 | 1,500 |
| German | `wd:Q188` | `de` | 1,500 | 1,500 |
| French | `wd:Q150` | `fr` | 1,488 | 1,500 |
| Spanish | `wd:Q1321` | `es` | 1,500 | 1,500 |
| Portuguese | `wd:Q5146` | `pt` | 712 | 1,166 |
| Turkish | `wd:Q256` | `tr` | 1,500 | 1,500 |
| Swedish | `wd:Q9027` | `sv` | 593 | 1,314 |
| Finnish | `wd:Q1412` | `fi` | 913 | 957 |
| Icelandic | `wd:Q294` | `is` | 1,500 | 35 |
| Yoruba | `wd:Q34311` | `yo` | 49 | 97 |
| Igbo | `wd:Q33578` | `ig` | 58 | 20 |
| Swahili | `wd:Q7838` | `sw` | 5 | 3 |
| Hindi (romanised) | `wd:Q1568` | `en` | 265 | 24 |
| Arabic (romanised) | `wd:Q13955` | `en` | 1,177 | 1,349 |
| Vietnamese | `wd:Q9199` | `vi` | 69 | 59 |
| Japanese (romanised) | `wd:Q5287` | `en` | 1,500 | 1,500 |
| Korean (romanised) | `wd:Q9176` | `en` | 1,500 | 145 |
| Chinese (romanised, pinyin) | `wd:Q7850` | `en` | 1,500 | 342 |

`LIMIT 1500` is the "a few thousand per language at most" the task set;
several languages fetched fewer because Wikidata's own coverage — items
carrying `P407` for that language *and* at least one sitelink — runs out
before the limit does (Swahili, Igbo, Yoruba, Vietnamese and Icelandic's own
family-name stock all sit under 100). Swahili's five given and three family
names are kept for completeness; they do not move any measurement below.

**What happened to a raw fetched name before it became a `names.txt` line.**
`internal/textsig`'s tokenizer (`unicode.IsLetter`-run splitting, `dict.go`)
looks up one letter-run at a time, never a whole cell, so a multi-word fetched
name (`"Abd al-Karim"`, a compound family name) is split into its own letter
runs the same way a sampled value would be, and each run — lower-cased — is
a candidate line on its own rather than the multi-word string verbatim, which
the tokenizer could never match as a unit. Three filters then apply to every
candidate, in this order, and a name the existing `names.txt` already carried
skips all three (the header's own precision rules bind new entries, not old
ones): (1) shorter than three letters is dropped, the same floor
`TestDictionaryKeepsItsPrecisionRules` holds the whole file to; (2) a candidate
that is also an ordinary English word per this machine's `/usr/share/dict/words`
(Web2, BSD-licensed, not shipped — a filter input, not a dependency) is meant
to be dropped, the same "no name that is also an ordinary English word" rule
the existing file states — **the T-0188 review round found this filter had not
actually run**: 23 given-section promotions of surname-section English words
(`long`, `sun`, `young`, `berry`, `lee`, `lin`, `dang`, `kang`, `wang`, `yang`,
`berg`, `burke`, `duncan`, `francis`, `franklin`, `gordon`, `laine`, `lloyd`,
`mitchell`, `morris`, `nelson`, `santos`, `ahmed`) and the surname `has` were
present in `/usr/share/dict/words` and should have been dropped; they were
removed from the file in the review-round fix. What the filter does not and
cannot catch, and what is knowingly still in the file: Web2 is an unabridged
dictionary and defines many ordinary English words that are also extremely
common given names in their own right (`mark`, `paul`, `john`, `leo`, `iris`,
`anna`, `david`, among others) — those stay, because a candidate the *pull*
itself sourced as a given name is not the "ordinary word wearing a name's
clothes" case this filter and the header's rule exist to catch (street names
and colour-name pairs built from the *surname* section's roughly two hundred
ordinary nouns, `dict.go`'s own narrowing). (3) a candidate that is also a place name — a country, a
capital, or a city over 100,000 population, read from Wikidata in English and
in each of the twelve native-orthography languages above (`wd:Q515`/subclasses
with `wdt:P1082 > 100000`, `wd:Q6256` for countries, `wd:Q5119` for capitals) —
is dropped. The third filter exists because a family name and a place name are
the same word constantly (an Italian surname list free of Italian city names
is not the same list): `TestCompositeAddressAcrossFieldsFailsClosed` caught
`Milano` — an Italian surname *and* Italy's second city — turning an address
composite's decision to `person_name` before this filter was added, and it is
the reason a second Wikidata pull backs this file's exclusions rather than a
hand-typed one.

## `mask/words_corpus.go` — 2020 Census given-name and surname stock (T-0303, T-0304)

`mask/words_corpus.go`'s `censusGivenWords` and `censusSurnameWords` are
**not** derived from Wikidata or from any of the ten `testdata/torture/`
projects above; they are data taken from the U.S. Census Bureau's 2020
Census names release,
<https://www.census.gov/topics/population/genealogy/data/2020_names.html>.
Both source files are works of the U.S. Government and so are in the public
domain in the United States under **17 U.S.C. § 105** ("Copyright protection
… is not available for any work of the United States Government"), not
licensed to this project under any open-source licence. The Bureau asks
that a use of its data be cited
(<https://www.census.gov/about/policies/citation.html>); this project's
citation is: U.S. Census Bureau, 2020 Census, "Frequently Occurring
Surnames from the 2020 Census" / "Frequently Occurring First Names in the
2020 Census by Sex."

| File | URL | Date fetched | sha256 |
|---|---|---|---|
| `Names2020_FirstNames_Sex_Top1000.xlsx` | <https://www2.census.gov/topics/genealogy/2020surnames/Names2020_FirstNames_Sex_Top1000.xlsx> | 2026-09-22 | `b7e8a8ea8cf5babe0220aa9f0f19294664a1818fc61acd43efcd3ff356fa9676` |
| `Names2020_LastNames_RaceHispanic_Top1000.xlsx` | <https://www2.census.gov/topics/genealogy/2020surnames/Names2020_LastNames_RaceHispanic_Top1000.xlsx> | 2026-09-22 | `89108b7321fc4656fba390ea665680ac651786d51effc51da54d4b8be3fdbf6b` |

**The cut.** `tools/names` (`tools/names/README.md` has the full extraction
and generation procedure) reads only each file's `name`, `sex`, `rank` and
`count` columns — the surnames file's six race and Hispanic-origin
proportion columns are never read, copied or embedded anywhere in this
project. From the first-names file it derives a per-sex rank (the file
itself carries one overall rank and a `MALE`/`FEMALE` count pair, not a
per-sex rank), takes the union of the top 500 male-ranked and top 500
female-ranked given names, dedupes and lowercases: **958 given names**.
"Male-ranked"/"female-ranked" here means ranked by that sex's count within
the same 1,000-name overall list, not a per-sex top-500 the Bureau itself
publishes — the bottom of the male-ranked 500 is thin (e.g. `KENNEDY`,
male rank 500, male count 6,742) and reaches names more commonly read as
female by count (`tools/names/README.md` has the full derivation and a
worked example). From the surnames file it takes all top-1,000 surnames,
lowercased: **1,000 surnames**. Neither list is filtered against any other
corpus. Since T-0304 they are the `mask` module's only person-name lists:
`mask/words.go` builds `givenNames` and `surnames` from these two constants
(the constants themselves stay in `mask/words_corpus.go`), and every masked
given name, surname, full name and email local part is drawn from them. The
hand-curated lists `mask/words.go` carried before, and T-0287's synthetic
role tokens, are gone (`mask/CLAUDE.md`, "The name lists").

## Copyleft summary

Five of the ten fixtures are under copyleft licences: `discourse`
(GPL-2.0-or-later); `mastodon`, `metabase`, and `plausible` (AGPL-3.0); and
`odoo` (LGPL-3.0). `gitlab` is permissively licensed (MIT) despite upstream
also containing a proprietary `ee/` subtree, because the file used here sits
outside that subtree — see the table's caveat for that row. For each
copyleft row, the table above states whether the fixture is an unmodified
dump, a generated dump, or a trimmed/edited subset, per each directory's own
`README.md`.

## Not independently verifiable

- **`odoo`**: the pin is a Docker image digest, and the official image build
  installs a dated nightly package rather than checking out a specific
  `odoo/odoo` commit. The `18.0` branch (and its current LICENSE file) is the
  closest verifiable upstream reference; the exact source snapshot backing
  the nightly `.deb` used to build the pinned digest cannot be independently
  confirmed from the GitHub repository alone.

Date verified: 2026-09-14.
