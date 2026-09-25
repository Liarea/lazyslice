// SPDX-License-Identifier: Apache-2.0

package verify

import (
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
// pipeline.Decision.LeafMap, and the classification's phone region.
type leafPolicy struct {
	keys   map[string]pipeline.Category
	region string
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
// target could hold unchanged.
func leafRule(p leafPolicy, chain []string, text string, valued bool) leafVerdict {
	keys := p.keys
	if keys == nil {
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
// a per-leaf map, and the rule says a category's own masker, not free_text,
// replaced the leaf. With no map every leaf was free_text filler, which the net
// has always read, and still does.
func replacedByCategoryMasker(p leafPolicy, l leaf) bool {
	if p.keys == nil || !l.str {
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

// generatedFromMaskedLeaves reports a generated column over masked columns of
// its own table only (generatedFromMasked, ADR-015) at least one of which is a
// masked document carrying per-leaf categories. Such a column is computed by
// the target from leaves transform either replaced through a category masker,
// whose output is still a value of that category, or copied because no
// validator recognised them; so a hit from one of the leaf maskers'
// categories there is the masker's own output, and the net skips exactly
// those validators for it (netColumn). What that does not see, stated: an
// expression that assembles a personal value out of copied leaves none of
// which is personal alone — the quasi-identifier false negative
// ARCHITECTURE.md section 6 item 6 already lists.
func (s *state) generatedFromMaskedLeaves(t ref.TableRef, expr string) bool {
	if !s.generatedFromMasked(t, expr) {
		return false
	}
	for _, id := range exprIdentifiers(expr) {
		d, ok := s.decision(ref.ColumnRef{Table: t, Column: id})
		if ok && d.Masked && d.LeafMap() != nil {
			return true
		}
	}
	return false
}
