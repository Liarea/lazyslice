// SPDX-License-Identifier: Apache-2.0

package classify

import (
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/textsig"
)

// The thresholds ARCHITECTURE.md §4's value signals are scored against, and the
// conversion from one sampled value to the strings a validator reads. Samples
// arrive through pipeline.Sampler, taken at introspect; nothing here sees a
// database.
//
// A validator returns a verdict on one value and lives in internal/textsig,
// which internal/verify imports too (tracker T-0055). The scoring is this
// package's and stays here: classify.go counts verdicts over the non-NULL
// samples and compares the ratio against the thresholds below. No validator
// ever reaches a reason string, only the count and the fixed phrase that names
// it.

// validatorThreshold is ARCHITECTURE.md §4's "≥80% of non-null samples
// validating". It is the same number on both branches of the scoring rule: a
// name hit plus this ratio is `certain`, and this ratio with no name hit is
// `likely`.
const validatorThreshold = 0.8

// weakThreshold is where a value signal stops being noise and starts being a
// reason to look at the column's neighbours. ARCHITECTURE.md §4 does not name
// it; see internal/classify/CLAUDE.md, "Decisions made during implementation".
const weakThreshold = 0.5

// minSamples is how many non-NULL samples a value signal needs before it can
// decide anything on its own. One row that happens to parse as an address is
// not evidence about a column; it is evidence about a row.
const minSamples = 3

// The validators themselves live in internal/textsig, which internal/verify's
// second net imports too (tracker T-0055). What stays here is the half that is
// about *this* package's inputs: turning one sampled value into the strings a
// validator reads, and the JSON leaf walk that asks whether a document carries
// personal data at all.

// ---------- turning a sampled value into strings ----------

// scalarsOf reduces one sampled value to the strings the validators run over,
// for a column of this type.
//
// The array branch is tracker T-0103. scalars() below flattens an array only
// when the driver handed back a slice, and pgx does that only for an array type
// its map knows: the source pool runs in QueryExecModeExec and registers no
// user types (T-0076), so a citext[] of addresses arrives as the single string
// "{a@b.test,c@d.test}", no validator matches it, and the column is decided
// `none` and copied verbatim (THREAT_MODEL.md T1). When the column's type says
// array and the sample is one string, the string is read back as an array
// literal and the validators see the addresses inside it.
//
// It is a fallback and not a replacement: a sample the splitter cannot read
// falls through to scalars(), which treats it as one opaque value, which is the
// behaviour before this change and never worse than it.
func scalarsOf(ct columnType, v any) []string {
	if ct.Array {
		if s, ok := literalText(v); ok {
			if elems, ok := splitArrayLiteral(s); ok {
				out := make([]string, 0, len(elems))
				for _, e := range elems {
					out = append(out, scalars(e)...)
				}
				return out
			}
		}
	}
	return scalars(v)
}

// literalText is the sampled value as the server's text form, for the two
// callers that read a literal back. A []byte is the same bytes: pgx hands an
// unregistered type back as text, and whether that arrives as a string or as
// raw bytes is a driver detail, not a decision about the column.
func literalText(v any) (string, bool) {
	switch t := v.(type) {
	case string:
		return t, true
	case []byte:
		return string(t), true
	}
	return "", false
}

// scalars reduces one sampled value to the strings the validators run over. An
// array yields its elements, because ARCHITECTURE.md §4 classifies an array on
// its element type; a NULL yields nothing at all, because the ratio is over the
// non-NULL samples.
func scalars(v any) []string {
	if v == nil {
		return nil
	}
	if elems, ok := arrayElements(v); ok {
		out := make([]string, 0, len(elems))
		for _, e := range elems {
			out = append(out, scalars(e)...)
		}
		return out
	}
	if s, ok := asText(v); ok {
		return []string{s}
	}
	return nil
}

// arrayElements reports the elements of an array value. A []byte is bytea, not
// an array, and a string is not a sequence for this purpose.
func arrayElements(v any) ([]any, bool) {
	switch t := v.(type) {
	case []byte, string:
		return nil, false
	case []any:
		return t, true
	}
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Slice && rv.Kind() != reflect.Array {
		return nil, false
	}
	out := make([]any, 0, rv.Len())
	for i := 0; i < rv.Len(); i++ {
		out = append(out, rv.Index(i).Interface())
	}
	return out, true
}

// asText renders one scalar. It is deliberately narrow: a type it does not know
// yields nothing rather than a Go rendering of a struct, which no validator
// could read and which could carry a production value into a comparison it was
// never meant for.
func asText(v any) (string, bool) {
	switch t := v.(type) {
	case nil:
		return "", false
	case string:
		return t, true
	case []byte:
		return string(t), true
	case bool:
		return fmt.Sprintf("%t", t), true
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return fmt.Sprintf("%d", t), true
	case float32, float64:
		return fmt.Sprintf("%v", t), true
	case time.Time:
		return t.Format(time.RFC3339), true
	case fmt.Stringer:
		return t.String(), true
	}
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return "", false
		}
		return asText(rv.Elem().Interface())
	}
	return "", false
}

// ---------- JSON ----------

// jsonLeaf is one scalar leaf of a sampled JSON document, with the key that
// names it. ARCHITECTURE.md §4 masks a document leaf by leaf, choosing each
// string leaf's masker by running its key through the name rules; the
// classifier's job here is narrower — deciding whether the document carries
// personal data at all.
type jsonLeaf struct {
	Key   string
	Value string
}

// jsonLeaves walks a sampled JSON value. A document that does not parse yields
// nothing, which leaves the column classified on its type alone: json and jsonb
// are type signals in their own right, so an unparseable document is still
// masked.
func jsonLeaves(v any) []jsonLeaf {
	var doc any
	switch t := v.(type) {
	case nil:
		return nil
	case []byte:
		if json.Unmarshal(t, &doc) != nil {
			return nil
		}
	case string:
		if json.Unmarshal([]byte(t), &doc) != nil {
			return nil
		}
	default:
		doc = v
	}
	var out []jsonLeaf
	walkJSON("", doc, &out, 0)
	return out
}

const jsonMaxDepth = 16

func walkJSON(key string, v any, out *[]jsonLeaf, depth int) {
	if depth > jsonMaxDepth || len(*out) > 4096 {
		return
	}
	switch t := v.(type) {
	case map[string]any:
		for k, child := range t {
			walkJSON(k, child, out, depth+1)
		}
	case []any:
		for _, child := range t {
			walkJSON(key, child, out, depth+1)
		}
	case string:
		*out = append(*out, jsonLeaf{Key: key, Value: t})
	case nil:
	default:
		if s, ok := asText(v); ok {
			*out = append(*out, jsonLeaf{Key: key, Value: s})
		}
	}
}

// jsonLeafIsPersonal reports whether a leaf carries personal data, by the same
// rules the classifier applies to a column: the leaf's key through the name
// rules, or the leaf's value through the validators.
func jsonLeafIsPersonal(p *compiledPack, leaf jsonLeaf) bool {
	if leaf.Key != "" {
		if pat, ok := p.match(normaliseName(leaf.Key)); ok && pat.Category != pipeline.CatFreeText {
			return true
		}
	}
	return textsig.ValidEmail(leaf.Value) || textsig.ValidPhone(leaf.Value) ||
		textsig.ValidIP(leaf.Value) || textsig.ValidIBAN(leaf.Value) ||
		textsig.ValidLuhn(leaf.Value)
}
