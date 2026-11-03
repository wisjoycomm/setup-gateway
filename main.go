// setup-gateway installs the bundled development toolkit and configures coding
// clients for gateway, OAuth, API-key or browser authentication.
//
//	claude    ~/.claude/settings.json            env block (Anthropic wire)
//	codex     ~/.codex/config.toml               [model_providers.<id>] (Responses wire)
//	agy       Antigravity CLI                    no custom endpoint; reported, never written
//	cursor    env OPENAI_BASE_URL / OPENAI_API_KEY
//	opencode  ~/.config/opencode/opencode.jsonc  providers.<id>
//
// Claude authentication changes preserve hooks and unrelated settings. Changed
// configuration files receive unique backups before replacement.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
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
	path := filepath.Join(home, ".codex", "config.toml")
	text := ""
	if b, err := os.ReadFile(path); err == nil {
		text = string(b)
	}
	re := regexp.MustCompile(`(?ms)^\[model_providers\.` + regexp.QuoteMeta(provider) + `\][^\[]*`)
	text = re.ReplaceAllString(text, "")
	line := "model_provider = " + tomlString(provider)
	if regexp.MustCompile(`(?m)^model_provider\s*=`).MatchString(text) {
		text = regexp.MustCompile(`(?m)^model_provider\s*=.*$`).ReplaceAllString(text, line)
	} else {
		text = line + "\n" + text
	}
	// The key goes in a header rather than env_key: a terminal or Windows
	// Terminal opened before setup never sees a new user variable, and codex
	// then fails with "Missing environment variable".
	block := fmt.Sprintf("\n[model_providers.%s]\nname = %s\nbase_url = %s\nwire_api = \"responses\"\nhttp_headers = { \"Authorization\" = %s }\n",
		provider, tomlString(provider), tomlString(base), tomlString("Bearer "+key))
	text = strings.TrimRight(text, "\n") + "\n" + block
	if err := writeFile(path, text, dry); err != nil {
		return "", err
	}
	return fmt.Sprintf("%s (provider %s, default %s)", path, provider, model), nil
}

func tomlString(v string) string {
	return `"` + strings.ReplaceAll(strings.ReplaceAll(v, `\`, `\\`), `"`, `\"`) + `"`
}

// --- cursor ---------------------------------------------------------------

func setupCursor(base, key, model string, dry bool) (string, error) {
	if dry {
		return fmt.Sprintf("env OPENAI_BASE_URL, OPENAI_API_KEY (model %s)", model), nil
	}
	if err := setEnv("OPENAI_BASE_URL", base); err != nil {
		return "", err
	}
	if err := setEnv("OPENAI_API_KEY", key); err != nil {
		return "", err
	}
	return fmt.Sprintf("env OPENAI_BASE_URL + OPENAI_API_KEY set (suggested model %s)", model), nil
}

// --- opencode -------------------------------------------------------------

func setupOpenCode(home, base, key string, ids []string, provider string, dry bool) (string, error) {
	root := os.Getenv("XDG_CONFIG_HOME")
	if root == "" {
		root = filepath.Join(home, ".config")
	}
	path := filepath.Join(root, "opencode", "opencode.jsonc")
	doc := map[string]any{}
	if b, err := os.ReadFile(path); err == nil {
		clean := regexp.MustCompile(`,(\s*[}\]])`).ReplaceAll(stripComments(b), []byte("$1"))
		if len(strings.TrimSpace(string(clean))) == 0 {
			clean = []byte("{}")
		}
		if err := json.Unmarshal(clean, &doc); err != nil {
			return "", fmt.Errorf("cannot parse %s, left untouched: %w", path, err)
		}
	}
	providers, _ := doc["providers"].(map[string]any)
	if providers == nil {
		providers = map[string]any{}
	}
	models := map[string]any{}
	for _, id := range ids {
		models[id] = map[string]any{}
	}
	providers[provider] = map[string]any{
		"name":     provider,
		"env":      []string{strings.ToUpper(provider) + "_API_KEY"},
		"package":  "@opencode/ai/providers/openai-compatible",
		"settings": map[string]any{"baseURL": base},
		"models":   models,
	}
	doc["$schema"] = "https://opencode.ai/config.json"
	doc["providers"] = providers
	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return "", err
	}
	if err := writeFile(path, string(out)+"\n", dry); err != nil {
		return "", err
	}
	note := ""
	if !dry {
		note = registerOpenCodeKey(strings.ToUpper(provider)+"_API_KEY", key)
	}
	return fmt.Sprintf("%s (%d models as %s/<id>)%s", path, len(ids), provider, note), nil
}

// registerOpenCodeKey hands the key to the opencode background service. The
// service runs with its own environment, so a new user variable is not seen
// until it is registered there and the service restarts.
func registerOpenCodeKey(name, key string) string {
	bin, err := exec.LookPath("opencode")
	if err != nil {
		return ""
	}
	if err := exec.Command(bin, "service", "set", "env", name, key).Run(); err != nil {
		return "\n    could not register " + name + " with the opencode service: " + err.Error()
	}
	_ = exec.Command(bin, "service", "restart").Run()
	return "\n    " + name + " registered with the opencode service"
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
