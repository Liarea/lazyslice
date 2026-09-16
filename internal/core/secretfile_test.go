// SPDX-License-Identifier: Apache-2.0

package core

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// The 2026-09-15 red team against ARCHITECTURE.md §9 "The repository" and
// THREAT_MODEL.md T6: the masking key is A4, and both of these put it
// somewhere the repository rules do not reach while the run said nothing.
func TestSecretFileIsRefusedWhenItIsASymlink(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "Dropbox"), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	secret := filepath.Join(dir, "lazyslice.secret")
	if err := os.Symlink(filepath.Join(dir, "Dropbox", "leaked.key"), secret); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	r := &run{req: Request{SecretFile: secret}}

	err := r.checkSecretFile()
	var stop *Stop
	if !errors.As(err, &stop) {
		t.Fatalf("checkSecretFile returned %v, want a *Stop; the key would be written outside the repository", err)
	}
	if stop.Code != CodeSecretSymlink || stop.Exit != exitCredential {
		t.Fatalf("refused with %s exit %d, want %s exit %d",
			stop.Code, stop.Exit, CodeSecretSymlink, exitCredential)
	}
	if _, statErr := os.Stat(filepath.Join(dir, "Dropbox", "leaked.key")); statErr == nil {
		t.Error("the run wrote through the link anyway")
	}
}

func TestSecretFileIsRefusedWhenOthersCanReadIt(t *testing.T) {
	dir := t.TempDir()
	secret := filepath.Join(dir, "lazyslice.secret")
	if err := os.WriteFile(secret, []byte("00\n"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.Chmod(secret, 0o644); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	r := &run{req: Request{SecretFile: secret}}

	err := r.checkSecretFile()
	var stop *Stop
	if !errors.As(err, &stop) {
		t.Fatalf("checkSecretFile returned %v, want a *Stop; THREAT_MODEL.md T6 promises 0600", err)
	}
	if stop.Code != CodeSecretPermissive || stop.Exit != exitCredential {
		t.Fatalf("refused with %s exit %d, want %s exit %d",
			stop.Code, stop.Exit, CodeSecretPermissive, exitCredential)
	}
	if stop.Args["statement"] != "chmod 600 "+secret {
		t.Errorf("the refusal names %q, want the chmod to run", stop.Args["statement"])
	}
}

// R2-14 (THREAT_MODEL.md T6): a symlinked *parent directory* is invisible to
// the direct os.Lstat check above when the secret file itself does not exist
// yet — `ln -s ../Dropbox cloudkeys` followed by `--secret-file
// ./cloudkeys/lazyslice.secret` on a first run made os.Lstat return
// ErrNotExist, and both existing checks passed, while resolveKey would go on
// to write the key through the link and repo.Protect would add a .gitignore
// entry for a path that holds nothing.
func TestSecretFileIsRefusedWhenAParentDirectoryIsASymlink(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "repo"), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "Dropbox"), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.Mkdir(filepath.Join(dir, "repo", ".git"), 0o700); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
	if err := os.Symlink(filepath.Join(dir, "Dropbox"), filepath.Join(dir, "repo", "cloudkeys")); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	secret := filepath.Join(dir, "repo", "cloudkeys", "lazyslice.secret")
	r := &run{req: Request{SecretFile: secret, Workdir: filepath.Join(dir, "repo")}}

	err := r.checkSecretFile()
	var stop *Stop
	if !errors.As(err, &stop) {
		t.Fatalf("checkSecretFile returned %v, want a *Stop; the key would be written outside the repository", err)
	}
	if stop.Code != CodeSecretSymlink || stop.Exit != exitCredential {
		t.Fatalf("refused with %s exit %d, want %s exit %d",
			stop.Code, stop.Exit, CodeSecretSymlink, exitCredential)
	}
	if _, statErr := os.Stat(filepath.Join(dir, "Dropbox", "lazyslice.secret")); statErr == nil {
		t.Error("the run wrote through the link anyway")
	}
}

// R2-14, not closed (2026-09-16 round 2 red team): checkSecretParentSymlink
// used to resolve its own repo root from filepath.Dir(SecretFile), which
// walks *through* the symlinked component it is trying to catch.
// repo.Root's ancestor search Lstats ".git" inside each candidate, and Lstat
// follows every intermediate path element — so when the symlink's *target*
// is itself a git repository (a cloud-synced folder that happens to be one,
// or one an attacker plants), repo.Root finds that ".git" through the link
// on the very first candidate and hands back the symlinked path itself as
// the root. filepath.Rel(root, dir) then collapses to "." before the walk
// below ever runs, and the symlinked directory is never Lstat'ed.
func TestSecretFileIsRefusedWhenASymlinkedParentsTargetIsItselfARepository(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "repo", ".git"), 0o700); err != nil {
		t.Fatalf("mkdir repo/.git: %v", err)
	}
	// Dropbox is a repository of its own — the planted (or coincidental)
	// ".git" that let repo.Root resolve through the link.
	if err := os.MkdirAll(filepath.Join(dir, "Dropbox", ".git"), 0o700); err != nil {
		t.Fatalf("mkdir Dropbox/.git: %v", err)
	}
	if err := os.Symlink(filepath.Join(dir, "Dropbox"), filepath.Join(dir, "repo", "cloudkeys")); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	secret := filepath.Join(dir, "repo", "cloudkeys", "lazyslice.secret")
	r := &run{req: Request{SecretFile: secret, Workdir: filepath.Join(dir, "repo")}}

	err := r.checkSecretFile()
	var stop *Stop
	if !errors.As(err, &stop) {
		t.Fatalf("checkSecretFile returned %v, want a *Stop; the symlinked target is itself a repository, so repo.Root must not resolve through the link", err)
	}
	if stop.Code != CodeSecretSymlink || stop.Exit != exitCredential {
		t.Fatalf("refused with %s exit %d, want %s exit %d",
			stop.Code, stop.Exit, CodeSecretSymlink, exitCredential)
	}
	if _, statErr := os.Stat(filepath.Join(dir, "Dropbox", "lazyslice.secret")); statErr == nil {
		t.Error("the run wrote through the link anyway")
	}
}

// The nested form of the same gap: the secret file sits two levels below the
// symlink, so repo.Root's candidate that resolves through the link is the
// secret's *grandparent*, not its immediate parent, and the symlinked
// component itself is never on the walk unless the walk starts at a root the
// link cannot have supplied.
func TestSecretFileIsRefusedWhenASymlinkedGrandparentsTargetIsItselfARepository(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "repo", ".git"), 0o700); err != nil {
		t.Fatalf("mkdir repo/.git: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "Dropbox", ".git"), 0o700); err != nil {
		t.Fatalf("mkdir Dropbox/.git: %v", err)
	}
	if err := os.Symlink(filepath.Join(dir, "Dropbox"), filepath.Join(dir, "repo", "cloudkeys")); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	secret := filepath.Join(dir, "repo", "cloudkeys", "keys", "lazyslice.secret")
	r := &run{req: Request{SecretFile: secret, Workdir: filepath.Join(dir, "repo")}}

	err := r.checkSecretFile()
	var stop *Stop
	if !errors.As(err, &stop) {
		t.Fatalf("checkSecretFile returned %v, want a *Stop; cloudkeys itself is the symlink and must be caught even when the secret is nested below it", err)
	}
	if stop.Code != CodeSecretSymlink || stop.Exit != exitCredential {
		t.Fatalf("refused with %s exit %d, want %s exit %d",
			stop.Code, stop.Exit, CodeSecretSymlink, exitCredential)
	}
	if _, statErr := os.Stat(filepath.Join(dir, "Dropbox", "keys", "lazyslice.secret")); statErr == nil {
		t.Error("the run wrote through the link anyway")
	}
}

// A symlinked directory *outside* any repository is unaffected: there is no
// .gitignore boundary for a link to defeat, and the existing carve-out is
// "an operator naming a location" (the code comment on checkSecretFile).
func TestSecretFileWithASymlinkedParentButNoRepositoryIsAccepted(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "target"), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.Symlink(filepath.Join(dir, "target"), filepath.Join(dir, "link")); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	secret := filepath.Join(dir, "link", "lazyslice.secret")
	r := &run{req: Request{SecretFile: secret}}
	if err := r.checkSecretFile(); err != nil {
		t.Fatalf("checkSecretFile refused a symlinked parent outside any repository: %v", err)
	}
}

// R2-15 (THREAT_MODEL.md T6): a hard link gives the masking key a second name
// on disk that no path-based check — symlink or otherwise — can see, because
// the file the run reads and writes is still exactly the file it thinks it
// is.
func TestSecretFileIsRefusedWhenItHasMoreThanOneName(t *testing.T) {
	dir := t.TempDir()
	secret := filepath.Join(dir, "lazyslice.secret")
	if err := os.WriteFile(secret, []byte("00\n"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.Link(secret, filepath.Join(dir, "leaked.key")); err != nil {
		t.Fatalf("hard link: %v", err)
	}
	r := &run{req: Request{SecretFile: secret}}

	err := r.checkSecretFile()
	var stop *Stop
	if !errors.As(err, &stop) {
		t.Fatalf("checkSecretFile returned %v, want a *Stop; the key is reachable under a second name", err)
	}
	if stop.Code != CodeSecretHardlink || stop.Exit != exitCredential {
		t.Fatalf("refused with %s exit %d, want %s exit %d",
			stop.Code, stop.Exit, CodeSecretHardlink, exitCredential)
	}
}

// The two cases that must not refuse: a 0600 file, and no file at all (which is
// the first run, where resolveKey creates one 0600).
func TestAnOrdinarySecretFileIsAccepted(t *testing.T) {
	dir := t.TempDir()
	secret := filepath.Join(dir, "lazyslice.secret")
	r := &run{req: Request{SecretFile: secret}}
	if err := r.checkSecretFile(); err != nil {
		t.Fatalf("checkSecretFile refused an absent file: %v", err)
	}
	if err := os.WriteFile(secret, []byte("00\n"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := r.checkSecretFile(); err != nil {
		t.Fatalf("checkSecretFile refused a 0600 file: %v", err)
	}
}
