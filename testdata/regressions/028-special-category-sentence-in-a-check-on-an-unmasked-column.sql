-- root:   public.reg028_customers
-- take:   20
-- expect: exit 12 plan.refused.ddl_literal
-- found:  the round-3 red team, docs/reviews/2026-09-15-redteam/round3-still-leaking.json finding 15
-- why:    the same special-category sentence, on a column no rules.yml
--         pattern matches and this run therefore does not mask, still needs
--         the operator's own --unmask before its CHECK can cross into the
--         target (tracker T-0198)
--
-- 'label' matches no rules.yml pattern (unlike 'notes', which the free_text
-- pattern names) and carries no value signal of its own (two short,
-- ordinary rows), so internal/classify leaves it unmasked. Its CHECK
-- carries the round-3 red team's other canary, 'Ahmadiyya Muslim' -- two
-- words, no digit, nothing address- or name-shaped -- which crossed into
-- the target's pg_constraint verbatim under exit 0 before
-- textsig.SpecialCategoryVocabulary existed (it recognises "Muslim" and
-- names the category directly). Because the column is not masked, T-0198's
-- broadened "whatever it parses as" rule does not apply here -- that rule is
-- for a masked column's own, never-rewritten constraint -- so this is the
-- pre-existing plan.refused.ddl_literal path (ddlliteral.go's
-- refuseUnmaskedLiteral), at exit 12 with the operator's own
-- --unmask public.reg028_customers.label=REASON as the escape, the same
-- shape every other unmasked-column DDL literal refusal already has.

CREATE TABLE public.reg028_customers (
    id    integer PRIMARY KEY,
    label text
);

INSERT INTO public.reg028_customers (id, label) VALUES
    (1, 'standard'),
    (2, 'priority');

ALTER TABLE public.reg028_customers
  ADD CONSTRAINT reg028_label_not_canary
  CHECK (label IS NULL OR label <> 'Ahmadiyya Muslim');
