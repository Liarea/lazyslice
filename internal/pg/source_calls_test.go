// SPDX-License-Identifier: Apache-2.0

package pg

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

// TestSourceNeverCopiesOrBatches is the third of the three independent defences
// of the read side (ARCHITECTURE.md §2 "Source"). The READ ONLY transaction and
// the shape allowlist are runtime defences; this one reads the package.
//
// It is scoped to the methods that hold a source connection — Source and the
// reader it hands out — because the same package implements the target's Writer,
// where CopyFrom is the whole point. A helper called from a source method and
// declared elsewhere would slip past it; that is the limit of a check this
// cheap, and it is why it is the third defence and not the first.
func TestSourceNeverCopiesOrBatches(t *testing.T) {
	const forbidden = "CopyFrom, SendBatch or Prepare"
	banned := map[string]bool{"CopyFrom": true, "SendBatch": true, "Prepare": true}
	sourceReceivers := map[string]bool{"Source": true, "reader": true}

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("reading internal/pg: %v", err)
	}

	fset := token.NewFileSet()
	checked := 0
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatalf("parsing %s: %v", name, err)
		}
		checked++
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv == nil || len(fn.Recv.List) != 1 {
				continue
			}
			if !sourceReceivers[receiverName(fn.Recv.List[0].Type)] {
				continue
			}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok || !banned[sel.Sel.Name] {
					return true
				}
				t.Errorf("%s in %s calls %s on a source connection; the source uses Query and Exec only (%s)",
					fn.Name.Name, name, sel.Sel.Name, forbidden)
				return true
			})
		}
	}
	if checked == 0 {
		t.Fatal("no non-test file was read, so this test proved nothing")
	}
}

func receiverName(e ast.Expr) string {
	if star, ok := e.(*ast.StarExpr); ok {
		e = star.X
	}
	if id, ok := e.(*ast.Ident); ok {
		return id.Name
	}
	return ""
}
