-- root:   public.reg029_customers
-- take:   20
-- expect: exit 9 verify.refused.second_net
-- found:  T-0198 fix round, finding 4 (reviewer of T-0198's own landing)
-- why:    pipeline.CatSpecial had a value validator in both DDL-literal
--         passes (internal/plan/ddlliteral.go's strongValidators,
--         internal/verify/catalog.go's strongCatalogHit) but none in
--         internal/verify/validators.go's row-scanning second net, so a
--         digit/name-free special-category sentence in an unmasked column's
--         ROW value -- not its DDL -- still crossed the second net unseen
--         (tracker T-0231)
--
-- 'label' matches no rules.yml pattern (the same column 028 uses, for the
-- identical reason) and carries no value signal of its own by the rule
-- pack's own validators, so internal/classify leaves it unmasked -- and,
-- unlike 027 and 028, nothing about this column's own name says
-- "special_category" at all, which is why the row path needs its own value
-- validator rather than inheriting the two DDL passes' fix for free. One row
-- holds 'bipolar disorder', one of textsig.SpecialCategoryVocabulary's own
-- terms: no digit for addressLiteralShape/AddressShape to catch, no given-
-- and-surname pair for Dict.ProseName, nothing any of the eleven pre-T-0231
-- second-net entries recognised. Before validators.go carried a CatSpecial
-- entry this crossed into the target verbatim under exit 0, "no name or
-- value signal". With the entry, the second net's ordinary (non-strong,
-- non-dict) ratio rule applies: two rows is below minValues, so a column
-- that is not proven fails on any hit at all (T-0058's own floor), exactly
-- as address and credential already do at the same size.

CREATE TABLE public.reg029_customers (
    id    integer PRIMARY KEY,
    label text
);

INSERT INTO public.reg029_customers (id, label) VALUES
    (1, 'bipolar disorder'),
    (2, 'standard');
