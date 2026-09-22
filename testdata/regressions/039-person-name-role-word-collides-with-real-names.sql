-- root:       public.reg039_people
-- take:       12
-- expect:     ok
-- not-copied: public.reg039_people.first_name, public.reg039_people.last_name, public.reg039_people.full_name, public.reg039_people.email
-- found:      the T-0287 fix-round review (high finding 1): RoleGiven's and
--             RoleFamily's own word lists (mask/words.go, added by that
--             round's own earlier fix) were drawn from a synthetic
--             consonant-vowel vocabulary the round's own comment called
--             disjoint from real names "by construction", but a review
--             measured that 34 of the 1,800 tokens across both 900-word
--             lists were literal entries of internal/textsig/names.txt (siri,
--             nero, leni, sibel and thirty more), and materially more again
--             -- gale, sage, mari, boris, titus among them -- were ordinary
--             common given names and surnames the dictionary does not carry
--             at all. The residual scan checks every masked value against
--             the whole source column, so a first_name or last_name column
--             of a few thousand real rows had a real chance of the residual
--             scan's own confirmed-hit refusal (exit 9,
--             verify.refused.residual): a masked value equal to some *other*
--             row's real value in the same column, not a passthrough, an
--             ordinary coincidence over a small alphabet of real names.
-- why:        mask/words.go's roleGivenWords and roleFamilyWords are now
--             filtered against internal/textsig/names.txt, a larger
--             census-style given-name/surname corpus and the system English
--             dictionary (mask/CLAUDE.md's T-0287 section has the full
--             account), so neither list carries a token that is also a real
--             given name, surname or ordinary word by any of those checks.
--             This file seeds first_name and last_name with exactly the
--             common real names the review found colliding -- gale, sage,
--             mari, rani, sade, boris, titus, kota, mako and more, mixed
--             across both columns -- so a reintroduced overlap fails the
--             residual scan here under `make torture` rather than on a
--             stranger's production users table. mask/role_test.go's
--             TestRoleWordsExcludeKnownRealNames pins the same tokens
--             against the word lists directly, at the unit level, with no
--             database needed; this file is the end-to-end proof that a
--             source column actually holding them still loads clean.

CREATE TABLE public.reg039_people (
    person_id  bigint PRIMARY KEY,
    first_name text NOT NULL,
    last_name  text NOT NULL,
    full_name  text NOT NULL,
    email      text NOT NULL
);

INSERT INTO public.reg039_people (person_id, first_name, last_name, full_name, email) VALUES
    (1,  'Gale',  'Boris', 'Gale Boris',  'gale.boris1@realcorp.example'),
    (2,  'Sage',  'Titus', 'Sage Titus',  'sage.titus2@realcorp.example'),
    (3,  'Mari',  'Kota',  'Mari Kota',   'mari.kota3@realcorp.example'),
    (4,  'Rani',  'Mako',  'Rani Mako',   'rani.mako4@realcorp.example'),
    (5,  'Sade',  'Juli',  'Sade Juli',   'sade.juli5@realcorp.example'),
    (6,  'Bela',  'Mati',  'Bela Mati',   'bela.mati6@realcorp.example'),
    (7,  'Manu',  'Koda',  'Manu Koda',   'manu.koda7@realcorp.example'),
    (8,  'Nori',  'Tani',  'Nori Tani',   'nori.tani8@realcorp.example'),
    (9,  'Levon', 'Sama',  'Levon Sama',  'levon.sama9@realcorp.example'),
    (10, 'Daven', 'Nuno',  'Daven Nuno',  'daven.nuno10@realcorp.example'),
    (11, 'Lorin', 'Runo',  'Lorin Runo',  'lorin.runo11@realcorp.example'),
    (12, 'Deron', 'Gema',  'Deron Gema',  'deron.gema12@realcorp.example');
