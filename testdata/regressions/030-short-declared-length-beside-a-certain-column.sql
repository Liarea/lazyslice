-- root:   public.reg030_members
-- take:   10
-- expect: ok
-- found:  the 2026-09-15 red team round 4, the native-script variant against
--         the A2b rail itself
-- why:    unknownColumnsBesideCertain masks an unrecognised character column
--         as free_text beside a `certain` personal column, but its own
--         declared-length exclusion (minUnknownLen, sixteen) skipped a
--         varchar(12) column, so a name column of that length beside a real
--         email column copied verbatim under exit 0
-- not-copied: public.reg030_members.ስም
--
-- reg030_members is the exact table shape unknownColumnsBesideCertain was
-- built for: a character column with no name-rule match, no value-validator
-- hit and no type signal (`ስም`, Amharic for "name" -- a column name no rule
-- pack pattern reads either), sitting beside `email`, a `certain` column in
-- the *same* table. Before T-0239 the rail still refused to raise `ስም`
-- because its declared length, twelve, was under the rail's own floor of
-- sixteen -- the exclusion's own comment argued "free_text's filler does not
-- fit them", which was never true of a *non-unique* column (free_text's
-- generator fits any declared length down to one byte; the d_required domain
-- rule that can refuse a narrow column applies only under a unique index, and
-- this rail already excludes those). A short declared length is not evidence
-- the column is impersonal: a given name, a surname, a postcode, a national
-- ID and a phone number all fit in twelve characters, and an attacker -- or
-- an ordinary schema author -- picks the length. T-0239 lowered the floor to
-- two characters (nothing shorter can hold even a two-letter code) and left
-- the separate, value-based two-letter-code exclusion untouched.
--
-- Declared varchar(12) on purpose, matching the round-4 attack's own
-- reproduction; the control the attack ran alongside it (the identical
-- schema and values with the column declared `text` instead) was already
-- masked before this fix and is pinned at the unit level instead
-- (TestRedTeamRound4A2bShortDeclaredLengthBesideCertain,
-- internal/classify/redteam_test.go), since a torture regression cannot hold
-- two schemas in one run.

CREATE TABLE public.reg030_members (
    id     integer PRIMARY KEY,
    "ስም"  varchar(12),
    email  text NOT NULL
);

INSERT INTO public.reg030_members (id, "ስም", email) VALUES
    (1,  'ኣበበ ኪዳነ',   'user1@realcorp.example'),
    (2,  'ተስፋዬ ኪዳነ',  'user2@realcorp.example'),
    (3,  'ገብረ ኣበበ',   'user3@realcorp.example'),
    (4,  'ኪዳነ ተስፋዬ',  'user4@realcorp.example'),
    (5,  'ኣበበ ገብረ',   'user5@realcorp.example'),
    (6,  'ኣበበ ኪዳነ',   'user6@realcorp.example'),
    (7,  'ተስፋዬ ኪዳነ',  'user7@realcorp.example'),
    (8,  'ገብረ ኣበበ',   'user8@realcorp.example'),
    (9,  'ኪዳነ ተስፋዬ',  'user9@realcorp.example'),
    (10, 'ኣበበ ገብረ',   'user10@realcorp.example');
