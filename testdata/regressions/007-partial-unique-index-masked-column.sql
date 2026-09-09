-- root:   public.reg7_account
-- take:   40
-- expect: exit 12 plan.refused.unique_domain
-- found:  supabase-auth
-- why:    a masked column under a *partial* unique index collided when the index was recreated
--
-- Supabase's auth schema carries
--
--     CREATE UNIQUE INDEX confirmation_token_idx ON auth.users (confirmation_token)
--       WHERE confirmation_token::text !~ '^[0-9 ]*$';
--
-- — a partial unique index, which is how GoTrue lets many rows hold the empty
-- token and still keeps real tokens unique. `confirmation_token` classifies as
-- `credential`, whose only registered masker emits the fixed literal
-- `$lazyslice$invalid`; that literal does not match `^[0-9 ]*$`, so every masked
-- row is inside the index's predicate and every masked row is identical. The
-- rows went in and the index would not build over them:
--
--     lazyslice: could not create unique index "confirmation_token_idx"
--       (SQLSTATE 23505)
--
-- ARCHITECTURE.md §5's rule ("Unique indexes (including expression indexes such
-- as lower(email))") does not say what to do about a partial one, and the first
-- reading of `Decision.UniqueIndex` left partial indexes out — internal/transform
-- had always left them out, and a partial index genuinely does not say "every row
-- of this table holds a distinct value here", which is what d_required = n²/2ε is
-- computed against.
--
-- It does say something weaker that still has to be respected: every row *the
-- predicate admits* holds a distinct value. So a single-column partial unique
-- index now raises Decision.UniqueIndex like a total one, and d_required is
-- computed over the whole table's row count — which is an over-estimate,
-- because the predicate admits at most that many rows and usually fewer. Over-
-- estimating refuses a plan that might have loaded; under-estimating is the
-- 23505 above, one statement after every row has moved. The plan refusal prints
-- three escapes and the collision prints none, so the over-estimate is the
-- direction taken, and T-0099 carries the exact rule along with the rest of §5's
-- unwritten index cases.
--
-- `expect:` is therefore the refusal. `reg7_account.label` is the control: it is
-- under a partial unique index too and is *not* masked, so nothing about it
-- changes.

CREATE TABLE public.reg7_account (
    id                 integer PRIMARY KEY,
    label              text,
    confirmation_token text
);

CREATE UNIQUE INDEX reg7_confirmation_token_idx
    ON public.reg7_account (confirmation_token)
    WHERE confirmation_token !~ '^[0-9 ]*$';

CREATE UNIQUE INDEX reg7_label_idx
    ON public.reg7_account (label)
    WHERE label IS NOT NULL;

CREATE TABLE public.reg7_session (
    id         integer PRIMARY KEY,
    account_id integer NOT NULL REFERENCES public.reg7_account (id),
    agent      text NOT NULL
);

INSERT INTO public.reg7_account (id, label, confirmation_token)
SELECT i, 'L' || i, md5('confirm' || i) || md5('confirm2' || i)
FROM generate_series(1, 120) AS g(i);

INSERT INTO public.reg7_session (id, account_id, agent)
SELECT i, i, 'Mozilla/5.0 session ' || i FROM generate_series(1, 120) AS g(i);
