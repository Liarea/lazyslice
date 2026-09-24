// SPDX-License-Identifier: Apache-2.0

package plan

import (
	"strconv"
	"strings"

	"github.com/Liarea/lazyslice/internal/event"
	"github.com/Liarea/lazyslice/internal/ref"
)

// Refusal is a plan-time stop with the event code and the exit code that go
// with it. The planner is pure and takes no event.Sink, so a refusal travels
// back as an error and core turns it into the Error event and the process exit
// (ARCHITECTURE.md §7; the same shape internal/classify uses for its codes).
//
// Args carries identifiers and counts only, drawn from the event.ArgKey enum:
// no key value, no predicate and no row ever reaches it (THREAT_MODEL.md T4).
// Message is a developer-facing sentence; what a user sees is rendered from
// internal/event/catalogue.yml by Code.
type Refusal struct {
	Code    event.Code
	Exit    int
	Table   ref.TableRef
	Column  string
	Args    event.Args
	Message string
}

// Error implements error.
func (r *Refusal) Error() string { return string(r.Code) + ": " + r.Message }

// refuse builds a Refusal. Args is always non-nil so that a renderer never has
// to distinguish "no arguments" from "not set".
func refuse(code event.Code, exit int, table ref.TableRef, msg string, args event.Args) *Refusal {
	if args == nil {
		args = event.Args{}
	}
	return &Refusal{Code: code, Exit: exit, Table: table, Args: args, Message: msg}
}

// Refusals is more than one plan-time refusal from a single run.
//
// Dogfood session 1 (T-0318) took nine runs to reach a green verify, each one
// refused on the next single cause in isolation: one join table with no
// identity, then ten masked columns under unique indexes named one at a time,
// then a second such table found only once the first was cleared. Four checks
// — resolveIdentities (no_identity), checkWriteBack (unwritable),
// applySkipAndPrivileges's skip-cannot-drop-a-parent branch (skip_parent) and
// checkUniqueDomain (unique_domain, and its equality_group sibling) — collect
// every refusal they can find instead of stopping at the first, and plan()
// returns them together as this type when any were found, so an operator
// fixes a run's independent causes together rather than one exit-12 at a
// time. Every other refusal in this package is still the single *Refusal it
// always was: plan.go's plan() builds this type only once a collecting check
// has found something to put in it (its own doc comment on run.refusals and
// run.fail has the exact points).
//
// Code, Exit, Table, Column, Args and Message are not fields on this type:
// core.asStop and cmd/lazyslice both read a single refusal's shape to build
// the process's exit code and its "lazyslice: code: message" line, and the
// first member — first in the order plan() runs its checks, which is also
// the order the four are listed above — is what answers for the whole run,
// because every one of them is already ADR-005's exit 12 regardless of which
// is first. internal/core sends one Error event per member, in order, so the
// transcript itself names all of them; only the process's own single exit
// code and the one line cmd/lazyslice prints after it collapse to the first.
type Refusals []*Refusal

// Unwrap exposes every member to errors.As and errors.Is, so a caller that
// still asks "is this error a *plan.Refusal" (asStop's own fallback case,
// and every existing single-refusal test in this package that has not been
// taught about Refusals) finds the first one -- the one whose Code and Exit
// answer for the whole run (see the type's own doc comment) -- without this
// package or internal/core needing two ways to read a plan-time refusal.
func (rs Refusals) Unwrap() []error {
	out := make([]error, len(rs))
	for i, r := range rs {
		out[i] = r
	}
	return out
}

// Error renders every member, one per line, so a bare %v — --debug's trail, a
// test failure message — still shows the whole set and not only the one
// asStop treats as authoritative.
func (rs Refusals) Error() string {
	var b strings.Builder
	for i, r := range rs {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(strconv.Itoa(i + 1))
		b.WriteString(". ")
		b.WriteString(r.Error())
	}
	return b.String()
}
