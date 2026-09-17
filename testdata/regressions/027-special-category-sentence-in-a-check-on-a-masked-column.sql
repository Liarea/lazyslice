-- root:   public.reg027_patients
-- take:   20
-- expect: exit 13 target.schema.literal_not_rewritable
-- found:  the round-3 red team, docs/reviews/2026-09-15-redteam/round3-still-leaking.json finding 15
-- why:    a masked column's own CHECK carried a special-category sentence with
--         no digit and no name pair, so it was short of every validator
--         internal/plan/ddlliteral.go's strongValidators carried and crossed
--         into the target's catalog under exit 0 (tracker T-0198)
--
-- 'diagnosis' matches rules.yml's special_category name pattern
-- (diagnos[ei]s), so internal/classify masks the column on its name alone
-- (ARCHITECTURE.md section 4, "special categories mask on name alone") --
-- with no value signal at all, since neither of the two sample rows is
-- itself a special-category value. The CHECK's own literal, 'HIV positive,
-- CD4 210', is the round-3 red team's own canary: it carries a digit ("210")
-- but no street-suffix word, so the corroborated addressLiteralShape this
-- file's own strongValidators list already carries does not fire on it, and
-- it is four words short of Dict.ProseName's six-word floor, so free_text
-- does not fire either. Before T-0198's special_category vocabulary
-- validator, strongHit found nothing here and the CHECK -- and its literal
-- health record -- crossed into the target's pg_constraint verbatim, exit 0.
--
-- Fixed by textsig.SpecialCategoryVocabulary, which recognises "HIV" and
-- names the category directly (the refusal reads "parses as
-- special_category") -- the ordinary strongHit path every other DDL-literal
-- category already takes. T-0198's own broadened rule (refusing a masked
-- column's CHECK on any literal no validator recognised at all) does not
-- carry this file: the T-0198 fix round (2026-09-16) scoped that rule to a
-- DEFAULT and a generated expression only, after two real schemas (not this
-- one) found the escape it offered a CHECK or an index could force
-- unmasking a column that had nothing to do with the refused literal
-- (internal/plan/CLAUDE.md's own T-0198 section). This file's own refusal
-- was never evidence for that half regardless -- 'HIV positive, CD4 210'
-- was always a strongHit once the vocabulary validator existed, so the two
-- rules were never both load-bearing for the same file at once.

CREATE TABLE public.reg027_patients (
    id        integer PRIMARY KEY,
    diagnosis text
);

INSERT INTO public.reg027_patients (id, diagnosis) VALUES
    (1, 'routine checkup'),
    (2, 'annual physical');

ALTER TABLE public.reg027_patients
  ADD CONSTRAINT reg027_diagnosis_not_canary
  CHECK (diagnosis IS NULL OR diagnosis <> 'HIV positive, CD4 210');
