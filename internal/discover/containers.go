// SPDX-License-Identifier: Apache-2.0

package discover

import (
	"context"
	"fmt"
	"net/netip"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/client"

	"github.com/Liarea/lazyslice/internal/discover/provision"
	"github.com/Liarea/lazyslice/internal/dsn"
	"github.com/Liarea/lazyslice/internal/pipeline"
)

// The compose labels rung 3 reads. Every connection-bearing fact comes off
// these and off the live port bindings, never off a compose file (ADR-008 §4).
const (
	labelService    = "com.docker.compose.service"
	labelWorkingDir = "com.docker.compose.project.working_dir"
	labelOneOff     = "com.docker.compose.oneoff"
)

// postgresPort is the port a Postgres container listens on inside its network
// namespace. A container publishing it is a Postgres container whatever its
// image is called, which is how postgis/ and timescale/ images are found.
const postgresPort = 5432

// dockerAPI is the part of the Docker client rung 3 uses. It is an interface so
// that de-duplication, naming and the working_dir filter can be tested without
// a daemon, and so that this package cannot reach a method that writes:
// creating or starting a container is internal/discover/provision's alone.
type dockerAPI interface {
	Ping(ctx context.Context, options client.PingOptions) (client.PingResult, error)
	ContainerList(ctx context.Context, options client.ContainerListOptions) (client.ContainerListResult, error)
	ContainerInspect(ctx context.Context, id string, options client.ContainerInspectOptions) (client.ContainerInspectResult, error)
}

// containers is rungs 3 and 4: running and exited Postgres containers on a
// local Docker endpoint.
//
// It lists with All: true so that both rungs come from the same call (ADR-008
// §4). A running container is dialled like any other candidate; an exited one
// carries Reachable false with "the container is stopped" as its reason, which
// is what makes it a rung-4 candidate rather than an eligible target: ADR-008
// §6 asks Q1' about it *before* the gate runs, because reachability is the
// precondition of every gate rule.
//
// local is the caller's answer to "is this endpoint local", and it is a
// parameter rather than a re-derivation because address normalisation is a
// function of (binding, endpointIsLocal) and it is a programming error to
// normalise a binding from a daemon that is not this machine (ADR-008 §4).
func containers(ctx context.Context, api dockerAPI, workdir string, local bool) (out []found, matchedProject bool, err error) {
	if !local {
		return nil, false, fmt.Errorf("discover: refusing to read containers from a non-local docker endpoint")
	}
	list, err := api.ContainerList(ctx, client.ContainerListOptions{All: true})
	if err != nil {
		return nil, false, fmt.Errorf("discover: listing containers: %w", err)
	}

	postgres := make([]container.Summary, 0, len(list.Items))
	for _, c := range list.Items {
		if c.Labels[labelOneOff] == "True" || c.Labels[labelOneOff] == "true" {
			continue
		}
		if !isPostgres(c) {
			continue
		}
		postgres = append(postgres, c)
	}

	inProject, matchedProject := filterToProject(postgres, workdir)
	for _, c := range inProject {
		build := candidateFor
		if c.State != container.StateRunning {
			build = stoppedCandidateFor
		}
		f, ok := build(ctx, api, c)
		if !ok {
			continue
		}
		out = append(out, f)
	}
	return out, matchedProject, nil
}

// isPostgres reports whether a container is a Postgres server: it publishes or
// exposes 5432, or its image says so. The port comes first because it is the
// fact that does not depend on a naming convention.
func isPostgres(c container.Summary) bool {
	for _, p := range c.Ports {
		if p.PrivatePort == postgresPort {
			return true
		}
	}
	return strings.Contains(strings.ToLower(c.Image), "postgres")
}

// filterToProject applies ARCHITECTURE.md §9's working_dir filter: the compose
// project whose working_dir label is the cwd or an ancestor, then any.
//
// A container this tool provisioned belongs to no compose project and carries
// provision.LabelWorkingDir instead, so the filter reads that label too. Without
// it, a developer whose project does have a compose Postgres service would have
// their own --create-target container filtered out of every later run.
//
// The "then any" fallback is why the caller prints which provenance it used
// (ADR-008 §4): the empty case is common, not exotic. matched says which of the
// two happened, so that the header can say so rather than being silent.
func filterToProject(cs []container.Summary, workdir string) (out []container.Summary, matched bool) {
	if workdir == "" {
		return cs, false
	}
	abs, err := filepath.Abs(workdir)
	if err != nil {
		return cs, false
	}
	for _, c := range cs {
		for _, label := range []string{labelWorkingDir, provision.LabelWorkingDir} {
			dir := c.Labels[label]
			if dir != "" && underOrEqual(abs, dir) {
				out = append(out, c)
				break
			}
		}
	}
	if len(out) == 0 {
		return cs, false
	}
	return out, true
}

// underOrEqual reports whether dir is the working directory or one of its
// ancestors, which is the relation "this container belongs to this project".
func underOrEqual(workdir, dir string) bool {
	rel, err := filepath.Rel(filepath.Clean(dir), workdir)
	if err != nil {
		return false
	}
	return rel == "." || !strings.HasPrefix(rel, "..")
}

// candidateFor builds one rung-3 candidate from a running container.
//
// The host and port come from the live published binding; the role, database
// and password come from the container's own environment, which THREAT_MODEL.md
// A3 names as one of the places a source credential lives. Nothing here reads a
// compose file: compose contributes a service *name* and nothing else.
func candidateFor(ctx context.Context, api dockerAPI, c container.Summary) (found, bool) {
	host, port, ok := binding(c)
	if !ok {
		return found{}, false
	}

	var env []string
	if insp, err := api.ContainerInspect(ctx, c.ID, client.ContainerInspectOptions{}); err == nil && insp.Container.Config != nil {
		env = insp.Container.Config.Env
	}
	user, database, password := provision.Credentials(provision.EnvMap(env))

	d, ref, err := dsn.Parse(provision.ConnString(host, port, user, database, password))
	if err != nil {
		return found{}, false
	}
	return found{
		dsn: d,
		cand: pipeline.Candidate{
			Ref:        ref,
			Provenance: pipeline.FromContainer,
			Label:      displayName(c),
			// Local is true for the reason ARCHITECTURE.md §2 gives — the
			// container runs on this machine — and never because a binding
			// string was rewritten (ADR-008 §4).
			Local: true,
		},
	}, true
}

// stoppedCandidateFor builds one rung-4 candidate from an exited container.
//
// It is not dialled and never can be: the reason on the line is "the container
// is stopped", which is what ADR-008 §6 asks Q1' about before the gate runs. The
// host binding comes from HostConfig.PortBindings rather than the summary's
// Ports, because a container that is not running publishes nothing; a container
// whose binding names no concrete host port is skipped, since neither this run
// nor the developer could reach it without starting it and asking again.
func stoppedCandidateFor(ctx context.Context, api dockerAPI, c container.Summary) (found, bool) {
	insp, err := api.ContainerInspect(ctx, c.ID, client.ContainerInspectOptions{})
	if err != nil {
		return found{}, false
	}
	host, port, ok := configuredBinding(insp)
	if !ok {
		return found{}, false
	}

	user, database, password := provision.Credentials(provision.EnvMap(configEnv(insp)))

	d, ref, err := dsn.Parse(provision.ConnString(host, port, user, database, password))
	if err != nil {
		return found{}, false
	}
	return found{
		dsn:         d,
		stopped:     true,
		containerID: c.ID,
		cand: pipeline.Candidate{
			Ref:        ref,
			Provenance: pipeline.FromStoppedContainer,
			Label:      displayName(c),
			Local:      true,
			ConnectErr: stoppedReason,
		},
	}, true
}

// stoppedReason is what the candidate list prints beside a rung-4 container. It
// is the reason a gate rule cannot be evaluated for it, and the reason Q1' is
// asked before the gate rather than after (ADR-008 §6).
const stoppedReason = "the container is stopped"

// configuredBinding is the host binding a stopped container is configured with.
//
// A HostIP of 0.0.0.0 or :: is loopback-equivalent here for the same reason it
// is in binding and only for that reason: the caller has established the daemon
// is this machine. An empty HostPort is a dynamic publish whose port the daemon
// picks at start, and it yields no candidate.
func configuredBinding(insp client.ContainerInspectResult) (host string, port int, ok bool) {
	if insp.Container.HostConfig == nil {
		return "", 0, false
	}
	want, err := network.ParsePort("5432/tcp")
	if err != nil {
		return "", 0, false
	}
	for _, b := range insp.Container.HostConfig.PortBindings[want] {
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

// binding picks the published host binding for 5432 and normalises it.
//
// A published 0.0.0.0 or :: means "every interface of the machine running the
// daemon"; it is loopback-equivalent here and only here, because the caller has
// already established that the daemon is this machine. A binding with no host
// IP counts as unpublished and yields no candidate (ADR-008 §4).
func binding(c container.Summary) (host string, port int, ok bool) {
	for _, p := range c.Ports {
		if p.PrivatePort != postgresPort || p.PublicPort == 0 || p.Type != "tcp" {
			continue
		}
		if !p.IP.IsValid() {
			continue
		}
		addr := p.IP
		if addr.IsUnspecified() {
			addr = netip.MustParseAddr("127.0.0.1")
		}
		return addr.String(), int(p.PublicPort), true
	}
	return "", 0, false
}

// displayName is the compose service when present, else the container name with
// its leading "/" stripped, else the container ID (ADR-008 §4).
func displayName(c container.Summary) string {
	if s := c.Labels[labelService]; s != "" {
		return s
	}
	if n := containerName(c); n != "" {
		return n
	}
	return c.ID
}

func containerName(c container.Summary) string {
	if len(c.Names) > 0 {
		return strings.TrimPrefix(c.Names[0], "/")
	}
	return ""
}

// configEnv is the container's declared environment, or nothing when the daemon
// reported no config for it.
func configEnv(insp client.ContainerInspectResult) []string {
	if insp.Container.Config == nil {
		return nil
	}
	return insp.Container.Config.Env
}
