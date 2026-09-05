//go:build integration

package invariants

import (
	"context"
	"encoding/json"
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
//     target and must find nothing to mask. This is §6 item 4's second net run
//     from outside the process, and it catches a masker that let a category
//     through on a column the 200-row sample under-represented.
//  2. A grep the tool has no say in: every email address and every phone
//     number that exists in the source is looked for in every cell of the
//     target. A classifier that never flagged a column at all passes the first
//     half and fails this one — which is the failure this suite exists to make
//     impossible to miss.
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

			t.Run("classifier", func(t *testing.T) { assertClassifierFindsNothing(ctx, t, db) })
			t.Run("grep", func(t *testing.T) {
				assertNoSourceLiteralSurvives(ctx, t, db, sourceLiterals)
			})
		})
	}
}

// assertClassifierFindsNothing runs `classify --json` against the target.
func assertClassifierFindsNothing(ctx context.Context, t *testing.T, db *databases) {
	t.Helper()

	res := runTool(ctx, t, db.dir,
		"classify", "--source", db.target, "--json", "--no-config", "--yes",
		"--secret-file", db.secretPath(),
	)
	if res.exit != 0 {
		t.Fatalf("I2: classifying the target:\n  %s", res)
	}

	events := parseNDJSON(t, res.stdout)
	named := 0
	var hits []string
	for _, e := range events {
		if e.Column == "" {
			continue
		}
		named++
		if !e.flagged() {
			continue
		}
		hits = append(hits, e.Table+"."+e.Column+" ("+e.Code+")")
	}

	// A predicate that matches nothing would make this half of I2 pass on any
	// output at all, including none. If the classify stage never names a
	// column, the contract this test reads has changed and the test is the
	// thing to fix.
	if named == 0 {
		t.Fatalf("I2: `classify --json` on the target named no column in %d event(s); "+
			"this test cannot tell a clean target from a silent one. "+
			"ARCHITECTURE.md §4 says every decision has one line naming table and column",
			len(events))
	}
	if len(hits) > 0 {
		sort.Strings(hits)
		t.Errorf("I2: the classifier flags %d column(s) of the target it just loaded:\n  %s",
			len(hits), strings.Join(hits, "\n  "))
	}
}

// assertNoSourceLiteralSurvives greps the target for the source's addresses
// and numbers.
func assertNoSourceLiteralSurvives(ctx context.Context, t *testing.T, db *databases, source personalLiterals) {
	t.Helper()

	leaks := findLeaks(scanCells(ctx, t, connect(ctx, t, db.target)), source)
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

// flaggedCodePrefixes are the codes this test reads as "the classifier would
// mask this column".
//
// ARCHITECTURE.md §4's threshold is `possible` and above, and §6 item 4 makes
// a column of the target reaching `possible` a failure with exit 9. Args
// carries no category or confidence key (§7), so the Code is where that lands.
// These are the prefixes the classify and verify catalogues are expected to
// use; when the catalogue lands with other spellings, this list is what
// changes, and the vacuity check above is what stops that being silent.
var flaggedCodePrefixes = []string{
	"classify.masked",
	"classify.flagged",
	"classify.personal",
	"verify.residual",
	"verify.second_net",
}

// flagged says whether this event reports a column the classifier would mask.
func (e ndjsonEvent) flagged() bool {
	if e.Kind == "warn" || e.Kind == "error" {
		return true
	}
	for _, prefix := range flaggedCodePrefixes {
		if strings.HasPrefix(e.Code, prefix) {
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
