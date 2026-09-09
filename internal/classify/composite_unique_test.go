// SPDX-License-Identifier: Apache-2.0

package classify

import (
	"testing"

	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
)

// The unit pin for `raiseCompositeUnique` and `sampleDistinct`.
//
// `Decision.UniqueIndex` is what `internal/plan`'s checkUniqueDomain refuses on
// and what `internal/transform` turns into `Constraints.Unique`, so which
// columns this raises decides both whether a run stops at plan and, when it does
// not, how wide a domain the masker draws from. Both directions cost something
// real and both were measured on the torture schemas: raising every key column
// refused runs that could not collide
// (testdata/regressions/003-composite-unique-index-is-not-a-unique-column.sql),
// raising none loaded rows that did
// (testdata/regressions/004-composite-unique-index-all-masked.sql). Those two
// files need Docker and `make torture`; the cases below are the same four
// shapes over a recorded schema, in milliseconds.
func TestRaiseCompositeUnique(t *testing.T) {
	t.Parallel()

	tRef := ref.TableRef{Schema: "public", Name: "content_type"}
	c := func(name string) ref.ColumnRef { return ref.ColumnRef{Table: tRef, Column: name} }

	// index builds one unique index over the named columns.
	index := func(name string, partial, expression bool, def string, cols ...string) pipeline.Index {
		return pipeline.Index{
			Name: name, Columns: cols, Unique: true,
			Partial: partial, Expression: expression, Immediate: true, Def: def,
		}
	}

	// build runs the classifier over one table carrying one composite unique
	// index. `full_name` is the masked key column of every case, `app_label` is
	// a masked-or-not label that repeats, and `blob_id` is the surrogate key.
	build := func(idx pipeline.Index, samples mapSampler) *pipeline.Classification {
		t.Helper()
		table := tt("public", "content_type", []string{"id"},
			tc("id", "integer"),
			tc("app_label", "text"),
			tc("model", "text"),
			tc("full_name", "text"),
			tc("blob_id", "bigint"),
		)
		table.Indexes = []pipeline.Index{idx}
		cls, err := New().Classify(&pipeline.Schema{
			Tables:      []pipeline.Table{table},
			Fingerprint: "composite-unique",
		}, samples, nil)
		if err != nil {
			t.Fatalf("Classify: %v", err)
		}
		return cls
	}

	// names is the masked column of every case: a person's name, which every
	// rule pack decides as person_name and the threshold masks.
	names := anyOf("Ada Lovelace", "Grace Hopper", "Alan Turing", "Katherine Johnson")
	repeating := anyOf("Ada Lovelace", "Ada Lovelace", "Ada Lovelace", "Ada Lovelace")

	t.Run("ADistinctUnmaskedKeyColumnSuppressesTheRaise", func(t *testing.T) {
		// Regression 003: `blob_id` is a surrogate key copied verbatim and it
		// holds every row apart on its own, whatever the masked columns become.
		cls := build(
			index("ct_key", false, false, "CREATE UNIQUE INDEX ct_key ON public.content_type USING btree (full_name, blob_id)",
				"full_name", "blob_id"),
			mapSampler{
				c("full_name"): names,
				c("blob_id"):   anyOf(int64(1), int64(2), int64(3), int64(4)),
			})
		if d := decision(t, cls, c("full_name")); d.UniqueIndex {
			t.Errorf("content_type.full_name raised UniqueIndex; blob_id is distinct and unmasked, so the tuple cannot collide: %+v", d)
		}
	})

	t.Run("AnAllNullUnmaskedKeyColumnStillSuppressesTheRaise", func(t *testing.T) {
		// calcom's `Role_name_teamId_key ON public."Role" (name, "teamId")`
		// with `teamId` NULL in all three rows. A NULL key component is equal to
		// nothing under a plain unique index, so the tuple cannot collide
		// however `name` is masked — and the table was sampled, which is what
		// says the absence is a NULL rather than an unreadable relation.
		cls := build(
			index("ct_key", false, false, "CREATE UNIQUE INDEX ct_key ON public.content_type USING btree (full_name, blob_id)",
				"full_name", "blob_id"),
			mapSampler{c("full_name"): names})
		if d := decision(t, cls, c("full_name")); d.UniqueIndex {
			t.Errorf("content_type.full_name raised UniqueIndex, and blob_id is NULL in every sampled row: %+v", d)
		}
	})

	t.Run("ATableNothingWasSampledFromRaisesItsMaskedKeyColumns", func(t *testing.T) {
		// The fail-open this used to be, and the case the one above is told
		// apart from: introspect tolerates a relation the role may not read, and
		// a partitioned table with no leaves has nothing to read. With no sample
		// anywhere in the table, no key column may be counted as holding the
		// tuple apart.
		cls := build(
			index("ct_key", false, false, "CREATE UNIQUE INDEX ct_key ON public.content_type USING btree (full_name, blob_id)",
				"full_name", "blob_id"),
			mapSampler{})
		if d := decision(t, cls, c("full_name")); !d.UniqueIndex {
			t.Errorf("content_type.full_name did not raise UniqueIndex, and nothing in its table was sampled at all: %+v", d)
		}
	})

	t.Run("NullsNotDistinctWithdrawsTheNullArgument", func(t *testing.T) {
		// PostgreSQL 15's `NULLS NOT DISTINCT`: a NULL does collide with a NULL,
		// so an all-NULL key column holds nothing apart and the masked columns
		// carry the tuple on their own.
		cls := build(
			index("ct_key", false, false,
				"CREATE UNIQUE INDEX ct_key ON public.content_type USING btree (full_name, blob_id) NULLS NOT DISTINCT",
				"full_name", "blob_id"),
			mapSampler{c("full_name"): names})
		if d := decision(t, cls, c("full_name")); !d.UniqueIndex {
			t.Errorf("content_type.full_name did not raise UniqueIndex under a NULLS NOT DISTINCT index whose other key column is all NULL: %+v", d)
		}
	})

	t.Run("EveryMaskedKeyColumnIsRaisedWhenNothingIsDistinct", func(t *testing.T) {
		// Regression 004: Django's django_content_type is UNIQUE (app_label,
		// model) with both columns masked and neither distinct, and the load
		// died on the constraint. Nothing in the sample says which column
		// carries the uniqueness, so each is held to d_required.
		cls := build(
			index("ct_key", false, false, "CREATE UNIQUE INDEX ct_key ON public.content_type USING btree (app_label, full_name)",
				"app_label", "full_name"),
			mapSampler{
				c("app_label"): anyOf("auth", "auth", "auth", "auth"),
				c("full_name"): repeating,
			})
		if d := decision(t, cls, c("full_name")); !d.UniqueIndex {
			t.Errorf("content_type.full_name did not raise UniqueIndex, and no key column of the index is distinct: %+v", d)
		}
	})

	t.Run("APartialCompositeIsJudgedToo", func(t *testing.T) {
		// calcom's `Watchlist_type_value_global_key ON public."Watchlist" (type,
		// value) WHERE "organizationId" IS NULL` is this shape and `value` is
		// masked. Skipping partial composites raised the field on no column at
		// all, which is strictly less than the code this replaced did.
		cls := build(
			index("ct_key", true, false,
				"CREATE UNIQUE INDEX ct_key ON public.content_type USING btree (app_label, full_name) WHERE (blob_id IS NULL)",
				"app_label", "full_name"),
			mapSampler{
				c("app_label"): anyOf("auth", "auth", "auth", "auth"),
				c("full_name"): repeating,
			})
		if d := decision(t, cls, c("full_name")); !d.UniqueIndex {
			t.Errorf("content_type.full_name did not raise UniqueIndex under a partial composite unique index: %+v", d)
		}
	})

	t.Run("AnExpressionCompositeIsJudgedToo", func(t *testing.T) {
		// introspect leaves Index.Columns empty for an expression index, so the
		// key columns come out of the definition; reading Columns alone could
		// never match a multi-column expression index at all.
		cls := build(
			index("ct_key", false, true,
				"CREATE UNIQUE INDEX ct_key ON public.content_type USING btree (lower(app_label), lower(full_name))"),
			mapSampler{
				c("app_label"): anyOf("auth", "auth", "auth", "auth"),
				c("full_name"): repeating,
			})
		if d := decision(t, cls, c("full_name")); !d.UniqueIndex {
			t.Errorf("content_type.full_name did not raise UniqueIndex under a composite expression unique index: %+v", d)
		}
	})
}

// TestSampleDistinct pins the two answers apart, because raiseCompositeUnique
// reads them in opposite directions: no samples is the fail-closed "distinct"
// for a masked key column and the fail-closed "not held apart" for an unmasked
// one, and one boolean cannot say both.
func TestSampleDistinct(t *testing.T) {
	t.Parallel()

	tRef := ref.TableRef{Schema: "public", Name: "t"}
	c := func(name string) ref.ColumnRef { return ref.ColumnRef{Table: tRef, Column: name} }
	st := &state{sampler: mapSampler{
		c("distinct"):  anyOf("a", "b", "c"),
		c("repeating"): anyOf("a", "a", "b"),
		c("empty"):     anyOf(),
	}}

	for _, tc := range []struct {
		col               string
		distinct, sampled bool
	}{
		{"distinct", true, true},
		{"repeating", false, true},
		{"empty", true, false},
		{"absent", true, false},
	} {
		distinct, sampled := st.sampleDistinct(c(tc.col))
		if distinct != tc.distinct || sampled != tc.sampled {
			t.Errorf("sampleDistinct(%s) = (%v, %v), want (%v, %v)",
				tc.col, distinct, sampled, tc.distinct, tc.sampled)
		}
	}
}
