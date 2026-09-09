--
-- gitlab/generate.sql
--
-- The 34 tables this subset keeps (see README.md for how they were chosen) are
-- the account and work-tracking core: organizations, namespaces, projects,
-- users and the things attached to them.
--
-- What GitLab contributes to the set is *width*. `users` alone has more than
-- seventy columns, most of them defaulted, several of them encrypted
-- (`encrypted_otp_secret`, `encrypted_static_object_token`) with their IV and
-- salt in neighbouring columns; `namespaces` is self-referencing through
-- `parent_id`, which is the shape that makes a naive parent walk climb for ever.
-- Both are here.
--

SELECT lazyslice_torture.fill('public.organizations', 5);
SELECT lazyslice_torture.fill('public.namespaces', 300);
SELECT lazyslice_torture.fill('public.namespace_settings', 300);
SELECT lazyslice_torture.fill('public.users', 300);
SELECT lazyslice_torture.fill('public.user_details', 300);
SELECT lazyslice_torture.fill('public.user_preferences', 300);
SELECT lazyslice_torture.fill('public.emails', 300);
SELECT lazyslice_torture.fill('public.identities', 200);
SELECT lazyslice_torture.fill('public.personal_access_tokens', 200);
SELECT lazyslice_torture.fill('public.projects', 200);
SELECT lazyslice_torture.fill('public.project_settings', 200);
SELECT lazyslice_torture.fill('public.project_authorizations', 300);
SELECT lazyslice_torture.fill('public.members', 300);
-- milestones is CHECK (num_nonnulls(group_id, project_id) = 1): exactly one
-- owner, and both columns are nullable, so the generator would write neither.
SELECT lazyslice_torture.fill('public.milestones', 100,
    '{"project_id": "(SELECT p.id FROM public.projects p ORDER BY p.id OFFSET ((s.i - 1) % 200) LIMIT 1)"}');
-- issues carries validate_work_item_type_on_insert_or_update_issues, a BEFORE
-- INSERT trigger that raises 'Specified system defined work item type does not
-- exist' for any work_item_type_id above 9 that is not a custom type. Nothing
-- in the catalogue says so, which is what the override argument is for.
SELECT lazyslice_torture.fill('public.issues', 400, '{"work_item_type_id": "1 + (s.i % 9)"}');
SELECT lazyslice_torture.fill('public.issue_assignees', 300);
SELECT lazyslice_torture.fill('public.merge_requests', 200);
-- notes carries `CHECK (num_nonnulls(namespace_id, organization_id,
-- project_id) >= 1)` over three nullable columns — GitLab's sharding-key
-- idiom — so one of them has to be chosen for it.
SELECT lazyslice_torture.fill('public.notes', 400,
    '{"project_id": "(SELECT p.id FROM public.projects p ORDER BY p.id OFFSET ((s.i - 1) % 200) LIMIT 1)"}');
SELECT lazyslice_torture.fill('public.todos', 300);
-- events has the same sharding-key check and, unlike notes, no user foreign
-- key at all: author_id is a bare bigint. Both facts are the fixture's point.
SELECT lazyslice_torture.fill('public.events', 400,
    '{"project_id": "(SELECT p.id FROM public.projects p ORDER BY p.id OFFSET ((s.i - 1) % 200) LIMIT 1)",
      "author_id": "(SELECT p.id FROM public.users p ORDER BY p.id OFFSET ((s.i - 1) % 300) LIMIT 1)"}');
SELECT lazyslice_torture.fill('public.abuse_reports', 100);

-- The people.
UPDATE public.users u
SET name = g.given || ' ' || g.family,
    username = lower(g.given) || '.' || lower(g.family) || u.id,
    email = lower(g.given) || '.' || lower(g.family) || u.id || '@' || g.domain,
    encrypted_password = '$2a$10$' || md5('pw' || u.id) || substr(md5('pw2' || u.id), 1, 21),
    current_sign_in_ip = '10.' || (u.id % 250) || '.' || ((u.id / 250) % 250) || '.21',
    last_sign_in_ip = '10.' || (u.id % 250) || '.' || ((u.id / 250) % 250) || '.22',
    public_email = CASE WHEN u.id % 4 = 0
                        THEN lower(g.given) || '.' || lower(g.family) || u.id || '@' || g.domain
                        ELSE '' END,
    created_at = TIMESTAMP '2025-01-01 00:00:00' + (u.id || ' hours')::interval,
    updated_at = TIMESTAMP '2025-01-02 00:00:00' + (u.id || ' hours')::interval
FROM (
    SELECT id,
           -- Twenty by twenty rather than ten by ten: `namespaces` is unique on
           -- (name, parent_id, type) and every one of the 300 personal
           -- namespaces below is named after its user, so the 300 names have to
           -- be distinct. (id %% 20, id / 20 %% 20) is injective for id < 400.
           (ARRAY['Arjuna','Beata','Cleo','Damir','Elsa','Fikri','Gala','Hugh','Ilse','Jamaal',
                  'Kaya','Liesl','Mirek','Noora','Ozan','Piia','Quirin','Rune','Suvi','Tarek'])[1 + (id % 20)] AS given,
           (ARRAY['Ahuja','Balog','Chen','Dukic','Ekstrom','Fadel','Guerra','Hollis','Ivanic','Jaffar',
                  'Kovacic','Lindholm','Mwangi','Nagy','Ortega','Pajari','Quintero','Radev','Solberg','Tadesse'])[1 + ((id / 20) % 20)] AS family,
           (ARRAY['pinewick.test','saltmarsh.test','underhill.test'])[1 + (id % 3)] AS domain
    FROM public.users
) g
WHERE g.id = u.id;

-- A secondary address is a second copy of the same personal datum in a table
-- most people forget exists.
UPDATE public.emails e
SET email = 'alt.' || u.email,
    confirmed_at = CASE WHEN e.id % 3 = 0 THEN TIMESTAMP '2025-02-01 00:00:00' END
FROM public.users u
WHERE u.id = e.user_id;

UPDATE public.user_details d
SET job_title = (ARRAY['Platform engineer','Support lead','SRE','Technical writer'])[1 + (d.user_id % 4)],
    bio = 'Reachable at ' || u.email || ' during Europe/Lisbon hours.',
    linkedin = 'in/' || split_part(u.username, '.', 1) || '-' || d.user_id,
    phone = '+351 21 ' || lpad((400000 + d.user_id)::text, 6, '0'),
    email_otp_last_sent_to = u.email,
    location = (ARRAY['Porto','Tbilisi','Wellington','Kigali'])[1 + (d.user_id % 4)]
FROM public.users u
WHERE u.id = d.user_id;

-- A namespace's name is a person's name, for the 300 personal namespaces every
-- GitLab install has one of per user.
UPDATE public.namespaces n
SET name = u.name,
    path = u.username,
    type = 'User',
    owner_id = u.id,
    -- The self-reference: a fifth of these hang off another namespace, so the
    -- fixture has a parent chain and not only a flat list.
    parent_id = CASE WHEN r.k % 5 = 0 THEN p.id END
FROM (
    SELECT id, row_number() OVER (ORDER BY id) AS k FROM public.namespaces
) r
JOIN LATERAL (SELECT id, name, username, email FROM public.users ORDER BY id OFFSET ((r.k - 1) % 300) LIMIT 1) u ON true
JOIN LATERAL (SELECT id FROM public.namespaces ORDER BY id LIMIT 1) p ON true
WHERE r.id = n.id;

-- A personal access token's digest is a credential; its name is not.
UPDATE public.personal_access_tokens
SET name = 'ci-runner-' || id,
    token_digest = encode(sha256(('pat' || id)::bytea), 'hex');

-- Free text written by people, about people.
UPDATE public.issues i
SET title = 'Cannot reset password for ' || u.email,
    description = 'Reported by ' || u.name || ' <' || u.email || '>, phone +351 21 ' || lpad((100000 + r.k)::text, 6, '0') || '.'
FROM (
    SELECT id, row_number() OVER (ORDER BY id) AS k FROM public.issues
) r
JOIN LATERAL (SELECT name, email FROM public.users ORDER BY id OFFSET ((r.k - 1) % 300) LIMIT 1) u ON true
WHERE r.id = i.id;

UPDATE public.notes n
SET note = CASE WHEN r.k % 3 = 0
                THEN 'Reassigned, see the linked issue.'
                ELSE 'Confirmed with ' || u.email || ' on the phone.'
           END
FROM (
    SELECT id, row_number() OVER (ORDER BY id) AS k FROM public.notes
) r
JOIN LATERAL (SELECT email FROM public.users ORDER BY id OFFSET ((r.k - 1) % 300) LIMIT 1) u ON true
WHERE r.id = n.id;

UPDATE public.abuse_reports a
SET message = 'Account ' || u.username || ' (' || u.email || ') is posting spam.',
    reporter_id = u.id,
    user_id = u2.id
FROM (
    SELECT id, row_number() OVER (ORDER BY id) AS k FROM public.abuse_reports
) r
JOIN LATERAL (SELECT id, username, email FROM public.users ORDER BY id OFFSET ((r.k - 1) % 300) LIMIT 1) u ON true
JOIN LATERAL (SELECT id FROM public.users ORDER BY id OFFSET ((r.k * 7 - 1) % 300) LIMIT 1) u2 ON true
WHERE r.id = a.id;
