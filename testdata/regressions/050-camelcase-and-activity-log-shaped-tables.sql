-- root:       public.reg050_accounts
-- take:       30
-- expect:     ok
-- found:      the 2026-09-25 JSON red team, round 1 (docs/reviews/2026-09-25-redteam-json/round1.json, attacker 3's log-shape-rule mismatch, entry 26), not a torture schema
-- why:        internal/classify's rule pack and internal/transform's own copy
--             of its log-shaped-table rule disagreed: a CamelCase "AuditLog"
--             table (Prisma's default) and an *_activity table were reported
--             "replaced whole" by the plan while transform walked them leaf
--             by leaf and copied every signal-free leaf, under exit 0
-- not-copied: public."AuditLog".changes, public.reg050_account_activity.detail
--
-- Not a reduction of a torture-schema failure, on 010's precedent: the red
-- team's own probe schema is already the smallest that fails, and this file
-- is its two leaking tables.
--
-- What happened. internal/classify's rules.yml carries one log_shaped rule,
-- matched over the normalised (case-split, lower-cased) table name:
-- (^|_)(audit|audits|log|logs|history|histories|event|events|activity|
-- activities|trace|traces)(_|$). internal/transform kept a second, separate
-- copy (logTableWords), a fixed eight-word list -- audit(s), log(s),
-- history/histories, event(s), no activity, no trace -- matched by splitting
-- the table name on '_' alone, with no CamelCase normalisation. "AuditLog"
-- normalises to "audit_log" and matches the rule pack's regex on the "log"
-- word, but splits to the single token "auditlog" under transform's own
-- rule, which is in neither half of its eight-word list;
-- "reg050_account_activity" matches the rule pack's regex on "activity"
-- alone, a word transform's list never carried at all. The plan printed
-- "jsonb in a log-shaped table: the document is replaced whole" for both
-- columns (the reasons line, sourced from the rule pack), and internal/
-- transform walked both leaf by leaf instead of collapsing them, copying
-- every leaf with no signal of its own. Before T-0272 that mismatch cost
-- nothing -- every walked leaf was masked regardless of what its key said --
-- and T-0272's per-leaf rule, which copies a leaf whose every enclosing key
-- carries no signal, is what turned it into a leak.
--
-- The fix (T-0398). A LogShaped flag on pipeline.Decision, set by
-- internal/classify's finalise from the rule pack's own regex on every
-- column of a matching table, whatever the column's family -- the identical
-- call the reasons line's own fragment already makes, so the two can never
-- disagree again. internal/transform's maskDocument and internal/verify's
-- own restatement of the per-leaf rule (jsonleaf.go) both read it now,
-- through their own leafPolicy, in place of logTableWords, which is gone.
--
-- account_id is a plain surrogate FK, copied verbatim either side of the fix.
-- email is the harness's own control: I2's leak check wants at least one
-- ordinary masked email to compare against, and its values share nothing
-- with the two documents' opaque keys.

CREATE TABLE public.reg050_accounts (
    id    integer PRIMARY KEY,
    email text NOT NULL
);

CREATE TABLE public."AuditLog" (
    id         integer PRIMARY KEY,
    account_id integer NOT NULL REFERENCES public.reg050_accounts(id),
    changes    jsonb NOT NULL
);

CREATE TABLE public.reg050_account_activity (
    id         integer PRIMARY KEY,
    account_id integer NOT NULL REFERENCES public.reg050_accounts(id),
    detail     jsonb NOT NULL
);

INSERT INTO public.reg050_accounts (id, email)
SELECT g, 'wren.calloway' || g || '@regression.test'
FROM generate_series(1, 30) AS g;

INSERT INTO public."AuditLog" (id, account_id, changes)
SELECT g, g,
       jsonb_build_object('a', 'Wren Calloway', 'b', 'Larkspur Row', 'c', 'Ysolde Brackenridge')
FROM generate_series(1, 30) AS g;

INSERT INTO public.reg050_account_activity (id, account_id, detail)
SELECT g, g,
       jsonb_build_object('a', 'Wren Calloway', 'b', 'Ysolde Brackenridge', 'c', 'Mozilla/5.0')
FROM generate_series(1, 30) AS g;
