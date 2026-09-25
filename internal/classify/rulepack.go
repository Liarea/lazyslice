// SPDX-License-Identifier: Apache-2.0

package classify

import (
	_ "embed"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/goccy/go-yaml"

	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/mask"
)

// rulesYAML is the rule pack. It is embedded, so there is no file to point a
// flag at and no rule to load at runtime: the classifier is not pluggable
// (ADR-006).
//
//go:embed rules.yml
var rulesYAML []byte

// rulePack is rules.yml as written.
type rulePack struct {
	Version       string             `yaml:"version"`
	Categories    []categoryRule     `yaml:"categories"`
	Patterns      []patternRule      `yaml:"patterns"`
	TablePatterns []tablePatternRule `yaml:"table_patterns"`
	Tables        tableRules         `yaml:"tables"`
}

type categoryRule struct {
	Category string   `yaml:"category"`
	Masker   string   `yaml:"masker"`
	Accepts  []string `yaml:"accepts"`
}

type patternRule struct {
	Name     string `yaml:"name"`
	Category string `yaml:"category"`
	Priority int    `yaml:"priority"`
	Match    string `yaml:"match"`
	// Unless is a regexp over the same normalised column name as Match; a
	// column it matches is outside the rule altogether (T-0313: a
	// `*_file_name` column is a label, not a person's name).
	Unless string `yaml:"unless"`
	// CorroboratedBy is a regexp over the normalised table name and the
	// normalised column name. A rule carrying one decides a column at
	// `possible` only when it matches either, or when the samples corroborate
	// the rule instead (T-0313; classify.go's bareNameVerdict).
	CorroboratedBy string `yaml:"corroborated_by"`
}

// tablePatternRule is one row of table_patterns: a name rule that only
// applies within a table its own `table` regexp matches (T-0119). `match`,
// `category` and `priority` are read exactly as patternRule's are; `table` is
// the regexp a name rule has no field for.
type tablePatternRule struct {
	Name     string `yaml:"name"`
	Category string `yaml:"category"`
	Priority int    `yaml:"priority"`
	Table    string `yaml:"table"`
	Match    string `yaml:"match"`
}

type tableRules struct {
	LogShaped string `yaml:"log_shaped"`
}

// compiledPattern is one name rule with its regexp built. tableRe is nil for
// an ordinary patterns: rule, which applies inside every table; a
// table_patterns: rule sets it, and matchColumn skips the rule in a table its
// tableRe does not match.
type compiledPattern struct {
	Name     string
	Category pipeline.Category
	Priority int
	re       *regexp.Regexp
	tableRe  *regexp.Regexp
	// unless and corroborate are patternRule's Unless and CorroboratedBy,
	// compiled, or nil where the row has none (T-0313).
	unless      *regexp.Regexp
	corroborate *regexp.Regexp
}

// matches reports whether the rule's own column regexp matches a normalised
// column name that its unless regexp does not.
func (pat compiledPattern) matches(normalised string) bool {
	if !pat.re.MatchString(normalised) {
		return false
	}
	return pat.unless == nil || !pat.unless.MatchString(normalised)
}

// needsCorroboration reports whether the rule decides a column only with
// corroboration (T-0313).
func (pat compiledPattern) needsCorroboration() bool { return pat.corroborate != nil }

// corroboratedByName reports whether the rule's corroborated_by regexp matches
// the normalised table name or the normalised column name, and returns the
// word it matched there (without the underscores that bound it), which is
// what the reason line names.
func (pat compiledPattern) corroboratedByName(normalisedTable, normalised string) (string, bool) {
	if pat.corroborate == nil {
		return "", false
	}
	for _, name := range [2]string{normalisedTable, normalised} {
		if m := pat.corroborate.FindString(name); m != "" {
			return strings.Trim(m, "_"), true
		}
	}
	return "", false
}

// compiledPack is the rule pack in the form the scorer uses.
type compiledPack struct {
	Version  string
	Patterns []compiledPattern
	// ColumnPatterns is Patterns plus every table_patterns: rule, resorted
	// together by the one priority line the two share. matchColumn reads this;
	// match reads Patterns alone, because the JSON-leaf path that calls it
	// (jsonLeafIsPersonal) has no table to test a table-scoped rule against.
	ColumnPatterns []compiledPattern
	// Masker is the mask.ID a decision in that category carries.
	Masker map[pipeline.Category]mask.ID
	// Accepts is the type-family set each category's maskers accept. A nil set
	// means every family: only special_category has one, because it is scored
	// certain by name alone and masks an enum (testdata/README.md trap 24).
	Accepts   map[pipeline.Category]map[string]bool
	logShaped *regexp.Regexp
}

// accepted reports whether a category's maskers accept a type family.
func (p *compiledPack) accepted(cat pipeline.Category, family string) bool {
	set, ok := p.Accepts[cat]
	if !ok {
		return false
	}
	if set == nil {
		return true
	}
	return set[family]
}

// logShapedTable reports whether a table is named like the audit, log, history
// or event tables whose jsonb ARCHITECTURE.md §4 replaces whole.
//
// It tries the regex against two different folds of name and ORs the result,
// rather than against normaliseName alone (T-0398 review round, high
// finding). normaliseName's needsBreak splits a trailing lower-case run off
// an all-caps word ("IDToken" -> "id_token"), which is right for an acronym
// but wrong for a plural: "EVENTs" normalises to "even_ts", "LOGs" to
// "lo_gs", "AUDITs" to "audi_ts" -- none of which the log_shaped regex's
// whole-word match can ever see, so a table named that way (or with the
// upper-case word embedded, "user_LOGs", "x_LOGs_y") was reported log-shaped
// by nothing here while the reason fragment and the deleted logTableWords
// copy this rule replaced both called it one. lowerUnderscoreFold is
// normaliseName without the case-driven word splitting -- lower-case the
// letters, turn every other non-name rune into '_', and leave the rest
// alone, the same fold logTableWords used (strings.ToLower plus a split on
// '_') -- so the two folds together are a strict superset of either alone:
// normaliseName still catches a CamelCase table with no underscore at all
// ("AuditLog"), and lowerUnderscoreFold catches the all-caps-plural and
// embedded-acronym shapes normaliseName's word-splitting defeats. Root
// CLAUDE.md's rule is "when in doubt, mask it": this must only ever gain
// matches over either fold alone, never lose one, so the two are ORed and
// neither replaces the other.
func (p *compiledPack) logShapedTable(name string) bool {
	if p.logShaped == nil {
		return false
	}
	return p.logShaped.MatchString(normaliseName(name)) || p.logShaped.MatchString(lowerUnderscoreFold(name))
}

// lowerUnderscoreFold lower-cases name and turns every rune that is not a
// lower-case letter, a digit or '_' into '_', with no case-boundary word
// splitting at all -- see logShapedTable, the one caller this exists for.
func lowerUnderscoreFold(s string) string {
	out := make([]rune, 0, len(s))
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '_':
			out = append(out, r)
		case r >= 'A' && r <= 'Z':
			out = append(out, r-'A'+'a')
		default:
			out = append(out, '_')
		}
	}
	return string(out)
}

// pack is the compiled rule pack. It is loaded once; a malformed pack is a
// build-time mistake, so the error is returned from Classify rather than
// panicking at init, which would take an unrelated subcommand down with it.
var pack = sync.OnceValues(loadPack)

func loadPack() (*compiledPack, error) {
	return decodePack(rulesYAML)
}

// decodePack is loadPack's body, over an arbitrary rule pack rather than only
// the embedded one, so that a malformed-pack error path can be exercised
// directly (rulepack_test.go) instead of only through the one rule pack that
// ships.
func decodePack(rulesYAML []byte) (*compiledPack, error) {
	var raw rulePack
	if err := yaml.Unmarshal(rulesYAML, &raw); err != nil {
		return nil, fmt.Errorf("classify: reading the embedded rule pack: %w", err)
	}
	if raw.Version == "" {
		return nil, fmt.Errorf("classify: the embedded rule pack has no version")
	}
	c := &compiledPack{
		Version: raw.Version,
		Masker:  map[pipeline.Category]mask.ID{},
		Accepts: map[pipeline.Category]map[string]bool{},
	}
	for _, cr := range raw.Categories {
		cat := pipeline.Category(cr.Category)
		if cat == pipeline.CatNone {
			return nil, fmt.Errorf("classify: the rule pack declares a masker for %q, the category that names the absence of a signal", cat)
		}
		if cr.Masker == "" {
			return nil, fmt.Errorf("classify: category %q has no masker; every category a column can be masked under has one (ADR-006)", cat)
		}
		c.Masker[cat] = mask.ID(cr.Masker)
		if len(cr.Accepts) == 1 && cr.Accepts[0] == "*" {
			c.Accepts[cat] = nil
			continue
		}
		set := map[string]bool{}
		for _, f := range cr.Accepts {
			set[f] = true
		}
		c.Accepts[cat] = set
	}
	for _, pr := range raw.Patterns {
		cat := pipeline.Category(pr.Category)
		if _, ok := c.Masker[cat]; !ok {
			return nil, fmt.Errorf("classify: pattern %q names category %q, which the rule pack does not declare", pr.Name, cat)
		}
		re, err := regexp.Compile(pr.Match)
		if err != nil {
			return nil, fmt.Errorf("classify: pattern %q: %w", pr.Name, err)
		}
		if bad, ok := ParseReason(render("name_match", pr.Name)); !ok {
			return nil, fmt.Errorf("classify: pattern name %q does not render inside the reason grammar: %q", pr.Name, bad)
		}
		cp := compiledPattern{
			Name:     pr.Name,
			Category: cat,
			Priority: pr.Priority,
			re:       re,
		}
		if pr.Unless != "" {
			if cp.unless, err = regexp.Compile(pr.Unless); err != nil {
				return nil, fmt.Errorf("classify: pattern %q: unless: %w", pr.Name, err)
			}
		}
		if pr.CorroboratedBy != "" {
			// The only value evidence that can corroborate a name rule is the
			// name dictionary (bareNameVerdict), so the field means nothing on
			// any other category, and a rule that carried it anyway would be
			// decided on evidence about a different category.
			if cat != pipeline.CatPersonName {
				return nil, fmt.Errorf("classify: pattern %q: corroborated_by is only read on a %s rule, and this one is %s", pr.Name, pipeline.CatPersonName, cat)
			}
			if cp.corroborate, err = regexp.Compile(pr.CorroboratedBy); err != nil {
				return nil, fmt.Errorf("classify: pattern %q: corroborated_by: %w", pr.Name, err)
			}
		}
		c.Patterns = append(c.Patterns, cp)
	}
	if len(c.Patterns) == 0 {
		return nil, fmt.Errorf("classify: the embedded rule pack has no name patterns")
	}
	// Highest priority first, then by name, so that two rules matching one
	// column resolve the same way on every run and in every build.
	byPriorityThenName := func(pats []compiledPattern) func(i, j int) bool {
		return func(i, j int) bool {
			if pats[i].Priority != pats[j].Priority {
				return pats[i].Priority > pats[j].Priority
			}
			return pats[i].Name < pats[j].Name
		}
	}
	sort.SliceStable(c.Patterns, byPriorityThenName(c.Patterns))
	c.ColumnPatterns = append(c.ColumnPatterns, c.Patterns...)
	for _, tr := range raw.TablePatterns {
		cat := pipeline.Category(tr.Category)
		if _, ok := c.Masker[cat]; !ok {
			return nil, fmt.Errorf("classify: table pattern %q names category %q, which the rule pack does not declare", tr.Name, cat)
		}
		if tr.Table == "" {
			return nil, fmt.Errorf("classify: table pattern %q has no table: regexp; an empty one would compile and match every table, turning it into a global rule", tr.Name)
		}
		if tr.Match == "" {
			return nil, fmt.Errorf("classify: table pattern %q has no match: regexp; an empty one would compile and match every column in a matching table", tr.Name)
		}
		tableRe, err := regexp.Compile(tr.Table)
		if err != nil {
			return nil, fmt.Errorf("classify: table pattern %q: table: %w", tr.Name, err)
		}
		re, err := regexp.Compile(tr.Match)
		if err != nil {
			return nil, fmt.Errorf("classify: table pattern %q: match: %w", tr.Name, err)
		}
		if bad, ok := ParseReason(render("name_match", tr.Name)); !ok {
			return nil, fmt.Errorf("classify: table pattern name %q does not render inside the reason grammar: %q", tr.Name, bad)
		}
		c.ColumnPatterns = append(c.ColumnPatterns, compiledPattern{
			Name:     tr.Name,
			Category: cat,
			Priority: tr.Priority,
			re:       re,
			tableRe:  tableRe,
		})
	}
	sort.SliceStable(c.ColumnPatterns, byPriorityThenName(c.ColumnPatterns))
	if raw.Tables.LogShaped != "" {
		re, err := regexp.Compile(raw.Tables.LogShaped)
		if err != nil {
			return nil, fmt.Errorf("classify: the log-shaped table rule: %w", err)
		}
		c.logShaped = re
	}
	return c, nil
}

// match returns the highest-priority name rule that matches a normalised name.
func (p *compiledPack) match(normalised string) (compiledPattern, bool) {
	for _, pat := range p.Patterns {
		if pat.matches(normalised) {
			return pat, true
		}
	}
	return compiledPattern{}, false
}

// matchColumn returns the highest-priority rule that matches a column, over
// both patterns: and table_patterns: (T-0119): a table-scoped rule is tried
// only where its own tableRe matches normalisedTable, so the same priority
// line orders a table-scoped rule against a name rule exactly as it orders two
// name rules against each other. normalisedTable and normalised are both
// normaliseName's output.
func (p *compiledPack) matchColumn(normalisedTable, normalised string) (compiledPattern, bool) {
	return p.matchColumnAfter(normalisedTable, normalised, "")
}

// matchColumnAfter is matchColumn over the rules that sort after the one named
// after, or over every rule when after is "". decide asks it for the next rule
// down when a rule that needs corroboration did not get it (T-0313), so a
// column that also matches a lower rule -- `content_name` is free_text as well
// as a bare name -- is decided by that rule instead of by nothing.
func (p *compiledPack) matchColumnAfter(normalisedTable, normalised, after string) (compiledPattern, bool) {
	skipping := after != ""
	for _, pat := range p.ColumnPatterns {
		if skipping {
			skipping = pat.Name != after
			continue
		}
		if pat.tableRe != nil && !pat.tableRe.MatchString(normalisedTable) {
			continue
		}
		if pat.matches(normalised) {
			return pat, true
		}
	}
	return compiledPattern{}, false
}

// normaliseName folds an identifier into the form the rule pack is written
// against: lower case, camelCase split on, and a digit run split off the letters
// before it. nasty.sql's public."LegacyCustomer"."EmailAddress" is why: a rule
// written as (^|_)e_?mails?(_|$) has to see "email_address", and "address2" has
// to be seen as "address_2" or the address rule misses the second line of an
// address (testdata/README.md trap 9).
func normaliseName(s string) string {
	runes := []rune(s)
	out := make([]rune, 0, len(runes)+4)
	for i, r := range runes {
		if i > 0 && needsBreak(runes, i) && len(out) > 0 && out[len(out)-1] != '_' {
			out = append(out, '_')
		}
		switch {
		case r >= 'A' && r <= 'Z':
			out = append(out, r-'A'+'a')
		case isNameRune(r):
			out = append(out, r)
		default:
			if len(out) > 0 && out[len(out)-1] != '_' {
				out = append(out, '_')
			}
		}
	}
	return string(out)
}

func isNameRune(r rune) bool {
	return r == '_' || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') ||
		(r >= 'A' && r <= 'Z') || r > 127
}

// needsBreak reports whether a token boundary falls immediately before runes[i]:
// lower-to-upper ("EmailAddress"), letter-to-digit ("address2") and the second
// capital of an acronym run followed by a lower-case letter ("IDToken").
func needsBreak(runes []rune, i int) bool {
	prev, cur := runes[i-1], runes[i]
	upper := func(r rune) bool { return r >= 'A' && r <= 'Z' }
	lower := func(r rune) bool { return r >= 'a' && r <= 'z' }
	digit := func(r rune) bool { return r >= '0' && r <= '9' }
	switch {
	case (lower(prev) || digit(prev)) && upper(cur):
		return true
	case (lower(prev) || upper(prev)) && digit(cur):
		return true
	case digit(prev) && (lower(cur) || upper(cur)):
		return true
	case upper(prev) && upper(cur) && i+1 < len(runes) && lower(runes[i+1]):
		return true
	}
	return false
}
