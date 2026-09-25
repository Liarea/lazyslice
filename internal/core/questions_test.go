// SPDX-License-Identifier: Apache-2.0

package core

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Liarea/lazyslice/internal/discover"
	"github.com/Liarea/lazyslice/internal/event"
	"github.com/Liarea/lazyslice/internal/pipeline"
)

// The first-run questions, as a whole run meets them (ADR-017, proposed,
// T-0343). ADR-008's one-question rule let the target question (Q1/Q1′) take
// a run's only question, and the root question (Q2) then defaulted silently:
// dogfood session 3 was asked Q1 and never saw Q2 (T-0331). The rule is now
// "no question whose default was already shown", so at a terminal every open
// item is asked, in ladder order, and a headless run asks nothing.

// q1Question and q2Question are the leading words of ADR-008 §6's Q1 and Q2
// prompts, which is all a case below compares.
const (
	q1Question = "no local postgres found to load into. start one?"
	q2Question = "root table? [public.customers]"
)

// questionPrompter is a controlling terminal that records every question put
// to it and answers each with its default: Enter, every time.
type questionPrompter struct{ asked []string }

func (p *questionPrompter) Confirm(q string, def bool) (bool, error) {
	p.asked = append(p.asked, q)
	return def, nil
}

func (p *questionPrompter) Ask(q string, def string) (string, error) {
	p.asked = append(p.asked, q)
	return def, nil
}

func (p *questionPrompter) Close() error { return nil }

// fakeDockerDaemon is a local Docker endpoint that answers a ping and lists
// no containers, so the real ladder reaches Q1's state — a source, nothing
// target-shaped, a usable local endpoint — with no daemon on the machine and
// no container created. tcp:// on a loopback literal is local by ADR-008 §3,
// and --docker-host is the first step of the resolution order, so the
// developer's own daemon is never consulted.
func fakeDockerDaemon(t *testing.T) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch {
		case strings.HasSuffix(req.URL.Path, "/_ping"):
			w.Header().Set("Api-Version", "1.47")
			w.Header().Set("Content-Type", "text/plain")
			_, _ = w.Write([]byte("OK"))
		case strings.HasSuffix(req.URL.Path, "/containers/json"):
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte("[]"))
		default:
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"message":"not found"}`))
		}
	}))
	t.Cleanup(srv.Close)
	return "tcp://" + srv.Listener.Addr().String()
}

// hermeticLadder empties every variable rungs 1 and 2 and the Docker client
// read, so the only candidate source is the fake daemon and the workdir is an
// empty temporary directory.
func hermeticLadder(t *testing.T) string {
	t.Helper()
	for _, name := range []string{
		"DATABASE_URL", "POSTGRES_URL", "PG_URL", "DB_URL",
		"PGSERVICE", "PGHOST", "PGHOSTADDR", "PGDATABASE", "PGPORT", "PGUSER",
		"DOCKER_HOST", "DOCKER_CONTEXT", "DOCKER_TLS_VERIFY", "DOCKER_CERT_PATH",
	} {
		t.Setenv(name, "")
	}
	return t.TempDir()
}

// q1AnsweredYes stands in for discover.Resolve on the one path a unit test
// cannot take through it: Q1 at a terminal, answered yes. The real ladder
// dials the source for its major before it asks and provisions a container
// after, and both need a live server. Everything else about the contract is
// discover's own, pinned in internal/discover/question_test.go: Q1 goes to
// Options.Prompter, which is the prompter core copied from the request, and
// the Result reports Asked. That Asked is true here is the point: core used to
// read it to skip Q2, and must not any more.
func q1AnsweredYes(t *testing.T) func(context.Context, discover.Options, event.Sink) (discover.Result, error) {
	return func(_ context.Context, o discover.Options, _ event.Sink) (discover.Result, error) {
		if o.Yes || o.NoControllingTerminal || o.Prompter == nil {
			t.Fatal("q1AnsweredYes reached on a headless run; the headless cases use the real ladder")
		}
		yes, err := o.Prompter.Confirm(q1Question+" postgres:16 as lazyslice-target-shop on port 5433 [Y/n]", true)
		if err != nil || !yes {
			t.Fatalf("Q1 answered %v, %v; this stand-in only models the yes branch", yes, err)
		}
		return discover.Result{
			Source: o.Source, SourceProvenance: pipeline.FromFlag,
			Target:           "postgres://postgres:x@127.0.0.1:5433/postgres",
			TargetProvenance: pipeline.FromContainer, TargetLabel: "lazyslice-target-shop",
			TargetContainerID: "lazyslice-target-shop",
			Asked:             true,
		}, nil
	}
}

// TestFirstRunAsksEveryOpenQuestionAtATerminalAndNoneHeadless is the four
// combinations of target open or settled and root open or settled, once at a
// terminal and twice headless (no controlling terminal, and --yes with a
// terminal present). Each case runs resolveEndpoints and then rootQuestion,
// the order execute runs them in.
//
// "Target settled" is --target, "root settled" is --root. The terminal case
// with the target settled and the root open is README's quickstart without its
// --root: one question, Q2.
func TestFirstRunAsksEveryOpenQuestionAtATerminalAndNoneHeadless(t *testing.T) {
	const source = "postgres://ls:x@127.0.0.1:55701/shop"
	const target = "postgres://ls:x@127.0.0.1:55702/shop_dev"

	type terminal int
	const (
		atATerminal terminal = iota
		noControllingTerminal
		yesFlag
	)
	for _, term := range []terminal{atATerminal, noControllingTerminal, yesFlag} {
		for _, targetOpen := range []bool{true, false} {
			for _, rootOpen := range []bool{true, false} {
				name := map[terminal]string{
					atATerminal: "terminal", noControllingTerminal: "no terminal", yesFlag: "--yes",
				}[term]
				name += map[bool]string{true: ", target open", false: ", target settled"}[targetOpen]
				name += map[bool]string{true: ", root open", false: ", root settled"}[rootOpen]

				t.Run(name, func(t *testing.T) {
					workdir := hermeticLadder(t)
					p := &questionPrompter{}
					req := Request{Source: source, Workdir: workdir, prompter: p}
					switch term {
					case atATerminal:
						// The prompter above is the terminal.
					case noControllingTerminal:
						req.noTerminal = true
					case yesFlag:
						req.Yes = true
					}
					if !targetOpen {
						req.Target = target
					}
					if !rootOpen {
						req.Root = "public.orders"
					}
					headless := term != atATerminal
					if targetOpen && headless {
						req.DockerHost = fakeDockerDaemon(t)
					}

					c := &eventCollector{}
					r := &run{req: normalise(req), sink: c}
					if targetOpen && !headless {
						r.resolve = q1AnsweredYes(t)
					}

					err := r.resolveEndpoints(context.Background())
					if targetOpen && headless {
						// Neither settled, or only the root: Q1's headless
						// answer is its hard failure, and the run never
						// reaches Q2.
						var stop *Stop
						if !errors.As(err, &stop) {
							t.Fatalf("resolveEndpoints = %v, want a Stop", err)
						}
						if stop.Exit != 4 || stop.Code != discover.CodeTargetNone ||
							stop.Args[event.ArgFlag] != "--create-target" {
							t.Errorf("stop = %s exit %d flag %q, want %s exit 4 naming --create-target",
								stop.Code, stop.Exit, stop.Args[event.ArgFlag], discover.CodeTargetNone)
						}
						if len(p.asked) != 0 {
							t.Errorf("a headless run asked %q", p.asked)
						}
						return
					}
					if err != nil {
						t.Fatalf("resolveEndpoints: %v", err)
					}

					r.schema = rootQuestionSchema()
					if err := r.rootQuestion(); err != nil {
						t.Fatalf("rootQuestion: %v", err)
					}

					var want []string
					if !headless && targetOpen {
						want = append(want, q1Question)
					}
					if !headless && rootOpen {
						want = append(want, q2Question)
					}
					if len(p.asked) != len(want) {
						t.Fatalf("asked %q, want questions starting %q", p.asked, want)
					}
					for i := range want {
						if !strings.HasPrefix(p.asked[i], want[i]) {
							t.Errorf("question %d = %q, want it to start %q", i+1, p.asked[i], want[i])
						}
					}

					wantRoot := "public.customers"
					if !rootOpen {
						wantRoot = "public.orders"
					}
					if got := rootDecision(t, c).Args[event.ArgTable]; got != wantRoot {
						t.Errorf("root = %q, want %q", got, wantRoot)
					}
				})
			}
		}
	}
}

// TestHeadlessFirstRunWithNothingSettledStopsNamingCreateTarget is the whole
// Run, not two of its stages: no --target, no --root, no committed
// lazyslice.yml, no controlling terminal, and a local Docker endpoint that
// answers with nothing on it. ADR-017 changes what a terminal is asked and
// nothing a headless run does, so this is still exit 4, target.refused.none,
// naming --create-target, and no root is decided because nothing ran past
// discovery.
func TestHeadlessFirstRunWithNothingSettledStopsNamingCreateTarget(t *testing.T) {
	workdir := hermeticLadder(t)
	p := &questionPrompter{}
	c := &eventCollector{}
	_, err := Run(context.Background(), Request{
		Source:     "postgres://ls:x@127.0.0.1:55701/shop",
		Workdir:    workdir,
		ConfigPath: filepath.Join(workdir, DefaultConfigPath),
		DockerHost: fakeDockerDaemon(t),
		prompter:   p,
		noTerminal: true,
	}, c)

	var stop *Stop
	if !errors.As(err, &stop) {
		t.Fatalf("Run = %v, want a Stop", err)
	}
	if stop.Exit != 4 || stop.Code != discover.CodeTargetNone || stop.Args[event.ArgFlag] != "--create-target" {
		t.Errorf("stop = %s exit %d flag %q, want %s exit 4 naming --create-target",
			stop.Code, stop.Exit, stop.Args[event.ArgFlag], discover.CodeTargetNone)
	}
	if len(p.asked) != 0 {
		t.Errorf("a headless run asked %q", p.asked)
	}
	if n := countEvents(c, CodeRootChosen); n != 0 {
		t.Errorf("%s sent %d time(s), want 0: the run stops at discovery", CodeRootChosen, n)
	}
}
