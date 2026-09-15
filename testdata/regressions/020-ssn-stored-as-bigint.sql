-- root:   public.reg020_payroll
-- take:   5
-- expect: exit 9 verify.refused.second_net
-- found:  the 2026-09-15 red team round 2, R2-04/A9b
-- why:    a bigint column can hold no hyphen and drops a leading zero, so a
--         dashed SSN with area 0-something renders as an eight-digit number
--         -- a shape textsig.ValidNationalID's dashed regexp never matches,
--         even once it is registered on both nets (018's fix). Before
--         T-0187's digits-family entry this column was reported "no name or
--         value signal" and crossed into the target verbatim under exit 0.
--
-- What happened, and why this file's own header is a refusal and not `ok`.
-- `taxref` has no name rule pack pattern matches (`tax_?ids?` needs the
-- underscore or the bare word, and `taxref` is neither) and, unlike 018's
-- `code` column, its values can never reach internal/classify's national_id
-- entry either: ValidNationalID is deliberately narrow and does not recover a
-- bigint's dropped leading zero (internal/textsig/CLAUDE.md's rule that a
-- validator changed there must never widen the DDL-literal passes' one-hit
-- refusal). So internal/classify still decides this column `none` and
-- internal/transform still copies it. What closes the leak is
-- internal/verify's second net: its new digits-family entry
-- (ValidNationalIDDigits, ratio-scored rather than strong, mirroring T-0136's
-- own Luhn text/digits split) reads the loaded, unmasked column, recovers the
-- zero-padded shape, and refuses the run at exit 9 -- a refusal over an
-- already-loaded target rather than a silent leak, which is the same
-- "refusal instead of a mask" shape this codebase already accepts for a
-- proven bigint column with a Luhn minority hit (internal/classify/CLAUDE.md's
-- strongHit note). Closing the classify-side gap too -- teaching the ordered
-- validators list itself to recover a numeric family's dropped leading zero --
-- is a wider change than this task's paths reach and is reported separately.
--
-- Why the five values changed, and why this file also carries an `email`
-- column now (the T-0187 third review round, finding 1). That round found
-- the digits-family entry's ratio, on its own, cannot tell a sparse column
-- of independently assigned identifiers from a sparse column of ordinary
-- fixed-prefix reference numbers -- both clear the SSA's exclusion ranges at
-- essentially 1.0 -- so the entry now refuses only with corroboration: a
-- rules.yml national_id name-pattern hit on the column itself, or a
-- certain-or-likely personal column in the same table. Checked directly
-- rather than assumed, per the finding's own instruction: `taxref`'s name
-- still matches no pattern (above), so the second signal has to come from a
-- neighbour -- but this file's *original* five values (from "078-05-1001"
-- and neighbouring numbers) turned out to be an unrelated trap of their own:
-- three of the five coincidentally clear one of internal/classify's own
-- checksum-only national_id formats (BSN's 11-proef, or Australia's TFN) on
-- the bare digit string, which put `taxref` itself at internal/classify's
-- `low` confidence -- "no name signal" but a real value signal -- rather than
-- the `none` this file's own prose always claimed. `low` is below the mask
-- threshold on its own, so the original file still passed before this
-- change; once a personal neighbour is added for corroboration, the
-- *existing* neighbouring-column rule (unrelated to this finding) raises any
-- `low` column beside a `likely` one to `possible`, which masks `taxref`
-- outright and defeats the regression a different way. The five values below
-- are chosen the same way 022's ten are -- real-shaped, and confirmed by
-- direct computation to clear none of internal/classify's checksum-only or
-- structured national_id formats -- so `taxref` reaches internal/classify's
-- decide() at `none` exactly as the prose above says, `email` gives
-- `reg020_payroll` the neighbour signal `taxref`'s own refusal now needs, and
-- `taxref` still refuses on its own digits-family ratio (1.0, neither dense
-- nor a date) once corroborated.

CREATE TABLE public.reg020_payroll (
    id      integer PRIMARY KEY,
    taxref  bigint NOT NULL,
    email   text NOT NULL
);

INSERT INTO public.reg020_payroll (id, taxref, email) VALUES
    (1, 234056789, 'alan.turing1@realcorp.example'),
    (2, 56781234, 'alan.turing2@realcorp.example'),
    (3, 345678901, 'alan.turing3@realcorp.example'),
    (4, 87234567, 'alan.turing4@realcorp.example'),
    (5, 456789012, 'alan.turing5@realcorp.example');
