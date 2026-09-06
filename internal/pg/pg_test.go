// SPDX-License-Identifier: Apache-2.0

package pg

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/Liarea/lazyslice/internal/ref"
)

// A unique-violation Detail quotes the row that collided, so it is a value
// leaving the process by way of an error message (THREAT_MODEL.md T4). This is
// the test that says the default drops it.
func TestRenderErrorDropsRowValues(t *testing.T) {
	e := &pgconn.PgError{
		Code:    "23505",
		Message: `duplicate key value violates unique constraint "users_email_key"`,
		Detail:  `Key (email)=(alice@example.com) already exists.`,
		Where:   `COPY users, line 3: "alice@example.com"`,
		Hint:    `Try alice+1@example.com.`,
	}

	got := RenderError(e, false)
	for _, secret := range []string{"alice@example.com", "alice+1@example.com", "line 3"} {
		if strings.Contains(got, secret) {
			t.Errorf("RenderError(e, false) = %q, which carries %q", got, secret)
		}
	}
	if !strings.Contains(got, "23505") || !strings.Contains(got, "users_email_key") {
		t.Errorf("RenderError(e, false) = %q, want the SQLSTATE and the constraint name", got)
	}

	// --show-row-values-in-errors is a flag whose name says what it does, so
	// with it the same fields are kept.
	withValues := RenderError(e, true)
	for _, want := range []string{"alice@example.com", "line 3", "alice+1@example.com"} {
		if !strings.Contains(withValues, want) {
			t.Errorf("RenderError(e, true) = %q, want it to carry %q", withValues, want)
		}
	}
}

func TestRenderErrorOfNil(t *testing.T) {
	if got := RenderError(nil, true); got != "" {
		t.Errorf("RenderError(nil, true) = %q, want the empty string", got)
	}
}

// notEncodable is the shape of a value pgtype cannot encode. pgtype formats it
// with %#v into the error text, which is how a row value gets into an error
// that is not a *pgconn.PgError.
type notEncodable struct{ Email string }

// failingCopier stands in for pgx on the write side.
type failingCopier struct{ err error }

func (c failingCopier) CopyFrom(context.Context, pgx.Identifier, []string, pgx.CopyFromSource) (int64, error) {
	return 0, c.err
}

// RenderError covers the Postgres errors. The write side also produces driver
// errors that are not Postgres errors, and pgx's encode error quotes the value
// it could not encode, so the copy path is the other half of the one redaction
// pass the binary has (THREAT_MODEL.md T4).
func TestCopyFromWithholdsTheDriversRowValues(t *testing.T) {
	// The text pgtype produces, reproduced rather than provoked: pgtype's
	// Map.Encode formats the offending value with %#v, pgx returns it unchanged
	// through encodeCopyValue and CopyFrom.
	driverErr := fmt.Errorf("unable to encode %#v into binary format for text (OID 25): cannot find encode plan",
		notEncodable{Email: "alice@example.com"})
	if !strings.Contains(driverErr.Error(), "alice@example.com") {
		t.Fatalf("the fixture no longer carries a value: %v", driverErr)
	}

	table := ref.TableRef{Schema: "public", Name: "customers"}
	rows := make(chan []any)
	close(rows)
	_, err := copyFrom(context.Background(), failingCopier{err: driverErr}, table, []string{"email"}, rows)
	if err == nil {
		t.Fatal("copyFrom returned no error")
	}

	for _, rendered := range []string{err.Error(), RenderAnyError(err, false)} {
		if strings.Contains(rendered, "alice@example.com") {
			t.Errorf("the copy error prints %q, which carries the row value", rendered)
		}
		if !strings.Contains(rendered, "public.customers") {
			t.Errorf("the copy error prints %q, want it to still name the table", rendered)
		}
	}

	// --show-row-values-in-errors is a flag whose name says what it does, and it
	// is the only thing that reaches the driver's own words.
	if withValues := RenderAnyError(err, true); !strings.Contains(withValues, "alice@example.com") {
		t.Errorf("RenderAnyError(err, true) = %q, want the driver's message", withValues)
	}
}

// A Postgres error is not withheld: RenderError already drops the fields that
// quote a row, and the SQLSTATE is the half worth printing.
func TestCopyFromKeepsAPostgresError(t *testing.T) {
	pgErr := &pgconn.PgError{Code: "23505", Message: `duplicate key value violates unique constraint "customers_pkey"`}
	rows := make(chan []any)
	close(rows)
	_, err := copyFrom(context.Background(), failingCopier{err: pgErr}, ref.TableRef{Schema: "public", Name: "customers"},
		[]string{"id"}, rows)

	var got *pgconn.PgError
	if !errors.As(err, &got) {
		t.Fatalf("copyFrom(%v) = %v; want the PgError still reachable for the exit code", pgErr, err)
	}
	if rendered := RenderAnyError(err, false); !strings.Contains(rendered, "23505") {
		t.Errorf("RenderAnyError = %q, want the SQLSTATE", rendered)
	}
}
