-- root:   public.reg10_item
-- take:   20
-- expect: ok
-- found:  the 2026-09-09 review, finding 3 (probe schema s_fk_masker)
-- why:    the plan escalated a unique column's masker and not its foreign-key child's, so equal values masked differently and the key did not validate
-- unique-masked: public.reg10_token.token
-- equal-masked:  public.reg10_item.token = public.reg10_token.token
--
-- The one regression that did not come from testdata/torture/. Finding 3 of
-- docs/reviews/2026-09-09/REVIEW.md reduced it to two tables before anybody
-- here did, and this is that reduction checked in, with rows enough that the
-- unique-index rule has a row count to work with.
--
-- ARCHITECTURE.md section 4 propagates a *category* along a foreign key, so
-- both ends of a key agree about what they hold. Nothing propagated the
-- *masker*. Section 5's unique-index rule then rewrote one column's masker at a
-- time (internal/plan/unique.go): reg10_token.token is under a unique index, so
-- it escalated to credential_unique, and reg10_item.token is not, so it kept the
-- category default, the fixed literal $lazyslice$invalid. One value in the
-- source, two values in the target:
--
--     public.reg10_token: 20 rows
--     public.reg10_item: 20 rows
--     the foreign key reg10_item_token_fkey on public.reg10_item does not
--     validate: the slice is missing rows it references
--     lazyslice: insert or update on table "reg10_item" violates foreign key
--     constraint "reg10_item_token_fkey" (SQLSTATE 23503)
--
-- exit 8, after the extract had moved every row of both tables. The review's
-- own transcript is docs/reviews/2026-09-09/evidence/fk_masker.log.
--
-- T-0132's fix is in internal/plan/equality.go: the masker is chosen per
-- equality group -- the columns connected by any foreign key, transitively, that
-- classification has put in one category -- and it is the widest generator any
-- member needs, checked to fit every member. Here that is credential_unique,
-- which both text columns can hold, so both ends mask to the same token and the
-- key validates.
--
-- What the three header keys assert, and why one alone would not:
--
--   `expect` is ok -- the run reaches exit 0 rather than the loader's exit 8.
--     On its own this would also pass a run that copied both columns into the
--     target verbatim: equal inputs, equal outputs, a valid key, no masking.
--   `unique masked` -- reg10_token.token carries mask.CredentialUniquePrefix on
--     every value and no two rows alike. That is the half exit 0 cannot carry.
--   `equal masked` -- every value of reg10_item.token is a value of
--     reg10_token.token. The foreign key says so too, and this says it against
--     the loaded target rather than against an exit code, so the claim survives
--     a change that stops recreating the constraint.
--
-- reg10_item.token is deliberately not unique and holds repeats: the defect is
-- exactly that the non-unique end was left on the category default, and a
-- column that was also unique would have been escalated by the old rule for its
-- own reasons and hidden the bug.
--
-- reg10_item.contact carries the email addresses every `expect: ok` regression
-- needs: the harness's leak check refuses to pass a source holding no email
-- address and no phone number, because finding none in the target would then
-- prove nothing (README.md, "What each file asserts").

CREATE TABLE public.reg10_token (
    id    integer PRIMARY KEY,
    token text NOT NULL UNIQUE
);

CREATE TABLE public.reg10_item (
    id      integer PRIMARY KEY,
    token   text NOT NULL REFERENCES public.reg10_token (token),
    contact text NOT NULL
);

INSERT INTO public.reg10_token (id, token)
SELECT i, 'sk_live_' || md5('reg10-token-' || i::text)
FROM generate_series(1, 20) AS g(i);

INSERT INTO public.reg10_item (id, token, contact)
SELECT i,
       (SELECT t.token FROM public.reg10_token t WHERE t.id = 1 + ((i - 1) % 20)),
       'person' || i || '@regression.test'
FROM generate_series(1, 40) AS g(i);
