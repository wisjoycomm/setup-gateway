package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func claudeLine(id, cwd, ts string, input, output, read, write int) string {
	return fmt.Sprintf(`{"type":"assistant","cwd":%q,"timestamp":%q,"requestId":"r-%s","message":{"id":%q,"model":"claude-x","usage":{"input_tokens":%d,"output_tokens":%d,"cache_read_input_tokens":%d,"cache_creation_input_tokens":%d}}}`+"\n",
		cwd, ts, id, id, input, output, read, write)
}

func codexRecord(turn, resp, ts string, input, cached, output int) string {
	return fmt.Sprintf(`{"timestamp":%q,"type":"token_usage_record","payload":{"turn_id":%q,"response_id":%q,"usage":{"input_tokens":%d,"cached_input_tokens":%d,"output_tokens":%d}}}`+"\n",
		ts, turn, resp, input, cached, output)
}

func codexCount(ts string, input, cached, output int) string {
	return fmt.Sprintf(`{"timestamp":%q,"type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":%d,"cached_input_tokens":%d,"output_tokens":%d}}}}`+"\n",
		ts, input, cached, output)
}

func codexContext(cwd, model string) string {
	return fmt.Sprintf(`{"timestamp":"2026-01-01T00:00:00Z","type":"turn_context","payload":{"model":%q,"cwd":%q}}`+"\n", model, cwd)
}

func testScanner(t *testing.T) (*sessionScanner, string) {
	t.Helper()
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	t.Setenv("CODEX_HOME", "")
	home := t.TempDir()
	return newSessionScanner(home), home
}

func TestClaudeRepeatedStreamLinesCountOnce(t *testing.T) {
	s, home := testScanner(t)
	path := filepath.Join(home, ".claude", "projects", "p", "a.jsonl")
	writeTestFile(t, path, claudeLine("m1", "/w/game", "2026-01-01T00:00:00Z", 10, 1, 100, 5)+
		claudeLine("m1", "/w/game", "2026-01-01T00:00:01Z", 10, 30, 100, 5)+
		`{"type":"user","message":{"usage":{"input_tokens":999}}}`+"\n")
	// A resumed session copies the old reply into a new file.
	writeTestFile(t, filepath.Join(home, ".claude", "projects", "p", "b.jsonl"),
		claudeLine("m1", "/w/game", "2026-01-01T00:00:01Z", 10, 30, 100, 5)+
			claudeLine("m2", "/w/game", "2026-01-01T00:00:02Z", 1, 1, 0, 0))
	s.scan(time.Time{})
	_, total := s.summarize(usageFilter{GroupBy: []string{"project"}})
	if total.Calls != 2 || total.Output != 31 || total.Input != 11 || total.CacheRead != 100 || total.Total != 147 {
		t.Errorf("total = %+v", total.usageTotal)
	}
}

func TestCodexRecordsSeparateCacheAndSurviveFork(t *testing.T) {
	s, home := testScanner(t)
	body := codexContext(`\\?\C:\w\app`, "gpt-x") +
		codexRecord("t1", "r1", "2026-01-01T00:00:01Z", 1000, 400, 50) + codexCount("2026-01-01T00:00:01Z", 1000, 400, 50) +
		codexRecord("t2", "r2", "2026-01-01T00:00:02Z", 1000, 400, 50) + codexCount("2026-01-01T00:00:02Z", 2000, 800, 100)
	writeTestFile(t, filepath.Join(home, ".codex", "sessions", "2026", "rollout-a.jsonl"), body)
	writeTestFile(t, filepath.Join(home, ".codex", "sessions", "2026", "rollout-fork.jsonl"), body)
	s.scan(time.Time{})
	list, total := s.summarize(usageFilter{GroupBy: []string{"project", "client"}})
	if total.Calls != 2 || total.Input != 1200 || total.CacheRead != 800 || total.Output != 100 || total.Total != 2100 {
		t.Errorf("total = %+v", total.usageTotal)
	}
	if len(list) != 1 || list[0].Project != "app" || list[0].Client != "codex" || list[0].LastModel != "gpt-x" {
		t.Errorf("list = %+v", list)
	}
}

func TestCodexOlderRolloutUsesRunningTotal(t *testing.T) {
	s, home := testScanner(t)
	writeTestFile(t, filepath.Join(home, ".codex", "archived_sessions", "rollout-old.jsonl"), codexContext("/w/old", "gpt-o")+
		codexCount("2026-01-01T00:00:01Z", 100, 40, 10)+
		codexCount("2026-01-01T00:00:02Z", 100, 40, 10)+ // a rate-limit refresh repeats the total
		codexCount("2026-01-01T00:00:03Z", 300, 90, 30))
	s.scan(time.Time{})
	_, total := s.summarize(usageFilter{GroupBy: []string{"model"}})
	if total.Calls != 2 || total.Input != 210 || total.CacheRead != 90 || total.Output != 30 {
		t.Errorf("total = %+v", total.usageTotal)
	}
}

func TestScanReadsOnlyAppendedAndWaitsForFullLine(t *testing.T) {
	s, home := testScanner(t)
	path := filepath.Join(home, ".claude", "projects", "p", "a.jsonl")
	first := claudeLine("m1", "/w/game", "2026-01-01T00:00:00Z", 1, 1, 0, 0)
	second := claudeLine("m2", "/w/game", "2026-01-01T00:00:01Z", 2, 2, 0, 0)
	writeTestFile(t, path, first+second[:20])
	s.scan(time.Time{})
	if _, total := s.summarize(usageFilter{}); total.Calls != 1 {
		t.Fatalf("calls = %d", total.Calls)
	}
	if err := os.WriteFile(path, []byte(first+second), 0o644); err != nil {
		t.Fatal(err)
	}
	s.scan(time.Time{})
	if _, total := s.summarize(usageFilter{}); total.Calls != 2 || total.Input != 3 {
		t.Errorf("total = %+v", total.usageTotal)
	}
}

func TestSummarizeFiltersWindowProjectAndApp(t *testing.T) {
	s, home := testScanner(t)
	writeTestFile(t, filepath.Join(home, ".claude", "projects", "p", "a.jsonl"),
		claudeLine("old", "/w/game", "2026-01-01T00:00:00Z", 5, 0, 0, 0)+
			claudeLine("new", "/w/game", "2026-02-01T00:00:00Z", 7, 0, 0, 0)+
			claudeLine("other", "/w/site", "2026-02-01T00:00:00Z", 9, 0, 0, 0))
	s.scan(time.Time{})
	since := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
	if _, total := s.summarize(usageFilter{Since: since, Project: "GAME"}); total.Input != 7 {
		t.Errorf("project filter: %+v", total.usageTotal)
	}
	if _, total := s.summarize(usageFilter{Client: "codex"}); total.Calls != 0 {
		t.Errorf("app filter: %+v", total.usageTotal)
	}
}

func TestProjectNameJoinsWorktreesToMainRepo(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "my-game")
	sub := filepath.Join(repo, "src", "deep")
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if got := projectNameOf(sub); got != "my-game" {
		t.Errorf("subfolder = %q", got)
	}
	worktree := filepath.Join(root, "orca", "T-01")
	writeTestFile(t, filepath.Join(worktree, ".git"), "gitdir: "+filepath.ToSlash(repo)+"/.git/worktrees/T-01\n")
	if got := projectNameOf(worktree); got != "my-game" {
		t.Errorf("worktree = %q", got)
	}
	if got := projectNameOf(filepath.Join(root, "gone", "app")); got != "app" {
		t.Errorf("deleted = %q", got)
	}
	// Tests run inside this repo, so a walk from a relative path would find its .git.
	for _, foreign := range []string{`C:\w\app`, `\\?\C:\w\app`, "w/app", "app"} {
		if got := projectNameOf(foreign); got != "app" {
			t.Errorf("%q = %q", foreign, got)
		}
	}
	if got := projectNameOf(""); got != "(không rõ)" {
		t.Errorf("empty = %q", got)
	}
}

func TestRenderTopMarksActiveProjectsFirst(t *testing.T) {
	s, home := testScanner(t)
	now := time.Now().UTC()
	writeTestFile(t, filepath.Join(home, ".claude", "projects", "p", "a.jsonl"),
		claudeLine("a", "/w/big-old", now.Add(-3*time.Hour).Format(time.RFC3339), 5_000_000, 0, 0, 0)+
			claudeLine("b", "/w/small-new", now.Add(-10*time.Second).Format(time.RFC3339), 900, 0, 0, 0))
	s.scan(time.Time{})
	list, total := s.projectStatus(time.Time{})
	out := renderTop(list, total, "trong 24h", now, 0, 0)
	if strings.Index(out, "small-new") > strings.Index(out, "big-old") {
		t.Errorf("order:\n%s", out)
	}
	if !strings.Contains(out, "● small-new") || strings.Contains(out, "● big-old") || !strings.Contains(out, "5.0M") {
		t.Errorf("render:\n%s", out)
	}
}

func TestRunningProjectsDropsIdleOnes(t *testing.T) {
	now := time.Now()
	row := func(name string, age time.Duration) usageRow {
		return usageRow{usageTotal: usageTotal{Project: name}, Last: now.Add(-age)}
	}
	list := []usageRow{row("new", time.Second), row("recent", 9*time.Minute), row("idle", 11*time.Minute), row("open-idle", time.Hour)}
	got := runningProjects(list, []string{"open-idle", "open-unused"}, now)
	var names []string
	for _, r := range got {
		names = append(names, r.Project)
	}
	if strings.Join(names, ",") != "new,recent,open-idle,open-unused" {
		t.Errorf("running = %v", names)
	}
	if !got[3].Last.IsZero() || got[3].Total != 0 {
		t.Errorf("unused row = %+v", got[3])
	}
	many := make([]usageRow, overlayMaxRows+5)
	for i := range many {
		many[i] = row(fmt.Sprint(i), time.Second)
	}
	if got := runningProjects(many, nil, now); len(got) != overlayMaxRows {
		t.Errorf("cap = %d", len(got))
	}
}

func TestOpenSessionMarkersNeedALiveProcess(t *testing.T) {
	s, home := testScanner(t)
	dir := filepath.Join(home, ".claude", "sessions")
	writeTestFile(t, filepath.Join(dir, "1.json"), fmt.Sprintf(`{"pid":%d,"cwd":"/w/open-one"}`, os.Getpid()))
	writeTestFile(t, filepath.Join(dir, "2.json"), `{"pid":2147483000,"cwd":"/w/closed"}`)
	writeTestFile(t, filepath.Join(dir, "3.json"), `{ broken`)
	if got := s.openClaudeProjects(); len(got) != 1 || got[0] != "open-one" {
		t.Errorf("open = %v", got)
	}
}

func TestMachineTotalsAddEveryProject(t *testing.T) {
	s, home := testScanner(t)
	now := time.Now().UTC()
	writeTestFile(t, filepath.Join(home, ".claude", "projects", "p", "a.jsonl"),
		claudeLine("a", "/w/one", now.Add(-time.Hour).Format(time.RFC3339), 1000, 0, 0, 0)+
			claudeLine("b", "/w/two", now.Add(-72*time.Hour).Format(time.RFC3339), 2000, 0, 0, 0)+
			claudeLine("c", "/w/three", now.Add(-20*24*time.Hour).Format(time.RFC3339), 4000, 0, 0, 0)+
			claudeLine("d", "/w/four", now.Add(-60*24*time.Hour).Format(time.RFC3339), 8000, 0, 0, 0))
	s.scan(time.Time{})
	want := "24 giờ 1.0k · 7 ngày 3.0k · 30 ngày 7.0k · toàn bộ 15.0k"
	if got := s.machineTotals(now); !strings.Contains(got, want) {
		t.Errorf("got %q", got)
	}
}

func TestTopOnceWithoutSessions(t *testing.T) {
	t.Setenv("CLAUDE_CONFIG_DIR", t.TempDir())
	t.Setenv("CODEX_HOME", t.TempDir())
	var out bytes.Buffer
	if err := runTop([]string{"-once"}, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Chưa có lượt gọi") {
		t.Errorf("out = %s", out.String())
	}
}
