// SPDX-License-Identifier: Apache-2.0

package mask

import (
	"strconv"
	"strings"
	"time"
)

func itoa(n int64) string { return strconv.FormatInt(n, 10) }

// pad renders n with exactly width digits, zero-filled.
func pad(n int64, width int) string {
	s := itoa(n)
	if len(s) >= width {
		return s
	}
	return strings.Repeat("0", width-len(s)) + s
}

func satAdd(a, b int64) int64 {
	if a > 1<<62 || b > 1<<62 {
		return 1 << 62
	}
	return a + b
}

// ---------- national_id ----------

const nationalIDDigits = 9

// nationalIDMasker emits a fixed-width run of digits. The width comes from the
// column, never from the input, so a nine-digit and a six-digit identifier
// are indistinguishable after masking.
type nationalIDMasker struct{}

func nationalIDLen(c Constraints) int {
	if l := checkLength(c); l > 0 && (c.MaxLen <= 0 || l <= c.MaxLen) {
		return l
	}
	return digitBudget(c, nationalIDDigits)
}

func (nationalIDMasker) Domain(c Constraints) int64 {
	if d, ok := labelDomain(c); ok {
		return d
	}
	n := nationalIDLen(c)
	if n < 1 {
		return 0
	}
	return satMul(9, pow10(n-1))
}

func (nationalIDMasker) Mask(h [32]byte, _ Value, c Constraints) (Value, error) {
	if v, ok := labelValue(h, c); ok {
		return v, nil
	}
	n := nationalIDLen(c)
	if n < 1 {
		return Value{}, ErrNoRoom
	}
	return Value{Text: newStream(h).digits(n, false)}, nil
}

// ---------- financial_account ----------

const financialDigits = 16

// financialAccountMasker emits a Luhn-valid run of digits, because a card
// number that fails the application's own check digit is a masked column that
// breaks the fixture it was made for. It is not a real account: nothing of the
// original survives, the issuer prefix included.
type financialAccountMasker struct{}

func financialLen(c Constraints) int {
	if l := checkLength(c); l > 0 && (c.MaxLen <= 0 || l <= c.MaxLen) {
		return l
	}
	return digitBudget(c, financialDigits)
}

func (financialAccountMasker) Domain(c Constraints) int64 {
	if d, ok := labelDomain(c); ok {
		return d
	}
	n := financialLen(c)
	if n < 2 {
		return 0
	}
	return satMul(9, pow10(n-2))
}

func (financialAccountMasker) Mask(h [32]byte, _ Value, c Constraints) (Value, error) {
	if v, ok := labelValue(h, c); ok {
		return v, nil
	}
	n := financialLen(c)
	if n < 2 {
		return Value{}, ErrNoRoom
	}
	body := newStream(h).digits(n-1, false)
	return Value{Text: body + luhnCheckDigit(body)}, nil
}

// luhnCheckDigit is the digit that makes body ‖ digit pass the Luhn check.
func luhnCheckDigit(body string) string {
	sum := 0
	double := true
	for i := len(body) - 1; i >= 0; i-- {
		d := int(body[i] - '0')
		if double {
			d *= 2
			if d > 9 {
				d -= 9
			}
		}
		double = !double
		sum += d
	}
	return itoa(int64((10 - sum%10) % 10))
}

// ---------- person_date ----------

// The window a masked person-date lands in. It is wide enough that a birth
// date, a date of death and a date of diagnosis all look ordinary, and it is
// fixed rather than derived from the input, so no part of the original date —
// not the decade, not the month, not the day of the week — survives.
var (
	dateStart = time.Date(1940, 1, 1, 0, 0, 0, 0, time.UTC)
	dateDays  = int64(time.Date(2010, 1, 1, 0, 0, 0, 0, time.UTC).Sub(dateStart).Hours() / 24)
)

const (
	dateLayout      = "2006-01-02"
	timestampLayout = "2006-01-02 15:04:05"
	secondsPerDay   = 86400
)

// personDateMasker emits a date, or a timestamp for a column that holds one.
type personDateMasker struct{}

func personDateWidth(c Constraints) int {
	if c.TypeTag == famTimestamp {
		return len(timestampLayout)
	}
	return len(dateLayout)
}

func (m personDateMasker) Domain(c Constraints) int64 {
	if d, ok := labelDomain(c); ok {
		return d
	}
	if c.TypeTag != famDate && c.TypeTag != famTimestamp && !fits(strings.Repeat("x", personDateWidth(c)), c) {
		return 0
	}
	if c.TypeTag == famTimestamp {
		return satMul(dateDays, secondsPerDay)
	}
	return dateDays
}

func (m personDateMasker) Mask(h [32]byte, _ Value, c Constraints) (Value, error) {
	if v, ok := labelValue(h, c); ok {
		return v, nil
	}
	if m.Domain(c) == 0 {
		return Value{}, ErrNoRoom
	}
	s := newStream(h)
	d := dateStart.AddDate(0, 0, int(s.intn(dateDays)))
	if c.TypeTag == famTimestamp {
		d = d.Add(time.Duration(s.intn(secondsPerDay)) * time.Second)
		return Value{Text: d.Format(timestampLayout)}, nil
	}
	return Value{Text: d.Format(dateLayout)}, nil
}

// ---------- the generic fallback ----------
//
// Every generator above knows its category. generic is what a category with no
// shape of its own gets — special_category over an open domain, a JSON leaf
// with no key rule — and it is also the answer to THREAT_MODEL.md T12's first
// failure mode: a generator that meets a type it did not expect returns a
// value of that type, never its input.

func genericDomain(c Constraints) int64 {
	if l := labels(c); len(l) > 0 {
		return int64(len(l))
	}
	switch c.TypeTag {
	case famBoolean:
		return 2
	case famInteger, famBigint, famNumeric, famFloat:
		n := digitBudget(c, nationalIDDigits)
		if n < 1 {
			return 0
		}
		return satMul(9, pow10(n-1))
	case famDate, famTimestamp:
		return personDateMasker{}.Domain(c)
	case famUUID:
		return satPow(2, 122)
	case famBytea:
		return satPow(2, 64)
	case famJSON, famJSONB, famHstore:
		return freeTextMasker{}.Domain(Constraints{})
	default:
		return freeTextMasker{}.Domain(c)
	}
}

func generic(s *stream, c Constraints) (Value, error) {
	if l := labels(c); len(l) > 0 {
		return Value{Text: l[s.intn(int64(len(l)))]}, nil
	}
	switch c.TypeTag {
	case famBoolean:
		if s.intn(2) == 1 {
			return Value{Text: "true"}, nil
		}
		return Value{Text: "false"}, nil
	case famInteger, famBigint, famNumeric, famFloat:
		n := digitBudget(c, nationalIDDigits)
		if n < 1 {
			return Value{}, ErrNoRoom
		}
		return Value{Text: s.digits(n, false)}, nil
	case famDate, famTimestamp:
		d := dateStart.AddDate(0, 0, int(s.intn(dateDays)))
		if c.TypeTag == famTimestamp {
			d = d.Add(time.Duration(s.intn(secondsPerDay)) * time.Second)
			return Value{Text: d.Format(timestampLayout)}, nil
		}
		return Value{Text: d.Format(dateLayout)}, nil
	case famUUID:
		return Value{Text: uuidFrom(s)}, nil
	case famBytea:
		b := make([]byte, 16)
		s.read(b)
		return Value{Bytes: b}, nil
	case famJSON, famJSONB:
		return Value{Text: `{"value":"` + filler(s, 16) + `"}`}, nil
	case famHstore:
		return Value{Text: `"value"=>"` + filler(s, 16) + `"`}, nil
	default:
		n := freeTextMax(c)
		if n < 1 {
			return Value{}, ErrNoRoom
		}
		return Value{Text: filler(s, 1+int(s.intn(int64(n))))}, nil
	}
}

// zeroValue is the value a collapse writes into a column that cannot take
// NULL. It carries no information at all, which is the point.
func zeroValue(c Constraints) Value {
	if l := labels(c); len(l) > 0 {
		return Value{Text: l[0]}
	}
	switch c.TypeTag {
	case famBoolean:
		return Value{Text: "false"}
	case famInteger, famBigint, famNumeric, famFloat:
		return Value{Text: "0"}
	case famDate:
		return Value{Text: "1970-01-01"}
	case famTimestamp:
		return Value{Text: "1970-01-01 00:00:00"}
	case famUUID:
		return Value{Text: "00000000-0000-0000-0000-000000000000"}
	case famBytea:
		return Value{Bytes: []byte{}}
	case famJSON, famJSONB:
		return Value{Text: "{}"}
	case famInet, famCIDR:
		return Value{Text: "192.0.2.0"}
	case famMacaddr:
		return Value{Text: "00:00:5e:00:53:00"}
	default:
		return Value{Text: ""}
	}
}
