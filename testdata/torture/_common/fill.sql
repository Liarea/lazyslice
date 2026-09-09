--
-- fill.sql: the row generator every torture schema's generate.sql calls.
--
-- The ten schemas under testdata/torture/ are real, upstream schemas: nobody
-- wrote them to be filled by hand, and between them they carry about a thousand
-- tables. So the per-schema generate.sql says *which* tables to fill and how
-- many rows, and this file works out *what to put in them* from the catalogue.
--
-- The contract, in one line: after `SELECT lazyslice_torture.fill('t', n)`,
-- table t holds n more rows, every one of which satisfies t's NOT NULL columns,
-- its single- and multi-column foreign keys, and its unique constraints,
-- without a single value depending on random(), now() or any other
-- non-deterministic source.
--
-- Determinism is load-bearing. Invariant I3 says the same source, secret and
-- config produce a byte-identical target; a fixture built with random() would
-- make I3 unfalsifiable on these schemas, because the two runs would be reading
-- the same rows but nobody could tell whether they were. Every expression this
-- file generates is a pure function of the row index `i`.
--
-- What it deliberately does *not* do:
--
--   * Fill nullable columns. A NULL is a legitimate value and half the point of
--     a subsetting fixture is that MATCH SIMPLE foreign keys with a NULL
--     component reference nothing. The personal data every schema needs for the
--     masking invariant goes in through the per-schema generate.sql's own
--     UPDATE statements, which is also where it can be hand-labelled for
--     docs/TORTURE.md's PII truth sets.
--   * Guess at a CHECK constraint. If a table has one this generator cannot
--     satisfy, the INSERT fails and names the constraint, and the schema's
--     generate.sql gets an explicit hand-written INSERT for that table instead.
--   * Produce a value in a masker's own output space. ARCHITECTURE.md §5 says
--     `email` emits under example.com/.net/.org and `network_id` emits in the
--     RFC 5737 documentation ranges, so a source value already in either space
--     is a documented false-positive surface for the residual scan (T-0059,
--     T-0073). Generated addresses are under `.test` and generated IPv4 is in
--     10.0.0.0/8 for that reason. The same rule covers the *word lists*: the
--     `person_name` and `address` maskers draw from `mask/words.go`, so no name
--     a per-schema generate.sql invents may be a word in them. Four given names
--     and four surnames shared with those lists made
--     `TestTortureSchemas/calcom` fail about one run in five at exit 9;
--     testdata/torture/README.md carries the whole story.
--
-- The whole schema is dropped by _common/cleanup.sql after generate.sql runs,
-- so the source database lazyslice sees is upstream's schema and nothing else.
--

CREATE SCHEMA IF NOT EXISTS lazyslice_torture;

--
-- base_type resolves a type OID through any chain of domains to the type that
-- actually decides what a literal has to look like. pagila's public.year and
-- Odoo's several domains are why this is a loop rather than one lookup.
--
CREATE OR REPLACE FUNCTION lazyslice_torture.base_type(t oid)
RETURNS oid
LANGUAGE plpgsql
IMMUTABLE
AS $fn$
DECLARE
    cur oid := t;
    kind "char";
    next oid;
    hops integer := 0;
BEGIN
    LOOP
        SELECT typtype, typbasetype INTO kind, next FROM pg_type WHERE oid = cur;
        EXIT WHEN kind IS DISTINCT FROM 'd' OR next = 0;
        cur := next;
        hops := hops + 1;
        IF hops > 16 THEN
            RAISE EXCEPTION 'lazyslice_torture: domain chain for type % is longer than 16 hops', t;
        END IF;
    END LOOP;
    RETURN cur;
END
$fn$;

--
-- text_len is the declared length of a character type, or NULL when it has
-- none. atttypmod carries length + 4 for varchar/char/bit; -1 means unlimited.
--
CREATE OR REPLACE FUNCTION lazyslice_torture.text_len(typmod integer)
RETURNS integer
LANGUAGE sql
IMMUTABLE
AS $fn$
    SELECT CASE WHEN typmod > 4 THEN typmod - 4 END
$fn$;

--
-- value_expr returns the SQL text of an expression producing one value of the
-- given type for row index i, referred to as `s.i`.
--
-- `tag` is a per-column string mixed into every text-shaped value, so that two
-- columns of the same table never generate the same string and a multi-column
-- unique index over two text columns is satisfied by construction.
--
-- Every branch is deterministic in i. The unknown-type branch raises rather
-- than falling back to NULL: a NOT NULL column filled with NULL would fail
-- later with a message about the constraint rather than about the gap here.
--
CREATE OR REPLACE FUNCTION lazyslice_torture.value_expr(
    typid oid, typmod integer, tag text, unique_col boolean
)
RETURNS text
LANGUAGE plpgsql
AS $fn$
DECLARE
    base oid := lazyslice_torture.base_type(typid);
    tname text;
    tkind "char";
    telem oid;
    tcat "char";
    len integer := lazyslice_torture.text_len(typmod);
    body text;
    labels integer;
BEGIN
    SELECT typname, typtype, typelem, typcategory INTO tname, tkind, telem, tcat
    FROM pg_type WHERE oid = base;

    -- Enums cycle through their labels in sort order. A single-column unique
    -- index over an enum cannot be satisfied beyond `labels` rows, so that case
    -- is refused here rather than at the INSERT.
    IF tkind = 'e' THEN
        SELECT count(*) INTO labels FROM pg_enum WHERE enumtypid = base;
        IF unique_col THEN
            RAISE EXCEPTION
                'lazyslice_torture: % is an enum with % label(s) under a single-column unique index; '
                'fill it with an explicit INSERT in the schema''s generate.sql', tag, labels;
        END IF;
        RETURN format(
            '(SELECT e.enumlabel::%s FROM pg_enum e WHERE e.enumtypid = %L::oid '
            'ORDER BY e.enumsortorder OFFSET ((s.i - 1) %% %s) LIMIT 1)',
            format_type(typid, typmod), base, labels);
    END IF;

    -- An array is one element of its element type, so that a column declared
    -- text[] holds {something} rather than {}: §5 preserves an empty collection
    -- and the invariant suite reads an empty array as "the masker was never
    -- run", so a fixture full of empty arrays would assert nothing.
    IF tcat = 'A' AND telem <> 0 THEN
        RETURN format('ARRAY[%s]::%s',
            lazyslice_torture.value_expr(telem, -1, tag, false),
            format_type(typid, typmod));
    END IF;

    body := format('%L || s.i::text', tag || '-');

    RETURN CASE tname
        WHEN 'int2'      THEN '(s.i % 32000)::smallint'
        WHEN 'int4'      THEN 's.i'
        WHEN 'int8'      THEN 's.i::bigint'
        WHEN 'oid'       THEN 's.i::oid'
        WHEN 'numeric'   THEN 's.i::numeric'
        WHEN 'float4'    THEN 's.i::real'
        WHEN 'float8'    THEN 's.i::double precision'
        WHEN 'money'     THEN 's.i::numeric::money'
        WHEN 'bool'      THEN '(s.i % 2 = 0)'
        WHEN 'uuid'      THEN format('md5(%L || s.i::text)::uuid', tag)
        WHEN 'date'      THEN 'DATE ''2024-01-01'' + s.i'
        WHEN 'timestamp' THEN 'TIMESTAMP ''2024-01-01 00:00:00'' + (s.i || '' minutes'')::interval'
        WHEN 'timestamptz' THEN 'TIMESTAMPTZ ''2024-01-01 00:00:00+00'' + (s.i || '' minutes'')::interval'
        WHEN 'time'      THEN '(TIME ''00:00:00'' + (s.i % 1440 || '' minutes'')::interval)'
        WHEN 'timetz'    THEN '((TIME ''00:00:00'' + (s.i % 1440 || '' minutes'')::interval) AT TIME ZONE ''UTC'')'
        WHEN 'interval'  THEN '(s.i || '' minutes'')::interval'
        WHEN 'json'      THEN 'json_build_object(''i'', s.i)'
        WHEN 'jsonb'     THEN 'jsonb_build_object(''i'', s.i)'
        WHEN 'bytea'     THEN 'decode(md5(s.i::text), ''hex'')'
        WHEN 'xml'       THEN '(''<n>'' || s.i || ''</n>'')::xml'
        WHEN 'tsvector'  THEN format('to_tsvector(''simple'', %L || s.i::text)', tag || ' ')
        WHEN 'tsquery'   THEN format('to_tsquery(''simple'', %L || s.i::text)', tag || '')
        -- 10.0.0.0/8, never a documentation range: see the header.
        WHEN 'inet'      THEN '((''10.'' || ((s.i / 65024) % 256) || ''.'' || ((s.i / 254) % 256) || ''.'' || (s.i % 254 + 1))::inet)'
        WHEN 'cidr'      THEN '((''10.'' || ((s.i / 65024) % 256) || ''.'' || ((s.i / 254) % 256) || ''.0/24'')::cidr)'
        WHEN 'macaddr'   THEN '((''08:00:2b:'' || lpad(to_hex((s.i / 65536) % 256), 2, ''0'') || '':'' || lpad(to_hex((s.i / 256) % 256), 2, ''0'') || '':'' || lpad(to_hex(s.i % 256), 2, ''0''))::macaddr)'
        WHEN 'bit'       THEN format('(repeat(''0'', %s))::bit(%s)', COALESCE(len, 1), COALESCE(len, 1))
        WHEN 'varbit'    THEN '(''1''::varbit)'
        WHEN 'int4range' THEN '(int4range(s.i, s.i + 10))'
        WHEN 'int8range' THEN '(int8range(s.i::bigint, s.i::bigint + 10))'
        WHEN 'numrange'  THEN '(numrange(s.i::numeric, s.i::numeric + 10))'
        WHEN 'daterange' THEN '(daterange(DATE ''2024-01-01'' + s.i, DATE ''2024-01-01'' + s.i + 10))'
        WHEN 'tsrange'   THEN '(tsrange(TIMESTAMP ''2024-01-01'' + (s.i || '' minutes'')::interval, TIMESTAMP ''2024-01-01'' + (s.i || '' minutes'')::interval + interval ''1 hour''))'
        WHEN 'tstzrange' THEN '(tstzrange(TIMESTAMPTZ ''2024-01-01+00'' + (s.i || '' minutes'')::interval, TIMESTAMPTZ ''2024-01-01+00'' + (s.i || '' minutes'')::interval + interval ''1 hour''))'
        WHEN 'hstore'    THEN format('hstore(''i'', s.i::text)')
        WHEN 'text'      THEN body
        WHEN 'citext'    THEN format('(%s)::citext', body)
        WHEN 'name'      THEN format('right(%s, 63)::name', body)
        WHEN 'varchar'   THEN
            CASE WHEN len IS NULL THEN body
                 -- right(), not left(): the row index is what makes the value
                 -- unique, so it is the end that has to survive the truncation.
                 ELSE format('right(%s, %s)', body, len) END
        WHEN 'bpchar'    THEN
            CASE WHEN len IS NULL THEN body
                 ELSE format('right(%s, %s)', body, len) END
        ELSE NULL
    END;
END
$fn$;

--
-- required reports whether a column has to be written.
--
-- `attnotnull` is the obvious half. The other half is GitLab's: adding NOT NULL
-- to a large table rewrites it, so GitLab adds `CHECK (col IS NOT NULL) NOT
-- VALID` instead and validates it afterwards, and `public.namespaces`,
-- `public.projects` and a dozen other tables in that fixture have required
-- columns that `pg_attribute.attnotnull` says nothing about. A generator that
-- read only attnotnull would leave them NULL and fail on a constraint whose
-- name — `check_2eae3bdf93` — says nothing either.
--
-- The definition is matched exactly rather than parsed: only the one-column
-- form `CHECK ((col IS NOT NULL))`, which is the idiom, and nothing cleverer.
--
CREATE OR REPLACE FUNCTION lazyslice_torture.required(tbl regclass, att smallint)
RETURNS boolean
LANGUAGE sql
STABLE
AS $fn$
    SELECT a.attnotnull
        OR EXISTS (
            SELECT 1 FROM pg_constraint c
            WHERE c.conrelid = tbl AND c.contype = 'c'
              AND pg_get_constraintdef(c.oid) IN (
                    'CHECK ((' || quote_ident(a.attname) || ' IS NOT NULL))',
                    'CHECK ((' || quote_ident(a.attname) || ' IS NOT NULL)) NOT VALID')
        )
    FROM pg_attribute a
    WHERE a.attrelid = tbl AND a.attnum = att
$fn$;

--
-- fill inserts n rows into tbl.
--
-- Only columns that are NOT NULL, have no default, are not identity and are not
-- generated are written: everything else the server can fill for itself, and a
-- fixture that overrode a default would be testing this file rather than the
-- schema.
--
-- Foreign keys are satisfied by picking a parent row, and the pick is a
-- mixed-radix counter across the table's foreign keys: the k-th key's parent is
-- chosen at offset (i-1) / (product of the radices before it) modulo its own
-- row count. That is what makes a join table's (a_id, b_id) unique constraint
-- satisfiable — a plain (i-1) % count on both columns repeats the same pair
-- every count rows — and it is deterministic, which random() would not be. When
-- n exceeds the product of the radices the combinations must repeat, and this
-- raises rather than letting the INSERT fail on a unique constraint with a
-- message about the schema.
--
-- `overrides` maps a column name to the SQL text of the expression to write
-- into it, `s.i` being the row index. It is the escape hatch for a column whose
-- correct value is a fact about the application rather than about the type —
-- GitLab's `issues.work_item_type_id` has a BEFORE INSERT trigger that raises
-- unless the value is one of nine known ids, and no amount of reading the
-- catalogue would tell anyone that. A column named here is written whether or
-- not it is required, so it also serves to fill a nullable column the fixture
-- wants populated.
CREATE OR REPLACE FUNCTION lazyslice_torture.fill(tbl regclass, n integer, overrides jsonb DEFAULT '{}'::jsonb)
RETURNS integer
LANGUAGE plpgsql
AS $fn$
DECLARE
    col record;
    fk record;
    names text[] := '{}';
    exprs text[] := '{}';
    written text[] := '{}';
    fk_of_attnum jsonb := '{}'::jsonb;   -- attnum -> [parent expr]
    divisor bigint := 1;
    parent_rows bigint;
    combinations bigint := 1;
    order_by text;
    expr text;
    stmt text;
    unique_col boolean;
    tag text;
    key text;
BEGIN
    IF n <= 0 THEN
        RAISE EXCEPTION 'lazyslice_torture: fill(%, %) asks for no rows', tbl, n;
    END IF;

    FOR key IN SELECT jsonb_object_keys(overrides) LOOP
        IF NOT EXISTS (
            SELECT 1 FROM pg_attribute a
            WHERE a.attrelid = tbl AND a.attname = key AND a.attnum > 0 AND NOT a.attisdropped
        ) THEN
            RAISE EXCEPTION 'lazyslice_torture: fill(%) has an override for %, which is not a column of it',
                tbl, key;
        END IF;
    END LOOP;

    -- One pass over the table's foreign keys, in constraint-name order so the
    -- radix assignment is stable across runs, recording the expression each
    -- referencing column gets.
    -- The order matters, and it is not alphabetical. The k-th key's parent is
    -- picked at offset (i-1)/divisor, so the *first* key varies every row and
    -- the last one barely varies at all; a key whose columns are under a unique
    -- index therefore has to come first, or n rows all get the same parent for
    -- it and the insert fails on that index. GitLab's issue_assignees is the
    -- case: its primary key is (user_id, issue_id) and it has a third foreign
    -- key on namespace_id, and sorting by name alone put namespace_id first and
    -- made every row's (user_id, issue_id) identical.
    FOR fk IN
        SELECT c.oid, c.conname, c.conrelid, c.confrelid, c.conkey, c.confkey,
               EXISTS (
                   SELECT 1 FROM pg_index x
                   WHERE x.indrelid = tbl AND x.indisunique
                     AND (x.indkey::smallint[])[0:x.indnkeyatts - 1] && c.conkey
               ) AS under_unique_index
        FROM pg_constraint c
        WHERE c.conrelid = tbl AND c.contype = 'f' AND c.conparentid = 0
        ORDER BY 7 DESC, c.conname
    LOOP
        -- Only keys every one of whose columns this function is going to fill.
        -- A key with a nullable component is left alone: MATCH SIMPLE says a
        -- row with any NULL in the key references nothing, so it is satisfied
        -- without a parent, and inventing one here would remove the NULL that
        -- makes the fixture interesting.
        CONTINUE WHEN EXISTS (
            SELECT 1 FROM unnest(fk.conkey) AS k(attnum)
            JOIN pg_attribute a ON a.attrelid = fk.conrelid AND a.attnum = k.attnum
            LEFT JOIN pg_attrdef d ON d.adrelid = a.attrelid AND d.adnum = a.attnum
            WHERE NOT lazyslice_torture.required(fk.conrelid, a.attnum) OR d.adbin IS NOT NULL
               OR a.attidentity <> '' OR a.attgenerated <> ''
        );

        EXECUTE format('SELECT count(*) FROM %s', fk.confrelid::regclass) INTO parent_rows;
        IF parent_rows = 0 THEN
            RAISE EXCEPTION
                'lazyslice_torture: %.% references % through %, and % holds no rows; '
                'fill the parent first in this schema''s generate.sql',
                tbl, '', fk.confrelid::regclass, fk.conname, fk.confrelid::regclass;
        END IF;

        SELECT string_agg(quote_ident(a.attname), ', ' ORDER BY k.ord)
        INTO order_by
        FROM unnest(fk.confkey) WITH ORDINALITY AS k(attnum, ord)
        JOIN pg_attribute a ON a.attrelid = fk.confrelid AND a.attnum = k.attnum;

        FOR col IN
            SELECT k.ord, k.attnum AS child_attnum,
                   (SELECT a.attname FROM pg_attribute a
                     WHERE a.attrelid = fk.confrelid AND a.attnum = fk.confkey[k.ord]) AS parent_col
            FROM unnest(fk.conkey) WITH ORDINALITY AS k(attnum, ord)
        LOOP
            -- The same ORDER BY and the same OFFSET in every member column's
            -- subquery is what makes a composite key come from one parent row.
            expr := format(
                '(SELECT p.%I FROM %s p ORDER BY %s OFFSET (((s.i - 1) / %s) %% %s) LIMIT 1)',
                col.parent_col, fk.confrelid::regclass, order_by, divisor, parent_rows);
            fk_of_attnum := jsonb_set(fk_of_attnum, ARRAY[col.child_attnum::text], to_jsonb(expr));
        END LOOP;

        divisor := divisor * parent_rows;
        combinations := combinations * parent_rows;
    END LOOP;

    -- Repeating a foreign-key combination is only a problem when a unique index
    -- forbids it, and it only forbids it when every one of that index's key
    -- columns is one this function drives from a parent: an index that also
    -- covers an ordinary column is satisfied by that column's own row index. A
    -- child table with one parent and more rows than its parent has is the
    -- normal case, not an error, so the guard is this narrow on purpose.
    IF combinations < n AND EXISTS (
        SELECT 1 FROM pg_index x
        WHERE x.indrelid = tbl AND x.indisunique AND x.indnatts > 0
          AND NOT EXISTS (
              SELECT 1 FROM unnest(x.indkey[0:x.indnkeyatts - 1]) AS k(attnum)
              WHERE NOT (fk_of_attnum ? k.attnum::text)
          )
    ) THEN
        RAISE EXCEPTION
            'lazyslice_torture: fill(%, %) would repeat a foreign-key combination under a unique '
            'index, because its parents admit only % distinct ones. Fill the parents with more '
            'rows, or ask for fewer here',
            tbl, n, combinations;
    END IF;

    -- Which columns get written.
    --
    -- The normal answer is "the NOT NULL ones with no default": the server can
    -- fill the rest, and a generator that overrode a default would be testing
    -- itself rather than the schema. There is one exception, and Mastodon is
    -- what forces it. `accounts.username` is `character varying DEFAULT ''
    -- NOT NULL` under `CREATE UNIQUE INDEX ... ON accounts (lower(username),
    -- lower(domain))`, so leaving it to its default puts the empty string in
    -- every row and the second insert fails on the unique index. So a defaulted
    -- column is written too when it is covered by a unique index *and* its
    -- default is a constant — no parenthesis anywhere in it, which is what
    -- keeps `nextval('..._id_seq')` and Mastodon's own
    -- `timestamp_id('accounts')` out: those defaults already produce a distinct
    -- value per row, and overriding them would throw away the shape of the key
    -- this fixture exists to carry.
    --
    -- "Covered by a unique index" reads the index *definition* rather than
    -- indkey, because `lower(username)` is an expression index and its column
    -- is not in indkey at all. The definition is cut at ` USING ` first, so that
    -- an index whose own *name* contains the column name — Rails names them
    -- `index_accounts_on_username_and_domain_lower` — does not match on that
    -- alone.
    FOR col IN
        SELECT a.attnum, a.attname, a.atttypid, a.atttypmod
        FROM pg_attribute a
        LEFT JOIN pg_attrdef d ON d.adrelid = a.attrelid AND d.adnum = a.attnum
        WHERE a.attrelid = tbl
          AND a.attnum > 0
          AND NOT a.attisdropped
          AND lazyslice_torture.required(tbl, a.attnum)
          AND a.attidentity = ''
          AND a.attgenerated = ''
          AND (
              d.adbin IS NULL
              OR (pg_get_expr(d.adbin, d.adrelid) NOT LIKE '%(%'
                  AND EXISTS (
                      SELECT 1 FROM pg_index x
                      WHERE x.indrelid = tbl AND x.indisunique
                        AND (a.attnum = ANY (x.indkey::smallint[])
                             OR substring(pg_get_indexdef(x.indexrelid) FROM ' USING .*')
                                ~ ('\m' || a.attname || '\M'))
                  ))
          )
        ORDER BY a.attnum
    LOOP
        names := names || quote_ident(col.attname);
        written := written || col.attname;

        IF overrides ? col.attname THEN
            exprs := exprs || (overrides ->> col.attname);
            CONTINUE;
        END IF;

        IF fk_of_attnum ? col.attnum::text THEN
            exprs := exprs || (fk_of_attnum ->> col.attnum::text);
            CONTINUE;
        END IF;

        -- Only a *single-column* unique index makes the value itself have to be
        -- distinct; in a multi-column one another column can carry the
        -- distinctness. The flag is what stops value_expr handing back a
        -- cycling enum label for a column that cannot repeat.
        SELECT EXISTS (
            SELECT 1 FROM pg_index x
            WHERE x.indrelid = tbl AND x.indisunique
              AND x.indnatts = 1 AND x.indkey[0] = col.attnum
        ) INTO unique_col;

        tag := left(tbl::text || '.' || col.attname, 40);
        expr := lazyslice_torture.value_expr(col.atttypid, col.atttypmod, tag, unique_col);
        IF expr IS NULL THEN
            RAISE EXCEPTION
                'lazyslice_torture: no generator for %.% of type %; add one to '
                'testdata/torture/_common/fill.sql value_expr, or fill this table by hand',
                tbl, col.attname, format_type(col.atttypid, col.atttypmod);
        END IF;
        exprs := exprs || expr;
    END LOOP;

    -- Overrides for columns the loop above did not visit: a nullable column, or
    -- one with a default the fixture wants to replace.
    FOR key IN SELECT jsonb_object_keys(overrides) LOOP
        IF NOT (key = ANY (written)) THEN
            names := names || quote_ident(key);
            exprs := exprs || (overrides ->> key);
        END IF;
    END LOOP;

    IF array_length(names, 1) IS NULL THEN
        -- Every column has a default or is generated: the table is filled by
        -- asking for n default rows.
        stmt := format('INSERT INTO %s SELECT FROM generate_series(1, %s) AS s(i)', tbl, n);
    ELSE
        stmt := format('INSERT INTO %s (%s) SELECT %s FROM generate_series(1, %s) AS s(i)',
                       tbl, array_to_string(names, ', '), array_to_string(exprs, ', '), n);
    END IF;

    BEGIN
        EXECUTE stmt;
    EXCEPTION WHEN OTHERS THEN
        RAISE EXCEPTION E'lazyslice_torture: filling % failed: %\nstatement: %', tbl, SQLERRM, stmt;
    END;

    RETURN n;
END
$fn$;
