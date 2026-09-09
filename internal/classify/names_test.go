// SPDX-License-Identifier: Apache-2.0

package classify

import (
	"testing"

	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
)

// Column names taken from four open-source schemas — Discourse, GitLab,
// Mastodon and Supabase's auth schema — with a hand label for each and a
// printed confusion matrix.
//
// Why these: they are large, public, unrelated to each other, and none of them
// was consulted while the rule pack was written, so the names below are the
// closest thing this package has to a held-out set. They are names and types
// only: no row of anyone's data is in this repository, and none is needed to
// measure a name-based classifier.
//
// The first three were the fifty-name fixture. The Supabase block is T-0104:
// every spelling the credential and online_id rules gained is measured here,
// beside the names that were already scored, so that widening a pattern is
// scored against the same matrix as everything else rather than only against
// the ten columns it was written for. The held-out claim is weaker for that
// block — those rules were written *from* those columns — which is why the
// block is named and not folded in silently.
//
// The label answers one question — "in that application, does this column hold
// personal data about an identifiable person?" — and it is a judgement, so the
// disagreements are printed by name rather than only counted. A false negative
// here is the failure THREAT_MODEL.md T1 is about; a false positive costs a
// column that did not need masking.

type namedColumn struct {
	source   string // the schema the name was taken from
	table    string
	column   string
	typeName string
	personal bool
}

var heldOutNames = []namedColumn{
	// Discourse.
	{"discourse", "users", "username", "text", true},
	{"discourse", "users", "name", "text", true},
	{"discourse", "users", "created_at", "timestamp with time zone", false},
	{"discourse", "users", "last_seen_at", "timestamp with time zone", false},
	{"discourse", "users", "trust_level", "integer", false},
	{"discourse", "users", "admin", "boolean", false},
	{"discourse", "users", "ip_address", "inet", true},
	{"discourse", "users", "registration_ip_address", "inet", true},
	{"discourse", "users", "password_hash", "character varying(64)", true},
	{"discourse", "users", "salt", "character varying(32)", true},
	{"discourse", "user_emails", "email", "character varying(513)", true},
	{"discourse", "user_profiles", "bio_raw", "text", true},
	{"discourse", "user_profiles", "location", "text", true},
	{"discourse", "user_profiles", "website", "text", true},
	{"discourse", "posts", "raw", "text", true},
	{"discourse", "posts", "cooked", "text", true},
	{"discourse", "topics", "title", "character varying(255)", false},
	{"discourse", "user_auth_tokens", "auth_token", "character varying(255)", true},

	// GitLab.
	{"gitlab", "users", "email", "character varying(255)", true},
	{"gitlab", "users", "encrypted_password", "character varying(255)", true},
	{"gitlab", "users", "username", "character varying(255)", true},
	{"gitlab", "users", "name", "character varying(255)", true},
	{"gitlab", "users", "current_sign_in_ip", "character varying(255)", true},
	{"gitlab", "users", "last_sign_in_ip", "character varying(255)", true},
	{"gitlab", "users", "sign_in_count", "integer", false},
	{"gitlab", "users", "confirmed_at", "timestamp without time zone", false},
	{"gitlab", "user_details", "bio", "text", true},
	{"gitlab", "user_details", "location", "character varying(255)", true},
	{"gitlab", "user_details", "linkedin", "character varying(500)", true},
	{"gitlab", "user_details", "twitter", "character varying(500)", true},
	{"gitlab", "user_details", "skype", "character varying(500)", true},
	{"gitlab", "projects", "name", "character varying(255)", false},
	{"gitlab", "projects", "description", "text", false},
	{"gitlab", "namespaces", "path", "character varying(255)", false},
	{"gitlab", "personal_access_tokens", "token_digest", "character varying(255)", true},
	{"gitlab", "merge_requests", "title", "character varying(255)", false},
	{"gitlab", "notes", "note", "text", true},

	// Mastodon.
	{"mastodon", "accounts", "username", "character varying(255)", true},
	{"mastodon", "accounts", "display_name", "character varying(255)", true},
	{"mastodon", "accounts", "note", "text", true},
	{"mastodon", "accounts", "domain", "character varying(255)", false},
	{"mastodon", "accounts", "private_key", "text", true},
	// Relabelled personal by T-0121 (settled 2026-09-09), together with
	// supabase.webauthn_credentials.public_key below and with the credential
	// rule in rules.yml that now matches both. It sat at not-personal from the
	// day this fixture was written until then, on the argument that a
	// per-account public key is published in the actor document and so is not a
	// secret; what settled it is the other half of T-0104's question, that it is
	// a stable identifier for exactly one person and nothing a development
	// database does needs the real one. Moving a label and the rule scored
	// against it in one change is normally what this file forbids; it is the
	// exception the decision itself asked for, the two labels and the rule were
	// moved by a task that did not write the T-0104 patterns, and the movement
	// is recorded in docs/TORTURE.md rather than only here.
	{"mastodon", "accounts", "public_key", "text", true},
	{"mastodon", "users", "email", "character varying(255)", true},
	{"mastodon", "users", "encrypted_password", "character varying(255)", true},
	{"mastodon", "users", "sign_up_ip", "inet", true},
	{"mastodon", "users", "current_sign_in_at", "timestamp without time zone", false},
	{"mastodon", "users", "locale", "character varying(255)", false},
	{"mastodon", "statuses", "text", "text", true},
	{"mastodon", "statuses", "spoiler_text", "text", true},

	// GitLab, the identity half (T-0104). extern_uid is the id the external
	// identity provider issues for this person.
	{"gitlab", "identities", "extern_uid", "character varying(255)", true},

	// Supabase auth (T-0104). These are the ten columns docs/TORTURE.md's
	// hand-labelled truth set recorded as missed at recall 0.800, which is 0.980
	// there now with one of the ten left;
	// supabase_misses_test.go pins each one's decision individually, and this is
	// where they are scored beside everything else.
	{"supabase", "flow_state", "auth_code", "text", true},
	{"supabase", "mfa_challenges", "otp_code", "text", true},
	{"supabase", "mfa_recovery_codes", "code_hash", "text", true},
	{"supabase", "oauth_authorizations", "authorization_code", "text", true},
	{"supabase", "oauth_client_states", "code_verifier", "text", true},
	{"supabase", "scim_users", "external_id", "text", true},
	{"supabase", "identities", "provider_id", "text", true},
	{"supabase", "webauthn_credentials", "credential_id", "bytea", true},
	// Labelled to agree with mastodon's above, for the reason written there.
	{"supabase", "webauthn_credentials", "public_key", "bytea", true},
	// Still a false negative, and deliberately left as one: a rule matching
	// `parents?` would mask every parent_id join key in every schema there is.
	// See supabase_misses_test.go.
	{"supabase", "refresh_tokens", "parent", "character varying(255)", true},
	// The negatives of the same schema, so that the block is not all-positive
	// and a pattern that widened too far is visible here as a false positive.
	{"supabase", "users", "created_at", "timestamp with time zone", false},
	{"supabase", "mfa_factors", "factor_type", "text", false},
	{"supabase", "sessions", "not_after", "timestamp with time zone", false},
}

// namesSchema turns the fixture into a schema, keeping each source's tables
// apart so that the neighbouring-column rule sees the tables the names actually
// come from.
func namesSchema() (*pipeline.Schema, map[string]bool) {
	type key struct{ source, table string }
	order := []key{}
	cols := map[key][]pipeline.Column{}
	truth := map[string]bool{}
	for _, n := range heldOutNames {
		k := key{n.source, n.table}
		if _, ok := cols[k]; !ok {
			order = append(order, k)
		}
		cols[k] = append(cols[k], tc(n.column, n.typeName))
		truth[(ref.ColumnRef{
			Table:  ref.TableRef{Schema: n.source, Name: n.table},
			Column: n.column,
		}).String()] = n.personal
	}
	schema := &pipeline.Schema{Fingerprint: "held-out-names"}
	for _, k := range order {
		schema.Tables = append(schema.Tables, tt(k.source, k.table, nil, cols[k]...))
	}
	return schema, truth
}

// TestFiftyNamesFromThreeSchemas prints the confusion matrix and holds the two
// rates to a floor. The floor on recall is the strict one, for the reason
// THREAT_MODEL.md T1 gives: a column this classifier misses is cleartext in the
// target under a green tick.
//
// The name is kept from when the fixture was fifty names from three schemas
// (T-0104 added the fourth), because it is the name internal/classify/CLAUDE.md
// and internal/textsig/CLAUDE.md tell a reader to run.
//
// The exact count is asserted rather than a minimum: a fixture that quietly
// shrinks is a measurement that quietly stops covering something, and the whole
// value of this file is that the rates below are over a set nobody trimmed to
// make them look better. That cuts both ways, and it is why no *label* here
// moved in the change that widened the rules scored against it (T-HARD-B): a
// relabelled column turns a false positive into a true positive without the
// classifier doing anything, and it is not the rule author's call to make in
// the same commit. Exactly one label has ever moved — public_key, both rows,
// under T-0121's own decision that the column is a credential — and it moved in
// a task that wrote none of the patterns it is scored against. What that
// movement did to the rates, measured either side of it: 44 → 46 true
// positives, 17 → 15 true negatives, two false positives and one false negative
// unchanged, precision 0.957 → 0.958 and recall 0.978 → 0.979. No label here is
// open now.
func TestFiftyNamesFromThreeSchemas(t *testing.T) {
	t.Parallel()
	if len(heldOutNames) != 64 {
		t.Fatalf("the fixture holds %d names, and it held 64: adding one is a decision, "+
			"and removing one is a measurement that stopped covering something", len(heldOutNames))
	}
	schema, truth := namesSchema()
	cls, err := New().Classify(schema, mapSampler{}, nil)
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	s := score(t, cls, truth)
	s.print(t, "Columns from Discourse, GitLab, Mastodon and Supabase auth, names and types only:")

	if s.recall() < 0.95 {
		t.Errorf("recall = %.3f, want at least 0.95 (THREAT_MODEL.md T1)", s.recall())
	}
	if s.precision() < 0.80 {
		t.Errorf("precision = %.3f, want at least 0.80", s.precision())
	}
}
