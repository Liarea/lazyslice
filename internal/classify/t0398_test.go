// SPDX-License-Identifier: Apache-2.0

package classify

import (
	"strings"
	"testing"

	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
)

// T-0398 (the 2026-09-25 JSON red team, round 1, entry 26). The rule pack's
// log_shaped regex, over the normalised (case-split, lower-cased) table
// name, is the single log-shaped rule (st.pack.logShapedTable). Before this
// task internal/transform kept its own copy -- logTableWords, an
// underscore-only split with no CamelCase normalisation and a shorter word
// list with no activity or trace -- that agreed with it only by accident.
// Decision.LogShaped is what closes the gap: it is set from the identical
// call the reason line's "jsonb in a log-shaped table" fragment already
// uses, so the two can never disagree about which tables matched.
func TestLogShapedMatchesTheRulePackWhereverTheReasonLineDoes(t *testing.T) {
	for _, c := range []struct {
		table string
		want  bool
	}{
		{"AuditLog", true}, // Prisma's default: CamelCase, no underscore at all
		{"audit_log", true},
		{"user_activity", true}, // not in internal/transform's old logTableWords at all
		{"activities", true},
		{"request_trace", true}, // "trace" is not in internal/transform's old list either
		{"traces", true},
		{"event_log_entries", true},
		{"catalogue", false}, // "log" as a substring, not a word
		{"blogposts", false},
		{"people", false},
		// T-0398 review round, high finding: normaliseName's acronym-boundary
		// rule (upper, upper, then lower) splits a trailing lower-case plural
		// off an all-caps word -- "EVENTs" normalises to "even_ts" and "LOGs"
		// to "lo_gs" -- so the regex's whole-word match missed both, the same
		// leak the deleted internal/transform copy (logTableWords, a plain
		// lower-case-and-split-on-'_' word match) never had.
		{"EVENTs", true},
		{"LOGs", true},
		{"user_LOGs", true},
		{"AUDIT_LOG", true},
		{"Events", true},
		{"Order_HISTORIES", true},
	} {
		t.Run(c.table, func(t *testing.T) {
			table := ref.TableRef{Schema: "public", Name: c.table}
			schema := &pipeline.Schema{Tables: []pipeline.Table{tt("public", c.table, []string{"id"},
				tc("id", "bigint"), tc("payload", "jsonb"))}}
			sampler := mapSampler{}
			for g := 1; g <= 5; g++ {
				sampler[col(table, "id")] = append(sampler[col(table, "id")], int64(g))
				sampler[col(table, "payload")] = append(sampler[col(table, "payload")], `{"a": "no signal here"}`)
			}
			cls, err := New().Classify(schema, sampler, nil)
			if err != nil {
				t.Fatalf("Classify: %v", err)
			}
			d := decision(t, cls, col(table, "payload"))
			if d.LogShaped != c.want {
				t.Errorf("%s.payload.LogShaped = %v, want %v", c.table, d.LogShaped, c.want)
			}
			saysReplacedWhole := strings.Contains(d.Reason, "log-shaped table")
			if saysReplacedWhole != c.want {
				t.Errorf("%s.payload reason = %q, want the log-shaped fragment present: %v", c.table, d.Reason, c.want)
			}
			if d.LogShaped != saysReplacedWhole {
				t.Errorf("%s.payload: LogShaped=%v but the reason line's own claim is %v -- the field and the reason must never disagree",
					c.table, d.LogShaped, saysReplacedWhole)
			}
		})
	}
}

// LogShaped is a table-level answer, set on every column of a log-shaped
// table and not only its jsonb ones (internal/classify/classify.go's
// finalise): internal/transform's maskDocument only ever reads it for a
// document column, but the field itself does not know that, the same way
// NeverMasked and the corroboration fields are computed once per table and
// carried onto every column in it.
func TestLogShapedIsSetOnEveryColumnOfTheTable(t *testing.T) {
	table := ref.TableRef{Schema: "public", Name: "AuditLog"}
	schema := &pipeline.Schema{Tables: []pipeline.Table{tt("public", "AuditLog", []string{"id"},
		tc("id", "bigint"), tc("actor_email", "text"), tc("changes", "jsonb"))}}
	sampler := mapSampler{}
	for g := 1; g <= 5; g++ {
		sampler[col(table, "id")] = append(sampler[col(table, "id")], int64(g))
		sampler[col(table, "actor_email")] = append(sampler[col(table, "actor_email")], "wren.calloway@example.test")
		sampler[col(table, "changes")] = append(sampler[col(table, "changes")], `{"a": "no signal here"}`)
	}
	cls, err := New().Classify(schema, sampler, nil)
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	for _, name := range []string{"id", "actor_email", "changes"} {
		if d := decision(t, cls, col(table, name)); !d.LogShaped {
			t.Errorf(`"AuditLog".%s.LogShaped = false, want true: LogShaped is a table-level fact`, name)
		}
	}
}
