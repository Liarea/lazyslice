// SPDX-License-Identifier: Apache-2.0

// Package dsn parses connection strings and holds the two types that keep
// passwords out of everything else.
//
// DSN is the whole connection string, password included. It is the one type in
// lazyslice that can hold a credential, and no serialisable structure may reach
// it: TestNoValueBearingFieldSerialised walks event.Event, pipeline.Config,
// pipeline.Plan and pipeline.Report and fails if any of them can
// (THREAT_MODEL.md T4).
//
// Ref is the redacted form — host, port, database, user — and it is what the
// candidate list, the decision header, lazyslice.yml and lazyslice_meta carry.
// Ref.String never prints a password because Ref never holds one.
//
// Normalisation lives here and nowhere else (ARCHITECTURE.md §9 rule 1, rule 2;
// internal/dsn/CLAUDE.md): "localhost", "127.0.0.1" and "::1" are one endpoint,
// a missing port is 5432, and the gate in internal/pg compares normalised
// references rather than re-implementing any of it.
package dsn

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net"
	"net/url"
	"path"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
)

// DefaultPort is the port a reference with no port names. Two references that
// differ only in whether the port was written are one endpoint, and the gate
// must see them as one (ARCHITECTURE.md §9 rule 1).
const DefaultPort = 5432

// ErrNoHost is returned when a reference carries no host and therefore cannot
// be compared with another. It is an error rather than "not equal" because a
// wrong "not equal" here is a write to production (THREAT_MODEL.md T2).
var ErrNoHost = errors.New("dsn: reference has no host, so it cannot be compared")

// ErrNoDatabase is returned when a reference carries no database name.
var ErrNoDatabase = errors.New("dsn: reference has no database, so it cannot be compared")

// ErrMultipleHosts is returned when a connection string names more than one
// endpoint.
//
// libpq — and pgconn with it — accepts a comma-separated host list
// (postgres://u@a:5432,b:5432/app, host=a,b) and connects to whichever answers
// first. A Ref built from the first of them describes a connection that may not
// be the one the driver makes, and the gate's identity and locality rules
// (ARCHITECTURE.md §9 rules 1 and 2) would then judge an endpoint the run never
// touches: a loopback host written first and a production host written second
// would read as local and not-the-source while the write went to production
// (THREAT_MODEL.md T2). A multi-host DATABASE_URL is exactly what discovery
// rung 1 picks up verbatim, so this fails closed rather than picking a host.
var ErrMultipleHosts = errors.New("dsn: the connection string names more than one host; lazyslice needs one endpoint per connection string")

// DSN is a full connection string. Treat it as a credential: log it nowhere,
// put it in no struct that is written to a file or an event, and hand it only
// to the driver. Redact returns the form that is safe to show.
type DSN string

// Ref is the redacted identity of a database. It is safe to print, to write to
// lazyslice.yml and to send in an event.
type Ref struct {
	Host     string
	Port     int
	Database string
	User     string
}

// String renders the reference as "user@host:port/database", the form the
// decision header and the candidate list use. It cannot print a password
// because Ref does not hold one.
func (r Ref) String() string {
	s := ""
	if r.User != "" {
		s = r.User + "@"
	}
	s += r.Host
	if r.Port != 0 {
		s += ":" + strconv.Itoa(r.Port)
	}
	if r.Database != "" {
		s += "/" + r.Database
	}
	return s
}

// Fingerprint is sha256(host:port/database)[:16] in hex, the value
// lazyslice_meta records as source_fingerprint. It identifies a source without
// recording a DSN, so a marker table cannot leak where the data came from.
//
// It is computed over the *normalised* host and port, so that a run reached
// through "localhost" and a run reached through "127.0.0.1" produce the same
// fingerprint and the marker stays bound (ARCHITECTURE.md §11.2). A reference
// whose host cannot be normalised is fingerprinted over what it does carry,
// because a fingerprint is a label and never a gate: the gate's own comparison
// is SameEndpoint, which returns an error instead.
func (r Ref) Fingerprint() string {
	host, port := r.Host, r.Port
	if n, err := r.Normalised(); err == nil {
		host, port = n.Host, n.Port
	}
	sum := sha256.Sum256([]byte(host + ":" + strconv.Itoa(port) + "/" + r.Database))
	return hex.EncodeToString(sum[:8])
}

// Normalised returns the reference with its host and port in canonical form:
// every loopback spelling becomes "localhost", an IP literal becomes its
// canonical text, a name becomes its lower-case form without a trailing dot, a
// Unix socket directory becomes "unix:" plus the cleaned path, and a zero port
// becomes DefaultPort. User and Database are carried through unchanged, because
// a database name is case-sensitive in Postgres and the role does not enter any
// comparison.
//
// A name is never resolved. Resolution would make the answer depend on DNS at
// the moment the gate ran, and ARCHITECTURE.md §9 says a hostname is never
// resolved to decide locality.
func (r Ref) Normalised() (Ref, error) {
	host, err := normaliseHost(r.Host)
	if err != nil {
		return Ref{}, err
	}
	n := r
	n.Host = host
	if n.Port == 0 {
		n.Port = DefaultPort
	}
	return n, nil
}

// Loopback reports whether the reference names a database on this machine: a
// Unix socket, a loopback IP literal, or the name "localhost".
//
// The name is admitted here although ARCHITECTURE.md §9 refuses it for a Docker
// endpoint. The two are different questions: there, "local" authorises reading
// a daemon's containers over TCP, and `tcp://localhost:2375` is a plausible
// tunnel to somebody else's daemon; here, "local" is the ordinary spelling of
// the target in every compose file and connection string a developer writes,
// and refusing it would make `--allow-remote-target localhost` the normal case,
// which would teach the flag away. See internal/dsn/CLAUDE.md, "Decisions made
// during implementation".
func (r Ref) Loopback() bool {
	host, err := normaliseHost(r.Host)
	if err != nil {
		return false
	}
	return host == canonicalLoopback || strings.HasPrefix(host, unixPrefix)
}

// SameEndpoint reports whether two references name the same normalised
// host:port/database. The target gate refuses when the target and the source
// answer true (ARCHITECTURE.md §9 rule 1), so this is the rule that stops a run
// writing to its own source.
//
// It returns an error rather than a comparison when either side cannot be
// normalised, because a wrong false here is a write to production: the gate
// must fail on the error and may never read the false as "not the source".
func (r Ref) SameEndpoint(other Ref) (bool, error) {
	a, err := r.Normalised()
	if err != nil {
		return false, err
	}
	b, err := other.Normalised()
	if err != nil {
		return false, err
	}
	if a.Database == "" || b.Database == "" {
		return false, ErrNoDatabase
	}
	return a.Host == b.Host && a.Port == b.Port && a.Database == b.Database, nil
}

// SameCluster reports whether two references name the same normalised
// host:port, whatever database they name on it.
//
// It is not a refusal on its own: `app` and `app_test` in one container are the
// common compose setup and are eligible, with `same cluster as source` printed
// as a warning (ARCHITECTURE.md §9 rule 1). The gate prefers the server's
// system_identifier, which sees through a pooler alias; this is the answer when
// the identifier is not readable.
func (r Ref) SameCluster(other Ref) (bool, error) {
	a, err := r.Normalised()
	if err != nil {
		return false, err
	}
	b, err := other.Normalised()
	if err != nil {
		return false, err
	}
	return a.Host == b.Host && a.Port == b.Port, nil
}

const (
	canonicalLoopback = "localhost"
	unixPrefix        = "unix:"
)

func normaliseHost(h string) (string, error) {
	h = strings.TrimSpace(h)
	if h == "" {
		return "", ErrNoHost
	}
	// A Unix socket directory, or Linux's abstract namespace. It is a path, not
	// a name: it is compared as itself and never folded into loopback, because
	// two clusters on one machine listen on two different socket directories.
	if strings.HasPrefix(h, "/") {
		return unixPrefix + path.Clean(h), nil
	}
	if strings.HasPrefix(h, "@") {
		return unixPrefix + h, nil
	}

	h = strings.ToLower(strings.TrimSuffix(h, "."))
	h = strings.TrimSuffix(strings.TrimPrefix(h, "["), "]")
	if h == "" {
		return "", ErrNoHost
	}
	if ip := net.ParseIP(h); ip != nil {
		if ip.IsLoopback() {
			return canonicalLoopback, nil
		}
		return ip.String(), nil
	}
	return h, nil
}

// Parse splits a connection string into the DSN handed to the driver and the
// redacted Ref everything else uses.
//
// Both spellings libpq accepts are accepted, because both are spellings a
// developer's environment already holds: the URI form
// (postgres://user:pw@host:5432/db) and the keyword/value form
// (host=... dbname=...). Parsing is pgconn's, which is the parser the driver
// will use on the same string, so the Ref describes the connection that will
// actually be made rather than a second reading of the text — including the
// PGHOST, PGPORT, PGUSER and PGDATABASE defaults libpq fills in, and the
// service file, which are exactly rung 2 of the discovery ladder
// (ARCHITECTURE.md §9).
//
// A string naming more than one host is refused with ErrMultipleHosts rather
// than reduced to its first: a Ref that describes one of several possible
// connections cannot be compared, and the comparison is the gate.
func Parse(s string) (DSN, Ref, error) {
	if strings.TrimSpace(s) == "" {
		return "", Ref{}, errors.New("dsn: empty connection string")
	}
	cfg, err := pgconn.ParseConfig(s)
	if err != nil {
		// pgconn's error quotes the string it failed on, which is the string
		// that may hold the password, so it is not wrapped.
		return "", Ref{}, errors.New("dsn: the connection string could not be parsed")
	}
	if err := singleEndpoint(cfg); err != nil {
		return "", Ref{}, err
	}
	return DSN(s), Ref{
		Host:     cfg.Host,
		Port:     int(cfg.Port),
		Database: cfg.Database,
		User:     cfg.User,
	}, nil
}

// singleEndpoint reports whether cfg describes one endpoint.
//
// pgconn puts every host of a comma-separated list into Fallbacks, and it also
// puts the primary host there twice for the TLS downgrade `sslmode=prefer`
// implies — same host, same port, a different TLSConfig. A repeat of the
// primary host:port is therefore one endpoint and is allowed; any other
// host:port is a second endpoint and is ErrMultipleHosts. The comparison is on
// the distinct host:port set for that reason, and it is unnormalised on
// purpose: a fallback that merely spells the primary differently is still a
// string this package cannot promise describes one connection.
func singleEndpoint(cfg *pgconn.Config) error {
	for _, f := range cfg.Fallbacks {
		if f == nil {
			continue
		}
		if f.Host != cfg.Host || f.Port != cfg.Port {
			return ErrMultipleHosts
		}
	}
	return nil
}

// Redact returns a connection string with no password in it, for the one case
// where the string itself must be shown.
//
// It rebuilds the string from the parsed reference rather than cutting the
// password out of the text. A textual redaction has to find the password to
// remove it, and misses it whenever the spelling is one it did not anticipate;
// rebuilding cannot carry across a field it does not name. The rebuilt string
// therefore carries the endpoint and nothing else: connection parameters
// (sslmode, application_name, and any other) are dropped, because none of them
// is what the caller wanted to show and one of them could hold a secret.
func Redact(d DSN) (string, error) {
	_, ref, err := Parse(string(d))
	if err != nil {
		return "", err
	}
	u := url.URL{Scheme: "postgres", Path: "/" + ref.Database}
	if ref.User != "" {
		u.User = url.User(ref.User)
	}
	if strings.HasPrefix(ref.Host, "/") || strings.HasPrefix(ref.Host, "@") {
		// A socket directory is not a URL host; libpq's own spelling for it is
		// the host parameter.
		u.RawQuery = url.Values{"host": {ref.Host}, "port": {strconv.Itoa(port(ref))}}.Encode()
		return u.String(), nil
	}
	u.Host = hostPort(ref)
	return u.String(), nil
}

func port(r Ref) int {
	if r.Port == 0 {
		return DefaultPort
	}
	return r.Port
}

func hostPort(r Ref) string {
	host := r.Host
	if strings.Contains(host, ":") && !strings.HasPrefix(host, "[") {
		host = "[" + host + "]"
	}
	return host + ":" + strconv.Itoa(port(r))
}
