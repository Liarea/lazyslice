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

	// CodeWhereSyntax is exit 2: the --where predicate carries something the
	// source's statement allowlist will not admit inside the seed shape — a
	// statement separator, a comment introducer, a backslash, a dollar sign, or
	// parentheses that do not balance (internal/pg/tracer.go's {where}). It is
	// refused here, naming the character and its position, so that the operator
	// is not told only that "the source refused a statement" and so that a
	// recorded allowlist violation always means a bug in our own SQL generation
	// rather than somebody's regex (where.go, THREAT_MODEL.md T9).
	CodeWhereSyntax event.Code = "plan.refused.where_syntax"

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

	// CodeUnwritable is exit 12: a column the classification masks whose type
	// cannot hold what its category's masker emits -- `credential` on a
	// timestamp, `address` on a tsvector. mask declares, per category, the type
	// tags its generators can be written into (mask/writable.go), and this is
	// where the plan says no. It exists so that internal/transform is never the
	// first place a type mismatch is discovered: transform can only refuse per
	// value, which is exit 7 in the middle of a run with rows already moved
	// (writeback.go, T-0054).
	CodeUnwritable event.Code = "plan.refused.unwritable"

	// CodeUniqueDomain is exit 12: a masked column under a unique index whose
	// widest registered generator cannot emit d_required = n²/2ε distinct
	// values at ε = 10⁻⁶ (ARCHITECTURE.md §5). The refusal names the column, d,
	// d_required and the three escapes; unique.go is the check.
	CodeUniqueDomain event.Code = "plan.refused.unique_domain"

	// CodeEqualityGroup is exit 12: a set of masked columns joined by declared
	// foreign keys, which therefore have to mask to the same value, and no one
	// registered generator fits all of them (ARCHITECTURE.md §5's 2026-09-14
	// amendment; equality.go's fitsGroup). It is deliberately not
	// CodeUniqueDomain, which this refusal borrowed when T-0132 landed: two of
	// the three causes — a generator no member's type can hold, and members that
	// would not mask alike — reach it with no member under a unique index at
	// all, and "is under a unique index" is then a false statement about the
	// operator's schema (T-0132 review).
	CodeEqualityGroup event.Code = "plan.refused.equality_group"

	// CodeDDLLiteral is exit 12: a string literal inside a column default, a
	// CHECK constraint or a generated expression on a column this run does not
	// mask, which a strong validator reads as an email address, a telephone
	// number or a payment card. §11.1 recreates that text verbatim, so the
	// literal would cross into the target as it stands (ARCHITECTURE.md §11.1's
	// 2026-09-14 amendment; ddlliteral.go). The escape is the per-column
	// --unmask, with its reason.
	CodeDDLLiteral event.Code = "plan.refused.ddl_literal"

	// CodeLiteralNotRewritable is exit 13: the same literal inside an object on
	// a *masked* column, where lazyslice can neither leave it nor rewrite it —
	// a CHECK or generated expression, whose meaning is the application's, or a
	// default this stage has no way to substitute into (an array, a document,
	// a nextval, or a run with no key). It is a target-schema refusal for the
	// same reason target.schema.not_recreatable is: the target cannot be built
	// from this source without either leaking or changing what the application
	// checks.
	CodeLiteralNotRewritable event.Code = "target.schema.literal_not_rewritable"

	// CodeTypeLiteral is exit 13: an enum label, or a domain's DEFAULT or
	// CHECK, carries a literal a strong validator hits. ARCHITECTURE.md §11.1
	// recreates a type verbatim — internal/load/ddl writes every enum label as
	// a string literal and replays a domain's whole CREATE statement — so the
	// value crosses into the target exactly as a column DEFAULT does, and the
	// 2026-09-15 red team walked an email address and a phone number through
	// both routes under exit 0 with nothing in the yml.
	//
	// It is 13 and never 12, and it is never rewritten. An enum label cannot
	// be rewritten at all: every row of every column of that type references
	// the label by value, so masking it would either break the column or
	// silently remap rows. A domain's DEFAULT belongs to the type rather than
	// to a column, so there is no single masker that could produce a
	// replacement — the columns using the domain may be masked under different
	// categories, or not masked at all.
	//
	// The escape is --allow-type-literal TYPE=REASON, and it is the only one.
	// This comment used to name --skip-table, which cannot clear this refusal
	// by any route: --skip-table drops a table to *schema only*, so its DDL and
	// every type that DDL names are still recreated, and nothing prunes
	// Schema.Enums or Schema.Domains. Any source schema with one such label was
	// therefore unrunnable, under a message naming a flag with no effect on it
	// (the T-REDFIX review's fourth finding). internal/verify's catalog pass
	// honours the same opt-out through Plan.AllowedTypeLiterals, so the run
	// that gets past this does not fail at exit 9 on the same object.
	CodeTypeLiteral event.Code = "target.schema.type_literal"

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
