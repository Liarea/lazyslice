// SPDX-License-Identifier: Apache-2.0

package plan

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
)

// What §3.2 does to a real fixture is asserted in plan_integration_test.go
// (Trap6). What is held here is the half of the inference that is a pure
// function of a string: which table names a `_type` value could stand for, and
// which table a set of candidates resolves to. Getting these wrong is silent —
// a candidate that names nothing produces an unmapped value rather than an
// error — so the mapping is pinned by name rather than only by the one fixture
// whose two values both happen to be table names already.

// §3.2's rule is "underscore and pluralise", so the pluralised form comes
// first for every value: a `User` that resolved to a `user` table in preference
// to `users` would be the rule inverted. The un-pluralised forms follow it as a
// documented fallback for the `_type` column that holds a table name (the shape
// testdata/nasty.sql trap 6 carries).
func TestRailsCandidatesAreTheNamesAValueCouldBe(t *testing.T) {
	for _, c := range []struct {
		value string
		want  []string
	}{
		// testdata/nasty.sql trap 6: the value is already the table name, and
		// only the fallback can place it.
		{"people", []string{"peoples", "people"}},
		{"projects", []string{"projectses", "projects"}},
		// Rails writes the class name. `Person` is the irregular that matters:
		// the canonical polymorphic example stores it against a `people` table.
		{"Person", []string{"people", "person"}},
		{"Order", []string{"orders", "order"}},
		{"LineItem", []string{"line_items", "lineitem", "line_item"}},
		{"Company", []string{"companies", "company"}},
		{"Address", []string{"addresses", "address"}},
		// A namespaced class is tried whole and demodulized, because a Rails
		// app may have either table, and the rule's form of each comes before
		// either fallback.
		{"Admin::User", []string{"admin_users", "users", "admin::user", "admin_user", "user"}},
	} {
		if got := railsCandidates(c.value); fmt.Sprint(got) != fmt.Sprint(c.want) {
			t.Errorf("railsCandidates(%q) = %v, want %v", c.value, got, c.want)
		}
	}
}

// Django's is §3.2's rule and nothing else: `<app_label>_<model>`. A bare
// `user` fallback would bind an `auth`/`user` content type to any app's
// `public.user`, which is a wrong edge reported as a resolved one where the
// feature already has a finding — the unmapped value — for a name it cannot
// place.
func TestDjangoCandidatesFollowTheDefaultTableName(t *testing.T) {
	got := djangoCandidates("auth", "user")
	want := []string{"auth_user"}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("djangoCandidates(auth, user) = %v, want %v", got, want)
	}
	if got := djangoCandidates("auth", ""); got != nil {
		t.Errorf("djangoCandidates with no model = %v, want none", got)
	}
	if got := djangoCandidates("", "user"); got != nil {
		t.Errorf("djangoCandidates with no app label = %v, want none: the default table name has both halves", got)
	}
}

// A candidate is resolved against the planned tables, preferring the schema the
// pair is in. A name carried by two other schemas resolves to neither: a guess
// between them would put rows in the slice for a reason the plan cannot state.
func TestTableNamedPrefersThePairsOwnSchema(t *testing.T) {
	p := &run{byName: map[string][]ref.TableRef{
		"people":   {{Schema: "archive", Name: "people"}, {Schema: "public", Name: "people"}},
		"projects": {{Schema: "billing", Name: "projects"}},
		"notes":    {{Schema: "archive", Name: "notes"}, {Schema: "billing", Name: "notes"}},
	}}

	if got, ok := p.tableNamed([]string{"people"}, "public"); !ok || got != (ref.TableRef{Schema: "public", Name: "people"}) {
		t.Errorf("tableNamed(people, public) = %v %v, want public.people", got, ok)
	}
	if got, ok := p.tableNamed([]string{"projects"}, "public"); !ok || got != (ref.TableRef{Schema: "billing", Name: "projects"}) {
		t.Errorf("tableNamed(projects, public) = %v %v, want the one match", got, ok)
	}
	if got, ok := p.tableNamed([]string{"notes"}, "public"); ok {
		t.Errorf("tableNamed(notes, public) = %v, want no answer: two schemas carry it", got)
	}
	// The candidates are tried in order, so a plural that names a table is used
	// only when the singular does not.
	if got, ok := p.tableNamed([]string{"person", "people"}, "public"); !ok || got.Name != "people" {
		t.Errorf("tableNamed([person people]) = %v %v, want public.people", got, ok)
	}
}

// The pair is what the plan reports, and a `_type` value never reaches a
// message at all: no event, reason, refusal or explanation string carries a
// value read from a source row, and no digest of one either
// (docs/reviews/2026-09-09/REVIEW.md finding 2, THREAT_MODEL.md T4 and T13,
// T-0131). unmappedFinding and unknownFindings take an identifier and a
// count — never the value — so a value cannot reach one of these strings by
// construction. A keyed HMAC digest stood here briefly; the orchestrator's
// answer to the 2026-09-14 review finding on it was no digest at all, keyed
// or not, because a keyed digest is still an oracle for anyone holding the
// run key, and the operator who needs the actual values can read the column
// at the source themselves.
func TestUnmappedFindingCarriesOnlyAnIdentifierAndACount(t *testing.T) {
	pair := polymorphicPair{
		Table:   ref.TableRef{Schema: "public", Name: "attachments"},
		TypeCol: "owner_type",
		IDCol:   "owner_id",
	}
	want := "public.attachments.owner_type: 3 distinct values mapping to no table; " +
		"inspect the distinct values of owner_type on public.attachments in the source to see what they are"
	if got := unmappedFinding(pair, 3); got != want {
		t.Errorf("unmappedFinding(pair, 3) = %q, want %q", got, want)
	}
	if got := unmappedFinding(pair, 1); !strings.Contains(got, "1 distinct value mapping to no table") {
		t.Errorf("unmappedFinding(pair, 1) = %q, want the singular form", got)
	}
}

// A `_type` value the walk meets and the sample never produced has no edge, so
// its rows' parents are not followed. That is the sample being a sample, and it
// is only tolerable while it is said out loud: a pair reported as resolved while
// an unknown number of its references were dropped is research/COMPLAINTS.md
// FK-10, the silently narrow slice this feature exists to make impossible.
//
// The integration suite cannot reach this: a sample large enough to be worth
// bounding is larger than a fixture, and every fixture table is analysed, so
// what it holds is what the sample sees. The bookkeeping is held here instead.
func TestUnknownTypeValuesAreReportedNotDropped(t *testing.T) {
	attachments := polymorphicPair{
		Table:   ref.TableRef{Schema: "public", Name: "attachments"},
		TypeCol: "owner_type",
		IDCol:   "owner_id",
	}
	comments := polymorphicPair{
		Table:   ref.TableRef{Schema: "public", Name: "comments"},
		TypeCol: "subject_type",
		IDCol:   "subject_id",
	}
	p := &run{unknownTypes: map[polymorphicPair]*unknownValues{}}

	// Out of pair order and with a repeat, because the walk meets values in the
	// order rows arrive and the plan reports them in an order two runs share.
	p.noteUnknownType(comments, "Ticket")
	p.noteUnknownType(attachments, "Widget")
	p.noteUnknownType(attachments, "Event")
	p.noteUnknownType(attachments, "Widget")

	// One finding per pair, not per value: an identifier and a count, never a
	// value or a digest of one (T-0131).
	want := []string{
		`public.attachments (owner_type, owner_id): 2 distinct values of owner_type the sample did not produce; ` +
			`inspect the distinct values of owner_type on public.attachments in the source to see what they are`,
		`public.comments (subject_type, subject_id): 1 distinct value of subject_type the sample did not produce; ` +
			`inspect the distinct values of subject_type on public.comments in the source to see what they are`,
	}
	got := p.unknownFindings()
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("unknownFindings() =\n%v\nwant\n%v", got, want)
	}
	// None of the three sampled values themselves may appear in the findings:
	// the count is the whole point (T-0131).
	for _, raw := range []string{"Event", "Widget", "Ticket"} {
		for _, line := range got {
			if strings.Contains(line, raw) {
				t.Errorf("unknownFindings() line %q carries the raw value %q", line, raw)
			}
		}
	}

	// The findings are bounded like the sample is: a `_type` column holding
	// free text must not turn the list into a copy of the column, and it stays
	// one finding for the pair — past the bound is a bigger count, not a
	// second line.
	q := &run{unknownTypes: map[polymorphicPair]*unknownValues{}}
	for i := 0; i < polymorphicValueCap*3; i++ {
		q.noteUnknownType(attachments, fmt.Sprintf("value-%03d", i))
	}
	got = q.unknownFindings()
	if len(got) != 1 {
		t.Fatalf("unknownFindings() returned %d entries, want 1: one finding per column regardless of how many "+
			"distinct values were seen", len(got))
	}
	if !strings.Contains(got[0], "more than 50 distinct values") {
		t.Errorf("the finding is %q, want it to say the cap was reached", got[0])
	}
}

// Detection is the first step of §3.2 and it is the one thing that must not
// fire on an edge PostgreSQL already knows: a `_id` column covered by a declared
// foreign key is that edge, and the planner follows it as a constraint.
func TestPolymorphicPairsSkipADeclaredEdge(t *testing.T) {
	attachments := ref.TableRef{Schema: "public", Name: "attachments"}
	comments := ref.TableRef{Schema: "public", Name: "comments"}
	tables := []pipeline.Table{
		{Ref: attachments, Columns: []pipeline.Column{
			{Name: "owner_type", TypeName: "text", TypeOID: oidText},
			{Name: "owner_id", TypeName: "bigint", TypeOID: oidInt8},
		}},
		{Ref: comments, Columns: []pipeline.Column{
			{Name: "subject_type", TypeName: "text", TypeOID: oidText},
			{Name: "subject_id", TypeName: "bigint", TypeOID: oidInt8},
		}},
	}
	outgoing := map[ref.TableRef][]pipeline.ForeignKey{
		comments: {{Name: "comments_subject_id_fkey", Child: comments, ChildCols: []string{"subject_id"}}},
	}
	got := polymorphicPairs(tables, outgoing)
	if len(got) != 1 || got[0].Table != attachments || got[0].TypeCol != "owner_type" {
		t.Fatalf("polymorphicPairs = %v, want only public.attachments (owner_type, owner_id)", got)
	}
	if want := "public.attachments (owner_type, owner_id)"; got[0].String() != want {
		t.Errorf("String() = %q, want %q", got[0].String(), want)
	}
}
