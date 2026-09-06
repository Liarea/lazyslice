// SPDX-License-Identifier: Apache-2.0

//go:build integration

package verify

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/Liarea/lazyslice/internal/classify"
	"github.com/Liarea/lazyslice/internal/dsn"
	"github.com/Liarea/lazyslice/internal/event"
	"github.com/Liarea/lazyslice/internal/extract"
	"github.com/Liarea/lazyslice/internal/introspect"
	"github.com/Liarea/lazyslice/internal/load"
	"github.com/Liarea/lazyslice/internal/pg"
	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/plan"
	"github.com/Liarea/lazyslice/internal/ref"
	"github.com/Liarea/lazyslice/internal/testutil"
	"github.com/Liarea/lazyslice/internal/transform"
	"github.com/Liarea/lazyslice/mask"
)

// What verify says about a target is a statement about two real databases: the
// residual filter is built by the real transform over the real classification,
// the target is written by the real loader, and the confirmation probes go to a
// real source through the real statement allowlist. So these run the whole
// pipeline over testdata/pagila and then hand the result to Verify.
//
// One green case and one per failure §6 can produce, because a check nobody has
// ever seen fail is a check nobody has tested:
//   - a correct target passes every check;
//   - a target holding one source email in a column the run masked fails the
//     residual scan, naming the table and the column (tracker T-0035, the
//     negative control: without it a Verify that returned nil would pass every
//     invariant in internal/invariants);
//   - the same target with no probe budget fails as *unconfirmable*, which is
//     the branch that proves this stage fails closed rather than printing a
//     note;
//   - a target holding unmasked personal data in a column the classifier did
//     not mask fails the second net;
//   - a target whose rows no longer satisfy a foreign key fails FK validation;
//   - a target holding one row too many fails the row count;
//   - a target whose sequence was rolled back fails the sequence check.

// The slice. customer is the root CONCEPT.md's own example uses, and at 50 rows
// it reaches address, city, country, store, staff, film, inventory, rental and
// the partitioned payment table — and, for this suite, customer.email,
// customer.first_name, address.phone and staff.password, which are the columns
// the classifier masks.
var (
	rootTable = ref.TableRef{Schema: "public", Name: "customer"}
	take      = 50
)

// run is one whole pipeline over one pair of containers.
type run struct {
	sourceURL, targetURL string
	src                  *pg.Source
	schema               *pipeline.Schema
	cls                  *pipeline.Classification
	plan                 *pipeline.Plan
	res                  pipeline.Residual
	lr                   *pipeline.LoadResult
	writer               pipeline.Writer
}

// readableWriter is a pipeline.Writer that can also be read from.
//
// ARCHITECTURE.md §2's Writer has Exec, CopyFrom and Begin and no Query, and
// every check in §6 is a read of the target, so Verify asks the Writer it is
// handed for a Query method (verify.go, "Reading the target"). internal/pg's
// writer does not have one yet; this is what core will need there, and until it
// is added this test is the only thing that supplies it.
type readableWriter struct {
	pipeline.Writer
	conn *pgx.Conn
}

func (w readableWriter) Query(ctx context.Context, sql string, args ...any) (pipeline.Rows, error) {
	return w.conn.Query(ctx, sql, args...)
}

// schemaSampler is pipeline.Sampler over the rows introspect already took.
type schemaSampler struct{ schema *pipeline.Schema }

func (s schemaSampler) Samples(c ref.ColumnRef) []any {
	for i := range s.schema.Tables {
		t := &s.schema.Tables[i]
		if t.Ref != c.Table {
			continue
		}
		idx := -1
		for j, col := range t.Columns {
			if col.Name == c.Column {
				idx = j
				break
			}
		}
		if idx < 0 {
			return nil
		}
		out := make([]any, 0, len(t.Samples))
		for _, row := range t.Samples {
			if idx < len(row) {
				out = append(out, row[idx])
			}
		}
		return out
	}
	return nil
}

func introspectAndPlanShapes() []pg.Shape {
	var out []pg.Shape
	for _, s := range introspect.Shapes() {
		out = append(out, pg.Shape{Name: s.Name, SQL: s.SQL})
	}
	for _, s := range plan.Shapes() {
		out = append(out, pg.Shape{Name: s.Name, SQL: s.SQL})
	}
	return out
}

// pipelineRun loads pagila into a source container, runs introspect, classify,
// plan, extract, transform and load into a target container, releases the
// snapshot and returns everything Verify is given.
func pipelineRun(ctx context.Context, t *testing.T) *run {
	t.Helper()
	testutil.SkipWithoutDocker(ctx, t)

	sourceURL := testutil.Postgres(ctx, t, "")
	if err := testutil.LoadPagila(ctx, sourceURL); err != nil {
		t.Fatalf("loading pagila: %v", err)
	}
	targetURL := testutil.Postgres(ctx, t, "")

	src, err := pg.OpenSource(ctx, dsn.DSN(sourceURL), introspectAndPlanShapes()...)
	if err != nil {
		t.Fatalf("opening the source: %v", err)
	}
	t.Cleanup(src.Close)

	id, err := src.Snapshot(ctx)
	if err != nil {
		t.Fatalf("exporting the snapshot: %v", err)
	}
	released := false
	t.Cleanup(func() {
		if !released {
			_ = src.Release(context.WithoutCancel(ctx))
		}
	})
	reader, err := src.Reader(ctx, id)
	if err != nil {
		t.Fatalf("opening a reader on the snapshot: %v", err)
	}
	// Closed here as well as on the happy path below. t.Fatalf anywhere between
	// the two runs the cleanups in reverse, and pg.Source.Close waits on the
	// pool: a reader still checked out turns any failure in this function into a
	// hang until the test binary's own timeout, which is how the transform
	// blocker above presented itself rather than as the error it is.
	readerClosed := false
	t.Cleanup(func() {
		if !readerClosed {
			_ = reader.Close(context.WithoutCancel(ctx))
		}
	})

	schema, err := introspect.New().Introspect(ctx, reader)
	if err != nil {
		t.Fatalf("introspecting: %v", err)
	}
	cls, err := classify.New().Classify(schema, schemaSampler{schema: schema}, nil)
	if err != nil {
		t.Fatalf("classifying: %v", err)
	}
	root := rootTable
	p, err := plan.New().Plan(ctx, reader, schema, cls, pipeline.PlanRequest{Root: &root, Take: take})
	if err != nil {
		t.Fatalf("planning: %v", err)
	}

	var extra []pg.Shape
	for _, s := range extract.Shapes(p) {
		extra = append(extra, pg.Shape{Name: s.Name, SQL: s.SQL})
	}
	for _, s := range Shapes(p, cls) {
		extra = append(extra, pg.Shape{Name: s.Name, SQL: s.SQL})
	}
	if regErr := src.Register(extra...); regErr != nil {
		t.Fatalf("registering the extract and verify shapes: %v", regErr)
	}

	target, err := pg.OpenTarget(ctx, dsn.DSN(targetURL))
	if err != nil {
		t.Fatalf("opening the target: %v", err)
	}
	t.Cleanup(target.Close)
	w, err := target.Writer(ctx)
	if err != nil {
		t.Fatalf("opening a target writer: %v", err)
	}

	key, err := mask.NewKey()
	if err != nil {
		t.Fatalf("making a masking key: %v", err)
	}
	res := transform.NewResidual(p.Estimate.Rows * int64(len(cls.Decisions)))

	batches := make(chan pipeline.RowBatch, 8)
	masked := make(chan pipeline.RowBatch, 8)
	extractErr := make(chan error, 1)
	transformErr := make(chan error, 1)
	loadDone := make(chan struct{})
	go func() { extractErr <- extract.New(schema).Extract(ctx, reader, p, batches) }()
	// The transform goroutine keeps draining after a failure, on both sides. A
	// masker refusal that stopped it would otherwise leave extract blocked on a
	// channel nobody reads, and a loader that returned early would leave this
	// blocked on one — either way the failure would be a hung test rather than
	// the error that caused it.
	go func() {
		defer close(masked)
		tr := transform.New(schema)
		var first error
		for b := range batches {
			if first != nil {
				continue
			}
			out, maskErr := tr.Transform(b, cls, &key, res)
			if maskErr != nil {
				first = maskErr
				continue
			}
			select {
			case masked <- out:
			case <-loadDone:
				first = errors.New("the loader stopped reading the batches")
			}
		}
		transformErr <- first
	}()

	_, sourceRef, err := dsn.Parse(sourceURL)
	if err != nil {
		t.Fatalf("parsing the source url: %v", err)
	}
	lr, loadErr := load.New(load.Run{
		ToolVersion:               "test",
		SourceFingerprint:         sourceRef.Fingerprint(),
		ClassificationFingerprint: cls.Fingerprint,
		SecretFingerprint:         key.Fingerprint(),
	}, nil).Load(ctx, w, p, schema, masked)
	close(loadDone)
	if err := <-extractErr; err != nil {
		t.Fatalf("extract: %v", err)
	}
	if err := <-transformErr; err != nil {
		var refusal *transform.Refusal
		if errors.As(err, &refusal) {
			t.Fatalf("transform refused %s: %v", refusal.Col, err)
		}
		t.Fatalf("transform: %v", err)
	}
	if loadErr != nil {
		t.Fatalf("load: %v", loadErr)
	}

	// ARCHITECTURE.md §6 item 2: extract finishes and the run snapshot is
	// released before verify starts. Everything verify reads from the source
	// after this goes through Source.Short.
	if err := reader.Close(ctx); err != nil {
		t.Fatalf("closing the snapshot reader: %v", err)
	}
	readerClosed = true
	if err := src.Release(ctx); err != nil {
		t.Fatalf("releasing the snapshot: %v", err)
	}
	released = true

	return &run{
		sourceURL: sourceURL, targetURL: targetURL, src: src,
		schema: schema, cls: cls, plan: p, res: res, lr: lr,
		writer: readableWriter{Writer: w, conn: connect(ctx, t, targetURL)},
	}
}

func (r *run) verify(ctx context.Context) (*pipeline.Report, error) {
	return r.verifyWith(ctx, Options{})
}

func (r *run) verifyWith(ctx context.Context, o Options) (*pipeline.Report, error) {
	return New(o).Verify(ctx, r.src, r.writer, r.schema, r.plan, r.cls, r.res, r.lr)
}

func connect(ctx context.Context, t *testing.T, url string) *pgx.Conn {
	t.Helper()
	conn, err := pgx.Connect(ctx, url)
	if err != nil {
		t.Fatalf("connecting: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close(context.WithoutCancel(ctx)) })
	return conn
}

// maskedColumn is a column of the slice the classifier masked, preferring
// public.customer.email: it is the column tracker T-0035 names, and a fixture
// whose email column stopped being masked would make the negative control
// vacuous.
func maskedColumn(t *testing.T, r *run) ref.ColumnRef {
	t.Helper()
	want := ref.ColumnRef{Table: rootTable, Column: "email"}
	if d, ok := r.cls.Decisions[want]; ok && d.Masked {
		return want
	}
	t.Fatalf("the classifier did not mask %s, so the negative control would prove nothing", want)
	return ref.ColumnRef{}
}

func checkNamed(report *pipeline.Report, name string) (pipeline.Check, bool) {
	for _, c := range report.Checks {
		if c.Name == name && !c.Passed {
			return c, true
		}
	}
	return pipeline.Check{}, false
}

// passedCodes is the one code each check emits to say it found nothing.
//
// The other Passed lines a report carries are §6 item 5's *reported* cases — a
// step the plan gives no count to check, a sequence owned by no column, a
// sample that could not be fetched, residual hits the source says are absent.
// Those are neither a pass nor a failure and they legitimately sit beside
// either, which is why this is keyed on the code and not on Passed alone.
var passedCodes = map[string]event.Code{
	checkResidual:       CodeResidualPassed,
	checkUnconfirmable:  CodeResidualProbes,
	checkSecondNet:      CodeSecondNetPassed,
	checkFK:             CodeFKPassed,
	checkRowCount:       CodeRowCountPassed,
	checkSequences:      CodeSequencesPassed,
	checkUnmaskedSameAs: CodeSamplePassed,
}

// noCheckBothWays is the rule all seven checks follow: a check's "passed" line
// is never appended beside a failure of the same name. Report.Checks is what the
// run prints, so a passing line under a failing one of the same name is a green
// tick on the very check that produced the exit code. This is asserted on every
// failing report below, because it is invisible on a green one.
func noCheckBothWays(t *testing.T, report *pipeline.Report) {
	t.Helper()
	failed := map[string]bool{}
	for _, c := range report.Checks {
		if !c.Passed {
			failed[c.Name] = true
		}
	}
	for _, c := range report.Checks {
		if !c.Passed || !failed[c.Name] || c.Code != passedCodes[c.Name] {
			continue
		}
		t.Errorf("the report carries %q both passed (%s) and failed; a reader sees a green tick "+
			"on the check that produced exit %d", c.Name, c.Code, report.ExitCode)
	}
}

// unmaskedTextColumn is a column of a loaded table the run left unmasked and the
// second net reads, which is where planted personal data has to go for the net
// to be the thing that catches it.
func unmaskedTextColumn(t *testing.T, r *run, want ref.ColumnRef) ref.ColumnRef {
	t.Helper()
	if d, ok := r.cls.Decisions[want]; ok && (d.Masked || optedOut(d)) {
		t.Fatalf("the classifier masked or opted out %s, so the second net would never read it", want)
	}
	return want
}

// A correct target passes every check. This is the case the others are read
// against: without it, a residual scan that fails on everything would look like
// a working control.
func TestVerifyPassesACorrectTarget(t *testing.T) {
	ctx := context.Background()
	r := pipelineRun(ctx, t)

	report, err := r.verify(ctx)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if report.ExitCode != 0 {
		t.Errorf("a correct target exits %d", report.ExitCode)
	}
	for _, c := range report.Checks {
		if !c.Passed {
			t.Errorf("check %q failed on a correct target: %s on %s.%s (%d)",
				c.Name, c.Code, c.Table, c.Column, c.Count)
		}
	}

	// Every check in §6 must actually have run: a report of nothing is what
	// tracker T-0035 was opened about.
	for _, name := range order {
		found := false
		for _, c := range report.Checks {
			if c.Name == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("the report carries no %q check at all", name)
		}
	}
	// A check that looked at nothing is not a check that passed. Without this,
	// a classification with no masked column and a plan with no steps would
	// produce the same green report as a correct run — which is the shape of
	// the gap tracker T-0035 was opened about. The count is on the check's own
	// "passed" line; the reported lines beside it (a schema-only step holding
	// no rows, a lookup with none in the source) legitimately count zero.
	didSomething := map[string]event.Code{
		checkResidual:  CodeResidualPassed,
		checkSecondNet: CodeSecondNetPassed,
		checkRowCount:  CodeRowCountPassed,
		checkFK:        CodeFKPassed,
	}
	for name, code := range didSomething {
		looked := int64(0)
		for _, c := range report.Checks {
			if c.Name == name && c.Code == code {
				looked = c.Count
			}
		}
		if looked == 0 {
			t.Errorf("check %q passed over nothing at all", name)
		}
	}

	// The classify/transform defect T-0054 fixed used to force this run through
	// a per-column --unmask prior on five pagila columns; Classify above is
	// handed a nil prior, so this is the assertion that the fix holds: fulltext
	// is masked as derived_text rather than dying at transform, and the four
	// last_update columns the same defect touched are copied unmasked rather
	// than opted out.
	fulltext := ref.ColumnRef{Table: ref.TableRef{Schema: "public", Name: "film"}, Column: "fulltext"}
	if d, ok := r.cls.Decisions[fulltext]; !ok || d.Category != pipeline.CatDerivedText || !d.Masked {
		t.Errorf("public.film.fulltext decided %v masked=%v, want derived_text masked", d.Category, d.Masked)
	}
	target := connect(ctx, t, r.targetURL)
	var total, nonEmpty int
	if err := target.QueryRow(ctx,
		`SELECT count(*), count(*) FILTER (WHERE fulltext::text <> '') FROM public.film`,
	).Scan(&total, &nonEmpty); err != nil {
		t.Fatalf("reading target film.fulltext: %v", err)
	}
	if total == 0 {
		t.Fatalf("the slice copied no film rows, so the derived_text assertion proves nothing")
	}
	if nonEmpty != 0 {
		t.Errorf("%d rows of public.film.fulltext hold a non-empty tsvector, want every row masked to the empty one", nonEmpty)
	}
	for _, table := range []string{"actor", "address", "category", "film"} {
		col := ref.ColumnRef{Table: ref.TableRef{Schema: "public", Name: table}, Column: "last_update"}
		d, ok := r.cls.Decisions[col]
		switch {
		case !ok:
			t.Errorf("%s: no decision for last_update, want it decided unmasked", col)
		case d.Masked:
			t.Errorf("%s decided masked (%s), want last_update copied unmasked", col, d.Reason)
		}
	}

	t.Logf("checks=%d rows=%d probes=%d unconfirmed=%d elapsed=%s",
		len(report.Checks), len(report.Rows), report.Probes, report.Unconfirmed, report.Elapsed)
}

// The negative control (tracker T-0035, ARCHITECTURE.md §6 items 1 to 3).
//
// A value the run masked is put back into the target exactly as the source
// holds it — which is what a masker that passed a value through would have left
// there (THREAT_MODEL.md T12) — and verify must find it, confirm it against the
// source and exit 9 naming the table and the column.
func TestVerifyFailsOnASourceValueInAMaskedColumn(t *testing.T) {
	ctx := context.Background()
	r := pipelineRun(ctx, t)
	col := maskedColumn(t, r)

	// A value the run masked: the email of a customer the slice copied, read
	// from the source itself.
	source := connect(ctx, t, r.sourceURL)
	var id int32
	var email string
	if err := source.QueryRow(ctx,
		`SELECT customer_id, email FROM public.customer WHERE email IS NOT NULL ORDER BY customer_id LIMIT 1`,
	).Scan(&id, &email); err != nil {
		t.Fatalf("reading a source email: %v", err)
	}

	target := connect(ctx, t, r.targetURL)
	tag, err := target.Exec(ctx,
		`UPDATE public.customer SET email = $1 WHERE customer_id = $2`, email, id)
	if err != nil {
		t.Fatalf("planting the source email in the target: %v", err)
	}
	if tag.RowsAffected() != 1 {
		t.Fatalf("planted the email in %d rows; the slice does not hold customer %d", tag.RowsAffected(), id)
	}

	report, err := r.verify(ctx)
	if report == nil {
		t.Fatalf("Verify returned no report: %v", err)
	}
	if report.ExitCode != 9 {
		t.Fatalf("exit code %d, want 9 (residual personal data)", report.ExitCode)
	}
	var refusal *Refusal
	if !errors.As(err, &refusal) {
		t.Fatalf("Verify returned %v, want a *Refusal", err)
	}
	if refusal.Code != CodeRefusedResidual {
		t.Errorf("code %s, want %s", refusal.Code, CodeRefusedResidual)
	}
	if refusal.Table != col.Table || refusal.Column != col.Column {
		t.Errorf("the refusal names %s.%s, want %s", refusal.Table, refusal.Column, col)
	}
	if refusal.Error() == "" || strings.Contains(refusal.Error(), email) {
		t.Errorf("the refusal quotes the value it found; it may name only the table and the column")
	}
	c, ok := checkNamed(report, checkResidual)
	if !ok {
		t.Fatal("the report carries no failing residual check")
	}
	if c.Table != col.Table || c.Column != col.Column {
		t.Errorf("the residual check names %s.%s, want %s", c.Table, c.Column, col)
	}
	if report.Probes == 0 {
		t.Error("the confirmation sent no probe to the source, so the hit was never confirmed")
	}
	noCheckBothWays(t, report)
}

// A target whose rows no longer satisfy a foreign key fails FK validation with
// ADR-005's exit 8, naming the constraint.
//
// The constraint itself is dropped first, which is the point: this check counts
// orphan rows rather than reading pg_constraint.convalidated, so it holds even
// when the catalogue says everything is fine.
func TestVerifyFailsOnABrokenForeignKey(t *testing.T) {
	ctx := context.Background()
	r := pipelineRun(ctx, t)

	target := connect(ctx, t, r.targetURL)
	if _, err := target.Exec(ctx,
		`ALTER TABLE public.customer DROP CONSTRAINT customer_address_id_fkey`); err != nil {
		t.Fatalf("dropping the constraint: %v", err)
	}
	tag, err := target.Exec(ctx,
		`UPDATE public.customer SET address_id = 999999
		 WHERE customer_id = (SELECT min(customer_id) FROM public.customer)`)
	if err != nil {
		t.Fatalf("breaking the edge: %v", err)
	}
	if tag.RowsAffected() != 1 {
		t.Fatalf("broke the edge in %d rows", tag.RowsAffected())
	}

	report, err := r.verify(ctx)
	if report == nil {
		t.Fatalf("Verify returned no report: %v", err)
	}
	if report.ExitCode != 8 {
		t.Fatalf("exit code %d, want 8 (FK verification)", report.ExitCode)
	}
	var refusal *Refusal
	if !errors.As(err, &refusal) {
		t.Fatalf("Verify returned %v, want a *Refusal", err)
	}
	if refusal.Code != CodeRefusedFK {
		t.Errorf("code %s, want %s", refusal.Code, CodeRefusedFK)
	}
	if refusal.Table != rootTable {
		t.Errorf("the refusal names %s, want %s", refusal.Table, rootTable)
	}
	if refusal.Column != "customer_address_id_fkey" {
		t.Errorf("the refusal names the constraint %q", refusal.Column)
	}
	if refusal.Count != 1 {
		t.Errorf("the refusal counts %d orphan rows, want 1", refusal.Count)
	}
	if _, ok := checkNamed(report, checkFK); !ok {
		t.Error("the report carries no failing fk check")
	}
	noCheckBothWays(t, report)
}

// The unconfirmable branch (ARCHITECTURE.md §6 item 3, THREAT_MODEL.md T12).
//
// The same planted value as the negative control, with --residual-probe-cap set
// so that no probe may be issued at all. The hit is then a hit nobody can ask
// the source about, and §6 item 3 makes that exit 9 with the reason — never a
// printed note. This is the branch that decides whether this stage fails closed,
// and it is the one a stub could pass without.
func TestVerifyFailsWhenAHitCannotBeConfirmed(t *testing.T) {
	ctx := context.Background()
	r := pipelineRun(ctx, t)
	col := maskedColumn(t, r)

	source := connect(ctx, t, r.sourceURL)
	var id int32
	var email string
	if err := source.QueryRow(ctx,
		`SELECT customer_id, email FROM public.customer WHERE email IS NOT NULL ORDER BY customer_id LIMIT 1`,
	).Scan(&id, &email); err != nil {
		t.Fatalf("reading a source email: %v", err)
	}
	target := connect(ctx, t, r.targetURL)
	if _, err := target.Exec(ctx,
		`UPDATE public.customer SET email = $1 WHERE customer_id = $2`, email, id); err != nil {
		t.Fatalf("planting the source email in the target: %v", err)
	}

	// A negative cap is Options' "no probe may be issued at all".
	report, err := r.verifyWith(ctx, Options{ProbeCap: -1})
	if report == nil {
		t.Fatalf("Verify returned no report: %v", err)
	}
	if report.ExitCode != 9 {
		t.Fatalf("exit code %d, want 9 (a residual hit that could not be confirmed)", report.ExitCode)
	}
	var refusal *Refusal
	if !errors.As(err, &refusal) {
		t.Fatalf("Verify returned %v, want a *Refusal", err)
	}
	if refusal.Code != CodeRefusedUnconfirmable {
		t.Errorf("code %s, want %s", refusal.Code, CodeRefusedUnconfirmable)
	}
	if refusal.Table != col.Table || refusal.Column != col.Column {
		t.Errorf("the refusal names %s.%s, want %s", refusal.Table, refusal.Column, col)
	}
	if refusal.Reason != reasonProbeCap {
		t.Errorf("the refusal reads %q, want %q", refusal.Reason, reasonProbeCap)
	}
	if report.Probes != 0 {
		t.Errorf("the run issued %d probes under a cap that forbids every one", report.Probes)
	}
	if _, ok := checkNamed(report, checkUnconfirmable); !ok {
		t.Error("the report carries no failing residual_unconfirmable check")
	}
	noCheckBothWays(t, report)
}

// The second net (ARCHITECTURE.md §6 item 4, THREAT_MODEL.md T1).
//
// Addresses are planted in a column the classifier did not mask, which is what
// a classifier that under-read a 200-row sample would have left there. The net
// scans the column's whole contents, the address validator reaches the ratio,
// and the run is exit 9 naming the table, the column and the category.
func TestVerifyFailsOnUnmaskedPersonalDataInAnUnmaskedColumn(t *testing.T) {
	ctx := context.Background()
	r := pipelineRun(ctx, t)
	col := unmaskedTextColumn(t, r, ref.ColumnRef{
		Table: ref.TableRef{Schema: "public", Name: "film"}, Column: "title",
	})

	target := connect(ctx, t, r.targetURL)
	tag, err := target.Exec(ctx,
		`UPDATE public.film SET title = '221B Baker Street'`)
	if err != nil {
		t.Fatalf("planting addresses in %s: %v", col, err)
	}
	if tag.RowsAffected() < int64(minValues) {
		t.Fatalf("planted %d rows; the net needs at least %d non-NULL values to say anything",
			tag.RowsAffected(), minValues)
	}

	report, err := r.verify(ctx)
	if report == nil {
		t.Fatalf("Verify returned no report: %v", err)
	}
	if report.ExitCode != 9 {
		t.Fatalf("exit code %d, want 9 (a column the target holds unmasked validates as a category)", report.ExitCode)
	}
	var refusal *Refusal
	if !errors.As(err, &refusal) {
		t.Fatalf("Verify returned %v, want a *Refusal", err)
	}
	if refusal.Code != CodeRefusedSecondNet {
		t.Fatalf("code %s, want %s", refusal.Code, CodeRefusedSecondNet)
	}
	if refusal.Table != col.Table || refusal.Column != col.Column {
		t.Errorf("the refusal names %s.%s, want %s", refusal.Table, refusal.Column, col)
	}
	if refusal.Reason != "address" {
		t.Errorf("the refusal reads %q, want the validator name %q", refusal.Reason, "address")
	}
	if strings.Contains(refusal.Error(), "221B") {
		t.Error("the refusal quotes the value it found; it may name only the table, the column and the category")
	}
	c, ok := checkNamed(report, checkSecondNet)
	if !ok {
		t.Fatal("the report carries no failing second_net check")
	}
	if c.Table != col.Table || c.Column != col.Column {
		t.Errorf("the second_net check names %s.%s, want %s", c.Table, c.Column, col)
	}
	// The defect this test was written for: secondNet used to append its
	// passing line unconditionally, so a target that had just failed exit 9 for
	// unmasked personal data also printed "N columns were scanned and none of
	// them validates as a category".
	noCheckBothWays(t, report)
}

// The row count (ARCHITECTURE.md §6 item 5). A step holds exactly the rows the
// plan says it holds; one row more is a failure of the movement of rows between
// extract and load, which internal/verify/CLAUDE.md reads as ADR-005's exit 7.
func TestVerifyFailsOnARowCountThePlanDoesNotPredict(t *testing.T) {
	ctx := context.Background()
	r := pipelineRun(ctx, t)
	extra := ref.TableRef{Schema: "public", Name: "country"}

	target := connect(ctx, t, r.targetURL)
	// A country_id below the minimum, so the sequence's last_value is still the
	// column's maximum and this test fails one check and not two.
	tag, err := target.Exec(ctx,
		`INSERT INTO public.country (country_id, country, last_update)
		 VALUES ((SELECT min(country_id) FROM public.country) - 1, 'Verifyland', now())`)
	if err != nil {
		t.Fatalf("adding a row the plan does not predict: %v", err)
	}
	if tag.RowsAffected() != 1 {
		t.Fatalf("inserted %d rows", tag.RowsAffected())
	}

	report, err := r.verify(ctx)
	if report == nil {
		t.Fatalf("Verify returned no report: %v", err)
	}
	if report.ExitCode != 7 {
		t.Fatalf("exit code %d, want 7 (extract or load)", report.ExitCode)
	}
	var refusal *Refusal
	if !errors.As(err, &refusal) {
		t.Fatalf("Verify returned %v, want a *Refusal", err)
	}
	if refusal.Code != CodeRefusedRowCount {
		t.Fatalf("code %s, want %s", refusal.Code, CodeRefusedRowCount)
	}
	if refusal.Table != extra {
		t.Errorf("the refusal names %s, want %s", refusal.Table, extra)
	}
	if _, ok := checkNamed(report, checkRowCount); !ok {
		t.Error("the report carries no failing row_count check")
	}
	noCheckBothWays(t, report)
}

// The sequence check (ARCHITECTURE.md §6 item 5, THREAT_MODEL.md T8). A
// sequence rolled back to the start over a column that holds rows is the state
// where the application's first INSERT collides with a copied row, and it is
// exit 7 naming the column.
func TestVerifyFailsOnASequenceThatWasNotReset(t *testing.T) {
	ctx := context.Background()
	r := pipelineRun(ctx, t)

	target := connect(ctx, t, r.targetURL)
	if _, err := target.Exec(ctx,
		`SELECT setval('public.customer_customer_id_seq', 1, false)`); err != nil {
		t.Fatalf("rolling the sequence back: %v", err)
	}

	report, err := r.verify(ctx)
	if report == nil {
		t.Fatalf("Verify returned no report: %v", err)
	}
	if report.ExitCode != 7 {
		t.Fatalf("exit code %d, want 7 (extract or load)", report.ExitCode)
	}
	var refusal *Refusal
	if !errors.As(err, &refusal) {
		t.Fatalf("Verify returned %v, want a *Refusal", err)
	}
	if refusal.Code != CodeRefusedSequence {
		t.Fatalf("code %s, want %s", refusal.Code, CodeRefusedSequence)
	}
	if refusal.Table != rootTable || refusal.Column != "customer_id" {
		t.Errorf("the refusal names %s.%s, want %s.customer_id", refusal.Table, refusal.Column, rootTable)
	}
	if refusal.Reason != reasonNotCalled {
		t.Errorf("the refusal reads %q, want %q", refusal.Reason, reasonNotCalled)
	}
	if _, ok := checkNamed(report, checkSequences); !ok {
		t.Error("the report carries no failing sequences check")
	}
	noCheckBothWays(t, report)
}
