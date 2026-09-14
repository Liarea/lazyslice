-- root:   public.reg9_site
-- take:   20
-- expect: ok
-- found:  plausible
-- why:    a citext[] of addresses arrives as one text literal and is masked element-wise
--
-- The other half of 005, and the one that carries personal data.
--
-- Plausible's `monthly_reports.recipients` is `citext[] NOT NULL DEFAULT
-- ARRAY[]::citext[]`: the addresses a site's monthly report is emailed to. 005
-- reduced the *loading* of such a column and had to keep its values "short and
-- dull" — `ARRAY['wk','mo']` — to steer around this defect, and says so in its
-- own comment. This file is the same column with the values it really holds.
--
-- The source pool registers no user types (T-0076), and pgx hands back a slice
-- only for an array type its own map knows, so a `citext[]` arrives as the
-- single string `{a@b.test,c@d.test}`. Three stages then read it:
--
--   * internal/classify used to see one opaque value, match no validator and
--     decide `none`, so the addresses were copied into the target verbatim
--     under exit 0. T-0103 made it split the literal, so the column is decided
--     `email`.
--   * internal/transform used to mask it as one scalar, because its
--     element-wise path fired on a `[]any` alone: the whole literal became one
--     fake address, and `CopyFrom` then refused it for an `_citext` column —
--     "cannot find encode plan" — at exit 7, with the earlier tables committed
--     and the rows of this one already moving. That is what T-0118 fixed:
--     transform parses the literal, masks each element under the column's
--     masker with h computed per element, and writes a literal back.
--   * internal/plan used to refuse the column at exit 12 in between the two,
--     so that landing the classify half alone could not half-load a target.
--     That refusal (`arrayArrivesAsLiteral` in internal/plan/writeback.go) was
--     a stand-in for the masker and T-0127 is what removes it, now that both
--     the masker (T-0118) and the residual scan's own reader of the literal
--     (T-0129) exist.
--
-- **The header now asserts the load, not the refusal.** T-0127 removed
-- `arrayArrivesAsLiteral`, so the plan no longer refuses this column and the
-- run reaches the loader. What this file asserts is the whole chain: the run
-- exits 0, so the literal is something `CopyFrom` can write; and the suite's
-- own leak check (`assertTortureNoLiteralSurvives`, which the harness runs only
-- for a file expecting exit 0) finds none of the source's addresses in the
-- target, so the elements inside the literal were masked rather than copied.
-- This is the first commit under which that check actually runs for this file
-- — before it, the harness returned at the refusal for any file expecting a
-- non-zero exit, so the element-wise masker had unit coverage in
-- internal/transform/array_test.go only, and no end-to-end evidence from the
-- CLI.
--
-- What that leak check is *not* is the residual scan. It is this suite's own
-- external grep of the target, and it covers this fixture only. Inside the
-- product, ARCHITECTURE.md §6 item 1's residual scan used to be blind to a
-- masked array that arrives as a literal: internal/verify read the column back
-- as one string, so the per-element filter entries transform makes matched
-- nothing and the scan passed green. T-0129 landed first and fixed that:
-- internal/verify's `arrayHits` splits the literal with transform's own
-- grammar and tests one filter entry per element, and a masked array column
-- whose value it cannot split is exit 9 naming the column rather than a green
-- tick (internal/transform/CLAUDE.md carries the rule). So now that T-0127 has
-- landed, the second net and the residual scan are both looking inside the
-- braces, and this file's own grep is corroboration rather than the only
-- evidence — which is the order T-0127's log required and the reason it was
-- ordered behind T-0129.
--
-- The addresses are under `.test` rather than `example.com`, `example.net` or
-- `example.org`: those three are the domains the email masker itself emits
-- (ARCHITECTURE.md §5), so a source address in one of them can equal a masked
-- one and the leak check would fail on the fixture rather than on the code
-- (T-0073).

CREATE EXTENSION IF NOT EXISTS citext;

CREATE TABLE public.reg9_site (
    id          integer PRIMARY KEY,
    domain      citext NOT NULL,
    owner_email text NOT NULL
);

CREATE TABLE public.reg9_report (
    id         integer PRIMARY KEY,
    site_id    integer NOT NULL REFERENCES public.reg9_site (id),
    -- The column this file exists for. NOT NULL with an array default, as
    -- Plausible declares it, so the empty array is a value that has to survive
    -- masking as well.
    recipients citext[] NOT NULL DEFAULT ARRAY[]::citext[]
);

INSERT INTO public.reg9_site (id, domain, owner_email)
SELECT i, ('site' || i || '.fixture.test')::citext, 'owner' || i || '@fixture.test'
FROM generate_series(1, 60) AS g(i);

INSERT INTO public.reg9_report (id, site_id, recipients)
SELECT i,
       i,
       CASE
           -- Every shape of the literal the masker has to write back: several
           -- elements, one element, and the empty array the column defaults to.
           WHEN i % 10 = 0 THEN ARRAY[]::citext[]
           WHEN i % 3 = 0 THEN ARRAY['ada.lovelace' || i || '@fixture.test']::citext[]
           ELSE ARRAY[
               'ada.lovelace' || i || '@fixture.test',
               'grace.hopper' || i || '@reports.fixture.test',
               'alan.turing' || i || '@fixture.test'
           ]::citext[]
       END
FROM generate_series(1, 60) AS g(i);
