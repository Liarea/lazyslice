// SPDX-License-Identifier: Apache-2.0

package dsn

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"
)

// The password is the reason this package exists, so the first test is that it
// cannot get out of it: not through the Ref, not through String, not through
// Redact (THREAT_MODEL.md T5).
func TestParseKeepsThePasswordOutOfTheRef(t *testing.T) {
	const secret = "hunter2"
	for _, tc := range []struct {
		in         string
		wantParams map[string]string
	}{
		{"postgres://app:" + secret + "@db.example.com:6432/shop?sslmode=require",
			map[string]string{"sslmode": "require"}},
		{"host=db.example.com port=6432 dbname=shop user=app password=" + secret, nil},
	} {
		in := tc.in
		d, ref, err := Parse(in)
		if err != nil {
			t.Fatalf("Parse(%q): %v", strings.ReplaceAll(in, secret, "…"), err)
		}
		if string(d) != in {
			t.Errorf("Parse returned a DSN that is not the string it was given")
		}
		want := Ref{Host: "db.example.com", Port: 6432, Database: "shop", User: "app", Params: tc.wantParams}
		if !reflect.DeepEqual(ref, want) {
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

// docs/reviews/2026-09-09/REVIEW.md finding 6: sslmode=verify-full and
// sslrootcert are exactly the pair pgconn.Config consumes into a tls.Config
// and keeps no string form of, so they are the case Params exists for.
//
// sslrootcert names a file pgconn.ParseConfig reads and parses as PEM
// immediately (it builds the cert pool at parse time, not at connect time),
// so the path has to name a real, valid certificate or Parse itself refuses
// the string for a reason unrelated to this test.
func TestParseKeepsAllowlistedParams(t *testing.T) {
	caPath := writeTestCA(t)
	for _, in := range []string{
		"postgres://app@db.example.com:6432/shop?sslmode=verify-full&sslrootcert=" + url.QueryEscape(caPath),
		"host=db.example.com port=6432 dbname=shop user=app sslmode=verify-full sslrootcert=" + caPath,
	} {
		_, ref, err := Parse(in)
		if err != nil {
			t.Fatalf("Parse(%q): %v", in, err)
		}
		want := map[string]string{"sslmode": "verify-full", "sslrootcert": caPath}
		if !reflect.DeepEqual(ref.Params, want) {
			t.Errorf("Parse(%q).Params = %#v, want %#v", in, ref.Params, want)
		}
		if got, want := ref.String(), "app@db.example.com:6432/shop (+2 params)"; got != want {
			t.Errorf("Ref.String() = %q, want %q", got, want)
		}
	}
}

// writeTestCA writes a self-signed certificate to a file in t.TempDir and
// returns its path, for a sslrootcert value pgconn.ParseConfig will accept.
func writeTestCA(t *testing.T) string {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generating test CA key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(time.Hour),
		IsCA:         true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("creating test CA certificate: %v", err)
	}
	path := filepath.Join(t.TempDir(), "ca.pem")
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("creating %s: %v", path, err)
	}
	defer f.Close()
	if err := pem.Encode(f, &pem.Block{Type: "CERTIFICATE", Bytes: der}); err != nil {
		t.Fatalf("encoding test CA certificate: %v", err)
	}
	return path
}

// Never password, never sslpassword, never a whole DSN in Params — and a
// param outside the allowlist is named in ExtractParams's dropped list rather
// than silently kept or silently lost.
func TestExtractParamsDropsWhatIsNotAllowlisted(t *testing.T) {
	in := "postgres://app:hunter2@db.example.com:6432/shop" +
		"?sslmode=verify-full&sslpassword=hunter3&target_session_attrs=read-write"
	params, dropped := ExtractParams(in)
	if got, want := params, map[string]string{"sslmode": "verify-full"}; !reflect.DeepEqual(got, want) {
		t.Errorf("ExtractParams params = %#v, want %#v", got, want)
	}
	if got, want := dropped, []string{"target_session_attrs"}; !reflect.DeepEqual(got, want) {
		t.Errorf("ExtractParams dropped = %#v, want %#v (never password, never sslpassword)", got, want)
	}
	for _, secret := range []string{"hunter2", "hunter3"} {
		if strings.Contains(fmt.Sprint(params), secret) || strings.Contains(fmt.Sprint(dropped), secret) {
			t.Errorf("ExtractParams(%q) leaked a credential", strings.ReplaceAll(in, secret, "…"))
		}
	}
}

// docs/reviews/2026-09-14, finding 5: "options" is on AllowedParams by key,
// but unlike every other entry it is libpq's own channel for setting an
// arbitrary server GUC, so a key match alone let a committed lazyslice.yml set
// search_path or similar on the source session. A safe options value (a
// timeout, on allowedOptionSettings) survives; one naming anything else is
// dropped whole, both through FilterAllowedParams (a Ref built from a
// committed file) and through ExtractParams (a fresh connection string).
func TestOptionsIsValidatedByValueNotJustByKey(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   string
		keep bool
	}{
		{"a single allowed timeout", "-c statement_timeout=5000", true},
		{"two allowed timeouts", "-c statement_timeout=5000 -c lock_timeout=1000", true},
		{"search_path is not on the allowlist", "-c search_path=public", false},
		{"one allowed setting beside one that is not", "-c statement_timeout=5000 -c search_path=public", false},
		{"not -c syntax at all", "enable_seqscan=off", false},
		{"empty", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := FilterAllowedParams(map[string]string{"options": tc.in})
			_, kept := got["options"]
			if kept != tc.keep {
				t.Errorf("FilterAllowedParams({options: %q})[\"options\"] present = %v, want %v", tc.in, kept, tc.keep)
			}
		})
	}
}

// The same value-level check applies on the ExtractParams side (a fresh
// connection string), and a rejected options value is reported through the
// dropped list — the same channel warnDroppedParams already uses to tell an
// operator a setting did not survive — rather than vanishing with no trace.
func TestExtractParamsRejectsAnUnsafeOptionsValue(t *testing.T) {
	in := "postgres://app@db.example.com:6432/shop?sslmode=verify-full&options=" +
		url.QueryEscape("-c search_path=public")
	params, dropped := ExtractParams(in)
	if _, ok := params["options"]; ok {
		t.Errorf("ExtractParams(%q).params = %#v, want options dropped", in, params)
	}
	if !slices.Contains(dropped, "options") {
		t.Errorf("ExtractParams(%q).dropped = %#v, want it to name options", in, dropped)
	}
}

// A Ref built with no Params is IsZero exactly when the four identity fields
// are all empty — internal/emit used to compare Ref with == for this before
// Params made Ref incomparable.
func TestRefIsZero(t *testing.T) {
	if !(Ref{}).IsZero() {
		t.Error("Ref{}.IsZero() = false, want true")
	}
	if (Ref{Host: "db.example.com"}).IsZero() {
		t.Error("a Ref naming a host is not zero")
	}
	if (Ref{Params: map[string]string{"sslmode": "require"}}).IsZero() {
		t.Error("a Ref carrying Params is not zero, even with no identity fields set")
	}
}

// Parse stays generic on a bad parameter — this is what ParseError exists
// beside it for, and the two must keep disagreeing this way (docs/reviews,
// 2026-09-14, finding 2).
func TestParseErrorNamesWhatParseWontSay(t *testing.T) {
	const secret = "hunter2"
	in := "postgres://app:" + secret + "@db.example.com:6432/shop?sslmode=verify-ful"

	if _, _, err := Parse(in); err == nil || strings.Contains(err.Error(), secret) || strings.Contains(err.Error(), "sslmode") {
		t.Errorf("Parse(%q) err = %v, want a generic error naming neither the password nor sslmode", strings.ReplaceAll(in, secret, "…"), err)
	}

	err := ParseError(in)
	if err == nil {
		t.Fatal("ParseError: want an error for an invalid sslmode, got nil")
	}
	if !strings.Contains(err.Error(), "sslmode") {
		t.Errorf("ParseError = %q, want it to name sslmode", err)
	}
	// ParseError's own doc comment promises this only for a string built from
	// a Ref, which never holds a password; pgconn redacts one anyway
	// (belt and suspenders — this pins that behaviour rather than relying on
	// it, since ParseError must never be pointed at operator-typed input).
	if strings.Contains(err.Error(), secret) {
		t.Errorf("ParseError = %q, leaked the password", err)
	}
}
