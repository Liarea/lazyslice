-- root:   public.reg051_widgets
-- take:   30
-- expect: exit 12 plan.refused.unwritable
-- found:  the 2026-09-25 JSON red team, round 1 (docs/reviews/2026-09-25-redteam-json/round1.json, attacker 2's A13, entry 14), not a torture schema
-- why:    a composite column's own type held a jsonb field, and internal/classify
--         classified the column over the record's own text form: compositeSignal
--         ran each whole-value validator over the jsonb field's already-unquoted
--         text as one opaque value and never walked inside it, so the email in
--         the document underneath was never read as its own value -- the round's
--         own reproduction crossed under exit 0.
--
-- This is not a reduction of a torture-schema failure, on 010's own precedent:
-- it is the red team's own probe schema, already the smallest that fails.
--
-- What happened. compositeSignal (internal/classify/classify.go) splits a
-- composite's sampled text form into its fields with splitCompositeLiteral,
-- which already unquotes each field and undoes its doubled quotes -- and runs
-- every validator over each field and over the whole literal, which is what
-- T-0094 already closed for an ordinary scalar field -- a bare email in a text
-- field of the record was always caught, because the whole field text IS the
-- address. A json, jsonb or hstore field is different: its own value is a
-- small document, and a validator asks whether the *whole* field matches its
-- shape, never whether a value is embedded somewhere inside it -- reading a
-- validator's shape over the document's own text is not the same question as
-- reading it out of the document underneath. THREAT_MODEL.md T1's composite
-- row said "a value hit at possible or above is a plan refusal", which
-- promised nothing about a json field's own contents, and this schema is
-- exactly that gap: reg051_widgets.w never scores a hit from any field or from
-- the whole literal, so before the fix it was decided `none` and copied.
--
-- The fix (T-0399). internal/classify's decideComposite runs a structural
-- check first, over the composite's own CREATE TYPE definition
-- (compositeDocumentField, types.go): a composite whose type holds a json,
-- jsonb or hstore field -- or a nested composite that does -- is `possible`
-- on the type alone, whatever the samples say, the same outcome a value hit
-- already produces. internal/plan's existing composite refusal
-- (plan.refused.unwritable, exit 12, T-0094) now names the field and its
-- family in its message as well as the column, so the operator sees which
-- field made the record unsafe rather than only that the type is a composite.
-- internal/verify's second net carries the identical structural check
-- (compositedoc.go) as a second, independent look, for a copy that somehow
-- reached the target with one anyway -- an operator's own --unmask excepted.
--
-- Before the fix this run exited 0 with every row's jsonb field holding its
-- email address verbatim, e.g. row 1's w = (a,"{""m"": ""ana.fake1@regression.test""}").

CREATE TYPE public.reg051_wrap AS (
    tag text,
    doc jsonb
);

CREATE TABLE public.reg051_widgets (
    id integer PRIMARY KEY,
    w  public.reg051_wrap NOT NULL
);

INSERT INTO public.reg051_widgets (id, w)
SELECT g, ROW('a', jsonb_build_object('m', 'ana.fake' || g || '@regression.test'))::public.reg051_wrap
FROM generate_series(1, 30) AS g;
