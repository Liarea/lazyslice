// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

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
	regexp.MustCompile(`no-mask|disable-mask|skip-mask|unsafe`),
	regexp.MustCompile(`^allow-nonempty-target$`),
	regexp.MustCompile(`^replace$`),
	regexp.MustCompile(`^allow-ctid$`),
	regexp.MustCompile(`^rules$`),
}

func TestForbiddenFlagsDoNotExist(t *testing.T) {
	req := core.NewRequest()
	root := newCommandTree(context.Background(), &req, io.Discard)

	check := func(name string) {
		for _, re := range forbidden {
			if re.MatchString(name) {
				t.Errorf("flag --%s matches %q, which ARCHITECTURE.md section 8 forbids", name, re)
			}
		}
	}

	root.PersistentFlags().VisitAll(func(f *pflag.Flag) { check(f.Name) })
	root.Flags().VisitAll(func(f *pflag.Flag) { check(f.Name) })
	for _, c := range root.Commands() {
		c.Flags().VisitAll(func(f *pflag.Flag) { check(f.Name) })
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
// The filter is keyed on the stage and not on whether the request changed,
// because the run --tui exists for is the one where the operator retuned --take
// and the plan is the only thing that differs.
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
		sink := afterThePlan(event.SinkFunc(func(e event.Event) { seen = append(seen, e) }), unmaskChanged)
		for _, e := range stream {
			sink.Send(e)
		}
		return seen
	}

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
