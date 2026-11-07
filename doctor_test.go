package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeTranscript(t *testing.T, home, name, cwd string, fallback bool) {
	t.Helper()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	line := func(v map[string]any) string { b, _ := json.Marshal(v); return string(b) + "\n" }
	text := line(map[string]any{"type": "user", "cwd": cwd, "timestamp": now, "message": map[string]any{"role": "user", "content": "hi"}})
	if fallback {
		text += `{"type":"api-request-blob","message":{"content":[{"type":"text","text":"ToolSearch is no longer available. Tools listed earlier"}]}}` + "\n"
	}
	text += line(map[string]any{"type": "assistant", "cwd": cwd, "timestamp": now,
		"message": map[string]any{"id": "m1", "usage": map[string]any{"cache_read_input_tokens": 300000}}})
	text += line(map[string]any{"type": "assistant", "cwd": cwd, "timestamp": now,
		"message": map[string]any{"id": "m1", "usage": map[string]any{"cache_read_input_tokens": 300000}}})
	writeTestFile(t, filepath.Join(home, ".claude", "projects", "p", name+".jsonl"), text)
}

func TestDoctorFindsToolSearchFallbackAndProxiesProject(t *testing.T) {
	home := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	project := filepath.Join(t.TempDir(), "game")
	_ = os.MkdirAll(filepath.Join(project, ".git"), 0o700)
	worktree := filepath.Join(t.TempDir(), "wt")
	writeTestFile(t, filepath.Join(worktree, ".git"), "gitdir: "+filepath.ToSlash(project)+"/.git/worktrees/wt\n")
	writeTranscript(t, home, "a", worktree, true)
	writeTranscript(t, home, "b", project, false)

	hits := scanToolSearchFallback(home, time.Now().Add(-time.Hour))
	h := hits[project]
	if len(hits) != 1 || h == nil || h.Sessions != 1 || h.After != 300000 {
		t.Fatalf("worktree session must fold into its repo and count each reply once: %+v", hits)
	}
	findings := checkToolSearch(doctorEnv{home: home, since: time.Hour})
	if len(findings) != 1 || findings[0].fix == nil || !strings.Contains(findings[0].Fix, "proxy") {
		t.Fatalf("want one fixable finding, got %+v", findings)
	}
}

func TestDoctorSkipsHealthySessions(t *testing.T) {
	home := t.TempDir()
	project := filepath.Join(t.TempDir(), "ok")
	_ = os.MkdirAll(filepath.Join(project, ".git"), 0o700)
	writeTranscript(t, home, "a", project, false)
	if f := checkToolSearch(doctorEnv{home: home, since: time.Hour}); len(f) != 0 {
		t.Fatalf("no fallback, no finding: %+v", f)
	}
}

func TestDoctorFlagsOldAddonAndOldPlan(t *testing.T) {
	home := t.TempDir()
	writeTestFile(t, filepath.Join(proxyDir(home), "usage_meter.py"), "old addon")
	project := t.TempDir()
	_ = os.MkdirAll(filepath.Join(project, ".git"), 0o700)
	writeTestFile(t, filepath.Join(project, "PLAN.md"), oldPlan)
	findings := append(checkProxyHealth(doctorEnv{home: home}), checkPlanFormat(doctorEnv{home: home, cwd: project})...)
	keys := ""
	for _, f := range findings {
		keys += f.Key + " "
	}
	if !strings.Contains(keys, "proxy-addon") || !strings.Contains(keys, "plan:") {
		t.Fatalf("missing findings: %s", keys)
	}
}

func TestDoctorDryRunChangesNothing(t *testing.T) {
	home := t.TempDir()
	t.Setenv("USERPROFILE", home)
	t.Setenv("HOME", home)
	writeTestFile(t, filepath.Join(proxyDir(home), "usage_meter.py"), "old addon")
	var out bytes.Buffer
	if err := runDoctor([]string{"-dry-run"}, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if readText(t, filepath.Join(proxyDir(home), "usage_meter.py")) != "old addon" || !strings.Contains(out.String(), "addon cũ") {
		t.Fatalf("dry run must only report:\n%s", out.String())
	}
}
