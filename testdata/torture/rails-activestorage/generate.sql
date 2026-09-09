--
-- rails-activestorage/generate.sql
--
-- The smallest schema in the set, and the one carrying the shape phase 5 owes
-- an answer to: ActiveStorage's `active_storage_attachments` points at whatever
-- it is attached to through the pair (record_type, record_id), which is a
-- reference with no foreign key behind it. ARCHITECTURE.md §3.2's polymorphic
-- inference is what has to notice that, so the fixture has to actually contain
-- it: the attachments here point at real `posts` rows, and half of them at rows
-- a `--take` of the blobs will not have selected.
--
-- `active_storage_attachments` is filled by hand rather than by
-- lazyslice_torture.fill for exactly that reason — the generator only knows how
-- to satisfy a declared foreign key, and record_id is not one. Writing it by
-- hand is also what lets the pair be consistent: record_type 'Post' with a
-- record_id that is a post.
--

SELECT lazyslice_torture.fill('public.posts', 200);
SELECT lazyslice_torture.fill('public.comments', 400);
SELECT lazyslice_torture.fill('public.active_storage_blobs', 300);
SELECT lazyslice_torture.fill('public.active_storage_variant_records', 300);

-- The polymorphic edge, written out. Every blob is attached to exactly one
-- post; posts.id is dense from 1, so the modulo lands on a real row.
INSERT INTO public.active_storage_attachments (name, record_type, record_id, blob_id, created_at)
SELECT 'cover',
       'Post',
       (SELECT p.id FROM public.posts p ORDER BY p.id OFFSET (b.id % 200) LIMIT 1),
       b.id,
       TIMESTAMP '2025-03-01 00:00:00' + (b.id || ' minutes')::interval
FROM public.active_storage_blobs b;

-- Personal data. `author_email` and `author_name` say what they are; `body`,
-- `title` and `filename` do not, and all three carry the same address in some
-- rows. docs/TORTURE.md labels each of them.
UPDATE public.posts p
SET title = g.given || ' ' || g.family || ' — trip report ' || p.id,
    body = 'Booked by ' || g.given || ' ' || g.family || ' (' || g.mail || '). '
           || 'Call ' || g.phone || ' if the ferry is cancelled.',
    author_email = g.mail,
    created_at = TIMESTAMP '2025-01-01 00:00:00' + (p.id || ' hours')::interval,
    updated_at = TIMESTAMP '2025-01-02 00:00:00' + (p.id || ' hours')::interval
FROM (
    SELECT id,
           (ARRAY['Aoife','Bilal','Ceren','Dmytro','Esi','Fabio','Gitika','Hanne','Iker','Junko'])[1 + (id % 10)] AS given,
           (ARRAY['Byrne','Chaudhry','Demir','Egorov','Frimpong','Gallone','Halvorsen','Iqbal','Jokinen','Kaur'])[1 + ((id / 10) % 10)] AS family,
           lower((ARRAY['aoife','bilal','ceren','dmytro','esi','fabio','gitika','hanne','iker','junko'])[1 + (id % 10)])
             || '.' || lower((ARRAY['byrne','chaudhry','demir','egorov','frimpong','gallone','halvorsen','iqbal','jokinen','kaur'])[1 + ((id / 10) % 10)])
             || id || '@' || (ARRAY['ferryline.test','coastwise.test','pierhead.test'])[1 + (id % 3)] AS mail,
           '+44 7700 9' || lpad((100000 + id)::text, 6, '0') AS phone
    FROM public.posts
) g
WHERE g.id = p.id;

UPDATE public.comments c
SET author_name = p.title,
    author_email = 'reply+' || c.id || '@' || (ARRAY['ferryline.test','coastwise.test','pierhead.test'])[1 + (c.id % 3)],
    body = 'Reply to ' || p.author_email || ' about post ' || p.id,
    created_at = TIMESTAMP '2025-01-05 00:00:00' + (c.id || ' minutes')::interval,
    updated_at = TIMESTAMP '2025-01-06 00:00:00' + (c.id || ' minutes')::interval
FROM public.posts p
WHERE p.id = c.post_id;

-- A filename is free text that people put names in, and ActiveStorage stores it
-- verbatim. `metadata` is the same story one level down.
UPDATE public.active_storage_blobs b
SET filename = lower(regexp_replace(split_part(p.title, ' — ', 1), '\s+', '-', 'g')) || '-passport-' || b.id || '.pdf',
    content_type = 'application/pdf',
    service_name = 'local',
    -- What ActiveStorage actually writes. It carried the uploader's address in
    -- an earlier draft of this fixture, which found T-0102 — a text column
    -- holding a JSON document is invisible to §4's JSON rule and was copied
    -- verbatim — but ActiveStorage does not put an address there, and a fixture
    -- that asserts a leak nobody has decided to fix is a fixture that stays red.
    metadata = '{"identified":true,"analyzed":true,"width":1280,"height":960}',
    checksum = md5('blob' || b.id),
    created_at = TIMESTAMP '2025-03-01 00:00:00' + (b.id || ' minutes')::interval
FROM public.active_storage_attachments a
JOIN public.posts p ON p.id = a.record_id AND a.record_type = 'Post'
WHERE a.blob_id = b.id;
