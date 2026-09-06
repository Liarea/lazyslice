// SPDX-License-Identifier: Apache-2.0

package dsn

import (
	"errors"
	"strings"
	"testing"
)

// The password is the reason this package exists, so the first test is that it
// cannot get out of it: not through the Ref, not through String, not through
// Redact (THREAT_MODEL.md T5).
func TestParseKeepsThePasswordOutOfTheRef(t *testing.T) {
	const secret = "hunter2"
	for _, in := range []string{
		"postgres://app:" + secret + "@db.example.com:6432/shop?sslmode=require",
		"host=db.example.com port=6432 dbname=shop user=app password=" + secret,
	} {
		d, ref, err := Parse(in)
		if err != nil {
			t.Fatalf("Parse(%q): %v", strings.ReplaceAll(in, secret, "…"), err)
		}
		if string(d) != in {
			t.Errorf("Parse returned a DSN that is not the string it was given")
		}
		want := Ref{Host: "db.example.com", Port: 6432, Database: "shop", User: "app"}
		if ref != want {
			t.Errorf("Parse ref = %#v, want %#v", ref, want)
		}
		if strings.Contains(ref.String(), secret) {
			t.Errorf("Ref.String() = %q, which carries the password", ref.String())
		}
		red, err := Redact(d)
		if err != nil {
			t.Fatalf("Redact: %v", err)
		}
		if strings.Contains(red, secret) {
			t.Errorf("Redact = %q, which carries the password", red)
		}
		if !strings.Contains(red, "db.example.com:6432") || !strings.Contains(red, "/shop") {
			t.Errorf("Redact = %q, want it to still name the endpoint", red)
		}
	}
}

func TestParseRefusesAnEmptyString(t *testing.T) {
	if _, _, err := Parse("  "); err == nil {
		t.Fatal("Parse(\"  \") returned no error")
	}
}

// The gate's rule 1 is "normalised host:port/database". These are the spellings
// of one endpoint a developer actually has in front of them.
func TestSameEndpointFoldsTheLoopbackSpellings(t *testing.T) {
	base := Ref{Host: "127.0.0.1", Port: 5432, Database: "shop"}
	for _, same := range []Ref{
		{Host: "localhost", Port: 5432, Database: "shop"},
		{Host: "LOCALHOST.", Database: "shop"}, // no port: 5432 is the default
		{Host: "::1", Port: 5432, Database: "shop"},
		{Host: "[::1]", Port: 5432, Database: "shop"},
		{Host: "127.0.0.2", Port: 5432, Database: "shop"},
	} {
		got, err := base.SameEndpoint(same)
		if err != nil {
			t.Fatalf("SameEndpoint(%v): %v", same, err)
		}
		if !got {
			t.Errorf("SameEndpoint(%v) = false, want true: it is the same database", same)
		}
	}

	for _, other := range []Ref{
		{Host: "127.0.0.1", Port: 5432, Database: "shop_test"}, // second database, same cluster
		{Host: "127.0.0.1", Port: 5433, Database: "shop"},      // second cluster
		{Host: "db.example.com", Port: 5432, Database: "shop"},
	} {
		got, err := base.SameEndpoint(other)
		if err != nil {
			t.Fatalf("SameEndpoint(%v): %v", other, err)
		}
		if got {
			t.Errorf("SameEndpoint(%v) = true, want false", other)
		}
	}
}

// A wrong "not equal" here is a write to production, so an incomparable
// reference is an error and never a false (THREAT_MODEL.md T2).
func TestSameEndpointErrorsRatherThanGuessing(t *testing.T) {
	full := Ref{Host: "127.0.0.1", Port: 5432, Database: "shop"}

	if _, err := full.SameEndpoint(Ref{Port: 5432, Database: "shop"}); !errors.Is(err, ErrNoHost) {
		t.Errorf("SameEndpoint with no host: err = %v, want ErrNoHost", err)
	}
	if _, err := full.SameEndpoint(Ref{Host: "127.0.0.1", Port: 5432}); !errors.Is(err, ErrNoDatabase) {
		t.Errorf("SameEndpoint with no database: err = %v, want ErrNoDatabase", err)
	}
}

// The common compose setup — app and app_test in one container — is the same
// cluster and is eligible, so the two questions have to be separable.
func TestSameClusterIgnoresTheDatabase(t *testing.T) {
	a := Ref{Host: "localhost", Port: 5432, Database: "app"}
	b := Ref{Host: "127.0.0.1", Port: 5432, Database: "app_test"}

	if same, err := a.SameCluster(b); err != nil || !same {
		t.Errorf("SameCluster = %v, %v; want true, nil", same, err)
	}
	if same, err := a.SameEndpoint(b); err != nil || same {
		t.Errorf("SameEndpoint = %v, %v; want false, nil", same, err)
	}
}

func TestLoopback(t *testing.T) {
	for _, r := range []Ref{
		{Host: "localhost"},
		{Host: "127.0.0.1"},
		{Host: "::1"},
		{Host: "/var/run/postgresql"},
	} {
		if !r.Loopback() {
			t.Errorf("Ref{Host: %q}.Loopback() = false, want true", r.Host)
		}
	}
	for _, r := range []Ref{
		{Host: "db.example.com"},
		{Host: "10.0.0.7"},
		{Host: ""},
	} {
		if r.Loopback() {
			t.Errorf("Ref{Host: %q}.Loopback() = true, want false", r.Host)
		}
	}
}

// The marker binds a target to a source by this fingerprint, so two spellings
// of one endpoint have to produce one fingerprint or a reload would look
// unbound and refuse (ARCHITECTURE.md §11.2).
func TestFingerprintIsOverTheNormalisedEndpoint(t *testing.T) {
	a := Ref{Host: "localhost", Database: "shop", User: "app"}
	b := Ref{Host: "127.0.0.1", Port: 5432, Database: "shop", User: "someone_else"}
	if a.Fingerprint() != b.Fingerprint() {
		t.Errorf("fingerprints differ for one endpoint: %s vs %s", a.Fingerprint(), b.Fingerprint())
	}
	c := Ref{Host: "localhost", Database: "shop_test"}
	if a.Fingerprint() == c.Fingerprint() {
		t.Error("two databases on one cluster share a fingerprint")
	}
	if len(a.Fingerprint()) != 16 {
		t.Errorf("Fingerprint() = %q, want 16 hex characters", a.Fingerprint())
	}
}

// A Unix socket is two clusters' worth of ambiguity if it is folded into
// loopback: /var/run/postgresql and /tmp are different servers.
func TestSocketDirectoriesAreNotFoldedIntoLoopback(t *testing.T) {
	a := Ref{Host: "/var/run/postgresql", Database: "shop"}
	b := Ref{Host: "/tmp", Database: "shop"}
	c := Ref{Host: "127.0.0.1", Database: "shop"}

	for _, pair := range []struct{ x, y Ref }{{a, b}, {a, c}} {
		same, err := pair.x.SameEndpoint(pair.y)
		if err != nil {
			t.Fatalf("SameEndpoint(%q, %q): %v", pair.x.Host, pair.y.Host, err)
		}
		if same {
			t.Errorf("SameEndpoint(%q, %q) = true, want false", pair.x.Host, pair.y.Host)
		}
	}
	same, err := a.SameEndpoint(Ref{Host: "/var/run/postgresql/", Database: "shop"})
	if err != nil || !same {
		t.Errorf("one socket directory spelled two ways: %v, %v", same, err)
	}
}

// libpq accepts a comma-separated host list and connects to whichever answers,
// so a Ref built from the first host would describe an endpoint the driver may
// never touch — and the gate compares that Ref (THREAT_MODEL.md T2). A string
// lazyslice cannot faithfully describe is an error, not a first host.
func TestParseRefusesMultipleHosts(t *testing.T) {
	for _, in := range []string{
		"postgres://u:hunter2@127.0.0.1:5432,prod.example.com:5432/app",
		"postgres://u@127.0.0.1,prod.example.com/app",
		"host=127.0.0.1,prod.example.com dbname=app user=u",
	} {
		_, ref, err := Parse(in)
		if !errors.Is(err, ErrMultipleHosts) {
			t.Errorf("Parse(%q) = %#v, %v; want ErrMultipleHosts", in, ref, err)
		}
		if _, err := Redact(DSN(in)); !errors.Is(err, ErrMultipleHosts) {
			t.Errorf("Redact(%q) err = %v; want ErrMultipleHosts, because the redacted "+
				"string would name only the first host", in, err)
		}
	}

	// The TLS downgrade sslmode=prefer implies is also carried in Fallbacks, as
	// a repeat of the primary host and port. That is one endpoint.
	for _, in := range []string{
		"postgres://u@127.0.0.1:5432/app?sslmode=prefer",
		"postgres://u@127.0.0.1:5432/app",
		"host=/var/run/postgresql dbname=app user=u",
	} {
		if _, _, err := Parse(in); err != nil {
			t.Errorf("Parse(%q): %v; want it accepted, it names one endpoint", in, err)
		}
	}
}
