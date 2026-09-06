// SPDX-License-Identifier: Apache-2.0

package mask

import (
	"sort"
	"strings"
)

// The embedded word lists. They are part of the mapping, because K_cat is
// derived from the category and the generator indexes into these lists: adding
// a word changes every masked value in the categories that draw from it, which
// is a release note and a "classification changed" line, not a tidy-up
// (ARCHITECTURE.md section 5 "Determinism scope"). They are ours rather than
// gofakeit's for exactly that reason (ADR-006 "Not dependencies").
//
// Everything here is ASCII and lower case. A generator applies whatever
// capitalisation the output shape wants, so the same list serves an email
// local part and a person's name.
const (
	givenWords = `alice amara amelia andre anna anton arjun asha aurora ayla bram bruno camila
carlos cato chloe clara cyrus dalia daniel dario dawit dilan dmitri eda eli elena elias elin
emeka emil emma enzo esme ewan farah felix fiona freya gabriel gita greta hana hassan hector
helena hugo ida idris ilya ines ira isaac ivan jamal jana jasper javier jonas jorge josef juno
kai kaito kamil karim kasia keiko kenji khalid kira klara lars laila leon lena liam lina livia
lucas luisa mads maja malik marek maria mateo maya mei milan mira mirza nadia nasir nina noor
nuru olga oliver omar oscar otto paulo petra pia priya quinn rafael rania ravi rhea rita rosa
ruben ruth sami sana sasha selma sergei simon sofia soren stefan suri tariq tessa theo tomas
tova uma vera viktor vlad wren yara yosef yuki zaid zara zoe`

	surnameWords = `abara adler ahmed alvarez andersen antonov arnaud bakker baros bauer bennett
berg bianchi blanco boateng borg bosch brandt cabrera campos castro cheng cohen conti cortez
costa dahl dalgaard delgado diallo dimas dubois duran egan engel eriksen esposito faber farrell
fischer fontaine fowler gallo garcia gomez greco gruber haas hansen hayes herrera hoffman horvat
ibrahim iversen jansen jelinek jensen kaminski kane keller khan klein koch kovac kruger lambert
larsen lawson leclerc lehman leroy lindqvist lopez lozano lund madsen mahmoud marino martel
mendes meyer mitchell moreau moretti morales mueller nakamura navarro nielsen novak nowak okafor
olsen ortiz osei palmer pavlov pereira petrov popescu porter ramos reyes richter rivera roche
romano rossi roux ruiz sadiq salazar sandu santos sato schmidt schulz serrano silva simic
sinclair soares sorensen stein suzuki tanaka tavares thomsen torres tran ueda vargas vega verner
vidal vogel wagner walsh weber wilson yamada yilmaz zhang ziegler`

	streetWords = `alder ash aspen beech birch bramble briar cedar cherry chestnut clover cypress
dogwood elder elm fern fir ginkgo hazel heather hemlock hickory holly ivy juniper larch laurel
lilac linden magnolia maple mulberry oak olive orchard palm pine poplar primrose quince redwood
rowan sage sequoia spruce sumac sycamore tamarack teak thistle thorn tupelo walnut willow yew`

	suffixWords = `avenue close court crescent drive gardens grove lane mews place road row street
terrace walk way`

	fillerWords = `aggregate batch buffer cadence cluster column cycle dataset default delta entry
factor field filter format gradient header index input interval item label layer ledger level
limit margin marker matrix measure method metric module node offset option output packet panel
parcel pattern phase pointer policy prefix process profile queue range record region report
request result routine sample scalar schema scope sector segment sequence series session signal
slice socket source spectrum stack stage stream string subject summary symbol system table target
template thread token topic trace unit value vector vertex volume window`
)

// nanpAreaCodes are the North American area codes for which libphonenumber
// accepts +1 <area> 555 01NN, the range reserved for fiction. The list is
// embedded rather than probed at start-up on purpose: probing would make the
// masked value depend on the phonenumbers version, so a dependency bump would
// silently remap every phone column. TestNANPAreaCodesAreStillValid checks the
// list against the library instead, so a bump that invalidates one is loud.
const nanpAreaCodes = `201202203204205206207208209210212213214215216217218219220223` +
	`224225226227228229231234235236239240248249250251252253254256` +
	`257260262263267269270272276279281283289301302303304305306307` +
	`308309310312313314315316317318319320321323324325326327329330` +
	`331332334336337339340341343346347350351352353354360361363364` +
	`365367368369380382385386401402403404405406407408409410412413` +
	`414415416417418419423424425428430431432434435437438440442443` +
	`445447448450458463464468469470474475478479480484500501502503` +
	`504505506507508509510512513514515516517518519520521522525526` +
	`527528529530531532533534539540541544548551557559561562563564` +
	`566567570571572573574575577579580581582584585586587588600601` +
	`602603604605606607608609610612613614615616617618619620622623` +
	`626628629630631633636639640641645646647650651656657658659660` +
	`661662667669670671672678680681682683686689701702703704705706` +
	`707708709712713714715716717718719720724725726727728730731732` +
	`734737738740742743747748753754757760762763765769770771772773` +
	`774775778779780781782784785786787800801802803804805806807808` +
	`809810812813814815816817818819820821825826828829830831832833` +
	`835838839840843844845847848849850854855856857858859860862863` +
	`864865866867870872873876877878879888900901902903904905906907` +
	`908909910912913914915916917918919920925928929930931934936937` +
	`938939940941942943945947948949951952954956959970971972973975` +
	`978979980984985986989`

// maxFit is the largest length the fit table indexes. Every embedded word is
// far shorter; a budget above it is treated as unbounded.
const maxFit = 32

// wordList is an embedded list ordered by length, so the words that fit a
// budget are always a prefix of it and both the count and the choice are O(1).
type wordList struct {
	words []string
	fit   [maxFit + 1]int // fit[m] = how many words are at most m bytes long
}

func newWordList(s string) *wordList {
	w := &wordList{words: strings.Fields(s)}
	sort.Slice(w.words, func(i, j int) bool {
		if len(w.words[i]) != len(w.words[j]) {
			return len(w.words[i]) < len(w.words[j])
		}
		return w.words[i] < w.words[j]
	})
	for m := 0; m <= maxFit; m++ {
		n := 0
		for _, x := range w.words {
			if len(x) <= m {
				n++
			}
		}
		w.fit[m] = n
	}
	return w
}

// count is how many words fit a budget. A budget of 0 means unbounded and a
// negative budget means the caller already knows nothing fits.
func (w *wordList) count(budget int) int {
	if budget < 0 {
		return 0
	}
	if budget == 0 || budget > maxFit {
		return len(w.words)
	}
	return w.fit[budget]
}

// longest is the length of the longest word in the list.
func (w *wordList) longest() int { return len(w.words[len(w.words)-1]) }

// shortest is the length of the shortest word in the list.
func (w *wordList) shortest() int { return len(w.words[0]) }

var (
	givenNames  = newWordList(givenWords)
	surnames    = newWordList(surnameWords)
	streets     = newWordList(streetWords)
	suffixes    = newWordList(suffixWords)
	fillers     = newWordList(fillerWords)
	areaCodes   = splitThrees(nanpAreaCodes)
	emailHosts  = []string{"example.com", "example.net", "example.org"}
	docPrefixes = []string{"192.0.2.", "198.51.100.", "203.0.113."}
)

func splitThrees(s string) []string {
	out := make([]string, 0, len(s)/3)
	for i := 0; i+3 <= len(s); i += 3 {
		out = append(out, s[i:i+3])
	}
	return out
}

// pairCount is how many (a, b) pairs fit budget bytes with a separator of
// sepLen between them. A budget of 0 means unbounded, a negative one nothing.
func pairCount(a, b *wordList, sepLen, budget int) int64 {
	if budget < 0 {
		return 0
	}
	if budget == 0 || budget >= a.longest()+sepLen+b.longest() {
		return int64(len(a.words)) * int64(len(b.words))
	}
	var total int64
	for _, x := range a.words {
		room := budget - sepLen - len(x)
		if room < b.shortest() {
			break // words are ordered by length, so nothing after x fits either
		}
		total += int64(b.count(room))
	}
	return total
}

// pairAt returns the n-th pair in the same order pairCount counts them. n must
// be in [0, pairCount).
func pairAt(a, b *wordList, sepLen, budget int, n int64) (string, string) {
	if budget == 0 || budget >= a.longest()+sepLen+b.longest() {
		wide := int64(len(b.words))
		return a.words[n/wide], b.words[n%wide]
	}
	for _, x := range a.words {
		room := budget - sepLen - len(x)
		if room < b.shortest() {
			break
		}
		k := int64(b.count(room))
		if n < k {
			return x, b.words[n]
		}
		n -= k
	}
	// Unreachable for an n below pairCount; the shortest pair is the honest
	// fallback rather than a panic in a masking path.
	return a.words[0], b.words[0]
}
