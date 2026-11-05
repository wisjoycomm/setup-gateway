@AGENTS.md

## Claude Code only
- Search and read with your built-in tools instead of the shell table in AGENTS.md:
  - Grep (it runs ripgrep) for code search. Use output_mode `files_with_matches` or `count` and `head_limit` to keep results small.
  - Glob for finding files by name.
  - Read with offset/limit for line ranges. Edit only works on files opened with Read, so never read source with bat or cat.
- Use Bash only for tools with no built-in equivalent: `jq`, `eza`, `gh`, `git`, test runners.
- Edit: keep `old_string` to the smallest unique snippet (usually 2-5 lines). Don't stretch it to include the enclosing function header.
- Skills: `/orca-orchestrator <goal>` in the main repo folder, `/orca-stateless-worker T-01` in a worker worktree.
