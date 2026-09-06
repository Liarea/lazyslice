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

	ctr, err := testcontainers.Run(ctx, image,
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
	)
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

	endpoint, err := ctr.PortEndpoint(ctx, "5432/tcp", "")
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
