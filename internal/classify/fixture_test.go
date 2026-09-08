// SPDX-License-Identifier: Apache-2.0

package classify

import (
	"crypto/sha256"
	"encoding/hex"

	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
)

// A recorded introspection of testdata/nasty.sql, and the sample values
// introspect would have taken from it.
//
// It is recorded rather than introspected because this package is pure and its
// tests need no database: internal/introspect's own integration tests are what
// assert that a real PostgreSQL returns this shape, and
// internal/testutil/fixtures_test.go is what asserts that nasty.sql still has
// these columns and these rows. What is here is the classifier's half of the
// contract — given this schema and these samples, these decisions — and it runs
// in milliseconds with `go test ./internal/classify/...`.
//
// The sample values are the fixture's own rows, copied from testdata/nasty.sql.
// They are test data about five fictional people, not production values.

func fp(parts ...string) string {
	h := sha256.Sum256([]byte(joinFP(parts)))
	return hex.EncodeToString(h[:])[:8]
}

func joinFP(parts []string) string {
	out := ""
	for _, p := range parts {
		out += p + "\x00"
	}
	return out
}

// tc is one recorded column.
func tc(name, typeName string) pipeline.Column {
	return pipeline.Column{
		Name:        name,
		TypeName:    typeName,
		Nullable:    true,
		Fingerprint: fp(name, typeName),
	}
}

func generated(c pipeline.Column, expr string) pipeline.Column {
	c.Generated = expr
	return c
}

// tt is one recorded table.
func tt(schema, name string, pk []string, cols ...pipeline.Column) pipeline.Table {
	return pipeline.Table{
		Ref:     ref.TableRef{Schema: schema, Name: name},
		Columns: cols,
		PK:      pk,
	}
}

func fk(name string, child ref.TableRef, childCols []string, parent ref.TableRef, parentCols []string) pipeline.ForeignKey {
	return pipeline.ForeignKey{
		Name: name, Child: child, ChildCols: childCols,
		Parent: parent, ParentCols: parentCols, Validated: true,
	}
}

var (
	tPeople     = ref.TableRef{Schema: "public", Name: "people"}
	tOrders     = ref.TableRef{Schema: "public", Name: "orders"}
	tOrderItems = ref.TableRef{Schema: "public", Name: "order_items"}
	tTenantU    = ref.TableRef{Schema: "public", Name: "tenant_users"}
	tSessions   = ref.TableRef{Schema: "public", Name: "tenant_user_sessions"}
	tFlags      = ref.TableRef{Schema: "public", Name: "tenant_user_flags"}
	tOrgs       = ref.TableRef{Schema: "public", Name: "organisations"}
	tTeams      = ref.TableRef{Schema: "public", Name: "teams"}
	tProjects   = ref.TableRef{Schema: "public", Name: "projects"}
	tAttach     = ref.TableRef{Schema: "public", Name: "attachments"}
	tEvents     = ref.TableRef{Schema: "public", Name: "events"}
	tEvents24   = ref.TableRef{Schema: "public", Name: "events_2024"}
	tEvents25   = ref.TableRef{Schema: "public", Name: "events_2025"}
	tLegacy     = ref.TableRef{Schema: "public", Name: "LegacyCustomer"}
	tSites      = ref.TableRef{Schema: "public", Name: "sites"}
	tDevices    = ref.TableRef{Schema: "public", Name: "devices"}
	tReadings   = ref.TableRef{Schema: "public", Name: "device_readings"}
	tAudit      = ref.TableRef{Schema: "public", Name: "audit_log"}
	tClicks     = ref.TableRef{Schema: "public", Name: "click_stream"}
	tInvoices   = ref.TableRef{Schema: "billing", Name: "invoices"}
	tStream     = ref.TableRef{Schema: "public", Name: "stream_rows"}
)

func col(t ref.TableRef, name string) ref.ColumnRef { return ref.ColumnRef{Table: t, Column: name} }

// nastySchema is the recorded schema.
func nastySchema() *pipeline.Schema {
	eventCols := func() []pipeline.Column {
		return []pipeline.Column{
			tc("event_id", "bigint"),
			tc("person_id", "bigint"),
			tc("occurred_at", "timestamp with time zone"),
			tc("kind", "text"),
			tc("payload", "jsonb"),
		}
	}
	events := tt("public", "events", []string{"event_id", "occurred_at"}, eventCols()...)
	events.Partitioned = true
	events.PartitionKey = []string{"occurred_at"}
	events.Partitions = []ref.TableRef{tEvents24, tEvents25}
	events.SampledFrom = &tEvents24

	events24 := tt("public", "events_2024", []string{"event_id", "occurred_at"}, eventCols()...)
	events24.Parent = &tEvents
	events25 := tt("public", "events_2025", []string{"event_id", "occurred_at"}, eventCols()...)
	events25.Parent = &tEvents

	audit := tt("public", "audit_log", nil,
		tc("entry_uid", "text"),
		tc("person_id", "bigint"),
		tc("action", "text"),
		tc("client_ip", "inet"),
		tc("occurred_at", "timestamp with time zone"),
	)
	// Def is pg_get_indexdef verbatim. It is recorded because the expression
	// index is the only place the columns an index covers can be read from:
	// pg_index reports no attnum for an expression, so introspect leaves
	// Index.Columns empty and internal/classify parses the definition.
	audit.Indexes = []pipeline.Index{
		{
			Name: "audit_log_entry_uid_key", Columns: []string{"entry_uid"}, Unique: true, Immediate: true,
			Def: "CREATE UNIQUE INDEX audit_log_entry_uid_key ON public.audit_log USING btree (entry_uid)",
		},
		{
			Name: "audit_log_recent_action_key", Columns: []string{"action"}, Unique: true, Partial: true, Immediate: true,
			Def: "CREATE UNIQUE INDEX audit_log_recent_action_key ON public.audit_log USING btree (action) WHERE (occurred_at >= '2025-01-01 00:00:00+00'::timestamp with time zone)",
		},
		{
			Name: "audit_log_lower_entry_uid_key", Unique: true, Expression: true, Immediate: true,
			Def: "CREATE UNIQUE INDEX audit_log_lower_entry_uid_key ON public.audit_log USING btree (lower(entry_uid))",
		},
	}

	legacy := tt("public", "LegacyCustomer", []string{"CustomerID"},
		tc("CustomerID", "integer"),
		tc("MigratedFromPersonID", "integer"),
		tc("EmailAddress", "text"),
		tc("ContactNumber", "character varying(15)"),
		tc("MobileNumber", "text"),
		tc("Notes", "text"),
	)
	legacy.Indexes = []pipeline.Index{
		{Name: "LegacyCustomer_EmailAddress_key", Columns: []string{"EmailAddress"}, Unique: true, Immediate: true},
		{Name: "LegacyCustomer_ContactNumber_key", Columns: []string{"ContactNumber"}, Unique: true, Immediate: true},
	}
	legacy.Columns[2].Checks = []string{`CHECK (("EmailAddress" ~~ '%@%.%'::text))`}

	people := tt("public", "people", []string{"person_id"},
		tc("person_id", "bigint"),
		tc("manager_id", "bigint"),
		tc("given_name", "text"),
		tc("family_name", "text"),
		generated(tc("display_name", "text"), "(given_name || ' '::text) || family_name"),
		tc("email_verified", "boolean"),
		tc("ref", "text"),
		tc("status", "public.account_status"),
		tc("marital_status", "public.marital_status"),
		tc("contact", "jsonb"),
		tc("alt_emails", "text[]"),
		tc("notes", "text"),
		tc("preferred_order_id", "bigint"),
	)
	people.Columns[0].Identity = "a"

	return &pipeline.Schema{
		ServerVersion: 160000,
		Schemas:       []string{"billing", "public"},
		Enums: map[string][]string{
			"public.account_status": {"pending", "active", "suspended", "closed"},
			"public.marital_status": {"single", "married", "civil_partnership", "divorced", "widowed", "undisclosed"},
		},
		Tables: []pipeline.Table{
			tt("billing", "invoices", []string{"invoice_id"},
				tc("invoice_id", "bigint"),
				tc("person_id", "bigint"),
				tc("bill_to_email", "text"),
				tc("bill_to_phone", "text"),
				tc("total_pence", "bigint"),
				tc("issued_on", "date"),
			),
			legacy,
			tt("public", "attachments", []string{"attachment_id"},
				tc("attachment_id", "bigint"),
				tc("owner_type", "text"),
				tc("owner_id", "bigint"),
				tc("uploaded_by_person_id", "bigint"),
				tc("filename", "text"),
				tc("uploaded_by", "text"),
			),
			audit,
			tt("public", "click_stream", nil,
				tc("person_id", "bigint"),
				tc("url", "text"),
				tc("clicked_at", "timestamp with time zone"),
			),
			tt("public", "device_readings", []string{"device_id", "taken_at"},
				tc("device_id", "uuid"),
				tc("taken_at", "timestamp with time zone"),
				tc("celsius", "numeric(5,2)"),
			),
			tt("public", "devices", []string{"device_id"},
				tc("device_id", "uuid"),
				tc("site_code", "text"),
				tc("asset_tag", "text"),
				tc("owned_by", "text"),
			),
			events, events24, events25,
			tt("public", "order_items", []string{"order_id", "line_no"},
				tc("order_id", "bigint"),
				tc("line_no", "integer"),
				tc("sku", "text"),
				tc("qty", "integer"),
			),
			tt("public", "orders", []string{"order_id"},
				tc("order_id", "bigint"),
				tc("person_id", "bigint"),
				tc("placed_at", "timestamp with time zone"),
				tc("total_pence", "integer"),
			),
			tt("public", "organisations", []string{"organisation_id"},
				tc("organisation_id", "integer"),
				tc("name", "text"),
				tc("founded_by", "bigint"),
				tc("primary_team_id", "integer"),
			),
			people,
			tt("public", "projects", []string{"project_id"},
				tc("project_id", "integer"),
				tc("name", "text"),
				tc("owner_organisation_id", "integer"),
			),
			tt("public", "sites", []string{"site_code"},
				tc("site_code", "text"),
				tc("name", "text"),
				tc("owner_person_id", "bigint"),
				tc("contact_email", "text"),
			),
			tt("public", "stream_rows", []string{"stream_row_id"},
				tc("stream_row_id", "bigint"),
				tc("person_id", "bigint"),
				tc("email", "text"),
				tc("body", "text"),
			),
			tt("public", "teams", []string{"team_id"},
				tc("team_id", "integer"),
				tc("name", "text"),
				tc("lead_project_id", "integer"),
			),
			tt("public", "tenant_user_flags", []string{"flag_id"},
				tc("flag_id", "bigint"),
				tc("tenant_id", "integer"),
				tc("user_id", "integer"),
				tc("flag", "text"),
			),
			tt("public", "tenant_user_sessions", []string{"session_id"},
				tc("session_id", "bigint"),
				tc("tenant_id", "integer"),
				tc("user_id", "integer"),
				tc("started_at", "timestamp with time zone"),
				tc("origin", "inet"),
				tc("adapter", "macaddr"),
			),
			tt("public", "tenant_users", []string{"tenant_id", "user_id"},
				tc("tenant_id", "integer"),
				tc("user_id", "integer"),
				tc("owner_person_id", "bigint"),
				tc("email", "text"),
				tc("joined_on", "date"),
			),
		},
		FKs: []pipeline.ForeignKey{
			fk("invoices_person_id_fkey", tInvoices, []string{"person_id"}, tPeople, []string{"person_id"}),
			fk("LegacyCustomer_MigratedFromPersonID_fkey", tLegacy, []string{"MigratedFromPersonID"}, tPeople, []string{"person_id"}),
			fk("attachments_uploaded_by_person_id_fkey", tAttach, []string{"uploaded_by_person_id"}, tPeople, []string{"person_id"}),
			fk("audit_log_person_id_fkey", tAudit, []string{"person_id"}, tPeople, []string{"person_id"}),
			fk("click_stream_person_id_fkey", tClicks, []string{"person_id"}, tPeople, []string{"person_id"}),
			fk("device_readings_device_id_fkey", tReadings, []string{"device_id"}, tDevices, []string{"device_id"}),
			fk("devices_site_code_fkey", tDevices, []string{"site_code"}, tSites, []string{"site_code"}),
			fk("events_person_id_fkey", tEvents, []string{"person_id"}, tPeople, []string{"person_id"}),
			fk("order_items_order_id_fkey", tOrderItems, []string{"order_id"}, tOrders, []string{"order_id"}),
			fk("orders_person_id_fkey", tOrders, []string{"person_id"}, tPeople, []string{"person_id"}),
			fk("organisations_founded_by_fkey", tOrgs, []string{"founded_by"}, tPeople, []string{"person_id"}),
			fk("organisations_primary_team_id_fkey", tOrgs, []string{"primary_team_id"}, tTeams, []string{"team_id"}),
			fk("people_manager_id_fkey", tPeople, []string{"manager_id"}, tPeople, []string{"person_id"}),
			fk("people_preferred_order_id_fkey", tPeople, []string{"preferred_order_id"}, tOrders, []string{"order_id"}),
			fk("projects_owner_organisation_id_fkey", tProjects, []string{"owner_organisation_id"}, tOrgs, []string{"organisation_id"}),
			fk("sites_owner_person_id_fkey", tSites, []string{"owner_person_id"}, tPeople, []string{"person_id"}),
			fk("stream_rows_person_id_fkey", tStream, []string{"person_id"}, tPeople, []string{"person_id"}),
			fk("teams_lead_project_id_fkey", tTeams, []string{"lead_project_id"}, tProjects, []string{"project_id"}),
			fk("tenant_user_flags_tenant_user_fkey", tFlags, []string{"tenant_id", "user_id"}, tTenantU, []string{"tenant_id", "user_id"}),
			fk("tenant_user_sessions_tenant_id_user_id_fkey", tSessions, []string{"tenant_id", "user_id"}, tTenantU, []string{"tenant_id", "user_id"}),
			fk("tenant_users_owner_person_id_fkey", tTenantU, []string{"owner_person_id"}, tPeople, []string{"person_id"}),
		},
		Fingerprint: "recorded-nasty-fixture",
	}
}

// mapSampler is pipeline.Sampler over a recorded map. A column absent from the
// map has no samples, which is what introspect reports for an empty table.
type mapSampler map[ref.ColumnRef][]any

func (m mapSampler) Samples(c ref.ColumnRef) []any { return m[c] }

// nastySamples are the rows testdata/nasty.sql inserts, as introspect would
// hand them to the classifier.
func nastySamples() mapSampler {
	s := mapSampler{}
	s[col(tPeople, "person_id")] = anyOf(int64(90000), int64(90007), int64(90014), int64(90021), int64(90028))
	s[col(tPeople, "manager_id")] = anyOf(nil, int64(90000), int64(90000), int64(90007), int64(90007))
	s[col(tPeople, "given_name")] = anyOf("Ada", "Grace", "Alan", "Katherine", "Edsger")
	s[col(tPeople, "family_name")] = anyOf("Lovelace", "Hopper", "Turing", "Johnson", "Dijkstra")
	s[col(tPeople, "display_name")] = anyOf("Ada Lovelace", "Grace Hopper", "Alan Turing", "Katherine Johnson", "Edsger Dijkstra")
	s[col(tPeople, "email_verified")] = anyOf(true, false, true, true, false)
	s[col(tPeople, "ref")] = anyOf(
		"ada.lovelace@fixture.test", "grace.hopper@fixture.test", "alan.turing@fixture.test",
		"katherine.johnson@fixture.test", "edsger.dijkstra@fixture.test")
	s[col(tPeople, "status")] = anyOf("active", "active", "suspended", "pending", "closed")
	s[col(tPeople, "marital_status")] = anyOf("married", "single", "civil_partnership", "widowed", "undisclosed")
	s[col(tPeople, "contact")] = anyOf(
		`{"profile": {"contact": {"email": "ada.lovelace@fixture.test", "phone": "+44 20 7946 0958"}, "locale": "en-GB"}, "tags": ["founder"]}`,
		`{"profile": {"contact": {"email": "grace.hopper@fixture.test", "phone": "+1 415 555 0132"}, "locale": "en-US"}, "tags": ["admin"]}`,
		`{"profile": {"contact": {"email": "alan.turing@fixture.test", "phone": "+44 161 496 0123"}, "locale": "en-GB"}, "tags": []}`,
		`{"profile": {"contact": {"email": "katherine.johnson@fixture.test", "phone": "+1 757 555 0188"}, "locale": "en-US"}, "tags": ["reviewer"]}`,
		`{"profile": {"contact": {"email": "edsger.dijkstra@fixture.test", "phone": "+31 20 555 0177"}, "locale": "nl-NL"}, "tags": ["archived"]}`)
	s[col(tPeople, "alt_emails")] = []any{
		[]any{"ada@corp.invalid", "a.lovelace@corp.invalid"},
		[]any{"ghopper@corp.invalid"},
		[]any{nil, "a.turing@corp.invalid"},
		nil,
		[]any{},
	}
	s[col(tPeople, "notes")] = anyOf(
		"Ada Lovelace asked that Grace Hopper be copied on the renewal. Call back on +44 20 7946 0958.",
		"Grace Hopper prefers email. Escalation contact is Alan Turing.",
		"Suspended pending review. Raised by Katherine Johnson on 2024-03-02.",
		"Katherine Johnson is the reviewer of record for Alan Turing.",
		"Account closed at the request of Edsger Dijkstra.")
	s[col(tPeople, "preferred_order_id")] = anyOf(int64(200000), int64(200006), nil, int64(200009), nil)

	s[col(tTenantU, "email")] = anyOf(
		"ada.lovelace@fixture.test", "grace.hopper@fixture.test",
		"alan.turing@fixture.test", "katherine.johnson@fixture.test")
	s[col(tTenantU, "joined_on")] = anyOf("2024-01-05", "2024-01-06", "2024-02-11", "2024-02-12")

	s[col(tSessions, "origin")] = anyOf("10.20.30.40", "10.20.30.41", "172.16.5.6", "2001:db8::1", "172.16.5.7")
	s[col(tSessions, "adapter")] = anyOf(
		"08:00:2b:01:02:03", "08:00:2b:01:02:04", "08:00:2b:01:02:05", nil, "08:00:2b:01:02:06")

	s[col(tAudit, "entry_uid")] = anyOf("AE-0001", "AE-0002", "AE-0003", "AE-0004")
	s[col(tAudit, "action")] = anyOf("login", "login", "password.reset", "account.review")
	s[col(tAudit, "client_ip")] = anyOf("10.20.30.40", "10.20.30.41", "2001:db8::7", "192.168.9.10")

	s[col(tAttach, "filename")] = anyOf("signature.png", "id-scan.pdf", "spec-v3.pdf", "orphan.txt")
	s[col(tAttach, "uploaded_by")] = anyOf(
		"ada.lovelace@fixture.test", "grace.hopper@fixture.test",
		"alan.turing@fixture.test", "nobody@example.invalid")
	s[col(tAttach, "owner_type")] = anyOf("people", "people", "projects", "people")

	s[col(tEvents, "kind")] = anyOf("order.placed", "order.placed", "order.placed", "account.suspended", "account.reviewed")
	s[col(tEvents, "payload")] = anyOf(
		`{"actor": {"contact": {"email": "ada.lovelace@fixture.test", "phone": "+44 20 7946 0958"}}, "order_id": 200000}`,
		`{"actor": {"contact": {"email": "ada.lovelace@fixture.test", "phone": "+44 20 7946 0958"}}, "order_id": 200003}`,
		`{"actor": {"contact": {"email": "grace.hopper@fixture.test", "phone": "+1 415 555 0132"}}, "order_id": 200006}`,
		`{"actor": {"contact": {"email": "katherine.johnson@fixture.test", "phone": "+1 757 555 0188"}}, "reason": "review"}`,
		`{"actor": {"contact": {"email": "katherine.johnson@fixture.test", "phone": "+1 757 555 0188"}}}`)

	s[col(tLegacy, "EmailAddress")] = anyOf(
		"ada.lovelace@fixture.test", "grace.hopper@fixture.test", "alan.turing@fixture.test")
	s[col(tLegacy, "ContactNumber")] = anyOf("+447700900123", "+14155550132", nil)
	s[col(tLegacy, "MobileNumber")] = anyOf("+44 7700 900123", "+1 415 555 0132", nil)
	s[col(tLegacy, "Notes")] = anyOf(
		"Migrated from the 1998 system. Contact is Ada Lovelace.",
		"Do not merge with Grace Hopper's new record.",
		"Left blank on purpose.")

	s[col(tSites, "site_code")] = anyOf("SITE-LDN", "SITE-NYC")
	s[col(tSites, "name")] = anyOf("London", "New York")
	s[col(tSites, "contact_email")] = anyOf("site.london@fixture.test", "site.newyork@fixture.test")

	s[col(tDevices, "asset_tag")] = anyOf("AT-0001", "AT-0002", "AT-0003")
	s[col(tDevices, "owned_by")] = anyOf("ada.lovelace@fixture.test", "grace.hopper@fixture.test", nil)

	s[col(tClicks, "url")] = anyOf(
		"https://example.com/pricing", "https://example.com/pricing", "https://example.com/docs")

	s[col(tInvoices, "bill_to_email")] = anyOf(
		"accounts@fixture.test", "grace.hopper@fixture.test", "katherine.johnson@fixture.test")
	s[col(tInvoices, "bill_to_phone")] = anyOf("+44 20 7946 0958", "+1 415 555 0132", nil)

	s[col(tOrgs, "name")] = anyOf("Analytical Engines Ltd", "Compiler Works")
	s[col(tTeams, "name")] = anyOf("Punch Cards", "Runtime")
	s[col(tProjects, "name")] = anyOf("Difference Engine", "A-0 System")
	s[col(tOrderItems, "sku")] = anyOf("SKU-0001", "SKU-0002", "SKU-0003", "SKU-0004", "SKU-0005")
	s[col(tFlags, "flag")] = anyOf("beta", "beta", "unassigned")
	return s
}

func anyOf(v ...any) []any { return v }

// classifyNasty runs the classifier over the recorded fixture.
func classifyNasty(prior *pipeline.Config) (*pipeline.Classification, error) {
	return New().Classify(nastySchema(), nastySamples(), prior)
}
