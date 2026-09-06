// SPDX-License-Identifier: Apache-2.0

package mask

import (
	"bytes"
	"encoding/json"
	"net/netip"
	"regexp"
	"strconv"
	"strings"
	"time"

	pn "github.com/nyaruka/phonenumbers"
	"golang.org/x/text/cases"
	"golang.org/x/text/unicode/norm"
)

// The type tags. A tag names the *canonical form*, not the column's type, so a
// bigint and a text copy of one identifier reach the same digest
// (ARCHITECTURE.md section 5). Canonical returns the tag it produced; the
// column's own type family stays in Constraints.TypeTag and only shapes the
// output. That is the resolution of the scaffold review note on mask.Canonical
// (tracker T-0020): the tag is returned, not promised.
const (
	TagText      = "text"
	TagEmail     = "email"
	TagE164      = "e164"
	TagDigits    = "digits"
	TagDecimal   = "decimal"
	TagDate      = "date"
	TagTimestamp = "timestamp"
	TagAlnum     = "alnum"
	TagIP        = "ip"
	TagMAC       = "mac"
	TagUUID      = "uuid"
	TagURL       = "url"
	TagBytes     = "bytes"
	TagJSON      = "json"
)

var (
	reSpace = regexp.MustCompile(`\s+`)
	reMAC   = regexp.MustCompile(`^[0-9A-Fa-f]{2}([:-][0-9A-Fa-f]{2}){5}$`)
	reUUID  = regexp.MustCompile(`^[0-9a-fA-F]{8}-?[0-9a-fA-F]{4}-?[0-9a-fA-F]{4}-?[0-9a-fA-F]{4}-?[0-9a-fA-F]{12}$`)
	folder  = cases.Fold()
)

// fold is the text canonicalisation every string category starts from: NFKC
// normalisation, case folding, collapsed internal whitespace and trimmed ends.
// Two values that differ only in those respects mask alike, which is what a
// join across two spellings of one name needs.
func fold(s string) string {
	return strings.TrimSpace(reSpace.ReplaceAllString(folder.String(norm.NFKC.String(s)), " "))
}

func keepIf(s string, keep func(rune) bool) string {
	var b strings.Builder
	for _, r := range s {
		if keep(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func onlyDigits(s string) string {
	return keepIf(s, func(r rune) bool { return r >= '0' && r <= '9' })
}

func onlyAlnum(s string) string {
	return keepIf(strings.ToUpper(s), func(r rune) bool {
		return (r >= '0' && r <= '9') || (r >= 'A' && r <= 'Z')
	})
}

// trimZeros is the decimal normalisation of ARCHITECTURE.md section 5, so that
// "000123" and 123 are one identifier.
func trimZeros(s string) string {
	t := strings.TrimLeft(s, "0")
	if t == "" {
		return "0"
	}
	return t
}

// dateLayouts are the forms a person's date is written in that this module
// recognises. A value in none of them canonicalises as folded text, which is
// still deterministic; it only means two spellings of one date do not meet.
var dateLayouts = []string{
	time.RFC3339Nano,
	time.RFC3339,
	"2006-01-02 15:04:05.999999-07",
	"2006-01-02 15:04:05.999999",
	"2006-01-02 15:04:05",
	"2006-01-02T15:04:05",
	"2006-01-02",
	"02/01/2006",
	"01/02/2006",
	"02.01.2006",
	"20060102",
}

// Canonical is the per-category canonical form of a value and the tag naming
// that form. It never returns the value unchanged by accident: a category with
// no canonicalisation of its own folds the text, which is still a decision.
//
// It is exported because internal/transform needs the same bytes for the
// residual filter (ARCHITECTURE.md section 6). The returned Value holds
// production data.
func Canonical(cat Category, in Value, c Constraints) (Value, string, error) {
	if in.Null {
		return in, "", nil
	}
	if len(in.Bytes) > 0 {
		return Value{Bytes: in.Bytes}, TagBytes, nil
	}
	s := in.Text
	switch cat {
	case CatEmail:
		return Value{Text: fold(s)}, TagEmail, nil
	case CatPhone:
		return canonicalPhone(s, c)
	case CatGeo:
		if f, err := strconv.ParseFloat(strings.TrimSpace(s), 64); err == nil {
			return Value{Text: strconv.FormatFloat(f, 'g', -1, 64)}, TagDecimal, nil
		}
		return Value{Text: fold(s)}, TagText, nil
	case CatPersonDate:
		return canonicalDate(s)
	case CatNationalID, CatFinancial:
		a := onlyAlnum(s)
		if a != "" && onlyDigits(a) == a {
			return Value{Text: trimZeros(a)}, TagDecimal, nil
		}
		return Value{Text: a}, TagAlnum, nil
	case CatNetworkID:
		v, tag := canonicalNetwork(s)
		return v, tag, nil
	case CatOnlineID:
		return canonicalOnline(s)
	case CatSemiStruct:
		var buf bytes.Buffer
		if err := json.Compact(&buf, []byte(s)); err == nil {
			return Value{Text: buf.String()}, TagJSON, nil
		}
		return Value{Text: fold(s)}, TagText, nil
	case CatBinary:
		return Value{Text: s}, TagBytes, nil
	case CatNone, CatPersonName, CatAddress, CatCredential, CatFreeText, CatSpecial:
		return Value{Text: fold(s)}, TagText, nil
	default:
		return Value{Text: fold(s)}, TagText, nil
	}
}

func canonicalPhone(s string, c Constraints) (Value, string, error) {
	try := []struct{ text, region string }{
		{s, c.Region},
		{s, ""},
		{"+" + onlyDigits(s), ""},
	}
	for _, t := range try {
		if t.text == "" || t.text == "+" {
			continue
		}
		num, err := pn.Parse(t.text, t.region)
		if err != nil {
			continue
		}
		if pn.IsValidNumber(num) || pn.IsPossibleNumber(num) {
			return Value{Text: pn.Format(num, pn.E164)}, TagE164, nil
		}
	}
	if d := onlyDigits(s); d != "" {
		return Value{Text: trimZeros(d)}, TagDigits, nil
	}
	return Value{Text: fold(s)}, TagText, nil
}

func canonicalDate(s string) (Value, string, error) {
	t := strings.TrimSpace(s)
	for _, layout := range dateLayouts {
		parsed, err := time.Parse(layout, t)
		if err != nil {
			continue
		}
		if layout == "2006-01-02" || layout == "02/01/2006" || layout == "01/02/2006" ||
			layout == "02.01.2006" || layout == "20060102" {
			return Value{Text: parsed.UTC().Format("2006-01-02")}, TagDate, nil
		}
		return Value{Text: parsed.UTC().Format(time.RFC3339)}, TagTimestamp, nil
	}
	return Value{Text: fold(t)}, TagText, nil
}

func canonicalNetwork(s string) (Value, string) {
	t := strings.TrimSpace(s)
	if a, err := netip.ParseAddr(t); err == nil {
		return Value{Text: a.String()}, TagIP
	}
	if p, err := netip.ParsePrefix(t); err == nil {
		return Value{Text: p.Masked().String()}, TagIP
	}
	if reMAC.MatchString(t) {
		return Value{Text: strings.ToLower(strings.ReplaceAll(t, "-", ":"))}, TagMAC
	}
	return Value{Text: fold(t)}, TagText
}

func canonicalOnline(s string) (Value, string, error) {
	t := strings.TrimSpace(s)
	if reUUID.MatchString(t) {
		return Value{Text: strings.ToLower(strings.ReplaceAll(t, "-", ""))}, TagUUID, nil
	}
	if strings.Contains(t, "://") || strings.HasPrefix(strings.ToLower(t), "www.") {
		return Value{Text: strings.ToLower(t)}, TagURL, nil
	}
	return Value{Text: fold(t)}, TagText, nil
}
