// SPDX-License-Identifier: Apache-2.0

//go:build integration

package pg

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Liarea/lazyslice/internal/dsn"
	"github.com/Liarea/lazyslice/internal/testutil"
)

// leaseIdleWait is how long the lease is left sitting idle in its transaction
// while the server's idle_in_transaction_session_timeout is one second. It is a
// lower bound on a wait, so a slow machine cannot flake it: the test fails only
// if the lease is gone by the time it looks, which is the server having
// terminated it.
const leaseIdleWait = 3 * time.Second

// The lease holds an open transaction for the whole run (lease.go), which puts
// it in reach of one server setting that the session-level lock it replaced was
// not: idle_in_transaction_session_timeout. A target that sets it — some managed
// services do, and so does an operator who has been bitten by a forgotten psql —
// would terminate the lease's session part-way through a run and leave the run
// holding nothing at all, with no error anywhere, because the connection is not
// used again until Release.
//
// So AcquireLease disarms it for its own transaction, locally, and this is the
// proof: one second on the database, three seconds of the lease sitting idle,
// and the lease must still be the ownership it says it is — which is asked in
// the only way that matters, by a second run that must still be refused.
func TestTheLeaseOutlivesAnIdleInTransactionTimeout(t *testing.T) {
	ctx := context.Background()
	testutil.SkipWithoutDocker(ctx, t)

	url := testutil.Postgres(ctx, t, "")
	_, r, err := dsn.Parse(url)
	if err != nil {
		t.Fatalf("parsing the container url: %v", err)
	}

	admin, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatalf("opening the admin pool: %v", err)
	}
	defer admin.Close()

	// On the database, so that every connection made after this inherits it —
	// the lease's included, which is the point.
	alter := `ALTER DATABASE ` + pgx.Identifier{r.Database}.Sanitize() +
		` SET idle_in_transaction_session_timeout = '1s'`
	if _, alterErr := admin.Exec(ctx, alter); alterErr != nil {
		t.Fatalf("setting idle_in_transaction_session_timeout on the target database: %v", alterErr)
	}

	first, err := OpenTarget(ctx, dsn.DSN(url))
	if err != nil {
		t.Fatalf("opening the target: %v", err)
	}
	defer first.Close()

	lease, err := first.AcquireLease(ctx, "run-one")
	if err != nil {
		t.Fatalf("taking the run lease on a target with an idle-in-transaction timeout: %v", err)
	}
	defer lease.Release(context.WithoutCancel(ctx))

	// The lease does nothing for the length of a run. This is that.
	time.Sleep(leaseIdleWait)

	second, err := OpenTarget(ctx, dsn.DSN(url))
	if err != nil {
		t.Fatalf("opening the second run's target: %v", err)
	}
	defer second.Close()

	stolen, err := second.AcquireLease(ctx, "run-two")
	if stolen != nil {
		defer stolen.Release(context.WithoutCancel(ctx))
	}
	var alreadyHeld *LeaseHeld
	if !errors.As(err, &alreadyHeld) {
		t.Fatalf("the second run's AcquireLease = %v, want a *LeaseHeld: the first run's lease did not "+
			"survive %s of sitting idle in its transaction, so the server's "+
			"idle_in_transaction_session_timeout terminated it and the run went on writing a target it no "+
			"longer owned. AcquireLease sets that timeout to 0 for its own transaction (lease.go)",
			err, leaseIdleWait)
	}
	if alreadyHeld.Holder != "run-one" {
		t.Errorf("the refusal names %q as the holder, want %q", alreadyHeld.Holder, "run-one")
	}
}
