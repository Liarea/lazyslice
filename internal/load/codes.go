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
)

// The exit codes ADR-005 assigns: 7 "extract or load", 8 "FK verification".
const (
	exitLoad = 7
	exitFK   = 8
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
