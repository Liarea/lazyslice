-- root:   public.reg5_site
-- take:   20
-- expect: ok
-- found:  plausible
-- why:    an array of an extension's base type had no codec on the target connection
--
-- Plausible's `monthly_reports.recipients` is `citext[] NOT NULL DEFAULT
-- ARRAY[]::citext[]` — the list of addresses a site's monthly report is emailed
-- to. The load reached that table and died inside the driver:
--
--     ✗ the rows of public.monthly_reports did not go in: SQLSTATE 08P01;
--       the table is empty, not half loaded
--     lazyslice: insufficient data left in message (SQLSTATE 08P01)
--
-- Reduced to one table it is a different SQLSTATE and the same defect:
--
--     lazyslice: number of array dimensions (2069967229) exceeds the maximum
--       allowed (6) (SQLSTATE 54000)
--
-- Both are the wire format going wrong, which is what an unregistered type looks
-- like from the far end of a binary `COPY`.
--
-- ARCHITECTURE.md §11.1 and ADR-005 say the target's user-defined types are
-- "registered in AfterConnect", and internal/pg/types.go does exactly that — for
-- `typtype IN ('e', 'd', 'c')`: enums, domains and composites, the names taken
-- from `Schema.Enums`, `Schema.Domains` and `Schema.Composites`. `citext` is
-- none of the three. It is a **base** type (`typtype = 'b'`) that an extension
-- installs, so nothing asked for it, nothing registered it, and `_citext` — the
-- array type pgx builds over it — was never built at all.
--
-- The typeRegistry comment in internal/pg/types.go had already measured the
-- shape and named it one line too narrowly: "array of a domain over text: 54000
-- unregistered, loads registered". An array over an *extension base type* is the
-- same measurement and was not in the list.
--
-- The fix keeps the source of the names where it was — the schema says which
-- extensions a recreated column depends on (`Schema.Extensions`) — and widens
-- the target-side lookup to the base types those extensions own. Nothing is
-- registered for an extension the source does not use.
--
-- This fixture also carries the scalar case (`reg5_site.label citext`), which
-- loaded before the fix and must keep loading, and an `hstore` column — which
-- turned out to be a second, older defect of the same shape and is fixed in the
-- same place. A scalar `hstore` column could never be loaded at all: hstore is
-- not string-category, its binary form is nothing like its text form, and with
-- no codec registered pgx sent the text bytes as the binary body and the server
-- answered 08P01. Measured on a one-table database against the build before this
-- change as well as after it, so it is not a regression of the citext fix but
-- something the citext fix uncovered one column later. pgx ships
-- pgtype.HstoreCodec; registering it is the fix. Discourse installs the hstore
-- extension, which is how it came to be in this file.

CREATE EXTENSION IF NOT EXISTS citext;
CREATE EXTENSION IF NOT EXISTS hstore;

CREATE TABLE public.reg5_site (
    id     integer PRIMARY KEY,
    label  citext NOT NULL,
    -- An address, so the regression suite's leak check has something to look
    -- for; a fixture with no personal data proves nothing about masking.
    owner_email text NOT NULL
);

CREATE TABLE public.reg5_report (
    id         integer PRIMARY KEY,
    site_id    integer NOT NULL REFERENCES public.reg5_site (id),
    recipients citext[] NOT NULL DEFAULT ARRAY[]::citext[],
    settings   hstore
);

INSERT INTO public.reg5_site (id, label, owner_email)
SELECT i, ('Site ' || i)::citext, 'owner' || i || '@regression.test'
FROM generate_series(1, 60) AS g(i);

INSERT INTO public.reg5_report (id, site_id, recipients, settings)
SELECT i,
       i,
       -- Short, dull and deliberately not personal. An array of an *extension*
       -- type is sampled as the single string `{a,b}` — pgx hands back a slice
       -- only for an array type its own map knows, and the source pool
       -- registers none — so the classifier reads the whole literal as one
       -- value. Anything longer than fifteen characters with a digit in it then
       -- reads as a secret, the column is masked as a credential, and the fixed
       -- literal is handed to CopyFrom as a scalar for an `_citext` column:
       -- "unable to encode \"$lazyslice$invalid\" into binary format for
       -- _citext: cannot find encode plan". That is T-0103, not this file's
       -- defect, and keeping richer values here would make this regression fail
       -- for the other reason for ever.
       ARRAY['wk', 'mo']::citext[],
       hstore('locale', 'en') || hstore('site', i::text)
FROM generate_series(1, 60) AS g(i);
