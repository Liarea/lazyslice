// SPDX-License-Identifier: Apache-2.0

//go:build integration

package invariants

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/goccy/go-yaml"
	"github.com/jackc/pgx/v5"

	"github.com/Liarea/lazyslice/mask"
)

// TestI2NothingFlaggedSurvives is invariant I2: no flagged column in the
// target holds a value the classifier's own detectors would flag.
//
// Three halves, because each of the first two alone can be fooled and neither
// of them looks at what the masker actually produced.
//
//  1. The tool's own opinion: `lazyslice classify --json` is pointed at the
//     target and must find nothing to mask in any column the run neither masked
//     nor opted out — §6 item 4's "every unmasked, non-opted-out column". This
//     is that second net run from outside the process, and it catches a masker
//     that let a category through on a column the 200-row sample
//     under-represented.
//  2. A grep the tool has no say in: every email address and every phone
//     number that exists in the source is looked for in every cell of the
//     target. A classifier that never flagged a column at all passes the first
//     half and fails this one — which is the failure this suite exists to make
//     impossible to miss.
//  3. A comparison the run cannot scope: for every column the run itself
//     recorded as masked, the distinct values in the target and the distinct
//     values in the source must not overlap.
//
// The third half exists because the first two leave one shape wide open, and
// it is not a hypothetical one. Half 1 is scoped by the run's own yml —
// readColumnScope exempts every column carrying a `masker:` — so a run that
// wrote `masker:` against every column has no in-scope column left and passes
// vacuously. Half 2 knows exactly two categories, email and phone. So a
// pipeline that implemented the email and phone maskers and copied everything
// else through verbatim passes both halves with pagila's first_name,
// last_name, address, postal_code and staff.password landing in the target
// byte-identical to production. That is also what a correct classifier plus a
// masker registry that falls back to passthrough for a category nobody has
// written yet looks like from the outside.
//
// Half 3 needs no knowledge of categories or codes, and widening the masked
// set cannot dodge it: every column added to that set adds an assertion rather
// than removing one. What is excluded from it is exactly what §5 says must
// reuse an admissible value — see assertMaskedValuesAreNew — and generated and
// surrogate columns, which §4 never masks and which therefore never carry a
// `masker:`.
//
// All three halves are about values in the target, so all three are vacuous on
// a target with no rows in it. assertTargetHoldsMaskedRows is the guard, and
// it asks for more than "not empty": every masked column whose source column
// holds a value has to arrive holding one too.
func TestI2NothingFlaggedSurvives(t *testing.T) {
	ctx := context.Background()

	for _, f := range fixtures {
		t.Run(f.name, func(t *testing.T) {
			db := start(ctx, t, f)
			source := connect(ctx, t, db.source)

			sourceCells := scanCells(ctx, t, source)
			sourceLiterals := collectPersonalLiterals(sourceCells)
			if len(sourceLiterals) == 0 {
				t.Fatalf("I2: no email address or phone number was found in the %s source, so the grep "+
					"below would pass whatever the tool did; the detectors in scan_test.go and the fixture "+
					"have drifted apart", f.name)
			}

			db.snapshot(ctx, t, f, f.root, f.take)

			scope := readColumnScope(t, db.configPath())
			target := connect(ctx, t, db.target)
			assertMaskedColumnsExist(ctx, t, f.name, source, target, scope)

			targetCells := scanCells(ctx, t, target)
			assertTargetHoldsMaskedRows(t, f.name, sourceCells, targetCells, scope)

			t.Run("classifier", func(t *testing.T) { assertClassifierFindsNothing(ctx, t, db, scope) })
			t.Run("grep", func(t *testing.T) {
				assertNoSourceLiteralSurvives(t, targetCells, sourceLiterals)
			})
			t.Run("values", func(t *testing.T) {
				assertMaskedValuesAreNew(t, f.name, sourceCells, targetCells, scope,
					admissibleDomains(ctx, t, source),
					emittingColumns(ctx, t, db.configPath(), source, target))
			})
		})
	}
}

// assertMaskedColumnsExist fails when a key of the emitted yml's `columns:`
// map names no column of either database.
//
// It runs before the two guards below, because both of them are driven by that
// map and both treat a key that matches nothing as a column with no values to
// look at: assertTargetHoldsMaskedRows puts it in `skipped` and
// assertMaskedValuesAreNew skips it for an empty source set. A masked column
// that matches nothing is therefore exempt from every assertion in this file,
// silently, and the exemption grows with each key that fails to match — which
// is reporting clean by omission, the shape flaggedColumns already refuses.
//
// The two ways a key can fail to match are both live. §10's examples are all
// unquoted lower case, so the yml's spelling of a quoted identifier is
// undecided and `public."LegacyCustomer"."EmailAddress"` is as likely as the
// bare form (traps 9 and 23); parseColumnRef takes either. And a partitioned
// table's rows live in its leaves while the yml names the root (§3.3), which
// is why the source's catalogue is consulted for the root as well
// (allColumns includes relkind 'p').
func assertMaskedColumnsExist(
	ctx context.Context, t *testing.T, fixture string, source, target *pgx.Conn, scope columnScope,
) {
	t.Helper()

	known := allColumns(ctx, t, source)
	for ref := range allColumns(ctx, t, target) {
		known[ref] = true
	}

	var unknown []string
	for _, ref := range scope.maskedColumns() {
		if known[ref] {
			continue
		}
		unknown = append(unknown, scope.masked[ref].key+"  (read as "+ref.String()+")")
	}
	if len(unknown) == 0 {
		return
	}
	t.Fatalf("I2: %d of the %d column(s) the %s run's emitted yml records as masked name no column of "+
		"either database, so every assertion in this test would skip them and report clean by "+
		"omission. The identifier is compared unquoted and a partition leaf is attributed to its "+
		"root (§3.3), so this is a key that names nothing rather than a spelling difference:\n  %s",
		len(unknown), len(scope.masked), fixture, strings.Join(unknown, "\n  "))
}

// assertTargetHoldsMaskedRows fails when the target holds nothing for the
// three halves below to look at.
//
// "Nothing" has three shapes and all of them pass I2 silently: a target with
// no rows at all; a target that loaded rows but not into any column the run
// said it masked; and a target that loaded one masked column and left the rest
// empty, which is the shape a partly-implemented masker produces. A pipeline
// that copied the root's --take rows and stopped satisfies every other
// invariant in this package too, so this is where that is caught.
//
// Three exemptions, all of them things §5, §10 and ADR-010 say are correct
// rather than things that make the test easier. A column whose masker is
// `null` has no value to arrive with, by definition. A column whose masker is
// `derived_text` has none either: ADR-010 masks a tsvector to the *empty*
// tsvector — the column is derived from text this run may have masked, so
// re-deriving it would carry the source's own words into the target — and
// pagila's public.film.fulltext arrives empty, which scanCells drops with
// every other empty value. Reading that as "the masker's output was never
// loaded" fails a run that did exactly what the ADR says. And a column whose
// source holds no non-empty value anywhere cannot produce one in the target
// either: NULL stays NULL and the empty string stays the empty string (§5), so
// demanding a value there would fail a correct run. Everything else has to be
// there.
//
// All three are counted as skipped rather than waved through, so the vacuity
// guard at the end still fires on a run whose every masked column is one of
// them. `null` is counted for the same reason as the other two and not for a
// weaker one: a run whose entire masked set is `masker: "null"` has given the
// three halves below nothing to examine, and letting it fall through the switch
// silently would leave `skipped` short of `len(scope.masked)`, the guard quiet,
// and I2 green over a target nobody looked at.
func assertTargetHoldsMaskedRows(t *testing.T, fixture string, sourceCells, targetCells []cell, scope columnScope) {
	t.Helper()

	if len(targetCells) == 0 {
		t.Fatalf("I2: the %s target holds no non-null value in any table, so all three halves of this "+
			"invariant pass whatever the masker did", fixture)
	}

	sourceHasValue := columnsWithValues(sourceCells)
	targetHasValue := columnsWithValues(targetCells)
	loadedTables := map[tableRef]bool{}
	for _, c := range targetCells {
		loadedTables[c.Column.Table] = true
	}

	var empty, skipped []string
	for _, ref := range scope.maskedColumns() {
		switch {
		case targetHasValue[ref]:
		case scope.producesNoValue(ref):
			// masker: "null" — nothing to look for, and nothing the halves
			// below can examine either, so it is counted.
			skipped = append(skipped, ref.String())
		case scope.producesEmptyValue(ref):
			// masker: derived_text — the correct output is the empty tsvector
			// (ADR-010), which scanCells drops like every other empty value.
			skipped = append(skipped, ref.String())
		case !sourceHasValue[ref]:
			// The source column is NULL or '' everywhere, and §5 keeps both.
			skipped = append(skipped, ref.String())
		case !loadedTables[ref.Table]:
			// The whole table is absent from the target (SchemaOnly, skipped,
			// or unreached). assertSliceReached is what says whether that is
			// allowed; this half is about the masker, not the walk.
			skipped = append(skipped, ref.String())
		default:
			empty = append(empty, ref.String())
		}
	}

	if len(empty) > 0 {
		t.Fatalf("I2: the %s target holds no value in %d of the %d column(s) the emitted yml records as "+
			"masked, and each of them holds values in the source, so the masker's output for them was "+
			"never loaded and the halves below are about columns nobody masked:\n  %s",
			fixture, len(empty), len(scope.masked), strings.Join(empty, "\n  "))
	}
	if len(skipped) == len(scope.masked) {
		t.Fatalf("I2: not one of the %s target's %d masked column(s) carries a value that could be "+
			"examined (all of them are NULL or '' in the source, set to NULL by masker \"null\", "+
			"masked to the empty tsvector by derived_text, or in a table the target does not hold), "+
			"so every assertion below is about nothing", fixture, len(scope.masked))
	}
}

// assertMaskedValuesAreNew is the half the run cannot scope: no value in a
// masked column of the target may be a value of that column in the source.
//
// It is driven by the run's own masked list, which is what makes it
// undodgeable. Recording every column as masked, which is how a pipeline would
// blind the classifier half, adds assertions here rather than removing them;
// recording none of them fails readColumnScope's vacuity guard.
//
// It compares distinct values rather than rows, because the target's rows are
// a subset and there is no key to line them up with.
//
// # What §5 says must overlap, and is therefore excluded
//
// Disjointness is the right property only for a column whose generator has
// room to avoid the source's values. §5 names four cases where it has not, and
// each of them is a *correct* run that this assertion would otherwise report
// as "copied through":
//
//   - NULL and the empty string, which §5 keeps as they are and §6 item 6
//     lists as stated false negatives. scanCells drops both, so they never
//     reach here.
//   - An empty array and an empty JSON document, for the same reason and by
//     the same words: nasty's people.alt_emails carries '{}'::text[] on row
//     90028 (trap 15, "an empty array left empty") and events.payload is
//     replaced whole with `{}` (§4, trap 16b). Cast to text both are the
//     non-empty string `{}`, so preservedEmpty drops them here instead.
//   - A small admissible domain. §5: when `d < 2 × distinct(samples)` the
//     masking is "a stable substitution over a small alphabet", the column is
//     listed under `small_domain:`, and for a special category the masker
//     "collapses the column to one fixed label (the first enum label, or the
//     type's zero value)". nasty's people.marital_status is exactly that: a
//     special category by name, six labels, collapsed to 'single' — which is
//     row 90007's real value (trap 24). people.email_verified is the boolean
//     shape of the same thing: whatever a boolean masker emits is in
//     {true, false}, which is the source's own set. Disjointness is
//     unsatisfiable for these columns, so they are excluded by two signals:
//     the run's own `small_domain:` list (§10), and the target catalogue's own
//     count of the values the column can hold (admissibleDomains) measured
//     against §5's own `2 × distinct` threshold. The catalogue signal is there
//     because a run that simply omitted the column from `small_domain:` must
//     not be able to turn this exclusion into a failure — or the exclusion
//     into a way to dodge it.
//
// An excluded column is still covered by everything else in this file: it is
// in the classifier half, in the grep half, and in
// assertTargetHoldsMaskedRows. What it is not covered by is a §5 property no
// invariant can check from outside — that the small-domain column was listed
// in the yml and in the plan, and that a special category was collapsed rather
// than substituted. That needs the sample the classifier took, which is inside
// the run; it is a `lazyslice verify` assertion for phase 4, not one this
// suite can make.
//
// One way this could fail on a correct run: §6 item 6 lists "a masked value
// coinciding with another row's real value" as a stated false negative of the
// residual scan, and a generator drawing from an embedded word list can emit a
// name that is genuinely in the source. ADR-015 makes that a property of the
// masker rather than of the fixture: for a column whose masker has a
// vocabulary (mask.Emitting — person_name), this half holds the column to what
// that masker promises instead of to disjointness. Every masked value must be
// one mask.Emits accepts, and no row may hold its own source value (equal over
// letters and digits, by primary key); a masked value equal to another row's
// value is logged, because on a correct run it is expected. Every other column
// keeps the disjointness rule, and the grep half keeps its position for every
// email and phone in the source.
func assertMaskedValuesAreNew(
	t *testing.T, fixture string, sourceCells, targetCells []cell, scope columnScope,
	domains map[columnRef]int64, emitting map[columnRef]emittingColumn,
) {
	t.Helper()

	sourceValues := valuesByColumn(sourceCells, scope.masked)
	targetValues := valuesByColumn(targetCells, scope.masked)

	var survived, excluded, overlapped []string
	compared := 0
	for _, ref := range scope.maskedColumns() {
		want := sourceValues[ref]
		if len(want) == 0 {
			continue
		}
		if why := smallDomainReason(ref, scope, domains, len(want)); why != "" {
			excluded = append(excluded, ref.String()+": "+why)
			continue
		}
		compared++

		if e, ok := emitting[ref]; ok {
			// ADR-015: a column whose masker draws real words from a list is
			// held to what that masker promises — every masked value is one it
			// could have produced, and no row keeps its own source value — and
			// a masked value equal to *another* row's real value is logged, not
			// failed, because on a correct run it is expected.
			survived = append(survived, e.failures...)
			if n := overlapCount(want, targetValues[ref]); n > 0 {
				overlapped = append(overlapped, ref.String()+": "+strconv.Itoa(n)+" of "+
					strconv.Itoa(len(targetValues[ref]))+" distinct masked value(s) equal another row's value")
			}
			continue
		}

		var kept, example = 0, ""
		for value := range targetValues[ref] {
			if !want[value] {
				continue
			}
			kept++
			if example == "" {
				example = value
			}
		}
		if kept == 0 {
			continue
		}
		survived = append(survived, ref.String()+": "+strconv.Itoa(kept)+" of "+
			strconv.Itoa(len(targetValues[ref]))+" distinct value(s) in the target are values of that "+
			"column in the source; one of them is "+truncate(example))
	}

	if len(overlapped) > 0 {
		sort.Strings(overlapped)
		t.Logf("I2: %d masked column(s) of the %s target draw from a name list (ADR-015) and hold a name that "+
			"is also a real value elsewhere in the column; no row kept its own and every value is on the list, "+
			"so this is the coincidence the residual scan explains, not a copy:\n  %s",
			len(overlapped), fixture, strings.Join(overlapped, "\n  "))
	}
	if len(excluded) > 0 {
		sort.Strings(excluded)
		t.Logf("I2: %d masked column(s) of the %s target are outside this comparison because §5 requires "+
			"their masked values to come from the same admissible set as the source's:\n  %s",
			len(excluded), fixture, strings.Join(excluded, "\n  "))
	}
	if compared == 0 {
		t.Fatalf("I2: every one of the %s run's %d masked column(s) is either empty in the source or "+
			"excluded as small-domain, so this half asserts nothing at all. Either the fixture has lost "+
			"the columns this check is for, or the run recorded only small-domain columns as masked",
			fixture, len(scope.masked))
	}
	if len(survived) == 0 {
		return
	}
	t.Errorf("I2: the %s run recorded %d column(s) as masked, %d of them were compared, and %d still "+
		"hold the source's own values, so those columns were copied through rather than masked:\n  %s",
		fixture, len(scope.masked), compared, len(survived), strings.Join(survived, "\n  "))
}

// smallDomainReason says why a masked column's masked values are allowed to be
// the source's own, or "" when they are not.
//
// distinctInSource stands in for §5's `distinct(samples)`: the classifier's
// sample is inside the run and this suite cannot see it, and the source's own
// distinct count is the closest thing it can measure. It is an over-estimate,
// which makes the exclusion narrower rather than wider.
func smallDomainReason(ref columnRef, scope columnScope, domains map[columnRef]int64, distinctInSource int) string {
	if scope.masked[ref].smallDomain {
		return "the run lists it under `small_domain:` in the emitted yml (§10)"
	}
	d, known := domains[ref]
	if !known {
		return ""
	}
	if d >= int64(2*distinctInSource) {
		return ""
	}
	return "its type admits " + strconv.FormatInt(d, 10) + " value(s) and the source holds " +
		strconv.Itoa(distinctInSource) + " distinct one(s), so §5's `d < 2 × distinct` holds and " +
		"masking it is a substitution over that set (or, for a special category, a collapse to one " +
		"of its values)"
}

// preservedEmpty reports a value §5 keeps as it is because it is empty: an
// empty array or an empty JSON document, rendered as text. NULL and the empty
// string never reach here, because scanCells drops them.
func preservedEmpty(value string) bool {
	return value == "{}" || value == "[]"
}

// columnsWithValues is the set of columns holding at least one non-NULL,
// non-empty value.
func columnsWithValues(cells []cell) map[columnRef]bool {
	out := map[columnRef]bool{}
	for _, c := range cells {
		out[c.Column] = true
	}
	return out
}

// valuesByColumn collects the distinct values of the columns in want, dropping
// the empty collections §5 preserves.
func valuesByColumn(cells []cell, want map[columnRef]maskedColumn) map[columnRef]map[string]bool {
	out := map[columnRef]map[string]bool{}
	for _, c := range cells {
		if _, ok := want[c.Column]; !ok {
			continue
		}
		if preservedEmpty(c.Value) {
			continue
		}
		if out[c.Column] == nil {
			out[c.Column] = map[string]bool{}
		}
		out[c.Column][c.Value] = true
	}
	return out
}

// assertClassifierFindsNothing runs `classify --json` against the target.
//
// The assertion is scoped to §6 item 4, which covers "every unmasked,
// non-opted-out column". Two kinds of column are therefore outside it. A
// column the run masked with a category masker is *expected* to classify as
// its category — a masked email is still an email. A column carrying an
// `unmask:` block keeps the source's own values in the target by design (§10's
// public.film.description), so the classifier is expected to flag it too, and
// a run `verify` passes with exit 0 must not fail here. Both come out of the
// emitted yml, which is the run's own statement of which columns they are.
func assertClassifierFindsNothing(ctx context.Context, t *testing.T, db *databases, scope columnScope) {
	t.Helper()

	// The positive control, and the reason the code list below can be trusted.
	//
	// flaggedCodePrefixes is a claim about a catalogue that phase 4 writes. If
	// the classify stage lands spelling its decisions any other way, every
	// predicate here matches nothing, `hits` is empty for every possible
	// target, and this half of I2 passes unconditionally on a classifier that
	// is flagging every column it sees. So the same predicate is first run
	// against the source, which the grep half has just proved is full of
	// addresses and numbers: on that input it has to find something.
	//
	// The negative control this test cannot supply (a source value planted in a
	// masked target column must make `lazyslice verify` exit 9 naming the table
	// and column) is scheduled as tracker task T-0035 in phase 4.
	if flagged := flaggedColumns(t, classifyEvents(ctx, t, db, db.source, "the source")); len(flagged) == 0 {
		t.Fatalf("I2: `classify --json` on the source flagged no column under any of the codes this "+
			"test knows (%s), and the source is the fixture whose addresses and numbers the grep half "+
			"proves are there. The classify stage has landed with codes flaggedCodePrefixes does not "+
			"name, so the assertion on the target would pass on any output at all: put the codes the "+
			"stage emits (internal/event/catalogue.yml) into that list.",
			strings.Join(flaggedCodePrefixes, ", "))
	}

	var hits []string
	for _, e := range flaggedColumns(t, classifyEvents(ctx, t, db, db.target, "the target")) {
		// The stream's own spelling of the identifier, parsed the same way the
		// yml's is, so an event naming public."LegacyCustomer"."EmailAddress"
		// and a yml naming public.LegacyCustomer.EmailAddress are one column.
		// An event this test cannot parse is reported rather than dropped: the
		// alternative is to treat it as outside the net, which is the
		// clean-by-omission failure again.
		ref, ok := parseColumnRef(e.Table + "." + e.Column)
		if ok && scope.outsideSecondNet[ref] {
			// §6 item 4: a column a category masker owns, or one the run was
			// told to leave alone, is outside the net.
			continue
		}
		name := e.Table + "." + e.Column
		if !ok {
			name += " (unparseable as schema.table.column, so it could not be matched against the yml)"
		}
		hits = append(hits, name+" ("+e.Code+")")
	}
	if len(hits) > 0 {
		sort.Strings(hits)
		t.Errorf("I2: the classifier flags %d column(s) of the target that the run neither masked nor "+
			"opted out:\n  %s", len(hits), strings.Join(hits, "\n  "))
	}
}

// classifyEvents runs `classify --json` against one database and parses the
// stream.
//
// --config points into a directory that holds no yml, and that is the load
// bearing half of the pair. --no-config suppresses only the *write* (§8: "Do
// not write lazyslice.yml"); the read still defaults to ./lazyslice.yml, and
// the working directory here is db.dir, which is exactly where the snapshot
// under test just wrote its own. Fed that file, the classifier is not an
// independent opinion of the target but an echo of the run's Columns map, its
// extra_patterns and any opt-out it recorded — and an opt-out would blind the
// one check meant to catch it.
func classifyEvents(ctx context.Context, t *testing.T, db *databases, connURL, what string) []ndjsonEvent {
	t.Helper()

	res := runTool(ctx, t, db.dir,
		"classify", "--source", connURL, "--json", "--yes",
		"--no-config", "--config", filepath.Join(t.TempDir(), "lazyslice.yml"),
		"--secret-file", db.secretPath(),
	)
	if res.exit != 0 {
		t.Fatalf("I2: classifying %s:\n  %s", what, res)
	}
	return parseNDJSON(t, res.stdout)
}

// flaggedColumns returns the events that report a column the classifier would
// mask, and fails on anything it cannot read.
//
// Two failures rather than a quiet answer. A stream that names no column at
// all means the contract this test reads has changed and it can no longer tell
// a clean target from a silent one. A column-scoped code that is in neither
// list means the catalogue has been renamed under it: the message names the
// code, so the fix is a one-line edit rather than an archaeology exercise, and
// the alternative — treating an unknown code as "not flagged" — is the exact
// shape of a test that disarms itself.
func flaggedColumns(t *testing.T, events []ndjsonEvent) []ndjsonEvent {
	t.Helper()

	named := 0
	var flagged []ndjsonEvent
	unknown := map[string]bool{}
	for _, e := range events {
		if e.Column == "" {
			continue
		}
		named++
		switch {
		case matchesAnyPrefix(e.Code, flaggedCodePrefixes):
			flagged = append(flagged, e)
		case matchesAnyPrefix(e.Code, copiedCodePrefixes):
		default:
			unknown[e.Code] = true
		}
	}

	if named == 0 {
		t.Fatalf("I2: `classify --json` named no column in %d event(s); this test cannot tell a clean "+
			"target from a silent one. ARCHITECTURE.md §4 says every decision has one line naming "+
			"table and column", len(events))
	}
	if len(unknown) > 0 {
		codes := make([]string, 0, len(unknown))
		for code := range unknown {
			codes = append(codes, code)
		}
		sort.Strings(codes)
		t.Fatalf("I2: `classify --json` named a column under %d code(s) this test does not recognise "+
			"as either a masking decision or a copying one: %s. It cannot tell which of them mean "+
			"personal data survived, so it would report clean by omission; add each to "+
			"flaggedCodePrefixes or copiedCodePrefixes", len(codes), strings.Join(codes, ", "))
	}
	return flagged
}

// assertNoSourceLiteralSurvives greps the target for the source's addresses
// and numbers.
func assertNoSourceLiteralSurvives(t *testing.T, targetCells []cell, source personalLiterals) {
	t.Helper()

	leaks := findLeaks(targetCells, source)
	if len(leaks) == 0 {
		return
	}

	byColumn := map[string]int{}
	example := map[string]leak{}
	for _, l := range leaks {
		key := l.Column.String()
		byColumn[key]++
		if _, seen := example[key]; !seen {
			example[key] = l
		}
	}

	names := make([]string, 0, len(byColumn))
	for k := range byColumn {
		names = append(names, k)
	}
	sort.Strings(names)

	var b strings.Builder
	b.WriteString("I2: " + strconv.Itoa(len(leaks)) +
		" value(s) from the source survive into the target:")
	for _, name := range names {
		l := example[name]
		b.WriteString("\n  " + name + ": " + strconv.Itoa(byColumn[name]) +
			" hit(s); one of them is a value of " + l.Source)
	}
	t.Error(b.String())
}

// ---------- the --json stream ----------

// ndjsonEvent is one line of `--json`, read leniently.
//
// The stream is event.Event written verbatim (§7), and this suite is
// deliberately not compiled against that struct: I2 is a statement about the
// command's output, so it reads the output rather than the type behind it. It
// accepts a stage or kind written either as its name or as its ordinal, and a
// table written either as a string or as {schema, name}, because a renderer
// that changes its spelling should fail on the assertion, not on the parse.
type ndjsonEvent struct {
	Stage  string
	Kind   string
	Code   string
	Table  string
	Column string
}

// flaggedCodePrefixes and copiedCodePrefixes are the two decisions §4 says a
// classify run reaches for every column: mask it, or copy it.
//
// ARCHITECTURE.md §4's threshold is `possible` and above, and §6 item 4 makes
// a column of the target reaching `possible` a failure with exit 9. Args
// carries no category or confidence key (§7), so the Code is where that lands,
// and these are the prefixes the classify and verify catalogues are expected
// to use.
//
// They are a claim about a file that does not exist yet: internal/event/
// catalogue.yml carries only the three ADR-008 codes today. When the classify
// stage lands, this list is what changes — and it cannot change silently,
// because flaggedColumns fails on a column-scoped code in neither list and
// assertClassifierFindsNothing requires this list to match something on a
// source full of personal data before it will believe a clean target.
//
// The two `verify.` prefixes are the exception, and they are unreachable on
// purpose. No command this suite runs emits them: `internal/verify` is a no-op
// returning ErrNotImplemented, and I2 points `classify` at the target rather
// than `verify`. So a `verify` stage that lands as `return nil` — the residual
// scan and the second net both switched off — passes all six invariants in
// this package. **Nothing here can catch that, and the check that would is a
// phase 4 task against the gate 4 verify stage**: a negative control that
// writes a known source email literal into a target column the run recorded as
// masked, runs `lazyslice verify`, and asserts exit 9 naming the table and the
// column (§6 items 1 to 3). Until that exists, treat a green I2 as a statement
// about the masker, never about `verify`.
var (
	flaggedCodePrefixes = []string{
		"classify.masked",
		"classify.flagged",
		"classify.personal",
		"verify.residual",
		"verify.second_net",
	}

	copiedCodePrefixes = []string{
		"classify.copied",
		"classify.none",
		"classify.unmasked",
		"classify.skipped",
		"classify.generated",
		"classify.surrogate",
	}
)

// matchesAnyPrefix is the code test. It is a prefix match because a code is
// "<subject>.<verdict>.<detail>" (§7) and the detail is not this suite's
// business.
func matchesAnyPrefix(code string, prefixes []string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(code, prefix) {
			return true
		}
	}
	return false
}

var (
	stageNames = []string{"discover", "introspect", "classify", "plan", "extract", "transform", "load", "verify", "emit"}
	kindNames  = []string{"stage_start", "stage_done", "progress", "decision", "question", "info", "warn", "error"}
)

// parseNDJSON reads the --json stream, failing on a line that is not JSON at
// all: --json promises NDJSON on stdout, and a half-rendered line is a bug in
// the renderer rather than something to skip past.
func parseNDJSON(t *testing.T, out string) []ndjsonEvent {
	t.Helper()

	var events []ndjsonEvent
	for i, line := range strings.Split(out, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var raw map[string]json.RawMessage
		if err := json.Unmarshal([]byte(line), &raw); err != nil {
			t.Fatalf("I2: line %d of `classify --json` is not JSON: %v\n  %s", i+1, err, truncate(line))
		}
		lower := make(map[string]json.RawMessage, len(raw))
		for k, v := range raw {
			lower[strings.ToLower(k)] = v
		}
		events = append(events, ndjsonEvent{
			Stage:  enumField(lower["stage"], stageNames),
			Kind:   enumField(lower["kind"], kindNames),
			Code:   stringField(lower["code"]),
			Table:  tableField(lower["table"]),
			Column: stringField(lower["column"]),
		})
	}
	return events
}

// enumField reads a field written either as a name or as an ordinal.
func enumField(raw json.RawMessage, names []string) string {
	if s := stringField(raw); s != "" {
		return s
	}
	var n int
	if err := json.Unmarshal(raw, &n); err != nil || n < 0 || n >= len(names) {
		return ""
	}
	return names[n]
}

func stringField(raw json.RawMessage) string {
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return ""
	}
	return s
}

// tableField reads a table written either as "schema.name" or as an object
// with schema and name fields.
func tableField(raw json.RawMessage) string {
	if s := stringField(raw); s != "" {
		return s
	}
	var obj map[string]string
	if err := json.Unmarshal(raw, &obj); err != nil {
		return ""
	}
	schema, name := obj["Schema"], obj["Name"]
	if schema == "" {
		schema = obj["schema"]
	}
	if name == "" {
		name = obj["name"]
	}
	switch {
	case schema != "" && name != "":
		return schema + "." + name
	case name != "":
		return name
	default:
		return ""
	}
}

// emittingColumn is one masked column of the emitted yml whose masker has a
// vocabulary (mask.Emitting, ADR-015), and what holding it to that vocabulary
// found.
type emittingColumn struct {
	id       mask.ID
	role     mask.Role
	failures []string
}

// emittingColumns reads the emitted yml's masker and role for every masked
// column, keeps the ones whose masker has a vocabulary, and checks each against
// both databases (assertEmittingColumn). The yml is read here rather than in
// readColumnScope because the role is ADR-015's alone and nothing else in this
// suite reads it.
func emittingColumns(
	ctx context.Context, t *testing.T, configPath string, source, target *pgx.Conn,
) map[columnRef]emittingColumn {
	t.Helper()

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("I2: reading %s: %v", configPath, err)
	}
	var cfg struct {
		Columns map[string]struct {
			Masker string         `yaml:"masker"`
			Role   string         `yaml:"role"`
			Unmask map[string]any `yaml:"unmask"`
		} `yaml:"columns"`
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("I2: parsing %s: %v", configPath, err)
	}
	out := map[columnRef]emittingColumn{}
	for name, col := range cfg.Columns {
		if col.Masker == "" || col.Unmask != nil {
			continue
		}
		ref, ok := parseColumnRef(name)
		if !ok {
			continue
		}
		id, role := mask.ID(col.Masker), mask.Role(col.Role)
		if !mask.Emitting(id, mask.Constraints{Role: role}) {
			continue
		}
		e := emittingColumn{id: id, role: role}
		e.failures = assertEmittingColumn(ctx, t, source, target, ref, e)
		out[ref] = e
	}
	return out
}

// assertEmittingColumn is ADR-015's two promises for one column, as findings:
// every distinct masked value (every element, for an array) is one mask.Emits
// accepts for the column's masker and role; and, for a scalar column of a table
// with a primary key, no target row holds a value equal over its letters and
// digits to its own source row's value (mask.FoldEqual, the comparison the
// masker's redraw guarantees against). A value is never printed, only counts.
func assertEmittingColumn(
	ctx context.Context, t *testing.T, source, target *pgx.Conn, ref columnRef, e emittingColumn,
) []string {
	t.Helper()

	ident := pgx.Identifier{ref.Column}.Sanitize()
	var isArray bool
	if err := target.QueryRow(ctx, `SELECT t.typcategory = 'A'
	      FROM pg_attribute a JOIN pg_type t ON t.oid = a.atttypid
	     WHERE a.attrelid = $1::regclass AND a.attname = $2 AND NOT a.attisdropped`,
		ref.Table.quoted(), ref.Column).Scan(&isArray); err != nil {
		t.Fatalf("I2: reading %s's type in the target: %v", ref, err)
	}

	var failures []string
	q := fmt.Sprintf(`SELECT DISTINCT %s::text FROM %s WHERE %s IS NOT NULL`, ident, ref.Table.quoted(), ident)
	if isArray {
		q = fmt.Sprintf(`SELECT DISTINCT e::text FROM %s, unnest(%s) AS e WHERE e IS NOT NULL`,
			ref.Table.quoted(), ident)
	}
	off := 0
	for _, v := range stringsOf(ctx, t, target, q) {
		if v == "" {
			continue
		}
		if !mask.Emits(e.id, mask.Value{Text: v}, mask.Constraints{Role: e.role}) {
			off++
		}
	}
	if off > 0 {
		failures = append(failures, ref.String()+": "+strconv.Itoa(off)+" distinct masked value(s) are not "+
			"in the vocabulary of masker "+string(e.id)+", so something other than that masker wrote them")
	}
	if isArray {
		return failures
	}

	pk := stringsOf(ctx, t, target, fmt.Sprintf(`SELECT a.attname::text
	      FROM pg_index i JOIN pg_attribute a ON a.attrelid = i.indrelid AND a.attnum = ANY (i.indkey)
	     WHERE i.indrelid = %s::regclass AND i.indisprimary
	     ORDER BY array_position(i.indkey::int2[], a.attnum)`, quoteLiteral(ref.Table.quoted())))
	if len(pk) == 0 {
		t.Logf("I2: %s has no primary key, so only its vocabulary is checked here; the residual scan's "+
			"count check is what stands for its rows", ref)
		return failures
	}
	keyExpr := make([]string, len(pk))
	for i, c := range pk {
		keyExpr[i] = pgx.Identifier{c}.Sanitize() + "::text"
	}
	rowsQ := fmt.Sprintf(`SELECT concat_ws(chr(31), %s), %s::text FROM %s WHERE %s IS NOT NULL`,
		strings.Join(keyExpr, ", "), ident, ref.Table.quoted(), ident)
	src := pairsOf(ctx, t, source, rowsQ)
	kept := 0
	for key, v := range pairsOf(ctx, t, target, rowsQ) {
		if s, ok := src[key]; ok && v != "" && mask.FoldEqual(s, v) {
			kept++
		}
	}
	if kept > 0 {
		failures = append(failures, ref.String()+": "+strconv.Itoa(kept)+" row(s) hold their own source value "+
			"(equal over letters and digits), which the masker's redraw makes impossible for a masked value")
	}
	return failures
}

// overlapCount is how many distinct target values are also source values.
func overlapCount(source, target map[string]bool) int {
	n := 0
	for v := range target {
		if source[v] {
			n++
		}
	}
	return n
}

func stringsOf(ctx context.Context, t *testing.T, conn *pgx.Conn, q string) []string {
	t.Helper()
	rows, err := conn.Query(ctx, q)
	if err != nil {
		t.Fatalf("I2: %v", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			t.Fatalf("I2: %v", err)
		}
		out = append(out, v)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("I2: %v", err)
	}
	return out
}

func pairsOf(ctx context.Context, t *testing.T, conn *pgx.Conn, q string) map[string]string {
	t.Helper()
	rows, err := conn.Query(ctx, q)
	if err != nil {
		t.Fatalf("I2: %v", err)
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			t.Fatalf("I2: %v", err)
		}
		out[k] = v
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("I2: %v", err)
	}
	return out
}

func quoteLiteral(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }
