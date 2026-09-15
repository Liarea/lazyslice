-- root:   public.reg017_events
-- take:   20
-- expect: exit 13 target.schema.type_literal
-- found:  the 2026-09-15 red team, attack A12
-- why:    a DOMAIN's DEFAULT lives in pg_type.typdefault, not pg_attrdef, so
--         the catalog pass's three reads could not see it and the address
--         crossed into the target's catalog under exit 0 -- where the
--         application's next INSERT that omits the column materialises it back
--         into a row
--
-- This is the 2026-09-09 review's finding 5 mechanism exactly, one catalog
-- table to the left of where T-0134 looked. A column DEFAULT was covered; a
-- domain DEFAULT was covered by nothing, and a domain's CHECK was covered only
-- by internal/verify (pg_constraint carries it with conrelid 0) and not by
-- internal/plan, which read no domain at all.
--
-- Why this is a refusal and not a rewrite. A domain's DEFAULT belongs to the
-- type rather than to a column, and the columns declared over the domain may be
-- masked under different categories or not masked at all, so there is no single
-- masker whose output would be the right replacement -- the same argument
-- internal/plan's columnDefault already makes for a default shape it declines.
-- So: exit 13 naming the domain, never the literal (THREAT_MODEL.md T4), at
-- plan, before anything in the target has been touched. The escape is
-- --allow-type-literal public.tenant_d=REASON (the T-REDFIX review's fourth
-- finding): a type refusal that no flag could clear made a source schema
-- carrying one such domain unrunnable, and --skip-table is not that flag --
-- it drops a table to schema only and the type is recreated regardless.
--
-- Before the fix this run exited 0 with
-- tenant_d|'domdefault.canary@bigcorp.com'::text in the target's pg_type.

CREATE DOMAIN public.tenant_d AS text DEFAULT 'domdefault.canary@bigcorp.com';

CREATE TABLE public.reg017_events (
    id     integer NOT NULL,
    tenant text NOT NULL,
    note   public.tenant_d,
    PRIMARY KEY (id, tenant)
);

INSERT INTO public.reg017_events (id, tenant, note) VALUES
    (1, 'ordinary', 'x'),
    (2, 'ordinary', 'y');
