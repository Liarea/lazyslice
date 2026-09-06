// SPDX-License-Identifier: Apache-2.0

package transform

import (
	"math/big"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
	"github.com/Liarea/lazyslice/mask"
)

// The two halves of T-0054's promise about this package: that it names a
// column's family the same way the plan-time write-back check does, and that
// its own refusal is a backstop the plan can no longer let anything reach.

// TestFamilyNamesMatchMask reads this package's family vocabulary against
// mask.TypeTag, which owns the table both this package and internal/plan reduce
// a catalog type name with. The constants here are still spelled out, because
// isDocument and maxLen branch on three of them and mask's are unexported; this
// is what keeps the two spellings one vocabulary.
func TestFamilyNamesMatchMask(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ typeName, want string }{
		{"text", famText},
		{"character varying", famVarchar},
		{"character", famBpchar},
		{"citext", famCitext},
		{"boolean", famBoolean},
		{"integer", famInteger},
		{"bigint", famBigint},
		{"numeric", famNumeric},
		{"double precision", famFloat},
		{"date", famDate},
		{"timestamp with time zone", famTimestamp},
		{"time without time zone", famTime},
		{"interval", famInterval},
		{"uuid", famUUID},
		{"inet", famInet},
		{"cidr", famCIDR},
		{"macaddr", famMacaddr},
		{"bytea", famBytea},
		{"json", famJSON},
		{"jsonb", famJSONB},
		{"hstore", famHstore},
		{"tsvector", famTSVector},
	} {
		got, ok := mask.TypeTag(tc.typeName)
		if !ok || got != tc.want {
			t.Errorf("mask.TypeTag(%q) = %q, %v; this package calls it %q", tc.typeName, got, ok, tc.want)
		}
	}
	// An enum and anything else the catalog can name are not families mask has
	// a tag for: this package calls them famEnum and famOther after asking the
	// schema, and internal/plan leaves them unjudged.
	if _, ok := mask.TypeTag("public.account_status"); ok {
		t.Error("mask.TypeTag names a family for an enum; shapeOf and the plan check both expect it not to")
	}
	if _, ok := mask.TypeTag("box"); ok {
		t.Errorf("mask.TypeTag names a family for a type this module does not model; %q is what shapeOf calls one", famOther)
	}
}

// probeValue is a value of the Go kind pgx decodes a column of this type into.
// It is what makes the refusal real: coerce switches on the kind the value
// arrived as, and a masker's text that will not parse back into it is exactly
// the exit-7 refusal T-0054 was opened for.
//
// The six families that used to be given a plain Go string here — time,
// interval, numeric, inet, cidr and macaddr — are the ones that made the
// assertion vacuous: coerce's `case nil, string` hands the masker's text
// straight back, so no combination on them could ever refuse and the loop said
// nothing about them. They now carry what pgx actually decodes those columns
// into (internal/transform/value.go names the same list), so textOf's
// Stringer/Valuer path and coerce's reflective scanBack are both exercised.
func probeValue(family string) any {
	switch family {
	case famBoolean:
		return true
	case famInteger:
		return int32(4711)
	case famBigint:
		return int64(90000)
	case famFloat:
		return float64(51.5074)
	case famDate, famTimestamp:
		return time.Date(2017, 2, 15, 9, 34, 33, 0, time.UTC)
	case famUUID:
		return [16]byte{0x0f, 0x1e, 0x2d, 0x3c, 0x4b, 0x5a, 0x69, 0x78,
			0x87, 0x96, 0xa5, 0xb4, 0xc3, 0xd2, 0xe1, 0xf0}
	case famBytea:
		return []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}
	case famJSON, famJSONB:
		return map[string]any{"contact": map[string]any{"email": "ada.lovelace@example.com"}}
	case famInet:
		return netip.MustParsePrefix("203.0.113.7/32")
	case famCIDR:
		return netip.MustParsePrefix("203.0.113.0/24")
	case famMacaddr:
		return net.HardwareAddr{0x08, 0x00, 0x2b, 0x01, 0x02, 0x03}
	case famTime:
		return pgtype.Time{Microseconds: 34473000000, Valid: true}
	case famInterval:
		return pgtype.Interval{Days: 3, Valid: true}
	case famTSVector:
		// pgx has no codec for tsvector, so it arrives as its text form.
		return "'ada':1 'lovelac':2"
	case famNumeric:
		return pgtype.Numeric{Int: big.NewInt(123456), Exp: -2, Valid: true}
	default:
		// Every text family, and the address-shaped, high-entropy, mixed
		// string that trips more than one validator at once.
		return "9393 Elm Mews, Flat 2b"
	}
}

// probeTypes is one catalog spelling per family, as introspect would record it.
var probeTypes = map[string]string{
	famText:      "text",
	famVarchar:   "character varying(64)",
	famBpchar:    "character(16)",
	famCitext:    "citext",
	famBoolean:   "boolean",
	famInteger:   "integer",
	famBigint:    "bigint",
	famNumeric:   "numeric(10,2)",
	famFloat:     "double precision",
	famDate:      "date",
	famTimestamp: "timestamp with time zone",
	famTime:      "time without time zone",
	famInterval:  "interval",
	famUUID:      "uuid",
	famInet:      "inet",
	famCIDR:      "cidr",
	famMacaddr:   "macaddr",
	famBytea:     "bytea",
	famJSON:      "json",
	famJSONB:     "jsonb",
	famHstore:    "hstore",
	famTSVector:  "tsvector",
}

// everyCategory is the categories a column can be masked under. CatNone is not
// one of them: it is what a copied column carries and it reaches no masker.
var everyCategory = []pipeline.Category{
	pipeline.CatPersonName, pipeline.CatEmail, pipeline.CatPhone, pipeline.CatAddress,
	pipeline.CatGeo, pipeline.CatPersonDate, pipeline.CatNationalID, pipeline.CatFinancial,
	pipeline.CatNetworkID, pipeline.CatOnlineID, pipeline.CatCredential, pipeline.CatFreeText,
	pipeline.CatSpecial, pipeline.CatBinary, pipeline.CatSemiStruct,
	pipeline.CatDerivedText,
}

// TestTransformNeverRefusesWhatThePlanCheckAdmits is the statement that this
// package's exit-7 refusal is now a backstop rather than a discovery.
//
// It masks one probe column under every category on every type family — 16 x 22
// columns, each with a value of the Go kind pgx decodes that type into — and
// asserts one direction: a combination this package refuses is one
// mask.Writable already said no to, which is what internal/plan refuses at exit
// 12 before a row moves. The other direction is deliberately not asserted: a
// combination Writable rejects may still survive here — coerce keeps the
// masker's text for any kind it has no parse back into, which is the honest
// answer for a type this package does not model — and a refusal the plan
// already made is not a bug in this package.
//
// The failure this test would have caught: credential on a timestamp. Before
// T-0054 mask.Writable did not exist, the classifier decided it on pagila's
// every last_update, and the first thing to notice was the refusal below.
func TestTransformNeverRefusesWhatThePlanCheckAdmits(t *testing.T) {
	t.Parallel()
	k := key(t, 0x5c)
	for _, cat := range everyCategory {
		ids := mask.Candidates(mask.Category(cat))
		if len(ids) == 0 {
			t.Errorf("category %s has no registered masker", cat)
			continue
		}
		for family, typeName := range probeTypes {
			table := pipeline.Table{
				Ref:     tbl("probe"),
				Columns: []pipeline.Column{{Name: "c", TypeName: typeName, Nullable: true}},
			}
			schema := &pipeline.Schema{Tables: []pipeline.Table{table}}
			c := ref.ColumnRef{Table: table.Ref, Column: "c"}
			cls := &pipeline.Classification{Decisions: map[ref.ColumnRef]pipeline.Decision{
				c: {Col: c, Category: cat, Confidence: pipeline.ConfCertain, Masker: ids[0], Masked: true},
			}}
			batch := pipeline.RowBatch{
				Table: table.Ref,
				Cols:  []string{"c"},
				Rows:  [][]any{{probeValue(family)}},
			}
			tr := New(schema)
			_, err := tr.Transform(batch, cls, &k, NewResidual(10))
			if err == nil {
				continue
			}
			shape := tr.(transformer).shapeOf(&table, table.Columns[0])
			if mask.Writable(mask.Category(cat), ids[0], shape.constraints) {
				t.Errorf("%s on a %s column: the plan-time check admits it and transform refused it at exit 7: %v",
					cat, family, err)
			}
		}
	}
}

// TestDerivedTextEmptiesATSVector is the write path T-0054 added: a tsvector is
// masked to the empty tsvector, which travels as the empty string and is what
// an empty tsvector literal holds. It is never copied — the lexemes are the words of the column it was
// derived from, which may itself be masked.
func TestDerivedTextEmptiesATSVector(t *testing.T) {
	t.Parallel()
	table := pipeline.Table{
		Ref: tbl("film"),
		Columns: []pipeline.Column{
			{Name: "film_id", TypeName: "integer"},
			{Name: "fulltext", TypeName: "tsvector"},
		},
		PK: []string{"film_id"},
	}
	schema := &pipeline.Schema{Tables: []pipeline.Table{table}}
	c := ref.ColumnRef{Table: table.Ref, Column: "fulltext"}
	cls := &pipeline.Classification{Decisions: map[ref.ColumnRef]pipeline.Decision{
		c: {
			Col: c, Category: pipeline.CatDerivedText,
			Confidence: pipeline.ConfCertain, Masker: mask.MaskerDerivedText, Masked: true,
		},
	}}
	batch := pipeline.RowBatch{
		Table: table.Ref,
		Cols:  []string{"film_id", "fulltext"},
		Rows: [][]any{
			{int32(1), "'academi':1 'battl':15 'canadian':20"},
			{int32(2), "'ace':1 'administr':9 'china':20"},
		},
	}
	k := key(t, 0x7e)
	out, err := New(schema).Transform(batch, cls, &k, NewResidual(10))
	if err != nil {
		t.Fatalf("Transform: %v", err)
	}
	for i, row := range out.Rows {
		got, ok := row[1].(string)
		if !ok {
			t.Fatalf("row %d: fulltext masked to %T, want the text form the loader writes back", i, row[1])
		}
		if got != "" {
			t.Errorf("row %d: fulltext masked to %q, want the empty tsvector", i, got)
		}
	}
	// The column's shape is a tsvector, and only derived_text can be written
	// into one: nothing else in the rule pack may reach it.
	shape := New(schema).(transformer).shapeOf(&table, table.Columns[1])
	if shape.family != famTSVector {
		t.Errorf("fulltext is family %q, want %q", shape.family, famTSVector)
	}
	if !mask.Writable(mask.CatDerivedText, mask.MaskerDerivedText, shape.constraints) {
		t.Error("derived_text cannot be written into a tsvector, which is the only type it accepts")
	}
}
