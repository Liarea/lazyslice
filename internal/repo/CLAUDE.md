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
