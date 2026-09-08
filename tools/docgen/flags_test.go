// SPDX-License-Identifier: Apache-2.0

package main

import (
	"strings"
	"testing"
)

// TestParseHelpGroups pins parseHelpGroups against a realistic --help excerpt,
// two header shapes groupHeaderRE now recognises (a digit, an ampersand) that
// it used to fall through on, a header shape it still does not recognise
// while a previous group is open, and a flag line carrying pflag's
// NoOptDefVal bracket syntax. The NoOptDefVal case, and the still-unrecognised
// header case, used to vanish from docs/FLAGS.md with `make docs-check` still
// green, because docs-check diffs generator output against generator output;
// checkCompleteness is what turns each into a build failure instead.
func TestParseHelpGroups(t *testing.T) {
	tests := []struct {
		name      string
		help      string
		wantErr   bool
		wantRows  int // total rows across every group, only checked when wantErr is false
		wantGroup string
	}{
		{
			name: "ordinary flags, short and long, bool and typed, with a real default",
			help: "Usage:\n" +
				"  lazyslice [DSN] [flags]\n" +
				"\n" +
				"discover:\n" +
				"      --source string   Names the source; a non-Postgres scheme or unsupported major exits 2\n" +
				"      --create-target   Start postgres:<source major> as lazyslice-target-<project> instead of asking\n" +
				"\n" +
				"plan:\n" +
				"  -n, --take int   Root rows, ORDER BY identity LIMIT N (default 500)\n" +
				"\n" +
				"other:\n" +
				"  -h, --help      help for lazyslice\n",
			wantErr:   false,
			wantRows:  4,
			wantGroup: "discover",
		},
		{
			// "postgres 14" (a digit) and "load & verify" (an ampersand) are
			// both real section-title shapes now, not fall-through cases: a
			// section title is free to carry a version number or join two
			// stage names, and a header this narrow used to force
			// checkCompleteness above.
			name: "a digit and an ampersand in a header title are recognised as their own groups",
			help: "Usage:\n" +
				"  lazyslice [DSN] [flags]\n" +
				"\n" +
				"discover:\n" +
				"      --source string   Names the source\n" +
				"\n" +
				"postgres 14:\n" +
				"      --strict-schema   Exit 10 on any column the committed yml has never seen\n" +
				"\n" +
				"load & verify:\n" +
				"      --residual-probe-cap int   Source confirmation probes per run\n",
			wantErr:   false,
			wantRows:  3,
			wantGroup: "postgres 14",
		},
		{
			// A comma is still outside groupHeaderRE's charset. What matters
			// here is not the header itself but that "discover" was already
			// open when it appears: bareHeaderRE must still close `current`
			// rather than leave --residual-probe-cap to be silently
			// misattributed to "discover" — the realistic shape of the
			// vanish bug (an unrecognised header following an open group),
			// as opposed to the disused-header shape below.
			name: "an unrecognised header while a previous group is open closes that group instead of misattributing its flags",
			help: "Usage:\n" +
				"  lazyslice [DSN] [flags]\n" +
				"\n" +
				"discover:\n" +
				"      --source string   Names the source\n" +
				"\n" +
				"load, verify:\n" +
				"      --residual-probe-cap int   Source confirmation probes per run\n",
			wantErr: true,
		},
		{
			name: "a NoOptDefVal flag has no two-space separator before its bracketed default",
			help: "Usage:\n" +
				"  lazyslice [DSN] [flags]\n" +
				"\n" +
				"render:\n" +
				"      --colour[=\"auto\"]   Colourise output; bare form means auto\n" +
				"      --json               NDJSON events on stdout\n",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			groups, err := parseHelpGroups(tc.help)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("parseHelpGroups(): got no error, want one (groups: %+v)", groups)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseHelpGroups(): %v", err)
			}
			var got int
			var sawGroup bool
			for _, g := range groups {
				got += len(g.rows)
				if g.title == tc.wantGroup {
					sawGroup = true
				}
			}
			if got != tc.wantRows {
				t.Errorf("parsed %d row(s), want %d (groups: %+v)", got, tc.wantRows, groups)
			}
			if !sawGroup {
				t.Errorf("group %q not found in %+v", tc.wantGroup, groups)
			}
		})
	}
}

// TestParseHelpGroupsAgainstRealHelp guards the parser against the actual
// registered flag set: every line `go run ./cmd/lazyslice --help` prints that
// looks like a flag must have become exactly one row, the same invariant
// checkCompleteness enforces at `make docs` time, run here without a subprocess.
func TestParseHelpGroupsAgainstRealHelp(t *testing.T) {
	help := `Usage:
  lazyslice [DSN] [flags]

Commands:
  classify     Classify every column and print the reasons
  doctor       Print what lazyslice can see and what it cannot prove
  introspect   Read the source catalog and print it
  plan         Print the subset plan and stop
  verify       Re-run the whole slice into the target and re-check it (drops and reloads the target)
  version      Print the version, commit, Go version and supported Postgres majors

discover:
      --allow-remote-target string   Permit a non-local target whose host equals HOST; without it a remote target is exit 4
      --create-target                Start postgres:<source major> as lazyslice-target-<project> instead of asking
      --docker-host string           Docker endpoint for container discovery (default: DOCKER_HOST, context, default sockets)
      --password-command string      Command whose stdout is the password; recorded in the yml as a reference, never its output
      --reconfigure                  Ignore an existing lazyslice.yml and run the first-run path
      --require-read-only-role       Exit 6 when the source role holds INSERT, UPDATE or DELETE
      --source string                Names the source; a non-Postgres scheme or unsupported major exits 2
      --target string                Names the target; never bypasses the gate

other:
  -h, --help      help for lazyslice
      --version   Print the version, commit, Go version and supported Postgres majors
`
	groups, err := parseHelpGroups(help)
	if err != nil {
		t.Fatalf("parseHelpGroups(): %v", err)
	}
	var got int
	for _, g := range groups {
		got += len(g.rows)
	}
	const want = 10 // 8 discover flags + -h/--help + --version
	if got != want {
		t.Errorf("parsed %d row(s), want %d", got, want)
	}
	if !strings.Contains(help, "--source") {
		t.Fatal("test fixture is missing --source; the fixture drifted from itself")
	}
}

// TestParseHelpGroupsNonQuotedDefault pins a pflag default rendering
// pflagDefaultRE's old value alternation did not cover: a duration. That
// shape used to parse with an empty def and the "(default 30s)" suffix left
// inside the Description, silently, with checkCompleteness unable to notice
// because the line still became exactly one row.
func TestParseHelpGroupsNonQuotedDefault(t *testing.T) {
	help := "Usage:\n" +
		"  lazyslice [DSN] [flags]\n" +
		"\n" +
		"plan:\n" +
		"      --timeout duration   How long to wait (default 30s)\n"

	groups, err := parseHelpGroups(help)
	if err != nil {
		t.Fatalf("parseHelpGroups(): %v", err)
	}
	row := groups[0].rows[0]
	if row.def != "30s" {
		t.Errorf("def = %q, want %q (a non-quoted pflag default like a duration must still be split out of the Description)", row.def, "30s")
	}
	wantHelp := "How long to wait"
	if row.help != wantHelp {
		t.Errorf("help = %q, want %q", row.help, wantHelp)
	}
}

// TestParseHelpGroupsWrappedDescription pins pflag's own description-wrapping
// continuation line: a second line, indented to the description column and
// carrying no flag token, that flagLineRE cannot match. That line used to be
// dropped silently, with checkCompleteness unable to notice because
// flagTokenRE does not count it as an expected flag line either.
func TestParseHelpGroupsWrappedDescription(t *testing.T) {
	help := "Usage:\n" +
		"  lazyslice [DSN] [flags]\n" +
		"\n" +
		"discover:\n" +
		"      --source string   Names the source; a non-Postgres scheme or unsupported\n" +
		"                        major exits 2\n"

	groups, err := parseHelpGroups(help)
	if err != nil {
		t.Fatalf("parseHelpGroups(): %v", err)
	}
	want := "Names the source; a non-Postgres scheme or unsupported major exits 2"
	got := groups[0].rows[0].help
	if got != want {
		t.Errorf("help = %q, want %q (a wrapped continuation line must fold into the row, not vanish)", got, want)
	}
}
