// SPDX-License-Identifier: Apache-2.0

package classify

import (
	"strings"
	"testing"

	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
)

// TestVersionNamedColumnIsNotMaskedAsNetworkID pins the first half of
// tracker T-0317: dogfood session 1's last_player_version held dot-separated
// integers of the same shape an IPv4 address is ("1.2.3.4"), and 93 of 168
// samples parsed as one -- below validatorThreshold, so bestSignal's
// strongHit branch masked the whole column as free_text, "a strong validator
// hit below the category threshold". A column named for its own version,
// build or release is never a network address, whatever its digits parse as.
func TestVersionNamedColumnIsNotMaskedAsNetworkID(t *testing.T) {
	t.Parallel()

	tbl := ref.TableRef{Schema: "public", Name: "t317_versions"}
	schema := &pipeline.Schema{
		Tables: []pipeline.Table{
			tt("public", "t317_versions", nil,
				tc("last_player_version", "text"), // networkIDVetoed
				tc("app_build_number", "text"),    // networkIDVetoed, word in the middle
				tc("hotline_ref", "text"),         // control: the same values, no rule-pack name matches this
			),
		},
	}
	// The same shape the dogfood report named: a minority of values parse as
	// an IPv4 address (six of ten, well under validatorThreshold's 0.8), the
	// rest are ordinary version-looking strings that parse as nothing.
	ipShaped := anyOf("1.2.3.4", "2.5.1.0", "10.14.2.1", "0.9.12.3", "3.3.3.3", "7.1.0.2",
		"beta-rc1", "nightly", "dev-build", "unreleased")

	samples := mapSampler{
		ref.ColumnRef{Table: tbl, Column: "last_player_version"}: ipShaped,
		ref.ColumnRef{Table: tbl, Column: "app_build_number"}:    ipShaped,
		ref.ColumnRef{Table: tbl, Column: "hotline_ref"}:         ipShaped,
	}
	cls, err := New().Classify(schema, samples, nil)
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}

	for _, name := range []string{"last_player_version", "app_build_number"} {
		if d := decision(t, cls, col(tbl, name)); d.Masked {
			t.Errorf("t317_versions.%s = %s/%v masked (%s): a version/build column's dotted digits "+
				"are not a network address (T-0317)", name, d.Category, d.Confidence, d.Reason)
		}
	}

	// The control proves the test actually exercises the strongHit path: with
	// no veto word in its name, the identical values still mask the column,
	// so the two columns above are unmasked because of the veto and not
	// because the values themselves stopped looking like IP addresses.
	control := decision(t, cls, col(tbl, "hotline_ref"))
	if !control.Masked || control.Category != pipeline.CatFreeText {
		t.Fatalf("t317_versions.hotline_ref = %+v, want masked free_text: precondition for the test above", control)
	}
	if bad, ok := ParseReason(control.Reason); !ok {
		t.Errorf("t317_versions.hotline_ref reason %q holds a fragment no template produced: %q", control.Reason, bad)
	}
	if !strings.Contains(control.Reason, phraseIP) {
		t.Errorf("t317_versions.hotline_ref reason = %q, want the IP-parsing phrase named", control.Reason)
	}
}

// TestKeyNamedColumnIsNotMaskedAsGuessedPhone pins the second half of tracker
// T-0317: dogfood session 1's license_key held ten-digit samples, 8 of 10 of
// which cleared a guessed region's numbering plan (guessedPhoneHit, T-0221),
// and guessedPhoneColumns masked it phone beside a personal neighbour -- the
// wrong shape for a key. A column named for a key, code, license, serial or
// token is never offered to the guessed-region fallback at all.
func TestKeyNamedColumnIsNotMaskedAsGuessedPhone(t *testing.T) {
	t.Parallel()

	// The same values TestGuessedRegionPhoneCorroboration already verified,
	// by direct computation against textsig.ValidPhoneRegion, clear one of
	// phoneGuessRegions on every sample.
	guessed := []any{"9231278675", "7543856411", "5101878760", "5526624009", "9743547657"}

	tbl := ref.TableRef{Schema: "public", Name: "t317_licenses"}
	schema := &pipeline.Schema{
		Tables: []pipeline.Table{
			tt("public", "t317_licenses", nil,
				tc("license_key", "text"), // phoneGuessVetoed
				tc("serial_number", "text"),
				tc("hotline_ref", "text"), // control: no veto word, same corroboration
				tc("email", "text"),       // certain: the neighbour signal
			),
		},
	}
	samples := mapSampler{
		ref.ColumnRef{Table: tbl, Column: "license_key"}:   guessed,
		ref.ColumnRef{Table: tbl, Column: "serial_number"}: guessed,
		ref.ColumnRef{Table: tbl, Column: "hotline_ref"}:   guessed,
		ref.ColumnRef{Table: tbl, Column: "email"}: anyOf(
			"a@fixture.test", "b@fixture.test", "c@fixture.test", "d@fixture.test"),
	}
	cls, err := New().Classify(schema, samples, nil)
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}

	// license_key and serial_number are still masked -- the neighbouring
	// email column is certain, and a signal-less character column beside it
	// is the ordinary T-0311 sweep's free_text catch-all -- but never as
	// phone: that is the one category this task's veto must keep off a
	// column named for a key, code, license, serial or token.
	for _, name := range []string{"license_key", "serial_number"} {
		d := decision(t, cls, col(tbl, name))
		if d.Category == pipeline.CatPhone {
			t.Errorf("t317_licenses.%s = %s/%v masked (%s): a key/serial column is not a phone number, "+
				"whatever a guessed region makes of its digits (T-0317)", name, d.Category, d.Confidence, d.Reason)
		}
	}

	// The control proves the test actually exercises guessedPhoneColumns'
	// corroboration path: with no veto word in its name and the same
	// personal neighbour, the identical values still mask the column.
	control := decision(t, cls, col(tbl, "hotline_ref"))
	if !control.Masked || control.Category != pipeline.CatPhone {
		t.Fatalf("t317_licenses.hotline_ref = %+v, want masked as phone beside its email neighbour: "+
			"precondition for the test above", control)
	}
}

// TestVersionNamedColumnWithGenuineIPsStaysMaskedNetworkID pins the T-0317
// review round's finding 2: the first landing of networkIDVetoed's
// withoutNetworkID removed the network_id validators from a version-named
// column's list outright, unconditionally on the values -- so a column that
// genuinely holds IP addresses, and only coincidentally carries a veto word
// in its name (build_host, release_server, client_version_origin), reached
// the target unmasked. The veto is only for the minority strongHit path
// (bestSignal's below-validatorThreshold branch); a column whose values
// clear validatorThreshold on IP or MAC -- a real majority, not a
// coincidence -- must still mask network_id.
func TestVersionNamedColumnWithGenuineIPsStaysMaskedNetworkID(t *testing.T) {
	t.Parallel()

	tbl := ref.TableRef{Schema: "public", Name: "t317_hosts"}
	schema := &pipeline.Schema{
		Tables: []pipeline.Table{
			tt("public", "t317_hosts", nil,
				tc("build_host", "text"), // networkIDVetoed, but 100% real IPs
			),
		},
	}
	genuineIPs := anyOf("10.0.0.1", "10.0.0.2", "10.0.0.3", "10.0.0.4", "10.0.0.5")

	samples := mapSampler{
		ref.ColumnRef{Table: tbl, Column: "build_host"}: genuineIPs,
	}
	cls, err := New().Classify(schema, samples, nil)
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}

	d := decision(t, cls, col(tbl, "build_host"))
	if !d.Masked || d.Category != pipeline.CatNetworkID {
		t.Errorf("t317_hosts.build_host = %s/%v masked=%v (%s), want masked network_id: "+
			"a column whose values genuinely are addresses is not spared by a coincidental "+
			"name match (T-0317 review round, finding 2)", d.Category, d.Confidence, d.Masked, d.Reason)
	}
}
