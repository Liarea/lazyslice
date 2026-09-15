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
// Structure and key names are kept and every scalar leaf is replaced. A string
// leaf goes through the free_text masker; a number leaf becomes a number of the
// same kind derived from h; a boolean leaf becomes a boolean derived from h;
// null stays null; arrays and objects keep their shape.
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
//	string leaf   mask.Canonical(free_text, the string), keyed by its path
//	number leaf   mask.Canonical(free_text, the JSON spelling), keyed by its path
//	boolean leaf  not recorded — a two-valued domain carries no residual signal
//	null leaf     not recorded — it is not masked
//	document      mask.Canonical(semi_structured, the source text) under the
//	              empty path, and only when a collapse changed it
//
// free_text is what §4's default gives an unrecognised key, and it is now what
// every string leaf gets: the alternative was a second name-rule pack inside
// this package, whose vocabulary no other package could see (see CLAUDE.md).
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

// logTableWords are the table-name words §4 names: "any jsonb in a table named
// like audit|log|history|event" is replaced with {} rather than walked leaf by
// leaf. Plurals are included because a table is more often called events than
// event (testdata/README.md trap 16b is public.events).
var logTableWords = map[string]bool{
	"audit": true, "audits": true,
	"log": true, "logs": true,
	"history": true, "histories": true,
	"event": true, "events": true,
}

// logShaped reports whether a table's name carries one of those words as a
// word. It is matched on underscore-separated segments rather than as a
// substring, so "catalogue" is not a log and "audit_log" is.
func logShaped(t ref.TableRef) bool {
	for _, part := range strings.Split(strings.ToLower(t.Name), "_") {
		if logTableWords[part] {
			return true
		}
	}
	return false
}

// leafCategory is the category every JSON leaf is masked and canonicalised
// under. ARCHITECTURE.md §4 sends a string leaf's key name "through the name
// rules", which are the embedded rule pack in internal/classify — another stage
// package, which internal/CLAUDE.md forbids reaching into. A hand-written table
// here was a second rule pack: a different vocabulary, a different match, and
// outside Classification.Fingerprint, so editing it would silently change
// masked output; and because it was unexported, internal/verify could not
// reproduce the canonical bytes of a leaf it classified, which is the residual
// control failing open (THREAT_MODEL.md T12).
//
// So this is §4's own default and nothing more, until §14's one-level JSON key
// collection lands in internal/classify and can set the category per leaf.
// Every string leaf is still masked; what is lost is the shape of the fake — an
// email leaf becomes free text rather than an address. CLAUDE.md records it.
const leafCategory = pipeline.CatFreeText

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
func keyCategory(name string) (pipeline.Category, bool) {
	switch {
	case textsig.ValidEmail(name):
		return pipeline.CatEmail, true
	case textsig.ValidPhone(name):
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
	name string,
	k mask.Key,
) (masked string, canonical []byte, record bool, err error) {
	cat, ok := keyCategory(name)
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

// decodeDocument parses a document into the shape the walker takes. A value pgx
// already decoded (a json or jsonb column comes back as map[string]any,
// []any or a scalar) is walked as it stands; a string — which is what a domain
// over jsonb, an hstore or an unregistered type arrives as — is parsed here,
// with json.Number so that an integer leaf is not silently a float64.
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
// document is walked.
func (t transformer) maskDocument(
	col ref.ColumnRef,
	shape columnShape,
	v any,
	key mask.Key,
	res pipeline.Residual,
) (any, error) {
	// An hstore is replaced whole rather than walked: nothing here parses one,
	// and a document nothing can parse must not be copied through.
	if shape.family == famHstore || logShaped(col.Table) {
		return t.collapseDocument(col, shape, v, key, res)
	}

	doc, fromText, ok := decodeDocument(v)
	if !ok {
		// A document this package cannot parse is replaced whole, never copied
		// through (THREAT_MODEL.md T12).
		return t.collapseDocument(col, shape, v, key, res)
	}
	walked, err := t.walk(col, "$", doc, key, res)
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

// walk replaces every scalar leaf of a decoded document, in sorted key order so
// that the same document always draws the same values, and adds each masked
// leaf to the residual filter under its own path.
func (t transformer) walk(
	col ref.ColumnRef,
	path string,
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
			maskedName, canon, record, err := t.maskKey(col, name, k)
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
			child, err := t.walk(col, childPath, n[name], k, res)
			if err != nil {
				return nil, err
			}
			out[maskedName] = child
		}
		return out, nil
	case []any:
		out := make([]any, len(n))
		for i, item := range n {
			child, err := t.walk(col, path+"["+strconv.Itoa(i)+"]", item, k, res)
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
		return t.maskLeafString(col, path, n, k, res)
	case bool:
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
		return t.maskLeafNumber(col, path, n.String(), func(s string) any { return json.Number(s) }, k, res)
	case float64:
		// A jsonb column pgx already decoded arrives with float64 leaves, so an
		// integral leaf has to be spotted by its value and not by its Go kind.
		in := strconv.FormatFloat(n, 'g', -1, 64)
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

// maskLeafString masks one string leaf under leafCategory.
func (t transformer) maskLeafString(
	col ref.ColumnRef,
	path, in string,
	k mask.Key,
	res pipeline.Residual,
) (any, error) {
	c := leafConstraints()
	id, err := mask.Pick(mask.Category(leafCategory), c)
	if err != nil {
		return nil, &Refusal{Code: CodeMasker, Exit: exitTransform, Col: col, Path: path, Masker: string(id), Reason: err}
	}
	r, err := mask.Apply(k, mask.Category(leafCategory), id, mask.Value{Text: in}, c)
	if err != nil {
		return nil, &Refusal{Code: CodeMasker, Exit: exitTransform, Col: col, Path: path, Masker: string(id), Reason: err}
	}
	if r.Masked {
		// Through addLeaf and not res.Add(…, r.Canonical) directly: the two are
		// the same bytes, and one call site is what lets internal/verify be
		// written against a stated convention rather than against this file.
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
