--
-- django/generate.sql
--
-- Fills the ten tables `django-admin startproject` plus `manage.py migrate`
-- leaves behind. The order is the foreign-key order: a parent is always filled
-- before the child that picks rows out of it.
--
-- The UPDATE statements at the bottom are the point of the fixture. This schema
-- is one of the three docs/TORTURE.md labels by hand, so the personal data goes
-- in deliberately and in two shapes:
--
--   * columns whose name says what they hold — email, first_name, last_name;
--   * a column whose name says nothing — django_admin_log.object_repr, which is
--     the string Django's admin logs for the object that was changed and which
--     in a real deployment routinely carries the changed user's address.
--
-- The second shape is the one worth having. A classifier that reads only column
-- names finds the first three and reports clean over the fourth, and the truth
-- set in docs/TORTURE.md is what turns that into a recall number.
--
-- Addresses are under `.test` and never example.com/.net/.org: ARCHITECTURE.md
-- §5 makes those three the `email` masker's own output space, so a source value
-- there is a documented false-positive surface for the residual scan (T-0073).
--

-- Django's own registries. They are filled with what Django actually puts in
-- them — 'auth', 'add_user', '0001_initial' — and not with the generator's
-- `table.column-N` strings, because docs/TORTURE.md hand-labels this schema and
-- a precision figure measured against invented values would be measuring the
-- fixture. Short, low-entropy, unmistakably not personal data.
INSERT INTO public.django_content_type (app_label, model)
SELECT (ARRAY['auth','admin','contenttypes','sessions','shop','billing','support','audit'])[1 + (i / 5)],
       (ARRAY['user','group','permission','logentry','session'])[1 + (i % 5)]
FROM generate_series(0, 39) AS g(i);

INSERT INTO public.auth_permission (name, content_type_id, codename)
SELECT (ARRAY['Can add','Can change','Can delete','Can view','Can export'])[1 + (i % 5)] || ' ' || ct.model,
       ct.id,
       (ARRAY['add','change','delete','view','export'])[1 + (i % 5)] || '_' || ct.model || '_' || i
FROM generate_series(1, 200) AS g(i)
JOIN LATERAL (SELECT id, model FROM public.django_content_type ORDER BY id OFFSET ((i - 1) % 40) LIMIT 1) ct ON true;

INSERT INTO public.auth_group (name)
SELECT (ARRAY['Editors','Moderators','Support','Finance','Read only'])[1 + (i % 5)] || ' ' || i
FROM generate_series(1, 20) AS g(i);
SELECT lazyslice_torture.fill('public.auth_user', 300);
SELECT lazyslice_torture.fill('public.auth_user_groups', 300);
SELECT lazyslice_torture.fill('public.auth_user_user_permissions', 300);
SELECT lazyslice_torture.fill('public.django_admin_log', 400);
SELECT lazyslice_torture.fill('public.django_session', 120);
SELECT lazyslice_torture.fill('public.django_migrations', 30);

-- Personal data, deterministic in the row's own id so the fixture is stable.
UPDATE public.auth_user u
SET username   = lower(g.given) || '.' || lower(g.family) || (u.id % 97),
    first_name = g.given,
    last_name  = g.family,
    email      = lower(g.given) || '.' || lower(g.family) || (u.id % 97) || '@' || g.domain,
    password   = 'pbkdf2_sha256$720000$' || md5('salt' || u.id) || '$' || md5('hash' || u.id) || '=',
    last_login = TIMESTAMPTZ '2025-02-01 08:00:00+00' + (u.id || ' hours')::interval
FROM (
    SELECT id,
           (ARRAY['Amarah','Bjorn','Chidi','Dagny','Ekene','Farida','Goran','Hyeon','Ingrid','Jarrah',
                  'Kwame','Lucia','Mateus','Nadira','Oleg','Priyal','Quentin','Raniya','Sione','Tomasz'])
               [1 + (id % 20)] AS given,
           (ARRAY['Adeyemi','Bergqvist','Cortazar','Dlamini','Eriksson','Fontanet','Gudmundsson','Haddad',
                  'Ibrahimi','Jansson','Kowalski','Lindholm','Morvan','Nakagawa','Okonkwo','Petrenko',
                  'Quintana','Rasmussen','Sandoval','Takahashi'])
               [1 + ((id / 20) % 20)] AS family,
           (ARRAY['northwind-labs.test','borealis-works.test','tessellate.test','harborlight.test'])
               [1 + (id % 4)] AS domain
    FROM public.auth_user
) g
WHERE g.id = u.id;

-- The unlabelled carrier: an admin log line that quotes the user it is about.
-- Two thirds of the rows carry an address, the rest do not, so a classifier
-- that samples the column sees a real mixture rather than a uniform one.
UPDATE public.django_admin_log l
SET object_repr = CASE WHEN l.id % 3 = 0
                       THEN 'Group object (' || l.id || ')'
                       ELSE 'User: ' || u.email
                  END,
    change_message = CASE WHEN l.id % 3 = 0
                          THEN '[{"changed": {"fields": ["name"]}}]'
                          ELSE '[{"changed": {"fields": ["email"], "was": "' || u.email || '"}}]'
                     END,
    object_id = u.id::text
FROM public.auth_user u
WHERE u.id = l.user_id;

-- Django's migration table, with the names Django writes.
UPDATE public.django_migrations
SET app = (ARRAY['auth','admin','contenttypes','sessions'])[1 + (id % 4)],
    name = lpad(id::text, 4, '0') || '_' || (ARRAY['initial','alter_user','add_index','remove_field'])[1 + (id % 4)];

-- A session payload is a signed blob in production; it is a credential, not a
-- name, and it is in the truth set as one.
UPDATE public.django_session
SET session_data = encode(convert_to('{"_auth_user_id":"' || (length(session_key) + 1) || '","_auth_user_backend":"django.contrib.auth.backends.ModelBackend"}', 'UTF8'), 'base64');
