// SPDX-License-Identifier: Apache-2.0

package classify

import (
	"strings"
	"testing"

	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
)

// The unit half of testdata/regressions/008-name-hit-on-an-unaccepted-type-
// drops-the-type-signal.sql.
//
// That file is the reduction of a live leak (THREAT_MODEL.md T1): Supabase's
// `auth.users.raw_user_meta_data` is jsonb holding a person's name, address,
// email and phone, its *name* matches the free_text rule on its leading `raw_`,
// free_text does not accept jsonb — and `decide` recorded `free_text` at `low`
// and copied the column verbatim, while the jsonb column beside it with no name
// at all was masked. It runs behind `integration && torture` and needs Docker,
// twenty containers and an hour. This runs in milliseconds on every change,
// which is what a leak fix needs: the regression file says the defect happened,
// this says it is still fixed.
//
// Three columns, which are the three the regression file names: the leak, and
// the two controls that must not move with it.
func TestNameHitOnAnUnacceptedTypeKeepsTheTypeSignal(t *testing.T) {
	t.Parallel()

	tAccount := ref.TableRef{Schema: "public", Name: "reg8_account"}
	schema := &pipeline.Schema{
		Tables: []pipeline.Table{
			tt("public", "reg8_account", []string{"id"},
				tc("id", "integer"),
				tc("raw_user_meta_data", "jsonb"),
				tc("identity_data", "jsonb"),
				tc("email_verified", "boolean"),
			),
		},
		Fingerprint: "reg8",
	}

	// The documents in `raw_user_meta_data` are deliberately impersonal. A
	// provider profile carrying a name, an email address and a phone number —
	// which is what the real column holds — gives the validators a value signal,
	// and the value signal decides the column on its own at `likely`, masked.
	// That is a different branch of `decide` and it was never the defect. The
	// branch this test pins is the one the defect was in: a rejected name hit
	// and samples that say nothing, where before the fix the type was never
	// consulted and the column came out `low` and copied. A settings document is
	// how a jsonb column looks in the sample of a table whose personal rows are
	// past the sample window.
	settings := func(i string) string {
		return `{"theme": "dark", "rows_per_page": 2` + i + `, "locale": "en-GB", "sidebar": "pinned"}`
	}
	samples := mapSampler{
		col(tAccount, "id"):                 anyOf(int64(1), int64(2), int64(3), int64(4)),
		col(tAccount, "raw_user_meta_data"): anyOf(settings("1"), settings("2"), settings("3"), settings("4")),
		col(tAccount, "identity_data"): anyOf(
			`{"sub": "0f1e", "email": "anouk.bakhuis1@regression.test"}`,
			`{"sub": "1a2b", "email": "anouk.bakhuis2@regression.test"}`,
			`{"sub": "2b3c", "email": "anouk.bakhuis3@regression.test"}`,
			`{"sub": "3c4d", "email": "anouk.bakhuis4@regression.test"}`),
		col(tAccount, "email_verified"): anyOf(true, false, true, false),
	}

	cls, err := New().Classify(schema, samples, nil)
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}

	t.Run("TheRejectedNameHitFallsBackToTheType", func(t *testing.T) {
		d := decision(t, cls, col(tAccount, "raw_user_meta_data"))
		if !d.Masked {
			t.Fatalf("reg8_account.raw_user_meta_data is not masked, so it is copied verbatim: %+v", d)
		}
		if d.Category != pipeline.CatSemiStruct {
			t.Errorf("reg8_account.raw_user_meta_data category = %q, want %q (the type decides when the name is rejected)",
				d.Category, pipeline.CatSemiStruct)
		}
		if d.Confidence != pipeline.ConfPossible {
			t.Errorf("reg8_account.raw_user_meta_data confidence = %v, want possible — the same the column would have had with no name at all",
				d.Confidence)
		}
		if !strings.Contains(d.Reason, "jsonb") {
			t.Errorf("reg8_account.raw_user_meta_data reason = %q, want the type named", d.Reason)
		}
	})

	t.Run("TheUnnamedControlIsUnchanged", func(t *testing.T) {
		d := decision(t, cls, col(tAccount, "identity_data"))
		if !d.Masked || d.Category != pipeline.CatSemiStruct {
			t.Errorf("reg8_account.identity_data = %+v, want a masked semi_structured column", d)
		}
	})

	t.Run("ATypeWithNoSignalStillLandsAtLow", func(t *testing.T) {
		// The other control, and the reason this fix is narrow: boolean has no
		// entry in typeSignals, so there is no type evidence to fall back to and
		// ARCHITECTURE.md §4's "recorded at low and copied" is the whole answer.
		// testdata/README.md trap 19 is the same shape.
		d := decision(t, cls, col(tAccount, "email_verified"))
		if d.Masked {
			t.Errorf("reg8_account.email_verified is masked: %+v", d)
		}
		if d.Confidence != pipeline.ConfLow {
			t.Errorf("reg8_account.email_verified confidence = %v, want low", d.Confidence)
		}
	})
}
