# internal/discover

The discovery ladder (rungs 0-5: yml, env/`.env`, libpq, running container,
stopped container, compose name) and candidate verification/de-duplication.
Subpackages: `dockerctx/` resolves the Docker endpoint; `provision/` is the
`--create-target` path. The target *gate* is not here — that's
`internal/pg` (`Target.Gate`), because gating needs a live connection this
package's 1s dial budget does not allow for.

**Contract.** ARCHITECTURE.md §2 "discover" and §9: implements
`pipeline.Discoverer.Discover(ctx, workdir, sink) ([]Candidate, error)`.
`provision/` implements `pipeline.Provisioner`.

**Rules.**
- 2s total listing budget, 1s per-candidate dial (ARCHITECTURE.md §9) — do not
  add a probe that can exceed either.
- Inside the dial: at most three statements per candidate (version, the
  `pg_class` count/hint, `to_regclass('lazyslice_meta')`). Never probe
  emptiness table by table here; that is the gate's job, after discovery
  (THREAT_MODEL.md T2).
- `provision/` is the *only* code in the tree that creates or starts a
  container, and only behind `--create-target` or a "yes" to Q1 — never called
  unconditionally.
- Source selection (most-local reachable candidate with the most tables, never
  a question) lives here; do not turn it into a question without an
  ARCHITECTURE.md §9 change.

**Test.** `go test ./internal/discover/...`; container-touching parts need
`go test -tags integration ./internal/discover/...`.

**Never:** create or start a container outside `provision/` or outside
`--create-target`/Q1; run an unbounded emptiness probe inside the dial; let
discovery itself decide a target is eligible — only `Target.Gate` does that.
