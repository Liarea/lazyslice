// SPDX-License-Identifier: Apache-2.0

package dockerctx

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

// The six steps of ADR-008 §3, in order, including the two that matter: the
// literal context name "default" short-circuiting without touching the store,
// and a context whose Host is empty falling back to the platform socket rather
// than resolving to nothing.
//
// This is the ADR's own TestDockerHostResolutionOrder.
func TestDockerHostResolutionOrder(t *testing.T) {
	dir := t.TempDir()
	writeContext(t, dir, "colima", "unix:///Users/dev/.colima/docker.sock")
	writeContext(t, dir, "hostless", "")
	writeConfig(t, dir, "colima")

	for _, tc := range []struct {
		name     string
		flag     string
		env      map[string]string
		wantHost string
		wantFrom string
		wantNote bool
	}{
		{
			name: "step 1: the flag wins and nothing below runs",
			flag: "tcp://127.0.0.1:2375",
			env: map[string]string{
				"DOCKER_HOST":    "unix:///ignored.sock",
				"DOCKER_CONTEXT": "colima",
			},
			wantHost: "tcp://127.0.0.1:2375",
			wantFrom: FromFlag,
		},
		{
			name:     "step 2: DOCKER_HOST beats the context",
			env:      map[string]string{"DOCKER_HOST": "unix:///run/user/1000/docker.sock", "DOCKER_CONTEXT": "colima"},
			wantHost: "unix:///run/user/1000/docker.sock",
			wantFrom: FromDockerHost,
		},
		{
			name:     "step 3: DOCKER_CONTEXT names the context",
			env:      map[string]string{"DOCKER_CONTEXT": "colima"},
			wantHost: "unix:///Users/dev/.colima/docker.sock",
			wantFrom: FromDockerContext + " colima",
		},
		{
			name:     "step 3: currentContext when DOCKER_CONTEXT is unset",
			env:      map[string]string{},
			wantHost: "unix:///Users/dev/.colima/docker.sock",
			wantFrom: FromContext + " colima",
		},
		{
			name:     "step 4: the literal default short-circuits to the platform socket",
			env:      map[string]string{"DOCKER_CONTEXT": "default"},
			wantHost: defaultSocket(),
			wantFrom: FromDefaultSocket,
		},
		{
			name:     "step 5: a context the store does not carry falls back with a reason",
			env:      map[string]string{"DOCKER_CONTEXT": "absent"},
			wantHost: defaultSocket(),
			wantFrom: FromDefaultSocket,
			wantNote: true,
		},
		{
			name:     "step 6: a context whose Host is empty falls back with a reason",
			env:      map[string]string{"DOCKER_CONTEXT": "hostless"},
			wantHost: defaultSocket(),
			wantFrom: FromDefaultSocket,
			wantNote: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("DOCKER_CONFIG", dir)
			t.Setenv("DOCKER_HOST", "")
			t.Setenv("DOCKER_CONTEXT", "")
			for k, v := range tc.env {
				t.Setenv(k, v)
			}

			e, err := Resolve(t.Context(), tc.flag)
			if err != nil {
				t.Fatalf("Resolve: %v", err)
			}
			if e.Host != tc.wantHost {
				t.Errorf("host = %q, want %q", e.Host, tc.wantHost)
			}
			if e.From != tc.wantFrom {
				t.Errorf("from = %q, want %q", e.From, tc.wantFrom)
			}
			if tc.wantNote && e.Note == "" {
				t.Error("the fallback printed no reason, so the developer sees an empty candidate list and no cause")
			}
			if !tc.wantNote && e.Note != "" {
				t.Errorf("unexpected note %q", e.Note)
			}
		})
	}
}

// Locality is the precondition of rungs 3 and 4, Q1 and the Provisioner
// (THREAT_MODEL.md T2), so it fails closed: a hostname is never resolved, which
// makes tcp://localhost non-local.
func TestLocalIsDecidedWithoutResolvingNames(t *testing.T) {
	for _, tc := range []struct {
		host string
		want bool
	}{
		{"unix:///var/run/docker.sock", true},
		{"npipe:////./pipe/docker_engine", true},
		{"tcp://127.0.0.1:2375", true},
		{"tcp://127.9.9.9:2375", true},
		{"tcp://[::1]:2375", true},
		{"tcp://localhost:2375", false},
		{"tcp://staging:2375", false},
		{"tcp://10.0.0.4:2375", false},
		{"ssh://dev@build-box", false},
		{"", false},
		{"/var/run/docker.sock", false},
	} {
		if got := (Endpoint{Host: tc.host}).Local(); got != tc.want {
			t.Errorf("Local(%q) = %v, want %v", tc.host, got, tc.want)
		}
	}
	if !(Endpoint{Host: "ssh://dev@build-box"}).SSH() {
		t.Error("an ssh:// endpoint must be recognised: tunnelling it needs a third subprocess")
	}
}

func writeConfig(t *testing.T, dir, current string) {
	t.Helper()
	body := `{"currentContext":"` + current + `"}`
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(body), 0o600); err != nil {
		t.Fatalf("writing config.json: %v", err)
	}
}

// writeContext writes one context in the layout a real `docker context create`
// produces, which was read off this machine's own store before the reader in
// dockerctx.go was written (ADR-008 §3).
func writeContext(t *testing.T, dir, name, host string) {
	t.Helper()
	sum := sha256.Sum256([]byte(name))
	meta := filepath.Join(dir, "contexts", "meta", hex.EncodeToString(sum[:]))
	if err := os.MkdirAll(meta, 0o750); err != nil {
		t.Fatalf("creating the context store: %v", err)
	}
	body := `{"Name":"` + name + `","Metadata":{},"Endpoints":{"docker":{"Host":"` + host + `","SkipTLSVerify":false}}}`
	if err := os.WriteFile(filepath.Join(meta, "meta.json"), []byte(body), 0o600); err != nil {
		t.Fatalf("writing meta.json: %v", err)
	}
}
