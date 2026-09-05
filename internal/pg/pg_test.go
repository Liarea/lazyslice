package pg

import (
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
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
