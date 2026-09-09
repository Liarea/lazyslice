-- root:   public.reg8_account
-- take:   50
-- expect: ok
-- found:  supabase-auth
-- why:    a name hit the column's type does not accept removed the masking its type alone would have given
--
-- Supabase's `auth.users.raw_user_meta_data` is `jsonb`, and it is where the
-- identity provider's profile lands: the full name, the email address, the phone
-- number, the postal address. Two jsonb columns of that schema, side by side,
-- came out of the classifier like this:
--
--     auth.identities.identity_data
--       category: semi_structured   confidence: possible   masker: semi_structured
--       reason: type jsonb is a semi_structured type; no name signal
--
--     auth.users.raw_user_meta_data
--       category: free_text         confidence: low        (no masker)
--       reason: name matches free_text; jsonb is not an accepted type for free_text
--
-- The second was copied into the target in cleartext, and invariant I2's grep
-- half found two hundred of the source's own addresses sitting in it. The
-- difference between the two columns is that one of them has a *name* — and the
-- name made it less safe, which is the wrong direction for every rule in
-- CLAUDE.md and THREAT_MODEL.md T1.
--
-- The cause is where the type signal is consulted. ARCHITECTURE.md §4 gives
-- json, jsonb, hstore, bytea and tsvector a signal from their type alone, and
-- `decide` read `typeSignals[family]` only in its **default** branch — the one
-- reached when there is no name hit and no value signal. A name hit on a family
-- the category does not accept took the `hasName && !nameAccepted` branch, which
-- recorded `low` with the conflict named and never looked at the type at all.
-- `low` is below the mask threshold, so the column was copied.
--
-- The fix: in that branch, when the samples say nothing, the column's *type*
-- decides, exactly as it would have with no name at all. A rejected name hit is
-- an absence of evidence and it cannot be worth less than that.
--
-- What this file reproduces, and the controls with it:
--
--   reg8_account.raw_user_meta_data  jsonb, whose name matches the free_text rule
--                                on its leading `raw_` (which does not accept
--                                jsonb): must be masked.
--   reg8_account.identity_data   jsonb, no name hit: the control that was always
--                                masked and must stay masked.
--   reg8_account.email_verified  boolean, name matches email (which does not
--                                accept boolean): the *other* control. Boolean
--                                has no type signal, so this column must stay at
--                                `low` and be copied — testdata/README.md trap 19
--                                is the same shape and §4 is deliberate about it.
--
-- The assertion is the regression suite's leak check rather than the exit code:
-- the run always exited 0, which is exactly what made this worth finding.

CREATE TABLE public.reg8_account (
    id             integer PRIMARY KEY,
    raw_user_meta_data  jsonb NOT NULL,
    identity_data  jsonb NOT NULL,
    email_verified boolean NOT NULL
);

CREATE TABLE public.reg8_session (
    id         integer PRIMARY KEY,
    account_id integer NOT NULL REFERENCES public.reg8_account (id),
    agent      text NOT NULL
);

INSERT INTO public.reg8_account (id, raw_user_meta_data, identity_data, email_verified)
SELECT i,
       jsonb_build_object(
           'full_name', 'Anouk Bakker ' || i,
           'email', 'anouk.bakker' || i || '@regression.test',
           'phone_number', '+3197010' || lpad((200000 + i)::text, 6, '0'),
           'address', i || ' Kanaalstraat, Utrecht'),
       jsonb_build_object(
           'sub', md5('sub' || i),
           'email', 'anouk.bakker' || i || '@regression.test'),
       (i % 3 = 0)
FROM generate_series(1, 120) AS g(i);

INSERT INTO public.reg8_session (id, account_id, agent)
SELECT i, i, 'Mozilla/5.0 session ' || i FROM generate_series(1, 120) AS g(i);
