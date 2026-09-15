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
