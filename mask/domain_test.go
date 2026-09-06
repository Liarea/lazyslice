// SPDX-License-Identifier: Apache-2.0

package mask

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"math"
	"testing"
)

// digestOf is the i-th of a run of independent digests, so that a test can
// walk a generator's output space without a key schedule.
func digestOf(i int) [32]byte {
	var b [8]byte
	binary.BigEndian.PutUint64(b[:], uint64(i))
	return sha256.Sum256(b[:])
}

// distinctOutputs masks n independent digests and counts the distinct results.
// It is how a Domain() is checked against the generator rather than against
// its author's intention: mask/CLAUDE.md forbids a Domain that reports more
// than the generator can emit, and only counting finds one that does.
func distinctOutputs(t *testing.T, id ID, in Value, c Constraints, n int) int {
	t.Helper()
	m, ok := Get(id)
	if !ok {
		t.Fatalf("no masker %q", id)
	}
	seen := map[string]struct{}{}
	var b [8]byte
	for i := 0; i < n; i++ {
		binary.BigEndian.PutUint64(b[:], uint64(i))
		out, err := m.Mask(sha256.Sum256(b[:]), in, c)
		if err != nil {
			t.Fatalf("%s: %v", id, err)
		}
		seen[rendered(out)] = struct{}{}
	}
	return len(seen)
}

// Every generator's Domain() is an upper bound on what it emits, and a tight
// one: a figure far above the truth would let the planner accept a unique
// column that then collides at load, which is the failure the refusal exists
// to prevent.
func TestDomainMatchesWhatTheGeneratorEmits(t *testing.T) {
	cases := []struct {
		name  string
		id    ID
		in    Value
		c     Constraints
		draws int
	}{
		{"phone", MaskerPhone, Value{Text: "+12125559999"}, Constraints{TypeTag: famText}, 400_000},
		{"ipv4", MaskerNetworkID, Value{Text: "10.0.0.1"}, Constraints{TypeTag: famInet}, 20_000},
		{"mac", MaskerNetworkID, Value{Text: "aa:bb:cc:dd:ee:ff"}, Constraints{TypeTag: famMacaddr}, 8_000},
		// A MAC-shaped value in a text column takes the MAC branch, whose 256
		// values are narrower than the IPv4 block's 768: the rule pack accepts
		// text, varchar and citext for network_id, so the column admits both.
		{"mac in text", MaskerNetworkID, Value{Text: "3c:22:fb:aa:bb:cc"},
			Constraints{TypeTag: famText}, 8_000},
		// The URL branch of online_id is one base32 symbol wide in a
		// varchar(25); the handle branch it used to report is billions.
		{"online_id url in a bounded varchar", MaskerOnlineID,
			Value{Text: "https://social.example/someone"},
			Constraints{TypeTag: famVarchar, MaxLen: 25}, 2_000},
		{"person_date", MaskerPersonDate, Value{Text: "1974-03-02"}, Constraints{TypeTag: famDate}, 400_000},
		{"geo two places", MaskerGeo, Value{Text: "1"}, Constraints{TypeTag: famVarchar, MaxLen: 6}, 300_000},
		{"name narrow", MaskerPersonName, Value{Text: "x"}, Constraints{TypeTag: famVarchar, MaxLen: 6}, 6_000},
		{"postcode", MaskerAddress, Value{Text: "x"}, Constraints{TypeTag: famVarchar, MaxLen: 3}, 600_000},
		{"special enum", MaskerSpecial, Value{Text: "x"},
			Constraints{TypeTag: famEnum, EnumLabels: []string{"a", "b", "c"}}, 500},
		{"credential", CredentialMasker, Value{Text: "x"}, Constraints{TypeTag: famText}, 100},
		{"null", MaskerNull, Value{Text: "x"}, Constraints{TypeTag: famBytea, Nullable: true}, 100},
	}
	for _, tc := range cases {
		m, _ := Get(tc.id)
		want := m.Domain(tc.c)
		got := int64(distinctOutputs(t, tc.id, tc.in, tc.c, tc.draws))
		if got > want {
			t.Errorf("%s: emitted %d distinct values, Domain says %d — Domain under-reports",
				tc.name, got, want)
		}
		// With draws ≫ domain, coverage should be near total. A Domain far
		// above what the generator reaches is the dangerous direction.
		if float64(got) < 0.95*float64(want) {
			t.Errorf("%s: emitted only %d of the %d values Domain claims", tc.name, got, want)
		}
	}
}

// The wide generators cannot be enumerated, so the claim checked here is the
// one section 5 makes about them: at least 2⁶⁴, and no collisions in a large
// sample.
func TestWideGeneratorsClearTheUniqueThreshold(t *testing.T) {
	cases := []struct {
		name string
		id   ID
		c    Constraints
	}{
		{"email unique", MaskerEmail, Constraints{TypeTag: famText, Unique: true}},
		{"ip_unique", MaskerIPUnique, Constraints{TypeTag: famInet}},
		{"online_id", MaskerOnlineID, Constraints{TypeTag: famText}},
	}
	for _, tc := range cases {
		m, _ := Get(tc.id)
		if d := m.Domain(tc.c); d < math.MaxInt64 {
			t.Errorf("%s: Domain is %d, section 5 asks for at least 2⁶⁴", tc.name, d)
		}
		const draws = 50_000
		if got := distinctOutputs(t, tc.id, Value{Text: "x"}, tc.c, draws); got != draws {
			t.Errorf("%s: %d distinct values in %d draws", tc.name, got, draws)
		}
	}
}

// A column too small for any masked value has a domain of 0 and refuses,
// rather than truncating a value into it — a truncated value keeps a prefix.
func TestATooSmallColumnHasNoDomainAndRefuses(t *testing.T) {
	cases := []struct {
		id ID
		c  Constraints
	}{
		{MaskerEmail, Constraints{TypeTag: famVarchar, MaxLen: 12}},
		{MaskerPersonName, Constraints{TypeTag: famVarchar, MaxLen: 2}},
		{MaskerPhone, Constraints{TypeTag: famVarchar, MaxLen: 9}},
		{MaskerGeo, Constraints{TypeTag: famVarchar, MaxLen: 3}},
		{MaskerIPUnique, Constraints{TypeTag: famVarchar, MaxLen: 20}},
		{MaskerPersonDate, Constraints{TypeTag: famVarchar, MaxLen: 8}},
	}
	for _, tc := range cases {
		m, _ := Get(tc.id)
		if d := m.Domain(tc.c); d != 0 {
			t.Errorf("%s on varchar(%d): Domain is %d, want 0", tc.id, tc.c.MaxLen, d)
		}
		if _, err := m.Mask([32]byte{}, Value{Text: "x"}, tc.c); !errors.Is(err, ErrNoRoom) {
			t.Errorf("%s on varchar(%d): got %v, want ErrNoRoom", tc.id, tc.c.MaxLen, err)
		}
	}
}

// ColumnDomain is the other half of the admissible domain: what the column
// itself can hold, whatever the generator could produce.
func TestColumnDomain(t *testing.T) {
	cases := []struct {
		name string
		c    Constraints
		want int64
	}{
		{"enum", Constraints{TypeTag: famEnum, EnumLabels: []string{"a", "b"}}, 2},
		{"boolean", Constraints{TypeTag: famBoolean}, 2},
		{"check list", Constraints{TypeTag: famText,
			Checks: []string{"CHECK ((status = ANY (ARRAY['new'::text, 'old'::text, 'gone'::text])))"}}, 3},
		{"check in", Constraints{TypeTag: famText, Checks: []string{"CHECK ((kind IN ('a', 'b')))"}}, 2},
		{"varchar(1)", Constraints{TypeTag: famVarchar, MaxLen: 1}, 95},
		{"integer", Constraints{TypeTag: famInteger}, 1 << 32},
	}
	for _, tc := range cases {
		if got := ColumnDomain(tc.c); got != tc.want {
			t.Errorf("%s: ColumnDomain = %d, want %d", tc.name, got, tc.want)
		}
	}
}

// A masked enum emits a member label, because an arbitrary string fails the
// load with 22P02.
func TestAMaskedEnumEmitsAMemberLabel(t *testing.T) {
	c := Constraints{TypeTag: famEnum, EnumLabels: []string{"single", "married", "widowed"}}
	m, _ := Get(MaskerSpecial)
	for i := 0; i < 200; i++ {
		var b [8]byte
		binary.BigEndian.PutUint64(b[:], uint64(i))
		out, err := m.Mask(sha256.Sum256(b[:]), Value{Text: "married"}, c)
		if err != nil {
			t.Fatal(err)
		}
		if out.Text != "single" && out.Text != "married" && out.Text != "widowed" {
			t.Fatalf("a masked enum emitted %q", out.Text)
		}
	}
}

func TestRequiredAndMaxRowsInvertEachOther(t *testing.T) {
	// ε = 10⁻⁶: d_required = n² / 2ε = n² × 500,000.
	if got := Required(500); got != 500*500*500_000 {
		t.Fatalf("Required(500) = %d", got)
	}
	if Required(0) != 0 {
		t.Fatal("Required(0) should be 0")
	}
	if got := Required(math.MaxInt64); got != math.MaxInt64 {
		t.Fatalf("Required saturates rather than wrapping: got %d", got)
	}
	for _, n := range []int64{1, 10, 500, 10_000} {
		if got := MaxRows(Required(n)); got != n {
			t.Errorf("MaxRows(Required(%d)) = %d", n, got)
		}
	}
}

// Section 5's other domain rule, the one that runs on every masked column:
// below twice the distinct sampled values, masking is a substitution that
// frequency recovers, and the run says so rather than refusing.
func TestSmallDomainIsReported(t *testing.T) {
	small := Constraints{TypeTag: famEnum,
		EnumLabels: []string{"a", "b", "c", "d", "e", "f", "g", "h"}, Distinct: 8}
	if !Small(MaskerSpecial, small) {
		t.Fatal("eight admissible values against eight distinct samples is a small domain")
	}
	wide := Constraints{TypeTag: famText, MaxLen: 100, Distinct: 200}
	if Small(MaskerFreeText, wide) {
		t.Fatal("a free-text column is not a small domain")
	}
	// With no samples the rule falls back to the domain itself. Reporting
	// nothing would leave a one-label column masked, listed nowhere, and
	// silently substituted — the direction that fails open.
	if !Small(MaskerSpecial, Constraints{TypeTag: famEnum, EnumLabels: []string{"a"}}) {
		t.Fatal("a one-label column is a small domain whether or not anybody counted its values")
	}
	if Small(MaskerFreeText, Constraints{TypeTag: famText, MaxLen: 100}) {
		t.Fatal("an unsampled free-text column is not a small domain")
	}
}

// A special category over a small domain is not substituted at all: section 5
// says substitution over eight values is not a mask, so the column collapses.
func TestSpecialCategoryCollapsesOverASmallDomain(t *testing.T) {
	c := Constraints{TypeTag: famEnum,
		EnumLabels: []string{"single", "married", "widowed"}, Distinct: 3}
	m, _ := Get(MaskerSpecial)
	if d := m.Domain(c); d != 1 {
		t.Fatalf("a collapsed column has one value, Domain says %d", d)
	}
	for i := 0; i < 50; i++ {
		var b [8]byte
		binary.BigEndian.PutUint64(b[:], uint64(i))
		out, err := m.Mask(sha256.Sum256(b[:]), Value{Text: "married"}, c)
		if err != nil {
			t.Fatal(err)
		}
		if out.Text != "single" {
			t.Fatalf("collapse should be the first label, got %q", out.Text)
		}
	}
	nullable := Constraints{TypeTag: famText, Nullable: true, MaxLen: 2, Distinct: 200}
	out, err := m.Mask([32]byte{}, Value{Text: "x"}, nullable)
	if err != nil {
		t.Fatal(err)
	}
	if !out.Null {
		t.Fatalf("a nullable column with no labels collapses to NULL, got %+v", out)
	}
}

// The collapse must not depend on a field the caller may not have filled in.
// Constraints.Distinct is 0 when nobody counted, which section 4 makes a normal
// path — "special categories by name alone → certain", a partitioned table with
// no leaves — and a control that switches itself off at a zero value is a
// column of real diagnoses under a report that calls it masked.
func TestSpecialCategoryCollapsesWhenDistinctIsUnknown(t *testing.T) {
	c := Constraints{TypeTag: famEnum, EnumLabels: []string{"hiv_positive", "hiv_negative"}}
	m, _ := Get(MaskerSpecial)
	if d := m.Domain(c); d != 1 {
		t.Fatalf("Domain = %d, want 1: an unsampled two-label special category collapses", d)
	}
	for i := 0; i < 100; i++ {
		out, err := m.Mask(digestOf(i), Value{Text: "hiv_positive"}, c)
		if err != nil {
			t.Fatal(err)
		}
		if out.Text != "hiv_positive" {
			t.Fatalf("substitution over two labels is not a mask: got %q", out.Text)
		}
	}
	if !Small(MaskerSpecial, c) {
		t.Error("a collapsed column must still be listed under small_domain:")
	}
}

// ARCHITECTURE.md section 5 "preserve what the application checks": an enum or
// a CHECK value list closes the column whatever category it was classified
// under, and a value outside the set fails the load with 22P02 or 23514. It is
// not the special_category masker's private rule.
func TestAClosedColumnOnlyEverGetsOneOfItsLabels(t *testing.T) {
	country := Constraints{TypeTag: famText,
		Checks: []string{"CHECK ((country = ANY (ARRAY['GB'::text, 'US'::text])))"}}
	enum := Constraints{TypeTag: famEnum, EnumLabels: []string{"alpha", "beta", "gamma"}}
	cases := []struct {
		id ID
		c  Constraints
	}{
		{MaskerAddress, country}, {MaskerEmail, country}, {MaskerFreeText, country},
		{MaskerPersonName, enum}, {MaskerOnlineID, enum}, {MaskerPhone, enum},
		{MaskerPhoneUnique, enum}, {MaskerNationalID, enum}, {MaskerFinancial, enum},
		{MaskerNetworkID, enum}, {MaskerIPUnique, enum}, {MaskerPersonDate, enum},
		{MaskerGeo, enum}, {MaskerSemiStruct, enum}, {MaskerSpecial, enum},
	}
	for _, tc := range cases {
		m, _ := Get(tc.id)
		allowed := labels(tc.c)
		if got := m.Domain(tc.c); got > int64(len(allowed)) {
			t.Errorf("%s: Domain = %d, the column holds %d labels", tc.id, got, len(allowed))
		}
		for i := 0; i < 100; i++ {
			out, err := m.Mask(digestOf(i), Value{Text: "original"}, tc.c)
			if err != nil {
				t.Fatalf("%s: %v", tc.id, err)
			}
			member := false
			for _, l := range allowed {
				if out.Text == l {
					member = true
				}
			}
			if !member {
				t.Fatalf("%s emitted %q, which is not one of %v", tc.id, out.Text, allowed)
			}
		}
	}
}
