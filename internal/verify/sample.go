// SPDX-License-Identifier: Apache-2.0

package verify

import (
	"context"
	"fmt"
	"strings"

	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
)

// The unmasked sample comparison (ARCHITECTURE.md section 6 item 5).
//
// A sample of rows per table is fetched by identity from both sides — from the
// source through the short second transaction, from the target directly — and
// every column the run did not mask is compared value for value. The short
// transaction sees a newer snapshot than extract did, so a row that changed
// since is reported as "differs from source (source changed since snapshot)"
// and counted like an unconfirmed residual hit, and a row the source no longer
// holds is reported too. Neither fails the run.

// The type OIDs that travel in a chunk as themselves (internal/extract/keys.go,
// which this mirrors: the planner encodes the key sets and both readers join
// against them, so the two have to agree on whether a value arriving through
// unnest is cast back to the column's own type).
const (
	oidInt2    uint32 = 21
	oidInt8    uint32 = 20
	oidInt4    uint32 = 23
	oidText    uint32 = 25
	oidBpchar  uint32 = 1042
	oidVarchar uint32 = 1043
	oidUUID    uint32 = 2950
)

// samples is item 5's sample comparison.
func (s *state) samples(ctx context.Context) error {
	compared := int64(0)
	for _, step := range s.steps {
		n, err := s.sampleStep(ctx, step)
		if err != nil {
			return err
		}
		compared += n
	}
	// The same rule the other six checks follow: never a pass beside a failure
	// of the same name (fk.go, counts.go, residual.go, secondnet.go).
	if !s.failed(checkUnmaskedSameAs) {
		s.pass(checkUnmaskedSameAs, CodeSamplePassed, compared)
	}
	return nil
}

// sampleStep compares one step's sample and returns how many rows it compared.
func (s *state) sampleStep(ctx context.Context, step pipeline.Step) (int64, error) {
	table := s.tables[step.Table]
	if step.Keys == nil || step.Keys.Len() == 0 || len(step.Identity.Columns) == 0 {
		// A Lookup step carries no key set (pipeline.Step), so there is nothing
		// to fetch the source's copy of a row by. Reported, not failed.
		s.report(checkUnmaskedSameAs, CodeSampleReported, step.Table, "", 0)
		return 0, nil
	}
	if s.identityMasked(step) {
		s.report(checkUnmaskedSameAs, CodeSampleReported, step.Table, "", 0)
		return 0, nil
	}
	compare := s.unmaskedColumns(table, step.Identity.Columns)
	if len(compare) == 0 {
		s.report(checkUnmaskedSameAs, CodeSampleReported, step.Table, "", 0)
		return 0, nil
	}

	chunk, ok := firstChunk(step.Keys, sampleRows)
	if !ok {
		return 0, nil
	}
	idCols := step.Identity.Columns
	casts, ok := s.joinCasts(table, idCols)
	if !ok {
		s.report(checkUnmaskedSameAs, CodeSampleReported, step.Table, "", 0)
		return 0, nil
	}

	cols := append(append([]string{}, idCols...), compare...)
	sql := sampleSQL(step.Table, cols, idCols, casts, chunk)
	args := chunkArgs(chunk, len(idCols))

	targetRows, err := rowValues(ctx, s.target, sql, len(cols), args...)
	if err != nil {
		return 0, fmt.Errorf("verify: reading a sample of %s from the target: %w", step.Table, err)
	}
	if len(targetRows) == 0 {
		return 0, nil
	}
	sourceRows, ok := s.sourceSample(ctx, sql, len(cols), args)
	if !ok {
		// Section 6 makes an unconfirmable *residual hit* exit 9; a sample it
		// could not fetch is one of item 5's reported cases, so it is reported
		// and the run carries on.
		s.report(checkUnmaskedSameAs, CodeSampleReported, step.Table, "", 0)
		return 0, nil
	}

	bySource := make(map[string][]any, len(sourceRows))
	for _, row := range sourceRows {
		bySource[identityKey(row[:len(idCols)])] = row
	}

	var missing, differing int64
	for _, row := range targetRows {
		source, ok := bySource[identityKey(row[:len(idCols)])]
		if !ok {
			missing++
			continue
		}
		for i := range compare {
			j := len(idCols) + i
			if !sameValue(row[j], source[j]) {
				differing++
				break
			}
		}
	}
	if missing > 0 {
		s.report(checkUnmaskedSameAs, CodeSampleAbsent, step.Table, "", missing)
	}
	if differing > 0 {
		s.report(checkUnmaskedSameAs, CodeSampleDiffers, step.Table, "", differing)
	}
	return int64(len(targetRows)), nil
}

// boundedKeys is the accessor a pipeline.KeySet may offer for the first chunk
// alone. ARCHITECTURE.md section 2's KeySet has Len, Bytes and Chunks, and
// Chunks materialises the *whole* set as typed arrays: internal/plan's
// implementation allocates fresh arrays per chunk and copies every tuple into
// them, so asking a step at the --row-budget ceiling for chunks of 100 in order
// to read the first 100 allocates a second copy of that step's key set, per
// table, at verify time — after --memory-budget (section 8, exit 11) has been
// checked at plan and can no longer refuse anything.
//
// **Owed: pipeline.KeySet needs `FirstChunk(n int) Chunk` beside `Chunks`, and
// internal/plan's two key sets need to implement it.** Until then the fallback
// is Chunks, which is what every KeySet does today — a bounded sample is worth
// less than the sample compare it would otherwise skip on the largest tables.
// This is the same shape of deviation as verify.go's targetReader and it is
// recorded in internal/verify/CLAUDE.md with it.
type boundedKeys interface {
	FirstChunk(n int) pipeline.Chunk
}

// firstChunk is one chunk's worth of a step's keys, and whether there was one.
func firstChunk(keys pipeline.KeySet, n int) (pipeline.Chunk, bool) {
	if b, ok := keys.(boundedKeys); ok {
		c := b.FirstChunk(n)
		return c, c != nil && c.Len() > 0
	}
	chunks := keys.Chunks(n)
	if len(chunks) == 0 {
		return nil, false
	}
	return chunks[0], true
}

// sourceSample fetches one sample from the source through the short
// transaction, and reports whether it could.
//
// The source's answer is optional here in a way it is never optional in the
// residual scan: a sample that cannot be fetched is one of section 6 item 5's
// reported cases, while a residual hit that cannot be confirmed is exit 9. So
// the error is swallowed rather than returned, and the caller reports the
// table.
func (s *state) sourceSample(ctx context.Context, sql string, ncols int, args []any) ([][]any, bool) {
	short, err := s.shortReader(ctx)
	if err != nil {
		return nil, false
	}
	rows, err := rowValues(ctx, short, sql, ncols, args...)
	if err != nil {
		return nil, false
	}
	return rows, true
}

// identityMasked reports a step whose identity columns are not the same on both
// sides. A masked key is a real case — section 4 propagates a masked parent key
// onto every column referencing it — and the plan's keys are the source's, so
// there is no join that reaches the same row in both databases.
func (s *state) identityMasked(step pipeline.Step) bool {
	for _, c := range step.Identity.Columns {
		if d, ok := s.decision(ref.ColumnRef{Table: step.Table, Column: c}); ok && d.Masked {
			return true
		}
	}
	return false
}

// unmaskedColumns is every column of a table the run copied verbatim: not
// masked, and not generated — a generated column is computed by the target from
// the columns it depends on, so a masked dependency makes it differ by design
// and the loader never writes one at all.
func (s *state) unmaskedColumns(t *pipeline.Table, identity []string) []string {
	isIdentity := make(map[string]bool, len(identity))
	for _, c := range identity {
		isIdentity[c] = true
	}
	var out []string
	for _, c := range t.Columns {
		if isIdentity[c.Name] || c.Generated != "" {
			continue
		}
		if d, ok := s.decision(ref.ColumnRef{Table: t.Ref, Column: c.Name}); ok && d.Masked {
			continue
		}
		out = append(out, c.Name)
	}
	return out
}

// joinCasts is the cast the chunk side of each key comparison carries, and
// whether every identity column was found in the schema.
func (s *state) joinCasts(t *pipeline.Table, idCols []string) ([]string, bool) {
	casts := make([]string, len(idCols))
	for i, name := range idCols {
		col, ok := columnOf(t, name)
		if !ok {
			return nil, false
		}
		casts[i] = joinCast(col)
	}
	return casts, true
}

// joinCast is internal/extract/keys.go's: "" when the value arrives in the
// column's own type, and the column's type otherwise, because a key of a type
// with no array form in the chunk grammar travels as text.
//
// bpchar is deliberately not cast back: character(n) is blank-padded on disk
// and the planner reads such a key as text, so casting it back would compare a
// padded value against a trimmed key and match no row.
func joinCast(col pipeline.Column) string {
	switch col.TypeOID {
	case oidInt2, oidInt4, oidInt8, oidText, oidVarchar, oidBpchar, oidUUID:
		return ""
	}
	if strings.TrimPrefix(col.TypeName, "public.") == "citext" {
		return ""
	}
	return "::" + col.TypeName
}

// chunkArgs is one chunk as bound parameters: one typed array per identity
// column. []any is never among them, because pgx cannot infer an array OID for
// it (ARCHITECTURE.md section 2 "Chunk").
func chunkArgs(ch pipeline.Chunk, n int) []any {
	args := make([]any, n)
	for i := range n {
		args[i] = ch.Column(i)
	}
	return args
}

// identityKey is a row's identity as one comparable string. It is built from
// the same rendering both sides were read into (value.go), and it never leaves
// this function: it is a map key inside one comparison.
func identityKey(values []any) string {
	parts := make([]string, len(values))
	for i, v := range values {
		if v == nil {
			parts[i] = "\x00"
			continue
		}
		parts[i] = textOf(v)
	}
	return strings.Join(parts, "\x1f")
}
