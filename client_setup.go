package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"github.com/tailscale/hujson"
)

func codexConfigPath(home string) string {
	dir := os.Getenv("CODEX_HOME")
	if dir == "" {
		dir = filepath.Join(home, ".codex")
	}
	return filepath.Join(dir, "config.toml")
}

func applyCodex(home, auth, base, key, model, provider string, dry bool) (string, error) {
	path := codexConfigPath(home)
	doc := map[string]any{}
	b, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	if len(b) > 0 {
		if err = toml.Unmarshal(b, &doc); err != nil {
			return "", fmt.Errorf("invalid Codex TOML; left untouched: %w", err)
		}
	}
	delete(doc, "model_provider")
	delete(doc, "model")
	delete(doc, "forced_login_method")
	if model != "" {
		doc["model"] = model
	}
	if auth == "gateway" {
		providers, ok := doc["model_providers"].(map[string]any)
		if !ok {
			providers = map[string]any{}
		}
		providers[provider] = map[string]any{"name": provider, "base_url": base, "wire_api": "responses", "requires_openai_auth": false, "experimental_bearer_token": key}
		doc["model_providers"] = providers
		doc["model_provider"] = provider
	}
	data, err := toml.Marshal(doc)
	if err != nil {
		return "", err
	}
	if err = writeFile(path, string(data), dry); err != nil {
		return "", err
	}
	return path, nil
}

func applyOpenCode(home, base, key string, ids []string, provider, npm string, dry bool) (string, error) {
	root := os.Getenv("XDG_CONFIG_HOME")
	if root == "" {
		root = filepath.Join(home, ".config")
	}
	dir := filepath.Join(root, "opencode")
	// JSONC wins when both exist. Read both to retain unrelated settings.
	doc := map[string]any{}
	path := filepath.Join(dir, "opencode.json")
	for _, name := range []string{"opencode.json", "opencode.jsonc"} {
		file := filepath.Join(dir, name)
		b, err := os.ReadFile(file)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return "", err
		}
		clean, err := hujson.Standardize(b)
		if err != nil {
			return "", fmt.Errorf("invalid %s; left untouched: %w", file, err)
		}
		var layer map[string]any
		if err = json.Unmarshal(clean, &layer); err != nil || layer == nil {
			return "", fmt.Errorf("%s must contain a JSON object", file)
		}
		mergeObject(doc, layer)
		path = file
	}
	providers, _ := doc["provider"].(map[string]any)
	if providers == nil {
		providers = map[string]any{}
	}
	entry, _ := providers[provider].(map[string]any)
	if entry == nil {
		entry = map[string]any{}
	}
	models := map[string]any{}
	for _, id := range ids {
		models[id] = map[string]any{"name": id}
	}
	entry["npm"] = npm
	entry["name"] = provider
	entry["options"] = map[string]any{"baseURL": base, "apiKey": key}
	entry["models"] = models
	providers[provider] = entry
	doc["provider"] = providers
	doc["$schema"] = "https://opencode.ai/config.json"
	if len(ids) > 0 {
		doc["model"] = provider + "/" + ids[0]
	}
	// Remove only this tool's old, unsupported provider entry during migration.
	if old, ok := doc["providers"].(map[string]any); ok {
		if item, ok := old[provider].(map[string]any); ok && item["package"] == "@opencode/ai/providers/openai-compatible" {
			delete(old, provider)
			if len(old) == 0 {
				delete(doc, "providers")
			}
		}
	}
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return "", err
	}
	if err = writeFile(path, string(data)+"\n", dry); err != nil {
		return "", err
	}
	return path, nil
}

func mergeObject(dest, src map[string]any) {
	for k, v := range src {
		a, aOK := dest[k].(map[string]any)
		b, bOK := v.(map[string]any)
		if aOK && bOK {
			mergeObject(a, b)
		} else {
			dest[k] = v
		}
	}
}

func configureClient(args []string, input io.Reader, out io.Writer) error {
	if len(args) == 0 {
		fmt.Fprintln(out, "Cau hinh: claude, codex, opencode, cursor, cursor-agent, antigravity, antigravity-ide, orca")
		id := menuAnswer(input, out, "Ung dung (Enter = quay lai)")
		if id == "" {
			return nil
		}
		args = []string{id}
	}
	id := args[0]
	if id == "claude" {
		return runCLI(append([]string{"-only", "claude"}, args[1:]...), input, out)
	}
	if _, err := appByID(id); err != nil {
		return err
	}
	if id != "codex" && id != "opencode" {
		if len(args) > 1 {
			return errors.New("this app uses its own sign-in UI; configure accepts only its app ID")
		}
		switch id {
		case "cursor-agent":
			fmt.Fprintln(out, "Run agent login for your Cursor account. Cursor's API key is not a Claude OAuth token. Custom gateway routing is not configured by this tool.")
		case "cursor":
			fmt.Fprintln(out, "Open Cursor > Settings > Models: enter a provider API key and, where supported, Override OpenAI Base URL. Sign in from Cursor's account UI. Global OPENAI_* variables are not written.")
		case "antigravity", "agy", "antigravity-ide":
			fmt.Fprintln(out, "Start agy / Antigravity IDE and use the Google sign-in flow. This tool does not configure a custom endpoint for Antigravity.")
		case "orca":
			fmt.Fprintln(out, "Open Orca, add your project folder, then select an installed coding agent. Configure Claude/Codex separately in this menu; Orca has no shared gateway token setting.")
		}
		return nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	return configureAPIClient(id, args[1:], input, out, home)
}

func configureAPIClient(id string, args []string, input io.Reader, out io.Writer, home string) error {
	var err error
	flags := flag.NewFlagSet("configure "+id, flag.ContinueOnError)
	flags.SetOutput(out)
	auth := flags.String("auth", "", "login, api-key, gateway")
	base := flags.String("base-url", "", "custom gateway domain")
	provider := flags.String("provider-id", "gateway", "gateway provider name; direct OpenCode API: openai or anthropic")
	models := flags.String("models", "", "comma-separated model IDs; first is the default")
	keyStdin := flags.Bool("key-stdin", false, "read one credential line from stdin")
	skip := flags.Bool("skip-check", false, "skip gateway model discovery")
	dry := flags.Bool("dry-run", false, "no files, network, installation or login")
	noLogin := flags.Bool("no-login", false, "prepare normal login without starting it")
	device := flags.Bool("device-auth", false, "Codex device-code login")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("unexpected configuration arguments")
	}
	if *auth == "" {
		fmt.Fprintf(out, "%s: 1. Login  2. API key  3. Custom gateway\n", id)
		*auth = map[string]string{"1": "login", "2": "api-key", "3": "gateway"}[menuAnswer(input, out, "Method [1-3]")]
	}
	if !contains([]string{"login", "api-key", "gateway"}, *auth) {
		return errors.New("choose login, api-key or gateway; Claude session tokens do not authenticate this app")
	}
	if !validName(*provider) {
		return errors.New("invalid provider-id")
	}
	if *device && (id != "codex" || *auth != "login") {
		return errors.New("device-auth requires Codex login")
	}
	if *noLogin && *auth != "login" {
		return errors.New("no-login applies only to normal login")
	}
	if *auth == "login" && (*base != "" || *keyStdin || *models != "") {
		return errors.New("normal login does not accept a gateway, credential or model override")
	}
	if *auth == "api-key" && *base != "" {
		return errors.New("use gateway mode for a custom domain")
	}
	if id == "opencode" && *auth == "api-key" {
		if *provider == "gateway" {
			*provider = "openai"
			if len(args) == 0 {
				if answer := menuAnswer(input, out, "API provider [openai / anthropic; Enter = openai]"); answer != "" {
					*provider = answer
				}
			}
		}
		if !contains([]string{"openai", "anthropic"}, *provider) {
			return errors.New("direct OpenCode API requires provider-id openai or anthropic")
		}
	}
	key := ""
	if *auth != "login" {
		if *auth == "gateway" && *base == "" {
			if *keyStdin {
				return errors.New("key-stdin requires base-url for gateway mode")
			}
			*base = menuAnswer(input, out, "Gateway domain")
		}
		reader := bufio.NewReader(&oneByteReader{input})
		if *keyStdin {
			key = prompt(reader, io.Discard, "")
		} else {
			key = strings.TrimSpace(os.Getenv("SETUP_GATEWAY_KEY"))
			if key == "" {
				key, err = promptSecret(input, reader, out, "API key / gateway token")
				if err != nil {
					return err
				}
			}
		}
		if key == "" || strings.ContainsAny(key, " \r\n\t") {
			return errors.New("a single API key is required")
		}
		if strings.HasPrefix(key, "sk-ant-oat") || strings.HasPrefix(key, "sk-ant-sid") || strings.HasPrefix(key, "sessionKey=") {
			return errors.New("Claude OAuth/session credentials cannot be used as this API key")
		}
		if id == "codex" && *auth == "api-key" && strings.HasPrefix(key, "sk-ant-") {
			return errors.New("Codex direct API mode needs an OpenAI API key, not an Anthropic key")
		}
	}
	ids := splitList(*models)
	if *auth == "gateway" {
		*base, err = gatewayURL(*base)
		if err != nil {
			return err
		}
		if !*dry && !*skip {
			found, e := discoverModels(connectionProfile{BaseURL: *base, Credential: key, Header: "bearer"})
			if e != nil {
				return e
			}
			if len(ids) == 0 {
				ids = found
			}
		}
		if len(ids) == 0 && !*dry {
			return errors.New("provide -models with the gateway's actual model IDs")
		}
		if *models == "" && len(ids) > 0 {
			if len(args) == 0 {
				fmt.Fprintln(out, "Available model IDs:", strings.Join(ids, ", "))
				model := menuAnswer(input, out, "Default model ID")
				if model == "" {
					return errors.New("select a model ID")
				}
				ids = append([]string{model}, ids...)
			} else if id == "codex" {
				model := pick(ids, []string{"gpt"}, nil)
				if model == "" {
					return errors.New("no GPT model found; pass -models with the gateway's Codex-compatible model ID")
				}
				ids = append([]string{model}, ids...)
			}
		}
	}
	app, _ := appByID(id)
	binary := findApp(app, home)
	needsProcess := id == "codex" && (*auth == "api-key" || *auth == "login" && !*noLogin) || id == "opencode" && *auth == "login" && !*noLogin
	if needsProcess && binary == "" && !*dry {
		return fmt.Errorf("%s is missing; run setup-gateway apps install %s first", id, id)
	}
	if *dry {
		fmt.Fprintf(out, "Dry run: configure %s with %s; no network, files, installation or login.\n", id, *auth)
		return nil
	}
	if id == "codex" {
		model := ""
		if len(ids) > 0 {
			model = ids[0]
		}
		path, e := applyCodex(home, *auth, *base, key, model, *provider, false)
		if e != nil {
			return e
		}
		fmt.Fprintln(out, "Configured:", path)
		fmt.Fprintln(out, "Custom gateways must support POST /v1/responses. Existing shell OPENAI_BASE_URL overrides may need removal.")
		if needsProcess {
			cmd := []string{binary, "login"}
			stdin := input
			if *auth == "api-key" {
				cmd = append(cmd, "--with-api-key")
				stdin = strings.NewReader(key + "\n")
			} else if *device {
				cmd = append(cmd, "--device-auth")
			}
			return runChild(cmd, stdin, out, cleanCodexEnvironment(os.Environ()))
		}
	} else if *auth == "login" {
		fmt.Fprintln(out, "OpenCode: use auth login to choose an account/provider, then /models to select its model. Existing custom providers remain available.")
		if needsProcess {
			return runChild([]string{binary, "auth", "login"}, input, out, os.Environ())
		}
	} else {
		npm := "@ai-sdk/openai-compatible"
		if *auth == "api-key" {
			if *provider == "gateway" {
				*provider = "openai"
			}
			switch *provider {
			case "openai":
				*base = "https://api.openai.com/v1"
				npm = "@ai-sdk/openai"
			case "anthropic":
				*base = "https://api.anthropic.com/v1"
				npm = "@ai-sdk/anthropic"
			default:
				return errors.New("direct OpenCode API requires provider-id openai or anthropic")
			}
		}
		path, e := applyOpenCode(home, *base, key, ids, *provider, npm, false)
		if e != nil {
			return e
		}
		fmt.Fprintln(out, "Configured:", path)
	}
	return nil
}

func cleanCodexEnvironment(env []string) []string {
	var result []string
	for _, item := range env {
		name, _, _ := strings.Cut(item, "=")
		if !contains([]string{"OPENAI_API_KEY", "OPENAI_BASE_URL", "CODEX_API_KEY", "CODEX_ACCESS_TOKEN", "SETUP_GATEWAY_KEY"}, strings.ToUpper(name)) {
			result = append(result, item)
		}
	}
	return result
}

// Prevent a configuration wizard from consuming the next menu answer.
type oneByteReader struct{ io.Reader }

func (r *oneByteReader) Read(p []byte) (int, error) {
	if len(p) > 1 {
		p = p[:1]
	}
	return r.Reader.Read(p)
}
