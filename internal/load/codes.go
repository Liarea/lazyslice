// SPDX-License-Identifier: Apache-2.0

package load

import (
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/Liarea/lazyslice/internal/event"
	"github.com/Liarea/lazyslice/internal/ref"
)

// The event codes the load stage renders.
//
// Loader.Load takes no event.Sink in ARCHITECTURE.md section 2, because
// core.Run is the only producer of events (section 7). Two of the codes below
// are nevertheless sent from inside this package through the sink core hands to
// New: section 11.1 requires every drop to be printed *before* it happens, and a
// list returned at the end of the stage is a list printed after the table is
// gone. The refusals are carried in the error Load returns and rendered by core
// from the catalogue, exactly as internal/extract and internal/plan do.
//
// Every code here has a row in internal/event/catalogue.yml, which is the source
// of docs/ERRORS.md. An event carries identifiers and counts and nothing else
// (THREAT_MODEL.md T4): the refusals below name a table, an object and a
// SQLSTATE, and never a row, a key or a statement.
const (
	// CodeDropping is the line printed before each table of the target is
	// dropped (ARCHITECTURE.md section 11.1, ADR-005 "Target").
	CodeDropping event.Code = "load.target.dropping"

	// CodeQuarantineDropping is the line printed before each table DropLoaded
	// drops, after core found the target holds personal data on a
	// residual-class verify failure (T-0133, 2026-09-14). It is a code of its
	// own rather than a reuse of CodeDropping because the two happen at
	// different, easily confused moments — before the rows go in, and after a
	// verify failure says they should not have — and a transcript reader
	// deserves to be able to tell "the ordinary reload truncation" from "your
	// last run leaked and this is the cleanup" apart.
	CodeQuarantineDropping event.Code = "load.target.quarantine_dropping"

	// CodeQuarantineDroppingObject is the same line for a non-table object the
	// run created — a domain, an enum type, a sequence (the 2026-09-15 red
	// team's A07). It is its own code because the quarantine's promise is that
	// the target ends the run holding nothing this run wrote, and a transcript
	// that named only the tables was the evidence for a promise it was not
	// keeping: the domain whose CHECK carried the address verify had just
	// refused the run over was still there, unmentioned.
	CodeQuarantineDroppingObject event.Code = "load.target.quarantine_dropping_object"

	// CodeTableLoaded reports one table's committed row count.
	CodeTableLoaded event.Code = "load.table.loaded"

	// CodeRefusedDDL is exit 7: the target refused a statement recreating the
	// schema.
	CodeRefusedDDL event.Code = "load.refused.ddl"

	// CodeRefusedCopy is exit 7: the rows of one table did not go in.
	CodeRefusedCopy event.Code = "load.refused.copy"

	// CodeRefusedFKInvalid is exit 8: a foreign key was created and does not
	// validate, so the slice is not referentially complete. ADR-005's exit
	// table gives foreign-key verification its own code, and this is that
	// failure found one stage earlier than verify.
	CodeRefusedFKInvalid event.Code = "load.refused.fk_invalid"

	// The three refusals of ARCHITECTURE.md section 11.2's lock-and-recheck
	// (T-0130, 2026-09-14). All three are exit 4 and not exit 7, because none of
	// them is a load that went wrong: each is the target turning out not to be
	// the database the gate approved, which is the same refusal the gate itself
	// makes, arriving later because the evidence arrived later.

	// CodeRefusedTargetLocked is exit 4: the table about to be dropped could not
	// be locked without waiting, so something else is using the target.
	CodeRefusedTargetLocked event.Code = "load.refused.target_locked"

	// CodeRefusedTargetChanged is exit 4: the gate approved this table because
	// it was empty and, under the drop's own ACCESS EXCLUSIVE lock, it is not.
	// Somebody wrote to the target after it was approved, and their rows are
	// not ours to delete.
	CodeRefusedTargetChanged event.Code = "load.refused.target_changed"

	// CodeRefusedMarkerChanged is exit 4: the gate approved the truncation
	// because a bound marker row authorised it, and that row is gone or has
	// changed since. The authorisation is the row, so a row that is not the one
	// the gate read authorises nothing.
	CodeRefusedMarkerChanged event.Code = "load.refused.marker_changed"
)

// The exit codes ADR-005 assigns: 4 "target refused", 7 "extract or load",
// 8 "FK verification".
const (
	exitTarget = 4
	exitLoad   = 7
	exitFK     = 8
)

// Refusal is a load failure core can render from the catalogue. It carries the
// code, the exit, the table and the object the statement named; it never carries
// the statement, a key or a row.
type Refusal struct {
	Code  event.Code
	Exit  int
	Table ref.TableRef
	// Object is an index, constraint or sequence name — never a value.
	Object string
	// SQLState is the server's five-character code, which is an identifier and
	// not a value. It is the whole of what is kept from the driver's error.
	SQLState string
	// Rows is the row count a lock-and-recheck refusal names (T-0130): the
	// number of rows the table held when the gate had approved it as empty. It
	// is a count and therefore printable (THREAT_MODEL.md T4); it is zero for
	// every other refusal, whose templates do not reference it.
	Rows int64
	// err is the underlying error, kept so that a caller which needs the
	// driver's own words (--show-row-values-in-errors) can still reach them.
	err error
}

func (r *Refusal) Error() string {
	where := r.Table.String()
	if r.Object != "" {
		where += " (" + r.Object + ")"
	}
	if r.SQLState != "" {
		return fmt.Sprintf("load: %s on %s (SQLSTATE %s)", r.Code, where, r.SQLState)
	}
	return fmt.Sprintf("load: %s on %s: %v", r.Code, where, r.err)
}

// Unwrap gives access to the driver's error. internal/pg has already replaced
// the text of a copy error that would quote a row (THREAT_MODEL.md T4), so what
// is reachable here is either a *pgconn.PgError, a context error or an error
// whose own text is already safe.
func (r *Refusal) Unwrap() error { return r.err }

// refuse builds a *Refusal, keeping the SQLSTATE and dropping every other field
// of a PgError: Detail, Where and Hint quote the offending row.
func refuse(code event.Code, exit int, table ref.TableRef, object string, err error) *Refusal {
	r := &Refusal{Code: code, Exit: exit, Table: table, Object: object, err: err}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		r.SQLState = pgErr.Code
	}
	return r
}

// refuseChanged builds the lock-and-recheck refusal: exit 4, the table, and the
// count that says what changed. rows is a number and nothing else.
func refuseChanged(code event.Code, table ref.TableRef, rows int64, because string) *Refusal {
	r := refuse(code, exitTarget, table, "", errors.New(because))
	r.Rows = rows
	return r
}

// sqlStateLockNotAvailable is 55P03, which is the only thing LOCK TABLE ...
// NOWAIT raises when the lock is simply held by somebody else.
//
// Every other failure of that statement is a load failure and not a contended
// target, and telling them apart matters twice: CodeRefusedTargetLocked is exit
// 4 with a message that says "something else is using this database", and
// dropTable retries it three times. sqlTableExists asks to_regclass, which
// answers non-NULL for any relation kind, so the statement can also raise 42809
// (a sequence or an index under that name: "is not a table"), 42501
// (insufficient privilege) or 42P01 (the relation went away between the
// existence check and the LOCK). None of those becomes free by waiting a tenth
// of a second, and none of them means another session holds the target.
const sqlStateLockNotAvailable = "55P03"

// lockNotAvailable reports whether err is the server saying the lock was taken.
// An error carrying no SQLSTATE at all — a dropped connection, a cancelled
// context — is not it either: those are the load failing, which is exit 7.
func lockNotAvailable(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == sqlStateLockNotAvailable
}
