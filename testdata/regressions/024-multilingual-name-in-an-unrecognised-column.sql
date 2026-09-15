-- root:   public.reg024_records
-- take:   10
-- expect: ok
-- found:  the 2026-09-15 red team round two, R2-05/A10, and T-0188
-- why:    a person's full name in a language names.txt did not carry, in a
--         column called `label` whose name matches no rule, in a table with
--         no other personal column so the neighbouring-column rule cannot
--         fire -- reported "no name or value signal" and crossed into the
--         target verbatim under exit 0
-- not-copied: public.reg024_records.label
--
-- A10's own attack used Khmer, Lao and Amharic transliterations, and T-0188's
-- own concerns record that those three stay an open residual (tracker
-- T-0197): Wikidata's CC0 given/family-name coverage for them is three,
-- twelve and thirteen items respectively, nowhere near the "a few thousand
-- per language" the task set, so widening the dictionary cannot close that
-- instance -- the class A10's own finding names is unbounded by language
-- regardless of source. What T-0188 sourced instead is twenty languages
-- named by the task, and this file is A10's own shape reduced against ten of
-- them: a `label` column, no name-rule match, no neighbouring personal
-- column, one full name per row, each a given name immediately followed by a
-- surname that internal/textsig/names.txt did not carry before this task --
-- German, French, Spanish, Portuguese, Turkish, Hindi (romanised), Arabic
-- (romanised), Japanese (romanised), Korean (romanised) and Chinese
-- (romanised, pinyin). Verified both ways against a checkout of
-- internal/textsig/names.txt from before T-0188's change: every one of the
-- ten reads false through Dict.LooksLikeName there and true through it on
-- this task's names.txt, the same shape 018's own header verified
-- ValidNationalID against a scratch copy of textsig.
--
-- reg024_records carries `label` alone, on purpose (018's own note explains
-- why, and A10's own attack shape depends on it): the email address
-- assertTortureNoLiteralSurvives needs is in a second table, reached from
-- the root by a foreign key, so the neighbouring-column rule never sees a
-- `certain` column in reg024_records itself.

CREATE TABLE public.reg024_records (
    id     integer PRIMARY KEY,
    label  text NOT NULL
);

CREATE TABLE public.reg024_notes (
    id         integer PRIMARY KEY,
    record_id  integer NOT NULL REFERENCES public.reg024_records(id),
    email      text NOT NULL
);

INSERT INTO public.reg024_records (id, label) VALUES
    (1,  'Adolf Strauss'),
    (2,  'Mia Trudeau'),
    (3,  'Aida Hernández'),
    (4,  'Zita Araújo'),
    (5,  'Alina Kaşıkçı'),
    (6,  'Mallika Thapa'),
    (7,  'Abdullah Bushnak'),
    (8,  'Daisuke Satō'),
    (9,  'Jeong Hwang'),
    (10, 'Tíngtíng Gao');

INSERT INTO public.reg024_notes (id, record_id, email) VALUES
    (1,  1,  'grace.hopper1@realcorp.example'),
    (2,  2,  'grace.hopper2@realcorp.example'),
    (3,  3,  'grace.hopper3@realcorp.example'),
    (4,  4,  'grace.hopper4@realcorp.example'),
    (5,  5,  'grace.hopper5@realcorp.example'),
    (6,  6,  'grace.hopper6@realcorp.example'),
    (7,  7,  'grace.hopper7@realcorp.example'),
    (8,  8,  'grace.hopper8@realcorp.example'),
    (9,  9,  'grace.hopper9@realcorp.example'),
    (10, 10, 'grace.hopper10@realcorp.example');
