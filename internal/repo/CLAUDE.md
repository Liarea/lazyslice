# internal/repo

Protects the files lazyslice writes that must never be committed:
`./lazyslice.secret` and any `mapping_file:` path named by the yml. Finds the
repository root, manages `.gitignore`, and checks whether a path is tracked by
git. Nothing else touches git in this codebase.

**Contract.** ARCHITECTURE.md §9 "The repository" (the four-step order in
full) and THREAT_MODEL.md T6: `repo.Root(dir) (string, error)` and
`repo.Protect(path, mappingPaths []string) (State, error)`.

**Rules.**
- The order is fixed and every branch prints what it did (ARCHITECTURE.md §9):
  no repo → write unprotected, say so; repo + writable `.gitignore` → append
  and say so; repo + absent/unwritable `.gitignore` → **do not create** the
  secret file, use an ephemeral key, say so (`--require-key` → exit 5); tracked
  check via `git ls-files --error-unmatch` (exit 1 = untracked, exit 0 =
  tracked → exit 2 naming `git rm --cached`, exit 128 = no repo). Do not
  reorder these or collapse the "unwritable gitignore" case into "write it
  anyway" — refusing beats writing un-ignored.
- `git` is one of the binary's exactly two subprocesses
  (`--password-command` is the other, THREAT_MODEL.md T4) — this package must
  never spawn anything else, and it passes `git` a path, never a row value.
- With `git` absent from `PATH`, the tracked check is skipped and the header
  says so; the `.gitignore` entry still protects a never-tracked file.

**Test.** `go test ./internal/repo/...` against a temp git repo fixture (with
and without a `.gitignore`, tracked and untracked cases); a test in this
package or `internal/discover` also enforces the two-subprocess allowlist.

**Never:** create `lazyslice.secret` when `.gitignore` cannot be written; pass
git anything but a path; spawn a subprocess other than `git`.

## Decisions made during implementation

- **`Protect` decides and never writes the secret.** It reports a `State` and
  the caller acts on it: a package that both decided and wrote would have no way
  to print the decision before acting on it, and §9 requires every branch to say
  what it did. `State.MayWriteSecret` is the one question the caller has.
- **An absent `.gitignore` is not created.** §9 step 3 puts "absent" and
  "unwritable" in one branch and gives both an ephemeral key. Creating one would
  be lazyslice writing a file in the operator's repository, on the first run,
  that they did not ask for.
- **Entries are written anchored** (`/lazyslice.secret`), and an existing
  unanchored `lazyslice.secret` counts as present: an operator who already
  ignored the file gets no noise in their diff.
- **A path outside the repository yields no entry.** `.gitignore` cannot reach
  it, and `--secret-file` outside the tree is the remedy §9 already prints.
- **`git ls-files` runs under a 10 s timeout** and receives `-C <root>`,
  `--error-unmatch` and a `--`-separated path, so a path beginning with a dash
  is a path. Exit 0 is tracked, 1 is untracked, 128 is "not a repository"; any
  other exit is an error, because a git that answered something we do not
  understand has not said the file is untracked.
- **`RemoveFromIndex`** renders the `git rm --cached` command the refusal names,
  so the message and the check cannot drift apart.
- **The tracked check makes the path absolute first** (T-CORE review,
  2026-09-06). `git -C <root>` resolves a *relative* pathspec against the
  repository root, while every path this package is given is relative to the
  process working directory — `core.DefaultSecretFile` is literally
  `./lazyslice.secret`. From a subdirectory the check therefore asked about
  `<root>/lazyslice.secret`, got exit 1, and reported a committed key as
  untracked: a fail-open on THREAT_MODEL.md T6. Every test here passed an
  absolute path, so none of them could see it;
  `TestTrackedSecretIsFoundThroughARelativePath` is the one that can.
- **`appendMissing`'s "present" is a text match and is no longer trusted on
  its own** (T-0251, THREAT_MODEL.md T6's 2026-09-17 amendment, the round 5
  red team). A `.gitignore` holding `lazyslice.secret` followed by its own
  negation, `!lazyslice.secret`, reads as "the entry is present" to a line
  search and as "not ignored" to git, which resolves `check-ignore` in the
  file's own last-match-wins order. `Protect` now asks git directly, once,
  right after `appendMissing` runs: `checkIgnored` calls
  `git -C <root> check-ignore -v --no-index -- <secret path>` and reads the
  `-v` output's matching rule, because exit 0 alone still only means "some
  rule matched" and does not distinguish an ignoring rule from a negating
  one (`negatedRule`, reading the pattern field of the "<source>:<lineno>:
  <pattern>" line). `State.GitignoreWritable` is set from this verified
  answer, never from `appendMissing`'s own writable bool directly. Exit 1
  (not ignored), exit 128 (no repository), a rule beginning with `!`, and
  git being absent from `PATH` are all treated alike — "not verified
  protected" — which is a behaviour change from before this task: with git
  absent, `GitignoreWritable` used to come from `appendMissing` alone and
  could be true; it cannot be true now, because a text match that cannot be
  checked against git is not a verified one. `TestNegatedGitignoreEntry
  DoesNotProtect` and `TestGitAbsentFallsBackToEphemeral` pin both halves of
  that change.
- **`checkIgnored` reuses `isTracked`'s shape**: the same `gitTimeout`, the
  same "make the path absolute first" reasoning (`-C <root>` resolves a
  relative pathspec against the repository root, and every path this
  package receives is relative to the process working directory), and the
  same `--` before the path. It differs only in reading `-v`'s stdout
  instead of the exit code alone, because `check-ignore`'s exit code cannot
  by itself answer the question this check exists to ask.
- **`checkIgnored` reads `-v -z --stdin`, not plain `-v`'s colon-joined line**
  (T-0251 round 6, the review that found the round 5 fix above still
  leaking). Plain `-v` prints `<source>:<lineno>:<pattern>\t<pathname>`, and
  `negatedRule` used to split that on colons from the left — but `<source>`
  is a filesystem path, and git prints it *absolute* whenever the matching
  rule comes from `core.excludesFile` or a nested in-repo `.gitignore`
  rather than the root one, which can itself contain a colon on any
  platform and always does on Windows (`C:/Users/...`). A negating rule
  whose source contained a colon was cut in two by that split and read as
  "not negated" — `GitignoreWritable` came back `true` for a path git would
  still commit. `checkIgnored` now runs `check-ignore -v -z --no-index
  --stdin` (`-z` "only makes sense with --stdin", so the path travels on
  stdin, NUL-terminated, instead of as an argument) and
  `parseCheckIgnoreZ` reads its NUL-separated
  `<source>\0<lineno>\0<pattern>\0<pathname>\0` fields directly, with
  nothing to split on. `State` gained two fields for this:
  `GitignoreAppendable` (`appendMissing`'s own writable bool, kept separate
  from the verified `GitignoreWritable`) and `GitignoreNegatedBy`
  (`"<source>:<lineno>"`, set only when the matching rule negates the
  entry), so a caller can tell "the file could not be written to at all"
  apart from "it was written to, but git still does not ignore the path" —
  `internal/core`'s T-0251 round 6 section records the message-side half of
  this. `TestParseCheckIgnoreZNegation` pins the parser directly, over a
  colon-bearing absolute source and a Windows-shaped `C:/...` one, and
  `TestNegatedGitignoreEntryWithColonSource` reproduces the leak end to end
  through `Protect`, using a nested `sub:dir/.gitignore` — which beats the
  root `.gitignore` Protect appends to in git's own precedence, the one
  in-repo way a colon-bearing source can be the rule that actually decides
  the outcome.
