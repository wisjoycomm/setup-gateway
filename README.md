# Wisjoy setup-gateway

One standalone CLI for setting up a development machine: the original
**Bo_cong_cu** toolkit plus Claude authentication and custom API gateways.
Windows, macOS and Linux builds embed all toolkit scripts. No Go or Node.js is
needed to run the downloaded executable.

Vietnamese instructions: [HUONG-DAN.md](HUONG-DAN.md).

## Start on a new Windows machine

Open **CMD** and paste this single command. It downloads and verifies the release,
installs it into your user PATH, then opens the setup menu immediately:

```bat
powershell -NoProfile -ExecutionPolicy Bypass -Command "& ([scriptblock]::Create((irm 'https://github.com/wisjoycomm/setup-gateway/releases/latest/download/install.ps1'))) -RunSetup"
```

No manual download or extraction is needed. You can also run `install.cmd` from
a source checkout. In later terminals, run `setup-gateway` again. A terminal that
was already open may need to be restarted before it sees the updated PATH.

For an offline transfer of the CLI itself:

Download and extract `setup-gateway-windows-amd64.zip` from GitHub Releases, then
double-click **Run-Setup.cmd**. The menu offers:

1. New-machine setup: choose tools, optional window manager, then Claude auth.
2. Claude authentication only.
3. Install command-line tools.
4. Install/configure a window manager.
5. Uninstall command-line tools.
6. Uninstall window-manager setup.
7. List and activate saved Claude connection profiles.

The menu asks before starting a toolkit module. Package installers can request
administrator privileges. Docker and removal of an existing GlazeWM installation
have separate prompts. A new-machine setup does not require installing every group.

To install the CLI into your user PATH from a published release:

```powershell
& ([scriptblock]::Create((irm 'https://github.com/wisjoycomm/setup-gateway/releases/latest/download/install.ps1'))) -RunSetup
```

Or download `install.ps1` and run `./install.ps1 -RunSetup`. Use `-Version vX.Y.Z`
to select a release. `-InstallDir PATH -NoPath` installs into a selected directory
without modifying PATH. The installer detects Windows x64/ARM64, verifies SHA-256,
and makes the command available in the current and future terminals.

macOS / Linux:

```sh
curl -fsSL https://raw.githubusercontent.com/wisjoycomm/setup-gateway/main/install.sh | sh
~/.local/bin/setup-gateway
```

The Unix installer requires a release containing `checksums.txt`. It supports
Intel/Apple Silicon macOS and x64/ARM64 Linux. Add `~/.local/bin` to PATH if prompted.
For a source checkout that has not been released yet, use the local build below;
the public install commands fetch the latest **published** binary, not local changes.

## Bundled tools

```text
setup-gateway
setup-gateway onboard
setup-gateway tools list
setup-gateway tools install cli
setup-gateway tools install cli -groups 1,2,4
setup-gateway tools install wm
setup-gateway tools install wm -dry-run
setup-gateway tools uninstall cli
setup-gateway tools uninstall wm
```

| Group | Contents |
| --- | --- |
| CLI 1: terminal | Starship, Nerd Font, fzf, zoxide; PowerShell 7, Windows Terminal and Clink on Windows |
| CLI 2: utilities | ripgrep, fd, bat, eza, jq, lazygit, delta, btop, GitHub CLI |
| CLI 3: development | Git, fnm/Node.js, uv/Python, Neovim, Antigravity IDE, optional Docker |
| CLI 4: agents | Claude Code, Codex CLI, Antigravity CLI |
| Window manager | Windows: Komorebi + whkd. macOS: AeroSpace. Linux: configuration for existing i3/Hyprland |

Availability varies by OS/package manager. Shell setup includes the original
marked profile blocks, aliases, prompt/font, Git defaults and Node LTS setup.
Window-manager setup changes window layout, shortcuts, startup and taskbar/Dock
integration. Linux users install their desktop session separately.

The scripts come from the owner's `Bo_cong_cu`; see [module provenance](toolkit/README.md).
Their original package-selection and uninstall prompts remain available. Uninstall
can remove previously installed applications if you select them; use only the
configuration cleanup when you want to retain those applications.

## Claude: four authentication methods

Run `setup-gateway auth` for a guided choice. The wizard detects missing Claude
Code and offers its official installer. For flag-based setup, add
`-install-claude` to install it if missing. Git for Windows is optional; installing
the toolkit's development/agent group provides it.

| Method | Result in Claude settings | Credential |
| --- | --- | --- |
| `oauth` / `session-key` | `CLAUDE_CODE_OAUTH_TOKEN` | Full `sk-ant-oat...` OAuth token supplied by your account administrator |
| `api-key` | `ANTHROPIC_API_KEY` | Anthropic Console API key, direct API billing |
| `gateway` | `ANTHROPIC_BASE_URL` + `ANTHROPIC_AUTH_TOKEN` | Custom domain and gateway-issued Bearer credential |
| `login` | Removes old credential/routing overrides, runs `claude auth login --claudeai` | Normal browser login |

`session-key` is an alias for the OAuth method discussed here. A browser
`sessionKey` cookie (`sk-ant-sid...`) is rejected: it is not a Claude Code OAuth
credential. An OAuth token must not be pasted into the gateway API-key field.
The CLI rejects that mistake before sending a request.

### OAuth token, no browser login

```text
setup-gateway auth -auth oauth -install-claude -profile work
```

Paste the full `sk-ant-oat...` token at the hidden prompt. You do not need to run
`claude setup-token` if an administrator has already supplied a valid token.
OAuth tokens are saved without a verification request; Claude checks validity
on the first model request. A token from `claude setup-token` is documented as
lasting one year, but can be revoked earlier. Its prefix alone does not reveal
its remaining lifetime.

### Direct Anthropic API key

```text
setup-gateway auth -auth api-key -install-claude -profile direct-api
```

Uses the official Anthropic endpoint and `x-api-key`. Model discovery validates
access without generating a completion. Use `-skip-check` to configure offline.

### Custom gateway/domain

```text
setup-gateway auth -auth gateway -only claude -base-url https://ai.adnx.vn -profile adnx
```

Enter a key issued by **that gateway**, not the OAuth token. A bare domain,
`/v1` URL, or pasted `/v1/models` URL is accepted. HTTPS is required except for
localhost development endpoints. Path prefixes such as `/proxy/v1` are retained.

If the gateway expects `x-api-key` instead of `Authorization: Bearer`:

```text
setup-gateway auth -auth gateway -only claude -base-url https://example.com -auth-header x-api-key
```

A gateway without model discovery can be configured with explicit model IDs:

```text
setup-gateway auth -auth gateway -only claude -base-url https://example.com -skip-check -models cc/claude-opus,cc/claude-sonnet,cc/claude-haiku
```

Use the actual IDs from your gateway; the example IDs are placeholders. Explicit
IDs can also use provider-specific aliases. Discovery checks `/v1/models` only;
it does not prove that every advertised model can answer a generation request.

### Normal browser login

```text
setup-gateway auth -auth login -install-claude
```

To prepare configuration without opening the browser yet:

```text
setup-gateway auth -auth login -no-login
claude auth login --claudeai
```

## Save and switch connections

```text
setup-gateway -list-profiles
setup-gateway -use-profile adnx
setup-gateway -use-profile work -run
```

Profiles live in `~/.setup-gateway/profiles/`. Saving a profile also activates it.
Activation updates the user Claude settings (`CLAUDE_CONFIG_DIR` is respected),
removes previous auth variables and gateway model aliases such as `cc/...`, then
writes only the selected connection. Hooks, plugins, status line, permissions,
model settings and unrelated environment settings are preserved. Old
`apiKeyHelper`, `modelOverrides`, `modelPicker` and `fallbackModel` routing values
are removed to prevent a previous provider from taking over.

Profiles select one active user configuration; they are not automatic failover
or isolated per-session accounts. Start a fresh Claude session after switching.
Run `/status` to verify the credential source, base URL and model.

`-run` launches Claude with inherited auth/provider/model environment overrides
removed. It does not change the user/machine environment or project/managed
settings. Those other settings may still control the connection. An ordinary
`claude` launch can inherit old terminal variables; remove those or use `-run`.
Managed organizational policies continue to apply.

## Existing multi-client gateway support

The original flag-based invocation remains supported:

```text
setup-gateway -base-url https://example.com -only claude,codex,opencode,cursor
setup-gateway -base-url https://example.com -provider-id mygw
```

The second command defaults to the original client list. In the interactive menu,
Claude alone is the default; you can select all clients for gateway setup.
OAuth, direct Anthropic API-key, browser login and named profiles are Claude-only.
Antigravity CLI has no supported custom endpoint and is reported as skipped.

| Client | Configuration |
| --- | --- |
| Claude | `~/.claude/settings.json`; model aliases discovered from gateway |
| Codex | `~/.codex/config.toml`; gateway Responses API provider and credential |
| OpenCode | `~/.config/opencode/opencode.jsonc`; gateway models and service credential registration |
| Cursor | `OPENAI_BASE_URL` and `OPENAI_API_KEY` for future terminals |

## Preview, credentials and backups

```text
setup-gateway auth -auth oauth -dry-run
setup-gateway auth -auth gateway -only claude -base-url https://example.com -dry-run
setup-gateway tools install cli -groups 1,4 -dry-run
```

Dry runs do not write files, contact endpoints, install anything or launch login.
Credentials are hidden during interactive entry and are not printed in previews,
errors or profile listings. Automation can use `-key-stdin` (one input line) or
`SETUP_GATEWAY_KEY`; `-key` exists for compatibility but exposes the value in shell
history/process arguments. Keep real credentials out of commands you share.

Auth profiles and client configuration contain credentials locally. They are not
portable encrypted vaults: protect these files and their backups. Auth/config
writes use owner-only Unix permissions, private temporary files and unique
`.bak-<timestamp>` backups. Windows files inherit the user's directory ACLs.
Malformed Claude settings are refused without replacement. Toolkit scripts retain
their own `.bak-wisjoy` backup conventions.

## Build and verification

Use the Go version declared in `go.mod` (currently Go 1.27.1):

```sh
go test ./...
go vet ./...
go build -o setup-gateway .
```

On Windows, build a portable ZIP containing the executable, launcher and README:

```powershell
./build.ps1
# Optional: all six platform/architecture binaries plus checksums
./build.ps1 -All
```

Output is in `dist/` and is excluded from Git. The binary includes every toolkit
script. CI runs Go tests/vet and script syntax checks; installing or uninstalling
real applications and interactive OAuth are deliberately not performed by tests.
Tagging a reviewed release with `v*` triggers the release workflow to build all
binaries, a Windows ZIP and `checksums.txt`. Workflow dispatch builds downloadable
CI artifacts without publishing a release.
