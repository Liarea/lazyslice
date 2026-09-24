// SPDX-License-Identifier: Apache-2.0

package textsig

import (
	"strings"
)

// The shapes LooksSecret refuses before it measures entropy (tracker T-0315,
// dogfood session 1). Each is a value an application writes and reads back by
// its own meaning — a stored file's name, a content digest, a class or module
// name, an environment variable's name — and each clears every guard the
// entropy check had: sixteen characters or more, no space, no "@", two
// character classes, entropy at or above the floor. On a production Rails
// schema that read six filename columns, four MD5 columns, two
// file-fingerprint columns and three single-table-inheritance `type` columns
// as credentials, masked each to the fixed "$lazyslice$invalid", and the
// application raised on every row whose class name had become that literal.
//
// None of them is a validator and none is exported: they are guards inside
// LooksSecret, the same kind of exclusion ValidUUID and ValidURL already are
// there, and both of this package's callers get them by calling LooksSecret.
// Each is written to refuse the shapes real secrets are issued in —
// base64/base64url runs, JWTs, prefixed API keys, bcrypt and argon2 hashes, hex
// tokens of any length other than a digest's — and secretshape_test.go holds those
// on the secret side of the line, including a generated population of random
// tokens (TestLooksSecretStillReadsRandomTokens).

// fileExtensions are the extensions fileNameShape accepts, lower case. The
// list is closed on purpose: a generic "a dot and one to five alphanumerics"
// also takes every hostname ("api.example.com") and any token a service
// happens to write with a dotted suffix, and a value that is neither a file
// nor a secret is better left to the entropy check than spared by it — when in
// doubt, mask it. It covers what the dogfood columns held (exported reports,
// logos, media files, screenshots, software releases, webcam shots) and their
// obvious neighbours. "key", "pem", "p12" and "pfx" are deliberately absent: a
// column of those names is a column of private keys' file names, and one a
// reviewer should see masked.
var fileExtensions = map[string]bool{
	// images
	"png": true, "jpg": true, "jpeg": true, "jpe": true, "gif": true, "webp": true,
	"bmp": true, "tif": true, "tiff": true, "svg": true, "ico": true, "heic": true,
	"heif": true, "avif": true, "psd": true, "raw": true, "dng": true, "cr2": true,
	"nef": true, "eps": true,
	// audio and video
	"mp3": true, "wav": true, "m4a": true, "aac": true, "ogg": true, "oga": true,
	"opus": true, "flac": true, "wma": true, "aiff": true, "mid": true, "midi": true,
	"mp4": true, "m4v": true, "mov": true, "avi": true, "mkv": true, "webm": true,
	"wmv": true, "flv": true, "mpg": true, "mpeg": true, "3gp": true, "ogv": true,
	"m3u8": true, "h264": true,
	// documents, exports and data files
	"pdf": true, "doc": true, "docx": true, "xls": true, "xlsx": true, "xlsm": true,
	"ppt": true, "pptx": true, "odt": true, "ods": true, "odp": true, "rtf": true,
	"txt": true, "csv": true, "tsv": true, "json": true, "ndjson": true, "xml": true,
	"yaml": true, "yml": true, "html": true, "htm": true, "md": true, "log": true,
	"ics": true, "epub": true, "pages": true, "numbers": true,
	"sql": true, "parquet": true,
	// archives
	"zip": true, "gz": true, "tgz": true, "tar": true, "bz2": true, "xz": true,
	"7z": true, "rar": true, "zst": true,
	// software releases and packages
	"exe": true, "msi": true, "msix": true, "dmg": true, "pkg": true, "deb": true,
	"rpm": true, "apk": true, "aab": true, "ipa": true, "appimage": true, "jar": true,
	"war": true, "bin": true, "img": true, "iso": true, "whl": true, "gem": true,
	"nupkg": true, "snap": true, "hex": true, "ota": true,
	// fonts and web assets
	"ttf": true, "otf": true, "woff": true, "woff2": true, "css": true, "js": true,
}

// fileNameShape reports whether s ends in a dot and one of fileExtensions,
// case-insensitively, after a non-empty stem that does not itself end in a
// dot or a path separator: "IMG_20240301_101512.JPG", "report-2024-q1.xlsx",
// "uploads/9f8b1c2d3e4a5b6c7d8e.png", "Setup-4.2.1-x64.exe". The stem is not
// read, because a stored upload is often named by a random token and that is
// exactly the column this exists for: the value is the file's name, and the
// token in it names a file, not an account.
func fileNameShape(s string) bool {
	dot := strings.LastIndexByte(s, '.')
	if dot < 1 || dot == len(s)-1 {
		return false
	}
	if c := s[dot-1]; c == '.' || c == '/' || c == '\\' {
		return false
	}
	return fileExtensions[strings.ToLower(s[dot+1:])]
}

// sparedShape is the T-0315 guard LooksSecret applies before entropy. A file
// name answers only to the file-name rule: its stem splits at the extension
// dot into identifier words ("hopper_grace_1906_birth_cert.pdf" is two), so
// letting namespacedIdentifier read it too would spare the very file names
// impersonalFileName leaves with the entropy check for carrying a name.
func sparedShape(s string) bool {
	if _, ok := FileNameStem(s); ok {
		return impersonalFileName(s)
	}
	return fixedHexDigest(s) || namespacedIdentifier(s) || envVarName(s)
}

// impersonalFileName is the guard LooksSecret applies: fileNameShape, unless
// the name before the extension carries a word from the name dictionary
// (Dict.ContainsName). "aoife-byrne-passport-3.pdf" is a document about a
// person, and the entropy check was the one validator that read a column of
// them (rails-activestorage's active_storage_blobs.filename, a true positive
// in docs/TORTURE.md's truth set); sparing it would copy the name verbatim.
// It stays with the entropy check, which reads it as it did before T-0315 —
// the direction THREAT_MODEL.md T1 wants a miss to fall in.
//
// That answers for one value and not for a column: a column of file names
// only some of which carry a name the dictionary holds (mastodon's
// `photo-<username>-<n>.jpg`, about half) would fall under the validator
// threshold and be copied, the other half's names with it. What a column
// needs is the callers' question, because a share is a threshold and this
// package holds none: FileNameStem is what they ask it with.
func impersonalFileName(s string) bool {
	stem, ok := FileNameStem(s)
	return ok && !Dictionary().ContainsName(stem)
}

// FileNameStem reports whether s is a file's name as LooksSecret's file-name
// guard reads one (a known extension after a non-empty stem; fileNameShape),
// and returns the part before the extension. Like the identifier shapes in
// identifier.go it is not a validator and nothing masks a column because a
// value matches it: internal/classify and internal/verify read it to count
// how many of a column's file names carry a word from the name dictionary,
// and to put the column's name-free file names back under the entropy check
// when enough of them do (T-0315).
func FileNameStem(s string) (string, bool) {
	s = strings.TrimSpace(s)
	if !fileNameShape(s) {
		return "", false
	}
	return s[:strings.LastIndexByte(s, '.')], true
}

// digestHexLen is the hex length of each digest fixedHexDigest accepts under
// an algorithm prefix: MD5, SHA-1 and SHA-256, the three lengths T-0315 names.
var digestHexLen = map[string]int{"md5": 32, "sha1": 40, "sha-1": 40, "sha256": 64, "sha-256": 64}

// fixedHexDigest reports whether s is a content digest written in one of the
// three ways applications store one:
//
//   - bare hex of exactly 32, 40 or 64 characters (MD5, SHA-1, SHA-256), in one
//     case — a digest encoder writes all lower or all upper, never both;
//   - the same under an algorithm prefix and ":", "=" or "-" ("sha256:…",
//     "md5=…"), where the hex length must be the named algorithm's own;
//   - colon-separated byte pairs of 16, 20 or 32 bytes, the form an SSH or TLS
//     fingerprint is printed in ("43:51:43:a1:b5:fc:…").
//
// Every other hex length is left to the entropy check, so a 48-, 96- or
// 128-character hex token stays a secret. A 32- or 64-character hex *token*
// (SecureRandom.hex's default output) cannot be told from a digest by its
// value, and is a credential here only when its column's name says so — the
// rule pack's credential pattern, which this does not touch; that residual is
// THREAT_MODEL.md T1's T-0315 amendment.
func fixedHexDigest(s string) bool {
	if bareDigestHex(s) {
		return true
	}
	for algo, n := range digestHexLen {
		if len(s) == len(algo)+1+n && strings.EqualFold(s[:len(algo)], algo) &&
			strings.IndexByte(":=-", s[len(algo)]) >= 0 {
			return bareDigestHex(s[len(algo)+1:])
		}
	}
	return colonHexPairs(s)
}

func bareDigestHex(s string) bool {
	switch len(s) {
	case 32, 40, 64:
	default:
		return false
	}
	lower, upper := false, false
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= '0' && c <= '9':
		case c >= 'a' && c <= 'f':
			lower = true
		case c >= 'A' && c <= 'F':
			upper = true
		default:
			return false
		}
	}
	return !lower || !upper
}

func colonHexPairs(s string) bool {
	parts := strings.Split(s, ":")
	switch len(parts) {
	case 16, 20, 32:
	default:
		return false
	}
	lower, upper := false, false
	for _, p := range parts {
		if len(p) != 2 {
			return false
		}
		for i := 0; i < 2; i++ {
			c := p[i]
			switch {
			case c >= '0' && c <= '9':
			case c >= 'a' && c <= 'f':
				lower = true
			case c >= 'A' && c <= 'F':
				upper = true
			default:
				return false
			}
		}
	}
	return !lower || !upper
}

// namespacedIdentifier reports whether s is two or more identifier words
// joined by "::" or ".": a Ruby constant path ("Billing::Invoices::PdfExport"),
// a Java or Python dotted class name ("com.example.reports.CsvFormatter",
// "app.formatters.JsonLines2"), a Rails STI or polymorphic type
// ("Admin::ScheduledReport"). Each segment must be identifierWord's shape.
//
// A dotted value is spared only when a segment is written in camelCase
// (camelHump) and no word of it is in the name dictionary. A dotted handle —
// "katherine.johnson84", "Margaret.Okafor1990", a user name an application
// stores as an external reference — is also identifier words joined by a dot,
// and before this guard the entropy check was the one validator that read a
// column of them; no person writes their handle in camelCase, and one who
// does ("Katherine.McDonald84") is still caught by the given name. An
// all-lower dotted module path ("billing.invoice_mailer.v2") is the cost: it
// stays with the entropy check, which masks it as it did before T-0315 — the
// direction THREAT_MODEL.md T1 wants a miss to fall in. A "::" path is
// spared as it stands: nobody writes a person's name with one.
//
// The segment test is where the secrets are kept out. A JWT and a Discord
// bot token are also dotted runs of letters and digits, and their segments
// are random base64url: identifierWord refuses a segment in which a lone
// lower-case letter sits between two other letters or digits ("MTk4NjIy…" has
// "Tk4"), which a written identifier almost never has and a random run of
// any length almost always does. TestLooksSecretStillReadsRandomTokens
// measures that over a generated population.
func namespacedIdentifier(s string) bool {
	segs := strings.Split(strings.ReplaceAll(s, "::", "."), ".")
	if len(segs) < 2 {
		return false
	}
	humped := false
	for _, seg := range segs {
		// An empty segment (a leading, trailing or doubled separator) is not
		// a namespace, and identifierWord refuses it.
		if !identifierWord(seg) {
			return false
		}
		humped = humped || camelHump(seg)
	}
	if strings.Contains(s, "::") {
		return true
	}
	return humped && !Dictionary().ContainsName(s)
}

// camelHump reports whether seg has a word boundary inside it written the
// camelCase way: a capital after a lower-case letter or a digit ("CsvFormatter",
// "OAuth2Callback"), or a capital that ends a run of capitals and begins a
// word ("XMLParser"). A capitalised word alone ("Johnson") has none.
func camelHump(seg string) bool {
	isUpper := func(c byte) bool { return c >= 'A' && c <= 'Z' }
	isLower := func(c byte) bool { return c >= 'a' && c <= 'z' }
	for i := 1; i < len(seg); i++ {
		if !isUpper(seg[i]) {
			continue
		}
		p := seg[i-1]
		if isLower(p) || (p >= '0' && p <= '9') {
			return true
		}
		if isUpper(p) && i+1 < len(seg) && isLower(seg[i+1]) {
			return true
		}
	}
	return false
}

// envVarName reports whether s is an environment variable's name written the
// conventional way: upper-case words joined by underscores, beginning with a
// letter, at least two words ("AWS_S3_BUCKET_V2", "SENTRY_DSN_EU2_PROD").
// Without a digit such a name is one character class and never reached the
// entropy floor; with one, it did.
//
// Each word is capitals followed by at most two digits, or one or two digits
// alone ("S3", "OAUTH2", "EU2", "V10"), never a digit followed by a capital:
// that is how a person numbers a word, and it refuses a random run of
// capitals and digits grouped by underscores ("X7K2P_9QMZ4"), which is how a
// recovery code or a licence key could be written.
// TestLooksSecretStillReadsRandomTokens measures it.
//
// Two things keep a person out of it. An upper-case reference built from a
// name and a year or a number ("DUBLIN_GRACE_HOPPER_1906",
// "BYRNE_AOIFE_19870412") has the same shape, and the digits are what brought
// it to the entropy floor: a run of three digits or more is a year, a date or
// a person's number far more often than part of a variable's name, so it is
// not spared. And a name with a word from the name dictionary in it is not
// spared either. Either way the entropy check reads it as it did before
// T-0315.
func envVarName(s string) bool {
	words := strings.Split(s, "_")
	if len(words) < 2 || s[0] < 'A' || s[0] > 'Z' {
		return false
	}
	for _, w := range words {
		if w == "" {
			return false
		}
		digit, digits := false, 0
		for i := 0; i < len(w); i++ {
			c := w[i]
			switch {
			case c >= '0' && c <= '9':
				digit = true
				if digits++; digits > 2 {
					return false
				}
			case c >= 'A' && c <= 'Z':
				if digit {
					return false
				}
			default:
				return false
			}
		}
	}
	return !Dictionary().ContainsName(s)
}

// identifierWord reports whether seg is one identifier as a person writes
// one: a letter or underscore first, then letters, digits and underscores, at
// most 64 characters and four digits, with at least one letter, and no
// lower-case letter standing alone where a written identifier never puts one.
//
// A lone lower-case letter is one with no lower-case letter on either side.
// It is allowed in three places only: first in the segment or after an
// underscore ("v2", "iOS", "x86_64", "a_b"), and after a capital that itself
// begins a word ("UserId", "OkResponse", "DbExporter": the capital follows a
// lower-case letter, a digit, an underscore or the segment's start). Anywhere
// else — after a digit ("a4b"), or after a capital that follows another
// capital ("MTk4", "IPv4") — it is the mark of a random run, which is what a
// base64url segment of a JWT or a bot token is.
//
// It errs towards refusing: "IPv4Address" and "HTTPsClient" are identifiers it
// does not accept, and a value built from one stays with the entropy check,
// which masks it — the direction T1 wants a miss to fall in.
func identifierWord(seg string) bool {
	if seg == "" || len(seg) > 64 {
		return false
	}
	if c := seg[0]; c != '_' && !isASCIILetter(c) {
		return false
	}
	lower := func(i int) bool { return i >= 0 && i < len(seg) && seg[i] >= 'a' && seg[i] <= 'z' }
	upper := func(i int) bool { return i >= 0 && i < len(seg) && seg[i] >= 'A' && seg[i] <= 'Z' }
	digits, letters, uppers, acronyms := 0, 0, 0, 0
	for i := 0; i < len(seg); i++ {
		c := seg[i]
		switch {
		case lower(i):
			letters++
			if lower(i-1) || lower(i+1) || i == 0 || seg[i-1] == '_' {
				continue
			}
			// Lone, after a capital or a digit: allowed only after a capital
			// that begins a word.
			if !upper(i-1) || (i-1 > 0 && upper(i-2)) {
				return false
			}
		case upper(i):
			letters++
			uppers++
			if upper(i+1) && !upper(i-1) {
				acronyms++
			}
		case c >= '0' && c <= '9':
			digits++
			if lower(i + 1) {
				return false
			}
		case c == '_':
		default:
			return false
		}
	}
	if letters == 0 || digits > 4 {
		return false
	}
	if uppers == letters || uppers == 0 {
		// One case: snake_case or SCREAMING_SNAKE, with nothing further to
		// read in the case pattern.
		return true
	}
	// Mixed case: camelCase or PascalCase, whose capitals begin words, so
	// fewer than half of its letters are capitals and at most one run of
	// capitals is an acronym ("XMLHttpRequest", "IOError").
	return 2*uppers < letters && acronyms <= 1
}

func isASCIILetter(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}
