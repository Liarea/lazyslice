-- root:   public.reg3_blob
-- take:   50
-- expect: ok
-- found:  rails-activestorage, supabase-auth, calcom, gitlab, mastodon
-- why:    every column of a *composite* unique index was marked Decision.UniqueIndex
--
-- ARCHITECTURE.md §5's unique-index rule is about a column that carries the
-- uniqueness on its own: "d_required = n² / 2ε ... the plan picks ... the
-- registered generator with the largest Domain() that fits the column". A
-- column that is one of four in `CREATE UNIQUE INDEX ... ON t (record_type,
-- record_id, name, blob_id)` is not that column — the other three are what make
-- the row distinct — and §5 says nothing about the composite case at all.
--
-- `internal/classify`'s indexKeys set `unique[c] = true` for **every** column of
-- **every** unique index, composite, partial and expression alike. Nothing read
-- it until the plan-time domain rule landed
-- (001-unique-index-masking-collision.sql); the moment it did, five of the ten
-- schemas in testdata/torture/ refused at plan on a column that has no
-- uniqueness requirement of its own:
--
--     ✗ public.active_storage_attachments.name is under a unique index: the
--       widest masker for person_name emits 20445 distinct values and 100 row(s)
--       of public.active_storage_attachments need 5000000000 ...
--
-- `active_storage_attachments.name` is the literal string 'cover' or 'avatar'.
-- It is in `index_active_storage_attachments_uniqueness` with three other
-- columns, none of which is masked, and it could not collide if it tried.
--
-- The fix is to make Decision.UniqueIndex mean what §5 means: a single-column
-- primary key, a single-column non-partial unique index, or a single-column
-- *expression* unique index (§5 names `lower(email)` as exactly the case that
-- drives the generator choice). That is also the predicate
-- internal/transform's uniqueColumn already used, which
-- internal/transform/CLAUDE.md says the two have to agree on.
--
-- What this file reproduces: `reg3_attachment.name` is one column of a
-- four-column unique index and is decided person_name, so before the fix the
-- run refused at exit 12; `reg3_blob.token` is alone under a unique index and
-- is decided credential, so it must still refuse — which is why this fixture
-- has no such column and 002/001 carry that half.
--
-- The composite case that §5 genuinely does not cover — every column of a
-- composite unique index masked, so the tuple can collide after masking — is
-- not decided here and is not decided anywhere: it is filed as T-0099.

CREATE TABLE public.reg3_blob (
    id        integer PRIMARY KEY,
    filename  text NOT NULL,
    -- The regression suite asserts that no address of the source's survives into
    -- the target, and refuses a source that has none to look for: a fixture with
    -- no personal data proves nothing about masking.
    uploader_email text NOT NULL,
    byte_size bigint NOT NULL
);

CREATE TABLE public.reg3_attachment (
    id          integer PRIMARY KEY,
    name        text NOT NULL,
    record_type varchar(40) NOT NULL,
    record_id   bigint NOT NULL,
    blob_id     integer NOT NULL REFERENCES public.reg3_blob (id)
);

-- The four-column unique index. `name` is in it and is not what makes a row
-- distinct.
CREATE UNIQUE INDEX reg3_attachment_uniqueness
    ON public.reg3_attachment (record_type, record_id, name, blob_id);

-- A two-column primary key, for the other half of the same mistake: a column of
-- a composite primary key is not a unique column either.
CREATE TABLE public.reg3_membership (
    blob_id   integer NOT NULL REFERENCES public.reg3_blob (id),
    full_name text NOT NULL,
    role      text NOT NULL,
    PRIMARY KEY (blob_id, full_name)
);

INSERT INTO public.reg3_blob (id, filename, uploader_email, byte_size)
SELECT i, 'passport-scan-' || i || '.pdf', 'uploader' || i || '@regression.test', 1000 + i
FROM generate_series(1, 200) AS g(i);

INSERT INTO public.reg3_attachment (id, name, record_type, record_id, blob_id)
SELECT i,
       (ARRAY['cover','avatar','attachment'])[1 + (i % 3)],
       'Post',
       i,
       i
FROM generate_series(1, 200) AS g(i);

INSERT INTO public.reg3_membership (blob_id, full_name, role)
SELECT i,
       (ARRAY['Ana Aluko','Bram Brandt','Cai Calvo','Dilnoza Duman','Emil Ekman'])[1 + (i % 5)] || ' ' || i,
       'member'
FROM generate_series(1, 200) AS g(i);
