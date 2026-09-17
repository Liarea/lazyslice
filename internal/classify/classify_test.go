// SPDX-License-Identifier: Apache-2.0

package classify

import (
	"sort"
	"strings"
	"testing"

	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
	"github.com/Liarea/lazyslice/mask"
)

func mustClassify(t *testing.T, prior *pipeline.Config) *pipeline.Classification {
	t.Helper()
	cls, err := classifyNasty(prior)
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	return cls
}

func decision(t *testing.T, cls *pipeline.Classification, c ref.ColumnRef) pipeline.Decision {
	t.Helper()
	d, ok := cls.Decisions[c]
	if !ok {
		t.Fatalf("no decision for %s; the classifier must decide every column", c)
	}
	return d
}

// TestNastyTraps is the classifier's half of testdata/README.md's traps: the
// false positive that must not be masked, the false negative that must be, and
// the three shapes that are not scalar text.
func TestNastyTraps(t *testing.T) {
	t.Parallel()
	cls := mustClassify(t, nil)

	t.Run("EmailVerifiedIsNotFlagged", func(t *testing.T) {
		// testdata/README.md trap 19. The name matches every email rule anyone
		// would write; the type says it cannot be an address. ARCHITECTURE.md §4
		// records it at low with a reason naming the conflict, and the
		// neighbouring-column rule may not raise it even though people carries
		// several columns at likely or above.
		d := decision(t, cls, col(tPeople, "email_verified"))
		if d.Masked {
			t.Errorf("people.email_verified is masked: %+v", d)
		}
		if d.Confidence != pipeline.ConfLow {
			t.Errorf("people.email_verified confidence = %v, want low", d.Confidence)
		}
		if d.Masker != "" {
			t.Errorf("people.email_verified carries masker %q; an unmasked column has none", d.Masker)
		}
		if !strings.Contains(d.Reason, "boolean is not an accepted type for email") {
			t.Errorf("people.email_verified reason = %q, want the type conflict named", d.Reason)
		}
	})

	t.Run("RefIsFlaggedByItsValues", func(t *testing.T) {
		// testdata/README.md trap 20. The name says nothing; only the samples do.
		d := decision(t, cls, col(tPeople, "ref"))
		if !d.Masked {
			t.Fatalf("people.ref is not masked: %+v", d)
		}
		if d.Category != pipeline.CatEmail {
			t.Errorf("people.ref category = %q, want email", d.Category)
		}
		if d.Confidence != pipeline.ConfLikely {
			t.Errorf("people.ref confidence = %v, want likely (values only)", d.Confidence)
		}
		if !strings.Contains(d.Reason, "no name signal") {
			t.Errorf("people.ref reason = %q, want it to say the values decided", d.Reason)
		}
		if !strings.Contains(d.Reason, "samples parse as addresses") {
			t.Errorf("people.ref reason = %q, want the validator named", d.Reason)
		}
	})

	t.Run("JSONBIsFlagged", func(t *testing.T) {
		// testdata/README.md traps 16a and 16b.
		contact := decision(t, cls, col(tPeople, "contact"))
		if !contact.Masked || contact.Category != pipeline.CatSemiStruct {
			t.Errorf("people.contact = %+v, want a masked semi_structured column", contact)
		}
		payload := decision(t, cls, col(tEvents, "payload"))
		if !payload.Masked || payload.Category != pipeline.CatSemiStruct {
			t.Errorf("events.payload = %+v, want a masked semi_structured column", payload)
		}
		if !strings.Contains(payload.Reason, "log-shaped table") {
			t.Errorf("events.payload reason = %q, want the log-shaped rule named (trap 16b)", payload.Reason)
		}
		if strings.Contains(contact.Reason, "log-shaped table") {
			t.Errorf("people.contact reason = %q; people is not a log-shaped table", contact.Reason)
		}
	})

	t.Run("NotesAreFlaggedAsFreeText", func(t *testing.T) {
		// testdata/README.md trap 17.
		d := decision(t, cls, col(tPeople, "notes"))
		if !d.Masked || d.Category != pipeline.CatFreeText {
			t.Errorf("people.notes = %+v, want a masked free_text column", d)
		}
		legacy := decision(t, cls, col(tLegacy, "Notes"))
		if !legacy.Masked || legacy.Category != pipeline.CatFreeText {
			t.Errorf(`LegacyCustomer."Notes" = %+v, want a masked free_text column`, legacy)
		}
	})

	t.Run("TextArrayIsFlaggedOnItsElementType", func(t *testing.T) {
		// testdata/README.md trap 15.
		d := decision(t, cls, col(tPeople, "alt_emails"))
		if !d.Masked || d.Category != pipeline.CatEmail {
			t.Errorf("people.alt_emails = %+v, want a masked email column", d)
		}
		if !strings.Contains(d.Reason, "classified on the element type text") {
			t.Errorf("people.alt_emails reason = %q, want the element type named", d.Reason)
		}
	})
}

// TestNastyOtherColumns covers the decisions the traps above depend on being
// right for a different reason: the two enums, the type signals, the generated
// column and the surrogate keys.
func TestNastyOtherColumns(t *testing.T) {
	t.Parallel()
	cls := mustClassify(t, nil)

	t.Run("AnEnumNothingFlagsIsCopied", func(t *testing.T) {
		d := decision(t, cls, col(tPeople, "status")) // trap 13
		if d.Masked {
			t.Errorf("people.status is masked: %+v; no name and no value signal hits it", d)
		}
	})

	t.Run("AnEnumASpecialCategoryFlagsIsMasked", func(t *testing.T) {
		d := decision(t, cls, col(tPeople, "marital_status")) // trap 24
		if !d.Masked || d.Category != pipeline.CatSpecial {
			t.Errorf("people.marital_status = %+v, want a masked special_category column", d)
		}
		if d.Confidence != pipeline.ConfCertain {
			t.Errorf("people.marital_status confidence = %v, want certain by name alone", d.Confidence)
		}
	})

	t.Run("TypeSignalsAreThreeSeparateThings", func(t *testing.T) {
		// testdata/README.md trap 18: the type alone is likely, the name and the
		// type agreeing is certain, and a run that scores them alike has
		// collapsed two signals into one.
		origin := decision(t, cls, col(tSessions, "origin"))
		if origin.Category != pipeline.CatNetworkID || origin.Confidence != pipeline.ConfLikely {
			t.Errorf("tenant_user_sessions.origin = %+v, want network_id at likely", origin)
		}
		adapter := decision(t, cls, col(tSessions, "adapter"))
		if adapter.Category != pipeline.CatNetworkID || !adapter.Masked {
			t.Errorf("tenant_user_sessions.adapter = %+v, want a masked network_id column", adapter)
		}
		clientIP := decision(t, cls, col(tAudit, "client_ip"))
		if clientIP.Category != pipeline.CatNetworkID || clientIP.Confidence != pipeline.ConfCertain {
			t.Errorf("audit_log.client_ip = %+v, want network_id at certain", clientIP)
		}
	})

	t.Run("GeneratedAndSurrogateKeysAreNeverMasked", func(t *testing.T) {
		for _, tc := range []struct {
			c    ref.ColumnRef
			says string
		}{
			{col(tPeople, "display_name"), "generated column"},
			{col(tPeople, "person_id"), "surrogate key"},
			{col(tPeople, "manager_id"), "foreign key to public.people"},
			{col(tOrders, "person_id"), "foreign key to public.people"},
		} {
			d := decision(t, cls, tc.c)
			if d.Masked {
				t.Errorf("%s is masked: %+v", tc.c, d)
			}
			if !strings.Contains(d.Reason, tc.says) {
				t.Errorf("%s reason = %q, want it to explain %q", tc.c, d.Reason, tc.says)
			}
		}
	})

	t.Run("APersonalKeyIsStillMasked", func(t *testing.T) {
		// devices.device_id is a uuid primary key whose name matches online_id,
		// and device_readings.device_id references it. ARCHITECTURE.md §4's
		// exemption is for surrogate keys; a key the name rules place in a
		// category is not one, and copying it verbatim would be an identifier
		// for a person's device in the target in cleartext.
		for _, c := range []ref.ColumnRef{col(tDevices, "device_id"), col(tReadings, "device_id")} {
			d := decision(t, cls, c)
			if !d.Masked || d.Category != pipeline.CatOnlineID {
				t.Errorf("%s = %+v, want a masked online_id column", c, d)
			}
			if strings.Contains(d.Reason, "preserved verbatim") {
				t.Errorf("%s reason = %q; a key with a category is not a surrogate key", c, d.Reason)
			}
		}
	})

	t.Run("TwoValuesBelowMinSamplesStillDecide", func(t *testing.T) {
		// devices.owned_by is two email addresses and a NULL in a three-row
		// table, under a name no rule matches (fixture_test.go). Returning
		// before the validators ran because the column holds fewer than
		// minSamples non-NULL values decided it `none`, internal/transform
		// copied it, and a production email address reached the target in
		// cleartext under exit 0 (THREAT_MODEL.md T1, tracker T-0058).
		// TestI2NothingFlaggedSurvives/nasty is what found that, and it is
		// behind a build tag and a Docker daemon; this is the same claim in
		// `make test`, so restoring the early return fails here first.
		d := decision(t, cls, col(tDevices, "owned_by"))
		if !d.Masked || d.Category != pipeline.CatEmail {
			t.Fatalf("devices.owned_by = %+v, want a masked email column; below minSamples a "+
				"strong ratio still decides", d)
		}
		if !strings.Contains(d.Reason, "2/2 samples parse as addresses") {
			t.Errorf("devices.owned_by reason = %q, want the two-of-two ratio named", d.Reason)
		}
		if !strings.Contains(d.Reason, "no name signal") {
			t.Errorf("devices.owned_by reason = %q, want it to say the values decided", d.Reason)
		}
	})

	t.Run("EveryMaskedColumnHasAMasker", func(t *testing.T) {
		// Over the prior-bearing classifications as well as the plain one:
		// Masker is chosen from the category, and a yml raise is the one input
		// that can move a category without a rule behind it, so the invariant
		// has to hold on that path or transform is handed "mask this column"
		// with nothing to mask it with (ADR-006).
		for _, prior := range priorsUnderTest(t) {
			for c, d := range mustClassify(t, prior).Decisions {
				switch {
				case d.Masked && d.Masker == "":
					t.Errorf("%s is masked with no masker: %+v", c, d)
				case d.Masked && d.Category == pipeline.CatNone:
					t.Errorf("%s is masked as the no-signal category: %+v", c, d)
				case !d.Masked && d.Masker != "":
					t.Errorf("%s is not masked but names masker %q", c, d.Masker)
				}
			}
		}
	})

	t.Run("SamplesFromAPartitionAreNamed", func(t *testing.T) {
		d := decision(t, cls, col(tEvents, "payload"))
		if !strings.Contains(d.Reason, "samples from partition public.events_2024") {
			t.Errorf("events.payload reason = %q, want the leaf named (ARCHITECTURE.md §4)", d.Reason)
		}
	})

	t.Run("TheUniqueIndexIsRecorded", func(t *testing.T) {
		d := decision(t, cls, col(tLegacy, "EmailAddress"))
		if !d.UniqueIndex {
			t.Errorf(`LegacyCustomer."EmailAddress" = %+v, want UniqueIndex set`, d)
		}
		if !d.Masked || d.Category != pipeline.CatEmail {
			t.Errorf(`LegacyCustomer."EmailAddress" = %+v, want a masked email column`, d)
		}
		partial := decision(t, cls, col(tAudit, "action"))
		if !partial.UniqueIndex {
			t.Errorf("audit_log.action = %+v, want UniqueIndex set by the partial index", partial)
		}
	})

	t.Run("AQuotedCamelCaseNameStillMatches", func(t *testing.T) {
		for _, c := range []ref.ColumnRef{
			col(tLegacy, "EmailAddress"),
			col(tLegacy, "ContactNumber"),
			col(tLegacy, "MobileNumber"),
		} {
			if d := decision(t, cls, c); !d.Masked {
				t.Errorf("%s = %+v, want it masked; the name rules read EmailAddress as email_address", c, d)
			}
		}
	})
}

// TestNeighbouringColumnRule is the recall bias ARCHITECTURE.md §4 and
// THREAT_MODEL.md T1 both name, and the one exception to it: a column at low
// beside a column at likely is raised to possible and masked, unless its name
// hit was on a type its category does not accept.
func TestNeighbouringColumnRule(t *testing.T) {
	t.Parallel()
	member := ref.TableRef{Schema: "public", Name: "member"}
	schema := &pipeline.Schema{
		Tables: []pipeline.Table{
			tt("public", "member", nil,
				tc("email", "text"),         // certain: the name and the values agree
				tc("contact_point", "text"), // low: half the samples are address-shaped
				tc("nickname", "boolean"),   // low: a name hit on a type online_id refuses
			),
		},
	}
	samples := mapSampler{
		ref.ColumnRef{Table: member, Column: "email"}: anyOf(
			"a@fixture.test", "b@fixture.test", "c@fixture.test", "d@fixture.test"),
		// address is not a strong validator (validators.go), so this stays a
		// weak signal at `low` for the neighbouring-column rule to raise,
		// unlike a strong validator's hit below threshold (sig.strongHit),
		// which decides free_text on its own (finding 7).
		ref.ColumnRef{Table: member, Column: "contact_point"}: anyOf(
			"42 Cedar Street", "17 Birch Lane", "unknown", "n/a"),
		ref.ColumnRef{Table: member, Column: "nickname"}: anyOf(true, false, true, false),
	}
	cls, err := New().Classify(schema, samples, nil)
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}

	raised := cls.Decisions[ref.ColumnRef{Table: member, Column: "contact_point"}]
	if !raised.Masked {
		t.Errorf("member.contact_point = %+v, want it raised to possible and masked", raised)
	}
	if raised.Source != pipeline.ByNeighbour {
		t.Errorf("member.contact_point source = %v, want ByNeighbour", raised.Source)
	}
	if !strings.Contains(raised.Reason, "raised by the neighbouring-column rule") {
		t.Errorf("member.contact_point reason = %q, want the rule named", raised.Reason)
	}

	if d := cls.Decisions[ref.ColumnRef{Table: member, Column: "nickname"}]; d.Masked {
		t.Errorf("member.nickname = %+v; a type-conflicting name hit is never raised above low", d)
	}
}

// TestGuessedRegionPhoneCorroboration is T-0221's own gate: with no
// --phone-region configured, a character column whose values clear one of
// phoneGuessRegions is masked as phone only beside a proven personal
// neighbour, and is left alone with none. The values are real ones
// (verified by direct computation against textsig.ValidPhoneRegion, the way
// testdata/regressions/026's own header explains) and not phone-shaped by
// construction, on purpose: this is exactly the "any region agrees" claim
// the corroboration gate exists to distrust on its own.
func TestGuessedRegionPhoneCorroboration(t *testing.T) {
	t.Parallel()
	guessed := []any{"9231278675", "7543856411", "5101878760", "5526624009", "9743547657"}

	corroborated := ref.TableRef{Schema: "public", Name: "corroborated_orders"}
	uncorroborated := ref.TableRef{Schema: "public", Name: "uncorroborated_orders"}
	schema := &pipeline.Schema{
		Tables: []pipeline.Table{
			tt("public", "corroborated_orders", nil,
				tc("hotline_ref", "text"), // no name rule matches this
				tc("email", "text"),       // certain: the neighbour signal
			),
			tt("public", "uncorroborated_orders", nil,
				// A different column name from corroborated_orders' on
				// purpose: sameColumnName (pass 4) shares one category
				// across every column sharing a name anywhere in the
				// schema, which would otherwise smuggle the first table's
				// corroborated decision onto this one and prove nothing
				// about the gate this test exists to check.
				tc("order_ref", "text"), // identical values, no neighbour at all
			),
		},
	}
	samples := mapSampler{
		ref.ColumnRef{Table: corroborated, Column: "hotline_ref"}: guessed,
		ref.ColumnRef{Table: uncorroborated, Column: "order_ref"}: guessed,
		ref.ColumnRef{Table: corroborated, Column: "email"}: anyOf(
			"a@fixture.test", "b@fixture.test", "c@fixture.test", "d@fixture.test"),
	}
	cls, err := New().Classify(schema, samples, nil)
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}

	withNeighbour := cls.Decisions[ref.ColumnRef{Table: corroborated, Column: "hotline_ref"}]
	if !withNeighbour.Masked || withNeighbour.Category != pipeline.CatPhone {
		t.Errorf("corroborated_orders.hotline_ref = %+v, want masked as phone beside its email neighbour",
			withNeighbour)
	}
	if bad, ok := ParseReason(withNeighbour.Reason); !ok {
		t.Errorf("corroborated_orders.hotline_ref reason %q holds a fragment no template produced: %q",
			withNeighbour.Reason, bad)
	}
	if !strings.Contains(withNeighbour.Reason, "guessed-region") {
		t.Errorf("corroborated_orders.hotline_ref reason = %q, want the guessed-region corroboration named",
			withNeighbour.Reason)
	}

	withoutNeighbour := cls.Decisions[ref.ColumnRef{Table: uncorroborated, Column: "order_ref"}]
	if withoutNeighbour.Masked {
		t.Errorf("uncorroborated_orders.order_ref = %+v, want left unmasked: no name and no neighbour "+
			"corroborate the guessed-region hit", withoutNeighbour)
	}
}

// TestConfiguredPhoneRegionMasksNationalFormatColumn is the T-0221 review
// round's finding 3, first half: the configured-region row path
// (buildValidators' spliced-in strong entry, and decide's own
// "phone region assumed" reason fragment) was pinned only by
// testdata/regressions/025 under `make torture`, which is Docker-gated and
// does not run in `make check` -- nothing in this package's own suite ever
// classified a column with prior.PhoneRegion set at all, so a stub
// buildValidators that returned baseValidators unchanged would still pass
// `go test ./internal/classify/...`.
//
// The column name ("kontaktnr") and the values are testdata/regressions/025's
// own, real UK numbers written plainly with spaces and brackets, chosen
// there because rules.yml's phone pattern does not match the name: the mask
// has to come from the configured-region value entry alone, with no name
// signal and no personal neighbour to lean on.
func TestConfiguredPhoneRegionMasksNationalFormatColumn(t *testing.T) {
	t.Parallel()
	tbl := ref.TableRef{Schema: "public", Name: "reg025_tickets"}
	schema := &pipeline.Schema{
		Tables: []pipeline.Table{
			tt("public", "reg025_tickets", nil,
				tc("kontaktnr", "text"),
			),
		},
	}
	samples := mapSampler{
		ref.ColumnRef{Table: tbl, Column: "kontaktnr"}: anyOf(
			"07911 123456", "020 7946 0958", "(0161) 496 0123", "07911 123456"),
	}
	prior := &pipeline.Config{PhoneRegion: "GB"}
	cls, err := New().Classify(schema, samples, prior)
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}

	d := cls.Decisions[ref.ColumnRef{Table: tbl, Column: "kontaktnr"}]
	if !d.Masked || d.Category != pipeline.CatPhone {
		t.Fatalf("reg025_tickets.kontaktnr = %+v, want masked as phone under the configured region", d)
	}
	if bad, ok := ParseReason(d.Reason); !ok {
		t.Errorf("reg025_tickets.kontaktnr reason %q holds a fragment no template produced: %q", d.Reason, bad)
	}
	if !strings.Contains(d.Reason, "phone region assumed: GB") {
		t.Errorf("reg025_tickets.kontaktnr reason = %q, want the configured region named", d.Reason)
	}
}

// TestNameMatchedPhoneColumnMasksOnNameAlone is the T-0221 review round's
// finding 3, second half, corrected: the review proved that
// guessedPhoneColumns' gate never had a reachable name-match arm at all --
// this suite's original TestGuessedRegionPhoneCorroborationByNameAlone
// passed, but only because decide's ordinary hasName && nameAccepted branch
// (not guessedPhoneColumns' corroboration) already masks a phone-pattern
// name at ConfPossible with no value signal required, and rules.yml's phone
// entry accepts every character family the guessed-region feature runs on,
// so a name match always reaches that branch before guessedPhoneColumns
// (which only ever sees a column still below ConfPossible) can see it.
// work.nameMatchedPhone and the gate's "!w.nameMatchedPhone &&" condition
// were therefore dead code and have been removed; guessedPhoneColumns'
// only corroboration is now Decision.TableHasLikelyPersonalColumn, exercised
// by TestGuessedRegionPhoneCorroboration above.
//
// This test is kept, renamed, to pin the branch that does mask this column:
// the table holds one column and nothing else, so there is no personal
// neighbour, and the only signal is the column's own name matching
// rules.yml's phone pattern ("mobile"). The values are the same
// guessed-region shapes TestGuessedRegionPhoneCorroboration already
// verified (by direct computation against textsig.ValidPhoneRegion) clear
// one of phoneGuessRegions, though decide's name-match branch does not need
// them to: it masks on the name alone.
func TestNameMatchedPhoneColumnMasksOnNameAlone(t *testing.T) {
	t.Parallel()
	guessed := []any{"9231278675", "7543856411", "5101878760", "5526624009", "9743547657"}

	tbl := ref.TableRef{Schema: "public", Name: "reg_contacts_by_name"}
	schema := &pipeline.Schema{
		Tables: []pipeline.Table{
			tt("public", "reg_contacts_by_name", nil,
				tc("mobile", "text"), // matches rules.yml's phone pattern; no other column
			),
		},
	}
	samples := mapSampler{
		ref.ColumnRef{Table: tbl, Column: "mobile"}: guessed,
	}
	cls, err := New().Classify(schema, samples, nil)
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}

	d := cls.Decisions[ref.ColumnRef{Table: tbl, Column: "mobile"}]
	if !d.Masked || d.Category != pipeline.CatPhone {
		t.Errorf("reg_contacts_by_name.mobile = %+v, want masked as phone on the name match alone, "+
			"with no personal neighbour in the table", d)
	}
	if bad, ok := ParseReason(d.Reason); !ok {
		t.Errorf("reg_contacts_by_name.mobile reason %q holds a fragment no template produced: %q", d.Reason, bad)
	}
}

// TestConfigCannotLowerConfidence is ADR-004's tighten-only rule and
// THREAT_MODEL.md T3's control: the yml supplies opt-outs and raises, never a
// way to reduce a category or a confidence.
func TestConfigCannotLowerConfidence(t *testing.T) {
	t.Parallel()
	base := mustClassify(t, nil)
	refCol := col(tPeople, "ref")
	notes := col(tPeople, "notes")

	prior := &pipeline.Config{
		ExtraPatterns: []pipeline.Pattern{
			{Name: `\Aref\z`, Category: pipeline.CatFreeText, Confidence: pipeline.ConfLow},
		},
		Columns: map[ref.ColumnRef]pipeline.ColumnConfig{
			notes: {Category: pipeline.CatNone, Confidence: pipeline.ConfNone},
		},
	}
	got := mustClassify(t, prior)

	if a, b := base.Decisions[refCol], got.Decisions[refCol]; b.Confidence < a.Confidence || b.Category != a.Category {
		t.Errorf("a lower-confidence pattern changed %s from %v/%v to %v/%v", refCol,
			a.Category, a.Confidence, b.Category, b.Confidence)
	}
	if a, b := base.Decisions[notes], got.Decisions[notes]; b.Confidence < a.Confidence || !b.Masked {
		t.Errorf("a lower-confidence column entry changed %s from %v/%v to %v/%v (masked=%t)", notes,
			a.Category, a.Confidence, b.Category, b.Confidence, b.Masked)
	}
}

// TestAnOptOutExpiresWithTheType is ARCHITECTURE.md §10's type fingerprint: an
// opt-out on a column that became something else is ignored, and the column is
// masked again rather than staying exempt.
func TestAnOptOutExpiresWithTheType(t *testing.T) {
	t.Parallel()
	notes := col(tPeople, "notes")
	live := mustClassify(t, nil).Decisions[notes].TypeFP

	current := &pipeline.Config{Columns: map[ref.ColumnRef]pipeline.ColumnConfig{
		notes: {Unmask: &pipeline.Unmask{Reason: "product text", By: "sam", TypeFP: live}},
	}}
	if d := mustClassify(t, current).Decisions[notes]; d.Masked {
		t.Errorf("%s = %+v, want a live opt-out honoured", notes, d)
	}

	stale := &pipeline.Config{Columns: map[ref.ColumnRef]pipeline.ColumnConfig{
		notes: {Unmask: &pipeline.Unmask{Reason: "product text", By: "sam", TypeFP: "deadbeef"}},
	}}
	cls := mustClassify(t, stale)
	if d := cls.Decisions[notes]; !d.Masked {
		t.Errorf("%s = %+v, want a stale opt-out ignored", notes, d)
	}
	if len(cls.Expired) != 1 || cls.Expired[0] != notes {
		t.Errorf("Expired = %v, want exactly %s", cls.Expired, notes)
	}
}

// TestDriftIsEveryColumnTheYmlHasNotSeen is ADR-004 "Drift": a column absent
// from the committed file is classified fresh and reported, which is what
// --strict-schema turns into exit 10.
func TestDriftIsEveryColumnTheYmlHasNotSeen(t *testing.T) {
	t.Parallel()
	notes := col(tPeople, "notes")
	prior := &pipeline.Config{Columns: map[ref.ColumnRef]pipeline.ColumnConfig{notes: {}}}
	cls := mustClassify(t, prior)
	if len(cls.Drift) == 0 {
		t.Fatal("Drift is empty; every column but one is new to this file")
	}
	for _, c := range cls.Drift {
		if c == notes {
			t.Errorf("%s is reported as drift, but the file carries it", c)
		}
	}
	for i := 1; i < len(cls.Drift); i++ {
		if !cls.Drift[i-1].Less(cls.Drift[i]) {
			t.Fatalf("Drift is not in (schema, table, column) order at %d: %v", i, cls.Drift)
		}
	}
}

// TestClassificationIsDeterministic is what ADR-004 rests on: two runs over one
// snapshot classify identically, down to the fingerprint.
func TestClassificationIsDeterministic(t *testing.T) {
	t.Parallel()
	a, b := mustClassify(t, nil), mustClassify(t, nil)
	if a.Fingerprint != b.Fingerprint {
		t.Errorf("fingerprints differ across runs: %q and %q", a.Fingerprint, b.Fingerprint)
	}
	if len(a.Fingerprint) != 16 {
		t.Errorf("fingerprint = %q, want 16 hex characters (ARCHITECTURE.md §5)", a.Fingerprint)
	}
	for c, d := range a.Decisions {
		if b.Decisions[c] != d {
			t.Errorf("%s differs between runs: %+v and %+v", c, d, b.Decisions[c])
		}
	}
}

// TestReasonGrammar is ARCHITECTURE.md §2 "Value-free types": every reason the
// classifier can emit parses back against the template set in reasons.go, so no
// sample value can reach the yml, an event or a rendered line through a reason.
func TestReasonGrammar(t *testing.T) {
	t.Parallel()
	for _, prior := range priorsUnderTest(t) {
		cls := mustClassify(t, prior)
		for c, d := range cls.Decisions {
			if bad, ok := ParseReason(d.Reason); !ok {
				t.Errorf("%s reason %q holds a fragment no template produced: %q", c, d.Reason, bad)
			}
		}
	}
}

// TestReasonGrammarRejectsProse is the other half: a string the templates did
// not produce must not parse, or the test above would prove nothing.
func TestReasonGrammarRejectsProse(t *testing.T) {
	t.Parallel()
	for _, s := range []string{
		"",
		"name matches email; 200/200 samples parse as ada.lovelace@fixture.test",
		"values look like ada.lovelace@fixture.test",
		"name matches email address of the customer",
		"no name or value signal at all",
		"opt-out recorded in lazyslice.yml: product catalogue text",
	} {
		if _, ok := ParseReason(s); ok {
			t.Errorf("ParseReason(%q) accepted a string no template produced", s)
		}
	}
}

// TestReasonGrammarCoversPhoneRegion pins T-0221's two new fragments directly,
// because neither is ever produced by the fixtures priorsUnderTest classifies
// (none sets Config.PhoneRegion or holds a value that clears a guessed
// region), so TestReasonGrammar above never exercises regionAssumed's or
// guessedPhoneColumns' own render call — only the regression suite
// (testdata/regressions/025 and 026, under make torture) does, end to end.
func TestReasonGrammarCoversPhoneRegion(t *testing.T) {
	t.Parallel()
	for _, s := range []string{
		render("phone_region_configured", "GB"),
		render("phone_region_guessed"),
	} {
		if bad, ok := ParseReason(s); !ok {
			t.Errorf("reason %q does not parse against its own template (%q)", s, bad)
		}
	}
}

// TestEveryCategoryHasAMasker walks the rule pack against pipeline's category
// list. CatNone is excluded by name rather than by a wildcard: it is what a
// copied column carries and it reaches no masker (ADR-006 "Consequences").
func TestEveryCategoryHasAMasker(t *testing.T) {
	t.Parallel()
	p, err := pack()
	if err != nil {
		t.Fatalf("rule pack: %v", err)
	}
	all := []pipeline.Category{
		pipeline.CatPersonName, pipeline.CatEmail, pipeline.CatPhone, pipeline.CatAddress,
		pipeline.CatGeo, pipeline.CatPersonDate, pipeline.CatNationalID, pipeline.CatFinancial,
		pipeline.CatNetworkID, pipeline.CatOnlineID, pipeline.CatCredential, pipeline.CatFreeText,
		pipeline.CatSpecial, pipeline.CatBinary, pipeline.CatSemiStruct,
		pipeline.CatDerivedText,
	}
	for _, cat := range all {
		if p.Masker[cat] == "" {
			t.Errorf("category %q has no masker in the rule pack", cat)
		}
	}
	if _, ok := p.Masker[pipeline.CatNone]; ok {
		t.Errorf("the rule pack declares a masker for %q", pipeline.CatNone)
	}
	if len(p.Masker) != len(all) {
		t.Errorf("the rule pack declares %d categories, pipeline declares %d", len(p.Masker), len(all))
	}
}

// TestEveryRulePackMaskerResolves walks every masker id rules.yml names
// against the mask registry. TestEveryCategoryHasAMasker only checks that a
// category's masker field is non-empty; a typo'd id (or one for a generator
// that was renamed or never registered) would still pass that test and only
// fail at plan time, on whatever database happened to hit that category
// first. mask.Get understands the one parametric id, "fixed:LITERAL", so this
// walks the compiled ids as written rather than re-deriving the fixed: form.
func TestEveryRulePackMaskerResolves(t *testing.T) {
	t.Parallel()
	p, err := pack()
	if err != nil {
		t.Fatalf("rule pack: %v", err)
	}
	for cat, id := range p.Masker {
		if _, ok := mask.Get(id); !ok {
			t.Errorf("category %q names masker %q, which does not resolve in the mask registry", cat, id)
		}
	}
}

// TestRulePackAgreesWithMaskAboutTypes walks the rule pack's accepts: lists
// against mask's own declaration of which type tags each category's generators
// can be written into (mask/writable.go).
//
// The two exist for different readers and are checked against each other rather
// than merged. This package gates a *decision* by the list: a signal for a
// category the column's type cannot hold is recorded at low and never masked
// (ARCHITECTURE.md §4, and T-0054 extended it from the name signal to the value
// signal). internal/plan gates the *plan* by mask's list, over the category a
// decision ended up with rather than over each step to it. If they disagreed,
// one of the two gates would be answering a question about a column the other
// had already let through, which is the failure T-0054 closed: classify decided
// credential on a timestamp, and transform was the first thing to notice, at
// exit 7 with rows already moved.
func TestRulePackAgreesWithMaskAboutTypes(t *testing.T) {
	t.Parallel()
	p, err := pack()
	if err != nil {
		t.Fatalf("rule pack: %v", err)
	}
	for cat, accepts := range p.Accepts {
		got := map[string]bool{}
		for _, tag := range mask.WritableTypes(mask.Category(cat)) {
			got[tag] = true
		}
		if len(got) == 0 {
			t.Errorf("the rule pack has category %q and mask declares no type it can be written into", cat)
			continue
		}
		if accepts == nil {
			// The rule pack's ["*"]: every family, which only special_category
			// has. mask declares no such entry, deliberately, and this is the
			// one place the two lists part company. The pack decides
			// special_category on any type at all — it is scored `certain` by
			// name alone (ARCHITECTURE.md §4), and silencing it by type would
			// copy an hiv_status column in cleartext — while the generator
			// falls through to the free-text filler on time, interval, inet,
			// cidr, macaddr and tsvector, which none of those can hold. The gap
			// is a plan refusal at exit 12 before a row moves, not a value the
			// loader chokes on mid-run (mask/writable.go, internal/plan).
			if cat != pipeline.CatSpecial {
				t.Errorf("category %q accepts every family in the rule pack; only special_category may", cat)
			}
			if len(got) == 0 {
				t.Errorf("category %q accepts every family in the rule pack and mask can write into none", cat)
			}
			continue
		}
		for fam := range accepts {
			if !got[fam] {
				t.Errorf("the rule pack accepts %s on a %s column and mask cannot write one there", cat, fam)
			}
		}
		for fam := range got {
			if !accepts[fam] {
				t.Errorf("mask can write %s into a %s column and the rule pack accepts only %v; "+
					"the plan would then admit a column the classifier declined to decide",
					cat, fam, keysOf(accepts))
			}
		}
	}
}

// TestValueSignalSurvivesATypeNothingCanJudge holds T-0054's accepted-types
// gate to its own scope. The gate exists so that a validator cannot decide a
// column whose type the category's masker demonstrably cannot write into — a
// `credential` on a timestamp. It must not reach a column that *is* writable,
// or one nothing downstream knows anything about, because the answer there is
// a cleartext copy under a green tick (CLAUDE.md, "when in doubt, mask it";
// THREAT_MODEL.md T1).
//
// Three families are outside it (silencedByType): an enum, which every
// generator answers with one of its own labels, and xml and the catch-all
// `other`, which are not a family that refuses the category but a type this
// package has no family for. All three are masked here; the timestamptz beside
// them, whose family is known and does refuse the category, is not — and the
// neighbouring-column rule may not raise it either, though three columns of
// this table are `likely` email.
func TestValueSignalSurvivesATypeNothingCanJudge(t *testing.T) {
	t.Parallel()
	tbl := ref.TableRef{Schema: "public", Name: "records"}
	schema := &pipeline.Schema{
		Enums: map[string][]string{"public.contact_kind": {"work", "home"}},
		Tables: []pipeline.Table{
			tt("public", "records", []string{"id"},
				tc("id", "bigint"),
				tc("ref_a", "public.contact_kind"),
				tc("ref_b", "xml"),
				tc("ref_c", "public.geometry"),
				tc("ref_d", "timestamp with time zone"),
			),
		},
	}
	// Test data about five fictional people, as elsewhere in this package.
	emails := anyOf(
		"ada.lovelace@fixture.test", "grace.hopper@fixture.test", "alan.turing@fixture.test",
		"katherine.johnson@fixture.test", "edsger.dijkstra@fixture.test")
	samples := mapSampler{
		col(tbl, "id"):    anyOf(int64(1), int64(2), int64(3), int64(4), int64(5)),
		col(tbl, "ref_a"): emails,
		col(tbl, "ref_b"): emails,
		col(tbl, "ref_c"): emails,
		col(tbl, "ref_d"): emails,
	}
	cls, err := New().Classify(schema, samples, nil)
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	for _, c := range []struct{ column, why string }{
		{"ref_a", "an enum is writable under every category: every generator answers a labelled column with one of its labels"},
		{"ref_b", "xml is a type this package has no family for, and nothing downstream is behind a silence here"},
		{"ref_c", "an extension type is a type this package has no family for"},
	} {
		d := decision(t, cls, col(tbl, c.column))
		if !d.Masked {
			t.Errorf("records.%s is not masked (%s): %+v", c.column, c.why, d)
		}
		if d.Category != pipeline.CatEmail {
			t.Errorf("records.%s category = %q, want email", c.column, d.Category)
		}
	}
	stamp := decision(t, cls, col(tbl, "ref_d"))
	if stamp.Masked {
		t.Errorf("records.ref_d is masked: a timestamptz cannot hold what any text category emits (%+v)", stamp)
	}
	if stamp.Confidence != pipeline.ConfLow {
		t.Errorf("records.ref_d confidence = %v, want low", stamp.Confidence)
	}
}

// TestStrongMinorityHitMasksColumnAsFreeText is the T-0136 review's finding 3:
// the classify-side unit coverage for signals.strongHit / decide's
// sig.strongHit branch was missing entirely (a decide() that dropped the
// `case sig.strongHit != nil` arm still passed `go test ./internal/classify`
// unchanged before this test existed). One email address among nineteen
// ordinary strings in a proven text column is a 5% ratio -- under both
// weakThreshold and validatorThreshold -- so before T-0136 the column fell
// through to `none` and internal/transform copied the email verbatim
// (docs/reviews/2026-09-09/REVIEW.md finding 7, evidence/sparse_email.log).
func TestStrongMinorityHitMasksColumnAsFreeText(t *testing.T) {
	t.Parallel()
	tbl := ref.TableRef{Schema: "public", Name: "sparse"}
	schema := &pipeline.Schema{
		Tables: []pipeline.Table{
			tt("public", "sparse", []string{"id"},
				tc("id", "bigint"),
				tc("misc_field", "text"),
			),
		},
	}
	values := make([]any, 0, 20)
	for i := 0; i < 19; i++ {
		values = append(values, "an ordinary string that is not personal data")
	}
	values = append(values, "ada.lovelace@fixture.test")
	samples := mapSampler{
		col(tbl, "id"):         anyOf(int64(1), int64(2), int64(3)),
		col(tbl, "misc_field"): values,
	}
	cls, err := New().Classify(schema, samples, nil)
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	d := decision(t, cls, col(tbl, "misc_field"))
	if d.Category != pipeline.CatFreeText {
		t.Errorf("sparse.misc_field category = %q, want free_text", d.Category)
	}
	if d.Confidence != pipeline.ConfPossible {
		t.Errorf("sparse.misc_field confidence = %v, want possible", d.Confidence)
	}
	if !d.Masked {
		t.Errorf("sparse.misc_field is not masked: %+v", d)
	}
	if !strings.Contains(d.Reason, "strong validator hit below the category threshold") {
		t.Errorf("sparse.misc_field reason = %q, want the strong_hit_free_text fragment", d.Reason)
	}
}

// TestNonStrongMinorityHitDoesNotMask is TestStrongMinorityHitMasksColumnAsFreeText's
// negative control: a validator the review's finding 3 also asked for, so the
// strongHit branch is shown to be reachable only through a *strong* validator
// (email, phone, network_id, Luhn, online_id) and not through address, which
// is a shape guess rather than a parse (validators.go's own comment). Address
// cannot be run at the same 1-in-20 minority ratio as the email case above and
// land anywhere but `none` -- weakThreshold is 0.5, and a hit that never
// reaches it is not evidence of anything -- so this asks the question the
// review meant to ask a different way: at a ratio that *does* clear
// weakThreshold but not validatorThreshold, a non-strong hit still lands at
// `low` (copied) rather than being promoted to a masked free_text column the
// way a strong hit at a much lower ratio is.
func TestNonStrongMinorityHitDoesNotMask(t *testing.T) {
	t.Parallel()
	tbl := ref.TableRef{Schema: "public", Name: "sparse2"}
	schema := &pipeline.Schema{
		Tables: []pipeline.Table{
			tt("public", "sparse2", []string{"id"},
				tc("id", "bigint"),
				tc("misc_field", "text"),
			),
		},
	}
	values := make([]any, 0, 20)
	for i := 0; i < 10; i++ {
		values = append(values, "12 Rue de Rivoli")
	}
	for i := 0; i < 10; i++ {
		values = append(values, "an ordinary string that is not personal data")
	}
	samples := mapSampler{
		col(tbl, "id"):         anyOf(int64(1), int64(2), int64(3)),
		col(tbl, "misc_field"): values,
	}
	cls, err := New().Classify(schema, samples, nil)
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	d := decision(t, cls, col(tbl, "misc_field"))
	if d.Masked {
		t.Errorf("sparse2.misc_field is masked: %+v, want copied at low (address is not a strong validator)", d)
	}
	if d.Confidence != pipeline.ConfLow {
		t.Errorf("sparse2.misc_field confidence = %v, want low", d.Confidence)
	}
	if d.Category != pipeline.CatAddress {
		t.Errorf("sparse2.misc_field category = %q, want address", d.Category)
	}
}

// TestStrongHitOnAnUnwritableFamilyDoesNotMask is the T-0136 review's finding
// 1: bestSignal used to gate signals.strongHit on
// silencedByType(p, hit.cat, family) -- the *hit's own* category's accepted
// families -- but decide() never assigns the hit's own category to the
// column, it always assigns free_text. financial_account accepts bigint, so
// a bigint column with a single Luhn-passing sample among an otherwise
// ordinary set of thirteen-digit identifiers cleared the old gate and was
// decided free_text/Masked=true on a family free_text's own masker cannot
// write into (rules.yml's free_text accepts text/varchar/bpchar/citext
// only) -- mask.Writable(free_text, ..., bigint) is false, and
// internal/plan/writeback.go refused the whole run at exit 12. This asserts
// the column is left unmasked instead, for internal/verify's second net
// (the family-split Luhn entry, finding 2) to catch.
func TestStrongHitOnAnUnwritableFamilyDoesNotMask(t *testing.T) {
	t.Parallel()
	tbl := ref.TableRef{Schema: "public", Name: "orders"}
	schema := &pipeline.Schema{
		Tables: []pipeline.Table{
			tt("public", "orders", []string{"id"},
				tc("id", "bigint"),
				tc("reference", "bigint"),
			),
		},
	}
	// Twenty ordinary thirteen-digit identifiers, none of which happens to
	// pass the Luhn check digit, plus one that does -- 4111111111119 is a
	// Luhn-valid thirteen-digit number (the sixteen-digit test card
	// 4111111111111111 truncated and its check digit corrected).
	values := make([]any, 0, 20)
	for i := 0; i < 19; i++ {
		values = append(values, int64(1300000000000)+int64(i))
	}
	values = append(values, int64(4111111111119))
	samples := mapSampler{
		col(tbl, "id"):        anyOf(int64(1), int64(2), int64(3)),
		col(tbl, "reference"): values,
	}
	cls, err := New().Classify(schema, samples, nil)
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	d := decision(t, cls, col(tbl, "reference"))
	if d.Masked && d.Category == pipeline.CatFreeText {
		t.Fatalf("orders.reference = %+v, masked as free_text on a bigint column -- "+
			"mask.Writable(free_text, ..., bigint) is false, so this is an exit-12 refusal waiting to happen", d)
	}
	// The fix leaves the column unmasked (CatNone) rather than routing it to
	// any other category: bigint has no typeSignals entry, so decide()'s
	// default branch is CatNone/unmasked, exactly as it was before
	// signals.strongHit existed. A regression that instead routed it
	// elsewhere (e.g. sig.weak, sig.refused) would still be worth catching,
	// so the whole decision is pinned, not only the free_text branch.
	if d.Masked {
		t.Errorf("orders.reference = %+v, want unmasked (a minority Luhn hit on bigint reaches neither "+
			"validatorThreshold nor weakThreshold, and strongHit is gated off this family now)", d)
	}
	if d.Category != pipeline.CatNone {
		t.Errorf("orders.reference category = %q, want none", d.Category)
	}
}

// keysOf is the sorted key set of a family set, for a failure message.
func keysOf(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// TestNormaliseName covers the identifier folding the whole rule pack is
// written against.
func TestNormaliseName(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ in, want string }{
		{"EmailAddress", "email_address"},
		{"ContactNumber", "contact_number"},
		{"CustomerID", "customer_id"},
		{"MigratedFromPersonID", "migrated_from_person_id"},
		{"address2", "address_2"},
		{"email_verified", "email_verified"},
		{"Notes", "notes"},
		{"first name", "first_name"},
	} {
		if got := normaliseName(tc.in); got != tc.want {
			t.Errorf("normaliseName(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// priorsUnderTest is the set of committed lazyslice.yml files the invariants
// above and TestReasonGrammar are asserted over. It is one list rather than one
// per test because the invariants — every masked column has a masker, every
// reason parses — are claims about the yml path as much as about the classifier,
// and a prior that only one of them sees is a prior the other cannot fail on.
func priorsUnderTest(t *testing.T) []*pipeline.Config {
	t.Helper()
	return []*pipeline.Config{
		nil,
		{
			ExtraPatterns: []pipeline.Pattern{
				{Name: `sku`, Category: pipeline.CatOnlineID, Confidence: pipeline.ConfCertain},
				// A raise that names no category at all.
				{Name: `\Aqty\z`, Confidence: pipeline.ConfCertain},
				// A raise written against a table name, not a column name.
				{Name: `\Aorder_items\z`, Category: pipeline.CatSpecial, Confidence: pipeline.ConfCertain},
			},
			Columns: map[ref.ColumnRef]pipeline.ColumnConfig{
				col(tPeople, "notes"): {Unmask: &pipeline.Unmask{
					Reason: "a free-form reason a user wrote", By: "sam", TypeFP: fp("notes", "text")}},
				col(tLegacy, "Notes"):     {Unmask: &pipeline.Unmask{Reason: "another one", By: "flag"}},
				col(tAudit, "entry_uid"):  {Category: pipeline.CatOnlineID, Confidence: pipeline.ConfCertain},
				col(tOrders, "placed_at"): {},
				// A category the column's type family refuses.
				col(tPeople, "ref"): {Category: pipeline.CatSemiStruct, Confidence: pipeline.ConfCertain},
				// An opt-out that records no fingerprint, and one whose
				// fingerprint is stale.
				col(tSites, "contact_email"): {Unmask: &pipeline.Unmask{Reason: "shared inbox", By: "sam"}},
				col(tDevices, "owned_by"): {Unmask: &pipeline.Unmask{
					Reason: "stale", By: "sam", TypeFP: "deadbeef"}},
			},
		},
	}
}

// TestAPersonalKeyIsNotASurrogateKey is the scope of ARCHITECTURE.md §4's
// never-masked rule: the exemption is for "surrogate keys (id bigint and the FK
// columns that reference them)", and a natural key that carries personal data is
// not one. Exempting every integer or uuid key would copy an msisdn or an NHS
// number into the target in cleartext under exit 0 (THREAT_MODEL.md T1), and
// would make §4's own propagation sentence unreachable for the case it is most
// often needed in.
func TestAPersonalKeyIsNotASurrogateKey(t *testing.T) {
	t.Parallel()
	subs := ref.TableRef{Schema: "public", Name: "subscribers"}
	visits := ref.TableRef{Schema: "public", Name: "visits"}
	schema := &pipeline.Schema{
		Tables: []pipeline.Table{
			tt("public", "subscribers", []string{"msisdn"},
				tc("msisdn", "bigint"),     // a natural key that is a phone number
				tc("nhs_number", "bigint"), // a natural identifier beside it
				tc("account_id", "bigint"), // a surrogate key by any reading
			),
			tt("public", "visits", []string{"visit_id"},
				tc("visit_id", "bigint"),
				tc("msisdn", "bigint"),
				tc("address_id", "integer"), // a name hit on a type address refuses
			),
			tt("public", "addresses", []string{"address_id"},
				tc("address_id", "integer"),
				tc("line_1", "text"),
			),
		},
		FKs: []pipeline.ForeignKey{
			fk("visits_msisdn_fkey", visits, []string{"msisdn"}, subs, []string{"msisdn"}),
			fk("visits_address_id_fkey", visits, []string{"address_id"},
				ref.TableRef{Schema: "public", Name: "addresses"}, []string{"address_id"}),
		},
	}
	cls, err := New().Classify(schema, nil, nil)
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	for _, tc := range []struct {
		c    ref.ColumnRef
		cat  pipeline.Category
		mask bool
	}{
		{ref.ColumnRef{Table: subs, Column: "msisdn"}, pipeline.CatPhone, true},
		{ref.ColumnRef{Table: subs, Column: "nhs_number"}, pipeline.CatNationalID, true},
		{ref.ColumnRef{Table: visits, Column: "msisdn"}, pipeline.CatPhone, true},
		{ref.ColumnRef{Table: subs, Column: "account_id"}, pipeline.CatNone, false},
		{ref.ColumnRef{Table: visits, Column: "visit_id"}, pipeline.CatNone, false},
	} {
		d := cls.Decisions[tc.c]
		if d.Masked != tc.mask || d.Category != tc.cat {
			t.Errorf("%s = category %q masked %t, want %q and %t (reason %q)",
				tc.c, d.Category, d.Masked, tc.cat, tc.mask, d.Reason)
		}
	}
	// A key the classifier found nothing in keeps the exemption and says so.
	if d := cls.Decisions[ref.ColumnRef{Table: visits, Column: "visit_id"}]; !strings.Contains(d.Reason, "surrogate key") {
		t.Errorf("visits.visit_id reason = %q, want the key exemption named", d.Reason)
	}
	// The exemption is lost only above the mask threshold. address_id is a name
	// hit on a type its category refuses, which §4 pins at low, so it keeps the
	// verbatim reason it had before.
	addr := cls.Decisions[ref.ColumnRef{Table: visits, Column: "address_id"}]
	if addr.Masked || !strings.Contains(addr.Reason, "preserved verbatim") {
		t.Errorf("visits.address_id = %+v, want an unmasked key with the exemption named", addr)
	}
}

// TestAMaskedKeyOverridesTheColumnsThatReferenceIt is ARCHITECTURE.md §4's
// propagation sentence — "a masked PK or unique column's decision overrides the
// decision on every column referencing it" — for the case it exists to close: an
// integer or uuid FK child whose own name and values say nothing at all, which
// markNeverMasked exempts as a surrogate key before propagation has run.
//
// Both failures are real if the exemption stands. The child ships the parent's
// personal values in cleartext under exit 0 (THREAT_MODEL.md T1), and the parent
// is replaced while the child is not, so the load hits a foreign key violation
// or silently orphans the rows (T8). The two ends converge on the parent's
// category and masker for the same reason: two maskers over one set of values
// is a broken join.
func TestAMaskedKeyOverridesTheColumnsThatReferenceIt(t *testing.T) {
	t.Parallel()
	people := ref.TableRef{Schema: "public", Name: "people"}
	visits := ref.TableRef{Schema: "public", Name: "visits"}
	labs := ref.TableRef{Schema: "public", Name: "lab_samples"}

	visitsTable := tt("public", "visits", []string{"visit_id"},
		tc("visit_id", "bigint"),
		tc("subject_ref", "bigint"), // no name signal and no samples: exempt on its own
	)
	visitsTable.Indexes = []pipeline.Index{{
		Name: "visits_subject_ref_key", Columns: []string{"subject_ref"}, Unique: true, Immediate: true,
	}}
	schema := &pipeline.Schema{
		Tables: []pipeline.Table{
			tt("public", "people", []string{"nhs_number"}, tc("nhs_number", "bigint")),
			visitsTable,
			tt("public", "lab_samples", nil,
				tc("sample_id", "bigint"),
				tc("patient_ref", "bigint"),
			),
		},
		// The grandchild edge is listed first, so that one sweep in FK order
		// would leave lab_samples.patient_ref verbatim.
		FKs: []pipeline.ForeignKey{
			fk("lab_samples_patient_ref_fkey", labs, []string{"patient_ref"}, visits, []string{"subject_ref"}),
			fk("visits_subject_ref_fkey", visits, []string{"subject_ref"}, people, []string{"nhs_number"}),
		},
	}
	cls, err := New().Classify(schema, nil, nil)
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}

	parent := cls.Decisions[ref.ColumnRef{Table: people, Column: "nhs_number"}]
	if !parent.Masked || parent.Category != pipeline.CatNationalID {
		t.Fatalf("people.nhs_number = %+v, want a masked national_id key", parent)
	}
	for _, c := range []ref.ColumnRef{
		{Table: visits, Column: "subject_ref"},
		{Table: labs, Column: "patient_ref"},
	} {
		d := cls.Decisions[c]
		if !d.Masked {
			t.Errorf("%s = %+v, want it masked: it holds the values its parent is masked for", c, d)
		}
		if d.Category != parent.Category || d.Masker != parent.Masker {
			t.Errorf("%s = category %q masker %q, want its parent's %q and %q",
				c, d.Category, d.Masker, parent.Category, parent.Masker)
		}
		if strings.Contains(d.Reason, "preserved verbatim") {
			t.Errorf("%s reason = %q; the key exemption is lifted, not printed beside the propagation", c, d.Reason)
		}
		if !strings.Contains(d.Reason, "propagated through foreign key") {
			t.Errorf("%s reason = %q, want the propagation named", c, d.Reason)
		}
	}
	// A key with nothing above it keeps the exemption.
	if d := cls.Decisions[ref.ColumnRef{Table: visits, Column: "visit_id"}]; d.Masked ||
		!strings.Contains(d.Reason, "surrogate key") {
		t.Errorf("visits.visit_id = %+v, want the exemption kept: nothing masked references it", d)
	}
}

// TestAKeyChildWhoseParentIsCopiedKeepsTheExemption is tracker T-0120's
// reconciliation, the other direction of the one above.
//
// markNeverMasked reads a column's own signals, so an integer or uuid FK child
// that reaches `possible` on a *name* hit alone loses the surrogate-key
// exemption while the primary key it references — the same values, no name hit —
// keeps it. `identities.provider_id uuid` is the shape auth schemas actually
// carry: the name is `online_id` by rules.yml's IdP rule, the parent
// `sso_providers.id` is a surrogate key, and masking the child alone replaces
// the values on one end of the edge only. The load then adds the constraint NOT
// VALID, internal/verify/fk.go counts the orphans and the run fails at exit 8
// (THREAT_MODEL.md T8) — and it protects nothing, because a child's values are a
// subset of the parent key's and the parent copied them verbatim.
func TestAKeyChildWhoseParentIsCopiedKeepsTheExemption(t *testing.T) {
	t.Parallel()
	providers := ref.TableRef{Schema: "auth", Name: "sso_providers"}
	devices := ref.TableRef{Schema: "auth", Name: "devices"}
	identities := ref.TableRef{Schema: "auth", Name: "identities"}

	schema := &pipeline.Schema{
		Tables: []pipeline.Table{
			tt("auth", "sso_providers", []string{"id"}, tc("id", "uuid")),
			// A masked uuid key: `device_id` is online_id by name, so the
			// exemption is lost on the parent's own signals.
			tt("auth", "devices", []string{"device_id"}, tc("device_id", "uuid")),
			tt("auth", "identities", []string{"id"},
				tc("id", "uuid"),
				tc("provider_id", "uuid"),
				tc("external_id", "uuid"),
			),
			// The same column under an edge Postgres never checked.
			tt("auth", "legacy_links", []string{"link_id"},
				tc("link_id", "bigint"),
				tc("provider_id", "uuid"),
			),
		},
		FKs: []pipeline.ForeignKey{
			{
				Name: "legacy_links_provider_id_fkey", Validated: false,
				Child: ref.TableRef{Schema: "auth", Name: "legacy_links"}, ChildCols: []string{"provider_id"},
				Parent: providers, ParentCols: []string{"id"},
			},
			fk("identities_provider_id_fkey", identities, []string{"provider_id"}, providers, []string{"id"}),
			// external_id has two parents, one copied and one masked. The
			// masked one has to win, or the exemption this pass hands back
			// would be a recall hole (THREAT_MODEL.md T1).
			fk("identities_external_id_fkey", identities, []string{"external_id"}, providers, []string{"id"}),
			fk("identities_external_id_device_fkey", identities, []string{"external_id"}, devices, []string{"device_id"}),
		},
	}
	cls, err := New().Classify(schema, nil, nil)
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}

	if d := cls.Decisions[ref.ColumnRef{Table: providers, Column: "id"}]; d.Masked {
		t.Fatalf("auth.sso_providers.id = %+v, want the surrogate key copied", d)
	}
	child := cls.Decisions[ref.ColumnRef{Table: identities, Column: "provider_id"}]
	if child.Masked {
		t.Errorf("auth.identities.provider_id = %+v, want it copied: its parent key is copied, so masking this end orphans the row and hides nothing", child)
	}
	if child.Masker != "" {
		t.Errorf("auth.identities.provider_id masker = %q, want none on a copied column", child.Masker)
	}
	// The name signal is kept on the line rather than erased: the report says
	// what the column looked like *and* which key overruled it.
	if child.Category != pipeline.CatOnlineID {
		t.Errorf("auth.identities.provider_id category = %q, want %q kept", child.Category, pipeline.CatOnlineID)
	}
	if !strings.Contains(child.Reason, "a copied surrogate key: preserved verbatim") {
		t.Errorf("auth.identities.provider_id reason = %q, want the parent key named", child.Reason)
	}
	if bad, ok := ParseReason(child.Reason); !ok {
		t.Errorf("reason %q does not parse: %q", child.Reason, bad)
	}

	both := cls.Decisions[ref.ColumnRef{Table: identities, Column: "external_id"}]
	if !both.Masked || both.Category != pipeline.CatOnlineID {
		t.Errorf("auth.identities.external_id = %+v, want it masked: one of its two parents is masked", both)
	}
	if strings.Contains(both.Reason, "preserved verbatim") {
		t.Errorf("auth.identities.external_id reason = %q, want the exemption blanked where propagation took over", both.Reason)
	}
	if !strings.Contains(both.Reason, "propagated through foreign key") {
		t.Errorf("auth.identities.external_id reason = %q, want the propagation named", both.Reason)
	}

	// The subset argument is only true where Postgres checked it. An
	// unvalidated constraint is a hint over rows it never verified, and
	// internal/plan will not even follow it to fetch the parent row, so the
	// child can hold a value that is nowhere else in the snapshot.
	legacy := cls.Decisions[ref.ColumnRef{Table: ref.TableRef{Schema: "auth", Name: "legacy_links"}, Column: "provider_id"}]
	if !legacy.Masked {
		t.Errorf("auth.legacy_links.provider_id = %+v, want it masked: NOT VALID is a hint, not a guarantee that the parent holds the value", legacy)
	}
}

// TestConfigCannotRaiseWithoutAUsableCategory is the other half of ADR-004's
// tighten-only rule. A raise is applied only when it leaves a category the
// column's type family accepts, because Decision.Masker is chosen from the
// category: a decision that says "mask this" with no category is one transform
// has nothing to mask with (ADR-006), and a JSON masker on a text column is a
// pairing the rule pack's own `accepts:` list says cannot exist.
func TestConfigCannotRaiseWithoutAUsableCategory(t *testing.T) {
	t.Parallel()

	t.Run("AConfidenceWithNoCategoryIsNotApplied", func(t *testing.T) {
		widget := ref.TableRef{Schema: "public", Name: "widget"}
		schema := &pipeline.Schema{Tables: []pipeline.Table{
			tt("public", "widget", nil, tc("sku", "text")),
		}}
		prior := &pipeline.Config{ExtraPatterns: []pipeline.Pattern{
			{Name: `sku`, Confidence: pipeline.ConfCertain},
		}}
		cls, err := New().Classify(schema, nil, prior)
		if err != nil {
			t.Fatalf("Classify: %v", err)
		}
		d := cls.Decisions[ref.ColumnRef{Table: widget, Column: "sku"}]
		if d.Masked || d.Category != pipeline.CatNone || d.Masker != "" {
			t.Errorf("widget.sku = %+v, want it left alone: a raise with no category has no masker", d)
		}
		if !strings.Contains(d.Reason, "no usable category") {
			t.Errorf("widget.sku reason = %q, want the dropped raise named", d.Reason)
		}
	})

	t.Run("ACategoryTheTypeRefusesIsNotTaken", func(t *testing.T) {
		refCol := col(tPeople, "ref")
		prior := &pipeline.Config{Columns: map[ref.ColumnRef]pipeline.ColumnConfig{
			refCol: {Category: pipeline.CatSemiStruct, Confidence: pipeline.ConfCertain},
		}}
		d := decision(t, mustClassify(t, prior), refCol)
		if d.Category != pipeline.CatEmail {
			t.Errorf("people.ref category = %q, want email kept: semi_structured accepts no text column", d.Category)
		}
		if d.Masker != "semi_structured" && !strings.Contains(d.Reason, "not an accepted type for semi_structured") {
			t.Errorf("people.ref reason = %q, want the conflict recorded", d.Reason)
		}
		if d.Confidence != pipeline.ConfCertain {
			t.Errorf("people.ref confidence = %v, want the raise still applied to the classifier's category", d.Confidence)
		}
	})

	t.Run("AConfidenceOverATypeConflictingCategoryIsNotApplied", func(t *testing.T) {
		// The gate is on the category the raise would leave, not only on the one
		// the yml named. people.email_verified is `email` at low on a boolean —
		// ARCHITECTURE.md §4 pins it there and the neighbouring-column rule may
		// not raise it — so a yml entry that supplies a confidence and no usable
		// category must not be the one route to the email masker on a boolean.
		verified := col(tPeople, "email_verified")
		for _, tc := range []struct {
			name  string
			prior *pipeline.Config
		}{
			{"by column, confidence only", &pipeline.Config{
				Columns: map[ref.ColumnRef]pipeline.ColumnConfig{
					verified: {Confidence: pipeline.ConfCertain},
				},
			}},
			{"by pattern, confidence only", &pipeline.Config{
				ExtraPatterns: []pipeline.Pattern{
					{Name: `\Aemail_verified\z`, Confidence: pipeline.ConfCertain},
				},
			}},
			{"by pattern, with a category the type refuses", &pipeline.Config{
				ExtraPatterns: []pipeline.Pattern{
					{Name: `\Aemail_verified\z`, Category: pipeline.CatSemiStruct, Confidence: pipeline.ConfCertain},
				},
			}},
		} {
			d := decision(t, mustClassify(t, tc.prior), verified)
			if d.Masked || d.Masker != "" {
				t.Errorf("%s: people.email_verified = %+v, want it left unmasked with no masker", tc.name, d)
			}
			if d.Confidence != pipeline.ConfLow {
				t.Errorf("%s: people.email_verified confidence = %v, want low", tc.name, d.Confidence)
			}
			if !strings.Contains(d.Reason, "no usable category") {
				t.Errorf("%s: people.email_verified reason = %q, want the dropped raise named", tc.name, d.Reason)
			}
		}
	})
}

// TestAnExtraPatternMatchesTheTableName is pipeline.Pattern's own contract —
// "regex over column or table name". A user who writes a pattern for a table of
// patient records to raise the whole table must not get silence, because a
// raise that quietly does nothing is a recall failure the user believes they
// have fixed (THREAT_MODEL.md T3).
func TestAnExtraPatternMatchesTheTableName(t *testing.T) {
	t.Parallel()
	patients := ref.TableRef{Schema: "public", Name: "patients"}
	schema := &pipeline.Schema{Tables: []pipeline.Table{
		tt("public", "patients", nil, tc("code", "text"), tc("summary_line", "text")),
	}}
	prior := &pipeline.Config{ExtraPatterns: []pipeline.Pattern{
		{Name: `\Apatients\z`, Category: pipeline.CatSpecial, Confidence: pipeline.ConfCertain},
	}}
	cls, err := New().Classify(schema, nil, prior)
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	for _, name := range []string{"code", "summary_line"} {
		d := cls.Decisions[ref.ColumnRef{Table: patients, Column: name}]
		if !d.Masked || d.Category != pipeline.CatSpecial {
			t.Errorf("patients.%s = %+v, want the whole table raised by its name", name, d)
		}
	}
}

// TestAnOptOutWithNoFingerprintIsIgnored is the fail-closed half of
// THREAT_MODEL.md T3's control. The control is that an opt-out records the
// column's type fingerprint and is ignored when it changes; an opt-out that
// records none is one nothing can ever revoke, so a hand-written or older
// lazyslice.yml cannot switch masking off permanently by leaving a field out.
func TestAnOptOutWithNoFingerprintIsIgnored(t *testing.T) {
	t.Parallel()
	notes := col(tPeople, "notes")

	for _, tc := range []struct {
		name string
		u    pipeline.Unmask
	}{
		{"no fingerprint", pipeline.Unmask{Reason: "hand written", By: "sam"}},
		{"no reason", pipeline.Unmask{By: "sam", TypeFP: fp("notes", "text")}},
		{"nothing at all", pipeline.Unmask{}},
	} {
		u := tc.u
		cls := mustClassify(t, &pipeline.Config{Columns: map[ref.ColumnRef]pipeline.ColumnConfig{
			notes: {Unmask: &u},
		}})
		if d := cls.Decisions[notes]; !d.Masked {
			t.Errorf("%s: %s = %+v, want the opt-out ignored and the column masked", tc.name, notes, d)
		}
		if len(cls.Expired) != 1 || cls.Expired[0] != notes {
			t.Errorf("%s: Expired = %v, want exactly %s", tc.name, cls.Expired, notes)
		}
	}

	// A --unmask flag opt-out is made for one run and dies with it, so it
	// carries no fingerprint by construction and is still honoured.
	flagged := mustClassify(t, &pipeline.Config{Columns: map[ref.ColumnRef]pipeline.ColumnConfig{
		notes: {Unmask: &pipeline.Unmask{Reason: "one run only", By: "flag"}},
	}})
	if d := flagged.Decisions[notes]; d.Masked {
		t.Errorf("%s = %+v, want a --unmask opt-out honoured", notes, d)
	}
}

// TestAUniqueExpressionIndexIsRecorded is ARCHITECTURE.md §5's "unique indexes
// (including expression indexes such as lower(email))". The flag is what makes
// the plan pick a generator whose domain reaches the row count; without it the
// load fails on a unique violation, which is the failure the domain check
// exists to prevent.
func TestAUniqueExpressionIndexIsRecorded(t *testing.T) {
	t.Parallel()
	users := ref.TableRef{Schema: "public", Name: "users"}
	u := tt("public", "users", nil, tc("email", "text"), tc("other", "text"))
	u.Indexes = []pipeline.Index{{
		Name: "users_lower_email_key", Unique: true, Expression: true, Immediate: true,
		Def: "CREATE UNIQUE INDEX users_lower_email_key ON public.users USING btree (lower((email)::text))",
	}}
	cls, err := New().Classify(&pipeline.Schema{Tables: []pipeline.Table{u}}, nil, nil)
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	if d := cls.Decisions[ref.ColumnRef{Table: users, Column: "email"}]; !d.UniqueIndex {
		t.Errorf("users.email = %+v, want UniqueIndex set by the expression index", d)
	}
	if d := cls.Decisions[ref.ColumnRef{Table: users, Column: "other"}]; d.UniqueIndex {
		t.Errorf("users.other = %+v, want UniqueIndex unset: no index covers it", d)
	}
	// The nasty fixture's own expression index, beside the partial one.
	nasty := mustClassify(t, nil)
	if d := decision(t, nasty, col(tAudit, "entry_uid")); !d.UniqueIndex {
		t.Errorf("audit_log.entry_uid = %+v, want UniqueIndex set", d)
	}
}

// TestATypeIsFoundThroughItsSchemaQualification is ARCHITECTURE.md §11.1 item
// 2's "CREATE EXTENSION ... SCHEMA <schema>" reaching the classifier:
// pg_catalog.format_type qualifies any type that is not visible in the
// connection's search_path, so citext, public.citext and extensions.citext are
// three spellings of one type. Reading only the first as citext would classify
// an email column of the other two as famOther, which no category accepts.
func TestATypeIsFoundThroughItsSchemaQualification(t *testing.T) {
	t.Parallel()
	for _, spelling := range []string{"citext", "public.citext", "extensions.citext"} {
		tbl := ref.TableRef{Schema: "public", Name: "account"}
		schema := &pipeline.Schema{Tables: []pipeline.Table{
			tt("public", "account", nil, tc("email", spelling), tc("props", strings.Replace(spelling, "citext", "hstore", 1))),
		}}
		cls, err := New().Classify(schema, nil, nil)
		if err != nil {
			t.Fatalf("Classify: %v", err)
		}
		email := cls.Decisions[ref.ColumnRef{Table: tbl, Column: "email"}]
		if !email.Masked || email.Category != pipeline.CatEmail {
			t.Errorf("an email column of type %q = %+v, want a masked email column", spelling, email)
		}
		props := cls.Decisions[ref.ColumnRef{Table: tbl, Column: "props"}]
		if !props.Masked || props.Category != pipeline.CatSemiStruct {
			t.Errorf("an hstore column spelled like %q = %+v, want a masked semi_structured column", spelling, props)
		}
	}
	// An enum is resolved by both spellings too: Schema.Enums is keyed
	// "nspname.typname" and format_type writes an in-search_path enum bare.
	tbl := ref.TableRef{Schema: "public", Name: "account"}
	schema := &pipeline.Schema{
		Enums: map[string][]string{"public.mood": {"ok", "bad"}},
		Tables: []pipeline.Table{
			tt("public", "account", nil, tc("marital_status", "mood")),
		},
	}
	cls, err := New().Classify(schema, nil, nil)
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	if d := cls.Decisions[ref.ColumnRef{Table: tbl, Column: "marital_status"}]; !d.Masked {
		t.Errorf("an enum column spelled bare = %+v, want it masked as a special category", d)
	}
}

// TestByteaIsNotReadAsText is ARCHITECTURE.md §4's separate rule for bytea:
// "bytea in a person-shaped column is binary_personal and set to NULL". A bytea
// sample is arbitrary binary, and running it through the text validators makes a
// PNG read as an address — a decision the rule pack itself contradicts, since
// address accepts no bytea column.
func TestByteaIsNotReadAsText(t *testing.T) {
	t.Parallel()
	tbl := ref.TableRef{Schema: "public", Name: "uploads"}
	schema := &pipeline.Schema{Tables: []pipeline.Table{
		tt("public", "uploads", nil, tc("blob", "bytea")),
	}}
	png := func(n byte) any {
		return []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a, 0, 0, n, 'I', 'H', 'D', 'R', ' ', '1', '2', ' ', 'a', 'b'}
	}
	samples := mapSampler{
		ref.ColumnRef{Table: tbl, Column: "blob"}: anyOf(png(1), png(2), png(3), png(4)),
	}
	cls, err := New().Classify(schema, samples, nil)
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	d := cls.Decisions[ref.ColumnRef{Table: tbl, Column: "blob"}]
	if d.Category != pipeline.CatNone {
		t.Errorf("uploads.blob = %+v, want no category: a text validator may not decide a bytea column", d)
	}
}

// TestReasonGrammarQuotesOddIdentifiers is ARCHITECTURE.md §10's "every reason:
// string parses against the template set", held against the identifiers
// PostgreSQL actually allows. A hyphen in a schema name is ordinary, and a
// reason this package renders must parse back whatever the database is called.
func TestReasonGrammarQuotesOddIdentifiers(t *testing.T) {
	t.Parallel()
	parent := ref.TableRef{Schema: "my-app", Name: "user events"}
	child := ref.TableRef{Schema: "my-app", Name: "child; rows"}
	p := tt("my-app", "user events", []string{"main id"},
		tc("main id", "text"), tc("email", "text"), tc("half done", "text"))
	c := tt("my-app", "child; rows", nil, tc("main id", "text"))
	schema := &pipeline.Schema{
		Tables: []pipeline.Table{p, c},
		FKs: []pipeline.ForeignKey{
			fk(`fk with "space"`, child, []string{"main id"}, parent, []string{"main id"}),
		},
	}
	samples := mapSampler{
		ref.ColumnRef{Table: parent, Column: "main id"}: anyOf("a@b.com", "c@d.com", "e@f.com", "g@h.com"),
		ref.ColumnRef{Table: parent, Column: "email"}:   anyOf("a@b.com", "c@d.com", "e@f.com", "g@h.com"),
		// address, not email: address is not a strong validator (validators.go),
		// so this stays a weak signal at `low` for the neighbouring-column rule
		// to raise, which is the fragment this fixture needs. A strong
		// validator's hit at the same ratio decides free_text on its own
		// (finding 7) and never reaches the rule.
		ref.ColumnRef{Table: parent, Column: "half done"}: anyOf("42 Cedar Street", "17 Birch Lane", "nope", "nah"),
	}
	cls, err := New().Classify(schema, samples, nil)
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	saw := map[string]bool{}
	for cc, d := range cls.Decisions {
		if bad, ok := ParseReason(d.Reason); !ok {
			t.Errorf("%s reason %q holds a fragment no template produced: %q", cc, d.Reason, bad)
		}
		for _, frag := range []string{"neighbouring-column rule", "foreign key"} {
			if strings.Contains(d.Reason, frag) {
				saw[frag] = true
			}
		}
	}
	if len(saw) != 2 {
		t.Errorf("the fixture rendered %d of the two fragments that interpolate a table name; it proves nothing about the rest", len(saw))
	}
}
