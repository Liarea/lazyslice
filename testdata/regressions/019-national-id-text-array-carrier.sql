-- root:   public.reg019_audit_docs
-- take:   5
-- expect: ok
-- found:  the 2026-09-15 red team round 2, R2-03/A7
-- why:    the same missing national_id validator (018's own why) reached
--         through the array carrier: a `text[]` of UK National Insurance
--         numbers reached the target verbatim -- the array machinery split
--         the elements correctly and offered each to the validators, there
--         was simply no national_id validator to hit
-- not-copied: public.reg019_audit_docs.id_list
--
-- What happened. A7's own text also planted a plain SSN in a column literally
-- named `code` beside this array (018 is that half, reduced to its own file
-- so each carrier has one regression to itself) and a JSON document whose only
-- personal datum was an SSN at depth 3, which is not reproduced here because
-- 013's json-object-key regression already exercises the JSON leaf path this
-- fix reuses without change -- internal/classify's leaf signal and
-- internal/verify's document walk both ask the same ordered validators list
-- about a leaf that a national_id entry now answers for like any other.
-- What this file is for is the array path specifically: internal/classify's
-- scalarsOf and internal/verify's array handling both split the literal
-- correctly before T-0187 landed and still validated nothing, because the
-- element strings themselves reached no national_id validator on either net.
--
-- reg019_audit_docs carries `id_list` alone, on purpose (018's own note
-- explains why): the email address assertTortureNoLiteralSurvives needs is in
-- a second table, reached from the root by a foreign key.

CREATE TABLE public.reg019_audit_docs (
    id       integer PRIMARY KEY,
    id_list  text[] NOT NULL
);

CREATE TABLE public.reg019_notes (
    id       integer PRIMARY KEY,
    doc_id   integer NOT NULL REFERENCES public.reg019_audit_docs(id),
    email    text NOT NULL
);

INSERT INTO public.reg019_audit_docs (id, id_list) VALUES
    (1, ARRAY['AB123456D', 'CE234567A']),
    (2, ARRAY['GH345678B']),
    (3, ARRAY['JK456789C']),
    (4, ARRAY['LM567890D']),
    (5, ARRAY['NP112233B']);

INSERT INTO public.reg019_notes (id, doc_id, email) VALUES
    (1, 1, 'ada.lovelace1@realcorp.example'),
    (2, 2, 'ada.lovelace2@realcorp.example'),
    (3, 3, 'ada.lovelace3@realcorp.example'),
    (4, 4, 'ada.lovelace4@realcorp.example'),
    (5, 5, 'ada.lovelace5@realcorp.example');
