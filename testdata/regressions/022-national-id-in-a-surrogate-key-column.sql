-- root:   public.reg022_citizens
-- take:   10
-- expect: exit 9 verify.refused.second_net
-- found:  the T-0187 review round (2026-09-15), finding 3
-- why:    the first fix for finding 1 (021's own file) exempted the
--         digits-family national_id entry outright for any column
--         internal/classify decided is a surrogate key, read off classify's
--         own "preserved verbatim" reason -- but classify grants that
--         exemption precisely when it found nothing personal in the key
--         column, which is the same miss this net exists to catch a second
--         time. A primary key of real SSNs, in a column named so no
--         rules.yml pattern matches it, was exempted by classify, skipped
--         by this net, and crossed into the target verbatim at exit 0 --
--         before the exemption existed at all, the same column correctly
--         refused.
--
-- citizen_no holds ten real-shaped SSNs (valid under the SSA's own
-- exclusion ranges) chosen to fail every checksum-only national_id format as
-- well, so internal/classify's own value validators find nothing here
-- either -- the column reaches internal/classify's decide() at `none`,
-- reason "no name or value signal; surrogate key: preserved verbatim", the
-- same "not proven personal" state 020's taxref column reaches, except this
-- column IS the primary key rather than an ordinary column beside one.
--
-- The fix (internal/verify/secondnet.go's digitRange, validators.go's
-- sequenceExempt) reads the column's own values during the scan instead of
-- classify's decision: a surrogate key's own values are a dense run
-- (id, id+1, id+2, ...), and these ten are not -- they are spread across a
-- range many orders of magnitude wider than the sample, which is what a set
-- of independently assigned national identifiers looks like whether or not
-- the column happens to be a primary key. So the exemption does not fire,
-- the digits-family entry runs, and the ratio is 1.0.
--
-- 021-ordinary-numeric-columns-clear-the-ssn-ratio.sql is this file's own
-- mirror image: a dense surrogate primary key holding ordinary sequential
-- values, which the same fix must still pass.
--
-- Why this file also carries an `email` column now (the T-0187 third review
-- round, finding 1). The round after this one found the ratio above, on its
-- own, cannot tell a sparse column of independently assigned identifiers
-- (this file's own point) from a sparse column of ordinary fixed-prefix
-- reference numbers -- both clear the SSA's exclusion ranges at essentially
-- 1.0 -- so the entry now refuses only with corroboration: a rules.yml
-- national_id name-pattern hit on the column itself, or a certain-or-likely
-- personal column in the same table. `citizen_no` has neither on its own --
-- its name still matches no pattern (above), and before this change
-- `reg022_citizens` carried no other column that classify decides is
-- personal at all -- so without a change here this file would flip from
-- refusing to `ok` under the new rule, which is not the finding 022 exists
-- to pin. `email` is a plain, ordinary personal column added for exactly
-- that reason -- it is not itself the leak this file is about, and it is
-- masked like any other email column -- and it is what gives
-- `reg022_citizens` the neighbour signal `citizen_no`'s own refusal now
-- needs. `joined_on` is left as it was: an ordinary date with no name rules.yml's
-- person_date pattern matches, so it is not what supplies the signal.

CREATE TABLE public.reg022_citizens (
    citizen_no  bigint PRIMARY KEY,
    joined_on   date NOT NULL,
    email       text NOT NULL
);

INSERT INTO public.reg022_citizens (citizen_no, joined_on, email) VALUES
    (450205958, '2019-03-11', 'margaret.hamilton1@realcorp.example'),
    (457756677, '2020-07-02', 'margaret.hamilton2@realcorp.example'),
    (343649590, '2018-11-23', 'margaret.hamilton3@realcorp.example'),
    (715337899, '2021-05-14', 'margaret.hamilton4@realcorp.example'),
    (753394142, '2017-09-30', 'margaret.hamilton5@realcorp.example'),
    (634558842, '2022-01-08', 'margaret.hamilton6@realcorp.example'),
    (244921135, '2016-06-19', 'margaret.hamilton7@realcorp.example'),
    (619389362, '2023-02-27', 'margaret.hamilton8@realcorp.example'),
    (882033568, '2015-12-04', 'margaret.hamilton9@realcorp.example'),
    (710300489, '2024-04-16', 'margaret.hamilton10@realcorp.example');
