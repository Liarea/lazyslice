// SPDX-License-Identifier: Apache-2.0

package plan

import "github.com/Liarea/lazyslice/internal/event"

// The event codes the plan stage renders. Planner.Plan takes no event.Sink —
// core.Run is the only producer of events (ARCHITECTURE.md §7) — so these are
// declared here, beside the refusals that carry them, and emitted by core from
// the *Refusal the planner returns. Every one of them has a row in
// internal/event/catalogue.yml, which is the source of docs/ERRORS.md.
const (
	// CodeNoRoot is exit 2: --root names a table the schema does not have, or
	// the schema has no table to slice from. It is a usage error, not a plan
	// refusal, because nothing about the source decided it.
	CodeNoRoot event.Code = "plan.refused.no_root"

	// CodeKeyColumn is exit 2: --key names a column the table does not have, or
	// a system column. There is no ctid rung on the identity ladder (ADR-005:
	// ctid is not reproducible across VACUUM FULL, so it breaks invariants I3
	// and I5), and this is where asking for one lands.
	CodeKeyColumn event.Code = "plan.refused.key_column"

	// CodeKeyNotUnique is exit 12: an explicit --key was probed like any other
	// candidate and does not identify a row. Accepting it would produce a slice
	// whose rows are silently the wrong ones (testdata/README.md trap 12).
	CodeKeyNotUnique event.Code = "plan.refused.key_not_unique"

	// CodeNoIdentity is exit 12: the identity ladder in §3.4 ran out. The
	// remedy is --key TABLE=COL,COL or --skip-table TABLE.
	CodeNoIdentity event.Code = "plan.refused.no_identity"

	// CodeUnreadable is exit 12: the source role cannot read a table the slice
	// needs as a parent, or cannot read the root (§3.6). The refusal carries
	// the GRANT statement to run.
	CodeUnreadable event.Code = "plan.refused.unreadable"

	// CodeSkipParent is exit 12: --skip-table named a table the slice needs as
	// a parent. Skipping it would leave the slice referentially incomplete.
	CodeSkipParent event.Code = "plan.refused.skip_parent"

	// CodeRowBudget is exit 11: the walk went past --row-budget
	// (THREAT_MODEL.md T11). It names the table that crossed the line.
	CodeRowBudget event.Code = "plan.refused.row_budget"

	// CodeMemoryBudget is exit 11: the key sets plus the residual filter went
	// past --memory-budget (THREAT_MODEL.md T11).
	CodeMemoryBudget event.Code = "plan.refused.memory_budget"

	// CodeNotRecreatable is exit 13: a foreign key the target's schema cannot
	// carry (ARCHITECTURE.md §11.1, ForeignKey.NotRecreatable). It is raised at
	// plan, before the snapshot is used for keys and before anything in the
	// target is dropped.
	CodeNotRecreatable event.Code = "target.schema.not_recreatable"
)

// The exit codes ADR-005 assigns the refusals above.
const (
	exitUsage  = 2
	exitBudget = 11
	exitPlan   = 12
	exitSchema = 13
)
