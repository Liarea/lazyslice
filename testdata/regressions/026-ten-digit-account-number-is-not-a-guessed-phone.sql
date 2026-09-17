-- root:       public.reg026_orders
-- take:       10
-- expect:     ok
-- found:      the T-0221 review round (this task's own brief)
-- not-masked: public.reg026_orders.account_no
-- why:        with no --phone-region configured, internal/classify tries a
--             short built-in list of common regions before giving up on a
--             national-format phone reading (T-0221); an ordinary ten-digit
--             account number clears at least one of those regions'
--             numbering-plan validity checks often enough by chance --
--             measured well over half of random ten-digit numbers across
--             fifteen regions -- that masking on the guess alone would cost
--             a real, non-personal column to a guess nobody asked the tool
--             to make. Without corroboration, this column must stay exactly
--             as it was: unmasked, and reported so.
--
-- Every value below was chosen because it independently clears
-- textsig.ValidPhoneRegion under at least one of internal/classify's
-- phoneGuessRegions (verified by direct computation against the function,
-- not by construction): a fixture that happened not to trip any region would
-- prove nothing about the corroboration gate, the way an SSN chosen to fail
-- every checksum-only national_id format proved nothing about
-- requiresCorroboration until 020's own header explains the same trap. The
-- table carries no other column at all -- no name rule internal/classify's
-- phone pattern matches, and no neighbouring column for the
-- TableHasLikelyPersonalColumn half of the gate to read either -- so this is
-- the shape neither corroboration signal can reach, on purpose: the guess
-- must decide nothing here, or the run is wrong in the T-0221 direction
-- rather than the THREAT_MODEL.md T1 one.
--
-- account_no is text, not bigint, and that is deliberate rather than
-- incidental (the review round that followed this file's first draft): the
-- guessed-region pass only ever runs over a character family
-- (text/varchar/bpchar/citext), because a numeric family is exactly what
-- internal/verify's own national_id digits entry exists to scan unmasked,
-- ratio-scored and itself corroboration-gated (T-0187 third review round;
-- see internal/classify/classify.go's own comment on the point) --
-- 020-ssn-stored-as-bigint.sql is precisely that column, and a bigint here
-- would have raced the same coincidence 020's own header already explains
-- for a different category. ordinary_id is present only so the table is not
-- reduced to a single column and to give assertTortureNoLiteralSurvives an
-- email to find elsewhere in the slice (a second table, reached by the
-- foreign key, exactly as 018 through 020 use one).

CREATE TABLE public.reg026_orders (
    id          integer PRIMARY KEY,
    account_no  text NOT NULL,
    ordinary_id bigint NOT NULL
);

CREATE TABLE public.reg026_customers (
    id         integer PRIMARY KEY,
    order_id   integer NOT NULL REFERENCES public.reg026_orders(id),
    email      text NOT NULL
);

INSERT INTO public.reg026_orders (id, account_no, ordinary_id) VALUES
    (1, '9231278675', 500001),
    (2, '7543856411', 500002),
    (3, '5101878760', 500003),
    (4, '5526624009', 500004),
    (5, '9743547657', 500005),
    (6, '6214237261', 500006),
    (7, '2883513247', 500007),
    (8, '8062614208', 500008),
    (9, '4805492868', 500009),
    (10, '7461764184', 500010);

INSERT INTO public.reg026_customers (id, order_id, email) VALUES
    (1, 1, 'grace.hopper1@realcorp.example'),
    (2, 2, 'grace.hopper2@realcorp.example'),
    (3, 3, 'grace.hopper3@realcorp.example'),
    (4, 4, 'grace.hopper4@realcorp.example'),
    (5, 5, 'grace.hopper5@realcorp.example'),
    (6, 6, 'grace.hopper6@realcorp.example'),
    (7, 7, 'grace.hopper7@realcorp.example'),
    (8, 8, 'grace.hopper8@realcorp.example'),
    (9, 9, 'grace.hopper9@realcorp.example'),
    (10, 10, 'grace.hopper10@realcorp.example');
