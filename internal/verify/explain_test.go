// SPDX-License-Identifier: Apache-2.0

package verify

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
	"github.com/Liarea/lazyslice/mask"
)

// ADR-015 (explain.go) against an in-memory source and target. The fake below
// answers exactly the statements this package builds — the target's column and
// row scans, the source's two column probes and the row check — and records
// every statement the source is sent, so each case can say how many probes it
// spent and every one is checked against the Shapes allowlist.

// listNamesMasker is a custom masker registered under the person_name category
// from outside package mask. It draws from the very list the built-in one does,
// so on a target it looks exactly like a correct run — and it must not be
// explained, because nothing outside package mask can claim a vocabulary.
const listNamesMasker mask.ID = "verify_test_list_names"

type listNames struct{}

func (listNames) Mask(h [32]byte, _ mask.Value, _ mask.Constraints) (mask.Value, error) {
	w := mask.RoleWords(mask.RoleGiven)
	return mask.Value{Text: w[int(h[0])%len(w)]}, nil
}
func (listNames) Domain(mask.Constraints) int64 { return 1 }

func init() { mask.Register(listNamesMasker, mask.CatPersonName, listNames{}) }

// givenWords is n distinct words of the RoleGiven list, title-cased as the
// masker emits them.
func givenWords(t *testing.T, n int) []string {
	t.Helper()
	w := mask.RoleWords(mask.RoleGiven)
	if len(w) < n {
		t.Fatalf("the RoleGiven list has %d words, the case needs %d", len(w), n)
	}
	out := make([]string, n)
	for i := range out {
		out[i] = strings.ToUpper(w[i][:1]) + w[i][1:]
	}
	return out
}

// ---------- the fake world ----------

type fakeTable struct {
	ref    ref.TableRef
	cols   []pipeline.Column
	pk     []string
	source [][]any
	target [][]any
}

func (ft *fakeTable) idx(name string) int {
	for i, c := range ft.cols {
		if c.Name == name {
			return i
		}
	}
	return -1
}

type fakeWorld struct {
	tables       map[ref.TableRef]*fakeTable
	sent         []string
	failRowCheck bool
}

func newWorld(tables ...*fakeTable) *fakeWorld {
	w := &fakeWorld{tables: map[ref.TableRef]*fakeTable{}}
	for _, t := range tables {
		w.tables[t.ref] = t
	}
	return w
}

// pipeline.Source, of which verify uses Short alone.
func (w *fakeWorld) Privileges(context.Context) (pipeline.RolePrivileges, error) {
	return pipeline.RolePrivileges{}, nil
}
func (w *fakeWorld) Snapshot(context.Context) (pipeline.SnapshotID, error) { return "", nil }
func (w *fakeWorld) Reader(context.Context, pipeline.SnapshotID) (pipeline.Reader, error) {
	return nil, errors.New("no snapshot reader in this fake")
}
func (w *fakeWorld) Short(context.Context) (pipeline.Reader, error) { return sourceSide{w}, nil }
func (w *fakeWorld) Release(context.Context) error                  { return nil }
func (w *fakeWorld) Trace() []pipeline.TracedStatement              { return nil }

// selectList reads a statement this package built: its select list, unquoted
// and without an alias, and the table it names.
func (w *fakeWorld) selectList(sql string) ([]string, *fakeTable) {
	rest := strings.TrimPrefix(sql, "SELECT ")
	i := strings.Index(rest, " FROM ")
	var cols []string
	for _, c := range strings.Split(rest[:i], ", ") {
		if j := strings.Index(c, "."); j >= 0 {
			c = c[j+1:]
		}
		cols = append(cols, strings.ReplaceAll(strings.Trim(c, `"`), `""`, `"`))
	}
	for r, t := range w.tables {
		if strings.HasPrefix(rest[i+len(" FROM "):], quoteTable(r)) {
			return cols, t
		}
	}
	return nil, nil
}

// targetSide answers the two target scans.
type targetSide struct{ w *fakeWorld }

func (ts targetSide) Query(_ context.Context, sql string, _ ...any) (pipeline.Rows, error) {
	cols, t := ts.w.selectList(sql)
	if t == nil {
		return nil, fmt.Errorf("the fake target does not know %q", sql)
	}
	var out [][]any
	for _, row := range t.target {
		r := make([]any, len(cols))
		for i, c := range cols {
			r[i] = row[t.idx(c)]
		}
		out = append(out, r)
	}
	return &resultRows{rows: out}, nil
}

// sourceSide answers the column probes and the row check.
type sourceSide struct{ w *fakeWorld }

func (ss sourceSide) Close(context.Context) error { return nil }

func (ss sourceSide) Query(_ context.Context, sql string, args ...any) (pipeline.Rows, error) {
	w := ss.w
	w.sent = append(w.sent, sql)
	for r, t := range w.tables {
		for _, c := range t.cols {
			i := t.idx(c.Name)
			switch sql {
			case probeSQL(r, c.Name):
				for _, row := range t.source {
					for _, e := range elements(row[i]) {
						if textOf(e) == textOf(args[0]) {
							return &resultRows{rows: [][]any{{true}}}, nil
						}
					}
				}
				return &resultRows{rows: [][]any{{false}}}, nil
			case foldedProbeSQL(r, c.Name):
				for _, row := range t.source {
					for _, e := range elements(row[i]) {
						if strings.EqualFold(textOf(e), textOf(args[0])) {
							return &resultRows{rows: [][]any{{true}}}, nil
						}
					}
				}
				return &resultRows{rows: [][]any{{false}}}, nil
			}
		}
	}
	if strings.Contains(sql, " r JOIN unnest(") {
		if w.failRowCheck {
			return nil, errors.New("permission denied for table")
		}
		cols, t := w.selectList(sql)
		if t == nil {
			return nil, fmt.Errorf("the fake source does not know %q", sql)
		}
		ids := cols[:len(args)]
		var out [][]any
		for _, row := range t.source {
			for k := 0; k < arrayLen(args[0]); k++ {
				match := true
				for j, id := range ids {
					c := t.cols[t.idx(id)]
					v, ok := encodeKey(c, row[t.idx(id)])
					if !ok || row[t.idx(id)] == nil || !sameKey(v, arrayAt(args[j], k)) {
						match = false
						break
					}
				}
				if match {
					r := make([]any, len(cols))
					for i, c := range cols {
						r[i] = row[t.idx(c)]
					}
					out = append(out, r)
					break
				}
			}
		}
		return &resultRows{rows: out}, nil
	}
	return nil, fmt.Errorf("the fake source does not know %q", sql)
}

func arrayLen(a any) int {
	switch t := a.(type) {
	case []int64:
		return len(t)
	case []string:
		return len(t)
	case [][16]byte:
		return len(t)
	}
	return 0
}

func arrayAt(a any, i int) any {
	switch t := a.(type) {
	case []int64:
		return t[i]
	case []string:
		return t[i]
	case [][16]byte:
		return t[i]
	}
	return nil
}

func sameKey(a, b any) bool { return a == b }

// resultRows is a fake result set of any width.
type resultRows struct {
	rows [][]any
	i    int
}

func (r *resultRows) Next() bool { r.i++; return r.i <= len(r.rows) }
func (r *resultRows) Scan(dest ...any) error {
	row := r.rows[r.i-1]
	if len(dest) != len(row) {
		return fmt.Errorf("resultRows: %d destinations for %d columns", len(dest), len(row))
	}
	for i, d := range dest {
		switch p := d.(type) {
		case *any:
			*p = row[i]
		case *bool:
			b, ok := row[i].(bool)
			if !ok {
				return errors.New("resultRows: not a bool")
			}
			*p = b
		default:
			return fmt.Errorf("resultRows: destination %T", d)
		}
	}
	return nil
}
func (r *resultRows) Err() error { return nil }
func (r *resultRows) Close()     {}

// fakeFilter is the residual filter and the emitted count, answered exactly.
type fakeFilter struct {
	has     map[string]bool
	emitted map[string]int64
}

func fkey(c ref.ColumnRef, path string, canonical []byte) string {
	return c.String() + "|" + path + "|" + string(canonical)
}

func (f *fakeFilter) Add(c ref.ColumnRef, path string, canonical []byte) {
	f.has[fkey(c, path, canonical)] = true
}
func (f *fakeFilter) MayContain(c ref.ColumnRef, path string, canonical []byte) bool {
	return f.has[fkey(c, path, canonical)]
}
func (f *fakeFilter) AddEmitted(c ref.ColumnRef, path string, canonical []byte) {
	f.emitted[fkey(c, path, canonical)]++
}
func (f *fakeFilter) Emitted(c ref.ColumnRef, path string, canonical []byte) int64 {
	return f.emitted[fkey(c, path, canonical)]
}
func (f *fakeFilter) Cells() int64 { return int64(len(f.has)) }
func (f *fakeFilter) Bytes() int64 { return 0 }

// elements is a scalar as one element, or an array's non-NULL elements.
func elements(v any) []any {
	if a, ok := v.([]any); ok {
		var out []any
		for _, e := range a {
			if e != nil {
				out = append(out, e)
			}
		}
		return out
	}
	if v == nil {
		return nil
	}
	return []any{v}
}

// filterFor is what internal/transform would have recorded: every masked
// source value in the filter, and every value of `emittedFrom` (the target as
// transform wrote it, before any tampering a case applies afterwards) counted
// for a column whose masker has a vocabulary.
func filterFor(w *fakeWorld, cls *pipeline.Classification, emittedFrom map[ref.TableRef][][]any) *fakeFilter {
	f := &fakeFilter{has: map[string]bool{}, emitted: map[string]int64{}}
	for r, t := range w.tables {
		for _, c := range t.cols {
			col := ref.ColumnRef{Table: r, Column: c.Name}
			d, ok := cls.Decisions[col]
			if !ok || !d.Masked {
				continue
			}
			i := t.idx(c.Name)
			for _, row := range t.source {
				for _, e := range elements(row[i]) {
					if canon, ok, _ := canonicalOf(mask.Category(d.Category), e); ok {
						f.Add(col, "", canon)
					}
				}
			}
			if !mask.Emitting(d.Masker, mask.Constraints{Role: d.Role}) {
				continue
			}
			for _, row := range emittedFrom[r] {
				for _, e := range elements(row[i]) {
					if canon, ok, _ := canonicalOf(mask.Category(d.Category), e); ok {
						f.AddEmitted(col, "", canon)
					}
				}
			}
		}
	}
	return f
}

func snapshotTargets(w *fakeWorld) map[ref.TableRef][][]any {
	out := map[ref.TableRef][][]any{}
	for r, t := range w.tables {
		rows := make([][]any, len(t.target))
		for i, row := range t.target {
			rows[i] = append([]any(nil), row...)
		}
		out[r] = rows
	}
	return out
}

// scene is one case: a world, its plan and classification, and the filter.
type scene struct {
	w    *fakeWorld
	plan *pipeline.Plan
	cls  *pipeline.Classification
	res  pipeline.Residual
	opts Options
}

func (sc *scene) run(t *testing.T) *state {
	t.Helper()
	schema := &pipeline.Schema{}
	for _, ft := range sc.w.tables {
		schema.Tables = append(schema.Tables, pipeline.Table{Ref: ft.ref, Columns: ft.cols, PK: ft.pk})
	}
	s := &state{
		opts: sc.opts, src: sc.w, target: targetSide{sc.w}, schema: schema, plan: sc.plan,
		cls: sc.cls, res: sc.res, lr: &pipeline.LoadResult{},
		tables: map[ref.TableRef]*pipeline.Table{}, rows: map[ref.TableRef]int64{},
	}
	for i := range schema.Tables {
		s.tables[schema.Tables[i].Ref] = &schema.Tables[i]
	}
	s.steps = sc.plan.Steps
	if err := s.residualScan(context.Background()); err != nil {
		t.Fatalf("residualScan: %v", err)
	}
	// Every statement the source was sent matches a shape Shapes exports.
	tr := tracerFor(t, sc.plan, sc.cls)
	for _, sql := range sc.w.sent {
		if admits(t, tr, sql) == "" {
			t.Errorf("the source was sent a statement no shape admits: %s", sql)
		}
	}
	return s
}

func (sc *scene) columnProbes() int {
	n := 0
	for _, sql := range sc.w.sent {
		if strings.HasPrefix(sql, "SELECT EXISTS") {
			n++
		}
	}
	return n
}

func (sc *scene) rowChecks() int {
	n := 0
	for _, sql := range sc.w.sent {
		if strings.Contains(sql, " r JOIN unnest(") {
			n++
		}
	}
	return n
}

func exitOf(s *state) int {
	if f := s.firstFailure(); f != nil {
		return f.Exit
	}
	return 0
}

func explainedLines(s *state) []pipeline.Check {
	var out []pipeline.Check
	for _, c := range s.checks {
		if c.Code == CodeResidualExplained {
			out = append(out, c)
		}
	}
	return out
}

// people is the standard fixture: a table keyed by a bigint id whose
// first_name is masked as a given name, and whose email is masked as email.
// Row i's source first_name is names[i]; its target first_name is the next
// row's source name, so every masked value is some other row's real value —
// the coincidence ADR-015 exists to explain.
func people(t *testing.T, names []string) (*fakeTable, *pipeline.Plan, *pipeline.Classification) {
	t.Helper()
	tbl := ref.TableRef{Schema: "public", Name: "people"}
	ft := &fakeTable{
		ref: tbl,
		cols: []pipeline.Column{
			{Name: "id", TypeName: "bigint", TypeOID: oidInt8},
			{Name: "first_name", TypeName: "text", TypeOID: oidText, Nullable: true},
			{Name: "email", TypeName: "text", TypeOID: oidText, Nullable: true},
			{Name: "phone", TypeName: "text", TypeOID: oidText, Nullable: true},
		},
		pk: []string{"id"},
	}
	for i, n := range names {
		next := names[(i+1)%len(names)]
		ft.source = append(ft.source, []any{int64(i + 1), n,
			fmt.Sprintf("real.person%d@realcorp.example", i), fmt.Sprintf("+1 415 555 %04d", 2000+i)})
		ft.target = append(ft.target, []any{int64(i + 1), next,
			fmt.Sprintf("u%d@example.com", i), fmt.Sprintf("+1 201 555 %04d", 100+i)})
	}
	plan := &pipeline.Plan{Steps: []pipeline.Step{{
		Table: tbl, Mode: pipeline.ChildOK, Keys: keys{n: len(names)},
		Identity: pipeline.Identity{Kind: pipeline.IdentityPK, Columns: []string{"id"}, Types: []uint32{oidInt8}},
	}}}
	c := func(n string) ref.ColumnRef { return ref.ColumnRef{Table: tbl, Column: n} }
	cls := &pipeline.Classification{Decisions: map[ref.ColumnRef]pipeline.Decision{
		c("id"): {Col: c("id"), Category: pipeline.CatNone},
		c("first_name"): {Col: c("first_name"), Category: pipeline.CatPersonName, Masked: true,
			Masker: mask.MaskerPersonName, Role: mask.RoleGiven},
		c("email"): {Col: c("email"), Category: pipeline.CatEmail, Masked: true, Masker: mask.MaskerEmail},
		c("phone"): {Col: c("phone"), Category: pipeline.CatPhone, Masked: true, Masker: mask.MaskerPhone},
	}}
	return ft, plan, cls
}

func newScene(ft *fakeTable, plan *pipeline.Plan, cls *pipeline.Classification) *scene {
	w := newWorld(ft)
	return &scene{w: w, plan: plan, cls: cls, res: filterFor(w, cls, snapshotTargets(w))}
}

// ---------- the cases ----------

// A masked name that equals another row's real name, which the list contains
// and the masker produced every copy of, is explained: exit 0, no column
// probe, one row check for the whole column, and one explained line carrying a
// count and nothing else.
func TestACoincidenceIsExplained(t *testing.T) {
	ft, plan, cls := people(t, givenWords(t, 6))
	sc := newScene(ft, plan, cls)
	s := sc.run(t)
	if exitOf(s) != 0 {
		t.Fatalf("exit %d (%s), want 0", exitOf(s), s.firstFailure().Reason)
	}
	if n := sc.columnProbes(); n != 0 {
		t.Errorf("%d column probes, want 0: an explained hit spends none", n)
	}
	if n := sc.rowChecks(); n != 1 {
		t.Errorf("%d row checks, want 1", n)
	}
	lines := explainedLines(s)
	if len(lines) != 1 {
		t.Fatalf("%d explained lines, want 1", len(lines))
	}
	if lines[0].Count != 6 || lines[0].Column != "first_name" || !lines[0].Passed {
		t.Errorf("explained line %+v, want 6 values in first_name, passing", lines[0])
	}
	assertValueFree(t, s, ft)
}

// The same with the table's rows a lookup step with no primary key: there is
// no row identity, so the count check alone explains it, and not one statement
// reaches the source.
func TestATableWithoutAPrimaryKeyIsExplainedByCountAlone(t *testing.T) {
	ft, plan, cls := people(t, givenWords(t, 6))
	ft.pk = nil
	plan.Steps[0] = pipeline.Step{Table: ft.ref, Mode: pipeline.Lookup}
	sc := newScene(ft, plan, cls)
	s := sc.run(t)
	if exitOf(s) != 0 {
		t.Fatalf("exit %d (%s), want 0", exitOf(s), s.firstFailure().Reason)
	}
	if len(sc.w.sent) != 0 || s.probes != 0 {
		t.Errorf("%d statements and %d probes reached the source, want none", len(sc.w.sent), s.probes)
	}
	if lines := explainedLines(s); len(lines) != 1 || lines[0].Count != 6 {
		t.Errorf("explained lines %+v, want one counting 6", lines)
	}
}

// A row that kept its own source value, which the masker "produced" (the
// count check passes, because the self-map is what transform counted), is
// found by the row check.
func TestASameRowPassthroughIsRefused(t *testing.T) {
	for _, tc := range []struct {
		name  string
		value func(string) string
	}{
		{"verbatim", func(s string) string { return s }},
		{"case-folded", strings.ToUpper},
		{"re-spaced", func(s string) string { return " " + s + "  " }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			names := givenWords(t, 6)
			ft, plan, cls := people(t, names)
			// Row 3 keeps its own name; the name it would have had goes nowhere.
			ft.target[2][1] = tc.value(names[2])
			ft.target[1][1] = names[0] // keep row 2's coincidence distinct from row 3's value
			sc := newScene(ft, plan, cls)
			s := sc.run(t)
			f := s.firstFailure()
			if f == nil || f.Exit != 9 || f.Code != CodeRefusedResidual || f.Reason != reasonSameRow {
				t.Fatalf("failure %+v, want exit 9 %s %q", f, CodeRefusedResidual, reasonSameRow)
			}
			if f.Column != "first_name" {
				t.Errorf("the refusal names %s, want first_name", f.Column)
			}
			if len(explainedLines(s)) != 0 {
				t.Error("a column that failed its row check still printed an explained line")
			}
			assertValueFree(t, s, ft)
		})
	}
}

// A value the target holds one more time than transform emitted it is a copy
// the masker did not make: the column probe runs on it, confirms it, and the
// run is exit 9 with the over-count reason.
func TestOneCopyOverTheEmittedCountIsProbed(t *testing.T) {
	names := givenWords(t, 6)
	ft, plan, cls := people(t, names)
	sc := newScene(ft, plan, cls)
	// After transform: row 5 now also holds row 1's real name, a value the
	// masker emitted once (for row 6) and the target holds twice.
	ft.target[4][1] = names[0]
	s := sc.run(t)
	f := s.firstFailure()
	if f == nil || f.Exit != 9 || f.Code != CodeRefusedResidual || f.Reason != reasonOverCount {
		t.Fatalf("failure %+v, want exit 9 %s %q", f, CodeRefusedResidual, reasonOverCount)
	}
	if sc.columnProbes() == 0 {
		t.Error("the over-count value was refused without a probe confirming the source holds it")
	}
	assertValueFree(t, s, ft)
}

// A name the vocabulary does not contain takes the column probe at once, as
// every hit did before ADR-015.
func TestAnOffListNameIsProbedAsToday(t *testing.T) {
	names := givenWords(t, 5)
	ft, plan, cls := people(t, append(names, "Wolfgangina"))
	// Row 1 holds row 6's real, off-list name.
	ft.target[0][1] = "Wolfgangina"
	sc := newScene(ft, plan, cls)
	s := sc.run(t)
	f := s.firstFailure()
	if f == nil || f.Exit != 9 || f.Code != CodeRefusedResidual || f.Reason != reasonStillHolds {
		t.Fatalf("failure %+v, want exit 9 %s %q", f, CodeRefusedResidual, reasonStillHolds)
	}
	if sc.columnProbes() == 0 {
		t.Error("an off-list name was refused without the column probe")
	}
}

// Email and phone have no vocabulary: a masked value equal to another row's
// real value is confirmed and refused exactly as before.
func TestEmailAndPhoneKeepTheColumnProbe(t *testing.T) {
	for _, column := range []string{"email", "phone"} {
		t.Run(column, func(t *testing.T) {
			ft, plan, cls := people(t, givenWords(t, 4))
			i := ft.idx(column)
			ft.target[0][i] = ft.source[1][i]
			sc := newScene(ft, plan, cls)
			s := sc.run(t)
			f := s.firstFailure()
			if f == nil || f.Exit != 9 || f.Column != column || f.Reason != reasonStillHolds {
				t.Fatalf("failure %+v, want exit 9 on %s", f, column)
			}
		})
	}
}

// A composite unique identity (uuid, int8): the row check encodes both and
// finds a row that kept its own value; a row with a NULL identity part learns
// nothing, so its own passthrough is left to the count check, which passes it.
func TestACompositeIdentityWithANullPartLearnsNothing(t *testing.T) {
	names := givenWords(t, 4)
	tbl := ref.TableRef{Schema: "public", Name: "members"}
	uid := func(b byte) [16]byte { var u [16]byte; u[0], u[15] = 0xab, b; return u }
	ft := &fakeTable{
		ref: tbl,
		cols: []pipeline.Column{
			{Name: "org", TypeName: "uuid", TypeOID: oidUUID},
			{Name: "n", TypeName: "bigint", TypeOID: oidInt8, Nullable: true},
			{Name: "first_name", TypeName: "text", TypeOID: oidText},
		},
	}
	for i, nm := range names {
		var n any = int64(i)
		if i == 2 {
			n = nil
		}
		ft.source = append(ft.source, []any{uid(byte(i)), n, nm})
		ft.target = append(ft.target, []any{uid(byte(i)), n, names[(i+1)%len(names)]})
	}
	// Row 3 (NULL n) keeps its own name, and row 2 takes row 4's so row 3's
	// value stays emitted once.
	ft.target[2][2] = names[2]
	ft.target[1][2] = names[3]
	ft.target[3][2] = names[0]
	ft.target[0][2] = names[1]
	plan := &pipeline.Plan{Steps: []pipeline.Step{{
		Table: tbl, Mode: pipeline.ChildOK, Keys: keys{n: 4},
		Identity: pipeline.Identity{Kind: pipeline.IdentityUnique, Columns: []string{"org", "n"}},
	}}}
	c := func(n string) ref.ColumnRef { return ref.ColumnRef{Table: tbl, Column: n} }
	cls := &pipeline.Classification{Decisions: map[ref.ColumnRef]pipeline.Decision{
		c("first_name"): {Col: c("first_name"), Category: pipeline.CatPersonName, Masked: true,
			Masker: mask.MaskerPersonName, Role: mask.RoleGiven},
	}}
	sc := newScene(ft, plan, cls)
	s := sc.run(t)
	if exitOf(s) != 0 {
		t.Fatalf("exit %d (%s), want 0: a NULL identity part learns nothing", exitOf(s), s.firstFailure().Reason)
	}
	if sc.rowChecks() != 1 {
		t.Errorf("%d row checks, want 1", sc.rowChecks())
	}

	// And the same identity finds a row that kept its own value when the
	// identity is whole.
	ft.target[0][2] = names[0]
	ft.target[3][2] = names[1]
	sc = newScene(ft, plan, cls)
	s = sc.run(t)
	if f := s.firstFailure(); f == nil || f.Reason != reasonSameRow {
		t.Fatalf("failure %+v, want %q through the (uuid, int8) identity", f, reasonSameRow)
	}
}

// A target row whose source row has since been deleted: the row check finds
// nothing to compare, learns nothing, and the column is explained.
func TestADeletedSourceRowLearnsNothing(t *testing.T) {
	ft, plan, cls := people(t, givenWords(t, 6))
	sc := newScene(ft, plan, cls)
	ft.source = ft.source[:5] // row 6 is gone from the source since the snapshot
	s := sc.run(t)
	if exitOf(s) != 0 {
		t.Fatalf("exit %d (%s), want 0", exitOf(s), s.firstFailure().Reason)
	}
	if sc.columnProbes() != 0 {
		t.Error("an absent row fell back to the column probe")
	}
}

// 2,500 coincidences are three row-check statements, sent as each batch of
// 1,000 fills, and none of them spends --residual-probe-cap: a cap that
// forbids every confirmation probe still runs the row check, which binds no
// candidate value, and the column is explained.
func TestManyCoincidencesAreBatchedAndSpendNoProbe(t *testing.T) {
	w := mask.RoleWords(mask.RoleGiven)
	names := make([]string, 2500)
	for i := range names {
		names[i] = strings.ToUpper(w[i%len(w)][:1]) + w[i%len(w)][1:]
	}
	ft, plan, cls := people(t, names)
	sc := newScene(ft, plan, cls)
	s := sc.run(t)
	if exitOf(s) != 0 {
		t.Fatalf("exit %d (%s), want 0", exitOf(s), s.firstFailure().Reason)
	}
	if s.probes != 0 || sc.rowChecks() != 3 || sc.columnProbes() != 0 {
		t.Errorf("probes %d (row checks %d, column probes %d), want 0, 3 and 0",
			s.probes, sc.rowChecks(), sc.columnProbes())
	}

	ft, plan, cls = people(t, names[:10])
	sc = newScene(ft, plan, cls)
	sc.opts = Options{ProbeCap: -1}
	s = sc.run(t)
	if exitOf(s) != 0 {
		t.Fatalf("exit %d (%s), want 0 under a cap that forbids every probe", exitOf(s), s.firstFailure().Reason)
	}
	if s.probes != 0 || sc.rowChecks() != 1 {
		t.Errorf("probes %d, row checks %d, want 0 and 1", s.probes, sc.rowChecks())
	}
}

// A row identity the plan does not know to be unique is no row identity. Rows
// 1 and 2 share a pseudo-key, and row 2's masked name is row 1's real one: a
// row check over that key would compare row 1's source against row 2's target
// and refuse a correct run, so none is sent and the count check explains the
// column. A unique identity the target nonetheless holds twice compares every
// target row that holds it, so a row that kept its own value is not hidden
// behind the other.
func TestADuplicatedIdentity(t *testing.T) {
	t.Run("a pseudo-key is not used", func(t *testing.T) {
		names := givenWords(t, 6)
		ft, plan, cls := people(t, names)
		ft.source[1][0], ft.target[1][0] = int64(1), int64(1)
		ft.target[1][1] = names[0]
		plan.Steps[0].Identity.Kind = pipeline.IdentityPseudo
		sc := newScene(ft, plan, cls)
		s := sc.run(t)
		if exitOf(s) != 0 {
			t.Fatalf("exit %d (%s), want 0: a pseudo-key must not match one row to another", exitOf(s), s.firstFailure().Reason)
		}
		if sc.rowChecks() != 0 || sc.columnProbes() != 0 {
			t.Errorf("row checks %d, column probes %d, want none over a pseudo-key", sc.rowChecks(), sc.columnProbes())
		}
		if lines := explainedLines(s); len(lines) != 1 || lines[0].Count != 6 {
			t.Errorf("explained lines %+v, want one counting 6", lines)
		}
	})
	t.Run("a unique key held twice compares both rows", func(t *testing.T) {
		names := givenWords(t, 6)
		ft, plan, cls := people(t, names)
		// Row 1 kept its own name; row 2 carries row 1's identity with a
		// masked name, and is the later of the two.
		ft.target[0][1] = names[0]
		ft.target[1][0] = int64(1)
		plan.Steps[0].Identity.Kind = pipeline.IdentityUnique
		sc := newScene(ft, plan, cls)
		s := sc.run(t)
		f := s.firstFailure()
		if f == nil || f.Exit != 9 || f.Reason != reasonSameRow {
			t.Fatalf("failure %+v, want exit 9 %q", f, reasonSameRow)
		}
		assertValueFree(t, s, ft)
	})
}

// A row check the source refuses is exit 9, not a coincidence.
func TestARowCheckThatFailsIsUnconfirmable(t *testing.T) {
	ft, plan, cls := people(t, givenWords(t, 6))
	sc := newScene(ft, plan, cls)
	sc.w.failRowCheck = true
	s := sc.run(t)
	f := s.firstFailure()
	if f == nil || f.Exit != 9 || f.Code != CodeRefusedUnconfirmable || f.Reason != reasonProbeFailed {
		t.Fatalf("failure %+v, want exit 9 %s %q", f, CodeRefusedUnconfirmable, reasonProbeFailed)
	}
	if sc.columnProbes() != 0 {
		t.Error("a failed row check fell back to the column probe")
	}
}

// A text[] of given names is counted per element and never row-checked; one
// element more than transform emitted is probed and refused.
func TestAnArrayOfNamesIsCountedPerElement(t *testing.T) {
	names := givenWords(t, 6)
	tbl := ref.TableRef{Schema: "public", Name: "aliases"}
	ft := &fakeTable{
		ref: tbl,
		cols: []pipeline.Column{
			{Name: "id", TypeName: "bigint", TypeOID: oidInt8},
			{Name: "names", TypeName: "text[]", TypeOID: 1009},
		},
		pk: []string{"id"},
	}
	for i := range 3 {
		ft.source = append(ft.source, []any{int64(i), []any{names[2*i], names[2*i+1]}})
		ft.target = append(ft.target, []any{int64(i), []any{names[(2*i+2)%6], nil, names[(2*i+3)%6]}})
	}
	plan := &pipeline.Plan{Steps: []pipeline.Step{{
		Table: tbl, Mode: pipeline.ChildOK, Keys: keys{n: 3},
		Identity: pipeline.Identity{Kind: pipeline.IdentityPK, Columns: []string{"id"}},
	}}}
	c := ref.ColumnRef{Table: tbl, Column: "names"}
	cls := &pipeline.Classification{Decisions: map[ref.ColumnRef]pipeline.Decision{
		c: {Col: c, Category: pipeline.CatPersonName, Masked: true, Masker: mask.MaskerPersonName, Role: mask.RoleGiven},
	}}
	sc := newScene(ft, plan, cls)
	s := sc.run(t)
	if exitOf(s) != 0 {
		t.Fatalf("exit %d (%s), want 0", exitOf(s), s.firstFailure().Reason)
	}
	if len(sc.w.sent) != 0 {
		t.Errorf("%d statements reached the source for an array column, want none", len(sc.w.sent))
	}
	if lines := explainedLines(s); len(lines) != 1 || lines[0].Count != 6 {
		t.Errorf("explained lines %+v, want one counting 6 elements", lines)
	}

	ft.target[0] = []any{int64(0), []any{names[2], names[3], names[0]}}
	s = sc.run(t)
	if f := s.firstFailure(); f == nil || f.Reason != reasonOverCount {
		t.Fatalf("failure %+v, want %q for an element over its count", f, reasonOverCount)
	}
}

// A custom masker registered under person_name draws from the same list, and
// its column is not explained: mask.Emits is false for it, and the column
// probe confirms the coincidence as it always did.
func TestACustomNameMaskerIsNotExplained(t *testing.T) {
	ft, plan, cls := people(t, givenWords(t, 6))
	col := ref.ColumnRef{Table: ft.ref, Column: "first_name"}
	d := cls.Decisions[col]
	d.Masker = listNamesMasker
	cls.Decisions[col] = d
	if mask.Emits(listNamesMasker, mask.Value{Text: ft.target[0][1].(string)}, mask.Constraints{Role: mask.RoleGiven}) {
		t.Fatal("mask.Emits answers true for a masker registered outside package mask")
	}
	sc := newScene(ft, plan, cls)
	s := sc.run(t)
	f := s.firstFailure()
	if f == nil || f.Exit != 9 || f.Reason != reasonStillHolds || f.Column != "first_name" {
		t.Fatalf("failure %+v, want exit 9 on first_name", f)
	}
}

// A JSON leaf holding a list name is a document hit: never explained, and
// untestable by either probe, so exit 9.
func TestAJSONLeafHoldingAListNameIsUntestable(t *testing.T) {
	name := givenWords(t, 1)[0]
	tbl := ref.TableRef{Schema: "public", Name: "profiles"}
	ft := &fakeTable{
		ref: tbl,
		cols: []pipeline.Column{
			{Name: "id", TypeName: "bigint", TypeOID: oidInt8},
			{Name: "doc", TypeName: "jsonb", TypeOID: 3802},
		},
		pk:     []string{"id"},
		source: [][]any{{int64(1), map[string]any{"given": name}}},
		target: [][]any{{int64(1), map[string]any{"given": name}}},
	}
	plan := &pipeline.Plan{Steps: []pipeline.Step{{
		Table: tbl, Mode: pipeline.ChildOK, Keys: keys{n: 1},
		Identity: pipeline.Identity{Kind: pipeline.IdentityPK, Columns: []string{"id"}},
	}}}
	c := ref.ColumnRef{Table: tbl, Column: "doc"}
	cls := &pipeline.Classification{Decisions: map[ref.ColumnRef]pipeline.Decision{
		c: {Col: c, Category: pipeline.CatSemiStruct, Masked: true, Masker: mask.MaskerSemiStruct},
	}}
	w := newWorld(ft)
	f := &fakeFilter{has: map[string]bool{}, emitted: map[string]int64{}}
	canon, _, _ := canonicalOf(mask.CatFreeText, name)
	f.Add(c, "$.given", canon)
	sc := &scene{w: w, plan: plan, cls: cls, res: f}
	s := sc.run(t)
	got := s.firstFailure()
	if got == nil || got.Code != CodeRefusedUnconfirmable || got.Reason != reasonNoProbe {
		t.Fatalf("failure %+v, want %s %q", got, CodeRefusedUnconfirmable, reasonNoProbe)
	}
}

// ADR-015's second-net clause: a generated full_name over two masked name
// columns holds the masker's own words and is skipped by the dictionary rule;
// the same expression over an unmasked column is scanned as it always was.
func TestAGeneratedNameOverMaskedColumnsIsSkippedByTheDictionaryRule(t *testing.T) {
	full := []any{
		"Grace Hopper", "Katherine Johnson", "Alan Turing", "Mary Jackson",
		"Mary Smith", "John Smith", "James Brown",
	}
	for _, tc := range []struct {
		name     string
		expr     string
		wantFail bool
	}{
		{"over two masked name columns", `((first_name || ' '::text) || last_name)`, false},
		{"over a quoted masked column", `(("First" || ' '::text) || last_name)`, false},
		{"over an unmasked column", `((nickname || ' '::text) || last_name)`, true},
		{"over no column of its table", `'x'::text`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			table := customers()
			c := func(n string) ref.ColumnRef { return ref.ColumnRef{Table: table, Column: n} }
			masked := func(n string) pipeline.Decision {
				return pipeline.Decision{Col: c(n), Category: pipeline.CatPersonName, Masked: true,
					Masker: mask.MaskerPersonName}
			}
			s := &state{
				schema: &pipeline.Schema{},
				target: oneColumn{vals: full},
				steps:  []pipeline.Step{{Table: table, Mode: pipeline.ChildOK}},
				tables: map[ref.TableRef]*pipeline.Table{table: {Ref: table, Columns: []pipeline.Column{
					{Name: "first_name", TypeName: "text"},
					{Name: "First", TypeName: "text"},
					{Name: "last_name", TypeName: "text"},
					{Name: "nickname", TypeName: "integer"},
					{Name: "full_name", TypeName: "text", Generated: tc.expr},
				}}},
				cls: &pipeline.Classification{Decisions: map[ref.ColumnRef]pipeline.Decision{
					c("first_name"): masked("first_name"),
					c("First"):      masked("First"),
					c("last_name"):  masked("last_name"),
					c("nickname"):   {Col: c("nickname"), Category: pipeline.CatNone},
					c("full_name"):  {Col: c("full_name"), Category: pipeline.CatNone},
				}},
			}
			if err := s.secondNet(context.Background()); err != nil {
				t.Fatalf("secondNet: %v", err)
			}
			var names []string
			for _, f := range s.failures {
				if f.Column == "full_name" {
					names = append(names, f.Reason)
				}
			}
			if tc.wantFail && (len(names) != 1 || names[0] != "person_name") {
				t.Errorf("full_name failures %v, want one person_name", names)
			}
			if !tc.wantFail && len(names) != 0 {
				t.Errorf("full_name failures %v, want none", names)
			}
		})
	}
}

// Shapes registers a row check for a loaded table with an emitting masked
// column and a usable identity, and for nothing else; every statement the row
// check builds matches it and no other table's.
func TestShapesRegistersARowCheckOnlyWhereOneCanRun(t *testing.T) {
	tr := func(n string) ref.TableRef { return ref.TableRef{Schema: "public", Name: n} }
	c := func(t, n string) ref.ColumnRef { return ref.ColumnRef{Table: tr(t), Column: n} }
	name := func(t, n string) pipeline.Decision {
		return pipeline.Decision{Col: c(t, n), Category: pipeline.CatPersonName, Masked: true,
			Masker: mask.MaskerPersonName, Role: mask.RoleGiven}
	}
	id := func(cols ...string) pipeline.Identity { return pipeline.Identity{Columns: cols} }
	plan := &pipeline.Plan{Steps: []pipeline.Step{
		{Table: tr("eligible"), Mode: pipeline.ChildOK, Keys: keys{n: 1}, Identity: id("id")},
		{Table: tr("masked_key"), Mode: pipeline.ChildOK, Keys: keys{n: 1}, Identity: id("email")},
		{Table: tr("no_names"), Mode: pipeline.ChildOK, Keys: keys{n: 1}, Identity: id("id")},
		{Table: tr("custom"), Mode: pipeline.ChildOK, Keys: keys{n: 1}, Identity: id("id")},
		{Table: tr("empty"), Mode: pipeline.SchemaOnly},
		{Table: tr("lookup"), Mode: pipeline.Lookup},
	}}
	custom := name("custom", "first_name")
	custom.Masker = listNamesMasker
	cls := &pipeline.Classification{Decisions: map[ref.ColumnRef]pipeline.Decision{
		c("eligible", "first_name"):   name("eligible", "first_name"),
		c("masked_key", "first_name"): name("masked_key", "first_name"),
		c("masked_key", "email"): {Col: c("masked_key", "email"), Category: pipeline.CatEmail,
			Masked: true, Masker: mask.MaskerEmail},
		c("no_names", "email"): {Col: c("no_names", "email"), Category: pipeline.CatEmail,
			Masked: true, Masker: mask.MaskerEmail},
		c("custom", "first_name"): custom,
		c("empty", "first_name"):  name("empty", "first_name"),
		c("lookup", "first_name"): name("lookup", "first_name"),
	}}
	var got []string
	for _, st := range Shapes(plan, cls) {
		if strings.HasPrefix(st.Name, "verify.rowcheck.") {
			got = append(got, st.Name)
		}
	}
	want := []string{"verify.rowcheck.public.eligible", "verify.rowcheck.public.lookup"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("row-check shapes %v, want %v", got, want)
	}

	tracer := tracerFor(t, plan, cls)
	for _, tc := range []struct {
		name  string
		cols  []string
		ids   []string
		casts []string
		ch    tupleChunk
	}{
		{"one int8 key", []string{"id", "first_name"}, []string{"id"}, []string{""},
			tupleChunk{casts: []string{"::int8[]"}, cols: []any{[]int64{1}}, n: 1}},
		{"a (uuid, int8) key", []string{"org", "n", "first_name"}, []string{"org", "n"}, []string{"", ""},
			tupleChunk{casts: []string{"::uuid[]", "::int8[]"}, cols: []any{[][16]byte{{1}}, []int64{1}}, n: 1}},
		{"a key cast back", []string{"at", "first_name"}, []string{"at"}, []string{"::timestamp with time zone"},
			tupleChunk{casts: []string{"::text[]"}, cols: []any{[]string{"x"}}, n: 1}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, table := range []string{"eligible", "lookup"} {
				sql := rowCheckSQL(tr(table), tc.cols, tc.ids, tc.casts, tc.ch)
				if got := admits(t, tracer, sql); got != "verify.rowcheck.public."+table {
					t.Errorf("rowCheckSQL on %s is admitted as %q", table, got)
				}
			}
			sql := rowCheckSQL(tr("no_names"), tc.cols, tc.ids, tc.casts, tc.ch)
			if got := admits(t, tracer, sql); got != "" {
				t.Errorf("a row check on a table with no emitting column is admitted as %q", got)
			}
		})
	}
}

// assertValueFree is the guard every ADR-015 case runs: no source or target
// value of the fixture appears in any check or refusal the scan recorded.
func assertValueFree(t *testing.T, s *state, ft *fakeTable) {
	t.Helper()
	var values []string
	for _, rows := range [][][]any{ft.source, ft.target} {
		for _, row := range rows {
			for i, v := range row {
				if ft.cols[i].Name == "id" || v == nil {
					continue
				}
				if str := strings.TrimSpace(textOf(v)); len(str) > 2 {
					values = append(values, str)
				}
			}
		}
	}
	var texts []string
	for _, c := range s.checks {
		texts = append(texts, fmt.Sprintf("%s %s %s %s %d", c.Name, c.Code, c.Table, c.Column, c.Count))
	}
	for _, f := range s.failures {
		texts = append(texts, f.Error(), f.Reason)
	}
	for _, text := range texts {
		for _, v := range values {
			if strings.Contains(strings.ToLower(text), strings.ToLower(v)) {
				t.Fatalf("a check or refusal carries a row value: %q", text)
			}
		}
	}
}
