-- root:       public.reg047_members
-- take:       30
-- expect:     ok
-- found:      the 2026-09-25 JSON red team, round 1 (docs/reviews/2026-09-25-redteam-json/round1.json, attacker 3's national-format phone under a leaf key the name rules miss), not a torture schema
-- why:        a national-format phone number under a document key no name
--             rule names was copied at exit 0 with no --phone-region, while
--             the same values in a scalar column beside the same personal
--             neighbour were masked on a guessed-region hit
-- not-copied: public.reg047_members.doc, public.reg047_members.whatsapp
--
-- Not a reduction of a torture-schema failure, on 010's precedent: the red
-- team's own probe schema is already the smallest that fails, and this file is
-- its one leaking column beside its scalar twin.
--
-- What happened. With no --phone-region, a national-format number is read
-- under a short list of guessed regions and masked only with corroboration: a
-- proven personal neighbour in the same table (T-0221's guessedPhoneColumns).
-- The scalar `whatsapp` column below gets that and was masked. The same
-- numbers under a `whatsApp` key inside `doc` were not: internal/classify's
-- per-leaf map (T-0272) named the key through the name rules only, no rule
-- names it, so the map called it none; internal/transform's value half asks a
-- leaf's phone number in international form, or under a configured region,
-- and there was none; and every leaf of every document was copied, so each
-- whole document crossed verbatim.
--
-- The fix (T-0394). The map has a value half: a key whose string leaves
-- across the samples parse as phone numbers under the guessed regions, at the
-- scalar path's own ratio, is given `phone` when the scalar path's
-- corroboration holds -- a proven personal neighbour in the table, here
-- `email` -- or the document carries a person-identifying key of its own.
-- internal/transform masks every leaf under it through the phone masker, and
-- internal/verify's second net, reading the same map, leaves those leaves to
-- the residual scan.
--
-- `kind` carries no personal data and is in every document so that one masked
-- leaf changes the document's text and nothing else could: `not-copied:`
-- greps the target for each whole source document, and before the fix each
-- one crossed whole. The scalar `whatsapp` is the control, masked before and
-- after. `email` is the corroborating neighbour, and it is here for the
-- harness too: its I2 grep refuses a source holding no email address or phone
-- number it can recognise, and it recognises neither national number.

CREATE TABLE public.reg047_members (
    id       integer PRIMARY KEY,
    email    text NOT NULL,
    whatsapp text NOT NULL,
    doc      jsonb NOT NULL
);

INSERT INTO public.reg047_members (id, email, whatsapp, doc)
SELECT g,
       'quillon.varda' || g || '@regression.test',
       '020 7946 6' || lpad(g::text, 3, '0'),
       jsonb_build_object('whatsApp', '020 7946 6' || lpad(g::text, 3, '0'), 'kind', 'standard')
FROM generate_series(1, 30) AS g;
