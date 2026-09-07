// SPDX-License-Identifier: Apache-2.0

//go:build integration

package invariants

import (
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/goccy/go-yaml"
)

// emittedConfig is the part of the emitted lazyslice.yml (§10) this suite
// reads: the root and the take (I5), which columns the run decided to mask,
// which it was told to leave alone, and which of the masked ones it recorded
// as having a small admissible domain.
//
// It is read rather than assumed because §6 item 4 scopes the second net to
// "every column of the loaded target that is not fully masked by a category
// masker: every unmasked, non-opted-out column, and the string leaves of every
// masked JSON column". A masked email lands in example.com and a masked phone
// in the 555-01XX range (§5), and §4 scores a name hit plus ≥80% of validating
// samples as `certain`, so `classify` pointed at a *correct* target flags
// public.customer.email exactly as it flagged the source's. Asserting "the
// classifier flags nothing anywhere in the target" would therefore be a
// permanent false failure; the yml is where the run says which columns it
// masked and which it opted out, and §6 item 4 puts both outside the net.
//
// Everything else in the file is ignored. This is not a parser for §10 and
// must not become one: I5 is the test that says the file is complete, by
// feeding it back in.
type emittedConfig struct {
	// Root and Take are read as scalars because I5 asserts that the file
	// names them. A substring search over the file's text cannot: any
	// `columns:` key beginning `public.customer.` contains "public.customer",
	// and "3" occurs in `depth: 3` and in every hex fingerprint.
	Root string `yaml:"root"`
	Take int    `yaml:"take"`

	Columns map[string]struct {
		Category string         `yaml:"category"`
		Masker   string         `yaml:"masker"`
		Unmask   map[string]any `yaml:"unmask"`
	} `yaml:"columns"`

	// SmallDomain is §10's top-level `small_domain:` list: the masked columns
	// whose substitution §5 says is recoverable by frequency. It is the run's
	// own statement of where §5's small-domain rules applied, and I2 reads it
	// so that a column §5 requires to reuse an admissible value is not
	// reported as unmasked.
	SmallDomain []string `yaml:"small_domain"`
}

// maskedColumn is one entry of the emitted yml's `columns:` map that carries a
// `masker:`.
type maskedColumn struct {
	// key is the column's spelling in the yml, kept verbatim so that every
	// message names the string a reader will find in the file.
	key string

	// masker is the name the yml gives. It is kept because one of them means
	// "no value": `null` (§10's public.staff.picture) is a correct masker
	// whose output is NULL by definition, so it is the one masked column that
	// must not be required to arrive with a value.
	masker string

	// smallDomain is set when the yml lists the column under `small_domain:`.
	smallDomain bool
}

// columnScope is the sets of columns I2 reads out of the emitted yml. They are
// kept apart because they are not interchangeable: one says what the masker
// did, the other says what the second net covers.
//
// Both are keyed by a parsed columnRef rather than by the yml's raw string, so
// that `public."LegacyCustomer"."EmailAddress"` and the catalogue's
// (public, LegacyCustomer, EmailAddress) are the same column. The yml's
// identifier spelling is undecided — §10's examples are all unquoted lower
// case — and a comparison that depended on it would drop nasty's quoted
// columns silently (testdata/README.md traps 9 and 23).
type columnScope struct {
	// masked is the columns a category masker owns. It is the set
	// assertTargetHoldsMaskedRows demands a loaded value in — an opted-out
	// column arriving with a value proves nothing about the masker — the set
	// assertMaskedValuesAreNew compares against the source, and the set the
	// vacuity guard below counts.
	masked map[columnRef]maskedColumn

	// outsideSecondNet is masked plus every column carrying an `unmask:` block.
	// §6 item 4 scopes the net to "every unmasked, non-opted-out column", so an
	// opt-out is outside it by the same words that put a masked column outside
	// it. §10's own example is public.film.description with `masker: free_text`
	// and `unmask: {reason: "product catalogue text, no personal data"}`: that
	// column keeps the source's free text in the target by design, `classify`
	// re-flags it at `possible` (§4), and holding it to "nothing flagged" would
	// fail I2 on a run `verify` passes with exit 0.
	outsideSecondNet map[columnRef]bool
}

// readEmittedConfig parses the emitted yml.
func readEmittedConfig(t *testing.T, path string) emittedConfig {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("the emitted %s, which is where the run says what it did (§10), could not be read: %v",
			path, err)
	}
	var cfg emittedConfig
	if unmarshalErr := yaml.Unmarshal(data, &cfg); unmarshalErr != nil {
		t.Fatalf("parsing the emitted %s: %v", path, unmarshalErr)
	}
	return cfg
}

// readColumnScope reads the emitted yml and splits its columns into the sets
// above.
func readColumnScope(t *testing.T, path string) columnScope {
	t.Helper()

	cfg := readEmittedConfig(t, path)

	scope := columnScope{
		masked:           map[columnRef]maskedColumn{},
		outsideSecondNet: map[columnRef]bool{},
	}
	smallDomain := map[columnRef]bool{}
	var unparseable []string
	for _, name := range cfg.SmallDomain {
		ref, ok := parseColumnRef(name)
		if !ok {
			unparseable = append(unparseable, "small_domain: "+name)
			continue
		}
		smallDomain[ref] = true
	}

	var optedOut []string
	for name, col := range cfg.Columns {
		ref, ok := parseColumnRef(name)
		if !ok {
			unparseable = append(unparseable, "columns: "+name)
			continue
		}
		switch {
		case col.Unmask != nil:
			optedOut = append(optedOut, name)
			scope.outsideSecondNet[ref] = true
		case col.Masker != "":
			scope.masked[ref] = maskedColumn{key: name, masker: col.Masker, smallDomain: smallDomain[ref]}
			scope.outsideSecondNet[ref] = true
		}
	}

	if len(unparseable) > 0 {
		sort.Strings(unparseable)
		t.Fatalf("I2: %d key(s) of the emitted %s are not schema-qualified column names, so this test "+
			"cannot say which column they are about and would report clean by omission:\n  %s",
			len(unparseable), path, strings.Join(unparseable, "\n  "))
	}

	// The guard counts the masker-backed set alone. Widening it to
	// outsideSecondNet would let a run that opted every column out report
	// "nothing was masked" as a pass.
	if len(scope.masked) == 0 {
		sort.Strings(optedOut)
		t.Fatalf("I2: %s records %d column(s), %d of them opted out (%s), and not one masked, so the "+
			"target holds the source's own values and every assertion below is about a copy",
			path, len(cfg.Columns), len(optedOut), strings.Join(optedOut, ", "))
	}
	return scope
}

// maskedColumns is the masked set in a fixed order, so that every message this
// package prints about it is stable.
func (s columnScope) maskedColumns() []columnRef {
	out := make([]columnRef, 0, len(s.masked))
	for ref := range s.masked {
		out = append(out, ref)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].String() < out[j].String() })
	return out
}

// producesNoValue reports that the column's masker is the one whose correct
// output is nothing: `null` sets the column to NULL (§10's
// public.staff.picture, `masker: "null"`). Every other masker in the registry
// returns a value for a value, so a masked column arriving empty is a finding
// rather than a design.
func (s columnScope) producesNoValue(ref columnRef) bool {
	return s.masked[ref].masker == "null"
}

// producesEmptyValue reports the other masker whose correct output holds
// nothing to examine: `derived_text` (ADR-010) masks a tsvector to the *empty*
// tsvector, because the column is derived from text the run may have masked
// and a re-derivation would leak the source's own words. §10's pagila run
// records `public.film.fulltext` with `masker: derived_text`, and the value
// value that arrives is the empty tsvector — which scanCells drops, exactly as
// it drops NULL and the empty string, so the column reads as one the target
// never loaded.
//
// It is kept apart from producesNoValue because the two say different things:
// `null` writes NULL and this writes a value that is empty. Both are read out
// of the run's own `masker:` name, which is the same place the `small_domain:`
// exemption in assertMaskedValuesAreNew is read from — a run cannot escape a
// guard by declaring one of these without also declaring the masker whose
// output every other assertion in this file then holds it to.
func (s columnScope) producesEmptyValue(ref columnRef) bool {
	return s.masked[ref].masker == "derived_text"
}
