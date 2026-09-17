#!/usr/bin/env python3
# SPDX-License-Identifier: Apache-2.0
"""lazyslice tracker: GitHub Issues and Projects for open work, markdown archive for closed history.

GitHub is the source of truth for a task that is open, in progress or blocked (T-0196).
tracker/tasks/*.md holds only closed and cancelled history; nothing writes there again
except this script archiving a task at the moment it closes. tracker/epics/*.md is
unaffected and still hand-managed here.

Every subcommand keeps its pre-T-0196 arguments and stdout contract:
  tools/tracker.py new       --epic E1 --title "..." [--phase N] [--owner opus] [--goal "..."] [--accept "..."]
                             # prints the new id on its own line
  tools/tracker.py start     T-0001
  tools/tracker.py log       T-0001 "note"
  tools/tracker.py block     T-0001 --reason "..."
  tools/tracker.py move      T-0001 --epic E5 --phase 5
  tools/tracker.py cancel    T-0001 --reason "..."
  tools/tracker.py close     T-0001 --outcome done --postmortem "went well | went badly | change next time"
  tools/tracker.py close-json FILE.json
  tools/tracker.py list      [--status open|in_progress|done|cancelled|blocked] [--epic E1]
                             # prints "id  status  epic  owner  title", one task per line
  tools/tracker.py show      T-0001          # title, fields, body, log comments (T-0196: replaces reading the file)
  tools/tracker.py board                     # rewrites tracker/BOARD.md from GitHub plus the archive
  tools/tracker.py validate                  # exit 1 on a malformed archive or epic file
  tools/tracker.py migrate   [--dry-run]     # one-time: move today's open task files onto GitHub
  tools/tracker.py epic new  --id E1 --title "..." [--phase N]

Every subcommand that talks to GitHub goes through gh() below, and through it alone,
so tests can replace it with a fake that never touches the network (see test_tracker.py).
A mutating command that cannot reach GitHub (gh missing, unauthenticated, or the network
down) exits non-zero with one sentence saying which and changes nothing locally. `list`
and `show` fall back to the archive plus the last tracker/BOARD.md instead, and say so.

Only the orchestrator writes here. Agents return structured results; the orchestrator
records them. The one exception root CLAUDE.md carries is a developer or reviewer filing
its own out-of-scope finding with `new --epic E9`.
"""
import argparse
import datetime
import glob
import json
import os
import re
import subprocess
import sys

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
TASKS = os.path.join(ROOT, "tracker", "tasks")
EPICS = os.path.join(ROOT, "tracker", "epics")
BOARD = os.path.join(ROOT, "tracker", "BOARD.md")
STATUSES = ("open", "in_progress", "done", "cancelled", "blocked")

# The GitHub side of the mirror T-0196 replaces: one repo, one Project v2, moved
# here from tools/tracker_sync/tracker_sync.py rather than duplicated.
OWNER, REPO, PROJECT = "Liarea", "Liarea/lazyslice", "3"
OWNER_TIERS = ("human", "opus", "sonnet", "haiku", "fable")
MILESTONE_BY_PHASE = {"5": "v0.1.0", "6": "v0.2.0", "7": "v1.0.0"}
PHASE_BY_MILESTONE = {v: k for k, v in MILESTONE_BY_PHASE.items()}


class GhUnavailable(Exception):
    """gh could not do what we asked: missing, unauthenticated, or no network."""


def today():
    return datetime.date.today().isoformat()


def slug(s):
    return re.sub(r"[^a-z0-9]+", "-", s.lower()).strip("-")[:48]


# ---------------------------------------------------------------------------
# gh: the one function that shells out. Tests replace this name directly
# (tracker.gh = fake) so no subcommand ever needs its own seam.
# ---------------------------------------------------------------------------
def gh(*args, input_text=None):
    try:
        proc = subprocess.run(["gh", *args], input=input_text, capture_output=True, text=True)
    except FileNotFoundError:
        raise GhUnavailable("gh is not installed; install the GitHub CLI (https://cli.github.com) to use the tracker")
    if proc.returncode != 0:
        err = (proc.stderr or "").strip()
        low = err.lower()
        if "gh auth login" in low or "authentication" in low or "not logged" in low:
            raise GhUnavailable("gh is not authenticated; run 'gh auth login' (and 'gh auth refresh -s project' for the board)")
        if any(s in low for s in ("could not resolve host", "network is unreachable", "connection refused", "dial tcp", "timeout", "temporary failure in name resolution")):
            raise GhUnavailable("could not reach GitHub; check the network connection")
        raise GhUnavailable(f"gh failed: {err or ('exit ' + str(proc.returncode))}")
    return proc.stdout.strip()


# ---------------------------------------------------------------------------
# Archive: tracker/tasks/*.md, closed and cancelled history only, plus
# tracker/epics/*.md, which this task does not move to GitHub.
# ---------------------------------------------------------------------------
def quote(v):
    # A double-quoted YAML scalar escapes an inner double quote; T-0188's reviewer found a title that a real YAML reader rejects (2026-09-15).
    return str(v).replace('\\', '\\\\').replace('"', '\\"')


def unquote(v):
    if len(v) >= 2 and v[0] == '"' and v[-1] == '"':
        return v[1:-1].replace('\\"', '"').replace('\\\\', '\\')
    return v.strip('"')


def parse(path):
    text = open(path, encoding="utf-8").read()
    m = re.match(r"^---\n(.*?)\n---\n(.*)$", text, re.S)
    if not m:
        raise ValueError(f"{path}: missing frontmatter")
    fm = {}
    for line in m.group(1).splitlines():
        if ":" in line:
            k, v = line.split(":", 1)
            fm[k.strip()] = unquote(v.strip())
    return fm, m.group(2)


def dump(path, fm, body):
    head = "\n".join(f'{k}: "{quote(v)}"' if " " in str(v) or '"' in str(v) or v == "" else f"{k}: {v}" for k, v in fm.items())
    open(path, "w", encoding="utf-8").write(f"---\n{head}\n---\n{body}")


def archive_path(tid):
    hits = glob.glob(os.path.join(TASKS, f"{tid}-*.md")) + glob.glob(os.path.join(TASKS, f"{tid}.md"))
    return hits[0] if hits else None


def find_archive(tid):
    path = archive_path(tid)
    if not path:
        sys.exit(f"no task {tid}")
    return path


def archive_tasks():
    out = []
    for p in sorted(glob.glob(os.path.join(TASKS, "T-*.md"))):
        fm, _ = parse(p)
        out.append(fm)
    return out


def sections_of(body):
    """Tolerant section splitter: '## Name' or a lone '**Name**' line starts a
    section. Text before the first header (tracker_sync's body_of left the
    goal unlabelled) becomes the 'Goal' section, so an issue body written by
    either generation reads back sensibly."""
    header_re = re.compile(r"^(?:##\s*(?P<h1>.+?)|\*\*(?P<h2>[^*]+)\*\*)\s*$")
    sections, current, buf = {}, "Goal", []
    for line in body.splitlines():
        m = header_re.match(line.strip())
        if m:
            sections[current] = "\n".join(buf).strip()
            current = (m.group("h1") or m.group("h2")).strip()
            buf = []
        else:
            buf.append(line)
    sections[current] = "\n".join(buf).strip()
    return sections


def write_archive(tid, title, epic, phase, owner, created, status, outcome, goal, accept, log_lines, postmortem):
    os.makedirs(TASKS, exist_ok=True)
    path = os.path.join(TASKS, f"{tid}-{slug(title)}.md")
    fm = {
        "id": tid, "title": title, "epic": epic, "phase": phase or "", "status": status,
        "owner": owner or "", "created": created or today(), "started": "", "closed": today(), "outcome": outcome,
    }
    log = "\n".join(log_lines) if log_lines else f"- {today()} closed: {outcome}"
    body = (
        f"\n# {tid} · {title}\n\n## Goal\n\n{goal or ''}\n\n## Acceptance\n\n{accept or ''}\n\n"
        f"## Log\n\n{log}\n\n## Post-mortem\n\n{postmortem}\n"
    )
    dump(path, fm, body)
    return path


# ---------------------------------------------------------------------------
# GitHub mapping: status/epic/owner/kind <-> Project v2 fields and labels,
# moved from tools/tracker_sync/tracker_sync.py (T-0196 deletes that file).
# ---------------------------------------------------------------------------
_FIELDS_CACHE = None
_PROJECT_ID_CACHE = None
_LABELS_CACHE = None


def reset_caches():
    """Tests call this between cases so a fake gh's call count is predictable."""
    global _FIELDS_CACHE, _PROJECT_ID_CACHE, _LABELS_CACHE
    _FIELDS_CACHE = None
    _PROJECT_ID_CACHE = None
    _LABELS_CACHE = None


def get_labels():
    """The repo's actual label names. The Epic and Owner project fields carry
    more options (e.g. E7, 'fable') than the repo has labels for, so an
    epic/owner label is only worth requesting when it is one gh already knows
    about — see epic_label() and cmd_new/create_issue_for/cmd_move below."""
    global _LABELS_CACHE
    if _LABELS_CACHE is None:
        data = json.loads(gh("label", "list", "--repo", REPO, "--json", "name", "--limit", "200"))
        _LABELS_CACHE = {l["name"] for l in data}
    return _LABELS_CACHE


def label_exists(name):
    return bool(name) and name in get_labels()


def get_fields():
    global _FIELDS_CACHE
    if _FIELDS_CACHE is None:
        data = json.loads(gh("project", "field-list", PROJECT, "--owner", OWNER, "--format", "json", "--limit", "50"))
        _FIELDS_CACHE = {f["name"]: f for f in data["fields"]}
    return _FIELDS_CACHE


def project_id():
    global _PROJECT_ID_CACHE
    if _PROJECT_ID_CACHE is None:
        _PROJECT_ID_CACHE = json.loads(gh("project", "view", PROJECT, "--owner", OWNER, "--format", "json"))["id"]
    return _PROJECT_ID_CACHE


def field_option_id(field_name, option_name):
    for o in get_fields().get(field_name, {}).get("options", []):
        if o["name"] == option_name:
            return o["id"]
    return None


def epic_option_name(epic_id):
    for o in get_fields().get("Epic", {}).get("options", []):
        if o["name"].split(" ", 1)[0] == epic_id:
            return o["name"]
    return None


def epic_label(epic_id):
    name = epic_option_name(epic_id)
    if not name:
        return None
    rest = name.split(" ", 1)[1] if " " in name else ""
    return f"epic:{epic_id}-{slug(rest)}" if rest else f"epic:{epic_id.lower()}"


def set_select_field(item_id, field_name, option_name):
    if not option_name:
        return
    oid = field_option_id(field_name, option_name)
    if not oid:
        return
    gh("project", "item-edit", "--project-id", project_id(), "--id", item_id,
       "--field-id", get_fields()[field_name]["id"], "--single-select-option-id", oid)


def set_text_field(item_id, field_name, text):
    fields = get_fields()
    if field_name not in fields:
        return
    gh("project", "item-edit", "--project-id", project_id(), "--id", item_id,
       "--field-id", fields[field_name]["id"], "--text", text)


def kind_of(title):
    # Ported verbatim from tracker_sync.py's kind_of.
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


def milestone_for(epic, phase):
    if epic == "E9":
        return "Later"
    return MILESTONE_BY_PHASE.get(str(phase or ""), "")


def phase_from_milestone(ms):
    if not ms:
        return ""
    return PHASE_BY_MILESTONE.get(ms.get("title", ""), "")


def status_label(epic, status):
    if status == "open":
        return "Backlog" if epic == "E9" else "Ready"
    return {"in_progress": "In progress", "blocked": "Blocked", "done": "Done", "cancelled": "Cancelled"}[status]


def status_from_label(label):
    return {"Backlog": "open", "Ready": "open", "In progress": "in_progress", "Blocked": "blocked",
            "Done": "done", "Cancelled": "cancelled"}.get(label, "open")


def title_tid(title):
    m = re.match(r"^(T-\d+):", title or "")
    return m.group(1) if m else None


def strip_prefix(title, tid):
    return title[len(tid) + 1:].strip() if (title or "").startswith(tid + ":") else (title or "")


def task_body(goal, accept):
    return f"{goal or '_(no goal recorded)_'}\n\n**Acceptance**\n\n{accept or chr(0x2014)}"


def project_items():
    return json.loads(gh("project", "item-list", PROJECT, "--owner", OWNER, "--limit", "1000", "--format", "json"))["items"]


def find_project_item(tid):
    for it in project_items():
        if it.get("tracker id") == tid or title_tid(it.get("title", "")) == tid:
            return it
    return None


def require_open_item(tid):
    it = find_project_item(tid)
    if it is not None and status_from_label(it.get("status", "")) not in ("done", "cancelled"):
        return it
    if archive_path(tid):
        sys.exit(f"{tid} is already closed; see tracker/tasks/ (the closed-history archive)")
    if it is not None:
        sys.exit(f"{tid} is closed on GitHub but was not archived; run 'tools/tracker.py migrate' or file this as a bug")
    sys.exit(f"no open task {tid}")


def issue_detail(number):
    return json.loads(gh("issue", "view", str(number), "--repo", REPO,
                          "--json", "createdAt,body,comments,title,number,milestone,state"))


def epic_short(item_epic):
    return (item_epic or "").split(" ", 1)[0]


def board_open_rows():
    """Parsed from the last committed tracker/BOARD.md, for the offline fallback."""
    if not os.path.exists(BOARD):
        return []
    line_re = re.compile(r"^- (T-\d+) \[(\w+)\] (\S+) · (.+?) \(([^)]*)\)$")
    rows = []
    for line in open(BOARD, encoding="utf-8"):
        m = line_re.match(line.strip())
        if m:
            tid, status, epic, title, owner = m.groups()
            rows.append({"id": tid, "status": status, "epic": epic, "owner": owner, "title": title})
    return rows


def refresh_board_or_warn():
    try:
        board()
    except GhUnavailable as e:
        print(f"tracker: the change above landed on GitHub, but tracker/BOARD.md could not be refreshed ({e})", file=sys.stderr)


# ---------------------------------------------------------------------------
# Subcommands
# ---------------------------------------------------------------------------
def epic_new(a):
    os.makedirs(EPICS, exist_ok=True)
    path = os.path.join(EPICS, f"{a.id}.md")
    if os.path.exists(path):
        sys.exit(f"{a.id} exists")
    dump(path, {"id": a.id, "title": a.title, "phase": a.phase or "", "status": "open", "created": today(), "closed": ""},
         f"\n# {a.id} · {a.title}\n\n## Goal\n\n{a.goal or ''}\n\n## Outcome\n\n_(filled on close)_\n\n## Post-mortem\n\n_(filled on close)_\n")
    print(path)


def cmd_new(a):
    tid = next_id()
    kind = kind_of(a.title)
    labels = [f"type:{kind}"]
    el = epic_label(a.epic)
    if el and label_exists(el):
        labels.append(el)
    owner = a.owner or ""
    if owner in OWNER_TIERS and label_exists(f"owner:{owner}"):
        labels.append(f"owner:{owner}")
    if not owner:
        labels.append("filed-by-agent")
    ms = milestone_for(a.epic, a.phase)
    args = ["issue", "create", "--repo", REPO, "--title", f"{tid}: {a.title}", "--body", task_body(a.goal, a.accept)]
    for l in labels:
        args += ["--label", l]
    if ms:
        args += ["--milestone", ms]
    url = gh(*args)
    number = url.rstrip("/").split("/")[-1]
    try:
        item_id = json.loads(gh("project", "item-add", PROJECT, "--owner", OWNER, "--url", url, "--format", "json"))["id"]
        set_select_field(item_id, "Status", status_label(a.epic, "open"))
        set_select_field(item_id, "Epic", epic_option_name(a.epic))
        if owner:
            set_select_field(item_id, "Owner", owner)
        set_select_field(item_id, "Kind", kind)
        set_text_field(item_id, "Tracker id", tid)
    except GhUnavailable as e:
        sys.exit(f"tracker: {e}; {tid}'s issue was created at {url} but not added to project {PROJECT} — "
                 f"run 'gh project item-add {PROJECT} --owner {OWNER} --url {url}' and set its Tracker id field to {tid}")
    gh("issue", "comment", number, "--repo", REPO, "--body", f"{today()} created")
    print(tid)
    refresh_board_or_warn()


def cmd_start(a):
    it = require_open_item(a.id)
    number = it["content"]["number"]
    gh("issue", "comment", str(number), "--repo", REPO, "--body", f"{today()} started")
    set_select_field(it["id"], "Status", status_label(epic_short(it.get("epic")), "in_progress"))
    print(it["content"]["url"])
    refresh_board_or_warn()


def cmd_log(a):
    it = require_open_item(a.id)
    number = it["content"]["number"]
    gh("issue", "comment", str(number), "--repo", REPO, "--body", f"{today()} {a.note}")
    print(it["content"]["url"])
    refresh_board_or_warn()


def cmd_block(a):
    it = require_open_item(a.id)
    number = it["content"]["number"]
    gh("issue", "comment", str(number), "--repo", REPO, "--body", f"{today()} blocked: {a.reason}")
    set_select_field(it["id"], "Status", "Blocked")
    print(it["content"]["url"])
    refresh_board_or_warn()


def cmd_move(a):
    it = require_open_item(a.id)
    number = it["content"]["number"]
    old_epic = epic_short(it.get("epic"))
    new_epic = a.epic
    old_label, new_label = epic_label(old_epic), epic_label(new_epic)
    edit_args = ["issue", "edit", str(number), "--repo", REPO]
    if old_label and old_label != new_label and label_exists(old_label):
        edit_args += ["--remove-label", old_label]
    if new_label and label_exists(new_label):
        edit_args += ["--add-label", new_label]
    ms = milestone_for(new_epic, a.phase)
    if ms:
        edit_args += ["--milestone", ms]
    if len(edit_args) > 5:  # something to edit beyond the bare "issue edit N --repo R"
        gh(*edit_args)
    set_select_field(it["id"], "Epic", epic_option_name(new_epic))
    current_status = status_from_label(it.get("status", ""))
    set_select_field(it["id"], "Status", status_label(new_epic, current_status))
    # The narrative comment goes last: everything above is the mutation that
    # must actually succeed before we tell a reader it happened.
    gh("issue", "comment", str(number), "--repo", REPO, "--body", f"{today()} moved to {new_epic} phase {a.phase or ''}".rstrip())
    print(it["content"]["url"])
    refresh_board_or_warn()


def cmd_close(a):
    it = require_open_item(a.id)
    number = it["content"]["number"]
    detail = issue_detail(number)
    epic = epic_short(it.get("epic"))
    owner = it.get("owner", "") or ""
    title = strip_prefix(detail["title"], a.id)
    sec = sections_of(detail.get("body", ""))
    log_lines = [f"- {c['createdAt'][:10]} {c['body']}" for c in detail.get("comments", [])]
    log_lines.append(f"- {today()} closed: {a.outcome}")
    set_select_field(it["id"], "Status", status_label(epic, "done"))
    gh("issue", "close", str(number), "--repo", REPO, "--comment", a.postmortem)
    path = write_archive(a.id, title, epic, phase_from_milestone(detail.get("milestone")), owner,
                          detail.get("createdAt", "")[:10], "done", a.outcome,
                          sec.get("Goal", ""), sec.get("Acceptance", ""), log_lines, a.postmortem)
    print(path)
    refresh_board_or_warn()


def cmd_cancel(a):
    it = require_open_item(a.id)
    number = it["content"]["number"]
    detail = issue_detail(number)
    epic = epic_short(it.get("epic"))
    owner = it.get("owner", "") or ""
    title = strip_prefix(detail["title"], a.id)
    sec = sections_of(detail.get("body", ""))
    log_lines = [f"- {c['createdAt'][:10]} {c['body']}" for c in detail.get("comments", [])]
    log_lines.append(f"- {today()} cancelled: {a.reason}")
    postmortem = f"Cancelled. Reason: {a.reason}"
    set_select_field(it["id"], "Status", status_label(epic, "cancelled"))
    gh("issue", "close", str(number), "--repo", REPO, "--comment", postmortem, "--reason", "not planned")
    path = write_archive(a.id, title, epic, phase_from_milestone(detail.get("milestone")), owner,
                          detail.get("createdAt", "")[:10], "cancelled", "cancelled",
                          sec.get("Goal", ""), sec.get("Acceptance", ""), log_lines, postmortem)
    print(path)
    refresh_board_or_warn()


def close_json(a):
    data = json.load(open(a.file))
    for item in data:
        if item.get("status") == "cancelled":
            cmd_cancel(argparse.Namespace(id=item["id"], reason=item.get("reason", "")))
        else:
            cmd_close(argparse.Namespace(id=item["id"], outcome=item.get("outcome", "done"), postmortem=item.get("postmortem", "")))


def next_id():
    archive_ids = []
    for p in glob.glob(os.path.join(TASKS, "T-*.md")):
        m = re.match(r"T-(\d+)", os.path.basename(p))
        if m:
            archive_ids.append(int(m.group(1)))
    titles = json.loads(gh("issue", "list", "--repo", REPO, "--state", "all", "--json", "title", "--limit", "1000"))
    issue_ids = [int(m.group(1)) for t in titles if (m := re.match(r"^T-(\d+):", t["title"]))]
    all_ids = archive_ids + issue_ids
    return f"T-{(max(all_ids) + 1 if all_ids else 1):04d}"


def list_(a):
    rows = []
    try:
        for it in project_items():
            status = status_from_label(it.get("status", ""))
            if status in ("done", "cancelled"):
                continue
            tid = it.get("tracker id") or title_tid(it.get("title", ""))
            if not tid:
                continue
            rows.append({"id": tid, "status": status, "epic": epic_short(it.get("epic")),
                         "owner": it.get("owner", "") or "", "title": strip_prefix(it.get("title", ""), tid)})
    except GhUnavailable as e:
        print(f"tracker: {e}; showing the archive and the last tracker/BOARD.md for open work", file=sys.stderr)
        rows = board_open_rows()
    for fm in archive_tasks():
        rows.append({"id": fm["id"], "status": fm["status"], "epic": fm["epic"], "owner": fm.get("owner", ""), "title": fm["title"]})
    rows.sort(key=lambda r: int(r["id"][2:]))
    for r in rows:
        if a.status and r["status"] != a.status:
            continue
        if a.epic and r["epic"] != a.epic:
            continue
        print(f"{r['id']}  {r['status']:<12} {r['epic']:<5} {r['owner']:<8} {r['title']}")


def show(a):
    path = archive_path(a.id)
    if path:
        fm, body = parse(path)
        print(f"{fm['id']}: {fm['title']}")
        print(f"status={fm['status']} epic={fm['epic']} phase={fm.get('phase','')} owner={fm.get('owner','')} outcome={fm.get('outcome','')}")
        print()
        print(body.strip())
        return
    try:
        it = find_project_item(a.id)
        if it is None:
            sys.exit(f"no task {a.id}")
        detail = issue_detail(it["content"]["number"])
        title = strip_prefix(detail["title"], a.id)
        print(f"{a.id}: {title}")
        print(f"status={status_from_label(it.get('status',''))} epic={epic_short(it.get('epic'))} "
              f"owner={it.get('owner','') or ''} kind={it.get('kind','')} "
              f"milestone={(detail.get('milestone') or {}).get('title','')}")
        print()
        print((detail.get("body") or "").strip())
        if detail.get("comments"):
            print()
            print("Log:")
            for c in detail["comments"]:
                print(f"- {c['createdAt'][:10]} {c['body']}")
    except GhUnavailable as e:
        row = next((r for r in board_open_rows() if r["id"] == a.id), None)
        if not row:
            sys.exit(f"tracker: {e}; {a.id} is not in the archive or the last tracker/BOARD.md")
        print(f"tracker: {e}; showing the last tracker/BOARD.md", file=sys.stderr)
        print(f"{row['id']}: {row['title']}")
        print(f"status={row['status']} epic={row['epic']} owner={row['owner']}")


def board(a=None):
    open_rows = []
    for it in project_items():
        status = status_from_label(it.get("status", ""))
        if status in ("done", "cancelled"):
            continue
        tid = it.get("tracker id") or title_tid(it.get("title", ""))
        if not tid:
            continue
        open_rows.append({"id": tid, "status": status, "epic": epic_short(it.get("epic")),
                           "owner": it.get("owner", "") or "", "title": strip_prefix(it.get("title", ""), tid)})
    archived = archive_tasks()
    epics = {}
    for p in sorted(glob.glob(os.path.join(EPICS, "*.md"))):
        fm, _ = parse(p)
        epics[fm["id"]] = fm
    counted = open_rows + archived
    lines = [f"# Board · generated {today()} by tools/tracker.py (source: GitHub project {PROJECT} plus tracker/tasks/) — do not hand-edit\n",
             "| Epic | Phase | Open | In progress | Done | Cancelled | Blocked |", "|---|---|---|---|---|---|---|"]
    for eid, e in epics.items():
        c = {s: sum(1 for t in counted if t["epic"] == eid and t["status"] == s) for s in STATUSES}
        lines.append(f"| {eid} {e['title']} | {e.get('phase','')} | {c['open']} | {c['in_progress']} | {c['done']} | {c['cancelled']} | {c['blocked']} |")
    lines += ["", "## Open and in progress", ""]
    for t in sorted(open_rows, key=lambda t: int(t["id"][2:])):
        lines.append(f"- {t['id']} [{t['status']}] {t['epic']} · {t['title']} ({t.get('owner','')})")
    lines += ["", "## Recently closed", ""]
    for t in sorted([t for t in archived if t["status"] in ("done", "cancelled")], key=lambda t: t.get("closed", ""), reverse=True)[:25]:
        lines.append(f"- {t['id']} [{t['status']}] {t['epic']} · {t['title']} → {t.get('outcome','')}")
    open(BOARD, "w").write("\n".join(lines) + "\n")
    print(BOARD)


def validate(a):
    bad = 0
    pending_migration = 0
    for p in glob.glob(os.path.join(TASKS, "*.md")):
        try:
            fm, body = parse(p)
            if fm.get("status") in ("open", "in_progress", "blocked"):
                # Pre-T-0196 open work still on disk: a real defect once migrate
                # has run, but expected while it hasn't — call it out on its own
                # rather than folding it into "N invalid" with no next step.
                pending_migration += 1
                continue
            if fm.get("status") not in ("done", "cancelled"):
                raise ValueError(f"archived task has unrecognised status {fm.get('status')!r}; "
                                  "tracker/tasks/ is closed-history only since T-0196, open work lives on GitHub")
            if "_(filled on close" in body:
                raise ValueError("closed without post-mortem")
        except Exception as e:
            bad += 1
            print(f"INVALID {p}: {e}")
    for p in glob.glob(os.path.join(EPICS, "*.md")):
        try:
            parse(p)
        except Exception as e:
            bad += 1
            print(f"INVALID {p}: {e}")
    if pending_migration:
        bad += pending_migration
        print(f"{pending_migration} open task file(s) still await tools/tracker.py migrate; "
              "run that before treating them as invalid")
    print("ok" if not bad else f"{bad} invalid")
    sys.exit(1 if bad else 0)


def create_issue_for(tid, fm, body):
    """migrate's helper: creates the GitHub issue for a task file that pre-dates
    the mirror, preserving its own id rather than assigning a new one."""
    sec = sections_of(body)
    kind = kind_of(fm["title"])
    labels = [f"type:{kind}"]
    el = epic_label(fm["epic"])
    if el and label_exists(el):
        labels.append(el)
    owner = fm.get("owner", "")
    if owner in OWNER_TIERS and label_exists(f"owner:{owner}"):
        labels.append(f"owner:{owner}")
    if not owner:
        labels.append("filed-by-agent")
    ms = milestone_for(fm["epic"], fm.get("phase"))
    args = ["issue", "create", "--repo", REPO, "--title", f"{tid}: {fm['title']}",
            "--body", task_body(sec.get("Goal", ""), sec.get("Acceptance", ""))]
    for l in labels:
        args += ["--label", l]
    if ms:
        args += ["--milestone", ms]
    url = gh(*args)
    number = url.rstrip("/").split("/")[-1]
    item_id = json.loads(gh("project", "item-add", PROJECT, "--owner", OWNER, "--url", url, "--format", "json"))["id"]
    set_select_field(item_id, "Status", status_label(fm["epic"], fm["status"]))
    set_select_field(item_id, "Epic", epic_option_name(fm["epic"]))
    if owner:
        set_select_field(item_id, "Owner", owner)
    set_select_field(item_id, "Kind", kind)
    set_text_field(item_id, "Tracker id", tid)
    log = sec.get("Log", "")
    if log:
        gh("issue", "comment", number, "--repo", REPO, "--body", log)
    return url


def migrate(a):
    open_files = []
    for p in sorted(glob.glob(os.path.join(TASKS, "T-*.md"))):
        fm, body = parse(p)
        if fm.get("status") in ("open", "in_progress", "blocked"):
            open_files.append((p, fm, body))
    if not open_files:
        print("migrate: no open, in-progress or blocked task files under tracker/tasks/; nothing to do")
        return

    try:
        items = project_items()
        # state and url too: a title match alone isn't a verified match — see below.
        issues = json.loads(gh("issue", "list", "--repo", REPO, "--state", "all",
                                "--json", "title,state,url", "--limit", "1000"))
    except GhUnavailable as e:
        sys.exit(f"tracker: {e}")

    by_tid = {(it.get("tracker id") or title_tid(it.get("title", ""))): it for it in items}
    issue_by_tid = {}
    for t in issues:
        m = re.match(r"^(T-\d+):", t["title"])
        if m:
            issue_by_tid[m.group(1)] = t

    # A file is only safe to remove once its match is verified: the issue
    # itself must be OPEN (a CLOSED or MERGED title match means the file's
    # task and the issue are not the same live work), and it must have a
    # project item (added here if the issue predates being put on the
    # project) that is not Done/Cancelled — list/show/board/require_open_item
    # all read only open project items, so anything short of that would make
    # the task vanish from every source of truth instead of moving.
    to_remove, to_create, to_skip = [], [], []
    for p, fm, body in open_files:
        tid = fm["id"]
        issue = issue_by_tid.get(tid)
        if issue is None:
            to_create.append((p, fm, body))
            continue
        if issue.get("state") != "OPEN":
            to_skip.append((p, tid, f"migrate: {tid} matches {issue.get('url', 'an issue')} but it is "
                                     f"{issue.get('state', 'not open').lower()}, not open; leaving {p} in place "
                                     "(reopen the issue or file a report; migrate never deletes on a closed match)"))
            continue
        item = by_tid.get(tid)
        if item is not None and status_from_label(item.get("status", "")) in ("done", "cancelled"):
            to_skip.append((p, tid, f"migrate: {tid} matches {issue.get('url', 'an issue')} but its project card "
                                     f"is {item.get('status')}; leaving {p} in place"))
            continue
        to_remove.append({"path": p, "tid": tid, "issue": issue, "item": item})

    if a.dry_run:
        for r in to_remove:
            note = "" if r["item"] is not None else " (not yet on the project; would add it)"
            print(f"migrate --dry-run: {r['tid']} already has an open GitHub issue{note}; would remove {r['path']}")
        for p, fm, _ in to_create:
            print(f"migrate --dry-run: {fm['id']} has no GitHub issue; would create one, then remove {p}")
        for p, tid, msg in to_skip:
            print(msg)
        print(f"migrate --dry-run: {len(to_remove)} matched, {len(to_create)} would be created, "
              f"{len(to_skip)} left in place unmatched, 0 files removed (dry run moves nothing)")
        return

    created = 0
    for p, fm, body in to_create:
        try:
            url = create_issue_for(fm["id"], fm, body)
        except GhUnavailable as e:
            print(f"migrate: could not create an issue for {fm['id']} ({e}); leaving {p} in place")
            continue
        print(f"migrate: created {url} for {fm['id']}")
        to_remove.append({"path": p, "tid": fm["id"], "issue": None, "item": None})
        created += 1

    for p, tid, msg in to_skip:
        print(msg)

    removed = 0
    for r in to_remove:
        p, tid, issue, item = r["path"], r["tid"], r["issue"], r["item"]
        if issue is not None and item is None:
            try:
                json.loads(gh("project", "item-add", PROJECT, "--owner", OWNER, "--url", issue["url"], "--format", "json"))
            except GhUnavailable as e:
                print(f"migrate: {tid} has an open issue {issue['url']} but could not be added to project "
                      f"{PROJECT} ({e}); leaving {p} in place")
                continue
        os.remove(p)
        removed += 1
        print(f"migrate: removed {p} ({tid} now lives on GitHub)")

    refresh_board_or_warn()
    print(f"migrate: {created} issue(s) created, {removed} file(s) removed from tracker/tasks/, "
          f"{len(open_files) - removed} left in place (could not be matched or created)")


# ---------------------------------------------------------------------------
# CLI wiring
# ---------------------------------------------------------------------------
def build_parser():
    p = argparse.ArgumentParser()
    sub = p.add_subparsers(dest="cmd", required=True)

    e = sub.add_parser("epic")
    es = e.add_subparsers(dest="ecmd", required=True)
    en = es.add_parser("new")
    en.add_argument("--id", required=True)
    en.add_argument("--title", required=True)
    en.add_argument("--phase")
    en.add_argument("--goal")
    en.set_defaults(f=epic_new)

    n = sub.add_parser("new")
    n.add_argument("--epic", required=True)
    n.add_argument("--title", required=True)
    n.add_argument("--phase")
    n.add_argument("--owner")
    n.add_argument("--goal")
    n.add_argument("--accept")
    n.set_defaults(f=cmd_new)

    s = sub.add_parser("start")
    s.add_argument("id")
    s.set_defaults(f=cmd_start)

    l = sub.add_parser("log")
    l.add_argument("id")
    l.add_argument("note")
    l.set_defaults(f=cmd_log)

    c = sub.add_parser("close")
    c.add_argument("id")
    c.add_argument("--outcome", required=True)
    c.add_argument("--postmortem", required=True)
    c.set_defaults(f=cmd_close)

    x = sub.add_parser("cancel")
    x.add_argument("id")
    x.add_argument("--reason", required=True)
    x.set_defaults(f=cmd_cancel)

    b = sub.add_parser("block")
    b.add_argument("id")
    b.add_argument("--reason", required=True)
    b.set_defaults(f=cmd_block)

    li = sub.add_parser("list")
    li.add_argument("--status")
    li.add_argument("--epic")
    li.set_defaults(f=list_)

    sh = sub.add_parser("show")
    sh.add_argument("id")
    sh.set_defaults(f=show)

    sub.add_parser("board").set_defaults(f=board)
    sub.add_parser("validate").set_defaults(f=validate)

    mv = sub.add_parser("move")
    mv.add_argument("id")
    mv.add_argument("--epic", required=True)
    mv.add_argument("--phase")
    mv.set_defaults(f=cmd_move)

    cj = sub.add_parser("close-json")
    cj.add_argument("file")
    cj.set_defaults(f=close_json)

    mg = sub.add_parser("migrate")
    mg.add_argument("--dry-run", action="store_true")
    mg.set_defaults(f=migrate)

    return p


def main(argv=None):
    a = build_parser().parse_args(argv)
    try:
        a.f(a)
    except GhUnavailable as e:
        sys.exit(f"tracker: {e}")


if __name__ == "__main__":
    main()
