---
id: T-0280
title: "Launch-post drafts in docs/launch/: Show HN, r/PostgreSQL, r/devops, r/webdev, a blog post from the post-mortems, and ten places a listing PR is welcome"
epic: E6
phase: 6
status: done
owner: sonnet
created: 2026-09-22
started: ""
closed: 2026-09-22
outcome: done
---

# T-0280 · Launch-post drafts in docs/launch/: Show HN, r/PostgreSQL, r/devops, r/webdev, a blog post from the post-mortems, and ten places a listing PR is welcome

## Goal

docs/BUILD_PLAN.md PROMPT 6.3. Draft, for the maintainer to post (nothing is posted by this task): a Show HN title and first comment under 200 words that leads with the problem and the Snaplet and Neosync shutdowns (research/POSTMORTEMS.md; verify both are still shut as of the day, with links); a r/PostgreSQL post; a r/devops post; a r/webdev post; a short blog post 'What I learned reading Snaplet's and Neosync's issue trackers' drawn from research/POSTMORTEMS.md with every claim linked. Each post is honest that v0.1.0 is Postgres only and pseudonymises (README 'What a snapshot will not hide'), and ends with one specific question to readers. Also docs/launch/LISTINGS.md: ten awesome-lists and comparison pages where a PR adding lazyslice would be welcome, each with a link and the line the PR would add, checked to exist on the day. docs/CLAUDE.md currently says docs/launch/ has no assigned purpose: give it one there in the same change. Never write the maintainer's name; posts are signed 'the maintainer' where a signature is needed and the maintainer edits them.

## Acceptance

—

## Log

- 2026-09-22 2026-09-22 created
- 2026-09-22 closed: done

## Post-mortem

went well: six drafts and a listings file under docs/launch/, every shutdown and download figure re-fetched on the day, each post honest about Postgres-only and pseudonymisation and ending with one question; docs/CLAUDE.md now says what docs/launch/ is (5cba2bc) | went badly: the blog draft inherited two stale facts from research/POSTMORTEMS.md (a comment dated a year after a shutdown that was seven weeks, and an absolute npm figure that no longer held) and review caught both; the research file itself still carries them, outside the task's paths | change next time: a draft that cites a research document re-checks the research document's own claims it repeats, and files the correction for the research file in the same task (commit 5cba2bc; research fix filed as T-0285)
