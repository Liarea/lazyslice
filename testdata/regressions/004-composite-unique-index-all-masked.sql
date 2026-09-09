-- root:   public.reg4_person
-- take:   50
-- expect: ok
-- found:  django
-- why:    a composite unique index whose every key column is masked collided at load
-- unique-masked: public.reg4_grant.access_token, public.reg4_grant.refresh_token
--
-- The other side of
-- 003-composite-unique-index-is-not-a-unique-column.sql. Once
-- Decision.UniqueIndex meant "this column alone has to be distinct", a composite
-- unique index stopped raising it on any of its columns — which is right when
-- one of them is a surrogate key nobody masks, and wrong when *all* of them are
-- masked, because then the tuple is as collidable as its parts.
--
-- Django is where it showed up, one flag after 001 was fixed:
--
--     ✗ the rows of public.django_content_type did not go in: SQLSTATE 23505
--     lazyslice: duplicate key value violates unique constraint
--       "django_content_type_app_label_model_76bd3d3b_uniq" (SQLSTATE 23505)
--
-- `django_content_type` is `UNIQUE (app_label, model)`, two text columns, and
-- the classifier masks both.
--
-- The pair here is two `credential` columns rather than Django's two name-ish
-- ones, because `credential` had one registered masker and it was a fixed
-- literal with a domain of exactly 1: every row masked to `$lazyslice$invalid`
-- in both columns, so the second row collided, always, on every machine and at
-- every --take. Reproducing Django's own pair means betting on how many of 40
-- masked names happen to coincide, which is a flaky test rather than a
-- regression.
--
-- ARCHITECTURE.md §5 does not cover the composite case at all: its rule is
-- d_required = n²/2ε "with n the planned row count of the table", and it speaks
-- throughout of *the column*. What the fix does is therefore deliberately an
-- approximation and says so: a composite, non-partial unique index whose every
-- key column is masked raises Decision.UniqueIndex on each of them, so each is
-- held to d_required on its own. That is stricter than the truth — the tuple's
-- domain is the product of its columns' domains, not the smallest of them — and
-- it errs towards a plan refusal, which is the direction that fails safe. That
-- approximation is the accepted rule now, not a placeholder: docs/adr/011-
-- unique-index-domain-rule.md clause (a), accepted 2026-09-08, with §5 amended
-- to state it (T-0099, T-0107).
--
-- **`expect:` was that refusal until T-0113, and is now `ok`.** What changed is
-- not the classifier rule this file pins — raiseCompositeUnique still raises
-- both columns and each is still held to d_required on its own — but what the
-- plan can do about the raise: mask/gen_credential.go's `credential_unique`
-- (T-0098) gave CatCredential an alternate generator with a domain wide enough
-- for a unique column, so the widest-generator step of §5 now finds one and the
-- run loads instead of refusing at exit 12. The refusal was the fix working
-- with one generator; the load is the fix working with two.
--
-- `expect: ok` on its own would be a weaker assertion than the one it replaces,
-- because a run that copied both tokens verbatim also exits 0 — and copying
-- them is exactly what an operator's --unmask would have done here before the
-- escalation existed. So the header carries `unique-masked:` for both columns,
-- and the harness reads the target: every value prefixed `lazyslice-invalid-`,
-- and one distinct value per loaded row. A collision coming back fails on
-- distinctness, and a copy coming back fails on the prefix.
--
-- plan.refused.unique_domain is still a real refusal with a real message —
-- reachable by any masked unique column whose category has no wide generator —
-- and after this change nothing in testdata/regressions/ covers it. That is
-- recorded rather than papered over: it is a refusal to keep a regression for,
-- not a reason to keep asserting it from a fixture that no longer produces it.
-- Tracker T-0124 owes that reduction.

CREATE TABLE public.reg4_person (
    id    integer PRIMARY KEY,
    email text NOT NULL
);

CREATE TABLE public.reg4_grant (
    id            integer PRIMARY KEY,
    person_id     integer NOT NULL REFERENCES public.reg4_person (id),
    access_token  text NOT NULL,
    refresh_token text NOT NULL,
    CONSTRAINT reg4_grant_tokens_uniq UNIQUE (access_token, refresh_token)
);

INSERT INTO public.reg4_person (id, email)
SELECT i, 'person' || i || '@regression.test' FROM generate_series(1, 200) AS g(i);

INSERT INTO public.reg4_grant (id, person_id, access_token, refresh_token)
SELECT i, i, md5('access' || i) || md5('access2' || i), md5('refresh' || i) || md5('refresh2' || i)
FROM generate_series(1, 200) AS g(i);
