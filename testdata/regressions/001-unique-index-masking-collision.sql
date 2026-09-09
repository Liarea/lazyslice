-- root:   public.reg1_member
-- take:   20
-- expect: ok
-- found:  django, rails-activestorage, supabase-auth
-- why:    a masked column under a unique index collided at load (SQLSTATE 23505)
--
-- Django's `auth_group.name` is `varchar(150) NOT NULL UNIQUE`. The classifier
-- decides `person_name` on it, and the `person_name` masker draws from a name
-- list; twenty groups then masked to fewer than twenty distinct names and the
-- load died:
--
--     ✗ the rows of public.auth_group did not go in: SQLSTATE 23505;
--       the table is empty, not half loaded
--     lazyslice: duplicate key value violates unique constraint
--       "auth_group_name_key" (SQLSTATE 23505)
--
-- The same defect, on three different columns, in three of the ten schemas:
-- `active_storage_blobs.key` (rails, which failed one step later, when the
-- unique index was recreated over the loaded rows) and `refresh_tokens.token`
-- (supabase-auth).
--
-- ARCHITECTURE.md §5 already says what should have happened: "Unique indexes
-- ...: d_required = n² / 2ε at ε = 10⁻⁶, with n the planned row count of the
-- table. The plan picks, within the column's category, the registered generator
-- with the largest Domain() that fits the column ... When even the largest
-- generator has d < d_required, the column is refused by name at plan with exit
-- 12". `mask.Pick` implements exactly that and nothing called it:
-- `internal/plan/writeback.go` said so in a comment — "the unique-index domain
-- rule of ARCHITECTURE.md §5 ... is a separate plan-time check that has not
-- landed" — and `internal/transform` used the rule pack's default masker
-- whatever the column's constraints were. So a collision at load was the only
-- way anyone was ever going to find out.
--
-- What this file reproduces, in three columns rather than 370 tables:
--
--   reg1_member.full_name   text UNIQUE, classified person_name.
--                           The default person_name generator's domain is a
--                           name list; at 20 rows d_required is 2 × 10⁸, so
--                           there is no generator large enough and the run must
--                           refuse at plan — except that this column is *not*
--                           unique here, deliberately: see below.
--   reg1_member.email       text UNIQUE, classified email. §5's escalation has
--                           somewhere to go — the hash-derived-suffix generator
--                           has a domain above 2⁶⁴ — so the run must *succeed*
--                           and the target must hold 20 distinct addresses.
--   reg1_member.handle      varchar(12) UNIQUE, classified online_id.
--
-- `expect: ok` is the assertion that matters: the escalation happens, and the
-- run that used to die at 23505 now loads. A column that genuinely cannot be
-- escalated is the other half of the rule and it is refused at exit 12; the
-- torture schemas cover that path (odoo and discourse both refuse at plan) and
-- keeping it out of this file keeps this file about the collision.

CREATE TABLE public.reg1_member (
    id         integer PRIMARY KEY,
    full_name  text NOT NULL,
    email      text NOT NULL UNIQUE,
    handle     varchar(12) NOT NULL UNIQUE,
    note       text
);

CREATE TABLE public.reg1_note (
    id         integer PRIMARY KEY,
    member_id  integer NOT NULL REFERENCES public.reg1_member (id),
    body       text NOT NULL
);

INSERT INTO public.reg1_member (id, full_name, email, handle, note)
SELECT i,
       (ARRAY['Ada Achebe','Bilal Baumann','Coco Cabrera','Dilan Deniz','Eero Engberg'])[1 + (i % 5)],
       'person' || i || '@regression.test',
       'user' || lpad(i::text, 4, '0'),
       'Call +44 7700 9' || lpad((100000 + i)::text, 6, '0') || ' before Friday.'
FROM generate_series(1, 40) AS g(i);

INSERT INTO public.reg1_note (id, member_id, body)
SELECT i,
       1 + ((i - 1) % 40),
       'Reply to person' || (1 + ((i - 1) % 40)) || '@regression.test'
FROM generate_series(1, 80) AS g(i);
