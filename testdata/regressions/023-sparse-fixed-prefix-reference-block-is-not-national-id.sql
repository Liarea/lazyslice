-- root:   public.reg023_accounts
-- take:   10
-- expect: ok
-- found:  the T-0187 third review round (2026-09-15), finding 1
-- why:    021 closed the digits-family national_id entry's false positive on
--         a *dense* column (a surrogate key, or an ordinary business-number
--         block generated the same way) by reading the column's own values
--         for a packed numeric range, and on a date column by excluding a
--         value that is also a real calendar date directly. Neither closes
--         this shape: a *sparse* numeric column with a fixed leading prefix
--         and no check digit is neither dense nor a date, and it clears the
--         SSA's own exclusion ranges (no area 000/666/9xx, no group 00, no
--         serial 0000) at essentially 1.0 regardless of what it means --
--         measured at 494/500 for 500 account numbers of the form
--         100000000+rand(1e8), and 500/500 for 500 invoice numbers of the
--         form 202600000+7*rand(50000). An ordinary account_no or invoice_no
--         column of either shape refused an already-loaded run at exit 9
--         with no green path short of --unmask, which is not a defect the
--         operator can act on: there is no threshold under 1.0 that admits
--         it and still refuses a genuine leaked-identifier column, because
--         independently assigned identifiers clear the same ranges at the
--         same rate a fixed-prefix reference number does.
--
-- account_no here is exactly that shape, reduced to ten rows: a fixed
-- leading digit, eight further digits spread across a range many orders of
-- magnitude wider than the sample (min 100482913, max 193014477, a span of
-- about 92.5 million against ten rows -- digitRange.dense() needs the span
-- within twice the row count, so this is nowhere close to dense), chosen so
-- that all ten clear the SSA's exclusion ranges (ratio 1.0) the same way the
-- review's own probe did. Before this finding's fix that ratio alone was
-- enough to refuse the run at exit 9. The fix is corroboration
-- (requiresCorroboration, internal/verify/validators.go): the entry may now
-- refuse only when the column's own name matches rules.yml's national_id
-- pattern (Decision.NameMatchedNationalID) or a certain-or-likely personal
-- column sits in the same table (Decision.TableHasLikelyPersonalColumn).
-- `account_no` matches neither -- its name has no `national_?ids?`, `ssn`,
-- `tax_?ids?` or any other token the pattern carries (it is not even
-- `account_numbers?`, which is financial_account's pattern and needs
-- "number", not "no"), and `reg023_accounts` carries no other column at all
-- -- so the ratio is never asked and the run must load clean.
--
-- reg023_notes gives the vacuity guard something to find (018's own note
-- explains why: the point is that account_no is decided on its own shape,
-- not by a `certain` neighbour, so the email lives in a child table reached
-- by a foreign key rather than beside account_no in its own).

CREATE TABLE public.reg023_accounts (
    id          integer PRIMARY KEY,
    account_no  bigint NOT NULL
);

CREATE TABLE public.reg023_notes (
    id          integer PRIMARY KEY,
    account_id  integer NOT NULL REFERENCES public.reg023_accounts(id),
    email       text NOT NULL
);

INSERT INTO public.reg023_accounts (id, account_no) VALUES
    (1, 100482913),
    (2, 102958174),
    (3, 110982734),
    (4, 118839221),
    (5, 124470982),
    (6, 147822056),
    (7, 165500823),
    (8, 176611340),
    (9, 189325561),
    (10, 193014477);

INSERT INTO public.reg023_notes (id, account_id, email) VALUES
    (1, 1, 'joan.clarke1@realcorp.example'),
    (2, 2, 'joan.clarke2@realcorp.example'),
    (3, 3, 'joan.clarke3@realcorp.example'),
    (4, 4, 'joan.clarke4@realcorp.example'),
    (5, 5, 'joan.clarke5@realcorp.example'),
    (6, 6, 'joan.clarke6@realcorp.example'),
    (7, 7, 'joan.clarke7@realcorp.example'),
    (8, 8, 'joan.clarke8@realcorp.example'),
    (9, 9, 'joan.clarke9@realcorp.example'),
    (10, 10, 'joan.clarke10@realcorp.example');
