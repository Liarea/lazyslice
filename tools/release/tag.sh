#!/usr/bin/env bash
#
# tools/release/tag.sh — cut a release tag once every precondition the
# runbook names holds (docs/RUNBOOK.md, "Cutting a release"; tracker T-0155).
#
# `make tag TAG=vX.Y.Z` is the one way a tag leaves this checkout. The script
# refuses, with one sentence naming why, unless all of these hold:
#
#   1. TAG is a semantic version with a v prefix (a -suffix is allowed and
#      keeps the cask out of the tap, per .goreleaser.yaml's skip_upload).
#   2. The checkout is on main, clean, and HEAD is exactly origin/main.
#   3. TAG exists neither locally nor on origin: a tag is immutable, and a
#      failed proof is followed by the next v0.0.x, never by a moved tag.
#   4. README.md's `## Status` heading names TAG, the same test
#      .github/workflows/release.yml applies before it publishes anything.
#   5. Every `uses:` in release.yml resolves to a real tag, branch or commit
#      of that action — v0.0.1 failed at "Set up job" on a floating major
#      that the action does not publish, and no push to main runs that job.
#   6. `goreleaser check` accepts .goreleaser.yaml.
#   7. HEAD has a finished, green `ci` run; an unfinished one is waited for.
#   8. mask/ at HEAD is exactly the mask/vX.Y.Z tag go.mod requires, so the
#      release binary and `go install` of the same tag mask identically
#      (T-0290; goreleaser builds with GOWORK=off for the same reason).
#
# Then it tags HEAD, pushes the tag, finds the release run it starts and, by
# default, watches that run to the end and prints the release URL. It never
# builds, installs or publishes anything itself: goreleaser does that on the
# runner, and the brew-install proof on a machine that did not build it is a
# separate, human-witnessed step.
#
# A mask module tag, mask/vX.Y.Z (ROADMAP.md "Versioning and releases"),
# takes the same checks 1 to 3 and 7 and skips the rest: release.yml ignores
# mask/v* on purpose (the module is imported, never built into a binary), so
# there is no README heading to name it, no action to resolve, no goreleaser
# pipe and no run to watch. The tag is what lets go.mod require the module
# by version instead of a local replace (T-0285).
#
# Usage: tools/release/tag.sh vX.Y.Z|mask/vX.Y.Z [--dry-run] [--no-watch]
#   --dry-run   run every check and stop before tagging
#   --no-watch  push the tag and print the release run's URL without waiting

set -euo pipefail

TAG="${1:-}"
DRY_RUN=0
WATCH=1
for arg in "${@:2}"; do
	case "$arg" in
	--dry-run) DRY_RUN=1 ;;
	--no-watch) WATCH=0 ;;
	*)
		echo "tag: unknown argument '$arg' (usage: tools/release/tag.sh vX.Y.Z [--dry-run] [--no-watch])" >&2
		exit 2
		;;
	esac
done

log() { echo "==> tag: $*"; }
die() {
	echo "tag: REFUSED: $*" >&2
	exit 1
}

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

# ---------- 1. the tag's shape ----------
[ -n "$TAG" ] || die "no tag given; usage: make tag TAG=vX.Y.Z (or TAG=mask/vX.Y.Z for the mask module)"
MASK=0
case "$TAG" in mask/*) MASK=1 ;; esac
[[ "${TAG#mask/}" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.]+)?$ ]] ||
	die "'$TAG' is not a tag this project cuts (vX.Y.Z or mask/vX.Y.Z, optionally -suffix; ROADMAP.md 'Versioning and releases')"
if [ "$MASK" -eq 1 ]; then
	log "$TAG is a mask module tag: no release run follows it (release.yml ignores mask/v*); it exists so go.mod can require the module by version"
else
	case "$TAG" in
	*-*) log "$TAG carries a pre-release suffix: goreleaser publishes the binaries and skips the cask (skip_upload: auto)" ;;
	esac
fi

# ---------- 2. the checkout ----------
command -v gh >/dev/null 2>&1 || die "gh is not on PATH"
gh auth status >/dev/null 2>&1 || die "gh is not authenticated (gh auth login)"
branch="$(git rev-parse --abbrev-ref HEAD)"
[ "$branch" = "main" ] || die "on branch '$branch', not main"
[ -z "$(git status --porcelain)" ] || die "the working tree is not clean; commit or stash first"
git fetch -q origin main "+refs/tags/*:refs/tags/*"
sha="$(git rev-parse HEAD)"
upstream="$(git rev-parse origin/main)"
[ "$sha" = "$upstream" ] || die "HEAD ${sha:0:7} is not origin/main ${upstream:0:7}; push (or pull) first, a tag goes on what CI saw"

# ---------- 3. the tag is new ----------
if git rev-parse -q --verify "refs/tags/$TAG" >/dev/null; then
	die "$TAG already exists locally; a tag is immutable — cut the next one"
fi
if git ls-remote --exit-code --tags origin "refs/tags/$TAG" >/dev/null 2>&1; then
	die "$TAG already exists on origin; a tag is immutable — cut the next one"
fi

# ---------- 4. README names the tag ----------
# The same test release.yml applies (T-0259): the first `## Status` heading
# must contain the tag as a whole token. A mask tag is not a version of the
# tool and is not named there.
if [ "$MASK" -eq 0 ]; then
status_line="$(grep -m1 '^## Status' README.md || true)"
[ -n "$status_line" ] || die "README.md has no '## Status' heading"
escaped_tag="$(printf '%s' "$TAG" | sed -e 's/[.[\*^$]/\\&/g')"
if ! grep -qE "(^|[^A-Za-z0-9._-])${escaped_tag}([^A-Za-z0-9._-]|\$)" <<<"$status_line"; then
	die "README.md's Status heading does not name $TAG — release.yml would refuse the tag: '$status_line'"
fi
fi

# ---------- 8. mask/ is the version go.mod requires ----------
# With go.work at the root, an unreleased edit under mask/ passes every
# local check; the release (GOWORK=off) and `go install` would both build
# the older tagged mask instead. Cut mask/vX.Y.Z and bump the require first.
# Markdown under mask/ is excluded: a doc edit changes no masker.
if [ "$MASK" -eq 0 ]; then
mask_ver="$(awk '$1 == "github.com/Liarea/lazyslice/mask" { print $2; exit }' go.mod)"
[ -n "$mask_ver" ] || die "go.mod does not require github.com/Liarea/lazyslice/mask"
git rev-parse -q --verify "refs/tags/mask/$mask_ver" >/dev/null ||
	die "go.mod requires mask $mask_ver but there is no mask/$mask_ver tag; cut it with make tag TAG=mask/$mask_ver"
git diff --quiet "mask/$mask_ver" HEAD -- mask/ ':(exclude,glob)mask/**/*.md' ||
	die "mask/ at HEAD differs from mask/$mask_ver, the version go.mod requires; cut the next mask/vX.Y.Z with make tag, bump the require, then tag the tool"
log "mask/ at HEAD is mask/$mask_ver, the version go.mod requires"
fi

# ---------- 5. every action the release workflow pins resolves ----------
# v0.0.1, 2026-09-22: sigstore/cosign-installer@v4 failed the job at "Set up
# job" because that action publishes v4.x.y tags and floating v3/v2, but no
# floating v4. Nothing but a tag exercises release.yml, so check here.
workflow=.github/workflows/release.yml
[ -f "$workflow" ] || die "$workflow is missing"
if [ "$MASK" -eq 0 ]; then
while IFS= read -r use; do
	[ -n "$use" ] || continue
	case "$use" in ./*) continue ;; esac # a local action has no ref to check
	ref="${use##*@}"
	path="${use%@*}"
	owner="${path%%/*}"
	rest="${path#*/}"
	repo="${rest%%/*}"
	api="repos/$owner/$repo"
	if gh api "$api/git/ref/tags/$ref" --jq .ref >/dev/null 2>&1 ||
		gh api "$api/git/ref/heads/$ref" --jq .ref >/dev/null 2>&1 ||
		{ [[ "$ref" =~ ^[0-9a-f]{40}$ ]] && gh api "$api/commits/$ref" --jq .sha >/dev/null 2>&1; }; then
		log "action resolves: $use"
	else
		die "$workflow pins $use, and $owner/$repo has no tag, branch or commit '$ref' (gh api $api/tags lists what it has)"
	fi
# POSIX classes, not \s: BSD sed on macOS does not know \s and left the
# "- uses:" prefix in place, so every action read as unresolvable.
done < <(grep -E '^[[:space:]]*-?[[:space:]]*uses:' "$workflow" |
	sed -E 's/^[[:space:]]*-?[[:space:]]*uses:[[:space:]]*//; s/[[:space:]]+#.*$//; s/["'"'"']//g')
fi

# ---------- 6. goreleaser accepts its configuration ----------
if [ "$MASK" -eq 0 ]; then
GORELEASER="$(command -v goreleaser || true)"
[ -n "$GORELEASER" ] || die "goreleaser is not on PATH (make tools)"
"$GORELEASER" check >/dev/null 2>&1 || die "goreleaser check rejects .goreleaser.yaml; run it for the reason"
log "goreleaser check passed"
fi

# ---------- 7. CI is green on HEAD ----------
# release.yml refuses a tag whose commit has no finished, successful ci run;
# finding that out after the tag is pushed would waste a tag number.
run_json="$(gh run list --commit "$sha" --workflow ci.yml --limit 5 --json databaseId,status,conclusion,createdAt)"
run_id="$(jq -r 'sort_by(.createdAt) | last | .databaseId // empty' <<<"$run_json")"
[ -n "$run_id" ] || die "no ci run exists for ${sha:0:7}; push it and let CI answer first"
run_status="$(jq -r --arg id "$run_id" '.[] | select(.databaseId == ($id|tonumber)) | .status' <<<"$run_json")"
if [ "$run_status" != "completed" ]; then
	log "ci run $run_id on ${sha:0:7} is $run_status; waiting for it"
	gh run watch "$run_id" --interval 30 --exit-status >/dev/null 2>&1 || true
fi
conclusion="$(gh run view "$run_id" --json conclusion --jq .conclusion)"
[ "$conclusion" = "success" ] || die "ci run $run_id on ${sha:0:7} concluded '$conclusion', not success; a red main is the next task"
log "ci run $run_id on ${sha:0:7} is green"

if [ "$DRY_RUN" -eq 1 ]; then
	log "dry run: every check passed; $TAG would be cut on ${sha:0:7}"
	exit 0
fi

# ---------- the tag ----------
git tag -a "$TAG" -m "$TAG" "$sha"
log "tagged ${sha:0:7} as $TAG"
git push origin "refs/tags/$TAG"
if [ "$MASK" -eq 1 ]; then
	log "pushed $TAG; nothing runs for a mask tag. go.mod may now require github.com/Liarea/lazyslice/mask ${TAG#mask/}"
	exit 0
fi
log "pushed $TAG; release.yml is starting"

release_id=""
for _ in $(seq 1 18); do
	# gh's --jq takes a program only (no --arg), so the tag is interpolated
	# into it; a tag matched the shape check above, so it carries no quote.
	release_id="$(gh run list --workflow release.yml --limit 5 --json databaseId,headBranch,createdAt --jq "[.[] | select(.headBranch == \"$TAG\")] | sort_by(.createdAt) | last | .databaseId // empty")"
	[ -n "$release_id" ] && break
	sleep 10
done
[ -n "$release_id" ] || die "the tag is pushed but no release run for $TAG appeared in three minutes; check the Actions tab"
release_url="$(gh run view "$release_id" --json url --jq .url)"
log "release run $release_id: $release_url"

if [ "$WATCH" -eq 0 ]; then
	exit 0
fi

gh run watch "$release_id" --interval 30 --exit-status >/dev/null 2>&1 || true
conclusion="$(gh run view "$release_id" --json conclusion --jq .conclusion)"
if [ "$conclusion" != "success" ]; then
	echo "tag: the release run for $TAG concluded '$conclusion': $release_url" >&2
	echo "tag: nothing to undo — a tag is immutable; fix the cause, then cut the next tag" >&2
	exit 1
fi
log "release run for $TAG succeeded"
gh release view "$TAG" --json url --jq '"==> tag: release: " + .url' 2>/dev/null || true
