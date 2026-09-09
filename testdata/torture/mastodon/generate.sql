--
-- mastodon/generate.sql
--
-- Mastodon splits a person in two: `accounts` is the public identity — the
-- handle, the display name, the bio, the avatar — and `users` is the private
-- half — the address, the password digest, the sign-up IP, the confirmation
-- token. The two are one-to-one and the root of the torture run is `accounts`,
-- because 85 foreign keys point at it and none of them point at `users`. So
-- every private column in this fixture is reached the long way round, through a
-- child table's parent edge, which is the case a subsetting tool gets wrong
-- quietly.
--
-- Two more shapes:
--
--   * `accounts.id` and eight other primary keys default to
--     `timestamp_id('accounts')`, a plpgsql function returning a snowflake id
--     rather than a sequence value. testdata/torture/mastodon/README.md says
--     where that function comes from and why the fixture's copy has a fixed
--     salt.
--   * `notifications` points at what happened through (activity_type,
--     activity_id) with no foreign key behind it, the third polymorphic pair in
--     the set after ActiveStorage's and Metabase's.
--

SELECT lazyslice_torture.fill('public.accounts', 300);
SELECT lazyslice_torture.fill('public.users', 300);
SELECT lazyslice_torture.fill('public.account_stats', 300);
SELECT lazyslice_torture.fill('public.statuses', 400);
SELECT lazyslice_torture.fill('public.status_stats', 400);
SELECT lazyslice_torture.fill('public.favourites', 400);
SELECT lazyslice_torture.fill('public.follows', 400);
SELECT lazyslice_torture.fill('public.media_attachments', 200);
SELECT lazyslice_torture.fill('public.notifications', 300);
SELECT lazyslice_torture.fill('public.account_aliases', 150);
SELECT lazyslice_torture.fill('public.session_activations', 200);

-- The public half of a person.
UPDATE public.accounts a
SET username = lower(g.given) || '_' || lower(g.family) || g.n,
    domain = CASE WHEN g.n % 4 = 0 THEN 'remote-' || (g.n % 7) || '.social.test' END,
    display_name = g.given || ' ' || g.family,
    note = g.given || ' ' || g.family || '. Reach me at ' || g.mail || ' or ' || g.phone || '.',
    uri = 'https://' || COALESCE('remote-' || (g.n % 7) || '.social.test', 'home.social.test')
          || '/users/' || lower(g.given) || '_' || lower(g.family) || g.n,
    url = 'https://home.social.test/@' || lower(g.given) || '_' || lower(g.family) || g.n,
    avatar_file_name = lower(g.given) || '-' || g.n || '.png',
    created_at = TIMESTAMP '2025-01-01 00:00:00' + (g.n || ' hours')::interval,
    updated_at = TIMESTAMP '2025-01-02 00:00:00' + (g.n || ' hours')::interval
FROM (
    SELECT id,
           row_number() OVER (ORDER BY id) AS n,
           (ARRAY['Ayo','Bea','Cem','Dara','Eyal','Bracken','Gio','Hedda','Idrissa','Jae'])
               [1 + (row_number() OVER (ORDER BY id) % 10)::int] AS given,
           (ARRAY['Adebayo','Blanc','Cakir','Donnelly','Erez','Fogel','Grecu','Holm','Idrissi','Jang'])
               [1 + ((row_number() OVER (ORDER BY id) / 10) % 10)::int] AS family,
           lower((ARRAY['ayo','bea','cem','dara','eyal','bracken','gio','hedda','idrissa','jae'])
               [1 + (row_number() OVER (ORDER BY id) % 10)::int])
             || '.' || lower((ARRAY['adebayo','blanc','cakir','donnelly','erez','fogel','grecu','holm','idrissi','jang'])
               [1 + ((row_number() OVER (ORDER BY id) / 10) % 10)::int])
             || row_number() OVER (ORDER BY id)
             || '@' || (ARRAY['fernpost.test','tidalmail.test','quarrylane.test'])
               [1 + (row_number() OVER (ORDER BY id) % 3)::int] AS mail,
           '+353 1 4' || lpad((100000 + row_number() OVER (ORDER BY id))::text, 6, '0') AS phone
    FROM public.accounts
) g
WHERE g.id = a.id;

-- The private half.
UPDATE public.users u
SET email = split_part(a.note, ' ', 6),
    encrypted_password = '$2a$10$' || md5('pw' || u.id) || substr(md5('pw2' || u.id), 1, 21),
    confirmation_token = md5('confirm' || u.id),
    reset_password_token = CASE WHEN u.id % 9 = 0 THEN md5('reset' || u.id) END,
    sign_up_ip = ('10.' || (u.id % 250) || '.' || ((u.id / 250) % 250) || '.9')::inet,
    last_sign_in_at = TIMESTAMP '2025-08-01 00:00:00' + (u.id % 10000 || ' minutes')::interval,
    locale = (ARRAY['en','ga','pt','ja'])[1 + (u.id % 4)],
    created_at = TIMESTAMP '2025-01-01 00:00:00',
    updated_at = TIMESTAMP '2025-01-02 00:00:00'
FROM public.accounts a
WHERE a.id = u.account_id;

-- A post is free text, and free text is where the addresses that nobody
-- declared as an address column live.
UPDATE public.statuses s
SET text = CASE WHEN s.id % 3 = 0
                THEN 'Anyone got a spare ticket for Saturday?'
                ELSE 'DM me on ' || split_part(a.note, ' ', 6) || ' — faster than here.'
           END,
    uri = 'https://home.social.test/users/' || a.username || '/statuses/' || s.id,
    url = 'https://home.social.test/@' || a.username || '/' || s.id,
    language = 'en',
    created_at = TIMESTAMP '2025-07-01 00:00:00' + (s.id % 100000 || ' seconds')::interval,
    updated_at = TIMESTAMP '2025-07-01 00:00:00' + (s.id % 100000 || ' seconds')::interval
FROM public.accounts a
WHERE a.id = s.account_id;

-- The polymorphic pair, pointed at real rows.
UPDATE public.notifications n
SET activity_type = 'Status',
    activity_id = s.id,
    type = 'mention'
FROM (
    SELECT id, row_number() OVER (ORDER BY id) AS k FROM public.notifications
) r
JOIN LATERAL (SELECT id FROM public.statuses ORDER BY id OFFSET ((r.k - 1) % 400) LIMIT 1) s ON true
WHERE r.id = n.id;

-- An uploaded file's name and its alt text are both written by a person about a
-- person.
-- media_attachments.account_id is nullable, so the generator left it NULL — a
-- MATCH SIMPLE reference to nothing, which is a shape worth having in the
-- fixture and not one to fill in everywhere. Four fifths of the rows are given
-- an owner here; the rest stay orphaned, which is what an unattached upload
-- looks like in a real instance and what a subset must not invent a parent for.
UPDATE public.media_attachments m
SET account_id = CASE WHEN r.k % 5 <> 0 THEN a.id END,
    file_file_name = 'photo-' || a.username || '-' || r.k || '.jpg',
    file_content_type = 'image/jpeg',
    description = 'Photo of ' || a.display_name || ' at the harbour.',
    -- remote_url is NOT NULL DEFAULT '' upstream, so the "no remote copy"
    -- value is the empty string and not NULL.
    remote_url = CASE WHEN r.k % 5 = 0 THEN 'https://remote-3.social.test/media/' || md5(m.id::text) || '.jpg' ELSE '' END
FROM (
    SELECT id, row_number() OVER (ORDER BY id) AS k FROM public.media_attachments
) r
JOIN LATERAL (
    SELECT id, username, display_name FROM public.accounts ORDER BY id OFFSET ((r.k - 1) % 300) LIMIT 1
) a ON true
WHERE r.id = m.id;

UPDATE public.session_activations
SET ip = ('10.' || (id % 250) || '.' || ((id / 250) % 250) || '.11')::inet,
    user_agent = 'Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X) Mastodon/torture';
