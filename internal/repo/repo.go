// SPDX-License-Identifier: Apache-2.0

// Package repo protects the files lazyslice writes that must never be committed:
// ./lazyslice.secret and any mapping_file named by the yml (THREAT_MODEL.md T6,
// ARCHITECTURE.md section 9 "The repository").
//
// The order is fixed and every branch prints what it did:
//
//  1. No repository: the secret is written and the header says it is not
//     protected by .gitignore.
//  2. Repository with a writable .gitignore: the missing entries are appended
//     and the addition is printed. Whether the secret file is actually
//     protected is then verified with git itself (`git check-ignore -v -z
//     --no-index --stdin`), never inferred from the file's text alone — a
//     rule later negated in the same file reads as "present" to a text
//     search and as "not ignored" to git (T-0251).
//  3. Repository whose .gitignore is absent or unwritable: the secret file is
//     not created at all, the run uses an ephemeral key and says so, and
//     --require-key makes that exit 5.
//  4. Tracked check: with git on PATH, exit 0 from ls-files --error-unmatch
//     means the file is tracked, which is exit 2 naming "git rm --cached".
//     Without git the check is skipped and the header says so.
//
// git is one of the binary's two subprocesses; --password-command is the other.
// It receives a path and never a row value.
package repo

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ErrNoRepository is returned by Root when no ancestor of the directory holds a
// .git entry. It is not a failure on its own: step 1 of section 9 writes the
// secret anyway and says it is unprotected.
var ErrNoRepository = errors.New("repo: no git repository above this directory")

// snapshotsEntry and secretEntry are the two fixed entries section 9 step 2
// appends beside every mapping path.
const (
	secretEntry    = "lazyslice.secret"
	snapshotsEntry = "snapshots/"
)

// gitTimeout bounds the one subprocess this package spawns. `git ls-files` on a
// large repository is fast, and a git that hangs — a filesystem that has gone
// away, a credential helper waiting on a terminal — must not hang the run
// before it has read a row.
const gitTimeout = 10 * time.Second

// State is what Protect found, so the caller can print it and decide whether the
// masking key may be written to disk.
type State struct {
	// Root is the nearest ancestor of the working directory containing .git, ""
	// when there is no repository.
	Root string
	// GitignoreWritable reports whether git itself verifies the secret path is
	// ignored by a rule that does not begin with "!" — not merely that
	// .gitignore could be read and appended to. A file that lists the entry
	// and then negates it lets a text search find the entry "present" while
	// git commits the file anyway (T-0251); only a verified, non-negated
	// match sets this true. It is false whenever that cannot be verified —
	// .gitignore absent or unwritable, the path not ignored, no repository,
	// or git missing from PATH — because refusing to write beats writing
	// un-ignored.
	GitignoreWritable bool
	// GitignoreAppendable reports whether .gitignore itself could be read and
	// appended to (appendMissing's own verdict), which is a narrower question
	// than GitignoreWritable: it says nothing about whether git actually
	// leaves the path ignored. The caller uses it to tell "the file could not
	// be written to at all" (the original T6 refusal) apart from "it was
	// written to, but the entry does not protect the path" — git absent
	// (GitFound false) or a later rule negating the one just added
	// (GitignoreNegatedBy) — which is the T-0251 round 6 finding: printing
	// the first message for the second cause tells the operator to fix a
	// file that is already correct.
	GitignoreAppendable bool
	// GitignoreNegatedBy is set, as "<source>:<lineno>", when
	// GitignoreWritable is false because `git check-ignore -v` named a
	// matching rule whose pattern begins with "!" — a later line in the same
	// file (or another one in git's search order) that un-ignores the path
	// lazyslice just asked .gitignore to protect (T-0251). It is empty
	// whenever GitignoreWritable is true, and also empty when it is false for
	// any other reason (no repository, .gitignore unwritable, git absent, or
	// git verifying the path is simply not ignored by any rule at all), so an
	// empty value never by itself means "not negated" — the caller reads it
	// together with GitignoreAppendable and GitFound.
	GitignoreNegatedBy string
	// Added lists the entries this run appended.
	Added []string
	// Tracked lists paths git says are tracked, which is exit 2.
	Tracked []string
	// GitFound reports whether git was on PATH; when false the tracked check was
	// skipped and the header says so.
	GitFound bool
}

// MayWriteSecret reports whether the masking key may be written to disk.
//
// It is the one question the caller has, and it is a method rather than a rule
// re-derived at each call site: outside a repository the secret is written
// unprotected and said to be (step 1), inside one it is written only when
// .gitignore took the entry (steps 2 and 3). A tracked file is a refusal the
// caller raises from Tracked, not a state in which writing is allowed.
func (s State) MayWriteSecret() bool {
	return s.Root == "" || s.GitignoreWritable
}

// Root finds the repository root: the nearest ancestor of dir containing .git.
//
// .git is a directory in a normal clone and a file in a worktree or a submodule,
// so its kind is not checked. A dir that does not exist, or that cannot be made
// absolute, is an error; a walk that reaches the filesystem root without finding
// one is ErrNoRepository, which callers treat as "no repository" rather than as
// a failure.
func Root(dir string) (string, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("repo: resolving %s: %w", dir, err)
	}
	for {
		if _, statErr := os.Lstat(filepath.Join(abs, ".git")); statErr == nil {
			return abs, nil
		}
		parent := filepath.Dir(abs)
		if parent == abs {
			return "", ErrNoRepository
		}
		abs = parent
	}
}

// Protect runs the four steps of section 9 for the secret file and every mapping
// file, and reports what it did.
//
// secretPath is where the masking key would go; mappingPaths are the
// mapping_file: paths the yml names. Both may be relative, and both are resolved
// against the working directory of the process, because that is what the flags
// they came from mean.
//
// It creates nothing. Whether the secret file may be written is State's answer
// and the caller's action: this package refuses to be the thing that writes an
// un-ignored key, and a package that both decided and wrote would have no way to
// report the decision before acting on it.
func Protect(secretPath string, mappingPaths []string) (State, error) {
	var st State

	root, err := Root(filepath.Dir(orDot(secretPath)))
	switch {
	case errors.Is(err, ErrNoRepository):
		// Step 1: no repository. Nothing else runs, and the caller prints that
		// the secret is not protected by .gitignore.
		return st, nil
	case err != nil:
		return st, err
	}
	st.Root = root

	// Step 2 and 3: the .gitignore entries.
	entries, err := wantedEntries(root, secretPath, mappingPaths)
	if err != nil {
		return st, err
	}
	added, writable, err := appendMissing(filepath.Join(root, ".gitignore"), entries)
	if err != nil {
		return st, err
	}
	st.Added = added
	st.GitignoreAppendable = writable

	// git absent from PATH is not a failure for either the verification below
	// or the tracked check further down: both are skipped, the header says so,
	// and everything falls back to the ephemeral key (section 9 step 3) and to
	// exit 5 under --require-key, because refusing to write beats writing
	// un-ignored.
	git, found := lookGit()
	st.GitFound = found

	// Steps 2 and 3, verified: appendMissing reports only whether .gitignore
	// could be read and appended to, which is not the same question as
	// whether git will actually leave the secret path untracked — a rule
	// later negated in the same file answers "present" to a text search and
	// "not ignored" to git (T-0251, the round 5 red team's still-leaking
	// entry against this exact function). With git on PATH the verdict is
	// asked of git directly, on the real secret path; without git the text
	// match cannot be verified at all, so GitignoreWritable stays false and
	// the caller falls back to an ephemeral key exactly as it does when
	// .gitignore is absent or unwritable.
	if writable && found {
		ignored, negatedBy, ignErr := checkIgnored(git, root, secretPath)
		if ignErr != nil {
			return st, ignErr
		}
		st.GitignoreWritable = ignored
		st.GitignoreNegatedBy = negatedBy
	}

	// Step 4: the tracked check.
	if !found {
		return st, nil
	}

	paths := append([]string{secretPath}, mappingPaths...)
	for _, p := range paths {
		tracked, err := isTracked(git, root, p)
		if err != nil {
			return st, err
		}
		if tracked {
			st.Tracked = append(st.Tracked, p)
		}
	}
	return st, nil
}

// lookGit finds git on PATH. It returns a bool rather than an error because
// "git is not installed" is one of the four states section 9 step 4 names, not
// something that went wrong.
func lookGit() (string, bool) {
	path, err := exec.LookPath("git")
	if err != nil {
		return "", false
	}
	return path, true
}

// RemoveFromIndex is the command a tracked file's refusal names. It is rendered
// here, from a path, so that the message a user is given and the check that
// produced it cannot drift apart.
func RemoveFromIndex(path string) string { return "git rm --cached " + path }

// wantedEntries is the .gitignore lines section 9 step 2 requires: the secret,
// snapshots/, and every mapping path, each relative to the repository root when
// it is inside it.
func wantedEntries(root, secretPath string, mappingPaths []string) ([]string, error) {
	out := []string{secretEntry, snapshotsEntry}
	for _, p := range append([]string{secretPath}, mappingPaths...) {
		rel, err := entryFor(root, p)
		if err != nil {
			return nil, err
		}
		if rel != "" {
			out = append(out, rel)
		}
	}
	return dedupe(out), nil
}

// entryFor is one path as a .gitignore entry, or "" when the path is outside the
// repository, where .gitignore cannot reach it and the operator has already
// taken it out of harm's way.
func entryFor(root, path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("repo: resolving %s: %w", path, err)
	}
	rel, err := filepath.Rel(root, abs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", nil //nolint:nilerr // outside the repository is a verdict, not a failure
	}
	// .gitignore patterns are slash-separated on every platform, and one
	// anchored at the repository root begins with a slash: an unanchored
	// `lazyslice.secret` also matches a file of that name in any subdirectory,
	// which is wider than the file we are protecting but never narrower.
	return "/" + filepath.ToSlash(rel), nil
}

// appendMissing appends the entries the file does not already carry and reports
// which it added and whether the file was writable at all.
//
// An absent .gitignore is not created: section 9 step 3 puts "absent" and
// "unwritable" in the same branch, and the run uses an ephemeral key in both.
// Creating one would be lazyslice writing a file in the operator's repository
// that they did not ask for, on the first run, which CONCEPT.md refuses.
func appendMissing(path string, entries []string) (added []string, writable bool, err error) {
	body, err := os.ReadFile(path)
	if err != nil {
		// Absent, unreadable, or a directory: step 3 either way.
		return nil, false, nil
	}

	have := map[string]bool{}
	for _, line := range strings.Split(string(body), "\n") {
		have[strings.TrimSpace(line)] = true
	}
	var missing []string
	for _, e := range entries {
		// Both spellings count as present: an operator who wrote
		// `lazyslice.secret` unanchored has already ignored the file, and
		// appending the anchored form under it would be noise in their diff.
		if have[e] || have[strings.TrimPrefix(e, "/")] {
			continue
		}
		missing = append(missing, e)
	}
	if len(missing) == 0 {
		// Nothing to add. The file exists and holds the entries, so it is
		// writable enough for the only purpose this package has for it.
		return nil, true, nil
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, false, nil
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil && err == nil {
			added, writable, err = nil, false, fmt.Errorf("repo: writing %s: %w", path, closeErr)
		}
	}()

	var b strings.Builder
	if len(body) > 0 && !strings.HasSuffix(string(body), "\n") {
		b.WriteString("\n")
	}
	b.WriteString("\n# lazyslice: never commit the masking key or a snapshot\n")
	for _, e := range missing {
		b.WriteString(e + "\n")
	}
	if _, err := f.WriteString(b.String()); err != nil {
		return nil, false, fmt.Errorf("repo: writing %s: %w", path, err)
	}
	return missing, true, nil
}

// isTracked asks git whether one path is in the index.
//
// Exit 0 is tracked, exit 1 is not tracked, exit 128 is "not a repository",
// which section 9 step 4 treats as case 1. Anything else is a failure: a git
// that answered something we do not understand has not said the file is
// untracked, and this check exists to fail closed.
func isTracked(git, root, path string) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
	defer cancel()

	// The path is made absolute first. `-C <root>` makes git resolve a relative
	// pathspec against the repository root, while every path this package is
	// given is relative to the *process* working directory — core's default
	// secret path is literally "./lazyslice.secret". Handing that to git from a
	// subdirectory of the repository asked about <root>/lazyslice.secret and got
	// exit 1, which this function reports as "not tracked": a committed key in a
	// subdirectory passed the check that exists to catch it (THREAT_MODEL.md T6).
	// filepath.Abs resolves against the same working directory the caller meant,
	// so the two agree again.
	abs, err := filepath.Abs(path)
	if err != nil {
		return false, fmt.Errorf("repo: resolving %s: %w", path, err)
	}

	// -C <root> and a "--" separator, so that a path beginning with a dash is a
	// path and not a flag. git receives a path and nothing else.
	cmd := exec.CommandContext(ctx, git, "-C", root, "ls-files", "--error-unmatch", "--", abs)
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil

	runErr := cmd.Run()
	if runErr == nil {
		return true, nil
	}
	var exitErr *exec.ExitError
	if errors.As(runErr, &exitErr) {
		switch exitErr.ExitCode() {
		case 1:
			return false, nil
		case 128:
			return false, nil
		}
	}
	return false, fmt.Errorf("repo: git ls-files on %s: %w", path, runErr)
}

// checkIgnored asks git whether path is ignored by a rule that does not begin
// with "!" (T-0251, THREAT_MODEL.md T6). `git check-ignore` reports the
// *last* matching rule in the file — that is how a later line is allowed to
// override an earlier one — and it exits 0 whether that rule ignores the
// path or un-ignores it with a leading "!", so the exit code alone answers
// "did any rule match", never "will git actually leave this file untracked".
// `-v` prints the matching rule so the two can be told apart, and
// `--no-index` is used because there may be no commit yet for git to compare
// against and the question is only about .gitignore, never about the index.
//
// The matching rule is read with `-z --stdin` rather than plain `-v`'s
// colon-joined "<source>:<lineno>:<pattern>\t<pathname>" line, and the path
// is written to git's stdin instead of passed as an argument (`-z` "only
// makes sense with --stdin"). Plain `-v`'s line cannot be split on colons
// from the left: `<source>` is a filesystem path, and git prints it
// *absolute* whenever the matching rule comes from core.excludesFile rather
// than an in-repo .gitignore — which on every platform can itself contain a
// colon, and on Windows always does ("C:/Users/..."), so
// `strings.SplitN(line, ":", 3)` was cutting `<source>` itself in two and
// reading the back half of the path plus `<lineno>` as `<pattern>` (T-0251
// round 6, the still-leaking case against the round 5 fix below). `-z`'s
// NUL-separated "<source>\0<lineno>\0<pattern>\0<pathname>\0" has no such
// ambiguity: none of the four fields needs escaping to be told apart from
// the others.
//
// Exit 0 is a match (checked against the printed rule below); exit 1 is no
// match at all; exit 128 is "not a repository", which the caller already
// knows is not the case here but which check-ignore can still report for
// reasons of its own (a bare `.git` file it does not expect, for one) — both
// 1 and 128 mean "not ignored". Anything else is an error: a git that
// answered something this function does not understand has not said the
// path is protected.
//
// It returns whether the path is ignored and, when it is not because the
// matching rule negates it, that rule's "<source>:<lineno>" — joined with a
// colon for the caller to print, which is safe here because this is
// assembled by us from the already-separated fields, not sliced back out of
// git's colon-joined line.
func checkIgnored(git, root, path string) (ignored bool, negatedBy string, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
	defer cancel()

	// The same reasoning as isTracked's: every path this package is given is
	// relative to the *process* working directory, and -C root resolves a
	// relative pathspec against the repository root instead, so the path is
	// made absolute first.
	abs, err := filepath.Abs(path)
	if err != nil {
		return false, "", fmt.Errorf("repo: resolving %s: %w", path, err)
	}

	cmd := exec.CommandContext(ctx, git, "-C", root, "check-ignore", "-v", "-z", "--no-index", "--stdin")
	cmd.Stdin = strings.NewReader(abs + "\x00")
	var out strings.Builder
	cmd.Stdout = &out
	cmd.Stderr = nil

	runErr := cmd.Run()
	if runErr == nil {
		return parseCheckIgnoreZ(out.String())
	}
	var exitErr *exec.ExitError
	if errors.As(runErr, &exitErr) {
		switch exitErr.ExitCode() {
		case 1, 128:
			return false, "", nil
		}
	}
	return false, "", fmt.Errorf("repo: git check-ignore on %s: %w", path, runErr)
}

// parseCheckIgnoreZ reads one record of `git check-ignore -v -z --stdin`'s
// output: "<source>\0<lineno>\0<pattern>\0<pathname>\0". It reports the path
// ignored unless the matching pattern begins with "!", in which case it
// returns the rule's location instead.
func parseCheckIgnoreZ(output string) (ignored bool, negatedBy string, err error) {
	fields := strings.Split(strings.TrimSuffix(output, "\x00"), "\x00")
	if len(fields) < 4 {
		return false, "", fmt.Errorf("repo: unexpected check-ignore -z output: %q", output)
	}
	source, lineno, pattern := fields[0], fields[1], fields[2]
	if strings.HasPrefix(pattern, "!") {
		return false, source + ":" + lineno, nil
	}
	return true, "", nil
}

func dedupe(in []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		if seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

func orDot(p string) string {
	if p == "" {
		return "."
	}
	return p
}
