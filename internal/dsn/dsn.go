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
	"sort"
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
//
// Params carries the non-secret transport options a rerun needs to reproduce
// the connection it was built from — see AllowedParams for exactly which
// keys, and ExtractParams for how they are read out of a connection string.
// It holds no credential and no whole connection string: THREAT_MODEL.md T4,
// T5 apply to it exactly as they do to Host/Port/Database/User. Because it is
// a map, Ref is no longer comparable with ==; use IsZero for the one place
// that used to compare against the zero value.
//
// Params holds only what the connection string this Ref was Parsed from spells
// out — not what libpq's environment (PGSSLMODE, PGSSLROOTCERT, ...) or a
// PGSERVICE entry merged in on top of it. Unlike Host/Port/Database/User,
// which come from pgconn's own resolved Config and so include those defaults,
// Params comes from a second, literal reading of the string (see Parse). A
// connection made through discovery rung 2 (libpq env / service file,
// ARCHITECTURE.md §9) with sslmode set only by PGSSLMODE therefore records
// Params with no sslmode at all: this is tracked as T-0168 rather than closed
// here.
type Ref struct {
	Host     string
	Port     int
	Database string
	User     string
	Params   map[string]string
}

// String renders the reference as "user@host:port/database", the form the
// decision header and the candidate list use, followed by a parenthesised
// count of Params when it carries any. It cannot print a password because Ref
// does not hold one, and a value in Params could itself be sensitive enough
// to withhold from an ordinary log line (a certificate path names a
// filesystem layout), so String shows how many there are and never what they
// are.
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
	if n := len(r.Params); n > 0 {
		s += " (+" + strconv.Itoa(n) + " param"
		if n != 1 {
			s += "s"
		}
		s += ")"
	}
	return s
}

// IsZero reports whether r carries no data at all. Ref was comparable with ==
// before Params arrived; every place that compared against the zero value
// (there was exactly one, in internal/emit) uses this instead now.
func (r Ref) IsZero() bool {
	return r.Host == "" && r.Port == 0 && r.Database == "" && r.User == "" && len(r.Params) == 0
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
// (host=... dbname=...). Host, Port, Database and User are pgconn's, which is
// the parser the driver will use on the same string, so those four fields
// describe the connection that will actually be made rather than a second
// reading of the text — including the PGHOST, PGPORT, PGUSER and PGDATABASE
// defaults libpq fills in, and the service file, which are exactly rung 2 of
// the discovery ladder (ARCHITECTURE.md §9).
//
// Params is the one exception, and is a second reading (docs/reviews,
// 2026-09-09, finding 6; see internal/dsn/CLAUDE.md, "Decisions made during
// implementation"): pgconn.Config consumes sslmode, sslcert, sslkey and
// sslrootcert into a tls.Config and connect_timeout into a time.Duration, and
// keeps no string form of any of them, so there is nothing on *pgconn.Config
// for Params to be read from without parsing the string a second time. Because
// it is a second, literal reading of s, Params sees only what s itself spells
// out — never a setting libpq's environment or a service file merged in on
// top of it, unlike Host/Port/Database/User two paragraphs up. A connection
// string that relies on PGSSLMODE (rung 2, ARCHITECTURE.md §9) rather than
// spelling sslmode itself Parses to a Ref with no sslmode in Params at all;
// see Ref.Params's own doc comment and T-0168.
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
	params, _ := ExtractParams(s)
	return DSN(s), Ref{
		Host:     cfg.Host,
		Port:     int(cfg.Port),
		Database: cfg.Database,
		User:     cfg.User,
		Params:   params,
	}, nil
}

// ParseError re-parses s and returns pgconn's own error, unwrapped — the
// detail Parse itself discards.
//
// It exists for the one caller that already knows s cannot hold a credential:
// internal/discover's refDSNValidated (docs/reviews, 2026-09-14, finding 2),
// which builds s from a committed dsn.Ref via refDSN rather than from raw
// operator text, and a Ref cannot carry a password no matter what built it
// (THREAT_MODEL.md T4, T5) — so neither can a string assembled only from its
// fields. Parse's own doc comment explains why it cannot make that promise in
// general and returns a generic message instead: an operator-typed connection
// string reaching Parse (a bare --source, a rung-1 .env value) can hold one,
// and pgconn's error quotes the string it failed on.
//
// Nothing else may call this. A caller that cannot make the same guarantee
// about s — that it was built from a Ref and never read from operator input —
// must use Parse's own redacted error instead.
func ParseError(s string) error {
	_, err := pgconn.ParseConfig(s)
	return err
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

// AllowedParams are the only keys ExtractParams ever puts in a Ref's Params:
// the non-secret transport and TLS settings a rerun needs in order to
// reproduce the connection ARCHITECTURE.md §9's identity fields alone cannot
// describe (docs/reviews, 2026-09-09, finding 6). Never password, sslpassword,
// passfile or any key that could hold a whole connection string — those stay
// out no matter what a caller passes in, THREAT_MODEL.md T4/T5.
//
// "options" is allowlisted by key here and then checked again by value,
// through allowedOptionSettings: unlike every other entry, it is libpq's own
// channel for setting an arbitrary server GUC (docs/reviews, 2026-09-14,
// finding 5), so a key match alone is not enough for it.
var AllowedParams = []string{
	"application_name",
	"connect_timeout",
	"options",
	"sslcert",
	"sslkey",
	"sslmode",
	"sslrootcert",
}

var allowedParamSet = paramSet(AllowedParams)

// allowedOptionSettings are the only GUCs an "options" value may set.
//
// Every other key in AllowedParams is a transport or TLS setting pgconn
// consumes itself; "options" is different — libpq passes it straight through
// to the backend startup packet as "-c name=value ..." pairs, so on its own it
// is a channel for a committed lazyslice.yml to set an arbitrary server GUC on
// the session lazyslice opens against a live source or target (docs/reviews,
// 2026-09-14, finding 5): search_path, which changes unqualified name
// resolution during introspect and extract, row_security, and so on. The
// allowlist is confined to settings that make a session refuse to hang and
// change nothing about what it sees or does.
var allowedOptionSettings = paramSet([]string{
	"statement_timeout",
	"lock_timeout",
	"idle_in_transaction_session_timeout",
})

// sanitizeOptions parses an options value as libpq's own "-c name=value ..."
// syntax and returns it unchanged only if every setting it carries is on
// allowedOptionSettings. Anything else — a token that is not "-c", a value
// with no "=", a GUC not on the allowlist — fails closed and reports false:
// a value that mixes one allowed setting with one this package does not
// recognise is not obviously safe to keep half of, and this package has no
// way to tell "-c search_path=public" apart from "-c statement_timeout=5000
// -c search_path=public" without parsing the whole thing.
func sanitizeOptions(v string) bool {
	fields := strings.Fields(v)
	if len(fields) == 0 || len(fields)%2 != 0 {
		return false
	}
	for i := 0; i < len(fields); i += 2 {
		if fields[i] != "-c" {
			return false
		}
		name, _, ok := strings.Cut(fields[i+1], "=")
		if !ok || !allowedOptionSettings[strings.ToLower(name)] {
			return false
		}
	}
	return true
}

// FilterAllowedParams returns the subset of params whose (lower-cased) key is
// in AllowedParams. It exists for a Ref built from something other than a
// fresh Parse — internal/emit reading a committed lazyslice.yml is the one
// caller — so that "Ref never holds a credential" holds against that input
// too: a person can hand-edit the committed file, and a `params:` block with
// a `password:` key under it must not survive being read back in any more
// than it would have survived ExtractParams on a live connection string
// (THREAT_MODEL.md T4, T5).
//
// An "options" value that does not pass sanitizeOptions is dropped along with
// any other key that is not allowlisted at all: the committed file is text a
// person can hand-edit, and a params: block is not exempt from that check just
// because it named a key that is allowlisted in general.
func FilterAllowedParams(params map[string]string) map[string]string {
	if len(params) == 0 {
		return nil
	}
	var out map[string]string
	for k, v := range params {
		lk := strings.ToLower(k)
		if !allowedParamSet[lk] {
			continue
		}
		if lk == "options" && !sanitizeOptions(v) {
			continue
		}
		if out == nil {
			out = map[string]string{}
		}
		out[lk] = v
	}
	return out
}

// identityParams are the keys a connection string may carry that Ref already
// represents some other way, or that must never reach Params at all: the four
// identity fields, and the three ways libpq's own string form can carry a
// credential. None of these is "dropped" in ExtractParams's sense — they are
// handled, just not by Params — so they are excluded from the dropped list
// too, which exists to tell a caller about a key nothing is doing anything
// with.
var identityParams = paramSet([]string{
	"host", "hostaddr", "port",
	"dbname", "database",
	"user",
	"password", "passfile", "sslpassword",
})

func paramSet(keys []string) map[string]bool {
	m := make(map[string]bool, len(keys))
	for _, k := range keys {
		m[k] = true
	}
	return m
}

// ExtractParams reads s a second time for the parameters Parse's own
// pgconn.ParseConfig call cannot hand back (see Parse's doc comment). It
// returns the allowlisted parameters s carries, and the name of every other
// parameter s carries that is neither allowlisted nor an identity/credential
// key — the keys a caller should warn about, because they were in the
// connection string and are not in Params and are not held anywhere else
// either.
//
// A string this package cannot read a second time — one of the two forms
// Parse accepts, in a shape this simplified reader does not follow — returns
// no params and no drops rather than an error: by the time a caller has a
// string worth calling this on, Parse has already accepted it, and "no
// allowlisted param found" is a value ExtractParams can state honestly even
// when the reason is "could not tell".
func ExtractParams(s string) (params map[string]string, dropped []string) {
	settings, ok := readSettings(s)
	if !ok {
		return nil, nil
	}
	for k, v := range settings {
		switch {
		case k == "options" && !sanitizeOptions(v):
			// Allowlisted by key but not by value (allowedOptionSettings):
			// reported as dropped like any other key Params does not carry,
			// and never with the value that failed the check.
			dropped = append(dropped, k)
		case allowedParamSet[k]:
			if params == nil {
				params = map[string]string{}
			}
			params[k] = v
		case identityParams[k]:
			// Carried by Ref's other fields or by DSN itself; not a drop.
		default:
			dropped = append(dropped, k)
		}
	}
	sort.Strings(dropped)
	return params, dropped
}

// readSettings dispatches on the two connection-string forms Parse accepts
// and reads out every key=value pair either carries, lower-cased. It is
// deliberately not pgconn's own parser (unexported there, and this needs only
// the flat key/value map, not a *Config) — see Parse's doc comment on Params
// being a second reading.
func readSettings(s string) (map[string]string, bool) {
	if strings.HasPrefix(s, "postgres://") || strings.HasPrefix(s, "postgresql://") {
		return readURLSettings(s)
	}
	return readKeywordValueSettings(s)
}

func readURLSettings(s string) (map[string]string, bool) {
	u, err := url.Parse(s)
	if err != nil {
		return nil, false
	}
	out := map[string]string{}
	for k, v := range u.Query() {
		if len(v) == 0 {
			continue
		}
		out[strings.ToLower(k)] = v[0]
	}
	return out, true
}

// readKeywordValueSettings parses libpq's "key=value key='quoted value'" form.
// It follows the same grammar as pgconn's own (unexported) parser: an
// unquoted value runs to the next whitespace, a quoted one runs to the next
// unescaped quote, and \x inside either escapes x literally.
func readKeywordValueSettings(s string) (map[string]string, bool) {
	out := map[string]string{}
	rest := strings.TrimSpace(s)
	for rest != "" {
		eq := strings.IndexByte(rest, '=')
		if eq < 0 {
			return nil, false
		}
		key := strings.ToLower(strings.TrimSpace(rest[:eq]))
		rest = strings.TrimLeft(rest[eq+1:], " \t\n\r\v\f")

		var val string
		switch {
		case rest == "":
		case rest[0] == '\'':
			rest = rest[1:]
			var b strings.Builder
			closed := false
			for len(rest) > 0 {
				switch c := rest[0]; {
				case c == '\\' && len(rest) > 1:
					b.WriteByte(rest[1])
					rest = rest[2:]
				case c == '\'':
					rest = rest[1:]
					closed = true
				default:
					b.WriteByte(c)
					rest = rest[1:]
				}
				if closed {
					break
				}
			}
			if !closed {
				return nil, false
			}
			val = b.String()
		default:
			end := strings.IndexAny(rest, " \t\n\r\v\f")
			if end < 0 {
				end = len(rest)
			}
			val = rest[:end]
			rest = rest[end:]
		}
		if key != "" {
			out[key] = val
		}
		rest = strings.TrimLeft(rest, " \t\n\r\v\f")
	}
	return out, true
}
