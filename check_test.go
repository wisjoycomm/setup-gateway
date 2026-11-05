package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTestFile(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
}

func checkFixture(t *testing.T) string {
	t.Helper()
	t.Setenv("CODEX_HOME", "")
	home := t.TempDir()
	writeTestFile(t, filepath.Join(home, ".claude.json"),
		`{"mcpServers":{"blender":{"env":{"TOKEN":"secret-mcp-value"}},"rea":{}}}`)
	writeTestFile(t, filepath.Join(home, ".claude", "settings.json"),
		`{"env":{"ANTHROPIC_BASE_URL":"https://user:secret-url-value@ai.example.com/v1","ANTHROPIC_AUTH_TOKEN":"secret-token-value"}}`)
	writeTestFile(t, filepath.Join(home, ".claude", "CLAUDE.md"), "rules")
	writeTestFile(t, filepath.Join(home, ".claude", "skills", "shared-skill", "SKILL.md"), "x")
	writeTestFile(t, filepath.Join(home, ".claude", "skills", "synced", "bucket", "SKILL.md"), "x")
	writeTestFile(t, filepath.Join(home, ".agents", "skills", "shared-skill", "SKILL.md"), "x")
	writeTestFile(t, filepath.Join(home, ".agents", "skills", "codex-only", "SKILL.md"), "x")
	writeTestFile(t, filepath.Join(home, ".codex", "skills", ".system", "SKILL.md"), "x")
	writeTestFile(t, filepath.Join(home, ".codex", "config.toml"),
		"model_provider = \"gw\"\n[model_providers.gw]\nbase_url = \"https://ai.example.com\"\n[mcp_servers.rea]\ncommand = \"secret-toml-value\"\n")
	writeTestFile(t, filepath.Join(home, ".cursor", "mcp.json"), `{"mcpServers":{"only-cursor":{}}}`)
	writeTestFile(t, filepath.Join(home, ".wj", "profiles", "adnx.json"), `{"credential":"secret-profile-value"}`)
	return home
}

func installedOnly(ids ...string) func(string) bool {
	return func(id string) bool { return containsName(ids, id) }
}

func TestCheckStatusReportsDriftBetweenInstalledAgents(t *testing.T) {
	home := checkFixture(t)
	var out bytes.Buffer
	checkStatus(&out, home, installedOnly("claude", "codex"))
	text := out.String()
	for _, want := range []string{
		"MCP blender: có ở Claude; thiếu ở Codex",
		"Skill codex-only: có ở Codex; thiếu ở Claude",
		"Chỉ dẫn chung: có ở Claude; thiếu ở Codex",
		"gateway ai.example.com",
		"Profile đã lưu: adnx",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q in:\n%s", want, text)
		}
	}
	for _, unwanted := range []string{
		"MCP rea:", "Skill shared-skill:", "only-cursor:", "synced", ".system",
	} {
		if strings.Contains(text, unwanted) {
			t.Errorf("unexpected %q in:\n%s", unwanted, text)
		}
	}
}

func TestCheckStatusNeverPrintsValues(t *testing.T) {
	home := checkFixture(t)
	var out bytes.Buffer
	checkStatus(&out, home, installedOnly("claude", "codex", "cursor"))
	if strings.Contains(out.String(), "secret") {
		t.Fatalf("value leaked:\n%s", out.String())
	}
}

func TestCheckStatusChangesNothing(t *testing.T) {
	home := checkFixture(t)
	snapshot := func() map[string]string {
		files := map[string]string{}
		_ = filepath.Walk(home, func(path string, info os.FileInfo, err error) error {
			if err == nil && !info.IsDir() {
				data, _ := os.ReadFile(path)
				files[path] = string(data)
			}
			return nil
		})
		return files
	}
	before := snapshot()
	var out bytes.Buffer
	checkStatus(&out, home, installedOnly("claude"))
	after := snapshot()
	if len(before) != len(after) {
		t.Fatalf("file count changed: %d -> %d", len(before), len(after))
	}
	for path, text := range before {
		if after[path] != text {
			t.Errorf("%s changed", path)
		}
	}
}

func TestCheckStatusReportsBrokenConfigWithoutContents(t *testing.T) {
	home := checkFixture(t)
	path := filepath.Join(home, ".cursor", "mcp.json")
	writeTestFile(t, path, `{"mcpServers": secret-broken-value`)
	var out bytes.Buffer
	checkStatus(&out, home, installedOnly("cursor"))
	if !strings.Contains(out.String(), "Không đọc được "+path) {
		t.Errorf("broken file not reported:\n%s", out.String())
	}
	if strings.Contains(out.String(), "secret") {
		t.Errorf("contents leaked:\n%s", out.String())
	}
}

func TestCheckDriftIgnoresAgentsNotInstalled(t *testing.T) {
	states := []agentState{
		{config: agentConfig{name: "A"}, installed: true, mcp: []string{"x"}},
		{config: agentConfig{name: "B"}, installed: false},
	}
	if lines := checkDrift(states); len(lines) != 0 {
		t.Fatalf("drift = %v", lines)
	}
}

func TestStateDirUsesLegacyFolderUntilMigrated(t *testing.T) {
	home := t.TempDir()
	current := filepath.Join(home, ".wj")
	legacy := filepath.Join(home, "."+legacyName)
	if got := stateDir(home); got != current {
		t.Fatalf("fresh machine: %s", got)
	}
	writeTestFile(t, filepath.Join(legacy, "profiles", "a.json"), "{}")
	if got := stateDir(home); got != legacy {
		t.Fatalf("before migration: %s", got)
	}
	migrateState(home)
	if got := stateDir(home); got != current {
		t.Fatalf("after migration: %s", got)
	}
	if _, err := os.Stat(filepath.Join(current, "profiles", "a.json")); err != nil {
		t.Fatal(err)
	}
	if isDir(legacy) {
		t.Fatal("legacy folder should be gone")
	}
}

func TestMigrateStateKeepsExistingNewFolder(t *testing.T) {
	home := t.TempDir()
	writeTestFile(t, filepath.Join(home, ".wj", "keep.txt"), "new")
	writeTestFile(t, filepath.Join(home, "."+legacyName, "old.txt"), "old")
	migrateState(home)
	if _, err := os.Stat(filepath.Join(home, ".wj", "keep.txt")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(home, "."+legacyName, "old.txt")); err != nil {
		t.Fatal("legacy data must not be touched when both exist")
	}
}

func TestEnsureShortCommandCopiesOnlyLegacyBinary(t *testing.T) {
	dir := t.TempDir()
	legacy := filepath.Join(dir, legacyName+".exe")
	writeTestFile(t, legacy, "program")
	var out bytes.Buffer
	if err := ensureShortCommand(legacy, &out); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "wj.exe"))
	if err != nil || string(data) != "program" {
		t.Fatalf("wj.exe = %q, %v", data, err)
	}
	if _, err := os.Stat(legacy); err != nil {
		t.Fatal("legacy binary must stay in place")
	}

	other := filepath.Join(t.TempDir(), "wj.exe")
	writeTestFile(t, other, "program")
	out.Reset()
	if err := ensureShortCommand(other, &out); err != nil || out.Len() != 0 {
		t.Fatalf("new name should do nothing: %v %q", err, out.String())
	}
}

func TestShouldMigrateSkipsReadOnlyCommands(t *testing.T) {
	for _, args := range [][]string{{"check"}, {"version"}, {"help"}, {"apps", "install", "all", "-dry-run"}} {
		if shouldMigrate(args) {
			t.Errorf("%v should not migrate", args)
		}
	}
	for _, args := range [][]string{nil, {"proxy", "start"}, {"configure", "codex"}} {
		if !shouldMigrate(args) {
			t.Errorf("%v should migrate", args)
		}
	}
}
