// SPDX-License-Identifier: Apache-2.0

package plan

import (
	"fmt"

	"github.com/Liarea/lazyslice/internal/event"
	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
)

// The FK-pair refusal (T-0253, T-0257).
//
// internal/classify's unknownColumnsBesideCertain raises an unrecognised
// character column beside a certain personal column, but never alone when the
// column sits at either end of a validated foreign key: masking one end and
// leaving the other copied would put the identical values in the target
// unmasked on one side of the join. When a partner already carries a decision
// this rail must not override -- a type-conflicting name hit ARCHITECTURE.md
// §4 pins, or measured two-letter-code evidence the column is a code lookup,
// not personal data -- fkPairs (internal/classify/classify.go) refuses to
// raise either end and records that refusal on both: Decision.Refused names
// the reason and Decision.RefusedPartner names the other column
// (internal/pipeline/classify.go's own comment on the field).
//
// Until this file existed, nothing read that signal: both ends stayed
// Masked == false, and internal/transform copied them verbatim under exit
// 0 -- the pre-T-0253 leak, reopened for exactly the shape the round-5 red
// team's own fix text asked internal/plan to refuse instead
// (internal/classify/CLAUDE.md's "what this does not close" paragraph). This
// check reads Decision.Refused the way checkWriteBack reads Decision.Masked:
// over the tables still in scope, before the first key is fetched, and it
// stops at the first one it finds -- the refusal is symmetric across the
// pair, so naming one column names the whole pair.

// checkFKPairRefusal refuses a plan when a classify decision carries a
// non-empty Refused. The escape is --unmask on both columns of the pair:
// unmasking only one still leaves the other's identical production values in
// the target with no acknowledgement that they are there, the same "every
// column of the group or none" rule checkUniqueDomain's equality groups
// already hold an operator to.
func (p *run) checkFKPairRefusal() error {
	if p.cls == nil {
		return nil
	}
	for i := range p.tables {
		t := &p.tables[i]
		if !p.inScope[t.Ref] {
			continue
		}
		for _, col := range t.Columns {
			cref := ref.ColumnRef{Table: t.Ref, Column: col.Name}
			d, ok := p.cls.Decisions[cref]
			if !ok || d.Refused == "" {
				continue
			}
			partner := p.cls.Decisions[d.RefusedPartner]
			if unmaskedByOperator(d) && unmaskedByOperator(partner) {
				// Both ends have been explicitly accepted unmasked, each
				// with its own reason on record (pipeline.Unmask.Reason) --
				// the escape below is what taking it looks like.
				continue
			}
			return p.refuseFKPair(t.Ref, col.Name, d)
		}
	}
	return nil
}

// unmaskedByOperator reports whether a decision was left unmasked because the
// operator said so -- a --unmask flag or a committed lazyslice.yml entry --
// rather than because nothing raised it. It is the same distinction
// classify.go's applyPrior draws through Decision.Source; a column fkPairs
// refused to raise carries Source == pipeline.ByClassifier (the zero value)
// until an opt-out names it, whatever Decision.Masked says.
func unmaskedByOperator(d pipeline.Decision) bool {
	return d.Source == pipeline.ByFlagUnmask || d.Source == pipeline.ByYmlUnmask
}

// refuseFKPair builds the exit-12 refusal: both columns of the pair, the
// reason fkPairs recorded, and the --unmask escape for each -- in the style
// of uniqueDomainReason. event.ArgKey has no key for a foreign-key partner
// (the same borrowing plan.refused.equality_group's own catalogue comment
// makes), so the pair and the escapes travel in {reason}.
func (p *run) refuseFKPair(t ref.TableRef, col string, d pipeline.Decision) *Refusal {
	cref := ref.ColumnRef{Table: t, Column: col}
	partner := d.RefusedPartner
	reason := fmt.Sprintf("%s; --unmask %s=REASON and --unmask %s=REASON accept both ends unmasked",
		d.Refused, cref, partner)
	r := refuse(CodeFKPair, exitPlan, t,
		fmt.Sprintf("%s %s", cref, reason),
		event.Args{
			event.ArgTable:  t.String(),
			event.ArgColumn: col,
			event.ArgReason: reason,
		})
	r.Column = col
	return r
}
