-- root:   public.reg9_site
-- take:   20
-- expect: exit 12 plan.refused.unwritable
-- found:  plausible
-- why:    a citext[] of addresses arrived as one text literal and was masked as one scalar
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
--   * internal/plan refused the column at exit 12 in between the two, so that
--     landing the classify half alone could not half-load a target. That
--     refusal is a stand-in for the masker and goes with it.
--
-- **The header asserts the refusal, not the load, and that is the tree as it
-- stands.** internal/plan was outside T-0118's paths, so `arrayArrivesAsLiteral`
-- in internal/plan/writeback.go is still standing in for the masker that now
-- exists, and it refuses this column before a row moves:
--
--     plan.refused.unwritable: public.reg9_report.recipients is masked as email
--       and its type citext[] is an array no masker can write element-wise yet
--
-- So what this file asserts today is that the refusal is still the one the
-- operator gets, whole and coded, rather than the exit 7 mid-load that the
-- unfixed transform gave. A header saying `ok` here would have been a wish:
-- README.md's rule is that `expect: ok` means the run must now succeed, and
-- `make torture` has to be green at every commit or it stops being able to tell
-- a new defect from a known one.
--
-- **Tracker T-0127 flips this header back to `ok` in the same change that
-- removes `arrayArrivesAsLiteral`** — one line of removal there, one line here.
-- What the file will then assert is the whole chain: the run exits 0, so the
-- literal is something `CopyFrom` can write; and the suite's own leak check
-- (`assertTortureNoLiteralSurvives`, which the harness runs only for a file
-- expecting exit 0) finds none of the source's addresses in the target, so the
-- elements inside the literal were masked rather than copied. A run with either
-- half missing fails one of the two — the old transform on the first, the old
-- classify on the second. Until then the element-wise masker has unit coverage
-- in internal/transform/array_test.go and no end-to-end coverage from the CLI,
-- because the pipeline stops at plan before a row moves.
--
-- What that leak check is *not* is the residual scan. It is this suite's own
-- external grep of the target, and it covers this fixture only. Inside the
-- product, ARCHITECTURE.md §6 item 1's residual scan is blind to a masked array
-- that arrives as a literal (T-0129: internal/verify reads the column back as
-- one string, so the per-element filter entries transform makes match nothing
-- and the scan passes green). That blindness is free while `arrayArrivesAsLiteral`
-- stands, because no such column reaches a target at all. The change that flips
-- this header is the change that ends that, so T-0127 is ordered behind T-0129
-- and its log says so: a green run here would otherwise be the only evidence,
-- and one fixture's grep is not the control T12 is owed.
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
