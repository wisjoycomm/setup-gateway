package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestAppInstallSelectionAndSkip(t *testing.T) {
	var installed []string
	m := appManager{home: t.TempDir(), find: func(a appSpec, _ string) string {
		if a.ID == "claude" {
			return "/already/claude"
		}
		return ""
	}, install: func(a appSpec, p installPlan, _ io.Reader, _ io.Writer) error {
		installed = append(installed, a.ID)
		return nil
	}}
	var out bytes.Buffer
	if err := m.run([]string{"install", "claude,codex,orca"}, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if strings.Join(installed, ",") != "codex,orca" || !strings.Contains(out.String(), "SKIP") {
		t.Fatal(installed, out.String())
	}
	installed = nil
	if err := m.run([]string{"update", "claude"}, strings.NewReader(""), &out); err != nil || len(installed) != 1 {
		t.Fatal("explicit update skipped", err)
	}
	installed = nil
	if err := m.run([]string{"install", "codex,invalid"}, strings.NewReader(""), &out); err == nil || len(installed) != 0 {
		t.Fatal("invalid selection made partial changes")
	}
	if err := m.run([]string{"install", "codex,orca", "-dry-run"}, strings.NewReader(""), &out); err != nil || len(installed) != 0 {
		t.Fatal("dry run installed", err)
	}
	if err := m.run([]string{"status"}, strings.NewReader(""), &out); err != nil || len(installed) != 0 {
		t.Fatal("status installed", err)
	}
}

func TestInstallFailureContinuesAndReturnsError(t *testing.T) {
	var ids []string
	m := appManager{home: t.TempDir(), find: func(appSpec, string) string { return "" }, install: func(a appSpec, _ installPlan, _ io.Reader, _ io.Writer) error {
		ids = append(ids, a.ID)
		if a.ID == "claude" {
			return errors.New("offline")
		}
		return nil
	}}
	if err := m.run([]string{"install", "claude,codex"}, strings.NewReader(""), io.Discard); err == nil || len(ids) != 2 {
		t.Fatal(ids, err)
	}
}

func TestInstallerPlans(t *testing.T) {
	for _, platform := range []string{"windows", "darwin", "linux"} {
		for _, arch := range []string{"amd64", "arm64"} {
			for _, app := range appCatalog {
				p, err := planApp(app, platform, arch, false)
				if platform == "linux" && (app.ID == "cursor" || app.ID == "antigravity-ide") {
					if err == nil {
						t.Fatal("unsupported installer invented")
					}
					continue
				}
				if err != nil {
					t.Fatal(platform, arch, app.ID, err)
				}
				if p.URL == "" && p.Repo == "" && len(p.Command) == 0 {
					t.Fatal("empty installer")
				}
			}
		}
	}
	app, _ := appByID("orca")
	for _, p := range appCandidates(app, t.TempDir(), "linux") {
		if filepath.Base(p) == "orca" {
			t.Fatal("GNOME screen reader mistaken for Orca")
		}
	}
	if selfAsset("windows", "arm64") != "wj-windows-arm64.exe" {
		t.Fatal("wrong Windows ARM asset")
	}
}

func TestCachedDownloadIntegrityAndReuse(t *testing.T) {
	payload := []byte("fake installer, never executed")
	digest := sha256.Sum256(payload)
	expected := hex.EncodeToString(digest[:])
	calls := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++; _, _ = w.Write(payload) }))
	defer s.Close()
	a := releaseAsset{Name: "test.exe", URL: s.URL}
	cache := t.TempDir()
	path, err := cachedAsset(a, expected, cache)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = cachedAsset(a, expected, cache); err != nil || calls != 1 {
		t.Fatal("cache not reused", err, calls)
	}
	if err = os.WriteFile(path, []byte("corrupted"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err = cachedAsset(a, expected, cache); err != nil || calls != 2 {
		t.Fatal("corruption not repaired", err, calls)
	}
	if _, err = cachedAsset(a, strings.Repeat("0", 64), cache); err == nil {
		t.Fatal("bad checksum accepted")
	}
	entries, _ := os.ReadDir(cache)
	if len(entries) != 1 {
		t.Fatal("partial download left behind")
	}
}

type rejectTransport struct{ calls int }

func (r *rejectTransport) RoundTrip(*http.Request) (*http.Response, error) {
	r.calls++
	return nil, errors.New("network forbidden")
}

func TestMenuReturnsAfterReadOnlyOperationsWithoutNetwork(t *testing.T) {
	r := &rejectTransport{}
	previous := http.DefaultTransport
	http.DefaultTransport = r
	t.Cleanup(func() { http.DefaultTransport = previous })
	var out bytes.Buffer
	// Status, configuration guidance, cancelled toolkit install, help, invalid choice, exit.
	// Never choose menu items that install software: tests must not change the machine.
	err := mainMenu(strings.NewReader("8\n6\ncursor\n3\nn\nh\ninvalid\n0\n"), &out)
	if err != nil || r.calls != 0 {
		t.Fatal(err, r.calls)
	}
	if strings.Count(out.String(), menuPrompt) != 6 {
		t.Fatal("menu failed to return", out.String())
	}
	if !strings.Contains(out.String(), "Models") {
		t.Fatal("configuration was not reached")
	}
}

func TestReplaceExecutable(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "new")
	dest := filepath.Join(dir, "installed")
	_ = os.WriteFile(src, []byte("new version"), 0o600)
	_ = os.WriteFile(dest, []byte("old version"), 0o600)
	if err := replaceExecutable(src, dest); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(dest)
	if string(b) != "new version" {
		t.Fatal("update failed")
	}
	if err := replaceExecutable(filepath.Join(dir, "missing"), dest); err == nil {
		t.Fatal("missing update accepted")
	}
	b, _ = os.ReadFile(dest)
	if string(b) != "new version" {
		t.Fatal("failed update destroyed installation")
	}
}

func TestOfficialInstallerFailureIsReported(t *testing.T) {
	script := "#!/bin/bash\necho simulated-installer\nexit 7\n"
	if runtime.GOOS == "windows" {
		script = "Write-Output 'simulated-installer'\nexit 7\n"
	}
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, script) }))
	defer s.Close()
	var out bytes.Buffer
	app, _ := appByID("codex")
	err := executeAppInstall(app, installPlan{URL: s.URL}, strings.NewReader(""), &out)
	if err == nil || !strings.Contains(out.String(), "simulated-installer") {
		t.Fatal("installer failure not propagated", err, out.String())
	}
}

func TestWindowsCMDLauncher(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows wrapper")
	}
	path := filepath.Join(t.TempDir(), "fake command.cmd")
	if err := os.WriteFile(path, []byte("@echo off\r\necho %1 %2\r\nexit /b 7\r\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	err := runChild([]string{path, "login", "--with-api-key"}, strings.NewReader("fake-key\n"), &out, os.Environ())
	if err == nil || !strings.Contains(out.String(), "login --with-api-key") {
		t.Fatal("CMD wrapper lost arguments or status", err, out.String())
	}
}

func TestReplaceRunningExecutable(t *testing.T) {
	if os.Getenv("SETUP_GATEWAY_UPDATE_TEST") == "child" {
		self, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		if err = replaceExecutable(os.Getenv("SETUP_GATEWAY_UPDATE_SOURCE"), self); err != nil {
			t.Fatal(err)
		}
		return
	}
	dir := t.TempDir()
	current, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	copyPath := filepath.Join(dir, "running-test")
	if runtime.GOOS == "windows" {
		copyPath += ".exe"
	}
	if err = replaceExecutable(current, copyPath); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(dir, "new-image")
	if err = os.WriteFile(source, []byte("replacement"), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(copyPath, "-test.run=^TestReplaceRunningExecutable$")
	cmd.Env = append(os.Environ(), "SETUP_GATEWAY_UPDATE_TEST=child", "SETUP_GATEWAY_UPDATE_SOURCE="+source)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("running executable update: %v %s", err, output)
	}
	b, err := os.ReadFile(copyPath)
	if err != nil || string(b) != "replacement" {
		t.Fatal("running executable not replaced", err)
	}
}
