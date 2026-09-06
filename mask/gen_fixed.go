// SPDX-License-Identifier: Apache-2.0

package mask

import (
	"bytes"
	"encoding/json"
	"sort"
	"strings"
)

// CredentialLiteral is the value every credential column becomes. It is not a
// plausible-looking hash on purpose: a password column full of well-formed
// bcrypt is a column somebody will eventually try to crack, and a column full
// of $lazyslice$invalid is one nobody mistakes for real (ARCHITECTURE.md
// section 5).
const CredentialLiteral = "$lazyslice$invalid"

// CredentialMasker is the id the rule pack names for the credential category.
const CredentialMasker ID = FixedPrefix + CredentialLiteral

// fixedMasker writes one literal into every cell. Its domain is 1 and it says
// so, so a unique column can never be given one by accident.
type fixedMasker struct{ literal string }

func (m fixedMasker) Domain(_ Constraints) int64 { return 1 }

func (m fixedMasker) Mask(_ [32]byte, _ Value, c Constraints) (Value, error) {
	lit := m.literal
	if c.MaxLen > 0 && len(lit) > c.MaxLen {
		lit = lit[:c.MaxLen]
	}
	if c.TypeTag == famBytea {
		return Value{Bytes: []byte(lit)}, nil
	}
	return Value{Text: lit}, nil
}

// nullMasker empties the column. It is what binary_personal gets: a photograph
// or a signature has no fake worth generating, and a NULL is the only value
// that carries nothing at all. A column that cannot take NULL gets the type's
// zero value instead, so the load still succeeds.
type nullMasker struct{}

func (nullMasker) Domain(_ Constraints) int64 { return 1 }

func (nullMasker) Mask(_ [32]byte, _ Value, c Constraints) (Value, error) {
	if c.Nullable {
		return Value{Null: true}, nil
	}
	return zeroValue(c), nil
}

// semiStructuredMasker replaces every scalar leaf of a JSON document and keeps
// its structure and its key names, which is what CONCEPT.md promises and what
// ARCHITECTURE.md section 6 item 6 lists as a stated false negative: key names
// survive.
//
// It masks every string leaf as free text. The per-key refinement of
// ARCHITECTURE.md section 4 — running a leaf's key name through the name rules
// and masking the leaf under the category that comes back — needs the rule
// pack, which lives in internal/classify and cannot be imported here; section
// 12 puts JSON leaf walking in internal/transform for that reason, and this
// generator is the value-only floor beneath it.
//
// hstore is replaced with an empty map rather than walked: this module has no
// hstore parser, and an empty map is the one answer that cannot leak.
type semiStructuredMasker struct{}

func (semiStructuredMasker) Domain(c Constraints) int64 {
	if d, ok := labelDomain(c); ok {
		return d
	}
	if c.TypeTag == famHstore {
		return 1
	}
	// One string leaf's worth, which is the narrowest document that is not
	// already empty.
	return freeTextMasker{}.Domain(Constraints{})
}

func (semiStructuredMasker) Mask(h [32]byte, in Value, c Constraints) (Value, error) {
	if v, ok := labelValue(h, c); ok {
		return v, nil
	}
	s := newStream(h)
	if c.TypeTag == famHstore {
		return Value{Text: ""}, nil
	}
	doc, ok := decodeJSON(in.Text)
	if !ok {
		// A document this module cannot parse is replaced whole, never copied.
		return Value{Text: emptyDocument}, nil
	}
	out, ok := encodeJSON(maskJSON(s, doc))
	if !ok {
		return Value{Text: emptyDocument}, nil
	}
	return Value{Text: out}, nil
}

// emptyDocument is what a document this module cannot walk becomes. It is also
// the value ARCHITECTURE.md section 4 gives a log-shaped table's jsonb, which
// the rule pack reaches through "fixed:{}".
const emptyDocument = "{}"

func decodeJSON(s string) (any, bool) {
	dec := json.NewDecoder(strings.NewReader(s))
	dec.UseNumber()
	var doc any
	if err := dec.Decode(&doc); err != nil {
		return nil, false
	}
	return doc, true
}

func encodeJSON(doc any) (string, bool) {
	var out bytes.Buffer
	if err := json.NewEncoder(&out).Encode(doc); err != nil {
		return "", false
	}
	return strings.TrimRight(out.String(), "\n"), true
}

// maskJSON walks a decoded document. Object keys are visited in sorted order,
// never in Go's map order, so the same document always draws the same values
// from the same stream.
func maskJSON(s *stream, v any) any {
	switch t := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			t[k] = maskJSON(s, t[k])
		}
		return t
	case []any:
		for i := range t {
			t[i] = maskJSON(s, t[i])
		}
		return t
	case string:
		return filler(s, 1+int(s.intn(24)))
	case json.Number:
		if strings.ContainsAny(t.String(), ".eE") {
			return json.Number(s.digits(3, false) + "." + s.digits(2, true))
		}
		return json.Number(s.digits(1+int(s.intn(6)), false))
	case bool:
		return s.intn(2) == 1
	case nil:
		return nil
	default:
		// json.Decode produces nothing else; a future kind becomes null rather
		// than passing through (THREAT_MODEL.md T12).
		return nil
	}
}
