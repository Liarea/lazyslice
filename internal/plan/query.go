// SPDX-License-Identifier: Apache-2.0

package plan

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
)

// The reads the planner makes. Every one of them goes through pipeline.Reader,
// which is Query and nothing else: the planner cannot write, cannot create a
// temp table and cannot leave anything behind on the source.

// readKeys runs a key-returning statement and collects the rows into a key set.
// A row with a NULL in a key column is dropped rather than stored: a key with a
// NULL in it identifies nothing, and storing one would make the set's own
// ordering meaningless.
func (p *run) readKeys(ctx context.Context, sql string, types []keyType, args ...any) (*keys, error) {
	rows, err := p.r.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := newKeys(types)
	ints := make([]pgtype.Int8, len(types))
	texts := make([]pgtype.Text, len(types))
	uuids := make([]pgtype.UUID, len(types))
	dest := make([]any, len(types))
	for i, ty := range types {
		switch ty.kind {
		case kindInt:
			dest[i] = &ints[i]
		case kindUUID:
			dest[i] = &uuids[i]
		case kindText, kindBpchar, kindOther:
			dest[i] = &texts[i]
		default:
			dest[i] = &texts[i]
		}
	}

	for rows.Next() {
		if err := rows.Scan(dest...); err != nil {
			return nil, err
		}
		tuple := make([]keyValue, len(types))
		ok := true
		for i, ty := range types {
			switch ty.kind {
			case kindInt:
				if !ints[i].Valid {
					ok = false
					continue
				}
				tuple[i].i = ints[i].Int64
			case kindUUID:
				if !uuids[i].Valid {
					ok = false
					continue
				}
				tuple[i].u = uuids[i]
			case kindText, kindBpchar, kindOther:
				if !texts[i].Valid {
					ok = false
					continue
				}
				tuple[i].s = texts[i].String
			default:
				if !texts[i].Valid {
					ok = false
					continue
				}
				tuple[i].s = texts[i].String
			}
		}
		if ok {
			out.add(tuple)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// scalar runs a statement that returns exactly one row and scans it.
func (p *run) scalar(ctx context.Context, sql string, dest ...any) error {
	rows, err := p.r.Query(ctx, sql)
	if err != nil {
		return err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return err
		}
		return fmt.Errorf("plan: %q returned no row", sql)
	}
	if err := rows.Scan(dest...); err != nil {
		return err
	}
	return rows.Err()
}

// boundedCount is §3's bounded lookup count: it can never read more than 1,001
// rows, so a table wrongly thought small costs one page read and not a scan.
func (p *run) boundedCount(ctx context.Context, t ref.TableRef) (int64, error) {
	var n int64
	if err := p.scalar(ctx, boundedCountSQL(t), &n); err != nil {
		return 0, fmt.Errorf("plan: counting %s: %w", t, err)
	}
	return n, nil
}

// unreadableRelation is one relation the source role cannot SELECT, and the
// planned table it belongs to. The two differ for a partition leaf: §3.3 reads
// a partitioned table's keys from the root and lets Postgres route to the
// leaves, so an unreadable leaf is the root's refusal, and the GRANT that
// clears it names the leaf.
type unreadableRelation struct {
	table    ref.TableRef
	relation ref.TableRef
}

// readUnreadable is the relations the source role cannot SELECT, in (schema,
// name) order, each attributed to the planned table it belongs to.
func (p *run) readUnreadable(ctx context.Context) ([]unreadableRelation, error) {
	rows, err := p.r.Query(ctx, sqlUnreadableTables)
	if err != nil {
		return nil, fmt.Errorf("plan: reading table privileges: %w", err)
	}
	defer rows.Close()
	var out []unreadableRelation
	for rows.Next() {
		var schema, name string
		if err := rows.Scan(&schema, &name); err != nil {
			return nil, fmt.Errorf("plan: reading table privileges: %w", err)
		}
		rel := ref.TableRef{Schema: schema, Name: name}
		tbl := rel
		if root, ok := p.rootOf[rel]; ok {
			tbl = root
		}
		out = append(out, unreadableRelation{table: tbl, relation: rel})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("plan: reading table privileges: %w", err)
	}
	return out, nil
}

// readRole is the role name the GRANT statement in a refusal names.
func (p *run) readRole(ctx context.Context) (string, error) {
	var role string
	if err := p.scalar(ctx, sqlCurrentRole, &role); err != nil {
		return "", fmt.Errorf("plan: reading the source role: %w", err)
	}
	return role, nil
}

// The per-type widths the byte estimate uses. There is no width in
// pipeline.Schema and none of the numbers below is a promise: the estimate
// exists so that the plan can print an order of magnitude beside the row count
// (§3.5), and the flag that changes it is the one that changes the row count.
const (
	widthDefault  int64 = 32
	widthText     int64 = 32
	widthTextCap  int64 = 64
	widthDocument int64 = 128
)

// rowWidth is the estimated bytes one row of a table occupies.
func rowWidth(t pipeline.Table) int64 {
	var total int64
	for _, c := range t.Columns {
		total += columnWidth(c)
	}
	if total == 0 {
		return widthDefault
	}
	return total
}

func columnWidth(c pipeline.Column) int64 {
	switch c.TypeOID {
	case 16: // bool
		return 1
	case 21: // int2
		return 2
	case 23, 700, 1082, 1083: // int4, float4, date, time
		return 4
	case 20, 701, 1114, 1184: // int8, float8, timestamp, timestamptz
		return 8
	case 2950: // uuid
		return 16
	case 829: // macaddr
		return 8
	case 869, 650: // inet, cidr
		return 16
	case 1700: // numeric
		return 16
	case 114, 3802, 3614: // json, jsonb, tsvector
		return widthDocument
	case 25, 1042, 1043: // text, bpchar, varchar
		if c.TypMod > 4 && int64(c.TypMod)-4 < widthTextCap {
			return int64(c.TypMod) - 4
		}
		return widthText
	}
	return widthDefault
}

// The document-leaf count the residual filter estimate needs (§3, §6). A masked
// json, jsonb or hstore column has every scalar leaf of every document replaced
// (§4), and every replacement is one entry in the residual filter keyed by its
// path, so one such column costs a row as many cells as the document has
// leaves.

const (
	oidJSON  uint32 = 114
	oidJSONB uint32 = 3802

	// jsonLeafDefault is the leaves per document assumed for a masked document
	// column no sample parsed for: an unsampled table, an unreadable one, or an
	// hstore pgx handed back in a form this counter does not walk. Counting such
	// a column as one cell is the failure that matters — the memory budget is a
	// control against the OOM of THREAT_MODEL.md T11, and an estimate that is
	// low lets a run past it — while counting one high costs 29 bits a row.
	jsonLeafDefault = 8

	// jsonLeafMaxDepth and jsonLeafMax bound the walk over one sampled document,
	// the way internal/classify bounds its own: a sample is a value from
	// production and its shape is not ours to trust.
	jsonLeafMaxDepth = 16
	jsonLeafMax      = 4096
)

// documentType says whether a column holds a document whose leaves are masked
// one by one.
func documentType(c pipeline.Column) bool {
	if c.TypeOID == oidJSON || c.TypeOID == oidJSONB {
		return true
	}
	// hstore is an extension type, so it has no fixed OID and is recognised by
	// name, the way citext is in keyset.go.
	switch c.TypeName {
	case "hstore", "public.hstore":
		return true
	}
	return false
}

// leavesPerDocument is how many residual-filter cells one masked value of
// column idx costs, averaged over the samples that parsed and rounded up.
func leavesPerDocument(t pipeline.Table, idx int, c pipeline.Column) int {
	if !documentType(c) {
		return 1
	}
	total, docs := 0, 0
	for _, row := range t.Samples {
		if idx >= len(row) {
			continue
		}
		if n := countJSONLeaves(row[idx]); n > 0 {
			total += n
			docs++
		}
	}
	if docs == 0 {
		return jsonLeafDefault
	}
	if avg := (total + docs - 1) / docs; avg > 1 {
		return avg
	}
	return 1
}

// countJSONLeaves counts the scalar leaves of one sampled document. A document
// that does not parse counts none, and the caller falls back to the default.
func countJSONLeaves(v any) int {
	var doc any
	switch t := v.(type) {
	case nil:
		return 0
	case []byte:
		if json.Unmarshal(t, &doc) != nil {
			return 0
		}
	case string:
		if json.Unmarshal([]byte(t), &doc) != nil {
			return 0
		}
	default:
		doc = v
	}
	n := 0
	walkJSONLeaves(doc, &n, 0)
	return n
}

func walkJSONLeaves(v any, n *int, depth int) {
	if depth > jsonLeafMaxDepth || *n >= jsonLeafMax {
		return
	}
	switch t := v.(type) {
	case map[string]any:
		for _, child := range t {
			walkJSONLeaves(child, n, depth+1)
		}
	case []any:
		for _, child := range t {
			walkJSONLeaves(child, n, depth+1)
		}
	case nil:
	default:
		*n++
	}
}
