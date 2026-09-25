// SPDX-License-Identifier: Apache-2.0

package classify

import (
	"strings"

	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
)

// A generated column's reads of a document's leaves (T-0397, the 2026-09-25
// JSON red team's round 1, entry 24).
//
// A generated column is never masked: the target recomputes it from the
// columns its expression names (markNeverMasked). When one of those is a
// masked json or jsonb document, what the target computes is whatever
// internal/transform left in the leaves the expression reads, and the
// per-leaf rule (T-0272) copies a leaf nothing marks personal. The red team
// wrote `joined GENERATED ALWAYS AS ((identity_data ->> 'u') || '@' ||
// (identity_data ->> 'h'))`, and the same with a country code and a national
// number, and with a card's BIN and tail: no rule names u, h, cc, nsn, bin or
// tail and no validator recognises any of them alone, so each leaf was copied
// and each generated column held a real email, phone number or card number
// in every row -- while this package's own samples of the generated column
// said "30/30 samples parse as addresses".
//
// So the samples of the generated column are evidence about the leaves it
// reads. When they validate -- a validator decided the column (bestSignal's
// strong branch), recorded in base as work.generatedValue -- every key the
// expression reads through -> or ->> from a json or jsonb column of the same
// table is given the generated column's own category in that column's
// LeafKeys, where the map calls it none. internal/transform then masks every
// leaf under the key through that category's leaf masker, and the value the
// target recomputes is built from masked leaves. internal/verify's second net
// is the other half: it refuses a generated value a validator recognises
// unless the value is one of its own row's category-masked leaves
// (internal/verify/jsonleaf.go), which is what catches the shape when these
// samples did not.
//
// It only masks more: a key the map calls none had every leaf beneath it
// copied unless the value half recognised it, and after this it has every
// leaf masked. A key the map already names keeps its category, and a key
// the map does not hold -- one the samples never showed, or one transform
// masks as a value (strongKeyShape) -- is left alone, because every leaf
// beneath it is masked already, and entering it would move its leaves from
// free_text filler, which the net reads, to a category's masker, which the
// net leaves to the residual scan on a spelling the target may not share.

// generatedLeafKeys is the pass. It runs after every pass that can change a
// document's LeafKeys or a generated column's category, and before
// applyPrior, which reads neither.
func (st *state) generatedLeafKeys() {
	for _, c := range st.order {
		w := st.dec[c]
		if w == nil || !w.generated || w.generatedValue == "" {
			continue
		}
		cat := w.d.Category
		if cat == pipeline.CatNone {
			cat = w.generatedValue
		}
		idents, keys := generatedReads(w.column.Generated)
		if len(keys) == 0 {
			continue
		}
		for _, id := range idents {
			doc := st.dec[ref.ColumnRef{Table: c.Table, Column: id}]
			if doc == nil || doc.generated || doc.array || !isJSONFamily(doc.family) || doc.d.LeafKeys == nil {
				continue
			}
			for _, k := range keys {
				if prev, ok := doc.d.LeafKeys[k]; ok && prev == pipeline.CatNone {
					doc.d.LeafKeys[k] = cat
				}
			}
		}
	}
}

// generatedReads lists what a deparsed generated-column expression
// (pg_get_expr's spelling) reads: every identifier -- double-quoted exactly as
// written, bare folded to lower case, the way internal/verify's
// exprIdentifiers reads the same expression -- and every string literal that
// is the right operand of -> or ->>, the object keys the expression takes
// out of a document ('email' in `(identity_data ->> 'email'::text)`). Any
// other string literal is skipped whole. A bare word that is no column of the
// table (a function or type name) is harmless: the caller keeps only
// identifiers that name a document column.
func generatedReads(expr string) (idents, keys []string) {
	arrow := false
	for i := 0; i < len(expr); {
		c := expr[i]
		switch {
		case c == '\'' || ((c == 'E' || c == 'e') && i+1 < len(expr) && expr[i+1] == '\''):
			if c != '\'' {
				i++
			}
			lit, next := quotedToken(expr, i, '\'')
			if arrow {
				keys = append(keys, lit)
			}
			arrow = false
			i = next
		case c == '"':
			id, next := quotedToken(expr, i, '"')
			idents = append(idents, id)
			arrow = false
			i = next
		case strings.HasPrefix(expr[i:], "->"):
			i += 2
			if i < len(expr) && expr[i] == '>' {
				i++
			}
			arrow = true
		case exprIdentStart(c):
			j := i
			for j < len(expr) && (exprIdentStart(expr[j]) || (expr[j] >= '0' && expr[j] <= '9') || expr[j] == '$') {
				j++
			}
			idents = append(idents, strings.ToLower(expr[i:j]))
			arrow = false
			i = j
		case c == ' ' || c == '\t' || c == '\n' || c == '(':
			// Whitespace and an opening bracket between an arrow and its
			// operand leave the arrow pending.
			i++
		default:
			arrow = false
			i++
		}
	}
	return idents, keys
}

// quotedToken reads the quoted token that opens at expr[i] (quote is ' or "),
// a doubled quote read as one, and returns it with the index just past it.
func quotedToken(expr string, i int, quote byte) (string, int) {
	var b strings.Builder
	i++
	for i < len(expr) {
		if expr[i] == quote {
			if i+1 < len(expr) && expr[i+1] == quote {
				b.WriteByte(quote)
				i += 2
				continue
			}
			return b.String(), i + 1
		}
		b.WriteByte(expr[i])
		i++
	}
	return b.String(), i
}

func exprIdentStart(c byte) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c >= 0x80
}
