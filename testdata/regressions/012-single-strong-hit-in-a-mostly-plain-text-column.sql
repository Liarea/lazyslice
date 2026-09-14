-- root:   public.reg012_note
-- take:   20
-- expect: ok
-- found:  the 2026-09-09 review, finding 7 (evidence/sparse_email.log)
-- why:    a generic text column of one email address among nineteen ordinary
--         strings had a hit ratio of 5%, well under the 80% column threshold,
--         so the classifier decided `none` and the target held the source's
--         own email address under exit 0
--
-- The third regression that did not come from testdata/torture/. Finding 7 of
-- docs/reviews/2026-09-09/REVIEW.md is already the smallest schema that shows
-- it -- one table, one text column, twenty rows -- and evidence/sparse_email.log
-- is its transcript: "One email plus nineteen ok values in a generic text
-- column survived unchanged with exit 0."
--
-- What happened. ARCHITECTURE.md section 4's value signal is a ratio: at least
-- 80% of a column's non-null samples have to validate for a category before the
-- column is decided on its values alone. That is a good question for "what kind
-- of column is this" and a poor one for "does this column, about to be loaded
-- into the target, hold a value that is somebody's actual email address" -- a
-- ratio treats nineteen ordinary rows as evidence that the twentieth is not what
-- it plainly is. internal/classify/classify.go's bestSignal read one email
-- address out of twenty samples as 5%, below even the weak threshold, and
-- decided the column `none`; internal/transform copied it; internal/verify's
-- second net re-ran the same validator over the whole column, got the same 5%
-- ratio and the same answer, because it asked the same question of the same
-- evidence. Nothing in the run ever asked "is there a recognisable value in
-- here" instead of "is this column made of them".
--
-- The fix is two changes, in the two places that asked the ratio question.
-- internal/classify/classify.go's bestSignal now tracks the first *strong*
-- validator (a precise parse: email, phone, network_id, financial_account,
-- online_id -- internal/classify/classify.go's validators list, mirroring
-- internal/verify/validators.go's own `strong` field) that matches at least one
-- proven sample without reaching the column threshold, and decide() masks the
-- column as free_text on that alone: one recognisable email address among
-- ordinary strings is not proof the column *is* an email column, but it is proof
-- the column is not clean, and free_text's masker replaces every value with
-- generated prose rather than trying to keep the nineteen looking as they were.
-- internal/verify/secondnet.go carries the second half for any column this
-- misses: a *strong* validator hitting even one distinct value in an unmasked
-- column is exit 9 naming the column and the hit count, never the value,
-- whatever the ratio and whatever the column's size -- the net that used to ask
-- the same 80% question over the whole target now refuses on the strength of
-- one production value instead of waiting for four out of five. The ratio rule
-- is unchanged for the two validators that are a shape guess rather than a
-- parse (`credential`, `address`): a slug or a room number is not the same
-- claim as a valid email address, and neither net fails an ordinary column on
-- one occurrence of either.
--
-- Before the fix, this file's run was:
--
--     public.reg012_note.body: no name or value signal
--     public.reg012_note: 20 rows, child_ok; root
--     dropping public.reg012_note in the target
--     public.reg012_note: 20 rows
--
-- exit 0, with person19@source.invalid sitting in the target exactly as the
-- source wrote it. After the fix, internal/classify decides `body` free_text
-- and masks every row, so the run still exits 0 and the target holds no email
-- address at all -- checked by TestTortureRegressions' leak assertion, which is
-- the half of this file that matters: an unchanged exit code would have had
-- nothing to say about the defect either.

CREATE TABLE public.reg012_note (
    id   integer PRIMARY KEY,
    body text NOT NULL
);

INSERT INTO public.reg012_note (id, body)
SELECT i, 'ok, ticket ' || i || ' closed'
FROM generate_series(1, 19) AS g(i);

INSERT INTO public.reg012_note (id, body)
VALUES (20, 'person19@source.invalid');
