-- root:   public.reg4_person
-- take:   50
-- expect: exit 12 plan.refused.unique_domain
-- found:  django
-- why:    a composite unique index whose every key column is masked collided at load
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
-- ones, because `credential`'s only registered masker is a fixed literal with a
-- domain of exactly 1: every row masks to `$lazyslice$invalid` in both columns,
-- so the second row collides, always, on every machine and at every --take.
-- Reproducing Django's own pair means betting on how many of 40 masked names
-- happen to coincide, which is a flaky test rather than a regression.
--
-- ARCHITECTURE.md §5 does not cover the composite case at all: its rule is
-- d_required = n²/2ε "with n the planned row count of the table", and it speaks
-- throughout of *the column*. What the fix does is therefore deliberately an
-- approximation and says so: a composite, non-partial unique index whose every
-- key column is masked raises Decision.UniqueIndex on each of them, so each is
-- held to d_required on its own. That is stricter than the truth — the tuple's
-- domain is the product of its columns' domains, not the smallest of them — and
-- it errs towards a plan refusal, which is the direction that fails safe. The
-- exact composite rule is T-0099.
--
-- So `expect:` here is the refusal and not success: a run that loads this
-- fixture is the collision coming back.

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
