-- root:   public.reg11_item
-- take:   20
-- expect: exit 13 target.schema.literal_not_rewritable
-- found:  the 2026-09-09 review, finding 5 (probe schema s_ddl_default)
-- why:    a masked column's DEFAULT was recreated in the target verbatim, so the address in it survived the run and the next INSERT would put it back in a row
--
-- The second regression that did not come from testdata/torture/. Finding 5 of
-- docs/reviews/2026-09-09/REVIEW.md is already the smallest schema that fails:
-- one table, two columns, one row, and a column default holding an address.
--
-- What happened. internal/load/ddl copies a column default verbatim
-- (ddl.go's columnDef), because ARCHITECTURE.md section 11.1 recreates every
-- expression as the catalog's own text and there is deliberately no flag that
-- drops a default -- the application's first INSERT is the point of the tool.
-- The row in reg11_item.email was masked, the DEFAULT was not, and the run
-- exited 0 with ddl.canary@example.org sitting in the target's pg_attrdef:
--
--     public.items.email: name matches email; 1/1 samples parse as addresses
--     public.items: 1 rows, child_ok; root
--     dropping public.items in the target
--     public.items: 1 rows
--
-- exit 0. The transcript is docs/reviews/2026-09-09/evidence/ddl_default.log.
-- A row scan cannot establish that the database artefact holds no sensitive
-- literal, and the second net and the residual filter are both row scans.
--
-- The rule T-0134 landed is ARCHITECTURE.md section 11.1's 2026-09-14
-- amendment and THREAT_MODEL.md T1's: a string literal inside a DEFAULT, a
-- CHECK or a generated expression is inside the data boundary. A masked
-- column's default has its literals masked through the column's own masker;
-- one lazyslice cannot rewrite, and that a strong validator reads as an email
-- address, a phone number or a payment card, is exit 13 naming the object.
-- internal/plan/ddlliteral.go is the check, and internal/verify/catalog.go is
-- the second look at the artefact after the load.
--
-- **This header will move to `ok`, and the task that moves it is T-0161.**
-- The masking half is implemented and unit-tested (internal/plan's
-- TestAMaskedColumnsDefaultIsMaskedThroughItsOwnMasker), and it needs the run
-- key: pipeline.PlanRequest.Key. internal/core was outside T-0134's paths and
-- does not fill it yet, so every run reaching this file finds a masked default
-- it cannot rewrite and refuses at 13 rather than masking it. That is the
-- fail-closed direction and it is not the finished behaviour: with the key
-- wired, this run loads, the target's DEFAULT holds a masked address, and the
-- header says `ok` -- the same way 009's did when T-0127 landed.
--
-- reg11_item.email is deliberately the *masked* case. The unmasked case --
-- a literal a strong validator hits in the DDL of a column the run does not
-- mask, which is exit 12 plan.refused.ddl_literal with the --unmask escape --
-- has no reduction here yet and is owed one; internal/plan's unit tests carry
-- it in the meantime.

CREATE TABLE public.reg11_item (
    id    integer PRIMARY KEY,
    email text NOT NULL DEFAULT 'ddl.canary@example.org',
    note  text
);

INSERT INTO public.reg11_item (id, email, note)
SELECT i,
       'person' || i || '@regression.test',
       'note ' || i
FROM generate_series(1, 20) AS g(i);
