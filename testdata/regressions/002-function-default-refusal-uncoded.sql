-- root:   public.reg2_account
-- take:   10
-- expect: exit 13 target.schema.not_recreatable.function
-- found:  mastodon, gitlab
-- why:    the exit-13 refusal arrived as exit 1, run.refused.internal, "run with --debug"
--
-- Mastodon's primary keys default to `timestamp_id('accounts')`, a plpgsql
-- function the application installs (lib/mastodon/snowflake.rb); GitLab's
-- `organizations.uuid` defaults to `gen_random_uuid_v7()`. ARCHITECTURE.md
-- §11.1 says v1 does not recreate functions, and that a recreated object which
-- *depends* on one is exit 13, `target.schema.not_recreatable.function`, naming
-- the table, the column and the dependency. `internal/load/ddl.Recreatable`
-- raises exactly that refusal, with exactly that code and exit 13 inside it.
--
-- The operator saw this:
--
--     ✗ lazyslice failed for a reason it has no code for; run with --debug
--     lazyslice: run.refused.internal: ddl: target.schema.not_recreatable.function:
--       public.accounts.id depends on timestamp_id, which lazyslice does not recreate
--
-- Exit 1. The correct code is printed *inside* the message of the wrong one,
-- which is the worst of both: a CI job branching on the exit code sees "unknown
-- internal failure" for a refusal the tool understands completely, and the line
-- tells the reader to re-run with --debug for a stack trace that would have told
-- them nothing they were not already being shown.
--
-- The cause was one missing case. `core.asStop` converts each stage's own
-- refusal into the `Stop` cmd/lazyslice reads — plan's, extract's, transform's,
-- load's and verify's — and `*ddl.Refusal` is a sixth type, returned by
-- `load.Load` unwrapped, that nothing converted; so it fell through to the
-- final `wrap(CodeInternal, exitInternal, ...)`.
--
-- The refusal itself is right and stays. There is deliberately no flag that
-- drops the default and carries on (§11.1: "the application's first INSERT is
-- the point of the tool"), so `expect:` here is the refusal, not success: what
-- this file guards is that it arrives as exit 13 under its own code.
--
-- The remaining §11.1 gap is separate and is not this file's: the refusal is
-- specified to happen *at plan*, before the snapshot is used for keys, and it
-- still happens inside `load.Load`. internal/load/CLAUDE.md has carried that as
-- owed since the loader landed; T-0097 now carries the evidence that two of ten
-- real schemas reach it.

CREATE FUNCTION public.reg2_snowflake_id() RETURNS bigint
    LANGUAGE plpgsql
    AS $$
    BEGIN
        RETURN (((date_part('epoch', now()) * 1000))::bigint << 16) | (nextval('public.reg2_account_seq') & 65535);
    END
    $$;

CREATE SEQUENCE public.reg2_account_seq;

CREATE TABLE public.reg2_account (
    id       bigint DEFAULT public.reg2_snowflake_id() NOT NULL PRIMARY KEY,
    handle   text NOT NULL,
    email    text NOT NULL
);

CREATE TABLE public.reg2_post (
    id         integer PRIMARY KEY,
    account_id bigint NOT NULL REFERENCES public.reg2_account (id),
    body       text NOT NULL
);

INSERT INTO public.reg2_account (handle, email)
SELECT 'holder' || i, 'holder' || i || '@regression.test'
FROM generate_series(1, 30) AS g(i);

INSERT INTO public.reg2_post (id, account_id, body)
SELECT i, a.id, 'Written by ' || a.email
FROM generate_series(1, 60) AS g(i)
JOIN LATERAL (SELECT id, email FROM public.reg2_account ORDER BY id OFFSET ((i - 1) % 30) LIMIT 1) a ON true;
