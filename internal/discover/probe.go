// SPDX-License-Identifier: Apache-2.0

package discover

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/Liarea/lazyslice/internal/dsn"
	"github.com/Liarea/lazyslice/internal/pg"
)

// dialBudget is ARCHITECTURE.md §9's per-candidate dial: one second, whatever
// the candidate is. A candidate that has not answered in a second is not
// eligible for anything, and the run says so rather than waiting.
const dialBudget = time.Second

// The three statements ARCHITECTURE.md §9 allows inside the dial, and no
// fourth. Emptiness is never probed table by table here; that is the gate's
// job, after discovery (THREAT_MODEL.md T2).
const (
	sqlVersion = `SELECT current_setting('server_version_num')::int / 10000`

	// One batched pg_class query for both the user-table count the gate's cap
	// is checked against and the "probably empty" hint. relpages is a planner
	// statistic, so the hint can be wrong in both directions; it renders as
	// "probably empty" and never gates a write.
	sqlTables = `SELECT count(*)::int,
       coalesce(bool_and(c.relpages = 0), true)
  FROM pg_class c
  JOIN pg_namespace n ON n.oid = c.relnamespace
 WHERE c.relkind IN ('r', 'p')
   AND n.nspname NOT IN ('pg_catalog', 'information_schema')
   AND n.nspname NOT LIKE 'pg\_toast%'`

	sqlMarker = `SELECT to_regclass('lazyslice_meta') IS NOT NULL`
)

// noPassword is the ConnectErr of a candidate that could not be connected to
// because no password was available anywhere.
//
// Discovery never prompts for one: a password prompt cannot live inside the 1 s
// dial and the ladder must not stop four times for four password-less
// candidates. Q4 is asked only for an endpoint the operator named, after the
// ladder has finished printing (ADR-008 §6).
const noPassword = "no password (see ~/.pgpass, PGPASSFILE, --password-command)"

// badPassword is the ConnectErr of a candidate that had a password and was
// refused with it.
//
// It is deliberately not noPassword. SQLSTATE 28P01 renders as "password
// authentication failed for user ...", so a match on the word "password" alone
// reports a wrong credential as a missing one and sends the operator to
// ~/.pgpass to add the entry that is already there.
const badPassword = "the password was not accepted (SQLSTATE 28P01)"

// dialShapes is the statement allowlist the dial runs under.
//
// THREAT_MODEL.md T9's control is that every statement lazyslice sends to the
// source is allowlisted and read-only, and discovery dials the source like any
// other candidate — so the three statements are registered here and the pool
// carries the tracer that refuses a fourth. Passing a tracer to pg.Connect is
// also what brings default_transaction_read_only=on, set on every connection as
// it is opened, so the invariant holds for this connection as it does for the
// source pool's. A nil tracer here would have made the invariant false and left
// invariant I4's trace with no record of the connection at all.
func dialShapes() []pg.Shape {
	return []pg.Shape{
		{Name: "discover.version", SQL: sqlVersion},
		{Name: "discover.tables", SQL: sqlTables},
		{Name: "discover.marker", SQL: sqlMarker},
	}
}

// probe dials one candidate and fills in what the three statements answer.
//
// It never returns an error: an unreachable candidate is a candidate with
// Reachable false and a sanitised ConnectErr, which is what the candidate list
// prints and what makes it ineligible for both source and target.
func probe(ctx context.Context, f *found) {
	ctx, cancel := context.WithTimeout(ctx, dialBudget)
	defer cancel()

	havePassword := passwordAvailable(f.dsn)

	tracer, err := pg.NewTracer(dialShapes()...)
	if err != nil {
		// A shape that does not compile is a bug in this file, not a property
		// of the candidate; it is reported on the candidate line rather than
		// silently dialling without an allowlist.
		f.cand.ConnectErr = "the dial allowlist could not be compiled"
		return
	}

	pool, err := pg.Connect(ctx, f.dsn, tracer)
	if err != nil {
		f.cand.ConnectErr = connectErr(err, havePassword)
		return
	}
	defer pool.Close()

	var major int
	if err := pool.QueryRow(ctx, sqlVersion).Scan(&major); err != nil {
		f.cand.ConnectErr = connectErr(err, havePassword)
		return
	}
	var tables int
	var emptyHint bool
	if err := pool.QueryRow(ctx, sqlTables).Scan(&tables, &emptyHint); err != nil {
		f.cand.ConnectErr = connectErr(err, havePassword)
		return
	}
	var marked bool
	if err := pool.QueryRow(ctx, sqlMarker).Scan(&marked); err != nil {
		f.cand.ConnectErr = connectErr(err, havePassword)
		return
	}

	f.cand.Reachable = true
	f.cand.ConnectErr = ""
	f.cand.Version = major
	f.cand.Tables = tables
	f.cand.EmptyHint = emptyHint
	f.cand.Marked = marked
	// Empty stays nil and MetaSchema stays 0: both are the gate's to fill, and
	// a discovery that guessed either would be a discovery that decided
	// eligibility (ARCHITECTURE.md §2, §9).
}

// passwordAvailable reports whether any password source could supply one for
// this candidate: the connection string itself, $PGPASSWORD, or the password
// file. pgconn.ParseConfig resolves all three, and it is the same resolution
// the dial is about to do, so this cannot disagree with what was sent.
func passwordAvailable(d dsn.DSN) bool {
	cfg, err := pgconn.ParseConfig(string(d))
	if err != nil {
		return false
	}
	return cfg.Password != ""
}

// connectErr renders a failed dial in words that carry no credential.
//
// pg.RenderAnyError is the tree's one redaction pass for a driver error; the
// password cases are recognised on top of it, because "no password" is a thing
// the developer can act on and "SASL auth failed" is not. ADR-008 §6's "no
// password" wording is reserved for the case where no password was available at
// all: havePassword is what tells a missing credential from a rejected one, and
// the two send the operator to different places.
func connectErr(err error, havePassword bool) string {
	rendered := pg.RenderAnyError(err, false)
	lower := strings.ToLower(rendered)

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "28P01": // invalid_password
			if havePassword {
				return badPassword
			}
			return noPassword
		case "28000": // invalid_authorization_specification
			// No pg_hba.conf entry, or a role that may not connect from here.
			// Naming a password would be wrong in both cases, so the server's
			// own line stands.
			return firstLine(rendered)
		}
	}

	switch {
	case strings.Contains(lower, "password"), strings.Contains(lower, "sasl"):
		// The dial failed on authentication before the server answered with a
		// SQLSTATE — a SASL exchange pgconn could not start, for instance.
		if havePassword {
			return badPassword
		}
		return noPassword
	case strings.Contains(lower, "context deadline exceeded"), strings.Contains(lower, "i/o timeout"):
		return "did not answer within 1s"
	case strings.Contains(lower, "connection refused"):
		return "connection refused"
	case strings.Contains(lower, "no such host"):
		return "host not found"
	default:
		// One line, because the candidate list is one line per candidate: a
		// driver error that wraps four dial attempts turns a ladder into a
		// wall of text and hides the candidates that did answer.
		return firstLine(rendered)
	}
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	return strings.TrimRight(strings.TrimSpace(s), ":")
}
