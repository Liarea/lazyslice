-- root:       public.reg053_people
-- take:       30
-- expect:     ok
-- found:      the 2026-09-25 JSON red team, round 1 (docs/reviews/2026-09-25-redteam-json/round1.json, attacker 2's A7/A7b and A8, entries 16 and 21), not a torture schema
-- why:        an email, a phone number, a card and a national identifier
--             written in another code point or a reversible encoding --
--             fullwidth, en-dash or underscore or zero-width separated, a
--             trailing root dot, mailto:, percent-encoded -- were copied at
--             exit 0 from a neutral text column and from a JSON leaf under a
--             neutral key alike
-- not-copied: public.reg053_widgets.c01, public.reg053_widgets.c02, public.reg053_widgets.c03, public.reg053_widgets.c04, public.reg053_widgets.c05, public.reg053_widgets.c06, public.reg053_widgets.c07, public.reg053_widgets.c08, public.reg053_widgets.c09, public.reg053_widgets.w, public.reg053_encoded.e01, public.reg053_encoded.e02, public.reg053_encoded.e03, public.reg053_encoded.e04, public.reg053_encoded.x, public.reg053_wrapped.b01, public.reg053_wrapped.y
--
-- Not a reduction of a torture-schema failure, on 010's precedent: the red
-- team's own probe values, one neutral column and one neutral document key per
-- spelling, are already the smallest schema that fails.
--
-- What happened. Every validator is a parse, and internal/textsig's
-- Candidates offered each parser the value as written plus the folk spellings
-- of the 2026-09-15 A2 amendment (the at/dot words, spelled digits, spaced
-- groups, a display name). None of those undoes a code point or an encoding,
-- so an address with a fullwidth at sign (U+FF20), an international number in
-- fullwidth digits and a fullwidth plus, a card grouped with en dashes,
-- underscores or zero-width spaces, an address whose domain keeps the DNS
-- root's trailing dot, an address behind mailto: and a percent-encoded number
-- ("%2B44...") parsed as nothing. reg053_widgets has no personal column of its
-- own, so the rail beside a certain column never reached it, no name rule
-- names c01 to c09 or v1 to v9, and the columns and the leaves were copied.
--
-- reg053_encoded holds the spellings that were masked before the fix only by
-- accident (entry 21: "held only by accident of entropy"): a backslash-u
-- escaped address, a percent-encoded address, a base64 address and a number
-- behind sms:, each of which the entropy check read as a credential. They are
-- pinned so the fix keeps them masked by what they are. They are in their own
-- table because a column masked as a credential corroborates a guessed-region
-- phone reading of its neighbours, and in reg053_widgets that would have
-- masked c09 before the fix for a reason that has nothing to do with it.
--
-- Measured before the fix, against the parent commit's internal/textsig with
-- this file: c01 to c09 and every whole document in `w` crossed verbatim, and
-- the run exited 0. After it, nothing does.
--
-- reg053_wrapped is the T-0403 review round's finding: base64 as a shell
-- (`echo addr | base64`) or Postgres's own encode(..., 'base64') writes it,
-- with the input's trailing newline encoded and, past 57 bytes, the output
-- broken into 76-character lines. The first landing's base64 rule refused a
-- value holding a line break and a decode holding a non-printable rune, and
-- the entropy check does not read a value with a line break in it, so no
-- validator read b01 as an address. It was masked anyway, by accident, like
-- reg053_encoded's four: `lazyslice classify` against the first landing
-- reported b01 as "mixed digits and words" (a postal address) and `y` as a
-- jsonb column with no personal leaf, masked whole as semi_structured; after
-- the review round, b01 is "30/30 samples parse as addresses" and `y` holds
-- "personal data at a JSON leaf". The harness cannot tell those apart, so
-- not-copied: pins that it stays masked, and textsig's
-- TestEncodedSpellingsStillParse pins why. It is a table of its own so that
-- no masked neighbour decides it.
--
-- The fix (T-0403). Candidates now also offers each value once decoded
-- (internal/textsig/encodings.go): NFKC, format characters removed, Unicode
-- dashes as the hyphen, underscore and slash as group separators, a leading
-- mailto:, tel: or sms: removed, percent and backslash-u escapes undone, a
-- trailing root dot removed, and a base64 decode that is printable text of a
-- plausible length. The classifier, the second net and the per-leaf rule in
-- internal/transform all read the validators through Candidates, so the
-- columns are masked under their own categories and so is every leaf.
--
-- `reg053_people.contact` is a plain email column in the root, masked before
-- and after: it is here for the harness's own I2 check, which refuses a source
-- holding no email address or phone number it can recognise. `kind` carries no
-- personal data and is in both documents so that a masked leaf is what
-- changes a document's text. c01's local part varies by a letter, not a digit,
-- because the red team's own value was one character class, and a digit would
-- have made it a credential-shaped string the entropy check masks.
--
-- The code points are written as chr() so the file stays ASCII: 65312 is the
-- fullwidth at sign, 65291 the fullwidth plus, 65296 to 65305 the fullwidth
-- digits, 8211 the en dash, 8203 the zero-width space and 92 the backslash.
-- No value imitates a real provider's secret format; the card numbers are the
-- networks' published test numbers and the phone numbers are in Ofcom's
-- drama range.

CREATE TABLE public.reg053_people (
    id      integer PRIMARY KEY,
    contact text NOT NULL
);

CREATE TABLE public.reg053_widgets (
    id        integer PRIMARY KEY,
    person_id integer NOT NULL REFERENCES public.reg053_people (id),
    c01       text NOT NULL,
    c02       text NOT NULL,
    c03       text NOT NULL,
    c04       text NOT NULL,
    c05       text NOT NULL,
    c06       text NOT NULL,
    c07       text NOT NULL,
    c08       text NOT NULL,
    c09       text NOT NULL,
    w         jsonb NOT NULL
);

CREATE TABLE public.reg053_wrapped (
    id        integer PRIMARY KEY,
    person_id integer NOT NULL REFERENCES public.reg053_people (id),
    b01       text NOT NULL,
    y         jsonb NOT NULL
);

CREATE TABLE public.reg053_encoded (
    id        integer PRIMARY KEY,
    person_id integer NOT NULL REFERENCES public.reg053_people (id),
    e01       text NOT NULL,
    e02       text NOT NULL,
    e03       text NOT NULL,
    e04       text NOT NULL,
    x         jsonb NOT NULL
);

INSERT INTO public.reg053_people (id, contact)
SELECT g, 'orla.bexley' || g || '@regression.test'
FROM generate_series(1, 30) AS g;

INSERT INTO public.reg053_widgets (id, person_id, c01, c02, c03, c04, c05, c06, c07, c08, c09, w)
SELECT g, g, c01, c02, c03, c04, c05, c06, c07, c08, c09,
       jsonb_build_object('kind', 'standard',
                          'v1', c01, 'v2', c02, 'v3', c03, 'v4', c04, 'v5', c05,
                          'v6', c06, 'v7', c07, 'v8', c08, 'v9', c09)
FROM (
    SELECT g,
           -- c01: an address with a fullwidth at sign
           'ana.fake' || substr('abcdefghijklmnopqrstuvwxyzabcd', g, 1) || chr(65312) || 'example.org' AS c01,
           -- c02: an international number in fullwidth digits and plus
           translate('+44 20 7946 0' || lpad((100 + g)::text, 3, '0'), '+0123456789',
                     chr(65291) || chr(65296) || chr(65297) || chr(65298) || chr(65299) || chr(65300) ||
                     chr(65301) || chr(65302) || chr(65303) || chr(65304) || chr(65305)) AS c02,
           -- c03, c04, c05: a card grouped with en dashes, underscores, zero-width spaces
           replace(card, ' ', chr(8211)) AS c03,
           replace(card, ' ', '_') AS c04,
           replace(card, ' ', chr(8203)) AS c05,
           -- c06: an address whose domain keeps the root's trailing dot
           'ANA.FAKE' || g || '@EXAMPLE.ORG.' AS c06,
           -- c07: an address behind mailto:
           'mailto:ana.fake' || g || '@example.org' AS c07,
           -- c08: a percent-encoded international number
           '%2B44207946' || lpad((100 + g)::text, 4, '0') AS c08,
           -- c09: a US Social Security number grouped with en dashes
           '078' || chr(8211) || '05' || chr(8211) || lpad((1120 + g)::text, 4, '0') AS c09
    FROM (
        SELECT g,
               CASE g % 3
                   WHEN 0 THEN '4111 1111 1111 1111'
                   WHEN 1 THEN '5500 0000 0000 0004'
                   ELSE '4012 8888 8888 1881'
               END AS card
        FROM generate_series(1, 30) AS g
    ) AS cards
) AS spellings;

INSERT INTO public.reg053_encoded (id, person_id, e01, e02, e03, e04, x)
SELECT g, g, e01, e02, e03, e04,
       jsonb_build_object('kind', 'standard', 'v10', e01, 'v11', e02, 'v12', e03, 'v13', e04)
FROM (
    SELECT g,
           -- e01: an address with a backslash-u escaped at sign
           'ana.fake' || g || chr(92) || 'u0040example.org' AS e01,
           -- e02: a percent-encoded address
           'ana.fake' || g || '%40example.org' AS e02,
           -- e03: a base64 address
           encode(convert_to('ana.fake' || g || '@example.org', 'UTF8'), 'base64') AS e03,
           -- e04: a number behind sms:, with a body
           'sms:+44207946' || lpad((100 + g)::text, 4, '0') || '?body=hi' AS e04
    FROM generate_series(1, 30) AS g
) AS spellings;

-- b01: base64 of an address with the trailing newline echo keeps, long
-- enough that encode() breaks it over two lines, as MIME does
INSERT INTO public.reg053_wrapped (id, person_id, b01, y)
SELECT g, g, b01, jsonb_build_object('kind', 'standard', 'v20', b01)
FROM (
    SELECT g,
           encode(convert_to('ana.fake.with.a.considerably.longer.local.part' || g ||
                             '@example.org' || chr(10), 'UTF8'), 'base64') AS b01
    FROM generate_series(1, 30) AS g
) AS spellings;
