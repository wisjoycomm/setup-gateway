# PLAN.md

Next ID: T-01

<!--
How this file works
- Only the orchestrator edits this file. Workers never touch it: they reply with one handover line,
  and the orchestrator copies it into Result and ticks the box.
- Queue holds at most ~15 fully written tasks. Later work waits in Backlog as one line each and is
  turned into tasks wave by wave. Done + merged tasks move to PLAN-ARCHIVE.md (one line each).
- Commit PLAN.md (and AGENTS.md, CLAUDE.md, .claude/, .agents/) before launching workers: a worktree only sees committed files.
- Tasks that run at the same time must not share files or folders in Scope.
- Scope: files or folders in backticks, e.g. `src/lib/cart.ts`, `src/app/api/login/`, `Assets/Scripts/Player/`.
  Globs such as `src/**/*.test.ts` also work.
- Verify: one non-interactive command, run from the repo root, exit code 0 = pass.
  Keep it simple enough for both cmd and bash (`&&` is fine).
  Leave it empty to auto-detect: Unity EditMode tests, then the package.json test script (or tsc --noEmit), then pytest, then go test.
- Agent: claude, codex or agy (empty = claude). Decides the launch command the orchestrator prints.
- Status: [ ] todo · [~] running · [x] done (not merged yet) · [!] blocked
- Orchestrator: /orca-orchestrator <goal or handover lines>    (Codex: $orca-orchestrator ...)
- Worker:       /orca-stateless-worker T-01                    (Codex: $orca-stateless-worker T-01)
-->

## Queue

## Backlog

- Machine-level defaults in `wj onboard`: global instructions and MCP servers for each agent
- `wj init -update` to refresh Orca skills in projects made with an older kit
- Make the Orca skill scripts work without `pwsh` on macOS and Linux
- Generate the `.claude` and `.agents` Orca skill copies from one source
- Confirm where Antigravity reads user-level skills and MCP servers on each OS
- Confirm Codex works end to end through the gateway, including `codex resume`
- Token overlay for macOS and Linux
