#!/usr/bin/env python3
# SPDX-License-Identifier: Apache-2.0
"""relnotes: release notes from commit bodies between two refs.

Every task commit in this repository carries a headline `stage: title (T-id)`
and a body of bullets that describe what a user now sees (docs/OPERATING_MODEL.md,
"Commit discipline"; .claude/skills/lazyslice-commit). This script collects those
bullets between two refs and groups them by the headline's stage prefix, so
`make relnotes FROM=v0.1.0 TO=v0.2.0` is the release notes and nobody rewrites
history to get them.

Usage: relnotes.py FROM TO [--no-other]
  FROM  the previous tag (or any ref); exclusive
  TO    the ref being released; inclusive
  --no-other  omit commits that carry no bullet body
"""
import re
import subprocess
import sys

SECTIONS = [
    ("Safety and masking", {"classify", "verify", "transform", "mask", "plan", "textsig"}),
    ("Pipeline", {"core", "extract", "load", "pg", "introspect", "emit", "discover", "tui", "cli"}),
    ("Build, CI and release", {"foundations", "hardening", "ci", "bench", "release"}),
]
SKIP = re.compile(r"^(Tracker|ROADMAP|docs?/|Tracker:|chore:)", re.I)


def commits(frm, to):
    out = subprocess.check_output(
        ["git", "log", "--no-merges", "--format=%H%x1e%s%x1e%b%x1f", f"{frm}..{to}"], text=True)
    for rec in out.split("\x1f"):
        rec = rec.strip("\n")
        if not rec.strip():
            continue
        sha, subject, body = (rec.split("\x1e") + ["", ""])[:3]
        yield sha.strip(), subject.strip(), body


def bullets(body):
    out = []
    for line in body.splitlines():
        if line.startswith("- "):
            out.append(line[2:].strip())
        elif out and line.startswith("  ") and line.strip():
            out[-1] += " " + line.strip()
    return out


def section_for(stage):
    for name, stages in SECTIONS:
        if stage in stages:
            return name
    return "Other changes"


def main(argv):
    if len(argv) < 3:
        sys.exit(__doc__)
    frm, to = argv[1], argv[2]
    no_other = "--no-other" in argv[3:]
    grouped = {name: [] for name, _ in SECTIONS}
    grouped["Other changes"] = []
    subjects_only = []
    for sha, subject, body in commits(frm, to):
        if SKIP.match(subject):
            continue
        m = re.match(r"^([a-z][a-z0-9/-]*):\s*(.*?)(?:\s*\((T-\d+[^)]*)\))?$", subject)
        stage, title, task = (m.group(1), m.group(2), m.group(3)) if m else ("other", subject, None)
        bs = bullets(body)
        if not bs:
            subjects_only.append((title, sha[:7]))
            continue
        grouped[section_for(stage)].append((title, task, bs, sha[:7]))
    print(f"## Changes from {frm} to {to}\n")
    for name in list(grouped):
        items = grouped[name]
        if not items:
            continue
        print(f"### {name}\n")
        for title, task, bs, sha in items:
            ref = f" ({task}, {sha})" if task else f" ({sha})"
            print(f"**{title}**{ref}")
            for b in bs:
                print(f"- {b}")
            print()
    if subjects_only and not no_other:
        print("### Without release-note bullets\n")
        for title, sha in subjects_only:
            print(f"- {title} ({sha})")
        print()


if __name__ == "__main__":
    main(sys.argv)
