// SPDX-License-Identifier: Apache-2.0

package core

import (
	"strconv"
	"strings"
	"testing"

	"github.com/Liarea/lazyslice/internal/dsn"
	"github.com/Liarea/lazyslice/internal/event"
	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/verify"
)

// verifySummary must key on each check's Code, not on its Name plus "the last
// Passed entry wins": "row_count" and "residual" each carry more than one
// Passed check (a per-table report() alongside the check's own final pass()),
// and nothing enforces that pass() is appended after report() (T-0320 fix
// round, review finding 2). This report puts every pass() first and its
// report() entries after, the opposite of counts.go/residual.go's own order,
// so a reorder-sensitive implementation would read the wrong count.
func TestVerifySummaryKeysOnCodeNotOnNameOrOrder(t *testing.T) {
	report := &pipeline.Report{
		Checks: []pipeline.Check{
			{Name: "fk", Passed: true, Code: verify.CodeFKPassed, Count: 12},
			{Name: "row_count", Passed: true, Code: verify.CodeRowCountPassed, Count: 34},
			{Name: "residual", Passed: true, Code: verify.CodeResidualPassed, Count: 56},
			{Name: "second_net", Passed: true, Code: verify.CodeSecondNetPassed, Count: 78},
			// report() entries, same Name, appended after the check's own
			// pass() above — the reverse of counts.go's and residual.go's real
			// order, and carrying different counts, so a Name-keyed "last one
			// wins" reader would report these instead of the *Passed counts.
			{Name: "row_count", Passed: true, Code: verify.CodeRowCountReported, Count: 999},
			{Name: "residual", Passed: true, Code: verify.CodeResidualExplained, Count: 999},
			{Name: "residual", Passed: true, Code: verify.CodeUnconfirmed, Count: 999},
			// A failing check must never contribute a count.
			{Name: "fk", Passed: false, Code: verify.CodeRefusedFK, Count: 999},
		},
		Rows: map[pipeline.TableRef]int64{
			{Schema: "public", Name: "a"}: 3,
			{Schema: "public", Name: "b"}: 4,
		},
	}

	var got event.Event
	r := &run{sink: event.SinkFunc(func(e event.Event) { got = e })}
	r.verifySummary(report)

	if got.Code != CodeVerifySummary {
		t.Fatalf("code = %q, want %q", got.Code, CodeVerifySummary)
	}
	want := event.Args{
		event.ArgFKCount:       "12",
		event.ArgTableCount:    "34",
		event.ArgRowCount:      "7", // sum of report.Rows, not a check count
		event.ArgResidualCount: "56",
		event.ArgColumnCount:   "78",
	}
	for k, w := range want {
		if got.Args[k] != w {
			t.Errorf("args[%s] = %q, want %q", k, got.Args[k], w)
		}
	}
}

// targetConnectLine must key the docker-exec line on targetContainerID, never
// on targetProv alone or on targetLabel: a rung-0 or ladder-found container
// candidate with no validated container (targetContainerID empty) falls back
// to the generic target.connect line, and no case ever puts a password in the
// event's Args (THREAT_MODEL.md T5) even when the DSN the candidate was
// parsed from carried one.
func TestTargetConnectLinePicksTheRightCodeAndNeverLeaksAPassword(t *testing.T) {
	_, ref, err := dsn.Parse("postgres://alice:s3cret-pw@db.internal:5432/app")
	if err != nil {
		t.Fatalf("dsn.Parse: %v", err)
	}

	cases := []struct {
		name            string
		prov            pipeline.Provenance
		containerID     string
		passwordCommand string
		wantCode        event.Code
		wantContainer   string
	}{
		{
			name:          "running container with a validated container id",
			prov:          pipeline.FromContainer,
			containerID:   "myproj-db-1",
			wantCode:      CodeTargetConnectContainer,
			wantContainer: "myproj-db-1",
		},
		{
			name:          "stopped container with a validated container id",
			prov:          pipeline.FromStoppedContainer,
			containerID:   "myproj-db-1",
			wantCode:      CodeTargetConnectContainer,
			wantContainer: "myproj-db-1",
		},
		{
			name:     "container-shaped provenance but no validated container id (rung 0)",
			prov:     pipeline.FromContainer,
			wantCode: CodeTargetConnect,
		},
		{
			name:            "password-command target",
			prov:            pipeline.FromEnvVar,
			passwordCommand: "op read op://vault/item",
			wantCode:        CodeTargetConnectPasswordCommand,
		},
		{
			name:     "everything else falls back to the generic line",
			prov:     pipeline.FromEnvVar,
			wantCode: CodeTargetConnect,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got event.Event
			r := &run{
				sink:              event.SinkFunc(func(e event.Event) { got = e }),
				targetProv:        tc.prov,
				targetContainerID: tc.containerID,
				targetCand:        pipeline.Candidate{Ref: ref},
			}
			r.req.PasswordCommand = tc.passwordCommand

			r.targetConnectLine()

			if got.Code != tc.wantCode {
				t.Errorf("code = %q, want %q", got.Code, tc.wantCode)
			}
			if got.Args[event.ArgContainer] != tc.wantContainer {
				t.Errorf("container arg = %q, want %q", got.Args[event.ArgContainer], tc.wantContainer)
			}
			for k, v := range got.Args {
				if v == "" {
					continue
				}
				if strings.Contains(v, "s3cret-pw") {
					t.Errorf("args[%s] = %q, leaked the password", k, v)
				}
			}
			if got.Args[event.ArgHost] != ref.Host || got.Args[event.ArgPort] != strconv.Itoa(ref.Port) ||
				got.Args[event.ArgDatabase] != ref.Database || got.Args[event.ArgRole] != ref.User {
				t.Errorf("connection args = %+v, want host/port/database/role from the ref", got.Args)
			}
		})
	}
}
