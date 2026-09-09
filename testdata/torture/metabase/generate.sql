--
-- metabase/generate.sql
--
-- Metabase's application database is a BI tool's own state: the people who log
-- in, the questions they saved, and — the part that matters here — the
-- credentials for every warehouse it connects to. `metabase_database.details`
-- is a JSON blob holding a host, a user and a password, and it is the clearest
-- example in the whole set of a column whose name says nothing and whose
-- contents must not leave the building.
--
-- The other shape this schema contributes is `view_log` and `recent_views`,
-- which point at whatever was looked at through (model, model_id) — the same
-- undeclared polymorphic reference ActiveStorage uses, in a table with hundreds
-- of thousands of rows in a real install.
--
-- Metabase writes its schema with Liquibase and its identifiers are unquoted
-- lower case throughout, so nothing here exercises quoting; calcom is the
-- fixture for that.
--

SELECT lazyslice_torture.fill('public.core_user', 300);
SELECT lazyslice_torture.fill('public.permissions_group', 20);
SELECT lazyslice_torture.fill('public.permissions_group_membership', 300);
SELECT lazyslice_torture.fill('public.collection', 120);
SELECT lazyslice_torture.fill('public.metabase_database', 40);
SELECT lazyslice_torture.fill('public.metabase_table', 200);
SELECT lazyslice_torture.fill('public.metabase_field', 400);
SELECT lazyslice_torture.fill('public.report_card', 200);
SELECT lazyslice_torture.fill('public.report_dashboard', 100);
SELECT lazyslice_torture.fill('public.login_history', 400);
SELECT lazyslice_torture.fill('public.core_session', 300);
SELECT lazyslice_torture.fill('public.view_log', 400);

-- The people who log in.
UPDATE public.core_user u
SET first_name = g.given,
    last_name  = g.family,
    email      = (lower(g.given) || '.' || lower(g.family) || u.id || '@' || g.domain)::citext,
    password   = md5('pw' || u.id) || md5('pw2' || u.id),
    password_salt = md5('salt' || u.id),
    date_joined = TIMESTAMPTZ '2025-01-01 00:00:00+00' + (u.id || ' hours')::interval,
    last_login  = TIMESTAMPTZ '2025-08-01 00:00:00+00' + (u.id || ' minutes')::interval,
    settings   = '{"notebook-native-preview-shown":false}'
FROM (
    SELECT id,
           (ARRAY['Alma','Bode','Ciara','Deniz','Eliasz','Farhana','Gunnar','Hugues','Iris','Jonasz'])[1 + (id % 10)] AS given,
           (ARRAY['Andersson','Biancardi','Cerny','Dubourg','Ekwueme','Fischbach','Grigoryan','Horvath','Ivanova','Jakobsen'])[1 + ((id / 10) % 10)] AS family,
           (ARRAY['quarryside.test','loamfield.test','beacon-hill.test'])[1 + (id % 3)] AS domain
    FROM public.core_user
) g
WHERE g.id = u.id;

-- The warehouse credentials. In production this column is encrypted when
-- MB_ENCRYPTION_SECRET_KEY is set and plaintext JSON when it is not, and the
-- second is the common case; the fixture is the second.
UPDATE public.metabase_database d
SET name = 'Warehouse ' || d.id,
    engine = (ARRAY['postgres','mysql','redshift','bigquery-cloud-sdk'])[1 + (d.id % 4)],
    details = '{"host":"db-' || d.id || '.internal.quarryside.test",'
           || '"port":5432,'
           || '"dbname":"analytics",'
           || '"user":"metabase_ro",'
           || '"password":"' || md5('warehouse' || d.id) || '",'
           || '"ssl":true}';

-- Where people logged in from. `ip_address` is text, not inet, which is how
-- Metabase stores it and which is the case a type-driven classifier misses.
UPDATE public.login_history l
SET ip_address = '10.' || (l.id % 250) || '.' || ((l.id / 250) % 250) || '.7',
    device_description = 'Mozilla/5.0 (X11; Linux x86_64) Chrome/128.0.0.0 Safari/537.36',
    device_id = lpad(md5('dev' || l.id), 36, '0');

-- A saved question's name and description routinely carry a customer's name;
-- the query text carries their identifiers.
UPDATE public.report_card c
SET name = 'Churn risk — ' || u.first_name || ' ' || u.last_name,
    description = 'Requested by ' || u.email || ' on 2025-08-01.',
    dataset_query = '{"type":"native","native":{"query":"select * from orders where contact_email = ''' || u.email || '''"}}'
FROM public.core_user u
WHERE u.id = c.creator_id;

UPDATE public.report_dashboard d
SET name = 'Weekly review (' || u.first_name || ')',
    description = 'Owner ' || u.email
FROM public.core_user u
WHERE u.id = d.creator_id;
