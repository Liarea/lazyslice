// SPDX-License-Identifier: Apache-2.0

package classify

import (
	"testing"

	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
)

// The 2026-09-15 red team's classifier findings, one test per attack. Every
// one of these schemas reached the target with personal data in cleartext
// under exit 0, and in each case the report affirmatively said the column was
// clean.

// A1: PII column names in other languages and in abbreviations. The `email`
// column is the control the attack included: it was masked, and everything
// beside it was not.
func TestRedTeamA1NonEnglishColumnNames(t *testing.T) {
	t.Parallel()
	tbl := ref.TableRef{Schema: "public", Name: "klienci"}
	schema := &pipeline.Schema{Tables: []pipeline.Table{
		tt("public", "klienci", []string{"id"},
			tc("id", "bigint"),
			tc("email", "text"),
			tc("nazwisko", "text"),
			tc("imie", "text"),
			tc("pesel", "text"),
			tc("komorka", "text"),
			tc("mob", "text"),
			tc("fname", "text"),
			tc("lname", "text"),
			tc("cognome", "text"),
			tc("achternaam", "text"),
			tc("wohnort", "text"),
		),
	}}
	// Five fictional people, as elsewhere in this package.
	s := mapSampler{
		col(tbl, "email"):      anyOf("a@fixture.test", "b@fixture.test", "c@fixture.test"),
		col(tbl, "nazwisko"):   anyOf("Kowalski", "Szczepański", "Nowak"),
		col(tbl, "imie"):       anyOf("Bogusław", "Katarzyna", "Wojciech"),
		col(tbl, "pesel"):      anyOf("44051401359", "80010112345", "92120554321"),
		col(tbl, "komorka"):    anyOf("601 234 567", "602 345 678", "603 456 789"),
		col(tbl, "mob"):        anyOf("601 234 567", "602 345 678", "603 456 789"),
		col(tbl, "fname"):      anyOf("Giulia", "Lorenzo", "Chiara"),
		col(tbl, "lname"):      anyOf("Rossi", "Esposito", "Bianchi"),
		col(tbl, "cognome"):    anyOf("Ferrari", "Romano", "Greco"),
		col(tbl, "achternaam"): anyOf("Jansen", "Bakker", "Visser"),
		col(tbl, "wohnort"):    anyOf("Ulica Kwiatowa 12", "Via Roma 4", "Kerkstraat 9"),
	}
	cls, err := New().Classify(schema, s, nil)
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	for _, name := range []string{
		"email", "nazwisko", "imie", "pesel", "komorka", "mob",
		"fname", "lname", "cognome", "achternaam", "wohnort",
	} {
		d := cls.Decisions[col(tbl, name)]
		if !d.Masked {
			t.Errorf("klienci.%s is %s/%d and not masked: %s",
				name, d.Category, int(d.Confidence), d.Reason)
		}
	}
}

// A2b: the value-level dodge in a table where no column ever reaches `likely`,
// so the neighbouring-column rule cannot rescue anything. The names are
// non-English and the addresses are obfuscated, which is exactly the pair the
// multilingual dictionary and textsig.Candidates close.
func TestRedTeamA2bNoColumnNameSignalAtAll(t *testing.T) {
	t.Parallel()
	tbl := ref.TableRef{Schema: "public", Name: "entries"}
	schema := &pipeline.Schema{Tables: []pipeline.Table{
		tt("public", "entries", []string{"id"},
			tc("id", "bigint"),
			tc("label", "text"),
			tc("detail", "text"),
		),
	}}
	s := mapSampler{
		col(tbl, "label"): anyOf(
			"Nkechi Okonkwo", "Bogusław Szczepański", "Þórunn Jónsdóttir",
			"Wanjiru Kamau", "Ayşe Yıldırım"),
		col(tbl, "detail"): anyOf(
			"nkechi.okonkwo AT realcorp DOT example",
			"boguslaw.szczepanski AT realcorp DOT example",
			"thorunn.jonsdottir AT realcorp DOT example",
			"wanjiru.kamau AT realcorp DOT example",
			"ayse.yildirim AT realcorp DOT example"),
	}
	cls, err := New().Classify(schema, s, nil)
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	for _, name := range []string{"label", "detail"} {
		d := cls.Decisions[col(tbl, name)]
		if !d.Masked {
			t.Errorf("entries.%s is %s/%d and not masked: %s",
				name, d.Category, int(d.Confidence), d.Reason)
		}
	}
	if got := cls.Decisions[col(tbl, "detail")].Category; got != pipeline.CatEmail {
		t.Errorf("entries.detail is %s, want email: an obfuscated address is an address", got)
	}
}

// A4a: PII in a column the classifier marks safe by type. bestSignal returned
// before any validator ran for a bytea column, on the argument that a PNG
// reads as an address — true, and it does not cover a bytea holding printable
// UTF-8. The table has no `certain` column, so byteaInPersonShapedTable cannot
// fire either.
func TestRedTeamA4aPrintableBytea(t *testing.T) {
	t.Parallel()
	tbl := ref.TableRef{Schema: "public", Name: "assets"}
	schema := &pipeline.Schema{Tables: []pipeline.Table{
		tt("public", "assets", []string{"id"},
			tc("id", "bigint"),
			tc("blob_doc", "bytea"),
			tc("kind", "text"),
		),
	}}
	s := mapSampler{
		col(tbl, "blob_doc"): anyOf(
			// The red team's own payload, RFC 5322 name-addr and all.
			[]byte("Grace Hopper <grace.hopper1@realcorp.example> 078-05-1120"),
			[]byte("Katherine Johnson <katherine.johnson@realcorp.example> 078-05-1121"),
			[]byte("Ada Lovelace <ada.lovelace@realcorp.example> 078-05-1122"),
		),
		col(tbl, "kind"): anyOf("a", "b", "c"),
	}
	cls, err := New().Classify(schema, s, nil)
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	d := cls.Decisions[col(tbl, "blob_doc")]
	if !d.Masked {
		t.Fatalf("assets.blob_doc is %s/%d and not masked: %s", d.Category, int(d.Confidence), d.Reason)
	}
	if d.Category != pipeline.CatBinary {
		t.Errorf("assets.blob_doc is %s, want binary_personal: the bytea rule is the one that has a masker here", d.Category)
	}
}

// The other half of A4a, which is the reason the guard is on content and not
// on the family: a bytea holding actual binary must be unaffected. A PNG
// header renders as a string with digits and words in it, which addressShape
// reads as an address, and masking it under a text category would be the
// T-0054 failure the family guard was written for.
func TestRedTeamA4aBinaryByteaIsUntouched(t *testing.T) {
	t.Parallel()
	tbl := ref.TableRef{Schema: "public", Name: "files"}
	schema := &pipeline.Schema{Tables: []pipeline.Table{
		tt("public", "files", []string{"id"},
			tc("id", "bigint"),
			tc("body", "bytea"),
		),
	}}
	png := []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d, 'I', 'H', 'D', 'R', 0xff, 0xfe}
	s := mapSampler{col(tbl, "body"): anyOf(png, png, png)}
	cls, err := New().Classify(schema, s, nil)
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	if d := cls.Decisions[col(tbl, "body")]; d.Masked {
		t.Errorf("files.body is %s/%d and masked: a PNG is not personal data by shape", d.Category, int(d.Confidence))
	}
}

// The dead end between the two halves of A4a's fix (the T-REDFIX review's
// second finding): a bytea column that is half printable documents and half
// images. byteaTextSignal used to require 95% of the sample set to be readable
// before any validator ran, where internal/verify's second net asks
// textsig.PrintableText per value and has no column ratio — so this column was
// left unmasked here and refused at exit 9 there, with no green path, because
// binary_personal cannot be reached for it by any other route. The gate is per
// value on both sides now, so the column is masked and the run can be green.
func TestRedTeamA4aHalfPrintableByteaIsMasked(t *testing.T) {
	t.Parallel()
	tbl := ref.TableRef{Schema: "public", Name: "mixed"}
	schema := &pipeline.Schema{Tables: []pipeline.Table{
		tt("public", "mixed", []string{"id"},
			tc("id", "bigint"),
			tc("body", "bytea"),
		),
	}}
	png := []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d, 'I', 'H', 'D', 'R', 0xff, 0xfe}
	s := mapSampler{col(tbl, "body"): anyOf(
		[]byte("Grace Hopper <grace.hopper1@realcorp.example>"),
		png,
		[]byte("Ada Lovelace <ada.lovelace@realcorp.example>"),
		png,
	)}
	cls, err := New().Classify(schema, s, nil)
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	d := cls.Decisions[col(tbl, "body")]
	if !d.Masked || d.Category != pipeline.CatBinary {
		t.Fatalf("mixed.body is %s/%d masked=%v, want binary_personal and masked: "+
			"internal/verify refuses this column at exit 9 and only this side can give it a green path",
			d.Category, int(d.Confidence), d.Masked)
	}
}

// A2b's structural half: the neighbouring-column rule used to raise only `low`
// to `possible`, so a character column with no signal at all sat at `none`
// beside a `certain` personal column and was copied verbatim. CLAUDE.md's
// "when in doubt, mask it" is the direction this fails in.
func TestRedTeamNeighbourRaisesASilentColumn(t *testing.T) {
	t.Parallel()
	tbl := ref.TableRef{Schema: "public", Name: "patients"}
	schema := &pipeline.Schema{Tables: []pipeline.Table{
		tt("public", "patients", []string{"id"},
			tc("id", "bigint"),
			tc("email", "text"),
			tc("opaque", "text"),
			tc("created_at", "timestamp with time zone"),
			tc("count", "integer"),
		),
	}}
	s := mapSampler{
		col(tbl, "email"):  anyOf("a@fixture.test", "b@fixture.test", "c@fixture.test"),
		col(tbl, "opaque"): anyOf("zz-1", "zz-2", "zz-3"),
	}
	cls, err := New().Classify(schema, s, nil)
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	if d := cls.Decisions[col(tbl, "opaque")]; !d.Masked {
		t.Errorf("patients.opaque is %s/%d and not masked beside a certain email column: %s",
			d.Category, int(d.Confidence), d.Reason)
	}
	// The raise is for free text and nothing else: a key, a generated column,
	// a timestamp and an integer are untouched, because the rule's argument is
	// "we do not know what this character column holds" and none of those is a
	// character column.
	for _, name := range []string{"id", "created_at", "count"} {
		if d := cls.Decisions[col(tbl, name)]; d.Masked {
			t.Errorf("patients.%s is masked by the neighbour rule; it reaches only character columns", name)
		}
	}
}

// The 2026-09-15 round-4 red team's native-script variant against A2b's own
// fix (T-0239): the same Amharic names as TestRedTeamA2bNoColumnNameSignalAtAll,
// in a column declared varchar(12), in a table that DOES hold a `certain`
// email neighbour -- the exact shape unknownColumnsBesideCertain exists for,
// defeated by minUnknownLen's old floor of sixteen. A given name, a surname,
// a postcode, a national ID and a phone number all fit in twelve characters,
// so a short declared length was never evidence the column is impersonal; the
// control column (the same schema with the column declared `text`) is what
// tells "the length exclusion let it through" apart from "the rail does not
// fire here at all".
func TestRedTeamRound4A2bShortDeclaredLengthBesideCertain(t *testing.T) {
	t.Parallel()
	tbl := ref.TableRef{Schema: "public", Name: "members"}
	names := []string{"ኣበበ ኪዳነ", "ተስፋዬ ኪዳነ", "ገብረ ኣበበ", "ኪዳነ ተስፋዬ", "ኣበበ ገብረ"}
	newSchema := func(typeName string, typMod int32) *pipeline.Schema {
		return &pipeline.Schema{Tables: []pipeline.Table{
			tt("public", "members", []string{"id"},
				tc("id", "bigint"),
				pipeline.Column{
					Name: "ስም", TypeName: typeName, TypMod: typMod,
					Nullable: true, Fingerprint: fp("ስም", typeName),
				},
				tc("email", "text"),
			),
		}}
	}
	s := mapSampler{
		col(tbl, "ስም"):    anyOf(names[0], names[1], names[2], names[3], names[4]),
		col(tbl, "email"): anyOf("user1@realcorp.example", "user2@realcorp.example", "user3@realcorp.example"),
	}

	// The leaking shape: varchar(12), TypMod = declared length (12) + 4.
	cls, err := New().Classify(newSchema("character varying(12)", 16), s, nil)
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	if d := cls.Decisions[col(tbl, "ስም")]; !d.Masked {
		t.Errorf("members.ስም is %s/%d and not masked beside a certain email column: %s",
			d.Category, int(d.Confidence), d.Reason)
	}

	// The control from the attack's own reproduction: the identical schema and
	// samples with no declared length at all (TypMod 0, i.e. plain `text`) were
	// already masked before this fix; asserting it here too pins that the
	// varchar(12) run above is not masked for some unrelated reason.
	cls, err = New().Classify(newSchema("text", 0), s, nil)
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	if d := cls.Decisions[col(tbl, "ስም")]; !d.Masked {
		t.Errorf("members.ስም (declared text) is %s/%d and not masked beside a certain email column: %s",
			d.Category, int(d.Confidence), d.Reason)
	}
}

// The T-0239 fix-round review's own finding: unknownColumnsBesideCertain's
// declared-length floor (lowered by the test above) newly reached a
// validated foreign key's character-family column, at either end, and this
// package has no pass that reconciles a decision this rail alone made back
// across a join (indexFKColumns's own comment, and unknownColumnsBesideCertain's
// FK exclusion in raisableUnknown). testdata/regressions/031 pins the shape
// under the Docker-gated make torture; this is the same schema at the unit
// level so a future edit to indexFKColumns, raisableUnknown or a foreign
// key's Validated/Virtual handling cannot silently reintroduce the
// half-loaded-target shape with every committed, non-Docker check still
// green.
func TestRedTeamRound4A2bValidatedFKColumnsExcludedBothEnds(t *testing.T) {
	t.Parallel()
	currencies := ref.TableRef{Schema: "public", Name: "currencies"}
	members := ref.TableRef{Schema: "public", Name: "members"}
	newSchema := func(withFK bool) *pipeline.Schema {
		schema := &pipeline.Schema{Tables: []pipeline.Table{
			tt("public", "currencies", []string{"code"},
				pipeline.Column{
					Name: "code", TypeName: "character varying(3)", TypMod: 7,
					Nullable: false, Fingerprint: fp("code", "character varying(3)"),
				},
			),
			tt("public", "members", []string{"id"},
				tc("id", "bigint"),
				tc("email", "text"),
				pipeline.Column{
					Name: "currency", TypeName: "character varying(3)", TypMod: 7,
					Nullable: false, Fingerprint: fp("currency", "character varying(3)"),
				},
			),
		}}
		if withFK {
			schema.FKs = []pipeline.ForeignKey{
				fk("members_currency_fkey", members, []string{"currency"}, currencies, []string{"code"}),
			}
		}
		return schema
	}
	s := mapSampler{
		col(currencies, "code"): anyOf("USD", "EUR", "GBP", "JPY"),
		col(members, "email"): anyOf("member01@realcorp.example", "member02@realcorp.example",
			"member03@realcorp.example", "member04@realcorp.example"),
		col(members, "currency"): anyOf("USD", "EUR", "GBP", "JPY"),
	}

	// The FK shape (regression 031): both ends of a validated, non-virtual
	// foreign key stay unmasked, so the same values are not copied verbatim
	// on one side and replaced with free_text on the other.
	cls, err := New().Classify(newSchema(true), s, nil)
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	if d := cls.Decisions[col(currencies, "code")]; d.Masked {
		t.Errorf("currencies.code is %s/%d and masked despite being the parent end of a validated foreign key: %s",
			d.Category, int(d.Confidence), d.Reason)
	}
	if d := cls.Decisions[col(members, "currency")]; d.Masked {
		t.Errorf("members.currency is %s/%d and masked despite being the child end of a validated foreign key: %s",
			d.Category, int(d.Confidence), d.Reason)
	}

	// The control: the identical members.currency column with no FK edge at
	// all is still raised by unknownColumnsBesideCertain -- this pins the FK
	// exclusion above as the reason members.currency stays unmasked, rather
	// than an unrelated one (the column's shape not reaching the rail at
	// all).
	cls, err = New().Classify(newSchema(false), s, nil)
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	if d := cls.Decisions[col(members, "currency")]; !d.Masked {
		t.Errorf("members.currency (no FK) is %s/%d and not masked beside a certain email column: %s",
			d.Category, int(d.Confidence), d.Reason)
	}
}
