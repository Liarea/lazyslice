// SPDX-License-Identifier: Apache-2.0

package discover

import "github.com/Liarea/lazyslice/internal/event"

// The event codes this package emits. Every one has a row in
// internal/event/catalogue.yml, which is the source of docs/ERRORS.md.
const (
	// CodeCandidate and CodeCandidateUnreachable are the candidate list: one
	// line per candidate as it resolves (ARCHITECTURE.md §9). Both carry the
	// provenance, because a developer must be able to see why lazyslice thinks
	// a database is theirs.
	CodeCandidate            event.Code = "discover.candidate.found"
	CodeCandidateUnreachable event.Code = "discover.candidate.unreachable"

	// CodeDockerEndpoint names the endpoint rungs 3 and 4 will use and the
	// ADR-008 §3 step it came from.
	CodeDockerEndpoint event.Code = "discover.docker.endpoint"

	// The three ways container discovery is skipped. Each names the endpoint
	// that was actually resolved and the step it came from, never a hard-coded
	// path (ADR-008 §3 "Degradation").
	CodeDockerUnreachable event.Code = "discover.docker.unreachable"
	CodeDockerNotLocal    event.Code = "discover.docker.not_local"
	CodeDockerSSH         event.Code = "discover.docker.ssh"

	// CodeDockerContextUnreadable is step 5 falling back to step 4 with its
	// reason printed, rather than silently.
	CodeDockerContextUnreadable event.Code = "discover.docker.context_unreadable"

	// CodeComposeNoProject is the stated fallback for the working_dir filter
	// (ADR-008 §4): the header prints the provenance it actually used.
	CodeComposeNoProject event.Code = "discover.compose.no_project"

	// CodeComposeStopped names a compose Postgres service that is declared and
	// not running. lazyslice never starts the developer's own services
	// (ADR-008 §7); it prints the command that would.
	CodeComposeStopped event.Code = "discover.compose.stopped"

	// CodeEnvUnusable is a rung-1 name whose value does not parse whole as a
	// Postgres URI — an unexpanded ${PROD_URL}, or a fragment. lazyslice never
	// assembles a DSN from .env fragments (ADR-008 §2).
	CodeEnvUnusable event.Code = "discover.env.unusable"

	// CodeSourceChosen and CodeTargetChosen carry the provenance of each side.
	// They are not the decision header, which internal/core prints; they are
	// how the ladder says which candidate it handed over and under which rung.
	CodeSourceChosen event.Code = "discover.source.chosen"
	CodeTargetChosen event.Code = "discover.target.chosen"

	// CodeSourceNone is exit 3: nothing to read from, with the ladder already
	// printed above it and a command to run. It is core's code, reused so that
	// one run cannot report "no source" under two spellings.
	CodeSourceNone event.Code = "source.refused.none"

	// CodeTargetNone is exit 4: a source was found and nothing target-shaped
	// was, and neither question could be honoured or was answered yes. It is
	// Q1's headless failure (ADR-008 §6), and the flag it names is
	// --create-target where a container could be created and --target where
	// one could not.
	CodeTargetNone event.Code = "target.refused.none"

	// CodeTargetStartTimeout is ADR-008 §6 step 3: the container was started
	// and did not accept a connection inside the readiness budget. Exit 4. The
	// container is left as it was found — still running where it is still
	// running, exited where it died during startup — because lazyslice never
	// removes one, and that is what keeps `docker logs <name>` answerable
	// either way. ADR-008 §6 step 3, ARCHITECTURE.md §9 and question.go's
	// provisionRefusal still say "left running"; the amendment they need is
	// owed and recorded in provision/CLAUDE.md.
	//
	// The budget is provision's, not this package's: 60 s by default and
	// whatever $LAZYSLICE_PROVISION_READY_TIMEOUT names where it is set
	// (T-0075), so the {seconds} the catalogue row renders is the elapsed wait
	// the failure carries and not a constant written down twice.
	//
	// The catalogue row for this code cannot yet name the container or
	// `docker logs <name>`, which ADR-008 §6 asks for: there is no event.ArgKey
	// for a container name, and internal/event was outside the paths of the
	// task that wired Q1' (see this package's CLAUDE.md).
	CodeTargetStartTimeout event.Code = "target.refused.start_timeout"

	// CodeTargetDockerNotLocal is ADR-008 §3's refusal: --create-target against
	// a docker endpoint that is not local, or that does not answer. It is exit
	// 4 naming the endpoint and --docker-host, and it is a THREAT_MODEL.md T2
	// control rather than a phase-5 detail — a container created on someone
	// else's daemon is not local and lazyslice never removes one.
	CodeTargetDockerNotLocal event.Code = "target.refused.docker_not_local"
)

// The ADR-005 exit codes this package returns. They are repeated here rather
// than imported from cmd/lazyslice because the dependency goes the other way.
const (
	exitNoSource = 3
	exitTarget   = 4
)
