// SPDX-License-Identifier: Apache-2.0

package pg

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/Liarea/lazyslice/internal/dsn"
	"github.com/Liarea/lazyslice/internal/pipeline"
)

// The names RegisterTypes asks the target for are the source's, spelled as
// internal/introspect writes them into pipeline.Schema: qualified and unquoted.
// A different spelling matches no row of pg_type and the registration is a
// silent no-op — LoadTypes returns no error for a name it cannot find — so this
// is the one half of the mechanism a unit test can hold.
func TestUserTypeNamesAreTheSchemasEnumsDomainsAndComposites(t *testing.T) {
	s := &pipeline.Schema{
		Enums: map[string][]string{
			"public.account_status": {"pending", "active"},
			"billing.tier":          {"free"},
		},
		Domains:    []pipeline.NamedDef{{Name: "public.postal", Def: "CREATE DOMAIN ..."}},
		Composites: []pipeline.NamedDef{{Name: "public.money_amount", Def: "CREATE TYPE ..."}},
	}
	want := []string{"billing.tier", "public.account_status", "public.money_amount", "public.postal"}
	if got := userTypeNames(s); !reflect.DeepEqual(got, want) {
		t.Errorf("userTypeNames = %v, want %v", got, want)
	}
}

// A schema with no user-defined type asks for nothing, and a nil schema is not
// a panic: the loader calls this on every run, including one over a catalog
// that declares no type at all.
func TestUserTypeNamesOfASchemaWithNoUserTypes(t *testing.T) {
	if got := userTypeNames(nil); len(got) != 0 {
		t.Errorf("userTypeNames(nil) = %v, want none", got)
	}
	if got := userTypeNames(&pipeline.Schema{}); len(got) != 0 {
		t.Errorf("userTypeNames(empty) = %v, want none", got)
	}
}

// The array types are not spelled here, and that is deliberate: Postgres names
// the array type of a 63-character name by truncating it, and prepends
// underscores when the obvious spelling is taken, so `_name` is a guess and a
// wrong guess registers nothing. They are resolved from the target's own
// catalog instead (sqlTargetUserTypes).
func TestUserTypeNamesDoesNotGuessAnArrayTypesName(t *testing.T) {
	s := &pipeline.Schema{Enums: map[string][]string{"public.mood": {"sad"}}}
	for _, n := range userTypeNames(s) {
		if strings.Contains(n, "_mood") {
			t.Errorf("userTypeNames spelled the array type %q; the target's catalog names it, not us", n)
		}
	}
}

// internal/pg/CLAUDE.md's Never list, as a check rather than a convention: the
// source pool may not carry an AfterConnect hook (T-0076 — a session hook on a
// pooled source sets state on the pooler's shared server connection, and it
// outlived the run there). The hook this task added is the target's alone, and
// Connect refuses one on a tracer-carrying pool rather than trusting the two
// call sites to stay right.
func TestConnectRefusesAnAfterConnectHookOnASourcePool(t *testing.T) {
	tr, err := NewTracer(SourceShapes()...)
	if err != nil {
		t.Fatalf("compiling the source shapes: %v", err)
	}
	hook := func(context.Context, *pgx.Conn) error { return nil }
	pool, err := Connect(context.Background(), dsn.DSN(testDSN), tr, withAfterConnect(hook))
	if err == nil {
		pool.Close()
		t.Fatal("Connect opened a source pool carrying an AfterConnect hook")
	}
	if !strings.Contains(err.Error(), "AfterConnect") {
		t.Errorf("Connect refused with %q, want a message naming the hook", err)
	}
}

// The target pool carries the hook, and it carries nothing else new: the same
// call with no tracer opens.
func TestTheTargetPoolCarriesTheTypeRegistrationHook(t *testing.T) {
	target, err := OpenTarget(context.Background(), dsn.DSN(testDSN))
	if err != nil {
		t.Fatalf("OpenTarget: %v", err)
	}
	defer target.Close()
	if target.pool.Config().AfterConnect == nil {
		t.Error("the target pool has no AfterConnect hook, so no connection would ever register a type")
	}
	if target.types == nil {
		t.Fatal("the target has no type registry")
	}
	if got, exts := target.types.want(); len(got) != 0 || len(exts) != 0 {
		t.Errorf("the registry starts holding types %v and extensions %v; the target has none of the source's "+
			"types until the DDL has run", got, exts)
	}
}

// registerTypes with nothing to register does not reset the pool. A reset
// throws away the gate's connection for no reason, and the hook is a no-op
// without names anyway.
func TestRegisteringNoTypesLeavesThePoolAlone(t *testing.T) {
	target, err := OpenTarget(context.Background(), dsn.DSN(testDSN))
	if err != nil {
		t.Fatalf("OpenTarget: %v", err)
	}
	defer target.Close()

	// Through the Writer, because that is the only entry point: internal/load
	// holds a pipeline.Writer and the loader calls writer.RegisterTypes, which
	// is one of pipeline.Writer's four methods (T-0093) and so needs no
	// assertion here.
	w, err := target.Writer(context.Background())
	if err != nil {
		t.Fatalf("Writer: %v", err)
	}
	// The DSN points at nothing, so an eager acquire would fail. A registration
	// with no names must not attempt one.
	if err := w.RegisterTypes(context.Background(), &pipeline.Schema{}); err != nil {
		t.Errorf("RegisterTypes over a schema with no user types = %v, want no error and no connection", err)
	}
}

// compositeCodec exists because of one unexported function in pgx, and this is
// the assertion that pgx still has it.
//
// pgx's binary COPY encodes each value against the target's OID
// (encodeCopyValue, values.go). A composite arrives from the source as its text
// form — a Go string — and pgtype.CompositeCodec plans an encode for a
// CompositeIndexGetter and nothing else, so the direct encode fails and pgx
// falls back to tryScanStringCopyValueThenEncode: scan the string in *text*
// format into an `any`, then encode that `any` in binary. Both halves of that
// fallback are pgx internals and neither is part of its public contract. The
// three calls below are the fallback's own three steps, made through the
// exported API against a type map built by hand, so an upgrade that changes any
// of them fails here — by name — instead of failing testdata/nasty.sql trap 27
// in the integration suite with "cannot find encode plan" and nothing to say
// why. Measured against pgx v5.10.0.
func TestPgxStillFallsBackFromAStringToACompositeEncode(t *testing.T) {
	const oid = 90000 // no real type; the map is ours and nothing sends these bytes
	const text = "(1234.50,GBP)"

	fields := func(m *pgtype.Map) []pgtype.CompositeCodecField {
		num, ok := m.TypeForOID(pgtype.NumericOID)
		if !ok {
			t.Fatal("pgx's default type map has no numeric")
		}
		txt, ok := m.TypeForOID(pgtype.TextOID)
		if !ok {
			t.Fatal("pgx's default type map has no text")
		}
		return []pgtype.CompositeCodecField{
			{Name: "amount", Type: num},
			{Name: "currency", Type: txt},
		}
	}

	// Step 1, the control: the direct binary encode of the string must fail.
	// If it ever stops failing, pgx encodes a composite from its text form on
	// its own and compositeCodec has nothing left to do.
	plain := pgtype.NewMap()
	plain.RegisterType(&pgtype.Type{Name: "money_amount", OID: oid,
		Codec: &pgtype.CompositeCodec{Fields: fields(plain)}})
	if _, err := plain.Encode(oid, pgtype.BinaryFormatCode, text, nil); err == nil {
		t.Error("pgx now encodes a composite directly from its text form; compositeCodec may be redundant")
	}

	// Step 2 and 3 with pgx's own codec: scan the text form into an `any`, then
	// encode that `any` in binary. This is the dead end compositeCodec exists
	// for — DecodeValue gives a map[string]any, which is not a
	// CompositeIndexGetter either.
	var v any
	if err := plain.Scan(oid, pgtype.TextFormatCode, []byte(text), &v); err != nil {
		t.Fatalf("pgx cannot scan a composite's text form at all: %v", err)
	}
	if _, err := plain.Encode(oid, pgtype.BinaryFormatCode, v, nil); err == nil {
		t.Error("pgx's own composite scan now round-trips to a binary encode; compositeCodec may be redundant")
	}

	// The same three steps with compositeCodec, which is what a target
	// connection actually carries. All of them must work, or a composite column
	// cannot be copied.
	m := pgtype.NewMap()
	m.RegisterType(&pgtype.Type{Name: "money_amount", OID: oid,
		Codec: &compositeCodec{CompositeCodec: &pgtype.CompositeCodec{Fields: fields(m)}}})

	if _, err := m.Encode(oid, pgtype.BinaryFormatCode, text, nil); err == nil {
		t.Error("the direct binary encode succeeded, so this test is no longer exercising pgx's string fallback")
	}
	var got any
	if err := m.Scan(oid, pgtype.TextFormatCode, []byte(text), &got); err != nil {
		t.Fatalf("scanning %q in text format: %v", text, err)
	}
	if _, ok := got.(pgtype.CompositeIndexGetter); !ok {
		t.Fatalf("the text scan gave %T; pgx's fallback re-encodes what it scanned, and only a "+
			"pgtype.CompositeIndexGetter has a binary encode plan", got)
	}
	if _, err := m.Encode(oid, pgtype.BinaryFormatCode, got, nil); err != nil {
		t.Fatalf("re-encoding the scanned composite in binary: %v; pgx's "+
			"tryScanStringCopyValueThenEncode fallback (values.go) is what CopyFrom relies on here, "+
			"and a composite column cannot be loaded without it", err)
	}
}
