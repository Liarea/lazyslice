// SPDX-License-Identifier: Apache-2.0

package pg

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Liarea/lazyslice/internal/pipeline"
)

// sqlTargetUserTypes resolves the source's user-defined type names against the
// target's catalog, and names each one's array type alongside it.
//
// The array type is read rather than spelled `_name`, because Postgres does not
// promise that spelling: the array type of a 63-character name is truncated, and
// a name already taken gets underscores prepended until one is free. Guessing it
// would silently skip registration for exactly the column that needs it most —
// ARCHITECTURE.md §11.1's type list is what makes an enum array encodable at
// CopyFrom, and a name that resolves to nothing is not an error in pgx's
// LoadTypes, it is a type quietly left unregistered.
//
// A name the target does not have comes back as no row. That is the ordinary
// case for a target whose DDL has not run yet, not a failure: the registry is
// filled again after the DDL and the pool is reset, so a connection made before
// then simply carries fewer types than a connection made after.
//
// `typtype` covers the three classes Schema names, plus 'b': an extension's own
// base type — `citext` — is a base type, and registerExtensionBaseTypes puts it
// into the same $1 list once it has resolved the name from the extension. The
// join then finds its array type the same way, which is the whole point:
// pgx cannot build a codec for `_citext` unless `citext` is on the map first.
const sqlTargetUserTypes = `SELECT n.nspname || '.' || t.typname,
       CASE WHEN t.typarray <> 0 THEN an.nspname || '.' || a.typname END
FROM pg_type t
JOIN pg_namespace n ON n.oid = t.typnamespace
LEFT JOIN pg_type a ON a.oid = t.typarray
LEFT JOIN pg_namespace an ON an.oid = a.typnamespace
WHERE t.typtype IN ('e', 'd', 'c', 'b')
  AND n.nspname || '.' || t.typname = ANY($1::text[])`

// sqlTargetExtensionBaseTypes is the base types the source's extensions own, with
// the OID and the type category the server reports for each.
//
// It exists because pgx cannot resolve an array over one of them on its own.
// `conn.LoadTypes(["public._citext"])` walks to the element type, finds a base
// type that is neither user-defined in pgx's sense (enum, domain, composite,
// range, multirange) nor in its default map, and gives up — so `_citext` came
// back in loadTypes's skipped list and Plausible's
// `monthly_reports.recipients citext[]` wrote nonsense down the binary COPY
// (testdata/regressions/005-array-of-extension-type-not-registered.sql).
//
// Registering the element first is what lets LoadTypes build the array codec
// over it. Only the **string** category is registered, and that is the whole of
// the claim being made: a string-category type is a varlena whose binary form is
// its bytes, which is what pgtype.TextCodec encodes, and citext, ltree and
// their kind are that. A category-'U' type — hstore, PostGIS geometry,
// pgvector's vector — has a binary form of its own that TextCodec would corrupt,
// so it is left exactly as it was: unregistered, scalar columns loading as they
// always did, arrays over it still refused by the loader rather than silently
// mangled.
const sqlTargetExtensionBaseTypes = `SELECT n.nspname || '.' || t.typname, t.oid, t.typcategory
FROM pg_type t
JOIN pg_namespace n ON n.oid = t.typnamespace
WHERE t.typtype = 'b' AND t.typelem = 0 AND t.typarray <> 0
  AND EXISTS (
      SELECT 1 FROM pg_depend d
      JOIN pg_extension e ON e.oid = d.refobjid
      WHERE d.objid = t.oid
        AND d.classid = 'pg_type'::regclass
        AND d.refclassid = 'pg_extension'::regclass
        AND e.extname = ANY($1::text[]))`

// typeRegistry is the set of user-defined types every target connection must
// have registered on its pgx type map, and the AfterConnect hook that registers
// them (ARCHITECTURE.md §11.1, ADR-005: "types registered in AfterConnect").
//
// Without it a value of a user-defined type has no encode plan on the target and
// CopyFrom fails mid-table. Measured on postgres:16 through this package, one
// column at a time, with and without the registration:
//
//	composite                      42804 unregistered, loads registered
//	array of an enum               54000 unregistered, loads registered
//	array of a domain over text    54000 unregistered, loads registered
//	enum                           loads either way
//	domain over text               loads either way
//	array of a built-in type       loads either way
//
// So this is not a formality for four object classes: it is the difference
// between a load and a failure for a composite and for an array over *any*
// user-defined type, a domain included — which is why domains stay in the set
// even though a scalar domain column needs nothing (the server reports it as its
// base type on the wire). THREAT_MODEL.md T8's empty-or-complete property then
// cleans up after the failure, so the damage is a failed run rather than a
// half-loaded target; the run still fails.
//
// It is filled after the DDL has created the types and not at OpenTarget,
// because at OpenTarget the target does not have them yet: lazyslice owns the
// target schema and creates every type in it (§11.1 item 3). The names are read
// under the lock on every new connection, so the hook is correct both before the
// DDL (nothing to register) and after it (everything), and RegisterTypes resets
// the pool so that a connection made before the DDL — the gate's, the drop's,
// the DDL's own — is retired rather than reused with an empty type map.
type typeRegistry struct {
	mu    sync.RWMutex
	names []string
	// exts is the source's extension names. What they are for is resolved on
	// the target (sqlTargetExtensionBaseTypes), because an extension's base
	// types are not in Schema.Enums, Schema.Domains or Schema.Composites and
	// cannot be named from the source schema at all.
	exts []string
	// unregistered is what the hook asked for and could not get a codec for,
	// by name. It is a report and not a failure (loadTypes says why); the
	// column keeps the behaviour it had before any of this existed.
	unregistered map[string]bool
}

// want returns the names, and the extensions, to register on a new connection.
func (r *typeRegistry) want() (names, exts []string) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.names, r.exts
}

// use replaces the set. The slice is copied and never mutated afterwards, so
// want can hand it out under a read lock alone. The skip list is dropped with
// the old set: it is a statement about the names being replaced.
func (r *typeRegistry) use(names, exts []string) {
	sorted := append([]string(nil), names...)
	sort.Strings(sorted)
	sortedExts := append([]string(nil), exts...)
	sort.Strings(sortedExts)
	r.mu.Lock()
	defer r.mu.Unlock()
	r.names = sorted
	r.exts = sortedExts
	r.unregistered = nil
}

// skip records the names a connection could not register.
func (r *typeRegistry) skip(names []string) {
	if len(names) == 0 {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.unregistered == nil {
		r.unregistered = make(map[string]bool, len(names))
	}
	for _, n := range names {
		r.unregistered[n] = true
	}
}

// skipped is the sorted set of names no connection could register. Nothing in
// the load reads it; the type-registration tests do, because a skip is
// deliberately quiet everywhere else.
func (r *typeRegistry) skipped() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.unregistered))
	for n := range r.unregistered {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// afterConnect is the target pool's AfterConnect hook. It runs on every new
// target connection and on no source connection: a hook on the source pool is
// what T-0076 removed and what internal/pg/CLAUDE.md's Never list forbids,
// because session state set through a transaction-pooling PgBouncer outlives the
// run on a server connection that is not ours. Nothing here sets session state —
// it reads the catalog and fills a client-side type map — but the pool it is
// registered on is still the target's alone.
//
// **An error here kills every target connection**, because pgx fails the
// connection, which fails the acquire, which fails every acquire after it: the
// pool is dead and the run with it, at a point where the load has already
// dropped the target and run its DDL. So the only thing that returns one is a
// failure of the connection itself — the catalog read, or a cancelled context.
// A type pgx cannot build a codec for is skipped instead, and loadTypes is where
// that is decided.
func (r *typeRegistry) afterConnect(ctx context.Context, conn *pgx.Conn) error {
	names, exts := r.want()
	if len(names) == 0 && len(exts) == 0 {
		return nil
	}
	// The extensions' own string base types go on the map first: pgx needs the
	// element registered before it can build a codec for the array over it.
	extNames, err := registerExtensionBaseTypes(ctx, conn, exts)
	if err != nil {
		return err
	}
	present, err := targetTypeNames(ctx, conn, append(append([]string(nil), names...), extNames...))
	if err != nil {
		return err
	}
	if len(present) == 0 {
		return nil
	}
	types, skipped, err := loadTypes(ctx, conn, present)
	if err != nil {
		return fmt.Errorf("pg: registering the source's user-defined types on a target connection: %w", err)
	}
	r.skip(skipped)
	for _, t := range types {
		// In place, because an array type LoadTypes built over this one holds a
		// pointer to this very *pgtype.Type: replacing the entry in the map would
		// leave the array's element still carrying the plain codec.
		if cc, ok := t.Codec.(*pgtype.CompositeCodec); ok {
			t.Codec = &compositeCodec{CompositeCodec: cc}
		}
	}
	if len(types) == 0 {
		return nil
	}
	// LoadTypes already registers what it loaded on this connection's own map;
	// registering the returned types is the documented call and is idempotent, so
	// it is made rather than relying on the internal one staying.
	conn.TypeMap().RegisterTypes(types)
	return nil
}

// registerExtensionBaseTypes puts the string-category base types the source's
// extensions own onto this connection's type map, with pgtype.TextCodec, and
// returns their names so the caller can ask for their array types too.
//
// See sqlTargetExtensionBaseTypes for why only the string category, and for what
// this does not attempt. An extension the target does not have contributes no
// rows and no error: §11.1 item 2 creates the ones a recreated object depends
// on, and a connection made before that DDL simply registers fewer types, the
// same way the rest of this file already tolerates.
func registerExtensionBaseTypes(ctx context.Context, conn *pgx.Conn, exts []string) ([]string, error) {
	if len(exts) == 0 {
		return nil, nil
	}
	rows, err := conn.Query(ctx, sqlTargetExtensionBaseTypes, exts)
	if err != nil {
		return nil, fmt.Errorf("pg: reading the base types of the source's extensions on the target: %w", err)
	}
	defer rows.Close()
	var names []string
	for rows.Next() {
		var (
			name     string
			oid      uint32
			category string
		)
		if err := rows.Scan(&name, &oid, &category); err != nil {
			return nil, fmt.Errorf("pg: reading the base types of the source's extensions on the target: %w", err)
		}
		bare := name
		if i := strings.LastIndexByte(bare, '.'); i >= 0 {
			bare = bare[i+1:]
		}
		codec, ok := extensionCodec(bare, category)
		if !ok {
			continue
		}
		conn.TypeMap().RegisterType(&pgtype.Type{Name: bare, OID: oid, Codec: codec})
		names = append(names, name)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("pg: reading the base types of the source's extensions on the target: %w", err)
	}
	return names, nil
}

// extensionCodec is the codec to register for one of an extension's base types,
// and the whole of what this file claims to know about them.
//
// Two entries, and the reason for each is different.
//
// A **string-category** type is a varlena whose binary form is its bytes —
// `citext` and its kind — which is exactly what pgtype.TextCodec writes, so the
// category alone is enough and no list of names is needed.
//
// `hstore` is not string-category and its binary form is nothing like its text
// form, so the category rule would corrupt it. pgx ships a codec for it, and
// registering that is what makes an hstore column loadable at all: without it
// the value arrives as its text form, pgx sends those bytes as the binary body,
// and the server reports 08P01 ("insufficient data left in message") — measured
// on a one-column table, and true before any of this changed
// (testdata/regressions/005-array-of-extension-type-not-registered.sql).
// pgtype.HstoreCodec.DecodeValue returns a pgtype.Hstore, which is a
// HstoreValuer, so pgx's scan-the-string-then-encode fallback completes — the
// same route compositeCodec below relies on.
//
// Everything else is refused here rather than guessed at. A category-'U' type
// pgx ships no codec for — PostGIS geometry, pgvector's vector, ltree — keeps
// the behaviour it has always had: unregistered, and whatever that column did
// before, it still does.
func extensionCodec(name, category string) (pgtype.Codec, bool) {
	if name == "hstore" {
		return pgtype.HstoreCodec{}, true
	}
	if category == "S" {
		return pgtype.TextCodec{}, true
	}
	return nil, false
}

// loadTypes resolves as many of the names as pgx can build a codec for, and
// reports the rest rather than failing.
//
// pgx's LoadTypes is all-or-nothing over the whole list: it walks each name's
// dependency closure, and a dependency that is neither user-defined nor in
// pgx's own default type map ends the call with an error naming it — every type
// in the list is then left unregistered. A single such type therefore used to
// take down every target connection through afterConnect above. Measured, on a
// stock postgres:16 with no extension at all: `CREATE DOMAIN d AS money`,
// `AS pg_lsn` and `AS tsquery` each do it, and so does anything over citext
// (`CREATE DOMAIN email AS citext`, or a composite with a citext field) —
// citext being a type §11.1 item 2 recreates and internal/plan already
// special-cases, so a schema lazyslice supports. The load had dropped the
// target and run its DDL by then, so the operator was left with an empty target
// and a message from inside the driver.
//
// A name left unregistered is exactly the state that column was in before any
// of this existed: it loads if it did before (a scalar domain does) and fails
// at CopyFrom if it did not (a composite does, 42804). That is strictly better
// than a pool that cannot serve any table.
//
// The whole list is tried first, because that is one round trip and the answer
// for every schema whose types resolve. The retry is one call per name — pgx
// registers each name's closure on the connection's own map as it goes, so a
// composite over another user-defined type resolves in its own call — and the
// names that fail there are the ones returned as skipped.
func loadTypes(ctx context.Context, conn *pgx.Conn, names []string) ([]*pgtype.Type, []string, error) {
	types, err := conn.LoadTypes(ctx, names)
	if err == nil {
		return types, nil, nil
	}
	if ctx.Err() != nil {
		return nil, nil, ctx.Err()
	}
	var (
		loaded  []*pgtype.Type
		skipped []string
	)
	for _, n := range names {
		one, err := conn.LoadTypes(ctx, []string{n})
		if err != nil {
			if ctx.Err() != nil {
				return nil, nil, ctx.Err()
			}
			skipped = append(skipped, n)
			continue
		}
		loaded = append(loaded, one...)
	}
	return loaded, skipped, nil
}

// compositeCodec is pgx's CompositeCodec with one method replaced, and it is
// what makes a composite-typed column loadable at all.
//
// The value a composite column arrives with is a Go string holding the type's
// text form — `(1.5,GBP)` — because the source pool runs in
// pgx.QueryExecModeExec (pg.go), where every result comes back in text format,
// and internal/extract scans each cell into an `any`. pgx's binary COPY then has
// to encode that string into the target's composite OID, and
// pgtype.CompositeCodec.PlanEncode takes a CompositeIndexGetter and nothing
// else — a string gets no plan. pgx has one fallback for exactly this shape
// (tryScanStringCopyValueThenEncode in values.go): scan the string in *text*
// format into an `any`, then encode that `any` in binary. Scanning into an `any`
// runs the codec's DecodeValue, and CompositeCodec.DecodeValue returns a
// map[string]any — which is not a CompositeIndexGetter either, so the fallback
// dead-ends and the load fails with "cannot find encode plan" (measured).
//
// So DecodeValue here returns a value that *is* a CompositeIndexGetter, and the
// fallback completes. This is one method over the driver's own scanner, not a
// second composite implementation: the fields are read with pgx's own
// CompositeTextScanner and encoded by pgx's own CompositeCodec, and everything
// else about the codec is pgx's, embedded.
//
// It carries arrays of composites with it. pgtype.ArrayCodec decodes an element
// through the map by OID and encodes it back the same way, so an array whose
// element is this codec scans to a slice of composite getters and re-encodes.
//
// **This rests on pgx internals, and the version it was measured against is
// pgx v5.10.0.** tryScanStringCopyValueThenEncode is unexported and is not part
// of pgx's public contract; neither is the fact that it scans in text format,
// nor the fact that an ArrayCodec holds a pointer to the very *pgtype.Type
// LoadTypes returned (which is what makes the in-place mutation above work). An
// upgrade that drops the fallback would otherwise surface as trap 27 failing in
// the integration suite with "cannot find encode plan" and nothing saying why,
// so TestPgxStillFallsBackFromAStringToACompositeEncode asserts the fallback
// directly, in a unit test, with a message naming it.
type compositeCodec struct {
	*pgtype.CompositeCodec
}

// compositeFields is one composite value, as pgtype's own encoder wants it.
type compositeFields []any

var _ pgtype.CompositeIndexGetter = compositeFields(nil)

// IsNull is always false: a NULL composite reaches DecodeValue as a nil src and
// never becomes one of these.
func (f compositeFields) IsNull() bool { return false }

func (f compositeFields) Index(i int) any {
	if i < 0 || i >= len(f) {
		return nil
	}
	return f[i]
}

// DecodeValue returns the composite as a CompositeIndexGetter rather than as a
// map[string]any. The field values are whatever each field type's own codec
// decodes to, which is the representation that codec also encodes from, so the
// round trip through the map is closed for every field type pgx understands.
//
// **Text format only.** That is the whole of what the COPY fallback needs — it
// scans the source's text form — and it is as far as this override should
// reach: nothing but the fallback has a reason to see a type private to this
// package, so a binary read through a registered connection keeps pgx's own
// map[string]any.
//
// The reason first written here — that internal/verify reads target columns
// through this pool — was **wrong**, and the narrative built on it with it.
// internal/verify does not read through this pool: internal/core opens a second
// target pool of its own with pg.Connect(ctx, d, nil), no AfterConnect and no
// registration, and that is what verify's target reader is (internal/core's
// readableWriter). Measured on an unregistered pool, a composite scans into an
// `any` as the string "(1234.50,GBP)" and an enum array as "{pending,active}" —
// the same strings the source side gives — so this codec is not on verify's
// path at all and §6 item 5 compares like with like.
func (c *compositeCodec) DecodeValue(m *pgtype.Map, oid uint32, format int16, src []byte) (any, error) {
	if src == nil {
		return nil, nil
	}
	if format != pgtype.TextFormatCode {
		return c.CompositeCodec.DecodeValue(m, oid, format, src)
	}
	scanner := pgtype.NewCompositeTextScanner(m, src)
	fields := make(compositeFields, 0, len(c.Fields))
	for i := 0; scanner.Next() && i < len(c.Fields); i++ {
		var v any
		plan := m.PlanScan(c.Fields[i].Type.OID, format, &v)
		if plan == nil {
			return nil, fmt.Errorf("pg: no plan to read field %q (OID %d) of composite OID %d",
				c.Fields[i].Name, c.Fields[i].Type.OID, oid)
		}
		if err := plan.Scan(scanner.Bytes(), &v); err != nil {
			// The field's value is in the driver's error text, so it is not
			// wrapped: the field is named and nothing else (THREAT_MODEL.md T4).
			return nil, fmt.Errorf("pg: reading field %q of composite OID %d", c.Fields[i].Name, oid)
		}
		fields = append(fields, v)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("pg: reading a value of composite OID %d", oid)
	}
	return fields, nil
}

// targetTypeNames expands the source's type names into the names to hand
// LoadTypes: each type the target actually has, plus its array type.
func targetTypeNames(ctx context.Context, conn *pgx.Conn, names []string) ([]string, error) {
	rows, err := conn.Query(ctx, sqlTargetUserTypes, names)
	if err != nil {
		return nil, fmt.Errorf("pg: reading the target's user-defined types: %w", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var name string
		var array pgtype.Text
		if err := rows.Scan(&name, &array); err != nil {
			return nil, fmt.Errorf("pg: reading the target's user-defined types: %w", err)
		}
		out = append(out, name)
		if array.Valid {
			out = append(out, array.String)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("pg: reading the target's user-defined types: %w", err)
	}
	return out, nil
}

// userTypeNames is the source's enum, domain and composite type names, as
// internal/introspect wrote them into pipeline.Schema: qualified and unquoted,
// `public.account_status`, which is the spelling the catalog compares against.
//
// **Domains are in the set for the sake of arrays over them, measured.** A
// scalar domain column is reported as its base type on the wire and copies with
// nothing registered, so on its own it would not be here; an array of a domain
// is not the same case and does not — `postal[]` fails 54000 unregistered and
// loads registered (see typeRegistry). A domain whose base type pgx cannot build
// a codec for is skipped by loadTypes rather than failing the pool, which is
// what makes asking for all of them cheap.
//
// The array types are not here. They are resolved on the target instead
// (sqlTargetUserTypes), because their names are the server's to choose and not
// ours to spell.
func userTypeNames(s *pipeline.Schema) []string {
	if s == nil {
		return nil
	}
	n := len(s.Enums) + len(s.Domains) + len(s.Composites)
	seen := make(map[string]bool, n)
	names := make([]string, 0, n)
	add := func(n string) {
		if n == "" || seen[n] {
			return
		}
		seen[n] = true
		names = append(names, n)
	}
	for n := range s.Enums {
		add(n)
	}
	for _, d := range s.Domains {
		add(d.Name)
	}
	for _, d := range s.Composites {
		add(d.Name)
	}
	sort.Strings(names)
	return names
}

// extensionNames is the source's installed extensions, as internal/introspect
// recorded them: "every installed extension a recreated column, index or default
// depends on" (pipeline.Schema). registerExtensionBaseTypes turns them into type
// names on the target; nothing here can, because an extension's base type is not
// an enum, a domain or a composite and appears in none of Schema's type maps.
func extensionNames(s *pipeline.Schema) []string {
	if s == nil {
		return nil
	}
	seen := make(map[string]bool, len(s.Extensions))
	out := make([]string, 0, len(s.Extensions))
	for _, e := range s.Extensions {
		if e.Name == "" || seen[e.Name] {
			continue
		}
		seen[e.Name] = true
		out = append(out, e.Name)
	}
	sort.Strings(out)
	return out
}

// registerTypes registers the source's user-defined types on every target
// connection from here on (ARCHITECTURE.md §11.1, ADR-005). The schema is the
// one being loaded; the names come out of it.
//
// The one entry point is writer.RegisterTypes (writer.go), which is
// pipeline.TypeRegistrar and is what the loader calls: internal/load holds a
// Writer and never a Target. Target had an exported RegisterTypes of its own
// until this was reviewed; it had no caller but a test, and two public doors
// onto one private room is how the next reader ends up behind the wrong one.
//
// It is called after the DDL that creates those types has run, and it resets the
// pool: pgx's AfterConnect runs once per connection, so a connection opened
// before the types existed carries a type map without them and would fail the
// very CopyFrom this exists for. pgxpool.Reset destroys the idle connections and
// marks the busy ones for destruction on release, so the next acquire builds a
// connection that runs the hook again.
func registerTypes(ctx context.Context, pool *pgxpool.Pool, reg *typeRegistry, s *pipeline.Schema) error {
	if reg == nil || pool == nil {
		return errors.New("pg: registering the source's user-defined types: no target pool")
	}
	names := userTypeNames(s)
	exts := extensionNames(s)
	reg.use(names, exts)
	if len(names) == 0 && len(exts) == 0 {
		// Nothing to register, so the hook is a no-op and the connections already
		// open are as good as new ones. A pool reset here would cost the gate's
		// connection for nothing.
		return nil
	}
	pool.Reset()
	// One connection is made eagerly so that a target that cannot be connected to
	// at all is an error here, where the stage that asked can name it, rather than
	// an opaque acquire failure inside the first CopyFrom. It is not a check that
	// every type registered: a type pgx cannot build a codec for is skipped by the
	// hook (loadTypes), and reg.skipped() is where that is readable.
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("pg: registering the source's user-defined types on the target: %w", err)
	}
	conn.Release()
	return nil
}
