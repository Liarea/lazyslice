// SPDX-License-Identifier: Apache-2.0

package extract

import (
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/Liarea/lazyslice/internal/event"
	"github.com/Liarea/lazyslice/internal/ref"
)

// The event codes the extract stage renders. Extractor.Extract takes no
// event.Sink — core.Run is the only producer of events (ARCHITECTURE.md §7) —
// so this is declared here, beside the refusal that carries it, and emitted by
// core from the *Refusal Extract returns. It has a row in
// internal/event/catalogue.yml, which is the source of docs/ERRORS.md.
//
// An event carries identifiers and counts and nothing else (§7): the refusal
// below names a table and a SQLSTATE, and never a row, a key or a statement.
const (
	// CodeStandbyCancelled is exit 7: the source cancelled the read because the
	// snapshot extract is holding conflicts with recovery on a standby
	// (ADR-005: "Standby cancellation is exit 7 with a retry message"). It is
	// its own code because the remedy is not the operator's SQL but the
	// standby's max_standby_streaming_delay, or a run against the primary.
	CodeStandbyCancelled event.Code = "extract.refused.standby_cancelled"
)

// exitExtract is the exit code ADR-005 assigns extract ("7 extract or load").
const exitExtract = 7

// Refusal is an extract failure core can render from the catalogue. It carries
// the code, the exit and the table the read was against; it never carries the
// statement, a key or a row.
type Refusal struct {
	Code  event.Code
	Exit  int
	Table ref.TableRef
	// SQLState is the server's five-character code, which is an identifier and
	// not a value. It is the whole of what is kept from the driver's error.
	SQLState string
}

func (r *Refusal) Error() string {
	return fmt.Sprintf("extract: %s on %s (SQLSTATE %s)", r.Code, r.Table, r.SQLState)
}

// serializationFailure is the SQLSTATE PostgreSQL raises when it cancels a
// query on a standby because the snapshot it holds conflicts with replay
// ("canceling statement due to conflict with recovery", class 40, code 40001).
const serializationFailure = "40001"

// asRefusal turns a driver error into a *Refusal when it is one extract has a
// code for, and returns nil otherwise. It reads only the SQLSTATE: a PgError's
// Detail, Where and Hint quote the row and are dropped here as everywhere
// (THREAT_MODEL.md T4).
func asRefusal(t ref.TableRef, err error) *Refusal {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return nil
	}
	if pgErr.Code != serializationFailure {
		return nil
	}
	return &Refusal{
		Code:     CodeStandbyCancelled,
		Exit:     exitExtract,
		Table:    t,
		SQLState: pgErr.Code,
	}
}
