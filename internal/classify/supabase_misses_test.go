// SPDX-License-Identifier: Apache-2.0

package classify

import (
	"testing"

	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
)

// The ten columns docs/TORTURE.md's supabase-auth truth set records as missed:
// hand-labelled personal, classified below §4's mask threshold, and therefore
// copied into the target verbatim under exit 0. Recall on that schema is 0.800
// and this is the 0.200.
//
// **This test asserts a leak.** It exists because the alternative is worse: the
// ten are recorded in a prose table in docs/TORTURE.md and in tracker task
// T-0104, and nothing that runs stops an eleventh joining them, or the rules
// being narrowed until a column that is caught today is not. Every entry below
// is `wantMasked: false` today, and the day one of them flips this test fails
// and says which — which is the point. **When T-0104 lands, do not delete this
// file**: flip the entry to `wantMasked: true` and re-measure the recall number
// in docs/TORTURE.md against the same truth set, which is what T-0104 asks for.
//
// It is a name-rule test, not a schema replay. Each column is put in a table of
// its own with no samples, because seven of the ten are empty in the fixture and
// the name is the only signal any of them could have had — which is precisely
// the case T-0104 says the rule pack is thin for. Isolating them also keeps the
// neighbouring-column rule out of the answer, so a change to a *name pattern*
// is what moves this file.
func TestSupabaseAuthMissesArePinned(t *testing.T) {
	t.Parallel()

	cases := []struct {
		table, column, typeName string
		holds                   string
		wantMasked              bool
	}{
		{"flow_state", "auth_code", "text", "the OAuth authorization code", false},
		{"identities", "provider_id", "text", "the provider's subject id for the person", false},
		{"refresh_tokens", "parent", "character varying(255)", "another refresh token", false},
		{"mfa_challenges", "otp_code", "text", "the one-time code", false},
		{"mfa_recovery_codes", "code_hash", "text", "a recovery code", false},
		{"oauth_authorizations", "authorization_code", "text", "the authorization code", false},
		{"oauth_client_states", "code_verifier", "text", "the secret half of PKCE", false},
		{"scim_users", "external_id", "text", "the IdP's id for the person", false},
		{"webauthn_credentials", "credential_id", "bytea", "the authenticator's credential id", false},
		{"webauthn_credentials", "public_key", "bytea", "a stable per-person identifier", false},
	}

	for _, c := range cases {
		t.Run(c.table+"."+c.column, func(t *testing.T) {
			tbl := ref.TableRef{Schema: "auth", Name: c.table}
			cls, err := New().Classify(&pipeline.Schema{
				Tables: []pipeline.Table{tt("auth", c.table, []string{"id"},
					tc("id", "uuid"),
					tc(c.column, c.typeName),
				)},
				Fingerprint: "supabase-misses",
			}, mapSampler{}, nil)
			if err != nil {
				t.Fatalf("Classify: %v", err)
			}
			d := decision(t, cls, ref.ColumnRef{Table: tbl, Column: c.column})
			if d.Masked == c.wantMasked {
				return
			}
			if d.Masked {
				t.Fatalf("auth.%s.%s (%s) is now masked, and docs/TORTURE.md still records it as one of "+
					"supabase-auth's ten misses at recall 0.800. That is T-0104 landing: flip wantMasked here "+
					"and re-measure the truth set in docs/TORTURE.md.", c.table, c.column, c.holds)
			}
			t.Fatalf("auth.%s.%s (%s) was masked and is not any more: recall on supabase-auth has gone "+
				"backwards and this column now reaches the target in cleartext (THREAT_MODEL.md T1).",
				c.table, c.column, c.holds)
		})
	}
}
