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
// reads: which columns the run decided to mask.
//
// It is read rather than assumed because §6 item 4 scopes the second net to
// "every column of the loaded target that is not fully masked by a category
// masker". A masked email lands in example.com and a masked phone in the
// 555-01XX range (§5), and §4 scores a name hit plus ≥80% of validating
// samples as `certain`, so `classify` pointed at a *correct* target flags
// public.customer.email exactly as it flagged the source's. Asserting "the
// classifier flags nothing anywhere in the target" would therefore be a
// permanent false failure; the yml is where the run says which columns it
// masked, and those are the ones §6 item 4 exempts.
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

// maskedColumns returns the "schema.table.column" keys the emitted yml records
// as masked by a category masker.
//
// A column with an `unmask:` block is not one of them: §10 records the opt-out
// with its reason, and the whole point of the second net is that an opted-out
// column is still checked for personal data.
func maskedColumns(t *testing.T, path string) map[string]bool {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("I2: reading the emitted %s, which is where the run says which columns it masked "+
			"(§10) and therefore which ones §6 item 4 exempts from the second net: %v", path, err)
	}

	var cfg emittedConfig
	if unmarshalErr := yaml.Unmarshal(data, &cfg); unmarshalErr != nil {
		t.Fatalf("I2: parsing the emitted %s: %v", path, unmarshalErr)
	}

	masked := map[string]bool{}
	var optedOut []string
	for name, col := range cfg.Columns {
		switch {
		case col.Unmask != nil:
			optedOut = append(optedOut, name)
		case col.Masker != "":
			masked[name] = true
		}
	}
	if len(masked) == 0 {
		sort.Strings(optedOut)
		t.Fatalf("I2: %s records %d column(s), %d of them opted out (%s), and not one masked, so the "+
			"target holds the source's own values and every assertion below is about a copy",
			path, len(cfg.Columns), len(optedOut), strings.Join(optedOut, ", "))
	}
	return masked
}
