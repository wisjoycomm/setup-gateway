package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func claudeSettingsPath(home string) string {
	if dir := os.Getenv("CLAUDE_CONFIG_DIR"); dir != "" {
		return filepath.Join(dir, "settings.json")
	}
	return filepath.Join(home, ".claude", "settings.json")
}

func readSettings(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return map[string]any{}, nil
	}
	if err != nil {
		return nil, err
	}
	var root map[string]any
	if err := json.Unmarshal(data, &root); err != nil || root == nil {
		return nil, fmt.Errorf("file %s bị lỗi cú pháp JSON; tool không sửa nó, hãy sửa tay rồi chạy lại", path)
	}
	if env, exists := root["env"]; exists {
		if _, ok := env.(map[string]any); !ok {
			return nil, fmt.Errorf("mục env trong %s không đúng dạng; tool không sửa file", path)
		}
	}
	return root, nil
}

func isClaudeRoutingKey(key string) bool {
	key = strings.ToUpper(key)
	if strings.HasPrefix(key, "ANTHROPIC_DEFAULT_") || strings.HasPrefix(key, "ANTHROPIC_CUSTOM_MODEL_") {
		return true
	}
	return contains([]string{
		"ANTHROPIC_BASE_URL", "ANTHROPIC_AUTH_TOKEN", "ANTHROPIC_API_KEY", "ANTHROPIC_MODEL", "ANTHROPIC_SMALL_FAST_MODEL", "ANTHROPIC_CUSTOM_HEADERS",
		"ANTHROPIC_PROFILE", "ANTHROPIC_FEDERATION_RULE_ID", "ANTHROPIC_ORGANIZATION_ID",
		"CLAUDE_CODE_OAUTH_TOKEN", "CLAUDE_CODE_OAUTH_REFRESH_TOKEN", "CLAUDE_CODE_OAUTH_SCOPES",
		"CLAUDE_CODE_SUBAGENT_MODEL", "CLAUDE_CODE_SUBAGENT_MODEL_FORCE", "CLAUDE_CODE_ENABLE_GATEWAY_MODEL_DISCOVERY",
		"CLAUDE_CODE_USE_BEDROCK", "CLAUDE_CODE_USE_VERTEX", "CLAUDE_CODE_USE_FOUNDRY", "CLAUDE_CODE_USE_MANTLE",
	}, key)
}

func applyClaude(home string, p connectionProfile, dry bool) (string, error) {
	if err := p.validate(); err != nil {
		return "", err
	}
	path := claudeSettingsPath(home)
	root, err := readSettings(path)
	if err != nil {
		return "", err
	}
	env, _ := root["env"].(map[string]any)
	if env == nil {
		env = map[string]any{}
	}
	for key := range env {
		if isClaudeRoutingKey(key) {
			delete(env, key)
		}
	}
	// The old provider must not remap aliases or supply an alternate credential.
	for _, key := range []string{"apiKeyHelper", "modelOverrides", "modelPicker", "fallbackModel"} {
		delete(root, key)
	}
	root["model"] = "opus"
	switch p.Auth {
	case "oauth":
		env["CLAUDE_CODE_OAUTH_TOKEN"] = p.Credential
	case "api-key":
		env["ANTHROPIC_API_KEY"] = p.Credential
	case "gateway":
		env["ANTHROPIC_BASE_URL"] = strings.TrimSuffix(p.BaseURL, "/v1")
		if p.Header == "x-api-key" {
			env["ANTHROPIC_API_KEY"] = p.Credential
		} else {
			env["ANTHROPIC_AUTH_TOKEN"] = p.Credential
		}
		for _, family := range []string{"OPUS", "SONNET", "HAIKU", "FABLE"} {
			if model := newestModel(p.Models, strings.ToLower(family)); model != "" {
				env["ANTHROPIC_DEFAULT_"+family+"_MODEL"] = model
			}
		}
		// Claude Code asks the gateway for its model list at every start, so
		// /model always shows what the gateway currently serves.
		env["CLAUDE_CODE_ENABLE_GATEWAY_MODEL_DISCOVERY"] = "1"
		model := pick(p.Models, []string{"opus"}, []string{"review"})
		if model == "" {
			model = pick(p.Models, []string{"sonnet"}, []string{"review"})
		}
		if model == "" {
			model = pick(p.Models, []string{"claude"}, []string{"review"})
		}
		if model == "" && len(p.Models) > 0 {
			model = p.Models[0]
		}
		if model != "" {
			env["ANTHROPIC_MODEL"] = model
			root["model"] = model
		}
		if sub := pick(p.Models, []string{"haiku"}, nil); sub != "" {
			env["CLAUDE_CODE_SUBAGENT_MODEL"] = sub
		}
	}
	root["env"] = env
	data, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return "", err
	}
	if err := writeFile(path, string(data)+"\n", dry); err != nil {
		return "", err
	}
	return fmt.Sprintf("%s (%s; giữ nguyên hooks, plugin và cài đặt khác)", path, p.Auth), nil
}

// newestModel returns the latest gateway ID of a family: namespaced gateway IDs
// (cc/claude-fable-5-1) win over bare aliases, then the highest version string.
func newestModel(ids []string, family string) string {
	best := ""
	for _, id := range ids {
		low := strings.ToLower(id)
		if !strings.Contains(low, family) || strings.Contains(low, "review") {
			continue
		}
		if best == "" || newerModel(id, best) {
			best = id
		}
	}
	return best
}

// modelVersion extracts the dotted/hyphenated numbers after the family name:
// "cx/gpt-6.1-sol" -> [6 1], "cc/claude-fable-5-1" -> [5 1]. Dates such as
// 20251001 are ignored so they do not outrank real version numbers.
func modelVersion(id string) []int {
	var nums []int
	for _, part := range strings.FieldsFunc(id, func(r rune) bool { return r == '-' || r == '.' || r == '/' || r == '[' || r == ']' }) {
		n, err := strconv.Atoi(part)
		if err == nil && n < 1000 {
			nums = append(nums, n)
		}
	}
	return nums
}

// newerModel orders models: higher version first, then plain (no [1m] or
// -high style suffix), then namespaced gateway IDs, then name.
func newerModel(a, b string) bool {
	va, vb := modelVersion(a), modelVersion(b)
	for i := 0; i < len(va) && i < len(vb); i++ {
		if va[i] != vb[i] {
			return va[i] > vb[i]
		}
	}
	if len(va) != len(vb) {
		return len(va) > len(vb)
	}
	plain := func(s string) bool { return !strings.ContainsAny(s, "[]") }
	if plain(a) != plain(b) {
		return plain(a)
	}
	if strings.Contains(a, "/") != strings.Contains(b, "/") {
		return strings.Contains(a, "/")
	}
	return len(a) < len(b) || len(a) == len(b) && a < b
}

func cleanClaudeEnvironment(values []string) []string {
	var result []string
	for _, value := range values {
		name, _, _ := strings.Cut(value, "=")
		if !isClaudeRoutingKey(name) && !strings.EqualFold(name, "SETUP_GATEWAY_KEY") {
			result = append(result, value)
		}
	}
	return result
}

func warnEnvironment(out io.Writer) {
	var names []string
	for _, value := range os.Environ() {
		name, v, _ := strings.Cut(value, "=")
		if v != "" && isClaudeRoutingKey(name) {
			names = append(names, name)
		}
	}
	if len(names) > 0 {
		fmt.Fprintf(out, "Chú ý: terminal đang có biến %s (cấu hình cũ). Xoá chúng, hoặc mở Claude bằng -run để bỏ qua.\n", strings.Join(names, ", "))
	}
}
