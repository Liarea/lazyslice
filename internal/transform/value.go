// SPDX-License-Identifier: Apache-2.0

package transform

import (
	"database/sql"
	"database/sql/driver"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"time"

	"github.com/Liarea/lazyslice/mask"
)

// The two halves of the boundary between a scanned Postgres value and
// mask.Value.
//
// A masker's output is always text or bytes: ARCHITECTURE.md §5 fixes the
// signature as (h, mask.Value, mask.Constraints) -> mask.Value, and mask.Value
// carries Null, Text and Bytes and nothing else. Extract, though, hands this
// package the Go values pgx decoded — an int64, a time.Time, a netip.Prefix, a
// pgtype.Numeric — and the loader has to encode them back into the same column.
// So a masked cell goes out as the *same Go kind it came in as*, parsed back
// from the masker's text. A cell whose kind has no parse back keeps the text,
// which is what an unknown type arrives as in the first place.
//
// Nothing here formats a value into an error string: a value that will not
// parse is reported by its column and its type, never by its content
// (THREAT_MODEL.md T4).

// valueOf is the mask.Value for one scanned column value. Null is the SQL NULL;
// Bytes is bytea; everything else travels as its text form, because that is
// what the canonicalisers in mask read (mask.Canonical).
func valueOf(v any) mask.Value {
	switch t := v.(type) {
	case nil:
		return mask.Value{Null: true}
	case string:
		return mask.Value{Text: t}
	case []byte:
		return mask.Value{Bytes: t}
	}
	return mask.Value{Text: textOf(v)}
}

// textOf renders a scanned value in the text form Postgres itself would print,
// as closely as the decoded Go value allows. It is the input to
// canonicalisation, so two spellings of one value have to reach it the same
// way: an int64 and a numeric holding 42 both render "42".
func textOf(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case []byte:
		return string(t)
	case bool:
		return strconv.FormatBool(t)
	case int:
		return strconv.FormatInt(int64(t), 10)
	case int8:
		return strconv.FormatInt(int64(t), 10)
	case int16:
		return strconv.FormatInt(int64(t), 10)
	case int32:
		return strconv.FormatInt(int64(t), 10)
	case int64:
		return strconv.FormatInt(t, 10)
	case uint8:
		return strconv.FormatUint(uint64(t), 10)
	case uint16:
		return strconv.FormatUint(uint64(t), 10)
	case uint32:
		return strconv.FormatUint(uint64(t), 10)
	case uint64:
		return strconv.FormatUint(t, 10)
	case float32:
		return strconv.FormatFloat(float64(t), 'g', -1, 32)
	case float64:
		return strconv.FormatFloat(t, 'g', -1, 64)
	case time.Time:
		return t.UTC().Format(time.RFC3339Nano)
	case [16]byte:
		return uuidText(t)
	case map[string]any:
		return marshalJSON(t)
	case []any:
		return marshalJSON(t)
	case fmt.Stringer:
		return t.String()
	case driver.Valuer:
		dv, err := t.Value()
		if err != nil || dv == nil {
			return ""
		}
		return textOf(dv)
	}
	// A type nothing above names is rendered by its default formatting. It is
	// still deterministic, which is all canonicalisation needs; %v on a
	// production value never leaves this process, because the result is hashed
	// or masked and neither is printed.
	return fmt.Sprintf("%v", v)
}

// marshalJSON renders a decoded document as JSON text. encoding/json writes an
// object's keys in sorted order, so two runs over one document produce one
// string and the canonical form a digest is taken over is stable.
func marshalJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	return string(b)
}

// uuidText renders the 16 bytes pgx decodes a uuid into.
func uuidText(b [16]byte) string {
	h := hex.EncodeToString(b[:])
	return h[0:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:32]
}

// coerce parses a masker's output back into the Go kind the original value had,
// so the loader encodes a masked bigint as a bigint and not as a string.
//
// A kind with no parse back keeps the text. That is the honest answer for a
// type this package does not model: the value is masked either way, and the
// loader's encode is where a mismatch surfaces, loudly.
func coerce(out mask.Value, original any) (any, error) {
	if out.Null {
		return nil, nil
	}
	switch original.(type) {
	case nil, string:
		return out.Text, nil
	case []byte:
		if out.Bytes != nil {
			return out.Bytes, nil
		}
		return []byte(out.Text), nil
	case bool:
		return strconv.ParseBool(out.Text)
	case int:
		n, err := strconv.ParseInt(out.Text, 10, 0)
		return int(n), err
	case int16:
		n, err := strconv.ParseInt(out.Text, 10, 16)
		return int16(n), err
	case int32:
		n, err := strconv.ParseInt(out.Text, 10, 32)
		return int32(n), err
	case int64:
		return strconv.ParseInt(out.Text, 10, 64)
	case float32:
		f, err := strconv.ParseFloat(out.Text, 32)
		return float32(f), err
	case float64:
		return strconv.ParseFloat(out.Text, 64)
	case time.Time:
		return parseTime(out.Text)
	case [16]byte:
		return parseUUID(out.Text)
	}
	// pgtype.Numeric, pgtype.Interval and the other driver types implement
	// sql.Scanner over their own text form, so one reflective path covers every
	// one of them without this package importing pgtype and listing them.
	if v, ok := scanBack(out.Text, original); ok {
		return v, nil
	}
	return out.Text, nil
}

// scanBack builds a fresh value of the original's type and lets it scan the
// masker's text into itself.
func scanBack(text string, original any) (any, bool) {
	rt := reflect.TypeOf(original)
	if rt == nil {
		return nil, false
	}
	p := reflect.New(rt)
	s, ok := p.Interface().(sql.Scanner)
	if !ok {
		return nil, false
	}
	if err := s.Scan(text); err != nil {
		return nil, false
	}
	return p.Elem().Interface(), true
}

// timeLayouts are the forms a masked date or timestamp comes back in. mask's
// person_date generator emits the first two; the rest are the spellings a
// fixed: literal or a CHECK-derived label can carry.
var timeLayouts = []string{
	time.RFC3339Nano,
	time.RFC3339,
	"2006-01-02 15:04:05.999999-07",
	"2006-01-02 15:04:05.999999",
	"2006-01-02 15:04:05",
	"2006-01-02T15:04:05",
	"2006-01-02",
}

func parseTime(s string) (time.Time, error) {
	for _, layout := range timeLayouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC(), nil
		}
	}
	// The error names the layouts and not the text: the text is a masked value
	// and an error string is a way out of the process.
	return time.Time{}, fmt.Errorf("a masked timestamp is in none of the %d layouts this package parses", len(timeLayouts))
}

func parseUUID(s string) ([16]byte, error) {
	var out [16]byte
	clean := make([]byte, 0, 32)
	for i := 0; i < len(s); i++ {
		if s[i] != '-' {
			clean = append(clean, s[i])
		}
	}
	if len(clean) != 32 {
		return out, fmt.Errorf("a masked uuid is %d hex characters, not 32", len(clean))
	}
	if _, err := hex.Decode(out[:], clean); err != nil {
		return out, fmt.Errorf("a masked uuid is not hexadecimal")
	}
	return out, nil
}
