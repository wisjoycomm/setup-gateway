# setup-gateway

Points claude, codex, opencode, and cursor at one custom API gateway. Asks for
the domain and API key, fetches and checks `/v1/models`, then writes each CLI's
config. agy has no custom-endpoint setting, so it is skipped.

## Install

Windows (PowerShell):

```powershell
irm https://raw.githubusercontent.com/wisjoycomm/setup-gateway/main/install.ps1 | iex
```

macOS / Linux:

```bash
curl -fsSL https://raw.githubusercontent.com/wisjoycomm/setup-gateway/main/install.sh | bash
```

Then open a new terminal and run `setup-gateway`.
