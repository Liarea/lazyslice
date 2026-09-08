// SPDX-License-Identifier: Apache-2.0

package core

import "github.com/Liarea/lazyslice/internal/event"

// The event codes core emits in its own right: the ones about the run as a
// whole rather than about a stage's own findings. Every one has a row in
// internal/event/catalogue.yml, which is the source of docs/ERRORS.md.
const (
	// CodeSourceNone is exit 3: nothing to read from. Discovery is not in this
	// build (internal/discover is a scaffold), so it is what a run with no
	// --source gets, and the message names the flag rather than the stage.
	CodeSourceNone event.Code = "source.refused.none"

	// CodeTargetUnset is exit 4: openTarget was reached with no --target and no
	// ladder pick. It used to be miscoded as pg.CodeUnreachable, which reads as
	// a dial failure for a flag that was simply never given; the catalogue
	// string is "target.refused.missing", not "target.refused.none" — that
	// name is already discover.CodeTargetNone's, for the ladder's own "nothing
	// target-shaped found" refusal (internal/discover/codes.go), and a second
	// row for the same code string would make the catalogue ambiguous about
	// which message renders.
	CodeTargetUnset event.Code = "target.refused.missing"

	// CodeSourceChosen and CodeTargetChosen are the decision header's two
	// lines: which database is the source and which is the target, with the
	// provenance and the flag that changes it (ARCHITECTURE.md section 9).
	CodeSourceChosen event.Code = "source.decided"
	CodeTargetChosen event.Code = "target.decided"

	// CodeRoleWritable is the loudest line in the header: the source role can
	// write, and the statement block that creates a read-only one
	// (ARCHITECTURE.md section 9 "Decision header").
	CodeRoleWritable event.Code = "source.role.writable"

	// CodeRoleRefusedWritable is exit 6: --require-read-only-role and the role
	// holds INSERT, UPDATE or DELETE.
	CodeRoleRefusedWritable event.Code = "source.refused.writable_role"

	// CodeSourceViolation is exit 7: the source's shape allowlist refused a
	// statement this run sent. It is a bug in our own SQL generation, and it
	// fails the run rather than being swallowed (THREAT_MODEL.md T9).
	CodeSourceViolation event.Code = "source.refused.statement"

	// CodeTargetTruncating says the target carries our own bound marker and is
	// being reloaded (ARCHITECTURE.md section 11.2).
	CodeTargetTruncating event.Code = "target.marker.bound"

	// The three warnings a bound marker prints when the run that wrote it was
	// not this one (ARCHITECTURE.md section 11.2).
	CodeSecretChanged  event.Code = "target.marker.secret_changed"
	CodeClassChanged   event.Code = "target.marker.classification_changed"
	CodeVersionChanged event.Code = "target.marker.tool_changed"

	// CodeSchemaRead reports the catalog introspect found: the counts a reader
	// can check against their own database.
	CodeSchemaRead event.Code = "introspect.schema.read"

	// CodePlanStep is one line of the plan (ARCHITECTURE.md section 3.5): the
	// table, how it was reached and why.
	CodePlanStep event.Code = "plan.step"

	// CodePlanEstimate is the plan's estimate with the assumption it rests on.
	CodePlanEstimate event.Code = "plan.estimate"

	// CodePlanPolymorphic names a detected <x>_type/<x>_id pair that is not
	// followed, so that research/COMPLAINTS.md FK-10's silently empty slice is
	// impossible (ARCHITECTURE.md section 14).
	CodePlanPolymorphic event.Code = "plan.polymorphic.detected"

	// CodePlanPolymorphicInferred names one followed virtual edge: a
	// discriminator column whose sampled _type values resolved and whose
	// mapping the plan followed in the parent direction (ARCHITECTURE.md
	// section 3.2, amended 2026-09-08). Printed for every entry of
	// plan.Plan.Virtual, so the plan states every virtual edge it will
	// actually walk and not only the pairs it declined.
	CodePlanPolymorphicInferred event.Code = "plan.polymorphic.inferred"

	// CodePlanUnmapped names a sampled _type value that maps to no table. It is
	// the other half of section 3.2 and stays unemitted until the mapping half
	// ships; it is declared because the planner's Unmapped list is what feeds
	// it, and a list with no code to print it is a finding nobody sees.
	CodePlanUnmapped event.Code = "plan.polymorphic.unmapped"

	// CodeSecretWritten, CodeSecretEphemeral, CodeSecretUnprotected and
	// CodeSecretTracked are ARCHITECTURE.md section 9 "The repository", one
	// code per branch, because every branch prints what it did.
	CodeSecretWritten     event.Code = "secret.file.written"
	CodeSecretEphemeral   event.Code = "secret.key.ephemeral"
	CodeSecretUnprotected event.Code = "secret.file.unprotected" //nolint:gosec // G101: an event code, not a credential
	CodeSecretTracked     event.Code = "secret.file.tracked"     //nolint:gosec // G101: an event code, not a credential
	CodeSecretRefusedKey  event.Code = "secret.refused.no_key"
	CodeGitignoreAdded    event.Code = "secret.gitignore.added"
	CodeGitAbsent         event.Code = "secret.git.absent"

	// CodeConfigRead and CodeConfigWritten bracket the yml.
	CodeConfigRead    event.Code = "config.file.read"
	CodeConfigWritten event.Code = "config.file.written"

	// CodeConfigWhereWithheld is exit 2: the committed file records a
	// where_fingerprint and this run passed no --where, so the slice would
	// silently be a different one (ARCHITECTURE.md section 10).
	CodeConfigWhereWithheld event.Code = "config.refused.where_withheld"

	// CodeUsage is exit 2: a flag the operator has to fix, discovered after the
	// schema was read — an --unmask or a --skip-table that names no column or
	// table of this source.
	CodeUsage event.Code = "run.refused.usage"

	// CodeReviewedChanged is exit 12: this run is not the run the operator
	// reviewed. Request.Reviewed carries the schema fingerprint and the two
	// endpoints a preview pass resolved, and --tui's second pass takes a fresh
	// snapshot and walks the discovery ladder again, so the review could
	// otherwise be of a different schema of a different database from the one
	// that is written (core.go, Reviewed). The refusal names what changed and
	// happens after introspect, before classify, the plan and every write.
	//
	// The exit is ADR-005's 12, which that table calls "plan refused": this is a
	// refusal of the plan the operator approved, made before the planner runs,
	// and it is not one of the four cases the table lists. ADR-005 is owed
	// either a fifth case under 12 or an exit of its own; internal/core/CLAUDE.md
	// records it.
	CodeReviewedChanged event.Code = "core.refused.reviewed_changed"

	// CodeInternal is exit 1: something failed that is not one of ADR-005's
	// codes. It exists so that "something went wrong" is never printed as a
	// code a CI job branches on.
	CodeInternal event.Code = "run.refused.internal"

	// CodeInterrupted is exit 130: the context was cancelled, which is SIGINT or
	// SIGTERM (cmd/lazyslice installs the handler). It is a code of its own
	// because cmd/lazyslice returns 130 for a cancellation and core is the only
	// place a failure becomes an event: without it the transcript said exit 1
	// and the process said 130 for the same run.
	CodeInterrupted event.Code = "run.interrupted"

	// CodeProgressDropped is the count ARCHITECTURE.md section 7 asks for: the
	// bounded channel drops Progress events when it is full, "with the drop
	// counted", and this is where the count is printed — once, at the end, so
	// that a transcript missing progress lines says why (channel.go).
	CodeProgressDropped event.Code = "run.progress.dropped"
)

// The exit codes ADR-005 assigns, as core needs them. They are repeated here
// rather than imported from cmd/lazyslice because the dependency goes the other
// way: cmd builds a Request and reads a Stop, and never the reverse.
const (
	exitInternal     = 1
	exitUsage        = 2
	exitNoSource     = 3
	exitTarget       = 4
	exitCredential   = 5
	exitWritableRole = 6
	exitExtractLoad  = 7
	exitDrift        = 10
	exitReviewed     = 12
	exitInterrupted  = 130
)
