// SPDX-License-Identifier: Apache-2.0

package discover

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"

	"github.com/Liarea/lazyslice/internal/dsn"
	"github.com/Liarea/lazyslice/internal/pipeline"
)

// urlNames is rung 1's name set (ARCHITECTURE.md §9). A .env value is used only
// when it is one of these names and parses whole as a Postgres URI; lazyslice
// never assembles a DSN out of DB_HOST, DB_PORT and the rest, and never uses
// .env to interpolate a compose file (ADR-008 §2).
var urlNames = []string{"DATABASE_URL", "POSTGRES_URL", "PG_URL", "DB_URL"}

// envFiles is rung 1's file set, in the order it reads them. The process
// environment wins over all of them, because that is what the run will actually
// connect with.
var envFiles = []string{".env", ".env.local", ".env.development"}

// rung1 reads $DATABASE_URL and its three aliases from the process environment
// and then from .env, .env.local and .env.development in workdir.
//
// unusable carries the values that named a rung-1 variable and were not a whole
// Postgres URI — an unexpanded ${PROD_URL}, or a host with no scheme. They are
// reported with their reason and the ladder continues (ADR-008 §2); they are
// never repaired, because a DSN assembled from fragments is how you produce a
// connection string that does not connect.
func rung1(workdir string) (out []found, unusable []unusableEnv) {
	seen := map[string]bool{}

	take := func(name, value, where string) {
		if value == "" || seen[name] {
			return
		}
		d, ref, err := dsn.Parse(value)
		if err != nil || !postgresURI(value) {
			// The name is *not* marked seen. A value that could not be used has
			// not answered for its name, and a later file may: `.env` holding
			// DATABASE_URL=${PROD_URL} — ADR-008 §2's own example — beside
			// `.env.local` holding the real URI under the same name is the
			// standard dotenv override, not an exotic shape. Claiming the name
			// here would drop the one value that connects and leave the run
			// with nothing but the warning.
			unusable = append(unusable, unusableEnv{Name: name, Where: where, Reason: envReason(value)})
			return
		}
		seen[name] = true
		out = append(out, found{
			dsn: d,
			cand: pipeline.Candidate{
				Ref:        ref,
				Provenance: pipeline.FromEnvVar,
				Label:      name,
				Local:      ref.Loopback(),
			},
		})
	}

	for _, name := range urlNames {
		take(name, os.Getenv(name), "environment")
	}
	for _, file := range envFiles {
		path := filepath.Join(workdir, file)
		values, err := readEnvFile(path)
		if err != nil {
			continue
		}
		for _, name := range urlNames {
			take(name, values[name], file)
		}
	}
	return out, unusable
}

// unusableEnv is a rung-1 name whose value could not be used, with the reason
// the candidate list prints. Reason is a category, never the value.
type unusableEnv struct {
	Name   string
	Where  string
	Reason string
}

// envReason says why a value was not used, in words that carry no part of the
// value itself: a .env line can hold a production password.
func envReason(value string) string {
	if strings.Contains(value, "${") || strings.HasPrefix(value, "$") {
		return "names a variable rather than a value"
	}
	if !postgresURI(value) {
		return "not a postgres:// URI"
	}
	return "not a usable connection string"
}

// postgresURI reports whether the value is a whole Postgres URI. A libpq
// keyword string ("host=... port=...") parses in pgconn and is deliberately not
// accepted here: ADR-008 §2 says a .env variable contributes a complete URI or
// nothing.
func postgresURI(value string) bool {
	return strings.HasPrefix(value, "postgres://") || strings.HasPrefix(value, "postgresql://")
}

// readEnvFile parses a .env file into a name/value map.
//
// It is deliberately small: KEY=VALUE, an optional "export " prefix, # comments,
// and one layer of surrounding quotes removed. It performs no interpolation of
// any kind, which is the point — an interpolating reader is the mechanism by
// which ${DB_PORT:-5432} becomes a connection string that does not connect.
func readEnvFile(path string) (map[string]string, error) {
	f, err := os.Open(filepath.Clean(path))
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	out := map[string]string{}
	s := bufio.NewScanner(f)
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		name, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		out[strings.TrimSpace(name)] = unquote(strings.TrimSpace(value))
	}
	if err := s.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func unquote(v string) string {
	if len(v) >= 2 {
		if (v[0] == '"' && v[len(v)-1] == '"') || (v[0] == '\'' && v[len(v)-1] == '\'') {
			return v[1 : len(v)-1]
		}
	}
	return v
}
