---
name: lazyslice-commit
description: How a lazyslice commit is written and staged: a stage-prefixed headline with the tracker id, a body of release-note bullets that describe behaviour, trailers, path-scoped staging while a workflow runs, and the checks that must pass first. Use for every commit the orchestrator or a workflow makes.
---

# Committing

Release notes between two versions are generated from commit bodies (`make relnotes FROM=v0.1.0 TO=v0.2.0`, `tools/relnotes/relnotes.py`), so every commit that changes behaviour carries bullets a user can read. The maintainer, 2026-09-16: one-line commits that only reference tickets are unusable for that.

## Format

```
<stage>: <what changed, imperative, under 72 characters> (T-0187)

- <one user-visible change or fix per bullet, in plain words>
- <name the flag, exit code, message, column kind or behaviour; never a file, function or tracker id>
- <a refusal names the escape the operator keeps>

Task: T-0187
Review: 1 reviewer, 2 fix rounds
```

- `stage` is the pipeline stage or area: `classify`, `plan`, `extract`, `transform`, `mask`, `load`, `verify`, `emit`, `discover`, `core`, `pg`, `introspect`, `tui`, `cli`, `foundations`, `hardening`, `ci`, `bench`, `release`, `peripheral`. `tools/relnotes/relnotes.py` groups by it.
- The headline may reference the task; the bullets may not. Three to eight bullets for a code change; one for a documentation change.
- Housekeeping commits (`Tracker: ...`, `ROADMAP: ...`) and tooling commits (`chore: ...`: workflows, skills, tools/) are excluded from release notes by prefix; a tooling commit still gets bullets for the maintainer, a housekeeping one needs no body.
- Write the message to a file and commit with `git commit -F`; a multi-line `-m` in a shell quoting layer loses lines.

Good bullet: "A plain US social security number in a column whose name matches no rule is now masked; classify reports the column as national_id." Bad bullet: "Add ValidSSN to textsig and wire it in classify.go."

## Before committing

- The checks the task names have passed on this tree: `make check` always; the integration packages the task names; `make torture` when `testdata/` changed. Paste nothing; the commit is the claim and CI is the proof.
- Generated files are current: `make docs` after any change to `internal/event/catalogue.yml` or the command tree.
- Nothing stray is staged: no binaries, no scratch files, no `docs/reviews` evidence you did not mean to version.

## Staging

- While a workflow that commits is running, stage exact paths (`git add tracker/ docs/RUNBOOK.md`), never `git add -A`; an agent's half-written files would be swept into an unrelated commit (docs/OPERATING_MODEL.md, "Commit discipline"). With no workflow running, `git add -A` is fine after a `git status` read.
- A workflow's own commit step stages everything under the task's paths and commits once, after independent verification. Developers never commit.
- Push after each landing; CI on `main` is the evidence the roadmap cites, and the release workflow refuses a tag whose commit has no green run.

## Landing a task by hand

When a task blocks in a workflow and the orchestrator finishes it: verify the tree as the workflow would have (check, the named integration packages, torture if `testdata/` changed), commit in this format with the task's id in the headline and `Review: landed by hand after N rounds` in the trailer, close the task with a post-mortem that says why it blocked (lazyslice-tracker skill), then push.
