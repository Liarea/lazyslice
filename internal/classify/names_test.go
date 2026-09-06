// SPDX-License-Identifier: Apache-2.0

package classify

import (
	"testing"

	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
)

// Fifty column names taken from three open-source schemas — Discourse, GitLab
// and Mastodon — with a hand label for each and a printed confusion matrix.
//
// Why these three: they are large, public, unrelated to each other, and none of
// them was consulted while the rule pack was written, so the names below are
// the closest thing this package has to a held-out set. They are names and
// types only: no row of anyone's data is in this repository, and none is needed
// to measure a name-based classifier.
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

var fiftyNames = []namedColumn{
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
	{"mastodon", "accounts", "public_key", "text", false},
	{"mastodon", "users", "email", "character varying(255)", true},
	{"mastodon", "users", "encrypted_password", "character varying(255)", true},
	{"mastodon", "users", "sign_up_ip", "inet", true},
	{"mastodon", "users", "current_sign_in_at", "timestamp without time zone", false},
	{"mastodon", "users", "locale", "character varying(255)", false},
	{"mastodon", "statuses", "text", "text", true},
	{"mastodon", "statuses", "spoiler_text", "text", true},
}

// namesSchema turns the fixture into a schema, keeping each source's tables
// apart so that the neighbouring-column rule sees the tables the names actually
// come from.
func namesSchema() (*pipeline.Schema, map[string]bool) {
	type key struct{ source, table string }
	order := []key{}
	cols := map[key][]pipeline.Column{}
	truth := map[string]bool{}
	for _, n := range fiftyNames {
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
	schema := &pipeline.Schema{Fingerprint: "fifty-names"}
	for _, k := range order {
		schema.Tables = append(schema.Tables, tt(k.source, k.table, nil, cols[k]...))
	}
	return schema, truth
}

// TestFiftyNamesFromThreeSchemas prints the confusion matrix and holds the two
// rates to a floor. The floor on recall is the strict one, for the reason
// THREAT_MODEL.md T1 gives: a column this classifier misses is cleartext in the
// target under a green tick.
func TestFiftyNamesFromThreeSchemas(t *testing.T) {
	t.Parallel()
	if len(fiftyNames) != 50 {
		t.Fatalf("the fixture holds %d names, and it is called the fifty-name fixture", len(fiftyNames))
	}
	schema, truth := namesSchema()
	cls, err := New().Classify(schema, mapSampler{}, nil)
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	s := score(t, cls, truth)
	s.print(t, "Fifty columns from Discourse, GitLab and Mastodon, names and types only:")

	if s.recall() < 0.95 {
		t.Errorf("recall = %.3f, want at least 0.95 (THREAT_MODEL.md T1)", s.recall())
	}
	if s.precision() < 0.80 {
		t.Errorf("precision = %.3f, want at least 0.80", s.precision())
	}
}
