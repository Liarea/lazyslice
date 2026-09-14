// SPDX-License-Identifier: Apache-2.0

package core

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/Liarea/lazyslice/internal/dsn"
	"github.com/Liarea/lazyslice/internal/event"
	"github.com/Liarea/lazyslice/internal/extract"
	"github.com/Liarea/lazyslice/internal/introspect"
	"github.com/Liarea/lazyslice/internal/load"
	"github.com/Liarea/lazyslice/internal/load/ddl"
	"github.com/Liarea/lazyslice/internal/pg"
	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/plan"
	"github.com/Liarea/lazyslice/internal/ref"
	"github.com/Liarea/lazyslice/internal/transform"
	"github.com/Liarea/lazyslice/internal/verify"
	"github.com/Liarea/lazyslice/mask"
)

// sourceShapes is every statement shape the run may send to the source before
// the plan exists: introspect's and the planner's. Extract's and verify's are
// built from the plan and registered later (run.registerShapes).
//
// The conversion from each stage's own Statement to pg.Shape is core's job by
// construction: ARCHITECTURE.md section 2's import graph has the stage packages
// importing pipeline and nothing else of the tree, so the wiring that owns both
// sides is the only place that can.
func sourceShapes() []pg.Shape {
	var out []pg.Shape
	for _, s := range introspect.Shapes() {
		out = append(out, pg.Shape{Name: s.Name, SQL: s.SQL})
	}
	for _, s := range plan.Shapes() {
		out = append(out, pg.Shape{Name: s.Name, SQL: s.SQL})
	}
	return out
}

// asStop turns a stage's own refusal into the one type cmd/lazyslice reads.
//
// Every stage returns its own refusal — the stages take no event.Sink and core
// is the only producer of events (ARCHITECTURE.md section 7) — and each carries
// the event code and the ADR-005 exit that go with it. This is where the four
// become one, and it is deliberately exhaustive: an error no stage claimed is
// exit 1, never a code a CI job branches on.
func asStop(err error) error {
	if err == nil {
		return nil
	}
	var already *Stop
	if errors.As(err, &already) {
		return already
	}

	var planRefusal *plan.Refusal
	if errors.As(err, &planRefusal) {
		return &Stop{
			Code: planRefusal.Code, Exit: planRefusal.Exit,
			Table: planRefusal.Table, Column: planRefusal.Column,
			Args: planRefusal.Args, Message: planRefusal.Message, err: err,
		}
	}

	var extractRefusal *extract.Refusal
	if errors.As(err, &extractRefusal) {
		return &Stop{
			Code: extractRefusal.Code, Exit: extractRefusal.Exit, Table: extractRefusal.Table,
			Args: event.Args{
				event.ArgTable:  extractRefusal.Table.String(),
				event.ArgReason: extractRefusal.SQLState,
			},
			Message: extractRefusal.Error(), err: err,
		}
	}

	var transformRefusal *transform.Refusal
	if errors.As(err, &transformRefusal) {
		return &Stop{
			Code: transformRefusal.Code, Exit: transformRefusal.Exit,
			Table: transformRefusal.Col.Table, Column: transformRefusal.Col.Column,
			Args: event.Args{
				event.ArgTable:  transformRefusal.Col.Table.String(),
				event.ArgColumn: transformRefusal.Col.Column,
				event.ArgReason: transformRefusal.Masker,
			},
			Message: transformRefusal.Error(), err: err,
		}
	}

	var loadRefusal *load.Refusal
	if errors.As(err, &loadRefusal) {
		return &Stop{
			Code: loadRefusal.Code, Exit: loadRefusal.Exit, Table: loadRefusal.Table,
			Column: loadRefusal.Object,
			Args: event.Args{
				event.ArgTable:  loadRefusal.Table.String(),
				event.ArgColumn: loadRefusal.Object,
				event.ArgReason: loadRefusal.SQLState,
				// The count a lock-and-recheck refusal names: how many rows the
				// table held when the gate had approved it as empty
				// (ARCHITECTURE.md section 11.2). It is zero for every other load
				// refusal, whose templates do not reference {count}.
				event.ArgCount: strconv.FormatInt(loadRefusal.Rows, 10),
			},
			Message: loadRefusal.Error(), err: err,
		}
	}

	// internal/load/ddl's refusal is a sixth type and it arrives unwrapped:
	// run.go's planStage calls ddl.Recreatable and returns what it gets (before
	// T-0097 it was load.Load that did, as its first statement). Without this
	// case it fell through to the final wrap below and an
	// operator whose Mastodon or GitLab schema has a column default calling one
	// of the application's own functions -- ARCHITECTURE.md §11.1's exit 13,
	// with the table, the column and the dependency all named inside the
	// message -- was told "lazyslice failed for a reason it has no code for; run
	// with --debug" and given exit 1 to branch on
	// (testdata/regressions/002-function-default-refusal-uncoded.sql).
	//
	// The refusal itself is unchanged and stays: §11.1 has no flag that drops
	// the default and carries on. Where it is raised is no longer owed either:
	// §11.1 says at plan and T-0097 moved it there, so this case now converts a
	// refusal that arrives from planStage rather than from the loader.
	var ddlRefusal *ddl.Refusal
	if errors.As(err, &ddlRefusal) {
		return &Stop{
			Code: ddlRefusal.Code, Exit: ddlRefusal.Exit,
			Table: ddlRefusal.Table, Column: ddlRefusal.Object,
			Args: event.Args{
				event.ArgTable:  ddlRefusal.Table.String(),
				event.ArgColumn: ddlRefusal.Object,
				event.ArgReason: ddlRefusal.Dependency,
			},
			// The sentence, not ddlRefusal.Error(): that one prefixes its own
			// code, and Refusal.Error() prefixes it again, so the operator saw
			// "target.schema.not_recreatable.function: ddl:
			// target.schema.not_recreatable.function: ..." twice over.
			Message: ddlRefusalMessage(ddlRefusal), err: err,
		}
	}

	var verifyRefusal *verify.Refusal
	if errors.As(err, &verifyRefusal) {
		return &Stop{
			Code: verifyRefusal.Code, Exit: verifyRefusal.Exit,
			Table: verifyRefusal.Table, Column: verifyRefusal.Column,
			Args: event.Args{
				event.ArgTable:  verifyRefusal.Table.String(),
				event.ArgColumn: verifyRefusal.Column,
				event.ArgCount:  strconv.FormatInt(verifyRefusal.Count, 10),
				event.ArgReason: verifyRefusal.Reason,
			},
			Message: verifyRefusal.Error(), err: err,
		}
	}

	// A cancellation no stage claimed is SIGINT or SIGTERM (cmd/lazyslice
	// installs the handler), and it is exit 130, not exit 1. It is mapped here
	// rather than only in cmd/lazyslice so that the Error event core sends
	// carries the number the process exits with: run.report is the only place a
	// failure becomes an event, and an event saying exit 1 beside a process
	// returning 130 is a CI job branching on the wrong one. Every stage refusal
	// is converted above, so a refusal that merely *caused* a cancellation never
	// reaches this line.
	if errors.Is(err, context.Canceled) {
		return wrap(CodeInterrupted, exitInterrupted, err, "interrupted")
	}

	return wrap(CodeInternal, exitInternal, err, "%s", err.Error())
}

// ddlRefusalMessage renders internal/load/ddl's refusal as one sentence, naming
// the object the way ddl.Refusal.Error() does — table-qualified when there is a
// table, bare when the dependency is on an index or a constraint — and without
// the event code, which Refusal.Error() and the renderer both add for
// themselves.
func ddlRefusalMessage(r *ddl.Refusal) string {
	where := r.Object
	if r.Table.Name != "" {
		where = r.Table.String() + "." + r.Object
	}
	return fmt.Sprintf("%s depends on %s, which lazyslice does not recreate", where, r.Dependency)
}

// gateCode is the event code the gate's verdict carries, with a fallback for a
// refusal that arrived with none: an eligibility with no reason is still a
// refusal, and the line has to say something.
func gateCode(e pipeline.Eligibility) event.Code {
	if e.Reason != "" {
		return e.Reason
	}
	return pg.CodeProbeFailed
}

// refusedTables names the tables the emptiness check found occupied, in order,
// for the refusal's {reason}. It is a list of identifiers and never a count of
// rows: the gate does not count them (pg.RowsNotCounted).
func refusedTables(e pipeline.Eligibility) string {
	if len(e.RowCounts) == 0 {
		return ""
	}
	names := make([]string, 0, len(e.RowCounts))
	for t := range e.RowCounts {
		names = append(names, t.String())
	}
	sort.Strings(names)
	const most = 5
	if len(names) > most {
		return strings.Join(names[:most], ", ") +
			" and " + strconv.Itoa(len(names)-most) + " more"
	}
	return strings.Join(names, ", ")
}

// readOnlyRoleStatement is the block ARCHITECTURE.md section 9 prints as the
// loudest lines in the header when the source role can write: the statements
// that create a read-only role, with this database's own names substituted.
//
// It is rendered from identifiers here rather than linked to a document,
// because "ends in a command, not a doc link" is the rule that section cites
// (research/SQLIT_STUDY.md section 5.5 Scenario C).
func readOnlyRoleStatement(r dsn.Ref) string {
	db := quoteIdent(r.Database)
	return "CREATE ROLE lazyslice_ro LOGIN PASSWORD '…'; " +
		"GRANT CONNECT ON DATABASE " + db + " TO lazyslice_ro; " +
		"GRANT USAGE ON SCHEMA public TO lazyslice_ro; " +
		"GRANT SELECT ON ALL TABLES IN SCHEMA public TO lazyslice_ro;"
}

// modeName is a step's mode as the plan prints it.
func modeName(m pipeline.Mode) string {
	switch m {
	case pipeline.ChildOK:
		return "child_ok"
	case pipeline.ParentOnly:
		return "parent_only"
	case pipeline.Lookup:
		return "lookup"
	case pipeline.SchemaOnly:
		return "schema_only"
	default:
		return "unknown"
	}
}

// stepRows is how many rows a step will move, or -1 where the plan does not know
// (a lookup is copied whole; a schema-only table moves none).
func stepRows(s pipeline.Step) int64 {
	if s.Keys == nil {
		return 0
	}
	return int64(s.Keys.Len())
}

// hexKey is the text form of a masking key, which is what ./lazyslice.secret
// carries and what mask.ParseKey reads back.
func hexKey(k mask.Key) string { return hex.EncodeToString(k[:]) }

// ---------- resolving a name the operator typed ----------

// resolveTable turns the name in a flag or in the yml into a table of this
// source.
//
// A schema-qualified name is taken as it is. A bare name is resolved against the
// catalog and refused when it matches more than one schema: a flag that silently
// picked one of two tables would slice, or skip, the wrong one — and --skip-table
// on the wrong table is a table missing from the snapshot with nothing said.
func resolveTable(name string, schema *pipeline.Schema) (ref.TableRef, error) {
	parts, err := splitQualified(name)
	if err != nil {
		return ref.TableRef{}, err
	}
	switch len(parts) {
	case 2:
		if parts[0] == "" || parts[1] == "" {
			return ref.TableRef{}, fmt.Errorf("%q is not a schema-qualified table name", name)
		}
		return ref.TableRef{Schema: parts[0], Name: parts[1]}, nil
	case 1:
		return resolveBareTable(parts[0], name, schema)
	default:
		return ref.TableRef{}, fmt.Errorf("%q is not a table name: want SCHEMA.TABLE", name)
	}
}

func resolveBareTable(bare, name string, schema *pipeline.Schema) (ref.TableRef, error) {
	if bare == "" {
		return ref.TableRef{}, fmt.Errorf("%q is not a table name", name)
	}
	var found []ref.TableRef
	if schema != nil {
		for i := range schema.Tables {
			if schema.Tables[i].Ref.Name == bare {
				found = append(found, schema.Tables[i].Ref)
			}
		}
	}
	switch len(found) {
	case 1:
		return found[0], nil
	case 0:
		return ref.TableRef{}, fmt.Errorf("%q names no table in the source: qualify it as SCHEMA.TABLE", name)
	default:
		names := make([]string, 0, len(found))
		for _, t := range found {
			names = append(names, t.String())
		}
		sort.Strings(names)
		return ref.TableRef{}, fmt.Errorf("%q is in more than one schema (%s): qualify it",
			name, strings.Join(names, ", "))
	}
}

// resolveColumn turns --unmask's TABLE.COL or SCHEMA.TABLE.COL into a column of
// this source.
//
// It refuses a name that matches nothing, and that is the point: --unmask is the
// one safety rail the operator can pull, and an opt-out that silently never
// applied is indistinguishable, in the output, from one that did.
func resolveColumn(name string, schema *pipeline.Schema) (ref.ColumnRef, error) {
	parts, err := splitQualified(name)
	if err != nil {
		return ref.ColumnRef{}, err
	}
	if len(parts) < 2 || len(parts) > 3 {
		return ref.ColumnRef{}, fmt.Errorf("%q is not a column name: want TABLE.COLUMN or SCHEMA.TABLE.COLUMN", name)
	}
	column := parts[len(parts)-1]
	table, err := resolveTable(strings.Join(quoteEach(parts[:len(parts)-1]), "."), schema)
	if err != nil {
		return ref.ColumnRef{}, err
	}
	if !hasColumn(schema, table, column) {
		return ref.ColumnRef{}, fmt.Errorf("%s has no column %q in the source", table, column)
	}
	return ref.ColumnRef{Table: table, Column: column}, nil
}

func hasColumn(schema *pipeline.Schema, t ref.TableRef, column string) bool {
	if schema == nil {
		return false
	}
	for i := range schema.Tables {
		if schema.Tables[i].Ref != t {
			continue
		}
		for _, c := range schema.Tables[i].Columns {
			if c.Name == column {
				return true
			}
		}
	}
	return false
}

// quoteEach re-quotes parts that were split apart, so that a name with a dot or
// an upper-case letter in it survives being taken to pieces and put back.
func quoteEach(parts []string) []string {
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		out = append(out, quoteIdent(p))
	}
	return out
}

// quoteIdent quotes an identifier that is not already bare lower case, doubling
// an embedded quote, which is both SQL's rule and the yml's (internal/emit).
func quoteIdent(s string) string {
	bare := s != ""
	for i, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r == '_':
		case r >= '0' && r <= '9' && i > 0:
		default:
			bare = false
		}
		if !bare {
			break
		}
	}
	if bare {
		return s
	}
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}

// splitQualified splits a dotted, possibly-quoted name. A dot inside quotes is
// part of the identifier; "" inside a quoted part is one quote.
func splitQualified(s string) ([]string, error) {
	var parts []string
	var cur strings.Builder
	quoted := false
	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case c == '"' && quoted && i+1 < len(s) && s[i+1] == '"':
			cur.WriteByte('"')
			i++
		case c == '"':
			quoted = !quoted
		case c == '.' && !quoted:
			parts = append(parts, cur.String())
			cur.Reset()
		default:
			cur.WriteByte(c)
		}
	}
	if quoted {
		return nil, fmt.Errorf("%q has an unbalanced quote", s)
	}
	return append(parts, cur.String()), nil
}
