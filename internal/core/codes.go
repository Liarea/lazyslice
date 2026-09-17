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

	// CodeTargetSameCluster is ARCHITECTURE.md §9 rule 1's warning: the target
	// is a different database on the *same cluster* as the source. §9 and
	// THREAT_MODEL.md T2 both require it to be printed, internal/pg has
	// computed Eligibility.SameCluster since the gate was written, and until
	// the 2026-09-15 red team nothing in this package or internal/render read
	// the field — so a run that wrote a masked slice into the production
	// server's own `postgres` maintenance database said nothing about it and
	// exited 0. The gate's own test asserted the boolean and never the line,
	// which is why the omission survived.
	CodeTargetSameCluster event.Code = "target.warn.same_cluster"

	// CodeTargetGateSameCluster is T-0184 / ADR-013's gate-stage escalation
	// (review finding 1): the ladder already chose one target and the gate's
	// own Eligibility.SameCluster — the authoritative system_identifier/cluster
	// identity signal, arriving too late for discover to have refused on it
	// (openTarget's comment on e.SameCluster) — says it is on the source's own
	// cluster, headlessly. It is a distinct code from discover's
	// CodeTargetHeadlessSameCluster on purpose: that one's catalogue message
	// says "every reachable candidate is on {host}", which is true of the
	// ladder's own pre-selection refusal and not of this one, which is about
	// one already-chosen target the ladder's cheap clusterKey check missed
	// (a pooler, an SSH tunnel, host.docker.internal vs 127.0.0.1).
	CodeTargetGateSameCluster event.Code = "target.refused.gate_same_cluster"

	// CodeSourceStandby is T-0241's header line (round-4 red team,
	// docs/reviews/2026-09-15-redteam/round4-still-leaking.json, "a read
	// replica"): the source answered pg_is_in_recovery() true. Unlike the
	// cluster-identity fields, this is a direct, positive statement rather
	// than an inference from what two identities happen to share, and it is
	// printed whenever the source is a standby, not only headlessly — the
	// operator watching an interactive run is exactly who should be told the
	// database being read is not itself the primary.
	CodeSourceStandby event.Code = "source.warn.standby"

	// CodeSourceStandbyNoTarget is T-0241's second rail, exit 4: a headless
	// run — --yes, or no controlling terminal — whose source is a streaming
	// standby and whose --target was not named refuses outright, because
	// nothing this cheap can tell a standby's own primary apart from an
	// unrelated server by address alone (round-4's second reproduction, "a
	// target URL taken from a compose file that points at prod"). It is
	// independent of, and does not wait on, Eligibility.SameCluster: that
	// signal can still be fooled by a data_directory that genuinely differs
	// between a standby and its primary, and this rail does not depend on
	// the cluster identity being able to prove anything at all.
	CodeSourceStandbyNoTarget event.Code = "source.refused.standby_no_target"

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

	// CodePlanUnmapped reports, per polymorphic pair, how many distinct sampled
	// _type values map to no table; it never carries a value (T-0131). It is
	// the other half of section 3.2 and stays unemitted until the mapping half
	// ships; it is declared because the planner's Unmapped list is what feeds
	// it, and a list with no code to print it is a finding nobody sees.
	CodePlanUnmapped event.Code = "plan.polymorphic.unmapped"

	// CodePlanPendingKeyDefault is section 11.1 arm 1's finding for a run with
	// no masking key yet: a plan-only run with neither $LAZYSLICE_SECRET nor a
	// committed secret file plans anyway rather than refusing (T-0161), and
	// this is the one line it prints instead of masking — which masked
	// defaults will be masked once a key exists. It carries plan.Plan's
	// PendingKeyDefaults joined into one sentence, never fired for a run that
	// writes: internal/core resolves a key in full before planStage for any
	// such run, so PendingKeyDefaults is always empty there.
	CodePlanPendingKeyDefault event.Code = "plan.masked_default.pending_key"

	// CodeTypeLiteralAllowed names one --allow-type-literal opt-out this run
	// honours — from the flag or carried forward from the committed yml's
	// types: block — mirroring what the column equivalent already shows in a
	// masked/copied column's own reason (classify's "unmask_yml"/"unmask_flag"
	// fragments): THREAT_MODEL.md T1's A4b prices a type opt-out as turning off
	// both looks at that type's recreated DDL, so the run states, in the open,
	// which type and why, rather than only the absence of the exit-13 refusal
	// it would otherwise have hit.
	CodeTypeLiteralAllowed event.Code = "plan.type_literal.allowed"

	// CodeTypeLiteralOptOutExpired names a type whose --allow-type-literal
	// opt-out in the committed yml is not honoured this run: the type's
	// fingerprint (names.go's typeFingerprint) no longer matches the one
	// recorded, or the type is no longer in the source's Schema.Enums/Domains
	// at all. The column equivalent is classify.CodeColumnOptOutExpired; this
	// is its type-literal counterpart, raised here rather than in
	// internal/classify because a type opt-out has no classify-stage decision
	// to expire.
	CodeTypeLiteralOptOutExpired event.Code = "plan.type_literal.opt_out_expired"

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

	// CodeSecretGitignoreNegated, CodeSecretGitignoreUnverifiable and
	// CodeSecretGitignoreNotIgnored are the 2026-09-17 round 6 review of
	// T-0251: CodeSecretEphemeral's "cannot write .gitignore" is true only
	// when .gitignore itself could not be read or appended to
	// (repo.State.GitignoreAppendable false). .gitignore can be written to
	// successfully and the secret still not be protected — a later rule
	// un-ignoring the entry just added (repo.State.GitignoreNegatedBy names
	// it as "<source>:<lineno>"), or git being absent from PATH so the entry
	// could never be checked against git at all — and printing the
	// unwritable-file message for either case sends the operator to fix a
	// .gitignore that already holds the right line. A fourth shape —
	// .gitignore written, git on PATH and able to check, and the path still
	// not ignored with no negating rule to blame — gets its own code too
	// rather than folding into one of the other two and naming a cause that
	// is not the true one.
	CodeSecretGitignoreNegated      event.Code = "secret.gitignore.negated"
	CodeSecretGitignoreUnverifiable event.Code = "secret.gitignore.unverifiable"
	CodeSecretGitignoreNotIgnored   event.Code = "secret.gitignore.not_ignored"

	// CodeSecretSymlink and CodeSecretPermissive are the 2026-09-15 red team's
	// two findings against ARCHITECTURE.md §9 "The repository" and
	// THREAT_MODEL.md T6.
	//
	// The first: os.WriteFile follows a symlink, and both the .gitignore entry
	// and the `git ls-files --error-unmatch` tracked check were applied to the
	// link path rather than to what it resolves to. A ./lazyslice.secret left
	// as a symlink into a cloud-synced folder therefore took the key — T13's
	// guess-confirmation oracle for every snapshot ever made with it — out of
	// the repository entirely, while the transcript said "added
	// lazyslice.secret to .gitignore" and "wrote a new masking key to
	// ./lazyslice.secret".
	//
	// The second: T6 promises the key file is "created 0600" and nothing
	// re-checked an existing one, so a mode-0644 key on a shared machine or in
	// a CI image was readable by every account and the run said nothing.
	CodeSecretSymlink    event.Code = "secret.refused.symlink"
	CodeSecretPermissive event.Code = "secret.refused.permissive"

	// CodeSecretHardlink is the 2026-09-16 round 2 red team's R2-15: a hard
	// link on the secret file gives it a second name no path check, symlink or
	// otherwise, can see, since the file the run reads and writes is still
	// exactly the file it thinks it is. Exit 5, the same shape as
	// CodeSecretPermissive, because both say the key may already have been
	// read under a name .gitignore never protected.
	CodeSecretHardlink event.Code = "secret.refused.hardlink"

	// CodePasswordCommandWithheld is R2-15's sibling finding, R2-16: a
	// --password-command whose argv looks like it embeds a value rather than
	// fetching one (an `echo` or `printf` given the password as a literal
	// argument, most plainly) is withheld from lazyslice.yml rather than
	// written verbatim into a file whose header promises it never contains a
	// secret.
	CodePasswordCommandWithheld event.Code = "secret.password_command.withheld"

	// CodePasswordCommandFailed is T-0213: --password-command was set, no
	// earlier password source resolved for the endpoint (a password in the
	// DSN, $PGPASSWORD, ~/.pgpass), and the command itself did not supply one
	// — a non-zero exit, a timeout, or empty stdout. {reason} names which of
	// those it was and nothing else: never the command's stdout, which is the
	// value under refusal here (THREAT_MODEL.md T5).
	CodePasswordCommandFailed event.Code = "secret.password_command.failed"

	// CodeConfigRead and CodeConfigWritten bracket the yml.
	CodeConfigRead    event.Code = "config.file.read"
	CodeConfigWritten event.Code = "config.file.written"

	// CodeConfigWhereWithheld is exit 2: the committed file records a
	// where_fingerprint and this run passed no --where, so the slice would
	// silently be a different one (ARCHITECTURE.md section 10).
	CodeConfigWhereWithheld event.Code = "config.refused.where_withheld"

	// CodeConfigMappingFileUnsupported is exit 2: the committed file names
	// mapping_file for a column. ADR-012 defers the mapping-file contract past
	// v1 — nothing reads the CSV or applies a replacement
	// (docs/reviews/2026-09-09/REVIEW.md finding 10) — so naming it is refused
	// by name rather than silently ignored or round-tripped. T-0142 implements
	// the full contract.
	CodeConfigMappingFileUnsupported event.Code = "config.refused.mapping_file"

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

	// CodeQuarantineFailed is a warning: a residual-class verify failure (exit
	// 9) means the target holds personal data, closeRun tried to drop every
	// table this run loaded (load.DropLoaded), and that drop itself failed on
	// {table}. The run still exits on the verify failure's own code — that is
	// the finding worth reporting — but the operator needs to know the target
	// was not actually emptied, because the next run's gate will still truncate
	// it, and until then the data named in the verify failure is still there
	// (T-0133, 2026-09-14).
	CodeQuarantineFailed event.Code = "run.quarantine.failed"
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
	// exitResidual is 9: verify's residual scan, an unconfirmable hit, or the
	// second net. closeRun compares a *verify.Refusal's Exit against this to
	// decide whether a verify failure is the residual-class one T-0133 makes
	// core drop every table the run loaded for (verify/codes.go defines the
	// same 9 as its own unexported exitResidual; this is core's copy of the
	// number for the same reason exitExtractLoad above is one).
	exitResidual    = 9
	exitDrift       = 10
	exitReviewed    = 12
	exitInterrupted = 130
)
