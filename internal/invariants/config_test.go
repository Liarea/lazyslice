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
// reads: which columns the run decided to mask, and which it was told to leave
// alone.
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
	Columns map[string]struct {
		Category string         `yaml:"category"`
		Masker   string         `yaml:"masker"`
		Unmask   map[string]any `yaml:"unmask"`
	} `yaml:"columns"`
}

// columnScope is the two sets of "schema.table.column" keys I2 reads out of the
// emitted yml. They are kept apart because they are not interchangeable: one
// says what the masker did, the other says what the second net covers.
type columnScope struct {
	// masked is the columns a category masker owns. It is the set
	// assertTargetHoldsMaskedRows demands a loaded value in — an opted-out
	// column arriving with a value proves nothing about the masker — and it is
	// the set the vacuity guard below counts.
	masked map[string]bool

	// outsideSecondNet is masked plus every column carrying an `unmask:` block.
	// §6 item 4 scopes the net to "every unmasked, non-opted-out column", so an
	// opt-out is outside it by the same words that put a masked column outside
	// it. §10's own example is public.film.description with `masker: free_text`
	// and `unmask: {reason: "product catalogue text, no personal data"}`: that
	// column keeps the source's free text in the target by design, `classify`
	// re-flags it at `possible` (§4), and holding it to "nothing flagged" would
	// fail I2 on a run `verify` passes with exit 0.
	outsideSecondNet map[string]bool
}

// readColumnScope reads the emitted yml and splits its columns into the two
// sets above.
func readColumnScope(t *testing.T, path string) columnScope {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("I2: reading the emitted %s, which is where the run says which columns it masked "+
			"and which it opted out (§10) and therefore which ones §6 item 4 exempts from the second "+
			"net: %v", path, err)
	}

	var cfg emittedConfig
	if unmarshalErr := yaml.Unmarshal(data, &cfg); unmarshalErr != nil {
		t.Fatalf("I2: parsing the emitted %s: %v", path, unmarshalErr)
	}

	scope := columnScope{masked: map[string]bool{}, outsideSecondNet: map[string]bool{}}
	var optedOut []string
	for name, col := range cfg.Columns {
		switch {
		case col.Unmask != nil:
			optedOut = append(optedOut, name)
			scope.outsideSecondNet[name] = true
		case col.Masker != "":
			scope.masked[name] = true
			scope.outsideSecondNet[name] = true
		}
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
