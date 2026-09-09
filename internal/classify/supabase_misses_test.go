// SPDX-License-Identifier: Apache-2.0

package classify

import (
	"testing"

	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
)

// The ten columns docs/TORTURE.md's supabase-auth truth set records as missed:
// hand-labelled personal, classified below §4's mask threshold, and therefore
// copied into the target verbatim under exit 0. Recall on that schema was 0.800
// and this was the 0.200.
//
// **Eight of the ten are masked now** (T-0104): the credential and online_id
// name patterns in rules.yml gained the spellings a real authentication schema
// uses, and this file is the pin that says so. It was written asserting the
// leak, with the instruction to flip each entry rather than delete the file
// when the rules landed, and that is what has happened. It still fails in both
// directions — a column that stops being masked fails it too — which is the
// whole point of keeping it.
//
// **The other two are decisions and not omissions**, which is what T-0104 said
// of them: "two of the ten deserve a decision rather than a pattern".
//
// `webauthn_credentials.public_key` is the first, and it is still open (T-0121).
// A public key is published by design — WebAuthn hands it to every relying
// party and mastodon puts the account's in its actor document — so the
// credential rule would mask a value that is not secret to the fixed literal,
// at a priority that outranks everything else; and it is also a stable
// identifier for exactly one person, which is the online_id argument. Those are
// two different maskers and two different answers, the hand label in
// names_test.go moves with whichever wins, and neither the rule nor the label
// is a rule author's call to take alone. Until it is settled the column is
// copied, and that is recorded here rather than anywhere quieter.
//
// `refresh_tokens.parent` is the second.
// holds another refresh token in a column named after a tree edge, and T-0104
// says it deserves a decision rather than a pattern. There is no pattern to
// write: a rule matching `parents?` would match `parent_id` in every schema
// there is, at priority 80, and mask the join keys of half a database to the
// fixed literal — and this package's name rules see the column name alone, so
// "parent, in a table called refresh_tokens" is not a rule it can express. What
// catches it in a real run is the value signal: the column holds a token, and
// textsig.LooksSecret reads one. It is empty in the torture fixture, which is
// why it is still copied there. Widening the rule pack to catch it needs a
// table-scoped pattern, which is a rule-pack feature and not a rule — tracker
// task, not this change.
//
// It is a name-rule test, not a schema replay. Each column is put in a table of
// its own with no samples, because seven of the ten are empty in the fixture and
// the name is the only signal any of them could have had — which is precisely
// the case T-0104 said the rule pack was thin for. Isolating them also keeps the
// neighbouring-column rule out of the answer, so a change to a *name pattern*
// is what moves this file.
func TestSupabaseAuthMissesArePinned(t *testing.T) {
	t.Parallel()

	cases := []struct {
		table, column, typeName string
		holds                   string
		wantMasked              bool
	}{
		{"flow_state", "auth_code", "text", "the OAuth authorization code", true},
		{"identities", "provider_id", "text", "the provider's subject id for the person", true},
		// The two that are still copied, and on purpose: see the note above.
		{"refresh_tokens", "parent", "character varying(255)", "another refresh token", false},
		{"mfa_challenges", "otp_code", "text", "the one-time code", true},
		{"mfa_recovery_codes", "code_hash", "text", "a recovery code", true},
		{"oauth_authorizations", "authorization_code", "text", "the authorization code", true},
		{"oauth_client_states", "code_verifier", "text", "the secret half of PKCE", true},
		{"scim_users", "external_id", "text", "the IdP's id for the person", true},
		{"webauthn_credentials", "credential_id", "bytea", "the authenticator's credential id", true},
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
				t.Fatalf("auth.%s.%s (%s) is now masked, and this file still records it as one of "+
					"supabase-auth's misses. Flip wantMasked here and re-measure the truth set in "+
					"docs/TORTURE.md.", c.table, c.column, c.holds)
			}
			t.Fatalf("auth.%s.%s (%s) was masked and is not any more: recall on supabase-auth has gone "+
				"backwards and this column now reaches the target in cleartext (THREAT_MODEL.md T1).",
				c.table, c.column, c.holds)
		})
	}
}
