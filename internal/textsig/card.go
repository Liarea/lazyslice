// SPDX-License-Identifier: Apache-2.0

package textsig

// The payment-card validators (tracker T-0316). ValidLuhn is the check digit
// over any twelve-to-nineteen-digit run, and a check digit is one chance in
// ten: dogfood session 1 masked eight columns of ordinary identifiers on it —
// CRM and billing customer ids, subscription ids, invoice and estimate
// numbers, and a Rails schema_migrations.version timestamp — as free_text,
// "a strong validator hit below the category threshold". A payment card is
// more than a check digit. Its first digits are an issuer identification
// number from a published range, and each issuer issues only some lengths.
//
// So there are two readings here, beside ValidLuhn (which stays as it was for
// the callers that were not part of T-0316: the DDL-literal passes, the JSON
// key and leaf readers):
//
//   - ValidCard: twelve to nineteen digits, the Luhn check, and a known issuer
//     prefix. The length need only be in ISO/IEC 7812's range.
//   - CardShape: ValidCard, and the length is one the issuer of that prefix
//     actually issues. This is the "full shape" a caller asks for when the
//     column's name says the value is an identifier of its own.
//
// Which one a column is scored with is the caller's question (a column name is
// not this package's to read): internal/classify and internal/verify each keep
// the same name rule, by hand.
//
// The issuer table is Wikipedia's "Payment card number" IIN table
// (https://en.wikipedia.org/wiki/Payment_card_number#Issuer_identification_number_(IIN),
// read 2026-09-24). It errs wide: every range it lists is here, including
// UATP's single leading 1, because a card this table misses is a card copied
// verbatim (THREAT_MODEL.md T1), and a range it wrongly carries costs only the
// one-in-ten chance ValidLuhn already charged every digit run.

// ValidCard reports whether a value is a payment card number: twelve to
// nineteen digits once separators are removed, a valid Luhn check digit, and a
// first few digits inside a known issuer range. It reads every spelling
// Candidates yields, as ValidLuhn does.
func ValidCard(s string) bool {
	return anyCandidate(s, func(c string) bool { return validCard(c, false) })
}

// CardShape is ValidCard with the length also required to be one the issuer
// of the value's prefix issues (a Visa is 13, 16 or 19 digits, an American
// Express 15, a Mastercard 16).
func CardShape(s string) bool {
	return anyCandidate(s, func(c string) bool { return validCard(c, true) })
}

// cardLengths is a set of card lengths, bit n meaning n digits.
type cardLengths uint32

func lengths(ns ...int) cardLengths {
	var l cardLengths
	for _, n := range ns {
		l |= 1 << n
	}
	return l
}

func lengthRange(lo, hi int) cardLengths {
	var l cardLengths
	for n := lo; n <= hi; n++ {
		l |= 1 << n
	}
	return l
}

// issuerRange is one row of the IIN table: every number whose first
// len(lo) digits fall between lo and hi inclusive (lo and hi are the same
// length), issued at the lengths in n.
type issuerRange struct {
	lo, hi string
	n      cardLengths
}

var (
	len12to19     = lengthRange(12, 19)
	len14to19     = lengthRange(14, 19)
	len16to19     = lengthRange(16, 19)
	len15         = lengths(15)
	len16         = lengths(16)
	len19         = lengths(19)
	len16or18or19 = lengths(16, 18, 19)
)

// issuerRanges is the IIN table, one issuing network per comment. A prefix
// two networks share (Discover and Troy's 65, Maestro and Switch's 6759) is
// listed under each, and a value is a card if any row takes it.
var issuerRanges = []issuerRange{
	// American Express
	{"34", "34", len15},
	{"37", "37", len15},
	// Bankcard
	{"5610", "5610", len16},
	{"560221", "560225", len16},
	// China T-Union
	{"31", "31", len19},
	// China UnionPay
	{"62", "62", len16to19},
	// Diners Club International
	{"30", "30", len14to19},
	{"36", "36", len14to19},
	{"38", "39", len14to19},
	// Diners Club US & Canada
	{"55", "55", len16},
	// Discover
	{"6011", "6011", len16to19},
	{"644", "649", len16to19},
	{"65", "65", len16to19},
	// Discover, China UnionPay co-branded
	{"622126", "622925", len16to19},
	// UkrCart
	{"60400100", "60420099", len16to19},
	// RuPay
	{"60", "60", len16},
	{"65", "65", len16},
	{"81", "82", len16},
	{"508", "508", len16},
	// RuPay-JCB co-branded
	{"353", "353", len16},
	{"356", "356", len16},
	// InterPayment
	{"636", "636", len16to19},
	// InstaPayment
	{"637", "639", len16},
	// JCB
	{"3528", "3589", len16to19},
	// LankaPay
	{"357111", "357111", len16},
	// Laser
	{"6304", "6304", len16to19},
	{"6706", "6706", len16to19},
	{"6709", "6709", len16to19},
	{"6771", "6771", len16to19},
	// Maestro UK
	{"6759", "6759", len12to19},
	{"676770", "676770", len12to19},
	{"676774", "676774", len12to19},
	// Maestro
	{"5018", "5018", len12to19},
	{"5020", "5020", len12to19},
	{"5038", "5038", len12to19},
	{"5893", "5893", len12to19},
	{"6304", "6304", len12to19},
	{"6759", "6759", len12to19},
	{"6761", "6763", len12to19},
	// Dankort, and its Visa co-brand
	{"5019", "5019", len16},
	{"4571", "4571", len16},
	// Mir
	{"2200", "2204", len16to19},
	// BORICA
	{"2205", "2205", len16},
	// NPS Pridnestrovie
	{"6054740", "6054744", len16},
	// Mastercard
	{"2221", "2720", len16},
	{"51", "55", len16},
	// Solo
	{"6334", "6334", len16or18or19},
	{"6767", "6767", len16or18or19},
	// Switch
	{"4903", "4903", len16or18or19},
	{"4905", "4905", len16or18or19},
	{"4911", "4911", len16or18or19},
	{"4936", "4936", len16or18or19},
	{"564182", "564182", len16or18or19},
	{"633110", "633110", len16or18or19},
	{"6333", "6333", len16or18or19},
	{"6759", "6759", len16or18or19},
	// Troy
	{"65", "65", len16},
	{"9792", "9792", len16},
	// Visa
	{"4", "4", lengths(13, 16, 19)},
	// Visa Electron
	{"4026", "4026", len16},
	{"417500", "417500", len16},
	{"4844", "4844", len16},
	{"4913", "4913", len16},
	{"4917", "4917", len16},
	// UATP
	{"1", "1", len15},
	// Verve
	{"506099", "506198", len16or18or19},
	{"650002", "650027", len16or18or19},
	{"507865", "507964", len16or18or19},
	// Uzcard
	{"8600", "8600", len16},
	{"5614", "5614", len16},
	// HUMO
	{"9860", "9860", len16},
	// GPN
	{"1946", "1946", len16or18or19},
	{"50", "50", len16or18or19},
	{"56", "56", len16or18or19},
	{"58", "58", len16or18or19},
	{"60", "63", len16or18or19},
	// Napas
	{"9704", "9704", lengths(16, 19)},
	// WEX
	{"960046", "960046", len19},
}

// validCard is ValidCard over one spelling, and CardShape when exact is true.
func validCard(s string, exact bool) bool {
	if !validLuhn(s) {
		return false
	}
	digits := make([]byte, 0, 19)
	for i := 0; i < len(s); i++ {
		if c := s[i]; c >= '0' && c <= '9' {
			digits = append(digits, c)
		}
	}
	n := len(digits)
	for _, r := range issuerRanges {
		k := len(r.lo)
		if n < k {
			continue
		}
		p := string(digits[:k])
		if p < r.lo || p > r.hi {
			continue
		}
		if !exact || r.n&(1<<n) != 0 {
			return true
		}
	}
	return false
}
