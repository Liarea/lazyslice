-- root:       public.reg041_people
-- take:       60
-- expect:     ok
-- found:      ADR-015 ("Consequences") and T-0304, not a torture schema:
--             once a masked first_name and last_name are real Census
--             names, a column generated from them is a real given name
--             followed by a real surname in every row of the target --
--             the exact shape internal/verify's second net refuses at
--             exit 9 (verify.refused.second_net) as a person's name left
--             in cleartext, over a run that masked every name correctly.
-- why:        full_name is GENERATED ALWAYS AS (first_name || ' ' ||
--             last_name) STORED, so the target recomputes it from the two
--             masked columns and lazyslice never copies or masks it. The
--             second net skips its dictionary rule for a generated column
--             whose expression names only masked columns of its own table
--             (secondnet.go's generatedFromMasked; every validator that
--             carries a parse still runs over it), and the run must load
--             at exit 0. first_name and last_name are Census names
--             themselves, so their own coincidences exercise
--             verify.residual.explained in the same run. email is
--             there because the harness's leak check needs the source to
--             hold an address it can prove absent from the target.

CREATE TABLE public.reg041_people (
    person_id  bigint PRIMARY KEY,
    first_name text NOT NULL,
    last_name  text NOT NULL,
    full_name  text GENERATED ALWAYS AS (first_name || ' ' || last_name) STORED,
    email      text NOT NULL
);

INSERT INTO public.reg041_people (person_id, first_name, last_name, email) VALUES
    ( 1, 'Aidan', 'Andrews', 'aidan.andrews1@realcorp.example'),
    ( 2, 'Aubrey', 'Booker', 'aubrey.booker2@realcorp.example'),
    ( 3, 'Cameron', 'Chandler', 'cameron.chandler3@realcorp.example'),
    ( 4, 'Cory', 'Delacruz', 'cory.delacruz4@realcorp.example'),
    ( 5, 'Dylan', 'Francis', 'dylan.francis5@realcorp.example'),
    ( 6, 'Gabrielle', 'Han', 'gabrielle.han6@realcorp.example'),
    ( 7, 'Isabel', 'Hunter', 'isabel.hunter7@realcorp.example'),
    ( 8, 'Joey', 'Li', 'joey.li8@realcorp.example'),
    ( 9, 'Kent', 'Mckee', 'kent.mckee9@realcorp.example'),
    (10, 'Luke', 'Nieves', 'luke.nieves10@realcorp.example'),
    (11, 'Mia', 'Powell', 'mia.powell11@realcorp.example'),
    (12, 'Penelope', 'Salinas', 'penelope.salinas12@realcorp.example'),
    (13, 'Sabrina', 'Stout', 'sabrina.stout13@realcorp.example'),
    (14, 'Tanya', 'Warren', 'tanya.warren14@realcorp.example'),
    (15, 'Yesenia', 'Alvarado', 'yesenia.alvarado15@realcorp.example'),
    (16, 'Ann', 'Blanchard', 'ann.blanchard16@realcorp.example'),
    (17, 'Brett', 'Castaneda', 'brett.castaneda17@realcorp.example'),
    (18, 'Clarence', 'Davis', 'clarence.davis18@realcorp.example'),
    (19, 'Diego', 'Flynn', 'diego.flynn19@realcorp.example'),
    (20, 'Everett', 'Hale', 'everett.hale20@realcorp.example'),
    (21, 'Hayden', 'Huff', 'hayden.huff21@realcorp.example'),
    (22, 'Jerry', 'Lee', 'jerry.lee22@realcorp.example'),
    (23, 'Kathleen', 'Mcfarland', 'kathleen.mcfarland23@realcorp.example'),
    (24, 'Linda', 'Newton', 'linda.newton24@realcorp.example'),
    (25, 'Mateo', 'Poole', 'mateo.poole25@realcorp.example'),
    (26, 'Olga', 'Russell', 'olga.russell26@realcorp.example'),
    (27, 'Roger', 'Stephenson', 'roger.stephenson27@realcorp.example'),
    (28, 'Spencer', 'Walters', 'spencer.walters28@realcorp.example'),
    (29, 'Victoria', 'Ahmed', 'victoria.ahmed29@realcorp.example'),
    (30, 'Alma', 'Bishop', 'alma.bishop30@realcorp.example'),
    (31, 'Billy', 'Carrillo', 'billy.carrillo31@realcorp.example'),
    (32, 'Chad', 'Daniel', 'chad.daniel32@realcorp.example'),
    (33, 'Dave', 'Fitzpatrick', 'dave.fitzpatrick33@realcorp.example'),
    (34, 'Emilio', 'Guerra', 'emilio.guerra34@realcorp.example'),
    (35, 'Gracie', 'Howell', 'gracie.howell35@realcorp.example'),
    (36, 'Jared', 'Lawrence', 'jared.lawrence36@realcorp.example'),
    (37, 'Julian', 'Mccormick', 'julian.mccormick37@realcorp.example'),
    (38, 'Layla', 'Nash', 'layla.nash38@realcorp.example'),
    (39, 'Marie', 'Pierce', 'marie.pierce39@realcorp.example'),
    (40, 'Natalia', 'Roth', 'natalia.roth40@realcorp.example'),
    (41, 'Regina', 'Stanley', 'regina.stanley41@realcorp.example'),
    (42, 'Shelby', 'Walker', 'shelby.walker42@realcorp.example'),
    (43, 'Trent', 'Acevedo', 'trent.acevedo43@realcorp.example'),
    (44, 'Alec', 'Berg', 'alec.berg44@realcorp.example'),
    (45, 'Ayden', 'Cardenas', 'ayden.cardenas45@realcorp.example'),
    (46, 'Carlos', 'Cummings', 'carlos.cummings46@realcorp.example'),
    (47, 'Cynthia', 'Fields', 'cynthia.fields47@realcorp.example'),
    (48, 'Eduardo', 'Greer', 'eduardo.greer48@realcorp.example'),
    (49, 'Gene', 'Horne', 'gene.horne49@realcorp.example'),
    (50, 'Ivy', 'Landry', 'ivy.landry50@realcorp.example'),
    (51, 'Jonah', 'Mccall', 'jonah.mccall51@realcorp.example'),
    (52, 'Krista', 'Mullins', 'krista.mullins52@realcorp.example'),
    (53, 'Madeline', 'Petersen', 'madeline.petersen53@realcorp.example'),
    (54, 'Mikayla', 'Romero', 'mikayla.romero54@realcorp.example'),
    (55, 'Phyllis', 'Soto', 'phyllis.soto55@realcorp.example'),
    (56, 'Sandra', 'Villegas', 'sandra.villegas56@realcorp.example'),
    (57, 'Terri', 'Zheng', 'terri.zheng57@realcorp.example'),
    (58, 'Zoe', 'Benitez', 'zoe.benitez58@realcorp.example'),
    (59, 'April', 'Cameron', 'april.cameron59@realcorp.example'),
    (60, 'Brody', 'Crane', 'brody.crane60@realcorp.example');
