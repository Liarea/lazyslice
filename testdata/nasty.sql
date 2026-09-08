--
-- nasty.sql: the hostile fixture.
--
-- Every object in this file is a trap. testdata/README.md names each one and
-- states the behaviour lazyslice must show for it; nothing here is decoration,
-- and a trap that stops being interesting should be deleted from both files at
-- once rather than left to rot.
--
-- Loading it:
--
--     psql -f testdata/nasty.sql            -- fast; public.stream_rows and
--                                            -- public.stream_docs are both
--                                            -- created empty
--     psql -v big=1 -f testdata/nasty.sql   -- also fills stream_rows
--                                            -- (2,000,000 rows) and
--                                            -- stream_docs (1,000,000)
--
-- or, from Go, internal/testutil.LoadNasty(ctx, url, big), which cuts the gate
-- off the end of this file and issues the statements inside it itself when big
-- is set. Both fills are gated because every test that is not about streaming
-- pays for them otherwise.
--
-- The file is written to run on PostgreSQL 14 through 18 (ADR-003), with no
-- extensions and no superuser: everything here is core SQL that an ordinary
-- owner of the database can create.
--

SET client_min_messages = warning;

--
-- Trap: a schema other than public.
--
-- Nothing may assume search_path. Every table lazyslice reports, plans, masks
-- or loads is named (schema, name), and billing.invoices is the row that proves
-- an unqualified name would have collided or been missed.
--
CREATE SCHEMA billing;

--
-- Trap: an enum type.
--
-- A user-defined enum is not a text column: the target must have the type
-- before the table that uses it, the classifier must not sample it as free
-- text, and a masker that replaced a value with an arbitrary string would fail
-- the load with 22P02.
--
CREATE TYPE public.account_status AS ENUM ('pending', 'active', 'suspended', 'closed');

--
-- Trap: an enum the classifier will actually flag.
--
-- account_status above takes no name hit and no validator hit, so it
-- classifies at `none` and is copied: it proves the type exists in the target
-- and that nothing samples it as free text, and nothing else. marital_status
-- is the other half. The column name is a special category by name alone, so
-- ARCHITECTURE.md section 4 scores it `certain` and section 5 has to produce a
-- member label rather than an arbitrary string -- and with six labels the
-- domain is small, so section 5's "substitution over a small alphabet is not a
-- mask" rule collapses it to one fixed label instead.
--
CREATE TYPE public.marital_status AS ENUM
    ('single', 'married', 'civil_partnership', 'divorced', 'widowed', 'undisclosed');


--
-- people: the trap table.
--
-- Traps, in column order:
--   person_id      GENERATED ALWAYS AS IDENTITY, non-default start and increment
--   manager_id     self-referencing foreign key
--   display_name   generated column, derived from two columns that get masked
--   email_verified boolean whose name says "email" (false positive)
--   ref            text whose name says nothing and holds email addresses
--                  (false negative)
--   status         enum that nothing flags: copied, and it proves the type
--   marital_status enum that a special-category name rule flags: masked, and
--                  the mask must be a member label
--   contact        jsonb with an email and a phone two levels deep
--   alt_emails     text[] of email addresses, with a NULL element and an
--                  empty array so "same length, NULL preserved" is testable
--   notes          free text carrying full names and a phone number
--
CREATE TABLE public.people (
    person_id      bigint GENERATED ALWAYS AS IDENTITY (START WITH 90000 INCREMENT BY 7) PRIMARY KEY,
    manager_id     bigint REFERENCES public.people (person_id),
    given_name     text NOT NULL,
    family_name    text NOT NULL,
    display_name   text GENERATED ALWAYS AS (given_name || ' ' || family_name) STORED,
    email_verified boolean NOT NULL DEFAULT false,
    ref            text,
    status         public.account_status NOT NULL DEFAULT 'pending',
    marital_status public.marital_status NOT NULL DEFAULT 'undisclosed',
    contact        jsonb NOT NULL DEFAULT '{}'::jsonb,
    alt_emails     text[],
    notes          text
);

--
-- orders and order_items, plus people.preferred_order_id.
--
-- Trap: one table reached both as a parent and as a child.
--
-- ARCHITECTURE.md section 3 fixes the outcome: people.preferred_order_id
-- pushes orders as PARENT_ONLY, orders.person_id pushes the same rows as
-- CHILD_OK, and under FIFO the parent batch is popped first. A person's
-- preferred order that is also one of their own orders must be expanded once,
-- as a parent, and must not pull its order_items through that batch; the same
-- rows arrive again in the child batch, are already selected, and are skipped.
-- Two runs over one snapshot must produce identical selected sets.
--
-- The pair is also a two-table foreign-key cycle in its own right.
--
CREATE TABLE public.orders (
    order_id  bigint GENERATED BY DEFAULT AS IDENTITY (START WITH 200000 INCREMENT BY 3) PRIMARY KEY,
    person_id bigint NOT NULL REFERENCES public.people (person_id),
    placed_at timestamp with time zone NOT NULL,
    total_pence integer NOT NULL
);

CREATE TABLE public.order_items (
    order_id bigint NOT NULL REFERENCES public.orders (order_id),
    line_no  integer NOT NULL,
    sku      text NOT NULL,
    qty      integer NOT NULL,
    PRIMARY KEY (order_id, line_no)
);

ALTER TABLE public.people
    ADD COLUMN preferred_order_id bigint REFERENCES public.orders (order_id);


--
-- Trap: a composite primary key with a composite foreign key to it.
--
-- Key sets are pairs, not scalars. Every chunked read, every IN list and every
-- unnest of the plan has to carry both columns in the same order, and a
-- one-column shortcut anywhere produces a slice that is silently wrong rather
-- than an error.
--
-- tenant_user_sessions.user_id is nullable on purpose, and session 5044 has a
-- tenant with no user. Under MATCH SIMPLE -- the default, and what the
-- constraint below is -- a composite foreign key with any NULL component
-- references nothing at all, which is why ARCHITECTURE.md section 3's parent
-- step reads "WHERE for all c in fk.ChildCols: c IS NOT NULL". An
-- implementation that writes ANY instead of ALL pulls a tenant_users row for
-- that session that the row does not actually reference; one that drops the
-- predicate altogether can miss a parent and leave invariant I1 broken.
--
-- owner_person_id connects this component to public.people. Without it,
-- tenant_users, tenant_user_sessions and tenant_user_flags are unreachable
-- from the root every invariant slices from, every one of them is
-- Step{t, SchemaOnly}, and traps 4, 5 and 18 are never exercised by a run at
-- all: no chunked read, no COPY, no masking, no residual scan, zero rows in
-- the target. It is deliberately NOT NULL, so the edge is mandatory and the
-- component is always pulled.
--
CREATE TABLE public.tenant_users (
    tenant_id       integer NOT NULL,
    user_id         integer NOT NULL,
    owner_person_id bigint NOT NULL REFERENCES public.people (person_id),
    email           text NOT NULL,
    joined_on       date NOT NULL,
    PRIMARY KEY (tenant_id, user_id)
);

--
-- Trap: type signals with no name to help.
--
-- `origin` is inet and its name says nothing at all -- no name rule in any
-- language recognises it -- so the only thing that can classify it is the
-- type plus net.ParseIP over the samples. `adapter` is macaddr, named in
-- ARCHITECTURE.md section 4's v1 type-signal list and otherwise without a
-- fixture. 2001:db8::1 is in origin so that a masker which only understands
-- dotted quads fails visibly. The name-hit half of the same signal lives on
-- public.audit_log.client_ip, so the two branches can be told apart.
--
CREATE TABLE public.tenant_user_sessions (
    session_id bigint GENERATED BY DEFAULT AS IDENTITY (START WITH 5000 INCREMENT BY 11) PRIMARY KEY,
    tenant_id  integer NOT NULL,
    user_id    integer,
    started_at timestamp with time zone NOT NULL,
    origin     inet,
    adapter    macaddr,
    FOREIGN KEY (tenant_id, user_id) REFERENCES public.tenant_users (tenant_id, user_id)
);


--
-- Trap: the same composite foreign key, MATCH FULL.
--
-- ForeignKey.MatchFull is a field in ARCHITECTURE.md section 2, so it needs a
-- fixture. MATCH FULL is the opposite rule to the one above: a row must have
-- all of the referencing columns NULL or none of them, so flag 9004 (both NULL)
-- is legal and references nothing, and a half-NULL row cannot exist at all.
-- Introspection must report confmatchtype 'f' here and 's' on
-- tenant_user_sessions, because the two constraints have different meanings on
-- the same pair of columns.
--
CREATE TABLE public.tenant_user_flags (
    flag_id   bigint GENERATED BY DEFAULT AS IDENTITY (START WITH 9000 INCREMENT BY 2) PRIMARY KEY,
    tenant_id integer,
    user_id   integer,
    flag      text NOT NULL,
    CONSTRAINT tenant_user_flags_tenant_user_fkey
        FOREIGN KEY (tenant_id, user_id) REFERENCES public.tenant_users (tenant_id, user_id) MATCH FULL
);


--
-- Trap: a three-table foreign-key cycle.
--
-- organisations -> teams -> projects -> organisations. A traversal that has no
-- visited set does not terminate here, and a loader that orders tables by
-- dependency has no valid order at all: ARCHITECTURE.md section 11.1 requires
-- data first and foreign keys after, which is the only thing that makes this
-- loadable.
--
-- The columns are nullable so the rows can be inserted at all; the cycle is
-- closed by the UPDATEs further down, and organisations.primary_team_id is set
-- NOT NULL afterwards so that at least one edge of the cycle is mandatory.
--
-- founded_by connects the cycle to public.people, for the same reason
-- tenant_users.owner_person_id does: an unreachable cycle is SchemaOnly, and
-- "the planner must terminate, the loader must succeed, and primary_team_id is
-- NOT NULL so leaving the cycle-closing column null is not an escape" are all
-- vacuous on a zero-row load. The three-table cycle itself is untouched.
--
CREATE TABLE public.organisations (
    organisation_id integer GENERATED BY DEFAULT AS IDENTITY (START WITH 500 INCREMENT BY 3) PRIMARY KEY,
    name            text NOT NULL,
    founded_by      bigint NOT NULL REFERENCES public.people (person_id),
    primary_team_id integer
);

CREATE TABLE public.teams (
    team_id         integer GENERATED BY DEFAULT AS IDENTITY (START WITH 600 INCREMENT BY 3) PRIMARY KEY,
    name            text NOT NULL,
    lead_project_id integer
);

CREATE TABLE public.projects (
    project_id            integer GENERATED BY DEFAULT AS IDENTITY (START WITH 700 INCREMENT BY 3) PRIMARY KEY,
    name                  text NOT NULL,
    owner_organisation_id integer
);

ALTER TABLE public.organisations
    ADD CONSTRAINT organisations_primary_team_id_fkey
    FOREIGN KEY (primary_team_id) REFERENCES public.teams (team_id);

ALTER TABLE public.teams
    ADD CONSTRAINT teams_lead_project_id_fkey
    FOREIGN KEY (lead_project_id) REFERENCES public.projects (project_id);

ALTER TABLE public.projects
    ADD CONSTRAINT projects_owner_organisation_id_fkey
    FOREIGN KEY (owner_organisation_id) REFERENCES public.organisations (organisation_id);


--
-- Trap: a polymorphic association with no constraint.
--
-- owner_type names a table and owner_id names a row in it, and PostgreSQL
-- knows none of that. lazyslice infers the pair instead of following a
-- declared constraint (ARCHITECTURE.md section 3.2, amended 2026-09-08,
-- T-POLY): each distinct owner_type value is sampled and mapped to a table,
-- and each mapping becomes a virtual, parent-direction foreign key the plan
-- follows and prints under plan.polymorphic.inferred. A value no resolution
-- form maps to a table is reported once as unmapped rather than guessed.
--
-- Two resolution forms are exercised. 'people' and 'projects' are raw table
-- names, resolved by the fallback that tries the value un-pluralised and
-- un-mangled after the Rails form fails; 'Person' (attachment 845, below) is
-- Rails-spelled and resolves through the first form tried — underscore and
-- pluralise, 'Person' -> 'people'.
--
-- attachment 839 points at a row that does not exist; the edge is followed in
-- the parent direction only, so the dangling id is never pulled in as a
-- child and the row stays out of the slice.
--
-- uploaded_by_person_id is a real, declared edge into public.people, next to
-- the inferred pair. It is what puts these rows inside the slice regardless of
-- the pair, and that makes the trap stronger rather than weaker: the row is
-- selected through the declared edge, the polymorphic parent is separately
-- followed and reaches a row (public.projects 700, through attachment 826)
-- the declared edge does not, and the run says so. uploaded_by_person_id is
-- nullable, and attachment 839 leaves it NULL, so the dangling polymorphic
-- owner is still in the fixture on a row the slice does not reach through the
-- declared edge.
--
CREATE TABLE public.attachments (
    attachment_id        bigint GENERATED BY DEFAULT AS IDENTITY (START WITH 800 INCREMENT BY 13) PRIMARY KEY,
    owner_type           text NOT NULL,
    owner_id             bigint NOT NULL,
    uploaded_by_person_id bigint REFERENCES public.people (person_id),
    filename             text NOT NULL,
    uploaded_by          text
);


--
-- Trap: a partitioned table with two partitions.
--
-- The root is relkind 'p' and holds no rows of its own. TABLESAMPLE is refused
-- on it, so the classifier samples the largest leaf by reltuples (events_2024
-- here, deliberately larger than events_2025), attributes the samples to the
-- root, and says "samples from partition events_2024" in the explanation. The
-- planner and the loader address the root; nothing addresses a leaf by name.
--
-- The primary key has to include the partition key, which is why it is
-- (event_id, occurred_at) and not (event_id) -- another composite key, arrived
-- at the way real schemas arrive at one.
--
CREATE TABLE public.events (
    event_id    bigint NOT NULL,
    person_id   bigint NOT NULL REFERENCES public.people (person_id),
    occurred_at timestamp with time zone NOT NULL,
    kind        text NOT NULL,
    payload     jsonb NOT NULL DEFAULT '{}'::jsonb,
    PRIMARY KEY (event_id, occurred_at)
) PARTITION BY RANGE (occurred_at);

CREATE TABLE public.events_2024 PARTITION OF public.events
    FOR VALUES FROM ('2024-01-01 00:00:00+00') TO ('2025-01-01 00:00:00+00');

CREATE TABLE public.events_2025 PARTITION OF public.events
    FOR VALUES FROM ('2025-01-01 00:00:00+00') TO ('2026-01-01 00:00:00+00');


--
-- Trap: a quoted, mixed-case identifier.
--
-- public."LegacyCustomer" and its "EmailAddress" column exist only when they
-- are quoted. Any SQL lazyslice generates -- the count probe, the chunked read,
-- the residual scan, the COPY target, the emitted yml -- must quote them, and
-- an identifier concatenated into a string somewhere without quoting fails
-- here with 42P01 instead of quietly reading a different table.
--
-- "MigratedFromPersonID" is the declared edge into public.people. Without it
-- this table is unreachable from the root, drops to Step{t, SchemaOnly}, and
-- the whole claim above -- that the chunked read, the COPY column list and the
-- residual scan must quote the identifier -- is never exercised by a run,
-- because no table in the people component has a quoted column at all. It is
-- integer against a bigint primary key, which PostgreSQL allows and which puts
-- a widening key comparison in the fixture as well.
--
-- The table is also where ARCHITECTURE.md section 5's domain machinery is
-- trapped, because nothing else in testdata/ carries a unique index on a
-- masked column, a varchar(n) personal column or a parseable CHECK:
--
--   "EmailAddress"   unique + CHECK: uniqueness picks the generator with the
--                    largest Domain() in the category (email with a
--                    hash-derived suffix), and the CHECK shape must survive
--   "ContactNumber"  unique varchar(15): section 5's own worked example. The
--                    plain `phone` generator's Domain() is about 8x10^4, far
--                    under d_required = n^2/2e, so the plan must either choose
--                    phone_unique -- saying that libphonenumber validity is
--                    not preserved -- or refuse the column by name with exit
--                    12 printing d and d_required. Choosing `phone` and
--                    hoping is the bug this column exists to catch, and it
--                    fails at load with a unique violation whose Detail we
--                    drop.
--
CREATE TABLE public."LegacyCustomer" (
    "CustomerID"           integer GENERATED ALWAYS AS IDENTITY (START WITH 42 INCREMENT BY 1) PRIMARY KEY,
    "MigratedFromPersonID" integer REFERENCES public.people (person_id),
    "EmailAddress"         text NOT NULL,
    "ContactNumber"        character varying(15),
    "MobileNumber"         text,
    "Notes"                text,
    CONSTRAINT "LegacyCustomer_EmailAddress_check" CHECK ("EmailAddress" LIKE '%@%.%')
);

CREATE UNIQUE INDEX "LegacyCustomer_EmailAddress_key"
    ON public."LegacyCustomer" ("EmailAddress");
CREATE UNIQUE INDEX "LegacyCustomer_ContactNumber_key"
    ON public."LegacyCustomer" ("ContactNumber");


--
-- Trap: keys that are not integers.
--
-- ARCHITECTURE.md section 2 encodes one chunk of key tuples as one typed array
-- per identity column: []int64 for int2, int4 and int8, []string for text,
-- varchar, bpchar and citext, []pgtype.UUID for uuid, and the text form with a
-- cast for everything else. Getting a cast wrong there is not a loud failure:
-- unnest($1::text[]) joined against a uuid column matches nothing, and the
-- result is a chunk that returns fewer rows than it was given -- the silently
-- empty slice of research/COMPLAINTS.md FK-10 again. Every branch therefore
-- needs a key of its own kind, with a child table to follow the edge into.
--
--   sites.site_code            text                     -- []string
--   devices.device_id          uuid                     -- []pgtype.UUID
--   device_readings            (uuid, timestamptz)      -- a typed column and
--                                                       -- the fallback in one
--                                                       -- composite key
--
-- Both sites and devices have an outgoing foreign key, so neither is
-- lookup-shaped (section 3 copies a table whole only when it has none) and both
-- are read in chunks like anything else.
--
CREATE TABLE public.sites (
    site_code       text PRIMARY KEY,
    name            text NOT NULL,
    owner_person_id bigint NOT NULL REFERENCES public.people (person_id),
    contact_email   text
);

CREATE TABLE public.devices (
    device_id uuid PRIMARY KEY,
    site_code text NOT NULL REFERENCES public.sites (site_code),
    asset_tag text NOT NULL,
    owned_by  text
);

CREATE TABLE public.device_readings (
    device_id uuid NOT NULL REFERENCES public.devices (device_id),
    taken_at  timestamp with time zone NOT NULL,
    celsius   numeric(5,2) NOT NULL,
    PRIMARY KEY (device_id, taken_at)
);


--
-- Trap: no primary key, but one unique index that will do.
--
-- ARCHITECTURE.md section 3.4's identity ladder is --key, then the primary key,
-- then a unique index that is neither partial nor on an expression, then a
-- probed pseudo-key, then refusal. Every other table in this file stops at the
-- first rung. audit_log starts at the third: it has no primary key, one unique
-- index on entry_uid that qualifies, and two that must not be picked -- one
-- partial, one on lower(entry_uid). Identity here must come out as
-- IdentityUnique over (entry_uid).
--
-- client_ip is the name-hit half of the inet signal, opposite
-- tenant_user_sessions.origin, which has the type and no name. Under
-- ARCHITECTURE.md section 4 this one reaches `certain` (name plus net.ParseIP
-- agreeing) and that one reaches `likely` (values only); a run that classifies
-- them alike has collapsed two signals into one.
--
CREATE TABLE public.audit_log (
    entry_uid   text NOT NULL,
    person_id   bigint NOT NULL REFERENCES public.people (person_id),
    action      text NOT NULL,
    client_ip   inet,
    occurred_at timestamp with time zone NOT NULL
);

CREATE UNIQUE INDEX audit_log_entry_uid_key ON public.audit_log (entry_uid);
CREATE UNIQUE INDEX audit_log_recent_action_key ON public.audit_log (action)
    WHERE occurred_at >= '2025-01-01 00:00:00+00';
CREATE UNIQUE INDEX audit_log_lower_entry_uid_key ON public.audit_log (lower(entry_uid));


--
-- Trap: no row identity at all.
--
-- No primary key, no unique index, and two rows that are identical in every
-- column, so no pseudo-key probe can find a candidate either: there is no set
-- of columns that identifies a row in click_stream. ARCHITECTURE.md section 3.4
-- is explicit that lazyslice stops rather than guesses -- exit 12, naming the
-- table and --key table=col,col -- because a guessed identity produces a slice
-- whose rows are silently the wrong ones. This is the one table in testdata/
-- whose required behaviour is a refusal, so a regression that makes the planner
-- guess shows up here and nowhere else.
--
CREATE TABLE public.click_stream (
    person_id  bigint NOT NULL REFERENCES public.people (person_id),
    url        text NOT NULL,
    clicked_at timestamp with time zone NOT NULL
);


--
-- Trap: a cross-schema foreign key.
--
-- billing.invoices references public.people. The plan has to carry the schema
-- of both ends of the edge, and the target has to have schema billing before
-- the table is created in it.
--
CREATE TABLE billing.invoices (
    invoice_id    bigint GENERATED BY DEFAULT AS IDENTITY (START WITH 3000 INCREMENT BY 5) PRIMARY KEY,
    person_id     bigint NOT NULL REFERENCES public.people (person_id),
    bill_to_email text NOT NULL,
    bill_to_phone text,
    total_pence   bigint NOT NULL,
    issued_on     date NOT NULL
);


--
-- Trap: two million rows, on demand.
--
-- stream_rows is the fixture for the claim that extract streams rather than
-- buffers: a run against it must not grow the resident set with the row count,
-- and the progress line must move. It hangs off one person, so a slice rooted
-- at that person pulls all of it and a slice rooted at anyone else pulls none.
--
-- It is empty unless the file is loaded with -v big=1, because the fill costs a
-- few seconds and a couple of hundred megabytes of table, and every test that is
-- not about streaming would pay for both.
--
CREATE TABLE public.stream_rows (
    stream_row_id bigint GENERATED BY DEFAULT AS IDENTITY (START WITH 1 INCREMENT BY 1) PRIMARY KEY,
    person_id     bigint NOT NULL REFERENCES public.people (person_id),
    email         text NOT NULL,
    body          text NOT NULL
);

CREATE FUNCTION public.fill_stream_rows(n bigint) RETURNS bigint
    LANGUAGE plpgsql
    AS $fill$
DECLARE
    owner_id bigint;
BEGIN
    SELECT min(person_id) INTO STRICT owner_id FROM public.people;

    INSERT INTO public.stream_rows (stream_row_id, person_id, email, body)
    SELECT g,
           owner_id,
           'stream.' || g || '@example.invalid',
           repeat('x', 40) || g
    FROM generate_series(1, n) AS g;

    PERFORM setval(pg_get_serial_sequence('public.stream_rows', 'stream_row_id'), n);
    RETURN n;
END;
$fill$;


--
-- Trap 26: a million rows behind a text key.
--
-- stream_docs is stream_rows with the identity moved off bigint, and the two
-- are not the same test. A bigint key set is a []int64 and costs eight bytes a
-- row (ARCHITECTURE.md section 2, KeySet); a text key set is a slab of encoded
-- tuples costing the key's own bytes, a terminator and an eight-byte span, and
-- every chunk handed to the server is a []string of separately allocated
-- strings -- about 64 bytes a key. So a stage that materialises a whole key set
-- alongside the planner's pays about four times as much here per row as it does
-- on stream_rows, and this is the table that can fail the claim "extract's peak
-- is one chunk of keys and one batch of rows" while stream_rows still passes it
-- on margin.
--
-- The key is 'doc-' plus an md5: 36 characters, deterministic, so two loads of
-- this fixture produce the same keys in the same order. Every row hangs off the
-- same one person, as stream_rows' do, so a slice rooted at anyone else pulls
-- none of it.
--
-- The table and its fill function are declared here, beside stream_rows and
-- above the ANALYZE, so the fixture's schema is constant regardless of the
-- big flag: 26 tables either way, both unanalysed the same way until the
-- gate's own ANALYZE runs. Only the fill itself -- the million-row INSERT,
-- which costs seconds and tens of megabytes -- is gated, the same way
-- stream_rows' is.
--
CREATE TABLE public.stream_docs (
    doc_key   text   PRIMARY KEY,
    person_id bigint NOT NULL REFERENCES public.people (person_id),
    email     text   NOT NULL,
    body      text   NOT NULL
);

CREATE FUNCTION public.fill_stream_docs(n bigint) RETURNS bigint
    LANGUAGE plpgsql
    AS $filldocs$
DECLARE
    owner_id bigint;
BEGIN
    SELECT min(person_id) INTO STRICT owner_id FROM public.people;

    INSERT INTO public.stream_docs (doc_key, person_id, email, body)
    SELECT 'doc-' || md5(g::text),
           owner_id,
           'doc.' || g || '@example.invalid',
           repeat('y', 40) || g
    FROM generate_series(1, n) AS g;

    RETURN n;
END;
$filldocs$;


--
-- Trap: a partitioned root's own key is DEFERRABLE, a leaf carries a key the
-- root cannot hold, and an edge references the leaf.
--
-- ARCHITECTURE.md section 11.1 item 6 re-points a partition-referencing edge
-- onto the root when the root carries a key over the edge's columns
-- (introspect's hasKeyOver). It must not re-point when the root cannot hold
-- that key, and ForeignKey.NotRecreatable must be set instead so the planner
-- refuses at plan (section 11.1, exit 13, target.schema.not_recreatable)
-- before anything in the target is touched, rather than emitting an
-- ADD CONSTRAINT the target will reject after item 1 has already dropped
-- every user table.
--
-- price_lists is partitioned by region and its own key, UNIQUE (list_id,
-- region), is DEFERRABLE -- Postgres refuses a DEFERRABLE unique constraint
-- as a referenced key outright ("cannot use a deferrable unique constraint
-- for referenced table"), so this key can never back an edge even though it
-- is over columns the root does hold. price_lists_eu additionally carries
-- its own UNIQUE (list_id), which the root itself can never hold: a
-- partitioned table's own unique constraint must include every partition key
-- column, and region is that column here, so no key lacking it can ever
-- exist on the root. price_list_notes.list_id references price_lists_eu
-- (list_id) directly -- legal, because a leaf partition is an ordinary table
-- with its own key -- and that is the edge introspect must leave pointed at
-- the leaf, marked NotRecreatable, rather than silently re-pointed at a root
-- that cannot carry it.
--
-- price_lists.owner_person_id connects price_lists to public.people; without
-- it price_lists is unreachable from the root every invariant run slices
-- from and this trap costs nothing to pass. price_list_notes does not sit
-- behind that edge -- byRef (internal/plan/plan.go) never holds a partition,
-- so a table reachable only through a partition-referencing edge is
-- stranded for the planner even though pg_constraint shows a path -- so
-- price_list_notes.person_id below connects it to public.people directly,
-- independently of the trap edge.
--
-- The trap edge itself, price_list_notes.list_id -> price_lists_eu
-- (list_id), is added by a separate ALTER TABLE below, behind a psql
-- conditional on a "notrecreatable" variable -- the same device nasty.sql
-- uses to gate the 2,000,000-row fill further down. checkRecreatable
-- (internal/plan/plan.go) scans every edge in the schema before a root is
-- even chosen and refuses any Plan call over one carrying a NotRecreatable
-- edge, unconditionally -- so leaving the constraint on every load would
-- make nasty.sql refuse to plan for every caller, not only the two tests
-- this trap is for. testutil.LoadNasty always cuts the gated block out;
-- testutil.LoadNastyNotRecreatable puts it back.
--
CREATE TABLE public.price_lists (
    list_id         bigint NOT NULL,
    region          text NOT NULL,
    owner_person_id bigint NOT NULL REFERENCES public.people (person_id),
    currency        text NOT NULL,
    CONSTRAINT price_lists_list_id_region_key UNIQUE (list_id, region) DEFERRABLE
) PARTITION BY LIST (region);

CREATE TABLE public.price_lists_eu PARTITION OF public.price_lists
    FOR VALUES IN ('EU');
CREATE TABLE public.price_lists_us PARTITION OF public.price_lists
    FOR VALUES IN ('US');

ALTER TABLE public.price_lists_eu
    ADD CONSTRAINT price_lists_eu_list_id_key UNIQUE (list_id);

CREATE TABLE public.price_list_notes (
    note_id   bigint GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY,
    list_id   bigint NOT NULL,
    person_id bigint NOT NULL REFERENCES public.people (person_id),
    note      text NOT NULL
);

-- The gate. Everything above this line is unconditional; only the one
-- constraint below is not.
\if :{?notrecreatable}
ALTER TABLE public.price_list_notes
    ADD CONSTRAINT price_list_notes_list_id_fkey
    FOREIGN KEY (list_id) REFERENCES public.price_lists_eu (list_id);
\endif


--
-- Data.
--
-- Small, hand-written and stable: the counts in
-- internal/testutil/fixtures_test.go are these rows, so changing them changes
-- that test on purpose rather than by accident.
--
-- OVERRIDING SYSTEM VALUE is required because person_id and "CustomerID" are
-- GENERATED ALWAYS. That is the same clause lazyslice's loader must emit, so
-- writing the fixture this way is the first place the trap is exercised.
--

-- alt_emails covers all four array cases the masker has to survive, one per
-- row: two values, one value, an array whose first element is NULL, a NULL
-- column, and an empty array. "Map each element, keep the length, keep NULL as
-- NULL, keep an empty array empty" is only a testable claim because of the
-- last three.
--
INSERT INTO public.people
    (person_id, manager_id, given_name, family_name, email_verified, ref, status, marital_status,
     contact, alt_emails, notes)
OVERRIDING SYSTEM VALUE
VALUES
    (90000, NULL, 'Ada', 'Lovelace', true, 'ada.lovelace@fixture.test', 'active', 'married',
     '{"profile": {"contact": {"email": "ada.lovelace@fixture.test", "phone": "+44 20 7946 0958"}, "locale": "en-GB"}, "tags": ["founder"]}'::jsonb,
     ARRAY['ada@corp.invalid', 'a.lovelace@corp.invalid'],
     'Ada Lovelace asked that Grace Hopper be copied on the renewal. Call back on +44 20 7946 0958.'),
    (90007, 90000, 'Grace', 'Hopper', false, 'grace.hopper@fixture.test', 'active', 'single',
     '{"profile": {"contact": {"email": "grace.hopper@fixture.test", "phone": "+1 415 555 0132"}, "locale": "en-US"}, "tags": ["admin"]}'::jsonb,
     ARRAY['ghopper@corp.invalid'],
     'Grace Hopper prefers email. Escalation contact is Alan Turing.'),
    (90014, 90000, 'Alan', 'Turing', true, 'alan.turing@fixture.test', 'suspended', 'civil_partnership',
     '{"profile": {"contact": {"email": "alan.turing@fixture.test", "phone": "+44 161 496 0123"}, "locale": "en-GB"}, "tags": []}'::jsonb,
     ARRAY[NULL, 'a.turing@corp.invalid']::text[],
     'Suspended pending review. Raised by Katherine Johnson on 2024-03-02.'),
    (90021, 90007, 'Katherine', 'Johnson', true, 'katherine.johnson@fixture.test', 'pending', 'widowed',
     '{"profile": {"contact": {"email": "katherine.johnson@fixture.test", "phone": "+1 757 555 0188"}, "locale": "en-US"}, "tags": ["reviewer"]}'::jsonb,
     NULL,
     'Katherine Johnson is the reviewer of record for Alan Turing.'),
    (90028, 90007, 'Edsger', 'Dijkstra', false, 'edsger.dijkstra@fixture.test', 'closed', 'undisclosed',
     '{"profile": {"contact": {"email": "edsger.dijkstra@fixture.test", "phone": "+31 20 555 0177"}, "locale": "nl-NL"}, "tags": ["archived"]}'::jsonb,
     '{}'::text[],
     'Account closed at the request of Edsger Dijkstra.');

ALTER TABLE public.people ALTER COLUMN person_id RESTART WITH 90035;

INSERT INTO public.orders (order_id, person_id, placed_at, total_pence) VALUES
    (200000, 90000, '2024-02-01 10:00:00+00', 1250),
    (200003, 90000, '2024-03-11 11:30:00+00', 8400),
    (200006, 90007, '2024-04-02 09:15:00+00',  399),
    (200009, 90007, '2025-01-20 16:45:00+00', 15000),
    (200012, 90021, '2025-02-14 08:05:00+00',  2750);

ALTER TABLE public.orders ALTER COLUMN order_id RESTART WITH 200015;

INSERT INTO public.order_items (order_id, line_no, sku, qty) VALUES
    (200000, 1, 'SKU-0001', 2),
    (200000, 2, 'SKU-0002', 1),
    (200003, 1, 'SKU-0003', 5),
    (200006, 1, 'SKU-0001', 1),
    (200009, 1, 'SKU-0004', 3),
    (200009, 2, 'SKU-0005', 1),
    (200012, 1, 'SKU-0002', 4);

-- The parent edge. 200000 is one of Ada's own orders, which is the case
-- ARCHITECTURE.md section 3 uses; 200009 belongs to Grace and is reached only
-- as a parent of Katherine.
UPDATE public.people SET preferred_order_id = 200000 WHERE person_id = 90000;
UPDATE public.people SET preferred_order_id = 200006 WHERE person_id = 90007;
UPDATE public.people SET preferred_order_id = 200009 WHERE person_id = 90021;

INSERT INTO public.tenant_users (tenant_id, user_id, owner_person_id, email, joined_on) VALUES
    (1, 1, 90000, 'ada.lovelace@fixture.test',      '2024-01-05'),
    (1, 2, 90007, 'grace.hopper@fixture.test',      '2024-01-06'),
    (2, 1, 90014, 'alan.turing@fixture.test',       '2024-02-11'),
    (2, 7, 90021, 'katherine.johnson@fixture.test', '2024-02-12');

-- Session 5044 has a tenant and no user. Under MATCH SIMPLE that row references
-- no tenant_users row at all, so following its parent edge must select nothing
-- for it -- not tenant 2's other users, and not an error.
--
-- origin's IPv4 values are RFC 1918 private addresses, deliberately outside
-- the RFC 5737 blocks network_id emits into (ARCHITECTURE.md §5, trap 18):
-- a documentation-range source value here would make the residual scan
-- report a false hit. Keep new IPv4 fixture values out of those blocks.
INSERT INTO public.tenant_user_sessions (session_id, tenant_id, user_id, started_at, origin, adapter) VALUES
    (5000, 1, 1,    '2024-05-01 12:00:00+00', '10.20.30.40',   '08:00:2b:01:02:03'),
    (5011, 1, 2,    '2024-05-01 12:05:00+00', '10.20.30.41',   '08:00:2b:01:02:04'),
    (5022, 2, 1,    '2024-05-02 08:30:00+00', '172.16.5.6',    '08:00:2b:01:02:05'),
    (5033, 2, 7,    '2024-05-02 08:31:00+00', '2001:db8::1',   NULL),
    (5044, 2, NULL, '2024-05-03 07:00:00+00', '172.16.5.7',    '08:00:2b:01:02:06');

ALTER TABLE public.tenant_user_sessions ALTER COLUMN session_id RESTART WITH 5055;

-- Flag 9004 has both referencing columns NULL, which MATCH FULL allows and
-- which references nothing; a row with one of the two set and the other NULL
-- would be rejected by the constraint, which is the whole difference from
-- tenant_user_sessions.
INSERT INTO public.tenant_user_flags (flag_id, tenant_id, user_id, flag) VALUES
    (9000, 1,    1,    'beta'),
    (9002, 2,    7,    'beta'),
    (9004, NULL, NULL, 'unassigned');

ALTER TABLE public.tenant_user_flags ALTER COLUMN flag_id RESTART WITH 9006;

INSERT INTO public.organisations (organisation_id, name, founded_by) VALUES
    (500, 'Analytical Engines Ltd', 90000),
    (503, 'Compiler Works',         90007);

INSERT INTO public.teams (team_id, name) VALUES
    (600, 'Punch Cards'),
    (603, 'Runtime');

INSERT INTO public.projects (project_id, name) VALUES
    (700, 'Difference Engine'),
    (703, 'A-0 System');

ALTER TABLE public.organisations ALTER COLUMN organisation_id RESTART WITH 506;
ALTER TABLE public.teams        ALTER COLUMN team_id         RESTART WITH 606;
ALTER TABLE public.projects     ALTER COLUMN project_id      RESTART WITH 706;

-- Close the cycle.
UPDATE public.organisations SET primary_team_id = 600 WHERE organisation_id = 500;
UPDATE public.organisations SET primary_team_id = 603 WHERE organisation_id = 503;
UPDATE public.teams SET lead_project_id = 700 WHERE team_id = 600;
UPDATE public.teams SET lead_project_id = 703 WHERE team_id = 603;
UPDATE public.projects SET owner_organisation_id = 500 WHERE project_id = 700;
UPDATE public.projects SET owner_organisation_id = 503 WHERE project_id = 703;

ALTER TABLE public.organisations ALTER COLUMN primary_team_id SET NOT NULL;

-- 839's uploaded_by_person_id is NULL: the dangling polymorphic owner stays in
-- the fixture on the one row the declared edge does not reach.
-- 845's owner_type is 'Person', the Rails-spelled class name: it resolves
-- through the first form tried — underscore and pluralise, 'Person' ->
-- 'people' — rather than through the raw-table-name fallback 800, 813 and
-- 839 exercise. Its owner_id (90007, Grace Hopper) is a person already in the
-- fixture, and its own uploaded_by_person_id puts the row in the slice
-- through the declared edge regardless, so this row's job is exercising the
-- resolution form, not reaching a new one (attachment 826 already does that).
INSERT INTO public.attachments
    (attachment_id, owner_type, owner_id, uploaded_by_person_id, filename, uploaded_by)
VALUES
    (800,  'people',   90000, 90000, 'signature.png',   'ada.lovelace@fixture.test'),
    (813,  'people',   90007, 90007, 'id-scan.pdf',     'grace.hopper@fixture.test'),
    (826,  'projects',   700, 90014, 'spec-v3.pdf',     'alan.turing@fixture.test'),
    (839,  'people',   99999, NULL,  'orphan.txt',      'nobody@example.invalid'),
    (845,  'Person',  90007, 90007, 'contract.pdf',    'grace.hopper@fixture.test');

ALTER TABLE public.attachments ALTER COLUMN attachment_id RESTART WITH 852;

-- events_2024 is deliberately the larger leaf, so "sample the largest partition
-- by reltuples" has an unambiguous answer.
INSERT INTO public.events (event_id, person_id, occurred_at, kind, payload) VALUES
    (1, 90000, '2024-02-01 10:00:01+00', 'order.placed',
     '{"actor": {"contact": {"email": "ada.lovelace@fixture.test", "phone": "+44 20 7946 0958"}}, "order_id": 200000}'::jsonb),
    (2, 90000, '2024-03-11 11:30:01+00', 'order.placed',
     '{"actor": {"contact": {"email": "ada.lovelace@fixture.test", "phone": "+44 20 7946 0958"}}, "order_id": 200003}'::jsonb),
    (3, 90007, '2024-04-02 09:15:01+00', 'order.placed',
     '{"actor": {"contact": {"email": "grace.hopper@fixture.test", "phone": "+1 415 555 0132"}}, "order_id": 200006}'::jsonb),
    (4, 90014, '2024-03-02 14:00:00+00', 'account.suspended',
     '{"actor": {"contact": {"email": "katherine.johnson@fixture.test", "phone": "+1 757 555 0188"}}, "reason": "review"}'::jsonb),
    (5, 90021, '2024-06-30 23:59:59+00', 'account.reviewed',
     '{"actor": {"contact": {"email": "katherine.johnson@fixture.test", "phone": "+1 757 555 0188"}}}'::jsonb),
    (6, 90007, '2025-01-20 16:45:01+00', 'order.placed',
     '{"actor": {"contact": {"email": "grace.hopper@fixture.test", "phone": "+1 415 555 0132"}}, "order_id": 200009}'::jsonb),
    (7, 90021, '2025-02-14 08:05:01+00', 'order.placed',
     '{"actor": {"contact": {"email": "katherine.johnson@fixture.test", "phone": "+1 757 555 0188"}}, "order_id": 200012}'::jsonb);

-- "ContactNumber" is varchar(15) and unique, so every value here is distinct
-- and fits: it is the column ARCHITECTURE.md section 5's worked example is
-- about. 44 leaves it NULL, because a unique index permits repeated NULLs and
-- "NULL stays NULL" has to hold under a uniqueness-driven generator too.
INSERT INTO public."LegacyCustomer"
    ("CustomerID", "MigratedFromPersonID", "EmailAddress", "ContactNumber", "MobileNumber", "Notes")
OVERRIDING SYSTEM VALUE
VALUES
    (42, 90000, 'ada.lovelace@fixture.test',  '+447700900123', '+44 7700 900123',
     'Migrated from the 1998 system. Contact is Ada Lovelace.'),
    (43, 90007, 'grace.hopper@fixture.test',  '+14155550132',  '+1 415 555 0132',
     'Do not merge with Grace Hopper''s new record.'),
    (44, 90014, 'alan.turing@fixture.test',   NULL,            NULL,
     'Left blank on purpose.');

ALTER TABLE public."LegacyCustomer" ALTER COLUMN "CustomerID" RESTART WITH 45;

INSERT INTO public.sites (site_code, name, owner_person_id, contact_email) VALUES
    ('SITE-LDN', 'London',   90000, 'site.london@fixture.test'),
    ('SITE-NYC', 'New York', 90007, 'site.newyork@fixture.test');

INSERT INTO public.devices (device_id, site_code, asset_tag, owned_by) VALUES
    ('11111111-2222-4333-8444-555555555551', 'SITE-LDN', 'AT-0001', 'ada.lovelace@fixture.test'),
    ('11111111-2222-4333-8444-555555555552', 'SITE-LDN', 'AT-0002', 'grace.hopper@fixture.test'),
    ('11111111-2222-4333-8444-555555555553', 'SITE-NYC', 'AT-0003', NULL);

INSERT INTO public.device_readings (device_id, taken_at, celsius) VALUES
    ('11111111-2222-4333-8444-555555555551', '2025-03-01 00:00:00+00', 18.50),
    ('11111111-2222-4333-8444-555555555551', '2025-03-01 01:00:00+00', 18.25),
    ('11111111-2222-4333-8444-555555555552', '2025-03-01 00:00:00+00', 21.00),
    ('11111111-2222-4333-8444-555555555553', '2025-03-01 00:00:00+00', 19.75);

-- Two of the four rows are in the window the partial unique index covers, with
-- different actions, and lower(entry_uid) is unique too: all three indexes are
-- satisfied, and only audit_log_entry_uid_key is a legal identity.
--
-- client_ip's IPv4 values are RFC 1918 private addresses, deliberately
-- outside the RFC 5737 blocks network_id emits into (ARCHITECTURE.md §5,
-- trap 18): a documentation-range source value here would make the residual
-- scan report a false hit. Keep new IPv4 fixture values out of those blocks.
INSERT INTO public.audit_log (entry_uid, person_id, action, client_ip, occurred_at) VALUES
    ('AE-0001', 90000, 'login',           '10.20.30.40',   '2024-05-01 12:00:00+00'),
    ('AE-0002', 90007, 'login',           '10.20.30.41',   '2024-05-01 12:05:00+00'),
    ('AE-0003', 90014, 'password.reset',  '2001:db8::7',   '2025-02-01 09:00:00+00'),
    ('AE-0004', 90021, 'account.review',  '192.168.9.10',  '2025-02-02 09:00:00+00');

-- The first two rows are identical in every column. No subset of columns is
-- unique, so the pseudo-key probe has nothing to find and the run must stop.
INSERT INTO public.click_stream (person_id, url, clicked_at) VALUES
    (90000, 'https://example.com/pricing', '2025-04-01 10:00:00+00'),
    (90000, 'https://example.com/pricing', '2025-04-01 10:00:00+00'),
    (90007, 'https://example.com/docs',    '2025-04-01 10:01:00+00');

INSERT INTO billing.invoices (invoice_id, person_id, bill_to_email, bill_to_phone, total_pence, issued_on) VALUES
    (3000, 90000, 'accounts@fixture.test',        '+44 20 7946 0958',  9650, '2024-04-01'),
    (3005, 90007, 'grace.hopper@fixture.test',    '+1 415 555 0132',    399, '2024-05-01'),
    (3010, 90021, 'katherine.johnson@fixture.test', NULL,              2750, '2025-03-01');

ALTER TABLE billing.invoices ALTER COLUMN invoice_id RESTART WITH 3015;

INSERT INTO public.price_lists (list_id, region, owner_person_id, currency) VALUES
    (1, 'EU', 90000, 'EUR'),
    (2, 'EU', 90007, 'GBP'),
    (3, 'US', 90014, 'USD');

INSERT INTO public.price_list_notes (list_id, person_id, note) VALUES
    (1, 90000, 'Reviewed for the spring catalogue.'),
    (2, 90007, 'Currency confirmed with finance.');

-- pg_class.reltuples is -1 until something analyses the table, and the
-- classifier's partition choice and the plan's row estimates both read it.
ANALYZE;

--
-- The gate. Nothing above this line depends on it.
--
\if :{?big}
SELECT public.fill_stream_rows(2000000);
ANALYZE public.stream_rows;

SELECT public.fill_stream_docs(1000000);
ANALYZE public.stream_docs;
\endif
