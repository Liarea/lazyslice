#!/usr/bin/env python3
# SPDX-License-Identifier: Apache-2.0
"""Unit tests for tools/tracker.py (T-0196).

Every test replaces tracker.gh with a FakeGh that never touches the network or
the real repository: no test here may run a mutating command against
Liarea/lazyslice. FakeGh keeps a small in-memory model of GitHub (issues and
one Project v2's items) built from the real field-list and item shapes this
task's author captured with read-only `gh` calls against the live project, so
canned JSON matches what gh actually returns.

Run: python3 -m unittest discover -s tools -p 'test_tracker.py' -v
"""
import glob
import json
import os
import shutil
import subprocess
import sys
import tempfile
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import tracker  # noqa: E402

FIELDS = {
    "fields": [
        {"id": "F_STATUS", "name": "Status", "type": "ProjectV2SingleSelectField", "options": [
            {"id": "O_BACKLOG", "name": "Backlog"}, {"id": "O_READY", "name": "Ready"},
            {"id": "O_INPROG", "name": "In progress"}, {"id": "O_BLOCKED", "name": "Blocked"},
            {"id": "O_DONE", "name": "Done"}, {"id": "O_CANCELLED", "name": "Cancelled"},
        ]},
        {"id": "F_EPIC", "name": "Epic", "type": "ProjectV2SingleSelectField", "options": [
            {"id": "O_E0", "name": "E0 Frame"}, {"id": "O_E5", "name": "E5 Hardening"},
            {"id": "O_E6", "name": "E6 Launch"}, {"id": "O_E9", "name": "E9 Later"},
        ]},
        {"id": "F_OWNER", "name": "Owner", "type": "ProjectV2SingleSelectField", "options": [
            {"id": "O_HUMAN", "name": "human"}, {"id": "O_FABLE", "name": "fable"},
            {"id": "O_OPUS", "name": "opus"}, {"id": "O_SONNET", "name": "sonnet"}, {"id": "O_HAIKU", "name": "haiku"},
        ]},
        {"id": "F_KIND", "name": "Kind", "type": "ProjectV2SingleSelectField", "options": [
            {"id": "O_BUG", "name": "bug"}, {"id": "O_FEATURE", "name": "feature"},
            {"id": "O_SECURITY", "name": "security"}, {"id": "O_DECISION", "name": "decision"}, {"id": "O_CHORE", "name": "chore"},
        ]},
        {"id": "F_TID", "name": "Tracker id", "type": "ProjectV2Field"},
    ]
}


def parse_flags(args):
    """A tiny, generic --flag value / --flag(repeatable) parser good enough to
    dispatch the fake on, so the fake does not hardcode tracker.py's argument order."""
    positional, flags = [], {}
    i = 0
    while i < len(args):
        if args[i].startswith("--"):
            key = args[i][2:]
            if i + 1 < len(args) and not args[i + 1].startswith("--"):
                flags.setdefault(key, []).append(args[i + 1])
                i += 2
            else:
                flags.setdefault(key, []).append(True)
                i += 1
        else:
            positional.append(args[i])
            i += 1
    return positional, flags


class GhFailure(tracker.GhUnavailable):
    pass


class FakeGh:
    """Records every call; answers like the real gh CLI would for the calls
    tracker.py makes, against an in-memory issues/project-items model."""

    def __init__(self):
        self.calls = []
        self.issues = {}   # number -> dict(title, body, labels, milestone, state, comments)
        self.items = {}    # item id -> dict(content, epic, owner, kind, status, "tracker id", title, labels)
        self._next_number = 1
        self._next_item = 1
        self.fail = None  # set to a GhUnavailable to simulate an outage
        # The real repo's label set (T-0196 reviewer finding): three epic
        # labels, four owner labels (no "fable" — the Owner *field* has more
        # options than the repo has labels for), the fixed kind labels.
        self.labels = {
            "epic:E0-frame", "epic:E5-hardening", "epic:E6-launch", "epic:E9-later",
            "owner:human", "owner:opus", "owner:sonnet", "owner:haiku",
            "type:bug", "type:feature", "type:security", "type:decision", "type:chore",
            "filed-by-agent",
        }

    def field(self, field_id):
        for f in FIELDS["fields"]:
            if f["id"] == field_id:
                return f
        raise AssertionError(f"no such field id {field_id}")

    def option_name(self, field_id, option_id):
        for o in self.field(field_id).get("options", []):
            if o["id"] == option_id:
                return o["name"]
        raise AssertionError(f"no such option {option_id} on {field_id}")

    def __call__(self, *args, input_text=None):
        self.calls.append(args)
        if self.fail:
            raise self.fail
        pos, flags = parse_flags(args)
        top = tuple(pos[:2])

        if top == ("project", "field-list"):
            return json.dumps(FIELDS)
        if top == ("project", "view"):
            return json.dumps({"id": "PVT_PROJECT"})
        if top == ("project", "item-add"):
            url = flags["url"][0]
            number = int(url.rstrip("/").split("/")[-1])
            item_id = f"ITEM_{self._next_item}"
            self._next_item += 1
            issue = self.issues[number]
            self.items[item_id] = {
                "id": item_id, "content": {"number": number, "title": issue["title"], "body": issue["body"], "url": url},
                "title": issue["title"], "status": "Backlog", "epic": "", "owner": "", "kind": "", "tracker id": "",
            }
            return json.dumps({"id": item_id})
        if top == ("project", "item-edit"):
            item = self.items[flags["id"][0]]
            if "single-select-option-id" in flags:
                field = self.field(flags["field-id"][0])
                item[field["name"] if field["name"] != "Epic" else "epic"] = (
                    self.option_name(flags["field-id"][0], flags["single-select-option-id"][0])
                )
                # keep lower-case keys used elsewhere in this fake's model consistent with the real gh JSON casing
                name = field["name"]
                value = self.option_name(flags["field-id"][0], flags["single-select-option-id"][0])
                item[{"Status": "status", "Epic": "epic", "Owner": "owner", "Kind": "kind"}.get(name, name)] = value
            elif "text" in flags:
                field = self.field(flags["field-id"][0])
                if field["name"] == "Tracker id":
                    item["tracker id"] = flags["text"][0]
            return ""
        if top == ("project", "item-list"):
            return json.dumps({"items": list(self.items.values())})

        if top == ("issue", "create"):
            number = self._next_number
            self._next_number += 1
            self.issues[number] = {
                "title": flags["title"][0], "body": flags["body"][0],
                "labels": list(flags.get("label", [])), "milestone": flags.get("milestone", [None])[0],
                "state": "OPEN", "comments": [], "createdAt": "2026-09-01T00:00:00Z",
            }
            return f"https://github.com/{tracker.REPO}/issues/{number}"
        if top == ("issue", "comment"):
            number = int(pos[2])
            self.issues[number]["comments"].append({"body": flags["body"][0], "createdAt": "2026-09-17T00:00:00Z"})
            return ""
        if top == ("issue", "close"):
            number = int(pos[2])
            issue = self.issues[number]
            issue["state"] = "CLOSED"
            issue["comments"].append({"body": flags["comment"][0], "createdAt": "2026-09-17T00:00:00Z"})
            return ""
        if top == ("issue", "edit"):
            number = int(pos[2])
            issue = self.issues[number]
            for l in flags.get("remove-label", []):
                if l in issue["labels"]:
                    issue["labels"].remove(l)
            for l in flags.get("add-label", []):
                issue["labels"].append(l)
            if "milestone" in flags:
                issue["milestone"] = flags["milestone"][0]
            return ""
        if top == ("issue", "view"):
            number = int(pos[2])
            issue = self.issues[number]
            return json.dumps({
                "createdAt": issue["createdAt"], "body": issue["body"], "comments": issue["comments"],
                "title": issue["title"], "number": number,
                "milestone": {"title": issue["milestone"]} if issue["milestone"] else None, "state": issue["state"],
            })
        if top == ("issue", "list"):
            titles = [{"title": i["title"], "number": n, "state": i["state"],
                       "url": f"https://github.com/{tracker.REPO}/issues/{n}"}
                      for n, i in self.issues.items()]
            return json.dumps(titles)
        if top == ("label", "list"):
            return json.dumps([{"name": n} for n in sorted(self.labels)])

        raise AssertionError(f"FakeGh: unhandled call {args}")


class TrackerTestCase(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.mkdtemp()
        self.addCleanup(shutil.rmtree, self.tmp, ignore_errors=True)
        tracker.TASKS = os.path.join(self.tmp, "tasks")
        tracker.EPICS = os.path.join(self.tmp, "epics")
        tracker.BOARD = os.path.join(self.tmp, "BOARD.md")
        os.makedirs(tracker.TASKS)
        os.makedirs(tracker.EPICS)
        self.fake = FakeGh()
        self._real_gh = tracker.gh
        tracker.gh = self.fake
        tracker.reset_caches()
        self.addCleanup(setattr, tracker, "gh", self._real_gh)

    def run_cli(self, *args):
        tracker.main(list(args))

    def write_epic(self, eid="E5", title="Hardening"):
        capture(lambda: tracker.epic_new(_ns(id=eid, title=title, phase="5", goal="g")))


def _ns(**kw):
    import argparse
    return argparse.Namespace(**kw)


class NewAndListTest(TrackerTestCase):
    def test_new_prints_id_and_creates_ready_item(self):
        self.write_epic()
        out = capture(lambda: self.run_cli("new", "--epic", "E5", "--title", "Fix the thing", "--phase", "5", "--owner", "sonnet", "--goal", "g", "--accept", "a"))
        tid = out.strip().splitlines()[0]
        self.assertEqual(tid, "T-0001")
        item = next(iter(self.fake.items.values()))
        self.assertEqual(item["status"], "Ready")
        self.assertEqual(item["epic"], "E5 Hardening")
        self.assertEqual(item["owner"], "sonnet")
        self.assertIn("owner:sonnet", self.fake.issues[1]["labels"])
        self.assertIn("epic:E5-hardening", self.fake.issues[1]["labels"])

    def test_new_into_e9_without_owner_is_backlog_and_filed_by_agent(self):
        self.write_epic("E9", "Later")
        capture(lambda: self.run_cli("new", "--epic", "E9", "--title", "Some deferred idea", "--goal", "g"))
        item = next(iter(self.fake.items.values()))
        self.assertEqual(item["status"], "Backlog")
        self.assertIn("filed-by-agent", self.fake.issues[1]["labels"])

    def test_ids_increment_past_both_archive_and_github(self):
        self.write_epic()
        # Simulate an archived task T-0005 and an existing GitHub issue T-0003.
        tracker.write_archive("T-0005", "old", "E5", "5", "sonnet", "2026-01-01", "done", "done", "g", "a", ["- x"], "pm")
        self.fake.issues[99] = {"title": "T-0003: pre-existing", "body": "b", "labels": [], "milestone": None,
                                 "state": "OPEN", "comments": [], "createdAt": "2026-01-01T00:00:00Z"}
        out = capture(lambda: self.run_cli("new", "--epic", "E5", "--title", "next one", "--goal", "g"))
        self.assertEqual(out.strip().splitlines()[0], "T-0006")

    def test_list_shows_open_from_github_and_closed_from_archive(self):
        self.write_epic()
        capture(lambda: self.run_cli("new", "--epic", "E5", "--title", "Open task", "--phase", "5", "--goal", "g"))
        tracker.write_archive("T-0002", "Closed task", "E5", "5", "sonnet", "2026-01-01", "done", "done", "g", "a", ["- x"], "went well: x | went badly: y | change next time: z")
        out = capture(lambda: self.run_cli("list"))
        lines = out.strip().splitlines()
        self.assertTrue(any(l.startswith("T-0001") and "open" in l for l in lines))
        self.assertTrue(any(l.startswith("T-0002") and "done" in l for l in lines))

    def test_list_filters_by_status_and_epic(self):
        self.write_epic()
        self.write_epic("E9", "Later")
        capture(lambda: self.run_cli("new", "--epic", "E5", "--title", "In E5", "--goal", "g"))
        capture(lambda: self.run_cli("new", "--epic", "E9", "--title", "In E9", "--goal", "g"))
        out = capture(lambda: self.run_cli("list", "--epic", "E9"))
        self.assertIn("T-0002", out)
        self.assertNotIn("T-0001", out)

    def test_new_with_gh_unavailable_changes_nothing(self):
        self.write_epic()
        self.fake.fail = GhFailure("network is unreachable")
        with self.assertRaises(SystemExit):
            self.run_cli("new", "--epic", "E5", "--title", "X", "--goal", "g")
        self.assertEqual(self.fake.issues, {})
        self.assertFalse(os.path.exists(tracker.BOARD))
        self.assertEqual(glob.glob(os.path.join(tracker.TASKS, "T-*.md")), [])

    def test_new_with_unsupported_owner_skips_the_missing_label_instead_of_failing(self):
        # T-0196 reviewer finding: the Owner project field has a "fable"
        # option but the repo has no owner:fable label; gh issue create would
        # error on it, so it must be left off rather than passed.
        self.write_epic()
        out = capture(lambda: self.run_cli("new", "--epic", "E5", "--title", "Fable's task", "--owner", "fable", "--goal", "g"))
        tid = out.strip().splitlines()[0]
        self.assertEqual(tid, "T-0001")
        item = next(iter(self.fake.items.values()))
        self.assertEqual(item["owner"], "fable")  # project field is still set
        self.assertNotIn("owner:fable", self.fake.issues[1]["labels"])  # label just skipped

    def test_list_falls_back_to_board_when_gh_unavailable(self):
        self.write_epic()
        capture(lambda: self.run_cli("new", "--epic", "E5", "--title", "Offline row", "--goal", "g"))
        capture(lambda: self.run_cli("board"))
        self.fake.fail = GhFailure("network is unreachable")
        import contextlib
        import io
        out_buf, err_buf = io.StringIO(), io.StringIO()
        with contextlib.redirect_stdout(out_buf), contextlib.redirect_stderr(err_buf):
            self.run_cli("list")
        self.assertIn("network is unreachable", err_buf.getvalue())
        self.assertIn("T-0001", out_buf.getvalue())


class LifecycleTest(TrackerTestCase):
    def setUp(self):
        super().setUp()
        self.write_epic()
        capture(lambda: self.run_cli("new", "--epic", "E5", "--title", "Do the thing", "--phase", "5", "--owner", "sonnet", "--goal", "the goal", "--accept", "the acceptance"))

    def test_start_sets_in_progress_and_comments(self):
        capture(lambda: self.run_cli("start", "T-0001"))
        item = self.fake.items["ITEM_1"]
        self.assertEqual(item["status"], "In progress")
        self.assertTrue(any("started" in c["body"] for c in self.fake.issues[1]["comments"]))

    def test_log_adds_a_comment_without_changing_status(self):
        capture(lambda: self.run_cli("start", "T-0001"))
        capture(lambda: self.run_cli("log", "T-0001", "made progress"))
        self.assertTrue(any("made progress" in c["body"] for c in self.fake.issues[1]["comments"]))
        self.assertEqual(self.fake.items["ITEM_1"]["status"], "In progress")

    def test_block_sets_blocked(self):
        capture(lambda: self.run_cli("block", "T-0001", "--reason", "waiting on X"))
        self.assertEqual(self.fake.items["ITEM_1"]["status"], "Blocked")
        self.assertTrue(any("waiting on X" in c["body"] for c in self.fake.issues[1]["comments"]))

    def test_move_updates_epic_label_and_milestone(self):
        self.write_epic("E6", "Launch")
        capture(lambda: self.run_cli("move", "T-0001", "--epic", "E6", "--phase", "6"))
        self.assertEqual(self.fake.items["ITEM_1"]["epic"], "E6 Launch")
        self.assertEqual(self.fake.issues[1]["milestone"], "v0.2.0")
        self.assertIn("epic:E6-launch", self.fake.issues[1]["labels"])
        self.assertNotIn("epic:E5-hardening", self.fake.issues[1]["labels"])

    def test_move_into_e9_flips_status_to_backlog(self):
        self.write_epic("E9", "Later")
        capture(lambda: self.run_cli("move", "T-0001", "--epic", "E9"))
        self.assertEqual(self.fake.items["ITEM_1"]["status"], "Backlog")

    def test_close_writes_archive_file_and_closes_issue(self):
        out = capture(lambda: self.run_cli("close", "T-0001", "--outcome", "done", "--postmortem", "went well: x | went badly: y | change next time: z"))
        path = out.strip().splitlines()[0]
        self.assertTrue(os.path.exists(path))
        fm, body = tracker.parse(path)
        self.assertEqual(fm["status"], "done")
        self.assertEqual(fm["outcome"], "done")
        self.assertIn("the goal", body)
        self.assertIn("the acceptance", body)
        self.assertIn("went well: x", body)
        self.assertEqual(self.fake.issues[1]["state"], "CLOSED")
        self.assertEqual(self.fake.items["ITEM_1"]["status"], "Done")
        # closed task no longer appears as an open GitHub-backed row
        out2 = capture(lambda: self.run_cli("list"))
        self.assertTrue(any(l.startswith("T-0001") and " done " in l for l in out2.strip().splitlines()))

    def test_cancel_writes_archive_file_with_reason(self):
        out = capture(lambda: self.run_cli("cancel", "T-0001", "--reason", "no longer needed"))
        path = out.strip().splitlines()[0]
        fm, body = tracker.parse(path)
        self.assertEqual(fm["status"], "cancelled")
        self.assertIn("no longer needed", body)
        self.assertEqual(self.fake.issues[1]["state"], "CLOSED")

    def test_show_open_task_prints_body_and_comments(self):
        capture(lambda: self.run_cli("log", "T-0001", "a note"))
        out = capture(lambda: self.run_cli("show", "T-0001"))
        self.assertIn("Do the thing", out)
        self.assertIn("the goal", out)
        self.assertIn("a note", out)

    def test_show_closed_task_reads_archive_without_gh(self):
        capture(lambda: self.run_cli("close", "T-0001", "--outcome", "done", "--postmortem", "went well: x | went badly: y | change next time: z"))
        self.fake.fail = GhFailure("network is unreachable")
        out = capture(lambda: self.run_cli("show", "T-0001"))
        self.assertIn("Do the thing", out)

    def test_double_close_reports_already_closed(self):
        capture(lambda: self.run_cli("close", "T-0001", "--outcome", "done", "--postmortem", "went well: x | went badly: y | change next time: z"))
        with self.assertRaises(SystemExit) as cm:
            self.run_cli("close", "T-0001", "--outcome", "done", "--postmortem", "x")
        self.assertIn("already closed", str(cm.exception))

    def test_start_with_gh_unavailable_changes_nothing(self):
        before = list(self.fake.issues[1]["comments"])
        self.fake.fail = GhFailure("network is unreachable")
        with self.assertRaises(SystemExit):
            self.run_cli("start", "T-0001")
        self.assertEqual(self.fake.items["ITEM_1"]["status"], "Ready")
        self.assertEqual(self.fake.issues[1]["comments"], before)

    def test_log_with_gh_unavailable_changes_nothing(self):
        before = list(self.fake.issues[1]["comments"])
        self.fake.fail = GhFailure("network is unreachable")
        with self.assertRaises(SystemExit):
            self.run_cli("log", "T-0001", "a note")
        self.assertEqual(self.fake.issues[1]["comments"], before)

    def test_close_with_gh_unavailable_changes_nothing(self):
        self.fake.fail = GhFailure("network is unreachable")
        with self.assertRaises(SystemExit):
            self.run_cli("close", "T-0001", "--outcome", "done", "--postmortem", "went well: x | went badly: y | change next time: z")
        self.assertEqual(glob.glob(os.path.join(tracker.TASKS, "T-*.md")), [])
        self.assertEqual(self.fake.issues[1]["state"], "OPEN")
        self.assertEqual(self.fake.items["ITEM_1"]["status"], "Ready")

    def test_move_without_phase_leaves_milestone_stale(self):
        # T-0196 reviewer finding: move without --phase does not touch the
        # milestone, so it is silently left pointing at the old phase. This
        # pins today's behaviour rather than fixing it.
        self.write_epic("E6", "Launch")
        capture(lambda: self.run_cli("move", "T-0001", "--epic", "E6"))
        self.assertEqual(self.fake.items["ITEM_1"]["epic"], "E6 Launch")
        self.assertEqual(self.fake.issues[1]["milestone"], "v0.1.0")


class CloseJsonTest(TrackerTestCase):
    def test_close_json_closes_and_cancels(self):
        self.write_epic()
        capture(lambda: self.run_cli("new", "--epic", "E5", "--title", "A", "--goal", "g"))
        capture(lambda: self.run_cli("new", "--epic", "E5", "--title", "B", "--goal", "g"))
        data = [
            {"id": "T-0001", "status": "done", "outcome": "done", "postmortem": "went well: x | went badly: y | change next time: z"},
            {"id": "T-0002", "status": "cancelled", "reason": "dropped"},
        ]
        f = os.path.join(self.tmp, "close.json")
        json.dump(data, open(f, "w"))
        capture(lambda: self.run_cli("close-json", f))
        self.assertEqual(self.fake.issues[1]["state"], "CLOSED")
        self.assertEqual(self.fake.issues[2]["state"], "CLOSED")
        self.assertTrue(any(p.startswith(os.path.join(tracker.TASKS, "T-0001")) for p in glob.glob(os.path.join(tracker.TASKS, "*"))))


class BoardAndValidateTest(TrackerTestCase):
    def test_board_notes_it_is_generated(self):
        self.write_epic()
        capture(lambda: self.run_cli("new", "--epic", "E5", "--title", "A", "--goal", "g"))
        capture(lambda: self.run_cli("board"))
        text = open(tracker.BOARD).read()
        self.assertIn("generated", text.splitlines()[0])
        self.assertIn("T-0001", text)

    def test_validate_rejects_open_status_in_archive(self):
        tracker.dump(os.path.join(tracker.TASKS, "T-0001-bad.md"),
                     {"id": "T-0001", "title": "bad", "epic": "E5", "phase": "5", "status": "open",
                      "owner": "", "created": "2026-01-01", "started": "", "closed": "", "outcome": ""},
                     "\n## Goal\n\nx\n\n## Log\n\n- x\n\n## Post-mortem\n\ndone\n")
        with self.assertRaises(SystemExit):
            capture(lambda: self.run_cli("validate"))

    def test_validate_rejects_missing_postmortem(self):
        tracker.write_archive("T-0001", "t", "E5", "5", "sonnet", "2026-01-01", "done", "done", "g", "a", ["- x"], "_(filled on close: what went well, what went badly, what we change next time)_")
        with self.assertRaises(SystemExit):
            capture(lambda: self.run_cli("validate"))

    def test_validate_ok_on_clean_archive(self):
        tracker.write_archive("T-0001", "t", "E5", "5", "sonnet", "2026-01-01", "done", "done", "g", "a", ["- x"], "went well: x | went badly: y | change next time: z")
        with self.assertRaises(SystemExit) as cm:
            capture(lambda: self.run_cli("validate"))
        self.assertEqual(cm.exception.code, 0)


class MigrateTest(TrackerTestCase):
    def make_open_file(self, tid, title="A task", epic="E5", status="open"):
        path = os.path.join(tracker.TASKS, f"{tid}-{tracker.slug(title)}.md")
        tracker.dump(path, {"id": tid, "title": title, "epic": epic, "phase": "5", "status": status,
                             "owner": "sonnet", "created": "2026-01-01", "started": "", "closed": "", "outcome": ""},
                     f"\n## Goal\n\ngoal text\n\n## Acceptance\n\naccept text\n\n## Log\n\n- 2026-01-01 created\n\n## Post-mortem\n\n_(filled on close)_\n")
        return path

    def test_dry_run_creates_nothing_and_leaves_files(self):
        self.write_epic()
        p = self.make_open_file("T-0010")
        capture(lambda: self.run_cli("migrate", "--dry-run"))
        self.assertTrue(os.path.exists(p))
        self.assertEqual(self.fake.issues, {})

    def test_migrate_creates_missing_issue_and_removes_file(self):
        self.write_epic()
        p = self.make_open_file("T-0010")
        capture(lambda: self.run_cli("migrate"))
        self.assertFalse(os.path.exists(p))
        self.assertEqual(self.fake.issues[1]["title"], "T-0010: A task")

    def test_migrate_leaves_already_mirrored_task_alone_and_removes_only_the_file(self):
        self.write_epic()
        p = self.make_open_file("T-0011", title="Already mirrored")
        self.fake.issues[42] = {"title": "T-0011: Already mirrored", "body": "b", "labels": [], "milestone": None,
                                 "state": "OPEN", "comments": [], "createdAt": "2026-01-01T00:00:00Z"}
        capture(lambda: self.run_cli("migrate"))
        self.assertFalse(os.path.exists(p))
        self.assertEqual(len(self.fake.issues), 1)  # no duplicate was created

    def test_migrate_no_open_files_is_a_no_op(self):
        out = capture(lambda: self.run_cli("migrate", "--dry-run"))
        self.assertIn("nothing to do", out)

    def test_migrate_leaves_file_when_matching_issue_is_closed(self):
        # T-0196 reviewer finding (high): a title-prefix match alone is not a
        # verified match. A CLOSED issue with the same T-id must not cause the
        # open file to be deleted — that would make the task vanish from
        # every source of truth.
        self.write_epic()
        p = self.make_open_file("T-0012", title="Closed match")
        self.fake.issues[7] = {"title": "T-0012: Closed match", "body": "b", "labels": [], "milestone": None,
                                "state": "CLOSED", "comments": [], "createdAt": "2026-01-01T00:00:00Z"}
        out = capture(lambda: self.run_cli("migrate"))
        self.assertTrue(os.path.exists(p))
        self.assertEqual(len(self.fake.issues), 1)  # no duplicate issue created
        self.assertIn("not open", out)

    def test_migrate_adds_project_item_for_open_issue_outside_the_project(self):
        # An OPEN issue that was never added to Project 3 is invisible to
        # list/show/board; migrate must add it, not just delete the file.
        self.write_epic()
        p = self.make_open_file("T-0013", title="Open outside project")
        self.fake.issues[8] = {"title": "T-0013: Open outside project", "body": "b", "labels": [], "milestone": None,
                                "state": "OPEN", "comments": [], "createdAt": "2026-01-01T00:00:00Z"}
        capture(lambda: self.run_cli("migrate"))
        self.assertFalse(os.path.exists(p))
        self.assertTrue(any(it["content"]["number"] == 8 for it in self.fake.items.values()))


class GhWrapperTest(unittest.TestCase):
    """Exercises the real gh() wrapper's error classification, not the fake."""

    def test_missing_binary(self):
        def fake_run(*a, **kw):
            raise FileNotFoundError()
        old = subprocess.run
        subprocess.run = fake_run
        try:
            with self.assertRaises(tracker.GhUnavailable) as cm:
                tracker.gh("issue", "list")
            self.assertIn("not installed", str(cm.exception))
        finally:
            subprocess.run = old

    def test_unauthenticated(self):
        old = subprocess.run
        subprocess.run = lambda *a, **kw: _proc(1, "", "To authenticate, please run `gh auth login`.")
        try:
            with self.assertRaises(tracker.GhUnavailable) as cm:
                tracker.gh("issue", "list")
            self.assertIn("authenticated", str(cm.exception))
        finally:
            subprocess.run = old

    def test_network_down(self):
        old = subprocess.run
        subprocess.run = lambda *a, **kw: _proc(1, "", "dial tcp: lookup github.com: Temporary failure in name resolution")
        try:
            with self.assertRaises(tracker.GhUnavailable) as cm:
                tracker.gh("issue", "list")
            self.assertIn("network", str(cm.exception))
        finally:
            subprocess.run = old


def _proc(returncode, stdout, stderr):
    class P:
        pass
    p = P()
    p.returncode, p.stdout, p.stderr = returncode, stdout, stderr
    return p


class PureFunctionsTest(unittest.TestCase):
    def test_kind_of(self):
        self.assertEqual(tracker.kind_of("Should we use X or Y"), "decision")
        self.assertEqual(tracker.kind_of("A masked column leaks verbatim"), "security")
        self.assertEqual(tracker.kind_of("Loader fails on empty schema"), "bug")
        self.assertEqual(tracker.kind_of("README needs an update"), "chore")
        self.assertEqual(tracker.kind_of("Add support for arrays"), "feature")

    def test_status_label_roundtrip(self):
        for epic in ("E5", "E9"):
            for status in ("open", "in_progress", "blocked", "done", "cancelled"):
                label = tracker.status_label(epic, status)
                back = tracker.status_from_label(label)
                if status == "open":
                    self.assertEqual(back, "open")
                else:
                    self.assertEqual(back, status)

    def test_title_tid_and_strip_prefix(self):
        self.assertEqual(tracker.title_tid("T-0042: A title"), "T-0042")
        self.assertIsNone(tracker.title_tid("no id here"))
        self.assertEqual(tracker.strip_prefix("T-0042: A title", "T-0042"), "A title")


def capture(fn):
    import io
    import contextlib
    buf = io.StringIO()
    with contextlib.redirect_stdout(buf):
        fn()
    return buf.getvalue()


if __name__ == "__main__":
    unittest.main()
