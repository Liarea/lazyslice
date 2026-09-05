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
