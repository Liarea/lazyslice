-- root:   public.reg031_members
-- take:   20
-- expect: ok
-- found:  the T-0239 fix-round review
-- why:    unknownColumnsBesideCertain swept a validated foreign-key child's
--         character column into free_text while its parent stayed unmasked,
--         a decision internal/plan cannot catch and internal/load turns into
--         a half-loaded target
-- not-masked: public.reg031_currencies.code, public.reg031_members.currency
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
-- The fix excludes a validated, non-virtual foreign key's character-family
-- columns -- both ends -- from unknownColumnsBesideCertain's reach: a rail
-- with no evidence of its own about a column should not be the thing that
-- puts the two ends of a join out of step, the same reasoning the rail
-- already applies to a unique index and to an integer or uuid key column
-- (markNeverMasked's own exemption for "a generated column, a surrogate key,
-- a FK column"). Neither column is personal data -- an ISO-style currency
-- code is the same shape the rail's own two-letter-code exclusion already
-- carves out for a two-character code -- so both staying unmasked is the
-- correct outcome, not a residual gap; a genuinely personal FK-linked column
-- is still reached by every other pass (a name hit, a value validator, FK
-- propagation from a masked parent).
--
-- reg031_members.email carries the addresses every `expect: ok` regression
-- needs for the leak check (README.md, "What each file asserts").

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
