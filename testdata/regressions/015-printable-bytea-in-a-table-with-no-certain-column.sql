-- root:   public.reg015_assets
-- take:   20
-- expect: ok
-- found:  the 2026-09-15 red team, attack A4a
-- why:    a bytea column holding printable UTF-8 was copied verbatim: the
--         classifier returns before any validator runs for a bytea, and
--         internal/verify's second net has no bytea in its family set either,
--         so nothing on either side ever looked at the value
--
-- What happened. ARCHITECTURE.md section 4 gives bytea its own rule -- bytea in
-- a person-shaped column is binary_personal and set to NULL -- and
-- internal/classify's bestSignal short-circuits the family before the
-- validators, with a sound reason: a bytea sample is arbitrary binary, asText
-- renders it as a Go string, and a PNG read that way carries a number and two
-- words, which AddressShape calls an address. The reason is about *bytes*. It
-- does not cover a bytea holding a UTF-8 document, and the two routes that
-- could still have reached this column both miss it: no name rule matches
-- `blob_doc`, and byteaInPersonShapedTable needs a column at `certain` in the
-- same table, which this table deliberately has none of.
--
-- The fix guards on content instead of on family, on both sides. A bytea sample
-- set that is valid UTF-8 and at least 95% printable (textsig.PrintableText,
-- read by both nets so they cannot disagree) is offered to the validators, and
-- a hit makes the column binary_personal -- the one category rules.yml accepts
-- on this family, whose masker is already NULL, so there is no new masker and
-- no new writability question. internal/verify's netMode gained famBytea under
-- the same guard. A PNG fails the printability guard and is untouched, which is
-- what `binary` below is here to prove: it must stay in the target.
--
-- Before the fix the target held blob_doc byte for byte, including
-- grace.hopper1@realcorp.example, under exit 0. After it the column is
-- binary_personal and NULL, and the leak assertion finds nothing.

CREATE TABLE public.reg015_assets (
    id       integer PRIMARY KEY,
    kind     text NOT NULL,
    blob_doc bytea NOT NULL
);

-- reg015_notes exists so that the source holds an address in a column the leak
-- assertion can scan: I2's grep half reads the *text* columns of the source and
-- refuses to prove anything when it finds none, and a bytea is not one. It is a
-- child of reg015_assets so that it is actually loaded, and its address column
-- is the one column in this file that reaches `certain` -- deliberately in the
-- other table, because a `certain` column in reg015_assets would let
-- byteaInPersonShapedTable mask blob_doc and this file would pass for the wrong
-- reason.
CREATE TABLE public.reg015_notes (
    id       integer PRIMARY KEY,
    asset_id integer NOT NULL REFERENCES public.reg015_assets(id),
    email    text NOT NULL
);

INSERT INTO public.reg015_assets (id, kind, blob_doc) VALUES
    (1, 'note', convert_to('Grace Hopper <grace.hopper1@realcorp.example> 078-05-1120', 'UTF8')),
    (2, 'note', convert_to('Katherine Johnson <katherine.johnson@realcorp.example> 078-05-1121', 'UTF8')),
    (3, 'note', convert_to('Ada Lovelace <ada.lovelace@realcorp.example> 078-05-1122', 'UTF8')),
    (4, 'note', convert_to('Alan Turing <alan.turing@realcorp.example> 078-05-1123', 'UTF8')),
    (5, 'note', convert_to('Dorothy Vaughan <dorothy.vaughan@realcorp.example> 078-05-1124', 'UTF8'));

INSERT INTO public.reg015_notes (id, asset_id, email) VALUES
    (1, 1, 'grace.hopper1@realcorp.example'),
    (2, 2, 'katherine.johnson@realcorp.example'),
    (3, 3, 'ada.lovelace@realcorp.example'),
    (4, 4, 'alan.turing@realcorp.example'),
    (5, 5, 'dorothy.vaughan@realcorp.example');
