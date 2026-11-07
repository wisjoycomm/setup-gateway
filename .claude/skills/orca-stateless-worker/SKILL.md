---
name: orca-stateless-worker
description: Execute exactly one task from PLAN.md as a parallel worker in an Orca git worktree, verify it, and reply with a one-line handover. Use when the prompt names a task ID such as T-01 to work on.
argument-hint: <TASK_ID>
arguments: [task_id]
allowed-tools: Edit Write Bash(pwsh -NoProfile -NonInteractive -File .claude/skills/orca-stateless-worker/scripts/fast_verify.ps1 *)
---

# Orca worker: one task, verify, report, stop

Your task: **$task_id**. If that is not an ID like `T-01`, use the task ID from the prompt. If there is none, reply
`TASK: ? | STATUS: BLOCKED | FILES: - | SUMMARY: no task ID given` and stop.

You are one of several workers running at the same time, each in its own git worktree on its own branch.
You can't see the other workers' changes; they are merged later. Keep the change small, scoped and verified.

## 1. Load only your task
- PLAN.md is one table, one row per task:
  `| ID | Task | Pri | SP | Status | Agent | Depends | Scope | Verify | Notes | Session |`.
  Grep `PLAN.md` for `| $task_id |` and read only that one line.
- Note its **Scope**, **Verify** and **Depends**.
- If any Depends ID still has a row in your copy of PLAN.md, it hasn't been merged yet and its code is not in this
  worktree: report BLOCKED with `depends on <ID>` and stop. (Merged tasks are moved out of PLAN.md to PLAN-ARCHIVE.md.)

## 2. Find, then read
- Locate code with Grep/Glob. Read only the line ranges you need.
- Before changing a function's signature or behavior, Grep its callers so you don't break them.

## 3. Change only what is in Scope
- Edit existing files; use Write only for new files, and only inside Scope.
- Unity: `.meta` files next to in-scope files are fine.
- If the task truly needs a change outside Scope, keep it minimal and name the file in SUMMARY as `+out-of-scope: <path>`.
- Never edit PLAN.md. Never commit, push, stash, reset or clean: leave your changes uncommitted for review.

## 4. Verify
From the worktree root run (for Unity projects set the Bash timeout to 600000):

```
pwsh -NoProfile -NonInteractive -File .claude/skills/orca-stateless-worker/scripts/fast_verify.ps1 -TaskId $task_id
```

Act on the **last line** of the output:

| Last line | Meaning | What to do |
|---|---|---|
| `VERIFY: PASS` | checks passed | go to step 5 |
| `VERIFY: FAIL (...)` | your change broke something; counts as 1 strike | fix and rerun; on the 3rd FAIL stop and report BLOCKED with the key error |
| `VERIFY: NOT_VERIFIED (...)` | nothing was checked; not a pass and not a strike | if the reason names a local setup step (e.g. an install command), do it and rerun; otherwise report BLOCKED with that reason |

Also:
- A `WARN out-of-scope:` line means you touched a file outside Scope: revert that change unless step 3's exception applies.
- If the command times out, rerun it once (a fresh Unity worktree imports slowly). A second timeout means BLOCKED `verify timeout`.

## 5. Report and stop
Your final message is exactly one line, with no greeting and no explanation:

```
TASK: <ID> | STATUS: PASS|PARTIAL|BLOCKED | FILES: <changed files, comma-separated> | SUMMARY: <15 words max>
```

Use PARTIAL when part of the task is done and verified but some of it is left; say what is left in SUMMARY.

Examples:

```
TASK: T-01 | STATUS: PASS | FILES: src/auth/jwt.py, tests/test_auth.py | SUMMARY: Added expiry check to validate_token
TASK: T-02 | STATUS: BLOCKED | FILES: src/utils/parser.py | SUMMARY: 3x FAIL test_nested_payload: KeyError 'items' in parse_payload
```

Then end your turn. The orchestrator records the result in PLAN.md.
