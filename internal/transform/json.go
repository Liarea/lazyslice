// SPDX-License-Identifier: Apache-2.0

package transform

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"sort"
	"strconv"
	"strings"

	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
	"github.com/Liarea/lazyslice/internal/textsig"
	"github.com/Liarea/lazyslice/mask"
)

// JSON leaf walking (ARCHITECTURE.md §4 "Free text and JSON", §6 item 1).
//
// Structure and key names are kept. Which scalar leaves are replaced is the
// decision's per-leaf map (pipeline.Decision.LeafKeys, T-0272, the
// maintainer's T-0143 decision of 2026-09-24), read by leafRule below: a leaf
// with a personal signal on its key or its value is replaced, a leaf with none
// under keys the samples showed is copied, and every other leaf is replaced as
// every leaf was before. The map is read through pipeline.Decision.LeafMap, so
// a column whose own decision names a personal category (a jsonb
// `medical_history`), whose own name matched a personal rule its type did not
// satisfy (a jsonb `full_name` or `passwords`, T-0393), or that an operator
// raised has no map, and every one of its leaves is replaced. A replaced string leaf goes through its category's
// masker (leafMasker) or free_text; a replaced number leaf becomes a number of
// the same kind derived from h; a replaced boolean leaf becomes a boolean
// derived from h; null stays null; arrays and objects keep their shape.
//
// Every masked leaf enters the residual filter *separately, keyed by its JSON
// path*, which is what makes one surviving leaf findable inside a document that
// otherwise changed (§6 item 1, testdata/README.md trap 16a). The whole-document
// masker in the mask module cannot do that — it has no filter and no path — and
// §12 puts JSON leaf walking here for exactly that reason.
//
// The filter entries are the contract internal/verify has to reproduce, and it
// is a different stage package that may not import this one, so it is stated
// once, here and in internal/transform/CLAUDE.md, and every Add for a leaf goes
// through addLeaf so there is one place to read it:
//
//	string leaf   mask.Canonical(free_text, the string), keyed by its path,
//	              whichever category's masker replaced it
//	number leaf   mask.Canonical(free_text, the JSON spelling), keyed by its path
//	boolean leaf  not recorded — a two-valued domain carries no residual signal
//	null leaf     not recorded — it is not masked
//	copied leaf   not recorded — the target holds the same value (leafRule)
//	document      mask.Canonical(semi_structured, the source text) under the
//	              empty path, and only when a collapse changed it
//
// A replaced string leaf is recorded under free_text's canonical form whatever
// masker replaced it, so the entry never depends on which category leafRule
// chose: internal/verify tests every string leaf of the target under free_text
// at its path, as it did before per-leaf categories existed, and a copied leaf
// is simply not in the filter to be found.
//
// Key names survive masking, with one exception: a key that itself parses as
// an email, a phone number or a credit-card number under a strong validator
// (docs/reviews/2026-09-09/REVIEW.md finding 8, evidence/json_keys.log,
// T-0137) is masked through that category's own masker, so
// {"canary.person@example.org":"ok"} no longer keeps the address as a map
// key. An arbitrary identifier used as a key — a UUID, a slug, a national ID
// a strong validator cannot name — still survives; SECURITY.md states that as
// what remains. §6 item 6's stated false negative is narrowed by exactly this
// much and no further.

// emptyDocument is what a log-shaped table's document becomes (§4), and what a
// document this package cannot parse becomes. It is never the source document.
const emptyDocument = "{}"

// leafCategory is the category every masked JSON leaf is *canonicalised*
// under for the residual filter (addLeaf), and the masker a leaf gets when
// nothing names a better one. ARCHITECTURE.md §4 sends a string leaf's key name
// "through the name rules", which are the embedded rule pack in
// internal/classify — another stage package, which internal/CLAUDE.md forbids
// reaching into — so the name rules reach this package as data, the
// decision's LeafKeys map, which internal/classify fills from the sampled
// documents and internal/verify reads back (T-0272). A hand-written table here
// would be a second rule pack outside Classification.Fingerprint, which is
// what an earlier version of this file had and removed.
const leafCategory = pipeline.CatFreeText

// leafVerdict is what leafRule decides for one leaf: copy it, or mask it under
// cat's masker.
type leafVerdict struct {
	copy bool
	cat  pipeline.Category
}

// leafPolicy is what one document column's leaves are decided under: the
// decision's per-leaf map, read through pipeline.Decision.LeafMap so that the
// column's own category and source come first (nil for any column that is not
// the classifier's plain semi_structured verdict), the category the column's
// own name gave it when that name did not decide the column
// (pipeline.Decision.LeafNameCategory, T-0393; empty otherwise), the phone
// region the run classified under (pipeline.Classification.PhoneRegion),
// which the value half reads, and whether the column's table matched the rule
// pack's log_shaped rule (pipeline.Decision.LogShaped, T-0398): maskDocument
// reads that last field ahead of any of this, because a log-shaped table's
// document is replaced whole and never reaches leafRule at all.
type leafPolicy struct {
	keys      map[string]pipeline.Category
	name      pipeline.Category
	region    string
	logShaped bool
}

// policyOf is the leaf policy a masked document column's decision gives, the
// one place transform reads the decision's leaf half.
func policyOf(d pipeline.Decision, region string) leafPolicy {
	return leafPolicy{
		keys: d.LeafMap(), name: d.LeafNameCategory(), region: region,
		logShaped: d.LogShaped,
	}
}

// leafRule is T-0272's per-leaf decision, in this order:
//
//  1. p.keys is nil — the decision carries no map, because nothing was
//     sampled or nothing set one, or the column's own decision is a personal
//     category, an operator's raise, or a name hit its type did not accept
//     (pipeline.Decision.LeafMap) — and every leaf is masked: under the
//     column's own name's category through leafMasker when p.name holds one
//     (T-0393: every leaf of a jsonb `emails` gets the email masker, every
//     leaf of a jsonb `full_name` free_text), and as free_text otherwise,
//     exactly as before per-leaf categories existed. The value is not read:
//     the name has already said what every leaf holds.
//  2. The nearest enclosing key the map names with a category, walking out
//     from the leaf's own key to the document's root, masks the leaf under
//     that category (an "address" object's "line1" is an address).
//  3. A value a validator recognises (leafValueCategory) masks the leaf under
//     the validator's category, whatever its keys say. text is empty for a
//     boolean, which no validator reads.
//  4. A leaf with no enclosing key at all (a document that is a bare array or
//     scalar) is masked as free_text: there is no name to vouch for it.
//  5. A leaf whose every enclosing key is in the map, as CatNone, is copied.
//  6. Anything else — a key the samples never showed — is masked as free_text.
//
// chain is the leaf's enclosing object keys, root first, in the source's
// spelling. internal/verify restates this function over the target's spelling
// (internal/verify/jsonleaf.go); the two spellings differ only for a key
// keyCategory masked, and internal/classify never enters such a key in the
// map, so the two sides read the same verdict for every leaf the target could
// hold unchanged. Changing an arm here changes a contract verify is written
// against: change both in one commit.
func leafRule(p leafPolicy, chain []string, text string, valued bool) leafVerdict {
	keys := p.keys
	if keys == nil {
		if p.name != "" {
			return leafVerdict{cat: leafMasker(p.name)}
		}
		return leafVerdict{cat: leafCategory}
	}
	for i := len(chain) - 1; i >= 0; i-- {
		if cat, ok := keys[chain[i]]; ok && cat != pipeline.CatNone {
			return leafVerdict{cat: leafMasker(cat)}
		}
	}
	if valued {
		if cat, ok := leafValueCategory(text, p.region); ok {
			return leafVerdict{cat: leafMasker(cat)}
		}
	}
	if len(chain) == 0 {
		return leafVerdict{cat: leafCategory}
	}
	for _, k := range chain {
		if _, ok := keys[k]; !ok {
			return leafVerdict{cat: leafCategory}
		}
	}
	return leafVerdict{copy: true}
}

// leafValueCategory is the value half of a leaf's personal signal: the
// category of the first validator that recognises the value, in this order.
//
// It is every validator internal/verify's second net runs over a document's
// leaves (internal/verify/validators.go, every entry `applies` admits for a
// leaf), plus the classifier's own leaf questions (internal/classify's
// jsonLeafIsPersonal: email, phone, IP, IBAN and Luhn). The first half is what
// keeps a copied leaf from ever being a second-net refusal on a run that
// masked correctly — the net reads a masked document's copied leaves and
// refuses a strong hit at exit 9 — and the second is what keeps a leaf the
// classifier counted as personal from being copied. Each validator is taken
// at a single value, with no ratio and no corroboration: at a leaf, one hit is
// the signal. internal/verify/jsonleaf.go restates it; the two lists have to
// be the same list (TestLeafValueCategoryIsPinned on both sides).
//
// region is the run's configured phone region. The net reads a leaf's phone
// number under it as well as in international form (internal/verify's count,
// T-0221), so this does too: without it a national-format number under a key
// no rule names was copied here and refused by the net at exit 9 on a run
// that did nothing wrong (T-0272 review round, finding 2). An empty region is
// textsig.ValidPhone's international-only reading.
func leafValueCategory(s, region string) (pipeline.Category, bool) {
	switch {
	case s == "":
		return "", false
	case textsig.ValidEmail(s):
		return pipeline.CatEmail, true
	case textsig.ValidPhoneRegion(s, region):
		return pipeline.CatPhone, true
	case textsig.ValidCard(s) || textsig.CardShape(s) || textsig.ValidLuhn(s) || textsig.ValidIBAN(s):
		return pipeline.CatFinancial, true
	case textsig.ValidIP(s) || textsig.ValidMAC(s):
		return pipeline.CatNetworkID, true
	case textsig.ValidNationalIDStructured(s) || textsig.ValidNationalIDChecksumOnly(s) ||
		textsig.ValidNationalIDDigits(s):
		return pipeline.CatNationalID, true
	case textsig.ValidURL(s):
		return pipeline.CatOnlineID, true
	case textsig.LooksSecret(s):
		return pipeline.CatCredential, true
	case textsig.AddressShape(s):
		return pipeline.CatAddress, true
	case textsig.SpecialCategoryVocabulary(s):
		return pipeline.CatSpecial, true
	}
	return "", false
}

// leafMasker is the category whose masker replaces a string leaf leafRule
// masked under cat. A category keeps its own masker when that masker's output
// is a value of the category drawn from a domain large enough that it will not
// coincide with another row's real value at the same path: every leaf hit is
// untestable and exit 9 (internal/verify's confirm), and a leaf is never
// explained the way ADR-015 explains a masked name. So person_name (a
// vocabulary of real names), person_date (about 25,000 dates) and
// special_category (a label set) are masked as free_text, as they were
// before, and so is every category with no text masker a leaf could take.
func leafMasker(cat pipeline.Category) pipeline.Category {
	switch cat {
	case pipeline.CatEmail, pipeline.CatPhone, pipeline.CatAddress, pipeline.CatGeo,
		pipeline.CatNationalID, pipeline.CatFinancial, pipeline.CatNetworkID,
		pipeline.CatOnlineID, pipeline.CatCredential:
		return cat
	default:
		// Every other category — a vocabulary, a small domain, or no text
		// masker a leaf could take — is masked as free_text.
		return leafCategory
	}
}

// leafConstraints is what a JSON leaf's masker is given. A leaf has no column
// type of its own: it is text inside a document.
func leafConstraints() mask.Constraints { return mask.Constraints{TypeTag: famText} }

// keyCategory reports the category a JSON object key masks under, and whether
// the key needs masking at all (finding 8, T-0137). Only the three validators
// internal/textsig calls strong — email, phone, and the Luhn half of a
// financial account — parse a string precisely enough to say the key itself
// IS the thing it parses as; anything else is a shape guess this package must
// not act on, for the same reason internal/verify's second net does not
// (internal/verify/validators.go, "strong"). An arbitrary identifier used as a
// key — a UUID, a slug, a customer number — is not named by any of the three
// and stays; that is what SECURITY.md's limitation is narrowed to.
//
// region is the run's configured phone region (leafPolicy.region,
// pipeline.Classification.PhoneRegion), read as leafValueCategory reads it
// (T-0394, the 2026-09-25 JSON red team's A11). internal/verify's second net
// reads a masked document's keys under the same region, so a national-format
// number used as a key ("07911 123456" under GB) is masked here rather than
// copied and refused there at exit 9 with --skip-table as the only way past.
// The masker's output is the international form (mask/gen_phone.go), which
// the net's own international-only key skip recognises. An empty region is
// textsig.ValidPhone's international-only reading, as before.
// internal/classify's strongKeyShape asks the same question with the same
// region, so a key masked here never enters the leaf map.
func keyCategory(name, region string) (pipeline.Category, bool) {
	switch {
	case textsig.ValidEmail(name):
		return pipeline.CatEmail, true
	case textsig.ValidPhoneRegion(name, region):
		return pipeline.CatPhone, true
	case textsig.ValidLuhn(name):
		return pipeline.CatFinancial, true
	default:
		return "", false
	}
}

// maskKey masks one JSON object key when keyCategory names it, and returns
// every other key unchanged. It also reports the matched category and the
// masked key's own canonical bytes (when there is a residual entry to make),
// so that walk — the only caller — can record that entry at the key's real
// path in the *target* document, which is spelled with the masked key and not
// the source one; maskKey itself is handed no path to record it at (T-0137
// review, docs/reviews finding — see CLAUDE.md, "A masked object key is keyed
// at its own path").
//
// It goes through mask.Apply directly rather than through addLeaf: a key has
// no "value" of a leaf's kind, and mask.Apply is a pure function of the run
// key, the category and the key's own canonical text — no path, no column —
// so two equal source keys mask to equal fakes wherever they appear, which is
// what lets an identity-keyed map still be joined on after masking. (That is
// also why two *distinct* source keys can mask alike; walk refuses that case
// rather than silently dropping one — see its own comment.)
func (t transformer) maskKey(
	col ref.ColumnRef,
	name, region string,
	k mask.Key,
) (masked string, canonical []byte, record bool, err error) {
	cat, ok := keyCategory(name, region)
	if !ok {
		return name, nil, false, nil
	}
	c := leafConstraints()
	id, err := mask.Pick(mask.Category(cat), c)
	if err != nil {
		return "", nil, false, &Refusal{Code: CodeMasker, Exit: exitTransform, Col: col, Masker: string(id), Reason: err}
	}
	r, err := mask.Apply(k, mask.Category(cat), id, mask.Value{Text: name}, c)
	if err != nil {
		return "", nil, false, &Refusal{Code: CodeMasker, Exit: exitTransform, Col: col, Masker: string(id), Reason: err}
	}
	if r.Masked && len(r.Canonical) > 0 {
		return r.Out.Text, r.Canonical, true, nil
	}
	return r.Out.Text, nil, false, nil
}

// addLeaf records one leaf's source value in the residual filter under its own
// path. Every per-leaf Add goes through here, so the convention internal/verify
// has to reproduce has one definition and not five call sites (see the package
// comment above).
func addLeaf(res pipeline.Residual, col ref.ColumnRef, path, text string) error {
	if text == "" {
		// mask.Apply records nothing for a NULL or an empty value, and a filter
		// entry for a value the target also holds is a residual hit on a run
		// that masked correctly (§6 item 3 exits 9 on one).
		return nil
	}
	canon, _, err := mask.Canonical(mask.Category(leafCategory), mask.Value{Text: text}, leafConstraints())
	if err != nil {
		return err
	}
	if canon.Text == "" {
		return nil
	}
	res.Add(col, path, []byte(canon.Text))
	return nil
}

// decodeDocument parses a document into the shape the walker takes. A value
// this package was handed directly rather than read from the source (a test
// fixture, chiefly) is walked as it stands; a string — which is what a real
// read of a plain json or jsonb column now arrives as too (internal/pg's
// jsonTextRows, T-0402), the same text a domain over jsonb, an hstore or an
// unregistered type has always arrived as, since pgx has no codec for any of
// those either — is parsed here, with json.Number so that an integer leaf is
// not silently a float64.
func decodeDocument(v any) (doc any, fromText bool, ok bool) {
	s, isText := v.(string)
	if !isText {
		return v, false, true
	}
	dec := json.NewDecoder(strings.NewReader(s))
	dec.UseNumber()
	var out any
	if err := dec.Decode(&out); err != nil {
		return nil, true, false
	}
	return out, true, true
}

// encodeDocument renders a walked document back into the kind it arrived as.
func encodeDocument(doc any, fromText bool) (any, bool) {
	if !fromText {
		return doc, true
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	if err := enc.Encode(doc); err != nil {
		return nil, false
	}
	return strings.TrimRight(buf.String(), "\n"), true
}

// maskDocument is §4's document rule for one cell.
//
// A log-shaped table's document is replaced whole with {} — no per-leaf masker
// runs and no per-leaf filter entry exists for it — and the document itself
// enters the filter, so a document that survived is still found. Every other
// document is walked, leaf by leaf, under lp (leafRule).
func (t transformer) maskDocument(
	col ref.ColumnRef,
	shape columnShape,
	lp leafPolicy,
	v any,
	key mask.Key,
	res pipeline.Residual,
) (any, error) {
	// An hstore is replaced whole rather than walked: nothing here parses one,
	// and a document nothing can parse must not be copied through. A
	// log-shaped table's document is the other unconditional collapse
	// (§4); lp.logShaped is pipeline.Decision.LogShaped, the rule pack's own
	// answer (T-0398) and not a second copy of it.
	if shape.family == famHstore || lp.logShaped {
		return t.collapseDocument(col, shape, v, key, res)
	}

	doc, fromText, ok := decodeDocument(v)
	if !ok {
		// A document this package cannot parse is replaced whole, never copied
		// through (THREAT_MODEL.md T12).
		return t.collapseDocument(col, shape, v, key, res)
	}
	walked, err := t.walk(col, lp, "$", nil, doc, key, res)
	if err != nil {
		return nil, err
	}
	out, ok := encodeDocument(walked, fromText)
	if !ok {
		return t.collapseDocument(col, shape, v, key, res)
	}
	return out, nil
}

// collapseDocument replaces the whole document with {} and records the source
// document in the filter under the empty path.
//
// It records it only when the collapse changed something. mask.Apply records
// nothing for a value it passed through under the NULL or the empty rule
// (mask/mask.go), and a document that already *is* the collapse output — an
// empty payload `{}` in an audit table, an hstore already `”` — is the same
// case: seeding the filter with the value the target will hold makes §6 item
// 3's residual scan read `{}` back off the target, find it in the filter,
// confirm it in the source and exit 9 on a run that masked correctly. Empty
// payloads in log tables are common, so this is the likely case and not the
// exotic one.
func (t transformer) collapseDocument(
	col ref.ColumnRef,
	shape columnShape,
	v any,
	key mask.Key,
	res pipeline.Residual,
) (any, error) {
	canon, _, err := mask.Canonical(mask.Category(pipeline.CatSemiStruct), valueOf(v), shape.constraints)
	if err != nil {
		return nil, &Refusal{Code: CodeMasker, Exit: exitTransform, Col: col, Masker: string(mask.MaskerSemiStruct), Reason: err}
	}

	// out is what the target will hold, and outText is that value as the text
	// canonicalisation sees it.
	var out any = map[string]any{}
	outText := emptyDocument
	if shape.family == famHstore {
		// An hstore is not JSON and "{}" is not an empty hstore. This module
		// has no hstore parser, so the whole map is replaced by the empty one,
		// which is the same answer mask's semi_structured generator gives and
		// the one answer that cannot leak.
		out, outText = "", ""
	} else if _, isText := v.(string); isText {
		out = emptyDocument
	}

	outCanon, _, err := mask.Canonical(mask.Category(pipeline.CatSemiStruct), mask.Value{Text: outText}, shape.constraints)
	if err != nil {
		return nil, &Refusal{Code: CodeMasker, Exit: exitTransform, Col: col, Masker: string(mask.MaskerSemiStruct), Reason: err}
	}
	if canon.Text != "" && canon.Text != outCanon.Text {
		res.Add(col, "", []byte(canon.Text))
	}
	return out, nil
}

// walk replaces every scalar leaf of a decoded document leafRule masks, in
// sorted key order so that the same document always draws the same values,
// copies every leaf it does not, and adds each masked leaf to the residual
// filter under its own path. chain is the enclosing object keys, root first,
// in the source's spelling (leafRule reads them against lp).
func (t transformer) walk(
	col ref.ColumnRef,
	lp leafPolicy,
	path string,
	chain []string,
	node any,
	k mask.Key,
	res pipeline.Residual,
) (any, error) {
	switch n := node.(type) {
	case map[string]any:
		names := make([]string, 0, len(n))
		for name := range n {
			names = append(names, name)
		}
		sort.Strings(names)
		out := make(map[string]any, len(n))
		for _, name := range names {
			maskedName, canon, record, err := t.maskKey(col, name, lp.region, k)
			if err != nil {
				return nil, err
			}
			// The child's path is built from the *masked* key, not the
			// source one: internal/verify reproduces every path in this
			// table from the target alone (it cannot import this package),
			// and the target spells this position with the masked key. A
			// path built from the source key can never be found again by a
			// scan that only ever sees the target — that was the bug this
			// review round found (finding 1) and this is the fix.
			childPath := path + "." + maskedName
			// mask.Apply is a pure function of the category and the key's
			// own canonical text, so two distinct source keys that
			// canonicalise alike — "+1 415 555 2671" and "+14155552671",
			// two spellings of one email address, two spellings of one card
			// number — mask to the same fake key. Silently overwriting
			// out[maskedName] would drop one whole subtree from the target
			// with no error, no event and no counter (finding 2); refuse
			// the cell instead, the same way any other masker refusal does.
			if _, collide := out[maskedName]; collide {
				return nil, &Refusal{
					Code: CodeMasker, Exit: exitTransform, Col: col, Path: path, Masker: "json_key",
					Reason: errors.New("two source keys canonicalise alike and mask to the same object key"),
				}
			}
			if record {
				res.Add(col, childPath, canon)
			}
			// The full slice expression makes append copy, so no two
			// siblings' chains share a backing array.
			child, err := t.walk(col, lp, childPath, append(chain[:len(chain):len(chain)], name), n[name], k, res)
			if err != nil {
				return nil, err
			}
			out[maskedName] = child
		}
		return out, nil
	case []any:
		out := make([]any, len(n))
		for i, item := range n {
			child, err := t.walk(col, lp, path+"["+strconv.Itoa(i)+"]", chain, item, k, res)
			if err != nil {
				return nil, err
			}
			out[i] = child
		}
		return out, nil
	case nil:
		// null stays null (§4). It reveals that the leaf was null, which §6
		// item 6 already lists for a NULL column.
		return nil, nil
	case string:
		v := leafRule(lp, chain, n, true)
		if v.copy {
			// No signal on any enclosing key or on the value, under keys the
			// samples showed: copied, and not recorded, because the target
			// holds the same value (T-0272, the T-0143 decision).
			return n, nil
		}
		return t.maskLeafString(col, path, v.cat, n, k, res)
	case bool:
		if leafRule(lp, chain, "", false).copy {
			return n, nil
		}
		// A boolean is redrawn over its own two-valued domain, so half the run
		// keys leave it as it was. That is §6 item 6's small-domain caveat and
		// not a leak — but it means no filter entry may be made for it: the
		// entry would be the value the target holds half the time, which is a
		// guaranteed residual hit and an exit 9 on a run that masked correctly
		// (§6 item 3). A two-valued domain carries no residual signal anyway.
		h, err := leafDigest(k, "bool", path, strconv.FormatBool(n))
		if err != nil {
			return nil, &Refusal{Code: CodeMasker, Exit: exitTransform, Col: col, Path: path, Masker: "json_bool", Reason: err}
		}
		return h[0]&1 == 1, nil
	case json.Number:
		if leafRule(lp, chain, n.String(), true).copy {
			return n, nil
		}
		return t.maskLeafNumber(col, path, n.String(), func(s string) any { return json.Number(s) }, k, res)
	case float64:
		// A value this package was handed directly rather than read from the
		// source arrives with float64 leaves this way (T-0402: a real read's
		// json or jsonb cell is the source's own text now, decoded above with
		// UseNumber, so this arm is a value-arrived-by-hand fallback and not
		// the common case any more), so an integral leaf has to be spotted by
		// its value and not by its Go kind.
		in := strconv.FormatFloat(n, 'g', -1, 64)
		// The rule reads the plain decimal spelling: 'g' writes a ten-digit
		// phone number stored as a number as 4.155552671e+09, which no
		// validator recognises. The filter entry keeps 'g', which is what
		// internal/verify reproduces for a number leaf.
		if leafRule(lp, chain, strconv.FormatFloat(n, 'f', -1, 64), true).copy {
			return n, nil
		}
		return t.maskLeafNumber(col, path, in, func(s string) any {
			f, err := strconv.ParseFloat(s, 64)
			if err != nil {
				// The generator writes decimal digits; a value that will not
				// parse back is a bug here, and 0 is the one answer that
				// carries nothing of the source.
				return float64(0)
			}
			return f
		}, k, res)
	default:
		// Nothing else comes out of a JSON decoder. A future kind becomes null
		// rather than passing through (THREAT_MODEL.md T12).
		return nil, nil
	}
}

// maskLeafString masks one string leaf under cat's masker (leafMasker chose
// it), and under leafCategory's when cat's masker has no answer for this value:
// a key named "phone" can hold "ask reception", and a leaf is masked either
// way — only the shape of the fake depends on the category.
func (t transformer) maskLeafString(
	col ref.ColumnRef,
	path string,
	cat pipeline.Category,
	in string,
	k mask.Key,
	res pipeline.Residual,
) (any, error) {
	c := leafConstraints()
	if cat != leafCategory {
		if id, err := mask.Pick(mask.Category(cat), c); err == nil {
			r, err := mask.Apply(k, mask.Category(cat), id, mask.Value{Text: in}, c)
			if err == nil && !r.Out.Null {
				return t.recordLeaf(col, path, id, r, in, res)
			}
		}
	}
	id, err := mask.Pick(mask.Category(leafCategory), c)
	if err != nil {
		return nil, &Refusal{Code: CodeMasker, Exit: exitTransform, Col: col, Path: path, Masker: string(id), Reason: err}
	}
	r, err := mask.Apply(k, mask.Category(leafCategory), id, mask.Value{Text: in}, c)
	if err != nil {
		return nil, &Refusal{Code: CodeMasker, Exit: exitTransform, Col: col, Path: path, Masker: string(id), Reason: err}
	}
	return t.recordLeaf(col, path, id, r, in, res)
}

// recordLeaf adds a masked string leaf's source to the filter and returns the
// masker's output.
func (t transformer) recordLeaf(
	col ref.ColumnRef,
	path string,
	id mask.ID,
	r mask.Result,
	in string,
	res pipeline.Residual,
) (any, error) {
	if r.Masked {
		// Through addLeaf and not res.Add(…, r.Canonical) directly: r.Canonical
		// is the source under the masker's own category, and the filter entry
		// for a leaf is always free_text's, so that it does not depend on which
		// category leafRule chose; one call site is what lets internal/verify
		// be written against a stated convention rather than against this file.
		if err := addLeaf(res, col, path, in); err != nil {
			return nil, &Refusal{Code: CodeMasker, Exit: exitTransform, Col: col, Path: path, Masker: string(id), Reason: err}
		}
	}
	return r.Out.Text, nil
}

// maskLeafNumber replaces a number leaf with a number of the same kind derived
// from h: integral stays integral, fractional stays fractional (§4). Neither
// the magnitude nor the digit count of the source survives.
func (t transformer) maskLeafNumber(
	col ref.ColumnRef,
	path, in string,
	rebuild func(string) any,
	k mask.Key,
	res pipeline.Residual,
) (any, error) {
	h, err := leafDigest(k, "number", path, in)
	if err != nil {
		return nil, &Refusal{Code: CodeMasker, Exit: exitTransform, Col: col, Path: path, Masker: "json_number", Reason: err}
	}
	// The source spelling through addLeaf, not the raw bytes: a number leaf's
	// filter entry has to be computable by internal/verify, which has this
	// package's conventions and not its code.
	if err := addLeaf(res, col, path, in); err != nil {
		return nil, &Refusal{Code: CodeMasker, Exit: exitTransform, Col: col, Path: path, Masker: "json_number", Reason: err}
	}
	u := binary.BigEndian.Uint64(h[:8])
	if strings.ContainsAny(in, ".eE") {
		return rebuild(strconv.FormatUint(u%1000, 10) + "." + pad2(uint((u>>20)%100))), nil
	}
	return rebuild(strconv.FormatUint(u%1_000_000, 10)), nil
}

func pad2(n uint) string {
	if n < 10 {
		return "0" + strconv.FormatUint(uint64(n), 10)
	}
	return strconv.FormatUint(uint64(n), 10)
}

// leafDigest is h for a leaf kind the mask module has no generator for: a
// number or a boolean. The path is part of the digest so that the same number
// under two keys of one document does not have to become the same fake, and the
// category is semi_structured, so a leaf's key schedule is the document's and
// not a scalar column's.
func leafDigest(k mask.Key, tag, path, text string) ([32]byte, error) {
	return mask.Digest(k, mask.Category(pipeline.CatSemiStruct), tag,
		mask.Encode([]byte(path), []byte(text)))
}
