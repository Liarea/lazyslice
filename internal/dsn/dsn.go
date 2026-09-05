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
// Scaffold status: the types and the fingerprint are real; Parse is a no-op.
package dsn

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strconv"
)

// ErrNotImplemented is returned by every placeholder in the scaffold.
var ErrNotImplemented = errors.New("dsn: not implemented")

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
func (r Ref) Fingerprint() string {
	sum := sha256.Sum256([]byte(r.Host + ":" + strconv.Itoa(r.Port) + "/" + r.Database))
	return hex.EncodeToString(sum[:8])
}

// SameEndpoint reports whether two references name the same normalised
// host:port/database. The target gate refuses when the target and the source
// answer true (ARCHITECTURE.md section 9 rule 2).
func (r Ref) SameEndpoint(o Ref) bool {
	return Loopback(r.Host) == Loopback(o.Host) && r.Port == o.Port && r.Database == o.Database
}

// Loopback normalises the several spellings of the local machine to one, so
// that "localhost" and "127.0.0.1" are recognised as the same endpoint by the
// gate and as local by the locality rule.
//
// Scaffold status: no-op, returns h unchanged.
func Loopback(h string) string { return h }

// Parse splits a connection string into the DSN handed to the driver and the
// redacted Ref everything else uses.
//
// Scaffold status: no-op, returns ErrNotImplemented.
func Parse(_ string) (DSN, Ref, error) {
	return "", Ref{}, ErrNotImplemented
}

// Redact returns the connection string with the password removed, for the one
// case where the string itself must be shown.
//
// Scaffold status: no-op, returns ErrNotImplemented.
func Redact(_ DSN) (string, error) {
	return "", ErrNotImplemented
}
