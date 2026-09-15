-- root:   public.reg013_items
-- take:   20
-- expect: ok
-- found:  the 2026-09-09 review, finding 8 (evidence/json_keys.log)
-- why:    a jsonb document keyed by an email address kept the key verbatim --
--         internal/transform/json.go masked every leaf value but never looked
--         at a key -- so the address survived in the target under exit 0
--         even though the value beside it was masked
--
-- The fourth regression that did not come from testdata/torture/. Finding 8 of
-- docs/reviews/2026-09-09/REVIEW.md is already the smallest schema that shows
-- it -- one table, one jsonb column, one row -- and evidence/json_keys.log is
-- its transcript.
--
-- What happened. ARCHITECTURE.md section 4's JSON rule masks every scalar leaf
-- of a document and keeps its structure, which section 4 states as "key names
-- survive masking" -- a stated false negative for an arbitrary identifier used
-- as a key, and a review round accepted it as one (SECURITY.md). The review's
-- probe put a real email address in that position instead of an arbitrary
-- identifier: {"canary.person@example.org":"ok"}. The value "ok" was masked as
-- free_text as every string leaf is; the key was not looked at by anything --
-- internal/transform/json.go's walk copied every map key through unchanged --
-- so the target held {"canary.person@example.org":"<masked ok>"} under exit 0,
-- with the source's own email address sitting in the target's jsonb column.
--
-- The fix narrows that stated limitation to what a validator can actually
-- name. internal/transform/json.go's maskKey now runs a key through the same
-- three strong validators internal/verify/catalog.go and internal/plan's own
-- DDL-literal check already use -- email, phone, and the Luhn half of a
-- financial account -- and masks it through that category's masker when one
-- matches; the masked key's residual entry is recorded under the document's
-- own empty path rather than its JSON path, because the target spells that
-- position with the masked key, not the source one, and internal/verify's own
-- JSON walk (`keyHits`, columns.go's `documentKeys`) now tests every key it
-- finds in the target the same way, so a key that leaked unmasked is a residual
-- hit and not a silent pass. A key that is not one of the three -- a UUID, a
-- slug, a customer number -- still survives, which is what SECURITY.md's
-- narrowed limitation now says plainly rather than leaving the whole of
-- "arbitrary JSON keys" open.
--
-- Before the fix, this file's run held
-- {"canary.person@example.org": "<masked>"} in the target's jsonb column under
-- exit 0, with the key exactly as the source wrote it. After the fix the key
-- is masked to another address in the email masker's own domain, the run still
-- exits 0, and TestTortureRegressions' leak assertion -- the half of this file
-- that matters -- finds no trace of canary.person@example.org anywhere in the
-- target, key or value.

CREATE TABLE public.reg013_items (
    id      integer PRIMARY KEY,
    payload jsonb NOT NULL
);

INSERT INTO public.reg013_items (id, payload)
VALUES (1, '{"canary.person@example.org":"ok"}'::jsonb);
