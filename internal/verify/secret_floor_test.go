// SPDX-License-Identifier: Apache-2.0

package verify

import (
	"context"
	"testing"

	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
)

// The second net's half of T-0315. internal/classify no longer decides a
// column credential on the entropy check over fewer than five samples, nor
// asks it about a `type`, `klass` or `component_name` column, and leaves each
// such column unmasked; this net refusing the loaded target over the same
// column, at exit 9, would be the refusal with no green path short of
// --unmask that the classifier's change exists to remove. A file name, a
// fixed-length hex digest, a namespaced identifier and an environment
// variable's name are textsig.LooksSecret's own exclusions and reach this net
// through the same function. The last two cases are the half that must not
// move: five secret-shaped values in an ordinary column still refuse.
func TestTheSecondNetAgreesWithTheClassifierAboutEntropy(t *testing.T) {
	table := customers()
	tokens := []any{
		"q4r7uXzN8vK2mL9pQ3sT6wY1", "Zb3Xk9Lm2Qp7Rt5Vw8Yc1Nd4", "H7jK2mP9qR4sT8vW3xY6zB1c",
		"aB3dE6gH9jK2mN5pQ8rT1vW4", "M2nP5qR8sT1vW4xY7zB0cD3f",
	}
	classes := []any{
		"ScheduledReportExport", "WebhookDeliveryAttempt", "SlackNotificationRule",
		"DeviceHeartbeatEvent", "PlaylistScheduleOverride", "ScheduledReportExport",
	}

	cases := []struct {
		name     string
		column   string
		vals     []any
		wantFail string
	}{
		{"one secret-shaped value", "note", tokens[:1], ""},
		{"four secret-shaped values", "note", tokens[:4], ""},
		{"an STI type column", "type", classes, ""},
		{"a klass column", "klass", classes, ""},
		{"a component_name column", "component_name", classes, ""},
		{"a componentName column", "componentName", classes, ""},
		{"a Type column", "Type", classes, ""},
		{"file names", "note", []any{
			"Screenshot_2024-03-01_Ab3Xk9.png", "Screenshot_2024-03-02_Zq7Rt5.png",
			"webcam_20240301_101512_cam2.JPG", "player-4.2.1-Hk2mP-x64.exe", "export-2024-03-01T101512Z.csv",
		}, ""},
		{"md5 digests", "note", []any{
			"d41d8cd98f00b204e9800998ecf8427e", "9e107d9d372bb6826bd81d3542a419d6",
			"e4d909c290d0fb1ca068ffaddf22cbd0", "0cc175b9c0f1b6a831c399e269772661", "92eb5ffee6ae2fec3ad71c777531578f",
		}, ""},
		{"namespaced class names", "note", []any{
			"Billing::Invoices::PdfExport", "Admin::ScheduledReport", "Api::V2::UsersController",
			"Logger::JsonFormatter", "com.example.reports.CsvFormatter",
		}, ""},
		{"five secret-shaped values", "note", tokens, "credential"},
		{"file names half of which carry a dictionary name", "note", []any{
			"photo-grace_hopper1-1.jpg", "photo-dmytro_egorov2-2.jpg", "photo-grace_hopper3-3.jpg",
			"photo-dmytro_egorov4-4.jpg", "photo-grace_hopper5-5.jpg", "photo-dmytro_egorov6-6.jpg",
		}, "credential"},
		{"the same values in a column that is not called type", "kind", classes, "credential"},
		{"dotted handles built from people's names", "external_ref", []any{
			"katherine.johnson84", "margaret.okafor61", "christopher.fitzgerald72",
			"alexandra.henderson90", "katherine.blackwood63", "margaret.johnson77",
		}, "credential"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			col := ref.ColumnRef{Table: table, Column: c.column}
			s := &state{
				schema: &pipeline.Schema{},
				target: oneColumn{vals: c.vals},
				steps:  []pipeline.Step{{Table: table, Mode: pipeline.ChildOK}},
				tables: map[ref.TableRef]*pipeline.Table{
					table: {Ref: table, Columns: []pipeline.Column{{Name: col.Column, TypeName: "text"}}},
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
					t.Fatalf("the net failed %s on %v as %q; internal/classify leaves this column unmasked on purpose (T-0315)",
						col, c.vals, s.failures[0].Reason)
				}
				return
			}
			if len(s.failures) != 1 || s.failures[0].Reason != c.wantFail {
				t.Fatalf("the net recorded %v on %v, want one refusal naming %s; a column of secrets nobody masked is in the target",
					s.failures, c.vals, c.wantFail)
			}
		})
	}
}
