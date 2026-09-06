// SPDX-License-Identifier: Apache-2.0

package plan

import (
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
