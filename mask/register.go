// SPDX-License-Identifier: Apache-2.0

package mask

// The built-in registry. Every category in the rule pack has a masker here,
// because ADR-006 makes a category with no shipped masker impossible rather
// than merely discouraged.
//
// No test crosses the module boundary yet, so that is a convention and not
// something the build enforces: this module's TestEveryCategoryHasAMasker walks
// the Category constants below, and the parent's
// internal/classify TestEveryCategoryHasAMasker only asserts that each rule-pack
// row names a non-empty masker string — neither resolves a rules.yml id against
// this registry, so `masker: person_nam` would ship green and fail at transform.
// IDs() and Candidates() are exported for the walk that closes it; it belongs on
// the classify/plan side, which can import mask.
//
// Order matters within a category: the first entry is the default, and the
// rest are the alternates Pick considers for a column under a unique index.
const (
	MaskerEmail       ID = "email"
	MaskerPersonName  ID = "person_name"
	MaskerPhone       ID = "phone"
	MaskerPhoneUnique ID = "phone_unique"
	MaskerAddress     ID = "address"
	MaskerGeo         ID = "geo"
	MaskerPersonDate  ID = "person_date"
	MaskerNationalID  ID = "national_id"
	MaskerFinancial   ID = "financial_account"
	MaskerNetworkID   ID = "network_id"
	MaskerIPUnique    ID = "ip_unique"
	MaskerOnlineID    ID = "online_id"
	MaskerFreeText    ID = "free_text"
	MaskerSpecial     ID = "special_category"
	MaskerNull        ID = "null"
	MaskerSemiStruct  ID = "semi_structured"
)

func init() {
	Register(MaskerEmail, CatEmail, emailMasker{})
	Register(MaskerPersonName, CatPersonName, personNameMasker{})
	Register(MaskerPhone, CatPhone, phoneMasker{})
	Register(MaskerPhoneUnique, CatPhone, phoneUniqueMasker{})
	Register(MaskerAddress, CatAddress, addressMasker{})
	Register(MaskerGeo, CatGeo, geoMasker{})
	Register(MaskerPersonDate, CatPersonDate, personDateMasker{})
	Register(MaskerNationalID, CatNationalID, nationalIDMasker{})
	Register(MaskerFinancial, CatFinancial, financialAccountMasker{})
	Register(MaskerNetworkID, CatNetworkID, networkIDMasker{})
	Register(MaskerIPUnique, CatNetworkID, ipUniqueMasker{})
	Register(MaskerOnlineID, CatOnlineID, onlineIDMasker{})
	Register(CredentialMasker, CatCredential, fixedMasker{literal: CredentialLiteral})
	Register(MaskerFreeText, CatFreeText, freeTextMasker{})
	Register(MaskerSpecial, CatSpecial, specialCategoryMasker{})
	Register(MaskerNull, CatBinary, nullMasker{})
	Register(MaskerSemiStruct, CatSemiStruct, semiStructuredMasker{})
}
