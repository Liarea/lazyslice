# Runbook

How to drive this project, for the orchestrator session or a human picking it up cold.

## Where am I

```
cat CLAUDE.md | sed -n '/Current phase/,/^## Rules/p'   # phase and gate
python3 tools/tracker.py list --status open             # what is unfinished
python3 tools/tracker.py list --status blocked          # what needs a decision
cat tracker/BOARD.md                                    # the board
```

## Phase order and the workflow that runs it

| Phase | Workflow | Runs | Human needed |
|---|---|---|---|
| 0 Frame | written by hand | CONCEPT.md, OPERATING_MODEL.md, tracker | no |
| 1 Research | `.claude/workflows/research.js` | 8 docs, critique, revise, 5 guides, synthesis | no |
| 2 Architecture | `.claude/workflows/architecture.js` | benchmark, 3 proposals, 3 judges, ADRs, review | no |
| 3 Foundations | `.claude/workflows/foundations.js` with `{step}` | `scaffold`, `fixtures` (fixtures, ADR-008), `docs` (CLAUDE.md files, roadmap, ADR-008 sync), `invariants`, `review`; one step per window | no |
| 4 Vertical slice | `.claude/workflows/slice.js` | ten packages in dependency order through implement.js, stops at first block; resume by run id | no |
| 5 Hardening | `.claude/workflows/hardening.js` with `{step}` | `features` (TUI, provisioning, polymorphic, CI matrix), `backlog` (review follow-ups), `harden` (torture, failure UX, perf), `redteam` | no |
| 6 Launch | drafts only | README, GIF, posts | yes: create GitHub repo, post |
| 7 Breadth | implement.js per adapter, worktrees | MySQL, SQLite, SQL Server | no |
| 8 Client-facing | research only | interviews, hosted design, pricing | yes: everything commercial |

Run a phase from the orchestrator session with the Workflow tool and `scriptPath` pointing at the script. Each run reports a run id; to resume after an edit or interruption, pass `resumeFromRunId` and unchanged agents return cached results.

`implement.js` is the unit of work everywhere from phase 3 on. It takes `args {id, title, brief, model, paths, stage, integration}` and returns `merged` or `blocked` with findings. `slice.js` accepts `args {from: n}` to resume from stage n.

## After every workflow

1. Read the returned summaries, not the files, unless a decision depends on file contents.
2. Record outcomes: `python3 tools/tracker.py close T-xxxx --outcome ... --postmortem "..."`, or `block` with the findings.
3. Update the Current phase line in CLAUDE.md when a gate is met.
4. Commit: `git add -A && git commit -m "phase N: ..."`.
5. Read the gate in docs/BUILD_PLAN.md before starting the next phase. A gate item without evidence is not ticked.

## When a run dies on a usage limit

Agents fail with "You've hit your session limit" and the workflow returns with a failures list. Nothing is lost: files already written stay on disk and finished agents are cached. After the limit resets, relaunch with the same `scriptPath` and `resumeFromRunId`; only the failed agents run. Commit the partial output first so a later edit to the script cannot orphan it.

## When a task comes back blocked

Read the findings. Decide one of: split the task and rerun; write an ADR if the block is a design flaw; or cancel with a reason in the tracker. Never weaken a test or an invariant to unblock.

## Model routing

See docs/OPERATING_MODEL.md. Do not downgrade reviewers to save tokens.

## What only the human can do

Create the GitHub repository and push. Publish posts. Buy a domain. Anything involving an account, payment, or legal step. Leave these as open tasks in the tracker with owner "human".

## Cutting a release

1. The commit is on main and its `ci` run is green (public runners; `make torture` included since T-0140, gated to pushes to `main` so it does not run on every pull request). No tag on a red or unfinished commit: `release.yml`'s first step queries the checks API for a completed, successful `ci` run on the tagged commit's sha, restricted to `event=push&branch=main` — the only run kind that runs `torture` — and refuses to build if there is none, before checkout or `make check` run — the refusal is the point. The `bench` job's regression check (`make bench-compare`) gates the tagged commit against its own parent, not against a number recorded on some other run, so a green `ci` run there says this commit did not regress throughput relative to the one before it.
2. `git tag -a vX.Y.Z -m "vX.Y.Z"` then `git push origin vX.Y.Z`. goreleaser builds, signs, publishes the GitHub release (pre-release while `.goreleaser.yaml` says `prerelease: true`) and pushes the cask to `Liarea/homebrew-tap` with `HOMEBREW_TAP_TOKEN`.
3. On a machine that did not build it: `brew install Liarea/tap/lazyslice && lazyslice --version`. If the version does not print, the release is withdrawn, not patched in place.
4. The release notes are `make relnotes FROM=<previous tag> TO=<tag>`, generated in the workflow from the commit bodies; read them before tagging (`make relnotes` with no arguments previews from the last tag to HEAD) and fix a commit's bullets with a follow-up commit, never by rewriting history. Edit the notes on GitHub only to name a breaking change the bullets did not make obvious.

Before step 2, two things the workflow will refuse a tag over, both cheaper to get right first: README.md's `## Status` heading must name the tag being cut (T-0259; the check reads that one line), and the tag's commit must have a finished, green `ci` run, so push the Status edit, wait for CI, then tag that commit.

Which tags reach the tap (decided 2026-09-17, T-0155): every release is marked pre-release on GitHub because `.goreleaser.yaml` sets `release.prerelease: true`, and that setting does not keep a cask out of the tap. The cask's `skip_upload: auto` reads the *tag*: goreleaser skips the upload only when the tag itself carries a pre-release indicator (`v0.1.0-rc1`), and the workflow's "a release tag must have published a cask" step uses the same test, a dash in the tag name. So `v0.0.1` and `v0.1.0` publish a cask and `brew install Liarea/tap/lazyslice` gets them; a tag with a dash publishes binaries only. The v0.0.x pipeline proofs are plain tags for exactly that reason: a dashed tag would leave the tap empty and prove nothing about step 3. A tag is also permanent in one place nothing here controls: the Go module proxy keeps any version it has been asked for, so a withdrawn release is withdrawn from GitHub and the tap, not from `proxy.golang.org`.

What broke, and what changed (T-0155). **v0.0.1, 2026-09-22:** the release job failed at "Set up job", before its first step, because `sigstore/cosign-installer@v4` does not resolve: that action publishes exact `v4.x.y` tags and floating `v3`/`v2`, but no floating `v4`. Nothing was published — no GitHub release, no cask — and the tag stays as the record. The workflow now pins `@v4.1.2`, and every `uses:` in `release.yml` was checked against the action's real tags with `gh api repos/<owner>/<repo>/git/ref/tags/<ref>`; do that for any new action, because no push to main runs this job and a tag is the only thing that exercises it. Each failed proof is followed by the next `v0.0.x`, never by moving the tag: the tag is immutable even when it published nothing.

Versioning policy: ROADMAP.md, "Versioning and releases".
