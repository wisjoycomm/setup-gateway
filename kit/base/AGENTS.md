# AGENTS.md

Rules for every coding agent in this repo (Claude Code, Codex, Antigravity/agy). Claude Code has extra notes in CLAUDE.md.
Parallel workers launched from Orca also follow the `orca-stateless-worker` skill.

## Read less
- Find before you read: locate files and symbols first, then read only the lines you need. Don't read whole files over ~200 lines.
- Never read lockfiles, generated files, build output or caches.
- Never read or search `PLAN-ARCHIVE.md` unless asked about old tasks.

## Shell tools
These are installed. Use them instead of grep, find, cat, ls and tree. All of them respect `.gitignore`.

| Need | Command |
|---|---|
| Search code | `rg -n --max-columns=200 -g '!PLAN-ARCHIVE.md' 'pattern' src/` (add `-l` for file names only, `-c` for counts, `-t ts` / `-t cs` to filter by language) |
| Find files | `fd -t f 'name'` or `fd -e ts . src` |
| Read a line range | `bat -p --paging=never -r 40:90 path/to/file` |
| Project layout | `eza --tree -L 2 --git-ignore --icons=never` |
| JSON keys | `jq '.scripts' package.json` |
| Git | `git --no-pager diff --stat`, `git --no-pager log --oneline -10` |
| GitHub | `gh pr view 12 --json title,body,files`, `gh issue view 34 --json title,body` |

Never run interactive tools: `lazygit`, `btop`, `fzf`, `vim`, `less`, zoxide's `z` (use real paths), or `gh` commands that would prompt (pass all flags).

## Edit, don't rewrite
- Change existing files with small targeted edits or patches, never by rewriting the whole file. Create whole files only when they are new.
- Keep each edited snippet as small as possible while still unique in the file.

## Files you create
- Temp and scratch files (logs, test output, one-off scripts, downloads) go in `.tmp/` at the repo root, which is gitignored. Never leave them in the repo root or next to source files. Delete them when done.
- Don't write summary or report files (`SUMMARY.md`, `CHANGES.md`, `NOTES.md`...) after a task: report in chat instead.
- Write docs only when asked, in `docs/`. When you change behavior that a file in `docs/` describes, update that file in the same change.

## Commands
- Non-interactive only: no `git rebase -i`, no `git commit` without `-m`.
- Tests run once, never in watch mode. Prefer quiet output.
- Never run `git push`, `git reset --hard` or `git clean` unless the user asks.

## Stop after 3 failures
If the same build or test check fails 3 times in a row, stop and report the exact error. Don't keep looping.
