// SPDX-License-Identifier: Apache-2.0

package mask

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func applyOrFail(t *testing.T, k Key, tc tcase) Result {
	t.Helper()
	r, err := Apply(k, tc.cat, tc.id, Value{Text: tc.in}, tc.c)
	if err != nil {
		t.Fatalf("%s: %v", tc.name, err)
	}
	return r
}

func rendered(v Value) string {
	if v.Null {
		return "<null>"
	}
	if len(v.Bytes) > 0 {
		return fmt.Sprintf("%x", v.Bytes)
	}
	return v.Text
}

// transcript is the whole corpus masked under one key, one line per case. Two
// transcripts are equal exactly when the mapping is.
func transcript(t *testing.T, k Key) []string {
	t.Helper()
	out := make([]string, 0, len(corpus()))
	for _, tc := range corpus() {
		out = append(out, tc.name+"\t"+rendered(applyOrFail(t, k, tc).Out))
	}
	return out
}

func TestSameKeySameValueSameFake(t *testing.T) {
	k := testKey(t)
	first := transcript(t, k)
	for i := 0; i < 5; i++ {
		if got := transcript(t, k); !equalLines(got, first) {
			t.Fatal("the same key and the same values produced two mappings")
		}
	}
}

func TestDifferentKeyDifferentFake(t *testing.T) {
	a := transcript(t, testKey(t))
	b := transcript(t, otherKey(t))
	same := 0
	for i := range a {
		if a[i] == b[i] {
			same++
			t.Logf("unchanged under a different key: %s", a[i])
		}
	}
	// The collapsing maskers — a credential's fixed literal, a NULLed
	// photograph, a collapsed special category, an emptied tsvector — have a
	// domain of 1 and are meant to be key-independent. Everything else must
	// move.
	if same > 4 {
		t.Fatalf("%d of %d values were the same under two keys", same, len(a))
	}
	for _, tc := range corpus() {
		if tc.id == CredentialMasker || tc.id == MaskerNull || tc.id == MaskerDerivedText ||
			tc.name == "special enum" {
			continue
		}
		x := rendered(applyOrFail(t, testKey(t), tc).Out)
		y := rendered(applyOrFail(t, otherKey(t), tc).Out)
		if x == y {
			t.Errorf("%s: one value under two keys gave %q both times", tc.name, x)
		}
	}
}

// childEnv turns this test binary into the second process. The claim under
// test is that the mapping is a function of the key and the value alone: not
// of a global seed, a clock, an address, a map iteration order or anything
// else that differs between two runs of the same program.
const childEnv = "LAZYSLICE_MASK_TRANSCRIPT_CHILD"

const (
	beginMarker = "--- transcript begin"
	endMarker   = "--- transcript end"
)

func TestSameKeySameFakeAcrossProcesses(t *testing.T) {
	mine := transcript(t, testKey(t))

	if os.Getenv(childEnv) != "" {
		fmt.Println(beginMarker)
		for _, line := range mine {
			fmt.Println(line)
		}
		fmt.Println(endMarker)
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=^TestSameKeySameFakeAcrossProcesses$", "-test.v")
	cmd.Env = append(os.Environ(), childEnv+"=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("child run failed: %v\n%s", err, out)
	}
	theirs := parseTranscript(t, string(out))
	if len(theirs) == 0 {
		t.Fatalf("child produced no transcript:\n%s", out)
	}
	if !equalLines(mine, theirs) {
		t.Fatalf("the mapping differs between two processes:\nparent: %v\nchild:  %v", mine, theirs)
	}
}

func parseTranscript(t *testing.T, out string) []string {
	t.Helper()
	var lines []string
	in := false
	sc := bufio.NewScanner(strings.NewReader(out))
	sc.Buffer(make([]byte, 0, 1<<20), 1<<20)
	for sc.Scan() {
		switch line := sc.Text(); {
		case strings.HasPrefix(line, beginMarker):
			in = true
		case strings.HasPrefix(line, endMarker):
			in = false
		default:
			if in {
				lines = append(lines, line)
			}
		}
	}
	return lines
}

func equalLines(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// Two spellings of one value must meet, or a join across two tables breaks.
func TestCanonicalisationJoinsTwoSpellings(t *testing.T) {
	k := testKey(t)
	pairs := []struct {
		cat  Category
		id   ID
		a, b string
		c    Constraints
	}{
		{CatEmail, MaskerEmail, "Alice@Example.COM", " alice@example.com ", Constraints{TypeTag: famText}},
		{CatPersonName, MaskerPersonName, "Ada  Lovelace", "ada lovelace", Constraints{TypeTag: famText}},
		{CatNationalID, MaskerNationalID, "000123456", "123456", Constraints{TypeTag: famText}},
		{CatPhone, MaskerPhone, "+1 212-555-9999", "+12125559999", Constraints{TypeTag: famText}},
		{CatNetworkID, MaskerNetworkID, "2001:0DB8:0000::1", "2001:db8::1", Constraints{TypeTag: famInet}},
	}
	for _, p := range pairs {
		x, err := Apply(k, p.cat, p.id, Value{Text: p.a}, p.c)
		if err != nil {
			t.Fatal(err)
		}
		y, err := Apply(k, p.cat, p.id, Value{Text: p.b}, p.c)
		if err != nil {
			t.Fatal(err)
		}
		if rendered(x.Out) != rendered(y.Out) {
			t.Errorf("%s: %q and %q masked to %q and %q", p.cat, p.a, p.b,
				rendered(x.Out), rendered(y.Out))
		}
	}
}
