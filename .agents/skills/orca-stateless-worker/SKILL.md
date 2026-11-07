---
name: orca-stateless-worker
description: Execute exactly one task from PLAN.md as a parallel worker in an Orca git worktree, verify it, and reply with a one-line handover. Use when the prompt names a task ID such as T-01 to work on.
---

# Orca worker: one task, verify, report, stop

Your task ID is in the prompt (e.g. `T-01`). If there is none, reply
`TASK: ? | STATUS: BLOCKED | FILES: - | SUMMARY: no task ID given` and stop.

You are one of several workers running at the same time, each in its own git worktree on its own branch.
You can't see the other workers' changes; they are merged later. Keep the change small, scoped and verified.

## 1. Load only your task
- PLAN.md is one table, one row per task:
  `| ID | Task | Pri | SP | Status | Agent | Depends | Scope | Verify | Notes | Session |`.
  Print only your row: `rg -n -F '| T-01 |' PLAN.md` (with your ID).
- Note its **Scope**, **Verify** and **Depends**.
- If any Depends ID still has a row in PLAN.md, it hasn't been merged yet and its code is not in this
  worktree: report BLOCKED with `depends on <ID>` and stop. (Merged tasks are moved out of PLAN.md to PLAN-ARCHIVE.md.)

## 2. Find, then read
- Locate code with `rg -n` and `fd` (see the table in AGENTS.md). Read only the ranges you need with `bat -p --paging=never -r A:B`.
- Before changing a function's signature or behavior, `rg -n 'name('` for its callers so you don't break them.

## 3. Change only what is in Scope
- Change existing files with small patches; create files only inside Scope.
- Unity: `.meta` files next to in-scope files are fine.
- If the task truly needs a change outside Scope, keep it minimal and name the file in SUMMARY as `+out-of-scope: <path>`.
- Never edit PLAN.md. Never commit, push, stash, reset or clean: leave your changes uncommitted for review.

## 4. Verify
From the worktree root run (allow up to 10 minutes for Unity projects):

```
pwsh -NoProfile -NonInteractive -File .agents/skills/orca-stateless-worker/scripts/fast_verify.ps1 -TaskId T-01
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
