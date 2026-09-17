// SPDX-License-Identifier: Apache-2.0

package repo

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// gitRepo makes a temporary repository. It is a real one, because the tracked
// check is a real `git ls-files` and a fake would test the fake.
func gitRepo(t *testing.T) string {
	t.Helper()

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not on PATH, and the tracked check is a git subprocess")
	}
	dir := t.TempDir()
	// macOS puts a temp directory under /var, which is a symlink to /private/var.
	// Root resolves the real path through filepath.Abs on a path git also
	// resolved, so the two have to be compared as git sees them.
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatalf("resolving %s: %v", dir, err)
	}
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = resolved
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	run("init")
	run("config", "user.email", "test@example.invalid")
	run("config", "user.name", "test")
	return resolved
}

// Step 1: no repository. The secret is written anyway and the caller says so.
func TestNoRepository(t *testing.T) {
	dir := t.TempDir()
	st, err := Protect(filepath.Join(dir, "lazyslice.secret"), nil)
	if err != nil {
		t.Fatalf("Protect: %v", err)
	}
	if st.Root != "" {
		t.Errorf("Root = %q, want empty outside a repository", st.Root)
	}
	if !st.MayWriteSecret() {
		t.Error("outside a repository the secret is written and the header says it is unprotected")
	}
}

// Step 2: a writable .gitignore takes the entries, and only the missing ones.
func TestGitignoreTakesTheEntries(t *testing.T) {
	root := gitRepo(t)
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("bin/\n"), 0o600); err != nil {
		t.Fatalf("writing .gitignore: %v", err)
	}
	secret := filepath.Join(root, "lazyslice.secret")

	st, err := Protect(secret, nil)
	if err != nil {
		t.Fatalf("Protect: %v", err)
	}
	if st.Root != root {
		t.Errorf("Root = %q, want %q", st.Root, root)
	}
	if !st.GitignoreWritable || !st.MayWriteSecret() {
		t.Fatalf("the .gitignore is writable and Protect says %+v", st)
	}
	if !slices.Contains(st.Added, secretEntry) || !slices.Contains(st.Added, snapshotsEntry) {
		t.Errorf("Added = %v, want it to name %q and %q", st.Added, secretEntry, snapshotsEntry)
	}

	body, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	if err != nil {
		t.Fatalf("reading .gitignore: %v", err)
	}
	if !strings.Contains(string(body), "bin/") {
		t.Error("the existing entries were lost")
	}

	// A second call adds nothing: the entries are there.
	again, err := Protect(secret, nil)
	if err != nil {
		t.Fatalf("Protect: %v", err)
	}
	if len(again.Added) != 0 {
		t.Errorf("a second call added %v, want nothing", again.Added)
	}
}

// Step 3: no .gitignore at all. The file is not created, and the run uses an
// ephemeral key rather than writing an un-ignored one.
func TestAbsentGitignoreRefusesTheSecret(t *testing.T) {
	root := gitRepo(t)
	st, err := Protect(filepath.Join(root, "lazyslice.secret"), nil)
	if err != nil {
		t.Fatalf("Protect: %v", err)
	}
	if st.GitignoreWritable {
		t.Error("there is no .gitignore and Protect says it is writable")
	}
	if st.MayWriteSecret() {
		t.Error("inside a repository with no .gitignore the key must be ephemeral, never written")
	}
	if _, statErr := os.Stat(filepath.Join(root, ".gitignore")); !errors.Is(statErr, os.ErrNotExist) {
		t.Error("Protect created a .gitignore; it must not create a file the operator did not ask for")
	}
}

// Step 4: a tracked secret is exit 2 naming `git rm --cached`.
func TestTrackedSecretIsReported(t *testing.T) {
	root := gitRepo(t)
	secret := filepath.Join(root, "lazyslice.secret")
	if err := os.WriteFile(secret, []byte("00\n"), 0o600); err != nil {
		t.Fatalf("writing the secret: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(""), 0o600); err != nil {
		t.Fatalf("writing .gitignore: %v", err)
	}
	add := exec.Command("git", "add", "-f", "lazyslice.secret")
	add.Dir = root
	if out, err := add.CombinedOutput(); err != nil {
		t.Fatalf("git add: %v\n%s", err, out)
	}

	st, err := Protect(secret, nil)
	if err != nil {
		t.Fatalf("Protect: %v", err)
	}
	if !st.GitFound {
		t.Fatal("git is on PATH and Protect says it is not")
	}
	if len(st.Tracked) != 1 || !strings.HasSuffix(st.Tracked[0], "lazyslice.secret") {
		t.Errorf("Tracked = %v, want the secret", st.Tracked)
	}
	if got := RemoveFromIndex("lazyslice.secret"); got != "git rm --cached lazyslice.secret" {
		t.Errorf("RemoveFromIndex = %q", got)
	}
}

// An untracked file is not reported, which is the other half of the same check:
// a state that reported everything would refuse every first run.
func TestUntrackedSecretIsNotReported(t *testing.T) {
	root := gitRepo(t)
	secret := filepath.Join(root, "lazyslice.secret")
	if err := os.WriteFile(secret, []byte("00\n"), 0o600); err != nil {
		t.Fatalf("writing the secret: %v", err)
	}

	st, err := Protect(secret, nil)
	if err != nil {
		t.Fatalf("Protect: %v", err)
	}
	if len(st.Tracked) != 0 {
		t.Errorf("Tracked = %v, want nothing", st.Tracked)
	}
}

// The tracked check is made from a subdirectory with the relative path
// internal/core actually passes ("./lazyslice.secret", core.DefaultSecretFile).
//
// `git -C <root>` resolves a relative pathspec against the repository root, so
// the relative path meant one file to the caller and another to git: the check
// asked about <root>/lazyslice.secret, got exit 1, and reported a committed key
// in a subdirectory as untracked. That is a fail-open on THREAT_MODEL.md T6, and
// every other test here passes an absolute path, so none of them could see it.
func TestTrackedSecretIsFoundThroughARelativePath(t *testing.T) {
	root := gitRepo(t)
	sub := filepath.Join(root, "sub")
	if err := os.MkdirAll(sub, 0o750); err != nil {
		t.Fatalf("making %s: %v", sub, err)
	}
	if err := os.WriteFile(filepath.Join(sub, "lazyslice.secret"), []byte("00\n"), 0o600); err != nil {
		t.Fatalf("writing the secret: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(""), 0o600); err != nil {
		t.Fatalf("writing .gitignore: %v", err)
	}
	add := exec.Command("git", "add", "-f", "sub/lazyslice.secret")
	add.Dir = root
	if out, err := add.CombinedOutput(); err != nil {
		t.Fatalf("git add: %v\n%s", err, out)
	}

	t.Chdir(sub)
	st, err := Protect("./lazyslice.secret", nil)
	if err != nil {
		t.Fatalf("Protect: %v", err)
	}
	if !st.GitFound {
		t.Fatal("git is on PATH and Protect says it is not")
	}
	if len(st.Tracked) != 1 {
		t.Errorf("Tracked = %v, want ./lazyslice.secret: a committed key in a subdirectory "+
			"must not pass the check that exists to catch it", st.Tracked)
	}
}

// T-0251, the round 5 red team's still-leaking entry: a .gitignore that lists
// the secret and then negates it reads as "present" to a text search, and git
// itself reports the negation as the matching rule. Only a verified,
// non-negated match may set MayWriteSecret true.
func TestNegatedGitignoreEntryDoesNotProtect(t *testing.T) {
	root := gitRepo(t)
	if err := os.WriteFile(filepath.Join(root, ".gitignore"),
		[]byte("lazyslice.secret\n!lazyslice.secret\n"), 0o600); err != nil {
		t.Fatalf("writing .gitignore: %v", err)
	}
	secret := filepath.Join(root, "lazyslice.secret")

	st, err := Protect(secret, nil)
	if err != nil {
		t.Fatalf("Protect: %v", err)
	}
	if !st.GitFound {
		t.Fatal("git is on PATH and Protect says it is not")
	}
	if st.GitignoreWritable {
		t.Error("GitignoreWritable = true, want false: the negation is the matching rule")
	}
	if st.MayWriteSecret() {
		t.Error("MayWriteSecret() = true, want false: git would commit this file")
	}
	// lazyslice.secret is already present, in both forms, so appendMissing's
	// text search alone would have stopped right here and called the file
	// protected: only snapshots/ (unrelated to this negation) was missing.
	if slices.Contains(st.Added, secretEntry) {
		t.Errorf("Added = %v, want it not to add %q: it already reads as present", st.Added, secretEntry)
	}
}

// T-0251 round 6: negatedRule's colon-split misparsed <source> whenever it
// was itself an absolute path containing a colon — which git prints whenever
// the matching rule comes from core.excludesFile rather than an in-repo
// .gitignore, and which is the *only* shape a Windows source ever takes
// ("C:/Users/u/.gitignore_global"). parseCheckIgnoreZ replaces that split
// with `-z --stdin`'s NUL-separated fields, which need no such parsing.
func TestParseCheckIgnoreZNegation(t *testing.T) {
	tests := []struct {
		name       string
		output     string
		wantIgnore bool
		wantNegBy  string
	}{
		{
			name:       "plain in-repo source, not negated",
			output:     ".gitignore\x002\x00/lazyslice.secret\x00/repo/lazyslice.secret\x00",
			wantIgnore: true,
		},
		{
			name:       "plain in-repo source, negated",
			output:     ".gitignore\x002\x00!lazyslice.secret\x00/repo/lazyslice.secret\x00",
			wantIgnore: false,
			wantNegBy:  ".gitignore:2",
		},
		{
			name: "absolute source with a colon (core.excludesFile), negated",
			output: "/private/tmp/gitignoretest/glo:bal/ignore\x001\x00!lazyslice.secret\x00" +
				"/private/tmp/gitignoretest/lazyslice.secret\x00",
			wantIgnore: false,
			wantNegBy:  "/private/tmp/gitignoretest/glo:bal/ignore:1",
		},
		{
			name:       "windows-shaped source with a drive letter colon, negated",
			output:     "C:/Users/u/.gitignore_global\x003\x00!lazyslice.secret\x00C:/repo/lazyslice.secret\x00",
			wantIgnore: false,
			wantNegBy:  "C:/Users/u/.gitignore_global:3",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ignored, negatedBy, err := parseCheckIgnoreZ(tt.output)
			if err != nil {
				t.Fatalf("parseCheckIgnoreZ: %v", err)
			}
			if ignored != tt.wantIgnore {
				t.Errorf("ignored = %v, want %v", ignored, tt.wantIgnore)
			}
			if negatedBy != tt.wantNegBy {
				t.Errorf("negatedBy = %q, want %q", negatedBy, tt.wantNegBy)
			}
		})
	}
}

// TestNegatedGitignoreEntryWithColonSource reproduces T-0251 round 6 through
// Protect end to end: a .gitignore nested in a directory whose name contains
// a colon negates the secret entry, and — because a nested .gitignore beats
// the repository root's in git's own precedence — that negation, not the
// entry Protect appended at the root, is the rule that decides the outcome.
// Its <source> is therefore "sub:dir/.gitignore", exactly the shape that
// negatedRule's colon-split used to cut in half and silently read as
// "not negated" (secret.file.tracked never fired, the header claimed the key
// was protected, and it was committed anyway).
func TestNegatedGitignoreEntryWithColonSource(t *testing.T) {
	root := gitRepo(t)
	// A writable root .gitignore, so appendMissing has something to add the
	// unanchored `lazyslice.secret` entry to — the one that would otherwise
	// protect the file at any depth, including inside sub:dir below.
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), nil, 0o600); err != nil {
		t.Fatalf("writing root .gitignore: %v", err)
	}
	sub := filepath.Join(root, "sub:dir")
	if err := os.MkdirAll(sub, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sub, ".gitignore"), []byte("!lazyslice.secret\n"), 0o600); err != nil {
		t.Fatalf("writing nested .gitignore: %v", err)
	}
	secret := filepath.Join(sub, "lazyslice.secret")

	st, err := Protect(secret, nil)
	if err != nil {
		t.Fatalf("Protect: %v", err)
	}
	if !st.GitFound {
		t.Fatal("git is on PATH and Protect says it is not")
	}
	if st.GitignoreWritable {
		t.Error("GitignoreWritable = true, want false: sub:dir/.gitignore negates the entry Protect appended at the root")
	}
	if st.MayWriteSecret() {
		t.Error("MayWriteSecret() = true, want false: git would commit this file")
	}
	wantNegBy := filepath.Join("sub:dir", ".gitignore") + ":1"
	if st.GitignoreNegatedBy != wantNegBy {
		t.Errorf("GitignoreNegatedBy = %q, want %q", st.GitignoreNegatedBy, wantNegBy)
	}
}

// With git absent from PATH the entry can be verified by nobody, so the same
// fallback the "absent or unwritable .gitignore" case uses applies here too,
// even though .gitignore holds the (unverified) entry.
func TestGitAbsentFallsBackToEphemeral(t *testing.T) {
	root := gitRepo(t)
	if err := os.WriteFile(filepath.Join(root, ".gitignore"),
		[]byte("lazyslice.secret\n"), 0o600); err != nil {
		t.Fatalf("writing .gitignore: %v", err)
	}
	secret := filepath.Join(root, "lazyslice.secret")

	t.Setenv("PATH", "")
	st, err := Protect(secret, nil)
	if err != nil {
		t.Fatalf("Protect: %v", err)
	}
	if st.GitFound {
		t.Fatal("PATH was cleared and Protect still found git")
	}
	if st.GitignoreWritable {
		t.Error("GitignoreWritable = true, want false: it cannot be verified with no git on PATH")
	}
	if st.MayWriteSecret() {
		t.Error("MayWriteSecret() = true, want false: an unverified entry must fall back to the ephemeral key")
	}
}

func TestRootFindsTheRepository(t *testing.T) {
	root := gitRepo(t)
	deep := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(deep, 0o750); err != nil {
		t.Fatalf("making %s: %v", deep, err)
	}
	got, err := Root(deep)
	if err != nil || got != root {
		t.Errorf("Root(%q) = %q, %v, want %q", deep, got, err, root)
	}

	if _, err := Root(t.TempDir()); !errors.Is(err, ErrNoRepository) {
		t.Errorf("Root outside a repository = %v, want ErrNoRepository", err)
	}
}
