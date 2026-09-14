// SPDX-License-Identifier: Apache-2.0

package event

import (
	"reflect"
	"testing"
	"time"
)

// This package's own half of "value-free types" (ARCHITECTURE.md §2, §7;
// THREAT_MODEL.md T4). The other three — pipeline.Config, pipeline.Plan and
// pipeline.Report — cannot be checked from here: this package must never
// import pipeline (root CLAUDE.md, internal/CLAUDE.md, TestImportGraph), and a
// test that could see those types would need to. What is checked here is the
// half this package owns outright: Event itself, and — the piece the
// 2026-09-09 review (finding 2) found missing — that Args, and so every
// ArgKey including ArgReason, can structurally carry nothing but a string
// (T-0131).
//
// This is a type-level guard, not a content-level one. It proves a future
// field on Event cannot silently start carrying a rich value-bearing type; it
// cannot prove that a caller never *puts* a source value into the string a
// reason arg already is — Args wouldn't refuse "poly.canary@example.org" any
// more than it would refuse "public.items". That half is enforced at the call
// sites (internal/plan's valueDigest, internal/classify's reason grammar) and
// proved end-to-end by cmd/lazyslice's canary test, which runs the whole
// binary over a fixture seeded with a unique canary per text column and
// greps every output sink for it.

// TestEventIsValueFree walks Event's field types (recursively through named
// struct types, maps and their element types) and fails on the first field
// whose underlying kind is not a plain string, a plain integer, a bool, or
// time.Time. ref.TableRef, the one struct type Event carries, passes because
// walking it resolves to two plain strings (Schema and Name) and nothing
// else.
func TestEventIsValueFree(t *testing.T) {
	walkType(t, reflect.TypeFor[Event](), "Event", map[reflect.Type]bool{})
}

func walkType(t *testing.T, typ reflect.Type, path string, seen map[reflect.Type]bool) {
	t.Helper()
	if typ == reflect.TypeFor[time.Time]() {
		return
	}
	if seen[typ] {
		// A recursive or repeated type: already walked, and walking it again
		// cannot find a new field.
		return
	}
	seen[typ] = true

	switch typ.Kind() {
	case reflect.String, reflect.Int, reflect.Int64, reflect.Bool:
		return
	case reflect.Struct:
		for i := 0; i < typ.NumField(); i++ {
			f := typ.Field(i)
			walkType(t, f.Type, path+"."+f.Name, seen)
		}
	case reflect.Map:
		walkType(t, typ.Key(), path+"[key]", seen)
		walkType(t, typ.Elem(), path+"[value]", seen)
	case reflect.Slice, reflect.Array, reflect.Pointer:
		walkType(t, typ.Elem(), path+"[elem]", seen)
	default:
		t.Errorf("%s is %s (%s), which is not on event's value-free allow-list "+
			"(string, a plain integer, bool, or time.Time): a value-bearing type "+
			"must never become reachable from event.Event (THREAT_MODEL.md T4)",
			path, typ, typ.Kind())
	}
}

// TestArgsCanOnlyCarryStrings pins Args' value type directly, so a change from
// `map[ArgKey]string` to anything richer — a struct, an interface, a []byte —
// fails here first rather than being caught only by the broader walk above.
// Every ArgKey shares this one value type, ArgReason included: there is no
// richer "reason" carrier for a future call site to reach for instead of a
// digest or an identifier (T-0131).
func TestArgsCanOnlyCarryStrings(t *testing.T) {
	typ := reflect.TypeFor[Args]()
	if typ.Kind() != reflect.Map {
		t.Fatalf("Args is a %s, want a map", typ.Kind())
	}
	if key := typ.Key(); key != reflect.TypeFor[ArgKey]() {
		t.Errorf("Args is keyed by %s, want ArgKey", key)
	}
	if elem := typ.Elem(); elem.Kind() != reflect.String {
		t.Errorf("Args carries %s, want a plain string — every ArgKey, including %q, "+
			"can hold nothing richer", elem, ArgReason)
	}
}
