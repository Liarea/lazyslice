// SPDX-License-Identifier: Apache-2.0

package mask

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

// The guards Apply puts around a generator: the post-condition of
// THREAT_MODEL.md T12 and the recover of T7. They are exercised through
// maskCell rather than through Apply because a test masker registered under
// Register would join the registry every other test in this package walks
// (corpus_test.go's TestCorpusCoversTheRegistry is one row per registered
// masker), and a guard test has no business changing what the corpus covers.

const victim = "victim.canary@bigcorp.example"

// funcMasker is a generator written inline by a test: the shape a third party
// can register against this module, which is what the guards are for.
type funcMasker struct {
	mask   func(h [32]byte, in Value, c Constraints) (Value, error)
	domain int64
}

func (f funcMasker) Mask(h [32]byte, in Value, c Constraints) (Value, error) {
	return f.mask(h, in, c)
}

func (f funcMasker) Domain(Constraints) int64 { return f.domain }

// guarded runs one masker through the same path Apply does.
func guarded(t *testing.T, m Masker, cat Category, in Value, c Constraints) (Value, error) {
	t.Helper()
	k := testKey(t)
	canon, tag, err := Canonical(cat, in, c)
	if err != nil {
		t.Fatalf("Canonical: %v", err)
	}
	h, err := Digest(k, cat, tag, canon.bytes())
	if err != nil {
		t.Fatalf("Digest: %v", err)
	}
	return maskCell(m, cat, "rt_probe", h, in, canon, c)
}

func TestApplyRefusesAMaskerWhoseOutputFollowsItsInput(t *testing.T) {
	cases := []struct {
		name string
		mask func(h [32]byte, in Value, c Constraints) (Value, error)
	}{
		{
			"verbatim",
			func(_ [32]byte, in Value, _ Constraints) (Value, error) { return in, nil },
		},
		{
			"upper-cased, which is the same value",
			func(_ [32]byte, in Value, _ Constraints) (Value, error) {
				return Value{Text: strings.ToUpper(in.Text)}, nil
			},
		},
		{
			"re-punctuated, which is the same value",
			func(_ [32]byte, in Value, _ Constraints) (Value, error) {
				return Value{Text: strings.ReplaceAll(in.Text, ".", " . ")}, nil
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := funcMasker{mask: tc.mask, domain: 1 << 40}
			_, err := guarded(t, m, CatEmail, Value{Text: victim}, Constraints{TypeTag: "text"})
			if !errors.Is(err, ErrPassthrough) {
				t.Fatalf("err = %v, want ErrPassthrough", err)
			}
			if strings.Contains(strings.ToLower(err.Error()), "victim") {
				t.Errorf("the refusal quotes the value: %q", err)
			}
			if !strings.Contains(err.Error(), "rt_probe") {
				t.Errorf("the refusal does not name the masker: %q", err)
			}
		})
	}
}

// A generator that ignores its input and lands on it anyway is a coincidence,
// not a passthrough: at a domain of d it happens once in d distinct source
// values, and refusing it would abort a run over an ordinary short-id or
// birthdate column with a message about a masker bug that is not there.
func TestApplyAllowsAMaskedValueThatCoincidesWithItsSource(t *testing.T) {
	m := funcMasker{
		mask:   func([32]byte, Value, Constraints) (Value, error) { return Value{Text: victim}, nil },
		domain: 1 << 40,
	}
	out, err := guarded(t, m, CatEmail, Value{Text: victim}, Constraints{TypeTag: "text"})
	if err != nil {
		t.Fatalf("maskCell = %v, want no error", err)
	}
	if out.Text != victim {
		t.Fatalf("out = %q, want %q", out.Text, victim)
	}
}

// The Domain a masker declares is the masker's own answer, so the guard does
// not ask it: a hostile generator that claims one output and hands its input
// back is refused like any other.
func TestApplyRefusesAPassthroughThatClaimsASingleOutput(t *testing.T) {
	m := funcMasker{
		mask:   func(_ [32]byte, in Value, _ Constraints) (Value, error) { return in, nil },
		domain: 1,
	}
	_, err := guarded(t, m, CatEmail, Value{Text: victim}, Constraints{TypeTag: "text"})
	if !errors.Is(err, ErrPassthrough) {
		t.Fatalf("err = %v, want ErrPassthrough", err)
	}
}

func TestApplyAllowsTheTwoPathsThatMayReturnAMember(t *testing.T) {
	cases := []struct {
		name string
		in   Value
		c    Constraints
		m    Masker
	}{
		{
			// A closed column: every generator emits a label through
			// labelValue, and one row in it holds the label it already had.
			name: "a masked enum emits a member label",
			in:   Value{Text: "shipped"},
			c:    Constraints{TypeTag: "enum", EnumLabels: []string{"pending", "shipped"}},
			m: funcMasker{
				mask:   func([32]byte, Value, Constraints) (Value, error) { return Value{Text: "shipped"}, nil },
				domain: 2,
			},
		},
		{
			// A generator with one output under this column: the collapsed
			// special_category, null's zero value, fixed's literal.
			name: "a one-output generator emits its constant",
			in:   Value{Text: "0"},
			c:    Constraints{TypeTag: "integer"},
			m: funcMasker{
				mask:   func([32]byte, Value, Constraints) (Value, error) { return Value{Text: "0"}, nil },
				domain: 1,
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, err := guarded(t, tc.m, CatSpecial, tc.in, tc.c)
			if err != nil {
				t.Fatalf("maskCell = %v, want no error", err)
			}
			if out.Text != tc.in.Text {
				t.Fatalf("out = %q, want %q", out.Text, tc.in.Text)
			}
		})
	}
}

func TestApplyTurnsAPanickingMaskerIntoAValueFreeError(t *testing.T) {
	panics := []struct {
		name string
		val  any
	}{
		{"a string", fmt.Sprintf("masker blew up halfway on %q", victim)},
		{"an error", fmt.Errorf("cannot mask %q", victim)},
		{"nil", nil},
	}
	for _, tc := range panics {
		t.Run(tc.name, func(t *testing.T) {
			m := funcMasker{
				mask: func([32]byte, Value, Constraints) (Value, error) {
					panic(tc.val)
				},
				domain: 1 << 40,
			}
			out, err := guarded(t, m, CatEmail, Value{Text: victim}, Constraints{TypeTag: "text"})
			if !errors.Is(err, ErrMaskerPanic) {
				t.Fatalf("err = %v, want ErrMaskerPanic", err)
			}
			if strings.Contains(strings.ToLower(err.Error()), "victim") {
				t.Errorf("the recovered panic quotes the value: %q", err)
			}
			if !strings.Contains(err.Error(), "rt_probe") {
				t.Errorf("the error does not name the masker: %q", err)
			}
			if !out.Empty() {
				t.Errorf("out = %+v, want the zero Value", out)
			}
		})
	}
}

// The second call the post-condition makes is the generator's code too, and a
// panic from it is recovered and described like any other.
func TestApplyRecoversAPanicFromThePostConditionsSecondCall(t *testing.T) {
	calls := 0
	m := funcMasker{
		mask: func(_ [32]byte, in Value, _ Constraints) (Value, error) {
			calls++
			if calls > 1 {
				panic("probe blew up on " + victim)
			}
			return in, nil
		},
		domain: 1 << 40,
	}
	_, err := guarded(t, m, CatEmail, Value{Text: victim}, Constraints{TypeTag: "text"})
	if !errors.Is(err, ErrMaskerPanic) {
		t.Fatalf("err = %v, want ErrMaskerPanic", err)
	}
	if strings.Contains(strings.ToLower(err.Error()), "victim") {
		t.Errorf("the recovered panic quotes the value: %q", err)
	}
}

// A jsonb document whose leaves are all empty containers or nulls has no
// scalar for semi_structured to replace, so the generator returns it unchanged
// and both halves of the post-condition fire on a cell that proves nothing.
// {"tags":[]} is ordinary production data, and a refusal here would be
// deterministic under the key: the column, and so the database, could never be
// snapshotted.
func TestApplyAllowsADocumentWithNothingToMask(t *testing.T) {
	docs := []string{
		`{"a":{}}`,
		`{"a":[]}`,
		`{"prefs":{}}`,
		`{"tags":[]}`,
		`{"meta":{},"opts":[]}`,
		`{"a":{"b":{}}}`,
		`{"a":null}`,
		`{}`,
		`[]`,
	}
	m, ok := Get(MaskerSemiStruct)
	if !ok {
		t.Fatal("semi_structured is not registered")
	}
	for _, doc := range docs {
		t.Run(doc, func(t *testing.T) {
			out, err := guarded(t, m, CatSemiStruct, Value{Text: doc}, Constraints{TypeTag: "jsonb"})
			if err != nil {
				t.Fatalf("maskCell = %v, want no error", err)
			}
			if out.Text != doc {
				t.Fatalf("out = %q, want the document unchanged (%q)", out.Text, doc)
			}
		})
	}
}

// The same generator on a document that does carry a scalar leaf masks it, so
// nothing about the exemption above weakens the ordinary jsonb path.
func TestApplyMasksADocumentThatHasALeaf(t *testing.T) {
	docs := []string{`{"a":""}`, `{"a":1,"b":{}}`, `{"email":"` + victim + `"}`}
	m, ok := Get(MaskerSemiStruct)
	if !ok {
		t.Fatal("semi_structured is not registered")
	}
	for _, doc := range docs {
		t.Run(doc, func(t *testing.T) {
			out, err := guarded(t, m, CatSemiStruct, Value{Text: doc}, Constraints{TypeTag: "jsonb"})
			if err != nil {
				t.Fatalf("maskCell = %v, want no error", err)
			}
			if strings.Contains(out.Text, victim) {
				t.Fatalf("out = %q, which still carries the source value", out.Text)
			}
		})
	}
}

// The exemption is for a document with nothing in it, not for jsonb: a masker
// that hands back a document with a leaf in it is refused like any other.
func TestApplyRefusesAPassthroughOnADocumentWithALeaf(t *testing.T) {
	doc := `{"email":"` + victim + `","tags":[]}`
	m := funcMasker{
		mask:   func(_ [32]byte, in Value, _ Constraints) (Value, error) { return in, nil },
		domain: 1 << 40,
	}
	_, err := guarded(t, m, CatSemiStruct, Value{Text: doc}, Constraints{TypeTag: "jsonb"})
	if !errors.Is(err, ErrPassthrough) {
		t.Fatalf("err = %v, want ErrPassthrough", err)
	}
	if strings.Contains(strings.ToLower(err.Error()), "victim") {
		t.Errorf("the refusal quotes the value: %q", err)
	}
}
