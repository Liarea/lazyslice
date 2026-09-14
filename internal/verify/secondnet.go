// SPDX-License-Identifier: Apache-2.0

package verify

import (
	"context"
	"crypto/sha256"

	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
)

// The second net (ARCHITECTURE.md section 6 item 4).
//
// All eleven of the classifier's value validators, folded into the nine entries
// in validators.go (its two financial validators share one entry here, and so
// do its two network ones), run over the full contents of
// every column of the loaded target that is not fully masked by a category
// masker: every unmasked, non-opted-out column of a family this package can
// name and render, and the string leaves of every JSON column, masked or not. A
// column reaching the strong ratio is exit 9, and so is any hit at all on a
// column holding fewer than minValues non-NULL values, which has no ratio to
// reach it with (netColumn) — except under the dictionary rule below. The
// target is small, so this is a scan and not a sample, which is what makes it
// catch a column the 200-row sample under-represented.
//
// Two things it does not catch, stated here and in internal/verify/CLAUDE.md
// rather than implied:
//   - a category outside the rule pack (THREAT_MODEL.md T1 says so);
//   - a column of a family this package cannot name — famOther, and so a
//     tsvector or an enum (columns.go, netText).
//
// It also implements only the strong branch of ARCHITECTURE.md section 4's
// scoring: a weak ratio raised to `possible` by the neighbouring-column rule is
// not reached here (see CLAUDE.md).
//
// **The dictionary rule.** person_name and free_text arrived with the shared
// leaf package that holds the name dictionary (internal/textsig, tracker
// T-0055), and they are scored under a threshold of their own, because the
// classifier's would fail a target on an ordinary English word. Black, Brown,
// Hill, Green and Wood are all surnames, so a `product.colour` column is 100%
// "person_name" to the validator the classifier uses — and the two packages do
// different things with that answer. The classifier masks the column, which
// costs a lookup table; this net refuses a target that is already loaded, at
// exit 9, with no green path short of --unmask on a column that holds no
// personal data. So on this side a dictionary hit has to be a *shape* and not a
// word, and the shape is the same one for both validators — a given name
// immediately followed by a surname:
//
//	person_name counts textsig.Dict.NameShape — a whole value of two or three
//	dictionary words carrying that pair — never LooksLikeName, and never the
//	plain multi-token shape either, because green lane, west hill and hunter
//	green are all pairs of dictionary surnames (tracker T-0055 review);
//
//	free_text counts textsig.Dict.ProseName — six words or more carrying that
//	same pair — never Prose, which fires on one dictionary word and therefore
//	on "The supplier may terminate this agreement on thirty days notice.",
//	may being a surname like about two hundred other ordinary English words
//	in names.txt;
//
//	neither runs over a document's leaves at all (applies, below), because
//	internal/classify's leaf signal never consults the dictionary;
//
//	and either way the column fails only on the strong ratio across at least
//	minValues *distinct* hitting values, so neither the T-0058 "any hit below
//	minValues" branch nor one literal repeated down a column can carry it.
//
// What that costs is a real name column of one word per row — a `forename`
// column the classifier did not mask — which this net now passes, and a name
// column written surname-first, and a note naming a person the dictionary does
// not carry. That is the direction the asymmetry has to fail in: the
// alternative is exit 9 on a colour column, a street-name column or a column
// of contract clauses, which is a refusal an operator cannot act on and would
// learn to route around with --unmask.
//
// A column carrying an --unmask opt-out is deliberately outside this net: that
// is why the opt-out requires a reason and expires when the column's type
// changes (ARCHITECTURE.md section 8), rather than being a scan it would fail
// on every run.

// secondNet is item 4.
func (s *state) secondNet(ctx context.Context) error {
	scanned := int64(0)
	for _, step := range s.steps {
		table := s.tables[step.Table]
		for _, c := range table.Columns {
			col := ref.ColumnRef{Table: step.Table, Column: c.Name}
			mode, ok := s.netMode(col, c)
			if !ok {
				continue
			}
			scanned++
			if err := s.netColumn(ctx, col, mode); err != nil {
				return err
			}
		}
	}
	// Never beside a failure of the same name: Report.Checks is what the run
	// prints, and a passing "second_net" line under a failing one is a green
	// tick on the very check that produced exit 9. fk.go, counts.go,
	// residual.go and sample.go all guard the same way.
	if !s.failed(checkSecondNet) {
		s.pass(checkSecondNet, CodeSecondNetPassed, scanned)
	}
	return nil
}

// netMode is how one column is read by the net, and whether it is read at all.
type netMode struct {
	// leaves is true for a masked JSON column, whose string leaves are the net's
	// subject rather than the column's own value.
	leaves bool
	// array is true for an array column, whose elements are what the validators
	// run over: ARCHITECTURE.md section 4 classifies an array on its element
	// type, so the net has to ask the same question of the same values
	// (netValues).
	array bool
	// text and digits say which validators apply, from the column's family.
	text   bool
	digits bool
}

func (s *state) netMode(col ref.ColumnRef, c pipeline.Column) (netMode, bool) {
	family, array := s.shapeOf(c)
	d, has := s.decision(col)
	switch {
	case has && optedOut(d):
		// The one column deliberately outside this net (ARCHITECTURE.md section
		// 8): the opt-out carries a reason and expires on a type change instead.
		return netMode{}, false
	case document(family):
		// A masked document's masker was chosen per key by name, so the net
		// checks the leaves; an unmasked one had no masker at all, which is a
		// stronger reason to read its leaves and not a reason to skip it.
		return netMode{leaves: true, text: true}, true
	case has && d.Masked:
		return netMode{}, false
	case netText(family):
		return netMode{array: array, text: true}, true
	case numeric(family):
		return netMode{array: array, digits: true}, true
	}
	return netMode{}, false
}

// netColumn runs every applicable validator over one column's whole contents.
func (s *state) netColumn(ctx context.Context, col ref.ColumnRef, mode netMode) error {
	nonNull := int64(0)
	hits := make([]int64, len(validators))
	// distinct holds, for the dictionary-backed validators only, the digests of
	// up to minValues distinct values that hit. A digest rather than the value
	// because a free_text value can be a whole document and this set outlives
	// the row: at most minValues×32 bytes per validator, and no production value
	// held any longer than the scan of the row it came from.
	distinct := make([]map[[sha256.Size]byte]struct{}, len(validators))
	for i, val := range validators {
		if val.dict && applies(val, mode) {
			distinct[i] = make(map[[sha256.Size]byte]struct{}, minValues)
		}
	}

	err := s.scanColumn(ctx, col.Table, col.Column, func(v any) error {
		for _, text := range s.netValues(v, mode) {
			nonNull++
			for i, val := range validators {
				if !applies(val, mode) {
					continue
				}
				if val.ok(text) {
					hits[i]++
					if d := distinct[i]; d != nil && len(d) < minValues {
						d[sha256.Sum256([]byte(text))] = struct{}{}
					}
				}
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	if nonNull == 0 {
		return nil
	}
	// A column with fewer than minValues non-NULL values is *unproven*, not
	// clean. Returning nil here was a fail-open with nothing above it: a
	// three-row table yields two values, the ratio over them means nothing, and
	// public.devices.owned_by in testdata/nasty.sql — two email addresses and a
	// NULL — reached the target in cleartext under exit 0, with the classifier
	// silent for the same reason and this net silent after it (THREAT_MODEL.md
	// T1, tracker T-0058). So the validators run over
	// whatever there is and *any* hit fails: the threshold is what a ratio
	// buys, and below minValues there is no ratio to buy it with. One value
	// that parses as an email is still a production email address in the
	// target, which is the thing this net exists to refuse.
	//
	// The two dictionary-backed validators are outside this branch, in the loop
	// below: a dictionary word is not a parse, so "any hit" there would be exit
	// 9 on a two-row lookup table of English nouns (the dictionary rule, in the
	// package comment above).
	//
	// nonNull is counted over the *target*, so this branch is slice-size
	// dependent: a table --take reduces to two rows is unproven here and the
	// same table at --take 500 is not, which makes the verdict non-monotone in
	// the slice size for the two validators that carry no parse
	// (textsig.AddressShape, textsig.LooksSecret). That cost is weighed against
	// the leak above in internal/verify/CLAUDE.md, which also records the
	// narrowing to revisit if the refusal proves noisy.
	proven := nonNull >= minValues
	for i, val := range validators {
		if !applies(val, mode) || hits[i] == 0 {
			continue
		}
		ratio := float64(hits[i]) / float64(nonNull)
		if val.dict {
			// The dictionary rule (see the package comment above and
			// internal/verify/CLAUDE.md). Two differences from the six
			// validators that carry a parse, both narrowing, and both because a
			// dictionary word is a word an ordinary English column may hold:
			// the strong ratio is required whatever the column's size, so the
			// branch above this one does not extend to these two; and the hits
			// must be at least minValues *distinct* values, so a single
			// dictionary literal repeated down a column cannot reach exit 9.
			// The validators themselves are already the narrow ones —
			// NameShape, not LooksLikeName; ProseName, not Prose.
			if ratio < validatorThreshold || len(distinct[i]) < minValues {
				continue
			}
		} else if proven && ratio < validatorThreshold {
			continue
		}
		s.fail(&Refusal{
			Code: CodeRefusedSecondNet, Exit: exitResidual, Check: checkSecondNet,
			Table: col.Table, Column: col.Column, Count: hits[i], Reason: val.name,
		})
		// One category per column: the column is already exit 9, and a second
		// line naming a second validator over the same values says nothing more
		// about what to do next.
		return nil
	}
	return nil
}

// applies reports whether one validator runs over the values this mode yields.
//
// The two dictionary-backed validators are excluded from a document's leaves,
// and that is a deliberate hole with a task against it. internal/classify
// short-circuits every json/jsonb/hstore column into its own leaf signal
// (jsonSignal), which asks the key patterns plus email, phone, IP, IBAN and
// Luhn about a leaf and never consults the name dictionary at all — so a
// person_name or free_text refusal over leaves would be this net refusing a
// loaded target on evidence the classifier is structurally unable to have seen,
// with no green path short of --unmask on a column the classifier had no reason
// to mask. The other seven entries stay — email, phone, network_id,
// financial_account, online_id (T-0122), credential and address — and five of
// the shapes they ask about are the classifier's own leaf questions exactly:
// email, phone, IP, IBAN and Luhn. Four are not — MAC, URL, LooksSecret and
// AddressShape — and they stay because the argument above does not reach them:
// every one is a *parse* rather than a dictionary word, so a leaf that hits one
// is a value of that shape and not an ordinary English sentence, and a document
// is not a safer place to keep an address than the scalar column of the same
// table the classifier would have masked. When classify's leaf signal reads the
// dictionary, the dict exclusion comes off with it (tracker T-0087).
func applies(v validator, mode netMode) bool {
	if v.dict && mode.leaves {
		return false
	}
	return (mode.text && v.text) || (mode.digits && v.digits)
}

// netValues reduces one scanned value to the strings the validators run over:
// an array yields its elements, because section 4 classifies an array on its
// element type; a masked document yields its string leaves; a NULL yields
// nothing, because the ratio is over the non-NULL values.
func (s *state) netValues(v any, mode netMode) []string {
	if v == nil {
		return nil
	}
	if mode.leaves {
		ls := leaves(v)
		out := make([]string, 0, len(ls))
		for _, l := range ls {
			if l.str && l.text != "" {
				out = append(out, l.text)
			}
		}
		return out
	}
	if elems, ok := v.([]any); ok {
		out := make([]string, 0, len(elems))
		for _, e := range elems {
			if e == nil {
				continue
			}
			out = append(out, textOf(e))
		}
		return out
	}
	text := textOf(v)
	if mode.array {
		// The other carrier an array arrives in: the server's own text output
		// form, which is what an array of an element type the pool's map does
		// not know comes back as — a citext[] is the single string
		// "{a@b.test,c@d.test}" (arrayliteral.go, tracker T-0118 and T-0129).
		// Unsplit, that value reaches the validators as one string holding two
		// addresses and no validator recognises it, so the net that
		// THREAT_MODEL.md T1 names as one of two controls on classifier recall
		// sees nothing in exactly the column class §4 now classifies
		// element-wise.
		//
		// A literal that will not parse falls through to the whole value
		// rather than failing the run. This net's subject is a column nothing
		// masked, so there is no masker to have failed open: reading the whole
		// value is a recall hole of the kind famOther already is (columns.go),
		// and exit 9 on a column the classifier had no reason to mask would be
		// a refusal with no action behind it. The masked column, where the
		// argument is the opposite one, is refused in residual.go.
		if elems, err := arrayLiteralElements(text); err == nil {
			return elems
		}
	}
	return []string{text}
}
