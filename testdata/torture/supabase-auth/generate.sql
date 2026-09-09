--
-- supabase-auth/generate.sql
--
-- An authentication schema is the densest personal data in the set: every
-- column is either an identifier, a credential or a timestamp about a person.
-- It is one of the three schemas docs/TORTURE.md labels by hand for that
-- reason — there is nowhere for a classifier to hide.
--
-- Two shapes here are not in django's or rails-activestorage's:
--
--   * personal data inside JSON. `users.raw_user_meta_data` and
--     `identities.identity_data` are where the provider's profile lands, so the
--     name, the address and the avatar URL are values inside a jsonb document
--     rather than columns. ARCHITECTURE.md §4 has a rule for JSON columns and
--     this is what exercises it.
--   * a reference with no foreign key. `refresh_tokens.user_id` is varchar(255)
--     holding the user's uuid as text — upstream never declared the constraint —
--     so a subsetting tool that only follows declared edges will leave those
--     rows behind, correctly, and a tool that guesses will not.
--
-- Everything lives in the `auth` schema, so this fixture is also the one that
-- says a run works when nothing at all is in `public`.
--

SELECT lazyslice_torture.fill('auth.instances', 3);
SELECT lazyslice_torture.fill('auth.users', 300);
SELECT lazyslice_torture.fill('auth.identities', 300);
SELECT lazyslice_torture.fill('auth.sessions', 300);
SELECT lazyslice_torture.fill('auth.mfa_factors', 120);
SELECT lazyslice_torture.fill('auth.one_time_tokens', 150);
SELECT lazyslice_torture.fill('auth.audit_log_entries', 400);
SELECT lazyslice_torture.fill('auth.flow_state', 100);
SELECT lazyslice_torture.fill('auth.sso_providers', 6);
SELECT lazyslice_torture.fill('auth.sso_domains', 6);
SELECT lazyslice_torture.fill('auth.refresh_tokens', 400);

-- The people. `phone` is E.164 and deliberately outside the 555-01XX range the
-- `phone` masker emits into (ARCHITECTURE.md §5), the same way the addresses
-- stay off example.com/.net/.org (T-0073).
UPDATE auth.users u
SET email = g.mail,
    phone = '+3197010' || lpad((200000 + g.n)::text, 6, '0'),
    encrypted_password = '$2a$10$' || md5('pw' || g.n) || substr(md5('pw2' || g.n), 1, 21),
    confirmation_token = md5('confirm' || g.n),
    recovery_token = md5('recover' || g.n),
    email_change = CASE WHEN g.n % 5 = 0 THEN 'new.' || g.mail END,
    aud = 'authenticated',
    role = 'authenticated',
    raw_user_meta_data = jsonb_build_object(
        'full_name', g.given || ' ' || g.family,
        'email', g.mail,
        'phone_number', '+3197010' || lpad((200000 + g.n)::text, 6, '0'),
        'avatar_url', 'https://cdn.' || g.domain || '/avatars/' || lower(g.given) || '-' || g.n || '.png',
        'address', g.n || ' Kanaalstraat, Utrecht'),
    raw_app_meta_data = jsonb_build_object('provider', 'email', 'providers', jsonb_build_array('email')),
    created_at = TIMESTAMPTZ '2025-01-01 00:00:00+00' + (g.n || ' hours')::interval,
    updated_at = TIMESTAMPTZ '2025-01-02 00:00:00+00' + (g.n || ' hours')::interval,
    last_sign_in_at = TIMESTAMPTZ '2025-06-01 00:00:00+00' + (g.n || ' minutes')::interval
FROM (
    SELECT id,
           row_number() OVER (ORDER BY id) AS n,
           (ARRAY['Anouk','Berat','Catrin','Daan','Elif','Fenna','Gijs','Hafsa','Ivar','Jip'])
               [1 + (row_number() OVER (ORDER BY id) % 10)::int] AS given,
           (ARRAY['Aksoy','Bakhuis','Cruyff','Dijkstra','Es','Fransen','Gulen','Hoekstra','Ismail','Janssens'])
               [1 + ((row_number() OVER (ORDER BY id) / 10) % 10)::int] AS family,
           (ARRAY['veldhuis.test','kanaalwerk.test','stadsdeel.test'])
               [1 + (row_number() OVER (ORDER BY id) % 3)::int] AS domain,
           lower((ARRAY['anouk','berat','catrin','daan','elif','fenna','gijs','hafsa','ivar','jip'])
               [1 + (row_number() OVER (ORDER BY id) % 10)::int])
             || '.' || lower((ARRAY['aksoy','bakhuis','cruyff','dijkstra','es','fransen','gulen','hoekstra','ismail','janssens'])
               [1 + ((row_number() OVER (ORDER BY id) / 10) % 10)::int])
             || row_number() OVER (ORDER BY id)
             || '@' || (ARRAY['veldhuis.test','kanaalwerk.test','stadsdeel.test'])
               [1 + (row_number() OVER (ORDER BY id) % 3)::int] AS mail
    FROM auth.users
) g
WHERE g.id = u.id;

-- The provider profile, as the provider sent it.
UPDATE auth.identities i
SET identity_data = jsonb_build_object(
        'sub', u.id::text,
        'email', u.email,
        'email_verified', true,
        'name', u.raw_user_meta_data ->> 'full_name',
        'preferred_username', split_part(u.email, '@', 1)),
    provider = (ARRAY['email','google','github'])[1 + (('x' || substr(md5(i.id::text), 1, 4))::bit(16)::int % 3)],
    provider_id = u.id::text,
    -- identities.email is GENERATED ALWAYS AS (lower(identity_data ->> 'email'))
    -- upstream, so it is set by the identity_data above and cannot be written
    -- here. It is in the truth set as an email column all the same.
    created_at = TIMESTAMPTZ '2025-01-03 00:00:00+00',
    updated_at = TIMESTAMPTZ '2025-01-03 00:00:00+00'
FROM auth.users u
WHERE u.id = i.user_id;

-- The undeclared edge: the user's uuid, as text, in a column with no foreign
-- key behind it, next to a session that does have one.
UPDATE auth.refresh_tokens r
SET user_id = u.id::text,
    token = substr(md5('rt' || r.id::text), 1, 22) || r.id,
    parent = CASE WHEN r.id % 4 = 0 THEN substr(md5('rt' || (r.id - 1)::text), 1, 22) || (r.id - 1) END,
    revoked = (r.id % 7 = 0),
    session_id = s.id
FROM (
    SELECT id, row_number() OVER (ORDER BY id) AS n FROM auth.refresh_tokens
) k
JOIN LATERAL (SELECT id FROM auth.users ORDER BY id OFFSET ((k.n - 1) % 300) LIMIT 1) u ON true
JOIN LATERAL (SELECT id FROM auth.sessions ORDER BY id OFFSET ((k.n - 1) % 300) LIMIT 1) s ON true
WHERE k.id = r.id;

-- An audit trail records the address that acted and the address it acted on.
UPDATE auth.audit_log_entries a
SET ip_address = '10.42.' || ((a.id::text ~ '^[0-9]' )::int) || '.1',
    payload = jsonb_build_object(
        'action', 'login',
        'actor_username', u.email,
        'actor_id', u.id::text,
        'traits', jsonb_build_object('provider', 'email'))
FROM (
    SELECT id, row_number() OVER (ORDER BY id) AS n FROM auth.audit_log_entries
) k
JOIN LATERAL (SELECT id, email FROM auth.users ORDER BY id OFFSET ((k.n - 1) % 300) LIMIT 1) u ON true
WHERE k.id = a.id;

UPDATE auth.sessions s
SET ip = ('10.42.' || ((s.created_at IS NOT NULL)::int) || '.2')::inet,
    user_agent = 'Mozilla/5.0 (Macintosh; Intel Mac OS X 14_5) torture/' || substr(s.id::text, 1, 8),
    created_at = TIMESTAMPTZ '2025-06-01 00:00:00+00',
    updated_at = TIMESTAMPTZ '2025-06-01 01:00:00+00';
