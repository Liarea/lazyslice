// SPDX-License-Identifier: Apache-2.0

// Package dockerctx resolves the Docker endpoint lazyslice talks to.
//
// It exists as its own package because "which Docker am I talking to" is the
// question that makes container discovery either work or silently find nothing
// on a machine running Colima, Rancher Desktop or a rootless daemon, and that
// question should be answerable in isolation.
//
// The order is ADR-008 §3's six steps, which supersede ARCHITECTURE.md §8's
// three-item summary:
//
//  1. --docker-host, when given. Nothing below runs.
//  2. $DOCKER_HOST, when non-empty. Nothing below runs.
//  3. $DOCKER_CONTEXT, else the currentContext field of config.json in the
//     Docker CLI config directory (~/.docker, or $DOCKER_CONFIG when set).
//  4. A context name that is empty or the literal "default" resolves to the
//     platform default socket without touching the context store.
//  5. Otherwise the named context's metadata is read from the context store and
//     the "docker" endpoint's Host is used.
//  6. A context whose Host is empty resolves to the platform default socket.
//
// The store is read with encoding/json rather than github.com/docker/cli: that
// module is not in ARCHITECTURE.md §13 and this is two JSON files. The layout
// (<config>/contexts/meta/<sha256(name)>/meta.json, Endpoints.docker.Host) was
// verified against a context written by a real Docker CLI before this reader
// was written, as ADR-008 §3 requires.
package dockerctx

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"strings"

	"github.com/moby/moby/client"
)

// The steps a resolved endpoint can come from, as they are printed. They are
// the words a degradation message uses — ADR-008 §3 requires the message to
// name "the endpoint that was actually resolved and the step it came from",
// never a hard-coded path.
const (
	FromFlag          = "--docker-host"
	FromDockerHost    = "$DOCKER_HOST"
	FromDockerContext = "$DOCKER_CONTEXT"
	FromContext       = "docker context"
	FromDefaultSocket = "default socket"
)

// Endpoint is a resolved Docker endpoint and how it was found, so that the
// candidate list can say "no containers found (docker: colima context)" rather
// than nothing at all.
type Endpoint struct {
	Host string
	// From is one of the From* constants above: the step of ADR-008 §3 that
	// produced Host.
	From string
	// Note is a sanitised sentence about how the resolution went, empty when it
	// went as expected. A context store this package could not read fills it,
	// because falling back silently is how a developer ends up debugging an
	// empty candidate list.
	Note string
}

// String renders the endpoint and its step, the form every message about it
// uses: "unix:///var/run/docker.sock (default socket)".
func (e Endpoint) String() string {
	if e.Host == "" {
		return "(none)"
	}
	return e.Host + " (" + e.From + ")"
}

// Local reports whether the endpoint's containers run on this machine.
//
// ADR-008 §3: unix:// and npipe:// are local, and tcp:// is local only when its
// host is a loopback IP literal. A hostname is never resolved, so
// tcp://localhost:2375 is not local and the unresolvable case fails closed —
// resolving it would put a DNS lookup inside the 2 s listing budget and would
// make a safety property depend on /etc/hosts.
//
// Everything rungs 3 and 4, Q1 and pipeline.Provisioner may do is conditioned
// on this being true (THREAT_MODEL.md T2).
func (e Endpoint) Local() bool {
	scheme, rest, found := strings.Cut(e.Host, "://")
	if !found {
		return false
	}
	switch scheme {
	case "unix", "npipe":
		return true
	case "tcp":
		host, _, err := net.SplitHostPort(rest)
		if err != nil {
			host = rest
		}
		addr, err := netip.ParseAddr(strings.Trim(host, "[]"))
		if err != nil {
			// A hostname. Never resolved, so never local.
			return false
		}
		return addr.IsLoopback()
	default:
		return false
	}
}

// SSH reports whether the endpoint is an ssh:// one. It is one case of a
// non-local endpoint with a reason of its own: tunnelling it needs a third
// subprocess, and ARCHITECTURE.md §8 fixes the set at two (ADR-008 §3).
func (e Endpoint) SSH() bool { return strings.HasPrefix(e.Host, "ssh://") }

// ErrNoEndpoint is returned when no endpoint could be named at all. It is not a
// failure of the run: a Docker failure never aborts discovery, and rungs 0 to 2
// still produce candidates (ADR-008 §3 "Degradation").
var ErrNoEndpoint = errors.New("dockerctx: no docker endpoint could be resolved")

// Resolve picks the Docker endpoint. hostFlag is the value of --docker-host,
// empty when the flag was not given.
//
// The context is taken so that the signature does not change when a step gains
// a call that can block; nothing here does I/O beyond reading two local files.
func Resolve(_ context.Context, hostFlag string) (Endpoint, error) {
	if hostFlag != "" {
		return Endpoint{Host: hostFlag, From: FromFlag}, nil
	}
	if h := os.Getenv("DOCKER_HOST"); h != "" {
		return Endpoint{Host: h, From: FromDockerHost}, nil
	}

	dir := configDir()
	name, from := os.Getenv("DOCKER_CONTEXT"), FromDockerContext
	if name == "" {
		name, from = currentContext(dir), FromContext
	}
	if name == "" || name == "default" {
		// Step 4. The literal "default" short-circuits without touching the
		// store: on Windows it is what is actually stored in config.json, so
		// this is not an optimisation (ADR-008 §3).
		return defaultEndpoint(""), nil
	}

	host, err := contextHost(dir, name)
	if err != nil {
		// Deliberately not an error: a store layout this reader cannot read is
		// step 4's fallback with a printed reason, never a hard failure and
		// never a silent empty endpoint (ADR-008 §3). A Docker failure never
		// aborts the run.
		//nolint:nilerr // the fallback is the decision, and Note carries the reason
		return defaultEndpoint(fmt.Sprintf(
			"docker context %q: could not read endpoint, using %s", name, defaultSocket())), nil
	}
	if host == "" {
		// Step 6: `docker context create foo --docker "host="` is creatable and
		// resolves to the platform default socket, not to an empty endpoint.
		return defaultEndpoint(fmt.Sprintf(
			"docker context %q names no host, using %s", name, defaultSocket())), nil
	}
	return Endpoint{Host: host, From: from + " " + name}, nil
}

func defaultEndpoint(note string) Endpoint {
	return Endpoint{Host: defaultSocket(), From: FromDefaultSocket, Note: note}
}

// configDir is the Docker CLI config directory: $DOCKER_CONFIG when set, else
// ~/.docker. An unreadable home is an empty directory, which makes steps 3 and
// 5 miss and step 4 answer, rather than an error a Docker-less machine has to
// read.
func configDir() string {
	if d := os.Getenv("DOCKER_CONFIG"); d != "" {
		return d
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".docker")
}

// currentContext reads the currentContext field of config.json. Anything that
// goes wrong is an empty name, which step 4 turns into the platform socket.
func currentContext(dir string) string {
	if dir == "" {
		return ""
	}
	b, err := os.ReadFile(filepath.Join(dir, "config.json"))
	if err != nil {
		return ""
	}
	var cfg struct {
		CurrentContext string `json:"currentContext"`
	}
	if err := json.Unmarshal(b, &cfg); err != nil {
		return ""
	}
	return cfg.CurrentContext
}

// contextHost reads one context's metadata from the store and returns its
// "docker" endpoint's Host.
//
// The store's layout is <config>/contexts/meta/<sha256(name) in hex>/meta.json,
// holding {"Name":...,"Endpoints":{"docker":{"Host":"..."}}}. It was read off a
// context written by a real Docker CLI before this function was written
// (ADR-008 §3); a layout it cannot read is the caller's step-4 fallback with a
// printed reason, never a silent empty endpoint.
func contextHost(dir, name string) (string, error) {
	if dir == "" {
		return "", ErrNoEndpoint
	}
	sum := sha256.Sum256([]byte(name))
	path := filepath.Join(dir, "contexts", "meta", hex.EncodeToString(sum[:]), "meta.json")
	b, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return "", fmt.Errorf("dockerctx: reading the context store: %w", err)
	}
	var meta struct {
		Name      string `json:"Name"`
		Endpoints map[string]struct {
			Host string `json:"Host"`
		} `json:"Endpoints"`
	}
	if err := json.Unmarshal(b, &meta); err != nil {
		return "", fmt.Errorf("dockerctx: parsing the context store: %w", err)
	}
	return meta.Endpoints["docker"].Host, nil
}

// Dial opens a Docker API client against a resolved endpoint. The client is
// used read-only by discovery; only internal/discover/provision creates or
// starts anything.
//
// Two options, not three: ADR-008 §3 records that lazydocker's third,
// WithAPIVersionNegotiation, is deprecated and a no-op in the pinned
// moby/moby/client, so staticcheck fails make lint on it. client.FromEnv is not
// used, because it reads $DOCKER_HOST and ignores contexts entirely, which is
// the whole of why steps 3 to 6 above exist.
//
// The constructor is client.New and not ADR-008 §3's client.NewClientWithOpts
// for the same reason the ADR gives about the third option: in the pinned
// v0.6.0 NewClientWithOpts is documented "Deprecated: use [New]" and carries a
// //go:fix inline directive, so govet's inline analyzer fails make lint on it.
// The options and their order are the ADR's, unchanged
// (internal/discover/CLAUDE.md).
func Dial(e Endpoint) (*client.Client, error) {
	if e.Host == "" {
		return nil, ErrNoEndpoint
	}
	c, err := client.New(
		client.WithTLSClientConfigFromEnv(),
		client.WithHost(e.Host),
	)
	if err != nil {
		return nil, fmt.Errorf("dockerctx: opening a client for %s: %w", e, err)
	}
	return c, nil
}
