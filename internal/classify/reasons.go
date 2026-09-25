// SPDX-License-Identifier: Apache-2.0

package classify

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/Liarea/lazyslice/internal/ref"
)

// Decision.Reason is not free-form (ARCHITECTURE.md §2 "Value-free types").
// It is a list of fragments from the fixed set below, joined by "; ", and every
// placeholder in a fragment admits only an identifier, a category name, a type
// family, a fixed validator phrase or a count. A sample value can therefore
// never appear in a reason, and so never in lazyslice.yml or in an event.
//
// ParseReason is the enforcement: TestReasonGrammar renders every decision this
// package can produce and parses each one back against this set, failing on any
// string a fragment did not produce. Adding a template with a placeholder that
// could interpolate a value is the one change to this file that must never be
// made.

// A validator phrase is the closed vocabulary the "{n}/{m} samples {phrase}"
// fragment draws on. It is a phrase rather than an identifier because the
// example lines in ARCHITECTURE.md §10 are written that way; it is closed
// because an open one would be a free-form string field by another name.
const (
	phraseAddresses  = "parse as addresses"
	phraseNationalID = "parse as national identifiers"
	phraseNameDict   = "in name dictionary"
	phraseAddrShape  = "mixed digits and words"
	phraseE164       = "valid E.164"
	phraseIP         = "parse as IP addresses"
	phraseMAC        = "parse as MAC addresses"
	phraseLuhn       = "pass the Luhn check"
	phraseIBAN       = "pass the IBAN check"
	phraseSecrets    = "look like secrets"
	// phraseNamedFiles is the entropy validator's own reading of a column of
	// file names at least nameCorroborationThreshold of which carry a word
	// from the name dictionary (T-0315, namedFileNames): the column's
	// name-free file names are counted as secrets again, as they were before
	// textsig.LooksSecret stopped reading a file name as one.
	phraseNamedFiles = "are file names, a fifth or more carrying a dictionary name"
	phraseURL        = "parse as URLs"
	phraseProse      = "hold prose with dictionary names"
	phraseJSONLeaf   = "hold personal data at a JSON leaf"
	phraseByteaText  = "hold printable text that parses as personal data"
	// phraseE164Region is buildValidators' region-aware phone entry (T-0221):
	// a value that parses only under the configured --phone-region/
	// phone_region, not under phraseE164's fixed "ZZ" (international-only)
	// hint. decide's own regionAssumed appends a second fragment naming the
	// region itself.
	phraseE164Region = "valid national-format phone numbers"
	// phraseGuessedPhone is guessedPhoneHit's own (T-0221): a value that
	// parses under one of phoneGuessRegions, with no region configured. It
	// never decides a column by itself -- see guessedPhoneColumns.
	phraseGuessedPhone = "parse as phone numbers under a guessed region"
)

// validatorPhrases is every phrase the fragment set will accept.
var validatorPhrases = []string{
	phraseAddresses,
	phraseNationalID,
	phraseNameDict,
	phraseAddrShape,
	phraseE164,
	phraseIP,
	phraseMAC,
	phraseLuhn,
	phraseIBAN,
	phraseSecrets,
	phraseNamedFiles,
	phraseURL,
	phraseProse,
	phraseJSONLeaf,
	phraseByteaText,
	phraseE164Region,
	phraseGuessedPhone,
}

// The placeholder classes. Each is a character set, not a wildcard: a category
// cannot hold a digit and a count is digits only, so no fragment can be made to
// carry prose.
//
// An identifier is either bare or quoted. PostgreSQL identifiers are not
// restricted to the bare class -- a schema may be called my-app and a table
// "user events" -- and a reason that interpolated one of those verbatim would
// not parse against this set, which would break the invariant §10 states rather
// than the database that is at fault. quoteIdent renders anything outside the
// bare class in the quoted form below, whose contents can hold no unescaped
// quote and no semicolon at all, so a quoted identifier can neither end its own
// fragment nor forge the "; " that separates two.
const (
	reBare      = `[A-Za-z0-9_$]+`
	reQuoted    = `"(?:[^"\\;]|\\.)*"`
	reIdent     = `(?:` + reBare + `|` + reQuoted + `)`
	reQualified = reIdent + `(?:\.` + reIdent + `)+`
	reCategory  = `[a-z_]+`
	reFamily    = `[a-z0-9_]+`
	reCount     = `[0-9]+`
)

// bareIdent is the class quoteIdent may leave unquoted.
var bareIdent = regexp.MustCompile(`\A` + reBare + `\z`)

// quoteIdent renders one identifier inside a fragment. A name made only of the
// bare class is written as it is; anything else is written as a Go-quoted
// string, with every semicolon escaped as well, so that no identifier can carry
// the "; " that joinReason and ParseReason use as the fragment separator.
func quoteIdent(s string) string {
	if bareIdent.MatchString(s) {
		return s
	}
	return strings.ReplaceAll(strconv.Quote(s), ";", `\x3b`)
}

// quoteTable renders "schema.table" with each half quoted independently, which
// is what keeps a hyphen in a schema name inside the grammar.
func quoteTable(t ref.TableRef) string {
	return quoteIdent(t.Schema) + "." + quoteIdent(t.Name)
}

// quoteColumn renders "schema.table.column" the same way.
func quoteColumn(c ref.ColumnRef) string {
	return quoteTable(c.Table) + "." + quoteIdent(c.Column)
}

// fragment is one sentence of a reason.
type fragment struct {
	// name identifies the fragment in a test failure.
	name string
	// format is the fmt template used to render it.
	format string
	// pattern is the anchored regexp that parses it back.
	pattern string
	re      *regexp.Regexp
}

// fragments is the whole vocabulary of Decision.Reason. Nothing else is a legal
// reason.
var fragments = []*fragment{
	{
		name:    "no_signal",
		format:  "no name or value signal",
		pattern: `no name or value signal`,
	},
	{
		name:    "nothing_recognised",
		format:  "nothing recognised in %d samples, not proof the column is impersonal",
		pattern: `nothing recognised in ` + reCount + ` samples, not proof the column is impersonal`,
	},
	{
		// sub_threshold_signal is nothing_recognised's honest twin (T-0197
		// review finding 1): rendered instead of it when a validator matched
		// at least one sample but too few of them, or too few samples
		// overall, to decide the column -- so the line never claims nothing
		// was seen when something was.
		name:    "sub_threshold_signal",
		format:  "a validator matched below the threshold needed to decide in %d samples, not proof the column is impersonal",
		pattern: `a validator matched below the threshold needed to decide in ` + reCount + ` samples, not proof the column is impersonal`,
	},
	{
		name:    "no_name_signal",
		format:  "no name signal",
		pattern: `no name signal`,
	},
	{
		name:    "name_match",
		format:  "name matches %s",
		pattern: `name matches ` + reIdent,
	},
	{
		// T-0313: the four answers bareNameVerdict gives about a column the
		// rule pack's bare_name rule matched. The bare word says a column
		// holds *a* name, so each line says what corroborated it as a
		// person's, or that nothing did.
		name:    "bare_name_by_word",
		format:  "a bare name, corroborated by %s, a word for people",
		pattern: `a bare name, corroborated by ` + reIdent + `, a word for people`,
	},
	{
		name:    "bare_name_by_samples",
		format:  "a bare name, corroborated by %d/%d samples carrying a word from the name dictionary",
		pattern: `a bare name, corroborated by ` + reCount + `/` + reCount + ` samples carrying a word from the name dictionary`,
	},
	{
		name:    "bare_name_unproven",
		format:  "a bare name, masked on the name alone: %d samples are too few to check against the name dictionary",
		pattern: `a bare name, masked on the name alone: ` + reCount + ` samples are too few to check against the name dictionary`,
	},
	{
		name: "bare_name_uncorroborated",
		format: "a bare name, not corroborated: no word for people in the table or column name, " +
			"and %d/%d samples carry a word from the name dictionary, so the name alone decides no more than low",
		pattern: `a bare name, not corroborated: no word for people in the table or column name, ` +
			`and ` + reCount + `/` + reCount + ` samples carry a word from the name dictionary, so the name alone decides no more than low`,
	},
	{
		name:    "samples",
		format:  "%d/%d samples %s",
		pattern: reCount + `/` + reCount + ` samples (?:` + phraseAlternation + `)`,
	},
	{
		name:    "no_samples",
		format:  "no samples",
		pattern: `no samples`,
	},
	{
		name:    "type_conflict",
		format:  "%s is not an accepted type for %s",
		pattern: reFamily + ` is not an accepted type for ` + reCategory,
	},
	{
		name:    "type_signal",
		format:  "type %s is a %s type",
		pattern: `type ` + reFamily + ` is a ` + reCategory + ` type`,
	},
	{
		name:    "special_by_name",
		format:  "special category by name alone",
		pattern: `special category by name alone`,
	},
	{
		name:    "neighbour",
		format:  "raised by the neighbouring-column rule: %s has %d columns at likely or above",
		pattern: `raised by the neighbouring-column rule: ` + reQualified + ` has ` + reCount + ` columns at likely or above`,
	},
	{
		// The 2026-09-15 red team's A2b. The ordinary neighbour fragment above
		// is for a column that had a signal of its own; this one is for a
		// column that had none at all, which is a different claim and has to
		// read as one on the report.
		name:   "neighbour_unknown",
		format: "raised by the neighbouring-column rule: %s has %d columns at certain and nothing is known about this column's contents",
		pattern: `raised by the neighbouring-column rule: ` + reQualified + ` has ` + reCount +
			` columns at certain and nothing is known about this column's contents`,
	},
	{
		// T-0311: the three skips of the sweep above, each naming what it
		// found. They never raise anything; a column carrying one is copied,
		// and the line says why the certain neighbour did not change that.
		name: "spared_unique",
		format: "not swept by the neighbouring-column rule (%s has %d columns at certain): " +
			"under a unique index, which free_text could not keep distinct",
		pattern: `not swept by the neighbouring-column rule \(` + reQualified + ` has ` + reCount +
			` columns at certain\): under a unique index, which free_text could not keep distinct`,
	},
	{
		name: "spared_identifier",
		format: "not swept by the neighbouring-column rule (%s has %d columns at certain): " +
			"all %d samples are %s, an identifier shape and not free text",
		pattern: `not swept by the neighbouring-column rule \(` + reQualified + ` has ` + reCount +
			` columns at certain\): all ` + reCount + ` samples are (?:` + identifierAlternation +
			`), an identifier shape and not free text`,
	},
	{
		name: "spared_enum",
		format: "not swept by the neighbouring-column rule (%s has %d columns at certain): " +
			"enum-like, %d distinct values in %d samples and each seen at least twice " +
			"(the rule spares at most %d distinct values in %d or more samples)",
		pattern: `not swept by the neighbouring-column rule \(` + reQualified + ` has ` + reCount +
			` columns at certain\): enum-like, ` + reCount + ` distinct values in ` + reCount +
			` samples and each seen at least twice \(the rule spares at most ` + reCount +
			` distinct values in ` + reCount + ` or more samples\)`,
	},
	{
		name:    "fk_propagation",
		format:  "propagated through foreign key %s from %s",
		pattern: `propagated through foreign key ` + reIdent + ` from ` + reQualified,
	},
	{
		// T-0253: unknownColumnsBesideCertain's join-safety pairing
		// (fkPairs). A column at either end of a validated foreign key is
		// raised together with every column paired to it, never alone, so
		// each member's line names the other so the pairing is auditable
		// from either side.
		name:    "fk_pair",
		format:  "raised together with %s so a validated foreign key stays in agreement",
		pattern: `raised together with ` + reQualified + ` so a validated foreign key stays in agreement`,
	},
	{
		// T-0253/T-0257: fkPairs' own refusal, when a validated foreign
		// key's partner cannot be raised the same way. Rendered onto the
		// blocked column's ordinary Reason (so the explain line says why
		// nothing was raised) and, through the same fixed template set,
		// onto the new Decision.Refused field that internal/plan does
		// not read yet (see that field's own comment).
		name:    "fk_pair_refused",
		format:  "cannot be raised without copying foreign-key partner %s: refusing the pair rather than masking one end",
		pattern: `cannot be raised without copying foreign-key partner ` + reQualified + `: refusing the pair rather than masking one end`,
	},
	{
		name:    "same_name",
		format:  "column name %s is %s in %s",
		pattern: `column name ` + reIdent + ` is ` + reCategory + ` in ` + reQualified,
	},
	{
		name:    "generated",
		format:  "generated column: not copied, the target recomputes it",
		pattern: `generated column: not copied, the target recomputes it`,
	},
	{
		name:    "surrogate_key",
		format:  "surrogate key: preserved verbatim",
		pattern: `surrogate key: preserved verbatim`,
	},
	{
		name:    "fk_column",
		format:  "foreign key to %s: preserved verbatim",
		pattern: `foreign key to ` + reQualified + `: preserved verbatim`,
	},
	{
		// T-0314: schema_migrations, ar_internal_metadata and the rest of
		// pipeline.IsFrameworkMetadataTable's list.
		name:    "framework_metadata",
		format:  "framework metadata table: copied whole, never masked",
		pattern: `framework metadata table: copied whole, never masked`,
	},
	{
		// The child end of tracker T-0120's reconciliation (keyChildren). It
		// names the parent column, because "this column looks personal and is
		// copied anyway" is only answerable by pointing at the key that is
		// copied too.
		name:    "key_child_exempt",
		format:  "foreign key to %s, a copied surrogate key: preserved verbatim",
		pattern: `foreign key to ` + reQualified + `, a copied surrogate key: preserved verbatim`,
	},
	{
		name:    "unmask_yml",
		format:  "opt-out recorded in lazyslice.yml",
		pattern: `opt-out recorded in lazyslice.yml`,
	},
	{
		name:    "unmask_flag",
		format:  "opt-out recorded by --unmask",
		pattern: `opt-out recorded by --unmask`,
	},
	{
		// A user pattern is a regexp, which is not an identifier, so the
		// fragment names the source and not the rule. A reason that
		// interpolated a caller-supplied regexp would be a free-form string
		// field by another name.
		name:    "yml_raise",
		format:  "raised by a lazyslice.yml pattern",
		pattern: `raised by a lazyslice\.yml pattern`,
	},
	{
		name:    "yml_column",
		format:  "raised by lazyslice.yml for this column",
		pattern: `raised by lazyslice.yml for this column`,
	},
	{
		// A raise that names no category the column's type accepts cannot be
		// applied: Decision.Masker is chosen from the category, so a masked
		// decision with no category is a column the transform stage is told to
		// mask with nothing to mask it with (ADR-006). The raise is dropped and
		// this fragment says so on the column's own line, rather than the yml
		// silently doing nothing.
		name:    "yml_no_category",
		format:  "a lazyslice.yml raise names no usable category: not applied",
		pattern: `a lazyslice\.yml raise names no usable category: not applied`,
	},
	{
		name:    "bytea_person_shaped",
		format:  "bytea in a person-shaped table (%s has %d columns at certain)",
		pattern: `bytea in a person-shaped table \(` + reQualified + ` has ` + reCount + ` columns at certain\)`,
	},
	{
		name:    "partition_samples",
		format:  "samples from partition %s",
		pattern: `samples from partition ` + reQualified,
	},
	{
		name:    "no_leaf_samples",
		format:  "no samples: partitioned table with no leaves",
		pattern: `no samples: partitioned table with no leaves`,
	},
	{
		name:    "array_element",
		format:  "classified on the element type %s",
		pattern: `classified on the element type ` + reFamily,
	},
	{
		name:    "two_letter_codes",
		format:  "value shape: 2-letter codes",
		pattern: `value shape: 2-letter codes`,
	},
	{
		// T-0054. The fragment names the type rather than the column, because
		// the rule is about the type: every tsvector is derived text, whatever
		// it is called and whatever its values look like.
		name:    "derived_text",
		format:  "tsvector is derived from text that may be masked",
		pattern: `tsvector is derived from text that may be masked`,
	},
	{
		// T-0094. The fragment names the type family and not the column,
		// because the rule is about the type: no category accepts a composite
		// and no masker can write one, so a composite that carries personal
		// data is a refusal at plan and never a copy.
		name:    "composite_refused",
		format:  "composite type: no masker can write into one, so the plan refuses rather than copy it",
		pattern: `composite type: no masker can write into one, so the plan refuses rather than copy it`,
	},
	{
		name:    "composite_no_signal",
		format:  "composite type: its fields were read and none is personal data",
		pattern: `composite type: its fields were read and none is personal data`,
	},
	{
		// The same copy decision reached without a single value to read. The
		// fragment above claims a check that ran; this one says the check did
		// not, so the audit line for an empty table does not read "its fields
		// were read and none is personal data; no samples" (T-HARD-B).
		name:    "composite_no_sample",
		format:  "composite type: no field value was read, so only its name was checked",
		pattern: `composite type: no field value was read, so only its name was checked`,
	},
	{
		// T-0399, the 2026-09-25 JSON red team round 1, entry 14: a
		// composite's own type holds a json, jsonb or hstore field (or a
		// nested composite that does). compositeSignal runs every validator
		// over each field's text as one whole value -- not over the
		// document underneath it -- so a bare value inside the document is
		// read only by chance, not by a control. The column is refused on
		// the type alone, the same as a composite compositeSignal did find
		// a hit on, because there is no sample size that makes that chance
		// safe.
		name:    "composite_document_field",
		format:  "composite type %s has %s field %s, whose document a validator never reads inside -- only the field's own text as a whole",
		pattern: `composite type ` + reIdent + ` has ` + reFamily + ` field ` + reIdent + `, whose document a validator never reads inside -- only the field's own text as a whole`,
	},
	{
		name:    "json_log_shaped",
		format:  "jsonb in a log-shaped table: the document is replaced whole",
		pattern: `jsonb in a log-shaped table: the document is replaced whole`,
	},
	{
		// docs/reviews/2026-09-09/REVIEW.md finding 7: a strong validator
		// (a precise parse) matched at least one proven sample without the
		// column reaching validatorThreshold. The column is mixed rather
		// than reliably one category, so it is masked as free_text instead
		// of being copied on the strength of a minority ratio.
		name:    "strong_hit_free_text",
		format:  "a strong validator hit below the category threshold: masked as free_text, --unmask to keep it unmasked",
		pattern: `a strong validator hit below the category threshold: masked as free_text, --unmask to keep it unmasked`,
	},
	{
		// T-0221: names the region a national-format phone hit was read
		// under, so the operator can see the assumption rather than infer it
		// from the flag they may not have typed on this run (a committed
		// phone_region: carries forward with no flag needed). regionAssumed
		// in classify.go is what decides whether this is appended.
		name:    "phone_region_configured",
		format:  "phone region assumed: %s",
		pattern: `phone region assumed: ` + reIdent,
	},
	{
		// T-0221's other half: a guessed-region phone hit that
		// guessedPhoneColumns decided to trust, because the column's own name
		// already matches rules.yml's phone pattern or a neighbouring column
		// in the same table is proven personal (the T-0187 pattern
		// requiresCorroboration uses on internal/verify's side).
		name:    "phone_region_guessed",
		format:  "no --phone-region configured, masked on a guessed-region hit corroborated by name or a personal neighbour",
		pattern: `no --phone-region configured, masked on a guessed-region hit corroborated by name or a personal neighbour`,
	},
}

// phraseAlternation is the closed validator vocabulary, as a regexp branch. It
// is built in an init rather than written twice, so a phrase added to the list
// above cannot be forgotten here.
var phraseAlternation = buildPhraseAlternation()

func buildPhraseAlternation() string {
	return alternation(validatorPhrases)
}

// identifierAlternation is spare.go's closed identifier-shape vocabulary
// (T-0311), built the same way and for the same reason.
var identifierAlternation = alternation(identifierPhrases)

func alternation(phrases []string) string {
	quoted := make([]string, 0, len(phrases))
	for _, p := range phrases {
		quoted = append(quoted, regexp.QuoteMeta(p))
	}
	return strings.Join(quoted, "|")
}

func init() {
	for _, f := range fragments {
		f.re = regexp.MustCompile(`\A(?:` + f.pattern + `)\z`)
	}
}

// render builds one fragment. The caller passes only identifiers, category
// names, type families, validator phrases and counts; ParseReason is what makes
// that a checked claim rather than a convention.
func render(name string, args ...any) string {
	for _, f := range fragments {
		if f.name == name {
			if len(args) == 0 {
				return f.format
			}
			return fmt.Sprintf(f.format, args...)
		}
	}
	// Unreachable through the package's own callers: every name below is a
	// constant in this file. Returning the marker rather than panicking keeps a
	// mistake visible in a test instead of taking a run down.
	return "unknown reason fragment " + name
}

// joinReason joins fragments in the order they were produced, dropping empties.
func joinReason(parts ...string) string {
	kept := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			kept = append(kept, p)
		}
	}
	if len(kept) == 0 {
		return render("no_signal")
	}
	return strings.Join(kept, "; ")
}

// ParseReason reports whether s was produced by this file's fragment set, and
// names the first fragment that was not. It is exported so that the reason
// grammar can be asserted from anywhere a Decision reaches — the emitter's yml
// and the renderer's line, as well as this package's own tests.
func ParseReason(s string) (bad string, ok bool) {
	if s == "" {
		return "", false
	}
	for _, part := range strings.Split(s, "; ") {
		matched := false
		for _, f := range fragments {
			if f.re.MatchString(part) {
				matched = true
				break
			}
		}
		if !matched {
			return part, false
		}
	}
	return "", true
}
