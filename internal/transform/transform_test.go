// SPDX-License-Identifier: Apache-2.0

package transform

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
	"github.com/Liarea/lazyslice/mask"
)

// Everything this package promises is testable without a database: Transform is
// pure apart from Residual.Add, so a batch built by hand and replayed against
// the same key is the whole of the determinism contract invariant I3 rests on.

// ---------- fixtures ----------

func tbl(name string) ref.TableRef { return ref.TableRef{Schema: "public", Name: name} }

func col(table, name string) ref.ColumnRef {
	return ref.ColumnRef{Table: tbl(table), Column: name}
}

// fixture is the schema every test here masks against: one table with a scalar
// masked column under a unique index, an array column, a free-text column and a
// document column, plus a log-shaped table with a document column of its own.
func fixture() *pipeline.Schema {
	people := pipeline.Table{
		Ref: tbl("people"),
		Columns: []pipeline.Column{
			{Name: "person_id", TypeName: "bigint", TypeOID: 20},
			{Name: "email", TypeName: "text", TypeOID: 25},
			{Name: "ref", TypeName: "character varying(64)", TypeOID: 1043, TypMod: 68},
			{Name: "notes", TypeName: "text", TypeOID: 25, Nullable: true},
			{Name: "contact", TypeName: "jsonb", TypeOID: 3802},
			{Name: "alt_emails", TypeName: "text[]", TypeOID: 1009, Nullable: true},
			{Name: "status", TypeName: "public.account_status", Nullable: false},
			{Name: "national_id", TypeName: "bigint", TypeOID: 20, Nullable: true},
		},
		PK: []string{"person_id"},
		Indexes: []pipeline.Index{
			{Name: "people_email_key", Columns: []string{"email"}, Unique: true, Immediate: true},
		},
	}
	sites := pipeline.Table{
		Ref: tbl("sites"),
		Columns: []pipeline.Column{
			{Name: "site_code", TypeName: "text", TypeOID: 25},
			{Name: "contact_email", TypeName: "text", TypeOID: 25, Nullable: true},
		},
		PK: []string{"site_code"},
	}
	events := pipeline.Table{
		Ref: tbl("events"),
		Columns: []pipeline.Column{
			{Name: "event_id", TypeName: "bigint", TypeOID: 20},
			{Name: "payload", TypeName: "jsonb", TypeOID: 3802},
		},
		PK: []string{"event_id"},
	}
	return &pipeline.Schema{
		Tables: []pipeline.Table{people, sites, events},
		Enums:  map[string][]string{"public.account_status": {"active", "pending", "suspended", "closed"}},
	}
}

func classification() *pipeline.Classification {
	masked := func(c ref.ColumnRef, cat pipeline.Category, id mask.ID) pipeline.Decision {
		return pipeline.Decision{Col: c, Category: cat, Confidence: pipeline.ConfCertain, Masker: id, Masked: true}
	}
	d := map[ref.ColumnRef]pipeline.Decision{
		col("people", "email"):       masked(col("people", "email"), pipeline.CatEmail, mask.MaskerEmail),
		col("people", "ref"):         masked(col("people", "ref"), pipeline.CatEmail, mask.MaskerEmail),
		col("people", "notes"):       masked(col("people", "notes"), pipeline.CatFreeText, mask.MaskerFreeText),
		col("people", "contact"):     masked(col("people", "contact"), pipeline.CatSemiStruct, mask.MaskerSemiStruct),
		col("people", "alt_emails"):  masked(col("people", "alt_emails"), pipeline.CatEmail, mask.MaskerEmail),
		col("people", "status"):      masked(col("people", "status"), pipeline.CatSpecial, mask.MaskerSpecial),
		col("people", "national_id"): masked(col("people", "national_id"), pipeline.CatNationalID, mask.MaskerNationalID),
		col("sites", "contact_email"): masked(col("sites", "contact_email"),
			pipeline.CatEmail, mask.MaskerEmail),
		col("events", "payload"): masked(col("events", "payload"), pipeline.CatSemiStruct, mask.MaskerSemiStruct),
	}
	// person_id is a surrogate key: never masked, always explained (§4).
	d[col("people", "person_id")] = pipeline.Decision{Col: col("people", "person_id"), Category: pipeline.CatNone}
	return &pipeline.Classification{Decisions: d}
}

var peopleCols = []string{"person_id", "email", "ref", "notes", "contact", "alt_emails", "status", "national_id"}

func document() map[string]any {
	return map[string]any{
		"profile": map[string]any{
			"contact": map[string]any{
				"email": "ada.lovelace@example.com",
				"phone": "+44 20 7946 0958",
				"age":   float64(36),
				"score": 4.5,
				"admin": true,
				"note":  nil,
			},
			"locale": "en-GB",
		},
		"tags": []any{"founder", "reviewer"},
	}
}

func peopleBatch() pipeline.RowBatch {
	return pipeline.RowBatch{
		Table: tbl("people"),
		Cols:  peopleCols,
		Rows: [][]any{
			{
				int64(90000), "Ada.Lovelace@example.com", "ada.lovelace@example.com",
				"Ada Lovelace asked that Grace Hopper be copied on the renewal.",
				document(),
				[]any{"ada@example.org", nil, "a.lovelace@example.net"},
				"active", int64(123456789),
			},
			{
				int64(90007), "grace.hopper@example.com", "grace.hopper@example.com",
				"", // the empty string survives, and §6 item 6 says so
				document(),
				[]any{},
				"pending", nil,
			},
		},
		Last: true,
	}
}

func key(t *testing.T, b byte) mask.Key {
	t.Helper()
	var k mask.Key
	for i := range k {
		k[i] = b
	}
	return k
}

// run masks one batch and returns it. The batch is rebuilt each time, because
// Transform masks in place.
func run(t *testing.T, b pipeline.RowBatch, k mask.Key) pipeline.RowBatch {
	t.Helper()
	out, err := New(fixture()).Transform(b, classification(), &k, NewResidual(1000))
	if err != nil {
		t.Fatalf("Transform: %v", err)
	}
	return out
}

// ---------- determinism ----------

func TestTwoRunsWithOneKeyProduceTheSameBatch(t *testing.T) {
	k := key(t, 0x11)
	first := run(t, peopleBatch(), k)
	second := run(t, peopleBatch(), k)
	if !reflect.DeepEqual(first.Rows, second.Rows) {
		t.Errorf("two runs with one key differ:\n first %v\nsecond %v", first.Rows, second.Rows)
	}
}

func TestTwoKeysProduceDifferentValues(t *testing.T) {
	a := run(t, peopleBatch(), key(t, 0x11))
	b := run(t, peopleBatch(), key(t, 0x22))
	if reflect.DeepEqual(a.Rows, b.Rows) {
		t.Fatal("two different run keys produced the same masked batch")
	}
	for i, name := range peopleCols {
		switch name {
		case "person_id":
			// A surrogate key is preserved verbatim under every key
			// (ARCHITECTURE.md §4, THREAT_MODEL.md "Positions").
			if a.Rows[0][i] != b.Rows[0][i] {
				t.Errorf("%s changed between keys; it is never masked", name)
			}
			continue
		case "status":
			// special_category over four enum labels is a small domain, and §5
			// collapses it to one fixed label rather than pretending a
			// substitution over four values is a mask. The same label under
			// both keys is the rule working.
			continue
		}
		if reflect.DeepEqual(a.Rows[0][i], b.Rows[0][i]) {
			t.Errorf("people.%s masked to the same value under two keys: %v", name, a.Rows[0][i])
		}
	}
}

// ---------- uniqueness ----------

// A column under a unique index must not collide after masking: the load would
// fail on a unique violation whose PgError.Detail is dropped (§5, T4).
func TestDistinctValuesStayDistinctUnderAUniqueColumn(t *testing.T) {
	const n = 2000
	rows := make([][]any, 0, n)
	for i := range n {
		rows = append(rows, []any{
			int64(i), "person" + itoa(i) + "@example.com", "r" + itoa(i),
			"note", document(), []any{}, "active", nil,
		})
	}
	b := pipeline.RowBatch{Table: tbl("people"), Cols: peopleCols, Rows: rows, Last: true}
	out := run(t, b, key(t, 0x33))

	seen := make(map[string]int, n)
	for i, row := range out.Rows {
		v, ok := row[1].(string)
		if !ok {
			t.Fatalf("row %d masked people.email to %T, want a string", i, row[1])
		}
		if prev, dup := seen[v]; dup {
			t.Fatalf("rows %d and %d masked people.email to one value; the column is unique", prev, i)
		}
		seen[v] = i
	}
	if len(seen) != n {
		t.Errorf("%d distinct masked values for %d distinct addresses", len(seen), n)
	}
}

// Equal inputs mask alike, which is what makes a masked natural key still join.
// It is also §4's FK propagation seen from this side: the classifier gives a
// masked parent key and every column referencing it one category, and the
// determinism contract is what then makes the two ends agree.
func TestEqualValuesMaskAlikeAcrossColumnsOfOneCategory(t *testing.T) {
	k := key(t, 0x44)
	people := run(t, peopleBatch(), k)
	sites := pipeline.RowBatch{
		Table: tbl("sites"),
		Cols:  []string{"site_code", "contact_email"},
		Rows: [][]any{
			{"HQ", "ada.lovelace@example.com"},
			{"BRANCH", "  ADA.LOVELACE@EXAMPLE.COM "},
		},
		Last: true,
	}
	out, err := New(fixture()).Transform(sites, classification(), &k, NewResidual(10))
	if err != nil {
		t.Fatalf("Transform: %v", err)
	}
	// people.ref and sites.contact_email hold one address under one category and
	// neither is unique, so they mask to one value: that is what makes a masked
	// natural key still join, and it is §4's FK propagation seen from this end.
	if people.Rows[0][2] != out.Rows[0][1] {
		t.Errorf("one address in two columns of one category masked to %q and %q",
			people.Rows[0][2], out.Rows[0][1])
	}
	// Two spellings of one address reach one fake, because canonicalisation
	// folds case and trims (§5).
	if out.Rows[0][1] != out.Rows[1][1] {
		t.Errorf("two spellings of one address masked to %q and %q; canonicalisation folds case",
			out.Rows[0][1], out.Rows[1][1])
	}
	// people.email holds the same address and *is* under a unique index, so §5
	// gives it the generator with the larger domain and its value differs. A
	// masked value that collided across a unique index would fail the load on a
	// unique violation whose Detail is dropped.
	if people.Rows[0][1] == people.Rows[0][2] {
		t.Errorf("a unique column and a non-unique one masked one address alike: %q", people.Rows[0][1])
	}
}

// ---------- JSON ----------

// Every scalar leaf is replaced; structure and key names survive (§4,
// testdata/README.md trap 16a).
func TestEveryJSONLeafIsReplaced(t *testing.T) {
	out := run(t, peopleBatch(), key(t, 0x55))
	masked, ok := out.Rows[0][4].(map[string]any)
	if !ok {
		t.Fatalf("people.contact masked to %T, want the document's own shape", out.Rows[0][4])
	}

	source := document()
	sourceLeaves := leaves(t, source)
	maskedLeaves := leaves(t, masked)

	if len(sourceLeaves) != len(maskedLeaves) {
		t.Fatalf("the document has %d leaves and the masked one %d; structure must survive",
			len(sourceLeaves), len(maskedLeaves))
	}
	for path, want := range sourceLeaves {
		got, present := maskedLeaves[path]
		if !present {
			t.Errorf("the masked document has no leaf at %s; key names survive masking", path)
			continue
		}
		if want == nil {
			if got != nil {
				t.Errorf("%s was null and masked to %v; null stays null", path, got)
			}
			continue
		}
		if reflect.TypeOf(got) != reflect.TypeOf(want) {
			t.Errorf("%s was %T and masked to %T; a leaf keeps its kind", path, want, got)
		}
		if _, isBool := want.(bool); isBool {
			// A boolean is redrawn over a two-element domain, so under half the
			// run keys it comes back as it went in. Asserting inequality here
			// would make this test pass or fail on the fixture's key rather than
			// on the code: with the key at 0x11 the same assertion fails on a
			// correct implementation. The property a boolean leaf really has is
			// the distribution, and TestABooleanLeafIsRedrawnUnderTheRunKey has
			// it.
			continue
		}
		if reflect.DeepEqual(got, want) {
			t.Errorf("%s survived masking as %v", path, got)
		}
	}
}

// The boolean leaf's own property, over many keys rather than one: it is drawn
// from h, so across run keys both values appear. §6 item 6's small-domain caveat
// is why this is the strongest true statement about it — and why the leaf gets
// no residual filter entry (json.go), since an entry equal to the value the
// target holds would be a guaranteed hit and an exit 9 on a correct run.
func TestABooleanLeafIsRedrawnUnderTheRunKey(t *testing.T) {
	const path = "$.profile.contact.admin"
	seen := map[bool]int{}
	for b := range 32 {
		out := run(t, peopleBatch(), key(t, byte(b)))
		doc, ok := out.Rows[0][4].(map[string]any)
		if !ok {
			t.Fatalf("people.contact masked to %T", out.Rows[0][4])
		}
		v, ok := leaves(t, doc)[path].(bool)
		if !ok {
			t.Fatalf("%s masked to %T, want a bool", path, leaves(t, doc)[path])
		}
		seen[v]++
	}
	if seen[true] == 0 || seen[false] == 0 {
		t.Errorf("%s took only one value over 32 run keys: %v", path, seen)
	}
}

// A boolean leaf enters no filter entry, and neither does a null one. Both are
// the same rule: an entry the target's own value would match is a residual hit
// on a run that masked correctly (§6 item 3).
func TestABooleanLeafIsNotRecordedInTheFilter(t *testing.T) {
	k := key(t, 0x11)
	res := &recorder{inner: NewResidual(100)}
	if _, err := New(fixture()).Transform(peopleBatch(), classification(), &k, res); err != nil {
		t.Fatalf("Transform: %v", err)
	}
	for _, a := range res.adds {
		switch a.path {
		case "$.profile.contact.admin":
			t.Errorf("a boolean leaf was recorded in the filter as %q; half the run keys "+
				"leave it as it was, so the entry is a guaranteed residual hit", a.canonical)
		case "$.profile.contact.note":
			t.Errorf("a null leaf was recorded in the filter as %q; it is not masked", a.canonical)
		}
	}
}

// A jsonb column in a table named like an event log is replaced whole and not
// walked (§4, testdata/README.md trap 16b).
func TestALogShapedTableCollapsesTheDocument(t *testing.T) {
	k := key(t, 0x66)
	b := pipeline.RowBatch{
		Table: tbl("events"),
		Cols:  []string{"event_id", "payload"},
		Rows:  [][]any{{int64(1), document()}},
		Last:  true,
	}
	res := &recorder{inner: NewResidual(100)}
	out, err := New(fixture()).Transform(b, classification(), &k, res)
	if err != nil {
		t.Fatalf("Transform: %v", err)
	}
	doc, ok := out.Rows[0][1].(map[string]any)
	if !ok || len(doc) != 0 {
		t.Fatalf("events.payload masked to %#v, want an empty document", out.Rows[0][1])
	}
	for _, add := range res.adds {
		if add.path != "" {
			t.Errorf("a collapsed document added a per-leaf filter entry at %q; no per-leaf masker runs for it", add.path)
		}
	}
	if len(res.adds) != 1 {
		t.Errorf("a collapsed document added %d filter entries, want 1 for the document itself", len(res.adds))
	}
}

// A document that already is the collapse output records nothing. An audit
// table full of empty {} payloads is the common case, and seeding the filter
// with the value the target will hold makes §6 item 3 read {} off the target,
// find it in the filter, confirm it in the source and exit 9 on a run that
// masked correctly.
func TestADocumentThatIsAlreadyEmptyIsNotRecorded(t *testing.T) {
	k := key(t, 0x66)
	for name, payload := range map[string]any{
		"a decoded empty document":             map[string]any{},
		"an empty document as text":            "{}",
		"an empty document as text with space": " { } ",
	} {
		t.Run(name, func(t *testing.T) {
			b := pipeline.RowBatch{
				Table: tbl("events"),
				Cols:  []string{"event_id", "payload"},
				Rows:  [][]any{{int64(1), payload}},
				Last:  true,
			}
			res := &recorder{inner: NewResidual(100)}
			if _, err := New(fixture()).Transform(b, classification(), &k, res); err != nil {
				t.Fatalf("Transform: %v", err)
			}
			if len(res.adds) != 0 {
				t.Errorf("an already-empty payload added %d filter entries: %v", len(res.adds), res.adds)
			}
			if res.MayContain(col("events", "payload"), "", []byte("{}")) {
				t.Error("the filter answers for {}, which is what the target holds: verify would exit 9 on a correct run")
			}
		})
	}
}

// ---------- the refusal a masker raises ----------

// A masker refusal is exit 7 under transform.refused.masker, and it names the
// column and the masker and never the value (THREAT_MODEL.md T4). The guards
// above only cover the nil-argument checks, which are plain errors; this is the
// coded one core renders from the catalogue.
func TestAMaskerRefusalIsACodedExitSeven(t *testing.T) {
	k := key(t, 0xCC)
	const secret = "ada.lovelace@example.com"

	cases := map[string]mask.ID{
		// A decision that names no masker at all: caught while the batch is
		// planned, before a value is touched.
		"no masker named": "",
		// A masker id nothing in the compiled registry answers to. No masker may
		// be loaded at runtime (ADR-006), so this is a refusal and never a
		// lookup.
		"a masker the registry does not have": "no_such_masker",
	}
	for name, id := range cases {
		t.Run(name, func(t *testing.T) {
			cls := classification()
			c := col("people", "email")
			cls.Decisions[c] = pipeline.Decision{
				Col: c, Category: pipeline.CatEmail, Confidence: pipeline.ConfCertain,
				Masker: id, Masked: true,
			}
			_, err := New(fixture()).Transform(peopleBatch(), cls, &k, NewResidual(10))
			if err == nil {
				t.Fatal("Transform copied a value through that no masker masked")
			}
			var r *Refusal
			if !errors.As(err, &r) {
				t.Fatalf("Transform returned %T (%v), want a *transform.Refusal", err, err)
			}
			if r.Code != CodeMasker {
				t.Errorf("the refusal carries code %q, want %q", r.Code, CodeMasker)
			}
			if r.Exit != 7 {
				t.Errorf("the refusal exits %d, want 7 (ADR-005: extract or load)", r.Exit)
			}
			if r.Col != c {
				t.Errorf("the refusal names %s, want %s", r.Col, c)
			}
			if strings.Contains(r.Error(), secret) {
				t.Errorf("the refusal quotes the value it could not mask: %s", r.Error())
			}
		})
	}
}

// ---------- arrays, NULL and the empty string ----------

func TestArraysMaskElementWise(t *testing.T) {
	out := run(t, peopleBatch(), key(t, 0x77))
	arr, ok := out.Rows[0][5].([]any)
	if !ok {
		t.Fatalf("people.alt_emails masked to %T, want a slice", out.Rows[0][5])
	}
	if len(arr) != 3 {
		t.Fatalf("the array has %d elements, want 3: length is preserved", len(arr))
	}
	if arr[1] != nil {
		t.Errorf("a NULL element masked to %v; a NULL element stays NULL", arr[1])
	}
	for i, want := range []string{"ada@example.org", "a.lovelace@example.net"} {
		got := arr[i*2]
		if got == want {
			t.Errorf("element %d survived masking", i*2)
		}
		if _, isString := got.(string); !isString {
			t.Errorf("element %d masked to %T, want a string", i*2, got)
		}
	}
	if empty, isSlice := out.Rows[1][5].([]any); !isSlice || len(empty) != 0 {
		t.Errorf("an empty array masked to %#v; an empty array stays empty", out.Rows[1][5])
	}
}

func TestNullAndTheEmptyStringSurvive(t *testing.T) {
	out := run(t, peopleBatch(), key(t, 0x88))
	if out.Rows[1][3] != "" {
		t.Errorf("the empty string masked to %q; '' stays '' (§5)", out.Rows[1][3])
	}
	if out.Rows[1][7] != nil {
		t.Errorf("a NULL masked to %v; NULL stays NULL (§5)", out.Rows[1][7])
	}
}

// A masked value keeps the Go kind it arrived as, so the loader encodes a
// masked bigint as a bigint.
func TestAMaskedValueKeepsItsKind(t *testing.T) {
	out := run(t, peopleBatch(), key(t, 0x99))
	if _, ok := out.Rows[0][7].(int64); !ok {
		t.Errorf("a masked bigint came back as %T", out.Rows[0][7])
	}
	if got, ok := out.Rows[0][2].(string); !ok || len(got) > 64 {
		t.Errorf("a masked varchar(64) came back as %T of length %d", out.Rows[0][2], len(got))
	}
}

// A masked enum emits a valid label (§5).
func TestAMaskedEnumIsAValidLabel(t *testing.T) {
	out := run(t, peopleBatch(), key(t, 0xAA))
	labels := map[string]bool{"active": true, "pending": true, "suspended": true, "closed": true}
	for i, row := range out.Rows {
		s, ok := row[6].(string)
		if !ok || !labels[s] {
			t.Errorf("row %d masked people.status to %#v, which is not a label of the type", i, row[6])
		}
	}
}

// ---------- the residual filter ----------

type add struct {
	col       ref.ColumnRef
	path      string
	canonical string
}

type recorder struct {
	inner pipeline.Residual
	adds  []add
}

var _ pipeline.Residual = (*recorder)(nil)

func (r *recorder) Add(c ref.ColumnRef, path string, canonical []byte) {
	r.adds = append(r.adds, add{c, path, string(canonical)})
	r.inner.Add(c, path, canonical)
}

func (r *recorder) MayContain(c ref.ColumnRef, path string, canonical []byte) bool {
	return r.inner.MayContain(c, path, canonical)
}
func (r *recorder) Cells() int64 { return r.inner.Cells() }
func (r *recorder) Bytes() int64 { return r.inner.Bytes() }

func TestEveryMaskedCellAndEveryMaskedLeafEntersTheFilter(t *testing.T) {
	k := key(t, 0xBB)
	res := &recorder{inner: NewResidual(1000)}
	if _, err := New(fixture()).Transform(peopleBatch(), classification(), &k, res); err != nil {
		t.Fatalf("Transform: %v", err)
	}

	// The two document leaves trap 16a names, by their own paths.
	wantPaths := []string{"$.profile.contact.email", "$.profile.contact.phone", "$.tags[0]"}
	seen := map[string]bool{}
	for _, a := range res.adds {
		seen[a.path] = true
		if !res.MayContain(a.col, a.path, []byte(a.canonical)) {
			t.Errorf("the filter does not contain what it was just given: %s %s", a.col, a.path)
		}
	}
	for _, path := range wantPaths {
		if !seen[path] {
			t.Errorf("no filter entry for the leaf at %s; §6 item 1 keys each leaf by its path", path)
		}
	}
	// A value nothing masked is not in the filter. One probe is enough: the
	// filter is sized for 10^-6.
	if res.MayContain(col("people", "email"), "", []byte("nobody@example.invalid")) {
		t.Error("the filter contains a value the run never masked")
	}
}

// ---------- masked JSON object keys (T-0137 review round) ----------

// TestAMaskedKeysResidualEntryIsAtTheMaskedPath is finding 1: walk used to
// build a child's path from the source key while the masked key's own
// residual entry, and everything nested beneath it, is what the target
// spells with the masked key. internal/verify's keyHits and documentHits
// rebuild every path from the target alone, so a path built from the source
// key can never be found again — every leaf under a masked key was an
// untested blind spot in the residual scan. The child path must now be built
// from the masked key, and every entry recorded under it — the key's own and
// the leaf's beneath it — must use that same spelling.
func TestAMaskedKeysResidualEntryIsAtTheMaskedPath(t *testing.T) {
	k := key(t, 0x42)
	b := pipeline.RowBatch{
		Table: tbl("people"),
		Cols:  peopleCols,
		Rows: [][]any{{
			int64(1), "a@example.com", "a@example.com", "",
			map[string]any{"ada.lovelace@fixture.test": map[string]any{"note": "Jane Smith"}},
			nil, "active", nil,
		}},
		Last: true,
	}
	res := &recorder{inner: NewResidual(100)}
	out, err := New(fixture()).Transform(b, classification(), &k, res)
	if err != nil {
		t.Fatalf("Transform: %v", err)
	}
	doc, ok := out.Rows[0][4].(map[string]any)
	if !ok || len(doc) != 1 {
		t.Fatalf("people.contact masked to %#v, want one key", out.Rows[0][4])
	}
	var maskedKey string
	for name := range doc {
		maskedKey = name
	}
	if maskedKey == "ada.lovelace@fixture.test" {
		t.Fatal("the key did not change; it should have masked as an email")
	}

	wantKeyPath := "$." + maskedKey
	wantLeafPath := wantKeyPath + ".note"
	sourcePath := "$.ada.lovelace@fixture.test"

	seen := map[string]bool{}
	for _, a := range res.adds {
		seen[a.path] = true
		if !res.MayContain(a.col, a.path, []byte(a.canonical)) {
			t.Errorf("the filter does not contain what it was just given: %s", a.path)
		}
	}
	if !seen[wantKeyPath] {
		t.Errorf("no residual entry at %s (the masked key's own path); got paths %v", wantKeyPath, seen)
	}
	if !seen[wantLeafPath] {
		t.Errorf("no residual entry at %s (the leaf beneath the masked key); got paths %v", wantLeafPath, seen)
	}
	if seen[sourcePath] || seen[sourcePath+".note"] {
		t.Errorf("a residual entry was recorded under the source key's own path %s; "+
			"internal/verify rebuilds every path from the target and could never find it", sourcePath)
	}
}

// TestTwoKeysThatMaskAlikeAreRefused is finding 2: mask.Apply is a pure
// function of the canonical text, so two distinct source keys that
// canonicalise alike (two spellings of one email address, here) mask to the
// same fake key. Silently overwriting the first with the second would drop a
// whole subtree from the target with no error, no event and no counter.
func TestTwoKeysThatMaskAlikeAreRefused(t *testing.T) {
	k := key(t, 0x42)
	b := pipeline.RowBatch{
		Table: tbl("people"),
		Cols:  peopleCols,
		Rows: [][]any{{
			int64(1), "a@example.com", "a@example.com", "",
			map[string]any{
				"Ada.Lovelace@Fixture.Test": "first",
				"ada.lovelace@fixture.test": "second",
			},
			nil, "active", nil,
		}},
		Last: true,
	}
	_, err := New(fixture()).Transform(b, classification(), &k, NewResidual(100))
	if err == nil {
		t.Fatal("Transform returned no error; two keys that mask alike silently dropped one subtree")
	}
	var refusal *Refusal
	if !errors.As(err, &refusal) {
		t.Fatalf("Transform returned %T, want *Refusal", err)
	}
	if refusal.Code != CodeMasker {
		t.Errorf("refusal code is %s, want %s", refusal.Code, CodeMasker)
	}
	if refusal.Exit != exitTransform {
		t.Errorf("refusal exit is %d, want %d", refusal.Exit, exitTransform)
	}
	// THREAT_MODEL.md T4: no value ever reaches a refusal.
	for _, v := range []string{"Ada.Lovelace@Fixture.Test", "ada.lovelace@fixture.test"} {
		if strings.Contains(refusal.Error(), v) {
			t.Errorf("the refusal carries a source value: %q", refusal.Error())
		}
	}
}

// ---------- refusals ----------

func TestTransformRefusesWithoutTheThingsThatMakeItSafe(t *testing.T) {
	k := key(t, 0xCC)
	cases := map[string]func() error{
		"no classification": func() error {
			_, err := New(fixture()).Transform(peopleBatch(), nil, &k, NewResidual(10))
			return err
		},
		"no key": func() error {
			_, err := New(fixture()).Transform(peopleBatch(), classification(), nil, NewResidual(10))
			return err
		},
		"no residual filter": func() error {
			_, err := New(fixture()).Transform(peopleBatch(), classification(), &k, nil)
			return err
		},
		"no schema": func() error {
			_, err := New(nil).Transform(peopleBatch(), classification(), &k, NewResidual(10))
			return err
		},
	}
	for name, fn := range cases {
		t.Run(name, func(t *testing.T) {
			if err := fn(); err == nil {
				t.Error("Transform returned no error; masking without it would be a batch copied through")
			}
		})
	}
}

// A column with no decision is copied through: the threshold is the
// classifier's, and re-deciding here would be a second, quieter classifier.
func TestAnUndecidedColumnIsCopied(t *testing.T) {
	k := key(t, 0xDD)
	cls := &pipeline.Classification{Decisions: map[ref.ColumnRef]pipeline.Decision{}}
	b := peopleBatch()
	want := b.Rows[0][1]
	out, err := New(fixture()).Transform(b, cls, &k, NewResidual(10))
	if err != nil {
		t.Fatalf("Transform: %v", err)
	}
	if out.Rows[0][1] != want {
		t.Errorf("a column with no decision was changed: %v", out.Rows[0][1])
	}
}

// ---------- helpers ----------

// leaves flattens a document to path -> scalar, using the same path spelling
// the walker records in the filter.
func leaves(t *testing.T, doc any) map[string]any {
	t.Helper()
	out := map[string]any{}
	var walk func(path string, node any)
	walk = func(path string, node any) {
		switch n := node.(type) {
		case map[string]any:
			for k, v := range n {
				walk(path+"."+k, v)
			}
		case []any:
			for i, v := range n {
				walk(path+"["+itoa(i)+"]", v)
			}
		default:
			out[path] = node
		}
	}
	walk("$", doc)
	return out
}

func itoa(n int) string {
	var b strings.Builder
	if n == 0 {
		return "0"
	}
	digits := []byte{}
	for n > 0 {
		digits = append(digits, byte('0'+n%10))
		n /= 10
	}
	for i := len(digits) - 1; i >= 0; i-- {
		b.WriteByte(digits[i])
	}
	return b.String()
}

// jsonRoundTrip is what a document that reaches this package as text looks
// like, so the text path is covered as well as the decoded one.
func TestADocumentThatArrivesAsTextIsWalkedAndReturnedAsText(t *testing.T) {
	k := key(t, 0xEE)
	raw, err := json.Marshal(document())
	if err != nil {
		t.Fatalf("marshalling the fixture document: %v", err)
	}
	b := pipeline.RowBatch{
		Table: tbl("people"),
		Cols:  peopleCols,
		Rows:  [][]any{{int64(1), "a@example.com", "r", "n", string(raw), []any{}, "active", nil}},
		Last:  true,
	}
	out, err := New(fixture()).Transform(b, classification(), &k, NewResidual(100))
	if err != nil {
		t.Fatalf("Transform: %v", err)
	}
	text, ok := out.Rows[0][4].(string)
	if !ok {
		t.Fatalf("a document that arrived as text came back as %T", out.Rows[0][4])
	}
	if text == string(raw) {
		t.Fatal("the document survived masking unchanged")
	}
	var back map[string]any
	if err := json.Unmarshal([]byte(text), &back); err != nil {
		t.Fatalf("the masked document is not JSON: %v", err)
	}
	if len(leaves(t, back)) != len(leaves(t, document())) {
		t.Error("the masked document has a different number of leaves")
	}
}
