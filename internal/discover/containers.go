// SPDX-License-Identifier: Apache-2.0

package discover

import (
	"context"
	"fmt"
	"net/netip"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"

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

// containers is rung 3: running Postgres containers on a local Docker endpoint.
//
// It lists with All: true so that rung 4's exited containers come from the same
// call (ADR-008 §4); this build reports how many it saw and builds no candidate
// from them, because rung 4 is phase 5 (ARCHITECTURE.md §14).
//
// local is the caller's answer to "is this endpoint local", and it is a
// parameter rather than a re-derivation because address normalisation is a
// function of (binding, endpointIsLocal) and it is a programming error to
// normalise a binding from a daemon that is not this machine (ADR-008 §4).
func containers(ctx context.Context, api dockerAPI, workdir string, local bool) (out []found, stopped int, matchedProject bool, err error) {
	if !local {
		return nil, 0, false, fmt.Errorf("discover: refusing to read containers from a non-local docker endpoint")
	}
	list, err := api.ContainerList(ctx, client.ContainerListOptions{All: true})
	if err != nil {
		return nil, 0, false, fmt.Errorf("discover: listing containers: %w", err)
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
		if c.State != container.StateRunning {
			stopped++
			continue
		}
		f, ok := candidateFor(ctx, api, c)
		if !ok {
			continue
		}
		out = append(out, f)
	}
	return out, stopped, matchedProject, nil
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
		dir := c.Labels[labelWorkingDir]
		if dir != "" && underOrEqual(abs, dir) {
			out = append(out, c)
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

	user, database, password := "postgres", "postgres", ""
	if insp, err := api.ContainerInspect(ctx, c.ID, client.ContainerInspectOptions{}); err == nil && insp.Container.Config != nil {
		env := envMap(insp.Container.Config.Env)
		if v := env["POSTGRES_USER"]; v != "" {
			user, database = v, v
		}
		if v := env["POSTGRES_DB"]; v != "" {
			database = v
		}
		password = env["POSTGRES_PASSWORD"]
	}

	u := url.URL{
		Scheme:   "postgres",
		Host:     host + ":" + strconv.Itoa(port),
		Path:     "/" + database,
		RawQuery: "sslmode=disable",
	}
	if password == "" {
		u.User = url.User(user)
	} else {
		u.User = url.UserPassword(user, password)
	}

	d, ref, err := dsn.Parse(u.String())
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

func envMap(env []string) map[string]string {
	out := make(map[string]string, len(env))
	for _, e := range env {
		if name, value, ok := strings.Cut(e, "="); ok {
			out[name] = value
		}
	}
	return out
}
