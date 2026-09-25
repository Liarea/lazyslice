// SPDX-License-Identifier: Apache-2.0

package mask

import (
	"strings"
	"unicode/utf8"
)

// titleASCII capitalises the first byte. Every embedded word is ASCII, so this
// is the whole of the case handling a generated name needs.
func titleASCII(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// ---------- person_name ----------

// personNameMasker draws a name from the module's one pair of name lists,
// givenNames and surnames (the 2020 Census given names and surnames,
// words.go), shaped by Constraints.Role (T-0287). RoleGiven draws one given
// name, RoleFamily one surname. The zero value, RoleFull, gets "Given Surname"
// when the column is wide enough and a single given name that fits when it is
// not. A column too narrow for any name at all has a domain of 0 and is
// refused at plan rather than truncated here, because a truncated name keeps
// a length; so is one whose width leaves fewer than nameDomainFloor names to
// draw from (below). Before T-0304 the two single-word roles drew from synthetic
// syllable lists of their own and RoleFull from a hand-curated list of about
// 140 names each; mask/CLAUDE.md records why that changed.
//
// It is the one vocabulary masker (vocab.go, ADR-015): its vocabulary method
// answers, over exactly these lists, whether a value is one Mask could have
// produced, which is what lets the residual scan explain a masked name that
// equals some other row's real name; and maskCell redraws it whenever its
// output reads as its own input, so a masked name never equals its own
// source value.
type personNameMasker struct{}

// nameDomainFloor is the fewest names a person_name column may draw from, and
// it is ADR-015's floor on each list: at least 400, so that section 5's
// small_domain rule (d < 2 x distinct samples) never fires over a 200-row
// sample of a name column. The whole lists clear it (958 and 1,000); a narrow
// column's fitting subset may not — a varchar(2) fits two given names (Jo,
// Ty), and the redraw makes that an invertible swap — and the parent does not
// apply the generator half of that rule (mask/CLAUDE.md, T-0304), so the
// masker refuses such a column itself: its Domain is 0 and it is refused at
// plan, as it was before the Census lists brought two-letter names.
const nameDomainFloor = 400

// personNameForm is the form personNameMasker draws for c and how many values
// it has: a single given name, a single surname, or (RoleFull) a
// "Given Surname" pair when enough pairs fit and a single given name when not.
// n is 0 when no form fitting the column reaches nameDomainFloor.
func personNameForm(c Constraints) (list *wordList, pair bool, n int64) {
	budget := room(c)
	floor := func(l *wordList) (*wordList, bool, int64) {
		if k := int64(l.count(budget)); k >= nameDomainFloor {
			return l, false, k
		}
		return nil, false, 0
	}
	switch c.Role {
	case RoleGiven:
		return floor(givenNames)
	case RoleFamily:
		return floor(surnames)
	default:
		if k := pairCount(givenNames, surnames, 1, budget); k >= nameDomainFloor {
			return nil, true, k
		}
		return floor(givenNames)
	}
}

func (personNameMasker) Domain(c Constraints) int64 {
	if d, ok := labelDomain(c); ok {
		return d
	}
	_, _, n := personNameForm(c)
	return n
}

func (personNameMasker) Mask(h [32]byte, _ Value, c Constraints) (Value, error) {
	if v, ok := labelValue(h, c); ok {
		return v, nil
	}
	list, pair, n := personNameForm(c)
	if n == 0 {
		return Value{}, ErrNoRoom
	}
	s := newStream(h)
	if pair {
		g, sn := pairAt(givenNames, surnames, 1, room(c), s.intn(n))
		return Value{Text: titleASCII(g) + " " + titleASCII(sn)}, nil
	}
	return Value{Text: titleASCII(list.words[s.intn(n)])}, nil
}

// ---------- address ----------

const (
	addrNumberDigits = 4    // 1000 to 9999
	addrNumberDomain = 9000 // the four-digit numbers with no leading zero
	addrCodeMax      = 7    // the widest postcode-shaped token
	addrCodeAlphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"
)

// addressMasker emits a street address when the column is wide enough for one,
// and an alphanumeric code when it is not — which is the shape a postal_code
// or zip column has. The category covers both (the rule pack's address pattern
// matches street, city and postcode names alike) and Constraints carries no
// column name, so the width is the only signal there is; mask/CLAUDE.md records
// that.
type addressMasker struct{}

func addressStreetBudget(c Constraints) int {
	return fitBudget(room(c), addrNumberDigits+1)
}

func addressCodeLen(c Constraints) int {
	n := addrCodeMax
	if b := room(c); b > 0 {
		n = min(n, b)
	}
	return n
}

func (addressMasker) Domain(c Constraints) int64 {
	if d, ok := labelDomain(c); ok {
		return d
	}
	if n := pairCount(streets, suffixes, 1, addressStreetBudget(c)); n > 0 {
		return satMul(addrNumberDomain, n)
	}
	if n := addressCodeLen(c); n > 0 {
		return satPow(int64(len(addrCodeAlphabet)), n)
	}
	return 0
}

func (addressMasker) Mask(h [32]byte, _ Value, c Constraints) (Value, error) {
	// A country or region column constrained to a list of codes is an address
	// column too: the rule pack's address pattern matches province, county and
	// postal_code alike, and 9393 Elm Mews fails such a CHECK with 23514.
	if v, ok := labelValue(h, c); ok {
		return v, nil
	}
	s := newStream(h)
	if budget := addressStreetBudget(c); pairCount(streets, suffixes, 1, budget) > 0 {
		number := s.digits(addrNumberDigits, false)
		st, sfx := pairAt(streets, suffixes, 1, budget, s.intn(pairCount(streets, suffixes, 1, budget)))
		return Value{Text: number + " " + titleASCII(st) + " " + titleASCII(sfx)}, nil
	}
	n := addressCodeLen(c)
	if n <= 0 {
		return Value{}, ErrNoRoom
	}
	out := make([]byte, n)
	for i := range out {
		out[i] = addrCodeAlphabet[s.intn(int64(len(addrCodeAlphabet)))]
	}
	return Value{Text: string(out)}, nil
}

// ---------- geo ----------

// geoMasker emits a signed decimal inside ±90, which is admissible for a
// latitude and for a longitude, so one generator serves both without a column
// name to tell them apart.
type geoMasker struct{}

const geoDecimals = 6

func geoPlaces(c Constraints) int {
	if numericFamily(c.TypeTag) || c.MaxLen <= 0 {
		return geoDecimals
	}
	// "-90." is four characters before the decimals.
	d := c.MaxLen - 4
	if d < 0 {
		return -1
	}
	return min(geoDecimals, d)
}

func pow10(n int) int64 { return satPow(10, n) }

func (geoMasker) Domain(c Constraints) int64 {
	if n, ok := labelDomain(c); ok {
		return n
	}
	d := geoPlaces(c)
	if d < 0 {
		return 0
	}
	return satMul(180, pow10(d)) + 1
}

func (geoMasker) Mask(h [32]byte, _ Value, c Constraints) (Value, error) {
	if v, ok := labelValue(h, c); ok {
		return v, nil
	}
	d := geoPlaces(c)
	if d < 0 {
		return Value{}, ErrNoRoom
	}
	s := newStream(h)
	scale := pow10(d)
	n := s.intn(satMul(180, scale)+1) - satMul(90, scale)
	sign := ""
	if n < 0 {
		sign, n = "-", -n
	}
	out := sign + itoa(n/scale)
	if d > 0 {
		out += "." + pad(n%scale, d)
	}
	return Value{Text: out}, nil
}

// ---------- free_text ----------

// freeTextCap is the ceiling on a filler's length whatever the column allows,
// so that a text column does not get a four-kilobyte value where the source
// held a sentence. Since T-0328 it bounds only the one branch that still draws
// its length from h: an input longer than the column allows.
const freeTextCap = 4096

// freeTextExact is the exact length a CHECK requires, or 0 when there is
// none or it does not fit the column: a char_length CHECK wider than the
// column's own varchar(n) is a contradiction no row could ever satisfy, and
// treating it as the target length would make Mask emit a value longer than
// the column can hold instead of falling back to the ranged filler below.
func freeTextExact(c Constraints) int {
	if l := checkLength(c); l > 0 && (c.MaxLen <= 0 || l <= c.MaxLen) {
		return l
	}
	return 0
}

func freeTextMax(c Constraints) int {
	n := freeTextCap
	if c.MaxLen > 0 {
		n = min(n, c.MaxLen)
	}
	return n
}

// fillerInitials is how many distinct first letters the words free_text's
// input-fitted branches draw from have (21 over today's fittedFillers). It is
// the number of one-character values the narrow branch below can emit, and
// one character is that branch's narrowest length: a longer output cut to its
// first character is one of these, so no length emits fewer.
var fillerInitials = func() int {
	seen := map[byte]bool{}
	for _, w := range fittedFillers.words {
		seen[w[0]] = true
	}
	return len(seen)
}()

// freeTextInputLen is the length a masked value is fitted to (T-0328): the rune
// count of the input's canonical form — fold, which is what Canonical hashes
// for free_text — and never less than one. The canonical form alone, so that
// every pair of spellings that hash alike ("Admin " and "admin", "STRASSE"
// and "straße", "file" and its ligature spelling) is also fitted alike, as
// fold promises. NFKC and case folding can make it longer than the raw value
// (ß becomes ss, the ﬁ ligature fi); when that takes it past the column,
// Mask falls into its over-long branch, a length drawn from h inside the
// column, which is also one output for every such spelling. A
// whitespace-only value folds to nothing and is fitted to one, so it is
// still replaced.
func freeTextInputLen(in Value) int {
	return max(1, utf8.RuneCountInString(fold(in.Text)))
}

// freeTextMasker replaces the value with filler about as long as the input
// (T-0328, the maintainer's decision in T-0323): a role column holding admin,
// user and guest masks to three short values, not to three column-width
// paragraphs. Every character is still drawn from h; what the input decides is
// the length alone, and only approximately in a column wide enough for every
// filler word:
//
//   - a column that holds the longest filler word (nine characters) gets whole
//     words, one space apart, from the first word boundary at or past the
//     input's length and never past max(input length, nine) — a one-character
//     input becomes one word, a sentence a run of words a few characters
//     shorter than it at most (fillerAbout);
//   - a narrower column gets exactly the input's length, cut from the same
//     words (fillerFrom), because a whole word may not fit;
//   - an input whose canonical form is longer than the column allows (over
//     freeTextCap in an unbounded text column, or a value NFKC or case
//     folding lengthened past its varchar's width) keeps the column-width
//     behaviour: a length drawn from h inside [1, min(column, 4096)];
//   - an exact-length CHECK keeps its length, as before.
//
// The first two draw from fittedFillers, the filler words less the ones an
// application is likely to store as a state or level of its own (trace,
// default); the last two from the whole list, byte-identical to before.
//
// Nothing in the first two depends on the column's width beyond which side of
// nine it is, and a canonical form no longer than the raw value fits every
// column that holds it: two foreign-key-linked columns holding one value mask
// it alike whenever they fall on the same side, and their Domain() agrees
// exactly then, which is the question internal/plan's equality-group check
// asks. The one exception is a value whose canonical form folding made longer
// than the narrower of two such columns (ß, a ligature): that column takes
// the over-long branch and the other does not.
type freeTextMasker struct{}

// Domain is the narrowest length's count, because the length now comes from
// the value and Domain is handed only the column (mask/CLAUDE.md, "a generator
// that branches on the value reports the narrowest branch the column admits"):
// a one-character input is one whole filler word, len(fittedFillers.words)
// values, in a column of nine characters or more, and one letter,
// fillerInitials values, in a narrower one. Every longer input has at least as
// many outputs, since its first word or first character already distinguishes
// them. So a free_text column's admissible domain is 80 or 21 whatever its
// width, where
// it grew with the width before; the unique-index rule refused every unique
// free_text column at every row count before this as well (the widest,
// unbounded text, was 368,195 against d_required = 500,000 at one row).
func (freeTextMasker) Domain(c Constraints) int64 {
	if d, ok := labelDomain(c); ok {
		return d
	}
	if l := freeTextExact(c); l > 0 {
		return int64(max(1, fillers.count(l)))
	}
	m := freeTextMax(c)
	if m < 1 {
		return 0
	}
	if m < fittedFillers.longest() {
		return int64(fillerInitials)
	}
	return int64(len(fittedFillers.words))
}

func (freeTextMasker) Mask(h [32]byte, in Value, c Constraints) (Value, error) {
	if v, ok := labelValue(h, c); ok {
		return v, nil
	}
	s := newStream(h)
	if l := freeTextExact(c); l > 0 {
		return Value{Text: filler(s, l)}, nil
	}
	maxLen := freeTextMax(c)
	if maxLen < 1 {
		return Value{}, ErrNoRoom
	}
	n := freeTextInputLen(in)
	switch {
	case n > maxLen:
		return Value{Text: filler(s, 1+int(s.intn(int64(maxLen))))}, nil
	case maxLen < fittedFillers.longest():
		return Value{Text: fillerFrom(fittedFillers, s, n)}, nil
	default:
		return Value{Text: fillerAbout(s, n)}, nil
	}
}

// fillerRangeDomain is the lower bound on what a filler whose length is drawn
// from h inside [1, freeTextMax(c)] emits: for each admissible length, at least
// as many distinct values as there are filler words short enough to open one,
// and outputs of different lengths differ. It was free_text's own Domain until
// T-0328; special_category's generic text branch and semi_structured's string
// leaves still draw their length that way, so they keep it.
func fillerRangeDomain(c Constraints) int64 {
	if d, ok := labelDomain(c); ok {
		return d
	}
	if l := freeTextExact(c); l > 0 {
		return int64(max(1, fillers.count(l)))
	}
	var total int64
	for l := 1; l <= freeTextMax(c); l++ {
		total = satAdd(total, int64(max(1, fillers.count(l))))
	}
	return total
}

// filler builds exactly n bytes of neutral words.
func filler(s *stream, n int) string { return fillerFrom(fillers, s, n) }

// fillerFrom builds exactly n bytes of words drawn from w.
func fillerFrom(w *wordList, s *stream, n int) string {
	var b strings.Builder
	b.Grow(n + 16)
	for b.Len() < n {
		if b.Len() > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(w.words[s.intn(int64(len(w.words)))])
	}
	out := b.String()[:n]
	if strings.HasSuffix(out, " ") {
		out = out[:n-1] + "x"
	}
	return out
}

// fillerAbout builds whole filler words, one space apart, about n characters
// long: it stops at the first word boundary at or past n, and never runs past
// limit = max(n, the longest filler word), so the result is between n-4 and
// limit characters long and fits any column that holds both the input and the
// longest word. Its words come from fittedFillers: the first from the whole
// of it, a later one only from the words that still fit under limit.
func fillerAbout(s *stream, n int) string {
	w := fittedFillers
	limit := max(n, w.longest())
	var b strings.Builder
	b.Grow(limit)
	for b.Len() < n {
		room := limit - b.Len()
		if b.Len() > 0 {
			room-- // the separating space
		}
		if room < w.shortest() {
			break
		}
		k := w.count(room)
		if b.Len() > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(w.words[s.intn(int64(k))])
	}
	return b.String()
}

// ---------- online_id ----------

const (
	urlPrefix = "https://example.invalid/"
	uuidLen   = 36
)

type onlineShape int

const (
	onlineNone onlineShape = iota
	onlineUUID
	onlineURL
	onlineHandle
)

func onlineShapeFor(in Value, c Constraints) (onlineShape, int) {
	if c.TypeTag == famUUID {
		return onlineUUID, 0
	}
	t := strings.TrimSpace(in.Text)
	if reUUID.MatchString(t) && fits(strings.Repeat("x", uuidLen), c) {
		return onlineUUID, 0
	}
	if strings.Contains(t, "://") || strings.HasPrefix(strings.ToLower(t), "www.") {
		if n := urlTail(c); n >= 1 {
			return onlineURL, n
		}
	}
	// A handle: one filler word, an underscore and a base32 tail.
	n := handleTail(c)
	if n < 1 {
		return onlineNone, 0
	}
	return onlineHandle, n
}

// urlTail is how many base32 symbols fit after https://example.invalid/,
// capped at the 65 bits a unique column needs. Below one symbol the URL branch
// is unreachable and a URL-shaped value falls through to the handle.
func urlTail(c Constraints) int {
	n := uniqueSuffix
	if b := room(c); b > 0 {
		n = min(n, b-len(urlPrefix))
	}
	return n
}

// handleTail is how many base32 symbols fit after the shortest filler word and
// its underscore, capped at the 65 bits a unique column needs.
func handleTail(c Constraints) int {
	n := uniqueSuffix
	if b := room(c); b > 0 {
		n = min(n, b-fillers.shortest()-1)
	}
	if n < 1 || fillers.count(fitBudget(room(c), n+1)) == 0 {
		return 0
	}
	return n
}

// onlineIDMasker replaces a username, handle, device id or URL. A URL keeps
// only its shape and lands under example.invalid with a path drawn from h; a
// UUID column gets a UUID; everything else gets a filler word and a base32
// tail. Every branch is at least 2⁶⁵ wide when the column allows it, so a
// unique online_id column needs no separate generator.
type onlineIDMasker struct{}

func (onlineIDMasker) Domain(c Constraints) int64 {
	if d, ok := labelDomain(c); ok {
		return d
	}
	// Mask picks its branch from the *value*, and the column can hold any shape
	// the branches reach, so the honest figure is the narrowest of them and not
	// the handle's alone. A bounded varchar is where they come apart: at 25
	// characters the URL branch has one base32 symbol left and emits 32 values
	// while the handle branch emits billions, and reporting the handle's figure
	// is the over-reporting mask/CLAUDE.md calls the one forbidden direction.
	if c.TypeTag == famUUID {
		return satPow(2, 122)
	}
	n := handleTail(c)
	if n < 1 {
		// No handle fits, so no value fits: a handle is the branch every value
		// that is neither a UUID nor a URL takes.
		return 0
	}
	d := satMul(
		int64(fillers.count(fitBudget(room(c), n+1))),
		satPow(int64(len(b32alphabet)), n),
	)
	if u := urlTail(c); u >= 1 {
		d = min(d, satPow(int64(len(b32alphabet)), u))
	}
	if fits(strings.Repeat("x", uuidLen), c) {
		d = min(d, satPow(2, 122))
	}
	return d
}

func (onlineIDMasker) Mask(h [32]byte, in Value, c Constraints) (Value, error) {
	if v, ok := labelValue(h, c); ok {
		return v, nil
	}
	shape, n := onlineShapeFor(in, c)
	s := newStream(h)
	switch shape {
	case onlineUUID:
		return Value{Text: uuidFrom(s)}, nil
	case onlineURL:
		return Value{Text: urlPrefix + s.base32(n)}, nil
	case onlineHandle:
		count := fillers.count(fitBudget(room(c), n+1))
		if count == 0 {
			return Value{}, ErrNoRoom
		}
		w := fillers.words[s.intn(int64(count))]
		return Value{Text: w + "_" + s.base32(n)}, nil
	case onlineNone:
		return Value{}, ErrNoRoom
	}
	return Value{}, ErrNoRoom
}

func uuidFrom(s *stream) string {
	b := make([]byte, 16)
	s.read(b)
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 1
	const hexes = "0123456789abcdef"
	out := make([]byte, 0, uuidLen)
	for i, x := range b {
		if i == 4 || i == 6 || i == 8 || i == 10 {
			out = append(out, '-')
		}
		out = append(out, hexes[x>>4], hexes[x&0x0f])
	}
	return string(out)
}

// ---------- special_category ----------

// specialCategoryMasker is the one generator that refuses to pretend. A
// special category over a small admissible domain — eight diagnoses, three
// marital statuses, a boolean — is a stable substitution that frequency
// recovers, so instead of substituting it collapses the column to one fixed
// value and the explanation says "substitution over N values is not a mask"
// (ARCHITECTURE.md section 5). Over a domain that is not small it substitutes
// like any other category.
type specialCategoryMasker struct{}

// specialCollapses is section 5's small-domain rule read off the column rather
// than off the samples. Constraints.Distinct is 0 when nobody counted — section
// 4 makes that path normal, "special categories by name alone → certain", and a
// partitioned table with no leaves has no samples at all — and a collapse that
// switches itself off at a zero value is a control that defaults to off: an
// hiv_status enum would then be substituted 1:1 over its own two labels and
// half the rows would carry their true value under a column the report calls
// masked (THREAT_MODEL.md T1, T12). So the unknown case decides on the domain,
// which is what section 5's "at small d" is a property of, and fails closed.
func specialCollapses(c Constraints) bool {
	return smallDomain(genericDomain(c), c)
}

func (specialCategoryMasker) Domain(c Constraints) int64 {
	if specialCollapses(c) {
		return 1
	}
	return genericDomain(c)
}

func (specialCategoryMasker) Mask(h [32]byte, _ Value, c Constraints) (Value, error) {
	if specialCollapses(c) {
		return collapsed(c), nil
	}
	return generic(newStream(h), c)
}

// collapsed is the one fixed value a small special category becomes: the first
// enum label, or NULL when the column takes one, or the type's zero value.
func collapsed(c Constraints) Value {
	if l := labels(c); len(l) > 0 {
		return Value{Text: l[0]}
	}
	if c.Nullable {
		return Value{Null: true}
	}
	return zeroValue(c)
}
