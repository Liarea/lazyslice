// SPDX-License-Identifier: Apache-2.0

package mask

// Every masked phone number lands in the North American range reserved for
// fiction, +1 <area> 555 01NN. libphonenumber accepts it, which is the
// "preserve what the application checks" half of ARCHITECTURE.md section 5,
// and the region does not survive, which is the "nothing survives" half. The
// alternative reading of section 5 — the reserved range of the *detected*
// region — is not shipped: the only other range this module could verify (the
// Ofcom drama block) is a thousand numbers wide, and inventing per-region
// ranges we cannot check against libphonenumber would put invalid numbers into
// a column the application validates. mask/CLAUDE.md records the choice.

// nanpDomain is 447 area codes × the hundred numbers 555-0100 to 555-0199.
func nanpDomain() int64 { return int64(len(areaCodes)) * 100 }

// The two lengths a NANP number takes: ten digits national, eleven with the
// country code in front of them. A numeric column has to hold the whole number
// as a number, so the count is what digitBudget is asked for — an integer
// column holds nine digits and neither shape fits it, which is a refusal at
// plan and not a numeric field overflow (22003) at load.
const (
	phoneNationalDigits = 10
	phoneCountryDigits  = phoneNationalDigits + 1
)

// phoneShape is how many characters the column can give a phone number.
type phoneShape int

const (
	phoneNone     phoneShape = iota
	phoneE164                // +12125550142
	phoneDigits              // 12125550142, for a numeric column or a tight varchar
	phoneNational            // 2125550142
)

func phoneShapeFor(c Constraints) phoneShape {
	if numericFamily(c.TypeTag) {
		switch {
		case digitBudget(c, phoneCountryDigits) >= phoneCountryDigits:
			return phoneDigits
		case digitBudget(c, phoneNationalDigits) >= phoneNationalDigits:
			return phoneNational
		default:
			return phoneNone
		}
	}
	switch n := room(c); {
	case n == 0 || n >= 12:
		return phoneE164
	case n >= 11:
		return phoneDigits
	case n >= 10:
		return phoneNational
	default:
		return phoneNone
	}
}

// phoneMasker emits a valid, fictional North American number.
type phoneMasker struct{}

func (phoneMasker) Domain(c Constraints) int64 {
	if d, ok := labelDomain(c); ok {
		return d
	}
	if phoneShapeFor(c) == phoneNone {
		return 0
	}
	return nanpDomain()
}

func (phoneMasker) Mask(h [32]byte, _ Value, c Constraints) (Value, error) {
	if v, ok := labelValue(h, c); ok {
		return v, nil
	}
	shape := phoneShapeFor(c)
	if shape == phoneNone {
		return Value{}, ErrNoRoom
	}
	s := newStream(h)
	area := areaCodes[s.intn(int64(len(areaCodes)))]
	national := area + "555" + "01" + s.digits(2, true)
	var out string
	switch shape {
	case phoneE164:
		out = "+1" + national
	case phoneDigits:
		out = "1" + national
	case phoneNational, phoneNone:
		out = national
	}
	return Value{Text: out}, nil
}

// uniquePhoneDigits is the twelve digits ARCHITECTURE.md section 5 gives
// phone_unique, for a domain of 10¹².
const uniquePhoneDigits = 12

func uniquePhoneShape(c Constraints) (digits int, prefix string) {
	if numericFamily(c.TypeTag) {
		// The leading 1 is a digit of the number in a numeric column, so the
		// budget has to cover it: an integer column carries nine digits, which
		// is the 1 and eight more, and a bigint carries all thirteen.
		d := digitBudget(c, uniquePhoneDigits+1) - 1
		if d < 1 {
			return 0, ""
		}
		return min(d, uniquePhoneDigits), "1"
	}
	n := room(c)
	if n == 0 {
		return uniquePhoneDigits, "+1"
	}
	d := min(uniquePhoneDigits, n-2)
	if d < 1 {
		return 0, ""
	}
	return d, "+1"
}

// phoneUniqueMasker is the generator a phone column under a unique index gets:
// E.164-shaped, with the country code and up to twelve further digits from h,
// for a domain of 10¹². libphonenumber validity is **not** preserved and the
// plan's explanation says so — that is the price of a unique phone column, and
// the alternative is a collision at load with a PgError whose Detail we drop.
type phoneUniqueMasker struct{}

func (phoneUniqueMasker) Domain(c Constraints) int64 {
	if d, ok := labelDomain(c); ok {
		return d
	}
	d, _ := uniquePhoneShape(c)
	if d <= 0 {
		return 0
	}
	return satPow(10, d)
}

func (phoneUniqueMasker) Mask(h [32]byte, _ Value, c Constraints) (Value, error) {
	if v, ok := labelValue(h, c); ok {
		return v, nil
	}
	d, prefix := uniquePhoneShape(c)
	if d <= 0 {
		return Value{}, ErrNoRoom
	}
	s := newStream(h)
	return Value{Text: prefix + s.digits(d, true)}, nil
}
