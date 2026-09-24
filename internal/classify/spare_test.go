// SPDX-License-Identifier: Apache-2.0

package classify

import (
	"strings"
	"testing"

	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
)

// TestSweepSparesEnumIdentifierAndUniqueColumns is T-0311's pin, the shape
// dogfood session 1 found on a production Rails schema: a users table and a
// devices table, each with a certain email column, whose signal-less
// character columns were every one swept into free_text by
// unknownColumnsBesideCertain, so the copy held word salad where the
// application expects 'admin' or 'linux'.
//
// Each spared column must be copied and its line must say why; each control
// beside it must still be swept, because it is the shape the sweep exists for
// (THREAT_MODEL.md T1): a repeated native-script name, an ASCII handle seen
// once, a colour enum carrying a dictionary name, a gender enum, a path
// naming a person, and the review round's controls: dotted and month-name
// dates of birth, a bare home directory, blood groups, marital status,
// postcodes, ISO dates and CamelCase handles, and T-0354's 15-character hex tokens.
func TestSweepSparesEnumIdentifierAndUniqueColumns(t *testing.T) {
	t.Parallel()
	users := ref.TableRef{Schema: "public", Name: "users"}
	devices := ref.TableRef{Schema: "public", Name: "devices"}

	usersTable := tt("public", "users", []string{"id"},
		tc("id", "bigint"),
		tc("email", "text"),
		tc("role", "character varying(255)"),
		tc("state", "character varying(255)"),
		tc("uuid", "character varying(255)"),
		tc("ui_mode", "text"),
		tc("external_ref", "text"),
		tc("tag_a", "text"),
		tc("tag_b", "text"),
		tc("g", "text"),
	)
	usersTable.Indexes = []pipeline.Index{{
		Name: "users_external_ref_key", Columns: []string{"external_ref"}, Unique: true, Immediate: true,
	}}
	devicesTable := tt("public", "devices", []string{"id"},
		tc("id", "bigint"),
		tc("user_id", "bigint"),
		tc("owner_email", "text"),
		tc("os_type", "text"),
		tc("log_level", "text"),
		tc("timezone", "text"),
		tc("app_version", "text"),
		tc("hostname", "text"),
		tc("serial", "text"),
		tc("asset_path", "text"),
		tc("colour", "text"),
		tc("home_dir", "text"),
		tc("born_dotted", "text"),
		tc("born_slashed", "text"),
		tc("dir_b", "text"),
		tc("grp", "text"),
		tc("status2", "text"),
		tc("zone", "text"),
		tc("day", "text"),
		tc("tag_c", "text"),
		tc("code_b", "text"),
		tc("code_c", "text"),
		tc("ext_b", "text"),
	)
	schema := &pipeline.Schema{
		Tables: []pipeline.Table{usersTable, devicesTable},
		FKs: []pipeline.ForeignKey{
			fk("devices_user_id_fkey", devices, []string{"user_id"}, users, []string{"id"}),
		},
	}

	// twelve builds a column of twelve samples from (value, times) pairs.
	twelve := func(pairs ...any) []any {
		var out []any
		for i := 0; i < len(pairs); i += 2 {
			for n := 0; n < pairs[i+1].(int); n++ {
				out = append(out, pairs[i])
			}
		}
		if len(out) != 12 {
			t.Fatalf("fixture column has %d samples, want 12", len(out))
		}
		return out
	}
	each := func(format string, vals ...string) []any {
		out := make([]any, 0, len(vals))
		for _, v := range vals {
			out = append(out, strings.ReplaceAll(format, "%", v))
		}
		return out
	}
	digits := []string{"01", "02", "03", "04", "05", "06", "07", "08", "09", "10", "11", "12"}
	letters := []string{"alpha", "bravo", "charlie", "delta", "echo", "foxtrot", "golf", "hotel", "india", "juliet", "kilo", "lima"}

	s := mapSampler{
		col(users, "email"):   each("user%@realcorp.example", digits...),
		col(users, "role"):    twelve("admin", 3, "user", 7, "guest", 2),
		col(users, "state"):   twelve("active", 6, "pending", 3, "suspended", 3),
		col(users, "uuid"):    each("0b6f3e2a-9c4d-4e1f-8a7b-5d2c1e0f00%", digits...),
		col(users, "ui_mode"): twelve("light", 5, "dark", 5, "system", 2),
		// Under a unique index: spared, and the old silent skip now says so.
		col(users, "external_ref"): each("ref-%", letters...),
		// Controls, all still swept. A native-script name, each seen twice,
		// is an enumeration by count and not by token shape.
		col(users, "tag_a"): twelve("ኣበበ", 2, "ተስፋዬ", 2, "ገብረ", 2, "ኪዳነ", 2, "ሰላም", 2, "ማርታ", 2),
		// An ASCII token seen once each is not an enumeration.
		col(users, "tag_b"): each("zq%x", digits...),
		// A gender enumeration is personal data under any column name.
		col(users, "g"): twelve("male", 6, "female", 6),

		col(devices, "owner_email"): each("owner%@realcorp.example", digits...),
		col(devices, "os_type"):     twelve("linux", 4, "windows", 4, "macos", 2, "android", 2),
		col(devices, "log_level"):   twelve("debug", 3, "info", 5, "warn", 2, "error", 2),
		col(devices, "timezone"):    twelve("Europe/London", 6, "America/New_York", 4, "Europe/Berlin", 2),
		col(devices, "app_version"): each("2.%.0", "1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "11", "12"),
		col(devices, "hostname"):    each("api.%.prod.internal", letters...),
		// SHA-256 hex, a digest's own length: spared (T-0354). A 40-character
		// SHA-1 column would pin the same rule, but net.ParseMAC reads 40
		// bare hex characters as a hardware address, so the MAC validator
		// masks it before the sweep is asked; textsig's own test holds the
		// 40-character case.
		col(devices, "serial"):     each("3f9a2c7e1b4d8a60c5e79d02b7e4a1c86f35e0d41d8cd98f00b204e9800998%", digits...),
		col(devices, "asset_path"): each("/assets/devices/%.png", letters...),
		// A colour enumeration with a dictionary surname in it (green) under
		// the weak ratio, so nothing else decides it: the guard is what
		// keeps it swept.
		col(devices, "colour"): twelve("red", 4, "blue", 4, "green", 2, "orange", 2),
		// A path is an identifier shape until a segment names a person.
		col(devices, "home_dir"): each("/home/grace/%", letters...),
		// The T-0311 review's controls, one per guard, each swept before
		// T-0311 and each spared by its first landing. Unpadded dotted dates
		// of birth are semantic versions by grammar alone.
		col(devices, "born_dotted"): anyOf("5.3.1985", "12.11.1979", "7.8.1990", "1.1.1970", "3.4.1962",
			"9.10.1988", "2.6.1975", "11.12.1983", "4.9.1991", "6.1.1968"),
		// Slashed dates with a month name have a letter in a segment.
		col(devices, "born_slashed"): anyOf("05/Mar/1985", "12/Nov/1979", "07/Aug/1990", "01/Jan/1970",
			"03/Apr/1962", "09/Oct/1988", "02/Jun/1975", "11/Dec/1983", "04/Sep/1991", "06/Jan/1968"),
		// A login is not a dictionary word; a bare home directory is not
		// an application path.
		col(devices, "dir_b"): each("/home/zq%x", digits...),
		// Blood groups, a special category, and marital status.
		col(devices, "grp"):     twelve("A+", 4, "O-", 4, "B+", 2, "AB-", 2),
		col(devices, "status2"): twelve("married", 6, "divorced", 4, "widowed", 2),
		// Postcodes, digit-only, repeated because the sample is pages.
		col(devices, "zone"): twelve("94105", 4, "10001", 4, "60601", 4),
		// ISO dates, repeated.
		col(devices, "day"): twelve("1985-03-05", 4, "1979-11-12", 4, "1990-08-07", 4),
		// A CamelCase handle is one unknown word until it is split.
		col(devices, "tag_c"): twelve("JohnSmith", 6, "MaryJones", 6),
		// The review's second round: digit runs joined by a hyphen are no
		// date and no bare digit run. These ZIP+4 codes parse as a phone
		// number under no guessed region, so the sweep's guard is what
		// decides them.
		col(devices, "code_b"): twelve("00501-0001", 6, "00544-1234", 6),
		// A ZIP+4 and a local phone number that do parse as phone numbers:
		// the guessed-region phone signal masks them first, and whichever
		// decides, they must not be copied.
		// The T-0311 review's hex probe (T-0354): 15-character hex tokens,
		// each seen once, are no digest's length, so they are an API key,
		// a reset token or an invite code as far as the samples say, and
		// the sweep masks them as it did before T-0311.
		col(devices, "ext_b"):  each("3f9a2c7e1b4d8%", digits...),
		col(devices, "code_c"): twelve("94105-1234", 3, "10001-0001", 3, "555-1234", 3, "555-9876", 3),
	}

	cls, err := New().Classify(schema, s, nil)
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}

	for _, email := range []ref.ColumnRef{col(users, "email"), col(devices, "owner_email")} {
		if d := decision(t, cls, email); d.Confidence != pipeline.ConfCertain || !d.Masked {
			t.Fatalf("%s is %s/%d masked=%v, want a certain masked email: the fixture needs a certain neighbour",
				email.Column, d.Category, int(d.Confidence), d.Masked)
		}
	}

	spared := []struct {
		c    ref.ColumnRef
		want string
	}{
		{col(users, "role"), "enum-like, 3 distinct values in 12 samples"},
		{col(users, "state"), "enum-like, 3 distinct values in 12 samples"},
		{col(users, "ui_mode"), "enum-like, 3 distinct values in 12 samples"},
		{col(users, "uuid"), "all 12 samples are uuids"},
		{col(users, "external_ref"), "under a unique index"},
		{col(devices, "os_type"), "enum-like, 4 distinct values in 12 samples"},
		{col(devices, "log_level"), "enum-like, 4 distinct values in 12 samples"},
		{col(devices, "timezone"), "all 12 samples are paths"},
		{col(devices, "app_version"), "all 12 samples are semantic versions"},
		{col(devices, "hostname"), "all 12 samples are hostnames"},
		{col(devices, "serial"), "all 12 samples are hex digests"},
		{col(devices, "asset_path"), "all 12 samples are paths"},
	}
	for _, sp := range spared {
		d := decision(t, cls, sp.c)
		if d.Masked || d.Category != pipeline.CatNone {
			t.Errorf("%s.%s is %s/%d masked=%v, want spared from the sweep and copied: %s",
				sp.c.Table.Name, sp.c.Column, d.Category, int(d.Confidence), d.Masked, d.Reason)
			continue
		}
		if !strings.Contains(d.Reason, "not swept by the neighbouring-column rule") || !strings.Contains(d.Reason, sp.want) {
			t.Errorf("%s.%s is copied but its line does not say why (want %q): %s",
				sp.c.Table.Name, sp.c.Column, sp.want, d.Reason)
		}
		if bad, ok := ParseReason(d.Reason); !ok {
			t.Errorf("%s.%s reason does not parse at %q", sp.c.Table.Name, sp.c.Column, bad)
		}
	}

	for _, c := range []ref.ColumnRef{
		col(users, "tag_a"), col(users, "tag_b"), col(users, "g"),
		col(devices, "colour"), col(devices, "home_dir"),
		col(devices, "born_dotted"), col(devices, "born_slashed"), col(devices, "dir_b"),
		col(devices, "grp"), col(devices, "status2"), col(devices, "zone"), col(devices, "day"),
		col(devices, "tag_c"), col(devices, "code_b"), col(devices, "ext_b"),
	} {
		d := decision(t, cls, c)
		if !d.Masked {
			t.Errorf("%s.%s is %s/%d and copied, want it still swept (THREAT_MODEL.md T1): %s",
				c.Table.Name, c.Column, d.Category, int(d.Confidence), d.Reason)
		}
		if strings.Contains(d.Reason, "not swept") {
			t.Errorf("%s.%s is masked but its line claims it was spared: %s", c.Table.Name, c.Column, d.Reason)
		}
		// Masked by the sweep itself, not by some other signal the fixture
		// happened to trip: otherwise the control proves nothing about
		// the guards.
		if !strings.Contains(d.Reason, "nothing is known about this column's contents") {
			t.Errorf("%s.%s is masked, but not by the sweep this control is for: %s",
				c.Table.Name, c.Column, d.Reason)
		}
	}
	if d := decision(t, cls, col(devices, "code_c")); !d.Masked || strings.Contains(d.Reason, "not swept") {
		t.Errorf("devices.code_c (ZIP+4 and local phone numbers) is %s/%d masked=%v, want masked: %s",
			d.Category, int(d.Confidence), d.Masked, d.Reason)
	}
}

// TestSweepSparesNothingBelowTheEnumFloor holds the sample floors: three
// repeated values in six samples are not an enumeration (enumMinSamples),
// and two uuids are two rows rather than a column (minSamples), so both are
// still swept. A tiny table is where "when in doubt, mask it" decides.
func TestSweepSparesNothingBelowTheEnumFloor(t *testing.T) {
	t.Parallel()
	tbl := ref.TableRef{Schema: "public", Name: "accounts"}
	schema := &pipeline.Schema{Tables: []pipeline.Table{
		tt("public", "accounts", []string{"id"},
			tc("id", "bigint"),
			tc("email", "text"),
			tc("role", "text"),
			tc("token_ref", "text"),
		),
	}}
	s := mapSampler{
		col(tbl, "email"):     anyOf("a@realcorp.example", "b@realcorp.example", "c@realcorp.example", "d@realcorp.example", "e@realcorp.example", "f@realcorp.example"),
		col(tbl, "role"):      anyOf("admin", "admin", "user", "user", "guest", "guest"),
		col(tbl, "token_ref"): anyOf("0b6f3e2a-9c4d-4e1f-8a7b-5d2c1e0f0001", nil, nil, nil, nil, "0b6f3e2a-9c4d-4e1f-8a7b-5d2c1e0f0002"),
	}
	cls, err := New().Classify(schema, s, nil)
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	for _, name := range []string{"role", "token_ref"} {
		d := decision(t, cls, col(tbl, name))
		if !d.Masked {
			t.Errorf("accounts.%s is copied below the sample floor, want it still swept: %s", name, d.Reason)
		}
	}
}
