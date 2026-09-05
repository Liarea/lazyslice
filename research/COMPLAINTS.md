# COMPLAINTS

What people actually say, in their own words, about getting realistic data into a local or CI database. Every quote below is verbatim from a public source with a date and a link.

Compiled 2026-09-04.

## Method and caveats

- **Where I looked:** Hacker News (via the Algolia search API, both comment search and full thread fetches), GitHub issues on Greenmask, Neosync, Replibyte, Jailer and the Snaplet successor repos, GitLab issues on PostgreSQL Anonymizer (which lives on GitLab, not GitHub), Stack Overflow / Software Engineering SE / DBA SE via the Stack Exchange API, dev.to via its articles API, and vendor engineering blogs.
- **Where I could not look:** Reddit is not fetchable from this environment. One quote below (TR-12) is a Reddit comment reproduced second-hand in a dev.to post; it is flagged as such and should be treated as weaker evidence than the rest.
- **Vendor bias:** several sources are written by people selling a tool. Those are marked **[vendor]**. They are still useful — a vendor describing the problem is describing the problem their customers told them about — but they are not neutral. They are counted in the frequency table and shown in a separate column so the ranking can be re-checked without them. Doing so ties foreign keys and config burden at the top (12 each) and leaves CI last, but reshuffles the middle: unsupported-database rises to third, trust falls to fourth, leaked-PII to joint fifth. The middle of the table is not robust to that choice; the top and bottom are.
- **This is a convenience sample, not a survey.** The frequency table ranks how often a theme appears *in the material I could reach*, weighted toward places where frustrated people write things down (bug trackers, Show HN threads). Bug trackers over-represent crashes and under-represent "I gave up before filing anything". Treat the ranking as a strong hint about ordering, not a measurement.
- 80 numbered entries across 7 themes are collected below; several entries carry more than one quote (a report plus its confirming replies), so the total number of verbatim quotations is higher.

## Tool state as of September 2026

Context for the trust quotes. Verified today against the repos themselves, not from memory.

| Tool | State | Evidence |
|---|---|---|
| Greenmask | Active. Latest release v0.2.23, 2026-08-22; last push 2026-08-25; 1,757 stars | [releases](https://github.com/GreenmaskIO/greenmask/releases/tag/v0.2.23) |
| PostgreSQL Anonymizer (Dalibo) | Active on GitLab. Release 3.2 issue opened 2026-09-04; 3.1.2 released 2026-06-29 | [repo](https://gitlab.com/dalibo/postgresql_anonymizer), [releases](https://gitlab.com/dalibo/postgresql_anonymizer/-/releases) |
| Jailer | Active. Jailer 17.2.2 released 2026-08-19; last push 2026-09-04; 3,195 stars | [releases](https://github.com/Wisser/Jailer/releases/tag/v17.2.2) |
| Neosync | **Archived** 2025-08-30. Last release v0.5.41, 2025-07-11; 4,141 stars | [repo](https://github.com/nucleuscloud/neosync), [issue #3565](https://github.com/nucleuscloud/neosync/issues/3565) |
| Replibyte | **Effectively dead.** Last tagged release v0.10.0, 2022-10-14; open "Maintained?" issue since 2023; 4,409 stars | [releases](https://github.com/Qovery/Replibyte/releases/tag/v0.10.0), [issue #277](https://github.com/Qovery/Replibyte/issues/277) |
| Snaplet snapshot | **Archived.** Company shut down 2024; repo moved to `supabase-community/snapshot`, archived | [repo](https://github.com/supabase-community/snapshot), [Supabase post](https://supabase.com/blog/snaplet-is-now-open-source) |
| Snaplet seed | Community-maintained, drifting. Last release v0.98.0, 2024-07-30 | [releases](https://github.com/supabase-community/seed/releases/tag/v0.98.0) |

Three of the seven best-known tools in this space are archived or abandoned. That is itself the loudest complaint in the corpus.

---

## Theme 1 — Broken foreign keys and referential integrity

The most-repeated technical failure. It shows up as three distinct sub-problems: masking that desynchronises keys, subsetting that drops parents, and cycle handling that panics.

**FK-1** — [Hacker News, 2024-05-22](https://news.ycombinator.com/item?id=40446843), user `imiric`, on the Neosync Show HN:

> The problem I'm running into is referential integrity, as importing the anonymized data is raising unique and foreign key violations. The obfuscator tool is pretty minimal and has few knobs to tweak its output, so it's difficult to work around this, and I'm considering other options at this point.

**FK-2** — [Hacker News, 2024-05-22](https://news.ycombinator.com/item?id=40446813), `edrenova`, Neosync co-founder, replying in the same thread **[vendor]**:

> yeah the referential integrity and constraints part is usually the most complicated part and everyone does things differently which adds another layer of complexity on it

**FK-3** — [Hacker News, 2026-03-05](https://news.ycombinator.com/item?id=47261678), `nickzelei`, Neosync co-founder, on the dbslice Show HN, writing after Neosync was archived **[vendor]**:

> It was probably the hardest feature to fully solve for the customers that needed it the most, which were the ones with the most complex data sets and foreign key dependencies. To the point where it was almost impossible to do this, at least with syncing it directly to another Postgres database with everything in tact.

**FK-4** — [Hacker News, 2025-05-16](https://news.ycombinator.com/item?id=44008738), `tudorg`, answering "Ask HN: Any good tools to pgdump multi tenant database?":

> The problem is most of these tools can walk foreign keys, but only in one directions.

**FK-5** — [Hacker News, 2022-04-28](https://news.ycombinator.com/item?id=31188127), `husainfazel`, on the Replibyte Show HN:

> Database Subsetting: Scale down a production database to a more reasonable size What about this? Does the database need foreign keys to prevent related rows in tables being lost and are they just randomly deleting rows as the config seems to indicate

**FK-6** — [Greenmask issue #403, 2026-02-19](https://github.com/GreenmaskIO/greenmask/issues/403):

> The restore operation generates an error because it is not able to recreate the foreign keys between the two tables above. The reason is there are no rows in the *demo_persons* table in the target database while there were rows with ID 3 and 9 in the source.

The reporter later found the cause himself: *"The problem is due to a circular reference which is not present in my scripts above. My intention was to simplify the test case. I did not realize that the circular reference was the cause of the issue."*

**FK-7** — [Greenmask issue #329, 2025-08-13](https://github.com/GreenmaskIO/greenmask/issues/329), still open:

> I was using greenmask, but it fails since there is no cycle on the database tables. It seems the code was written in a way to complain when there is more than one, but in our case we don't have any cycles

A second user, `nazriel`, confirmed it 2025-08-20:

> I hit the panic `get one group cycle group is not allowed for multy cycles` and when building from source and debugging it turns out I have 0 cycles as well.

**FK-8** — [Greenmask issue #279, 2025-03-21](https://github.com/GreenmaskIO/greenmask/issues/279), still open — a subset panic that a second user re-confirmed with a different trigger on 2026-04-27:

> I hit this panic on a Postgres schema where a composite FK targets non-PK columns on the parent table (i.e. the FK references a column pair that isn't the parent's primary key). Same `index out of range` symptom as this issue, though the trigger is "FK targets non-PK columns" rather than self-reference specifically.

**FK-9** — [dev.to, 2026-08-27](https://dev.to/latryee/how-i-anonymized-relational-sql-dumps-without-breaking-foreign-key-relationships-24jm), `latryee`, explaining why they wrote yet another tool:

> Anonymizing a database dump sounds straightforward until the data is actually relational. You can replace names, emails, phone numbers, and other sensitive values quite easily. The difficult part is keeping the relationships between those values intact.

and:

> we have successfully hidden the original value, but we've also destroyed the relationship. The resulting dataset is much less useful for development and testing.

**FK-10** — [Snaplet seed issue #203, 2024-11-12](https://github.com/supabase-community/seed/issues/203), still open, one +1 from a different user seven months later:

> I have a table which can reference itself. […] However, with the seed utility, i get the error `AssertionError: Node items forms circular dependency: items -> items` and am unable to proceed. Is this not a supported use case?

**FK-11** — [Neosync issue #3227, 2025-02-06](https://github.com/nucleuscloud/neosync/issues/3227), user `Yarn-e`:

> I want to apply a subset filter to this table, and all the tables related to it should apply that subset filter to it. But it looks like my subset filter is not propagated to the tables that have a foreign key to it, as my 'Starting point' table has foreign keys too to other tables.

The maintainer's answer, 2025-02-26:

> Yeah it's a current design choice and system limitation. Foreign Key constraints are effectively directional graphs (DAGs) that follow a parent->child hierarchy.

**FK-12** — [Stack Overflow, 2010-12-21](https://stackoverflow.com/questions/4504140/populate-tables-with-test-data-whilst-maintaining-relational-integrity) — the same request, sixteen years ago:

> I was going to write a script to populate the tables with test data (10-20k rows or more) but I thought I ought to ask if there's something already out there that can generate test data based on the field types but ensure relational integrity at the same time?

**FK-13** — [Greenmask issue #319, 2025-07-21](https://github.com/GreenmaskIO/greenmask/issues/319), maintainer summarising user reports:

> People often say that they have legacy DB without primry keys. Need to implement virtual primary keys in order to define some virtual references on the schema where neither primary key nor foreign keys exists.

**FK-14** — [Greenmask issue #270, 2025-02-26](https://github.com/GreenmaskIO/greenmask/issues/270), on virtual references pointing at non-primary-key columns:

> Users should be able to specify the primary key of the referenced table in a virtual reference so that they can handle non-standard relationships in greenmask.

**FK-15** — [Xata blog, 2026-01-22](https://xata.io/blog/anonymization-the-missing-link-in-dev-workflows) **[vendor]**:

> Random masking breaks foreign keys and constraints and makes databases unusable for testing.

---

## Theme 2 — Leaked PII and masking that silently does nothing

The pattern is consistent and alarming: masking fails *quietly*. The dump succeeds, the exit code is zero, and the cleartext is in the target.

**PII-1** — [Hacker News, 2022-04-28](https://news.ycombinator.com/item?id=31189199), `onion2k`, the single most-cited complaint in this corpus:

> Another major problem with tools like replibyte is that people use them properly, and then a database schema changes, but people don't update their script to anonymize new tables or columns. Then a few months later someone notices sensitive data has made its way in to staging, and into the backups, and the database dumps devs made to debug things because "it's only staging data, who cares!"

**PII-2** — [Replibyte issue #269, 2023-03-28](https://github.com/Qovery/Replibyte/issues/269), open, three separate users hit it:

> Hi, i try to anonymize data on dump but when i restore it on my dev environnement i have real data do i miss something ?

`TheKipmaster`, 2023-05-17:

> I have the same issue. Seemingly correctly configured .yml file outputs exactly the same data. No transformation takes place.

`sealed-rayboutotte` eventually found the cause, 2024-01-12:

> Its the dump format ... if it uses sql statements (i.e. insert) then it should transform the data. But if it uses `COPY public.table (column_a, colum_b) FROM stdin;` **_then data is NOT transformed_**.

A silently-format-dependent masker is the worst possible failure mode: it looks like it worked.

**PII-3** — [PostgreSQL Anonymizer issue #531, 2025-05-09](https://gitlab.com/dalibo/postgresql_anonymizer/-/issues/531) — the same class of bug, opposite direction:

> If the "--inserts" option is used with `dump.sh`, no anonymization is done. […] Probably this is because `pg_dump` will use a Cursor instead of the COPY command when "--inserts" is specified.

**PII-4** — [PostgreSQL Anonymizer issue #668, 2026-08-25](https://gitlab.com/dalibo/postgresql_anonymizer/-/issues/668):

> `anon.anonymize_database_parallel()` fails silently: a perfectly maskable table in the same worker is left in cleartext, and the caller cannot tell the run was incomplete.

and:

> `t1` stays fully in cleartext: the worker's single transaction is rolled back by the `mv1` failure, undoing t1's masking too.

**PII-5** — [Hacker News, 2022-04-28](https://news.ycombinator.com/item?id=31190464), `tjpnz`, reviewing Replibyte's transformers:

> There's a transformer which appears to retain the first char on string fields. That's not safe if you're dealing with customer data.

Elaborated [the next day](https://news.ycombinator.com/item?id=31201089):

> It's not safe because I could potentially use that information to find a real customer in the DB. It becomes more problematic when working with data from Asian countries where it's possible (even common) for family and/or first names to consist of two or even a single character.

**PII-6** — [Hacker News, 2022-04-28](https://news.ycombinator.com/item?id=31190848), `micheljansen`, on why field-level masking is not enough:

> In practice, this means that any realistic production-derived data is either very likely to be still considered PII (and therefore much more demanding to handle safely and securely) or has to be mangled so much that it is no longer representative of production data.

**PII-7** — [Hacker News, 2022-04-28](https://news.ycombinator.com/item?id=31188966), `jlgaddis`:

> I'm (not) looking forward to the future data breach notifications / post-mortems that include something like "... our developers used a tool to copy the production database to a dev database on their laptop ...". Honestly, I'm kinda surprised by the lack of comments advocating against doing this.

**PII-8** — [Hacker News, 2014-08-02](https://news.ycombinator.com/item?id=8123705), `jzwinck`, writing about the MDN database disclosure — an actual leak with this exact cause:

> But a better solution would be to write a system which makes it much less likely to leak private data. For example by copying only whitelisted columns (so if new sensitive columns are added to the system they are not dumped by default).

**PII-9** — [Hacker News, 2019-03-23](https://news.ycombinator.com/item?id=19469128), `Raed667`, answering "what's the dumbest thing you've done on the job?":

> While inbording, i have been given shell script that is supposed to take a copy of the data in production, anonymize it, and set it up locally on my machine. It took 2 parameters, the first one is the IP address of the production database and the second one my IP address. I guess it was inevitable for those to get mixed up, and I guess no one did bother to prevent write access to production.

**PII-10** — [dev.to, 2026-03-24](https://dev.to/jakelaz/how-to-anonymize-pii-in-postgresql-for-development-hb2), Jake Laz **[vendor — canonical on basecut.dev]**, listing the failure modes of the post-restore `UPDATE` script:

> - Someone forgets to run the script, and real data ends up in a dev environment anyway.
> - The script is not versioned with the schema, so it breaks when new PII columns are added.
> - It replaces data inconsistently — the same customer gets a different fake email in `users` than in `audit_logs`, breaking join-based queries.
> - It runs after the fact, which means real data has already traveled through the restore pipeline.
> - It has no automated detection — every new PII column has to be added manually.

The same post, on why a hand-written column list is never complete:

> But in real production schemas, PII hides in less obvious places:
> - free-text fields like `notes`, `description`, `bio` that users fill in
> - `ip_address` columns in event logs and audit tables
> - `stripe_customer_id`, `paypal_email` — identifiers that link back to real people
> - JSONB columns that store user-submitted form data
> - `metadata` fields that accumulate whatever the app was logging at the time

**PII-11** — [gonymizer README](https://github.com/smithoss/gonymizer), a tool built at SmithRx for HIPAA data:

> THERE IS ABSOLUTELY NO GUARANTEE THAT USING THIS SOFTWARE WILL COMPLETE A CORRECT ANONYMIZATION OF YOUR DATA SET FOR COMPLIANCE PURPOSES.

---

## Theme 3 — Config burden

Nobody complains about YAML in the abstract. They complain that the first run demands a complete inventory of a schema nobody in the building fully understands, and that the inventory rots.

**CB-1** — [Hacker News, 2022-04-27](https://news.ycombinator.com/item?id=31187000), `fedeb95`, first reaction to Replibyte's Show HN:

> interesting, however couldn't it detect tables and columns automatically instead of having to specify them in the configuration file? If I understand correctly each table is to be specified by hand. Say I have nearly a hundred tables...

**CB-2** — [Hacker News, 2022-04-28](https://news.ycombinator.com/item?id=31190692), `nicoburns`, on whether the tool beats hand-rolling it:

> I suspect it's likely to take a couple of hours to set up this tool too!

He had [written the pg_dump/pg_restore script by hand the previous day](https://news.ycombinator.com/item?id=31186890): *"it only took a couple of hours to setup (and now I have a repeatable script), so this'll need to be implemented really well to provide value."*

**CB-3** — [Hacker News, 2024-05-22](https://news.ycombinator.com/item?id=40446590), `mathisd`, who worked on a commercial pseudonymisation toolchain:

> Referential constraint refer to ensuring some coherence / basic logic in the output data (ie. the anonymized street name must exist in the anonymized city). This was the most time consuming phase of the pseudonymization process.

and, on why config is impossible to write correctly:

> Also, a lot of the time client had no proper idea of what the field were and what they were truly containing (what format of phone number, we did find a lot of unusual things).

**CB-4** — [Hacker News, 2022-06-03](https://news.ycombinator.com/item?id=31605801), `2rsf`, on test data as a startup opportunity:

> A related problem related to data creation and subsetting is that one person (and practically nobody) knows the entire data relationships between subsystems, but you still need them to create your data.

**CB-5** — [Greenmask issue #321, 2025-07-28](https://github.com/GreenmaskIO/greenmask/issues/321), open — a user asking for safe-by-default behaviour:

> I would love the ability to define a transformer for columns that are not defined in the transformers list […] The use case is to avoid dumping data for columns that do not have a transformer defined so that we can ensure no sensitive information is dumped when new columns are added to tables.

**CB-6** — [Hacker News, 2022-04-28](https://news.ycombinator.com/item?id=31190131), `sverhagen`, independently proposing the same default:

> Like an opt-out mode where you have to specify transformations for all columns unless indicated otherwise. Or at least for text columns.

**CB-7** — [Snaplet seed issue #205, 2024-12-04](https://github.com/supabase-community/seed/issues/205), on a first run that demanded an OpenAI key:

> Quick Start is not so quick if you have to sign up for an open ai account.

and:

> If the user does not want to use AI then in my opinion it would be better to just add a generic projectDescription or skip it rather than shitting the bed here.

**CB-8** — [Stack Overflow, 2015-04-20](https://stackoverflow.com/questions/29757797/expensive-maintenance-with-automated-test-data):

> When there is a change in the model, we take a long time to correct all XML files (we have hundreds of XML files, a lot of them with redundancy). The complexity of creating an XML file manually discourages the programmer to explore different scenarios.

**CB-9** — [Software Engineering SE, 2011-10-10](https://softwareengineering.stackexchange.com/questions/113441/do-we-need-test-data-or-can-we-rely-on-unit-tests-and-manual-testing):

> We have problem with maintaining scripts for test data. The business logic is pretty complex and one "simple" change in the test data often produces several bugs in the application (which are not real bugs, just the product of invalid data). This has become big burden to the whole team because we are constantly creating and changing tables.

**CB-10** — [Neosync issue #3133, 2025-01-13](https://github.com/nucleuscloud/neosync/issues/3133), filed by the maintainers about their own product:

> it's really troublesome right now to find what columns are no longer there in the mapped schema. if a column is configured but isn't there anymore, it's difficult to fix this error `unable to continue: job mappings contain schemas, tables, or columns that were not found in the source connection`

**CB-11** — [Replibyte issue #261, 2023-01-30](https://github.com/Qovery/Replibyte/issues/261) — the config format itself is the limit:

> The database_subset.table is only allow one value.

**CB-12** — [Hacker News, 2019-06-16](https://news.ycombinator.com/item?id=20196993), `davismwfl`, describing the hand-rolled approach that "worked the best":

> This worked really well and yes, takes time initially to setup and takes some time to maintain, but it means you can reproduce a meaningful dataset into any environment for testing or development quickly and automated.

**CB-13** — [DBA Stack Exchange, 2017-03-23](https://dba.stackexchange.com/questions/168023/how-to-anonymize-pg-dump-output-before-it-leaves-server), asking for the tool this project proposes to be, and finding nothing:

> For development purposes, we dump the production database to local. It's fine because the DB is small enough. The company's growing and we want to reduce risks. To that end, we'd like to anonymize the data before it leaves the database server.

and:

> Is there a ready-made solution for this? […] I searched for `postgresql anonymize data dump before download` and variations, but I didn't see anything highly relevant.

---

## Theme 4 — Trust: abandonment, dread, and vetoes

Two flavours: distrust of the tool (will it still exist next year? did it actually mask everything?) and distrust of the practice (should we be doing this at all?).

**TR-1** — [Replibyte issue #277, "Maintained?", 2023-07-21](https://github.com/Qovery/Replibyte/issues/277), open for three years:

> Is this project being maintained or alternatively is it just stable? I don't see any non-doc related changes in 8 months now?

Follow-ups from three different users over the next two years. `macrozone`, 2025-01-24:

> hi @evoxmusic I would also be interested if you still plan to maintain it or hand it over to some other devs! I really like the approach and I don't see many alternatives that are as sophisticated as yours is

`patricktyndall`, 2025-03-19:

> Bumping on this. Would love to know if this is the right choice. Currently can't get `subset` working at all, so a little discouraged but I love the approach.

**TR-2** — [Neosync issue #3565, 2025-08-25](https://github.com/nucleuscloud/neosync/issues/3565):

> Hey, just wondering what the post acquisition plan is for this project? Is it going to continue to be maintained? Handed over to the community? Shuttered?

Maintainer `nickzelei`, same day:

> There are no current plans for any continued maintenance. We are still working with the acquirer on handing them the repository but ultimately it will be up to them as to what they decide to do with the repository. If you're using Neosync I'd recommend forking it at the current time and making any changes you wish.

**TR-3** — [Supabase blog, 2024-08-14](https://supabase.com/blog/snaplet-is-now-open-source), quoting Snaplet founder Peter Pistorius on the shutdown:

> Although the company is closing, my belief remains strong, so we are open-sourcing the tools we've built.

**TR-4** — [Hacker News, 2022-04-28](https://news.ycombinator.com/item?id=31189199), `onion2k`:

> Protecting user data is something that you need to be extremely vigilant about. In my experience, the less access I have to production data the happier I am. Copying it and using it in staging, even if you're careful about it, fills me with dread.

**TR-5** — [Hacker News, 2022-04-28](https://news.ycombinator.com/item?id=31190734), `nicoburns`:

> If syncing prod data then I'd definitely want to have very thorough filtering. But then at that point I'm not sure I'd trust this tool!

**TR-6** — [Hacker News, 2022-04-28](https://news.ycombinator.com/item?id=31189830), `krageon`:

> Unless you can exhaustively guarantee your customer-data containing production data will definitely be transformed into something completely unrecognisable and irreversible (and let's face it, you can never do so - systems change all the time), using this is irresponsible.

**TR-7** — [ThoughtWorks Technology Radar, "Production data in test environments", Hold, last updated 2022-03-29](https://www.thoughtworks.com/radar/techniques/production-data-in-test-environments) — surfaced in the Replibyte thread by `time4tea` as an argument against the whole category:

> There is little point in having elaborate controls around access to production data if that data is copied to a test database that can be accessed by every developer and QA. Although you can obfuscate the data, this tends to be applied only to specific fields, for example, credit card numbers.

and:

> We do recognize there are reasons for specific elements of production data to be copied, for example, in the reproduction of bugs or for training of specific ML models. Here our advice is to proceed with caution.

**TR-8** — [Hacker News, 2026-03-05](https://news.ycombinator.com/item?id=47259769), `patpatpat`, on the dbslice thread — the gate is social, not technical:

> I made one of these, however I still have to solve the PII issues convince the data custodians that it's safe to use.

**TR-9** — [Hacker News, 2026-03-05](https://news.ycombinator.com/item?id=47262861), `semiquaver`, on the cost of the ban:

> When I moved to big tech the rules against doing this were honestly one of the biggest drivers of reduced velocity I encountered. Many, many bugs and customer issues are very data dependent and can't easily be reproduced without access to the actual customer data. […] I think it's under-appreciated how much it slows down the real world progress of fixing customer-reported issues.

**TR-10** — [Hacker News, 2022-04-29](https://news.ycombinator.com/item?id=31201089), `tjpnz`, on why an opt-out flag is not enough:

> With regards to telemetry I'm aware that it can be disabled. But in my experience that would still result in a veto from the security teams I've worked with.

**TR-11** — [Greenmask issue #465, 2026-07-10](https://github.com/GreenmaskIO/greenmask/issues/465), on `validate` passing and `restore` failing:

> The dump-succeeds/restore-fails failure mode is the most expensive kind: in a scheduled pipeline the bad artefact is produced and stored successfully, and the breakage is only discovered by a downstream consumer, potentially much l[ater]

**TR-12** — [dev.to, 2025-10-28](https://dev.to/alex_hayward_ff2d88ff331e/building-a-test-data-platform-after-watching-teams-secretly-use-production-for-years-31b6), GoMask co-founder **[vendor]**, quoting a Reddit reply he received. **Second-hand — the original Reddit comment could not be fetched from this environment:**

> Everywhere I've worked with sensitive data, everybody ended up secretly working off prod.

His own gloss:

> So everyone takes the path of least resistance. Quietly use production data and hope compliance doesn't dig too deep.

---

## Theme 5 — Speed and resource cost

Two separate complaints get conflated: wall-clock time of a run, and memory blowing up on databases that are not large.

**SP-1** — [Replibyte issue #264, 2023-03-01](https://github.com/Qovery/Replibyte/issues/264), open, on a 15 GB source:

> I've observed network activity below 2MB/s while running replibyte. Eventually, the process is killed

Confirmed by a second user, `ikegentz`, 2023-08-22:

> Confirming the same issue, doesn't seem to matter where replibyte is running, I also get ~2MB/s and then the process is killed. FWIW I'm seeing this on mysql

**SP-2** — [Replibyte issue #293, 2024-01-30](https://github.com/Qovery/Replibyte/issues/293), on restoring 8 million rows / 4 GB:

> I started yesterday around 1pm EST and it it's now 10am EST the following day and it only has about 1.7million records in the replica db

**SP-3** — [Replibyte issue #268, 2023-06-14](https://github.com/Qovery/Replibyte/issues/268), comment by `maxleroy`:

> Same here, my database weighs less than 200Mb, and still I get OOM killed with 3GB of memory used by replibyte...

**SP-4** — [Replibyte issue #244, 2022-12-09](https://github.com/Qovery/Replibyte/issues/244), subsetting a ~250 MB database:

> the database is in total around 250mb (148mb if asking postgres with: `SELECT pg_size_pretty( pg_database_size('dbname') )`) and 51gb of memory taken up seems excessive.

**SP-5** — [Replibyte issue #289, 2023-12-24](https://github.com/Qovery/Replibyte/issues/289), on Pagila, a small sample database:

> the dump takes hours while on linux takes few minutes

**SP-6** — [Hacker News, 2022-04-27](https://news.ycombinator.com/item?id=31186965), `tehlike`, replying to someone who said hand-rolling took a couple of hours:

> I am on the same boat but couple hours is terrible still. The best is probably copying the data directory straight which should cut it down to seconds, but i have yet to automate that + there are production credentials/sensitive data problems that needs to be tackled too...

**SP-7** — [PostgreSQL Anonymizer issue #507, 2025-01-20](https://gitlab.com/dalibo/postgresql_anonymizer/-/issues/507):

> We observed that the `anon.anonymize_table()` function runs sequentially, which increases memory and I/O load for large tables.

**SP-8** — [PostgreSQL Anonymizer issue #637, 2026-05-13](https://gitlab.com/dalibo/postgresql_anonymizer/-/issues/637), on the cost of doing FK-correct grouping:

> as I saw `anon.dispatch` will look at all tables with reference recursively, this can be slow even on a small DB where almost all tables are linked to one another

**SP-9** — [Xata blog, 2026-01-22](https://xata.io/blog/anonymization-the-missing-link-in-dev-workflows) **[vendor]**, describing the workflow it is selling against:

> Production database snapshot (2-8 hours for 1TB+) Restore to staging environment (2-8 hours) Run Python scrubbing script (1-4 hours) Developer access (data already 12+ hours stale)

and:

> In reality, because the process takes so much time, its common for staging to lag production by weeks or months.

**SP-10** — [Hacker News, 2021-05-01](https://news.ycombinator.com/item?id=27007696), `john-tells-all`, in "Ask HN: How Long Is Your CI Process?":

> Speed up databases. Move from "install database and sample data interactively every time" to having a pre-baked Docker image with the database and seed data. Much faster: you get lower LAG and the same VALUE for the team.

---

## Theme 6 — Unsupported database, platform, or type

Almost always the same shape: "this is exactly what I need, and then I found it doesn't do X."

**DB-1** — [Hacker News, 2022-04-28](https://news.ycombinator.com/item?id=31189054), `dvasdekis`:

> I was thinking "oh! this is awesome!", and then noticed it didn't support MSSQL. Not to worry, I'll just contribute a connector. Let's take a look at their existing connector code... Not a single comment to say what anything does. Sigh. It's the same for the other drivers too.

**DB-2** — [Replibyte issue #263, 2023-02-07](https://github.com/Qovery/Replibyte/issues/263):

> Your project is awesome, however our main database is Cassandra. Sadly I do not (yet) develop with Rust so I cannot create a PR for that.

**DB-3** — [Replibyte issue #260, 2023-01-26](https://github.com/Qovery/Replibyte/issues/260):

> I would like to see CockroachDB as a supported database. It's nearly pg compatible but pg_dump does not work afaik.

**DB-4** — [Greenmask issue #222, "epic: MySQL support", opened 2024-10-15](https://github.com/GreenmaskIO/greenmask/issues/222) — still open on 2026-09-04, nearly two years later. Maintainer, 2025-04-15:

> The work turned out to be more complex than we thought, so we've had some delays.

**DB-5** — [Neosync issue #3411, 2025-03-26](https://github.com/nucleuscloud/neosync/issues/3411):

> The issue is specific to MySQL 5.7. When attempting the same setup with MySQL 8.0.40, also on AWS RDS, the connection is established successfully without any errors.

**DB-6** — [Greenmask issue #377, 2025-12-12](https://github.com/GreenmaskIO/greenmask/issues/377):

> I see there are windows binaries, but is windows actually fully supported? The default tmp_dir doesn't exist on windows and when I set it to an existing directory I get file locking error.

**DB-7** — [Hacker News, 2025-06-18](https://news.ycombinator.com/item?id=44310202), `Brycee`, on the VeilStream Show HN:

> How do you handle connection pooling? Does this interfere with pgbouncer or similar tools? Also, does this work with all PostgreSQL extensions (PostGIS, timescaledb, etc.)?

Founder's answer:

> PostGIS and other extensions are on the radar, but currently are not supported. The proxy works with the extensions, but can't mask the data yet.

**DB-8** — [Hacker News, 2025-06-18](https://news.ycombinator.com/item?id=44311946), `Ksbt`, in the same thread, on types rather than engines:

> What data types can Veilstream handle? Like can I mask nested jsonb, uuids, IP addresses, arrays? Would be wild if adding new filters was fast enough to support weird internal schemas or bespoke pii.

The answer was "kinda", "no, but I should", "yes", and "not yet" respectively.

**DB-9** — [Greenmask issue #444, 2026-05-11](https://github.com/GreenmaskIO/greenmask/issues/444), open:

> greenmask dump panics when processing the settings table (129 columns). Greenmask's COPY format decoder uses a fixed [128] array internally. Any table with ≥129 columns causes an index-out-of-range panic.

**DB-10** — [Snaplet seed issue #193, 2024-08-07](https://github.com/supabase-community/seed/issues/193), open, with a macOS +1 in 2025:

> I try to run `npx @snaplet/seed init` and got this error

**DB-11** — [Replibyte issue #310, 2025-09-25](https://github.com/Qovery/Replibyte/issues/310), open — the install path itself:

> Cannot build from source

---

## Theme 7 — CI integration

Thinner than the others, which is itself a finding: most people are not yet at the point of wiring this into CI, because they are still stuck on the earlier themes. What CI complaints exist are about exit codes, config discovery, install, and scheduling.

**CI-1** — [Greenmask issue #454, 2026-06-08](https://github.com/GreenmaskIO/greenmask/issues/454):

> For incorporating a greenmask configuration into a project, it would be helpful if CI could be configured to fail if there are any validation warnings at all - this would be functionally equivalent to `gcc -Werror`.

Shipped as `--strict` in v0.2.22, 2026-07-01.

**CI-2** — [Greenmask issue #455, 2026-06-09](https://github.com/GreenmaskIO/greenmask/issues/455), from the same user, one day later:

> Supporting something like `GREENMASK_CONFIG` would make it easier to iterate on greenmask commands/configs without having to re-type or alias it locally. I would love to be able to automatically configure this for our developers' environments.

**CI-3** — [Greenmask issue #333, 2025-08-15](https://github.com/GreenmaskIO/greenmask/issues/333) — install friction, offered as a PR by a user:

> Users could then run: `curl -fsSL https://greenmask.io/install.sh | sh`

**CI-4** — [Replibyte issue #242, 2022-11-27](https://github.com/Qovery/Replibyte/issues/242), open:

> Is there any built-in (or a workaround) config to make the backups and restorations automatic? I want to run Replibyte on docker and assign a source and datasource to it, so it can periodically backup the postgres database and upload it to S3

Answered by another user, not a maintainer, 2023-01-16:

> I'm not a replibyte contributor, but I don't believe this feature is supported by replibyte. I would recommend using cron or a CI tool like Github actions or buildkite to accomplish this.

**CI-5** — [Hacker News, 2022-12-22](https://news.ycombinator.com/item?id=34095802), `btown`, describing a working per-PR setup:

> within the specific preview namespace named after the PR ID, spins up and seeds with test data (in our case, a sanitized subset of production) a dedicated database statefulset

**CI-6** — [Hacker News, 2022-12-23](https://news.ycombinator.com/item?id=34103230), `nunez`, on why most teams do not get there:

> Data is an example of a challenging hurdle. It is (somewhat) straightforward to Terraform production and modularize it to make it repeatable. But what do you do when your most current tables have customer PII or other sensitive data in them and migrations are done manually during release? Now you need to audit the entire database for fields where PII might exist so that automation can be written to dump those databases and sanitize that data.

**CI-7** — [Hacker News, 2024-06-17](https://news.ycombinator.com/item?id=40707082), `MajimasEyepatch`, in "How to test without mocking":

> The test data is a harder problem to solve. […] However, this can get really nasty if there's a lot of dependencies between your tables. […] You can attempt to anonymize production data, but obviously that can go very wrong.

**CI-8** — [Hacker News, 2013-02-27](https://news.ycombinator.com/item?id=5295135), `Domenic_S`, describing the hand-built version everyone ends up with:

> Replace all personal user data with placeholders. This part can be tricky, because you have to find everywhere this lives (are form submissions stored and do they have PII?)

and the payoff:

> We've got a working, sanitized database dump ready and waiting every morning, and a fresh prod-like environment built for us when we log on. It's a beautiful thing.

---

## Frequency ranking

Ranked strictly by the number of numbered entries above — one entry is one distinct source (a person, an issue, a post) raising that theme. The counts are reproducible from the document: `grep -cE '^\*\*FK-[0-9]+\*\* — ' COMPLAINTS.md` and so on. Vendor-authored entries are counted but flagged in their own column.

| Rank | Theme | Entries | Vendor-authored | Sharpest single quote |
|---:|---|---:|---:|---|
| 1 | Broken foreign keys / referential integrity | 15 | 3 | *"To the point where it was almost impossible to do this"* (FK-3) |
| 2 | Config burden | 13 | 1 | *"Say I have nearly a hundred tables..."* (CB-1) |
| 3 | Trust: abandonment, dread, security veto | 12 | 2 | *"Copying it and using it in staging, even if you're careful about it, fills me with dread."* (TR-4) |
| =4 | Leaked PII / masking silently no-ops | 11 | 2 | *"i try to anonymize data on dump but when i restore it on my dev environnement i have real data"* (PII-2) |
| =4 | Unsupported database / platform / type | 11 | 0 | *"and then noticed it didn't support MSSQL"* (DB-1) |
| 6 | Speed and memory | 10 | 1 | *"my database weighs less than 200Mb, and still I get OOM killed with 3GB"* (SP-3) |
| 7 | CI integration | 8 | 0 | *"you have to find everywhere this lives"* (CI-8) |

Notes on the ranking, including where raw count and severity disagree:

- **Rank 1 and rank 4 (leaked PII) are the same complaint wearing different hats.** Both are "the tool did something to the data that I could not see and could not verify". FK breakage is loud (a constraint violation on load); PII leakage is silent (a successful run). Frequency ranks FK higher, but severity ranks PII higher, because you find out about the FK bug in minutes and about the PII bug in months. If lazysnap does one thing better than the field, make both *checkable after the run*.
- **Trust is ranked third by count and would rank higher on structural evidence.** Three of seven well-known tools in the table above are archived or dead. A developer choosing a tool in 2026 has watched Snaplet shut down, Neosync be acquired and archived, and Replibyte go four years without a release. "Why will you still exist in two years" is a real question for a new entrant, and a single static binary with no service dependency is a partial answer to it.
- **CI is last on count but not on importance.** It is under-represented because most complainants have not yet cleared ranks 1–4. The CI complaints that do exist are unglamorous and cheap to satisfy: a non-zero exit code on validation failure, config discoverable from an env var, a one-line install.
- **Themes overlap.** PII-1 (schema drift → leaked column) is as much a config-burden complaint as a leaked-PII one; I filed it once, under PII, rather than double-counting. That choice suppresses the config-burden count slightly. The ordering of ranks 2 through =4 is within noise of a different filing decision; the gap between rank 1 and rank 7 is not.

---

## What the CONCEPT.md principles address, and what they do not

Read against [CONCEPT.md](../CONCEPT.md).

### Directly addressed

**"Zero config. First run asks at most one question and then works. Configuration is emitted after a run as a record of what happened, never demanded before it."**

This is a direct hit on Theme 3, config burden (13 entries, second by frequency), and a partial hit on Theme 2, leaked PII. CB-1, CB-5, CB-6 and PII-10 are all asking for exactly this inversion: don't make me enumerate the schema, and don't let an un-enumerated column ship in cleartext. Emitting `lazysnap.yml` after the run also answers CB-2's "the hand-rolled script took two hours and now I have a repeatable script" — you get the repeatable artefact without the two hours.

**"Anything that might be personal data is masked unless the user opts a column out, and the tool explains why it masked each one."**

Deny-by-default is the fix PII-8 (`jzwinck`, on the MDN leak) named in 2014 and CB-5 filed against Greenmask in 2025. It is the single most-requested behaviour in Theme 2. The "explains why" half also does real work on Theme 4, trust: TR-8's *"convince the data custodians that it's safe to use"* is a request for an artefact you can hand to a person, and a per-column justification is that artefact.

**"We refuse to ship a flag that disables masking wholesale."**

Answers TR-6 and TR-10 — the objection is not "can it be turned off" but "can a security team be confident it wasn't". Note that TR-10 is specifically about a *disable-able* feature still failing review. The absence of the flag is the point.

**"Verifies foreign-key integrity in the target."**

This is the answer to the loud half of Theme 1, and it is the right shape: verify in the target, after load, not by trusting the plan. FK-6, FK-7, FK-8 and FK-10 are all cases where a tool believed its own plan and produced an unloadable artefact. TR-11 names why post-hoc verification matters more than pre-hoc validation: *"The dump-succeeds/restore-fails failure mode is the most expensive kind."*

**"Parents to completeness, children with caps, cycles handled."**

Names the three things that break in FK-6/7/8/10. "Cycles handled" is doing a lot of load-bearing work in one word; FK-7 shows a tool that crashed on the *absence* of cycles, which is the kind of bug that only surfaces when you test both branches.

**"One static binary, one-line install."**

Answers CI-3, DB-6, DB-10 and DB-11 directly. Four separate sources across four tools failed at install or on an unexpected platform.

**"The same command a human types works headless in CI."**

Answers CI-1, CI-2 and CI-4. These are cheap to satisfy and the concept already commits to them.

**"It never holds write access to the source."**

Answers PII-9 — the developer who swapped the source and destination IP arguments. Structural, not a warning in the docs.

### Partly addressed, with gaps

**Determinism across tables.** CONCEPT.md says "masks them deterministically so joins still work". That covers FK-9 and PII-10's third bullet. What it does not say is *deterministic across runs*, which is a different property and is what makes a snapshot diffable and a bug report reproducible. Worth being explicit about which one is promised. [Greenmask #325](https://github.com/GreenmaskIO/greenmask/issues/325) is a bug where the hash engine was not deterministic in the way users assumed.

**"Columns that look like personal data."** Classification is where PII-6 lands: *"any realistic production-derived data is either very likely to be still considered PII […] or has to be mangled so much that it is no longer representative."* Column-level classification cannot catch PII inside a free-text `notes` field or a JSONB blob (PII-10 lists both). The non-goal list excludes binary blobs but is silent on text and JSONB. That silence is a gap, not a decision.

**Speed.** The concept's example run is 38 seconds for 500 customers, which is the right target, but no principle commits to bounded memory. Theme 5 shows that memory, not time, is what actually kills these runs: SP-3 (200 MB DB, 3 GB RSS, OOM), SP-4 (250 MB DB, 51 GB RSS). "Streams the rows out" implies constant memory but does not promise it. Promise it.

### Not addressed

**Trust as a durability question (Theme 4, third by frequency).** Nothing in CONCEPT.md speaks to "will this still be maintained". The static-binary, no-service, no-cloud-dependency choice is a real structural answer — an abandoned static binary keeps working, an abandoned SaaS does not — but it is currently implicit. The three-of-seven-tools-archived fact is the strongest argument for the terminal-first, no-hosted-service shape, and it is worth stating as such.

**Theme 6, unsupported database, by explicit choice.** The non-goals list Postgres-only for v1. That is a defensible scope decision and I am not arguing against it, but it should be understood that it declines to serve 11 entries' worth of complaint, and that DB-4 (Greenmask's MySQL epic, open for ~2 years) shows the second engine is far more expensive than it looks. The concept's phrasing — "after the Postgres path is excellent" — is the right hedge.

**Schema drift over time.** PII-1 is the highest-signal single complaint in the corpus and it is a *longitudinal* failure: the config was correct when written and wrong three months later. CONCEPT.md's answer is "configuration is emitted after a run", which helps because each run re-derives classification rather than replaying a stale file. But if `lazysnap.yml` is committed and used in CI (as step 5 suggests), the committed file *is* the stale config, and the drift problem returns. There is no stated behaviour for "the schema has a column your committed config has never seen". The safe answer — mask it and fail loudly rather than pass it through — is not written down anywhere. This is the most important gap.

**Verification of masking, as distinct from verification of foreign keys.** Step 5 verifies FK integrity in the target. Nothing verifies that no cleartext PII reached the target. PII-2, PII-3 and PII-4 are three separate tools where masking silently no-opped and the run reported success. An after-the-fact scan of the target for residual PII is the analogue of the FK check, and it is absent.

**Anything for the person who is not allowed to do this at all.** TR-9 and TR-8 describe developers blocked by policy, not by tooling. The concept has no story for producing an auditable artefact a compliance function can sign off once, after which runs are self-service. dbslice's maintainer is building exactly this ("compliance profiles", "audit manifest", "signed-off config") and `patpatpat`'s reply — *"Sounds fantastic"* — suggests demand. Out of scope for v1, but worth an entry in the tracker.

---

## Gaps in this research

- **Reddit is unreachable from this environment.** r/ExperiencedDevs, r/dataengineering and r/PostgreSQL are where a lot of this complaining happens, and TR-12 is the only Reddit-origin material here, arriving second-hand through a vendor blog. This is the biggest hole.
- **No private / paid-tool complaints.** Tonic, Delphix, Redgate and Broadcom TDM customers complain in support portals and on Gartner Peer Insights, neither of which is publicly quotable. The paid tier of this market is invisible to this method, and it is plausible that its complaints differ (more "the license is expensive", less "the binary won't build").
- **Stack Overflow's full-text search is poor for this topic.** The `anonymization` tag is effectively empty on Stack Overflow; `database-testing` and `test-data` are dominated by fixture-framework questions from 2010–2015. Most of the useful Stack Exchange material was on Software Engineering SE and DBA SE, and there is not much of it. This corpus therefore skews toward HN and GitHub, which skews toward people who evaluate open-source tools rather than people who write a scrubbing SQL script and never mention it.
- **Survivorship bias in bug trackers.** Every complaint here came from someone who cared enough to file or post. The developer who ran `pg_dump | sed` and moved on is unrepresented, and that developer is probably the modal case.
- **No quantitative baseline.** I found no public survey with a sample size on "how do teams get data into dev/CI databases". Percentages anywhere in this space should be treated as unsourced until one is found.
