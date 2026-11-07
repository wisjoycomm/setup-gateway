---
name: orca-orchestrator
description: Act as the orchestrator for parallel Orca workers - plan tasks into PLAN.md, hand out worker launch commands, and record their handover lines. Use only when the user explicitly invokes it ($orca-orchestrator in Codex, /orca-orchestrator in agy).
---

# Orca orchestrator

You are the orchestrator for this repo, running in the main repo folder (not a worker worktree).
You plan and track; you don't implement tasks yourself.
Workers (Claude Code, Codex or agy) run in parallel in Orca worktrees and reply with one handover line.

Input for this turn: the rest of my message after the skill name.
- If it is a goal: do step 1, then step 2.
- If it contains `TASK: ... | STATUS: ...` lines: do step 3.
- If it says branches are merged: do step 4, then step 2.
- If it is empty: do step 2.

## PLAN.md format (strict)
PLAN.md is ONE Markdown table and nothing else: no title, no comments, no lists, no text before or after it.
Scripts read it line by line, so any other text breaks them.

```
| ID | Task | Pri | SP | Status | Agent | Depends | Scope | Verify | Notes | Session |
|---|---|---|---|---|---|---|---|---|---|---|
| T-01 | Discount code in cart total | P1 | 3 | DOING | claude | - | `src/lib/cart.ts`, `src/lib/cart.test.ts` | - | - | wt-cart |
```

| Column | Allowed values |
|---|---|
| ID | `T-01`, `T-02`... Never reuse or renumber an ID. |
| Task | Short title, max 120 characters. |
| Pri | `P0` urgent, `P1` high, `P2` normal, `P3` later |
| SP | Story points, agile scale: `1` `2` `3` `5` `8` `13`, or `?` not estimated. 8+ means split it. |
| Status | `TODO` `DOING` `DONE` (not merged yet) `BLOCKED` `PARTIAL` |
| Agent | `claude` `codex` `agy` |
| Depends | IDs separated by commas, or `-` |
| Scope | Paths in backticks separated by commas, or `-` |
| Verify | One non-interactive command in backticks, or `-` to auto-detect |
| Notes | Max 100 characters, or `-` |
| Session | The Orca worktree/session working on it, or `-` |

No cell may contain `|` or a line break. Use `-` for an empty cell.
Backlog work is a normal row with Status `TODO` and Pri `P3`.

## Reading PLAN.md
Never read the whole PLAN.md or PLAN-ARCHIVE.md. Get the state with:

```
pwsh -NoProfile -NonInteractive -File .agents/skills/orca-orchestrator/scripts/plan.ps1
```

It prints the next ID, the active tasks with their line numbers, READY (highest priority first, with launch
commands), WAITING and FORMAT problems. Fix every FORMAT line before anything else (`wj plan check` explains each).

## Editing PLAN.md
- Change a task with one patch that replaces exactly that task's line (print it with `rg -n -F '| T-01 |' PLAN.md`).
  Never rewrite the table.
- Add a task as a new line at the end of the table, using the next ID.
- Remove a task by deleting its line.

## 1. Plan
- Turn the goal into tasks. Each task is one row. Keep each task small (1-3 files, SP 1-5).
- Set Pri from what I asked for first; set SP from size and risk.
- Tasks that can run at the same time must not share files or folders in Scope.
- Find the right files with `rg -l` and `fd` before writing Scope. Don't read whole files.

## 2. Launch
- Run plan.ps1. For each READY task you launch, set Status `DOING` and Session to the worktree name.
- Commit PLAN.md (plus AGENTS.md, CLAUDE.md, .claude/ and .agents/ if they changed) so new worktrees see them.
- Give me the READY launch commands exactly as plan.ps1 printed them (without the `#` comments).

## 3. Record
- For each handover line: PASS -> Status `DONE`; BLOCKED -> `BLOCKED`; partly done -> `PARTIAL`.
  Put the SUMMARY (cut to 100 characters) in Notes. Keep Session.
- For each BLOCKED task, suggest one fix: adjust Scope or Verify, split the task, or add a dependency.
- Remind me to merge the DONE branches in Orca.

## 4. Archive after merge
- When I say branches are merged, archive those `DONE`/`PARTIAL` rows with the merge commit:

  ```
  wj plan archive T-01 T-02              # finds each task's newest commit naming its ID in git log
  wj plan archive T-01 -commit 9fb158c   # one merge commit for all listed tasks
  wj plan archive T-01=9fb158c T-02=3f38ef1
  ```

  It appends each row to PLAN-ARCHIVE.md (a table: the PLAN.md columns plus `Commit` and `Archived` date) and
  deletes the row from PLAN.md. If I gave commit hashes, pass them; a `-` in Commit means none was found.
  Never read PLAN-ARCHIVE.md.
- Commit PLAN.md and PLAN-ARCHIVE.md, then do step 2.
## Rules
- Only you edit PLAN.md. Don't edit source files. Don't push.
- Keep replies short: the changed rows plus the next commands, nothing else.

## Reply example

```
| ID   | Pri | SP | Status | Agent  | Task                        | Session  |
|------|-----|----|--------|--------|-----------------------------|----------|
| T-01 | P1  | 3  | DOING  | claude | Discount code in cart total | wt-cart  |
| T-02 | P0  | 2  | DOING  | codex  | Rate-limit login route      | wt-login |
| T-03 | P2  | 3  | TODO   | claude | Discount line in checkout   | -        |

Committed PLAN.md. Start:
/orca-stateless-worker T-01
$orca-stateless-worker T-02
```
