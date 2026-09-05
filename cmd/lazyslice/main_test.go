package main

import (
	"context"
	"os"
	"regexp"
	"slices"
	"testing"

	"github.com/spf13/pflag"
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
	root := newCommandTree(context.Background(), os.Stdout)

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
	root := newCommandTree(context.Background(), os.Stdout)

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
