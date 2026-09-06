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
	phraseAddresses = "parse as addresses"
	phraseNameDict  = "in name dictionary"
	phraseAddrShape = "mixed digits and words"
	phraseE164      = "valid E.164"
	phraseIP        = "parse as IP addresses"
	phraseMAC       = "parse as MAC addresses"
	phraseLuhn      = "pass the Luhn check"
	phraseIBAN      = "pass the IBAN check"
	phraseSecrets   = "look like secrets"
	phraseProse     = "hold prose with dictionary names"
	phraseJSONLeaf  = "hold personal data at a JSON leaf"
)

// validatorPhrases is every phrase the fragment set will accept.
var validatorPhrases = []string{
	phraseAddresses,
	phraseNameDict,
	phraseAddrShape,
	phraseE164,
	phraseIP,
	phraseMAC,
	phraseLuhn,
	phraseIBAN,
	phraseSecrets,
	phraseProse,
	phraseJSONLeaf,
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
		name:    "fk_propagation",
		format:  "propagated through foreign key %s from %s",
		pattern: `propagated through foreign key ` + reIdent + ` from ` + reQualified,
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
		name:    "json_log_shaped",
		format:  "jsonb in a log-shaped table: the document is replaced whole",
		pattern: `jsonb in a log-shaped table: the document is replaced whole`,
	},
}

// phraseAlternation is the closed validator vocabulary, as a regexp branch. It
// is built in an init rather than written twice, so a phrase added to the list
// above cannot be forgotten here.
var phraseAlternation = buildPhraseAlternation()

func buildPhraseAlternation() string {
	quoted := make([]string, 0, len(validatorPhrases))
	for _, p := range validatorPhrases {
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
