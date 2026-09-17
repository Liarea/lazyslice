-- root:   public.reg033_payroll
-- take:   10
-- expect: exit 9 verify.refused.second_net
-- found:  the 2026-09-15 red team round 4, the A9b dense bigint replay
--         (docs/reviews/2026-09-15-redteam/round4-still-leaking.json)
-- why:    a dense, zero-padded national identifier stored as bigint, beside
--         an `msisdn` column masked as `phone` on its name alone -- and
--         crossed verbatim under exit 0, because internal/verify's
--         corroboration gate counted only a `likely`-or-above neighbour and
--         `msisdn`'s own name-matched, value-unconfirmed decision landed
--         one confidence line below that.

-- What happened, and why this file, unlike 020/022/023, needs the
-- dense-sequence exemption itself to be at stake. `national_no` is dense --
-- ten consecutive values, span ten across ten rows -- which
-- digitRange.dense() (internal/verify/secondnet.go) exempts on its own: none
-- of 020, 022 or 023 tests a corroborated column that is ALSO dense, because
-- each of those needed a sparse or non-dense shape to survive
-- internal/classify's own key/date/sequence exclusions in the first place
-- (021 is the dense-and-uncorroborated control the other direction). Before
-- this task's fix, `val.sequenceExempt && dense` was checked before
-- `val.requiresCorroboration`, so a dense column skipped the entry outright
-- whatever corroborated() would have said -- density outranked
-- corroboration rather than yielding to it.
--
-- `msisdn` is one of rules.yml's own phone abbreviations, so
-- internal/classify's decide() takes it on the name alone; with no
-- --phone-region configured its plain national-format values never validate
-- under textsig.ValidPhone's international-only reading, so decide's own
-- name-match-with-nothing-from-the-values branch records ConfPossible, not
-- ConfLikely. `national_no` matches no rules.yml pattern ("national_no" is
-- neither `national_?ids?` nor `account_?numbers?` -- it needs "number(s)",
-- not "no", the same distinction testdata/regressions/023's own header
-- draws for `account_no`) and internal/classify's own national_id entry
-- never recovers a zero-padded digit run either, so it reaches decide() at
-- `none` and internal/transform copies it. Decision.TableHasLikelyPersonalColumn
-- never sees `msisdn` (ConfLikely floor, ConfPossible reached), so
-- corroborated() answered false and, even once it answers true,
-- `national_no`'s own density used to exempt the entry before
-- corroboration was ever asked.
--
-- The fix is Decision.TableHasMaskedPersonalColumn (a masked,
-- person-identifying neighbour at ConfPossible or above corroborates too)
-- and the dense-sequence exemption no longer outranking corroboration once
-- it exists to ask (internal/verify/CLAUDE.md's own T-0240 section has the
-- full account). Dropping either half leaves this file at exit 0.

CREATE TABLE public.reg033_payroll (
    id           integer PRIMARY KEY,
    national_no  bigint NOT NULL,
    msisdn       text NOT NULL
);

INSERT INTO public.reg033_payroll (id, national_no, msisdn) VALUES
    (1,  234056001, '07922123456'),
    (2,  234056002, '07922123457'),
    (3,  234056003, '07922123458'),
    (4,  234056004, '07922123459'),
    (5,  234056005, '07922123460'),
    (6,  234056006, '07922123461'),
    (7,  234056007, '07922123462'),
    (8,  234056008, '07922123463'),
    (9,  234056009, '07922123464'),
    (10, 234056010, '07922123465');
