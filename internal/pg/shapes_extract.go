// SPDX-License-Identifier: Apache-2.0

package pg

// The statement shapes extract will send to the source.
//
// The wall they exist to avoid is this package's: the planner's statements
// could not be registered at all until the allowlist grammar learned
// {selectlist}, {casts} and {keypred} (tracer.go), and extract's statements are
// the same joins over the same chunks. Reviewing them once, with the
// placeholders, is what keeps the task that writes internal/extract from
// finding the same grammar too narrow and either stalling or registering
// something looser.
//
// **Ownership: these belong in internal/extract, which is a no-op scaffold
// today.** ARCHITECTURE.md §2's import graph has each stage package declaring
// the statements it sends — internal/introspect.Shapes(),
// internal/plan.Shapes() — and internal/core converting and registering them.
// They are here only because the task that reviewed them (T-PGSHAPES) could
// write internal/pg and internal/plan and not internal/extract. The task that
// implements Extract moves them into an internal/extract.Shapes() over the
// statements it really builds and deletes this file; until then the templates
// below are a review of the forms, not a promise about text nobody has written
// yet.
const (
	// extractRowsShape is §12's "chunked typed unnest joins over the snapshot":
	// one table's copied columns for one chunk of that step's identity keys. It
	// is the planner's chunk join with the whole column list in place of the
	// identity columns, so a step's rows are read by key and never by predicate.
	extractRowsShape = `SELECT {selectlist} FROM {ident} t ` +
		`JOIN unnest({casts}) AS k({idents}) ON {keypred} ORDER BY {idents}`

	// extractLookupShape is the read for a Lookup step, which has no key set
	// (ARCHITECTURE.md §2 "Step": Keys is nil for Lookup and SchemaOnly) and is
	// copied whole. What bounds it in the plan is the planner: a table is a
	// lookup only after the bounded count in §3 proved it under a thousand rows
	// in this same snapshot. That is not enough for the template, and the LIMIT
	// is not decoration.
	//
	// Without it this shape is byte-for-byte plan.seed with the bound removed,
	// and the allowlist is one per-Source union that every stage registers into
	// additively (Tracer.Register: nothing removes a shape). So the moment
	// extract's shapes sit beside the planner's — the intended wiring — an
	// unbounded `SELECT ... FROM t ORDER BY ...` over any relation is admitted,
	// which is the statement internal/plan/shapes_test.go asserts must be
	// refused and the second layer internal/plan/shapes.go claims to have
	// (THREAT_MODEL.md T9). The allowlist is what has to hold when the planner
	// is the thing that is wrong, so the bound is in the template too.
	//
	// The bound extract issues is the lookup ceiling the planner already proved
	// the table is under (internal/plan's countProbeLimit, 1001), so it costs a
	// lookup read nothing and stops this being a general whole-table read.
	// TestTheComposedAllowlistStillRefusesAnUnboundedRead pins the composition.
	//
	// Bounded, this template is byte-for-byte plan.seed, so on a Source
	// carrying both a lookup read is traced under whichever of the two
	// registered first (Tracer.match takes the first shape that matches). That
	// costs the trace a name and costs the allowlist nothing, and it goes away
	// with the file: the internal/extract Shapes() that replaces this one knows
	// the lookup tables from the plan and should name each one in its own shape
	// (`FROM "public"."categories" t`), which is narrower than either.
	extractLookupShape = `SELECT {selectlist} FROM {ident} t ORDER BY {idents} LIMIT {int}`
)

// ExtractShapes are the statements ARCHITECTURE.md §2 and §12 give extract. See
// the ownership note above: internal/extract replaces this with its own
// Shapes() and must register what it actually sends.
func ExtractShapes() []Shape {
	return []Shape{
		{Name: "extract.rows", SQL: extractRowsShape},
		{Name: "extract.lookup", SQL: extractLookupShape},
	}
}
