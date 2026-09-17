// SPDX-License-Identifier: Apache-2.0

package core

import (
	"fmt"
	"strconv"

	"github.com/Liarea/lazyslice/internal/discover"
	"github.com/Liarea/lazyslice/internal/event"
	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/plan"
	"github.com/Liarea/lazyslice/internal/ref"
)

// maxRootCandidates is ADR-008 §3.1 and §7's "?" listing: the ranked top
// five, never the whole schema.
const maxRootCandidates = 5

// rootQuestion is ADR-008 §6's Q2: "root table? [<top-scoring table>]".
//
// It runs in execute, after introspectStage and before planStage, for every
// mode that reaches this point — ModeIntrospect, ModeDoctor and ModeClassify
// have already returned out of execute by the time it is called, so "the mode
// goes on to plan" needs no check here.
//
// It asks nothing at all, and takes the default silently, whenever any of
// these hold: --root was given; this request pins a Reviewed root from an
// earlier --tui pass (Request.Reviewed, T-0271 review finding 5); a committed
// lazyslice.yml already names a root; the run is headless by discover's own
// definition (--yes, or no controlling terminal); or discovery already asked
// Q1 or Q1' this run (r.askedQ1, from discover.Result.Asked — the
// one-question rule). Whatever happens, the chosen table is printed as a
// decision beside --root, exactly as the source and target decisions already
// print regardless of how their own endpoint was decided.
//
// The ranking is internal/plan's own (RankRoots, over CollapseForRanking's
// collapsed tables and foreign keys), exported for exactly this: this
// question and the planner's own chooseRoot must not be able to disagree
// about which table is top, or about which table a typed answer names.
// Ranking the raw introspected schema instead of the collapsed one let a
// partition leaf outrank its own root here while chooseRoot, which only ever
// sees the collapsed set, picked a different table entirely (T-0271 review).
// The answer — from the prompt, or the silent default — is kept as a
// resolved ref.TableRef on r.qRoot, which planRequest prefers over
// r.req.Root (run.go): a genuine override is validated once, against the
// same collapsed scope, rather than round-tripped through an unquoted
// "schema.name" string that a second, disagreeing parse could misread.
func (r *run) rootQuestion() error {
	if r.req.Root != "" {
		// resolveTable is planRequest's job for this case, exactly as it
		// always has been; a name that does not resolve is that refusal, not
		// a decision printed here first.
		t, err := resolveTable(r.req.Root, r.schema)
		if err != nil {
			// planRequest's own --root case raises this exact refusal
			// (plan.CodeNoRoot) a few statements later; printing a decision
			// for a name that does not resolve would be worse than printing
			// none.
			return nil //nolint:nilerr // deferred to planRequest's own --root refusal
		}
		r.send(event.Plan, event.Decision, CodeRootChosen, event.Args{
			event.ArgTable:  t.String(),
			event.ArgReason: "named by --root",
			event.ArgFlag:   "--root",
		})
		return nil
	}
	if r.req.Reviewed != nil && r.req.Reviewed.Root != (ref.TableRef{}) {
		// The --tui second pass: the operator already saw this root on the
		// plan screen and approved it, so this is not a second, independent
		// decision — it is the same one, restated, and asking again could in
		// principle answer differently from a second schema read (T-0271
		// review, finding 5). planRequest reads the same field directly rather
		// than round-tripping it through r.qRoot or r.req.Root.
		r.send(event.Plan, event.Decision, CodeRootChosen, event.Args{
			event.ArgTable:  r.req.Reviewed.Root.String(),
			event.ArgReason: "already reviewed",
			event.ArgFlag:   "--root",
		})
		return nil
	}
	if r.prior != nil && r.prior.Root != (ref.TableRef{}) {
		r.send(event.Plan, event.Decision, CodeRootChosen, event.Args{
			event.ArgTable:  r.prior.Root.String(),
			event.ArgReason: "recorded in " + r.req.ConfigPath,
			event.ArgFlag:   "--root",
		})
		return nil
	}

	scoped, scopedFKs := plan.CollapseForRanking(r.schema.Tables, r.schema.FKs)
	ranked := plan.RankRoots(scoped, scopedFKs)
	if len(ranked) == 0 {
		// Nothing to default to; the planner's own exit-2 refusal names this.
		return nil
	}
	def := ranked[0]
	table, reason := def.Table, def.Reason()

	if !r.askedQ1 && !discover.Headless(discover.Options{
		Yes: r.req.Yes, Prompter: r.req.prompter, NoControllingTerminal: r.req.noTerminal,
	}) {
		chosen, err := r.askRoot(def, ranked, scoped)
		if err != nil {
			return err
		}
		if chosen != (ref.TableRef{}) && chosen != def.Table {
			// A genuine override: named at the prompt, whether that answer
			// happened to be typed out or came back from "?" and a second
			// try. Kept as a resolved ref.TableRef on r.qRoot rather than
			// round-tripped through ref.TableRef.String() back onto
			// r.req.Root: that string is unquoted ("schema.name"), and a
			// schema or table name containing a dot survived resolveTable's
			// own quoted parse here only to be split on the wrong dot by
			// planRequest's second, disagreeing parse of the same table
			// (T-0271 review). planRequest prefers r.qRoot over r.req.Root
			// for exactly this reason, the same way it already prefers
			// r.prior.Root.
			table, reason = chosen, "named at the prompt"
			r.qRoot = &chosen
		}
		// chosen == def.Table (Enter, or the default's own name typed back)
		// leaves r.qRoot nil, same as taking the default silently: the
		// planner's own defaultRoot recomputes it, deterministically the same
		// table, from the identical ranking.
	}

	r.send(event.Plan, event.Decision, CodeRootChosen, event.Args{
		event.ArgTable:  table.String(),
		event.ArgReason: reason,
		event.ArgFlag:   "--root",
	})
	return nil
}

// askRoot puts Q2 to the controlling terminal and loops until a valid answer
// or the terminal goes away.
//
// Enter (an empty line) is Ask's own default, so it never reaches this
// function's resolveTable call at all — it comes back as def.Table's own
// name and resolves to exactly that table. "?" prints the ranked candidates
// (printRootCandidates) and re-asks. A name that resolves to nothing —
// unknown, ambiguous across schemas, or naming a table this run's own scope
// does not carry — re-asks with the reason (resolveTable's own error, the
// same one --root would be refused with, for the first two; a membership
// check against scope for the third). ADR-008 states a re-prompt-once rule
// only for a yes/no bracket (§7); it is silent on how many times an
// unresolved name may re-ask here, so this loops until a valid answer or
// EOF, which then takes the default exactly as every other headless case
// does (the terminal going away is Ask's own ErrNoTerminal, treated the same
// as any other Ask error: take the default, ask no more).
//
// scope is rootQuestion's own CollapseForRanking result: resolveTable's
// schema-qualified branch takes a name as given with no catalog lookup at
// all (names.go), which let an unknown qualified name — "public.nope", a
// typo a bare name would have caught — through to a decision line naming a
// table that does not exist, one this run then refused over minutes later at
// planStage. Checking the *same* set chooseRoot's own p.inScope enforces
// closes that gap for a bare name too: a partition leaf, which resolveTable
// can resolve (schema.Tables still carries it) but which is not a step
// chooseRoot will ever plan from (T-0271 review).
//
// The second return is nil error whenever the default should simply be
// taken (no controlling terminal, or the terminal went away); a non-nil error
// only ever comes from a stage this run must stop for, which nothing here
// currently raises, but the signature matches every other stage method's for
// the caller's own sake.
func (r *run) askRoot(def plan.RootCandidate, ranked []plan.RootCandidate, scope []pipeline.Table) (ref.TableRef, error) {
	p, done, ok := discover.OpenPrompter(discover.Options{
		Yes: r.req.Yes, Prompter: r.req.prompter, NoControllingTerminal: r.req.noTerminal,
	})
	if !ok {
		return ref.TableRef{}, nil
	}
	defer done()

	question := "root table? [" + def.Table.String() + "]"
	for {
		answer, err := p.Ask(question, def.Table.String())
		if err != nil {
			// The terminal went away mid-question: the headless answer
			// arriving late, same as Confirm's own ErrNoTerminal case.
			return ref.TableRef{}, nil
		}
		if answer == "?" {
			r.printRootCandidates(ranked)
			continue
		}
		t, terr := resolveTable(answer, r.schema)
		if terr == nil && !inRootScope(t, scope) {
			terr = fmt.Errorf("%q names no table this run can root from", answer)
		}
		if terr != nil {
			r.send(event.Plan, event.Warn, CodeRootUnknown, event.Args{
				event.ArgReason: terr.Error(),
			})
			continue
		}
		return t, nil
	}
}

// inRootScope reports whether t is one of scope's tables: the membership
// chooseRoot's own p.inScope enforces, over the identical CollapseForRanking
// set rootQuestion ranked from.
func inRootScope(t ref.TableRef, scope []pipeline.Table) bool {
	for i := range scope {
		if scope[i].Ref == t {
			return true
		}
	}
	return false
}

// printRootCandidates is ADR-008 §3.1 and §7's "?": the ranked top five with
// their score components, through the line printer (r.send) rather than
// written to stderr directly — until the Bubble Tea screens land, this is
// the line printer.
func (r *run) printRootCandidates(ranked []plan.RootCandidate) {
	n := len(ranked)
	if n > maxRootCandidates {
		n = maxRootCandidates
	}
	for i := 0; i < n; i++ {
		c := ranked[i]
		r.send(event.Plan, event.Info, CodeRootCandidate, event.Args{
			event.ArgCount:  strconv.Itoa(i + 1),
			event.ArgTable:  c.Table.String(),
			event.ArgReason: c.Reason(),
		})
	}
}
