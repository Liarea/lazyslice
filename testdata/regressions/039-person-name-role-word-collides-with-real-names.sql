-- root:       public.reg039_people
-- take:       100
-- expect:     ok
-- not-copied: public.reg039_people.email
-- found:      the T-0287 fix-round review (high finding 1), and inverted
--             since: RoleGiven's and RoleFamily's word lists were then
--             synthetic consonant-vowel tokens, meant to hold no real
--             name, because the residual scan confirmed a masked value
--             equal to ANY real value of the same source column and
--             refused the run at exit 9 (verify.refused.residual). The
--             review found real names among the tokens -- gale, sage,
--             mari, boris, titus and more -- and this file was written to
--             seed exactly those so that an overlap would refuse here.
--             ADR-015 (T-0302) then taught the residual scan to explain a
--             coincidence inside the masker's own vocabulary, and T-0304
--             made the vocabulary real names: the 2020 Census top given
--             names and surnames (mask/words_corpus.go), for every role
--             and for email local parts.
-- why:        a column of real names masked to real names is now the
--             ordinary case, and this file proves ADR-015's verify rule
--             end to end over it. Rows 1-12 keep the names the review
--             found (none is on the Census lists); rows 13-100 are Census
--             names, so under any key some masked first_name and
--             last_name values equal another row's real value in the same
--             column, and the run must still load at exit 0 with
--             verify.residual.explained counting them: the list contains
--             each, transform emitted every copy, and the row check by
--             primary key finds no row that kept its own. full_name is
--             RoleFull (a Census pair) and email a Census given.surname
--             local part under example.com/.net/.org. `not-copied:` names
--             only email: every source address carries a digit and
--             realcorp.example, so none can equal a masked one, where a
--             masked first_name, last_name or full_name equal to another
--             row's real one is the correct outcome and that key greps the
--             whole target for every source value.

CREATE TABLE public.reg039_people (
    person_id  bigint PRIMARY KEY,
    first_name text NOT NULL,
    last_name  text NOT NULL,
    full_name  text NOT NULL,
    email      text NOT NULL
);

INSERT INTO public.reg039_people (person_id, first_name, last_name, full_name, email) VALUES
    (  1, 'Gale', 'Boris', 'Gale Boris', 'gale.boris1@realcorp.example'),
    (  2, 'Sage', 'Titus', 'Sage Titus', 'sage.titus2@realcorp.example'),
    (  3, 'Mari', 'Kota', 'Mari Kota', 'mari.kota3@realcorp.example'),
    (  4, 'Rani', 'Mako', 'Rani Mako', 'rani.mako4@realcorp.example'),
    (  5, 'Sade', 'Juli', 'Sade Juli', 'sade.juli5@realcorp.example'),
    (  6, 'Bela', 'Mati', 'Bela Mati', 'bela.mati6@realcorp.example'),
    (  7, 'Manu', 'Koda', 'Manu Koda', 'manu.koda7@realcorp.example'),
    (  8, 'Nori', 'Tani', 'Nori Tani', 'nori.tani8@realcorp.example'),
    (  9, 'Levon', 'Sama', 'Levon Sama', 'levon.sama9@realcorp.example'),
    ( 10, 'Daven', 'Nuno', 'Daven Nuno', 'daven.nuno10@realcorp.example'),
    ( 11, 'Lorin', 'Runo', 'Lorin Runo', 'lorin.runo11@realcorp.example'),
    ( 12, 'Deron', 'Gema', 'Deron Gema', 'deron.gema12@realcorp.example'),
    ( 13, 'Abel', 'Ahmed', 'Abel Ahmed', 'abel.ahmed13@realcorp.example'),
    ( 14, 'Amy', 'Beasley', 'Amy Beasley', 'amy.beasley14@realcorp.example'),
    ( 15, 'Ayden', 'Bruce', 'Ayden Bruce', 'ayden.bruce15@realcorp.example'),
    ( 16, 'Brian', 'Chavez', 'Brian Chavez', 'brian.chavez16@realcorp.example'),
    ( 17, 'Cathy', 'Dang', 'Cathy Dang', 'cathy.dang17@realcorp.example'),
    ( 18, 'Constance', 'Espinoza', 'Constance Espinoza', 'constance.espinoza18@realcorp.example'),
    ( 19, 'Dennis', 'Garrett', 'Dennis Garrett', 'dennis.garrett19@realcorp.example'),
    ( 20, 'Elaine', 'Hanson', 'Elaine Hanson', 'elaine.hanson20@realcorp.example'),
    ( 21, 'Felipe', 'Howard', 'Felipe Howard', 'felipe.howard21@realcorp.example'),
    ( 22, 'Grace', 'Kirby', 'Grace Kirby', 'grace.kirby22@realcorp.example'),
    ( 23, 'Isabel', 'Lynn', 'Isabel Lynn', 'isabel.lynn23@realcorp.example'),
    ( 24, 'Jenna', 'Mclaughlin', 'Jenna Mclaughlin', 'jenna.mclaughlin24@realcorp.example'),
    ( 25, 'Josh', 'Murphy', 'Josh Murphy', 'josh.murphy25@realcorp.example'),
    ( 26, 'Keith', 'Patrick', 'Keith Patrick', 'keith.patrick26@realcorp.example'),
    ( 27, 'Lena', 'Raymond', 'Lena Raymond', 'lena.raymond27@realcorp.example'),
    ( 28, 'Lynn', 'Sanchez', 'Lynn Sanchez', 'lynn.sanchez28@realcorp.example'),
    ( 29, 'Maryann', 'Spears', 'Maryann Spears', 'maryann.spears29@realcorp.example'),
    ( 30, 'Molly', 'Vance', 'Molly Vance', 'molly.vance30@realcorp.example'),
    ( 31, 'Patsy', 'Wiggins', 'Patsy Wiggins', 'patsy.wiggins31@realcorp.example'),
    ( 32, 'Rick', 'Alvarez', 'Rick Alvarez', 'rick.alvarez32@realcorp.example'),
    ( 33, 'Sandy', 'Benjamin', 'Sandy Benjamin', 'sandy.benjamin33@realcorp.example'),
    ( 34, 'Stacy', 'Bullock', 'Stacy Bullock', 'stacy.bullock34@realcorp.example'),
    ( 35, 'Tonya', 'Chung', 'Tonya Chung', 'tonya.chung35@realcorp.example'),
    ( 36, 'Wilma', 'Davis', 'Wilma Davis', 'wilma.davis36@realcorp.example'),
    ( 37, 'Alfredo', 'Farmer', 'Alfredo Farmer', 'alfredo.farmer37@realcorp.example'),
    ( 38, 'Arlene', 'Gibson', 'Arlene Gibson', 'arlene.gibson38@realcorp.example'),
    ( 39, 'Bonnie', 'Harrington', 'Bonnie Harrington', 'bonnie.harrington39@realcorp.example'),
    ( 40, 'Carlos', 'Huerta', 'Carlos Huerta', 'carlos.huerta40@realcorp.example'),
    ( 41, 'Claudia', 'Koch', 'Claudia Koch', 'claudia.koch41@realcorp.example'),
    ( 42, 'Darlene', 'Magana', 'Darlene Magana', 'darlene.magana42@realcorp.example'),
    ( 43, 'Duane', 'Medrano', 'Duane Medrano', 'duane.medrano43@realcorp.example'),
    ( 44, 'Erin', 'Nelson', 'Erin Nelson', 'erin.nelson44@realcorp.example'),
    ( 45, 'George', 'Pena', 'George Pena', 'george.pena45@realcorp.example'),
    ( 46, 'Helen', 'Reynolds', 'Helen Reynolds', 'helen.reynolds46@realcorp.example'),
    ( 47, 'Janice', 'Saunders', 'Janice Saunders', 'janice.saunders47@realcorp.example'),
    ( 48, 'Joey', 'Steele', 'Joey Steele', 'joey.steele48@realcorp.example'),
    ( 49, 'Karina', 'Vega', 'Karina Vega', 'karina.vega49@realcorp.example'),
    ( 50, 'Krystal', 'Williamson', 'Krystal Williamson', 'krystal.williamson50@realcorp.example'),
    ( 51, 'Lorenzo', 'Arellano', 'Lorenzo Arellano', 'lorenzo.arellano51@realcorp.example'),
    ( 52, 'Mario', 'Bernal', 'Mario Bernal', 'mario.bernal52@realcorp.example'),
    ( 53, 'Michele', 'Bush', 'Michele Bush', 'michele.bush53@realcorp.example'),
    ( 54, 'Norma', 'Cline', 'Norma Cline', 'norma.cline54@realcorp.example'),
    ( 55, 'Raquel', 'Delarosa', 'Raquel Delarosa', 'raquel.delarosa55@realcorp.example'),
    ( 56, 'Ruben', 'Figueroa', 'Ruben Figueroa', 'ruben.figueroa56@realcorp.example'),
    ( 57, 'Shirley', 'Glenn', 'Shirley Glenn', 'shirley.glenn57@realcorp.example'),
    ( 58, 'Terry', 'Hayden', 'Terry Hayden', 'terry.hayden58@realcorp.example'),
    ( 59, 'Violet', 'Hunter', 'Violet Hunter', 'violet.hunter59@realcorp.example'),
    ( 60, 'Alan', 'Landry', 'Alan Landry', 'alan.landry60@realcorp.example'),
    ( 61, 'Angelo', 'Marks', 'Angelo Marks', 'angelo.marks61@realcorp.example'),
    ( 62, 'Bernard', 'Merritt', 'Bernard Merritt', 'bernard.merritt62@realcorp.example'),
    ( 63, 'Bryce', 'Nielsen', 'Bryce Nielsen', 'bryce.nielsen63@realcorp.example'),
    ( 64, 'Chase', 'Peters', 'Chase Peters', 'chase.peters64@realcorp.example'),
    ( 65, 'Cynthia', 'Richmond', 'Cynthia Richmond', 'cynthia.richmond65@realcorp.example'),
    ( 66, 'Dillon', 'Schroeder', 'Dillon Schroeder', 'dillon.schroeder66@realcorp.example'),
    ( 67, 'Ellie', 'Stokes', 'Ellie Stokes', 'ellie.stokes67@realcorp.example'),
    ( 68, 'Frederick', 'Villanueva', 'Frederick Villanueva', 'frederick.villanueva68@realcorp.example'),
    ( 69, 'Gwendolyn', 'Wong', 'Gwendolyn Wong', 'gwendolyn.wong69@realcorp.example'),
    ( 70, 'Jackson', 'Atkinson', 'Jackson Atkinson', 'jackson.atkinson70@realcorp.example'),
    ( 71, 'Jill', 'Blackwell', 'Jill Blackwell', 'jill.blackwell71@realcorp.example'),
    ( 72, 'Julia', 'Calhoun', 'Julia Calhoun', 'julia.calhoun72@realcorp.example'),
    ( 73, 'Kent', 'Collier', 'Kent Collier', 'kent.collier73@realcorp.example'),
    ( 74, 'Liam', 'Dixon', 'Liam Dixon', 'liam.dixon74@realcorp.example'),
    ( 75, 'Marcia', 'Fletcher', 'Marcia Fletcher', 'marcia.fletcher75@realcorp.example'),
    ( 76, 'Mayra', 'Goodman', 'Mayra Goodman', 'mayra.goodman76@realcorp.example'),
    ( 77, 'Nathan', 'Henry', 'Nathan Henry', 'nathan.henry77@realcorp.example'),
    ( 78, 'Peyton', 'Jackson', 'Peyton Jackson', 'peyton.jackson78@realcorp.example'),
    ( 79, 'Rodrigo', 'Lawson', 'Rodrigo Lawson', 'rodrigo.lawson79@realcorp.example'),
    ( 80, 'Sergio', 'Massey', 'Sergio Massey', 'sergio.massey80@realcorp.example'),
    ( 81, 'Suzanne', 'Miller', 'Suzanne Miller', 'suzanne.miller81@realcorp.example'),
    ( 82, 'Tyler', 'Norton', 'Tyler Norton', 'tyler.norton82@realcorp.example'),
    ( 83, 'Zoey', 'Pierce', 'Zoey Pierce', 'zoey.pierce83@realcorp.example'),
    ( 84, 'Alyssa', 'Robbins', 'Alyssa Robbins', 'alyssa.robbins84@realcorp.example'),
    ( 85, 'Austin', 'Shaffer', 'Austin Shaffer', 'austin.shaffer85@realcorp.example'),
    ( 86, 'Brenda', 'Sullivan', 'Brenda Sullivan', 'brenda.sullivan86@realcorp.example'),
    ( 87, 'Casey', 'Wagner', 'Casey Wagner', 'casey.wagner87@realcorp.example'),
    ( 88, 'Colton', 'Wyatt', 'Colton Wyatt', 'colton.wyatt88@realcorp.example'),
    ( 89, 'Deborah', 'Bailey', 'Deborah Bailey', 'deborah.bailey89@realcorp.example'),
    ( 90, 'Eduardo', 'Bond', 'Eduardo Bond', 'eduardo.bond90@realcorp.example'),
    ( 91, 'Ezra', 'Cano', 'Ezra Cano', 'ezra.cano91@realcorp.example'),
    ( 92, 'Glenda', 'Conrad', 'Glenda Conrad', 'glenda.conrad92@realcorp.example'),
    ( 93, 'Irene', 'Dorsey', 'Irene Dorsey', 'irene.dorsey93@realcorp.example'),
    ( 94, 'Jeanne', 'Foster', 'Jeanne Foster', 'jeanne.foster94@realcorp.example'),
    ( 95, 'Jorge', 'Gray', 'Jorge Gray', 'jorge.gray95@realcorp.example'),
    ( 96, 'Kay', 'Hess', 'Kay Hess', 'kay.hess96@realcorp.example'),
    ( 97, 'Lawrence', 'Jennings', 'Lawrence Jennings', 'lawrence.jennings97@realcorp.example'),
    ( 98, 'Luke', 'Leonard', 'Luke Leonard', 'luke.leonard98@realcorp.example'),
    ( 99, 'Martha', 'Mayer', 'Martha Mayer', 'martha.mayer99@realcorp.example'),
    (100, 'Mitchell', 'Montes', 'Mitchell Montes', 'mitchell.montes100@realcorp.example');
