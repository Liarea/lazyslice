// SPDX-License-Identifier: Apache-2.0

package testutil

import (
	"context"
	"io"
	"net"
	"testing"
)

// SecondEndpoint publishes a second endpoint onto the cluster connURL already
// names, and returns a connection URL for it.
//
// It is the aliasing ARCHITECTURE.md §9 rule 1 has to see through: one cluster
// reached over two transports — a second published port, a proxy, a pooler
// name, the unix socket — normalises to a different `host:port` every time, so
// the endpoint comparison alone cannot tell "the target is the source" from
// "the target is somewhere else". A test that wants that shape needs a real
// second route to one server, and this is the cheapest one that does not
// depend on how the container runtime publishes ports: a loopback listener in
// the test process that copies bytes both ways.
//
// The listener is closed when the test finishes. Connections already open when
// that happens are left to the process, which is ending: a test that closed
// them underneath a pool would report the shutdown rather than the assertion.
func SecondEndpoint(ctx context.Context, t *testing.T, connURL string) string {
	t.Helper()

	u, err := URL(connURL)
	if err != nil {
		t.Fatalf("testutil: parsing the connection url to proxy: %v", err)
	}
	backend := u.Host

	var lc net.ListenConfig
	ln, err := lc.Listen(ctx, "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("testutil: listening for the second endpoint: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })

	go func() {
		for {
			client, acceptErr := ln.Accept()
			if acceptErr != nil {
				return
			}
			go splice(client, backend)
		}
	}()

	second := *u
	second.Host = ln.Addr().String()
	return second.String()
}

// splice joins one accepted connection to a fresh connection to backend. It
// returns when either direction ends, which closes both.
func splice(client net.Conn, backend string) {
	defer client.Close()

	server, err := net.Dial("tcp", backend)
	if err != nil {
		return
	}
	defer server.Close()

	// Buffered for both directions: the one that is still copying when this
	// function returns must not block forever on a send nobody receives.
	done := make(chan error, 2)
	go func() { _, err := io.Copy(server, client); done <- err }()
	go func() { _, err := io.Copy(client, server); done <- err }()
	// Either end closing is how a proxied connection ends, so the error is the
	// end condition rather than something to report: the test asserts on what
	// the database answered, and there is no t here to report it to.
	<-done
}
