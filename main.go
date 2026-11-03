// setup-gateway installs the bundled development toolkit and configures coding
// clients for gateway, OAuth, API-key or browser authentication.
//
//	claude    ~/.claude/settings.json            env block (Anthropic wire)
//	codex     ~/.codex/config.toml               [model_providers.<id>] (Responses wire)
//	agy       Antigravity CLI                    no custom endpoint; reported, never written
//	cursor    guided configuration in the application
//	opencode  ~/.config/opencode/opencode.jsonc  provider.<id>
//
// Claude authentication changes preserve hooks and unrelated settings. Changed
// configuration files receive unique backups before replacement.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

func main() {
	if err := runApp(os.Args[1:], os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "setup-gateway:", err)
		os.Exit(1)
	}
}

// pick returns the first id containing every needle and none of the exclusions.
func pick(ids []string, needles []string, exclude []string) string {
	for _, id := range ids {
		low := strings.ToLower(id)
		ok := true
		for _, n := range needles {
			if !strings.Contains(low, n) {
				ok = false
			}
		}
		for _, x := range exclude {
			if strings.Contains(low, x) {
				ok = false
			}
		}
		if ok {
			return id
		}
	}
	return ""
}

// --- codex ----------------------------------------------------------------

func setupCodex(home, base, key, model, provider string, dry bool) (string, error) {
	return applyCodex(home, "gateway", base, key, model, provider, dry)
}

func setupCursor(base, key, model string, dry bool) (string, error) {
	return "MANUAL - Cursor > Settings > Models: enter your API key and supported Base URL override; use configure cursor for guidance. No global OPENAI variables were changed.", nil
}

func setupOpenCode(home, base, key string, ids []string, provider string, dry bool) (string, error) {
	return applyOpenCode(home, base, key, ids, provider, "@ai-sdk/openai-compatible", dry)
}

// setEnv persists a variable for future shells without printing it.
func setEnv(name, value string) error {
	if runtime.GOOS == "windows" {
		return setEnvWindows(name, value)
	}
	return setEnvUnix(name, value)
}

func setEnvUnix(name, value string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	rc := filepath.Join(home, ".profile")
	b, _ := os.ReadFile(rc)
	text := regexp.MustCompile(`(?m)^export `+regexp.QuoteMeta(name)+`=.*\n?`).ReplaceAll(b, nil)
	line := "export " + name + "='" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'\n"
	return writeFile(rc, string(text)+line, false)
}

// stripComments removes // and /* */ comments from JSONC, leaving string
// contents such as "https://..." intact.
func stripComments(b []byte) []byte {
	out := make([]byte, 0, len(b))
	inStr := false
	for i := 0; i < len(b); i++ {
		c := b[i]
		if inStr {
			out = append(out, c)
			if c == '\\' && i+1 < len(b) {
				i++
				out = append(out, b[i])
			} else if c == '"' {
				inStr = false
			}
			continue
		}
		if c == '"' {
			inStr = true
			out = append(out, c)
			continue
		}
		if c == '/' && i+1 < len(b) && b[i+1] == '/' {
			for i < len(b) && b[i] != '\n' {
				i++
			}
			if i < len(b) {
				out = append(out, '\n')
			}
			continue
		}
		if c == '/' && i+1 < len(b) && b[i+1] == '*' {
			i += 2
			for i+1 < len(b) && !(b[i] == '*' && b[i+1] == '/') {
				i++
			}
			i++
			continue
		}
		out = append(out, c)
	}
	return out
}
