// SPDX-License-Identifier: Apache-2.0

package mask

import "testing"

// tcase is one column shape and one production-shaped value for it. The corpus
// is shared by the determinism, format and pass-through tests so that a
// generator added to the registry without a row here is visible: every
// registered id must appear at least once (TestCorpusCoversTheRegistry).
type tcase struct {
	name string
	cat  Category
	id   ID
	in   string
	c    Constraints
}

func corpus() []tcase {
	return []tcase{
		{"email varchar", CatEmail, MaskerEmail, "Alice.Kaminski@Corp.example",
			Constraints{TypeTag: famVarchar, MaxLen: 50}},
		{"email unique", CatEmail, MaskerEmail, "Alice.Kaminski@Corp.example",
			Constraints{TypeTag: famVarchar, MaxLen: 100, Unique: true, Rows: 500}},
		{"email text", CatEmail, MaskerEmail, "b@c.example", Constraints{TypeTag: famText}},
		{"name varchar", CatPersonName, MaskerPersonName, "Zoë Müller",
			Constraints{TypeTag: famVarchar, MaxLen: 45}},
		{"name narrow", CatPersonName, MaskerPersonName, "Zoë Müller",
			Constraints{TypeTag: famVarchar, MaxLen: 6}},
		{"phone text", CatPhone, MaskerPhone, "+44 20 7946 0958",
			Constraints{TypeTag: famVarchar, MaxLen: 20, Region: "GB"}},
		{"phone bigint", CatPhone, MaskerPhone, "12125559999", Constraints{TypeTag: famBigint}},
		{"phone unique", CatPhone, MaskerPhoneUnique, "212-555-9999",
			Constraints{TypeTag: famVarchar, MaxLen: 20, Unique: true, Rows: 500}},
		{"address street", CatAddress, MaskerAddress, "47 Bell Street",
			Constraints{TypeTag: famVarchar, MaxLen: 50}},
		{"address postcode", CatAddress, MaskerAddress, "SW1A 1AA",
			Constraints{TypeTag: famVarchar, MaxLen: 10}},
		{"geo numeric", CatGeo, MaskerGeo, "51.5074", Constraints{TypeTag: famNumeric}},
		{"geo text", CatGeo, MaskerGeo, "-0.127758", Constraints{TypeTag: famVarchar, MaxLen: 12}},
		{"date", CatPersonDate, MaskerPersonDate, "1974-03-02", Constraints{TypeTag: famDate}},
		{"timestamp", CatPersonDate, MaskerPersonDate, "1974-03-02 11:00:00",
			Constraints{TypeTag: famTimestamp}},
		{"national id", CatNationalID, MaskerNationalID, "078-05-1120",
			Constraints{TypeTag: famVarchar, MaxLen: 11}},
		{"national id bigint", CatNationalID, MaskerNationalID, "78051120",
			Constraints{TypeTag: famBigint}},
		{"card", CatFinancial, MaskerFinancial, "4111 1111 1111 1111", Constraints{TypeTag: famText}},
		{"ipv4", CatNetworkID, MaskerNetworkID, "192.168.1.44", Constraints{TypeTag: famInet}},
		{"ipv6", CatNetworkID, MaskerNetworkID, "2a00:1450:4009:81f::200e",
			Constraints{TypeTag: famInet}},
		{"mac", CatNetworkID, MaskerNetworkID, "3c:22:fb:aa:bb:cc", Constraints{TypeTag: famMacaddr}},
		{"cidr", CatNetworkID, MaskerNetworkID, "10.0.0.0/8", Constraints{TypeTag: famCIDR}},
		{"ip unique", CatNetworkID, MaskerIPUnique, "192.168.1.44",
			Constraints{TypeTag: famInet, Unique: true, Rows: 1_000_000}},
		{"url", CatOnlineID, MaskerOnlineID, "https://social.example/someone",
			Constraints{TypeTag: famText}},
		{"url varchar", CatOnlineID, MaskerOnlineID, "https://social.example/someone",
			Constraints{TypeTag: famVarchar, MaxLen: 36, Unique: true, Rows: 100_000}},
		{"handle", CatOnlineID, MaskerOnlineID, "someone",
			Constraints{TypeTag: famVarchar, MaxLen: 30}},
		{"uuid", CatOnlineID, MaskerOnlineID, "3f5a1c2e-1111-4222-8333-444455556666",
			Constraints{TypeTag: famUUID}},
		{"credential", CatCredential, CredentialMasker, "$2y$10$abcdefghijklmnop",
			Constraints{TypeTag: famVarchar, MaxLen: 60}},
		{"free text", CatFreeText, MaskerFreeText, "a long bio about a real person",
			Constraints{TypeTag: famVarchar, MaxLen: 200}},
		{"special enum", CatSpecial, MaskerSpecial, "married",
			Constraints{TypeTag: famEnum, EnumLabels: []string{"single", "married", "widowed"}, Distinct: 3}},
		{"special text", CatSpecial, MaskerSpecial, "type 2 diabetes",
			Constraints{TypeTag: famText, Distinct: 8}},
		{"binary", CatBinary, MaskerNull, "PNGDATA",
			Constraints{TypeTag: famBytea, Nullable: true}},
		{"jsonb", CatSemiStruct, MaskerSemiStruct,
			`{"b":{"name":"Ada","age":36},"a":[1,true,null,"x"]}`, Constraints{TypeTag: famJSONB}},
	}
}

func TestCorpusCoversTheRegistry(t *testing.T) {
	seen := map[ID]bool{}
	for _, tc := range corpus() {
		seen[tc.id] = true
	}
	for _, id := range IDs() {
		if !seen[id] {
			t.Errorf("masker %q has no row in corpus(): add one with the shape it is for", id)
		}
	}
}
