// SPDX-License-Identifier: Apache-2.0

package render

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/Liarea/lazyslice/internal/event"
)

// The catalogue this package embeds is a copy of internal/event/catalogue.yml,
// because go:embed cannot reach outside a package directory and internal/event
// has no Go file to hold the embed (catalogue.go says so at length). This test
// is the whole of what makes the copy safe: a row added to the real catalogue
// and not copied here fails `make test`.
func TestCatalogueIsTheEventCatalogue(t *testing.T) {
	want, err := os.ReadFile("../event/catalogue.yml")
	if err != nil {
		t.Fatalf("reading the code catalogue: %v", err)
	}
	if !bytes.Equal(want, catalogueYML) {
		t.Errorf("internal/render/catalogue.yml differs from internal/event/catalogue.yml, which is "+
			"the one place a code is added (ARCHITECTURE.md section 12). Copy it:\n"+
			"  cp internal/event/catalogue.yml internal/render/catalogue.yml\n"+
			"(%d bytes here, %d bytes there)", len(catalogueYML), len(want))
	}
}

// Every row has to parse, and every row has to say the four things a renderer
// and docs/ERRORS.md need of it. A row with no message renders as an empty
// line, which is the one failure a reader of the transcript cannot see.
func TestEveryRowIsComplete(t *testing.T) {
	c, err := codes()
	if err != nil {
		t.Fatalf("%v", err)
	}
	if len(c) == 0 {
		t.Fatal("the catalogue is empty, so every rendered line would name a missing code")
	}
	for code, r := range c {
		switch {
		case code == "":
			t.Error("a row has no code")
		case r.Kind == "":
			t.Errorf("%s has no kind", code)
		case r.Message == "":
			t.Errorf("%s has no message", code)
		case r.Kind == "error" && r.Exit == 0:
			t.Errorf("%s is an error and carries no exit code (ARCHITECTURE.md section 7)", code)
		}
	}
}

// A template may only reference an argument key from the event.ArgKey enum.
// That is what keeps a row value out of a rendered line: Args is a fixed-key
// map, and a placeholder outside the enum could only ever be filled by
// something that is not one (THREAT_MODEL.md T4).
func TestTemplatesReferenceOnlyDeclaredArgKeys(t *testing.T) {
	known := map[event.ArgKey]bool{}
	for _, k := range []event.ArgKey{
		event.ArgTable, event.ArgColumn, event.ArgCount, event.ArgFlag,
		event.ArgProvenance, event.ArgRole, event.ArgVersion, event.ArgPath,
		event.ArgSeconds, event.ArgStatement, event.ArgHost, event.ArgDatabase,
		event.ArgStage, event.ArgReason,
	} {
		known[k] = true
	}

	c, err := codes()
	if err != nil {
		t.Fatalf("%v", err)
	}
	for code, r := range c {
		for _, key := range placeholders(r.Message) {
			if !known[key] {
				t.Errorf("%s references {%s}, which is not an event.ArgKey", code, key)
			}
		}
		for _, name := range r.Args {
			if !known[event.ArgKey(name)] {
				t.Errorf("%s declares the arg %q, which is not an event.ArgKey", code, name)
			}
		}
	}
}

// placeholders is every {key} in a template.
func placeholders(message string) []event.ArgKey {
	var out []event.ArgKey
	for {
		open := strings.IndexByte(message, '{')
		if open < 0 {
			return out
		}
		shut := strings.IndexByte(message[open:], '}')
		if shut < 0 {
			return out
		}
		shut += open
		out = append(out, event.ArgKey(message[open+1:shut]))
		message = message[shut+1:]
	}
}
