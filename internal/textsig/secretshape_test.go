// SPDX-License-Identifier: Apache-2.0

package textsig

import (
	"math/rand/v2"
	"strings"
	"testing"
)

// The shapes LooksSecret no longer reads as a credential (tracker T-0315,
// dogfood session 1). Every value here cleared the entropy check before the
// task — the test asserts that first, through looksSecretEntropyOnly, so a
// value that was never a false positive cannot stand in for one — and each is
// the shape of a column the dogfood run masked to "$lazyslice$invalid".
func TestLooksSecretRefusesApplicationShapes(t *testing.T) {
	t.Parallel()

	cases := []struct {
		shape string
		guard func(string) bool
		vals  []string
	}{
		{"file name", impersonalFileName, []string{
			// exported reports, logos, media files, screenshots, software
			// releases, webcam shots
			"export-2024-03-01T101512Z.csv",
			"Quarterly_Report_2024Q1.xlsx",
			"logo_8f3a2c91d7e4b6a0.png",
			"uploads/9f8b1c2d3e4a5b6c7d8e.png",
			"media/2024/03/clip_A7x92Kq.mp4",
			"Screenshot_2024-03-01_10-15-12.png",
			"Setup-4.2.1-x64.exe",
			"lazyslice_0.2.0_darwin_arm64.tar.gz",
			"webcam_20240301_101512_cam2.JPG",
			"IMG_20240301_101512.jpeg",
		}},
		{"hex digest", fixedHexDigest, []string{
			"d41d8cd98f00b204e9800998ecf8427e",                                 // md5
			"D41D8CD98F00B204E9800998ECF8427E",                                 // md5, upper
			"da39a3ee5e6b4b0d3255bfef95601890afd80709",                         // sha1
			"e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", // sha256
			"md5:d41d8cd98f00b204e9800998ecf8427e",
			"sha256=e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
			"SHA-1:da39a3ee5e6b4b0d3255bfef95601890afd80709",
			"43:51:43:a1:b5:fc:8b:b7:0a:3a:a9:b1:0f:66:73:a8", // an MD5 fingerprint
		}},
		{"namespaced identifier", namespacedIdentifier, []string{
			// Rails STI and polymorphic type columns, a formatter column
			"Billing::Invoices::PdfExport",
			"Admin::ScheduledReport",
			"Api::V2::UsersController",
			"Notifications::Webhook2Delivery",
			"Logger::JsonFormatter",
			"Integrations::Slack::OAuth2Callback",
			"com.example.reports.CsvFormatter",
			"app.formatters.JsonLines2",
		}},
		{"environment variable name", envVarName, []string{
			"AWS_S3_BUCKET_V2",
			"SENTRY_DSN_EU2_PROD",
			"OAUTH2_CLIENT_ID_PROD",
		}},
	}
	for _, c := range cases {
		for _, v := range c.vals {
			if !looksSecretEntropyOnly(v) {
				t.Errorf("%s %q: the entropy check never read it as a secret, so it pins nothing", c.shape, v)
				continue
			}
			if !c.guard(v) {
				t.Errorf("%s %q: the guard does not recognise it", c.shape, v)
			}
			if LooksSecret(v) {
				t.Errorf("LooksSecret(%q) = true: a %s is not a credential (T-0315)", v, c.shape)
			}
		}
	}
}

// The other side of the line: every secret shape the dogfood run caught must
// stay caught, and so must the neighbours of each guard's shape that are
// secrets or could be. This is the half of T-0315 a narrowing gets wrong.
func TestLooksSecretStillReadsSecrets(t *testing.T) {
	t.Parallel()

	secrets := []string{
		// API keys and tokens
		"zq_0eC54XbKozZVkuvhF8mog2sk",
		"zq_A0o0jJ2bX0xQ1yW9xJ4dM4sI7cO6wC5hA0oQ",
		"zq_8736-47766-Of6dJ4sI5nP0eG1mY2rT1kX8",
		"qXxriUKvvQCBT/X9QDTGN/bWqMspPAIOFWQXXRIU",
		"zq_APQGMFUB5UPYBCNO",
		"Qm8pZ3xR7tN1vW4yB6cD9fG2",      // has_secure_token, base58
		"q4r7uXzN-8vK_2mL9pQ3sT6w",      // Devise friendly_token
		"rL0Y20zC+Fzt72VPzMSk2A==",      // base64 of 16 random bytes
		"zq_m3HdP6YGHq5tG0cJ7fV8qA2fV9", // an OAuth access token with a dot
		// JWTs and dotted bot tokens
		"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.dozjgNryP4J3jVmNHl0w5N_XgL0n3I9PlFUP0THsR8U",
		"zq_0MlDkZEeaXIhbMRS3GzA0.Jf8KEJ.ArCny0MLD2zEea5i1Bm3sb2zAUj",
		"zq_cjrNlSWJReQV2rju8m7k3P.ZiN2bZGQs5CJrNl-48jreQ5wRjuqmJkvpziNe-5z2qs",
		"v4.local.Qm8pZ3xR7tN1vW4yB6cD9fG2hJ8kL0sD5",
		// password hashes
		"$2a$12$R9h/cIPz0gi.URNNX3kh2OPST9/PgBkqquzi.Ss7KIUgO2t0jWMUW",
		"$argon2id$v=19$m=65536,t=3,p=4$c29tZXNhbHQ$RdescudvJCsgt3ub+b+dWRWJTmaaJObG",
		"pbkdf2_sha256$600000$Qm8pZ3xR7tN1$Y2xq3r7uXzN8vK2mL9pQ3sT6w=",
		// hex of every length that is not a digest's
		"9f8b1c2d3e4a5b6c7d8e9f0a1b2c3d4e5f6a7b8c9d0e1f2a", // 48
		"9f8b1c2d3e4a5b6c7d8e9f0a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d", // 127
		"9F8b1c2d3e4a5b6c7d8e9f0a1b2c3d4e",        // 32, mixed case
		"sha256:9f8b1c2d3e4a5b6c7d8e9f0a1b2c3d4e", // wrong length for sha256
		// a private key's file name is not a spared file name
		"prod-signing-2024-Qm8pZ3.key",
		// nor is a document named after a person: the rails-activestorage
		// truth set's one labelled-personal filename shape
		"aoife-bilal-passport-3.pdf",
		"uploads/grace-hopper/cv_2024Q1.pdf",
	}
	for _, s := range secrets {
		if !LooksSecret(s) {
			t.Errorf("LooksSecret(%q) = false: T-0315 must not narrow the secrets validator past its four shapes", s)
		}
	}

	// Values a guard's shape takes that carry a person, or could: each was
	// read by the entropy check before T-0315 (asserted first) and still is.
	// A dotted handle is identifier words joined by a dot, an upper-case
	// reference built from a name is an environment variable's shape, and a
	// file name joined by underscores splits into identifier words at its
	// extension dot, so namespacedIdentifier must not spare what
	// impersonalFileName leaves alone. An all-lower dotted module path is the
	// stated cost of keeping the handles.
	personal := []string{
		"katherine.johnson84",
		"margaret.okafor1990",
		"Margaret.Okafor1990",
		"Katherine.McDonald84",
		"DUBLIN_GRACE_HOPPER_1906",
		"DUBLIN_AOIFE_BYRNE_1987", // neither name in the dictionary: the year keeps it
		"GRACE_HOPPER_KY_06",
		"hopper_grace_1906_birth_cert.pdf",
		"grace_hopper_passport_2024.pdf",
		"billing.invoice_mailer.v2",
	}
	for _, s := range personal {
		if !looksSecretEntropyOnly(s) {
			t.Errorf("%q: the entropy check never read it as a secret, so it pins nothing", s)
			continue
		}
		if !LooksSecret(s) {
			t.Errorf("LooksSecret(%q) = false: a guard spared a value that may carry a person (T-0315 review)", s)
		}
	}

	// Each guard on its own, over the lookalikes it must refuse.
	notFile := []string{"api.example.com", "zq_0eC54XbKozZVkuvhF8mog2sk", "report.", ".png", "a/.png", "backup.2024"}
	for _, s := range notFile {
		if fileNameShape(s) {
			t.Errorf("fileNameShape(%q) = true", s)
		}
	}
	notDigest := []string{
		"9f8b1c2d3e4a5b6c7d8e9f0a1b2c3d4",   // 31
		"9f8b1c2d3e4a5b6c7d8e9f0a1b2c3d4e5", // 33
		"9F8b1c2d3e4a5b6c7d8e9f0a1b2c3d4e",  // mixed case
		"md4:d41d8cd98f00b204e9800998ecf8427e",
		"43:51:43:a1:b5:fc:8b:b7:0a:3a:a9:b1:0f:66:73",    // 15 pairs
		"43:51:43:A1:b5:fc:8b:b7:0a:3a:a9:b1:0f:66:73:a8", // mixed case
		"g41d8cd98f00b204e9800998ecf8427e",
	}
	for _, s := range notDigest {
		if fixedHexDigest(s) {
			t.Errorf("fixedHexDigest(%q) = true", s)
		}
	}
	notIdent := []string{
		"Billing",                  // one segment
		"Billing::",                // empty segment
		".Billing.Invoice",         // leading dot
		"Billing:Invoice",          // a single colon is not a namespace
		"Billing::Invoice-Export",  // a hyphen is not an identifier character
		"2fa.Codes",                // a segment beginning with a digit
		"MTk4NjIy.Cl2FMQ",          // a lone lower-case letter after a capital run
		"a4b.Invoice",              // a digit before a lower-case letter
		"VsFkVKSO.SCRguITE",        // capitals are half the letters, two acronyms
		"Billing.Invoice12345Pdfs", // five digits
	}
	for _, s := range notIdent {
		if namespacedIdentifier(s) {
			t.Errorf("namespacedIdentifier(%q) = true", s)
		}
	}
	notEnv := []string{"AWS", "AWS__KEY", "_AWS_KEY", "Aws_Key_2", "AWS_KEY-2", "2FA_CODE", "X7K2P_9QMZ4_R8T3W", "SMTP_PORT_2525", "BYRNE_AOIFE_19870412"}
	for _, s := range notEnv {
		if envVarName(s) {
			t.Errorf("envVarName(%q) = true", s)
		}
	}
}

// TestLooksSecretStillReadsRandomTokens measures the guards over generated
// populations of the random shapes secrets are issued in. A guard that
// accepted a random run would drop a real token column to `none`, so the
// bound for every ungrouped shape and for a JWT is zero; a dotted shape of
// short random segments, and capitals and digits grouped by underscores
// (envVarName's lookalike), neither of which an issuer this test knows of
// uses, are bounded well under the one value in five that would move a column
// off validatorThreshold's 80%.
func TestLooksSecretStillReadsRandomTokens(t *testing.T) {
	t.Parallel()

	r := rand.New(rand.NewPCG(315, 2026))
	const b64u = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"
	const b62 = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	const hexl = "0123456789abcdef"
	const b36u = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	gen := func(alpha string, n int) string {
		var b strings.Builder
		for i := 0; i < n; i++ {
			b.WriteByte(alpha[r.IntN(len(alpha))])
		}
		return b.String()
	}
	pops := []struct {
		name  string
		gen   func() string
		bound float64
	}{
		{"base62, 16 to 64", func() string { return gen(b62, 16+r.IntN(49)) }, 0},
		{"base64url, 16 to 64", func() string { return gen(b64u, 16+r.IntN(49)) }, 0},
		{"JWT", func() string {
			return "eyJ" + gen(b64u, 17+r.IntN(40)) + "." + gen(b64u, 20+r.IntN(80)) + "." + gen(b64u, 43)
		}, 0},
		{"hex, 48", func() string { return gen(hexl, 48) }, 0},
		{"hex, 128", func() string { return gen(hexl, 128) }, 0},
		{"upper, 5_5_5_5", func() string {
			return gen(b36u, 5) + "_" + gen(b36u, 5) + "_" + gen(b36u, 5) + "_" + gen(b36u, 5)
		}, 0.02},
		{"bot token, 24.6.27", func() string { return gen(b62, 24) + "." + gen(b62, 6) + "." + gen(b62, 27) }, 0.001},
		{"dotted, 8.8.8", func() string { return gen(b62, 8) + "." + gen(b62, 8) + "." + gen(b62, 8) }, 0.02},
		{"dotted, 8.8", func() string { return gen(b62, 8) + "." + gen(b62, 8) }, 0.05},
	}
	const n = 20000
	for _, p := range pops {
		guarded := 0
		for i := 0; i < n; i++ {
			s := p.gen()
			if looksSecretEntropyOnly(s) && !LooksSecret(s) {
				guarded++
			}
		}
		rate := float64(guarded) / n
		t.Logf("%-22s %5d/%d guarded (%.4f)", p.name, guarded, n, rate)
		if rate > p.bound {
			t.Errorf("%s: %d/%d random tokens dropped out of LooksSecret, bound %.3f", p.name, guarded, n, p.bound)
		}
	}
}

// looksSecretEntropyOnly is LooksSecret as it stood before T-0315, kept here
// so each test above can prove a value was a false positive before it proves
// the value is not one now.
func looksSecretEntropyOnly(s string) bool {
	s = strings.TrimSpace(s)
	if len(s) < 16 || len(s) > 512 {
		return false
	}
	if ValidUUID(s) || ValidURL(s) || strings.ContainsAny(s, " \t\n@") {
		return false
	}
	if hexRE.MatchString(s) {
		return len(s) >= 32 && shannon(s) >= 3.0
	}
	classes := 0
	for _, in := range []func(rune) bool{
		func(r rune) bool { return r >= 'a' && r <= 'z' },
		func(r rune) bool { return r >= 'A' && r <= 'Z' },
		func(r rune) bool { return r >= '0' && r <= '9' },
	} {
		for _, r := range s {
			if in(r) {
				classes++
				break
			}
		}
	}
	return classes >= 2 && shannon(s) >= 3.2
}
