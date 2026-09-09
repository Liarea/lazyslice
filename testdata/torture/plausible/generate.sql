--
-- plausible/generate.sql
--
-- Plausible's Postgres side is the account database: people, the sites they
-- own, the invitations they send and the reports they get emailed. (The events
-- themselves live in ClickHouse and are not part of this schema, which is why
-- the fixture is all identity and no analytics.)
--
-- The shape this one contributes is `citext`. `users.email` and
-- `invitations.email` are case-insensitive text, which is a domain over text as
-- far as the catalogue is concerned, and a classifier or a masker that reads
-- `typname` rather than resolving the domain sees a type it does not know
-- instead of an email column.
--
-- The root the torture run slices from is `public.sites` — twenty incoming
-- foreign keys, more than any other table here — so the run reaches people
-- through `site_memberships`, upwards, rather than starting at them.
--

SELECT lazyslice_torture.fill('public.users', 300);
SELECT lazyslice_torture.fill('public.sites', 300);
SELECT lazyslice_torture.fill('public.site_memberships', 300);
-- goals carries `CHECK (event_name IS NOT NULL AND page_path IS NULL OR
-- event_name IS NULL AND page_path IS NOT NULL)` over two nullable columns, so
-- the generator cannot fill it: it writes NOT NULL columns and leaves nullable
-- ones alone, which is exactly the row this constraint forbids. Written out
-- here instead, which is what _common/fill.sql's header says to do.
INSERT INTO public.goals (event_name, page_path, inserted_at, updated_at, site_id, display_name)
SELECT CASE WHEN i % 2 = 0 THEN 'Signup ' || i END,
       CASE WHEN i % 2 = 1 THEN '/checkout/' || i END,
       TIMESTAMP '2025-03-01 00:00:00' + (i || ' minutes')::interval,
       TIMESTAMP '2025-03-01 00:00:00' + (i || ' minutes')::interval,
       (SELECT s.id FROM public.sites s ORDER BY s.id OFFSET ((i - 1) % 300) LIMIT 1),
       'Goal ' || i
FROM generate_series(1, 200) AS g(i);
SELECT lazyslice_torture.fill('public.funnels', 150);
SELECT lazyslice_torture.fill('public.funnel_steps', 150);
SELECT lazyslice_torture.fill('public.shared_links', 150);
SELECT lazyslice_torture.fill('public.invitations', 120);
SELECT lazyslice_torture.fill('public.api_keys', 100);
SELECT lazyslice_torture.fill('public.monthly_reports', 100);
SELECT lazyslice_torture.fill('public.weekly_reports', 100);
SELECT lazyslice_torture.fill('public.site_user_preferences', 100);

-- The people.
UPDATE public.users u
SET name = g.given || ' ' || g.family,
    email = (lower(g.given) || '.' || lower(g.family) || u.id || '@' || g.domain)::citext,
    password_hash = '$2b$12$' || md5('pw' || u.id) || substr(md5('pw2' || u.id), 1, 21),
    last_seen = TIMESTAMP '2025-07-01 00:00:00' + (u.id || ' minutes')::interval,
    inserted_at = TIMESTAMP '2025-01-01 00:00:00' + (u.id || ' hours')::interval,
    updated_at = TIMESTAMP '2025-01-02 00:00:00' + (u.id || ' hours')::interval
FROM (
    SELECT id,
           (ARRAY['Aisling','Brunon','Clarita','Dariusz','Eero','Freyja','Gustavo','Hanae','Ismael','Johanna'])[1 + (id % 10)] AS given,
           (ARRAY['Alvarado','Bergman','Costeira','Duarte','Eskola','Ferreira','Gronholm','Havel','Isaksen','Jorgensen'])[1 + ((id / 10) % 10)] AS family,
           (ARRAY['tidewater.test','glasshouse.test','northgate.test'])[1 + (id % 3)] AS domain
    FROM public.users
) g
WHERE g.id = u.id;

-- A site's domain is not personal data on its own, but a personal blog's is,
-- and a third of these are one. docs/TORTURE.md does not label this schema by
-- hand; the mixture is here because the classifier's sampling should see one.
UPDATE public.sites s
SET domain = CASE WHEN s.id % 3 = 0
                  THEN lower(replace(u.name, ' ', '-')) || '-' || s.id || '.tidewater.test'
                  ELSE 'shop-' || s.id || '.glasshouse.test'
             END,
    timezone = (ARRAY['Etc/UTC','Europe/Lisbon','Europe/Helsinki','America/Bogota'])[1 + (s.id % 4)],
    inserted_at = TIMESTAMP '2025-02-01 00:00:00' + (s.id || ' hours')::interval,
    updated_at = TIMESTAMP '2025-02-02 00:00:00' + (s.id || ' hours')::interval
FROM public.users u
WHERE u.id = ((s.id - 1) % 300) + 1;

-- An invitation carries the address of somebody who does not have an account
-- yet: personal data about a non-user, in a table nobody thinks of first.
UPDATE public.invitations i
SET email = ('invited.' || i.id || '@' || (ARRAY['tidewater.test','glasshouse.test','northgate.test'])[1 + (i.id % 3)])::citext,
    inserted_at = TIMESTAMP '2025-04-01 00:00:00' + (i.id || ' minutes')::interval,
    updated_at = TIMESTAMP '2025-04-01 00:00:00' + (i.id || ' minutes')::interval;

-- An API key hash is a credential. The prefix is not, and the pair is the
-- interesting case: two columns of the same table where only one must go.
UPDATE public.api_keys k
SET name = 'CI token ' || k.id,
    key_prefix = substr(md5('prefix' || k.id), 1, 6),
    key_hash = encode(sha256(('key' || k.id)::bytea), 'hex'),
    inserted_at = TIMESTAMP '2025-05-01 00:00:00' + (k.id || ' minutes')::interval,
    updated_at = TIMESTAMP '2025-05-01 00:00:00' + (k.id || ' minutes')::interval;

UPDATE public.shared_links l
SET name = 'Dashboard for ' || u.name,
    password_hash = CASE WHEN l.id % 4 = 0 THEN '$2b$12$' || md5('link' || l.id) || substr(md5('link2' || l.id), 1, 21) END
FROM public.users u
WHERE u.id = ((l.id - 1) % 300) + 1;
