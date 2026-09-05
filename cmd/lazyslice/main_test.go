package main

import (
	"bytes"
	"context"
	"io"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/spf13/pflag"

	"github.com/Liarea/lazyslice/internal/core"
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
		// The scaffold itself: every stage is a no-op, so a run that parses
		// cleanly still fails, and it fails as an internal error rather than a
		// usage one.
		{"a clean run in the scaffold", []string{"--unmask", "public.users.email=ticket 42"}, ExitInternal},
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
