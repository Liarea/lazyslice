// SPDX-License-Identifier: Apache-2.0

package core

import (
	"testing"

	"github.com/Liarea/lazyslice/internal/pipeline"
)

// typeAllowSchema is a source with one enum and one domain, used by every case
// below so that typeFingerprint (names.go) has something real to hash.
func typeAllowSchema() *pipeline.Schema {
	return &pipeline.Schema{
		Enums: map[string][]string{
			"public.status": {"active", "inactive"},
		},
		Domains: []pipeline.NamedDef{
			{Name: "public.ssn_domain", Def: "CHECK (VALUE ~ '^[0-9]{3}-[0-9]{2}-[0-9]{4}$')"},
		},
	}
}

// T-0186's review: the four states planRequest's merge of the committed yml's
// types: block can leave an entry in — matching fingerprint honoured,
// mismatched fingerprint expired, empty TypeFP expired, and a name no longer
// in Schema.Enums/Domains expired — verified by hand at review time and pinned
// here so a regression in the expiry path fails a test instead of only a
// review round.
func TestPlanRequestTypeAllowExpiry(t *testing.T) {
	schema := typeAllowSchema()
	statusFP, ok := typeFingerprint("public.status", schema)
	if !ok {
		t.Fatal("typeFingerprint did not find public.status")
	}

	cases := []struct {
		name    string
		prior   pipeline.TypeAllow
		honour  bool // whether it should survive into req.AllowTypeLiterals / r.typeAllow
		expired bool // whether it should be recorded in r.typeExpired
	}{
		{
			name:    "matching fingerprint is honoured",
			prior:   pipeline.TypeAllow{Reason: "labels are not personal data", By: "sam", TypeFP: statusFP},
			honour:  true,
			expired: false,
		},
		{
			name:    "mismatched fingerprint expires",
			prior:   pipeline.TypeAllow{Reason: "labels are not personal data", By: "sam", TypeFP: "deadbeef"},
			honour:  false,
			expired: true,
		},
		{
			name:    "empty TypeFP expires",
			prior:   pipeline.TypeAllow{Reason: "labels are not personal data", By: "sam", TypeFP: ""},
			honour:  false,
			expired: true,
		},
		{
			name:    "a name absent from Enums/Domains expires",
			prior:   pipeline.TypeAllow{Reason: "gone now", By: "sam", TypeFP: "anything"},
			honour:  false,
			expired: true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			typeName := "public.status"
			if c.name == "a name absent from Enums/Domains expires" {
				typeName = "public.retired_type"
			}
			prior := &pipeline.Config{Types: map[string]pipeline.TypeAllow{typeName: c.prior}}
			r := &run{
				req:    normalise(Request{}),
				sink:   &collector{},
				schema: schema,
				prior:  prior,
			}
			req, err := r.planRequest()
			if err != nil {
				t.Fatalf("planRequest: %v", err)
			}

			_, honoured := req.AllowTypeLiterals[typeName]
			if honoured != c.honour {
				t.Errorf("req.AllowTypeLiterals[%q] present = %v, want %v", typeName, honoured, c.honour)
			}
			_, inMap := r.typeAllow[typeName]
			if inMap != c.honour {
				t.Errorf("r.typeAllow[%q] present = %v, want %v", typeName, inMap, c.honour)
			}

			gotExpired := false
			for _, name := range r.typeExpired {
				if name == typeName {
					gotExpired = true
				}
			}
			if gotExpired != c.expired {
				t.Errorf("r.typeExpired contains %q = %v, want %v (typeExpired = %v)",
					typeName, gotExpired, c.expired, r.typeExpired)
			}
		})
	}
}

// A flag naming the same type as the yml overwrites it: `by: flag` and a
// fresh fingerprint, which is how the flag wins on the same type (root
// CLAUDE.md "a default the flag overrides, never a way to widen", applied to
// this opt-out the same way it applies to --unmask).
func TestPlanRequestFlagOverwritesYmlTypeAllow(t *testing.T) {
	schema := typeAllowSchema()
	statusFP, _ := typeFingerprint("public.status", schema)

	prior := &pipeline.Config{Types: map[string]pipeline.TypeAllow{
		"public.status": {Reason: "the committed reason", By: "sam", TypeFP: statusFP},
	}}
	req := normalise(Request{AllowTypeLiterals: map[string]string{"public.status": "the flag's own reason"}})
	r := &run{req: req, sink: &collector{}, schema: schema, prior: prior}

	planReq, err := r.planRequest()
	if err != nil {
		t.Fatalf("planRequest: %v", err)
	}

	if got := planReq.AllowTypeLiterals["public.status"]; got != "the flag's own reason" {
		t.Errorf("req.AllowTypeLiterals[public.status] = %q, want the flag's reason", got)
	}
	got, ok := r.typeAllow["public.status"]
	if !ok {
		t.Fatal("r.typeAllow[public.status] is missing")
	}
	if got.Reason != "the flag's own reason" || got.By != "flag" || got.TypeFP != statusFP {
		t.Errorf("r.typeAllow[public.status] = %+v, want reason %q, by flag, fingerprint %q",
			got, "the flag's own reason", statusFP)
	}
}

// planStage sends one plan.type_literal.allowed per honoured opt-out and one
// plan.type_literal.opt_out_expired per dropped yml entry, so an operator
// reading the transcript is told which types were exempted and which opt-outs
// lapsed, rather than only meeting exit 13 again on the ones that lapsed.
func TestPlanRequestPopulatesEventsForPlanStage(t *testing.T) {
	schema := typeAllowSchema()
	statusFP, _ := typeFingerprint("public.status", schema)

	prior := &pipeline.Config{Types: map[string]pipeline.TypeAllow{
		"public.status":     {Reason: "labels are not personal data", By: "sam", TypeFP: statusFP},
		"public.ssn_domain": {Reason: "stale", By: "sam", TypeFP: "deadbeef"},
	}}
	r := &run{req: normalise(Request{}), sink: &collector{}, schema: schema, prior: prior}
	if _, err := r.planRequest(); err != nil {
		t.Fatalf("planRequest: %v", err)
	}

	if _, ok := r.typeAllow["public.status"]; !ok {
		t.Error("public.status should be honoured and available for CodeTypeLiteralAllowed")
	}
	found := false
	for _, name := range r.typeExpired {
		if name == "public.ssn_domain" {
			found = true
		}
	}
	if !found {
		t.Errorf("public.ssn_domain should be in r.typeExpired for CodeTypeLiteralOptOutExpired, got %v", r.typeExpired)
	}
}
