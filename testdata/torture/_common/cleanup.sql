--
-- cleanup.sql: removes the generator from the source database.
--
-- _common/fill.sql puts its functions in the schema `lazyslice_torture` for one
-- reason: so that this file can take them out again. What lazyslice introspects
-- has to be the upstream schema and nothing else — a torture run that found an
-- extra schema, or a function nobody upstream wrote, would be measuring the
-- fixture rather than the fixture's subject.
--
-- ANALYZE runs here rather than in each generate.sql because every one of them
-- would otherwise end with the same line: pg_class.reltuples is -1 until it
-- does, and both the classifier's sampling and the planner's estimates read it.
--

DROP SCHEMA IF EXISTS lazyslice_torture CASCADE;

ANALYZE;
