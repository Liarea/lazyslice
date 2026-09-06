// SPDX-License-Identifier: Apache-2.0

package classify

import (
	_ "embed"
	"fmt"
	"regexp"
	"sort"
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
	Version    string         `yaml:"version"`
	Categories []categoryRule `yaml:"categories"`
	Patterns   []patternRule  `yaml:"patterns"`
	Tables     tableRules     `yaml:"tables"`
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
}

type tableRules struct {
	LogShaped string `yaml:"log_shaped"`
}

// compiledPattern is one name rule with its regexp built.
type compiledPattern struct {
	Name     string
	Category pipeline.Category
	Priority int
	re       *regexp.Regexp
}

// compiledPack is the rule pack in the form the scorer uses.
type compiledPack struct {
	Version  string
	Patterns []compiledPattern
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
func (p *compiledPack) logShapedTable(name string) bool {
	return p.logShaped != nil && p.logShaped.MatchString(normaliseName(name))
}

// pack is the compiled rule pack. It is loaded once; a malformed pack is a
// build-time mistake, so the error is returned from Classify rather than
// panicking at init, which would take an unrelated subcommand down with it.
var pack = sync.OnceValues(loadPack)

func loadPack() (*compiledPack, error) {
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
		c.Patterns = append(c.Patterns, compiledPattern{
			Name:     pr.Name,
			Category: cat,
			Priority: pr.Priority,
			re:       re,
		})
	}
	if len(c.Patterns) == 0 {
		return nil, fmt.Errorf("classify: the embedded rule pack has no name patterns")
	}
	// Highest priority first, then by name, so that two rules matching one
	// column resolve the same way on every run and in every build.
	sort.SliceStable(c.Patterns, func(i, j int) bool {
		if c.Patterns[i].Priority != c.Patterns[j].Priority {
			return c.Patterns[i].Priority > c.Patterns[j].Priority
		}
		return c.Patterns[i].Name < c.Patterns[j].Name
	})
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
		if pat.re.MatchString(normalised) {
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
