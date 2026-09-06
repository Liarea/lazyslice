// SPDX-License-Identifier: Apache-2.0

// Package ref holds the two identifiers every other package needs: TableRef
// and ColumnRef.
//
// It is a leaf: it imports nothing, not even fmt, so that no import cycle can
// ever form around it (ARCHITECTURE.md section 2 "Import graph"). internal/event
// imports ref; internal/pipeline imports ref, event, dsn and mask; the stage
// packages import pipeline. Nothing goes the other way, and TestImportGraph
// fails on any edge that does.
//
// Ordering everywhere in lazyslice is by (Schema, Name), then Column. The
// planner and the emitter rely on that order for determinism, so Less is
// defined here rather than being open-coded per package.
package ref

// TableRef names one table by schema and name. It is never a bare name: an
// unqualified table in a plan or an error message would be ambiguous across
// schemas and lazyslice supports more than one.
type TableRef struct {
	Schema string
	Name   string
}

// String renders the reference as "schema.name", which is the form
// lazyslice.yml, the plan and every event use.
func (t TableRef) String() string { return t.Schema + "." + t.Name }

// Less orders by (Schema, Name). Every iteration over tables uses it, so that
// two runs over one snapshot produce byte-identical output.
func (t TableRef) Less(o TableRef) bool {
	if t.Schema != o.Schema {
		return t.Schema < o.Schema
	}
	return t.Name < o.Name
}

// ColumnRef names one column of one table.
type ColumnRef struct {
	Table  TableRef
	Column string
}

// String renders the reference as "schema.table.column".
func (c ColumnRef) String() string { return c.Table.String() + "." + c.Column }

// Less orders by (Schema, Name, Column).
func (c ColumnRef) Less(o ColumnRef) bool {
	if c.Table != o.Table {
		return c.Table.Less(o.Table)
	}
	return c.Column < o.Column
}
