-- root:   public.reg016_tickets
-- take:   20
-- expect: exit 13 target.schema.type_literal
-- found:  the 2026-09-15 red team, attacks A4b and A11
-- why:    an enum label is a DDL string literal recreated verbatim by
--         internal/load/ddl, and neither internal/plan's literal pass nor
--         internal/verify's catalog pass read pg_enum, so an email address and
--         a phone number crossed into the target's pg_enum under exit 0
--
-- What happened. ARCHITECTURE.md section 11.1's 2026-09-14 amendment (T-0134)
-- put string literals in recreated DDL inside the data boundary, and its object
-- list is defaults, generated expressions, CHECK constraints and index
-- predicates. An enum's labels are the same kind of thing by the same argument
-- -- internal/load/ddl writes CREATE TYPE ... AS ENUM ('a','b') and the label
-- is a literal in it -- and the amendment's list omitted them. Both passes read
-- pg_attrdef, pg_constraint and pg_index; grep for pg_enum returned nothing.
--
-- Why this is a refusal and not a rewrite. Every row of every column of the
-- type references a label *by value*, so masking a label would either break the
-- column or silently remap rows -- and there is no --unmask that answers it,
-- because a label is not a column. So the correct outcome is exit 13 naming the
-- type and the position of the label, never the label text (THREAT_MODEL.md
-- T4). internal/verify's catalog pass reads pg_enum too, as the second look at
-- the artefact.
--
-- The escape is --allow-type-literal public.assignee=REASON. This header and
-- the refusal itself used to name --skip-table, which cannot clear it: that
-- flag drops a table to *schema only*, which still recreates the table's DDL
-- and therefore still recreates the type, and nothing prunes an enum from the
-- schema. A source carrying one such label could not be sliced at all, under a
-- message naming a flag with no effect on it -- the T-REDFIX review's fourth
-- finding, and the reason the per-type opt-out exists.
--
-- This file's run refused at nothing before the fix: it exited 0 and wrote
-- lazyslice.yml, with both production values in the target's pg_enum.

CREATE TYPE public.assignee AS ENUM ('unassigned', 'enum.canary@bigcorp.com', '+1-415-555-0199');

CREATE TABLE public.reg016_tickets (
    id    integer PRIMARY KEY,
    owner public.assignee NOT NULL,
    title text NOT NULL
);

INSERT INTO public.reg016_tickets (id, owner, title) VALUES
    (1, 'unassigned', 'a'),
    (2, 'unassigned', 'b');
