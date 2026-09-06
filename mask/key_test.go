// SPDX-License-Identifier: Apache-2.0

package mask

import (
	"encoding/hex"
	"fmt"
	"strings"
	"testing"
)

const testKeyHex = "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff"

func testKey(t *testing.T) Key {
	t.Helper()
	k, err := ParseKey(testKeyHex)
	if err != nil {
		t.Fatal(err)
	}
	return k
}

func otherKey(t *testing.T) Key {
	t.Helper()
	k, err := ParseKey(strings.Repeat("a1", KeyLen))
	if err != nil {
		t.Fatal(err)
	}
	return k
}

func TestNewKeyIsThirtyTwoRandomBytes(t *testing.T) {
	a, err := NewKey()
	if err != nil {
		t.Fatal(err)
	}
	b, err := NewKey()
	if err != nil {
		t.Fatal(err)
	}
	if a == b {
		t.Fatal("two run keys came back equal")
	}
	if len(a) != KeyLen {
		t.Fatalf("key length %d, want %d", len(a), KeyLen)
	}
}

func TestParseKeyRoundTripsAndRefusesTheRest(t *testing.T) {
	k := testKey(t)
	if got := hex.EncodeToString(k[:]); got != testKeyHex {
		t.Fatalf("round trip: got %s", got)
	}
	if _, err := ParseKey("  " + testKeyHex + "\n"); err != nil {
		t.Fatalf("surrounding whitespace should be ignored: %v", err)
	}
	for _, bad := range []string{"", "abcd", strings.Repeat("f", 63), "zz" + testKeyHex[2:]} {
		if _, err := ParseKey(bad); err == nil {
			t.Errorf("ParseKey(%q) was accepted", bad)
		}
	}
}

func TestFingerprintIsEightHexCharacters(t *testing.T) {
	fp := testKey(t).Fingerprint()
	if len(fp) != 8 {
		t.Fatalf("fingerprint %q is %d characters, want 8", fp, len(fp))
	}
	if fp == otherKey(t).Fingerprint() {
		t.Fatal("two keys share a fingerprint")
	}
	if strings.Contains(testKeyHex, fp) {
		t.Fatal("the fingerprint is a slice of the key itself")
	}
}

// A Key caught by a %v must not print the key. THREAT_MODEL.md A4 keeps K out
// of git, the yml, the target and events; a stray format verb is the way it
// would get into all four at once.
func TestKeyStringDoesNotCarryTheKey(t *testing.T) {
	k := testKey(t)
	s := fmt.Sprintf("%v %s", k, k)
	if strings.Contains(s, "00112233") {
		t.Fatalf("Key.String leaked key material: %s", s)
	}
	if !strings.Contains(s, k.Fingerprint()) {
		t.Fatalf("Key.String should carry the fingerprint: %s", s)
	}
}
