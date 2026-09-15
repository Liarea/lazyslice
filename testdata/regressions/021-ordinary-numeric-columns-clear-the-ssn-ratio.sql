-- root:   public.reg021_probe_orders
-- take:   10
-- expect: ok
-- found:  the T-0187 review round (2026-09-15), findings 1 and 3
-- why:    internal/verify's digits-family national_id entry has no check
--         digit -- only the SSA's own exclusion ranges (no area 000/666/9xx,
--         no group 00, no serial 0000) -- so it cleared ~91% of random
--         9-digit numbers and, over any realistic date range, effectively
--         100% of YYYYMMDD integer dates and of a dense run of assigned
--         numbers; an ordinary bigint primary key, an ordinary booking-date
--         integer column and an ordinary non-key business-number column all
--         refused the whole run at exit 9 with no green path short of
--         --unmask
--
-- What happened, and why this file no longer tunes a single row to sit just
-- under a threshold. The first fix (a threshold raised to 0.97, plus a skip
-- for a column internal/classify had decided is a surrogate key) was itself
-- wrong twice over: the 97%-of-YYYYMMDD-dates figure it was set above was an
-- artefact of averaging over 1950-2050 (over any realistic booking range the
-- true clear rate is 1.0, measured 1000/1000 across 2022-2024), and the
-- decision-based skip could never reach a *non-key* dense sequence at all --
-- this file's own reviewer measured 10000/10000 on a sequential 9-digit
-- business number with no primary key or uniqueness constraint anywhere near
-- it. Both are fixed structurally rather than by a tuned ratio: a value that
-- is also a real calendar date is excluded by
-- textsig.ValidNationalIDDigits directly (looksLikePlausibleDate,
-- internal/textsig/nationalid.go), and a column whose own values pack into a
-- dense numeric range -- a generated sequence, key or not -- is exempted by
-- internal/verify's own digitRange (secondnet.go), read from the values
-- during the scan and never from internal/classify's decision. So this file
-- now holds three columns that must each pass on the strength of their own
-- shape rather than a hand-placed exception: an ordinary surrogate primary
-- key (dense), an ordinary non-key business-number column (dense, no key or
-- uniqueness constraint at all), and a booking-date column whose every value
-- is a real 2024 date. None is tuned to sit near any threshold.
--
-- 022-national-id-in-a-surrogate-key-column.sql is this file's own mirror
-- image: a primary key of real, non-dense SSNs, which the fix above must
-- still refuse.

CREATE TABLE public.reg021_probe_orders (
    id           bigint PRIMARY KEY,
    business_ref bigint NOT NULL,
    booked_on    integer NOT NULL
);

-- Gives assertTortureNoLiteralSurvives -- the leak check every `expect: ok`
-- file gets automatically -- something to find, without putting a personal
-- column beside id, business_ref or booked_on in their own table (018's own
-- note explains why: the point is that none of the three is decided by a
-- `certain` neighbour, only by its own shape).
CREATE TABLE public.reg021_probe_notes (
    id        integer PRIMARY KEY,
    order_id  bigint NOT NULL REFERENCES public.reg021_probe_orders(id),
    email     text NOT NULL
);

INSERT INTO public.reg021_probe_orders (id, business_ref, booked_on) VALUES
    (100234567, 400100000, 20240103),
    (100234568, 400100001, 20240108),
    (100234569, 400100002, 20240115),
    (100234570, 400100003, 20240119),
    (100234571, 400100004, 20240202),
    (100234572, 400100005, 20240211),
    (100234573, 400100006, 20240229),
    (100234574, 400100007, 20240305),
    (100234575, 400100008, 20240318),
    (100234576, 400100009, 20240322);

INSERT INTO public.reg021_probe_notes (id, order_id, email) VALUES
    (1, 100234567, 'ada.lovelace1@realcorp.example'),
    (2, 100234568, 'ada.lovelace2@realcorp.example'),
    (3, 100234569, 'ada.lovelace3@realcorp.example'),
    (4, 100234570, 'ada.lovelace4@realcorp.example'),
    (5, 100234571, 'ada.lovelace5@realcorp.example'),
    (6, 100234572, 'ada.lovelace6@realcorp.example'),
    (7, 100234573, 'ada.lovelace7@realcorp.example'),
    (8, 100234574, 'ada.lovelace8@realcorp.example'),
    (9, 100234575, 'ada.lovelace9@realcorp.example'),
    (10, 100234576, 'ada.lovelace10@realcorp.example');
