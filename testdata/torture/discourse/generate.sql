--
-- discourse/generate.sql
--
-- Discourse is the fixture for a schema that barely uses foreign keys. 370
-- tables, 410 primary keys and 29 declared foreign keys: `posts.topic_id`,
-- `topics.category_id`, `user_emails.user_id` and almost every other reference
-- in the application is an integer column with an index and nothing else, kept
-- consistent by Rails rather than by the server.
--
-- That is the whole point of having it here. A subsetting tool that follows
-- declared edges will slice this database into a handful of tables and leave
-- 360 of them schema-only, and the important question is whether it *says so*
-- rather than whether it guesses. `public.user_emails` is the sharpest case:
-- every address in a Discourse install is in that table, it has no foreign key
-- to `public.users`, and a run rooted at users therefore does not reach it.
--
-- The tables here are filled anyway — a table that is empty in the source
-- proves nothing about a run that leaves it empty in the target.
--

SELECT lazyslice_torture.fill('public.users', 300);
SELECT lazyslice_torture.fill('public.user_profiles', 300);
SELECT lazyslice_torture.fill('public.user_emails', 300);
SELECT lazyslice_torture.fill('public.groups', 20);
SELECT lazyslice_torture.fill('public.categories', 20);
SELECT lazyslice_torture.fill('public.topics', 200);
SELECT lazyslice_torture.fill('public.posts', 400);
SELECT lazyslice_torture.fill('public.uploads', 150);
SELECT lazyslice_torture.fill('public.polls', 100);
SELECT lazyslice_torture.fill('public.poll_options', 300);
SELECT lazyslice_torture.fill('public.poll_votes', 300);
SELECT lazyslice_torture.fill('public.reviewables', 100);
SELECT lazyslice_torture.fill('public.reviewable_notes', 200);
SELECT lazyslice_torture.fill('public.user_security_keys', 150);
SELECT lazyslice_torture.fill('public.ad_plugin_house_ads', 20);
SELECT lazyslice_torture.fill('public.ad_plugin_house_ads_categories', 60);
SELECT lazyslice_torture.fill('public.ad_plugin_house_ads_groups', 60);
SELECT lazyslice_torture.fill('public.ad_plugin_house_ads_routes', 60);
SELECT lazyslice_torture.fill('public.ad_plugin_impressions', 300);

-- The people. Discourse keeps the handle in `users` and the address in
-- `user_emails`, one row per address with a `primary` flag.
UPDATE public.users u
SET username = lower(g.given) || lower(g.family) || u.id,
    username_lower = lower(g.given) || lower(g.family) || u.id,
    name = g.given || ' ' || g.family,
    title = CASE WHEN u.id % 11 = 0 THEN 'Moderator' END,
    registration_ip_address = ('10.' || (u.id % 250) || '.' || ((u.id / 250) % 250) || '.13')::inet,
    ip_address = ('10.' || (u.id % 250) || '.' || ((u.id / 250) % 250) || '.14')::inet,
    created_at = TIMESTAMP '2025-01-01 00:00:00' + (u.id || ' hours')::interval,
    updated_at = TIMESTAMP '2025-01-02 00:00:00' + (u.id || ' hours')::interval,
    last_seen_at = TIMESTAMP '2025-08-20 00:00:00' + (u.id || ' minutes')::interval,
    trust_level = u.id % 5
FROM (
    SELECT id,
           (ARRAY['Ana','Braam','Cai','Dilnoza','Emilis','Fionn','Gretel','Hamid','Inesa','Joris'])[1 + (id % 10)] AS given,
           (ARRAY['Aluko','Brandsma','Calvo','Duman','Ekman','Fahey','Grabowski','Hakim','Ivarsson','Jelinsky'])[1 + ((id / 10) % 10)] AS family
    FROM public.users
) g
WHERE g.id = u.id;

-- The addresses, in the table with no foreign key back to the people.
UPDATE public.user_emails e
SET email = lower(u.username) || '@' || (ARRAY['harbourline.test','stonecrop.test','wayfare.test'])[1 + (e.id % 3)],
    "primary" = (e.id % 2 = 1),
    created_at = TIMESTAMP '2025-01-01 00:00:00' + (e.id || ' hours')::interval,
    updated_at = TIMESTAMP '2025-01-02 00:00:00' + (e.id || ' hours')::interval
FROM public.users u
WHERE u.id = e.user_id;

UPDATE public.user_profiles p
SET bio_raw = 'I am ' || u.name || '. Mail me at ' || lower(u.username) || '@harbourline.test.',
    location = (ARRAY['Cork','Utrecht','Valparaíso','Ōtautahi'])[1 + (p.user_id % 4)],
    website = 'https://' || lower(u.username) || '.wayfare.test'
FROM public.users u
WHERE u.id = p.user_id;

-- Posts and topics: free text written by people about people.
UPDATE public.topics t
SET title = 'Ferry timetable question ' || t.id,
    fancy_title = 'Ferry timetable question ' || t.id,
    slug = 'ferry-timetable-question-' || t.id,
    user_id = u.id,
    last_post_user_id = u.id,
    created_at = TIMESTAMP '2025-05-01 00:00:00' + (t.id || ' hours')::interval,
    updated_at = TIMESTAMP '2025-05-01 00:00:00' + (t.id || ' hours')::interval,
    bumped_at = TIMESTAMP '2025-05-02 00:00:00' + (t.id || ' hours')::interval
FROM public.users u
WHERE u.id = ((t.id - 1) % 300) + 1;

UPDATE public.posts p
SET raw = CASE WHEN r.k % 4 = 0
               THEN 'Sailing at 07:40, no need to book.'
               ELSE 'Ask ' || u.name || ' — ' || lower(u.username) || '@harbourline.test — she has the roster.'
          END,
    cooked = '<p>' || CASE WHEN r.k % 4 = 0
                          THEN 'Sailing at 07:40, no need to book.'
                          ELSE 'Ask ' || u.name || ' — ' || lower(u.username) || '@harbourline.test — she has the roster.'
                     END || '</p>',
    user_id = u.id,
    last_editor_id = u.id,
    topic_id = t.id,
    -- (topic_id, post_number) is unique: the topic cycles every 200 rows, so
    -- the post number has to be the *quotient*, not another modulo.
    post_number = 1 + ((r.k - 1) / 200)
FROM (
    SELECT id, row_number() OVER (ORDER BY id) AS k FROM public.posts
) r
JOIN LATERAL (SELECT id, name, username FROM public.users ORDER BY id OFFSET ((r.k - 1) % 300) LIMIT 1) u ON true
JOIN LATERAL (SELECT id FROM public.topics ORDER BY id OFFSET ((r.k - 1) % 200) LIMIT 1) t ON true
WHERE r.id = p.id;

UPDATE public.uploads
SET original_filename = 'roster-' || id || '.pdf',
    url = '/uploads/default/original/1X/' || md5(id::text) || '.pdf',
    user_id = ((id - 1) % 300) + 1,
    extension = 'pdf';

-- The poll component's chain, whose every foreign key is nullable, so the
-- generator left all of it unreferenced. Pointing it at real rows is what makes
-- the slice from `public.users` reach anything at all: poll_votes -> users is
-- the only edge in this schema that carries a person two tables deep.
UPDATE public.polls SET post_id = ((id - 1) % 400) + 1;

UPDATE public.poll_options SET poll_id = ((id - 1) % 100) + 1;

-- poll_votes has no key of its own — no id, no primary key, just a unique index
-- over its three nullable foreign keys — so the row is addressed by ctid.
-- (That also makes it one of the two tables in this fixture whose identity
-- ladder, ARCHITECTURE.md §3.4, has to fall through to the unique index.)
UPDATE public.poll_votes v
SET poll_id = x.poll_id, poll_option_id = x.opt_id, user_id = x.user_id
FROM (
    SELECT r.ctid, o.poll_id, o.id AS opt_id, u.id AS user_id
    FROM (SELECT ctid, row_number() OVER (ORDER BY ctid) AS k FROM public.poll_votes) r
    JOIN LATERAL (SELECT id, poll_id FROM public.poll_options ORDER BY id OFFSET ((r.k - 1) % 300) LIMIT 1) o ON true
    JOIN LATERAL (SELECT id FROM public.users ORDER BY id OFFSET ((r.k - 1) % 300) LIMIT 1) u ON true
) x
WHERE v.ctid = x.ctid;

UPDATE public.ad_plugin_impressions i
SET user_id = ((i.id - 1) % 300) + 1,
    ad_plugin_house_ad_id = ((i.id - 1) % 20) + 1;

UPDATE public.reviewable_notes n
SET content = 'Reported by ' || u.name || ' (' || lower(u.username) || '@stonecrop.test).'
FROM public.users u
WHERE u.id = n.user_id;
