-- root:   public.reg046_members
-- take:   30
-- expect: ok
-- found:  the 2026-09-25 JSON red team, round 1 (docs/reviews/2026-09-25-redteam-json/round1.json, attacker 1's A12, A13 and A11), not a torture schema
-- why:    a jsonb column whose own name matched a personal rule that does not
--         accept jsonb was decided plain semi_structured by its type, so it
--         carried the per-leaf map, and every leaf with no signal of its own
--         -- a name, a street, a password, a date of birth -- was copied at
--         exit 0
-- not-copied: public.reg046_members.full_name, public.reg046_members.home_address, public.reg046_members.passwords, public.reg046_members.date_of_birth, public.reg046_members.national_id, public.reg046_members.emails, public.reg046_members.notes, public.reg046_members.by_phone, public.reg046_members.medical_history
--
-- Not a reduction of a torture-schema failure, on 010's precedent: the red
-- team's own probe schemas are already the smallest that fail, and this file
-- is their three attempts in one table.
--
-- What happened. Only special_category (accepts "*") and semi_structured
-- accept a document in rules.yml, so a jsonb column named full_name
-- (person_name), home_address (address), passwords (credential),
-- date_of_birth (person_date), national_id, emails (email), notes (free_text)
-- or by_phone (phone) takes decide's `hasName && !nameAccepted` branch and is
-- decided by its type, as 008 made it: semi_structured, possible, masked. That
-- decision was the same one a jsonb nothing names gets, so
-- pipeline.Decision.LeafMap handed internal/transform the per-leaf map
-- (T-0272), every key here is one no name rule names, no value validator
-- recognises any leaf, and every leaf was copied. SECURITY.md item 8 and
-- THREAT_MODEL.md T1's T-0272 amendment both said a column whose own name
-- marks it personal has every leaf masked; only special_category reached that
-- path.
--
-- The fix (T-0393). The fallback decision carries the name's category
-- (pipeline.Decision.NameHit), LeafMap is nil for it, and every leaf is
-- masked under that category through the leaf masker: the email masker for
-- emails, the phone masker for by_phone, the address, national_id and
-- credential maskers for theirs, and free_text for full_name, date_of_birth
-- and notes, whose own maskers a leaf does not take. internal/verify's second
-- net reads the same rule, so it leaves the category maskers' output to the
-- residual scan and still reads the free_text filler.
--
-- The documents are signal-free on purpose, which is what makes `not-copied:`
-- the check: that key greps the target for each whole source document, and
-- before the fix every one of them crossed whole. One masked leaf is enough to
-- change a document's text, so the unit tests in internal/classify
-- (t0393_test.go), internal/transform (leaf_test.go) and internal/verify
-- (jsonleaf_test.go) are what pin every leaf. The red team's by_phone
-- documents used phone numbers as object keys; that is a key-masking question
-- (item 8's residual, and a region the key masker does not yet read) and is
-- left out of this file so it tests the leaf rule alone.
--
-- medical_history is the control: special_category accepts jsonb, so its
-- decision was special_category and every leaf was already masked before the
-- fix, and must stay so. email is a plain text column of addresses, masked
-- before and after, and is here for the harness: its I2 grep refuses a source
-- that holds no email address or phone number, and the documents must hold
-- none, or a validator would mask a leaf before the fix and `not-copied:`
-- would stop seeing the bug.

CREATE TABLE public.reg046_members (
    id              integer PRIMARY KEY,
    full_name       jsonb NOT NULL,
    home_address    jsonb NOT NULL,
    passwords       jsonb NOT NULL,
    date_of_birth   jsonb NOT NULL,
    national_id     jsonb NOT NULL,
    emails          jsonb NOT NULL,
    notes           jsonb NOT NULL,
    by_phone        jsonb NOT NULL,
    medical_history jsonb NOT NULL,
    email           text NOT NULL
);

INSERT INTO public.reg046_members
    (id, full_name, home_address, passwords, date_of_birth, national_id, emails, notes, by_phone, medical_history, email)
SELECT g,
       jsonb_build_object('given', 'Wren', 'family', 'Calloway'),
       jsonb_build_object('a', 'Larkspur Row', 'b', 'Upper Dunmore', 'c', 'DN7'),
       jsonb_build_object('current', 'hunter2x', 'previous', 'tr0ub4dor'),
       jsonb_build_object('d', '1987-03-14'),
       jsonb_build_object('scheme', 'UK-NINO', 'holder', 'Wren Calloway'),
       jsonb_build_object('owner', 'Wren Calloway'),
       jsonb_build_object('entry', 'Wren Calloway seen for follow-up'),
       jsonb_build_object('label', 'Ysolde Brackenridge'),
       jsonb_build_object('entry', 'Wren Calloway seen for follow-up'),
       'wren.calloway' || g || '@regression.test'
FROM generate_series(1, 30) AS g;
