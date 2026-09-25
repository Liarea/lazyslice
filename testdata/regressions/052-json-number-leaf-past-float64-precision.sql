-- root:         public.reg052_widgets
-- take:         30
-- expect:       ok
-- phone-region: US
-- not-copied:   public.reg052_widgets.p1_rounded
-- found:        the 2026-09-25 JSON red team, round 1 (docs/reviews/2026-09-25-redteam-json/round1.json, attacker 2's A11, entry 14), not a torture schema
-- why:    a card number stored as a json number leaf, under a key no name rule
--         names, reached the target with its digits past 2^53 rounded away,
--         because pgx decodes a jsonb number to a float64 and the digits were
--         already gone before any validator ran -- entry 14's own reproduction
--         crossed under exit 0.
--
-- This is not a reduction of a torture-schema failure, on 010's own precedent:
-- it is the red team's own probe schema, already the smallest that fails.
--
-- What happened. internal/extract and internal/introspect both read a row
-- through internal/pg's source pool, scanning every column generically into a
-- *any: pgx's own JSON codec decodes a json or jsonb value in that shape with
-- encoding/json's plain Unmarshal, and a Go float64 has 52 mantissa bits, so
-- an integer past 2^53 loses its low digits before this program ever sees it
-- -- measured directly, p1's nineteen digits (4000123456789012343) came back
-- as 4.0001234567890125e18, rounding the last three digits away.
-- internal/transform's leafValueCategory asks the leaf's plain decimal
-- spelling whether it parses as a card number (a Luhn check over the exact
-- digits), and the rounded spelling failed it, so p1 read clean under a key
-- (p1) no rule pack pattern names and was copied. p0, a sixteen-digit card,
-- fits inside a float64's exact integer range and was never rounded -- it is
-- the control here, masked correctly before and after. p2, a national-format
-- phone number stored as a bare integer under --phone-region, was never
-- rounded either (phone numbers are well inside 2^53) and is the third leaf
-- the tracker task names, pinned here so the fix's regression covers the
-- whole set in one schema.
--
-- The fix (T-0402). internal/pg's source reader now hands back a json or
-- jsonb column's exact source text instead of letting pgx decode it -- the
-- same text a domain over jsonb has always arrived as -- so
-- internal/transform's, internal/classify's and internal/verify's own copies
-- of decodeDocument, which already decode a string with encoding/json's
-- Decoder and UseNumber, are what every number leaf takes now, and the
-- existing json.Number path masks it: the leaf comes back a small integer
-- with none of the source's magnitude or digit count, the same way every
-- other masked number leaf always has.
--
-- Before the fix this run exited 0 with p1 rounded and unmasked in the
-- target, e.g. row 1's w held {"p0": <masked>, "p1": 4000123456789012500,
-- "p2": 2025550001}. No value here imitates a real provider's secret format;
-- both card numbers are invented and Luhn-valid.
--
-- `contact` is a scalar email column, masked before and after: it is here for
-- the harness's own I2 check, which refuses a source holding no email address
-- or phone number it can recognise, and it recognises neither a bare number
-- nor a key no name rule names -- the same reason 047 carries one.
--
-- `p1_rounded` is what proves this on the one leaf the fix actually changes;
-- `expect: ok` alone does not, and neither does a plain `not-copied:` on `w`.
-- Section 6's residual scan never tests a *number* leaf at all
-- (`documentHits`, residual.go, "a value redrawn over a small domain carries
-- no residual signal") -- true of a masked value's small domain, not of a
-- leak that was never masked, so `expect: ok` alone gives this regression
-- nothing to fail on. `not-copied:` on `w` itself fares no better: p1 does
-- not survive as its own source digits, it survives *rounded*, and p0
-- (masked) and p2 (varies by row) both change the whole document's text
-- whether or not the fix is applied, so there is nothing stable to grep for.
-- `p1_rounded` is a generated column that recomputes, from this row's own
-- `w`, the exact plain-decimal spelling a float64 round trip through p1's
-- source digits produces -- the same "4000123456789012500" this file's own
-- prose names above, reconstructed in SQL from `w->>'p1'` cast through
-- `numeric` to `float8` and back to `text` (Postgres's float8 output is the
-- same shortest-round-trip digit string Go's strconv produces, confirmed
-- directly against a real server; it is emitted in scientific notation for a
-- value this size, so the CASE below re-expands it to the plain form
-- encoding/json's own float64 formatting used to write into the target).
-- Being `GENERATED ALWAYS AS ... STORED`, it is never copied from source --
-- Postgres recomputes it in the target from the target's own row, so a
-- surviving unmasked-and-rounded p1 there reproduces the same digits and
-- `not-copied:` finds them there; a p1 the fix redrew over the financial
-- domain does not. Confirmed directly against a real server: reverting the
-- internal/pg/source.go change alone reproduces `p1_rounded`'s value inside
-- the target's own `w` text and fails this check; the current tree passes
-- it.

CREATE TABLE public.reg052_widgets (
    id         integer PRIMARY KEY,
    contact    text NOT NULL,
    w          jsonb NOT NULL,
    p1_rounded text GENERATED ALWAYS AS (
        CASE WHEN position('e' in ((w ->> 'p1')::numeric::float8)::text) = 0
             THEN ((w ->> 'p1')::numeric::float8)::text
             ELSE regexp_replace(split_part(((w ->> 'p1')::numeric::float8)::text, 'e', 1), '\.', '')
                  || repeat('0',
                       (split_part(((w ->> 'p1')::numeric::float8)::text, 'e', 2))::int
                       - length(split_part(split_part(((w ->> 'p1')::numeric::float8)::text, 'e', 1), '.', 2))
                     )
        END
    ) STORED
);

INSERT INTO public.reg052_widgets (id, contact, w)
SELECT g,
       'wren.calloway' || g || '@regression.test',
       jsonb_build_object(
           'p0', 4111111111111111,
           'p1', 4000123456789012343,
           'p2', (2025550000 + g)
       )
FROM generate_series(1, 30) AS g;
