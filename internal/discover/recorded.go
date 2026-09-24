// SPDX-License-Identifier: Apache-2.0

package discover

import (
	"context"
	"errors"
	"net/url"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"

	"github.com/Liarea/lazyslice/internal/discover/dockerctx"
	"github.com/Liarea/lazyslice/internal/discover/provision"
	"github.com/Liarea/lazyslice/internal/dsn"
	"github.com/Liarea/lazyslice/internal/event"
	"github.com/Liarea/lazyslice/internal/pipeline"
)

// This file is ADR-016: what a committed lazyslice.yml's target record means
// when the record names a container lazyslice created.
//
// ADR-008 §1 made rung 0 an address: host, port, role and database, with the
// password left to PGPASSWORD, ~/.pgpass and --password-command. That covers a
// database the developer administers and not the one lazyslice made itself,
// whose random POSTGRES_PASSWORD lives in the container's environment and in
// one machine's state dir keyed by the directory's project name. Dogfood
// session 2 (T-0327) copied a session-1 lazyslice.yml to a new directory: the
// project name no longer matched, so no password was found, and the run
// stopped at exit 4 on a raw "password authentication failed (SQLSTATE
// 28P01)". On a second machine the container does not exist at all.
//
// So a record of lazyslice's own container names the container, not the port:
//
//   - --create-target outranks the record and provisions (or reuses) this
//     directory's own lazyslice-target-<project>, exactly as it would with no
//     record at all;
//   - otherwise, on a local and reachable Docker endpoint, the container is
//     looked up by the recorded name, read-only. One that carries
//     provision.LabelProject is ours: running, the run connects on its live
//     binding with the role and password from its own environment; stopped,
//     it is ADR-008 §6's Q1'; absent, it is Q1 with a lead that names it, and
//     headless it stops at exit 4 naming --create-target and --target;
//   - with no usable endpoint, or a container of that name that is not ours,
//     the record is an address again, as ADR-008 §1 had it.

// recordedContainer is the name of the container a committed target record
// names, when the record is one of lazyslice's own: `from: container`, a
// `service:` shaped like provision.Name's output, and a loopback address.
//
// Every other record keeps ADR-008 §1's meaning unchanged. The shape is not
// proof of ownership — the file is committed text — which is why the container
// must also carry provision.LabelProject before its environment is read.
func recordedContainer(cfg *pipeline.Config) (string, bool) {
	if cfg == nil || cfg.Target != pipeline.FromContainer {
		return "", false
	}
	if !provision.IsName(cfg.TargetLabel) || !cfg.TargetRef.Loopback() {
		return "", false
	}
	return cfg.TargetLabel, true
}

// resolveRecorded settles the target a committed record of lazyslice's own
// container names, once the source is known (the provisioning branches need
// the source's major).
//
// A nil candidate from recordedTarget with no error means Docker could not
// answer for the record, and the record falls back to being the address it
// was before ADR-016: rung0Target, with the state dir's remembered password
// where this directory's project name matches the label.
func resolveRecorded(ctx context.Context, o Options, res Result, name string, source *found, sink event.Sink) (Result, error) {
	f, named, asked, err := recordedTarget(ctx, o, name, source, sink)
	res.Asked = asked
	if err != nil {
		return res, err
	}
	if f == nil {
		d, err := rung0Target(o)
		if err != nil {
			return res, refuseInvalidRef(sink, "target", err)
		}
		if d != "" {
			res.Target = d
			res.TargetProvenance, res.TargetLabel = o.Config.Target, o.Config.TargetLabel
			res.TargetNamed = true
		}
		return res, nil
	}
	res.Target = string(f.dsn)
	res.TargetProvenance, res.TargetLabel = f.cand.Provenance, f.cand.Label
	res.TargetContainerID = f.containerID
	// Named only when the file's own container was reconnected or restarted.
	// A container --create-target or Q1 made is one nobody named — the same
	// answer noTarget's provisioning gives — and TargetNamed is what exempts a
	// target from the gate's headless same-cluster escalation (ADR-013).
	res.TargetNamed = named
	prov := provenanceOf(*f)
	if named {
		prov += " (./lazyslice.yml)"
	}
	send(sink, event.Decision, CodeTargetChosen, event.Args{
		event.ArgDatabase:   f.cand.Ref.Database,
		event.ArgHost:       f.cand.Ref.Host,
		event.ArgProvenance: prov,
		event.ArgFlag:       "--target",
	})
	return res, nil
}

// recordedTarget is ADR-016's decision for one record. See the file comment for
// the order; it returns the candidate, whether the file's own container is what
// it resolved to, whether a question reached the terminal, and the refusal.
func recordedTarget(ctx context.Context, o Options, name string, source *found, sink event.Sink) (*found, bool, bool, error) {
	api, dock := localDocker(ctx, o)
	if o.CreateTarget {
		// The same guard, before any container work, that noTarget applies
		// (ADR-008 §3, THREAT_MODEL.md T2).
		if !dock.usable() {
			return nil, false, false, refuseDockerNotLocal(dock, sink)
		}
		f, err := provisionTarget(ctx, o, source, dock, sink)
		return f, false, false, err
	}
	if !dock.usable() {
		return nil, false, false, nil
	}

	look, cancel := context.WithTimeout(ctx, listBudget)
	defer cancel()
	c, err := containerByName(look, api, name)
	if err != nil {
		// A daemon that answered a ping and then would not list is not
		// evidence the container is gone; the record stays an address.
		return nil, false, false, nil
	}
	if c == nil {
		f, asked, err := askQ1(ctx, o, source, dock, sink, missingLead(name), func() error {
			return refuseContainerMissing(sink, name)
		})
		return f, false, asked, err
	}
	if c.Labels[provision.LabelProject] == "" {
		// A container answers to the name and lazyslice did not create it:
		// its environment is somebody else's credential, so it is not read,
		// and the record is the address ADR-008 §1 made it.
		return nil, false, false, nil
	}
	if c.State != container.StateRunning {
		stopped, ok := stoppedCandidateFor(look, api, *c)
		if !ok {
			return nil, false, false, nil
		}
		stopped.containerID = name
		f, asked, err := startStopped(ctx, o, &stopped, dock, sink, o.Config.TargetRef.Database)
		return f, err == nil, asked, err
	}
	f, ok := reconnect(look, api, *c, name, o.Config.TargetRef)
	if !ok {
		return nil, false, false, nil
	}
	return &f, true, false, nil
}

// localDocker resolves the Docker endpoint and, when it is local, dials and
// pings it inside the listing budget. It prints nothing: rung 3 prints the
// endpoint for the ladder, and a run that only needs one named container has
// no candidate list for those lines to head.
func localDocker(ctx context.Context, o Options) (dockerAPI, dockerEndpoint) {
	endpoint, err := dockerctx.Resolve(ctx, o.DockerHost)
	if err != nil {
		return nil, dockerEndpoint{name: "(none)"}
	}
	dock := dockerEndpoint{endpoint: endpoint, name: endpoint.String(), local: endpoint.Local() && !endpoint.SSH()}
	if !dock.local {
		return nil, dock
	}
	ping, cancel := context.WithTimeout(ctx, listBudget)
	defer cancel()
	api, err := dialDocker(o, endpoint)
	if err == nil {
		_, err = api.Ping(ping, client.PingOptions{})
	}
	if err != nil {
		return nil, dock
	}
	dock.reachable = true
	return api, dock
}

// containerByName is the container that answers to exactly name, running or
// not, or nil. The daemon's name filter is a substring match, so the exact name
// is checked here, as provision's own byName does.
func containerByName(ctx context.Context, api dockerAPI, name string) (*container.Summary, error) {
	list, err := api.ContainerList(ctx, client.ContainerListOptions{
		All:     true,
		Filters: make(client.Filters).Add("name", name),
	})
	if err != nil {
		return nil, err
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

// reconnect is the running container a record names, as a candidate: the host
// and port of its live binding, the role and password from its own
// environment (THREAT_MODEL.md A3 names container env as a place a credential
// lives, and rung 3 already reads it there), and the database the file
// records, falling back to the environment's when the file records none.
//
// The binding is the container's, not the file's, on purpose: the container is
// what the record names, and the port it publishes today is where it answers.
func reconnect(ctx context.Context, api dockerAPI, c container.Summary, name string, recorded dsn.Ref) (found, bool) {
	host, port, ok := binding(c)
	if !ok {
		return found{}, false
	}
	insp, err := api.ContainerInspect(ctx, c.ID, client.ContainerInspectOptions{})
	if err != nil {
		return found{}, false
	}
	user, database, password := provision.Credentials(provision.EnvMap(configEnv(insp)))
	if recorded.Database != "" {
		database = recorded.Database
	}
	d, ref, err := dsn.Parse(provision.ConnString(host, port, user, database, password))
	if err != nil {
		return found{}, false
	}
	return found{
		dsn:         d,
		containerID: name,
		cand: pipeline.Candidate{
			Ref:        ref,
			Provenance: pipeline.FromContainer,
			Label:      name,
			// A container on a local daemon is on this machine (ADR-008 §3),
			// which is the only reason this is true.
			Local: true,
		},
	}, true
}

// onDatabase is a started container's connection pointed at the database the
// committed record names, so that a record loads into the same database whether
// its container was running (reconnect) or had to be started first.
//
// url.Parse's error quotes the string it failed on, which holds the password,
// so it is replaced rather than wrapped.
func onDatabase(res provision.Result, database string) (provision.Result, error) {
	u, err := url.Parse(res.DSN)
	if err != nil {
		return res, errors.New("the started container's connection string could not be parsed")
	}
	u.Path, u.RawPath = "/"+database, ""
	d, ref, err := dsn.Parse(u.String())
	if err != nil {
		return res, err
	}
	res.DSN, res.Candidate.Ref = string(d), ref
	return res, nil
}

// missingLead is Q1's first sentence when the committed record names a
// container this machine's Docker does not have (ADR-016).
func missingLead(name string) string {
	return "./lazyslice.yml's target is the container " + name + ", which this machine's docker does not have."
}

// refuseContainerMissing is ADR-016's headless stop, and the "no" to its Q1.
func refuseContainerMissing(sink event.Sink, name string) error {
	r := &Refusal{
		Code: CodeTargetContainerMissing, Exit: exitTarget,
		Args:    event.Args{event.ArgContainer: name},
		Message: "the target container " + name + " recorded in ./lazyslice.yml does not exist",
	}
	sendError(sink, r)
	return r
}
