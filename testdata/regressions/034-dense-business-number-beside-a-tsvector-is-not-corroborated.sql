-- root:   public.reg034_orders
-- take:   10
-- expect: ok
-- found:  the T-0240 review round (2026-09-17), high finding
-- why:    Decision.TableHasLikelyPersonalColumn -- one of corroborated's
--         three signals in internal/verify/secondnet.go -- counts a
--         neighbour at ConfLikely or above under ANY category, with no
--         identifiesAPerson test: internal/classify decides a tsvector
--         `derived_text` at ConfCertain by its type alone, on every schema
--         that has one, never because it holds a person's data. Reading
--         that signal to decide whether a DENSE column's sequence
--         exemption survives let a search-index column beside an ordinary,
--         non-key business-number column cancel that column's own
--         exemption: `business_ref` is not a key or an FK child
--         (neverMasked is false), and the table's only other column is the
--         tsvector, so corroborated answered true on the strength of a
--         neighbour that is not personal data and never will be. The ratio
--         was then scored and refused the run at exit 9 with no green path
--         short of --unmask, over a column with nothing wrong with it --
--         reproduced against this exact shape before the fix (a
--         `scr900_orders(id bigint PK, business_ref bigint holding
--         400100000+i, search tsvector)` table with the target already
--         dropped and loaded), and passing with the nine files of the
--         T-0240 task itself stashed, which is what proved it introduced
--         rather than pre-existing.
--
-- The fix is corroboratedForSequence (internal/verify/secondnet.go), read
-- only by the dense-sequence override: it answers with NameMatchedNationalID
-- and TableHasMaskedPersonalColumn alone, never
-- TableHasLikelyPersonalColumn. TableHasMaskedPersonalColumn's own gate
-- (maskedPersonalNeighbour) tests identifiesAPerson explicitly, and a
-- tsvector's derived_text category is one of the four identifiesAPerson
-- excludes by name (internal/classify/classify.go's own comment on the
-- point), so this table's search column can never corroborate anything
-- through it. requiresCorroboration's own three-signal `corroborated`
-- (secondnet.go) is untouched, so 032 and 033 -- which corroborate through
-- TableHasMaskedPersonalColumn -- still refuse: this file is their control
-- in the other direction, with no masked or matched personal column
-- anywhere in the table at all.
--
-- business_ref carries 021's own ten dense values verbatim (021's control
-- pins that this exact shape clears every other validator this net has),
-- so the only thing this file adds beyond 021 is the tsvector neighbour
-- that the bug read as corroboration and the fix does not.
--
-- A second table, reg034_notes, carries an email column the same way 021's
-- own reg021_probe_notes does: assertTortureNoLiteralSurvives -- the leak
-- check every `expect: ok` file gets automatically -- needs a source email
-- or phone number to grep for, and it has to sit outside reg034_orders so
-- it cannot itself corroborate business_ref (Decision.TableHasLikelyPersonalColumn
-- and Decision.TableHasMaskedPersonalColumn are both computed per table).

CREATE TABLE public.reg034_orders (
    id           integer PRIMARY KEY,
    business_ref bigint NOT NULL,
    search       tsvector NOT NULL
);

CREATE TABLE public.reg034_notes (
    id        integer PRIMARY KEY,
    order_id  integer NOT NULL REFERENCES public.reg034_orders(id),
    email     text NOT NULL
);

INSERT INTO public.reg034_orders (id, business_ref, search) VALUES
    (1,  400100000, to_tsvector('english', 'first order placed online')),
    (2,  400100001, to_tsvector('english', 'second order placed online')),
    (3,  400100002, to_tsvector('english', 'third order placed online')),
    (4,  400100003, to_tsvector('english', 'fourth order placed online')),
    (5,  400100004, to_tsvector('english', 'fifth order placed online')),
    (6,  400100005, to_tsvector('english', 'sixth order placed online')),
    (7,  400100006, to_tsvector('english', 'seventh order placed online')),
    (8,  400100007, to_tsvector('english', 'eighth order placed online')),
    (9,  400100008, to_tsvector('english', 'ninth order placed online')),
    (10, 400100009, to_tsvector('english', 'tenth order placed online'));

INSERT INTO public.reg034_notes (id, order_id, email) VALUES
    (1,  1,  'ada.lovelace1@realcorp.example'),
    (2,  2,  'ada.lovelace2@realcorp.example'),
    (3,  3,  'ada.lovelace3@realcorp.example'),
    (4,  4,  'ada.lovelace4@realcorp.example'),
    (5,  5,  'ada.lovelace5@realcorp.example'),
    (6,  6,  'ada.lovelace6@realcorp.example'),
    (7,  7,  'ada.lovelace7@realcorp.example'),
    (8,  8,  'ada.lovelace8@realcorp.example'),
    (9,  9,  'ada.lovelace9@realcorp.example'),
    (10, 10, 'ada.lovelace10@realcorp.example');
