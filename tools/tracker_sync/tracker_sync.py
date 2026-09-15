#!/usr/bin/env python3
# SPDX-License-Identifier: Apache-2.0
"""tracker_sync: one-way mirror of tracker/tasks into GitHub Issues and Project 3.

Until T-0196 reimplements tools/tracker.py over gh, the task files are the source of
truth and this script pushes them to GitHub: one issue per open or blocked task
(created on first sight), fields Epic, Owner, Kind, Status and Tracker id on the
project item, milestone from the phase, labels from epic, owner and kind. A task
that closes in the tracker closes its issue with the post-mortem as the comment.
Never the other direction: an edit on GitHub is overwritten by the next sync.

Usage: tracker_sync.py [--dry-run] [--all]   (--all also creates issues for done and cancelled tasks)
"""
import json
import os
import re
import subprocess
import sys
import tempfile

OWNER, REPO, PROJECT = "Liarea", "Liarea/lazyslice", "3"
ROOT = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
TASKS = os.path.join(ROOT, "tracker", "tasks")
DRY = "--dry-run" in sys.argv
ALL = "--all" in sys.argv


def gh(*args, input_text=None):
    if DRY and args[0] in ("issue", "project") and args[1] in ("create", "edit", "close", "item-add", "item-edit"):
        print("  dry:", " ".join(args)[:160])
        return ""
    return subprocess.check_output(["gh", *args], text=True, input=input_text).strip()


def parse(path):
    text = open(path, encoding="utf-8").read()
    fm, body = text.split("\n---\n", 1)[0], text.split("\n---\n", 1)[1]
    meta = {}
    for line in fm.splitlines():
        if ":" in line and not line.startswith("---"):
            k, v = line.split(":", 1)
            meta[k.strip()] = v.strip().strip('"')
    sections, current = {}, None
    for line in body.splitlines():
        if line.startswith("## "):
            current = line[3:].strip()
            sections[current] = []
        elif current:
            sections[current].append(line)
    for k in sections:
        sections[k] = "\n".join(sections[k]).strip()
    return meta, sections


def kind_of(title):
    t = title.lower()
    if re.search(r"decid|should |policy", t):
        return "decision"
    if re.search(r"leak|residu|not masked|crosses|verbatim|secret|passthrough|panick|refus|oracle|unmasked", t):
        return "security"
    if re.search(r"fail|flak|wrong|stale|does not|no longer|broken|collid|compares|missing|silent|never", t):
        return "bug"
    if re.search(r"docs?\b|readme|architecture\.md|claude\.md|record|regenerate|notes|comment", t):
        return "chore"
    return "feature"


def milestone_of(meta):
    if meta.get("epic") == "E9":
        return "Later"
    return {"5": "v0.1.0", "6": "v0.2.0", "7": "v1.0.0"}.get(meta.get("phase", ""), "")


def status_of(meta):
    s = meta.get("status", "open")
    if s == "open":
        return "Backlog" if meta.get("epic") == "E9" else "Ready"
    return {"blocked": "Blocked", "done": "Done", "cancelled": "Cancelled"}.get(s, "Backlog")


def body_of(meta, sec, fname):
    goal = sec.get("Goal", "").strip() or "_(no goal recorded)_"
    acc = sec.get("Acceptance", "").strip() or "—"
    log = sec.get("Log", "").strip()
    pm = sec.get("Post-mortem", "").strip()
    parts = [goal, "", "**Acceptance**", "", acc]
    if log:
        parts += ["", "**Log**", "", log]
    if pm and not pm.startswith("_("):
        parts += ["", "**Post-mortem**", "", pm]
    parts += ["", f"_Tracker file: `tracker/tasks/{fname}` — the file is the source of truth until T-0196 lands; edits here are overwritten by the next sync._"]
    return "\n".join(parts)


def main():
    fields = json.loads(gh("project", "field-list", PROJECT, "--owner", OWNER, "--format", "json", "--limit", "50"))["fields"]
    fid = {f["name"]: f for f in fields}
    project_id = json.loads(gh("project", "view", PROJECT, "--owner", OWNER, "--format", "json"))["id"]

    def opt(field, name):
        for o in fid[field].get("options", []):
            if o["name"] == name or o["name"].startswith(name + " "):
                return o["id"]
        return None

    issues = {}
    for it in json.loads(gh("issue", "list", "--repo", REPO, "--state", "all", "--limit", "1000", "--json", "number,title,state,url,labels,milestone")):
        m = re.match(r"^(T-\d+):", it["title"])
        if m:
            issues[m.group(1)] = it
    items = {}
    for it in json.loads(gh("project", "item-list", PROJECT, "--owner", OWNER, "--limit", "1000", "--format", "json"))["items"]:
        c = it.get("content") or {}
        if c.get("number"):
            items[c["number"]] = it["id"]

    created = updated = closed = 0
    for fname in sorted(os.listdir(TASKS)):
        if not fname.endswith(".md"):
            continue
        meta, sec = parse(os.path.join(TASKS, fname))
        tid, title, status = meta["id"], meta.get("title", ""), meta.get("status", "open")
        existing = issues.get(tid)
        if not existing and status not in ("open", "blocked") and not ALL:
            continue
        kind = kind_of(title)
        owner = meta.get("owner", "")
        labels = [f"type:{kind}"]
        epic = meta.get("epic", "")
        if epic in ("E5", "E6", "E9"):
            labels.append({"E5": "epic:E5-hardening", "E6": "epic:E6-launch", "E9": "epic:E9-later"}[epic])
        if owner in ("human", "opus", "sonnet", "haiku"):
            labels.append(f"owner:{owner}")
        if not owner:
            labels.append("filed-by-agent")
        ms = milestone_of(meta)
        body = body_of(meta, sec, fname)
        with tempfile.NamedTemporaryFile("w", suffix=".md", delete=False, encoding="utf-8") as tf:
            tf.write(body)
            bodyfile = tf.name
        try:
            if not existing:
                args = ["issue", "create", "--repo", REPO, "--title", f"{tid}: {title}", "--body-file", bodyfile, "--label", ",".join(labels)]
                if ms:
                    args += ["--milestone", ms]
                url = gh(*args)
                number = int(url.rstrip("/").split("/")[-1]) if url else 0
                created += 1
                print(f"created #{number} {tid}: {title[:60]}")
                if DRY:
                    continue
                item = json.loads(gh("project", "item-add", PROJECT, "--owner", OWNER, "--url", url, "--format", "json"))["id"]
            else:
                number = existing["number"]
                args = ["issue", "edit", str(number), "--repo", REPO, "--title", f"{tid}: {title}", "--body-file", bodyfile, "--add-label", ",".join(labels)]
                if ms:
                    args += ["--milestone", ms]
                gh(*args)
                updated += 1
                item = items.get(number)
                if not item and not DRY:
                    item = json.loads(gh("project", "item-add", PROJECT, "--owner", OWNER, "--url", existing["url"], "--format", "json"))["id"]
            if DRY or not item:
                continue
            for field, value in (("Status", status_of(meta)), ("Epic", epic), ("Owner", owner or None), ("Kind", kind)):
                oid = opt(field, value) if value else None
                if oid:
                    gh("project", "item-edit", "--project-id", project_id, "--id", item, "--field-id", fid[field]["id"], "--single-select-option-id", oid)
            gh("project", "item-edit", "--project-id", project_id, "--id", item, "--field-id", fid["Tracker id"]["id"], "--text", tid)
            if status in ("done", "cancelled") and (not existing or existing["state"] == "OPEN"):
                pm = sec.get("Post-mortem", "").strip()
                comment = ("Closed in the tracker as " + status + ". " + (pm if pm and not pm.startswith("_(") else "")).strip()
                gh("issue", "close", str(number), "--repo", REPO, "--comment", comment, *(["--reason", "not planned"] if status == "cancelled" else []))
                closed += 1
        finally:
            os.unlink(bodyfile)
    print(f"sync: {created} created, {updated} updated, {closed} closed")


if __name__ == "__main__":
    main()
