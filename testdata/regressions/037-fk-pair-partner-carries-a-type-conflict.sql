-- root:   public.reg037_members
-- take:   20
-- expect: exit 12 plan.refused.fk_pair
-- found:  tracker T-0257 (closed alongside T-0253)
-- why:    a validated foreign key's parent already carries a decision
--         ARCHITECTURE.md §4 forbids overriding -- a name hit ("dob") its
--         citext type does not accept, recorded as a type conflict at low
--         -- so fkPairs refuses to raise either end rather than mask the
--         child alone and leave the parent's identical values copied
--
-- reg037_profiles.dob is citext, so person_date's name pattern matches
-- "dob" but rules.yml's accepts list for person_date is
-- date/timestamp/text/varchar/bpchar and not citext -- and the column holds
-- no date-shaped values to fall back on, so internal/classify/classify.go's
-- decide() records the conflict at `low` (typeConflict set) rather than a
-- date. reg037_members.linkval, a validated foreign key child of
-- reg037_profiles.dob, has no name or value signal of its own -- "linkval"
-- matches no rule pack pattern, and its values are dob's own placeholder
-- text -- so unknownColumnsBesideCertain would otherwise raise it alone,
-- beside reg037_members.email (a certain personal column in the same
-- table), as free_text: the same leak T-0253 exists to close, since dob's
-- identical values would stay copied on the other side of the join.
--
-- fkPairs (internal/classify/classify.go) asks whether linkval's direct
-- partner, dob, can be raised the same way, and refuses: dob already
-- carries a decision ARCHITECTURE.md §4 never lets a raising pass move
-- (fkPartnerRaisable). Neither end is raised, and both now carry
-- Decision.Refused naming the other (internal/pipeline/classify.go).
-- internal/plan/fkpair.go's checkFKPairRefusal reads that signal and
-- refuses the run at exit 12, naming both reg037_members.linkval and
-- reg037_profiles.dob, with --unmask the escape for each -- in place of the
-- pre-T-0253 leak, where both columns loaded copied verbatim under exit 0.
--
-- reg037_members.email carries the addresses every `expect: ok` regression
-- would need for the leak check; this file expects a refusal instead, so
-- the harness checks the exit code and event code only (README.md, "What
-- each file asserts").

CREATE EXTENSION IF NOT EXISTS citext;

CREATE TABLE public.reg037_profiles (
    dob citext PRIMARY KEY
);

INSERT INTO public.reg037_profiles (dob) VALUES
    ('zzz-not-a-date-1'),
    ('zzz-not-a-date-2'),
    ('zzz-not-a-date-3'),
    ('zzz-not-a-date-4');

CREATE TABLE public.reg037_members (
    id      integer PRIMARY KEY,
    email   text NOT NULL,
    linkval citext NOT NULL REFERENCES public.reg037_profiles(dob)
);

INSERT INTO public.reg037_members (id, email, linkval)
SELECT i,
       'member' || lpad(i::text, 2, '0') || '@realcorp.example',
       (ARRAY['zzz-not-a-date-1', 'zzz-not-a-date-2', 'zzz-not-a-date-3', 'zzz-not-a-date-4'])[1 + ((i - 1) % 4)]
FROM generate_series(1, 20) AS g(i);
