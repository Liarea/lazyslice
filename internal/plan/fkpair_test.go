// SPDX-License-Identifier: Apache-2.0

package plan

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Liarea/lazyslice/internal/event"
	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
)

// The FK-pair refusal, held without a database, the way writeback_test.go
// holds checkWriteBack: a schema and a classification built by hand, so a
// revert of checkFKPairRefusal (or of the call to it in plan.go) fails a unit
// test rather than waiting for make torture to notice.

// fkPairTables is region_lookup/members, the same shape
// TestFKPairRefusedWhenPartnerIsTwoLetterCodes (internal/classify/
// redteam_test.go) classifies: a lookup table referenced by a validated
// foreign key from members.tag, beside members.email.
func fkPairTables() (members, lookup ref.TableRef, schema *pipeline.Schema) {
	lookup = ref.TableRef{Schema: "public", Name: "region_lookup"}
	members = ref.TableRef{Schema: "public", Name: "members"}
	schema = &pipeline.Schema{
		Tables: []pipeline.Table{
			{
				Ref: lookup,
				Columns: []pipeline.Column{
					{Name: "tag", TypeName: "text"},
				},
				PK: []string{"tag"},
			},
			{
				Ref: members,
				Columns: []pipeline.Column{
					{Name: "id", TypeName: "bigint", TypeOID: 20},
					{Name: "email", TypeName: "text"},
					{Name: "tag", TypeName: "text"},
				},
				PK: []string{"id"},
			},
		},
		FKs: []pipeline.ForeignKey{{
			Name:       "members_tag_fkey",
			Child:      members,
			ChildCols:  []string{"tag"},
			Parent:     lookup,
			ParentCols: []string{"tag"},
			Validated:  true,
		}},
	}
	return members, lookup, schema
}

// blockedPairClassification is the Decision pair fkPairs itself writes when a
// validated foreign key's partner cannot be raised the same way -- neither
// end masked, both carrying Refused and RefusedPartner
// (internal/classify/classify.go's unknownColumnsBesideCertain, T-0253). The
// reason text is the exact fragment classify's own reasons.go renders
// (fk_pair_refused), so this pins the same string the classifier would.
func blockedPairClassification(members, lookup ref.TableRef) *pipeline.Classification {
	memberTag := ref.ColumnRef{Table: members, Column: "tag"}
	lookupTag := ref.ColumnRef{Table: lookup, Column: "tag"}
	return &pipeline.Classification{
		Decisions: map[ref.ColumnRef]pipeline.Decision{
			memberTag: {
				Col:            memberTag,
				Category:       pipeline.CatNone,
				Masked:         false,
				Refused:        "cannot be raised without copying foreign-key partner public.region_lookup.tag: refusing the pair rather than masking one end",
				RefusedPartner: lookupTag,
			},
			lookupTag: {
				Col:            lookupTag,
				Category:       pipeline.CatNone,
				Masked:         false,
				Refused:        "cannot be raised without copying foreign-key partner public.members.tag: refusing the pair rather than masking one end",
				RefusedPartner: memberTag,
			},
		},
	}
}

// TestFKPairIsRefusedAtPlan is T-0257: internal/classify already refuses to
// raise either end of the pair and names both columns on Decision.Refused /
// Decision.RefusedPartner; before this check read that signal, both stayed
// Masked == false and internal/transform copied them verbatim under exit 0
// (the pre-T-0253 leak, reopened for this one shape). Replacing
// checkFKPairRefusal's body with `return nil` fails this test.
func TestFKPairIsRefusedAtPlan(t *testing.T) {
	t.Parallel()
	members, lookup, schema := fkPairTables()
	cls := blockedPairClassification(members, lookup)

	r := &countingReader{}
	_, err := New().Plan(context.Background(), r, schema, cls, pipeline.PlanRequest{Root: &members})
	var refusal *Refusal
	if !errors.As(err, &refusal) {
		t.Fatalf("Plan returned %v, want a *plan.Refusal", err)
	}
	if refusal.Code != CodeFKPair {
		t.Errorf("Code = %q, want %q", refusal.Code, CodeFKPair)
	}
	if refusal.Exit != 12 {
		t.Errorf("Exit = %d, want 12", refusal.Exit)
	}
	if refusal.Table != members {
		t.Errorf("Table = %s, want %s", refusal.Table, members)
	}
	if refusal.Column != "tag" {
		t.Errorf("Column = %q, want %q", refusal.Column, "tag")
	}
	// Both columns of the pair have to be named, or the operator cannot tell
	// which two --unmask flags the escape needs.
	for _, want := range []string{
		"public.members.tag", "public.region_lookup.tag",
		"--unmask public.members.tag=REASON", "--unmask public.region_lookup.tag=REASON",
	} {
		if !strings.Contains(refusal.Error(), want) {
			t.Errorf("message %q does not name %q", refusal.Error(), want)
		}
	}
	if got := refusal.Args[event.ArgTable]; got != members.String() {
		t.Errorf("args[table] = %q, want %q", got, members)
	}
	if got := refusal.Args[event.ArgColumn]; got != "tag" {
		t.Errorf("args[column] = %q, want %q", got, "tag")
	}
	if got := refusal.Args[event.ArgReason]; !strings.Contains(got, "public.region_lookup.tag") {
		t.Errorf("args[reason] = %q, want the partner column named", got)
	}
	// This is a plan-time refusal, before the first key is fetched: only the
	// privilege pass and the write-back check's own reads (none, here) should
	// have run.
	if r.queries > 1 {
		t.Errorf("the planner sent %d statements before refusing; only the privilege pass should run", r.queries)
	}
}

// fkPairRun builds a *run directly, the way unique_test.go's uniqueRun does,
// rather than through Plan: checkFKPairRefusal needs no row count and no
// query, only the scoped table list and the classification, and driving it
// through a fake reader would need a count query answered with a real row for
// every table's lookup check -- a cost only the end-to-end test (above) is
// worth paying.
func fkPairRun(members, lookup ref.TableRef, schema *pipeline.Schema, cls *pipeline.Classification) *run {
	return &run{
		schema:  schema,
		tables:  schema.Tables,
		inScope: map[ref.TableRef]bool{members: true, lookup: true},
		cls:     cls,
	}
}

// TestFKPairRefusalIsClearedWhenBothEndsAreUnmasked is the escape's other
// side: --unmask on only one column of the pair must not lift the refusal,
// because the other end's identical values would still reach the target with
// nothing recorded that they were left there -- the same "every column of
// the group or none" rule equality.go's equalityNote already holds an
// operator to. Only once both carry the operator's own acknowledgement
// (Decision.Source == ByFlagUnmask/ByYmlUnmask, the same field
// classify.go's applyPrior sets) does the run stop refusing.
func TestFKPairRefusalIsClearedWhenBothEndsAreUnmasked(t *testing.T) {
	t.Parallel()
	members, lookup, schema := fkPairTables()

	t.Run("NeitherUnmasked", func(t *testing.T) {
		cls := blockedPairClassification(members, lookup)
		p := fkPairRun(members, lookup, schema, cls)
		if err := p.checkFKPairRefusal(); err == nil {
			t.Fatal("checkFKPairRefusal returned nil, want a refusal")
		}
	})

	t.Run("OnlyOneUnmasked", func(t *testing.T) {
		cls := blockedPairClassification(members, lookup)
		d := cls.Decisions[ref.ColumnRef{Table: members, Column: "tag"}]
		d.Source = pipeline.ByFlagUnmask
		cls.Decisions[ref.ColumnRef{Table: members, Column: "tag"}] = d
		p := fkPairRun(members, lookup, schema, cls)
		if err := p.checkFKPairRefusal(); err == nil {
			t.Fatal("checkFKPairRefusal returned nil: unmasking only one end of the pair must not clear it")
		}
	})

	t.Run("BothUnmasked", func(t *testing.T) {
		cls := blockedPairClassification(members, lookup)
		memberTag := ref.ColumnRef{Table: members, Column: "tag"}
		lookupTag := ref.ColumnRef{Table: lookup, Column: "tag"}
		dm := cls.Decisions[memberTag]
		dm.Source = pipeline.ByFlagUnmask
		cls.Decisions[memberTag] = dm
		dl := cls.Decisions[lookupTag]
		dl.Source = pipeline.ByFlagUnmask
		cls.Decisions[lookupTag] = dl
		p := fkPairRun(members, lookup, schema, cls)
		if err := p.checkFKPairRefusal(); err != nil {
			t.Fatalf("checkFKPairRefusal refused a pair both ends of which the operator explicitly unmasked: %v", err)
		}
	})
}

// reconciledPairClassification is the metabase shape T-0258 narrows the
// refusal for: fkPairs refused the pair the way blockedPairClassification
// does (both Refused/RefusedPartner set, neither Masked at that point in the
// pipeline), but a later, separate classify pass -- propagateKeys or
// sameColumnName -- went on to carry the identical category onto both ends
// anyway, the way core_session.id's credential decision reaches
// login_history.session_id through FK propagation regardless of what fkPairs
// decided (docs/TORTURE.md's own T-0257 section). Refused stays set --
// nothing clears it -- but Masked and Category now agree on both ends.
func reconciledPairClassification(members, lookup ref.TableRef) *pipeline.Classification {
	cls := blockedPairClassification(members, lookup)
	memberTag := ref.ColumnRef{Table: members, Column: "tag"}
	lookupTag := ref.ColumnRef{Table: lookup, Column: "tag"}
	dm := cls.Decisions[memberTag]
	dm.Masked = true
	dm.Category = pipeline.CatFreeText
	cls.Decisions[memberTag] = dm
	dl := cls.Decisions[lookupTag]
	dl.Masked = true
	dl.Category = pipeline.CatFreeText
	cls.Decisions[lookupTag] = dl
	return cls
}

// TestFKPairRefusalIsSkippedWhenALaterPassReconcilesThePair is T-0258: a
// refusal fkPairs recorded before a later, separate pass (propagateKeys,
// sameColumnName) brought both ends into agreement anyway -- masked, under
// the same final category -- buys nothing, the metabase
// core_session.id/login_history.session_id shape docs/TORTURE.md's T-0257
// section names. Narrowing checkFKPairRefusal to skip that one shape must
// not touch the type-conflict and two-letter-code shapes, which
// TestFKPairIsRefusedAtPlan and TestFKPairRefusalIsClearedWhenBothEndsAreUnmasked
// above still hold: their partner never gets masked at all, so
// reconciledByALaterPass reports false for them.
func TestFKPairRefusalIsSkippedWhenALaterPassReconcilesThePair(t *testing.T) {
	t.Parallel()
	members, lookup, schema := fkPairTables()
	cls := reconciledPairClassification(members, lookup)

	p := fkPairRun(members, lookup, schema, cls)
	if err := p.checkFKPairRefusal(); err != nil {
		t.Fatalf("checkFKPairRefusal refused a pair a later pass already reconciled: %v", err)
	}
}

// TestFKPairRefusalStillFiresWhenOnlyOneEndIsMasked is the reconciliation
// check's own control: one end masked and the other still not is not the
// reconciled shape -- the partner never got raised at all, the type-conflict
// and two-letter-code shape -- so the refusal must still fire.
func TestFKPairRefusalStillFiresWhenOnlyOneEndIsMasked(t *testing.T) {
	t.Parallel()
	members, lookup, schema := fkPairTables()
	cls := blockedPairClassification(members, lookup)
	memberTag := ref.ColumnRef{Table: members, Column: "tag"}
	dm := cls.Decisions[memberTag]
	dm.Masked = true
	dm.Category = pipeline.CatFreeText
	cls.Decisions[memberTag] = dm

	p := fkPairRun(members, lookup, schema, cls)
	if err := p.checkFKPairRefusal(); err == nil {
		t.Fatal("checkFKPairRefusal returned nil: only one end masked is not a reconciled pair")
	}
}

// TestFKPairRefusalStillFiresWhenCategoriesDiffer is the reconciliation
// check's other control: both ends masked, but under different categories,
// is not the shape a downstream group would unify -- the refusal must still
// fire.
func TestFKPairRefusalStillFiresWhenCategoriesDiffer(t *testing.T) {
	t.Parallel()
	members, lookup, schema := fkPairTables()
	cls := reconciledPairClassification(members, lookup)
	lookupTag := ref.ColumnRef{Table: lookup, Column: "tag"}
	dl := cls.Decisions[lookupTag]
	dl.Category = pipeline.CatCredential
	cls.Decisions[lookupTag] = dl

	p := fkPairRun(members, lookup, schema, cls)
	if err := p.checkFKPairRefusal(); err == nil {
		t.Fatal("checkFKPairRefusal returned nil: differing categories is not a reconciled pair")
	}
}

// TestFKPairRefusalStillFiresWhenOneEndIsOperatorUnmaskedAndTheOtherMasked is
// the metabase shape itself, not the reduction: an operator's own --unmask on
// one end (Source == pipeline.ByFlagUnmask, Decision.Masked == false, for a
// reason unrelated to the pair) alongside propagateKeys carrying the
// identical category onto the other end, which genuinely gets masked
// (Decision.Masked == true). An earlier version of reconciledByALaterPass
// treated this as resolved and skipped the refusal; that is exactly the
// shape internal/classify's own markNeverMasked comment describes as broken
// -- a masked child whose validated-FK parent is copied verbatim, loaded with
// the edge NOT VALID, and failed by internal/verify/fk.go's orphan count at
// exit 8 (I1, THREAT_MODEL.md T8) -- so the refusal must still fire here and
// the operator must --unmask both ends to take the escape
// (docs/TORTURE.md's own T-0257 and T-0258 sections have the corrected
// account; the metabase fixture never demonstrated the failure only because
// login_history.session_id is NULL in every fixture row).
func TestFKPairRefusalStillFiresWhenOneEndIsOperatorUnmaskedAndTheOtherMasked(t *testing.T) {
	t.Parallel()
	members, lookup, schema := fkPairTables()
	cls := blockedPairClassification(members, lookup)
	memberTag := ref.ColumnRef{Table: members, Column: "tag"}
	lookupTag := ref.ColumnRef{Table: lookup, Column: "tag"}
	dm := cls.Decisions[memberTag]
	dm.Category = pipeline.CatCredential
	dm.Masked = false
	dm.Source = pipeline.ByFlagUnmask
	cls.Decisions[memberTag] = dm
	dl := cls.Decisions[lookupTag]
	dl.Category = pipeline.CatCredential
	dl.Masked = true
	cls.Decisions[lookupTag] = dl

	p := fkPairRun(members, lookup, schema, cls)
	if err := p.checkFKPairRefusal(); err == nil {
		t.Fatal("checkFKPairRefusal returned nil: one end operator-unmasked and the other masked verbatim-copies the unmasked end's value across a validated FK and must still refuse")
	}
}

// TestFKPairRefusalStillFiresWhenNeitherEndIsResolved is
// reconciledByALaterPass's own control on the "resolved" half: two ends
// sharing a category is not enough on its own if neither end ever actually
// masked or was explicitly unmasked by the operator -- that is fkPairs' own
// blocked-pair shape with nothing having moved since, and must still refuse.
func TestFKPairRefusalStillFiresWhenNeitherEndIsResolved(t *testing.T) {
	t.Parallel()
	members, lookup, schema := fkPairTables()
	cls := blockedPairClassification(members, lookup)
	memberTag := ref.ColumnRef{Table: members, Column: "tag"}
	lookupTag := ref.ColumnRef{Table: lookup, Column: "tag"}
	dm := cls.Decisions[memberTag]
	dm.Category = pipeline.CatCredential
	cls.Decisions[memberTag] = dm
	dl := cls.Decisions[lookupTag]
	dl.Category = pipeline.CatCredential
	cls.Decisions[lookupTag] = dl

	p := fkPairRun(members, lookup, schema, cls)
	if err := p.checkFKPairRefusal(); err == nil {
		t.Fatal("checkFKPairRefusal returned nil: a shared category with neither end masked nor operator-unmasked is not resolved")
	}
}

// TestFKPairWithNoRefusalIsNotRefusedAtPlan is the control: an ordinary
// classification, with neither column's Refused set, must not trip this
// check. Without it, a check that refused every table with a foreign key
// would pass the tests above.
func TestFKPairWithNoRefusalIsNotRefusedAtPlan(t *testing.T) {
	t.Parallel()
	members, lookup, schema := fkPairTables()
	cls := &pipeline.Classification{Decisions: map[ref.ColumnRef]pipeline.Decision{}}

	p := fkPairRun(members, lookup, schema, cls)
	if err := p.checkFKPairRefusal(); err != nil {
		t.Fatalf("checkFKPairRefusal refused a schema with no Decision.Refused set: %v", err)
	}
}
