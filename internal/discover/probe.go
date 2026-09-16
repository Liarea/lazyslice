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

// dialShapes is the statement allowlist the dial runs under: the three reads,
// plus the BEGIN and the ROLLBACK that carry them.
//
// THREAT_MODEL.md T9's control is that every statement lazyslice sends to the
// source is allowlisted and read-only, and discovery dials the source like any
// other candidate — so the three statements are registered here and the pool
// carries the tracer that refuses a fourth. A nil tracer here would have made
// the invariant false and left invariant I4's trace with no record of the
// connection at all.
//
// The read-only half is the transaction and not the connection. It was a
// session setting once — pg.Connect set default_transaction_read_only=on on
// every source connection as it was opened — and that setting is gone, because
// through a transaction-pooling PgBouncer it stayed on the pooler's shared
// server connection and left unrelated applications read-only after lazyslice
// exited (T-0076, internal/pg/pg.go). What replaced it is one
// `BEGIN ISOLATION LEVEL REPEATABLE READ READ ONLY` per call site, and between
// the two this dial had neither: three catalog reads on a source-mode pool with
// no transaction under them, on the path that dials production candidates
// (T-0081). So probe opens that transaction, and
// internal/pg's tracer now refuses any statement on an idle source connection
// rather than trusting a comment to be read (T-0082).
//
// The BEGIN and the ROLLBACK come from pg.SourceShapes rather than being
// written out here: internal/pg sends the same two literals, and two copies of
// an allowlisted literal are two things to keep in step. They are passed in
// rather than looked up again, because probe has already looked them up to send
// them and a second lookup is a second guard to keep in step with the first.
func dialShapes(begin, rollback pg.Shape) []pg.Shape {
	return []pg.Shape{
		{Name: "discover.version", SQL: sqlVersion},
		{Name: "discover.tables", SQL: sqlTables},
		{Name: "discover.marker", SQL: sqlMarker},
		begin,
		rollback,
	}
}

// transactionShapes is internal/pg's own BEGIN and ROLLBACK, taken from
// pg.SourceShapes by name so that the text this package sends is the text that
// package registered.
//
// ok is false when either name is missing, which means internal/pg renamed or
// dropped a shape this dial depends on. That is a build-time fact and not a
// property of the candidate, so probe reports it the way it reports a shape
// that will not compile: on the candidate line, without dialling. probe is the
// only caller and therefore the only guard — dialShapes takes the two shapes as
// arguments rather than looking them up a second time, because a second lookup
// is a second branch for the same impossible state, and the two would have to
// agree about what to do with it.
func transactionShapes() (begin, rollback pg.Shape, ok bool) {
	for _, s := range pg.SourceShapes() {
		switch s.Name {
		case "source.begin":
			begin = s
		case "source.rollback":
			rollback = s
		}
	}
	return begin, rollback, begin.SQL != "" && rollback.SQL != ""
}

// rollbackBudget bounds the ROLLBACK that ends the dial's transaction. It is
// spent after the dial budget has usually gone, on a context of its own, so a
// candidate that has stopped answering cannot hold the ladder open on the way
// out; the transaction ends by disconnection in that case, which is the same
// end by a worse route.
const rollbackBudget = 250 * time.Millisecond

// probe dials one candidate and fills in what the three statements answer, in
// one REPEATABLE READ READ ONLY transaction (see dialShapes for why the
// transaction and not the connection).
//
// It never returns an error: an unreachable candidate is a candidate with
// Reachable false and a sanitised ConnectErr, which is what the candidate list
// prints and what makes it ineligible for both source and target. A dial that
// broke a source rail is reported that way too, in those words rather than as a
// timeout (dialViolation), so the verdict of the check T-0082 armed is acted on
// here and not only recorded.
//
// What it returns is the tracer it dialled under, which is the only way a test
// can see what the dial sent statement by statement; no production caller needs
// it, because probe has already read its verdict.
//
// pwCmd and cache are Options.PasswordCommand and Options.pwCache. When f has
// no password from anywhere else and pwCmd is set, probe resolves it here,
// before dialBudget's context is created and before f is dialled: the command
// gets PasswordCommandTimeout's own 30 second budget rather than whatever is
// left of the 1 second dial budget (PasswordCommandTimeout's doc comment), and
// cache makes the resolution run at most once across every candidate this walk
// probes, not once per candidate.
func probe(ctx context.Context, f *found, pwCmd string, cache *passwordCache) *pg.Tracer {
	if pwCmd != "" && !passwordAvailable(f.dsn) {
		if pw, err := cache.resolve(ctx, pwCmd); err == nil && pw != "" {
			f.dsn = injectPassword(f.dsn, pw)
		}
		// A resolution failure is not reported on f here: cache.resolve has
		// already warned it once, on progress, and the candidate falls
		// through to the ordinary no-password dial below, which reports
		// noPassword on f.cand.ConnectErr exactly as it would have if
		// PasswordCommand had never been set.
	}

	ctx, cancel := context.WithTimeout(ctx, dialBudget)
	defer cancel()

	havePassword := passwordAvailable(f.dsn)

	begin, rollback, ok := transactionShapes()
	if !ok {
		f.cand.ConnectErr = "the dial's read-only transaction has no shape in internal/pg"
		return nil
	}

	tracer, err := pg.NewTracer(dialShapes(begin, rollback)...)
	if err != nil {
		// A shape that does not compile is a bug in this file, not a property
		// of the candidate; it is reported on the candidate line rather than
		// silently dialling without an allowlist.
		f.cand.ConnectErr = "the dial allowlist could not be compiled"
		return nil
	}

	pool, err := pg.Connect(ctx, f.dsn, tracer)
	if err != nil {
		f.cand.ConnectErr = dialErr(tracer, err, havePassword)
		return tracer
	}
	defer pool.Close()

	// One connection, not the pool: a transaction lives on the connection that
	// opened it, and pool.QueryRow may acquire a different one each time.
	conn, err := pool.Acquire(ctx)
	if err != nil {
		f.cand.ConnectErr = dialErr(tracer, err, havePassword)
		return tracer
	}
	defer conn.Release()

	if _, err := conn.Exec(ctx, begin.SQL); err != nil {
		f.cand.ConnectErr = dialErr(tracer, err, havePassword)
		return tracer
	}
	defer func() {
		end, endCancel := context.WithTimeout(context.WithoutCancel(ctx), rollbackBudget)
		defer endCancel()
		// Nothing to report: the transaction read three catalog rows and wrote
		// nothing, and the connection is about to be closed with the pool.
		_, _ = conn.Exec(end, rollback.SQL) //nolint:errcheck // teardown; the pool closes next and the transaction wrote nothing
	}()

	var major int
	if err := conn.QueryRow(ctx, sqlVersion).Scan(&major); err != nil {
		f.cand.ConnectErr = dialErr(tracer, err, havePassword)
		return tracer
	}
	var tables int
	var emptyHint bool
	if err := conn.QueryRow(ctx, sqlTables).Scan(&tables, &emptyHint); err != nil {
		f.cand.ConnectErr = dialErr(tracer, err, havePassword)
		return tracer
	}
	var marked bool
	if err := conn.QueryRow(ctx, sqlMarker).Scan(&marked); err != nil {
		f.cand.ConnectErr = dialErr(tracer, err, havePassword)
		return tracer
	}

	// The last check, on the path where every statement answered: a refusal
	// cancels the context pgx was going to use, so a statement the allowlist or
	// the transaction rail turned away usually surfaces as a scan error above —
	// but a rail that broke on a statement whose result nothing scanned would
	// otherwise leave a candidate reported as healthy. The tracer's verdict is
	// the authority on both, and this is where the dial stops ignoring it.
	if refused := dialViolation(tracer); refused != "" {
		f.cand.ConnectErr = refused
		return tracer
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
	return tracer
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

// dialErr renders a failed dial, asking the tracer first.
//
// internal/pg's tracer refuses a statement *by* cancelling the context pgconn
// checks before it writes, so pgx hands back a context error and nothing about
// the rail that broke: a dial that lost its BEGIN comes back "context
// canceled", which connectErr has no better word for than a driver line, and a
// dial whose statement was not on the allowlist comes back the same. Both are
// bugs in this file rather than properties of the candidate, and both are the
// exact failures T-0081 and T-0082 exist to make loud, so the tracer's verdict
// takes precedence over whatever pgx said about the cancellation.
func dialErr(tracer *pg.Tracer, err error, havePassword bool) string {
	if refused := dialViolation(tracer); refused != "" {
		return refused
	}
	return connectErr(err, havePassword)
}

// dialViolation is the tracer's verdict as a candidate line, or "" when the
// dial broke no rail.
//
// The candidate is left unreachable either way, which is the fail-safe
// direction — an unreachable candidate is eligible for neither source nor
// target — and the wording is what stops it being fail-silent: "did not answer
// within 1s" sends the operator to look at their database, and the database is
// not what is wrong.
func dialViolation(tracer *pg.Tracer) string {
	v := tracer.Violation()
	switch {
	case v == nil:
		return ""
	case errors.Is(v, pg.ErrOutsideTransaction):
		return "the dial sent a statement outside its read-only transaction (a source rail broke; this is a bug in lazyslice)"
	default:
		return "the dial sent a statement the source allowlist refused (a source rail broke; this is a bug in lazyslice)"
	}
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
	case strings.Contains(lower, "context canceled"):
		// Not a timeout: the dial's own context was cancelled, either by the
		// caller walking away or by internal/pg's tracer refusing a statement,
		// which it does by handing pgconn a context that is already done.
		// dialErr catches the second case ahead of this and names the rail;
		// this is the backstop, and it must not say "did not answer", which
		// would send the operator to a database that answered fine.
		return "the dial was cancelled"
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
