# internal/config

Machine-local state directory resolution only: `StateDir()`. This is not
`lazyslice.yml` (that's `internal/emit`, and it's committed); this is the
per-machine, never-committed directory holding what must survive between runs
on one laptop but never leave it — in v1, only the password of a container
`--create-target` provisioned.

**Contract.** ARCHITECTURE.md §9 "Provisioning": the provisioned container's
`POSTGRES_PASSWORD` is "stored only in the machine-local state dir (ADR-004)".
This package is that dir's only accessor.

**Rules.**
- No keyring integration in v1 (ADR-004) — do not add one speculatively; it is
  an explicit non-goal, not an oversight.
- Directory is created `0o700`; anything written under it that could be a
  credential inherits that permission, never looser.
- `$XDG_STATE_HOME/lazyslice` or the platform equivalent — do not invent a
  second location; every caller goes through `StateDir()`.

**Test.** `go test ./internal/config/...`.

**Never:** add a keyring dependency (ADR-004 says no; `go.mod`/`go.sum` are not
writable from this task's paths anyway); write anything here with permissions
looser than `0o700`; let a DSN or its password pass through this package
un-narrowed to the one field ADR-004 allows.

## Decisions made during implementation

- **`$XDG_STATE_HOME` wins on every platform**, not only where XDG is native: a
  developer who set it has said where their state goes.
- Without it: `$HOME/.local/state/lazyslice` on Unix, and `os.UserConfigDir()`
  (`~/Library/Application Support`, `%AppData%`) on macOS and Windows — Go's own
  resolution rather than a second spelling of ours.
- **An existing directory is tightened to `0o700`,** not accepted as it is:
  `MkdirAll` leaves an existing mode alone and umask can widen the one it
  creates, and the caller is about to write a password into it. The `Chmod` is
  skipped on Windows, which has no Unix mode bits.
