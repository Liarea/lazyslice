// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/Liarea/lazyslice/internal/core"
	"github.com/Liarea/lazyslice/internal/event"
	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/tui"
)

// The flag surface is a promise, not an implementation detail: docs/FLAGS.md is
// generated from ARCHITECTURE.md section 8 and CI fails on drift, so a flag that
// quietly disappears would take a documented behaviour with it. These two tests
// hold both ends of that promise on the empty implementation, which is the only
// thing there is to hold in phase 3.

// wantFlags is ARCHITECTURE.md section 8, transcribed. Adding a row here without
// adding it there, or the reverse, is the drift the test exists to catch.
var wantFlags = []string{
	"source", "target", "docker-host", "password-command", "create-target",
	"allow-remote-target", "require-read-only-role", "reconfigure",
	"root", "take", "where", "cap", "depth", "row-budget", "memory-budget",
	"key", "skip-table", "plan",
	"unmask", "strict-schema",
	"residual-probe-cap",
	"secret-file", "require-key",
	"single-connection",
	"show-row-values-in-errors",
	"config", "no-config", "yes",
	"json", "tui", "debug",
}

func TestV1FlagSurfaceIsRegistered(t *testing.T) {
	req := core.NewRequest()
	root := newCommandTree(context.Background(), &req, io.Discard)

	for _, name := range wantFlags {
		if root.PersistentFlags().Lookup(name) == nil {
			t.Errorf("flag --%s is in ARCHITECTURE.md section 8 but is not registered", name)
		}
	}

	// --take has the short form -n; nothing else in the table has one.
	if f := root.PersistentFlags().ShorthandLookup("n"); f == nil || f.Name != "take" {
		t.Error("-n must be the short form of --take")
	}

	// --version is a root flag rather than a persistent one, because it answers
	// before any stage runs.
	if root.Flags().Lookup("version") == nil {
		t.Error("--version is not registered")
	}

	wantCommands := []string{"introspect", "classify", "plan", "verify", "doctor", "version"}
	have := map[string]bool{}
	for _, c := range root.Commands() {
		have[c.Name()] = true
	}
	for _, name := range wantCommands {
		if !have[name] {
			t.Errorf("subcommand %q is in ARCHITECTURE.md section 8 but is not registered", name)
		}
	}
	for name := range have {
		if !slices.Contains(wantCommands, name) {
			t.Errorf("subcommand %q is registered but is not in ARCHITECTURE.md section 8", name)
		}
	}
}

// forbidden is ARCHITECTURE.md section 8's list of flags that must not exist.
// The first pattern is the one that matters: CLAUDE.md forbids a flag that
// disables masking wholesale, and a scaffold is exactly where one would be
// added by accident.
var forbidden = []*regexp.Regexp{
	regexp.MustCompile(`(?i)no-mask|disable-mask|skip-mask|unsafe`),
	regexp.MustCompile(`(?i)^allow-nonempty-target$`),
	regexp.MustCompile(`(?i)^replace$`),
	regexp.MustCompile(`(?i)^allow-ctid$`),
	regexp.MustCompile(`(?i)^rules$`),
}

// forbiddenFlagName reports whether name is a spelling ARCHITECTURE.md
// section 8 forbids, and if so, which rule it matched.
//
// unmask is checked separately from the forbidden regexps above rather than
// folded into one of them: the exact name "unmask" is the legitimate
// per-column --unmask TABLE.COL=REASON opt-out (ARCHITECTURE.md section 8),
// so it must pass, while any other flag name that merely contains "unmask"
// — --unmask-all, --global-unmask, a typo'd --unmask-table — would widen
// that opt-out past the reason-per-column requirement and must fail. This is
// the check T-CI5's review found missing: `make unsafe-flags` used to grep
// cmd/lazyslice's *Var/*VarP call sites in source, which cannot see a
// pointer-returning registration (fs.Bool("unmask-all", ...)), a Var(&v,
// "unmask-all", ...) registration, or a flag name literal that wraps across
// lines. Walking the registered pflag.FlagSet with VisitAll, as
// TestForbiddenFlagsDoNotExist below already does for the other forbidden
// names, sees every spelling and every hidden flag regardless of how it was
// registered.
func checkForbiddenFlagName(name string) (rule string, bad bool) {
	n := strings.ToLower(name)
	for _, re := range forbidden {
		if re.MatchString(n) {
			return re.String(), true
		}
	}
	if n != "unmask" && strings.Contains(n, "unmask") {
		return `unmask (exact) is the only permitted flag name containing "unmask"`, true
	}
	return "", false
}

// walkAllFlags visits every flag registered anywhere in the command tree
// rooted at c: both c's own PersistentFlags() and Flags(), recursively into
// every subcommand at every depth. cobra does not merge a command's
// PersistentFlags() into its Flags() until ParseFlags/LocalFlags runs, so a
// walk that only reads c.Flags() (as root.Commands() traversal without also
// visiting PersistentFlags() would) misses every subcommand's own persistent
// flags on an unparsed tree — exactly where a masking-disabling flag could be
// hidden. Visiting both sets on every node, recursively, has no such blind
// spot regardless of how deep a subcommand is nested.
func walkAllFlags(c *cobra.Command, visit func(name string)) {
	c.PersistentFlags().VisitAll(func(f *pflag.Flag) { visit(f.Name) })
	c.Flags().VisitAll(func(f *pflag.Flag) { visit(f.Name) })
	for _, sub := range c.Commands() {
		walkAllFlags(sub, visit)
	}
}

func TestForbiddenFlagsDoNotExist(t *testing.T) {
	req := core.NewRequest()
	root := newCommandTree(context.Background(), &req, io.Discard)

	check := func(name string) {
		if rule, bad := checkForbiddenFlagName(name); bad {
			t.Errorf("flag --%s matches %q, which ARCHITECTURE.md section 8 forbids", name, rule)
		}
	}

	walkAllFlags(root, check)
}

// TestForbiddenFlagsDoNotExist_SelfTestUnmaskAll is the negative self-test:
// a rail that has never been seen to fail on the case it exists for is not
// proven to catch anything (T-CI5 review, carried over from the grep-based
// `make unsafe-flags` recipe this replaces). It builds the real command tree
// via newCommandTree, registers a fake --unmask-all flag on four different
// flag sets within it — root persistent, root local, a subcommand's own
// Flags(), and a subcommand's own PersistentFlags() — and walks the tree
// with the same walkAllFlags helper TestForbiddenFlagsDoNotExist uses,
// asserting that every one of the four registrations is caught. Exercising
// the production traversal itself (rather than a detached scratch
// pflag.FlagSet standing in for it) is what proves the walk, not just the
// predicate, catches a subcommand's persistent flags — the exact blind spot
// a prior version of this rail had.
//
// Named with the TestForbiddenFlagsDoNotExist prefix so that `go test -run
// TestForbiddenFlagsDoNotExist` (an unanchored regexp match) picks up this
// self-test alongside the positive one; `make unsafe-flags` relies on that.
func TestForbiddenFlagsDoNotExist_SelfTestUnmaskAll(t *testing.T) {
	req := core.NewRequest()
	root := newCommandTree(context.Background(), &req, io.Discard)

	var sub *cobra.Command
	for _, c := range root.Commands() {
		if c.Name() == "introspect" {
			sub = c
			break
		}
	}
	if sub == nil {
		t.Fatal("self-test setup failed: no \"introspect\" subcommand found to plant a fake flag on")
	}

	plant := func(fs *pflag.FlagSet, name string) {
		var v bool
		fs.BoolVar(&v, name, false, "T-0074 self-test: must be rejected")
	}
	plant(root.PersistentFlags(), "unmask-all-root-persistent")
	plant(root.Flags(), "unmask-all-root-local")
	plant(sub.Flags(), "unmask-all-sub-local")
	plant(sub.PersistentFlags(), "unmask-all-sub-persistent")

	// newCommandTree already registers the real --unmask (ARCHITECTURE.md
	// section 8), so it is exercised without being re-registered here.
	var caught []string
	walkAllFlags(root, func(name string) {
		if _, bad := checkForbiddenFlagName(name); bad {
			caught = append(caught, name)
		}
	})

	for _, want := range []string{
		"unmask-all-root-persistent",
		"unmask-all-root-local",
		"unmask-all-sub-local",
		"unmask-all-sub-persistent",
	} {
		if !slices.Contains(caught, want) {
			t.Errorf("self-test failed: a planted --%s registration was not caught by "+
				"walkAllFlags/checkForbiddenFlagName, so this rail is not proven to fail on "+
				"the case it exists for", want)
		}
	}
	if slices.Contains(caught, "unmask") {
		t.Error("self-test failed: the exact, legitimate --unmask flag was rejected; " +
			"only names that merely contain \"unmask\" may be")
	}
}

// reportPanic is the recover path's rendering: CLAUDE.md's "no stack trace
// reaches the user without --debug" has no code behind it for a genuine
// panic unless this gate holds. Without --debug the trace must not appear at
// all — not truncated, not summarised, absent — and the hint must name the
// flag that would have shown it, in the same shape every other internal
// failure in this file uses.
func TestReportPanicShowsTheStackOnlyUnderDebug(t *testing.T) {
	var withoutDebug bytes.Buffer
	code := reportPanic(&withoutDebug, "boom", false)
	if code != ExitInternal {
		t.Errorf("reportPanic without --debug returned %d, want %d", code, ExitInternal)
	}
	out := withoutDebug.String()
	if !strings.Contains(out, "internal error: boom") {
		t.Errorf("reportPanic without --debug = %q, want it to name the panic value", out)
	}
	if !strings.Contains(out, "--debug") {
		t.Errorf("reportPanic without --debug = %q, want a hint naming --debug", out)
	}
	if strings.Contains(out, "goroutine") || strings.Contains(out, ".go:") {
		t.Errorf("reportPanic without --debug = %q, this looks like a stack trace reached the user with no --debug", out)
	}

	var withDebug bytes.Buffer
	code = reportPanic(&withDebug, "boom", true)
	if code != ExitInternal {
		t.Errorf("reportPanic with --debug returned %d, want %d", code, ExitInternal)
	}
	if !strings.Contains(withDebug.String(), "goroutine") {
		t.Errorf("reportPanic with --debug = %q, want a goroutine stack trace", withDebug.String())
	}
}

// run's own recover is exercised end to end by wrapping a command tree that
// panics inside guardedExecute, the same call run makes — this is not a
// hand-written stand-in for run's defer/recover, it is that code, called with
// a root built to fail the way a real bug would: mid-RunE, after flags are
// parsed, so req.Debug already carries what the operator passed.
func TestAPanicInACommandDoesNotCrashTheProcess(t *testing.T) {
	req := core.NewRequest()
	root := &cobra.Command{
		Use:          "boom",
		SilenceUsage: true,
		RunE: func(*cobra.Command, []string) error {
			panic("injected for TestAPanicInACommandDoesNotCrashTheProcess")
		},
	}

	var stderr bytes.Buffer
	got := guardedExecute(context.Background(), root, &stderr, &req)
	if got != ExitInternal {
		t.Errorf("guardedExecute after a panic = %d, want %d\nstderr: %s", got, ExitInternal, stderr.String())
	}
	if !strings.Contains(stderr.String(), "internal error") {
		t.Errorf("stderr after a panic = %q, want it to say \"internal error\"", stderr.String())
	}
	if strings.Contains(stderr.String(), "goroutine") {
		t.Errorf("stderr after a panic with no --debug = %q, want no stack trace", stderr.String())
	}
}

// Exit codes are part of the interface (ADR-005): a wrapper or a CI job branches
// on them, so "the operator mistyped a flag" (2) may never arrive as "lazyslice
// crashed" (1). Cobra raises its own parse errors, which is why this drives the
// whole command tree rather than report alone.
func TestExitCodes(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want int
	}{
		{"unknown flag", []string{"--nope"}, ExitUsage},
		{"unparseable int", []string{"--take", "abc"}, ExitUsage},
		{"unparseable depth", []string{"--depth", "notanint"}, ExitUsage},
		{"too many arguments", []string{"a", "b", "c"}, ExitUsage},
		{"unknown flag on a subcommand", []string{"introspect", "--nope"}, ExitUsage},
		{"unqualified unmask", []string{"--unmask", "notatable=because"}, ExitUsage},
		{"unmask with no reason", []string{"--unmask", "public.users.email"}, ExitUsage},
		{"version", []string{"version"}, ExitOK},
		{"--version", []string{"--version"}, ExitOK},
		{"help", []string{"--help"}, ExitOK},
		{"--take 0", []string{"--take", "0"}, ExitUsage},
		{"--cap 0", []string{"--cap", "0"}, ExitUsage},
		{"--cap TABLE=0", []string{"--cap", "public.payment=0"}, ExitUsage},
		{"--cap with no table", []string{"--cap", "=10"}, ExitUsage},
		{"--depth 0", []string{"--depth", "0"}, ExitUsage},
		{"--memory-budget nonsense", []string{"--memory-budget", "lots"}, ExitUsage},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if got := run(t.Context(), c.args, &stdout, &stderr); got != c.want {
				t.Errorf("run(%q) = %d, want %d\nstderr: %s", c.args, got, c.want, stderr.String())
			}
		})
	}
}

// --unmask is the one safety rail the operator can pull, so a spelling that
// could never match a column must be refused rather than stored: an opt-out
// that silently never applied looks exactly like one that did.
func TestUnmaskShapes(t *testing.T) {
	cases := []struct {
		name    string
		args    []string
		wantErr bool
		want    map[string]string
	}{
		{
			name: "qualified",
			args: []string{"public.users.email=ticket 42"},
			want: map[string]string{"public.users.email": "ticket 42"},
		},
		{
			name: "two columns",
			args: []string{"users.email=a", "users.phone=b"},
			want: map[string]string{"users.email": "a", "users.phone": "b"},
		},
		{name: "unqualified", args: []string{"notatable=because"}, wantErr: true},
		{name: "no reason", args: []string{"users.email"}, wantErr: true},
		{name: "empty reason", args: []string{"users.email="}, wantErr: true},
		{name: "trailing dot", args: []string{"users.=because"}, wantErr: true},
		{name: "leading dot", args: []string{".email=because"}, wantErr: true},
		{name: "the same column twice", args: []string{"users.email=a", "users.email=b"}, wantErr: true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := core.NewRequest()
			err := finish(nil, &req, &rawFlags{unmask: c.args})
			if c.wantErr {
				if err == nil {
					t.Fatalf("finish(--unmask %q) = nil, want a usage error", c.args)
				}
				if !strings.Contains(err.Error(), "usage") {
					t.Errorf("finish(--unmask %q) = %v, want it to wrap errUsage", c.args, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("finish(--unmask %q) = %v, want nil", c.args, err)
			}
			if len(req.Unmask) != len(c.want) {
				t.Fatalf("Unmask = %v, want %v", req.Unmask, c.want)
			}
			for k, v := range c.want {
				if req.Unmask[k] != v {
					t.Errorf("Unmask[%q] = %q, want %q", k, req.Unmask[k], v)
				}
			}
		})
	}
}

// The flag surface is more than a list of names: a default that changes
// silently changes what every run does, and a type that changes turns a
// documented flag into a parse error. ARCHITECTURE.md section 8's table gives
// both for every flag, and this is that column of it (T-0020's log: "main_test
// checks flag names only, not defaults or types").
func TestV1FlagDefaultsAndTypes(t *testing.T) {
	req := core.NewRequest()
	root := newCommandTree(context.Background(), &req, io.Discard)

	cases := []struct {
		name     string
		typeName string
		value    string
	}{
		{"source", "string", ""},
		{"target", "string", ""},
		{"docker-host", "string", ""},
		{"password-command", "string", ""},
		{"create-target", "bool", "false"},
		{"allow-remote-target", "string", ""},
		{"require-read-only-role", "bool", "false"},
		{"reconfigure", "bool", "false"},
		{"root", "string", ""},
		{"take", "int", strconv.Itoa(core.DefaultTake)},
		{"where", "string", ""},
		{"cap", "stringArray", "[]"},
		{"depth", "int", strconv.Itoa(core.DefaultDepth)},
		{"row-budget", "int64", strconv.FormatInt(core.DefaultRowBudget, 10)},
		{"memory-budget", "string", core.DefaultMemoryBudget},
		{"key", "stringArray", "[]"},
		{"skip-table", "stringArray", "[]"},
		{"plan", "bool", "false"},
		{"unmask", "stringArray", "[]"},
		{"strict-schema", "bool", "false"},
		{"residual-probe-cap", "int", strconv.Itoa(core.DefaultResidualProbeCap)},
		{"secret-file", "string", core.DefaultSecretFile},
		{"require-key", "bool", "false"},
		{"single-connection", "bool", "false"},
		{"show-row-values-in-errors", "bool", "false"},
		{"config", "string", core.DefaultConfigPath},
		{"no-config", "bool", "false"},
		{"yes", "bool", "false"},
		{"json", "bool", "false"},
		{"tui", "bool", "false"},
		{"debug", "bool", "false"},
	}

	for _, c := range cases {
		f := root.PersistentFlags().Lookup(c.name)
		if f == nil {
			t.Errorf("--%s is not registered", c.name)
			continue
		}
		if got := f.Value.Type(); got != c.typeName {
			t.Errorf("--%s is a %s, and ARCHITECTURE.md section 8 makes it a %s", c.name, got, c.typeName)
		}
		if f.DefValue != c.value {
			t.Errorf("--%s defaults to %q, and ARCHITECTURE.md section 8 gives it %q",
				c.name, f.DefValue, c.value)
		}
	}

	// The cap, key, skip-table and unmask flags are repeatable and land in
	// core.Request through a parser, so the default they *mean* is the one on
	// the request rather than the one pflag prints.
	if req.Cap != core.DefaultCap {
		t.Errorf("Request.Cap = %d, want the section 8 default %d", req.Cap, core.DefaultCap)
	}
}

// Each subcommand carries the mode core.Run branches on. Without it introspect,
// classify, verify and doctor are the same request (T-0020's log).
func TestSubcommandsCarryTheirMode(t *testing.T) {
	want := map[string]core.Mode{
		"introspect": core.ModeIntrospect,
		"classify":   core.ModeClassify,
		"plan":       core.ModePlan,
		"verify":     core.ModeVerify,
		"doctor":     core.ModeDoctor,
	}

	for name, mode := range want {
		t.Run(name, func(t *testing.T) {
			req := core.NewRequest()
			root := newCommandTree(context.Background(), &req, io.Discard)
			root.SetArgs([]string{name, "--source", "postgres://nobody@127.0.0.1:1/none"})
			root.SetOut(io.Discard)
			root.SetErr(io.Discard)
			// The run fails: there is no database at that address. The mode is
			// set before anything connects, which is what this asserts.
			_ = root.ExecuteContext(context.Background())

			if req.Mode != mode {
				t.Errorf("%s built Mode %v, want %v", name, req.Mode, mode)
			}
			if name == "plan" && !req.PlanOnly {
				t.Error("the plan subcommand must set PlanOnly, like --plan")
			}
		})
	}
}

// A flag the operator typed and a flag holding its own default are different
// facts: lazyslice.yml supplies a value for the second and never for the first
// (ARCHITECTURE.md section 10).
func TestExplicitFlagsAreRecorded(t *testing.T) {
	req := core.NewRequest()
	root := newCommandTree(context.Background(), &req, io.Discard)
	root.SetArgs([]string{"--take", "7", "--source", "postgres://nobody@127.0.0.1:1/none"})
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	_ = root.ExecuteContext(context.Background())

	if !req.Explicit["take"] {
		t.Error("--take was passed and Explicit does not record it, so the yml would override it")
	}
	if req.Explicit["depth"] {
		t.Error("--depth was not passed and Explicit records it, so the yml could not supply a depth")
	}
}

// --cap has two shapes and three ways to be wrong. Each of them is a value that
// would be stored and then match nothing.
func TestCapShapes(t *testing.T) {
	cases := []struct {
		name     string
		args     []string
		wantErr  bool
		wantCap  int
		wantMore map[string]int
	}{
		{name: "a bare count", args: []string{"25"}, wantCap: 25},
		{
			name:     "one table",
			args:     []string{"public.payment=10"},
			wantCap:  core.DefaultCap,
			wantMore: map[string]int{"public.payment": 10},
		},
		{name: "zero", args: []string{"0"}, wantErr: true},
		{name: "zero for a table", args: []string{"public.payment=0"}, wantErr: true},
		{name: "negative", args: []string{"-1"}, wantErr: true},
		{name: "no table", args: []string{"=10"}, wantErr: true},
		{name: "not a number", args: []string{"public.payment=lots"}, wantErr: true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := core.NewRequest()
			err := finish(nil, &req, &rawFlags{caps: c.args})
			if c.wantErr {
				if err == nil {
					t.Fatalf("finish(--cap %q) = nil, want a usage error", c.args)
				}
				if !strings.Contains(err.Error(), "usage") {
					t.Errorf("finish(--cap %q) = %v, want it to wrap errUsage", c.args, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("finish(--cap %q) = %v, want nil", c.args, err)
			}
			if req.Cap != c.wantCap {
				t.Errorf("Cap = %d, want %d", req.Cap, c.wantCap)
			}
			for table, n := range c.wantMore {
				if req.TableCaps[table] != n {
					t.Errorf("TableCaps[%q] = %d, want %d", table, req.TableCaps[table], n)
				}
			}
		})
	}
}

// `lazyslice introspect --json` writes its summary onto the same stdout the
// NDJSON sink is writing events to, so the whole stream has to stay NDJSON:
// ARCHITECTURE.md section 8 calls --json "NDJSON events on stdout", and any
// consumer of it does line-by-line json.Unmarshal. An indented summary put
// fifteen lines into that stream that are not JSON on their own.
func TestIntrospectJSONIsOneObjectPerLine(t *testing.T) {
	var buf bytes.Buffer

	// The events first, as a run would write them, then the summary.
	sink := sinkFor(core.Request{JSON: true}, &buf)
	sink.Send(event.Event{Stage: event.Introspect, Kind: event.Info, Code: "introspect.schema.read"})

	summary := &pipeline.SchemaSummary{
		ServerVersion: 160000,
		Schemas:       []string{"public"},
		Tables:        3,
		Columns:       9,
		ForeignKeys:   2,
		NotRecreated:  map[string]int{"view": 1},
		Fingerprint:   "abc123",
	}
	if err := writeSummary(&buf, summary); err != nil {
		t.Fatalf("writeSummary: %v", err)
	}

	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) < 2 {
		t.Fatalf("the stream has %d line(s), want an event line and a summary line", len(lines))
	}
	for i, line := range lines {
		var obj map[string]json.RawMessage
		if err := json.Unmarshal([]byte(line), &obj); err != nil {
			t.Errorf("line %d of --json output is not a JSON object: %v\n  %s", i+1, err, line)
		}
	}
}

// `--cap TABLE=N` is a per-table cap and must not read as "the operator set the
// global cap". pflag knows only that --cap was Changed, so recording that
// verbatim made a per-table cap silently discard the committed yml's global
// `cap:` and revert every other table to the built-in default
// (internal/core/run.go's planRequest reads Explicit["cap"]).
func TestPerTableCapIsNotAGlobalCap(t *testing.T) {
	perTable := core.NewRequest()
	root := newCommandTree(context.Background(), &perTable, io.Discard)
	root.SetArgs([]string{"--cap", "public.payment=10", "--source", "postgres://nobody@127.0.0.1:1/none"})
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	_ = root.ExecuteContext(context.Background())

	if perTable.Explicit["cap"] {
		t.Error("--cap public.payment=10 recorded an explicit global cap, so the yml's cap: would be ignored")
	}
	if perTable.TableCaps["public.payment"] != 10 {
		t.Errorf("TableCaps = %v, want public.payment=10", perTable.TableCaps)
	}

	// The bare form is the one that means the global cap, and it still does.
	bare := core.NewRequest()
	root = newCommandTree(context.Background(), &bare, io.Discard)
	root.SetArgs([]string{"--cap", "25", "--source", "postgres://nobody@127.0.0.1:1/none"})
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	_ = root.ExecuteContext(context.Background())

	if !bare.Explicit["cap"] || bare.Cap != 25 {
		t.Errorf("--cap 25 gave Cap %d, explicit %v; want 25 and true", bare.Cap, bare.Explicit["cap"])
	}
}

// `lazyslice` with no arguments walks the discovery ladder and, when nothing on
// it answers, stops at exit 3 naming --source (ARCHITECTURE.md sections 8 and
// 9). It is the whole of the first-run wiring in this file: the ladder fills in
// the two endpoints of core.Request and nothing else.
//
// The environment is emptied first because every rung reads one: the
// developer's own $DATABASE_URL or Docker context would otherwise decide what
// this test asserts, and a test whose answer depends on the machine it runs on
// is not a test of the ladder.
func TestNoArgumentsWalksTheLadderAndStopsAtExitThree(t *testing.T) {
	for _, n := range []string{
		"DATABASE_URL", "POSTGRES_URL", "PG_URL", "DB_URL",
		"PGSERVICE", "PGHOST", "PGHOSTADDR", "PGDATABASE", "PGPORT", "PGUSER",
		"PGPASSWORD", "PGPASSFILE", "DOCKER_CONTEXT", "DOCKER_CONFIG",
	} {
		t.Setenv(n, "")
	}
	// A non-local endpoint yields no rung 3 candidate and makes no socket call
	// (ADR-008 section 3), which is what keeps this test off the daemon.
	t.Setenv("DOCKER_HOST", "tcp://staging.example:2375")

	var stdout, stderr bytes.Buffer
	if got := run(t.Context(), []string{"--yes"}, &stdout, &stderr); got != ExitNoSource {
		t.Errorf("run() = %d, want %d\nstderr: %s", got, ExitNoSource, stderr.String())
	}
	if !strings.Contains(stderr.String(), "--source") {
		t.Errorf("the refusal does not name the flag that fixes it: %s", stderr.String())
	}
}

// TestEveryTUIActionHasFlag is ADR-002's first enforcement mechanism and root
// CLAUDE.md's hardest rule for internal/tui: "every TUI action must be
// reachable by a CLI flag first."
//
// It lives here rather than in internal/tui because this is where the flags
// are: internal/tui cannot import a main package, and a test that checked the
// binding table against a second, transcribed list of flag names would pass
// while the real command tree had lost the flag. It walks the registered
// persistent flags of the tree a user gets.
//
// Failing it has one fix and it is not to edit this test: add the flag to
// ARCHITECTURE.md section 8 and to bindFlags, then keep the binding.
func TestEveryTUIActionHasFlag(t *testing.T) {
	req := core.NewRequest()
	root := newCommandTree(context.Background(), &req, io.Discard)

	var actions int
	for _, b := range tui.Bindings() {
		if b.Nav {
			// A navigation binding changes no field of the request, which is
			// why it needs no flag. TestNavigationBindingsNameNoFlag and
			// TestNavigationBuildsNoRequest in internal/tui hold that claim, so
			// this test can take it.
			if b.Flag != "" {
				t.Errorf("the navigation binding %q names the flag --%s", b.Desc(), b.Flag)
			}
			continue
		}
		actions++
		if b.Flag == "" {
			t.Errorf("the action %q (%v) has no CLI flag", b.Desc(), b.Bind.Keys())
			continue
		}
		if root.PersistentFlags().Lookup(b.Flag) == nil {
			t.Errorf("the action %q builds --%s, which no command registers", b.Desc(), b.Flag)
		}
	}
	if actions == 0 {
		t.Fatal("the binding table has no actions, so this test proved nothing")
	}
}

// TestTUIActionFlagsAreInTheV1Surface: a flag registered but absent from
// ARCHITECTURE.md section 8 would pass the test above and still be a capability
// the document does not admit. wantFlags is that section, transcribed.
func TestTUIActionFlagsAreInTheV1Surface(t *testing.T) {
	admitted := map[string]bool{}
	for _, name := range wantFlags {
		admitted[name] = true
	}
	for _, b := range tui.Bindings() {
		if b.Nav {
			continue
		}
		if !admitted[b.Flag] {
			t.Errorf("the action %q builds --%s, which is not in ARCHITECTURE.md section 8",
				b.Desc(), b.Flag)
		}
	}
}

// TestTheLinePrinterIsTheDefault. ADR-002 makes the line printer the default
// because the transcript is the artefact, not because the TUI is expensive, so
// no combination of a TTY and a subcommand may enter the screens on its own.
func TestTheLinePrinterIsTheDefault(t *testing.T) {
	req := core.NewRequest()
	// io.Discard is not an *os.File, so this is also the pipe case: a run whose
	// stdout is redirected never draws an alternate screen over it.
	if wantsTUI(req, io.Discard) {
		t.Error("a run with no --tui entered the TUI")
	}

	req.TUI = true
	if wantsTUI(req, io.Discard) {
		t.Error("--tui entered the TUI with no terminal to draw on")
	}

	req.JSON = true
	if wantsTUI(req, os.Stdout) {
		t.Error("--tui with --json entered the TUI, which would draw over the NDJSON")
	}

	// --yes is the headless flag, and a pty is not evidence that anybody is
	// watching one: ssh -t, a CI job and tmux all allocate one. Opening the
	// screens there would block on a keypress nobody makes, and an unattended
	// run that hangs is worse than one that exits with a code.
	req = core.NewRequest()
	req.TUI = true
	req.Yes = true
	if wantsTUI(req, os.Stdout) {
		t.Error("--tui with --yes entered the TUI, where nothing can answer a keypress")
	}
}

// TestThePreviewPassKeepsTheModeAndItsLadder is the reason previewRequest
// exists. Mode is not only where the pipeline stops: internal/core chooses a
// source and a target off the discovery ladder for ModeRun alone, and prints
// the ladder and stops at exit 3 for every other mode. A preview that called
// itself ModePlan would therefore have made `lazyslice --tui` exit 3 in the
// very directory where plain `lazyslice` finds a container and runs
// (ARCHITECTURE.md section 9).
func TestThePreviewPassKeepsTheModeAndItsLadder(t *testing.T) {
	for _, mode := range []core.Mode{core.ModeRun, core.ModeVerify, core.ModeClassify, core.ModePlan} {
		req := core.NewRequest()
		req.Mode = mode

		preview := previewRequest(req)
		if preview.Mode != mode {
			t.Errorf("the preview pass for %v runs as %v, which changes which ladder it walks",
				mode, preview.Mode)
		}
		if !preview.PlanOnly {
			t.Errorf("the preview pass for %v does not stop at the plan, so it would write the target",
				mode)
		}
		// Nothing else about the run may differ: the screens are shown over the
		// request the operator made.
		want := req
		want.PlanOnly = true
		if !reflect.DeepEqual(preview, want) {
			t.Errorf("the preview pass for %v changed something besides --plan", mode)
		}
	}
}

// TestASecondPassRunsWhateverTheModeWhenTheScreensChangedTheRequest: the plan
// screen exists to change --take, --cap, --depth, --root and --skip-table, so
// `lazyslice plan --tui` has to recompute the plan the operator retuned. The
// footer advertises "enter" as "leave and run", and a mode that answered with
// the plan the operator has just rejected would be that promise broken.
func TestASecondPassRunsWhateverTheModeWhenTheScreensChangedTheRequest(t *testing.T) {
	before := core.NewRequest()
	before.Mode = core.ModePlan
	before.PlanOnly = true

	if requestChanged(before, before) {
		t.Error("a request nobody touched reads as changed, so every run would read the source twice")
	}

	for name, change := range map[string]func(r *core.Request){
		"--take":       func(r *core.Request) { r.Take = 50 },
		"--depth":      func(r *core.Request) { r.Depth = 2 },
		"--root":       func(r *core.Request) { r.Root = "public.customer" },
		"--cap":        func(r *core.Request) { r.TableCaps["public.payment"] = 10 },
		"--skip-table": func(r *core.Request) { r.SkipTables = append(r.SkipTables, "public.payment") },
		"--unmask":     func(r *core.Request) { r.Unmask["public.customer.email"] = "reviewed" },
		"--strict-schema": func(r *core.Request) {
			r.StrictSchema = true
			r.Explicit["strict-schema"] = true
		},
	} {
		after := core.NewRequest()
		after.Mode, after.PlanOnly = before.Mode, before.PlanOnly
		change(&after)
		if !requestChanged(before, after) {
			t.Errorf("%s from the screens does not reach the run, so the operator's change is dropped", name)
		}
	}
}

// TestTheSecondPassDoesNotReprintTheFirstsTranscript: both passes read the
// source, and the stages the screens cannot have changed produce the same
// discover, introspect and classify lines twice. ADR-002 calls the transcript
// the artefact a developer pastes into a compliance ticket, and two copies of a
// two-hundred-column classification is not a better one.
//
// The filter is keyed on the stage, and on the one question per stage that can
// move that stage's output: --unmask for the classification, and nothing for
// the plan, which is always reprinted. The review pin does not license dropping
// it — a step's mode and the virtual edges the slice follows come from the
// second pass's own sampled rows, not from the schema the pin holds still — and
// the transcript ADR-002 calls the compliance artefact has to be the plan of
// the pass that actually copied.
func TestTheSecondPassDoesNotReprintTheFirstsTranscript(t *testing.T) {
	stream := []event.Event{
		{Stage: event.Discover, Kind: event.Info, Code: core.CodeConfigRead},
		// The two lines naming the databases this run wrote are kept whatever
		// changed: the ladder is walked again, and a resolution that differed
		// from the approved one has to be somewhere in scrollback.
		{Stage: event.Discover, Kind: event.Decision, Code: core.CodeSourceChosen},
		{Stage: event.Discover, Kind: event.Decision, Code: core.CodeTargetChosen},
		{Stage: event.Introspect, Kind: event.Info, Code: core.CodeSchemaRead},
		{Stage: event.Classify, Kind: event.Decision, Code: "classify.masked.column"},
		{Stage: event.Plan, Kind: event.Info, Code: core.CodePlanStep},
		// A warning or a refusal is the one thing about the second pass the
		// operator has to read, whatever stage it came from.
		{Stage: event.Plan, Kind: event.Warn, Code: core.CodePlanPolymorphic},
		{Stage: event.Load, Kind: event.Info, Code: core.CodeConfigWritten},
	}

	collect := func(unmaskChanged bool) []event.Event {
		var seen []event.Event
		sink := afterThePlan(
			event.SinkFunc(func(e event.Event) { seen = append(seen, e) }),
			unmaskChanged)
		for _, e := range stream {
			sink.Send(e)
		}
		return seen
	}

	// The discover and introspect lines are the identical work; the plan, the
	// warning and the two decision lines are what the pass that wrote is the
	// only witness to.
	want := []event.Code{
		core.CodeSourceChosen,
		core.CodeTargetChosen,
		core.CodePlanStep,
		core.CodePlanPolymorphic,
		core.CodeConfigWritten,
	}
	var got []event.Code
	for _, e := range collect(false) {
		got = append(got, e.Code)
	}
	if !slices.Equal(got, want) {
		t.Errorf("the second pass printed %v, want %v", got, want)
	}

	// --unmask is the one thing the screens can change that moves the
	// classification, so it is the one case the classification is reprinted.
	got = nil
	for _, e := range collect(true) {
		got = append(got, e.Code)
	}
	if !slices.Contains(got, event.Code("classify.masked.column")) {
		t.Errorf("a second pass after --unmask changed printed %v, without the reclassification", got)
	}
}

// The second pass runs the pinned request, and not the one the screens returned.
//
// pinned is one assignment, and an assignment is exactly what goes missing:
// replacing it with `_ = reviewed` left every test in this package and in
// internal/core green while the review pin ceased to exist. So the behaviour is
// asserted here and the wiring — that runTUI hands *this* to core.Run — is
// asserted structurally below, because runTUI itself needs two Postgres servers
// and a terminal to reach.
func TestTheSecondPassCarriesTheReviewPin(t *testing.T) {
	before := core.NewRequest()
	before.Mode = core.ModeRun
	before.Root = "people"
	reviewed := &core.Reviewed{SchemaFingerprint: "ddl-1", ClassFingerprint: "cls-1", Source: "app@db:5432/app"}

	after := pinned(before, reviewed)
	if after.Reviewed != reviewed {
		t.Fatalf("pinned dropped the review pin (%v): the pass that writes would not be compared "+
			"against what the operator approved", after.Reviewed)
	}
	after.Reviewed = nil
	if !reflect.DeepEqual(after, before) {
		t.Errorf("pinned changed the request the screens returned:\n got %+v\nwant %+v", after, before)
	}
}

// The pin reaches core.Run, and core.Preview is what fills it.
//
// Structural, in the manner of internal/core's own wiring test: runTUI cannot
// be driven without two databases and a terminal, and the two lines that make
// --tui safe are both single expressions that a refactor can drop with nothing
// failing.
func TestRunTUIPinsTheSecondPassToThePreview(t *testing.T) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filepath.Join(".", "main.go"), nil, 0)
	if err != nil {
		t.Fatalf("parse main.go: %v", err)
	}

	var body *ast.BlockStmt
	for _, decl := range f.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.Name == "runTUI" {
			body = fn.Body
		}
	}
	if body == nil {
		t.Fatal("no runTUI in main.go: this test no longer guards anything")
	}

	var calls []string
	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		calls = append(calls, types.ExprString(call.Fun)+"("+argList(call)+")")
		return true
	})

	if !slices.ContainsFunc(calls, func(c string) bool { return strings.HasPrefix(c, "core.Preview(") }) {
		t.Errorf("runTUI does not call core.Preview: nothing produces the pin. Calls: %v", calls)
	}
	want := "core.Run(ctx, pinned(result.Request, reviewed), afterThePlan(render.NewLines(stdout), unmaskChanged))"
	if !slices.Contains(calls, want) {
		t.Errorf("runTUI's call to core.Run is not %s.\nCalls: %v\n"+
			"The pass that writes the target must be handed the pinned request.", want, calls)
	}
}

// argList renders a call's arguments the way they are written.
func argList(call *ast.CallExpr) string {
	args := make([]string, 0, len(call.Args))
	for _, a := range call.Args {
		args = append(args, types.ExprString(a))
	}
	return strings.Join(args, ", ")
}

// TestTUIAsksForNothingIntrospectOrDoctorCanShow: those two produce no
// classification and no plan, so --tui on either falls back to the lines it
// would have printed rather than opening two empty tables.
func TestTUIAsksForNothingIntrospectOrDoctorCanShow(t *testing.T) {
	for _, mode := range []core.Mode{core.ModeIntrospect, core.ModeDoctor} {
		req := core.NewRequest()
		req.TUI = true
		req.Mode = mode
		if wantsTUI(req, os.Stdout) {
			t.Errorf("--tui opened the screens for %v", mode)
		}
	}
}

// TestPreviewIsTheRunForTheModesThatStopThere: `classify`, `plan` and --plan
// all stop where the screens start, so re-running after the operator leaves
// would read the source twice to print the same thing. Every other mode has
// work left, and a second pass is what does it.
func TestPreviewIsTheRunForTheModesThatStopThere(t *testing.T) {
	for mode, want := range map[core.Mode]bool{
		core.ModeRun:        false,
		core.ModeVerify:     false,
		core.ModeClassify:   true,
		core.ModePlan:       true,
		core.ModeIntrospect: false,
		core.ModeDoctor:     false,
	} {
		req := core.NewRequest()
		req.Mode = mode
		if got := previewIsTheRun(req); got != want {
			t.Errorf("previewIsTheRun(%v) = %v, want %v", mode, got, want)
		}
	}

	// --plan on the root command stops after the plan without changing the
	// mode, so the mode alone cannot answer this.
	req := core.NewRequest()
	req.PlanOnly = true
	if !previewIsTheRun(req) {
		t.Error("--plan would run the pipeline a second time after the screens")
	}
}
