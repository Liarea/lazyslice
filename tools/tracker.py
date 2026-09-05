#!/usr/bin/env python3
"""lazysnap tracker: epics, tasks, post-mortems as markdown with YAML-ish frontmatter.

Usage:
  tools/tracker.py epic new  --id E1 --title "..." [--phase N]
  tools/tracker.py new       --epic E1 --title "..." [--phase N] [--owner opus] [--goal "..."] [--accept "..."]
  tools/tracker.py start     T-0001
  tools/tracker.py log       T-0001 "note"
  tools/tracker.py close     T-0001 --outcome done --postmortem "went well | went badly | change next time"
  tools/tracker.py cancel    T-0001 --reason "..."
  tools/tracker.py block     T-0001 --reason "..."
  tools/tracker.py list      [--status open|in_progress|done|cancelled|blocked] [--epic E1]
  tools/tracker.py board                     # rewrites tracker/BOARD.md
  tools/tracker.py validate                  # exit 1 on malformed files
  tools/tracker.py close-json FILE.json      # bulk close from a workflow result

Only the orchestrator writes here. Agents return structured results; the orchestrator records them.
"""
import argparse, datetime, json, os, re, sys, glob

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
TASKS = os.path.join(ROOT, "tracker", "tasks")
EPICS = os.path.join(ROOT, "tracker", "epics")
BOARD = os.path.join(ROOT, "tracker", "BOARD.md")
STATUSES = ("open", "in_progress", "done", "cancelled", "blocked")

def today(): return datetime.date.today().isoformat()
def slug(s): return re.sub(r"[^a-z0-9]+", "-", s.lower()).strip("-")[:48]

def parse(path):
    text = open(path, encoding="utf-8").read()
    m = re.match(r"^---\n(.*?)\n---\n(.*)$", text, re.S)
    if not m: raise ValueError(f"{path}: missing frontmatter")
    fm = {}
    for line in m.group(1).splitlines():
        if ":" in line:
            k, v = line.split(":", 1); fm[k.strip()] = v.strip().strip('"')
    return fm, m.group(2)

def dump(path, fm, body):
    head = "\n".join(f'{k}: "{v}"' if " " in str(v) or v == "" else f"{k}: {v}" for k, v in fm.items())
    open(path, "w", encoding="utf-8").write(f"---\n{head}\n---\n{body}")

def find(tid):
    hits = glob.glob(os.path.join(TASKS, f"{tid}-*.md")) + glob.glob(os.path.join(TASKS, f"{tid}.md"))
    if not hits: sys.exit(f"no task {tid}")
    return hits[0]

def next_id():
    ids = [int(re.match(r"T-(\d+)", os.path.basename(p)).group(1)) for p in glob.glob(os.path.join(TASKS, "T-*.md"))]
    return f"T-{(max(ids) + 1 if ids else 1):04d}"

def epic_new(a):
    os.makedirs(EPICS, exist_ok=True)
    path = os.path.join(EPICS, f"{a.id}.md")
    if os.path.exists(path): sys.exit(f"{a.id} exists")
    dump(path, {"id": a.id, "title": a.title, "phase": a.phase or "", "status": "open", "created": today(), "closed": ""},
         f"\n# {a.id} · {a.title}\n\n## Goal\n\n{a.goal or ''}\n\n## Outcome\n\n_(filled on close)_\n\n## Post-mortem\n\n_(filled on close)_\n")
    print(path)

def task_new(a):
    os.makedirs(TASKS, exist_ok=True)
    tid = next_id()
    path = os.path.join(TASKS, f"{tid}-{slug(a.title)}.md")
    dump(path, {"id": tid, "title": a.title, "epic": a.epic, "phase": a.phase or "", "status": "open",
                "owner": a.owner or "", "created": today(), "started": "", "closed": "", "outcome": ""},
         f"\n# {tid} · {a.title}\n\n## Goal\n\n{a.goal or ''}\n\n## Acceptance\n\n{a.accept or ''}\n\n## Log\n\n"
         f"- {today()} created\n\n## Post-mortem\n\n_(filled on close: what went well, what went badly, what we change next time)_\n")
    print(tid)

def set_status(tid, status, note=None, extra=None):
    path = find(tid); fm, body = parse(path)
    fm["status"] = status
    if status == "in_progress" and not fm.get("started"): fm["started"] = today()
    if status in ("done", "cancelled"): fm["closed"] = today()
    if extra: fm.update(extra)
    if note: body = body.replace("## Post-mortem", f"- {today()} {note}\n\n## Post-mortem", 1)
    dump(path, fm, body); return path, fm, body

def close(a):
    path, fm, body = set_status(a.id, "done", f"closed: {a.outcome}", {"outcome": a.outcome})
    body = re.sub(r"## Post-mortem\n\n.*$", f"## Post-mortem\n\n{a.postmortem}\n", body, flags=re.S)
    dump(path, fm, body); print(path)

def cancel(a):
    path, fm, body = set_status(a.id, "cancelled", f"cancelled: {a.reason}", {"outcome": "cancelled"})
    body = re.sub(r"## Post-mortem\n\n.*$", f"## Post-mortem\n\nCancelled. Reason: {a.reason}\n", body, flags=re.S)
    dump(path, fm, body); print(path)

def close_json(a):
    data = json.load(open(a.file))
    for item in data:
        ns = argparse.Namespace(id=item["id"], outcome=item.get("outcome", "done"), postmortem=item.get("postmortem", ""))
        if item.get("status") == "cancelled": cancel(argparse.Namespace(id=item["id"], reason=item.get("reason", "")))
        else: close(ns)

def all_tasks():
    out = []
    for p in sorted(glob.glob(os.path.join(TASKS, "T-*.md"))):
        fm, _ = parse(p); out.append(fm)
    return out

def list_(a):
    for fm in all_tasks():
        if a.status and fm["status"] != a.status: continue
        if a.epic and fm["epic"] != a.epic: continue
        print(f"{fm['id']}  {fm['status']:<12} {fm['epic']:<5} {fm.get('owner',''):<8} {fm['title']}")

def board(a=None):
    tasks = all_tasks(); epics = {}
    for p in sorted(glob.glob(os.path.join(EPICS, "*.md"))):
        fm, _ = parse(p); epics[fm["id"]] = fm
    lines = [f"# Board · {today()}\n", "| Epic | Phase | Open | In progress | Done | Cancelled | Blocked |", "|---|---|---|---|---|---|---|"]
    for eid, e in epics.items():
        c = {s: sum(1 for t in tasks if t["epic"] == eid and t["status"] == s) for s in STATUSES}
        lines.append(f"| {eid} {e['title']} | {e.get('phase','')} | {c['open']} | {c['in_progress']} | {c['done']} | {c['cancelled']} | {c['blocked']} |")
    lines += ["", "## Open and in progress", ""]
    for t in tasks:
        if t["status"] in ("open", "in_progress", "blocked"):
            lines.append(f"- {t['id']} [{t['status']}] {t['epic']} · {t['title']} ({t.get('owner','')})")
    lines += ["", "## Recently closed", ""]
    for t in sorted([t for t in tasks if t["status"] in ("done", "cancelled")], key=lambda t: t.get("closed", ""), reverse=True)[:25]:
        lines.append(f"- {t['id']} [{t['status']}] {t['epic']} · {t['title']} → {t.get('outcome','')}")
    open(BOARD, "w").write("\n".join(lines) + "\n"); print(BOARD)

def validate(a):
    bad = 0
    for p in glob.glob(os.path.join(TASKS, "*.md")) + glob.glob(os.path.join(EPICS, "*.md")):
        try:
            fm, body = parse(p)
            if fm.get("status") not in STATUSES: raise ValueError(f"bad status {fm.get('status')}")
            if fm["status"] in ("done", "cancelled") and "_(filled on close" in body: raise ValueError("closed without post-mortem")
        except Exception as e:
            bad += 1; print(f"INVALID {p}: {e}")
    print("ok" if not bad else f"{bad} invalid"); sys.exit(1 if bad else 0)

p = argparse.ArgumentParser(); sub = p.add_subparsers(dest="cmd", required=True)
e = sub.add_parser("epic"); es = e.add_subparsers(dest="ecmd", required=True); en = es.add_parser("new")
en.add_argument("--id", required=True); en.add_argument("--title", required=True); en.add_argument("--phase"); en.add_argument("--goal"); en.set_defaults(f=epic_new)
n = sub.add_parser("new"); n.add_argument("--epic", required=True); n.add_argument("--title", required=True); n.add_argument("--phase"); n.add_argument("--owner"); n.add_argument("--goal"); n.add_argument("--accept"); n.set_defaults(f=task_new)
s = sub.add_parser("start"); s.add_argument("id"); s.set_defaults(f=lambda a: print(set_status(a.id, "in_progress", "started")[0]))
l = sub.add_parser("log"); l.add_argument("id"); l.add_argument("note"); l.set_defaults(f=lambda a: print(set_status(a.id, parse(find(a.id))[0]["status"], a.note)[0]))
c = sub.add_parser("close"); c.add_argument("id"); c.add_argument("--outcome", required=True); c.add_argument("--postmortem", required=True); c.set_defaults(f=close)
x = sub.add_parser("cancel"); x.add_argument("id"); x.add_argument("--reason", required=True); x.set_defaults(f=cancel)
b = sub.add_parser("block"); b.add_argument("id"); b.add_argument("--reason", required=True); b.set_defaults(f=lambda a: print(set_status(a.id, "blocked", f"blocked: {a.reason}")[0]))
li = sub.add_parser("list"); li.add_argument("--status"); li.add_argument("--epic"); li.set_defaults(f=list_)
sub.add_parser("board").set_defaults(f=board)
sub.add_parser("validate").set_defaults(f=validate)
cj = sub.add_parser("close-json"); cj.add_argument("file"); cj.set_defaults(f=close_json)
a = p.parse_args(); a.f(a)
if a.cmd not in ("board", "validate", "list"): board()
