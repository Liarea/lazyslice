// SPDX-License-Identifier: Apache-2.0

package classify

import (
	"fmt"
	"testing"

	"github.com/Liarea/lazyslice/internal/pipeline"
	"github.com/Liarea/lazyslice/internal/ref"
	"github.com/Liarea/lazyslice/internal/textsig"
)

// Dogfood session 1's credential false positives, by shape (tracker T-0315).
// Every column in the first table was masked as `credential`, "N/N samples
// look like secrets", to the fixed "$lazyslice$invalid"; the STI `type`
// column's literal raised on every row the application loaded. The second
// table is the half that must not move: the real secrets the same run caught
// correctly.
func TestEntropyValidatorSparesApplicationShapes(t *testing.T) {
	t.Parallel()

	tFiles := ref.TableRef{Schema: "public", Name: "t315_assets"}
	tKeys := ref.TableRef{Schema: "public", Name: "t315_integrations"}
	tUploads := ref.TableRef{Schema: "public", Name: "t315_uploads"}
	tRefs := ref.TableRef{Schema: "public", Name: "t315_refs"}
	schema := &pipeline.Schema{
		Tables: []pipeline.Table{
			tt("public", "t315_assets", []string{"id"},
				tc("id", "integer"),
				tc("screenshot", "text"),
				tc("release_file", "character varying(255)"),
				tc("file_md5", "text"),
				tc("fingerprint", "text"),
				tc("type", "character varying(255)"),
				tc("klass", "text"),
				tc("component_name", "text"),
				tc("formatter", "text"),
				tc("env_var", "text"),
			),
			tt("public", "t315_integrations", []string{"id"},
				tc("id", "integer"),
				tc("api_key", "text"),
				tc("access_token", "text"),
				tc("encrypted_password", "character varying(255)"),
				tc("client_secret", "text"),
				tc("opaque_ref", "text"),
			),
			// A table of its own, so that no neighbouring credential column
			// raises the file names: the column rule is what must mask them.
			tt("public", "t315_uploads", []string{"id"},
				tc("id", "integer"),
				tc("upload_file", "text"),
			),
			// The review round's half (T-0315): a neutrally named column of
			// dotted handles, identifier words joined by a dot, which only
			// the entropy check reads.
			tt("public", "t315_refs", []string{"id"},
				tc("id", "integer"),
				tc("external_ref", "text"),
			),
		},
		Fingerprint: "t315",
	}

	const n = 20
	gen := func(f func(i int) string) []any {
		out := make([]any, n)
		for i := range out {
			out[i] = f(i)
		}
		return out
	}
	// hexOf is a deterministic hex run of the given length for row i: not a
	// real digest, but the same shape, which is all the validator reads.
	hexOf := func(i, length int) string {
		s := ""
		for j := 0; len(s) < length; j++ {
			s += fmt.Sprintf("%08x", uint32(2654435761*uint64(i*31+j+7)))
		}
		return s[:length]
	}
	// b62 is a random-looking base62 run for row i.
	b62 := func(i, length int) string {
		const a = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
		out := make([]byte, length)
		x := uint64(i*7919 + 104729)
		for j := range out {
			x = x*6364136223846793005 + 1442695040888963407
			out[j] = a[(x>>33)%uint64(len(a))]
		}
		return string(out)
	}
	stiClasses := []string{
		"ScheduledReportExport", "WebhookDeliveryAttempt", "SlackNotificationRule",
		"DeviceHeartbeatEvent", "PlaylistScheduleOverride",
	}

	samples := mapSampler{
		col(tFiles, "id"):           gen(func(i int) string { return fmt.Sprint(i + 1) }),
		col(tFiles, "screenshot"):   gen(func(i int) string { return fmt.Sprintf("Screenshot_2024-03-%02d_%s.png", i+1, b62(i, 6)) }),
		col(tFiles, "release_file"): gen(func(i int) string { return fmt.Sprintf("player-4.%d.%d-%s-x64.exe", i, i%3, b62(i, 5)) }),
		col(tFiles, "file_md5"):     gen(func(i int) string { return hexOf(i, 32) }),
		col(tFiles, "fingerprint"):  gen(func(i int) string { return "sha256:" + hexOf(i, 64) }),
		col(tFiles, "type"):         gen(func(i int) string { return stiClasses[i%len(stiClasses)] }),
		col(tFiles, "klass"):        gen(func(i int) string { return stiClasses[(i+2)%len(stiClasses)] }),
		col(tFiles, "component_name"): gen(func(i int) string {
			return []string{"PlaylistEditorV2", "DashboardWidget3", "ScheduleCalendar2"}[i%3]
		}),
		col(tFiles, "formatter"): gen(func(i int) string {
			return []string{"Reports::Formatters::CsvFormatter", "Reports::Formatters::XlsxFormatter", "Logger::JsonFormatter2"}[i%3]
		}),
		col(tFiles, "env_var"): gen(func(i int) string {
			return []string{"AWS_S3_BUCKET_V2", "SENTRY_DSN_EU2_PROD", "OAUTH2_CLIENT_ID_PROD"}[i%3]
		}),

		col(tKeys, "id"):                 gen(func(i int) string { return fmt.Sprint(i + 1) }),
		col(tKeys, "api_key"):            gen(func(i int) string { return hexOf(i+100, 32) }),
		col(tKeys, "access_token"):       gen(func(i int) string { return hexOf(i+200, 64) }),
		col(tKeys, "encrypted_password"): gen(func(i int) string { return "$2a$12$" + b62(i+300, 53) }),
		col(tKeys, "client_secret"):      gen(func(i int) string { return b62(i+400, 40) }),
		col(tKeys, "opaque_ref"):         gen(func(i int) string { return b62(i+500, 32) }),
		// mastodon's `photo-<username>-<n>.jpg`: every file is named after
		// its owner, and the dictionary holds half of the owners' names, so
		// value by value only half of them are left to the entropy check.
		col(tUploads, "id"): gen(func(i int) string { return fmt.Sprint(i + 1) }),
		col(tUploads, "upload_file"): gen(func(i int) string {
			return fmt.Sprintf("photo-%s_%s%d-%d.jpg",
				[]string{"grace", "dmytro"}[i%2], []string{"hopper", "egorov"}[i%2], i, i+1)
		}),
		col(tRefs, "id"): gen(func(i int) string { return fmt.Sprint(i + 1) }),
		col(tRefs, "external_ref"): gen(func(i int) string {
			return fmt.Sprintf("%s.%s%02d",
				[]string{"katherine", "margaret", "christopher", "alexandra"}[i%4],
				[]string{"johnson", "okafor", "fitzgerald", "henderson", "blackwood"}[i%5], 60+i)
		}),
	}

	// Precondition: every spared column's values cleared the entropy check as
	// it stood before T-0315, or the column pins nothing. The `type`, `klass`
	// and `component_name` values are asked of LooksSecret itself, because it
	// is their column name and not their shape that spares them.
	for _, name := range []string{"type", "klass", "component_name"} {
		for _, v := range samples[col(tFiles, name)] {
			if !textsig.LooksSecret(v.(string)) {
				t.Fatalf("precondition: %s value %q does not look like a secret, so the name exemption is not what spares it", name, v)
			}
		}
	}

	cls, err := New().Classify(schema, samples, nil)
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}

	for _, name := range []string{
		"screenshot", "release_file", "file_md5", "fingerprint",
		"type", "klass", "component_name", "formatter", "env_var",
	} {
		d := decision(t, cls, col(tFiles, name))
		if d.Category == pipeline.CatCredential || d.Masked {
			t.Errorf("t315_assets.%s = %s/%v masked=%v (%s): an application value is not a credential (T-0315)",
				name, d.Category, d.Confidence, d.Masked, d.Reason)
		}
	}
	for _, name := range []string{"api_key", "access_token", "encrypted_password", "client_secret", "opaque_ref"} {
		d := decision(t, cls, col(tKeys, name))
		if d.Category != pipeline.CatCredential || !d.Masked {
			t.Errorf("t315_integrations.%s = %s/%v masked=%v (%s): a real secret must stay caught (T-0315)",
				name, d.Category, d.Confidence, d.Masked, d.Reason)
		}
	}
	if d := decision(t, cls, col(tUploads, "upload_file")); d.Category != pipeline.CatCredential || !d.Masked {
		t.Errorf("t315_uploads.upload_file = %s/%v masked=%v (%s): file names half of which carry a dictionary name must stay masked (T-0315)",
			d.Category, d.Confidence, d.Masked, d.Reason)
	}
	if d := decision(t, cls, col(tRefs, "external_ref")); !d.Masked {
		t.Errorf("t315_refs.external_ref = %s/%v masked=%v (%s): dotted handles built from people's names must stay masked (T-0315)",
			d.Category, d.Confidence, d.Masked, d.Reason)
	}
}

// A strong entropy hit decides a column only over minSecretSamples or more
// (T-0315): dogfood session 1 masked two columns holding one and four values
// to the credential literal on the entropy check alone. Five values decide it
// as before, and a column named like a credential is masked on its name
// whatever its count.
func TestEntropyValidatorNeedsFiveSamples(t *testing.T) {
	t.Parallel()

	tbl := ref.TableRef{Schema: "public", Name: "t315_settings"}
	schema := &pipeline.Schema{
		Tables: []pipeline.Table{
			tt("public", "t315_settings", []string{"id"},
				tc("id", "integer"),
				tc("one_value", "text"),
				tc("four_values", "text"),
				tc("five_values", "text"),
				tc("webhook_secret", "text"),
			),
		},
		Fingerprint: "t315-count",
	}
	tokens := anyOf(
		"q4r7uXzN8vK2mL9pQ3sT6wY1", "Zb3Xk9Lm2Qp7Rt5Vw8Yc1Nd4", "H7jK2mP9qR4sT8vW3xY6zB1c",
		"aB3dE6gH9jK2mN5pQ8rT1vW4", "M2nP5qR8sT1vW4xY7zB0cD3f",
	)
	samples := mapSampler{
		col(tbl, "id"):             anyOf(int64(1), int64(2), int64(3), int64(4), int64(5)),
		col(tbl, "one_value"):      tokens[:1],
		col(tbl, "four_values"):    tokens[:4],
		col(tbl, "five_values"):    tokens,
		col(tbl, "webhook_secret"): tokens[:1],
	}
	for _, v := range tokens {
		if !textsig.LooksSecret(v.(string)) {
			t.Fatalf("precondition: %q does not look like a secret", v)
		}
	}
	cls, err := New().Classify(schema, samples, nil)
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}

	for _, name := range []string{"one_value", "four_values"} {
		d := decision(t, cls, col(tbl, name))
		if d.Confidence >= pipeline.ConfLikely {
			t.Errorf("t315_settings.%s = %s/%v (%s): an entropy hit over fewer than %d samples decided the column",
				name, d.Category, d.Confidence, d.Reason, minSecretSamples)
		}
	}
	if d := decision(t, cls, col(tbl, "five_values")); d.Category != pipeline.CatCredential || !d.Masked {
		t.Errorf("t315_settings.five_values = %s/%v masked=%v (%s): five secret-shaped samples must still decide credential",
			d.Category, d.Confidence, d.Masked, d.Reason)
	}
	if d := decision(t, cls, col(tbl, "webhook_secret")); d.Category != pipeline.CatCredential || !d.Masked {
		t.Errorf("t315_settings.webhook_secret = %s/%v masked=%v (%s): a credential name masks on one sample as before",
			d.Category, d.Confidence, d.Masked, d.Reason)
	}
}
