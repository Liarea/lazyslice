// SPDX-License-Identifier: Apache-2.0

package mask

import "slices"

// Can this category's masked output be written into this column?
//
// It is the question T-0054 was opened for. On pagila the classifier decided
// `credential` on every `last_update timestamptz` — the text form of a
// timestamp mixes character classes, carries no space and no "@", and clears
// the entropy threshold — and `address` on `film.fulltext tsvector`, whose
// lexemes read as digits and words. internal/transform was then handed the
// fixed-literal masker for a timestamp column, could not parse
// "$lazyslice$invalid" back into a time, and refused at exit 7 in the middle of
// a run that had already moved rows. Every whole-pipeline run over pagila died
// there.
//
// The fix has three parts and this is the one they share. internal/classify no
// longer decides a category its column's type cannot hold (the accepted-types
// gate now runs over the value signals too), internal/plan refuses before any
// row moves, and internal/transform keeps its refusal as a backstop. The
// classifier gates on the rule pack's `accepts:` lists; the planner gates on
// this table; TestRulePackAgreesWithMaskAboutTypes in internal/classify walks
// the two against each other, so the second declaration is a check on the first
// rather than a copy of it that can drift.
//
// The two lists are not the same question, and the test knows about the one
// place they part company. `accepts:` says which families a *decision* may be
// made on, and it errs towards deciding, because a category it silences is a
// column copied in cleartext. This table says which families a *generator* can
// write into, and it errs towards refusing, because a family it admits wrongly
// is a run that dies in the loader. Where this table is the narrower of the two
// -- special_category -- the gap is a plan refusal at exit 12, which is the
// direction that fails safe. Where it were ever the wider, the plan would admit
// something the classifier had already declined to decide, and the test fails.
//
// This module is the right place for the second one: whether a generator's
// output fits a column is a property of the generator, and the generators are
// here.

// writableTags is, per category, the type tags a value from that category's
// generators can be written into.
//
// It is deliberately a declaration and not a probe. A probe would have to run a
// generator and try to parse its output back, which is what internal/transform
// already does per value and per row; the point of the plan-time check is to
// answer the question once, before the snapshot is used, from something a
// reader can audit against the rule pack line by line.
var writableTags = map[Category][]string{
	CatEmail:      {famText, famVarchar, famBpchar, famCitext},
	CatPersonName: {famText, famVarchar, famBpchar, famCitext},
	CatPhone:      {famText, famVarchar, famBpchar, famCitext, famBigint, famInteger, famNumeric},
	CatAddress:    {famText, famVarchar, famBpchar, famCitext},
	CatGeo:        {famText, famVarchar, famBpchar, famCitext, famNumeric, famFloat},
	CatPersonDate: {famDate, famTimestamp, famText, famVarchar, famBpchar},
	CatNationalID: {famText, famVarchar, famBpchar, famCitext, famBigint, famInteger, famNumeric},
	CatFinancial:  {famText, famVarchar, famBpchar, famCitext, famBigint, famInteger, famNumeric},
	CatNetworkID:  {famInet, famCIDR, famMacaddr, famText, famVarchar, famBpchar, famCitext},
	CatOnlineID:   {famText, famVarchar, famBpchar, famCitext, famUUID},
	CatCredential: {famText, famVarchar, famBpchar, famCitext, famBytea},
	CatFreeText:   {famText, famVarchar, famBpchar, famCitext},
	// special_category has no "every family" entry, though the rule pack spells
	// its accepts: as ["*"]. The two lists answer different questions and this
	// is the one place they differ; TestRulePackAgreesWithMaskAboutTypes states
	// the asymmetry. The pack's ["*"] keeps the classifier deciding
	// special_category on any type at all -- it is scored `certain` by name
	// alone (ARCHITECTURE.md §4) and silencing it by type would copy an
	// hiv_status column in cleartext. What the *generator* can write is
	// narrower: specialCategoryMasker.Mask calls generic (gen_number.go), whose
	// switch covers boolean, the numeric families, date, timestamp, uuid, bytea
	// and the document families and falls through to the free-text filler for
	// everything else. Free-text filler is not a time, an interval, an address
	// or a tsvector, so those families are refused at plan with exit 12 instead
	// of failing in the loader mid-run: `health_check_ip inet` matches the
	// pattern on "health", and `medication_time time` on "medications".
	// (collapsed() can write a zero value into some of them, but Writable is
	// answered per column before the collapse rule is, and the direction that
	// fails safe is the refusal.)
	CatSpecial: {
		famText, famVarchar, famBpchar, famCitext,
		famBoolean, famInteger, famBigint, famNumeric, famFloat,
		famDate, famTimestamp, famUUID, famBytea,
		famJSON, famJSONB, famHstore,
	},
	CatBinary:      {famBytea},
	CatSemiStruct:  {famJSON, famJSONB, famHstore},
	CatDerivedText: {famTSVector},
}

// WritableTypes lists the type tags a category's generators can write into. It
// is exported for the test in internal/classify that reads it against the rule
// pack's accepts: lists; nothing in the pipeline reads the list itself, only
// Writable's answer about one column.
func WritableTypes(cat Category) []string {
	return slices.Clone(writableTags[cat])
}

// Writable reports whether a value the generator under id, masking for this
// category, can be written into a column with these constraints. It is two
// questions, and a column has to pass both:
//
//  1. Is there any value at all for the generator to emit here? That is the
//     admissible domain, and a zero one is the refusal Pick already makes for a
//     column too small to hold a masked value (ARCHITECTURE.md §5) — a `phone`
//     column declared `integer` is the live example: E.164 does not fit in nine
//     digits and the generator has nothing to write.
//  2. Can the column's type hold the *kind* of value the category emits? That
//     is writableTags. A domain says nothing about it: the credential literal
//     has a domain of 1 on every column in the world, and "$lazyslice$invalid"
//     is still not a timestamp.
//
// A category with no entry at all — CatNone, or a string no category names — is
// not writable anywhere: a column masked under a category no generator is
// registered for has nothing to write into it.
//
// A column with labels passes the second question under every category,
// whatever its type: every generator here begins with labelValue and answers a
// labelled column with one of its own labels (ARCHITECTURE.md §5, "a masked
// enum is a valid label"), so an enum or a CHECK-list column never depends on
// the family at all.
func Writable(cat Category, id ID, c Constraints) bool {
	if Admissible(id, c) <= 0 {
		return false
	}
	if len(labels(c)) > 0 {
		return true
	}
	return slices.Contains(writableTags[cat], c.TypeTag)
}
