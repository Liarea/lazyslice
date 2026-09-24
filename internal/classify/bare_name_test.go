// SPDX-License-Identifier: Apache-2.0

package classify

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
)

// T-0313, dogfood session 1: on a production Rails schema, 38 columns called
// just `name` (tags, folders, playlists, widgets, roles, languages, AI models,
// triggers) and every `*_file_name` column were masked as a person's name by
// the rule pack's bare `names?` word, and two of them, under unique indexes,
// refused the plan. rules.yml's bare_name rule now needs corroboration -- a
// word for people in the table or column name, or samples the name dictionary
// carries -- before it reaches `possible` (classify.go's bareNameVerdict).
//
// Both directions are pinned here, because a change that makes one of them
// pass by breaking the other is the change this file exists to catch: the
// labels must be copied, and every column the goal names as a person's must
// still be masked.

func bareNameSchema() (*pipeline.Schema, mapSampler) {
	tbl := func(name string) ref.TableRef { return ref.TableRef{Schema: "public", Name: name} }
	schema := &pipeline.Schema{
		Tables: []pipeline.Table{
			tt("public", "tags", []string{"id"}, tc("id", "bigint"), tc("name", "character varying(255)")),
			tt("public", "folders", []string{"id"}, tc("id", "bigint"), tc("name", "text"),
				tc("logo_file_name", "character varying(255)"), tc("transcode_file_file_name", "character varying(255)"),
				tc("file_name", "text")),
			tt("public", "ai_models", []string{"id"}, tc("id", "bigint"), tc("name", "text"), tc("display_name", "text")),
			tt("public", "plans", []string{"id"}, tc("id", "bigint"), tc("plan_name", "text")),
			tt("public", "languages", []string{"id"}, tc("id", "bigint"), tc("name", "character(20)")),
			tt("public", "users", []string{"id"}, tc("id", "bigint"), tc("name", "text")),
			tt("public", "staff", []string{"id"}, tc("id", "bigint"), tc("display_name", "text")),
			tt("public", "orders", []string{"id"}, tc("id", "bigint"), tc("customer_name", "text")),
			tt("public", "invoices", []string{"id"}, tc("id", "bigint"), tc("name", "text")),
			tt("public", "tickets", []string{"id"}, tc("id", "bigint"), tc("name", "text")),
			tt("public", "memos", []string{"id"}, tc("id", "bigint"), tc("name", "text")),
			tt("public", "drafts", []string{"id"}, tc("id", "bigint"), tc("name", "text")),
			tt("public", "widgets", []string{"id"}, tc("id", "bigint"), tc("name", "text"), tc("owner_email", "text")),
			tt("public", "contents", []string{"id"}, tc("id", "bigint"), tc("content_name", "text")),
			tt("public", "projects", []string{"id"}, tc("id", "bigint"), tc("name", "text")),
			tt("auth", "webauthn_credentials", []string{"id"}, tc("id", "bigint"), tc("friendly_name", "text")),
			tt("public", "clients", []string{"id"}, tc("id", "bigint"), tc("name", "text")),
			tt("public", "kyc_checks", []string{"id"}, append([]pipeline.Column{tc("id", "bigint")}, qualifiedNameColumns()...)...),
		},
	}
	labels := anyOf("urgent", "later", "ideas", "finance", "shopping", "recipes", "todo", "archive", "work", "travel")
	s := mapSampler{
		col(tbl("tags"), "name"):                        labels,
		col(tbl("folders"), "name"):                     anyOf("Inbox", "Sent", "Drafts", "Trash", "Spam", "Receipts", "Invoices"),
		col(tbl("folders"), "logo_file_name"):           anyOf("logo.png", "banner.jpg", "icon-512.png", "header_v2.svg"),
		col(tbl("folders"), "transcode_file_file_name"): anyOf("clip_1080p.mp4", "intro.webm", "outro.mov"),
		col(tbl("folders"), "file_name"):                anyOf("report-2024.pdf", "export.csv", "notes.txt"),
		col(tbl("ai_models"), "name"):                   anyOf("gpt-4o", "gpt-4o-mini", "llama-3-70b", "mistral-large", "whisper-1"),
		col(tbl("ai_models"), "display_name"):           anyOf("GPT-4o", "GPT-4o mini", "Llama 3 70B", "Mistral Large", "Whisper"),
		col(tbl("plans"), "plan_name"):                  anyOf("Free", "Pro", "Team", "Enterprise", "Starter"),
		col(tbl("languages"), "name"):                   anyOf("English             ", "Italian             ", "Japanese            ", "Mandarin            ", "French              ", "German              "),
		col(tbl("users"), "name"):                       anyOf("xX_zed_Xx", "qwop", "n00b", "kaz", "bzzt"),
		col(tbl("staff"), "display_name"):               anyOf("xX_zed_Xx", "qwop", "n00b", "kaz", "bzzt"),
		col(tbl("orders"), "customer_name"):             anyOf("xX_zed_Xx", "qwop", "n00b", "kaz", "bzzt"),
		col(tbl("invoices"), "name"):                    anyOf("Mary Smith", "Patricia Johnson", "Linda Williams", "Barbara Jones", "Elizabeth Brown"),
		col(tbl("tickets"), "name"):                     anyOf("Grace Hopper", "Printer jam", "VPN down", "Laptop refresh"),
		col(tbl("memos"), "name"):                       anyOf("Inbox", "Sent"),
		col(tbl("widgets"), "name"):                     anyOf("Revenue chart", "Active users", "Signups", "Churn", "Top pages"),
		col(tbl("widgets"), "owner_email"):              anyOf("a@fixture.test", "b@fixture.test", "c@fixture.test", "d@fixture.test", "e@fixture.test"),
		col(tbl("contents"), "content_name"):            labels,
		col(ref.TableRef{Schema: "auth", Name: "webauthn_credentials"}, "friendly_name"): anyOf("Grace's iPhone", "Alan's YubiKey", "Work laptop"),
		col(tbl("projects"), "name"): anyOf("王伟", "李娜", "张敏", "刘洋", "陈静"),
		col(tbl("clients"), "name"):  anyOf("王伟", "李娜", "张敏", "刘洋", "陈静"),
	}
	for _, c := range qualifiedNameColumns() {
		s[col(tbl("kyc_checks"), c.Name)] = anyOf("王伟", "李娜", "张敏", "刘洋", "陈静")
	}
	return schema, s
}

// qualifiedNameColumns are the `<qualifier>_name` spellings whose qualifier
// itself says the name is a person's (T-0313 review): they are person_name's
// row, not bare_name's, and are masked on the name alone even in a table not
// named for people and with samples the dictionary does not hold.
func qualifiedNameColumns() []pipeline.Column {
	var cols []pipeline.Column
	for _, n := range []string{"legal_name", "preferred_name", "nick_name", "birth_name", "married_name",
		"card_holder_name", "cardholder_name", "account_holder_name", "name_on_card", "billing_name",
		"shipping_name", "name_given", "name_family", "name_first", "name_last", "name_middle"} {
		cols = append(cols, tc(n, "text"))
	}
	return cols
}

func TestBareNameNeedsCorroboration(t *testing.T) {
	t.Parallel()
	schema, samples := bareNameSchema()
	cls, err := New().Classify(schema, samples, nil)
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	pub := func(table, column string) ref.ColumnRef {
		return ref.ColumnRef{Table: ref.TableRef{Schema: "public", Name: table}, Column: column}
	}
	for _, want := range []struct {
		col      ref.ColumnRef
		masked   bool
		category pipeline.Category
		reason   string // a substring the reason must carry
		why      string
	}{
		// The labels: copied, and the line says why.
		{pub("tags", "name"), false, pipeline.CatPersonName, "a bare name, not corroborated", "a tag list carries no dictionary name"},
		{pub("folders", "name"), false, pipeline.CatPersonName, "0/7 samples carry a word from the name dictionary", "mail folders are labels"},
		{pub("ai_models", "name"), false, pipeline.CatPersonName, "a bare name, not corroborated", "model identifiers are labels"},
		{pub("ai_models", "display_name"), false, pipeline.CatPersonName, "a bare name, not corroborated", "display_name of an object is a label"},
		{pub("plans", "plan_name"), false, pipeline.CatPersonName, "a bare name, not corroborated", "plan_name is a label"},
		{pub("languages", "name"), false, pipeline.CatPersonName, "a bare name, not corroborated", "pagila's own language names"},
		// A file name is outside the rule altogether, whatever it holds.
		{pub("folders", "logo_file_name"), false, pipeline.CatNone, "nothing recognised", "Paperclip's *_file_name is a label"},
		{pub("folders", "transcode_file_file_name"), false, pipeline.CatNone, "nothing recognised", "Paperclip's *_file_name is a label"},
		{pub("folders", "file_name"), false, pipeline.CatNone, "nothing recognised", "a file name is a label"},

		// The persons: masked, whatever the dictionary makes of the samples.
		{pub("users", "name"), true, pipeline.CatPersonName, "a bare name, corroborated by users, a word for people", "a users table names people"},
		{pub("staff", "display_name"), true, pipeline.CatPersonName, "corroborated by staff", "a staff table names people"},
		{pub("orders", "customer_name"), true, pipeline.CatPersonName, "corroborated by customer", "the column's own qualifier names a person"},
		{pub("invoices", "name"), true, pipeline.CatPersonName, "corroborated by 5/5 samples", "the dictionary carries the names"},
		{pub("tickets", "name"), true, pipeline.CatPersonName, "corroborated by 1/4 samples", "one name in four clears the threshold"},
		// docs/TORTURE.md labels supabase-auth's friendly_name personal: a
		// device named after its owner, as a phone names itself, carries the
		// owner's name in a possessive.
		{ref.ColumnRef{Table: ref.TableRef{Schema: "auth", Name: "webauthn_credentials"}, Column: "friendly_name"},
			true, pipeline.CatPersonName, "corroborated by 2/3 samples", "a possessive carries its owner's name"},
		// A CRM's clients table names people, whatever script they are in.
		{pub("clients", "name"), true, pipeline.CatPersonName, "corroborated by clients", "a clients table names people"},
		// Too few samples to ask the dictionary: masked on the name, as before.
		{pub("memos", "name"), true, pipeline.CatPersonName, "2 samples are too few", "an unproven column is not a clean one"},
		{pub("drafts", "name"), true, pipeline.CatPersonName, "0 samples are too few", "an empty table is masked on its name"},
		// A likely personal neighbour still raises an uncorroborated bare name.
		{pub("widgets", "name"), true, pipeline.CatPersonName, "raised by the neighbouring-column rule", "a likely neighbour is evidence about the table"},
		// A lower rule still names a column the bare name did not decide.
		{pub("contents", "content_name"), true, pipeline.CatFreeText, "name matches free_text", "content_name is free_text as well"},

		// THREAT_MODEL.md T1's stated residual: a person's name in a bare
		// name column of a table not named for people, in a script the
		// dictionary does not carry, is copied. It is pinned so that a change
		// that closes it is noticed and the residual text moved with it.
		{pub("projects", "name"), false, pipeline.CatPersonName, "0/5 samples carry a word from the name dictionary", "residual: non-Latin names in a non-person table"},
	} {
		d := decision(t, cls, want.col)
		if d.Masked != want.masked || d.Category != want.category {
			t.Errorf("%s = %s at %v masked=%v (%s), want %s masked=%v: %s",
				want.col, d.Category, d.Confidence, d.Masked, d.Reason, want.category, want.masked, want.why)
		}
		if !strings.Contains(d.Reason, want.reason) {
			t.Errorf("%s reason = %q, want it to carry %q", want.col, d.Reason, want.reason)
		}
		if bad, ok := ParseReason(d.Reason); !ok {
			t.Errorf("%s reason %q holds a fragment no template produced: %q", want.col, d.Reason, bad)
		}
	}

	// A qualifier that says the name is a person's is not a bare name: it is
	// masked on the name alone, in a table not named for people, with
	// samples the dictionary does not hold. Two are claimed by a
	// higher-priority rule before person_name (birth_name by person_date,
	// cardholder_name by financial_account), as they were before T-0313;
	// what is pinned is that the name alone masks each one.
	for _, c := range qualifiedNameColumns() {
		d := decision(t, cls, pub("kyc_checks", c.Name))
		if !d.Masked || !strings.HasPrefix(d.Reason, "name matches ") || strings.Contains(d.Reason, "bare name") {
			t.Errorf("kyc_checks.%s = %s at %v masked=%v (%s), want it masked on the name alone",
				c.Name, d.Category, d.Confidence, d.Masked, d.Reason)
		}
	}

	// users.name is masked, and every uncorroborated `name` column above is
	// named `name` too: sameColumnName must not carry users' people across to
	// the tags table.
	for _, c := range []ref.ColumnRef{pub("tags", "name"), pub("folders", "name"), pub("languages", "name")} {
		if d := decision(t, cls, c); strings.Contains(d.Reason, "column name name is") {
			t.Errorf("%s reason = %q: a same-named person column elsewhere raised an uncorroborated bare name", c, d.Reason)
		}
	}
}

// TestBareNameTruthSetsWithSamples is the recall measurement T-0313 asks for.
// Both hand-labelled truth sets classify names and types alone, which leaves
// every bare name unproven and so exactly as masked as before; this runs them
// again with samples on the bare-name columns, so the new rule is the one
// being measured. Pagila's are its own rows (testdata/pagila/pagila-data.sql);
// the held-out set's are illustrative, and the three personal ones are chosen
// so the dictionary carries none of them, which leaves the table name as the
// only corroboration: recall has to hold without the dictionary's help.
func TestBareNameTruthSetsWithSamples(t *testing.T) {
	t.Parallel()

	pagila := pagilaSamples()
	pt := func(name string) ref.TableRef { return ref.TableRef{Schema: "public", Name: name} }
	pagila[col(pt("category"), "name")] = anyOf("Action", "Animation", "Children", "Classics", "Comedy",
		"Documentary", "Drama", "Family", "Foreign", "Games", "Horror", "Music", "New", "Sci-Fi", "Sports", "Travel")
	pagila[col(pt("language"), "name")] = anyOf("English             ", "Italian             ",
		"Japanese            ", "Mandarin            ", "French              ", "German              ")
	cls, err := New().Classify(pagilaSchema(), pagila, nil)
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	s := score(t, cls, pagilaTruth)
	s.print(t, "Pagila, with its own category and language names sampled:")
	if s.recall() < 1.0 {
		t.Errorf("pagila recall = %.3f, want 1.0 (THREAT_MODEL.md T1)", s.recall())
	}
	for _, c := range []string{"category", "language"} {
		if d := decision(t, cls, col(pt(c), "name")); d.Masked {
			t.Errorf("pagila %s.name is masked (%s): its own rows are labels", c, d.Reason)
		}
	}

	schema, truth := namesSchema()
	nt := func(source, name string) ref.TableRef { return ref.TableRef{Schema: source, Name: name} }
	nicks := anyOf("xX_zed_Xx", "qwop", "n00b", "kaz", "bzzt")
	names := mapSampler{
		col(nt("discourse", "users"), "name"):           nicks,
		col(nt("gitlab", "users"), "name"):              nicks,
		col(nt("mastodon", "accounts"), "display_name"): nicks,
		col(nt("gitlab", "projects"), "name"):           anyOf("gitlab", "gitaly", "gitlab-runner", "omnibus-gitlab", "www-gitlab-com"),
	}
	cls, err = New().Classify(schema, names, nil)
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	s = score(t, cls, truth)
	s.print(t, "Discourse, GitLab, Mastodon and Supabase auth, with the four bare-name columns sampled:")
	if s.recall() < 1.0 {
		t.Errorf("held-out recall = %.3f, want 1.0 (THREAT_MODEL.md T1)", s.recall())
	}
	if d := decision(t, cls, col(nt("gitlab", "projects"), "name")); d.Masked {
		t.Errorf("gitlab.projects.name is masked (%s): project names are labels", d.Reason)
	}
}

// TestCorroboratedByIsReadOnAPersonNameRuleOnly pins decodePack's refusal of a
// corroborated_by field on any other category: the name dictionary is the
// only value evidence bareNameVerdict can corroborate a rule with, so on an
// email rule it would decide an email column on evidence about names.
func TestCorroboratedByIsReadOnAPersonNameRuleOnly(t *testing.T) {
	t.Parallel()
	const base = `
version: "test"
categories:
  - category: %s
    masker: "fixed:redacted"
    accepts: ["*"]
patterns:
  - name: a_rule
    category: %s
    priority: 10
    match: "^thing$"
    corroborated_by: "(^|_)users?(_|$)"
`
	if _, err := decodePack([]byte(fmt.Sprintf(base, "email", "email"))); err == nil {
		t.Errorf("decodePack accepted corroborated_by on an email rule")
	}
	if _, err := decodePack([]byte(fmt.Sprintf(base, "person_name", "person_name"))); err != nil {
		t.Errorf("decodePack refused corroborated_by on a person_name rule, the one it is for: %v", err)
	}
}
