// setup-gateway points every installed coding CLI at one OpenAI-compatible
// gateway and pulls the model list from <base-url>/models.
//
//	claude    ~/.claude/settings.json            env block (Anthropic wire)
//	codex     ~/.codex/config.toml               [model_providers.<id>] (Responses wire)
//	agy       Antigravity CLI                    no custom endpoint; reported, never written
//	cursor    env OPENAI_BASE_URL / OPENAI_API_KEY
//	opencode  ~/.config/opencode/opencode.jsonc  providers.<id>
//
// Existing settings are preserved; only the gateway keys change, and every
// file is copied to *.bak first. Nothing here touches a project repository.
package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"
)

func main() {
	base := flag.String("base-url", "", "gateway URL (optional; asked interactively when omitted)")
	key := flag.String("key", "", "API key (optional; asked interactively when omitted)")
	provider := flag.String("provider-id", "gateway", "provider id used by codex and opencode")
	only := flag.String("only", "claude,codex,agy,cursor,opencode", "comma-separated subset")
	dry := flag.Bool("dry-run", false, "print what would change without writing")
	flag.Parse()

	in := bufio.NewReader(os.Stdin)
	if *base == "" {
		*base = ask(in, "Custom domain (e.g. https://ai.example.com)")
	}
	if *key == "" {
		*key = ask(in, "API key")
	}
	if *base == "" || *key == "" {
		fmt.Fprintln(os.Stderr, "domain and API key are both required")
		os.Exit(2)
	}
	*base = normalize(*base)

	fmt.Println("checking", strings.TrimRight(*base, "/")+"/models ...")
	ids, err := fetchModels(*base, *key)
	if err != nil {
		fmt.Fprintln(os.Stderr, "gateway check failed:", err)
		os.Exit(1)
	}
	fmt.Printf("ok: %d models\n", len(ids))
	preview := ids
	if len(preview) > 5 {
		preview = preview[:5]
	}
	fmt.Println("    " + strings.Join(preview, ", ") + " ...")

	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	targets := map[string]func() (string, error){
		"claude":   func() (string, error) { return setupClaude(home, *base, *key, ids, *dry) },
		"codex":    func() (string, error) { return setupCodex(home, *base, *key, ids, *provider, *dry) },
		"cursor":   func() (string, error) { return setupCursor(*base, *key, ids, *dry) },
		"opencode": func() (string, error) { return setupOpenCode(home, *base, ids, *provider, *dry) },
	}
	for _, name := range strings.Split(*only, ",") {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if name == "agy" {
			fmt.Println("agy: SKIP - Antigravity CLI has no custom endpoint; it uses the signed-in account")
			continue
		}
		fn, ok := targets[name]
		if !ok {
			fmt.Fprintf(os.Stderr, "unknown target %q\n", name)
			os.Exit(2)
		}
		line, err := fn()
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %s\n", name, err)
			os.Exit(1)
		}
		fmt.Printf("%s: %s\n", name, line)
	}
	if *dry {
		fmt.Println("dry run: nothing written")
		return
	}
	fmt.Println("done. open a new terminal, then run claude, codex, or opencode.")
}

func ask(in *bufio.Reader, label string) string {
	fmt.Print(label + ": ")
	line, _ := in.ReadString('\n')
	return strings.TrimSpace(line)
}

// normalize accepts a bare domain, a host, or a full URL and returns a /v1 base.
func normalize(raw string) string {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimRight(raw, "/")
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	if !strings.HasSuffix(raw, "/v1") {
		raw += "/v1"
	}
	return raw
}

func fetchModels(base, key string) ([]string, error) {
	req, err := http.NewRequest(http.MethodGet, strings.TrimRight(base, "/")+"/models", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+key)
	client := &http.Client{Timeout: 30 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: %s", res.Status, truncate(string(body), 200))
	}
	var payload struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var ids []string
	for _, item := range payload.Data {
		if item.ID != "" && !seen[item.ID] {
			seen[item.ID] = true
			ids = append(ids, item.ID)
		}
	}
	sort.Strings(ids)
	if len(ids) == 0 {
		return nil, errors.New("endpoint returned no model ids")
	}
	return ids, nil
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

func backup(path string) {
	if _, err := os.Stat(path); err == nil {
		_ = copyFile(path, path+".bak")
	}
}

func writeFile(path, text string, dry bool) error {
	if dry {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	backup(path)
	return os.WriteFile(path, []byte(text), 0o644)
}

func copyFile(from, to string) error {
	in, err := os.ReadFile(from)
	if err != nil {
		return err
	}
	return os.WriteFile(to, in, 0o644)
}

// --- claude ---------------------------------------------------------------

func setupClaude(home, base, key string, ids []string, dry bool) (string, error) {
	path := filepath.Join(home, ".claude", "settings.json")
	root := readJSON(path)
	env, _ := root["env"].(map[string]any)
	if env == nil {
		env = map[string]any{}
	}
	// Claude Code appends /v1/messages itself, so it takes the bare origin.
	env["ANTHROPIC_BASE_URL"] = strings.TrimSuffix(base, "/v1")
	env["ANTHROPIC_AUTH_TOKEN"] = key
	pins := map[string]string{
		"ANTHROPIC_DEFAULT_OPUS_MODEL":   pick(ids, []string{"opus"}, []string{"review"}),
		"ANTHROPIC_DEFAULT_SONNET_MODEL": pick(ids, []string{"sonnet"}, []string{"review"}),
		"ANTHROPIC_DEFAULT_HAIKU_MODEL":  pick(ids, []string{"haiku"}, nil),
		"ANTHROPIC_DEFAULT_FABLE_MODEL":  pick(ids, []string{"fable"}, nil),
	}
	if pins["ANTHROPIC_DEFAULT_OPUS_MODEL"] != "" {
		env["ANTHROPIC_MODEL"] = pins["ANTHROPIC_DEFAULT_OPUS_MODEL"]
	}
	sub := pins["ANTHROPIC_DEFAULT_HAIKU_MODEL"]
	if sub == "" {
		sub = pins["ANTHROPIC_DEFAULT_OPUS_MODEL"]
	}
	if sub != "" {
		env["CLAUDE_CODE_SUBAGENT_MODEL"] = sub
	}
	var pinned []string
	for name, value := range pins {
		if value != "" {
			env[name] = value
			pinned = append(pinned, name+"="+value)
		}
	}
	sort.Strings(pinned)
	root["env"] = env
	out, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return "", err
	}
	return path + "\n    " + strings.Join(pinned, "\n    "), writeFile(path, string(out)+"\n", dry)
}

// --- codex ----------------------------------------------------------------

func setupCodex(home, base, key string, ids []string, provider string, dry bool) (string, error) {
	path := filepath.Join(home, ".codex", "config.toml")
	text := ""
	if b, err := os.ReadFile(path); err == nil {
		text = string(b)
	}
	re := regexp.MustCompile(`(?ms)^\[model_providers\.` + regexp.QuoteMeta(provider) + `\][^\[]*`)
	text = re.ReplaceAllString(text, "")
	model := pick(ids, []string{"gpt"}, []string{"review"})
	if model == "" {
		model = ids[0]
	}
	line := "model_provider = " + tomlString(provider)
	if regexp.MustCompile(`(?m)^model_provider\s*=`).MatchString(text) {
		text = regexp.MustCompile(`(?m)^model_provider\s*=.*$`).ReplaceAllString(text, line)
	} else {
		text = line + "\n" + text
	}
	envKey := strings.ToUpper(provider) + "_API_KEY"
	block := fmt.Sprintf("\n[model_providers.%s]\nname = %s\nbase_url = %s\nwire_api = \"responses\"\nenv_key = %s\n",
		provider, tomlString(provider), tomlString(base), tomlString(envKey))
	text = strings.TrimRight(text, "\n") + "\n" + block
	if err := writeFile(path, text, dry); err != nil {
		return "", err
	}
	if !dry {
		if err := setEnv(envKey, key); err != nil {
			return "", err
		}
	}
	return fmt.Sprintf("%s (provider %s, default %s)", path, provider, model), nil
}

func tomlString(v string) string {
	return `"` + strings.ReplaceAll(strings.ReplaceAll(v, `\`, `\\`), `"`, `\"`) + `"`
}

// --- cursor ---------------------------------------------------------------

func setupCursor(base, key string, ids []string, dry bool) (string, error) {
	model := pick(ids, []string{"gpt"}, []string{"review"})
	if model == "" {
		model = ids[0]
	}
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

func setupOpenCode(home, base string, ids []string, provider string, dry bool) (string, error) {
	root := os.Getenv("XDG_CONFIG_HOME")
	if root == "" {
		root = filepath.Join(home, ".config")
	}
	path := filepath.Join(root, "opencode", "opencode.jsonc")
	doc := map[string]any{}
	if b, err := os.ReadFile(path); err == nil {
		clean := regexp.MustCompile(`,(\s*[}\]])`).ReplaceAll(stripComments(b), []byte("$1"))
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
	return fmt.Sprintf("%s (%d models as %s/<id>)", path, len(ids), provider), nil
}

func readJSON(path string) map[string]any {
	b, err := os.ReadFile(path)
	if err != nil {
		return map[string]any{}
	}
	var v map[string]any
	if json.Unmarshal(b, &v) != nil || v == nil {
		return map[string]any{}
	}
	return v
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
	line := "export " + name + "=" + tomlString(value) + "\n"
	return os.WriteFile(rc, append(text, []byte(line)...), 0o644)
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
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
