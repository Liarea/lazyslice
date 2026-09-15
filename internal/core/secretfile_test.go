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
