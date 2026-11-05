package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSummarizeUsageGroupsAndFilters(t *testing.T) {
	now := time.Now().UTC()
	old := now.Add(-48 * time.Hour).Format(time.RFC3339)
	recent := now.Add(-time.Hour).Format(time.RFC3339)
	log := strings.Join([]string{
		`{"ts":"` + recent + `","client":"claude","model":"cc/claude-opus-5-5","status":200,"input":100,"output":50,"cache_read":1000,"cache_write":10}`,
		`{"ts":"` + recent + `","client":"claude","model":"cc/claude-opus-5-5","status":200,"input":1,"output":2,"cache_read":3,"cache_write":4}`,
		`{"ts":"` + recent + `","client":"codex","model":"cx/gpt-6.1-sol","status":429,"input":0,"output":0}`,
		`{"ts":"` + old + `","client":"codex","model":"cx/gpt-6.1-sol","status":200,"input":999,"output":999}`,
		`not json`,
	}, "\n")
	list, total, err := summarizeUsage(strings.NewReader(log), now.Add(-24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if total.Calls != 3 || total.Failed != 1 || total.Input != 101 || total.CacheRead != 1003 || total.Total != 1170 {
		t.Fatalf("bad total %+v", total)
	}
	if len(list) != 2 || list[0].Client != "claude" || list[0].Calls != 2 {
		t.Fatalf("bad groups %+v", list)
	}
	_, all, _ := summarizeUsage(strings.NewReader(log), time.Time{})
	if all.Calls != 4 {
		t.Fatal("-all ignored old records")
	}
	if groupDigits(1234567) != "1,234,567" || groupDigits(12) != "12" {
		t.Fatal("digit grouping")
	}
}

func TestProxyEnvRoutesOnlyTheChild(t *testing.T) {
	env := proxyEnv([]string{"PATH=/bin", "https_proxy=http://old", "NODE_EXTRA_CA_CERTS=/old.pem", "KEEP=1"}, "http://127.0.0.1:8877", "/ca.pem")
	joined := "\n" + strings.Join(env, "\n") + "\n"
	for _, want := range []string{"\nPATH=/bin\n", "\nKEEP=1\n", "\nHTTPS_PROXY=http://127.0.0.1:8877\n", "\nNODE_EXTRA_CA_CERTS=/ca.pem\n", "\nCODEX_CA_CERTIFICATE=/ca.pem\n", "\nNO_PROXY=localhost,127.0.0.1,::1\n"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing %q in %v", want, env)
		}
	}
	if strings.Contains(joined, "http://old") || strings.Contains(joined, "/old.pem") {
		t.Fatal("stale proxy settings kept")
	}
}

func TestProxyUsageWithoutLogAndDryRuns(t *testing.T) {
	home := t.TempDir()
	var out bytes.Buffer
	if err := proxyUsage(nil, &out, home); err != nil || !strings.Contains(out.String(), "Chưa có dữ liệu") {
		t.Fatal(err, out.String())
	}
	if err := proxyStart([]string{"-dry-run", "-port", "9001"}, strings.NewReader(""), &out, home); err != nil {
		t.Fatal(err)
	}
	if err := proxyRun([]string{"-dry-run", "-proxy", "http://10.0.0.5:8080", "codex"}, strings.NewReader(""), &out, home); err != nil {
		t.Fatal(err)
	}
	if err := proxyRun([]string{"-proxy", "socks5://x", "codex"}, strings.NewReader(""), &out, home); err == nil {
		t.Fatal("unsupported proxy URL accepted")
	}
	if _, err := os.Stat(filepath.Join(home, ".setup-gateway")); !os.IsNotExist(err) {
		t.Fatal("dry run wrote files")
	}
	if !strings.Contains(out.String(), "9001") || !strings.Contains(out.String(), "10.0.0.5:8080") {
		t.Fatal(out.String())
	}
}

func TestEnsureMitmAutoInstallNeedsConsent(t *testing.T) {
	home := t.TempDir()
	t.Setenv("PATH", "")
	t.Setenv("ProgramFiles", home)
	t.Setenv("LOCALAPPDATA", home)
	var calls [][]string
	previous := installApps
	installApps = func(args []string, _ io.Reader, _ io.Writer) error {
		calls = append(calls, args)
		return nil
	}
	t.Cleanup(func() { installApps = previous })
	var out bytes.Buffer
	// Scripts (non-terminal input) without -yes must not install anything.
	if _, err := ensureMitm(home, "mitmdump", false, strings.NewReader("y\n"), &out); err == nil || len(calls) != 0 {
		t.Fatal("installed without consent", err, calls)
	}
	// -yes installs through the normal app manager path.
	if _, err := ensureMitm(home, "mitmdump", true, strings.NewReader(""), &out); err == nil {
		t.Fatal("stub install cannot produce a binary; expected the new-terminal hint")
	}
	if len(calls) != 1 || strings.Join(calls[0], " ") != "install mitmproxy" {
		t.Fatal("wrong install call", calls)
	}
	// Proxy run with -yes also reaches the installer when nothing listens on the port.
	self, _ := os.Executable()
	if err := proxyRun([]string{"-yes", "-port", "1", self}, strings.NewReader(""), &out, home); err == nil || len(calls) != 2 {
		t.Fatal("one-command mode did not auto-install", err, calls)
	}
}

func TestHelpTopicsAndAddonEmbedded(t *testing.T) {
	for _, topic := range []string{"", "claude", "codex", "proxy", "apps", "tools", "words"} {
		var out bytes.Buffer
		args := []string{"help"}
		if topic != "" {
			args = append(args, topic)
		}
		if err := runApp(args, strings.NewReader(""), &out); err != nil || out.Len() < 200 {
			t.Fatalf("help %q: %v", topic, err)
		}
	}
	if err := runApp([]string{"help", "nope"}, strings.NewReader(""), &bytes.Buffer{}); err == nil {
		t.Fatal("unknown help topic accepted")
	}
	if !bytes.Contains(proxyAddon, []byte("class UsageMeter")) {
		t.Fatal("mitmproxy addon not embedded")
	}
	// The usage log must never carry request content or credentials.
	for _, forbidden := range []string{"headers.items", "authorization\"]", "get_content()"} {
		if bytes.Contains(proxyAddon, []byte(forbidden)) {
			t.Fatalf("addon may record sensitive data: %s", forbidden)
		}
	}
}
