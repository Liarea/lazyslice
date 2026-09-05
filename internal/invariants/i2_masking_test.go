//go:build integration

package invariants

import (
	"context"
	"encoding/json"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// TestI2NothingFlaggedSurvives is invariant I2: no flagged column in the
// target holds a value the classifier's own detectors would flag.
//
// Two halves, because either alone can be fooled.
//
//  1. The tool's own opinion: `lazyslice classify --json` is pointed at the
//     target and must find nothing to mask in any column the run did not mask.
//     This is §6 item 4's second net run from outside the process, and it
//     catches a masker that let a category through on a column the 200-row
//     sample under-represented.
//  2. A grep the tool has no say in: every email address and every phone
//     number that exists in the source is looked for in every cell of the
//     target. A classifier that never flagged a column at all passes the first
//     half and fails this one — which is the failure this suite exists to make
//     impossible to miss.
//
// Both halves are about values in the target, so both are vacuous on a target
// with no rows in it: findLeaks over zero cells returns nil and the classifier
// has nothing to classify. assertTargetHoldsMaskedRows is the guard, and it
// asks for more than "not empty" — it asks that a column the run recorded as
// masked actually arrived with a value in it.
func TestI2NothingFlaggedSurvives(t *testing.T) {
	ctx := context.Background()

	for _, f := range fixtures {
		t.Run(f.name, func(t *testing.T) {
			db := start(ctx, t, f)

			sourceLiterals := collectPersonalLiterals(scanCells(ctx, t, connect(ctx, t, db.source)))
			if len(sourceLiterals) == 0 {
				t.Fatalf("I2: no email address or phone number was found in the %s source, so the grep "+
					"below would pass whatever the tool did; the detectors in scan_test.go and the fixture "+
					"have drifted apart", f.name)
			}

			db.snapshot(ctx, t, f, f.root, f.take)

			masked := maskedColumns(t, db.configPath())
			targetCells := scanCells(ctx, t, connect(ctx, t, db.target))
			assertTargetHoldsMaskedRows(t, f.name, targetCells, masked)

			t.Run("classifier", func(t *testing.T) { assertClassifierFindsNothing(ctx, t, db, masked) })
			t.Run("grep", func(t *testing.T) {
				assertNoSourceLiteralSurvives(t, targetCells, sourceLiterals)
			})
		})
	}
}

// assertTargetHoldsMaskedRows fails when the target holds nothing for the two
// halves below to look at.
//
// "Nothing" has two shapes and both pass I2 silently: a target with no rows at
// all, and a target that loaded rows but not into any column the run said it
// masked. A pipeline that copied the root's --take rows and stopped satisfies
// every other invariant in this package too, so this is the one place that
// says so.
func assertTargetHoldsMaskedRows(t *testing.T, fixture string, cells []cell, masked map[string]bool) {
	t.Helper()

	if len(cells) == 0 {
		t.Fatalf("I2: the %s target holds no non-null value in any table, so both halves of this "+
			"invariant pass whatever the masker did", fixture)
	}
	for _, c := range cells {
		if masked[c.Table.String()+"."+c.Column] {
			return
		}
	}
	t.Fatalf("I2: the %s target holds %d non-null value(s) and not one of them is in any of the %d "+
		"column(s) the emitted yml records as masked, so nothing the masker touched was loaded and "+
		"both halves below are about columns nobody claimed to mask", fixture, len(cells), len(masked))
}

// assertClassifierFindsNothing runs `classify --json` against the target.
//
// The assertion is scoped to §6 item 4: a column the run masked with a
// category masker is *expected* to classify as its category — a masked email
// is still an email — so only the columns the run left alone are held to
// "nothing flagged". masked comes from the emitted yml, which is the run's own
// statement of which those are.
func assertClassifierFindsNothing(ctx context.Context, t *testing.T, db *databases, masked map[string]bool) {
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
		column := e.Table + "." + e.Column
		if masked[column] {
			continue // §6 item 4: a column a category masker owns is out of scope
		}
		hits = append(hits, column+" ("+e.Code+")")
	}
	if len(hits) > 0 {
		sort.Strings(hits)
		t.Errorf("I2: the classifier flags %d column(s) of the target that the run did not mask:\n  %s",
			len(hits), strings.Join(hits, "\n  "))
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
		key := l.Table.String() + "." + l.Column
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
