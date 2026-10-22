# setup-gateway

One command that points claude, codex, opencode, and cursor at your own API
gateway. It asks for the domain and the API key, fetches `/v1/models` to check
the gateway answers, then writes each CLI's config. Nothing else is changed,
and every file is copied to `*.bak` first.

agy is skipped: the Antigravity CLI has no custom-endpoint setting and takes
its models from the signed-in account.

## Install

Windows, in PowerShell:

```powershell
irm https://raw.githubusercontent.com/wisjoycomm/setup-gateway/main/install.ps1 | iex
```

macOS or Linux:

```bash
curl -fsSL https://raw.githubusercontent.com/wisjoycomm/setup-gateway/main/install.sh | bash
```

The script picks the right build for the machine. It covers Windows, macOS
(Apple Silicon and Intel), and Linux (x64 and arm64).

Open a new terminal afterwards so it picks up the change to `PATH`.

## Use

```
setup-gateway
```

It prompts for two things:

```
Custom domain (e.g. https://ai.example.com): ai.example.com
API key: sk-...
```

A bare domain is enough. `https://` and `/v1` are added when missing. If the
gateway does not answer, nothing is written.

What it writes:

| CLI | Where |
| --- | --- |
| claude | `~/.claude/settings.json`, and pins the opus, sonnet, haiku, and fable aliases to matching models |
| codex | `~/.codex/config.toml`, provider `gateway`, Responses API |
| opencode | `~/.config/opencode/opencode.jsonc`, one entry per model as `gateway/<id>` |
| cursor | `OPENAI_BASE_URL` and `OPENAI_API_KEY` in the environment |
| agy | skipped |

The API key goes into an environment variable for codex, opencode, and cursor.
Claude Code only reads its token from `settings.json`, so the key stays in that
file.

Open a new terminal and run `claude`, `codex`, or `opencode`.

## Options

```
setup-gateway                       ask for the domain and key
setup-gateway -dry-run              show what would change, write nothing
setup-gateway -only claude,codex    limit to some CLIs
setup-gateway -provider-id adnx     provider name used by codex and opencode
setup-gateway -base-url URL -key K  skip the prompts
```

## Build from source

Needs Go 1.21 or newer.

```bash
go build -o setup-gateway .
```

Cross-compile by setting `GOOS` and `GOARCH`, for example
`GOOS=linux GOARCH=amd64 go build -o setup-gateway .`.
