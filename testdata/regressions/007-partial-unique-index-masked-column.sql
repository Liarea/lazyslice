-- root:   public.reg7_account
-- take:   40
-- expect: ok
-- found:  supabase-auth
-- why:    a masked column under a *partial* unique index collided when the index was recreated
-- unique-masked: public.reg7_account.confirmation_token
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
-- direction taken. It is the accepted rule now, not a placeholder:
-- docs/adr/011-unique-index-domain-rule.md clause (b), accepted 2026-09-08,
-- with §5 amended to state it (T-0099, T-0107).
--
-- **`expect:` was that refusal until T-0113, and is now `ok`.** The classifier
-- rule this file pins is unchanged — a single-column partial unique index still
-- raises Decision.UniqueIndex and d_required is still computed over the whole
-- table — and what changed is what the plan can do about the raise:
-- mask/gen_credential.go's `credential_unique` (T-0098) gave CatCredential an
-- alternate generator wide enough for a unique column, so §5's widest-generator
-- step finds one and the run loads. The over-estimate is therefore still an
-- over-estimate and still errs the safe way; it simply no longer costs a
-- refusal for this category.
--
-- `expect: ok` alone would pass a run that copied the token verbatim, which is
-- the leak the column exists to prevent, so the header carries `unique-masked:`
-- and the harness reads the target: every confirmation_token prefixed
-- `lazyslice-invalid-`, and one distinct value per loaded row. That also
-- re-states the original defect in the target rather than in the exit code — a
-- repeat here is the index that would not build.
--
-- `reg7_account.label` is the control: it is under a partial unique index too
-- and is *not* masked, so nothing about it changes.
--
-- `reg7_account.email` came in with the header change and is not part of the
-- defect. Every regression that exits 0 is also put through the grep half of
-- invariant I2, which fails when the *source* holds no email address and no
-- phone number — finding none in the target would prove nothing about a fixture
-- that never had one. This file had none, because until T-0113 it never reached
-- that assertion: it was expected to refuse before anything was loaded.

CREATE TABLE public.reg7_account (
    id                 integer PRIMARY KEY,
    label              text,
    email              text NOT NULL,
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

INSERT INTO public.reg7_account (id, label, email, confirmation_token)
SELECT i, 'L' || i, 'person' || i || '@regression.test', md5('confirm' || i) || md5('confirm2' || i)
FROM generate_series(1, 120) AS g(i);

INSERT INTO public.reg7_session (id, account_id, agent)
SELECT i, i, 'Mozilla/5.0 session ' || i FROM generate_series(1, 120) AS g(i);
