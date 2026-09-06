// SPDX-License-Identifier: Apache-2.0

package mask

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// The colliding pairs of ARCHITECTURE.md §5. They are fixtures rather than a
// comment because a concatenating encoder passes every other test in this
// file: it is only these pairs that tell the two encodings apart.
func TestEncodeSeparatesCollidingPairs(t *testing.T) {
	pairs := [][2][]string{
		{{"email", "a@b.com"}, {"emai", "la@b.com"}},
		{{"a", "b.c"}, {"a.b", "c"}},
		{{"public.users", "email"}, {"public", "users.email"}},
		{{"", "ab"}, {"a", "b"}},
	}
	for _, p := range pairs {
		left := Encode([]byte(p[0][0]), []byte(p[0][1]))
		right := Encode([]byte(p[1][0]), []byte(p[1][1]))
		if bytes.Equal(left, right) {
			t.Errorf("Encode%v and Encode%v are the same bytes", p[0], p[1])
		}
	}
}

func TestEncodeIsLengthPrefixed(t *testing.T) {
	got := Encode([]byte("ab"), []byte(""), []byte("xyz"))
	want := []byte{0, 0, 0, 2, 'a', 'b', 0, 0, 0, 0, 0, 0, 0, 3, 'x', 'y', 'z'}
	if !bytes.Equal(got, want) {
		t.Fatalf("Encode: got %v, want %v", got, want)
	}
	if n := binary.BigEndian.Uint32(got[:4]); n != 2 {
		t.Fatalf("first length prefix: got %d, want 2", n)
	}
}

// A digest separates two categories even for one value, because K_cat is
// derived from the category. Section 5 rests on it: a column that moves
// category gets a new mapping, which is why the move is announced.
func TestDigestSeparatesCategories(t *testing.T) {
	k := testKey(t)
	a, err := Digest(k, CatEmail, TagEmail, []byte("alice@example.org"))
	if err != nil {
		t.Fatal(err)
	}
	b, err := Digest(k, CatFreeText, TagEmail, []byte("alice@example.org"))
	if err != nil {
		t.Fatal(err)
	}
	if a == b {
		t.Fatal("two categories produced one digest")
	}
}

// A bigint and a text copy of one identifier reach one digest, which is the
// whole point of the type tag naming the canonical form and not the column's
// type (ARCHITECTURE.md §5; the scaffold review note on mask.Canonical).
func TestOneIdentifierInTwoColumnTypesHashesAlike(t *testing.T) {
	k := testKey(t)
	text := Constraints{TypeTag: famText}
	number := Constraints{TypeTag: famBigint}

	ct, tagT, err := Canonical(CatNationalID, Value{Text: "000123456"}, text)
	if err != nil {
		t.Fatal(err)
	}
	cn, tagN, err := Canonical(CatNationalID, Value{Text: "123456"}, number)
	if err != nil {
		t.Fatal(err)
	}
	if tagT != tagN {
		t.Fatalf("type tags differ: %q and %q", tagT, tagN)
	}
	dt, err := Digest(k, CatNationalID, tagT, ct.bytes())
	if err != nil {
		t.Fatal(err)
	}
	dn, err := Digest(k, CatNationalID, tagN, cn.bytes())
	if err != nil {
		t.Fatal(err)
	}
	if dt != dn {
		t.Fatal("one identifier in two column types produced two digests")
	}
}
