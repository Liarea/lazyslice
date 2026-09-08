// SPDX-License-Identifier: Apache-2.0

package core

import (
	"bytes"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Liarea/lazyslice/internal/discover"
	"github.com/Liarea/lazyslice/internal/dsn"
	"github.com/Liarea/lazyslice/internal/event"
	"github.com/Liarea/lazyslice/internal/pg"
	"github.com/Liarea/lazyslice/internal/render"
)

// Every flag the ladder reads reaches the ladder.
//
// resolveEndpoints is the one place discover.Options is built, so an Options
// field that internal/discover reads and internal/core never sets is a flag
// that silently does nothing — which is what happened to --yes: ADR-008 §7
// makes it and "no controlling terminal" one path, and with Yes left at false
// a run under an allocated TTY (docker run -t, script(1), tmux) opened
// /dev/tty and blocked in the prompt with no timeout instead of taking Q1's
// headless refusal. A unit test cannot reach that state without a Docker
// daemon and a terminal, so the copy itself is what is pinned here.
//
// The rule is structural and covers the next field as well as this one: every
// exported discover.Options field whose name is also an exported core.Request
// field must appear as a key in that composite literal.
func TestEveryLadderOptionTheRequestCarriesIsCopied(t *testing.T) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filepath.Join(".", "run.go"), nil, 0)
	if err != nil {
		t.Fatalf("parse run.go: %v", err)
	}

	set := map[string]bool{}
	found := false
	ast.Inspect(f, func(n ast.Node) bool {
		lit, ok := n.(*ast.CompositeLit)
		if !ok {
			return true
		}
		sel, ok := lit.Type.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "Options" {
			return true
		}
		if pkg, ok := sel.X.(*ast.Ident); !ok || pkg.Name != "discover" {
			return true
		}
		found = true
		for _, elt := range lit.Elts {
			kv, ok := elt.(*ast.KeyValueExpr)
			if !ok {
				continue
			}
			if key, ok := kv.Key.(*ast.Ident); ok {
				set[key.Name] = true
			}
		}
		return true
	})
	if !found {
		t.Fatal("no discover.Options literal in run.go: this test no longer guards anything")
	}

	opts, req := reflect.TypeOf(discover.Options{}), reflect.TypeOf(Request{})
	for i := range opts.NumField() {
		name := opts.Field(i).Name
		if !opts.Field(i).IsExported() {
			continue
		}
		if _, ok := req.FieldByName(name); !ok {
			continue
		}
		if !set[name] {
			t.Errorf("discover.Options.%s is a Request field the ladder reads and "+
				"resolveEndpoints does not set: the flag does nothing", name)
		}
	}
}

// The unreachable-target refusal names the endpoint and the connect error.
//
// The catalogue row is "target {host} did not respond: {reason}", so a Stop
// built with no Args reaches the operator as "✗ target {host} did not respond:
// {reason}" — the placeholders raw. wrap sets no Args, which is why
// unreachableTarget exists.
func TestTheUnreachableTargetRefusalRendersWithNoPlaceholders(t *testing.T) {
	_, targetRef, err := dsn.Parse("postgres://app:hunter2@127.0.0.1:5433/appdb")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	// A real pgx dial failure is multi-line: the gate's own preamble on line
	// 1, then the driver's cause on the lines after it. A single-line error
	// here would pass even if unreachableTarget regressed to keeping only
	// the first line and dropping the cause (T-0071 review), so this pins
	// the multi-line shape a real dial actually produces.
	dialErr := errors.New("pg: gate: connecting to the target: failed to connect to `user=app database=appdb`:\n\t127.0.0.1:59999 (127.0.0.1): dial error: dial tcp 127.0.0.1:59999: connect: connection refused")
	s := unreachableTarget(targetRef, dialErr, "the target would not open")
	if s.Code != pg.CodeUnreachable || s.Exit != exitTarget {
		t.Fatalf("stop = %s/exit %d, want %s/exit %d", s.Code, s.Exit, pg.CodeUnreachable, exitTarget)
	}

	var buf bytes.Buffer
	render.NewLines(&buf).Send(event.Event{Kind: event.Error, Code: s.Code, Args: s.Args})
	line := buf.String()

	if strings.ContainsAny(line, "{}") {
		t.Errorf("rendered %q, want no unfilled placeholder", line)
	}
	if !strings.Contains(line, "127.0.0.1:5433/appdb") {
		t.Errorf("rendered %q, want the endpoint named", line)
	}
	if !strings.Contains(line, "connection refused") {
		t.Errorf("rendered %q, want the connect error named", line)
	}
	if strings.HasSuffix(strings.TrimRight(line, "\n"), ":") {
		t.Errorf("rendered %q, ends in a dangling colon that promises a reason and delivers none", line)
	}
	if strings.Contains(line, "hunter2") {
		t.Errorf("rendered %q, which carries the password", line)
	}
}
