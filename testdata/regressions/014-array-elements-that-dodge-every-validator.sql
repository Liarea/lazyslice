-- root:   public.reg014_subjects
-- take:   20
-- expect: ok
-- found:  the 2026-09-15 red team, attack A3 (array variant)
-- why:    a text[] whose elements are obfuscated email addresses reached the
--         target verbatim: the array machinery split the elements and offered
--         them to the validators, and no validator recognised
--         "grace.hopper AT realcorp DOT example" as an address
--
-- The fifth regression that did not come from testdata/torture/. The red team's
-- A3 array variant is already the smallest schema that shows it: one table, one
-- text[] column, and two sibling columns that *were* masked, which is what makes
-- the miss legible rather than arguable.
--
-- What happened. testdata/nasty.sql's array traps cover an array the driver
-- decodes, an array that arrives as a text literal, and an array of an
-- extension's base type. None of them covers an array whose *elements*
-- individually dodge every validator, and that is the case here: the split was
-- correct (T-0118, T-0129), each element reached ValidEmail as its own string,
-- and ValidEmail said no, because the address was written with " AT " and
-- " DOT " instead of "@" and ".". The column was decided `none`, transform
-- copied it, and internal/verify's second net asked the same validators the
-- same question of the same strings in the target and got the same answer, so
-- both of THREAT_MODEL.md T1's controls agreed the column was clean.
--
-- The fix is not in the array machinery, which was never at fault. It is
-- internal/textsig/candidates.go: ValidEmail, ValidPhone, ValidLuhn and
-- ValidIBAN now read every spelling textsig.Candidates yields — " at "/"(at)",
-- " dot "/"(dot)"/" kropka ", a run of digits spelled out in words, a value
-- grouped with spaces, and the address inside an RFC 5322 display name — so an
-- obfuscated address is the address it spells. Because internal/textsig is the
-- one home for a value shape (T-0055), the classifier's signals and the second
-- net both gained it in one change.
--
-- Before the fix the target held quiet = {"grace.hopper AT realcorp DOT
-- example", ...} byte for byte, under exit 0, with `contact` and `handle`
-- beside it correctly masked. After it the column is `email` and masked
-- element-wise, the run still exits 0, and TestTortureRegressions' leak
-- assertion finds no trace of realcorp.example in the target.

CREATE TABLE public.reg014_subjects (
    id      integer PRIMARY KEY,
    email   text NOT NULL,
    phone   text NOT NULL,
    quiet   text[] NOT NULL
);

INSERT INTO public.reg014_subjects (id, email, phone, quiet) VALUES
    (1, 'grace.hopper@realcorp.example',      '+44 20 7946 0958',
        ARRAY['grace.hopper AT realcorp DOT example', 'g.hopper AT realcorp DOT example']),
    (2, 'katherine.johnson@realcorp.example', '+44 20 7946 0959',
        ARRAY['katherine.johnson AT realcorp DOT example']),
    (3, 'ada.lovelace@realcorp.example',      '+44 20 7946 0960',
        ARRAY['ada.lovelace AT realcorp DOT example']),
    (4, 'alan.turing@realcorp.example',       '+44 20 7946 0961',
        ARRAY['alan.turing AT realcorp DOT example']),
    (5, 'dorothy.vaughan@realcorp.example',   '+44 20 7946 0962',
        ARRAY['dorothy.vaughan AT realcorp DOT example']);
