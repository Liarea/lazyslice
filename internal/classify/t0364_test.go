// SPDX-License-Identifier: Apache-2.0

package classify

import (
	"strings"
	"testing"

	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
)

// T-0364: a lazyslice.yml raise on a natural key -- which is also what
// internal/core makes of --mask and of a committed `mask:` block -- reaches
// every foreign-key child through the same propagation a key the classifier
// masked itself gets, so the parent and the child share one masker and the
// join survives. Before, applyPrior ran after foreignKeys, so the children
// stayed as the first sweep left them: a uuid FK child exempt as a key column,
// the parent's values in clear on the child side (THREAT_MODEL.md T1) and a
// broken join (T8), with no raise that could clear the child's exemption.
func TestAYmlRaiseOnANaturalKeyPropagatesToItsChildren(t *testing.T) {
	t.Parallel()
	accounts := ref.TableRef{Schema: "public", Name: "t364_accounts"}
	links := ref.TableRef{Schema: "public", Name: "t364_links"}
	hits := ref.TableRef{Schema: "public", Name: "t364_hits"}
	optedOut := ref.TableRef{Schema: "public", Name: "t364_exports"}
	codes := ref.TableRef{Schema: "public", Name: "t364_codes"}
	uses := ref.TableRef{Schema: "public", Name: "t364_uses"}

	accountsTable := tt("public", "t364_accounts", []string{"id"},
		tc("id", "bigint"),
		tc("ext_ref", "uuid"),
	)
	accountsTable.Indexes = []pipeline.Index{{
		Name: "t364_accounts_ext_ref_key", Columns: []string{"ext_ref"}, Unique: true, Immediate: true,
	}}
	linksTable := tt("public", "t364_links", []string{"id"},
		tc("id", "bigint"),
		tc("account_ref", "uuid"),
	)
	linksTable.Indexes = []pipeline.Index{{
		Name: "t364_links_account_ref_key", Columns: []string{"account_ref"}, Unique: true, Immediate: true,
	}}
	exportCol := tc("account_ref", "uuid")
	schema := &pipeline.Schema{
		Tables: []pipeline.Table{
			accountsTable,
			linksTable,
			tt("public", "t364_hits", []string{"id"}, tc("id", "bigint"), tc("link_ref", "uuid")),
			tt("public", "t364_exports", []string{"id"}, tc("id", "bigint"), exportCol),
			tt("public", "t364_codes", []string{"code"}, tc("code", "text")),
			tt("public", "t364_uses", []string{"id"}, tc("id", "bigint"), tc("code_ref", "text")),
		},
		// The grandchild edge first, so one sweep in FK order would miss it.
		FKs: []pipeline.ForeignKey{
			fk("t364_hits_link_ref_fkey", hits, []string{"link_ref"}, links, []string{"account_ref"}),
			fk("t364_links_account_ref_fkey", links, []string{"account_ref"}, accounts, []string{"ext_ref"}),
			fk("t364_exports_account_ref_fkey", optedOut, []string{"account_ref"}, accounts, []string{"ext_ref"}),
			fk("t364_uses_code_ref_fkey", uses, []string{"code_ref"}, codes, []string{"code"}),
		},
	}
	parent := col(accounts, "ext_ref")
	children := []ref.ColumnRef{col(links, "account_ref"), col(hits, "link_ref")}
	textParent, textChild := col(codes, "code"), col(uses, "code_ref")
	exported := col(optedOut, "account_ref")

	// Precondition: with no prior, nothing here is masked, so every
	// assertion below is the prior's doing.
	plain, err := New().Classify(schema, nil, nil)
	if err != nil {
		t.Fatalf("Classify with no prior: %v", err)
	}
	for _, c := range append([]ref.ColumnRef{parent, textParent, textChild, exported}, children...) {
		if d := plain.Decisions[c]; d.Masked {
			t.Fatalf("precondition: %s = %+v is masked with no prior, so the prior proves nothing", c, d)
		}
	}
	if d := plain.Decisions[children[0]]; !d.NeverMasked {
		t.Fatalf("precondition: %s = %+v, want the uuid FK child exempt as a key column", children[0], d)
	}

	prior := &pipeline.Config{Columns: map[ref.ColumnRef]pipeline.ColumnConfig{
		parent:     {Category: pipeline.CatOnlineID, Confidence: pipeline.ConfCertain},
		textParent: {Category: pipeline.CatOnlineID, Confidence: pipeline.ConfCertain},
		exported: {Unmask: &pipeline.Unmask{
			Reason: "t364 fixture opt-out", By: "yml", TypeFP: exportCol.Fingerprint,
		}},
	}}
	cls, err := New().Classify(schema, nil, prior)
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}

	for _, pair := range []struct {
		parent   ref.ColumnRef
		children []ref.ColumnRef
	}{
		{parent, children},
		{textParent, []ref.ColumnRef{textChild}},
	} {
		p := cls.Decisions[pair.parent]
		if !p.Masked || p.Category != pipeline.CatOnlineID || p.Masker == "" {
			t.Fatalf("%s = %+v, want the raised key masked as online_id", pair.parent, p)
		}
		for _, c := range pair.children {
			d := cls.Decisions[c]
			if !d.Masked || d.NeverMasked {
				t.Errorf("%s = %+v, want it masked: it holds the values its raised parent is masked for", c, d)
			}
			if d.Category != p.Category || d.Masker != p.Masker {
				t.Errorf("%s = category %q masker %q, want its parent's %q and %q, one masker across the join",
					c, d.Category, d.Masker, p.Category, p.Masker)
			}
			if d.Source != pipeline.ByFKPropagation {
				t.Errorf("%s source = %v, want %v", c, d.Source, pipeline.ByFKPropagation)
			}
			if strings.Contains(d.Reason, "preserved verbatim") {
				t.Errorf("%s reason = %q; the key exemption is lifted, not printed beside the propagation", c, d.Reason)
			}
			if n := strings.Count(d.Reason, "propagated through foreign key"); n != 1 {
				t.Errorf("%s reason = %q, want the propagation named once, got %d", c, d.Reason, n)
			}
			if bad, ok := ParseReason(d.Reason); !ok {
				t.Errorf("%s reason %q does not parse: %q", c, d.Reason, bad)
			}
		}
	}

	// A child with its own honoured opt-out stays copied and keeps saying why.
	if d := cls.Decisions[exported]; d.Masked || d.Source != pipeline.ByYmlUnmask {
		t.Errorf("%s = %+v, want its own unmask: honoured over the parent's raise", exported, d)
	}
}
