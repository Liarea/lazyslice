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
// and this was the 0.200. **All ten are masked now** (T-0104, `public_key` at
// T-0121, `refresh_tokens.parent` at T-0119) and recall is 1.000, re-measured in
// docs/TORTURE.md; this file is the column-by-column half of that number.
//
// It was written asserting the leak, with the instruction to flip each entry
// rather than delete the file when the rules landed, and that is what has
// happened to all ten now. It still fails in both directions — a column that
// stops being masked fails it too — which is the whole point of keeping it.
//
// **The other two were decisions and not omissions**, which is what T-0104 said
// of them: "two of the ten deserve a decision rather than a pattern". Both are
// decided now.
//
// `webauthn_credentials.public_key` is the first, decided at T-0121
// (2026-09-09): the column is `credential`, masked to the unusable fixed literal,
// or `credential_unique` where a unique index makes the literal collide. A
// public key is published by design — WebAuthn hands it to every relying party
// and mastodon puts the account's in its actor document — so it is not a secret;
// what decided it is that a key identifying exactly one person is a per-person
// identifier that outlives every other value in the row, and nothing a local
// development database does verifies an assertion or serves an actor document,
// so the real value buys nothing there. The hand labels in names_test.go moved
// with the rule, and docs/TORTURE.md records what the rates did.
//
// `refresh_tokens.parent` is the second, decided at T-0119: the column holds
// another refresh token in a column named after a tree edge. There is no *name*
// rule to write — a rule matching `parents?` would match `parent_id` in every
// schema there is, at priority 80, and mask the join keys of half a database to
// the fixed literal, and this package's name rules used to see the column name
// alone, so "parent, in a table called refresh_tokens" was not a rule they could
// express. `rules.yml`'s `table_patterns:` is that rule-pack feature: a name
// rule gated by a second regexp over the table, tried only within a table that
// regexp matches. `refresh_token_parent` there is `credential` at priority 80,
// scoped to `(^|_)refresh_?tokens?(_|$)`, and catches this column by name alone
// exactly where the value signal used to be the only thing that could —
// `textsig.LooksSecret` reads a token, but the torture fixture had none to read
// before the name rule ran; now the name decides it regardless.
//
// It is a name-rule test, not a schema replay. Each column is put in a table of
// its own with no samples, because seven of the ten are empty in the fixture and
// the name is the only signal any of them could have had — which is precisely
// the case T-0104 said the rule pack was thin for. Isolating them also keeps the
// neighbouring-column rule out of the answer, so a change to a *name pattern* or
// a *table pattern* is what moves this file.
func TestSupabaseAuthMissesArePinned(t *testing.T) {
	t.Parallel()

	cases := []struct {
		table, column, typeName string
		holds                   string
		wantMasked              bool
	}{
		{"flow_state", "auth_code", "text", "the OAuth authorization code", true},
		{"identities", "provider_id", "text", "the provider's subject id for the person", true},
		{"refresh_tokens", "parent", "character varying(255)", "another refresh token", true},
		{"mfa_challenges", "otp_code", "text", "the one-time code", true},
		{"mfa_recovery_codes", "code_hash", "text", "a recovery code", true},
		{"oauth_authorizations", "authorization_code", "text", "the authorization code", true},
		{"oauth_client_states", "code_verifier", "text", "the secret half of PKCE", true},
		{"scim_users", "external_id", "text", "the IdP's id for the person", true},
		{"webauthn_credentials", "credential_id", "bytea", "the authenticator's credential id", true},
		{"webauthn_credentials", "public_key", "bytea", "a stable per-person identifier", true},
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

// TestRefreshTokenParentRuleIsTableScoped is the negative case
// TestSupabaseAuthMissesArePinned cannot express: refresh_token_parent
// (rules.yml's table_patterns:, T-0119) must fire only inside a table its own
// `table:` regexp matches. Every column named `parent` anywhere else in this
// package's fixtures lives in a table called refresh_tokens (both cases
// above), so on its own that positive result does not prove the rule is
// scoped at all — deleting the tableRe gate in matchColumn and making the rule
// a global credential rule at priority 80 leaves every other test in this
// package green. These two tables are ordinary, unrelated parent-child join
// keys, exactly the columns T-0119's comment says an unscoped `parents?` rule
// would mismask, and they must stay unmasked.
func TestRefreshTokenParentRuleIsTableScoped(t *testing.T) {
	t.Parallel()

	cases := []struct{ table, holds string }{
		{"categories", "a self-referencing join key, not a credential"},
		{"comments", "a self-referencing join key, not a credential"},
	}

	for _, c := range cases {
		t.Run(c.table+".parent", func(t *testing.T) {
			tbl := ref.TableRef{Schema: "public", Name: c.table}
			cls, err := New().Classify(&pipeline.Schema{
				Tables: []pipeline.Table{tt("public", c.table, []string{"id"},
					tc("id", "uuid"),
					tc("parent", "character varying(255)"),
				)},
				Fingerprint: "table-scope-negative",
			}, mapSampler{}, nil)
			if err != nil {
				t.Fatalf("Classify: %v", err)
			}
			d := decision(t, cls, ref.ColumnRef{Table: tbl, Column: "parent"})
			if d.Masked {
				t.Fatalf("public.%s.parent (%s) was masked: refresh_token_parent's table: scope leaked "+
					"into a table it does not name, exactly what T-0119 wrote the scope to prevent "+
					"(reason: %q)", c.table, c.holds, d.Reason)
			}
		})
	}
}
