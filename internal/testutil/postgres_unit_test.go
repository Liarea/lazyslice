// SPDX-License-Identifier: Apache-2.0

package testutil

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
)

// fakePortEndpoint fails its first n calls, each with its own distinguishable
// error, then succeeds. It records every delay it was asked to sleep so the
// test can check the retry actually backs off instead of busy-looping.
type fakePortEndpoint struct {
	failures int
	calls    int
	sleeps   []time.Duration
}

// errPortNotMapped is the error the last configured failure returns, so a
// test that only cares about the sentinel (rather than which attempt
// produced it) can still use errors.Is.
var errPortNotMapped = fmt.Errorf("port 5432/tcp not found (attempt %d)", 0)

func (f *fakePortEndpoint) PortEndpoint(_ context.Context, _, _ string) (string, error) {
	f.calls++
	if f.calls <= f.failures {
		if f.calls == f.failures {
			return "", errPortNotMapped
		}
		return "", fmt.Errorf("port 5432/tcp not found (attempt %d)", f.calls)
	}
	return "127.0.0.1:54321", nil
}

func (f *fakePortEndpoint) sleep(d time.Duration) {
	f.sleeps = append(f.sleeps, d)
}

// No build tag and no Docker: this exercises the retry loop itself, not a
// real container, so it belongs in every `go test ./...` run (see T-0052).
func TestPortEndpointWithRetryRecoversFromTransientMiss(t *testing.T) {
	t.Parallel()

	f := &fakePortEndpoint{failures: 2}
	endpoint, err := portEndpointWithRetry(context.Background(), f, "5432/tcp", "", f.sleep, portEndpointAttempts, portEndpointBudget)
	if err != nil {
		t.Fatalf("portEndpointWithRetry: unexpected error: %v", err)
	}
	if endpoint != "127.0.0.1:54321" {
		t.Fatalf("portEndpointWithRetry: got endpoint %q, want 127.0.0.1:54321", endpoint)
	}
	if f.calls != 3 {
		t.Fatalf("portEndpointWithRetry: called PortEndpoint %d times, want 3 (2 failures + 1 success)", f.calls)
	}
	if len(f.sleeps) != 2 {
		t.Fatalf("portEndpointWithRetry: slept %d times, want 2 (one before each retry)", len(f.sleeps))
	}
	wantDelay := portEndpointBudget / portEndpointAttempts
	for i, d := range f.sleeps {
		if d != wantDelay {
			t.Fatalf("portEndpointWithRetry: sleep %d was %s, want %s (portEndpointBudget/portEndpointAttempts)", i, d, wantDelay)
		}
	}
}

func TestPortEndpointWithRetryGivesUpAfterAllAttempts(t *testing.T) {
	t.Parallel()

	f := &fakePortEndpoint{failures: portEndpointAttempts}
	_, err := portEndpointWithRetry(context.Background(), f, "5432/tcp", "", f.sleep, portEndpointAttempts, portEndpointBudget)
	if err == nil {
		t.Fatalf("portEndpointWithRetry: expected an error after %d failures, got nil", portEndpointAttempts)
	}
	if !errors.Is(err, errPortNotMapped) {
		t.Fatalf("portEndpointWithRetry: error %v does not wrap the underlying PortEndpoint error", err)
	}
	if !strings.Contains(err.Error(), "T-0052") {
		t.Fatalf("portEndpointWithRetry: error %q does not name the known race", err.Error())
	}
	if f.calls != portEndpointAttempts {
		t.Fatalf("portEndpointWithRetry: called PortEndpoint %d times, want %d", f.calls, portEndpointAttempts)
	}
	// One fewer sleep than attempts: no point backing off after the last try.
	if len(f.sleeps) != portEndpointAttempts-1 {
		t.Fatalf("portEndpointWithRetry: slept %d times, want %d", len(f.sleeps), portEndpointAttempts-1)
	}
}

func TestPortEndpointWithRetryReturnsContextErrorWithoutSleeping(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	f := &fakePortEndpoint{}
	_, err := portEndpointWithRetry(ctx, f, "5432/tcp", "", f.sleep, portEndpointAttempts, portEndpointBudget)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("portEndpointWithRetry: error %v, want context.Canceled", err)
	}
	if strings.Contains(err.Error(), "T-0052") {
		t.Fatalf("portEndpointWithRetry: error %q blames the T-0052 port-mapping race for a cancelled context", err.Error())
	}
	if f.calls != 0 {
		t.Fatalf("portEndpointWithRetry: called PortEndpoint %d times, want 0 (context was already done)", f.calls)
	}
	if len(f.sleeps) != 0 {
		t.Fatalf("portEndpointWithRetry: slept %d times, want 0 (context was already done)", len(f.sleeps))
	}
}

// fakePortEndpointAlwaysFails never succeeds, so it drives
// portEndpointWithRestart into its restart path deterministically.
type fakePortEndpointAlwaysFails struct {
	calls int
}

func (f *fakePortEndpointAlwaysFails) PortEndpoint(_ context.Context, _, _ string) (string, error) {
	f.calls++
	return "", fmt.Errorf("port 5432/tcp not found (attempt %d)", f.calls)
}

// fakeSecondContainer is the replacement portEndpointWithRestart's recreate
// returns. It embeds testcontainers.Container (nil) to satisfy the interface
// and overrides PortEndpoint to delegate to a fakePortEndpoint, since
// portEndpointWithRestart polls the recreated container the same way it
// polled the first one.
type fakeSecondContainer struct {
	testcontainers.Container
	endpoint interface {
		PortEndpoint(context.Context, string, string) (string, error)
	}
}

func (f *fakeSecondContainer) PortEndpoint(ctx context.Context, port, proto string) (string, error) {
	return f.endpoint.PortEndpoint(ctx, port, proto)
}

func TestPortEndpointWithRestartRecreatesOnceThenSucceeds(t *testing.T) {
	t.Parallel()

	first := &fakePortEndpointAlwaysFails{}
	second := &fakePortEndpoint{failures: 1}
	sleep := func(time.Duration) {}

	terminateCalls := 0
	terminate := func() error {
		terminateCalls++
		return nil
	}
	recreateCalls := 0
	newCtr := &fakeSecondContainer{endpoint: second}
	recreate := func(context.Context) (testcontainers.Container, error) {
		recreateCalls++
		return newCtr, nil
	}

	endpoint, ctr, err := portEndpointWithRestart(context.Background(), first, terminate, recreate, "5432/tcp", "", sleep)
	if err != nil {
		t.Fatalf("portEndpointWithRestart: unexpected error: %v", err)
	}
	if endpoint != "127.0.0.1:54321" {
		t.Fatalf("portEndpointWithRestart: got endpoint %q, want 127.0.0.1:54321", endpoint)
	}
	if ctr != newCtr {
		t.Fatalf("portEndpointWithRestart: returned container %v, want the recreated container", ctr)
	}
	if terminateCalls != 1 {
		t.Fatalf("portEndpointWithRestart: terminate called %d times, want 1", terminateCalls)
	}
	if recreateCalls != 1 {
		t.Fatalf("portEndpointWithRestart: recreate called %d times, want 1", recreateCalls)
	}
	if first.calls != portEndpointAttempts {
		t.Fatalf("portEndpointWithRestart: first container's PortEndpoint called %d times, want %d (the full budget before restarting)", first.calls, portEndpointAttempts)
	}
	if second.calls != 2 {
		t.Fatalf("portEndpointWithRestart: recreated container's PortEndpoint called %d times, want 2 (1 failure + 1 success)", second.calls)
	}
}

// TestPortEndpointWithRestartDoesNotRestartWhenThePortResolves is the fast
// path: a container whose port resolves, with or without a few transient
// misses along the way, must not be terminated and recreated. Driving every
// other portEndpointWithRestart test through a container that never resolves
// would leave an implementation that restarts unconditionally fully green.
func TestPortEndpointWithRestartDoesNotRestartWhenThePortResolves(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name     string
		failures int
	}{
		{"resolves first try", 0},
		{"resolves after a few transient misses", 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			f := &fakePortEndpoint{failures: tc.failures}
			sleep := func(time.Duration) {}
			terminateCalls, recreateCalls := 0, 0
			terminate := func() error { terminateCalls++; return nil }
			recreate := func(context.Context) (testcontainers.Container, error) {
				recreateCalls++
				return nil, nil
			}

			endpoint, ctr, err := portEndpointWithRestart(context.Background(), f, terminate, recreate, "5432/tcp", "", sleep)
			if err != nil {
				t.Fatalf("portEndpointWithRestart: unexpected error: %v", err)
			}
			if endpoint != "127.0.0.1:54321" {
				t.Fatalf("portEndpointWithRestart: got endpoint %q, want 127.0.0.1:54321", endpoint)
			}
			if terminateCalls != 0 || recreateCalls != 0 {
				t.Fatalf("portEndpointWithRestart: terminate/recreate called %d/%d times, want 0/0 (the port resolved within the first budget)", terminateCalls, recreateCalls)
			}
			// f does not implement testcontainers.Container, so
			// portEndpointWithRestart's success path returns a nil container
			// alongside the original f (see the type assertion it makes).
			if ctr != nil {
				t.Fatalf("portEndpointWithRestart: returned container %v, want nil (fakePortEndpoint is not a testcontainers.Container)", ctr)
			}
		})
	}
}

// TestPortEndpointWithRestartRoutesRecreateThroughReaperRetry drives recreate
// into the same "No such container" failure runWithReaperRetry exists to
// absorb, and asserts a second Run is attempted rather than the restart
// failing outright. The restart path terminates a container immediately
// before calling recreate, which is exactly the window that error occurs in.
func TestPortEndpointWithRestartRoutesRecreateThroughReaperRetry(t *testing.T) {
	t.Parallel()

	first := &fakePortEndpointAlwaysFails{}
	second := &fakePortEndpoint{failures: 0}
	sleep := func(time.Duration) {}
	terminate := func() error { return nil }

	recreateCalls := 0
	newCtr := &fakeSecondContainer{endpoint: second}
	recreate := func(context.Context) (testcontainers.Container, error) {
		recreateCalls++
		if recreateCalls == 1 {
			return nil, errors.New(`Error response from daemon: No such container: abc123`)
		}
		return newCtr, nil
	}

	endpoint, ctr, err := portEndpointWithRestart(context.Background(), first, terminate, recreate, "5432/tcp", "", sleep)
	if err != nil {
		t.Fatalf("portEndpointWithRestart: unexpected error: %v", err)
	}
	if endpoint != "127.0.0.1:54321" {
		t.Fatalf("portEndpointWithRestart: got endpoint %q, want 127.0.0.1:54321", endpoint)
	}
	if ctr != newCtr {
		t.Fatalf("portEndpointWithRestart: returned container %v, want the recreated container", ctr)
	}
	if recreateCalls != 2 {
		t.Fatalf("portEndpointWithRestart: recreate called %d times, want 2 (one stale-reaper failure, one retry)", recreateCalls)
	}
}

func TestPortEndpointWithRestartFailsNamingT0052AfterBothAttemptsExhausted(t *testing.T) {
	t.Parallel()

	first := &fakePortEndpointAlwaysFails{}
	sleep := func(time.Duration) {}
	terminate := func() error { return nil }
	second := &fakePortEndpointAlwaysFails{}
	newCtr := &fakeSecondContainer{endpoint: second}
	recreate := func(context.Context) (testcontainers.Container, error) {
		return newCtr, nil
	}

	_, ctr, err := portEndpointWithRestart(context.Background(), first, terminate, recreate, "5432/tcp", "", sleep)
	if err == nil {
		t.Fatalf("portEndpointWithRestart: expected an error after both attempts were exhausted")
	}
	if !strings.Contains(err.Error(), "T-0052") {
		t.Fatalf("portEndpointWithRestart: error %q does not name the known race", err.Error())
	}
	if !strings.Contains(err.Error(), "restart") {
		t.Fatalf("portEndpointWithRestart: error %q does not say it restarted", err.Error())
	}
	if !strings.Contains(err.Error(), portEndpointBudget.String()) {
		t.Fatalf("portEndpointWithRestart: error %q does not say how long it waited", err.Error())
	}
	if first.calls != portEndpointAttempts {
		t.Fatalf("portEndpointWithRestart: first container's PortEndpoint called %d times, want %d", first.calls, portEndpointAttempts)
	}
	if second.calls != portEndpointRestartAttempts {
		t.Fatalf("portEndpointWithRestart: recreated container's PortEndpoint called %d times, want %d (the shorter restart budget spent after restarting)", second.calls, portEndpointRestartAttempts)
	}
	if ctr != newCtr {
		t.Fatalf("portEndpointWithRestart: returned container %v, want the recreated container even on failure, so cleanup targets it", ctr)
	}
}

func TestPortEndpointWithRestartDoesNotRestartOnContextError(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	f := &fakePortEndpoint{}
	terminateCalls, recreateCalls := 0, 0
	terminate := func() error { terminateCalls++; return nil }
	recreate := func(context.Context) (testcontainers.Container, error) {
		recreateCalls++
		return nil, nil
	}

	_, _, err := portEndpointWithRestart(ctx, f, terminate, recreate, "5432/tcp", "", f.sleep)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("portEndpointWithRestart: error %v, want context.Canceled", err)
	}
	if terminateCalls != 0 || recreateCalls != 0 {
		t.Fatalf("portEndpointWithRestart: terminate/recreate called %d/%d times, want 0/0 (a cancelled context is not the T-0052 race)", terminateCalls, recreateCalls)
	}
}

// fakeReaperRun and fakeReaperTerminate below back
// TestRunWithReaperRetry*: runWithReaperRetry only threads its two injected
// functions together, so a real testcontainers.Container is never needed --
// nil satisfies the interface for these tests, since nothing here calls a
// method on it.

func TestRunWithReaperRetryRetriesOnceOnStaleReaperError(t *testing.T) {
	t.Parallel()

	var runCalls, terminateCalls int
	run := func(context.Context) (testcontainers.Container, error) {
		runCalls++
		if runCalls == 1 {
			return nil, errors.New(`Error response from daemon: No such container: abc123`)
		}
		return nil, nil
	}
	terminate := func(testcontainers.Container) error {
		terminateCalls++
		return nil
	}

	_, err, termErr := runWithReaperRetry(context.Background(), run, terminate)
	if err != nil {
		t.Fatalf("runWithReaperRetry: unexpected error: %v", err)
	}
	if termErr != nil {
		t.Fatalf("runWithReaperRetry: unexpected terminate error: %v", termErr)
	}
	if runCalls != 2 {
		t.Fatalf("runWithReaperRetry: run called %d times, want 2 (one retry)", runCalls)
	}
	if terminateCalls != 1 {
		t.Fatalf("runWithReaperRetry: terminate called %d times, want 1 (the orphaned first attempt)", terminateCalls)
	}
}

func TestRunWithReaperRetryDoesNotRetryOtherErrors(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("connection refused")
	var runCalls, terminateCalls int
	run := func(context.Context) (testcontainers.Container, error) {
		runCalls++
		return nil, wantErr
	}
	terminate := func(testcontainers.Container) error {
		terminateCalls++
		return nil
	}

	_, err, termErr := runWithReaperRetry(context.Background(), run, terminate)
	if !errors.Is(err, wantErr) {
		t.Fatalf("runWithReaperRetry: error %v, want %v", err, wantErr)
	}
	if termErr != nil {
		t.Fatalf("runWithReaperRetry: unexpected terminate error: %v", termErr)
	}
	if runCalls != 1 {
		t.Fatalf("runWithReaperRetry: run called %d times, want 1 (a non-reaper error must not retry)", runCalls)
	}
	if terminateCalls != 0 {
		t.Fatalf("runWithReaperRetry: terminate called %d times, want 0", terminateCalls)
	}
}

func TestRunWithReaperRetrySurfacesTerminateErrorWithoutFailingTheRetry(t *testing.T) {
	t.Parallel()

	wantTermErr := errors.New("terminate: no such container")
	runCalls := 0
	run := func(context.Context) (testcontainers.Container, error) {
		runCalls++
		if runCalls == 1 {
			return nil, errors.New(`Error response from daemon: No such container: abc123`)
		}
		return nil, nil
	}
	terminate := func(testcontainers.Container) error {
		return wantTermErr
	}

	_, err, termErr := runWithReaperRetry(context.Background(), run, terminate)
	if err != nil {
		t.Fatalf("runWithReaperRetry: unexpected error: %v", err)
	}
	if !errors.Is(termErr, wantTermErr) {
		t.Fatalf("runWithReaperRetry: terminate error %v, want %v", termErr, wantTermErr)
	}
}

func TestIsStaleReaperError(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"unrelated error", errors.New("connection refused"), false},
		{"ryuk stale container", errors.New(`Error response from daemon: No such container: abc123`), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := isStaleReaperError(tc.err); got != tc.want {
				t.Errorf("isStaleReaperError(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}
