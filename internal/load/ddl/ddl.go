// Package ddl generates the CREATE statements for the object classes v1
// recreates (ARCHITECTURE.md section 11.1). It is a deliberately narrow
// reimplementation of pg_dump --schema-only, and section 11.1 is its whole
// specification.
//
// v1 recreates, in this order: schemas, extensions, enum/domain/composite types,
// unowned sequences, tables with their columns and non-foreign constraints and,
// after the data, indexes, foreign keys, setval and ANALYZE. A partitioned
// source table becomes one plain table, which removes the masked-partition-key
// trap as a side effect.
//
// v1 does not recreate views, materialised views, functions, procedures,
// triggers, policies, rules, comments, privileges, publications, foreign tables,
// operators, operator classes, collations or partitions. None of those is a
// refusal on its own; each is counted and printed under not_recreated.
//
// A recreated object that depends on one we do not recreate is a refusal at
// plan, exit 13, before the snapshot is used for keys and before anything in the
// target is dropped. There is no flag that drops the offending default silently,
// because the application's first INSERT is the point of the tool.
//
// Scaffold status: no-op. Every function returns ErrNotImplemented.
package ddl

import (
	"errors"

	"github.com/Liarea/lazyslice/internal/pipeline"
)

// ErrNotImplemented is returned by every placeholder in the scaffold.
var ErrNotImplemented = errors.New("ddl: not implemented")

// PreData returns the statements that must run before any row is copied:
// schemas, extensions, types, sequences, tables and their non-foreign
// constraints.
//
// Scaffold status: no-op.
func PreData(_ *pipeline.Schema, _ *pipeline.Plan) ([]string, error) {
	return nil, ErrNotImplemented
}

// PostData returns the statements that run after every row is copied: indexes,
// foreign keys NOT VALID then VALIDATE CONSTRAINT, setval and ANALYZE.
//
// Scaffold status: no-op.
func PostData(_ *pipeline.Schema, _ *pipeline.Plan) ([]string, error) {
	return nil, ErrNotImplemented
}

// Recreatable checks, at plan time, that no recreated object depends on one v1
// does not recreate. A dependency is exit 13 naming the table, the column or
// index, the dependency and its kind.
//
// Scaffold status: no-op.
func Recreatable(_ *pipeline.Schema) error { return ErrNotImplemented }

// Fingerprint is sha256 over the recreated object classes only, as their DDL
// text, in the order PreData and PostData emit them. It is the same hash whether
// computed on the source or on a target we wrote, which is what the marker's
// binding needs; views and functions do not enter it.
//
// Scaffold status: no-op.
func Fingerprint(_ *pipeline.Schema) (string, error) { return "", ErrNotImplemented }
