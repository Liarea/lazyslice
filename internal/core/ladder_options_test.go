// SPDX-License-Identifier: Apache-2.0

package core

import (
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/Liarea/lazyslice/internal/discover"
	"github.com/Liarea/lazyslice/internal/dsn"
	"github.com/Liarea/lazyslice/internal/event"
	"github.com/Liarea/lazyslice/internal/pg"
	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
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
// field must appear in that composite literal *with the request's own value*.
//
// The key alone is not enough, which is T-0071's own post-mortem: a literal
// that wrote `Yes: false` or `Yes: req.Yes` on some other request would carry
// the key and still be the bug this exists to catch. So the value expression is
// compared as text against "r.req.<Name>". And the literal is counted, because
// a second discover.Options built somewhere else in run.go would be a ladder
// call this test never looked at while it went on passing over the first.
func TestEveryLadderOptionTheRequestCarriesIsCopied(t *testing.T) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filepath.Join(".", "run.go"), nil, 0)
	if err != nil {
		t.Fatalf("parse run.go: %v", err)
	}

	values := map[string]string{}
	literals := 0
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
		literals++
		for _, elt := range lit.Elts {
			kv, ok := elt.(*ast.KeyValueExpr)
			if !ok {
				continue
			}
			if key, ok := kv.Key.(*ast.Ident); ok {
				values[key.Name] = types.ExprString(kv.Value)
			}
		}
		return true
	})
	switch literals {
	case 1:
	case 0:
		t.Fatal("no discover.Options literal in run.go: this test no longer guards anything")
	default:
		t.Fatalf("run.go builds %d discover.Options literals; this test reads them as one, "+
			"so a flag dropped from any but the last would pass unseen", literals)
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
		got, ok := values[name]
		if !ok {
			t.Errorf("discover.Options.%s is a Request field the ladder reads and "+
				"resolveEndpoints does not set: the flag does nothing", name)
			continue
		}
		if want := "r.req." + name; got != want {
			t.Errorf("discover.Options.%s is set to %s, want %s: the ladder reads this field "+
				"and only the run's own request carries the operator's answer", name, got, want)
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
}

// The unreachable-target refusal's {reason} is redacted.
//
// {host} is a dsn.Ref and cannot carry a password, because Ref does not hold
// one — asserting on the password in the DSN above proved nothing (T-0071
// review). {reason} is the half that can leak: it is whatever error the dial
// returned, and a *pgconn.PgError quotes the conflicting row in Detail, Where
// and Hint. unreachableTarget renders it through pg.RenderAnyError with values
// off, which drops all three (THREAT_MODEL.md T4), and this is what says so.
func TestTheUnreachableTargetRefusalDropsAPgErrorsRowFields(t *testing.T) {
	_, targetRef, err := dsn.Parse("postgres://app@127.0.0.1:5433/appdb")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	pgErr := &pgconn.PgError{
		Severity: "ERROR",
		Code:     "23505",
		Message:  "duplicate key value violates unique constraint \"people_email_key\"",
		Detail:   "Key (email)=(alice@example.com) already exists.",
		Where:    "COPY people, line 41: \"alice@example.com\"",
		Hint:     "try alice@example.com",
	}

	s := unreachableTarget(targetRef, fmt.Errorf("pg: gate: connecting to the target: %w", pgErr), "the target would not open")

	var buf bytes.Buffer
	render.NewLines(&buf).Send(event.Event{Kind: event.Error, Code: s.Code, Args: s.Args})
	line := buf.String()

	if strings.Contains(line, "alice@example.com") {
		t.Errorf("rendered %q, which carries the row value the server quoted in Detail, Where and Hint", line)
	}
	if !strings.Contains(line, "23505") {
		t.Errorf("rendered %q, want the SQLSTATE, which is an identifier and the actionable half", line)
	}
	if strings.ContainsAny(line, "{}") {
		t.Errorf("rendered %q, want no unfilled placeholder", line)
	}
}

// A password the target refused is its own refusal, not "did not respond"
// with the driver's line as the whole reason (T-0327, dogfood session 2: a
// raw 'password authentication failed for user "postgres" (SQLSTATE 28P01)'
// was everything the operator was told).
func TestARefusedPasswordSaysWhereAPasswordComesFrom(t *testing.T) {
	_, targetRef, err := dsn.Parse("postgres://postgres@127.0.0.1:5433/postgres")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	pgErr := &pgconn.PgError{
		Severity: "FATAL", Code: "28P01",
		Message: `password authentication failed for user "postgres"`,
	}
	s := unreachableTarget(targetRef, fmt.Errorf("pg: gate: connecting to the target: %w", pgErr), "the target would not open")
	if s.Code != CodeTargetAuth || s.Exit != exitTarget {
		t.Fatalf("stop = %s/exit %d, want %s/exit %d", s.Code, s.Exit, CodeTargetAuth, exitTarget)
	}

	var buf bytes.Buffer
	render.NewLines(&buf).Send(event.Event{Kind: event.Error, Code: s.Code, Args: s.Args})
	line := buf.String()
	if strings.ContainsAny(line, "{}") {
		t.Errorf("rendered %q, want no unfilled placeholder", line)
	}
	for _, want := range []string{"127.0.0.1:5433/postgres", "postgres", "--password-command", "PGPASSWORD", "28P01"} {
		if !strings.Contains(line, want) {
			t.Errorf("rendered %q, want it to name %q", line, want)
		}
	}
	if strings.Contains(line, "did not respond") {
		t.Errorf("rendered %q: the server answered, so it did respond", line)
	}
}

// The not-empty refusal fills its template and, for lazyslice's own copy of a
// different source, says so (T-0327: it rendered a literal "{table}" and the
// word "not empty" over a target lazyslice itself had loaded).
func TestTheNotEmptyRefusalRendersWhole(t *testing.T) {
	_, targetRef, err := dsn.Parse("postgres://postgres@127.0.0.1:5433/postgres")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	occupied := map[ref.TableRef]int64{
		{Schema: "public", Name: "film"}:  pg.RowsNotCounted,
		{Schema: "public", Name: "actor"}: pg.RowsNotCounted,
	}
	for _, tc := range []struct {
		name string
		e    pipeline.Eligibility
		want string
	}{
		{"another source's copy", pipeline.Eligibility{Reason: pg.CodeNotEmpty, RowCounts: occupied, Marked: true}, "a copy of another source"},
		{"unmarked rows", pipeline.Eligibility{Reason: pg.CodeNotEmpty, RowCounts: occupied}, "an empty database"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := gateRefusal(tc.e, targetRef, "postgres")
			if s.Code != pg.CodeNotEmpty || s.Exit != exitTarget {
				t.Fatalf("stop = %s/exit %d, want %s/exit %d", s.Code, s.Exit, pg.CodeNotEmpty, exitTarget)
			}
			var buf bytes.Buffer
			render.NewLines(&buf).Send(event.Event{Kind: event.Error, Code: s.Code, Args: s.Args})
			line := buf.String()
			if strings.ContainsAny(line, "{}") {
				t.Errorf("rendered %q, want no unfilled placeholder", line)
			}
			for _, want := range []string{"public.actor, public.film", tc.want, "--target"} {
				if !strings.Contains(line, want) {
					t.Errorf("rendered %q, want it to say %q", line, want)
				}
			}
		})
	}
}

// openTarget with no target refuses under its own code and renders whole.
//
// It is reached with an empty --target and no ladder pick, and it used to be
// miscoded as pg.CodeUnreachable — a dial failure for a flag that was never
// given. The Stop carries {flag} because the catalogue row is "no target: pass
// {flag} postgres://...", and a Stop built by wrap alone sets no Args and
// reaches the operator with the placeholder raw (codes.go, CodeTargetUnset).
func TestOpenTargetWithNoTargetNamesTheFlag(t *testing.T) {
	r := &run{req: normalise(Request{Mode: ModeRun}), sink: event.Discard}

	var stop *Stop
	if err := r.openTarget(t.Context()); !errors.As(err, &stop) {
		t.Fatalf("openTarget = %v, want a Stop", err)
	}
	if stop.Code != CodeTargetUnset || stop.Exit != exitTarget {
		t.Fatalf("stop = %s/exit %d, want %s/exit %d",
			stop.Code, stop.Exit, CodeTargetUnset, exitTarget)
	}

	var buf bytes.Buffer
	render.NewLines(&buf).Send(event.Event{Kind: event.Error, Code: stop.Code, Args: stop.Args})
	line := buf.String()
	if strings.ContainsAny(line, "{}") {
		t.Errorf("rendered %q, want no unfilled placeholder", line)
	}
	if !strings.Contains(line, "--target") {
		t.Errorf("rendered %q, want the flag the operator has to pass", line)
	}
}
