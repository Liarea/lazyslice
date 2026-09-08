// SPDX-License-Identifier: Apache-2.0

package provision

import (
	"context"
	"errors"
	"io"
	"iter"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/image"
	"github.com/moby/moby/api/types/jsonstream"
	"github.com/moby/moby/api/types/mount"
	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/api/types/volume"
	"github.com/moby/moby/client"
)

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
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("password file is %v, want 0o600", info.Mode().Perm())
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
// instead of a daemon. It has no remove method for the same reason Docker does
// in this package's interface: there is no code path here that removes
// anything.
type fakeDaemon struct {
	images     []string
	byID       map[string]container.InspectResponse
	running    map[string]bool
	starts     map[string]bool
	created    client.ContainerCreateOptions
	volume     string
	volumes    map[string]bool
	pulled     string
	nextID     int
	createFail error
}

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

func (d *fakeDaemon) ContainerInspect(_ context.Context, id string, _ client.ContainerInspectOptions) (client.ContainerInspectResult, error) {
	c, ok := d.byID[id]
	if !ok {
		return client.ContainerInspectResult{}, errors.New("no such container")
	}
	c.State = &container.State{Running: d.running[id]}
	return client.ContainerInspectResult{Container: c}, nil
}

func (d *fakeDaemon) ContainerCreate(_ context.Context, options client.ContainerCreateOptions) (client.ContainerCreateResult, error) {
	if d.createFail != nil {
		return client.ContainerCreateResult{}, d.createFail
	}
	d.created = options
	d.nextID++
	id := "c" + strconv.Itoa(d.nextID)
	binding := options.HostConfig.PortBindings[network.MustParsePort("5432/tcp")]
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
	if _, ok := d.byID[id]; !ok {
		return client.ContainerStartResult{}, errors.New("no such container")
	}
	d.starts[id] = true
	d.running[id] = true
	return client.ContainerStartResult{}, nil
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
