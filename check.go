package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/pelletier/go-toml/v2"
	"github.com/tailscale/hujson"
)

const checkUsage = "cách dùng: wj check"

// mcpSource is one config file holding MCP servers as an object under key.
type mcpSource struct {
	path string
	key  string
}

// agentConfig lists where one agent keeps its user-level configuration.
type agentConfig struct {
	name         string
	apps         []string // any of these installed counts as installed
	instructions string   // empty: the agent has no user-level instructions file
	skillDirs    []string
	mcp          []mcpSource
	connection   func() string
}

type agentState struct {
	config       agentConfig
	installed    bool
	instructions bool
	mcp          []string
	skills       []string
	unreadable   []string
}

func runCheck(args []string, out io.Writer) error {
	if len(args) > 0 {
		return errors.New(checkUsage)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	checkStatus(out, home, func(id string) bool {
		app, err := appByID(id)
		return err == nil && findApp(app, home) != ""
	})
	return nil
}

func checkAgents(home string) []agentConfig {
	codexDir := filepath.Dir(codexConfigPath(home))
	gemini := filepath.Join(home, ".gemini")
	opencode := filepath.Join(home, ".config", "opencode")
	shared := filepath.Join(home, ".agents", "skills")
	return []agentConfig{
		{
			name:         "Claude",
			apps:         []string{"claude"},
			instructions: filepath.Join(home, ".claude", "CLAUDE.md"),
			skillDirs:    []string{filepath.Join(home, ".claude", "skills")},
			mcp:          []mcpSource{{filepath.Join(home, ".claude.json"), "mcpServers"}},
			connection:   func() string { return claudeConnection(home) },
		},
		{
			name:         "Codex",
			apps:         []string{"codex"},
			instructions: filepath.Join(codexDir, "AGENTS.md"),
			skillDirs:    []string{shared, filepath.Join(codexDir, "skills")},
			mcp:          []mcpSource{{codexConfigPath(home), "mcp_servers"}},
			connection:   func() string { return codexConnection(home) },
		},
		{
			name:         "Antigravity",
			apps:         []string{"antigravity", "antigravity-ide"},
			instructions: filepath.Join(gemini, "GEMINI.md"),
			skillDirs: []string{
				filepath.Join(gemini, "antigravity", "skills"),
				filepath.Join(gemini, "antigravity-cli", "skills"),
			},
			mcp: []mcpSource{
				{filepath.Join(gemini, "config", "mcp_config.json"), "mcpServers"},
				{filepath.Join(gemini, "antigravity", "mcp_config.json"), "mcpServers"},
				{filepath.Join(gemini, "antigravity-cli", "mcp_config.json"), "mcpServers"},
			},
			connection: func() string { return "trong app" },
		},
		{
			name:       "Cursor",
			apps:       []string{"cursor-agent", "cursor"},
			skillDirs:  []string{shared, filepath.Join(home, ".cursor", "skills")},
			mcp:        []mcpSource{{filepath.Join(home, ".cursor", "mcp.json"), "mcpServers"}},
			connection: func() string { return "trong app" },
		},
		{
			name:         "OpenCode",
			apps:         []string{"opencode"},
			instructions: filepath.Join(opencode, "AGENTS.md"),
			skillDirs: []string{
				filepath.Join(opencode, "skills"),
				filepath.Join(home, ".claude", "skills"),
				shared,
			},
			mcp: []mcpSource{
				{filepath.Join(opencode, "opencode.json"), "mcp"},
				{filepath.Join(opencode, "opencode.jsonc"), "mcp"},
			},
			connection: func() string { return opencodeConnection(opencode) },
		},
	}
}

// checkStatus only reads. It prints names, never values: MCP entries and config
// files can hold keys, so only object keys and gateway host names are reported.
func checkStatus(out io.Writer, home string, installed func(id string) bool) {
	fmt.Fprintf(out, "wj check %s - chỉ đọc, không ghi gì, không cần mạng\n\n", strings.TrimSpace(buildVersion))
	var states []agentState
	for _, config := range checkAgents(home) {
		states = append(states, readAgent(config, installed))
	}

	table := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(table, "AGENT\tCÀI\tKẾT NỐI\tCHỈ DẪN\tMCP\tSKILLS")
	for _, s := range states {
		fmt.Fprintf(table, "%s\t%s\t%s\t%s\t%s\t%d\n", s.config.name, yesNo(s.installed), s.config.connection(),
			instructionsLabel(s), namesOrDash(s.mcp), len(s.skills))
	}
	table.Flush()

	if profiles := profileNames(home); len(profiles) > 0 {
		fmt.Fprintln(out, "\nProfile đã lưu:", strings.Join(profiles, ", "))
	} else {
		fmt.Fprintln(out, "\nProfile đã lưu: chưa có")
	}
	for _, s := range states {
		for _, path := range s.unreadable {
			fmt.Fprintf(out, "Không đọc được %s (%s)\n", path, s.config.name)
		}
	}

	drift := checkDrift(states)
	if len(drift) == 0 {
		fmt.Fprintln(out, "\nCác agent đã cài đang khớp nhau.")
	} else {
		fmt.Fprintln(out, "\nLỆCH GIỮA CÁC AGENT ĐÃ CÀI")
		for _, line := range drift {
			fmt.Fprintln(out, " ", line)
		}
	}
}

func readAgent(config agentConfig, installed func(id string) bool) agentState {
	state := agentState{config: config}
	for _, id := range config.apps {
		state.installed = state.installed || installed(id)
	}
	if config.instructions != "" {
		_, err := os.Stat(config.instructions)
		state.instructions = err == nil
	}
	for _, dir := range config.skillDirs {
		state.skills = appendUnique(state.skills, skillNames(dir)...)
	}
	for _, source := range config.mcp {
		names, err := mcpNames(source)
		if err != nil {
			state.unreadable = append(state.unreadable, source.path)
			continue
		}
		state.mcp = appendUnique(state.mcp, names...)
	}
	sort.Strings(state.mcp)
	sort.Strings(state.skills)
	return state
}

// checkDrift reports what some installed agents have and others lack. Agents
// that are not installed are left out: they cannot be out of step.
func checkDrift(states []agentState) []string {
	var active []agentState
	for _, s := range states {
		if s.installed {
			active = append(active, s)
		}
	}
	var lines []string
	compare := func(label string, get func(agentState) []string) {
		owners := map[string][]string{}
		for _, s := range active {
			for _, name := range get(s) {
				owners[name] = append(owners[name], s.config.name)
			}
		}
		names := make([]string, 0, len(owners))
		for name := range owners {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			var missing []string
			for _, s := range active {
				if !containsName(owners[name], s.config.name) {
					missing = append(missing, s.config.name)
				}
			}
			if len(missing) > 0 {
				lines = append(lines, fmt.Sprintf("%s %s: có ở %s; thiếu ở %s", label, name,
					strings.Join(owners[name], ", "), strings.Join(missing, ", ")))
			}
		}
	}
	compare("MCP", func(s agentState) []string { return s.mcp })
	compare("Skill", func(s agentState) []string { return s.skills })

	var have, lack []string
	for _, s := range active {
		switch {
		case s.config.instructions == "":
		case s.instructions:
			have = append(have, s.config.name)
		default:
			lack = append(lack, s.config.name)
		}
	}
	if len(lack) > 0 && len(have) > 0 {
		lines = append(lines, fmt.Sprintf("Chỉ dẫn chung: có ở %s; thiếu ở %s", strings.Join(have, ", "), strings.Join(lack, ", ")))
	} else if len(lack) > 0 {
		lines = append(lines, "Chỉ dẫn chung: chưa agent nào có ("+strings.Join(lack, ", ")+")")
	}
	return lines
}

// skillNames lists folders holding a SKILL.md, which also skips hidden system
// folders and containers managed by the app itself.
func skillNames(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var names []string
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		if _, err := os.Stat(filepath.Join(dir, entry.Name(), "SKILL.md")); err == nil {
			names = append(names, entry.Name())
		}
	}
	return names
}

func mcpNames(source mcpSource) ([]string, error) {
	doc, err := readConfig(source.path)
	if err != nil {
		return nil, err
	}
	servers, _ := doc[source.key].(map[string]any)
	names := make([]string, 0, len(servers))
	for name := range servers {
		names = append(names, name)
	}
	return names, nil
}

// readConfig parses TOML, JSON or JSONC. A missing file is an empty document.
// Parser errors are dropped by callers because they can quote file contents.
func readConfig(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return map[string]any{}, nil
	}
	if err != nil {
		return nil, err
	}
	doc := map[string]any{}
	if strings.EqualFold(filepath.Ext(path), ".toml") {
		return doc, toml.Unmarshal(data, &doc)
	}
	clean, err := hujson.Standardize(data)
	if err != nil {
		return nil, err
	}
	return doc, json.Unmarshal(clean, &doc)
}

func claudeConnection(home string) string {
	doc, err := readConfig(filepath.Join(home, ".claude", "settings.json"))
	if err != nil {
		return "không đọc được"
	}
	env, _ := doc["env"].(map[string]any)
	base, _ := env["ANTHROPIC_BASE_URL"].(string)
	if base == "" {
		return "đăng nhập"
	}
	return "gateway " + hostOf(base)
}

func codexConnection(home string) string {
	doc, err := readConfig(codexConfigPath(home))
	if err != nil {
		return "không đọc được"
	}
	id, _ := doc["model_provider"].(string)
	if id == "" {
		return "đăng nhập"
	}
	providers, _ := doc["model_providers"].(map[string]any)
	provider, _ := providers[id].(map[string]any)
	base, _ := provider["base_url"].(string)
	if base == "" {
		return "provider " + id
	}
	return "gateway " + hostOf(base)
}

func opencodeConnection(dir string) string {
	var hosts []string
	for _, name := range []string{"opencode.json", "opencode.jsonc"} {
		doc, err := readConfig(filepath.Join(dir, name))
		if err != nil {
			return "không đọc được"
		}
		providers, _ := doc["provider"].(map[string]any)
		for _, value := range providers {
			provider, _ := value.(map[string]any)
			options, _ := provider["options"].(map[string]any)
			if base, _ := options["baseURL"].(string); base != "" {
				hosts = appendUnique(hosts, hostOf(base))
			}
		}
	}
	if len(hosts) == 0 {
		return "đăng nhập"
	}
	sort.Strings(hosts)
	return "gateway " + strings.Join(hosts, ", ")
}

// hostOf keeps only the host name: a URL can embed a user name and password.
func hostOf(raw string) string {
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Hostname() == "" {
		return "(địa chỉ lạ)"
	}
	return parsed.Hostname()
}

func profileNames(home string) []string {
	entries, err := os.ReadDir(filepath.Join(stateDir(home), "profiles"))
	if err != nil {
		return nil
	}
	var names []string
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".json") {
			names = append(names, strings.TrimSuffix(entry.Name(), ".json"))
		}
	}
	return names
}

func appendUnique(list []string, items ...string) []string {
	for _, item := range items {
		if !containsName(list, item) {
			list = append(list, item)
		}
	}
	return list
}

func containsName(list []string, name string) bool {
	for _, item := range list {
		if item == name {
			return true
		}
	}
	return false
}

func yesNo(value bool) string {
	if value {
		return "có"
	}
	return "chưa"
}

func instructionsLabel(state agentState) string {
	if state.config.instructions == "" {
		return "-"
	}
	return yesNo(state.instructions)
}

func namesOrDash(names []string) string {
	if len(names) == 0 {
		return "-"
	}
	return strings.Join(names, ", ")
}
