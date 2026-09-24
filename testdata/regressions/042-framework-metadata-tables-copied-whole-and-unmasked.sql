-- root:   public.reg042_widgets
-- take:   10
-- expect: ok
-- found:  dogfood session 1 (T-0314), not a torture schema
-- why:    schema_migrations had no foreign key reaching it -- the ordinary
--         shape of migration bookkeeping -- so section 3's own lookup rule
--         ("at least one incoming edge") left it SchemaOnly: zero migration
--         rows, and a fresh Rails checkout re-runs every migration against
--         it. ar_internal_metadata went the other way, copied verbatim with
--         its source environment row intact, and a fresh checkout refuses a
--         destructive rake task because the row it reads back still says
--         production.
-- not-masked: public.schema_migrations.version, public.ar_internal_metadata.key, public.ar_internal_metadata.value
--
-- Both tables are reduced to their real shape (Rails' own migrations) rather
-- than a synthetic one, because the defect is specifically about these two
-- names: internal/pipeline.IsFrameworkMetadataTable matches on them by name
-- and copies them whole as a Lookup step regardless of reachability
-- (internal/plan's own T-0314 entry), and internal/classify never masks a
-- column of one (that package's own T-0314 entry) whatever its values look
-- like. reg042_widgets is an ordinary, unrelated table so that neither
-- framework table is reached by the walk from anywhere -- which is the
-- point: dogfood session 1's whole complaint was that an unreachable table
-- was left empty.
--
-- schema_migrations.version's three values are ordinary Rails migration
-- timestamps (YYYYMMDDHHMMSS) chosen so that every one of them PASSES the
-- Luhn check (financial_account's validator): the shape dogfood session 1
-- met ("its digit strings pass the Luhn check", internal/classify/CLAUDE.md's
-- T-0314 entry). With the framework branch of markNeverMasked reverted the
-- classifier masks the column and the run exits 9 at verify, and with the
-- second net's T-0348 exemption reverted the copied column refuses the run
-- at exit 9 too, so `expect: ok` here proves both halves of the rule, not
-- only that the column name draws no hit.

-- owner_email exists only so the table is not reduced to a single column and
-- to give assertTortureNoLiteralSurvives an email to find and prove absent
-- from the target — every expect: ok regression gets that check for free,
-- and it fails vacuously over a source with no personal literal at all
-- (026's own header explains the same trap).
CREATE TABLE public.reg042_widgets (
    widget_id   bigint PRIMARY KEY,
    name        text NOT NULL,
    owner_email text NOT NULL
);

INSERT INTO public.reg042_widgets (widget_id, name, owner_email) VALUES
    (1, 'Widget One', 'aidan.andrews1@realcorp.example'),
    (2, 'Widget Two', 'aubrey.booker2@realcorp.example');

CREATE TABLE public.schema_migrations (
    version character varying NOT NULL PRIMARY KEY
);

INSERT INTO public.schema_migrations (version) VALUES
    ('20250101050000'), ('20250101130000'), ('20250101210000');

CREATE TABLE public.ar_internal_metadata (
    key        character varying NOT NULL PRIMARY KEY,
    value      character varying,
    created_at timestamp(6) without time zone NOT NULL,
    updated_at timestamp(6) without time zone NOT NULL
);

INSERT INTO public.ar_internal_metadata (key, value, created_at, updated_at) VALUES
    ('environment', 'production', '2025-01-01 00:00:00', '2025-01-01 00:00:00');
