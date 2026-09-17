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

// The T-0239 fix-round review's own finding, superseded by T-0253: an
// earlier version of this test pinned a blanket exclusion that left a
// validated foreign key's character-family columns unmasked on both ends
// (indexFKColumns, raisableUnknown). The round-5 red team's own FK variant
// (docs/reviews/2026-09-15-redteam/round5-still-leaking.json) is the
// disproof of the claim that exclusion shipped with — a genuinely personal
// FK-linked column is reached by every other pass — and TestRedTeamRound5FK
// NativeScriptChildBesideCertainColumn below is that disproof at the unit
// level. What this test now pins is fkPairs' own safe direction for the
// shape the exclusion really existed for: both ends of a validated foreign
// key are raised together, under the same category, so the join stays in
// agreement, rather than copied on both ends as before. testdata/
// regressions/031 pins the same schema under the Docker-gated make torture.
func TestRedTeamRound4A2bValidatedFKColumnsRaisedTogether(t *testing.T) {
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

	// The FK shape (regression 031, current header): both ends of a
	// validated, non-virtual foreign key are masked together, under the same
	// category, so the same values are not copied verbatim on one side and
	// replaced with free_text on the other.
	cls, err := New().Classify(newSchema(true), s, nil)
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	parent := cls.Decisions[col(currencies, "code")]
	child := cls.Decisions[col(members, "currency")]
	if !parent.Masked || parent.Category != pipeline.CatFreeText {
		t.Errorf("currencies.code is %s/%d masked=%v, want free_text masked: %s",
			parent.Category, int(parent.Confidence), parent.Masked, parent.Reason)
	}
	if !child.Masked || child.Category != pipeline.CatFreeText {
		t.Errorf("members.currency is %s/%d masked=%v, want free_text masked: %s",
			child.Category, int(child.Confidence), child.Masked, child.Reason)
	}
	if parent.Category != child.Category {
		t.Errorf("currencies.code (%s) and members.currency (%s) disagree: the load would VALIDATE a foreign key between two different masked categories",
			parent.Category, child.Category)
	}

	// The control: the identical members.currency column with no FK edge at
	// all is still raised by unknownColumnsBesideCertain on its own, and
	// currencies.code (now a plain column, referenced by nothing) is not --
	// this pins fkPairs, not an unrelated reason, as what pulls the parent in
	// above.
	cls, err = New().Classify(newSchema(false), s, nil)
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	if d := cls.Decisions[col(members, "currency")]; !d.Masked {
		t.Errorf("members.currency (no FK) is %s/%d and not masked beside a certain email column: %s",
			d.Category, int(d.Confidence), d.Reason)
	}
	if d := cls.Decisions[col(currencies, "code")]; d.Masked {
		t.Errorf("currencies.code (no FK, no certain column of its own) is %s/%d and masked: %s",
			d.Category, int(d.Confidence), d.Reason)
	}
}

// The 2026-09-17 round-5 red team's classifier attacker, FK variant
// (docs/reviews/2026-09-15-redteam/round5-still-leaking.json): the same
// native-script names as TestRedTeamRound4A2bShortDeclaredLengthBesideCertain
// above, made a validated foreign key child of a lookup table holding the
// same names, whose own primary key gives it no certain column for any other
// pass to key on. The T-0239 fix-round review's blanket FK exclusion took
// both ends out of unknownColumnsBesideCertain's reach entirely, and this is
// the disproof its own comment names: nothing else in this package reaches
// either column, so both crossed verbatim under exit 0. fkPairs now raises
// both together instead.
func TestRedTeamRound5FKNativeScriptChildBesideCertainColumn(t *testing.T) {
	t.Parallel()
	nameDim := ref.TableRef{Schema: "public", Name: "name_dim"}
	members := ref.TableRef{Schema: "public", Name: "members"}
	names := []string{"ኣበበ ኪዳነ", "ተስፋዬ ኪዳነ", "ገብረ ኣበበ", "ኪዳነ ተስፋዬ", "ኣበበ ገብረ"}
	nameCol := func(n string) pipeline.Column {
		return pipeline.Column{
			Name: n, TypeName: "character varying(12)", TypMod: 16,
			Nullable: false, Fingerprint: fp(n, "character varying(12)"),
		}
	}
	schema := &pipeline.Schema{
		Tables: []pipeline.Table{
			tt("public", "name_dim", []string{"ስም"}, nameCol("ስም")),
			tt("public", "members", []string{"id"},
				tc("id", "bigint"),
				tc("email", "text"),
				nameCol("ስም"),
			),
		},
		FKs: []pipeline.ForeignKey{
			fk("members_ስም_fkey", members, []string{"ስም"}, nameDim, []string{"ስም"}),
		},
	}
	s := mapSampler{
		col(nameDim, "ስም"): anyOf(names[0], names[1], names[2], names[3], names[4]),
		col(members, "email"): anyOf("member01@realcorp.example", "member02@realcorp.example",
			"member03@realcorp.example", "member04@realcorp.example", "member05@realcorp.example"),
		col(members, "ስም"): anyOf(names[0], names[1], names[2], names[3], names[4]),
	}
	cls, err := New().Classify(schema, s, nil)
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	parent := cls.Decisions[col(nameDim, "ስም")]
	child := cls.Decisions[col(members, "ስም")]
	if !parent.Masked || parent.Category != pipeline.CatFreeText {
		t.Errorf("name_dim.ስም is %s/%d masked=%v, want free_text masked: %s",
			parent.Category, int(parent.Confidence), parent.Masked, parent.Reason)
	}
	if !child.Masked || child.Category != pipeline.CatFreeText {
		t.Errorf("members.ስም is %s/%d masked=%v, want free_text masked: %s",
			child.Category, int(child.Confidence), child.Masked, child.Reason)
	}
	if parent.Category != child.Category {
		t.Errorf("name_dim.ስም (%s) and members.ስም (%s) disagree: the load would VALIDATE a foreign key between two different masked categories",
			parent.Category, child.Category)
	}
}

// T-0253's own residual, pinned rather than left implicit: fkPairs refuses
// to raise a column alone when its validated-foreign-key partner is itself a
// two-letter-code lookup -- real, measured evidence the partner's whole
// domain is a short code and not personal data, which this rail must not
// override on the strength of an unrelated neighbour two tables away.
// Neither end is masked here, exactly as neither was before T-0253 (never
// copying only one end of the pair) -- and both carry Decision.Refused
// naming the other end, which is this package's whole contribution: it
// leaves a decision unmade rather than guessing. internal/plan is what turns
// a non-empty Refused into an exit-12 refusal naming both columns, in place
// of the pre-T-0253 copy under exit 0 (tracker T-0257, closed alongside this
// one; internal/plan/fkpair.go). This test still asserts the classify-level
// shape only -- neither column masked, both Refused -- because what happens
// to a Decision once classify hands it to internal/plan is that package's
// own test to hold (internal/plan/fkpair_test.go).
func TestFKPairRefusedWhenPartnerIsTwoLetterCodes(t *testing.T) {
	t.Parallel()
	lookup := ref.TableRef{Schema: "public", Name: "region_lookup"}
	members := ref.TableRef{Schema: "public", Name: "members"}
	schema := &pipeline.Schema{
		Tables: []pipeline.Table{
			tt("public", "region_lookup", nil, tc("tag", "text")),
			tt("public", "members", []string{"id"},
				tc("id", "bigint"),
				tc("email", "text"),
				tc("tag", "text"),
			),
		},
		FKs: []pipeline.ForeignKey{
			fk("members_tag_fkey", members, []string{"tag"}, lookup, []string{"tag"}),
		},
	}
	s := mapSampler{
		col(lookup, "tag"): anyOf("EN", "FR", "DE"),
		col(members, "email"): anyOf("member01@realcorp.example", "member02@realcorp.example",
			"member03@realcorp.example"),
		// members.tag carries no samples of its own: raisableUnknown must see
		// it as blank, not as a two-letter-code column in its own right, so
		// this test is about the *partner's* value shape and not the
		// column's own.
	}
	cls, err := New().Classify(schema, s, nil)
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	if d := cls.Decisions[col(lookup, "tag")]; d.Masked {
		t.Errorf("region_lookup.tag is %s/%d and masked despite being a two-letter-code column: %s",
			d.Category, int(d.Confidence), d.Reason)
	}
	if d := cls.Decisions[col(members, "tag")]; d.Masked {
		t.Errorf("members.tag is %s/%d and masked alone, with its foreign-key partner still unmasked: %s",
			d.Category, int(d.Confidence), d.Reason)
	}
	member := cls.Decisions[col(members, "tag")]
	if member.Refused == "" || member.RefusedPartner != col(lookup, "tag") {
		t.Errorf("members.tag Refused=%q RefusedPartner=%v, want a refusal naming region_lookup.tag",
			member.Refused, member.RefusedPartner)
	}
	partner := cls.Decisions[col(lookup, "tag")]
	if partner.Refused == "" || partner.RefusedPartner != col(members, "tag") {
		t.Errorf("region_lookup.tag Refused=%q RefusedPartner=%v, want a refusal naming members.tag",
			partner.Refused, partner.RefusedPartner)
	}
}

// The type-conflict twin of the test above: the partner already carries a
// decision of its own -- a name hit (`dob`) its citext type does not accept,
// recorded at `low` with typeConflict set -- and ARCHITECTURE.md §4 never
// lets a raising pass move a type-conflicting decision. fkPairs refuses the
// pair rather than override it, for the same "never copy the pair, never
// override what ARCHITECTURE.md pins" reason as the two-letter-code case,
// and both ends carry Decision.Refused for the same reason that test's own
// comment gives -- which internal/plan now turns into an exit-12 refusal
// naming both columns (tracker T-0257, internal/plan/fkpair.go), in place of
// the pre-T-0253 copy under exit 0; this test still holds the classify-level
// shape alone.
func TestFKPairRefusedWhenPartnerHasTypeConflict(t *testing.T) {
	t.Parallel()
	profiles := ref.TableRef{Schema: "public", Name: "profiles"}
	members := ref.TableRef{Schema: "public", Name: "members"}
	schema := &pipeline.Schema{
		Tables: []pipeline.Table{
			tt("public", "profiles", nil, tc("dob", "citext")),
			tt("public", "members", []string{"id"},
				tc("id", "bigint"),
				tc("email", "text"),
				tc("linkval", "citext"),
			),
		},
		FKs: []pipeline.ForeignKey{
			fk("members_linkval_fkey", members, []string{"linkval"}, profiles, []string{"dob"}),
		},
	}
	s := mapSampler{
		// Not dates: profiles.dob's name matches person_date, whose accepts
		// list is date/timestamp/text/varchar/bpchar and not citext, so the
		// name hit is rejected by type -- and with no date-shaped value to
		// fall back on, decide() records the conflict rather than a date.
		col(profiles, "dob"): anyOf("zzz-not-a-date-1", "zzz-not-a-date-2", "zzz-not-a-date-3"),
		col(members, "email"): anyOf("member01@realcorp.example", "member02@realcorp.example",
			"member03@realcorp.example"),
	}
	cls, err := New().Classify(schema, s, nil)
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	if d := cls.Decisions[col(profiles, "dob")]; d.Category != pipeline.CatPersonDate || d.Masked {
		t.Fatalf("profiles.dob is %s/%d masked=%v, want person_date at low, unmasked (typeConflict), to set up this test: %s",
			d.Category, int(d.Confidence), d.Masked, d.Reason)
	}
	if d := cls.Decisions[col(members, "linkval")]; d.Masked {
		t.Errorf("members.linkval is %s/%d and masked alone, with its type-conflicting foreign-key partner unresolved: %s",
			d.Category, int(d.Confidence), d.Reason)
	}
	child := cls.Decisions[col(members, "linkval")]
	if child.Refused == "" || child.RefusedPartner != col(profiles, "dob") {
		t.Errorf("members.linkval Refused=%q RefusedPartner=%v, want a refusal naming profiles.dob",
			child.Refused, child.RefusedPartner)
	}
	parent := cls.Decisions[col(profiles, "dob")]
	if parent.Refused == "" || parent.RefusedPartner != col(members, "linkval") {
		t.Errorf("profiles.dob Refused=%q RefusedPartner=%v, want a refusal naming members.linkval",
			parent.Refused, parent.RefusedPartner)
	}
}

// T-0253's review round, medium finding: fkPairs used to walk the whole
// connected FK component, not just cref's direct partners, so a column in a
// table with no relationship at all to the certain neighbour that justified
// raising cref could veto the pairing on its own shape. Here,
// members.currency and currencies.code are the real pair beside members'
// certain email column; invoices.currency shares the same currencies.code
// parent but has nothing to do with members or its email, and is given a
// type (bigint) fkPartnerRaisable refuses on sight (not a character family).
// Before this fix, that refusal reached across two hops and left
// members.currency and currencies.code both unmasked -- the leak this rail
// exists to close, vetoed by a column that never should have had a vote.
// After it, fkPairs only asks about currencies.code, cref's direct partner;
// invoices.currency is left to propagateKeys (a separate, later pass), which
// records a type conflict on it without touching the parent's own masking.
func TestFKPairIgnoresAnUnrelatedGrandchildOfASharedLookupParent(t *testing.T) {
	t.Parallel()
	currencies := ref.TableRef{Schema: "public", Name: "currencies"}
	members := ref.TableRef{Schema: "public", Name: "members"}
	invoices := ref.TableRef{Schema: "public", Name: "invoices"}
	schema := &pipeline.Schema{
		Tables: []pipeline.Table{
			tt("public", "currencies", []string{"code"}, tc("code", "varchar(3)")),
			tt("public", "members", []string{"id"},
				tc("id", "bigint"),
				tc("email", "text"),
				tc("currency", "varchar(3)"),
			),
			tt("public", "invoices", []string{"id"},
				tc("id", "bigint"),
				// Not a character family, so fkPartnerRaisable refuses it on
				// sight -- standing in for any shape an unrelated column two
				// hops from cref could carry (a type conflict, a two-letter
				// code) that must not reach back to veto cref's own pair.
				tc("currency_ref", "bigint"),
			),
		},
		FKs: []pipeline.ForeignKey{
			fk("members_currency_fkey", members, []string{"currency"}, currencies, []string{"code"}),
			fk("invoices_currency_fkey", invoices, []string{"currency_ref"}, currencies, []string{"code"}),
		},
	}
	s := mapSampler{
		col(currencies, "code"): anyOf("USD", "EUR", "GBP"),
		col(members, "email"): anyOf("member01@realcorp.example", "member02@realcorp.example",
			"member03@realcorp.example"),
	}
	cls, err := New().Classify(schema, s, nil)
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	member := cls.Decisions[col(members, "currency")]
	parent := cls.Decisions[col(currencies, "code")]
	if !member.Masked || member.Category != pipeline.CatFreeText {
		t.Errorf("members.currency is %s/%d masked=%v, want free_text masked -- an unrelated invoices.currency_ref must not veto this pair: %s",
			member.Category, int(member.Confidence), member.Masked, member.Reason)
	}
	if !parent.Masked || parent.Category != pipeline.CatFreeText {
		t.Errorf("currencies.code is %s/%d masked=%v, want free_text masked: %s",
			parent.Category, int(parent.Confidence), parent.Masked, parent.Reason)
	}
	if member.Refused != "" || parent.Refused != "" {
		t.Errorf("members.currency Refused=%q currencies.code Refused=%q, want neither set: the pair is raised, not refused",
			member.Refused, parent.Refused)
	}
	// invoices.currency_ref is untouched by fkPairs (it is not cref's direct
	// partner) and propagateKeys refuses to move a category bigint cannot
	// accept onto it, so it stays copied -- a residual of its own type, not
	// this test's concern, and proof the unrelated column never got a veto.
	if child := cls.Decisions[col(invoices, "currency_ref")]; child.Masked {
		t.Errorf("invoices.currency_ref is %s/%d masked=%v, want copied: free_text is not an accepted category for bigint",
			child.Category, int(child.Confidence), child.Masked)
	}
}

// T-0253's second review round, high finding: bounding fkPairs to cref's
// direct partners closed the medium finding above but reopened the
// asymmetry T-0253 exists to fix, one hop further out. Here members.slug_leaf
// is cref, raised together with its direct partner name_mid.slug_mid -- but
// slug_mid is itself the *child* end of a further validated foreign key, to
// name_root.slug_root, and nothing except fkPairs' own upward walk ever
// reaches that further parent: propagateKeys only ever pushes a decision
// from a masked parent down to its children, never up from an unmasked one.
// Before the fix, name_root.slug_root stayed CatNone and was copied
// verbatim, holding the identical values members.slug_leaf and
// name_mid.slug_mid were being masked for (THREAT_MODEL.md T1), and the load
// would VALIDATE a foreign key between a masked child and an unmasked
// parent (the T-0132 half-loaded-target shape, T8).
//
// Every column in the chain has a distinct name (slug_root, slug_mid,
// slug_leaf) on purpose: identical names across the three tables would let
// the same_name pass (sameColumnName, a separate pass that runs after this
// rail) mask name_root.slug_root for an unrelated reason and hide the bug
// this test exists to catch.
func TestFKPairWalksUpwardThroughAChainedForeignKey(t *testing.T) {
	t.Parallel()
	nameRoot := ref.TableRef{Schema: "public", Name: "name_root"}
	nameMid := ref.TableRef{Schema: "public", Name: "name_mid"}
	members := ref.TableRef{Schema: "public", Name: "members"}
	names := []string{"ኣበበ ኪዳነ", "ተስፋዬ ኪዳነ", "ገብረ ኣበበ", "ኪዳነ ተስፋዬ", "ኣበበ ገብረ"}
	schema := &pipeline.Schema{
		Tables: []pipeline.Table{
			tt("public", "name_root", []string{"slug_root"}, tc("slug_root", "character varying(12)")),
			tt("public", "name_mid", []string{"slug_mid"}, tc("slug_mid", "character varying(12)")),
			tt("public", "members", []string{"id"},
				tc("id", "bigint"),
				tc("email", "text"),
				tc("slug_leaf", "character varying(12)"),
			),
		},
		FKs: []pipeline.ForeignKey{
			fk("name_mid_slug_mid_fkey", nameMid, []string{"slug_mid"}, nameRoot, []string{"slug_root"}),
			fk("members_slug_leaf_fkey", members, []string{"slug_leaf"}, nameMid, []string{"slug_mid"}),
		},
	}
	s := mapSampler{
		col(nameRoot, "slug_root"): anyOf(names[0], names[1], names[2], names[3], names[4]),
		col(nameMid, "slug_mid"):   anyOf(names[0], names[1], names[2], names[3], names[4]),
		col(members, "email"): anyOf("member01@realcorp.example", "member02@realcorp.example",
			"member03@realcorp.example", "member04@realcorp.example", "member05@realcorp.example"),
		col(members, "slug_leaf"): anyOf(names[0], names[1], names[2], names[3], names[4]),
	}
	cls, err := New().Classify(schema, s, nil)
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	root := cls.Decisions[col(nameRoot, "slug_root")]
	mid := cls.Decisions[col(nameMid, "slug_mid")]
	leaf := cls.Decisions[col(members, "slug_leaf")]
	if !root.Masked || root.Category != pipeline.CatFreeText {
		t.Errorf("name_root.slug_root is %s/%d masked=%v, want free_text masked -- the upward walk must reach it: %s",
			root.Category, int(root.Confidence), root.Masked, root.Reason)
	}
	if !mid.Masked || mid.Category != pipeline.CatFreeText {
		t.Errorf("name_mid.slug_mid is %s/%d masked=%v, want free_text masked: %s",
			mid.Category, int(mid.Confidence), mid.Masked, mid.Reason)
	}
	if !leaf.Masked || leaf.Category != pipeline.CatFreeText {
		t.Errorf("members.slug_leaf is %s/%d masked=%v, want free_text masked: %s",
			leaf.Category, int(leaf.Confidence), leaf.Masked, leaf.Reason)
	}
	if root.Category != mid.Category || mid.Category != leaf.Category {
		t.Errorf("name_root.slug_root (%s), name_mid.slug_mid (%s) and members.slug_leaf (%s) disagree: the load would VALIDATE a foreign key between differently masked ends",
			root.Category, mid.Category, leaf.Category)
	}
}
