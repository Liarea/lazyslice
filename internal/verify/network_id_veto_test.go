// SPDX-License-Identifier: Apache-2.0

package verify

import (
	"context"
	"testing"

	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
)

// TestTheSecondNetDoesNotReadAVersionColumnAsIPAddresses is T-0317's verify
// half (T-0360): the classifier leaves a version/build/release-named column
// unmasked when a minority of its dotted-integer values parse as IPv4, so the
// second net must not refuse the run over the same values; a column named for
// what it holds (an IP address) is still refused.
func TestTheSecondNetDoesNotReadAVersionColumnAsIPAddresses(t *testing.T) {
	table := ref.TableRef{Schema: "public", Name: "players"}
	vals := []any{"1.2.3", "1.2.3.4", "2.0.0", "10.0.1.7", "1.2.4", "3.1.0"}
	for _, c := range []struct {
		column   string
		wantFail string
	}{
		{column: "last_player_version"},
		{column: "app_build_number"},
		{column: "release_tag"},
		{column: "last_ip_address", wantFail: "network_id"},
	} {
		t.Run(c.column, func(t *testing.T) {
			col := ref.ColumnRef{Table: table, Column: c.column}
			s := &state{
				schema: &pipeline.Schema{},
				target: oneColumn{vals: vals},
				steps:  []pipeline.Step{{Table: table, Mode: pipeline.ChildOK}},
				tables: map[ref.TableRef]*pipeline.Table{
					table: {Ref: table, Columns: []pipeline.Column{{Name: c.column, TypeName: "text"}}},
				},
				cls: &pipeline.Classification{Decisions: map[ref.ColumnRef]pipeline.Decision{
					col: {Col: col, Category: pipeline.CatNone, Source: pipeline.ByClassifier},
				}},
			}
			if err := s.secondNet(context.Background()); err != nil {
				t.Fatalf("secondNet: %v", err)
			}
			if c.wantFail == "" {
				if len(s.failures) != 0 {
					t.Fatalf("the net refused %s on %v as %q; a version column's dotted integers are not addresses", col, vals, s.failures[0].Reason)
				}
				return
			}
			if len(s.failures) != 1 || s.failures[0].Reason != c.wantFail {
				t.Fatalf("the net recorded %d failures on %s, want one naming %s: the veto is by name, and this name says address", len(s.failures), col, c.wantFail)
			}
		})
	}
}
