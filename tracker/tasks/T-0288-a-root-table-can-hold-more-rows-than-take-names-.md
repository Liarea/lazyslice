---
id: T-0288
title: "A root table can hold more rows than --take names; say so in the flag's help or stop it"
epic: E6
phase: 6
status: done
owner: sonnet
created: 2026-09-22
started: ""
closed: 2026-09-22
outcome: done
---

# T-0288 · A root table can hold more rows than --take names; say so in the flag's help or stop it

## Goal

The first-run GIF (docs/media/first-run.gif, 2026-09-22) runs Pagila with --root public.customer -n 200 and the plan and load both report public.customer: 204 rows. cmd/lazyslice/main.go documents --take as 'Root rows, ORDER BY identity LIMIT N'. The likely cause: payment is a child of both customer and rental, and a payment whose customer_id differs from its rental's customer_id is pulled in through rental and then requires its own customer as a parent, so the root gains rows for referential completeness; verify that against the fixture (count payments where customer_id <> the rental's customer_id) before deciding. If that is it, the flag's help and ARCHITECTURE.md section 3 say the root holds at least N rows plus any the closure requires, and the plan line for the root says 'N chosen, M pulled in'; if it is not, it is a planner defect and gets a regression.

## Acceptance

—

## Log

- 2026-09-22 2026-09-22 created
- 2026-09-22 2026-09-22 moved to E6 phase 6
- 2026-09-22 closed: done

## Post-mortem

went well: d94e12b; the developer proved the cause on Pagila (payments whose own customer_id differs from their rental's pull 4 customers into a 200-customer root) and the root line now says 'root: 200 chosen, 4 pulled in by references'; make check, plan and core integration and CI run 35786031186 green | went badly: the first wording dropped the word root, which the plan screen matches exactly, so --tui lost its root-row strikethrough on exactly the Pagila run; the fix sat outside the task's paths and the E9 filing was refused five times by GitHub's GraphQL secondary limit, so the orchestrator landed it by hand (prefix kept, TUI accepts 'root: ', drift guard pins the format string) | change next time: a task that changes a plan.step Why must have internal/tui in its paths, since the plan screen parses that text
