// SPDX-License-Identifier: Apache-2.0

// Package provision is the --create-target path (ARCHITECTURE.md section 9
// "Provisioning"). It is the only code in lazyslice that creates or starts a
// container, and it is never called without --create-target or a "yes" to Q1
// or Q1' (ADR-008 section 6).
//
// It resolves nothing about the Docker endpoint itself: Dial takes the endpoint
// internal/discover already resolved through dockerctx, and ADR-008 section 3
// makes "the endpoint is local and reachable" the caller's precondition, which
// internal/discover checks before it ever builds a provisioner. A container
// created on someone else's daemon publishes on that host's interfaces and is
// not local (THREAT_MODEL.md T2).
//
// The container survives the run: it is the developer's local database from
// then on, and the next run finds it at ladder rung 3 with the marker, or at
// rung 0 with the credential this package remembered (Password). lazyslice
// never removes a container or a volume itself. The pair of commands that would
// is "docker rm -f lazyslice-target-<project> && docker volume rm
// lazyslice-target-<project>-data": "docker rm -v" removes anonymous volumes
// only, so on its own it leaves the cluster behind and the recreated container
// comes back with the same rows and the same password (see this package's
// CLAUDE.md; the refusal text itself lives in internal/pg).
package provision

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/mount"
	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/client"

	"github.com/Liarea/lazyslice/internal/config"
	"github.com/Liarea/lazyslice/internal/discover/dockerctx"
	"github.com/Liarea/lazyslice/internal/dsn"
	"github.com/Liarea/lazyslice/internal/pipeline"
)

const (
	// namePrefix is ARCHITECTURE.md section 9's container name: the compose
	// project name is what makes one developer's two checkouts two containers.
	namePrefix = "lazyslice-target-"
	// volumeSuffix names the volume that holds the cluster. ARCHITECTURE.md
	// section 9 says a *named* volume, so the data survives a docker stop and a
	// second run finds the marker where it left it.
	volumeSuffix = "-data"
	// dataDir is where the postgres image keeps PGDATA.
	dataDir = "/var/lib/postgresql/data"

	// postgresPort is the port inside the container's network namespace.
	postgresPort = "5432/tcp"

	// firstPort is where ARCHITECTURE.md section 9 starts looking for a free
	// loopback port, and lastPort is where this file gives up rather than
	// scanning the ephemeral range.
	firstPort = 5433
	lastPort  = 5632

	// readyBudget is the 60 s ARCHITECTURE.md section 9 and ADR-008 section 6
	// both give the container to accept a connection, and readyInterval is how
	// often it is asked.
	readyBudget   = 60 * time.Second
	readyInterval = 500 * time.Millisecond

	// role and database are the container's own POSTGRES_USER and POSTGRES_DB.
	// They are written into the container environment rather than left implicit
	// so that the next run's rung 3 reads them back from the same place it
	// reads every other container's (internal/discover/containers.go).
	role     = "postgres"
	database = "postgres"
)

// The labels a provisioned container carries. They are not compose labels and
// must never be spelled as compose labels: rung 3's working_dir filter reads
// com.docker.compose.project.working_dir, and a container that claimed to be
// part of the developer's compose project would appear inside `docker compose
// ps` output that no compose file describes.
const (
	// LabelProject is the compose project name the container was created for.
	LabelProject = "lazyslice.project"
	// LabelWorkingDir is the directory the run was made from, so that rung 3's
	// project filter can keep this container when the cwd matches even though
	// it belongs to no compose project.
	LabelWorkingDir = "lazyslice.working_dir"
)

// Name is the container ARCHITECTURE.md section 9 names for a project. It is
// exported because the questions and refusals that mention the container are
// written outside this package and must not spell the name a second way.
func Name(project string) string { return namePrefix + project }

// Volume is the named volume that holds the cluster for a project.
func Volume(project string) string { return Name(project) + volumeSuffix }

// Request is what provisioning needs from the ladder.
type Request struct {
	// Project is the compose project name; it names the container and the
	// volume.
	Project string
	// Workdir is the directory the run was made from. It is stamped on the
	// container as LabelWorkingDir so the next run recognises it as this
	// project's even though it belongs to no compose project.
	Workdir string
	// Major is the source server's major version: ARCHITECTURE.md section 9
	// creates postgres:<source major> so that a dump-shaped mismatch cannot
	// happen. Zero is a caller that did not probe its source, and is refused
	// rather than guessed at.
	Major int
	// Port is the loopback port the container publishes 5432 on. Zero asks
	// FreePort for one; a caller that has already named the port in Q1's
	// prompt passes the value it named, so the question and the container
	// cannot disagree.
	Port int
	// Progress, when non-nil, receives the one-line notes a human needs while
	// an image is pulled and a server starts — the two waits in this package
	// that can outlast a developer's patience. ARCHITECTURE.md section 9 asks
	// for the pull to be printed and internal/event/catalogue.yml has no row
	// for it yet, so the writer is passed in rather than a sentence being
	// rendered into an event with no template (see this package's CLAUDE.md).
	Progress io.Writer
}

// Result is what a provision or a start produced.
//
// It is the widening ADR-008 and tracker T-0063 asked for: pipeline.Provisioner
// returns a pipeline.Candidate, whose Ref is redacted by construction and
// therefore cannot carry the POSTGRES_PASSWORD this package generates, while
// the run has to connect with it. DSN is that credential: hand it to the run
// and to nothing else (THREAT_MODEL.md A3).
type Result struct {
	// Candidate is the container as the ladder prints it. Nothing on it is a
	// credential.
	Candidate pipeline.Candidate
	// DSN is the connection string, password included.
	DSN string
	// Container is the container's name, for the messages that name it.
	Container string
	// Created is true when this call created the container, and false when it
	// adopted one that already existed. ARCHITECTURE.md section 9: a second
	// --create-target starts a stopped container and reuses a running one.
	Created bool
}

// Provisioner creates and starts the target container.
//
// Nothing here takes an event.Sink: this package emits no event of its own.
// The provisioned container reaches the candidate list through
// internal/discover, which prints it with discover.candidate.found like any
// other candidate, and the two lines a human needs while an image is pulled go
// to Request.Progress.
//
// It is internal/discover's own interface rather than pipeline.Provisioner,
// which is declared in internal/pipeline and could not be widened from this
// task's paths; see this package's CLAUDE.md.
type Provisioner interface {
	// Provision is ARCHITECTURE.md section 9 "Provisioning": pull
	// postgres:<major> if absent, create lazyslice-target-<project> on a free
	// loopback port with a random POSTGRES_PASSWORD and a named volume, start
	// it, and wait up to 60 s for it to accept a connection. A container of
	// that name that already exists is started if stopped and reused if
	// running; it is never recreated and never removed.
	Provision(ctx context.Context, req Request) (Result, error)
	// Start is ADR-008 section 6's Q1': the only target-shaped candidate is a
	// stopped container, and the answer was yes. It starts that container by
	// ID and waits for it on the same 60 s budget. It creates nothing.
	Start(ctx context.Context, containerID string, req Request) (Result, error)
}

// NotReadyError is the 60 s budget running out (ADR-008 section 6 step 3). The
// container is left running; lazyslice never removes one.
type NotReadyError struct {
	Container string
	Waited    time.Duration
	Err       error
}

func (e *NotReadyError) Error() string {
	return fmt.Sprintf("provision: %s did not accept a connection within %s", e.Container, e.Waited)
}

func (e *NotReadyError) Unwrap() error { return e.Err }

// Docker is the part of the Docker client this package uses.
//
// It is declared here rather than shared with internal/discover on purpose:
// that package's dockerAPI carries only Ping, ContainerList and
// ContainerInspect, so no code outside this one can reach a method that writes
// (internal/discover/CLAUDE.md).
type Docker interface {
	ContainerList(ctx context.Context, options client.ContainerListOptions) (client.ContainerListResult, error)
	ContainerInspect(ctx context.Context, id string, options client.ContainerInspectOptions) (client.ContainerInspectResult, error)
	ContainerCreate(ctx context.Context, options client.ContainerCreateOptions) (client.ContainerCreateResult, error)
	ContainerStart(ctx context.Context, id string, options client.ContainerStartOptions) (client.ContainerStartResult, error)
	ImageList(ctx context.Context, options client.ImageListOptions) (client.ImageListResult, error)
	ImagePull(ctx context.Context, ref string, options client.ImagePullOptions) (client.ImagePullResponse, error)
	VolumeCreate(ctx context.Context, options client.VolumeCreateOptions) (client.VolumeCreateResult, error)
	// VolumeInspect answers "does this project's data volume already exist".
	// It is here and not only VolumeCreate because VolumeCreate on a name that
	// exists returns the existing volume rather than an error, so a create
	// path that only ever called it could not tell a fresh cluster from one
	// that already has a password.
	VolumeInspect(ctx context.Context, volumeID string, options client.VolumeInspectOptions) (client.VolumeInspectResult, error)
}

type provisioner struct {
	api Docker
	// ready dials the provisioned endpoint. It is a field so that the unit
	// tests can create a container against a fake daemon without a server to
	// connect to; every other caller gets the real handshake.
	ready func(ctx context.Context, connURL string) error
}

// New returns a provisioner talking to an already-built Docker client.
func New(api Docker) Provisioner { return &provisioner{api: api, ready: ready} }

// Dial builds the write-capable Docker client for an endpoint internal/discover
// has already resolved and already established is local and reachable (ADR-008
// section 3).
func Dial(e dockerctx.Endpoint) (Provisioner, error) {
	c, err := dockerctx.Dial(e)
	if err != nil {
		return nil, fmt.Errorf("provision: opening the docker endpoint: %w", err)
	}
	return New(c), nil
}

var _ Provisioner = (*provisioner)(nil)

// Provision is ARCHITECTURE.md section 9 "Provisioning".
func (p *provisioner) Provision(ctx context.Context, req Request) (Result, error) {
	name := Name(req.Project)

	existing, err := p.byName(ctx, name)
	if err != nil {
		return Result{}, err
	}
	if existing != nil {
		// ARCHITECTURE.md section 9: a second --create-target with the
		// container already present starts it if stopped and reuses it if
		// running. It is never recreated, because recreating it would discard
		// the volume the developer's previous snapshot is in.
		return p.adopt(ctx, req, existing.ID, name, false)
	}
	return p.create(ctx, req, name)
}

// Start is ADR-008 section 6's Q1': start a container that already exists,
// create nothing.
func (p *provisioner) Start(ctx context.Context, containerID string, req Request) (Result, error) {
	return p.adopt(ctx, req, containerID, containerID, false)
}

// adopt starts a container that already exists if it is not running, waits for
// it, and builds the candidate.
func (p *provisioner) adopt(ctx context.Context, req Request, id, name string, created bool) (Result, error) {
	insp, err := p.api.ContainerInspect(ctx, id, client.ContainerInspectOptions{})
	if err != nil {
		return Result{}, fmt.Errorf("provision: inspecting %s: %w", name, err)
	}
	if insp.Container.State == nil || !insp.Container.State.Running {
		note(req.Progress, "lazyslice: starting "+displayName(insp, name))
		if _, startErr := p.api.ContainerStart(ctx, id, client.ContainerStartOptions{}); startErr != nil {
			return Result{}, fmt.Errorf("provision: starting %s: %w", name, startErr)
		}
		// The published port and the state are both only true after the start,
		// so the inspection is redone rather than reused.
		insp, err = p.api.ContainerInspect(ctx, id, client.ContainerInspectOptions{})
		if err != nil {
			return Result{}, fmt.Errorf("provision: inspecting %s after starting it: %w", name, err)
		}
	}
	return p.settle(ctx, req, insp, displayName(insp, name), created)
}

// create is the whole of ARCHITECTURE.md section 9's create path.
func (p *provisioner) create(ctx context.Context, req Request, name string) (Result, error) {
	if req.Major <= 0 {
		return Result{}, errors.New("provision: the source server major is unknown, so postgres:<major> cannot be named")
	}
	image := "postgres:" + strconv.Itoa(req.Major)

	port := req.Port
	if port == 0 {
		var err error
		if port, err = FreePort(); err != nil {
			return Result{}, err
		}
	}
	volume := Volume(req.Project)
	secret, err := p.secretFor(ctx, name, volume)
	if err != nil {
		return Result{}, err
	}

	if pullErr := p.pull(ctx, image, req.Progress); pullErr != nil {
		return Result{}, pullErr
	}

	if _, volErr := p.api.VolumeCreate(ctx, client.VolumeCreateOptions{
		Name:   volume,
		Labels: labels(req),
	}); volErr != nil {
		return Result{}, fmt.Errorf("provision: creating the volume %s: %w", volume, volErr)
	}

	exposed, err := network.ParsePort(postgresPort)
	if err != nil {
		return Result{}, fmt.Errorf("provision: %w", err)
	}
	created, err := p.api.ContainerCreate(ctx, client.ContainerCreateOptions{
		Name: name,
		Config: &container.Config{
			Image: image,
			Env: []string{
				"POSTGRES_USER=" + role,
				"POSTGRES_DB=" + database,
				"POSTGRES_PASSWORD=" + secret,
			},
			Labels:       labels(req),
			ExposedPorts: network.PortSet{exposed: struct{}{}},
		},
		HostConfig: &container.HostConfig{
			// Bound to loopback and to nothing else. A container published on
			// 0.0.0.0 is a database on every interface of the machine, which is
			// the opposite of the locality THREAT_MODEL.md T2 asks of a target.
			PortBindings: network.PortMap{exposed: []network.PortBinding{{
				HostIP:   netip.MustParseAddr("127.0.0.1"),
				HostPort: strconv.Itoa(port),
			}}},
			Mounts: []mount.Mount{{
				Type:   mount.TypeVolume,
				Source: volume,
				Target: dataDir,
			}},
		},
	})
	if err != nil {
		return Result{}, fmt.Errorf("provision: creating %s: %w", name, err)
	}
	note(req.Progress, "lazyslice: created "+name+" from "+image+" on 127.0.0.1:"+strconv.Itoa(port))

	return p.adopt(ctx, req, created.ID, name, true)
}

// secretFor is the POSTGRES_PASSWORD the container is created with, and the
// order of its three cases is the whole of it.
//
// The postgres image runs initdb only on an *empty* PGDATA; on a data directory
// that already holds a cluster it skips initdb and ignores POSTGRES_PASSWORD
// entirely. ARCHITECTURE.md section 9 asks for a *named* volume, and "docker rm
// -v <container>" does not remove one, so a developer who removes the container
// and runs --create-target again lands here with the cluster still on disk.
// Minting a fresh password there produced a container whose environment
// disagreed with its own data directory: every readiness attempt failed
// authentication for 60 s, target.refused.start_timeout, and the same on every
// retry — and overwriting the state-dir file destroyed the one credential that
// did work, which left the volume unreachable by anything lazyslice offers.
//
//  1. A password already remembered for this container name is reused, and the
//     file is not rewritten. It is the only one a surviving cluster accepts,
//     and on a volume that does not survive it is simply what initdb is given.
//  2. Nothing remembered and no volume: a fresh password, written to the
//     machine-local state dir *before* the container exists, so a state dir
//     that cannot be written is a refusal with nothing created rather than a
//     container whose credential was lost between two statements.
//  3. Nothing remembered and a volume that already exists: a refusal naming the
//     command that removes it. lazyslice never removes a volume itself, and a
//     password it could invent is one the cluster will not accept.
func (p *provisioner) secretFor(ctx context.Context, name, volume string) (string, error) {
	if secret, ok := Password(name); ok {
		return secret, nil
	}
	if p.volumeExists(ctx, volume) {
		return "", fmt.Errorf(
			"provision: the volume %s already holds a cluster and no password for it is remembered, "+
				"so a new container could not authenticate against it: remove it with `docker volume rm %s` and run again",
			volume, volume)
	}
	secret, err := newPassword()
	if err != nil {
		return "", err
	}
	if err := remember(name, secret); err != nil {
		return "", err
	}
	return secret, nil
}

// volumeExists reports whether the daemon already has this named volume.
//
// Only a successful inspection counts as "it is there": a daemon that cannot
// answer is not evidence of a cluster, and the caller's other branch — mint and
// remember — is what this run would have done before the volume question was
// asked at all.
func (p *provisioner) volumeExists(ctx context.Context, name string) bool {
	_, err := p.api.VolumeInspect(ctx, name, client.VolumeInspectOptions{})
	return err == nil
}

// settle turns a running container into the candidate and the connection string
// the run uses, once it answers.
func (p *provisioner) settle(ctx context.Context, req Request, insp client.ContainerInspectResult, name string, created bool) (Result, error) {
	host, port, ok := published(insp)
	if !ok {
		return Result{}, fmt.Errorf("provision: %s publishes no host port for %s", name, postgresPort)
	}
	user, db, secret := Credentials(EnvMap(configEnv(insp)))
	connURL := ConnString(host, port, user, db, secret)

	note(req.Progress, "lazyslice: waiting for "+name+" to accept connections")
	started := time.Now()
	if err := p.wait(ctx, connURL); err != nil {
		return Result{}, &NotReadyError{Container: name, Waited: time.Since(started), Err: err}
	}

	d, ref, err := dsn.Parse(connURL)
	if err != nil {
		return Result{}, fmt.Errorf("provision: the endpoint of %s does not parse: %w", name, err)
	}
	return Result{
		Candidate: pipeline.Candidate{
			Ref:        ref,
			Provenance: pipeline.FromContainer,
			Label:      name,
			// A container this daemon runs is on this machine, and the daemon
			// being local is the caller's precondition (ADR-008 section 3).
			Local: true,
		},
		DSN:       string(d),
		Container: name,
		Created:   created,
	}, nil
}

// wait polls until the server answers a real startup handshake, which is what
// pg_isready reports and what a TCP connect does not: docker-proxy binds the
// host port the moment the container starts, and the postgres image runs initdb
// against a unix socket for several seconds after that.
func (p *provisioner) wait(ctx context.Context, connURL string) error {
	deadline := time.Now().Add(readyBudget)
	var last error
	for {
		attempt, cancel := context.WithTimeout(ctx, readyInterval*4)
		last = p.ready(attempt, connURL)
		cancel()
		if last == nil {
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if time.Now().After(deadline) {
			return last
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(readyInterval):
		}
	}
}

// ready is one pg_isready: the startup handshake and nothing else. No statement
// is sent, so there is no allowlist to register it against (THREAT_MODEL.md T9
// covers the statements a run sends, and this sends none).
func ready(ctx context.Context, connURL string) error {
	cfg, err := pgconn.ParseConfig(connURL)
	if err != nil {
		return fmt.Errorf("provision: parsing the provisioned endpoint: %w", err)
	}
	conn, err := pgconn.ConnectConfig(ctx, cfg)
	if err != nil {
		return err
	}
	return conn.Close(ctx)
}

// pull fetches postgres:<major> when the daemon does not already have it.
func (p *provisioner) pull(ctx context.Context, image string, progress io.Writer) error {
	have, err := p.api.ImageList(ctx, client.ImageListOptions{
		Filters: make(client.Filters).Add("reference", image),
	})
	if err == nil && len(have.Items) > 0 {
		return nil
	}
	note(progress, "lazyslice: pulling "+image)
	resp, err := p.api.ImagePull(ctx, image, client.ImagePullOptions{})
	if err != nil {
		return fmt.Errorf("provision: pulling %s: %w", image, err)
	}
	defer func() { _ = resp.Close() }()
	if err := resp.Wait(ctx); err != nil {
		return fmt.Errorf("provision: pulling %s: %w", image, err)
	}
	note(progress, "lazyslice: pulled "+image)
	return nil
}

// byName finds the container ARCHITECTURE.md section 9 names for this project,
// running or not. The name filter the daemon applies is a substring match, so
// the exact name is checked here.
func (p *provisioner) byName(ctx context.Context, name string) (*container.Summary, error) {
	list, err := p.api.ContainerList(ctx, client.ContainerListOptions{
		All:     true,
		Filters: make(client.Filters).Add("name", name),
	})
	if err != nil {
		return nil, fmt.Errorf("provision: listing containers: %w", err)
	}
	for i := range list.Items {
		for _, n := range list.Items[i].Names {
			if n == "/"+name || n == name {
				return &list.Items[i], nil
			}
		}
	}
	return nil, nil
}

func labels(req Request) map[string]string {
	return map[string]string{
		LabelProject:    req.Project,
		LabelWorkingDir: req.Workdir,
	}
}

// published is the host binding of 5432 as the daemon reports it after a start.
//
// A binding on 0.0.0.0 or :: is loopback-equivalent here for the same reason it
// is at rung 3 and only for that reason: the caller has already established
// that the daemon is this machine (ADR-008 sections 3 and 4).
func published(insp client.ContainerInspectResult) (host string, port int, ok bool) {
	if insp.Container.NetworkSettings == nil {
		return "", 0, false
	}
	want, err := network.ParsePort(postgresPort)
	if err != nil {
		return "", 0, false
	}
	for _, b := range insp.Container.NetworkSettings.Ports[want] {
		n, err := strconv.Atoi(b.HostPort)
		if err != nil || n == 0 {
			continue
		}
		addr := b.HostIP
		if !addr.IsValid() || addr.IsUnspecified() {
			addr = netip.MustParseAddr("127.0.0.1")
		}
		return addr.String(), n, true
	}
	return "", 0, false
}

func configEnv(insp client.ContainerInspectResult) []string {
	if insp.Container.Config == nil {
		return nil
	}
	return insp.Container.Config.Env
}

// EnvMap turns a container's KEY=VALUE environment into a map.
//
// It is exported, with Credentials and ConnString, so that internal/discover's
// rungs 3 and 4 and this package's own settle spell a container's endpoint one
// way. They live here rather than in internal/discover because the dependency
// runs that way — internal/discover imports this package and not the reverse.
func EnvMap(env []string) map[string]string {
	out := make(map[string]string, len(env))
	for _, e := range env {
		if name, v, found := strings.Cut(e, "="); found {
			out[name] = v
		}
	}
	return out
}

// Credentials is the role, database and password a Postgres container's
// environment names, with the postgres image's own defaults applied.
//
// POSTGRES_DB defaults to POSTGRES_USER and not to the literal "postgres":
// that is what the image does, and it is an ordinary compose setup. Getting it
// wrong pointed a run at a maintenance database that is empty, so it passed the
// gate, and the snapshot loaded into `postgres` instead of the developer's own
// database while the candidate line printed the right name.
func Credentials(env map[string]string) (user, db, password string) {
	user = value(env, "POSTGRES_USER", role)
	return user, value(env, "POSTGRES_DB", user), env["POSTGRES_PASSWORD"]
}

func value(env map[string]string, name, fallback string) string {
	if v := env[name]; v != "" {
		return v
	}
	return fallback
}

// displayName prefers the daemon's own name for the container, so that a Q1'
// start of somebody else's container is reported under the name they gave it.
func displayName(insp client.ContainerInspectResult, fallback string) string {
	if n := insp.Container.Name; n != "" {
		if n[0] == '/' {
			return n[1:]
		}
		return n
	}
	return fallback
}

// ConnString is the one place a container's endpoint becomes a connection
// string, so that a container this package started and a container the ladder
// merely listed cannot be spelled two ways.
func ConnString(host string, port int, user, db, secret string) string {
	u := url.URL{
		Scheme:   "postgres",
		Host:     net.JoinHostPort(host, strconv.Itoa(port)),
		Path:     "/" + db,
		RawQuery: "sslmode=disable",
	}
	if secret == "" {
		u.User = url.User(user)
	} else {
		u.User = url.UserPassword(user, secret)
	}
	return u.String()
}

// FreePort is ARCHITECTURE.md section 9's "a free loopback port from 5433
// upward". Binding and closing is the only portable way to ask, and the race it
// leaves is closed by the daemon refusing to publish a port that is taken.
//
// It is exported because Q1's prompt names the port before Provision runs
// (ADR-008 section 6); the value it names comes back as Request.Port.
func FreePort() (int, error) {
	for port := firstPort; port <= lastPort; port++ {
		l, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
		if err != nil {
			continue
		}
		if err := l.Close(); err != nil {
			continue
		}
		return port, nil
	}
	return 0, fmt.Errorf("provision: no free loopback port between %d and %d", firstPort, lastPort)
}

// newPassword is the random POSTGRES_PASSWORD of ARCHITECTURE.md section 9.
// 24 bytes from crypto/rand, in the URL alphabet so that it needs no escaping
// in the connection string it ends up in.
func newPassword() (string, error) {
	var b [24]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("provision: generating the container password: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b[:]), nil
}

// remember stores the password in the machine-local state dir, which
// ARCHITECTURE.md section 9 makes its only home: it is never written to
// lazyslice.yml, which is committed (ADR-004, THREAT_MODEL.md A3).
func remember(containerName, secret string) error {
	if !plainName(containerName) {
		return fmt.Errorf("provision: %q is not a container name this package stores a password under", containerName)
	}
	dir, err := config.StateDir()
	if err != nil {
		return fmt.Errorf("provision: %w", err)
	}
	dir = filepath.Join(dir, "targets")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("provision: creating %s: %w", dir, err)
	}
	path := filepath.Join(dir, containerName+".password")
	if err := os.WriteFile(path, []byte(secret+"\n"), 0o600); err != nil {
		return fmt.Errorf("provision: writing the container password: %w", err)
	}
	return nil
}

// Password reads back what remember stored, and reports whether there was
// anything to read.
//
// It is the reader ARCHITECTURE.md section 9's "the next run finds it" needs.
// lazyslice.yml records a reference and never a password (ADR-004), so a second
// run whose target comes off the committed file at rung 0 has no credential for
// a container this tool provisioned; without this it dialled the container with
// no password at all and stopped at exit 4 with "password authentication
// failed". The state dir is that credential's only home, and the container name
// is its key.
//
// The name is never taken from a committed file: callers compute it with Name
// from the project, and plainName refuses anything that is not a single path
// element, so a lazyslice.yml naming ../../id_rsa cannot become a file this
// reads.
//
// The value is a credential: hand it to the run and to nothing else
// (THREAT_MODEL.md A3).
func Password(containerName string) (string, bool) {
	if !plainName(containerName) {
		return "", false
	}
	dir, err := config.StateDir()
	if err != nil {
		return "", false
	}
	body, err := os.ReadFile(filepath.Join(dir, "targets", containerName+".password"))
	if err != nil {
		return "", false
	}
	secret := strings.TrimSpace(string(body))
	return secret, secret != ""
}

// plainName is "this is one path element and not a traversal".
func plainName(name string) bool {
	return name != "" && name != "." && name != ".." &&
		!strings.ContainsAny(name, `/\`) && filepath.Base(name) == name
}

// note writes one line for a human waiting on a pull or a start.
//
// ADR-008 section 7 already puts prompt text on this channel, and
// internal/event/catalogue.yml carries no row for a pull, so an event under a
// code with no template would render as a missing-row line rather than a
// sentence. When the catalogue gains those rows this becomes a sink send; see
// this package's CLAUDE.md.
func note(w io.Writer, line string) {
	if w == nil {
		return
	}
	if _, err := io.WriteString(w, line+"\n"); err != nil {
		// A progress line that cannot be written is not a reason to abandon a
		// container that is already half-created.
		return
	}
}
