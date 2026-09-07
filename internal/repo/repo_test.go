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
