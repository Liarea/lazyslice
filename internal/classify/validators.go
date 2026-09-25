// SPDX-License-Identifier: Apache-2.0

package classify

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
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

// minSecretSamples is how many non-NULL samples the entropy validator
// (textsig.LooksSecret, `credential`) needs before its ratio may decide a
// column (tracker T-0315). Every other validator decides at any count once it
// reaches validatorThreshold — below minSamples that is the T-0058 fail-closed
// reading, "both of two values parse as an email address" — but an entropy
// guess over one to four values is not the same claim as a parse over them:
// dogfood session 1 masked two columns holding one and four ordinary values to
// the credential literal on exactly that. Below it the validator can still
// record `low` (sig.weak, at minSamples or more) for the neighbouring-column
// rule to read, and a column whose name says credential is masked on the name
// as before; what is given up is an unnamed column of one to four real secrets,
// THREAT_MODEL.md T1's T-0315 amendment. internal/verify's credential entry
// carries the same number, so the second net does not refuse a column this
// package deliberately left unmasked.
const minSecretSamples = 5

// entropyExemptNames are the normalised column names whose values the entropy
// validator is not asked about at all (tracker T-0315). A Rails
// single-table-inheritance `type` column, its `klass` spelling, and a
// `component_name` hold a class or component name the application
// constantizes on every read: "ScheduledReportExport" clears the entropy floor
// with no namespace for textsig's guard to see, and the credential literal in
// its place raised on every row of the dogfood copy. The list is exact names
// only, not a pattern: a `password_type` or `token_type` column is decided by
// the credential name rule before this is read, and a polymorphic
// `commentable_type` is outside what T-0315 settled.
//
// Only the entropy validator is dropped; every other one still runs, and the
// name rules still apply, so a `type` column of email addresses is still an
// email column. internal/verify/validators.go's credential entry carries the
// same list, and the two are kept in step by hand, as that file's validator
// list already is with this package's.
var entropyExemptNames = map[string]bool{
	"type":           true,
	"klass":          true,
	"component_name": true,
}

// namedFileNames reports whether a column's samples are file names enough of
// which name a person for the whole column to be read as one (tracker T-0315).
// textsig.LooksSecret no longer reads a file name as a secret unless its stem
// carries a word from the name dictionary, which answers for one value; a
// column is a different question. The rails-activestorage fixture's
// `aoife-byrne-passport-3.pdf` carries a dictionary name in 72 of every 100
// rows and mastodon's `photo-<username>-<n>.jpg` in about half, because the
// dictionary does not hold every name: counted value by value, both columns
// fall under validatorThreshold and the other rows' names are copied with the
// column. So when at least nameCorroborationThreshold of the samples are file
// names whose stem carries a dictionary word -- the share T-0313's bare-name
// rule reads as a column of people's names -- bestSignal counts every file
// name in the column as the entropy validator's hit, as it did before T-0315,
// and the column is masked as `credential` with its own reason phrase. A column
// of screenshots, exports and release files carries none and is spared.
func namedFileNames(dict *textsig.Dict, values []string) bool {
	if len(values) == 0 {
		return false
	}
	named := 0
	for _, s := range values {
		if stem, ok := textsig.FileNameStem(s); ok && dict.ContainsName(stem) {
			named++
		}
	}
	return named > 0 && float64(named)/float64(len(values)) >= nameCorroborationThreshold
}

// withoutSecrets returns vs without the entropy validator, for a column
// entropyExemptNames names.
func withoutSecrets(vs []validatorEntry) []validatorEntry {
	out := make([]validatorEntry, 0, len(vs))
	for _, v := range vs {
		if v.cat != pipeline.CatCredential {
			out = append(out, v)
		}
	}
	return out
}

// identifierNameWords are the last words of a normalised column name that say
// the column holds an identifier of its own (tracker T-0316): an `id`, a
// `number`, a `version`, a `reference`, and their usual abbreviations.
// Dogfood session 1 masked eight such columns -- CRM and billing customer
// ids, subscription ids, invoice and estimate numbers, a schema_migrations
// `version` -- as free_text on a minority of values that passed the card
// check by chance. On a column named like this, the card entry asks
// textsig.CardShape (an issuer prefix and the length that issuer issues)
// instead of textsig.ValidCard (an issuer prefix at any length from twelve to
// nineteen): the name says the digits are the application's own, so a value
// must look like a card in every respect before it outweighs that. A real card
// number in such a column still passes CardShape and is masked as before.
//
// internal/verify/validators.go keeps the same list (cardIdentifierWords),
// by hand, so the second net does not refuse a column this package left
// unmasked on the same evidence.
var identifierNameWords = map[string]bool{
	"id":        true,
	"number":    true,
	"num":       true,
	"no":        true,
	"nr":        true,
	"version":   true,
	"ref":       true,
	"reference": true,
}

// identifierNamed reports whether a normaliseName'd column name ends in one of
// identifierNameWords.
func identifierNamed(normalised string) bool {
	last := normalised
	if i := strings.LastIndexByte(normalised, '_'); i >= 0 {
		last = normalised[i+1:]
	}
	return identifierNameWords[last]
}

// withCardShape returns vs with the card entry's check replaced by
// textsig.CardShape, for a column identifierNamed names (T-0316).
func withCardShape(vs []validatorEntry) []validatorEntry {
	out := make([]validatorEntry, len(vs))
	copy(out, vs)
	for i := range out {
		if out[i].phrase == phraseLuhn {
			out[i].ok = func(_ *textsig.Dict, s string) bool { return textsig.CardShape(s) }
		}
	}
	return out
}

// networkIDVetoWords are the normalised name tokens that say a column holds
// the application's own version, build or release identifier rather than a
// network address (tracker T-0317). Dogfood session 1 masked
// `last_player_version` as free_text: 93 of 168 samples are dot-separated
// integers of the same shape an IPv4 address is ("1.2.3.4"), so
// textsig.ValidIP parsed them and bestSignal's strongHit branch masked the
// column on a strong validator's minority hit. A version string is not a
// network address whatever its digits happen to parse as. See
// networkIDVetoed and withoutNetworkID, read in state.base.
var networkIDVetoWords = map[string]bool{
	"version": true,
	"build":   true,
	"release": true,
}

// networkIDVetoed reports whether any underscore-separated token of a
// normaliseName'd column name is a networkIDVetoWords word. Unlike
// identifierNamed (T-0316), which asks only about the last token because a
// card check is about what the whole value is, this checks every token: an
// `app_version_code` column carries the word in the middle, and the same
// coincidental IP shape applies there as it does at the end of a name.
func networkIDVetoed(normalised string) bool {
	for _, tok := range strings.Split(normalised, "_") {
		if networkIDVetoWords[tok] {
			return true
		}
	}
	return false
}

// withoutNetworkID returns vs with the network_id entries (IP, MAC) no longer
// marked strong, for a column networkIDVetoed names (T-0317 review round,
// finding 2). The first landing removed the entries outright, which also
// stopped a version-named column from being masked network_id when *all* of
// its values are genuine addresses -- probed with a `build_host` column of
// five real public IPv4s in a table with no certain neighbour: it masked
// network_id before this change and reached "nothing recognised" after,
// unmasked, because the veto did not depend on the values at all.
//
// bestSignal's ratio branch (the `ratio >= validatorThreshold` case) does not
// read v.strong; only the strongHit branch below validatorThreshold does
// (T-0136 finding 7). So a column that genuinely clears validatorThreshold on
// IP or MAC -- a majority of its samples really are addresses -- is unaffected
// and still masks network_id. What this withholds is only the minority
// strongHit path: a coincidental IP-shaped minority, the dogfood shape
// (93/168), no longer decides the column at all. Nothing else about the
// validator list changes, so a version column that also carries a real email
// or phone signal is still decided by that.
func withoutNetworkID(vs []validatorEntry) []validatorEntry {
	out := make([]validatorEntry, 0, len(vs))
	for _, v := range vs {
		if v.cat == pipeline.CatNetworkID {
			v.strong = false
		}
		out = append(out, v)
	}
	return out
}

// phoneGuessVetoWords are the normalised name tokens that say a column holds
// a key, code, license, serial or token of the application's own rather than
// a telephone number (tracker T-0317). Dogfood session 1 masked
// `license_key` as phone: its ten-digit samples cleared a guessed region's
// numbering plan on 8 of 10 rows (guessedPhoneHit, T-0221), and
// guessedPhoneColumns masked it on that corroborated guess alone -- the
// wrong shape for a key, whatever some numbering plan makes of its digits.
// See phoneGuessVetoed, read in state.base beside guessedPhoneHit.
var phoneGuessVetoWords = map[string]bool{
	"key":     true,
	"code":    true,
	"license": true,
	"serial":  true,
	"token":   true,
}

// phoneGuessVetoed reports whether any underscore-separated token of a
// normaliseName'd column name is a phoneGuessVetoWords word. It is checked
// only by the guessed-region fallback (base, guessedPhoneHit): a column whose
// name already matches rules.yml's own phone pattern is decided by that name
// rule first and never reaches this gate at all (guessedPhoneColumns' own
// comment on why that arm can never fire).
func phoneGuessVetoed(normalised string) bool {
	for _, tok := range strings.Split(normalised, "_") {
		if phoneGuessVetoWords[tok] {
			return true
		}
	}
	return false
}

// nameCorroborationThreshold is the share of a bare name column's samples
// that must carry a word from the name dictionary (textsig.Dict.ContainsName)
// before the samples corroborate the rule pack's bare_name rule (T-0313).
//
// It is deliberately far below weakThreshold, because the column's name is
// already half the evidence and the question is only whether the values
// contradict it. Measured on hand-made sample sets when it was chosen: label
// columns of the dogfood-session-1 kind -- tags, folders, roles, AI models,
// triggers, playlists, dashboard widgets, project names, company names,
// country names, pagila's own category and language names -- carry a
// dictionary word in 0% to 12% of their values (a language list's "Español"
// and "Deutsch"; a tag called "green"), and people's names carry one in 30% to
// 100% (an English list 100%, a fifteen-country list 80%, a list of names the
// dictionary mostly lacks, with titles and initials, 30%). A column of names in
// a script the dictionary does not carry scores 0% and is THREAT_MODEL.md T1's
// stated residual; bare_name_test.go pins both directions.
const nameCorroborationThreshold = 0.2

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

// jsonKeyLimit bounds how many distinct keys one column's map may hold. A key
// past it is simply not in the map, which internal/transform reads as a key the
// samples never showed: every leaf beneath it is masked (pipeline.Decision's
// LeafKeys). A column whose documents are keyed by identifiers — one key per
// customer — reaches it quickly, and that is the shape where copying anything
// on the strength of a key name is least safe anyway.
//
// Which keys are kept past the limit is decided in sorted order over every key
// the samples showed, never in the order a Go map or the sampler hands them
// over (T-0272 review round, finding 3; ARCHITECTURE.md's determinism rule),
// so one sample set always gives one map.
const jsonKeyLimit = 4096

// jsonKeyCategories is the per-leaf half of a json or jsonb column's decision
// (T-0272, the maintainer's T-0143 decision of 2026-09-24): every object key
// in the sampled documents, at any depth up to jsonMaxDepth, mapped to the
// category the rule pack's name rules give it, or CatNone when no rule names
// it. It is what lets internal/transform copy a configuration leaf and still
// mask an email leaf with the email masker, and what lets internal/verify ask
// the same question of the target.
//
// It reads the raw samples and not scalarsOf's strings: pgx hands a json or
// jsonb sample back already decoded, as a map[string]any or a []any, and
// asText renders neither, so the string path sees nothing of an object
// document read from a real database.
//
// A key that parses as an email address, a phone number or a Luhn-valid
// number is left out. internal/transform masks exactly those keys
// (json.go's keyCategory, the same three textsig validators, unspaced, the
// phone one under region), so such a key is a value, not a name, and leaving
// it out means a leaf beneath it is always masked -- on the transform side
// under the source key and on the verify side under the masked one, which is
// never in the map either, because each of the three maskers emits a value
// its own validator accepts.
//
// region is the run's configured phone region (Classify's st.region, which
// becomes pipeline.Classification.PhoneRegion and is what transform's
// keyCategory reads, T-0394); empty is the international-only reading.
//
// nil when no sample held an object key, which internal/transform reads the
// same way as an empty map: nothing is known about any key, so every leaf is
// masked.
func jsonKeyCategories(p *compiledPack, samples []any, region string) map[string]pipeline.Category {
	seen := map[string]struct{}{}
	for _, v := range samples {
		doc, ok := decodeSampleDocument(v)
		if !ok {
			continue
		}
		collectJSONKeys(doc, seen, 0)
	}
	if len(seen) == 0 {
		return nil
	}
	keys := make([]string, 0, len(seen))
	for k := range seen {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := map[string]pipeline.Category{}
	for _, k := range keys {
		if len(out) == jsonKeyLimit {
			break
		}
		if strongKeyShape(k, region) {
			continue
		}
		cat := pipeline.CatNone
		if pat, ok := p.match(normaliseName(k)); ok {
			cat = pat.Category
		}
		out[k] = cat
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// decodeSampleDocument is jsonLeaves' decoding, for one raw sample.
func decodeSampleDocument(v any) (any, bool) {
	var doc any
	switch t := v.(type) {
	case nil:
		return nil, false
	case []byte:
		if json.Unmarshal(t, &doc) != nil {
			return nil, false
		}
	case string:
		if json.Unmarshal([]byte(t), &doc) != nil {
			return nil, false
		}
	default:
		doc = v
	}
	return doc, true
}

// collectJSONKeys adds every object key of v, to jsonMaxDepth, to seen. It
// only gathers: the order a map ranges in decides nothing, because
// jsonKeyCategories sorts the whole set before the limit is applied.
func collectJSONKeys(v any, seen map[string]struct{}, depth int) {
	if depth > jsonMaxDepth {
		return
	}
	switch t := v.(type) {
	case map[string]any:
		for k, child := range t {
			seen[k] = struct{}{}
			collectJSONKeys(child, seen, depth+1)
		}
	case []any:
		for _, child := range t {
			collectJSONKeys(child, seen, depth+1)
		}
	}
}

// strongKeyShape is internal/transform's keyCategory question (json.go):
// whether a key parses as an email address, a phone number or a Luhn-valid
// number, which is when transform masks the key itself. The phone question
// reads the run's configured region, as keyCategory does (T-0394): a
// national-format key ("07911 123456" under GB) is masked by transform, so it
// is a value and not a name, and a leaf beneath it must not be copied on the
// strength of a map entry for it.
func strongKeyShape(k, region string) bool {
	return textsig.ValidEmail(k) || textsig.ValidPhoneRegion(k, region) || textsig.ValidLuhn(k)
}

// guessedPhoneLeafKeys is the value half of jsonKeyCategories (T-0394, the
// 2026-09-25 JSON red team's A26): the entries of keys, in sorted order, whose
// string leaves across the samples parse as phone numbers under
// phoneGuessRegions at the scalar path's own ratio -- guessedPhoneHit's
// question, at least minSamples leaves and validatorThreshold of them
// matching. A leaf counts toward the nearest object key enclosing it, which
// is the key leafRule reads first, and an array's elements count toward the
// key holding the array. Only a key the map calls CatNone is asked: a key
// with a category already masks every leaf beneath it, and a key that is not
// in the map (a strong shape, or past jsonKeyLimit) is unknown, which masks
// them too.
//
// It decides nothing by itself. guessedPhoneLeafColumns gives these keys
// CatPhone only with the scalar path's corroboration, so that a document of
// ten-digit order numbers is not masked on a numbering plan's coincidence any
// more than a column of them is (guessedPhoneColumns). A configured region is
// not read here: leafValueCategory already masks every leaf that parses under
// it, one value at a time, without corroboration.
func guessedPhoneLeafKeys(samples []any, keys map[string]pipeline.Category) []string {
	if len(keys) == 0 {
		return nil
	}
	leaves := map[string][]string{}
	for _, v := range samples {
		doc, ok := decodeSampleDocument(v)
		if !ok {
			continue
		}
		collectKeyStrings(doc, "", leaves, 0)
	}
	names := make([]string, 0, len(leaves))
	for k := range leaves {
		if cat, ok := keys[k]; ok && cat == pipeline.CatNone {
			names = append(names, k)
		}
	}
	sort.Strings(names)
	var out []string
	for _, k := range names {
		if guessedPhoneRatio(leaves[k]) {
			out = append(out, k)
		}
	}
	return out
}

// guessedPhoneRatio is guessedPhoneHit's own test -- at least minSamples
// values and validatorThreshold of them matching some region in
// phoneGuessRegions -- stopping as soon as the misses already rule the ratio
// out, because a document column can hold far more leaves than a scalar
// column holds samples, and a key of words is decided on its first few.
func guessedPhoneRatio(values []string) bool {
	n := len(values)
	if n < minSamples {
		return false
	}
	missed := 0
	for _, v := range values {
		if matchesAnyGuessRegion(v) {
			continue
		}
		missed++
		// The best the rest can do is match every one; below the threshold
		// even then, the key is out. The comparison is guessedPhoneColumns'
		// own, over that best case.
		if float64(n-missed)/float64(n) < validatorThreshold {
			return false
		}
	}
	return true
}

// keyStringCap bounds how many string leaves collectKeyStrings keeps for one
// key. The ratio over the first keyStringCap is the ratio the key is judged
// by; the samples are already bounded, so this only bounds a document that
// repeats one key thousands of times inside arrays.
const keyStringCap = 4096

// collectKeyStrings adds every non-empty string leaf of v, to jsonMaxDepth,
// to out under the nearest object key enclosing it (key; "" for a leaf with
// none, which no map entry names). An object's keys are walked in sorted
// order, so which leaves keyStringCap keeps is the same on every run
// (ARCHITECTURE.md's determinism rule, the reason jsonKeyCategories sorts).
func collectKeyStrings(v any, key string, out map[string][]string, depth int) {
	if depth > jsonMaxDepth {
		return
	}
	switch t := v.(type) {
	case map[string]any:
		names := make([]string, 0, len(t))
		for k := range t {
			names = append(names, k)
		}
		sort.Strings(names)
		for _, k := range names {
			collectKeyStrings(t[k], k, out, depth+1)
		}
	case []any:
		for _, child := range t {
			collectKeyStrings(child, key, out, depth+1)
		}
	case string:
		if key != "" && t != "" && len(out[key]) < keyStringCap {
			out[key] = append(out[key], t)
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
