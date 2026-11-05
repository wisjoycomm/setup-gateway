package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"golang.org/x/term"
)

// Claude Code and Codex record token usage in their own session files, so these
// commands need no proxy, no network and also see sessions started by Orca.

const (
	activeWindow   = 2 * time.Minute
	runningWindow  = 10 * time.Minute
	overlayMaxRows = 20
)

type sessionCall struct {
	When                                 time.Time
	Project, App, Model                  string
	Input, Output, CacheRead, CacheWrite int64
}

type sessionFile struct {
	offset     int64
	cwd, model string
	hasRecords bool
	lastTotal  codexTokens
}

type sessionScanner struct {
	claudeDir, codexDir string
	files               map[string]*sessionFile
	index               map[string]int
	calls               []sessionCall
	projects            map[string]string
}

func newSessionScanner(home string) *sessionScanner {
	claudeDir := os.Getenv("CLAUDE_CONFIG_DIR")
	if claudeDir == "" {
		claudeDir = filepath.Join(home, ".claude")
	}
	return &sessionScanner{
		claudeDir: claudeDir,
		codexDir:  filepath.Dir(codexConfigPath(home)),
		files:     map[string]*sessionFile{},
		index:     map[string]int{},
		projects:  map[string]string{},
	}
}

// scan reads only what was appended since the last scan. Files untouched since
// `since` cannot hold calls in that window and are skipped.
func (s *sessionScanner) scan(since time.Time) {
	s.scanTree(filepath.Join(s.claudeDir, "projects"), since, s.readClaudeLine)
	for _, dir := range []string{"sessions", "archived_sessions"} {
		s.scanTree(filepath.Join(s.codexDir, dir), since, s.readCodexLine)
	}
}

func (s *sessionScanner) scanTree(root string, since time.Time, parse func(string, *sessionFile, []byte)) {
	_ = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() || !strings.EqualFold(filepath.Ext(path), ".jsonl") {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return nil
		}
		state := s.files[path]
		if state == nil && info.ModTime().Before(since) {
			return nil
		}
		if state == nil || info.Size() < state.offset {
			state = &sessionFile{}
			s.files[path] = state
		}
		if info.Size() > state.offset {
			s.readAppended(path, state, parse)
		}
		return nil
	})
}

func (s *sessionScanner) readAppended(path string, state *sessionFile, parse func(string, *sessionFile, []byte)) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	if _, err = f.Seek(state.offset, io.SeekStart); err != nil {
		return
	}
	// Lines can be many megabytes (pasted images), so no bufio.Scanner.
	reader := bufio.NewReaderSize(f, 256<<10)
	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			return // a line still being written is read next time
		}
		state.offset += int64(len(line))
		parse(path, state, line)
	}
}

func (s *sessionScanner) record(key string, call sessionCall) {
	if i, ok := s.index[key]; ok {
		s.calls[i] = call
		return
	}
	s.index[key] = len(s.calls)
	s.calls = append(s.calls, call)
}

func (s *sessionScanner) readClaudeLine(path string, state *sessionFile, raw []byte) {
	if !bytes.Contains(raw, []byte(`"usage"`)) {
		return
	}
	var line struct {
		Type      string `json:"type"`
		Cwd       string `json:"cwd"`
		RequestID string `json:"requestId"`
		UUID      string `json:"uuid"`
		Timestamp string `json:"timestamp"`
		Message   struct {
			ID    string `json:"id"`
			Model string `json:"model"`
			Usage struct {
				Input      int64 `json:"input_tokens"`
				Output     int64 `json:"output_tokens"`
				CacheRead  int64 `json:"cache_read_input_tokens"`
				CacheWrite int64 `json:"cache_creation_input_tokens"`
			} `json:"usage"`
		} `json:"message"`
	}
	if json.Unmarshal(raw, &line) != nil || line.Type != "assistant" {
		return
	}
	when, err := time.Parse(time.RFC3339Nano, line.Timestamp)
	usage := line.Message.Usage
	if err != nil || usage.Input+usage.Output+usage.CacheRead+usage.CacheWrite == 0 {
		return
	}
	// A streamed reply repeats across lines and a resumed session can copy old
	// lines into a new file; the message id counts each reply once.
	key := line.Message.ID
	if key == "" {
		key = line.RequestID
	}
	if key == "" {
		key = path + "\x00" + line.UUID
	}
	s.record("claude\x00"+key, sessionCall{
		When: when, Project: s.projectOf(line.Cwd), App: "claude", Model: line.Message.Model,
		Input: usage.Input, Output: usage.Output, CacheRead: usage.CacheRead, CacheWrite: usage.CacheWrite,
	})
}

type codexTokens struct {
	Input      int64 `json:"input_tokens"`
	Cached     int64 `json:"cached_input_tokens"`
	CacheWrite int64 `json:"cache_write_input_tokens"`
	Output     int64 `json:"output_tokens"`
}

func (s *sessionScanner) readCodexLine(path string, state *sessionFile, raw []byte) {
	if !bytes.Contains(raw, []byte(`"session_meta"`)) && !bytes.Contains(raw, []byte(`"turn_context"`)) &&
		!bytes.Contains(raw, []byte(`"token_usage_record"`)) && !bytes.Contains(raw, []byte(`"token_count"`)) {
		return
	}
	var line struct {
		Timestamp string `json:"timestamp"`
		Type      string `json:"type"`
		Payload   struct {
			Type       string      `json:"type"`
			Cwd        string      `json:"cwd"`
			Model      string      `json:"model"`
			TurnID     string      `json:"turn_id"`
			ResponseID string      `json:"response_id"`
			Usage      codexTokens `json:"usage"`
			Info       struct {
				Total codexTokens `json:"total_token_usage"`
			} `json:"info"`
		} `json:"payload"`
	}
	if json.Unmarshal(raw, &line) != nil {
		return
	}
	payload := line.Payload
	if payload.Cwd != "" {
		state.cwd = payload.Cwd
	}
	if line.Type == "turn_context" && payload.Model != "" {
		state.model = payload.Model
	}
	when, err := time.Parse(time.RFC3339Nano, line.Timestamp)
	if err != nil {
		return
	}
	switch {
	case line.Type == "token_usage_record":
		// One record per response. A forked session copies old records, and the
		// turn + response ids count each of them once.
		state.hasRecords = true
		s.record("codex\x00"+payload.TurnID+"\x00"+payload.ResponseID, s.codexCall(state, when, payload.Usage))
	case line.Type == "event_msg" && payload.Type == "token_count" && !state.hasRecords:
		// Older Codex builds only write a running total.
		total := payload.Info.Total
		diff := codexTokens{total.Input - state.lastTotal.Input, total.Cached - state.lastTotal.Cached,
			total.CacheWrite - state.lastTotal.CacheWrite, total.Output - state.lastTotal.Output}
		if diff.Input > 0 || diff.Output > 0 {
			state.lastTotal = total
			s.record("codex\x00"+path+"\x00"+line.Timestamp, s.codexCall(state, when, diff))
		}
	}
}

// codexCall removes cached tokens from Codex's input so the columns mean the
// same thing as Claude's, where cache tokens are never part of input.
func (s *sessionScanner) codexCall(state *sessionFile, when time.Time, t codexTokens) sessionCall {
	input := t.Input - t.Cached - t.CacheWrite
	if input < 0 {
		input = 0
	}
	return sessionCall{When: when, Project: s.projectOf(state.cwd), App: "codex", Model: state.model,
		Input: input, Output: t.Output, CacheRead: t.Cached, CacheWrite: t.CacheWrite}
}

func (s *sessionScanner) projectOf(cwd string) string {
	if name, ok := s.projects[cwd]; ok {
		return name
	}
	name := projectNameOf(cwd)
	s.projects[cwd] = name
	return name
}

// projectNameOf is the main repository's folder name, so every Orca worktree of
// a repo counts as one project. Deleted folders fall back to the last path part.
func projectNameOf(cwd string) string {
	cwd = strings.TrimPrefix(cwd, `\\?\`)
	if cwd == "" {
		return "(không rõ)"
	}
	dir := filepath.Clean(cwd)
	for {
		data, err := os.ReadFile(filepath.Join(dir, ".git"))
		if err == nil {
			if main := worktreeMain(string(data)); main != "" {
				return filepath.Base(main)
			}
			return filepath.Base(dir)
		}
		if isDir(filepath.Join(dir, ".git")) {
			return filepath.Base(dir)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return filepath.Base(filepath.Clean(cwd))
		}
		dir = parent
	}
}

// worktreeMain reads a worktree's ".git" file: "gitdir: <repo>/.git/worktrees/<name>".
func worktreeMain(gitFile string) string {
	line := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(gitFile), "gitdir:"))
	slashed := filepath.ToSlash(line)
	i := strings.LastIndex(slashed, "/worktrees/")
	if i < 0 {
		return ""
	}
	repo := strings.TrimSuffix(strings.TrimSuffix(slashed[:i], "/.git"), ".git")
	return filepath.FromSlash(repo)
}

type usageRow struct {
	usageTotal
	Apps      []string
	LastModel string
	Last      time.Time
}

func (s *sessionScanner) summarize(f usageFilter) ([]usageRow, usageRow) {
	groups := map[string]*usageRow{}
	var all usageRow
	for _, call := range s.calls {
		if call.When.Before(f.Since) ||
			f.Project != "" && !strings.EqualFold(call.Project, f.Project) ||
			f.Client != "" && !strings.EqualFold(call.App, f.Client) {
			continue
		}
		var row usageRow
		for _, dim := range f.GroupBy {
			switch dim {
			case "project":
				row.Project = call.Project
			case "client":
				row.Client = call.App
			case "model":
				row.Model = call.Model
			}
		}
		key := row.Project + "\x00" + row.Client + "\x00" + row.Model
		g := groups[key]
		if g == nil {
			g = &row
			groups[key] = g
		}
		for _, t := range []*usageRow{g, &all} {
			t.Calls++
			t.Input += call.Input
			t.Output += call.Output
			t.CacheRead += call.CacheRead
			t.CacheWrite += call.CacheWrite
			t.Total += call.Input + call.Output + call.CacheRead + call.CacheWrite
			t.Apps = appendUnique(t.Apps, call.App)
			if !call.When.Before(t.Last) {
				t.Last, t.LastModel = call.When, call.Model
			}
		}
	}
	list := make([]usageRow, 0, len(groups))
	for _, g := range groups {
		sort.Strings(g.Apps)
		list = append(list, *g)
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].Total != list[j].Total {
			return list[i].Total > list[j].Total
		}
		return list[i].Project+list[i].Client+list[i].Model < list[j].Project+list[j].Client+list[j].Model
	})
	return list, all
}

func runUsage(args []string, out io.Writer) error {
	flags := flag.NewFlagSet("usage", flag.ContinueOnError)
	flags.SetOutput(out)
	window := flags.Duration("since", 24*time.Hour, "khoảng thời gian, ví dụ 1h, 24h, 168h (7 ngày)")
	all := flags.Bool("all", false, "toàn bộ phiên còn lưu trên máy")
	by := flags.String("by", "project-app", "gom theo: project, project-app, app (app + model), model")
	project := flags.String("project", "", "chỉ tính một project")
	client := flags.String("app", "", "chỉ tính một app: claude, codex")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("cách dùng: wj usage [-since 24h | -all] [-by project|project-app|app|model] [-project TÊN] [-app claude|codex]")
	}
	groupBy, ok := map[string][]string{
		"app":         {"client", "model"},
		"project":     {"project"},
		"project-app": {"project", "client"},
		"model":       {"model"},
	}[*by]
	if !ok {
		return errors.New("-by chỉ nhận: project, project-app, app, model")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	since := time.Now().Add(-*window)
	label := "trong " + shortWindow(*window)
	if *all {
		since, label = time.Time{}, "toàn bộ"
	}
	scanner := newSessionScanner(home)
	scanner.scan(since)
	list, total := scanner.summarize(usageFilter{Since: since, Project: *project, Client: *client, GroupBy: groupBy})
	fmt.Fprintf(out, "Token đã dùng (%s; Claude Code + Codex trên máy này)\n\n", label)
	if total.Calls == 0 {
		fmt.Fprintln(out, "Không có lượt gọi nào trong khoảng này.")
		return nil
	}
	headers := map[string]string{"project": "PROJECT", "client": "APP", "model": "MODEL"}
	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', tabwriter.AlignRight)
	var head []string
	for _, dim := range groupBy {
		head = append(head, headers[dim])
	}
	fmt.Fprintln(w, strings.Join(head, "\t")+"\tLƯỢT\tVÀO\tRA\tĐỌC CACHE\tGHI CACHE\tTỔNG\t")
	row := func(t usageRow, first string) {
		var cells []string
		for i, dim := range groupBy {
			value := map[string]string{"project": t.Project, "client": t.Client, "model": t.Model}[dim]
			if first != "" {
				value = ""
				if i == 0 {
					value = first
				}
			}
			cells = append(cells, value)
		}
		fmt.Fprintf(w, "%s\t%d\t%s\t%s\t%s\t%s\t%s\t\n", strings.Join(cells, "\t"), t.Calls,
			groupDigits(t.Input), groupDigits(t.Output), groupDigits(t.CacheRead), groupDigits(t.CacheWrite), groupDigits(t.Total))
	}
	for _, t := range list {
		row(t, "")
	}
	row(total, "TỔNG")
	if err := w.Flush(); err != nil {
		return err
	}
	fmt.Fprintln(out, "\nVÀO = token prompt mới; ĐỌC CACHE thường rẻ hơn nhiều so với VÀO. App khác (OpenCode...) đo bằng: wj proxy usage")
	return nil
}

func compactTokens(n int64) string {
	switch {
	case n >= 1_000_000_000:
		return fmt.Sprintf("%.1fG", float64(n)/1e9)
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1e6)
	case n >= 1_000:
		return fmt.Sprintf("%.1fk", float64(n)/1e3)
	}
	return fmt.Sprint(n)
}

func agoText(d time.Duration) string {
	switch {
	case d < time.Minute:
		return "vừa xong"
	case d < time.Hour:
		return fmt.Sprintf("%d phút", int(d.Minutes()))
	case d < 48*time.Hour:
		return fmt.Sprintf("%d giờ", int(d.Hours()))
	}
	return fmt.Sprintf("%d ngày", int(d.Hours()/24))
}

func cutRunes(text string, limit int) string {
	runes := []rune(text)
	if limit <= 0 || len(runes) <= limit {
		return text
	}
	if limit == 1 {
		return "…"
	}
	return string(runes[:limit-1]) + "…"
}

// projectStatus lists projects with the most recent activity first.
func (s *sessionScanner) projectStatus(since time.Time) ([]usageRow, usageRow) {
	list, total := s.summarize(usageFilter{Since: since, GroupBy: []string{"project"}})
	sort.SliceStable(list, func(i, j int) bool { return list[i].Last.After(list[j].Last) })
	return list, total
}

// machineTotals adds up every project on this machine over fixed windows.
func (s *sessionScanner) machineTotals(now time.Time) string {
	_, day := s.summarize(usageFilter{Since: now.Add(-24 * time.Hour)})
	_, week := s.summarize(usageFilter{Since: now.Add(-7 * 24 * time.Hour)})
	_, month := s.summarize(usageFilter{Since: now.Add(-30 * 24 * time.Hour)})
	_, all := s.summarize(usageFilter{})
	return fmt.Sprintf("Cả máy: 24 giờ %s · 7 ngày %s · 30 ngày %s · toàn bộ %s",
		compactTokens(day.Total), compactTokens(week.Total), compactTokens(month.Total), compactTokens(all.Total))
}

// overlayFooter is the machine total for the longer windows.
func (s *sessionScanner) overlayFooter(now time.Time) string {
	_, week := s.summarize(usageFilter{Since: now.Add(-7 * 24 * time.Hour)})
	_, month := s.summarize(usageFilter{Since: now.Add(-30 * 24 * time.Hour)})
	return fmt.Sprintf("7 ngày %s · 30 ngày %s", compactTokens(week.Total), compactTokens(month.Total))
}

// openClaudeProjects reads Claude Code's per-process session markers, so an idle
// session still counts as open. Codex has no such marker and is seen by activity.
func (s *sessionScanner) openClaudeProjects() []string {
	paths, _ := filepath.Glob(filepath.Join(s.claudeDir, "sessions", "*.json"))
	var open []string
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var marker struct {
			PID int    `json:"pid"`
			Cwd string `json:"cwd"`
		}
		if json.Unmarshal(data, &marker) == nil && marker.PID > 0 && marker.Cwd != "" && processAlive(marker.PID) {
			open = appendUnique(open, s.projectOf(marker.Cwd))
		}
	}
	return open
}

// runningProjects keeps projects that are open or used tokens within runningWindow.
// The row limit keeps the overlay on screen.
func runningProjects(list []usageRow, open []string, now time.Time) []usageRow {
	rows := []usageRow{}
	seen := []string{}
	for _, row := range list {
		if now.Sub(row.Last) < runningWindow || containsName(open, row.Project) {
			rows = append(rows, row)
			seen = append(seen, row.Project)
		}
	}
	for _, name := range open {
		if !containsName(seen, name) {
			rows = append(rows, usageRow{usageTotal: usageTotal{Project: name}, Apps: []string{"claude"}})
		}
	}
	if len(rows) > overlayMaxRows {
		rows = rows[:overlayMaxRows]
	}
	return rows
}

func shortWindow(d time.Duration) string {
	if d >= 48*time.Hour && d%(24*time.Hour) == 0 {
		return fmt.Sprintf("%d ngày", int(d.Hours()/24))
	}
	if d >= time.Hour && d%time.Hour == 0 {
		return fmt.Sprintf("%d giờ", int(d.Hours()))
	}
	return d.String()
}

func renderTop(list []usageRow, total usageRow, label string, now time.Time, width, height int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "wj top - token theo project, %s (%s)\n\n", label, now.Format("15:04:05"))
	if total.Calls == 0 {
		b.WriteString("Chưa có lượt gọi nào trong khoảng này.\n")
		return b.String()
	}
	showModel := width == 0 || width >= 100
	w := tabwriter.NewWriter(&b, 0, 0, 2, ' ', 0)
	model := func(text string) string {
		if showModel {
			return text + "\t"
		}
		return ""
	}
	fmt.Fprintf(w, "  PROJECT\tAPP\t%sLẦN CUỐI\t%6s\t%7s\t%7s\t%7s\t%7s\n", model("MODEL"), "LƯỢT", "VÀO", "RA", "CACHE", "TỔNG")
	shown := list
	if height > 8 && len(shown) > height-8 {
		shown = shown[:height-8]
	}
	for _, t := range shown {
		mark := " "
		if now.Sub(t.Last) < activeWindow {
			mark = "●"
		}
		fmt.Fprintf(w, "%s %s\t%s\t%s%s\t%6d\t%7s\t%7s\t%7s\t%7s\n", mark, cutRunes(t.Project, 24), strings.Join(t.Apps, ","),
			model(cutRunes(t.LastModel, 26)), agoText(now.Sub(t.Last)), t.Calls, compactTokens(t.Input), compactTokens(t.Output),
			compactTokens(t.CacheRead+t.CacheWrite), compactTokens(t.Total))
	}
	fmt.Fprintf(w, "  TỔNG\t\t%s\t%6d\t%7s\t%7s\t%7s\t%7s\n", model(""), total.Calls, compactTokens(total.Input), compactTokens(total.Output),
		compactTokens(total.CacheRead+total.CacheWrite), compactTokens(total.Total))
	_ = w.Flush()
	if len(shown) < len(list) {
		fmt.Fprintf(&b, "  ...và %d project khác\n", len(list)-len(shown))
	}
	return b.String()
}

func runTop(args []string, out io.Writer) error {
	flags := flag.NewFlagSet("top", flag.ContinueOnError)
	flags.SetOutput(out)
	window := flags.Duration("since", 24*time.Hour, "khoảng thời gian, ví dụ 1h, 24h, 168h (7 ngày)")
	interval := flags.Duration("interval", 3*time.Second, "bao lâu cập nhật một lần")
	once := flags.Bool("once", false, "in một lần rồi thoát")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if flags.NArg() != 0 || *interval < time.Second {
		return errors.New("cách dùng: wj top [-since 24h] [-interval 3s] [-once]")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	scanner := newSessionScanner(home)
	label := "trong " + shortWindow(*window)
	frame := func(width, height int) string {
		now := time.Now()
		scanner.scan(time.Time{})
		list, total := scanner.projectStatus(now.Add(-*window))
		return renderTop(list, total, label, now, width, height) + "\n" + scanner.machineTotals(now) + "\n"
	}
	stdout, isFile := out.(*os.File)
	if *once || !isFile || !term.IsTerminal(int(stdout.Fd())) {
		_, err = io.WriteString(out, frame(0, 0))
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	defer enableTerminalEscapes()()
	fmt.Fprint(out, "\x1b[?1049h\x1b[?25l")
	defer fmt.Fprint(out, "\x1b[?25h\x1b[?1049l")
	ticker := time.NewTicker(*interval)
	defer ticker.Stop()
	for {
		width, height, err := term.GetSize(int(stdout.Fd()))
		if err != nil {
			width, height = 0, 0
		}
		lines := strings.Split(frame(width, height)+"\nCtrl+C để thoát. ● = vừa chạy trong 2 phút.\n", "\n")
		for i, line := range lines {
			if width > 0 {
				lines[i] = cutRunes(line, width-1)
			}
		}
		fmt.Fprint(out, "\x1b[H"+strings.Join(lines, "\x1b[K\n")+"\x1b[J")
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}
