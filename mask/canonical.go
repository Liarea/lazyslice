package mask

import (
	"fmt"
	"strings"

	"github.com/nyaruka/phonenumbers"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"golang.org/x/text/unicode/norm"
)

// Canonicalisation runs before hashing so that two spellings of one value hash
// alike: "Alice@Example.COM " and "alice@example.com" must produce the same
// fake, or a snapshot would carry two different fakes for one person. Each
// function here is paired with a category in ARCHITECTURE.md section 5.

// CanonicalText is the default canonical form: NFKC-normalised, trimmed of
// surrounding space, case-folded. It is what person names and free text are
// hashed by.
func CanonicalText(s string) string {
	folded := cases.Fold().String(norm.NFKC.String(s))
	return strings.TrimSpace(folded)
}

// CanonicalEmail trims and case-folds an address. It deliberately does not
// parse: a value the classifier called an email but net/mail rejects still has
// to hash to something stable.
func CanonicalEmail(s string) string {
	return CanonicalText(s)
}

// CanonicalPhone returns the E.164 form of s under the region hint, which is
// the two-letter region the classifier inferred, or "" when it inferred none.
// A number that will not parse is returned in its default canonical text form
// and the boolean reports that it is not E.164.
func CanonicalPhone(s, region string) (string, bool) {
	num, err := phonenumbers.Parse(s, region)
	if err != nil {
		return CanonicalText(s), false
	}
	return phonenumbers.Format(num, phonenumbers.E164), true
}

// ValidPhone reports whether s is a valid number for the region hint. The
// classifier uses it as a value validator and the phone generator uses it to
// keep libphonenumber validity, which phone_unique deliberately does not.
func ValidPhone(s, region string) bool {
	num, err := phonenumbers.Parse(s, region)
	if err != nil {
		return false
	}
	return phonenumbers.IsValidNumber(num)
}

// Title is the display casing used by generators that emit a person name, kept
// here so that casing is one decision rather than one per generator.
func Title(s string) string {
	return cases.Title(language.English).String(s)
}

// Canonical returns the canonical form of in for the named category, together
// with the type tag to hash beside it. It is the single entry point transform
// uses, so that a category's canonical form is defined once.
func Canonical(category string, in Value, c Constraints) (Value, error) {
	if in.Null {
		return in, nil
	}
	if in.Bytes != nil {
		return in, nil
	}
	switch category {
	case "email":
		return Value{Text: CanonicalEmail(in.Text)}, nil
	case "phone":
		text, _ := CanonicalPhone(in.Text, c.Region)
		return Value{Text: text}, nil
	case "":
		return Value{}, fmt.Errorf("mask: Canonical called with no category: %w", ErrNotImplemented)
	default:
		return Value{Text: CanonicalText(in.Text)}, nil
	}
}
