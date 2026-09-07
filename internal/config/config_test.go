// SPDX-License-Identifier: Apache-2.0

package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// $XDG_STATE_HOME wins on every platform: a developer who set it has said where
// their state goes, and a tool that ignored it would write somewhere they do not
// back up.
func TestStateDirHonoursXDG(t *testing.T) {
	base := t.TempDir()
	t.Setenv("XDG_STATE_HOME", base)

	dir, err := StateDir()
	if err != nil {
		t.Fatalf("StateDir: %v", err)
	}
	if want := filepath.Join(base, dirName); dir != want {
		t.Errorf("StateDir = %q, want %q", dir, want)
	}

	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("the directory was not created: %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("%s is not a directory", dir)
	}
	if runtime.GOOS != "windows" {
		if perm := info.Mode().Perm(); perm != dirPerm {
			t.Errorf("mode = %o, want %o: everything under here could be a credential", perm, dirPerm)
		}
	}
}

// An existing directory with looser permissions is tightened rather than
// accepted: the caller is about to write a password into it.
func TestStateDirTightensAnExistingDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows has no Unix mode bits")
	}
	base := t.TempDir()
	t.Setenv("XDG_STATE_HOME", base)
	loose := filepath.Join(base, dirName)
	if err := os.MkdirAll(loose, 0o755); err != nil {
		t.Fatalf("making %s: %v", loose, err)
	}

	if _, err := StateDir(); err != nil {
		t.Fatalf("StateDir: %v", err)
	}
	info, err := os.Stat(loose)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != dirPerm {
		t.Errorf("mode = %o, want %o", perm, dirPerm)
	}
}

// Without XDG_STATE_HOME the platform's own location is used, and it always
// ends in the one directory name every caller goes through.
func TestStateDirFallsBackToThePlatform(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", "")
	t.Setenv("HOME", t.TempDir())

	dir, err := StateDir()
	if err != nil {
		t.Fatalf("StateDir: %v", err)
	}
	if !strings.HasSuffix(dir, string(filepath.Separator)+dirName) {
		t.Errorf("StateDir = %q, want it to end in %q", dir, dirName)
	}
}
