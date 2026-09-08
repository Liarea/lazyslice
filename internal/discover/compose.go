// SPDX-License-Identifier: Apache-2.0

package discover

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/goccy/go-yaml"
)

// composeFiles is the file set, in compose's own precedence order. The first
// one present is the project's.
var composeFiles = []string{
	"compose.yaml", "compose.yml",
	"docker-compose.yaml", "docker-compose.yml",
}

// composeProject is everything lazyslice takes from a compose file, and the
// type is the enforcement: there is no field here for a host, a port, a user, a
// password or a database name, so no code path can grow one.
//
// Compose YAML is a naming source and nothing else (ADR-004, ADR-008 §2).
// Reading the YAML to guess a port is how you produce a connection string that
// does not connect, and interpolating it from .env produces the same wrong
// string one layer further out.
type composeProject struct {
	// Name is the compose project name: the file's `name:`, else the directory
	// the file is in. It names the container Q1 offers to create.
	Name string
	// PostgresServices are the service names whose image looks like Postgres.
	// They are printed, never connected to.
	PostgresServices []string
	// File is the compose file that was read, relative to the working
	// directory, empty when there is none.
	File string
}

// readCompose reads the project's compose file for names.
//
// A missing or unparseable file is not an error: the project name falls back to
// the directory, which is compose's own default, and the ladder continues.
func readCompose(workdir string) composeProject {
	p := composeProject{Name: projectNameFromDir(workdir)}

	for _, name := range composeFiles {
		path := filepath.Join(workdir, name)
		b, err := os.ReadFile(filepath.Clean(path))
		if err != nil {
			continue
		}
		p.File = name

		// Only these two keys are read. A wider struct would be a place for a
		// host or a password to arrive later.
		var doc struct {
			Name     string `yaml:"name"`
			Services map[string]struct {
				Image string `yaml:"image"`
			} `yaml:"services"`
		}
		if err := yaml.Unmarshal(b, &doc); err != nil {
			return p
		}
		if doc.Name != "" {
			p.Name = doc.Name
		}
		for service, spec := range doc.Services {
			if strings.Contains(strings.ToLower(spec.Image), "postgres") {
				p.PostgresServices = append(p.PostgresServices, service)
			}
		}
		sort.Strings(p.PostgresServices)
		return p
	}
	return p
}

// projectNameFromDir is compose's default project name: the base name of the
// directory, lower-cased, with everything but letters, digits, underscores and
// hyphens removed.
func projectNameFromDir(workdir string) string {
	abs, err := filepath.Abs(workdir)
	if err != nil {
		abs = workdir
	}
	base := strings.ToLower(filepath.Base(abs))
	var b strings.Builder
	for _, r := range base {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '_', r == '-':
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		return "lazyslice"
	}
	return b.String()
}
