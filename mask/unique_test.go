// SPDX-License-Identifier: Apache-2.0

package mask

import (
	"errors"
	"fmt"
	"strconv"
	"testing"
)

// maskAll runs n distinct source values through one masker and returns how
// many distinct fakes came back.
func maskAll(t *testing.T, id ID, cat Category, c Constraints, n int, value func(int) string) int {
	t.Helper()
	k := testKey(t)
	seen := map[string]struct{}{}
	for i := 0; i < n; i++ {
		r, err := Apply(k, cat, id, Value{Text: value(i)}, c)
		if err != nil {
			t.Fatalf("%s: %v", id, err)
		}
		seen[rendered(r.Out)] = struct{}{}
	}
	return len(seen)
}

// The unique-index rule, from the side that matters: a column the planner
// accepted must not collide at load. Fifty thousand distinct addresses through
// the unique email generator produce fifty thousand distinct fakes.
func TestNoCollisionsUnderUniquenessPressure(t *testing.T) {
	const n = 50_000
	c := Constraints{TypeTag: famVarchar, MaxLen: 100, Unique: true, Rows: n}
	if _, err := Pick(CatEmail, c); err != nil {
		t.Fatalf("the planner would not have accepted this column: %v", err)
	}
	got := maskAll(t, MaskerEmail, CatEmail, c, n, func(i int) string {
		return fmt.Sprintf("person%d@corp.example", i)
	})
	if got != n {
		t.Fatalf("%d distinct fakes from %d distinct addresses: %d collisions", got, n, n-got)
	}

	// A unique phone column carries far fewer rows: phone_unique has a domain
	// of 10¹², so d_required caps it at MaxRows(10¹²) = 1,414 rows at ε = 10⁻⁶.
	const phoneRows = 1_000
	phone := Constraints{TypeTag: famVarchar, MaxLen: 15, Unique: true, Rows: phoneRows}
	id, err := Pick(CatPhone, phone)
	if err != nil {
		t.Fatalf("a unique varchar(15) phone column at %d rows: %v", phoneRows, err)
	}
	if id != MaskerPhoneUnique {
		t.Fatalf("uniqueness should have chosen %s, got %s", MaskerPhoneUnique, id)
	}
	if got := maskAll(t, id, CatPhone, phone, phoneRows, func(i int) string {
		return fmt.Sprintf("+1212555%04d", i)
	}); got != phoneRows {
		t.Fatalf("%d distinct fakes from %d numbers", got, phoneRows)
	}
}

// The refusal is not theatre: the default phone generator, whose domain is
// about 4 × 10⁴, really does collide well before the row counts the rule
// refuses it at. This is the test that would fail if Domain() were fiction.
func TestTheDefaultGeneratorCollidesWhenTheRuleSaysItWould(t *testing.T) {
	const n = 5_000
	c := Constraints{TypeTag: famText}
	got := maskAll(t, MaskerPhone, CatPhone, c, n, func(i int) string {
		return fmt.Sprintf("+1212555%04d", i)
	})
	if got == n {
		t.Fatal("the default phone generator produced no collisions in 5,000 rows; " +
			"either its domain grew or the test no longer pressures it")
	}
}

// ARCHITECTURE.md §5's worked example, both halves: a unique phone column is
// carried by phone_unique when the row count allows, and refused by name when
// it does not, with d, d_required and the largest row count that would fit.
func TestUniquePhoneColumnIsCarriedOrRefusedByName(t *testing.T) {
	ok := Constraints{TypeTag: famVarchar, MaxLen: 15, Unique: true, Rows: 500}
	if id, err := Pick(CatPhone, ok); err != nil || id != MaskerPhoneUnique {
		t.Fatalf("Pick = %q, %v; want %s", id, err, MaskerPhoneUnique)
	}

	tooMany := Constraints{TypeTag: famVarchar, MaxLen: 15, Unique: true, Rows: 2_000_000}
	_, err := Pick(CatPhone, tooMany)
	var de *DomainError
	if !errors.As(err, &de) {
		t.Fatalf("Pick = %v, want a *DomainError", err)
	}
	if de.Domain != 1_000_000_000_000 {
		t.Errorf("d = %d, want 10¹²", de.Domain)
	}
	if de.Required != Required(2_000_000) {
		t.Errorf("d_required = %d, want %d", de.Required, Required(2_000_000))
	}
	if de.MaxRows != 1_414 {
		t.Errorf("MaxRows = %d, want 1414 (√(10¹²/500000))", de.MaxRows)
	}
	if de.ID != MaskerPhoneUnique || de.Category != CatPhone {
		t.Errorf("the refusal must name the widest generator it tried: %+v", de)
	}
}

// Pick chooses within the category, and only for a unique column.
func TestPickChoosesTheAlternateOnlyForAUniqueColumn(t *testing.T) {
	inet := Constraints{TypeTag: famInet}
	if id, err := Pick(CatNetworkID, inet); err != nil || id != MaskerNetworkID {
		t.Fatalf("a plain inet column: Pick = %q, %v", id, err)
	}
	inet.Unique, inet.Rows = true, 1_000_000
	if id, err := Pick(CatNetworkID, inet); err != nil || id != MaskerIPUnique {
		t.Fatalf("a unique inet column: Pick = %q, %v; want %s", id, err, MaskerIPUnique)
	}

	// email needs no alternate: the generator itself widens under a unique
	// index, which is why lazyslice.yml still says "masker: email".
	text := Constraints{TypeTag: famText, Unique: true, Rows: 1_000_000}
	if id, err := Pick(CatEmail, text); err != nil || id != MaskerEmail {
		t.Fatalf("a unique email column: Pick = %q, %v", id, err)
	}

	// credential: the fixed literal is the default and stays it, and the
	// alternate is reached only under a unique index (T-0098).
	token := Constraints{TypeTag: famVarchar, MaxLen: 255}
	if id, err := Pick(CatCredential, token); err != nil || id != CredentialMasker {
		t.Fatalf("a plain credential column: Pick = %q, %v; want %s", id, err, CredentialMasker)
	}
	token.Unique, token.Rows = true, 1_000_000
	if id, err := Pick(CatCredential, token); err != nil || id != MaskerCredentialUnique {
		t.Fatalf("a unique credential column: Pick = %q, %v; want %s", id, err, MaskerCredentialUnique)
	}
}

// T-0098, the whole of it: before the second generator, every unique credential
// column was refused at exit 12 whatever the row count — MaxRows(1) is zero, so
// the refusal could not even name a --take that would work, and the operator's
// only escapes were --unmask (which copies the credential verbatim) and
// mapping_file:. Eighteen of the thirty-seven --unmask flags the ten torture
// schemas carry are tagged (T-0098) and were this; they are still in the
// catalogue until tracker T-0112 strips them and re-runs `make torture`.
func TestAUniqueCredentialColumnIsCarriedRatherThanRefused(t *testing.T) {
	// auth.refresh_tokens.token: varchar(255) under a unique index.
	c := Constraints{TypeTag: famVarchar, MaxLen: 255, Unique: true, Rows: 500}
	id, err := Pick(CatCredential, c)
	if err != nil {
		t.Fatalf("a unique credential column is still refused: %v", err)
	}
	if id != MaskerCredentialUnique {
		t.Fatalf("Pick = %q, want %s", id, MaskerCredentialUnique)
	}
	if d := Admissible(id, c); d < Required(c.Rows) {
		t.Fatalf("d = %d, d_required = %d", d, Required(c.Rows))
	}

	// And the values really are distinct: the refusal exists to stop a
	// collision at load, so the generator has to carry the rows the rule
	// admits.
	const n = 20_000
	pressure := Constraints{TypeTag: famVarchar, MaxLen: 255, Unique: true, Rows: n}
	if _, err := Pick(CatCredential, pressure); err != nil {
		t.Fatalf("the planner would not have accepted this column: %v", err)
	}
	got := maskAll(t, MaskerCredentialUnique, CatCredential, pressure, n, func(i int) string {
		return fmt.Sprintf("token-%d-%s", i, strconv.Itoa(i*7919))
	})
	if got != n {
		t.Fatalf("%d distinct fakes from %d distinct tokens: %d collisions", got, n, n-got)
	}
}

// The default is unchanged and it has to be: ARCHITECTURE.md §5 says a
// credential becomes a fixed unusable value, and a password column full of
// well-formed-looking tokens is a column somebody tries to crack. The alternate
// is not plausible either — every value it emits announces the tool — but it is
// the wider one, so only a unique column may reach it.
func TestANonUniqueCredentialColumnStillGetsTheFixedLiteral(t *testing.T) {
	c := Constraints{TypeTag: famText}
	id, err := Pick(CatCredential, c)
	if err != nil {
		t.Fatal(err)
	}
	if id != CredentialMasker {
		t.Fatalf("Pick = %q, want %s", id, CredentialMasker)
	}
	r, err := Apply(testKey(t), CatCredential, id, Value{Text: "$2y$10$abcdefghijklmnop"}, c)
	if err != nil {
		t.Fatal(err)
	}
	if r.Out.Text != CredentialLiteral {
		t.Fatalf("got %q, want %q", r.Out.Text, CredentialLiteral)
	}
}

func TestPickRefusesAColumnNoGeneratorFits(t *testing.T) {
	// A column too narrow for any masked value is not a d_required refusal:
	// no row count makes it fit, so it carries no d_required and no MaxRows to
	// print, and printing "0 rows need 0" against a table of 500 would be a
	// refusal the planner cannot render honestly.
	var nre *NoRoomError
	_, err := Pick(CatEmail, Constraints{TypeTag: famVarchar, MaxLen: 8, Rows: 500})
	if !errors.As(err, &nre) {
		t.Fatalf("a varchar(8) email column: %v, want a *NoRoomError", err)
	}
	if !errors.Is(err, ErrNoRoom) {
		t.Errorf("a NoRoomError should answer errors.Is(err, ErrNoRoom): %v", err)
	}
	if nre.ID != MaskerEmail || nre.Category != CatEmail || nre.MaxLen != 8 {
		t.Errorf("the refusal must name the column it could not fit: %+v", nre)
	}
	if _, err := Pick("no_such_category", Constraints{}); !errors.Is(err, ErrNoCategory) {
		t.Fatalf("%v, want ErrNoCategory", err)
	}
	if _, err := Apply(testKey(t), CatEmail, "no_such_masker", Value{Text: "x"},
		Constraints{TypeTag: famText}); !errors.Is(err, ErrUnknownMasker) {
		t.Fatalf("%v, want ErrUnknownMasker", err)
	}
}

// n is the whole of d_required, so a unique column whose row count nobody
// supplied cannot be compared against anything. Accepting it would let a
// generator of any width carry a unique column — a credential column resolves
// to fixed:$lazyslice$invalid, domain 1, and every row after the first
// collides — which is the unique violation at load that section 5 exists to
// move to plan time.
func TestAUniqueColumnWithNoRowCountIsRefused(t *testing.T) {
	c := Constraints{TypeTag: famVarchar, MaxLen: 15, Unique: true}
	if id, err := Pick(CatPhone, c); !errors.Is(err, ErrRowCountUnknown) {
		t.Fatalf("Pick = %q, %v; want ErrRowCountUnknown", id, err)
	}
	if id, err := Pick(CatCredential, c); !errors.Is(err, ErrRowCountUnknown) {
		t.Fatalf("a unique credential column: Pick = %q, %v", id, err)
	}
	c.Rows = 500
	if id, err := Pick(CatPhone, c); err != nil || id != MaskerPhoneUnique {
		t.Fatalf("the same column with a row count: Pick = %q, %v", id, err)
	}
}

// A unique column no generator fits at all must refuse by name — a
// *NoRoomError — whatever the row count says, because no row count would ever
// make it fit. Rows is left unset here (ErrRowCountUnknown's own trigger) to
// pin that the "no generator fits" refusal is checked first: comparing bestD
// against Required(0) (which is 0) would otherwise let a column with no
// admissible generator at all pass silently, and printing "0 rows need 0"
// would not even be the honest refusal either.
func TestPickOnAUniqueColumnRefusesNoFitRegardlessOfRowCount(t *testing.T) {
	var nre *NoRoomError
	c := Constraints{TypeTag: famVarchar, MaxLen: 8, Unique: true}
	_, err := Pick(CatEmail, c)
	if !errors.As(err, &nre) {
		t.Fatalf("a unique varchar(8) email column with no row count: %v, want a *NoRoomError", err)
	}
	if errors.Is(err, ErrRowCountUnknown) {
		t.Fatalf("a column no generator fits must not be reported as an unknown row count: %v", err)
	}
}

// A phone column typed integer is a shape the rule pack accepts, and eleven
// digits do not fit one: the number the generator would emit is above int4's
// maximum and the load fails with 22003. The refusal happens at plan instead.
func TestAPhoneNeverOverflowsAnIntegerColumn(t *testing.T) {
	var nre *NoRoomError
	if _, err := Pick(CatPhone, Constraints{TypeTag: famInteger, Rows: 500}); !errors.As(err, &nre) {
		t.Fatalf("a phone integer column: %v, want a *NoRoomError", err)
	}

	// Under a unique index phone_unique fits itself to the column instead, so
	// what it emits must still be an int4.
	unique := Constraints{TypeTag: famInteger, Unique: true, Rows: 10}
	id, err := Pick(CatPhone, unique)
	if err != nil || id != MaskerPhoneUnique {
		t.Fatalf("a unique phone integer column: Pick = %q, %v", id, err)
	}
	m, _ := Get(id)
	for i := 0; i < 500; i++ {
		out, err := m.Mask(digestOf(i), Value{Text: "12125559999"}, unique)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := strconv.ParseInt(out.Text, 10, 32); err != nil {
			t.Fatalf("%q does not fit an integer column: %v", out.Text, err)
		}
	}

	// A bigint column is wide enough for both, and neither shape changed.
	if got := phoneShapeFor(Constraints{TypeTag: famBigint}); got != phoneDigits {
		t.Errorf("a bigint phone column: shape %v, want phoneDigits", got)
	}
	if d, _ := uniquePhoneShape(Constraints{TypeTag: famBigint}); d != uniquePhoneDigits {
		t.Errorf("a bigint phone_unique column: %d digits, want %d", d, uniquePhoneDigits)
	}
}

// The URL branch of online_id is the narrow one in a bounded column, and Pick
// must refuse on it rather than on the handle branch's billions.
func TestAUniqueBoundedOnlineIDIsRefusedOnItsNarrowestBranch(t *testing.T) {
	c := Constraints{TypeTag: famVarchar, MaxLen: 25, Unique: true, Rows: 100_000}
	var de *DomainError
	_, err := Pick(CatOnlineID, c)
	if !errors.As(err, &de) {
		t.Fatalf("a unique varchar(25) online_id column: %v, want a *DomainError", err)
	}
	if de.Domain != 32 {
		t.Errorf("d = %d; the URL branch of a varchar(25) has one base32 symbol left", de.Domain)
	}
}

// Every category the rule pack can produce has a masker, and every masker
// resolves. ADR-006 makes a category with no shipped masker impossible.
func TestEveryCategoryHasAMasker(t *testing.T) {
	for _, cat := range []Category{
		CatPersonName, CatEmail, CatPhone, CatAddress, CatGeo, CatPersonDate,
		CatNationalID, CatFinancial, CatNetworkID, CatOnlineID, CatCredential,
		CatFreeText, CatSpecial, CatBinary, CatSemiStruct,
	} {
		ids := Candidates(cat)
		if len(ids) == 0 {
			t.Errorf("category %s has no masker", cat)
			continue
		}
		for _, id := range ids {
			if _, ok := Get(id); !ok {
				t.Errorf("category %s names %q, which does not resolve", cat, id)
			}
		}
	}
	if _, ok := Get(FixedPrefix + "{}"); !ok {
		t.Error("the fixed: prefix should resolve any literal")
	}
	if _, ok := Get("nope"); ok {
		t.Error("an unregistered id resolved")
	}
}
