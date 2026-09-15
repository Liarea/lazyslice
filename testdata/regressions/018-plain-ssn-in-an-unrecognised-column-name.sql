-- root:   public.reg018_records
-- take:   6
-- expect: ok
-- found:  the 2026-09-15 red team round 2, R2-02/A6
-- why:    a plain, unobfuscated US SSN in a column called `code` -- a name no
--         rule pack pattern matches, in a table with no other personal column
--         so the neighbouring-column rule cannot fire -- was reported "no name
--         or value signal" and crossed into the target verbatim under exit 0,
--         because national_id had no value validator anywhere on the row path
-- not-copied: public.reg018_records.code
--
-- What happened. internal/textsig.ValidNationalID was correct: a scratch copy
-- of the package returned true for every value below. The problem was that
-- nothing on the row-scanning path ever called it -- internal/classify's
-- ordered validators list had no CatNationalID entry, and internal/verify's
-- second net had no national_id entry either, so this is a miss-twice of
-- exactly the shape THREAT_MODEL.md T1's national_id note describes for the
-- DDL-literal passes and says nothing about for a row. No obfuscation is
-- required at all: this is the severe form of the 2026-09-15 round-1 red
-- team's A2, reduced to its minimum. The fix is two entries, one in each of
-- internal/classify/classify.go's validators list and
-- internal/verify/validators.go's, both reading textsig.ValidNationalID
-- (T-0187).
--
-- reg018_records itself carries no personal column beside `code`, on purpose:
-- A6's whole point is that the neighbouring-column rule (A2b, round 1) must
-- not be what saves it, so this file proves `code` is decided on its own
-- value signal and not by a `certain` neighbour raising it. The email address
-- the vacuity guard below needs lives in a second table instead, reached from
-- the root by a foreign key so the slice still loads it.

CREATE TABLE public.reg018_records (
    id    integer PRIMARY KEY,
    code  text NOT NULL
);

-- Gives assertTortureNoLiteralSurvives -- the leak check every `expect: ok`
-- file gets automatically, and which hard-fails when the source holds
-- neither an email address nor a phone number -- something to find, without
-- putting a `certain` personal column beside `code` in its own table.
CREATE TABLE public.reg018_contacts (
    id         integer PRIMARY KEY,
    record_id  integer NOT NULL REFERENCES public.reg018_records(id),
    email      text NOT NULL
);

INSERT INTO public.reg018_records (id, code) VALUES
    (1, '078-05-1001'),
    (2, '219-09-9999'),
    (3, '512-88-4321'),
    (4, '045-33-7788'),
    (5, '634-21-0099'),
    (6, '873-66-5500');

INSERT INTO public.reg018_contacts (id, record_id, email) VALUES
    (1, 1, 'grace.hopper1@realcorp.example'),
    (2, 2, 'grace.hopper2@realcorp.example'),
    (3, 3, 'grace.hopper3@realcorp.example'),
    (4, 4, 'grace.hopper4@realcorp.example'),
    (5, 5, 'grace.hopper5@realcorp.example'),
    (6, 6, 'grace.hopper6@realcorp.example');
