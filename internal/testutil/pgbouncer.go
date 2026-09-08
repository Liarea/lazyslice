// SPDX-License-Identifier: Apache-2.0

package testutil

import (
	"context"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/network"
	"github.com/testcontainers/testcontainers-go/wait"
)

// A pooled source endpoint is a supported v1 topology (ADR-005 "Pooled
// endpoints", ARCHITECTURE.md section 2 "Source.Reader", section 8's
// --single-connection), and until this file existed nothing in the suite spoke
// to a real pooler: internal/pg's TestConnectAddsNoStartupParameterAPoolerWouldRefuse
// checks the parameters lazyslice *sends*, against a list of what PgBouncer
// documents it accepts, and says so in its own comment. That test cannot fail
// on a list that has gone stale, and it cannot see anything else a pooler
// refuses. PgBouncer here is what answers.

const (
	// PgBouncerImage is the pooler a local integration run uses. It is pinned
	// rather than :latest so that a pooler behaviour change arrives as a
	// deliberate bump with a reason, not as one morning's red suite.
	PgBouncerImage = "edoburu/pgbouncer:v1.25.2-p0"

	// PgBouncerImageEnv overrides PgBouncerImage, as ImageEnv does for the
	// server.
	PgBouncerImageEnv = "LAZYSLICE_TEST_PGBOUNCER_IMAGE"

	// pgBouncerPort is the port the pooler listens on inside its container. It
	// is PgBouncer's own default, and it is named here because the wait strategy
	// and the port mapping have to agree with the LISTEN_PORT below.
	pgBouncerPort = "6432"

	// upstreamAlias is the name the pooler reaches the server by on the network
	// the two share. It is a network alias and not a container name, so nothing
	// depends on testcontainers' generated names.
	upstreamAlias = "lazyslice-upstream"
)

// PgBouncer starts a Postgres server and a PgBouncer in front of it, both on a
// Docker network of their own, and returns two connection URLs: the pooled one,
// which points at PgBouncer, and the direct one, which points at the server.
//
// Load a fixture through the direct URL and open the source through the pooled
// one: this pooler runs in transaction pooling mode, the mode that breaks the
// most assumptions and the mode research/OPEN_QUESTIONS.md item 1 is about, and
// the fixture loader speaks psql constructs a pooler has no reason to survive.
//
// Both containers and the network are torn down when the test finishes,
// including when it fails.
//
// image is the PgBouncer image; pass "" for $LAZYSLICE_TEST_PGBOUNCER_IMAGE, or
// PgBouncerImage when that is unset. The server image is chosen exactly as
// Postgres chooses it, so the CI matrix moves the server under the pooler
// without this function knowing about the matrix.
//
// settings are extra environment entries for the pooler container, merged over
// the defaults below in the order given. The edoburu image writes each one into
// the pgbouncer.ini it generates, so `MAX_DB_CONNECTIONS`, `DEFAULT_POOL_SIZE`
// and `QUERY_WAIT_TIMEOUT` are how a test asks for a pooler that runs out of
// server connections — the restrictive configuration ADR-005's serialised
// extract exists for. A caller may override a default, including POOL_MODE;
// what it cannot override is DATABASE_URL, which is this function's own wiring
// to the server it started.
//
// Both returned URLs carry a password, so both are a dsn.DSN by nature: hand
// them to a driver, and do not put either in an event, a config or a test name.
func PgBouncer(ctx context.Context, t *testing.T, image string, settings ...map[string]string) (pooled, direct string) {
	t.Helper()

	if image == "" {
		image = os.Getenv(PgBouncerImageEnv)
	}
	if image == "" {
		image = PgBouncerImage
	}

	net, err := network.New(ctx)
	if err != nil {
		t.Fatalf("testutil: creating the network the pooler and the server share: %v", err)
	}
	t.Cleanup(func() {
		stop, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
		defer cancel()
		if rmErr := net.Remove(stop); rmErr != nil {
			t.Logf("testutil: removing the pooler network: %v", rmErr)
		}
	})

	direct = postgresContainer(ctx, t, "", network.WithNetwork([]string{upstreamAlias}, net))

	// The pooler's own view of the server is the network alias and the
	// container port, never the host-mapped endpoint the test uses: inside the
	// network there is no port mapping to see.
	upstream := url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(user, password),
		Host:   upstreamAlias + ":5432",
		Path:   "/" + database,
	}

	env := map[string]string{
		// Transaction pooling: the mode a pooled source endpoint is
		// actually deployed in, and the one that cannot promise a session.
		"POOL_MODE":   "transaction",
		"LISTEN_PORT": pgBouncerPort,
		// plain, so the image writes the password into its userlist as
		// itself and the pooler can authenticate to the server in whatever
		// method the server asks for. Nothing here leaves the test's own
		// Docker network.
		"AUTH_TYPE": "plain",
		// IGNORE_STARTUP_PARAMETERS is deliberately *not* set. The image's
		// default is `extra_float_digits` and nothing else, which is the
		// stock PgBouncer behaviour this suite needs to meet: a startup
		// parameter of ours would be refused here exactly as it would be on
		// an operator's pooler, which is the whole point of the connection
		// being made through this container rather than asserted about.
	}
	for _, extra := range settings {
		for k, v := range extra {
			env[k] = v
		}
	}
	// Last, and after the caller's settings: the pooler's own view of the
	// server is this function's wiring and not a test's to redirect.
	env["DATABASE_URL"] = upstream.String()

	opts := []testcontainers.ContainerCustomizer{
		testcontainers.WithEnv(env),
		testcontainers.WithExposedPorts(pgBouncerPort + "/tcp"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("process up: PgBouncer").WithStartupTimeout(startupTimeout),
		),
		network.WithNetwork([]string{"lazyslice-pooler"}, net),
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
	// Registered before the error is checked, and nil-safe, for the reason
	// postgres.go gives: testcontainers.Run returns the container alongside its
	// error so that one which started and failed its wait strategy can still be
	// terminated.
	t.Cleanup(func() {
		stop, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
		defer cancel()
		if termErr := testcontainers.TerminateContainer(ctr, testcontainers.StopContext(stop)); termErr != nil {
			t.Logf("testutil: terminating %s: %v", image, termErr)
		}
	})
	if err != nil {
		t.Fatalf("testutil: starting %s: %v", image, err)
	}

	endpoint, err := portEndpointWithRetry(ctx, ctr, pgBouncerPort+"/tcp", "", time.Sleep)
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
	return u.String(), direct
}
