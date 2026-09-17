-- root:   public.reg035_members
-- take:   20
-- expect: exit 12 plan.refused.unique_domain
-- found:  the round-5 red team, the classifier attacker's FK variant
-- why:    unknownColumnsBesideCertain used to exclude a validated foreign
--         key's character-family columns outright, both ends, and a
--         native-script name held as an FK child -- beside a certain email
--         column, with a parent that has no certain column of its own --
--         crossed into the target verbatim on both ends under exit 0
--
-- docs/reviews/2026-09-15-redteam/round5-still-leaking.json's classifier
-- attacker replayed T-0239's own native-script attack -- five real Amharic
-- names, in a column named "ስም" ("name", in the language the column's own
-- name is written in too), declared varchar(12) -- and made it a validated
-- foreign key child of a lookup table holding the same five names. The
-- T-0239 fix-round review's own fix, in place when this attack ran, excluded
-- a validated foreign key's character-family columns from
-- unknownColumnsBesideCertain's reach entirely, at either end
-- (testdata/regressions/031's previous header has the reasoning it was
-- written under). reg035_name_dim has no `certain` column of its own -- it is
-- a bare lookup table, one column -- so no other pass in internal/classify
-- ever reached either end: `keyChildren` only reconciles the integer/uuid
-- key case, and `propagateKeys` only ever propagates a masked parent
-- forward, never a masked child back. Both reg035_name_dim."ስም" and
-- reg035_members."ስም" crossed into the target verbatim, beside a real
-- `email` column in reg035_members that WAS masked, reporting "nothing
-- recognised in N samples, not proof the column is impersonal" for a column
-- that held five real people's names sixty-one times over
-- (THREAT_MODEL.md T1).
--
-- Fixed by fkPairs (internal/classify/classify.go, called from
-- unknownColumnsBesideCertain, T-0253): reg035_members."ስም" already
-- qualifies for the rail on its own (a character column with no name or
-- value signal, beside a `certain` email column in the same table), and its
-- only foreign-key partner, reg035_name_dim."ስም", qualifies the same way --
-- character family, no name or value signal of its own, above the declared-
-- length floor -- so both are raised together, under the same category.
--
-- **The run then refuses rather than loads, and that is the correct
-- outcome, not a residual.** reg035_name_dim is a five-row lookup table
-- under a unique index (its primary key), and `free_text`'s masker draws
-- from a fixed word list: for a twelve-character column its widest
-- generator offers 635 distinct values, nowhere near the 12,500,000
-- ARCHITECTURE.md §5's d_required asks of a five-row unique column at
-- one-in-a-million collision odds. `internal/plan`'s existing, unmodified
-- unique-index domain check refuses at exit 12, naming both
-- reg035_name_dim."ስም" and reg035_members."ስም" together -- the same
-- foreign-key-group message T-0132's mechanism already prints -- with
-- `--unmask` the escape for each, naming the pair exactly the way the
-- round-5 attack's own fix text asked for. Nothing is loaded, so nothing is
-- copied: the leak this file existed to catch is closed by the refusal, not
-- by a mask this schema's own row count could never satisfy anyway.
--
-- A schema whose lookup table holds more rows, or whose masked column is
-- wide enough for a larger word-list domain, is expected to mask rather than
-- refuse -- this file pins the shape that cannot, deliberately: a small
-- lookup table is common, and this is where the round-5 attack landed.
--
-- reg035_members.email carries the addresses every `expect: ok` regression
-- would need for the leak check; this file expects a refusal instead, so the
-- harness checks the exit code and event code only (README.md, "What each
-- file asserts").

CREATE TABLE public.reg035_name_dim (
    "ስም" varchar(12) PRIMARY KEY
);

INSERT INTO public.reg035_name_dim ("ስም") VALUES
    ('ኣበበ ኪዳነ'),
    ('ተስፋዬ ኪዳነ'),
    ('ገብረ ኣበበ'),
    ('ኪዳነ ተስፋዬ'),
    ('ኣበበ ገብረ');

CREATE TABLE public.reg035_members (
    id    integer PRIMARY KEY,
    email text NOT NULL,
    "ስም"  varchar(12) NOT NULL REFERENCES public.reg035_name_dim("ስም")
);

INSERT INTO public.reg035_members (id, email, "ስም")
SELECT i,
       'member' || lpad(i::text, 2, '0') || '@realcorp.example',
       (ARRAY['ኣበበ ኪዳነ', 'ተስፋዬ ኪዳነ', 'ገብረ ኣበበ', 'ኪዳነ ተስፋዬ', 'ኣበበ ገብረ'])[1 + ((i - 1) % 5)]
FROM generate_series(1, 20) AS g(i);
