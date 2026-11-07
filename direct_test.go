package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDirectRemovesProjectProxyKeepsGateway(t *testing.T) {
	home := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	ca := mitmCAPath(home)
	_ = os.MkdirAll(filepath.Dir(ca), 0o700)
	_ = os.WriteFile(ca, []byte("ca"), 0o600)
	root := filepath.Join(t.TempDir(), "game")
	_ = os.MkdirAll(filepath.Join(root, ".git"), 0o700)
	var out bytes.Buffer
	if err := projectOn([]string{"-dir", root}, &out, home); err != nil {
		t.Fatal(err, out.String())
	}
	settings := claudeSettingsPath(home)
	_ = os.MkdirAll(filepath.Dir(settings), 0o700)
	gateway := `{"env":{"ANTHROPIC_BASE_URL":"https://gw.example","ANTHROPIC_AUTH_TOKEN":"secret"}}`
	_ = os.WriteFile(settings, []byte(gateway), 0o600)

	out.Reset()
	if err := runDirect([]string{"-dry-run"}, &out, home); err != nil {
		t.Fatal(err, out.String())
	}
	if !strings.Contains(out.String(), root) {
		t.Fatalf("dry run should list the project: %s", out.String())
	}
	if doc, _ := readJSONObject(projectSettingsPath(root)); doc["env"] == nil {
		t.Fatal("dry run changed the project settings")
	}

	out.Reset()
	if err := runDirect(nil, &out, home); err != nil {
		t.Fatal(err, out.String())
	}
	if _, err := os.Stat(projectSettingsPath(root)); err == nil {
		t.Fatal("project proxy settings should be gone")
	}
	if len(loadProjectIndex(home)) != 0 {
		t.Fatal("project index should be empty")
	}
	if data, _ := os.ReadFile(settings); string(data) != gateway {
		t.Fatalf("gateway settings must stay untouched: %s", data)
	}
}

func TestDirectRejectsExtraArgs(t *testing.T) {
	if err := runDirect([]string{"now"}, &bytes.Buffer{}, t.TempDir()); err == nil {
		t.Fatal("want usage error")
	}
}
