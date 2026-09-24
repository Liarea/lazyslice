-- root:   public.reg031_members
-- take:   20
-- expect: ok
-- found:  the T-0239 fix-round review, corrected by the round-5 red team (T-0253)
-- why:    unknownColumnsBesideCertain used to exclude a validated foreign
--         key's character-family columns outright, both ends, and left real
--         personal data unmasked on both sides wherever the shape carried
--         names rather than currency codes (round-5 red team, T-0253,
--         testdata/regressions/035); this schema is the control that pins
--         the correct outcome for the shape the exclusion was written for
-- not-masked: public.reg031_members.currency, public.reg031_currencies.code
-- equal-masked: public.reg031_members.currency = public.reg031_currencies.code
--
-- T-0239 lowered unknownColumnsBesideCertain's declared-length floor from
-- sixteen characters to two, so that a short character column with no name or
-- value signal, sitting beside a `certain` personal column in the same table,
-- is masked as free_text rather than skipped (THREAT_MODEL.md's A2b
-- amendment). The fix-round review found the shape that floor newly reaches
-- and should not: reg031_members.currency is exactly such a column -- no name
-- rule, no value hit, three characters wide -- beside `email`, a `certain`
-- column in the same table, and the rail masked it. But `currency` is also a
-- validated foreign key to reg031_currencies.code, a primary key holding
-- ordinary ISO currency codes, and nothing in this package propagates a
-- child's decision back to its parent (`propagateKeys` in
-- internal/classify/classify.go only ever runs parent to child). The result
-- was reg031_currencies.code copied verbatim while reg031_members.currency
-- was masked to free_text: the same values on both ends of a foreign key,
-- masked on one side and not the other. internal/plan's equality-group and
-- write-back checks both judge type, not cross-table agreement, so neither
-- catches it; internal/load adds the foreign key NOT VALID and VALIDATEs
-- after every row has been committed, so the run would die at VALIDATE with
-- the target already half loaded (plan/unique.go's own header names this
-- exact failure mode, tracker T-0132).
--
-- The fix-round review's own fix excluded a validated, non-virtual foreign
-- key's character-family columns -- both ends -- from
-- unknownColumnsBesideCertain's reach entirely, on the argument that neither
-- column here is personal data and that a genuinely personal FK-linked
-- column is still reached by every other pass. **That argument does not hold
-- for every schema this rail can see, only for this one.** The round-5 red
-- team's FK variant put the same native-script personal names T-0239's own
-- attack used behind a validated foreign key, with a parent that holds no
-- `certain` column of its own for any other pass to key on, and the blanket
-- exclusion copied real personal data verbatim on both ends under exit 0
-- (testdata/regressions/035-native-script-fk-child-beside-a-certain-column.sql).
--
-- The fix now is fkPairs (internal/classify/classify.go, called from
-- unknownColumnsBesideCertain, T-0253): a column this rail would otherwise
-- raise alone, at either end of a validated foreign key, is raised together
-- with every column connected to it across such an edge, under the same
-- category, so the join stays in agreement instead of disagreeing. **For
-- this schema that means the plan refuses rather than loads**: reg031_currencies
-- is a four-row lookup table under a unique index (its primary key), and
-- `free_text`'s masker draws from a fixed word list rather than an unbounded
-- alphabet -- three-letter words are scarce enough that its widest generator
-- offers only 3 distinct values here, nowhere near the ~8,000,000
-- ARCHITECTURE.md §5's d_required asks of a four-row unique column at
-- one-in-a-million collision odds. `internal/plan`'s existing, unmodified
-- unique-index domain check (`checkUniqueDomain`) refuses at exit 12 naming
-- both reg031_currencies.code and reg031_members.currency together -- the
-- same "joined by foreign keys and mask alike, so this refusal covers all of
-- them" message the fix-round review's own T-0132 mechanism already prints --
-- with `--unmask` the escape for each. That refusal is the correct answer
-- ARCHITECTURE.md §5 already specifies for a unique column a category's
-- generator cannot fill; it is not a new check, and it is not the
-- half-loaded-target failure mode this file used to pin, because nothing is
-- loaded at all. This regression exists to keep it that way: masking
-- currency codes was never the point, and a future change that made this
-- schema load with mismatched values would be the T-0132 defect all over
-- again.
--
-- **T-0311 (2026-09-24) moved this file from that refusal to `expect: ok`,
-- and the outcome is the one the paragraph above says was always the point.**
-- reg031_members.currency is twenty samples of four ISO codes, each seen five
-- times: an enumeration by internal/classify's T-0311 rule (at least ten
-- samples, at most twenty distinct values, each seen at least twice, every
-- value an ASCII token carrying no name). So unknownColumnsBesideCertain
-- spares it -- once fkPairs has agreed its partner could have been raised with
-- it, so a partner carrying a decision of its own still refuses (037) -- and
-- neither end is raised: both are copied, the same values on both sides of
-- the foreign key, which is the agreement T-0132 asks for, reached without
-- masking anything. `not-masked:` pins both ends copied, and `equal-masked:`
-- -- which only asserts that every child value is a parent value -- pins the
-- join, so a change that masked one end alone would still fail here.
-- `035` and `036`, the same shape carrying native-script names, still refuse
-- at exit 12: a non-ASCII value is never an enumeration member.
--
-- reg031_members.email carries the addresses the leak check every `expect:
-- ok` regression gets is run over (README.md, "What each file asserts").

CREATE TABLE public.reg031_currencies (
    code varchar(3) PRIMARY KEY,
    name text NOT NULL
);

INSERT INTO public.reg031_currencies (code, name) VALUES
    ('USD', 'US Dollar'),
    ('EUR', 'Euro'),
    ('GBP', 'Pound Sterling'),
    ('JPY', 'Japanese Yen');

CREATE TABLE public.reg031_members (
    id       integer PRIMARY KEY,
    email    text NOT NULL,
    currency varchar(3) NOT NULL REFERENCES public.reg031_currencies(code)
);

INSERT INTO public.reg031_members (id, email, currency) VALUES
    (1,  'member01@realcorp.example', 'USD'),
    (2,  'member02@realcorp.example', 'EUR'),
    (3,  'member03@realcorp.example', 'GBP'),
    (4,  'member04@realcorp.example', 'JPY'),
    (5,  'member05@realcorp.example', 'USD'),
    (6,  'member06@realcorp.example', 'EUR'),
    (7,  'member07@realcorp.example', 'GBP'),
    (8,  'member08@realcorp.example', 'JPY'),
    (9,  'member09@realcorp.example', 'USD'),
    (10, 'member10@realcorp.example', 'EUR'),
    (11, 'member11@realcorp.example', 'GBP'),
    (12, 'member12@realcorp.example', 'JPY'),
    (13, 'member13@realcorp.example', 'USD'),
    (14, 'member14@realcorp.example', 'EUR'),
    (15, 'member15@realcorp.example', 'GBP'),
    (16, 'member16@realcorp.example', 'JPY'),
    (17, 'member17@realcorp.example', 'USD'),
    (18, 'member18@realcorp.example', 'EUR'),
    (19, 'member19@realcorp.example', 'GBP'),
    (20, 'member20@realcorp.example', 'JPY');
