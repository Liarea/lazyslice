// SPDX-License-Identifier: Apache-2.0

package provision

import (
	"context"
	"errors"
	"io"
	"iter"
	"net/netip"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/image"
	"github.com/moby/moby/api/types/jsonstream"
	"github.com/moby/moby/api/types/mount"
	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/api/types/volume"
	"github.com/moby/moby/client"
)

// T-0333: a project name that already starts with "lazyslice-" (or
// "lazyslice_") doesn't read twice in the container name — the leading
// segment is stripped once before namePrefix is added. A name that merely
// contains "lazyslice" elsewhere, or is that prefix alone, is untouched.
func TestNameStripsALeadingLazysliceSegment(t *testing.T) {
	for _, tc := range []struct{ project, want string }{
		{"shop", "lazyslice-target-shop"},
		{"lazyslice-dogfood3", "lazyslice-target-dogfood3"},
		{"lazyslice_dogfood3", "lazyslice-target-dogfood3"},
		{"my-lazyslice-fork", "lazyslice-target-my-lazyslice-fork"},
		{"lazyslice", "lazyslice-target-lazyslice"},
		{"lazyslice-", "lazyslice-target-lazyslice-"},
	} {
		if got := Name(tc.project); got != tc.want {
			t.Errorf("Name(%q) = %q, want %q", tc.project, got, tc.want)
		}
	}
}

// The whole of ARCHITECTURE.md section 9's create path, against a fake daemon:
// the image is pulled because it is absent, the volume is named, the binding is
// loopback and nothing else, the password is random and reaches both the
// container's environment and the returned connection string, and the state dir
// holds it at 0o600.
func TestProvisionCreatesTheContainerSection9Describes(t *testing.T) {
	state := stateDir(t)
	d := newFakeDaemon()
	p := &provisioner{api: d, ready: answersAt("")}

	res, err := p.Provision(t.Context(), Request{Project: "shop", Workdir: "/src/shop", Major: 16})
	if err != nil {
		t.Fatalf("Provision: %v", err)
	}

	if !res.Created {
		t.Error("Created is false on a container this call made")
	}
	if res.Container != "lazyslice-target-shop" {
		t.Errorf("container = %q, want lazyslice-target-shop", res.Container)
	}
	if d.pulled != "postgres:16" {
		t.Errorf("pulled %q, want postgres:<source major>", d.pulled)
	}
	if d.volume != "lazyslice-target-shop-data" {
		t.Errorf("volume = %q, want the named volume section 9 asks for", d.volume)
	}

	create := d.created
	if create.Name != "lazyslice-target-shop" {
		t.Errorf("name = %q, want lazyslice-target-shop", create.Name)
	}
	if create.Config.Labels[LabelWorkingDir] != "/src/shop" {
		t.Errorf("labels = %v, want the working directory so the next run's project filter keeps it", create.Config.Labels)
	}
	if _, isCompose := create.Config.Labels["com.docker.compose.project.working_dir"]; isCompose {
		t.Error("the container claims a compose project no compose file describes")
	}

	bindings := create.HostConfig.PortBindings[network.MustParsePort("5432/tcp")]
	if len(bindings) != 1 {
		t.Fatalf("bindings = %v, want exactly one", bindings)
	}
	if bindings[0].HostIP.String() != "127.0.0.1" {
		t.Errorf("published on %s, want 127.0.0.1: a target on every interface is not a local target", bindings[0].HostIP)
	}
	port, err := strconv.Atoi(bindings[0].HostPort)
	if err != nil || port < firstPort {
		t.Errorf("host port = %q, want a free loopback port from %d upward", bindings[0].HostPort, firstPort)
	}

	if len(create.HostConfig.Mounts) != 1 || create.HostConfig.Mounts[0].Type != mount.TypeVolume ||
		create.HostConfig.Mounts[0].Source != "lazyslice-target-shop-data" ||
		create.HostConfig.Mounts[0].Target != dataDir {
		t.Errorf("mounts = %+v, want the named volume on %s", create.HostConfig.Mounts, dataDir)
	}

	secret := env(create.Config.Env)["POSTGRES_PASSWORD"]
	if len(secret) < 20 {
		t.Fatalf("POSTGRES_PASSWORD = %q, want a random value", secret)
	}
	if !strings.Contains(res.DSN, secret) {
		t.Error("the generated password did not reach the connection string, so the run cannot connect")
	}
	if strings.Contains(res.Candidate.Ref.String(), secret) {
		t.Error("the password reached the redacted reference, which is printed (THREAT_MODEL.md T4)")
	}
	if !d.running["c1"] {
		t.Error("the container was created and never started")
	}

	path := filepath.Join(state, "lazyslice", "targets", "lazyslice-target-shop.password")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("the password was not stored in the machine-local state dir: %v", err)
	}
	if strings.TrimSpace(string(body)) != secret {
		t.Errorf("state dir holds %q, want the container's own password", strings.TrimSpace(string(body)))
	}
	// Asserted where the guarantee exists. Windows has no POSIX mode bits and
	// os.Chmod there sets only the read-only attribute, so a file written with
	// 0o600 stats as -rw-rw-rw- and this assertion tests the operating system
	// rather than this package. Restricting the file by ACL on Windows is a
	// task of its own; see this package's CLAUDE.md.
	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat: %v", err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Errorf("password file is %v, want 0o600", info.Mode().Perm())
		}
	}
}

// A self-chosen port that loses the race to another process (this package's
// CLAUDE.md, "the integration (14) leg failed on something else again") does
// not fail the run: create() removes the container that lost it and retries
// on the next free port.
func TestASelfChosenPortThatLosesTheRaceIsRetried(t *testing.T) {
	stateDir(t)
	d := newFakeDaemon()
	d.blockFirstCreatedPort = true
	p := &provisioner{api: d, ready: answersAt("")}

	res, err := p.Provision(t.Context(), Request{Project: "shop", Workdir: "/src/shop", Major: 16})
	if err != nil {
		t.Fatalf("Provision: %v", err)
	}
	if !res.Created {
		t.Error("Created is false on a container the retry made")
	}
	if len(d.removed) != 1 {
		t.Fatalf("removed = %v, want exactly the one container that lost the race", d.removed)
	}
	if d.removed[0] != "c1" {
		t.Errorf("removed %v, want the first attempt, c1", d.removed[0])
	}
	if !d.running["c2"] {
		t.Error("the retry's own container was never started")
	}
	if _, stillThere := d.byID["c1"]; stillThere {
		t.Error("the container that lost the race is still on the daemon")
	}
	// The surviving container's binding has to be a different port from the
	// first attempt's, or an implementation that removed the loser and
	// retried on the identical port — the naive retry this package's CLAUDE.md
	// says was tried and did not work — would pass this test unchanged.
	if len(d.createdPorts) != 2 {
		t.Fatalf("createdPorts = %v, want exactly the lost attempt and the retry", d.createdPorts)
	}
	if d.createdPorts[1] == d.createdPorts[0] {
		t.Errorf("retry bound port %d, want something other than the lost port %d", d.createdPorts[1], d.createdPorts[0])
	}
}

// Q1's prompt names a port before Provision runs (this package's CLAUDE.md,
// "Request.Port is chosen by the caller"), but that number is a proposal, not
// a contract: Q1's window between FreePort's probe and the container's actual
// bind — a human sitting on the prompt, then an image pull — is the longest of
// any create path, so a caller-named port that loses the race is retried onto
// a different one exactly like a self-chosen port, and the operator is told
// about the change through Request.Progress.
func TestACallerNamedPortThatLosesTheRaceIsRetried(t *testing.T) {
	stateDir(t)
	d := newFakeDaemon()
	d.allocatedPort = 5433
	var progress strings.Builder
	p := &provisioner{api: d, ready: answersAt("")}

	res, err := p.Provision(t.Context(), Request{
		Project: "shop", Workdir: "/src/shop", Major: 16, Port: 5433, Progress: &progress,
	})
	if err != nil {
		t.Fatalf("Provision: %v", err)
	}
	if !res.Created {
		t.Error("Created is false on a container the retry made")
	}
	if len(d.removed) != 1 || d.removed[0] != "c1" {
		t.Fatalf("removed = %v, want exactly the first attempt, c1", d.removed)
	}
	if !d.running["c2"] {
		t.Error("the retry's own container was never started")
	}
	if len(d.createdPorts) != 2 {
		t.Fatalf("createdPorts = %v, want exactly the lost attempt and the retry", d.createdPorts)
	}
	if d.createdPorts[1] == 5433 {
		t.Errorf("retry bound port %d, want something other than the caller-named 5433", d.createdPorts[1])
	}
	if !strings.Contains(progress.String(), "5433 was already taken, using") {
		t.Errorf("progress = %q, want the operator told the caller-named port moved", progress.String())
	}
}

// postgres:18 owns /var/lib/postgresql itself and its entrypoint exits 1 rather
// than start when anything is mounted at /var/lib/postgresql/data
// (docker-library/postgres#1259). Below 18 the mount point cannot move, because
// the volume outlives the container and a run that mounted it somewhere else
// would find an empty PGDATA beside the developer's rows.
func TestTheClusterIsMountedWhereTheImageKeepsIt(t *testing.T) {
	for _, tc := range []struct {
		major int
		want  string
	}{
		{major: 14, want: "/var/lib/postgresql/data"},
		{major: 17, want: "/var/lib/postgresql/data"},
		{major: 18, want: "/var/lib/postgresql"},
	} {
		t.Run(strconv.Itoa(tc.major), func(t *testing.T) {
			stateDir(t)
			d := newFakeDaemon()
			p := &provisioner{api: d, ready: answersAt("")}

			if _, err := p.Provision(t.Context(), Request{Project: "shop", Major: tc.major}); err != nil {
				t.Fatalf("Provision: %v", err)
			}
			mounts := d.created.HostConfig.Mounts
			if len(mounts) != 1 || mounts[0].Target != tc.want {
				t.Errorf("postgres:%d mounts %+v, want the volume on %s", tc.major, mounts, tc.want)
			}
		})
	}
}

// A container that dies during startup is a failure this run can name, not a
// budget to sit out. Before this, the entrypoint refusing the mount layout and
// a server that is merely slow were the same observation — nothing answers —
// and the whole postgres:18 leg of the CI matrix reported "did not accept a
// connection within 1m0s" about a container that had exited one second in
// (tracker T-0075).
func TestAContainerThatExitsDuringStartupSaysSo(t *testing.T) {
	stateDir(t)
	d := newFakeDaemon()
	d.exitCode = 1
	d.dieOnStart = true
	// The dial would succeed; what ends the wait is the container being gone.
	p := &provisioner{api: d, ready: answersAt("")}

	_, err := p.Provision(t.Context(), Request{Project: "shop", Major: 18})
	var notReady *NotReadyError
	if !errors.As(err, &notReady) {
		t.Fatalf("err = %v, want a NotReadyError", err)
	}
	if !strings.Contains(err.Error(), "exited with status 1") {
		t.Errorf("err = %v, want the exit status the container stopped with", err)
	}
	if !strings.Contains(err.Error(), "docker logs lazyslice-target-shop") {
		t.Errorf("err = %v, want the command ADR-008 section 6 step 3 names", err)
	}
}

// The log brings the first dial forward; it never decides that one is made at
// all. A server that does not print the readiness line where `docker logs` can
// see it — logging_collector on, log_destination csvlog or jsonlog, a
// non-English lc_messages, or simply somebody else's tuned postgresql.conf on
// the container Q1' starts — is a server that answers, and a wait that read the
// log as a precondition refused those runs after the whole budget without
// spending a single attempt.
func TestALogWithoutTheReadinessLineStillGetsDialled(t *testing.T) {
	stateDir(t)
	t.Setenv(ReadyBudgetEnv, "2s")
	d := newFakeDaemon()
	d.logs = "LOG:  Datenbanksystem ist bereit, um Verbindungen anzunehmen\n"
	dialled := 0
	p := &provisioner{api: d, ready: func(context.Context, string) error {
		dialled++
		return nil
	}}

	if _, err := p.Provision(t.Context(), Request{Project: "shop", Major: 16}); err != nil {
		t.Fatalf("Provision: %v", err)
	}
	if dialled == 0 {
		t.Error("no attempt was made: the readiness line, and not the server, decided whether to dial")
	}
}

// The readiness budget is only ever checked between polls, so every daemon call
// inside one is bounded on its own. The caller's context carries no deadline —
// internal/discover hands the run's context straight through and the moby
// client sets no HTTP timeout — so a daemon that accepts a request and never
// answers would make the budget unenforceable and the wait would block past it
// indefinitely, which is the hang the budget exists to prevent.
func TestADaemonThatNeverAnswersDoesNotOutlastTheBudget(t *testing.T) {
	stateDir(t)
	d := newFakeDaemon()
	d.add("existing", "lazyslice-target-shop", 5433, false, nil)
	d.blockInspect = true
	p := &provisioner{api: d, ready: func(context.Context, string) error { return errors.New("connection refused") }}

	done := make(chan error, 1)
	go func() {
		done <- p.wait(t.Context(), "postgres://postgres@127.0.0.1:5433/postgres?sslmode=disable",
			time.Now().Add(readyInterval), &bootWatch{p: p, id: "existing", name: "lazyslice-target-shop"})
	}()
	select {
	case err := <-done:
		if err == nil {
			t.Error("the wait succeeded against a server that never answered")
		}
	case <-time.After(30 * time.Second):
		t.Fatal("the wait outlasted its budget: an unbounded daemon call inside a poll")
	}
}

// The budget is 60 s, which is the number ARCHITECTURE.md section 9 and ADR-008
// section 6 both carry, and ReadyBudgetEnv is how a CI runner asks for longer.
// A value that does not parse, or that is not positive, is the default: a
// mistyped timeout must not be the thing that turns the wait off.
func TestReadyBudgetIsSixtySecondsUnlessTheEnvironmentSaysOtherwise(t *testing.T) {
	for _, tc := range []struct {
		set  bool
		env  string
		want time.Duration
	}{
		{want: defaultReadyBudget},
		{set: true, env: "", want: defaultReadyBudget},
		{set: true, env: "3m", want: 3 * time.Minute},
		{set: true, env: " 3m ", want: 3 * time.Minute},
		{set: true, env: "banana", want: defaultReadyBudget},
		{set: true, env: "0", want: defaultReadyBudget},
		{set: true, env: "-5s", want: defaultReadyBudget},
	} {
		t.Run(tc.env, func(t *testing.T) {
			// Unset rather than absent: the developer running the suite may
			// have the variable in their own environment.
			t.Setenv(ReadyBudgetEnv, "")
			if tc.set {
				t.Setenv(ReadyBudgetEnv, tc.env)
			} else if err := os.Unsetenv(ReadyBudgetEnv); err != nil {
				t.Fatalf("unsetting %s: %v", ReadyBudgetEnv, err)
			}
			if got := readyBudget(); got != tc.want {
				t.Errorf("readyBudget() = %s, want %s", got, tc.want)
			}
		})
	}
	if defaultReadyBudget != 60*time.Second {
		t.Errorf("the default is %s; ARCHITECTURE.md section 9 and ADR-008 section 6 say 60 s, and both are frozen",
			defaultReadyBudget)
	}
}

// A daemon that publishes 5432 on both families lists both bindings, and an
// IPv4-only publish — which is what 0.0.0.0 and an explicit 127.0.0.1 both are
// — is not reachable on ::1 at all: every attempt inside the budget is refused
// for the same reason. So the IPv4 binding is preferred whichever order the
// daemon lists them in, and an IPv6-only publish is still used.
func TestPublishedPrefersTheIPv4Binding(t *testing.T) {
	for _, tc := range []struct {
		name     string
		bindings []network.PortBinding
		wantHost string
	}{
		{
			name: "ipv6 first",
			bindings: []network.PortBinding{
				{HostIP: netip.MustParseAddr("::1"), HostPort: "5433"},
				{HostIP: netip.MustParseAddr("127.0.0.1"), HostPort: "5433"},
			},
			wantHost: "127.0.0.1",
		},
		{
			name: "ipv4 first",
			bindings: []network.PortBinding{
				{HostIP: netip.MustParseAddr("127.0.0.1"), HostPort: "5433"},
				{HostIP: netip.MustParseAddr("::1"), HostPort: "5433"},
			},
			wantHost: "127.0.0.1",
		},
		{
			name:     "ipv6 only",
			bindings: []network.PortBinding{{HostIP: netip.MustParseAddr("::1"), HostPort: "5433"}},
			wantHost: "::1",
		},
		{
			name:     "unspecified is loopback",
			bindings: []network.PortBinding{{HostIP: netip.MustParseAddr("0.0.0.0"), HostPort: "5433"}},
			wantHost: "127.0.0.1",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			insp := client.ContainerInspectResult{Container: container.InspectResponse{
				NetworkSettings: &container.NetworkSettings{Ports: network.PortMap{
					network.MustParsePort("5432/tcp"): tc.bindings,
				}},
			}}
			host, port, ok := published(insp)
			if !ok || port != 5433 {
				t.Fatalf("published = %q, %d, %v; want the binding the daemon reports", host, port, ok)
			}
			if host != tc.wantHost {
				t.Errorf("host = %q, want %q", host, tc.wantHost)
			}
			// An IPv6 literal has to survive into the connection string with
			// its brackets, or the port becomes part of the address.
			if strings.Contains(host, ":") &&
				!strings.Contains(ConnString(host, port, "postgres", "postgres", ""), "["+host+"]:5433") {
				t.Errorf("ConnString(%q) does not bracket the IPv6 literal", host)
			}
		})
	}
}

// ARCHITECTURE.md section 9: a second --create-target with the container already
// present starts it if stopped and reuses it if running. Neither recreates it,
// because recreating it would discard the volume the last snapshot is in.
func TestProvisionAdoptsAContainerThatAlreadyExists(t *testing.T) {
	for _, tc := range []struct {
		name      string
		running   bool
		wantStart bool
	}{
		{name: "stopped is started", running: false, wantStart: true},
		{name: "running is reused", running: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stateDir(t)
			d := newFakeDaemon()
			d.add("existing", "lazyslice-target-shop", 5433, tc.running,
				[]string{"POSTGRES_USER=postgres", "POSTGRES_DB=postgres", "POSTGRES_PASSWORD=kept"})
			p := &provisioner{api: d, ready: answersAt("")}

			res, err := p.Provision(t.Context(), Request{Project: "shop", Major: 16})
			if err != nil {
				t.Fatalf("Provision: %v", err)
			}
			if res.Created {
				t.Error("Created is true for a container that already existed")
			}
			if d.created.Name != "" {
				t.Errorf("created %q; a container that already exists is never recreated", d.created.Name)
			}
			if got := d.starts["existing"]; got != tc.wantStart {
				t.Errorf("started = %v, want %v", got, tc.wantStart)
			}
			if !strings.Contains(res.DSN, "kept") {
				t.Errorf("dsn = %q, want the password from the container's own environment", res.Candidate.Ref)
			}
			if res.Candidate.Ref.Port != 5433 {
				t.Errorf("ref = %s, want the port the container publishes", res.Candidate.Ref)
			}
		})
	}
}

// The image is pulled only when the daemon does not have it: a pull that
// contacts the registry on every run turns an offline laptop into a broken one.
func TestAPresentImageIsNotPulled(t *testing.T) {
	stateDir(t)
	d := newFakeDaemon()
	d.images = []string{"postgres:16"}
	p := &provisioner{api: d, ready: answersAt("")}

	if _, err := p.Provision(t.Context(), Request{Project: "shop", Major: 16}); err != nil {
		t.Fatalf("Provision: %v", err)
	}
	if d.pulled != "" {
		t.Errorf("pulled %q, want no pull when the image is already here", d.pulled)
	}
}

// ADR-008 section 6 step 3: the container was started and did not answer inside
// the budget, so the caller gets the timeout it maps to
// target.refused.start_timeout, and the container is left running.
func TestAContainerThatNeverAnswersIsATimeout(t *testing.T) {
	stateDir(t)
	d := newFakeDaemon()
	p := &provisioner{api: d, ready: func(context.Context, string) error { return errors.New("connection refused") }}
	// The budget is not waited out in a unit test: a cancelled context is the
	// same exit from the loop, and what is under test is which error comes back.
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	_, err := p.Provision(ctx, Request{Project: "shop", Major: 16})
	var notReady *NotReadyError
	if !errors.As(err, &notReady) {
		t.Fatalf("err = %v, want a NotReadyError", err)
	}
	if !d.running["c1"] {
		t.Error("the container was removed or stopped; lazyslice never removes a container")
	}
}

// The source major is never guessed: postgres:<major> is what keeps a load from
// failing halfway through against an older server.
func TestAnUnknownSourceMajorIsRefused(t *testing.T) {
	stateDir(t)
	d := newFakeDaemon()
	p := &provisioner{api: d, ready: answersAt("")}

	if _, err := p.Provision(t.Context(), Request{Project: "shop"}); err == nil {
		t.Fatal("want a refusal with no source major, got none")
	}
	if d.created.Name != "" {
		t.Error("a container was created without a major to name its image")
	}
}

// Start is Q1': it starts a container that already exists and creates nothing.
func TestStartCreatesNothing(t *testing.T) {
	stateDir(t)
	d := newFakeDaemon()
	d.add("db", "shop-db-1", 5440, false, []string{"POSTGRES_USER=app", "POSTGRES_DB=shop"})
	p := &provisioner{api: d, ready: answersAt("")}

	res, err := p.Start(t.Context(), "db", Request{Project: "shop"})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if !d.starts["db"] {
		t.Error("the container was not started")
	}
	if d.created.Name != "" || d.volume != "" || d.pulled != "" {
		t.Error("Q1' created something; it may only start a container the developer already has")
	}
	if res.Container != "shop-db-1" {
		t.Errorf("container = %q, want the daemon's own name for it", res.Container)
	}
	if res.Candidate.Ref.Database != "shop" || res.Candidate.Ref.User != "app" {
		t.Errorf("ref = %s, want the role and database from the container's environment", res.Candidate.Ref)
	}
}

// The postgres image defaults POSTGRES_DB to POSTGRES_USER, not to the literal
// "postgres", and a Q1' start of a developer's own container is where that
// matters: POSTGRES_USER=app with no POSTGRES_DB is an ordinary compose setup.
// Naming "postgres" there handed the run a maintenance database that is empty,
// so it passed the gate and the snapshot loaded somewhere the operator was
// never shown — while the candidate line printed app@host/app.
func TestStartDefaultsTheDatabaseToTheRole(t *testing.T) {
	stateDir(t)
	d := newFakeDaemon()
	d.add("db", "shop-db-1", 5440, false, []string{"POSTGRES_USER=app", "POSTGRES_PASSWORD=pw"})
	p := &provisioner{api: d, ready: answersAt("")}

	res, err := p.Start(t.Context(), "db", Request{Project: "shop"})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if res.Candidate.Ref.Database != "app" || res.Candidate.Ref.User != "app" {
		t.Errorf("ref = %s, want app@127.0.0.1:5440/app: POSTGRES_DB defaults to POSTGRES_USER", res.Candidate.Ref)
	}
	if !strings.Contains(res.DSN, "/app?") {
		t.Errorf("dsn names a database the candidate line did not: %q", redact(res.DSN))
	}
}

// A volume that survived the container is a cluster whose password is already
// fixed. The postgres image skips initdb on a non-empty PGDATA and ignores
// POSTGRES_PASSWORD, so a freshly minted one cannot authenticate; and
// overwriting the state-dir file with it destroyed the only credential that
// could. The remembered password is reused and the file is left alone.
func TestASurvivingVolumeKeepsItsRememberedPassword(t *testing.T) {
	state := stateDir(t)
	d := newFakeDaemon()
	d.volumes["lazyslice-target-shop-data"] = true
	path := writePassword(t, state, "lazyslice-target-shop", "the-one-that-works")
	p := &provisioner{api: d, ready: answersAt("")}

	res, err := p.Provision(t.Context(), Request{Project: "shop", Major: 16})
	if err != nil {
		t.Fatalf("Provision: %v", err)
	}
	if got := env(d.created.Config.Env)["POSTGRES_PASSWORD"]; got != "the-one-that-works" {
		t.Errorf("POSTGRES_PASSWORD = %q, want the password the surviving cluster was built with", got)
	}
	if !strings.Contains(res.DSN, "the-one-that-works") {
		t.Errorf("dsn = %q, want the remembered credential", redact(res.DSN))
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading the state dir: %v", err)
	}
	if strings.TrimSpace(string(body)) != "the-one-that-works" {
		t.Errorf("state dir now holds %q; the one credential that works was overwritten", strings.TrimSpace(string(body)))
	}
}

// A surviving volume with no password remembered for it is a cluster lazyslice
// cannot name a credential for. It refuses, naming the command that removes the
// volume — "docker rm -v <container>" does not, which is how the state arises —
// and creates nothing.
func TestASurvivingVolumeWithNoRememberedPasswordIsRefused(t *testing.T) {
	stateDir(t)
	d := newFakeDaemon()
	d.volumes["lazyslice-target-shop-data"] = true
	p := &provisioner{api: d, ready: answersAt("")}

	_, err := p.Provision(t.Context(), Request{Project: "shop", Major: 16})
	if err == nil {
		t.Fatal("want a refusal for a volume whose password is unknown, got none")
	}
	if !strings.Contains(err.Error(), "docker volume rm lazyslice-target-shop-data") {
		t.Errorf("err = %v, want it to name the command that removes the volume", err)
	}
	if d.created.Name != "" || d.pulled != "" {
		t.Error("a container was created, or an image pulled, on a path that must refuse first")
	}
}

// Password is the reader that makes a provisioned target usable on the second
// run, and it never turns a name from a committed file into a path.
func TestPasswordReadsBackWhatRememberWroteAndRefusesATraversal(t *testing.T) {
	stateDir(t)
	if err := remember("lazyslice-target-shop", "s3cret"); err != nil {
		t.Fatalf("remember: %v", err)
	}
	if got, ok := Password("lazyslice-target-shop"); !ok || got != "s3cret" {
		t.Errorf("Password = %q, %v; want the remembered credential", got, ok)
	}
	if _, ok := Password("lazyslice-target-absent"); ok {
		t.Error("Password answered for a container nothing was stored for")
	}
	for _, name := range []string{"../../id_rsa", "a/b", ".."} {
		if _, ok := Password(name); ok {
			t.Errorf("Password(%q) read a file outside the targets directory", name)
		}
		if err := remember(name, "x"); err == nil {
			t.Errorf("remember(%q) wrote outside the targets directory", name)
		}
	}
}

// FreePort answers with a port a listener can take, from 5433 upward: Q1 names
// it in the question before the container exists.
func TestFreePortIsInRangeAndBindable(t *testing.T) {
	port, err := FreePort()
	if err != nil {
		t.Fatalf("FreePort: %v", err)
	}
	if port < firstPort || port > lastPort {
		t.Errorf("port = %d, want between %d and %d", port, firstPort, lastPort)
	}
}

// freePortExcluding's skip is create()'s retry asking for a port other than
// the one it just lost: a scan that ignored skip would hand the retry back
// the exact port whose bind just failed, which is the naive retry this
// package's CLAUDE.md says was tried and did not work.
func TestFreePortExcludingSkipsTheGivenPorts(t *testing.T) {
	first, err := freePortExcluding(nil)
	if err != nil {
		t.Fatalf("freePortExcluding(nil): %v", err)
	}
	second, err := freePortExcluding(map[int]bool{first: true})
	if err != nil {
		t.Fatalf("freePortExcluding(skip first): %v", err)
	}
	if second == first {
		t.Errorf("second = %d, want anything other than the excluded port %d", second, first)
	}
	if second < firstPort || second > lastPort {
		t.Errorf("second = %d, want between %d and %d", second, firstPort, lastPort)
	}
}

// Two passwords in a row are not the same one, which is the whole of "random".
func TestPasswordsDiffer(t *testing.T) {
	a, err := newPassword()
	if err != nil {
		t.Fatalf("newPassword: %v", err)
	}
	b, err := newPassword()
	if err != nil {
		t.Fatalf("newPassword: %v", err)
	}
	if a == b {
		t.Error("two generated passwords are equal")
	}
	if strings.ContainsAny(a, ":/@?#") {
		t.Errorf("password %q carries a character that changes the meaning of a connection string", a)
	}
}

// ---------- helpers ----------

// stateDir points the machine-local state dir at a temporary directory, so a
// test never writes into the developer's own, and returns its root.
func stateDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", dir)
	return dir
}

// writePassword puts a remembered credential in the state dir the way a
// previous run's remember would have, and returns its path.
func writePassword(t *testing.T, state, containerName, secret string) string {
	t.Helper()
	dir := filepath.Join(state, "lazyslice", "targets")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("creating %s: %v", dir, err)
	}
	path := filepath.Join(dir, containerName+".password")
	if err := os.WriteFile(path, []byte(secret+"\n"), 0o600); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
	return path
}

// redact keeps a failure message from printing a password.
func redact(connURL string) string {
	at := strings.LastIndex(connURL, "@")
	if at < 0 {
		return connURL
	}
	return "postgres://…" + connURL[at:]
}

// answersAt is a readiness probe that succeeds, optionally only for one URL.
func answersAt(want string) func(context.Context, string) error {
	return func(_ context.Context, connURL string) error {
		if want != "" && connURL != want {
			return errors.New("not this one")
		}
		return nil
	}
}

func env(vars []string) map[string]string {
	out := map[string]string{}
	for _, v := range vars {
		if name, value, ok := strings.Cut(v, "="); ok {
			out[name] = value
		}
	}
	return out
}

// fakeDaemon is the part of the Docker API this package uses, with a map
// instead of a daemon. ContainerRemove exists only for create()'s own
// port-allocation retry, which removes a container it just created and that
// never started — never one this package adopted or started for real.
type fakeDaemon struct {
	images     []string
	byID       map[string]container.InspectResponse
	running    map[string]bool
	starts     map[string]bool
	created    client.ContainerCreateOptions
	volume     string
	volumes    map[string]bool
	pulled     string
	logs       string
	exitCode   int
	dieOnStart bool
	// blockInspect is a daemon that accepts the request and never answers it,
	// which is the shape the readiness budget has to survive.
	blockInspect bool
	nextID       int
	createFail   error
	// allocatedPort, when nonzero, makes ContainerStart fail with the daemon's
	// "port is already allocated" wording for whichever container is asking to
	// bind exactly that host port — modelling the collision on the port a
	// container actually requests rather than on a bare boolean, so a fake
	// that lets a naive retry (remove the loser, ask for the identical port
	// again) succeed unchanged cannot pass the tests that pin this. Cleared to
	// 0 on ContainerRemove of the container that lost it.
	allocatedPort int
	removed       []string
	// createdPorts is the host port each ContainerCreate call was asked to
	// bind, in order — d.created only ever holds the most recent call, which
	// is not enough to compare a retry's port against the attempt it replaced.
	createdPorts []int
	// blockFirstCreatedPort makes ContainerCreate set allocatedPort to
	// whichever host port its very first call is asked to bind, once — for
	// pinning create()'s own retry against a self-chosen port, whose value a
	// test cannot name in advance the way it names a caller-supplied one.
	blockFirstCreatedPort bool
}

// requestedPort is the host port a container's own HostConfig asked
// ContainerCreate to bind — the fact ContainerStart checks the collision
// against, so the fake models the daemon's real failure mode: a bind for a
// specific port, not a start of a specific container.
func requestedPort(c container.InspectResponse) int {
	if c.HostConfig == nil {
		return 0
	}
	for _, b := range c.HostConfig.PortBindings[network.MustParsePort("5432/tcp")] {
		if n, err := strconv.Atoi(b.HostPort); err == nil {
			return n
		}
	}
	return 0
}

// fakeLogs is what ContainerLogs hands back: the daemon's stream is an
// io.ReadCloser and nothing here needs to close anything.
type fakeLogs struct{ io.Reader }

func (fakeLogs) Close() error { return nil }

func newFakeDaemon() *fakeDaemon {
	return &fakeDaemon{
		byID:    map[string]container.InspectResponse{},
		running: map[string]bool{},
		starts:  map[string]bool{},
		volumes: map[string]bool{},
	}
}

// add registers a container that already exists on the daemon.
func (d *fakeDaemon) add(id, name string, port int, running bool, vars []string) {
	d.byID[id] = container.InspectResponse{
		ID:    id,
		Name:  "/" + name,
		State: &container.State{Running: running},
		Config: &container.Config{
			Env: vars,
		},
		HostConfig: &container.HostConfig{PortBindings: network.PortMap{
			network.MustParsePort("5432/tcp"): {{HostPort: strconv.Itoa(port)}},
		}},
		NetworkSettings: &container.NetworkSettings{Ports: network.PortMap{
			network.MustParsePort("5432/tcp"): {{HostPort: strconv.Itoa(port)}},
		}},
	}
	d.running[id] = running
}

func (d *fakeDaemon) ContainerList(_ context.Context, _ client.ContainerListOptions) (client.ContainerListResult, error) {
	var out []container.Summary
	for id, c := range d.byID {
		out = append(out, container.Summary{ID: id, Names: []string{c.Name}})
	}
	return client.ContainerListResult{Items: out}, nil
}

func (d *fakeDaemon) ContainerInspect(ctx context.Context, id string, _ client.ContainerInspectOptions) (client.ContainerInspectResult, error) {
	if d.blockInspect {
		<-ctx.Done()
		return client.ContainerInspectResult{}, ctx.Err()
	}
	c, ok := d.byID[id]
	if !ok {
		return client.ContainerInspectResult{}, errors.New("no such container")
	}
	c.State = &container.State{Running: d.running[id], Status: "running"}
	if !d.running[id] {
		c.State.Status = "exited"
		c.State.ExitCode = d.exitCode
	}
	return client.ContainerInspectResult{Container: c}, nil
}

// ContainerLogs is the entrypoint's own output, which the wait reads to learn
// that the server is up. logs is what a healthy postgres prints; a test that
// wants a container that never comes up sets it to something else.
func (d *fakeDaemon) ContainerLogs(_ context.Context, id string, _ client.ContainerLogsOptions) (client.ContainerLogsResult, error) {
	if _, ok := d.byID[id]; !ok {
		return nil, errors.New("no such container")
	}
	body := d.logs
	if body == "" {
		body = readyLine + "\n" + readyLine + "\n"
	}
	return fakeLogs{Reader: strings.NewReader(body)}, nil
}

func (d *fakeDaemon) ContainerCreate(_ context.Context, options client.ContainerCreateOptions) (client.ContainerCreateResult, error) {
	if d.createFail != nil {
		return client.ContainerCreateResult{}, d.createFail
	}
	d.created = options
	d.nextID++
	id := "c" + strconv.Itoa(d.nextID)
	binding := options.HostConfig.PortBindings[network.MustParsePort("5432/tcp")]
	if len(binding) > 0 {
		if n, err := strconv.Atoi(binding[0].HostPort); err == nil {
			d.createdPorts = append(d.createdPorts, n)
		}
	}
	if d.blockFirstCreatedPort {
		d.blockFirstCreatedPort = false
		if len(binding) > 0 {
			if n, err := strconv.Atoi(binding[0].HostPort); err == nil {
				d.allocatedPort = n
			}
		}
	}
	d.byID[id] = container.InspectResponse{
		ID:              id,
		Name:            "/" + options.Name,
		Config:          options.Config,
		HostConfig:      options.HostConfig,
		NetworkSettings: &container.NetworkSettings{Ports: network.PortMap{network.MustParsePort("5432/tcp"): binding}},
	}
	return client.ContainerCreateResult{ID: id}, nil
}

func (d *fakeDaemon) ContainerStart(_ context.Context, id string, _ client.ContainerStartOptions) (client.ContainerStartResult, error) {
	c, ok := d.byID[id]
	if !ok {
		return client.ContainerStartResult{}, errors.New("no such container")
	}
	if d.allocatedPort != 0 && requestedPort(c) == d.allocatedPort {
		return client.ContainerStartResult{}, errors.New("Bind for 127.0.0.1:" + strconv.Itoa(d.allocatedPort) + " failed: port is already allocated")
	}
	d.starts[id] = true
	d.running[id] = !d.dieOnStart
	return client.ContainerStartResult{}, nil
}

func (d *fakeDaemon) ContainerRemove(_ context.Context, id string, _ client.ContainerRemoveOptions) (client.ContainerRemoveResult, error) {
	c, ok := d.byID[id]
	if !ok {
		return client.ContainerRemoveResult{}, errors.New("no such container")
	}
	delete(d.byID, id)
	delete(d.running, id)
	d.removed = append(d.removed, id)
	// The next attempt gets to start only if it asks for a different port: the
	// collision is cleared here because removing the loser is what frees the
	// port on a real daemon, not because a container was removed at all.
	if requestedPort(c) == d.allocatedPort {
		d.allocatedPort = 0
	}
	return client.ContainerRemoveResult{}, nil
}

func (d *fakeDaemon) ImageList(_ context.Context, _ client.ImageListOptions) (client.ImageListResult, error) {
	var out []image.Summary
	for _, ref := range d.images {
		out = append(out, image.Summary{RepoTags: []string{ref}})
	}
	return client.ImageListResult{Items: out}, nil
}

func (d *fakeDaemon) ImagePull(_ context.Context, ref string, _ client.ImagePullOptions) (client.ImagePullResponse, error) {
	d.pulled = ref
	d.images = append(d.images, ref)
	return fakePull{}, nil
}

func (d *fakeDaemon) VolumeCreate(_ context.Context, options client.VolumeCreateOptions) (client.VolumeCreateResult, error) {
	d.volume = options.Name
	// The daemon returns the existing volume rather than an error, which is the
	// behaviour that made a fresh password unusable against a surviving
	// cluster.
	d.volumes[options.Name] = true
	return client.VolumeCreateResult{}, nil
}

func (d *fakeDaemon) VolumeInspect(_ context.Context, id string, _ client.VolumeInspectOptions) (client.VolumeInspectResult, error) {
	if !d.volumes[id] {
		return client.VolumeInspectResult{}, errors.New("Error: No such volume: " + id)
	}
	return client.VolumeInspectResult{Volume: volume.Volume{Name: id}}, nil
}

type fakePull struct{}

func (fakePull) Read([]byte) (int, error) { return 0, io.EOF }
func (fakePull) Close() error             { return nil }
func (fakePull) Wait(context.Context) error {
	return nil
}
func (fakePull) JSONMessages(context.Context) iter.Seq2[jsonstream.Message, error] {
	return func(func(jsonstream.Message, error) bool) {}
}
