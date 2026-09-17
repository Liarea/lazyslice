// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

// flagRow is one registered pflag, as --help prints it, split into its parts.
type flagRow struct {
	short string // "n"; empty when the flag has no short form
	long  string // "take"
	typ   string // "int"; empty for a bool flag
	def   string // "500"; empty when --help printed no default
	help  string // the description, with any pflag-generated default stripped
}

// flagGroup is one --help section: a title ("discover", "other") and its rows
// in the order --help prints them.
type flagGroup struct {
	title string
	rows  []flagRow
}

// flagLineRE matches one flag line of pflag's FlagUsages() output, for
// example:
//
//	-n, --take int                 Root rows, ... (default 500)
//	    --debug                    Stack traces and the statement trace on error
//
// The optional short-flag group and the optional type-word group are what
// make one pattern read both a bool flag (no type word) and a flag with a
// short alias and a type.
var flagLineRE = regexp.MustCompile(`^\s*(?:-(\w), )?--([\w-]+)(?:\s+([A-Za-z][\w.]*))?\s{2,}(.*)$`)

// groupHeaderRE matches a --help section title on its own line: "discover:",
// "load and verify:", "other:". "Usage:" and "Commands:" are headers too, and
// parseHelpGroups skips them because neither carries flag lines. The title
// charset is deliberately wider than plain letters and spaces — digits and a
// handful of punctuation marks a real cobra group title can carry (an
// ampersand joining two stage names, a version number) — because a title
// this misses does not vanish: bareHeaderRE below still recognises it as
// *some* header and resets `current` to nil, but its flags are then dropped
// rather than merely misattributed to whatever group was open before it.
var groupHeaderRE = regexp.MustCompile(`^([A-Za-z][A-Za-z0-9 &_.'-]*):$`)

// bareHeaderRE matches any unindented line ending in a colon — the shape
// every real --help section title has, whether or not groupHeaderRE's
// narrower charset recognises it. parseHelpGroups uses it as a fallback: a
// line matching this but not groupHeaderRE resets `current` to nil instead of
// leaving whatever group was previously open still accepting flag lines, so
// an unrecognised header title turns into the "flags vanished" case
// checkCompleteness already catches, rather than silently misattributing
// those flags to the wrong group in docs/FLAGS.md. Flag lines and their
// pflag-wrapped continuation lines are always indented, so this never matches
// one of those.
var bareHeaderRE = regexp.MustCompile(`^\S.*:$`)

// pflagDefaultRE strips pflag's own "(default X)" suffix from a description,
// when it is one: at the very end of the line, with no colon directly after
// the word "default". A handful of flags in cmd/lazyslice write a conceptual
// default into their own help text instead — "(default: computed from the
// foreign-key graph)", "(default: DOCKER_HOST, ...)" — and those must survive
// intact; the colon immediately after "default" is what tells the two apart,
// and cmd/lazyslice/main.go has no third spelling as of this writing. The
// value itself is matched permissively (anything but a leading colon or an
// unbalanced paren) rather than enumerated by pflag type, because pflag
// renders a default differently per Value implementation — a quoted string,
// a bare number, a bool, a bracketed slice, but also a duration ("30s"), an
// IP, or any other Stringer — and a value-shape allowlist silently stops
// matching (and stops stripping the suffix out of the Description) the first
// time a flag of a type not on the list is added.
var pflagDefaultRE = regexp.MustCompile(`^(.*?)\s*\(default ([^:)][^)]*)\)$`)

// flagTokenRE matches the start of any --help line that names a flag — short
// form optional — independently of flagLineRE's stricter shape. It is the
// completeness check parseHelpGroups runs after parsing: `make docs-check`
// diffs generator output against generator output, so a parse bug that drops
// a flag produces the same wrong docs/FLAGS.md on both sides of that diff and
// is otherwise invisible. Two concrete pflag shapes flagLineRE cannot match,
// that this catches instead of silently omitting: a flag carrying
// NoOptDefVal (pflag renders `--name[="x"]`, with no `\s{2,}` separator before
// it) and a flag whose group header this file's groupHeaderRE does not
// recognise (an ampersand, digit or anything but letters and spaces in the
// title), which leaves `current` nil and the flag line never even reaches
// flagLineRE.
var flagTokenRE = regexp.MustCompile(`^\s*(?:-\w, )?--[A-Za-z][\w-]*`)

// generateFlags renders docs/FLAGS.md from `go run ./cmd/lazyslice --help`,
// grouped exactly as --help groups it: cmd/lazyslice's own flagGroup titles,
// plus cobra's "other" group for -h/--help and --version.
//
// It runs the real binary rather than importing cmd/lazyslice, which is
// package main and cannot be imported by another program; the trade a
// renderer of --help's own text makes is that the format is exactly what an
// operator already reads at the terminal, which is the one thing docs/FLAGS.md
// must never drift from.
func generateFlags(repoRoot string) (string, error) {
	out, err := runHelp(repoRoot)
	if err != nil {
		return "", err
	}
	groups, err := parseHelpGroups(out)
	if err != nil {
		return "", err
	}

	var b strings.Builder
	b.WriteString(generatedHeader("Flags", "`go run ./cmd/lazyslice --help`"))
	b.WriteString("One row per flag registered on the command tree, grouped as `--help` groups " +
		"them.\n\n")
	for _, g := range groups {
		if len(g.rows) == 0 {
			continue
		}
		fmt.Fprintf(&b, "## %s\n\n", g.title)
		b.WriteString("| Flag | Type | Default | Description |\n")
		b.WriteString("|---|---|---|---|\n")
		for _, r := range g.rows {
			name := "`--" + r.long + "`"
			if r.short != "" {
				name = "`-" + r.short + ", --" + r.long + "`"
			}
			typ := r.typ
			if typ == "" {
				typ = "bool"
			}
			def := r.def
			if def == "" {
				def = "-"
			}
			fmt.Fprintf(&b, "| %s | %s | %s | %s |\n", name, typ, def, mdEscapeText(r.help))
		}
		b.WriteString("\n")
	}
	return b.String(), nil
}

// runHelp builds and runs cmd/lazyslice with --help and returns its stdout.
// It is `go run` rather than a build-then-exec pair because docgen runs once
// per `make docs`, and go run's own build cache makes a second build no
// slower than a hand-managed one would be.
func runHelp(repoRoot string) (string, error) {
	cmd := exec.Command("go", "run", "./cmd/lazyslice", "--help")
	cmd.Dir = repoRoot
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("go run ./cmd/lazyslice --help: %w\n%s", err, stderr.String())
	}
	return stdout.String(), nil
}

// parseHelpGroups splits --help's output into its named sections and parses
// the flag lines out of each, skipping "Usage:" and "Commands:", which are
// headers with no flags under them. A line inside a recognised group that
// flagLineRE cannot parse is folded into the help text of the row before it,
// which is what a pflag description-wrapping continuation line actually is; a
// flag line that falls outside every group because its own header did not
// match groupHeaderRE — or matched no header pattern at all — still vanishes
// here, and that is what checkCompleteness below is for; it is the last thing
// this function does before returning.
func parseHelpGroups(help string) ([]flagGroup, error) {
	var groups []flagGroup
	var current *flagGroup
	skip := map[string]bool{"Usage": true, "Commands": true}

	for _, line := range strings.Split(help, "\n") {
		if m := groupHeaderRE.FindStringSubmatch(line); m != nil {
			title := m[1]
			if skip[title] {
				current = nil
				continue
			}
			groups = append(groups, flagGroup{title: title})
			current = &groups[len(groups)-1]
			continue
		}
		if bareHeaderRE.MatchString(line) {
			// An unindented, colon-terminated line that groupHeaderRE did not
			// recognise as a title: closing `current` here, rather than
			// leaving it open, keeps a flag line under this header from
			// being misattributed to whatever group preceded it.
			current = nil
			continue
		}
		if current == nil {
			continue
		}
		m := flagLineRE.FindStringSubmatch(line)
		if m == nil {
			// Not a flag line. The one shape worth keeping is pflag's own
			// wrapped-description continuation — indented text with no flag
			// token — which belongs to the row just parsed; anything else
			// (a blank line, a stray line inside a skipped section) trims to
			// nothing and is dropped.
			if trimmed := strings.TrimSpace(line); trimmed != "" && len(current.rows) > 0 {
				last := &current.rows[len(current.rows)-1]
				last.help = strings.TrimSpace(last.help + " " + trimmed)
			}
			continue
		}
		row := flagRow{short: m[1], long: m[2], typ: m[3], help: m[4]}
		if dm := pflagDefaultRE.FindStringSubmatch(row.help); dm != nil {
			row.help = strings.TrimSpace(dm[1])
			row.def = dm[2]
		}
		current.rows = append(current.rows, row)
	}
	if len(groups) == 0 {
		return nil, fmt.Errorf("parsed no flag groups from --help output (%d bytes)", len(help))
	}

	if err := checkCompleteness(help, groups); err != nil {
		return nil, err
	}
	return groups, nil
}

// checkCompleteness fails loudly when the number of --help lines that look
// like a flag (flagTokenRE, looser than flagLineRE) disagrees with the number
// of rows parseHelpGroups actually parsed into a row, so an unparseable or
// unattributed flag line is a build failure rather than a row silently
// missing from docs/FLAGS.md.
func checkCompleteness(help string, groups []flagGroup) error {
	var parsed int
	for _, g := range groups {
		parsed += len(g.rows)
	}

	var expected int
	var unmatched []string
	for _, line := range strings.Split(help, "\n") {
		if !flagTokenRE.MatchString(line) {
			continue
		}
		expected++
		if flagLineRE.FindStringSubmatch(line) == nil {
			unmatched = append(unmatched, strings.TrimSpace(line))
		}
	}
	if expected == parsed {
		return nil
	}

	msg := fmt.Sprintf("--help printed %d flag line(s) but only %d were parsed into a docs/FLAGS.md row",
		expected, parsed)
	if len(unmatched) > 0 {
		msg += fmt.Sprintf("; %d line(s) did not match flagLineRE: %s",
			len(unmatched), strings.Join(unmatched, " | "))
	} else {
		msg += " (a recognised flag line fell outside every group — check groupHeaderRE against the " +
			"section title above it)"
	}
	return fmt.Errorf("%s", msg)
}
