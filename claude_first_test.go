package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func init() {
	// Unit tests never touch a network: unless a test says otherwise, every model works.
	verifyClaudeModels = func(_ connectionProfile, ids []string) []string { return ids }
}

func stubVerify(t *testing.T, works func(id string) bool) {
	t.Helper()
	saved := verifyClaudeModels
	verifyClaudeModels = func(_ connectionProfile, ids []string) []string {
		var kept []string
		for _, id := range ids {
			if works(id) {
				kept = append(kept, id)
			}
		}
		return kept
	}
	t.Cleanup(func() { verifyClaudeModels = saved })
}

func gatewayProfile(models ...string) connectionProfile {
	return connectionProfile{Auth: "gateway", BaseURL: "https://gw.example/v1", Credential: testKey, Header: "bearer", Models: models}
}

func TestPickerDropsModelsThatFailClaudeCodeRequest(t *testing.T) {
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	stubVerify(t, func(id string) bool { return strings.Contains(id, "claude") || strings.Contains(id, "ok") })
	home := t.TempDir()
	line, err := applyClaude(home, gatewayProfile("cc/claude-opus-9", "cx/ok-1", "cx/bad-2"), false)
	if err != nil {
		t.Fatal(err)
	}
	text := readText(t, claudeSettingsPath(home))
	if !strings.Contains(text, "cx/ok-1") || strings.Contains(text, "cx/bad-2") {
		t.Errorf("picker should keep only the working model:\n%s", text)
	}
	if !strings.Contains(line, "bỏ 1 model") {
		t.Errorf("dropped model not reported: %s", line)
	}
}

func TestDefaultModelFallsBackToWorkingClaude(t *testing.T) {
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	stubVerify(t, func(id string) bool { return id == "cc/claude-sonnet-9" })
	home := t.TempDir()
	line, err := applyClaude(home, gatewayProfile("cc/claude-opus-9", "cc/claude-sonnet-9"), false)
	if err != nil {
		t.Fatal(err)
	}
	settings, _ := readSettings(claudeSettingsPath(home))
	if settings["model"] != "cc/claude-sonnet-9" || !strings.Contains(line, "đổi sang cc/claude-sonnet-9") {
		t.Errorf("model = %v, line = %s", settings["model"], line)
	}
}

func TestGatewayStillWrittenWhenNoClaudeModelAnswers(t *testing.T) {
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	stubVerify(t, func(string) bool { return false })
	home := t.TempDir()
	line, err := applyClaude(home, gatewayProfile("cc/claude-opus-9", "cx/gpt-9"), false)
	if err != nil {
		t.Fatal(err)
	}
	text := readText(t, claudeSettingsPath(home))
	if !strings.Contains(text, "https://gw.example") || strings.Contains(text, "modelPicker") || !strings.Contains(line, "CHƯA kiểm chứng") {
		t.Errorf("line = %s\n%s", line, text)
	}
}

func TestDryRunDoesNotVerifyModels(t *testing.T) {
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	saved := verifyClaudeModels
	verifyClaudeModels = func(connectionProfile, []string) []string {
		t.Fatal("dry run must not call the network")
		return nil
	}
	t.Cleanup(func() { verifyClaudeModels = saved })
	if _, err := applyClaude(t.TempDir(), gatewayProfile("cc/claude-opus-9", "cx/gpt-9"), true); err != nil {
		t.Fatal(err)
	}
}

// On 2026-11-06 a real gateway answered 400 "reasoning_effort: Extra inputs are
// not permitted" for Claude models when the request carried Claude Code's
// identity, and 200 for the same body from a plain client. A probe without that
// identity said "OK" while the app failed.
func TestProbeUsesClaudeCodeIdentityAndCatchesGatewayBug(t *testing.T) {
	var sawBeta bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req struct{ Model string }
		_ = json.Unmarshal(body, &req)
		claudeCode := strings.HasPrefix(r.Header.Get("User-Agent"), "claude-cli/") && r.Header.Get("x-app") == "cli"
		sawBeta = sawBeta || r.URL.Query().Get("beta") == "true" && r.Header.Get("anthropic-beta") != ""
		if claudeCode && strings.Contains(req.Model, "claude") {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, `{"type":"error","error":{"message":"reasoning_effort: Extra inputs are not permitted"}}`)
			return
		}
		fmt.Fprint(w, "event: message_start\ndata: {\"type\":\"message_start\"}\n\n")
	}))
	t.Cleanup(server.Close)
	conn := connectionProfile{Auth: "gateway", BaseURL: server.URL, Credential: testKey, Header: "bearer"}
	got := probeClaudeModels(conn, []string{"cc/claude-opus-9", "ds/deepseek-9"})
	if want := []string{"ds/deepseek-9"}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	if !sawBeta {
		t.Error("probe did not send ?beta=true and the anthropic-beta header")
	}
}

func TestDefaultUsesVerifiedOtherModelWhenNoClaudeWorks(t *testing.T) {
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	stubVerify(t, func(id string) bool { return id == "ds/ok-1" })
	home := t.TempDir()
	line, err := applyClaude(home, gatewayProfile("cc/claude-opus-9", "ds/ok-1", "ds/bad-2"), false)
	if err != nil {
		t.Fatal(err)
	}
	settings, _ := readSettings(claudeSettingsPath(home))
	if settings["model"] != "ds/ok-1" || !strings.Contains(line, "không model Claude nào chạy được") {
		t.Errorf("model = %v, line = %s", settings["model"], line)
	}
}

func TestNeutralHeadersAreOptInAndRemovedWhenOff(t *testing.T) {
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	home := t.TempDir()
	headers := func() any {
		settings, _ := readSettings(claudeSettingsPath(home))
		env, _ := settings["env"].(map[string]any)
		return env["ANTHROPIC_CUSTOM_HEADERS"]
	}
	p := gatewayProfile("cc/claude-opus-9")
	if _, err := applyClaude(home, p, false); err != nil || headers() != nil {
		t.Fatalf("default must not touch headers: %v %v", err, headers())
	}
	p.NeutralHeaders = true
	line, err := applyClaude(home, p, false)
	if err != nil || headers() != neutralHeaders || !strings.Contains(line, "header trung tính") {
		t.Fatalf("headers = %v, line = %s, err = %v", headers(), line, err)
	}
	p.NeutralHeaders = false
	if _, err := applyClaude(home, p, false); err != nil || headers() != nil {
		t.Fatalf("headers must be removed when turned off: %v %v", err, headers())
	}
}

func TestNeutralHeadersFixGatewayThatRejectsClaudeCode(t *testing.T) {
	var agents []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		agents = append(agents, r.Header.Get("User-Agent")+"|"+r.Header.Get("x-app"))
		if strings.HasPrefix(r.Header.Get("User-Agent"), "claude-cli/") || r.Header.Get("x-app") == "cli" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		fmt.Fprint(w, "event: message_start\ndata: {\"type\":\"message_start\"}\n\n")
	}))
	t.Cleanup(server.Close)
	conn := connectionProfile{Auth: "gateway", BaseURL: server.URL, Credential: testKey, Header: "bearer"}
	if got := probeClaudeModels(conn, []string{"cc/claude-opus-9"}); len(got) != 0 {
		t.Fatalf("Claude Code identity should be rejected, got %v", got)
	}
	conn.NeutralHeaders = true
	if got := probeClaudeModels(conn, []string{"cc/claude-opus-9"}); len(got) != 1 {
		t.Fatalf("neutral headers should pass, got %v", got)
	}
	if last := agents[len(agents)-1]; last != neutralUserAgent+"|"+neutralApp {
		t.Errorf("neutral probe sent %q", last)
	}
}

func TestNeutralHeadersFlagIsRememberedByProfile(t *testing.T) {
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	home := t.TempDir()
	if err := saveProfile(home, "gw", gatewayProfile("cc/claude-opus-9"), false); err != nil {
		t.Fatal(err)
	}
	var out strings.Builder
	if err := runCLIWithHome([]string{"-use-profile", "gw", "-neutral-headers", "-skip-check"}, strings.NewReader(""), &out, home); err != nil {
		t.Fatal(err, out.String())
	}
	if p, _ := loadProfile(home, "gw"); !p.NeutralHeaders {
		t.Error("profile did not remember -neutral-headers")
	}
	out.Reset()
	if err := runCLIWithHome([]string{"-use-profile", "gw", "-skip-check"}, strings.NewReader(""), &out, home); err != nil {
		t.Fatal(err, out.String())
	}
	if p, _ := loadProfile(home, "gw"); !p.NeutralHeaders {
		t.Error("plain -use-profile must keep the saved choice")
	}
	if err := runCLIWithHome([]string{"-use-profile", "gw", "-neutral-headers=false", "-skip-check"}, strings.NewReader(""), &out, home); err != nil {
		t.Fatal(err, out.String())
	}
	if p, _ := loadProfile(home, "gw"); p.NeutralHeaders {
		t.Error("-neutral-headers=false did not turn it off")
	}
}

func TestNeutralHeadersRejectedForNonGateway(t *testing.T) {
	var out strings.Builder
	err := runCLIWithHome([]string{"-auth", "login", "-no-login", "-neutral-headers"}, strings.NewReader(""), &out, t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "Gateway") {
		t.Fatalf("want a Gateway-only error, got %v", err)
	}
}

func TestProbeClaudeModelsKeepsOnlyWorkingOnes(t *testing.T) {
	server := fakeGateway(t)
	conn := connectionProfile{Auth: "gateway", BaseURL: server.URL, Credential: testKey, Header: "bearer"}
	got := probeClaudeModels(conn, []string{"cc/claude-opus-9", "cx/gpt-9", "cc/claude-sonnet-9"})
	if want := []string{"cc/claude-opus-9", "cc/claude-sonnet-9"}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}
