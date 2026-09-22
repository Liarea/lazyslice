-- root:       public.reg038_people
-- take:       5
-- expect:     ok
-- not-copied: public.reg038_people.first_name, public.reg038_people.last_name, public.reg038_people.full_name, public.reg038_people.email
-- found:      docs/media/first-run.gif's own last frame (2026-09-22): the
--             README's landing image read back public.customer.first_name
--             as "Emma Popescu" and last_name as "Oscar Adler" -- a
--             first-name column carrying a surname and a last-name column
--             carrying a whole name, visible to a stranger in the first ten
--             seconds
-- why:        person_name's masker (mask/gen_text.go) ignored the column it
--             was asked to fill and always emitted "Given Family": right for
--             a full_name column and wrong for first_name and last_name,
--             which held two words and a stray surname respectively.
--             ARCHITECTURE.md §5's own worked example never named the shape
--             because nothing before T-0287 carried a role past Category --
--             fixed by mask.Role (RoleGiven, RoleFamily, RoleFull), decided
--             at classify time from the column's own name
--             (internal/classify/classify.go's roleForColumn) and carried on
--             Decision.Role beside Category the way UniqueIndex reaches
--             mask.Constraints.Unique: internal/transform sets
--             Constraints.Role from the Decision, and
--             personNameMasker.Mask/.Domain branch on it -- a given name
--             only for RoleGiven, a surname only for RoleFamily, the
--             unchanged "Given Family" pair for RoleFull (the zero value, so
--             a caller built before this field existed sees no change).
--             Three columns here pin the three roles side by side: a stray
--             two-word "Ada Lovelace" masked into first_name, or a bare
--             surname into last_name, would still pass `not-copied:` (it is
--             not the *source's* value) but is exactly the defect this file
--             exists to keep out -- mask/role_test.go's
--             TestPersonNameRoleShape is the unit proof that first_name's
--             masked value is one word off the given-name list and
--             last_name's is one word off the surname list, and
--             internal/classify/role_test.go's
--             TestPersonNameRoleFromColumnName is the classify-time half:
--             "first_name"/"given_name"/"forename"/"fname" decide
--             mask.RoleGiven, "last_name"/"family_name"/"surname"/"lname"
--             decide mask.RoleFamily, and everything else -- "name",
--             "full_name" among them -- keeps mask.RoleFull. `email` carries
--             a real address so I2's own leak scan (every `expect: ok`
--             regression) has something a source literal could survive as,
--             the way every other file here does. Neither unit proof above
--             drives a value through the whole path this file exercises --
--             classify decides the Role, internal/transform's own plan()
--             carries it from Decision.Role onto mask.Constraints.Role, and
--             only then does the masker read it -- so deleting transform.go's
--             `plans[i].shape.constraints.Role = d.Role`, which drops every
--             person_name column back to RoleFull, would still pass this
--             file (`not-copied:` says nothing about shape) and
--             `make torture`. internal/transform's own
--             TestPersonNameRoleReachesTheMasker
--             (internal/transform/transform_test.go) closes that: it runs a
--             first_name/last_name/full_name batch through transformer.plan
--             and Mask together and asserts one word, one word and two
--             words, so that same deletion fails `make check` directly.

CREATE TABLE public.reg038_people (
    person_id  bigint PRIMARY KEY,
    first_name text NOT NULL,
    last_name  text NOT NULL,
    full_name  text NOT NULL,
    email      text NOT NULL
);

INSERT INTO public.reg038_people (person_id, first_name, last_name, full_name, email) VALUES
    (1, 'Margaret', 'Hamilton', 'Margaret Hamilton', 'margaret.hamilton1@realcorp.example'),
    (2, 'Grace', 'Hopper', 'Grace Hopper', 'grace.hopper2@realcorp.example'),
    (3, 'Katherine', 'Johnson', 'Katherine Johnson', 'katherine.johnson3@realcorp.example'),
    (4, 'Ada', 'Lovelace', 'Ada Lovelace', 'ada.lovelace4@realcorp.example'),
    (5, 'Hedy', 'Lamarr', 'Hedy Lamarr', 'hedy.lamarr5@realcorp.example');
