// SPDX-License-Identifier: Apache-2.0

package discover

import (
	"os"

	"github.com/Liarea/lazyslice/internal/dsn"
	"github.com/Liarea/lazyslice/internal/pipeline"
)

// libpqNames are the settings whose presence means the developer has told libpq
// where their database is. PGPASSWORD and PGPASSFILE are deliberately not among
// them: a password on its own names no endpoint.
var libpqNames = []string{"PGSERVICE", "PGHOST", "PGHOSTADDR", "PGDATABASE", "PGPORT", "PGUSER"}

// rung2 is the libpq environment: PG* and PGSERVICE (ARCHITECTURE.md §9).
//
// The reference is taken from pgconn's own parse of an empty URI, through
// dsn.Parse, rather than from a second reading of the variables here: pgconn
// fills in the PGHOST, PGPORT, PGUSER and PGDATABASE defaults and resolves the
// service file, so the Ref describes the connection that will actually be made.
func rung2() []found {
	if !anySet(libpqNames) {
		return nil
	}
	d, ref, err := dsn.Parse("postgres://")
	if err != nil || ref.Host == "" || ref.Database == "" {
		return nil
	}
	return []found{{
		dsn: d,
		cand: pipeline.Candidate{
			Ref:        ref,
			Provenance: pipeline.FromLibpq,
			Label:      setName(libpqNames),
			Local:      ref.Loopback(),
		},
	}}
}

func anySet(names []string) bool {
	for _, n := range names {
		if os.Getenv(n) != "" {
			return true
		}
	}
	return false
}

// setName is the first libpq variable that was actually set, which is what the
// candidate list prints as the provenance.
func setName(names []string) string {
	for _, n := range names {
		if os.Getenv(n) != "" {
			return n
		}
	}
	return ""
}
