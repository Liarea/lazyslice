// SPDX-License-Identifier: Apache-2.0

package core

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/Liarea/lazyslice/internal/event"
)

// ADR-012 (T-0138): a committed lazyslice.yml naming mapping_file for a
// column is a usage error, not a value silently round-tripped. readConfig has
// to turn internal/emit's *MappingFileError into a Stop at exit 2, under
// CodeConfigMappingFileUnsupported, naming the path, table and column —
// docs/reviews/2026-09-09/REVIEW.md finding 10's whole point was that the file
// claimed an escape nothing implemented, and a reviewer found no test
// anywhere pinned that the refusal actually fires on the CLI's own read path.
func TestReadConfigRefusesMappingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "lazyslice.yml")
	body := "version: 1\n" +
		"tool: 0.1.0\n" +
		"columns:\n" +
		"  public.customer.national_id:\n" +
		"    category: national_id\n" +
		"    confidence: certain\n" +
		"    mapping_file: mappings/national_id.csv\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	r := &run{req: normalise(Request{ConfigPath: path}), sink: event.Discard}
	err := r.readConfig()

	var stop *Stop
	if !errors.As(err, &stop) {
		t.Fatalf("readConfig err = %v, want a *Stop", err)
	}
	if stop.Code != CodeConfigMappingFileUnsupported {
		t.Errorf("code = %s, want %s", stop.Code, CodeConfigMappingFileUnsupported)
	}
	if stop.Exit != exitUsage {
		t.Errorf("exit = %d, want %d (exit 2)", stop.Exit, exitUsage)
	}
	if stop.Args[event.ArgPath] != path {
		t.Errorf("path arg = %q, want %q", stop.Args[event.ArgPath], path)
	}
	if stop.Args[event.ArgTable] != "public.customer" {
		t.Errorf("table arg = %q, want public.customer", stop.Args[event.ArgTable])
	}
	if stop.Args[event.ArgColumn] != "national_id" {
		t.Errorf("column arg = %q, want national_id", stop.Args[event.ArgColumn])
	}
	// r.prior must stay nil: a refused read must not hand the rest of the run
	// a config built from the refused file.
	if r.prior != nil {
		t.Error("readConfig set r.prior on a refused read")
	}
}
