// SPDX-License-Identifier: Apache-2.0

package verify

import (
	"strings"

	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
	"github.com/Liarea/lazyslice/internal/textsig"
)

// Per-leaf categories for a masked JSON document (T-0272, the maintainer's
// T-0143 decision of 2026-09-24): internal/transform masks a leaf with a
// personal signal on its key or its value and copies a leaf with none, under
// the decision's pipeline.Decision.LeafKeys map. This file is transform's
// leafRule, leafValueCategory and leafMasker (internal/transform/json.go)
// restated, because a stage package may not import another
// (internal/CLAUDE.md), the same "second copy" strongKeyCategory and
// catalog.go's strongCatalogHit already are.
//
// **What this package reads it for is narrow: the second net's skip, and
// nothing else.** The residual scan does not need it. Transform records every
// masked string leaf under free_text's canonical form whatever category's
// masker replaced it, and records nothing for a copied leaf, so documentHits
// tests every string leaf under free_text at its path exactly as it did
// before, and a copied leaf is simply not in the filter to be found. The
// second net does need it: a leaf transform replaced through the email masker
// is a fake address, which the net's own email validator recognises, and
// counting the masker's correct output as a hit would refuse every run with an
// email in a document at exit 9 — T-0172's bug, for a leaf rather than a key.
// So the net skips a leaf this rule says a *category* masker replaced, and the
// residual scan is what proves that leaf's source did not survive. A leaf the
// rule says was copied, or replaced with free_text filler, is still read.
//
// **Why that skip cannot hide a copied source value.** The net reads a string
// leaf with its own validators (validators.go, every entry `applies` admits for
// a leaf), and under --phone-region its phone entry also reads the
// configured region (count, T-0221). leafValueCategory below holds every one
// of them, the region included, because transform is handed the same region
// (pipeline.Classification.PhoneRegion, which internal/classify fills from the
// Config internal/core also builds Options.PhoneRegion from)
// (TestLeafValueCategoryCoversTheNetsLeafValidators, with and without a
// region). A leaf the rule says was copied therefore hits none of the net's
// validators; a leaf the rule says a category masker replaced was, on the
// transform side, replaced and recorded, because the rule is a function of
// the column's own decision, the key chain, the region and the value alone,
// and a copied value is the same value on both sides. The one way to break
// that is for the two restatements to drift apart, which is what
// TestLeafValueCategoryIsPinned on both sides is for.
//
// The region the rule reads here is the classification's, the one transform
// read, and not Options.PhoneRegion: if the two ever differed, a national
// number transform copied under no region would be read here as copied too,
// and the net's own region check would refuse it -- the failure closes, where
// reading Options' region would call the copy a masker's output and skip it.

// leafCategory is transform's: the canonical form of every masked leaf's
// filter entry, and the masker a leaf gets when nothing names a better one.
const leafCategory = pipeline.CatFreeText

// leafPolicy is internal/transform's: the decision's per-leaf map through
// pipeline.Decision.LeafMap, the category the column's own name gave every
// leaf when its type decided the column (pipeline.Decision.LeafNameCategory,
// T-0393), the classification's phone region, and whether the column's table
// matched the rule pack's log_shaped rule (pipeline.Decision.LogShaped,
// T-0398).
type leafPolicy struct {
	keys      map[string]pipeline.Category
	name      pipeline.Category
	region    string
	logShaped bool
}

// policyOf is internal/transform's: the leaf policy a masked document
// column's decision gives.
func policyOf(d pipeline.Decision, region string) leafPolicy {
	return leafPolicy{
		keys: d.LeafMap(), name: d.LeafNameCategory(), region: region,
		logShaped: d.LogShaped,
	}
}

// categorised reports whether any leaf under p can have been replaced through
// a category's own masker rather than free_text: a per-leaf map, or a column
// name whose category keeps its own masker for a leaf (T-0393's jsonb
// `emails`, `by_phone`, `home_address`, `passwords`, `national_id`). With
// neither, every masked leaf is free_text filler, which the net reads.
//
// A log-shaped column is never categorised, whatever LeafMap or
// LeafNameCategory answer (T-0398): internal/transform's maskDocument
// collapses such a column to {} before leafRule ever runs, so nothing under
// it was ever replaced through a category's own masker -- generatedFromMasked
// leaves below must not treat one of its keys as a generated column's own
// row value.
func (p leafPolicy) categorised() bool {
	return !p.logShaped && (p.keys != nil || leafMaskerEmits(p.name))
}

// leafVerdict is what leafRule decides for one leaf.
type leafVerdict struct {
	copy bool
	cat  pipeline.Category
}

// leafRule is internal/transform's leafRule, over the target's spelling of
// the key chain (root first). The spellings differ only for a key transform's
// keyCategory masked, and internal/classify never enters such a key in the
// map, under either spelling, so the verdict is the same for every leaf the
// target could hold unchanged. With no map, every leaf is masked under the
// column's own name's category through leafMasker when p.name holds one
// (T-0393), and as free_text otherwise; neither reads the key chain.
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

// leafValueCategory is internal/transform's, the same validators in the same
// order, the phone question under the run's region as the net's count asks it.
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

// leafMasker is internal/transform's: the categories whose own masker replaces
// a leaf, every other one masked as free_text.
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

// replacedByCategoryMasker reports whether the second net should leave one
// string leaf of a masked document to the residual scan: the decision carries
// a per-leaf map or a column-name category (T-0393), and the rule says a
// category's own masker, not free_text, replaced the leaf. With neither, every
// leaf was free_text filler, which the net has always read, and still does.
func replacedByCategoryMasker(p leafPolicy, l leaf) bool {
	if !p.categorised() || !l.str {
		return false
	}
	v := leafRule(p, l.keys, l.text, true)
	return !v.copy && v.cat != leafCategory
}

// leafMaskerEmits reports whether a leaf's own category masker emits values
// of cat: every category leafMasker keeps, and never free_text, whose filler
// no validator reads.
func leafMaskerEmits(cat pipeline.Category) bool {
	return cat != leafCategory && leafMasker(cat) == cat
}

// leafDocument is one masked document column a generated column's
// expression reads, with the leaf policy its decision gives (policyOf).
type leafDocument struct {
	column string
	policy leafPolicy
}

// generatedFromMaskedLeaves returns, for a generated column over masked
// columns of its own table only (generatedFromMasked, ADR-015), the masked
// document columns among them that carry per-leaf categories, or whose own
// name gave every leaf a category with its own masker (T-0393); nil for every
// other column. Such a column is computed by the target from leaves transform
// either replaced through a category masker, whose output is still a value of
// that category, or copied because no validator recognised them.
//
// **What the net may skip there is one value, not one category** (T-0397,
// the 2026-09-25 JSON red team's round 1, entry 24). The first version of
// this rail skipped every validator whose category a leaf masker emits for
// the whole column, whichever leaf the expression read, so a column that
// assembled an email from two copied leaves (`u || '@' || h`), a phone number
// from two more and a card number from two more crossed at exit 0 beside the
// Supabase column the rail was written for. netColumn now reads these
// documents in the same row as the generated value (sameRowMaskedLeaves) and
// skips a hit only when the value equals, after lower and btrim, a string
// leaf of that row the rule says a category masker replaced: `lower(leaf)`
// over the email masker's output is that output. Every other hit counts, so
// a value built out of copied leaves is refused like any unmasked column's.
func (s *state) generatedFromMaskedLeaves(t ref.TableRef, expr string) []leafDocument {
	if !s.generatedFromMasked(t, expr) {
		return nil
	}
	var out []leafDocument
	seen := map[string]bool{}
	for _, id := range exprIdentifiers(expr) {
		if seen[id] {
			continue
		}
		seen[id] = true
		d, ok := s.decision(ref.ColumnRef{Table: t, Column: id})
		if !ok || !d.Masked {
			continue
		}
		if p := policyOf(d, s.cls.PhoneRegion); p.categorised() {
			out = append(out, leafDocument{column: id, policy: p})
		}
	}
	return out
}

// sameRowMaskedLeaves is the set of one target row's string leaves, across
// the documents docs names (vals in the same order), that the rule says a
// category's own masker replaced, each folded by generatedFold. It is what
// netColumn compares a generated column's value with.
func sameRowMaskedLeaves(docs []leafDocument, vals []any) map[string]bool {
	out := map[string]bool{}
	for i, d := range docs {
		if i >= len(vals) || vals[i] == nil {
			continue
		}
		for _, l := range leaves(vals[i]) {
			if l.text != "" && replacedByCategoryMasker(d.policy, l) {
				out[generatedFold(l.text)] = true
			}
		}
	}
	return out
}

// generatedFold is lower(btrim(s)): the two things an expression over a leaf
// most often does to it (Supabase's `lower(identity_data ->> 'email')`), in
// the server's spelling -- btrim with no second argument trims spaces only.
func generatedFold(s string) string {
	return strings.ToLower(strings.Trim(s, " "))
}
