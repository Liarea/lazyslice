-- root:         public.reg048_rosters
-- take:         30
-- expect:       ok
-- found:        the 2026-09-25 JSON red team, round 1 (docs/reviews/2026-09-25-redteam-json/round1.json, attacker 1's A11, run 2), not a torture schema
-- phone-region: GB
-- not-copied:   public.reg048_rosters.by_phone, public.reg048_rosters.entries
-- why:          a document keyed by national-format phone numbers refused the
--               run at exit 9 under --phone-region GB, with --skip-table as
--               the only remedy it offered: transform masked a key only in
--               international form, and the second net read the surviving
--               national key under the region
--
-- Not a reduction of a torture-schema failure, on 010's precedent: the red
-- team's own probe schema is already the smallest that fails.
--
-- What happened. internal/transform's keyCategory (T-0137) masks an object
-- key that parses as an email address, a phone number or a card number, and
-- its phone question was textsig.ValidPhone, the international-only reading.
-- `+44 7911 13nnnn` was masked; `07911 12nnnn` was not. internal/verify's
-- second net reads every key of a masked document with its phone validator
-- under --phone-region (T-0221), found thirty national numbers still in the
-- target, and refused: `verify.refused.second_net_document_masked`, exit 9,
-- the target table left absent. `by_phone` is the red team's own column
-- (its leaves are masked on its name since T-0393, testdata/regressions/046);
-- `entries` is the same documents under a name no rule matches, where the
-- leaf under the national key was a name the per-leaf map could copy.
--
-- The fix (T-0394). keyCategory reads the run's region, so the national key
-- is masked through the phone masker, whose output is the international form
-- the net's key skip already leaves to the residual scan; internal/classify's
-- strongKeyShape asks the same question under the same region, so the key is
-- never in the leaf map and the leaf beneath it is masked too.
--
-- With no region the national key still survives (SECURITY.md item 8's key
-- residual): a key is masked on the strong validators alone, and a guessed
-- region is not one. `email` is here for the harness's I2 grep, which needs a
-- value it recognises in the source.

CREATE TABLE public.reg048_rosters (
    id       integer PRIMARY KEY,
    by_phone jsonb NOT NULL,
    entries  jsonb NOT NULL,
    email    text NOT NULL
);

INSERT INTO public.reg048_rosters (id, by_phone, entries, email)
SELECT g,
       jsonb_build_object('07911 12' || lpad(g::text, 4, '0'), 'Wren Calloway',
                          '+44 7911 13' || lpad(g::text, 4, '0'), 'Ysolde Brackenridge'),
       jsonb_build_object('07911 12' || lpad(g::text, 4, '0'), 'Wren Calloway',
                          '+44 7911 13' || lpad(g::text, 4, '0'), 'Ysolde Brackenridge'),
       'wren.calloway' || g || '@regression.test'
FROM generate_series(1, 30) AS g;
