-- root:       public.reg049_identities
-- take:       30
-- expect:     ok
-- found:      the 2026-09-25 JSON red team, round 1 (docs/reviews/2026-09-25-redteam-json/round1.json, entry 24, attacker 3), not a torture schema
-- why:        generated columns that assemble an email, a phone number and
--             a card number out of copied leaves of a masked jsonb crossed
--             at exit 0, because the second net skipped the email, phone and
--             card validators for any generated column over such a document,
--             whichever leaf it read
-- not-copied: public.reg049_identities.identity_data, public.reg049_identities.email, public.reg049_identities.joined, public.reg049_identities.dial, public.reg049_identities.dial_n, public.reg049_identities.pan
--
-- Not a reduction of a torture-schema failure, on 010's precedent: this is
-- the red team's own probe table, the Supabase auth.identities shape
-- (testdata/torture/supabase-auth) with four more generated columns beside
-- its `email`.
--
-- What happened. internal/classify's per-leaf map (T-0272) names u, h, cc,
-- nsn, n, bin and tail through the name rules only, and none names them; no
-- value validator recognises "quillon.varda7", "44" or "411111" alone; so
-- internal/transform copied every one of those leaves. The target recomputed
-- each generated column from the copied leaves: the real address, the real
-- E.164 number and a Luhn-valid card number in every row. internal/verify's
-- second net skips, for a generated column over a masked document with
-- per-leaf categories, what a leaf's category masker would emit -- so that
-- Supabase's `lower(identity_data ->> 'email')`, which holds the email
-- masker's output, does not refuse every run -- and it skipped those
-- validators for the whole column, so all four crossed under exit 0. The
-- classifier had read the generated columns' own samples and said "30/30
-- samples parse as addresses".
--
-- The fix (T-0397), in two halves. internal/classify gives every key a
-- generated column reads through -> or ->> the generated column's own
-- category, in the document's map, when the generated column's samples
-- validate, so the leaves are masked and the target recomputes the columns
-- from masked leaves. internal/verify skips a hit only when the generated
-- value equals, after lower and btrim, one of its own row's leaves a
-- category masker replaced -- `email` still passes -- and counts every
-- other hit, so a value assembled from copied leaves is refused.
--
-- `kind` is read by no generated column that validates and stays copied: it
-- is in every document so that the documents differ from their source by the
-- masked leaves alone. `not-copied:` greps the target for each whole source
-- document and for every source value of the five generated columns; before
-- the fix `joined`, `dial`, `dial_n` and `pan` crossed whole.

CREATE TABLE public.reg049_identities (
    id            bigint PRIMARY KEY,
    identity_data jsonb NOT NULL,
    email         text GENERATED ALWAYS AS (lower(identity_data ->> 'email')) STORED,
    joined        text GENERATED ALWAYS AS ((identity_data ->> 'u') || '@' || (identity_data ->> 'h')) STORED,
    dial          text GENERATED ALWAYS AS ('+' || (identity_data ->> 'cc') || (identity_data ->> 'nsn')) STORED,
    dial_n        text GENERATED ALWAYS AS ('+' || (identity_data ->> 'n')) STORED,
    pan           text GENERATED ALWAYS AS ((identity_data ->> 'bin') || (identity_data ->> 'tail')) STORED
);

INSERT INTO public.reg049_identities (id, identity_data)
SELECT g,
       jsonb_build_object(
           'email', 'quillon.varda' || g || '@fictionmail.example',
           'u',     'quillon.varda' || g,
           'h',     'fictionmail.example',
           'cc',    '44',
           'nsn',   '2079460' || lpad(g::text, 3, '0'),
           'n',     ('442079460' || lpad(g::text, 3, '0'))::bigint,
           'bin',   '411111',
           'tail',  '1111111111',
           'kind',  'standard')
FROM generate_series(1, 30) AS g;
