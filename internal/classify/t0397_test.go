// SPDX-License-Identifier: Apache-2.0

package classify

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
)

// T-0397 (the 2026-09-25 JSON red team, round 1, entry 24). Every value here
// is invented: the numbers are Ofcom's drama range, the card the documented
// test PAN, and the domain reserved-looking.

// identitiesDoc is the red team's identity_data for row g.
func identitiesDoc(g int) string {
	return fmt.Sprintf(`{"email": "quillon.varda%d@fictionmail.example", "u": "quillon.varda%d", "h": "fictionmail.example", `+
		`"cc": "44", "nsn": "2079460%03d", "n": 442079460%03d, "bin": "411111", "tail": "1111111111", "kind": "standard"}`, g, g, g, g)
}

// The expressions are pg_get_expr's own spelling of the red team's DDL.
var identitiesGenerated = map[string]string{
	"email":  `lower((identity_data ->> 'email'::text))`,
	"joined": `(((identity_data ->> 'u'::text) || '@'::text) || (identity_data ->> 'h'::text))`,
	"dial":   `(('+'::text || (identity_data ->> 'cc'::text)) || (identity_data ->> 'nsn'::text))`,
	"dial_n": `('+'::text || (identity_data ->> 'n'::text))`,
	"pan":    `((identity_data ->> 'bin'::text) || (identity_data ->> 'tail'::text))`,
	"label":  `upper((identity_data ->> 'kind'::text))`,
}

func identitiesSamples(name string, g int) any {
	switch name {
	case "email", "joined":
		return fmt.Sprintf("quillon.varda%d@fictionmail.example", g)
	case "dial", "dial_n":
		return fmt.Sprintf("+442079460%03d", g)
	case "pan":
		return "4111111111111111"
	case "label":
		return "STANDARD"
	}
	return nil
}

// A generated column whose samples validate raises the keys its expression
// reads, in the document it reads them from, to its own category: the leaves
// are masked rather than merely caught by the second net. The Supabase column
// reads a key the name rules already call email, a label over a key of words
// validates as nothing, and a key no expression reads stays none.
func TestAGeneratedColumnWhoseSamplesValidateRaisesTheKeysItReads(t *testing.T) {
	identities := ref.TableRef{Schema: "auth", Name: "t397_identities"}
	cols := []pipeline.Column{tc("id", "bigint"), tc("identity_data", "jsonb")}
	sampler := mapSampler{}
	for g := 1; g <= 30; g++ {
		sampler[col(identities, "id")] = append(sampler[col(identities, "id")], int64(g))
		sampler[col(identities, "identity_data")] = append(sampler[col(identities, "identity_data")], identitiesDoc(g))
	}
	for _, name := range []string{"email", "joined", "dial", "dial_n", "pan", "label"} {
		cols = append(cols, generated(tc(name, "text"), identitiesGenerated[name]))
		for g := 1; g <= 30; g++ {
			sampler[col(identities, name)] = append(sampler[col(identities, name)], identitiesSamples(name, g))
		}
	}
	schema := &pipeline.Schema{Tables: []pipeline.Table{tt("auth", "t397_identities", []string{"id"}, cols...)}}
	cls, err := New().Classify(schema, sampler, nil)
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	got := decision(t, cls, col(identities, "identity_data")).LeafKeys
	want := map[string]pipeline.Category{
		"email": pipeline.CatEmail,
		"u":     pipeline.CatEmail, "h": pipeline.CatEmail,
		"cc": pipeline.CatPhone, "nsn": pipeline.CatPhone, "n": pipeline.CatPhone,
		"bin": pipeline.CatFinancial, "tail": pipeline.CatFinancial,
		"kind": pipeline.CatNone,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("identity_data's LeafKeys = %v\nwant %v", got, want)
	}
	for _, name := range []string{"email", "joined", "dial", "dial_n", "pan"} {
		if d := decision(t, cls, col(identities, name)); d.Masked {
			t.Errorf("%s is masked; a generated column is recomputed by the target, never masked", name)
		}
	}
}

// The raise needs the samples, not the name: the same expressions over
// samples no validator recognises raise nothing, and an ordinary column
// holding the same values raises nothing either, because only a generated
// column's value is computed from the document.
func TestTheRaiseNeedsAGeneratedColumnWhoseSamplesValidate(t *testing.T) {
	identities := ref.TableRef{Schema: "auth", Name: "t397_quiet"}
	for _, c := range []struct {
		name      string
		generated bool
		sample    func(g int) any
	}{
		{"samples that validate as nothing", true, func(int) any { return "ledger matrix" }},
		{"an ordinary column", false, func(g int) any { return fmt.Sprintf("quillon.varda%d@fictionmail.example", g) }},
	} {
		t.Run(c.name, func(t *testing.T) {
			joined := tc("joined", "text")
			if c.generated {
				joined = generated(joined, identitiesGenerated["joined"])
			}
			sampler := mapSampler{}
			for g := 1; g <= 30; g++ {
				sampler[col(identities, "id")] = append(sampler[col(identities, "id")], int64(g))
				sampler[col(identities, "identity_data")] = append(sampler[col(identities, "identity_data")], identitiesDoc(g))
				sampler[col(identities, "joined")] = append(sampler[col(identities, "joined")], c.sample(g))
			}
			schema := &pipeline.Schema{Tables: []pipeline.Table{tt("auth", "t397_quiet", []string{"id"},
				tc("id", "bigint"), tc("identity_data", "jsonb"), joined)}}
			cls, err := New().Classify(schema, sampler, nil)
			if err != nil {
				t.Fatalf("Classify: %v", err)
			}
			keys := decision(t, cls, col(identities, "identity_data")).LeafKeys
			for _, k := range []string{"u", "h"} {
				if keys[k] != pipeline.CatNone {
					t.Errorf("LeafKeys[%s] = %q, want none", k, keys[k])
				}
			}
		})
	}
}

func TestGeneratedReadsTheArrowOperandsAndTheIdentifiers(t *testing.T) {
	for _, c := range []struct {
		expr         string
		idents, keys []string
	}{
		{`lower((identity_data ->> 'email'::text))`, []string{"lower", "identity_data", "text"}, []string{"email"}},
		{`(((d -> 'a'::text) ->> 'b'::text) || '@'::text)`, []string{"d", "text", "text", "text"}, []string{"a", "b"}},
		{`("Doc" ->> 'it''s'::text)`, []string{"Doc", "text"}, []string{"it's"}},
		{`(doc ->> E'k\\x'::text)`, []string{"doc", "text"}, []string{`k\\x`}},
		{`(doc -> 0)`, []string{"doc"}, nil},
		{`('->'::text || name)`, []string{"text", "name"}, nil},
	} {
		idents, keys := generatedReads(c.expr)
		if !reflect.DeepEqual(idents, c.idents) || !reflect.DeepEqual(keys, c.keys) {
			t.Errorf("generatedReads(%s) = %v, %v; want %v, %v", c.expr, idents, keys, c.idents, c.keys)
		}
	}
}
