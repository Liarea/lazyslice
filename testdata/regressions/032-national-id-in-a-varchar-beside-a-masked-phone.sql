-- root:   public.reg032_payroll
-- take:   10
-- expect: exit 9 verify.refused.second_net
-- found:  the 2026-09-15 red team round 4, the A9b varchar(9) replay
--         (docs/reviews/2026-09-15-redteam/round4-still-leaking.json)
-- why:    the same dense, zero-padded national identifier A9b's bigint form
--         used, moved into a varchar(9) column, beside an `msisdn` column
--         real enough to be masked as `phone` on its name alone -- and
--         crossed verbatim under exit 0 anyway, because
--         internal/verify/validators.go had no character-family entry that
--         could ever recognise the shape, and the corroboration gate that
--         should have refused it on the strength of `msisdn` counted the
--         table as holding no personal column at all.

-- What happened, and why fixing only one half would still leak this file.
-- `msisdn` is one of rules.yml's own phone abbreviations
-- ('(^|_)(...|msisdn|...)(_|$)'), so internal/classify's decide() takes it
-- on the name alone (hasName && nameAccepted); with no --phone-region
-- configured, its plain national-format values never validate under
-- textsig.ValidPhone's international-only reading, so the value signal is
-- nil and decide's own name-match-with-nothing-from-the-values branch
-- records ConfPossible, not ConfLikely or ConfCertain -- ARCHITECTURE.md
-- §4's own threshold for that shape of evidence. `taxref` matches no
-- rules.yml pattern at all ("taxref" is neither `tax_?ids?` nor any other
-- token the national_id pattern carries) and internal/classify's own
-- national_id entry never recovers a zero-padded, separator-free digit run
-- (internal/textsig/CLAUDE.md's own note on ValidNationalIDDigits being
-- deliberately outside internal/classify's reach), so `taxref` reaches
-- decide() at `none` and internal/transform copies it.
--
-- What is left to catch it is internal/verify's second net, and it used to
-- fail this file two different ways at once. First: nothing in
-- validators.go's character-family national_id entries (the structured six
-- and the checksum-only six) matches a bare nine-digit run with no
-- separator -- structured needs a dash, a letter or NIR's fifteen
-- characters, and none of the checksum-only six is nine digits under an
-- SSA-style exclusion-range check -- so the value never reached a validator
-- that recognised it at all, whatever corroborated() answered. Second, even
-- once the character-family twin of the digits-family entry is wired
-- (ok: textsig.ValidNationalIDDigits, T-0240): `taxref`'s own ten values
-- pack into a range ten wide across ten rows, which digitRange.dense()
-- exempts on its own, and before this task's fix that exemption fired
-- ahead of requiresCorroboration and skipped the entry outright, whatever
-- corroborated() would have said. And corroborated() itself said false: it
-- read Decision.TableHasLikelyPersonalColumn, which only counts a neighbour
-- at ConfLikely or above, and `msisdn` -- the only other column in the
-- table -- never reaches that floor on a name match with no value signal
-- of its own.
--
-- The fix is Decision.TableHasMaskedPersonalColumn (a masked,
-- person-identifying neighbour at ConfPossible or above corroborates too),
-- the character-family national_id entry, and the dense-sequence exemption
-- no longer outranking corroboration once it exists to ask
-- (internal/verify/CLAUDE.md's own T-0240 section has the full account).
-- All three are needed: dropping any one of them leaves this file at
-- exit 0.

CREATE TABLE public.reg032_payroll (
    id      integer PRIMARY KEY,
    taxref  varchar(9) NOT NULL,
    msisdn  text NOT NULL
);

INSERT INTO public.reg032_payroll (id, taxref, msisdn) VALUES
    (1,  '780510001', '07911123456'),
    (2,  '780510002', '07911123457'),
    (3,  '780510003', '07911123458'),
    (4,  '780510004', '07911123459'),
    (5,  '780510005', '07911123460'),
    (6,  '780510006', '07911123461'),
    (7,  '780510007', '07911123462'),
    (8,  '780510008', '07911123463'),
    (9,  '780510009', '07911123464'),
    (10, '780510010', '07911123465');
