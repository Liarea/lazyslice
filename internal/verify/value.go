// SPDX-License-Identifier: Apache-2.0

package verify

import (
	"bytes"
	"database/sql/driver"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/Liarea/lazyslice/mask"
)

// Turning a value the target holds back into the bytes the residual filter was
// keyed with (ARCHITECTURE.md section 6 item 1).
//
// internal/transform read the *source* value as pgx decoded it, rendered it as
// text, canonicalised it under the column's category and put those bytes in the
// filter. This package reads the *target* value of the same column, whose type
// is the same, and has to reproduce exactly that rendering: a value that
// shipped in cleartext is the same Go value on both sides, and bytes we cannot
// reproduce are a residual scan that reports no hit for a value that leaked
// (THREAT_MODEL.md T12).
//
// So textOf below is internal/transform's textOf. It is a copy: transform is
// another stage package and internal/CLAUDE.md forbids reaching into one. That
// duplication is the contract stated in internal/transform/CLAUDE.md ("the
// residual-filter contract"), and internal/verify/CLAUDE.md records that the
// fix is a shared home for it rather than a third copy.
//
// Nothing here formats a value into an error string: a value that will not
// render is reported by its column and its type, never by its content
// (THREAT_MODEL.md T4).

// valueOf is the mask.Value for one scanned column value.
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

// textOf renders a scanned value in the text form internal/transform rendered
// the source's value in.
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
	return fmt.Sprintf("%v", v)
}

// marshalJSON renders a decoded document as JSON text, keys sorted, which is
// what encoding/json does and what makes the canonical form stable.
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

// canonicalOf is the residual filter's key material for one value under one
// category: mask.Apply's own Result.Canonical, which is the canonical form of
// the value as internal/transform put it in the filter.
//
// It returns ok = false for a value transform would not have recorded: a NULL
// or an empty value, which mask.Apply passes through under its own two rules,
// and a canonical form with no bytes.
//
// The constraints are empty on purpose. mask.Canonical reads exactly one field
// of them — Region, the libphonenumber hint — and nothing in the tree sets it:
// internal/classify decided against a per-table region hint (its CLAUDE.md) and
// internal/transform builds its Constraints without one, so the source's bytes
// and these bytes are computed under the same empty hint. If a region hint ever
// lands, transform's canonical form changes and this must change with it, in the
// same commit, or every masked phone column becomes unscannable.
func canonicalOf(cat mask.Category, v any) ([]byte, bool, error) {
	in := valueOf(v)
	if in.Null || in.Empty() {
		return nil, false, nil
	}
	canon, _, err := mask.Canonical(cat, in, mask.Constraints{})
	if err != nil {
		return nil, false, err
	}
	if len(canon.Bytes) > 0 {
		return canon.Bytes, true, nil
	}
	if canon.Text == "" {
		return nil, false, nil
	}
	return []byte(canon.Text), true, nil
}

// sameValue reports whether two scanned values are the same value. It is the
// comparison the sample check makes between an unmasked target column and the
// source's own value for that row (section 6 item 5).
//
// NULL equals NULL and nothing else; bytea is compared as bytes; everything
// else is compared in the text form above, which is the form both sides were
// read into.
func sameValue(a, b any) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	ab, aIsBytes := a.([]byte)
	bb, bIsBytes := b.([]byte)
	if aIsBytes || bIsBytes {
		if !aIsBytes || !bIsBytes {
			return false
		}
		return bytes.Equal(ab, bb)
	}
	return textOf(a) == textOf(b)
}
