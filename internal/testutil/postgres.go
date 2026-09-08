// SPDX-License-Identifier: Apache-2.0

// Package testutil starts the databases the integration and invariant suites
// run against.
//
// Nothing in lazyslice is proved by a unit test alone. The subset planner, the
// gate, the loader and the residual scan are all statements about a real
// Postgres, so the tests that matter need a real Postgres, and the cheapest
// honest way to get one is a container per test binary.
//
// This package is imported only from _test.go files, so the released binary
// never links testcontainers or the Docker client it brings with it.
package testutil

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	// DefaultImage is the Postgres a local integration run uses. The CI matrix
	// runs the same suite on the oldest and newest supported majors (ADR-003);
	// this is the middle one, for a fast local run.
	DefaultImage = "postgres:16"

	// ImageEnv overrides DefaultImage, which is how the CI matrix runs the whole
	// suite on 14 and on 18 without every test knowing about the matrix.
	ImageEnv = "LAZYSLICE_TEST_POSTGRES_IMAGE"

	// user, password and database are fixed because the container is thrown
	// away with the test binary and nothing outside the test can reach it.
	user     = "lazyslice"
	password = "lazyslice"
	database = "lazyslice_test"

	// startupTimeout bounds a cold image pull plus initdb. A slower bound here
	// costs one flake per contributor with an empty image cache.
	startupTimeout = 3 * time.Minute

	// portEndpointAttempts and portEndpointBudget bound the retry of
	// PortEndpoint below: ~10 attempts spread over ~5 seconds. The wait
	// strategy above already confirmed Postgres is accepting connections, but
	// Docker's own port-mapping table can lag a beat behind that log line, so
	// the first PortEndpoint call can still report the port unmapped (T-0052).
	portEndpointAttempts = 10
	portEndpointBudget   = 5 * time.Second
)

// Postgres starts a Postgres container and returns a connection URL for it.
//
// The container is terminated when the test finishes, including when it fails,
// through t.Cleanup. Each call starts its own container: sharing one between
// tests would make the target gate's emptiness rule untestable, because a test
// would then see another test's rows.
//
// image is the container image to run; pass "" for $LAZYSLICE_TEST_POSTGRES_IMAGE,
// or DefaultImage when that is unset. The CI matrix sets it to postgres:14 and
// postgres:18 (ARCHITECTURE.md section 14).
//
// The returned URL carries a password, so it is a dsn.DSN by nature: hand it to
// a driver, and do not put it in an event, a config or a test name.
func Postgres(ctx context.Context, t *testing.T, image string) string {
	t.Helper()

	if image == "" {
		image = os.Getenv(ImageEnv)
	}
	if image == "" {
		image = DefaultImage
	}

	opts := []testcontainers.ContainerCustomizer{
		testcontainers.WithEnv(map[string]string{
			"POSTGRES_USER":     user,
			"POSTGRES_PASSWORD": password,
			"POSTGRES_DB":       database,
			// initdb is the slow part of a cold start and none of these
			// databases outlives the test binary, so durability buys nothing.
			"POSTGRES_INITDB_ARGS": "--nosync",
		}),
		testcontainers.WithExposedPorts("5432/tcp"),
		// The entrypoint starts the server once on a socket to run the init
		// scripts and then restarts it on TCP, so the log line appears twice and
		// only the second one means the port is open.
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(startupTimeout),
		),
	}

	ctr, err, termErr := runWithReaperRetry(ctx,
		func(ctx context.Context) (testcontainers.Container, error) {
			return testcontainers.Run(ctx, image, opts...)
		},
		func(c testcontainers.Container) error {
			return testcontainers.TerminateContainer(c)
		},
	)
	if termErr != nil {
		t.Logf("testutil: terminating orphaned %s after stale-reaper retry: %v", image, termErr)
	}
	// Registered before the error is checked, and nil-safe, because
	// testcontainers.Run returns a container alongside its error precisely so
	// that a container which started but failed its wait strategy can still be
	// terminated. That is the likely failure here, given the startup timeout
	// this file exists to bound, and Ryuk does not reap it wherever
	// TESTCONTAINERS_RYUK_DISABLED is set.
	t.Cleanup(func() {
		// A separate context: ctx may already be cancelled by the time the test
		// finishes, and a leaked container outlives the run.
		stop, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
		defer cancel()
		if termErr := testcontainers.TerminateContainer(ctr, testcontainers.StopContext(stop)); termErr != nil {
			t.Logf("testutil: terminating %s: %v", image, termErr)
		}
	})
	if err != nil {
		t.Fatalf("testutil: starting %s: %v", image, err)
	}

	endpoint, err := portEndpointWithRetry(ctx, ctr, "5432/tcp", "", time.Sleep)
	if err != nil {
		t.Fatalf("testutil: resolving the mapped port of %s: %v", image, err)
	}

	u := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(user, password),
		Host:     endpoint,
		Path:     "/" + database,
		RawQuery: "sslmode=disable",
	}
	return u.String()
}

// SkipWithoutDocker skips the calling test when no Docker endpoint is
// reachable, so that `go test -tags integration ./...` on a machine without
// Docker reports skips rather than a wall of failures that say nothing about
// the code.
func SkipWithoutDocker(ctx context.Context, t *testing.T) {
	t.Helper()

	provider, err := testcontainers.ProviderDocker.GetProvider()
	if err != nil {
		t.Skipf("testutil: no container provider: %v", err)
	}
	if err := provider.Health(ctx); err != nil {
		t.Skipf("testutil: docker is not reachable: %v", err)
	}
}

// isStaleReaperError reports whether err is the "No such container" failure
// Ryuk raises when it races a just-removed container from a prior test. It is
// a plain string match: the error crosses a Docker API boundary and
// testcontainers-go does not give it a sentinel or a type to compare against.
func isStaleReaperError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "No such container")
}

// runWithReaperRetry calls run once. If it fails with the stale-reaper race
// isStaleReaperError recognizes, it terminates the first attempt's container
// via terminate -- Ryuk never registered it either, since the reaper connect
// is what failed, so nothing else will reap it -- and calls run exactly once
// more.
//
// terminate's own error, if any, is returned as terminateErr rather than
// folded into err: it is a leak warning worth logging, not a reason to fail
// the run when the retry itself succeeded.
//
// run and terminate are both injected so a unit test can drive this against
// a fake instead of a real Docker daemon.
func runWithReaperRetry(
	ctx context.Context,
	run func(context.Context) (testcontainers.Container, error),
	terminate func(testcontainers.Container) error,
) (ctr testcontainers.Container, err error, terminateErr error) {
	ctr, err = run(ctx)
	if err != nil && isStaleReaperError(err) {
		// Ryuk (the reaper) is shared across the test binary's containers. If a
		// prior test's container was already removed (its own cleanup, or a
		// slow daemon), a Run that starts while Ryuk is still processing that
		// removal can fail registering this container with a "No such
		// container" error that has nothing to do with the image we asked for.
		// One retry is enough: Ryuk's bookkeeping is on the order of
		// milliseconds behind Docker's own state.
		terminateErr = terminate(ctr)
		ctr, err = run(ctx)
	}
	return ctr, err, terminateErr
}

// portEndpointer is the subset of testcontainers.Container that
// portEndpointWithRetry needs. It exists so a unit test can retry against a
// fake instead of a real container.
type portEndpointer interface {
	PortEndpoint(ctx context.Context, port, proto string) (string, error)
}

// portEndpointWithRetry resolves the mapped host:port for port/proto,
// retrying with a bounded backoff (portEndpointAttempts attempts spread over
// portEndpointBudget) before giving up. sleep is injected so a unit test can
// drive the loop without actually waiting.
func portEndpointWithRetry(ctx context.Context, c portEndpointer, port, proto string, sleep func(time.Duration)) (string, error) {
	delay := portEndpointBudget / portEndpointAttempts

	var lastErr error
	for attempt := 1; attempt <= portEndpointAttempts; attempt++ {
		// Checked before every call, including the first: a context that is
		// already cancelled or past its deadline is not the T-0052 port-mapping
		// race below, and burning the retry budget's sleeps on it would only
		// delay reporting an error that has nothing to do with that race.
		if ctxErr := ctx.Err(); ctxErr != nil {
			return "", ctxErr
		}
		endpoint, err := c.PortEndpoint(ctx, port, proto)
		if err == nil {
			return endpoint, nil
		}
		lastErr = err
		if attempt < portEndpointAttempts {
			sleep(delay)
		}
	}
	return "", fmt.Errorf("port not mapped after %d attempts over %s (known startup race between the container reporting ready and Docker's port table catching up, see T-0052): %w",
		portEndpointAttempts, portEndpointBudget, lastErr)
}

// URL parses a connection URL produced by Postgres. It exists so that a test
// that needs the host and port separately does not re-implement the parsing and
// get it subtly wrong.
func URL(connURL string) (*url.URL, error) {
	u, err := url.Parse(connURL)
	if err != nil {
		return nil, fmt.Errorf("testutil: parsing connection url: %w", err)
	}
	return u, nil
}
