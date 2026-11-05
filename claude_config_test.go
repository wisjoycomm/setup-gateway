package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func testHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", filepath.Join(home, ".claude"))
	t.Setenv("SETUP_GATEWAY_KEY", "")
	return home
}

func putSettings(t *testing.T, home, data string) string {
	t.Helper()
	path := claudeSettingsPath(home)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestSwitchAuthenticationPreservesUserSettings(t *testing.T) {
	home := testHome(t)
	original := `{"env":{"ANTHROPIC_AUTH_TOKEN":"old-gateway","ANTHROPIC_BASE_URL":"https://old.example","ANTHROPIC_DEFAULT_OPUS_MODEL":"cc/opus","ANTHROPIC_DEFAULT_HAIKU_MODEL":"cc/haiku","ANTHROPIC_MODEL":"cc/opus","CLAUDE_CODE_SUBAGENT_MODEL":"cc/haiku","CLAUDE_CODE_USE_BEDROCK":"1","KEEP_ME":"yes"},"hooks":{"Stop":[{"hooks":[{"type":"command","command":"orca-hook"}]}]},"enabledPlugins":{"example":true},"statusLine":{"type":"command","command":"status"},"permissions":{"allow":["Read"]},"theme":"dark","model":"cc/opus","apiKeyHelper":"old-helper","modelOverrides":{"opus":"cc/opus"},"fallbackModel":"cc/sonnet","modelPicker":{"models":[]}}`
	path := putSettings(t, home, original)
	before, _ := readSettings(path)
	profiles := []connectionProfile{
		{Auth: "oauth", Credential: "sk-ant-oat-test-token"},
		{Auth: "api-key", Credential: "sk-ant-api-test"},
		{Auth: "login"},
		{Auth: "gateway", BaseURL: "https://new.example", Credential: "new-gateway", Header: "x-api-key", Models: []string{"cc/claude-sonnet", "cc/claude-haiku"}},
		{Auth: "oauth", Credential: "sk-ant-oat-second"},
	}
	for _, p := range profiles {
		if _, err := applyClaude(home, p, false); err != nil {
			t.Fatal(err)
		}
		after, err := readSettings(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, key := range []string{"hooks", "enabledPlugins", "statusLine", "permissions", "theme"} {
			if !reflect.DeepEqual(before[key], after[key]) {
				t.Fatalf("%s changed in %s mode", key, p.Auth)
			}
		}
		for _, key := range []string{"apiKeyHelper", "modelOverrides", "fallbackModel", "modelPicker"} {
			if _, ok := after[key]; ok {
				t.Fatalf("old route %s survived", key)
			}
		}
		env := after["env"].(map[string]any)
		if env["KEEP_ME"] != "yes" {
			t.Fatal("unrelated env removed")
		}
		if _, ok := env["CLAUDE_CODE_USE_BEDROCK"]; ok {
			t.Fatal("old provider survived")
		}
		if p.Auth != "gateway" {
			if after["model"] != "opus" {
				t.Fatal("model alias not reset")
			}
			for key := range env {
				if strings.HasPrefix(key, "ANTHROPIC_DEFAULT_") || key == "ANTHROPIC_BASE_URL" || key == "ANTHROPIC_AUTH_TOKEN" || key == "CLAUDE_CODE_SUBAGENT_MODEL" {
					t.Fatalf("gateway field survived: %s", key)
				}
			}
		}
		switch p.Auth {
		case "oauth":
			if env["CLAUDE_CODE_OAUTH_TOKEN"] != p.Credential || env["ANTHROPIC_API_KEY"] != nil {
				t.Fatal("wrong OAuth credentials")
			}
		case "api-key":
			if env["ANTHROPIC_API_KEY"] != p.Credential || env["CLAUDE_CODE_OAUTH_TOKEN"] != nil {
				t.Fatal("wrong API credentials")
			}
		case "login":
			if len(env) != 1 {
				t.Fatalf("login still has credentials: %v", env)
			}
		case "gateway":
			if env["ANTHROPIC_BASE_URL"] != "https://new.example" || env["ANTHROPIC_API_KEY"] != p.Credential || env["CLAUDE_CODE_OAUTH_TOKEN"] != nil {
				t.Fatal("wrong gateway credentials")
			}
			// An env model would override the user's later /model choice.
			if env["ANTHROPIC_MODEL"] != nil || after["model"] != "cc/claude-sonnet" {
				t.Fatal("sonnet-only gateway did not select its model")
			}
		}
	}
	backups, _ := filepath.Glob(path + ".bak-*")
	if len(backups) != len(profiles) {
		t.Fatalf("want %d backups, got %d", len(profiles), len(backups))
	}
	data, _ := os.ReadFile(backups[0])
	if string(data) != original {
		t.Fatal("first backup differs from original")
	}
	if runtime.GOOS != "windows" {
		for _, file := range append(backups, path) {
			info, _ := os.Stat(file)
			if info.Mode().Perm() != 0o600 {
				t.Fatalf("insecure permissions on %s", file)
			}
		}
	}
}

func TestInvalidSettingsAreNotReplaced(t *testing.T) {
	for _, input := range []string{"broken {", "null", `{"env":"not-an-object"}`, `{"env":null}`} {
		t.Run(input, func(t *testing.T) {
			home := testHome(t)
			path := putSettings(t, home, input)
			if _, err := applyClaude(home, connectionProfile{Auth: "oauth", Credential: "sk-ant-oat-test"}, false); err == nil {
				t.Fatal("expected error")
			}
			data, _ := os.ReadFile(path)
			if string(data) != input {
				t.Fatal("invalid file was overwritten")
			}
			backups, _ := filepath.Glob(path + ".bak-*")
			if len(backups) != 0 {
				t.Fatal("unexpected backup/write")
			}
		})
	}
}

func TestProfilesKeepCredentialsOutOfListing(t *testing.T) {
	home := testHome(t)
	p := connectionProfile{Auth: "oauth", Credential: "sk-ant-oat-sensitive-test"}
	if err := saveProfile(home, "work", p, false); err != nil {
		t.Fatal(err)
	}
	loaded, err := loadProfile(home, "work")
	if err != nil || loaded.Credential != p.Credential {
		t.Fatalf("profile round trip failed: %v", err)
	}
	var out bytes.Buffer
	if err := listProfiles(home, &out); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), p.Credential) || !strings.Contains(out.String(), "work: oauth") {
		t.Fatalf("bad listing: %s", out.String())
	}
	for _, name := range []string{"../other", "a/b", `a\b`, ".", ""} {
		if err := saveProfile(home, name, p, false); err == nil {
			t.Fatalf("accepted %q", name)
		}
	}
}

func TestCleanChildEnvironment(t *testing.T) {
	input := []string{"PATH=/bin", "CLAUDE_CONFIG_DIR=/private", "ANTHROPIC_AUTH_TOKEN=secret", "anthropic_base_url=wrong", "CLAUDE_CODE_OAUTH_TOKEN=secret", "ANTHROPIC_DEFAULT_OPUS_MODEL=cc/opus", "SETUP_GATEWAY_KEY=secret", "KEEP=value"}
	want := []string{"PATH=/bin", "CLAUDE_CONFIG_DIR=/private", "KEEP=value"}
	if got := cleanClaudeEnvironment(input); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v", got)
	}
}

func TestCLIProfileActivationAndDryRun(t *testing.T) {
	home := testHome(t)
	var out bytes.Buffer
	args := []string{"-auth", "oauth", "-profile", "work", "-key-stdin"}
	if err := runCLIWithHome(args, strings.NewReader("sk-ant-oat-test\n"), &out, home); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "sk-ant-oat-test") {
		t.Fatal("credential printed")
	}
	if err := runCLIWithHome([]string{"-auth", "login", "-no-login"}, strings.NewReader(""), &out, home); err != nil {
		t.Fatal(err)
	}
	if err := runCLIWithHome([]string{"-use-profile", "work"}, strings.NewReader(""), &out, home); err != nil {
		t.Fatal(err)
	}
	path := claudeSettingsPath(home)
	data, _ := os.ReadFile(path)
	var root map[string]any
	if err := json.Unmarshal(data, &root); err != nil {
		t.Fatal(err)
	}
	if root["env"].(map[string]any)["CLAUDE_CODE_OAUTH_TOKEN"] != "sk-ant-oat-test" {
		t.Fatal("profile not activated")
	}
	args = []string{"-auth", "oauth", "-key", "sk-ant-oat-different", "-profile", "dry", "-dry-run", "-install-claude", "-run"}
	if err := runCLIWithHome(args, strings.NewReader(""), &out, home); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(data, after) {
		t.Fatal("dry run changed settings")
	}
	profile, _ := profilePath(home, "dry")
	if _, err := os.Stat(profile); !os.IsNotExist(err) {
		t.Fatal("dry run wrote a profile")
	}
}
