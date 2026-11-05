package main

import (
	"bufio"
	_ "embed"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"golang.org/x/term"
)

//go:embed proxy_addon.py
var proxyAddon []byte

const defaultProxyPort = 8877

const proxyHelp = `Đo lưu lượng / token qua mitmproxy (chạy tại máy, không gửi đi đâu)

  mitmproxy đứng giữa Claude/Codex và gateway (ví dụ ai.adnx.vn), ghi lại
  mỗi lượt gọi model: thời gian, ứng dụng, model, số token vào/ra/cache.
  KHÔNG ghi prompt, câu trả lời hay API key.

  CÁCH 1 - MỘT LỆNH (dễ nhất)
    wj proxy run claude     tự cài mitmproxy nếu thiếu, tự bật proxy,
    wj proxy run codex      mở app; thoát app là proxy tự tắt.

  CÁCH 2 - TỪNG BƯỚC (proxy chạy lâu, nhiều app dùng chung)
    wj proxy setup          cài mitmproxy + tạo chứng chỉ (một lần)
    wj proxy start          bật proxy, để cửa sổ này chạy
    wj proxy run claude     ở cửa sổ khác; tự dùng proxy đang chạy

  XEM ĐÃ DÙNG BAO NHIÊU (một log chung cho cả máy, mọi app: Claude, Codex, OpenCode...)
    wj proxy usage                  cả máy, 24 giờ qua, theo app + model
    wj proxy usage -by project      theo từng project
    wj proxy usage -by project-app  theo project + app
    wj proxy usage -project remora  chỉ một project
    wj proxy usage -app codex       chỉ một app
    -since 168h = 7 ngày;  -all = toàn bộ

  PROJECT: proxy run tự lấy tên thư mục git đang đứng (vd remora).
    Đặt tay: wj proxy run -project ten-du-an claude

  CÁCH 3 - BẬT CHO CẢ PROJECT (Claude Code; không cần gõ proxy run nữa)
    cd vào project, rồi:  wj proxy project on
    Sau đó cứ mở claude / claude --continue bình thường, mọi phiên đều được đếm
    (cần proxy start đang chạy). Tắt: proxy project off   Xem: proxy project list

  Lệnh khác:
    proxy status               kiểm tra mitmproxy, chứng chỉ, cổng, file log
    -yes                       (setup/start/run) tự cài mitmproxy, không hỏi
    proxy start -web           giao diện web xem từng request (http://127.0.0.1:8081)
    proxy start -port 9000     đổi cổng (mặc định 8877; nhớ dùng cùng -port khi run)
    proxy run -proxy http://IP:PORT -ca FILE.pem codex
                               dùng mitmproxy riêng của bạn (máy khác / cổng khác)

  Chỉ app mở bằng "proxy run" mới đi qua proxy. Cấu hình Claude/Codex
  không bị sửa; tắt proxy là mọi thứ trở lại như cũ.`

func proxyDir(home string) string { return filepath.Join(stateDir(home), "proxy") }

func proxyLogPath(home string) string { return filepath.Join(proxyDir(home), "usage.jsonl") }

func mitmCAPath(home string) string {
	return filepath.Join(home, ".mitmproxy", "mitmproxy-ca-cert.pem")
}

func runProxy(args []string, input io.Reader, out io.Writer) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	if len(args) == 0 || contains([]string{"help", "-h", "-help", "--help"}, args[0]) {
		fmt.Fprintln(out, proxyHelp)
		return nil
	}
	switch args[0] {
	case "start":
		return proxyStart(args[1:], input, out, home)
	case "run":
		return proxyRun(args[1:], input, out, home)
	case "setup":
		return proxySetup(args[1:], input, out, home)
	case "usage":
		return proxyUsage(args[1:], out, home)
	case "project":
		return proxyProject(args[1:], input, out, home)
	case "status":
		return proxyStatus(out, home)
	}
	return fmt.Errorf("không hiểu lệnh proxy %q; xem: wj proxy help", args[0])
}

func findMitm(home, tool string) string {
	app, _ := appByID("mitmproxy")
	dump := findApp(app, home)
	if dump == "" || tool == "mitmdump" {
		return dump
	}
	ext := filepath.Ext(dump)
	sibling := filepath.Join(filepath.Dir(dump), tool+ext)
	if info, err := os.Stat(sibling); err == nil && !info.IsDir() {
		return sibling
	}
	if path, err := exec.LookPath(tool); err == nil {
		return path
	}
	return ""
}

func proxyStart(args []string, input io.Reader, out io.Writer, home string) error {
	flags := flag.NewFlagSet("proxy start", flag.ContinueOnError)
	flags.SetOutput(out)
	port := flags.Int("port", defaultProxyPort, "cổng proxy trên 127.0.0.1")
	web := flags.Bool("web", false, "mở mitmweb (xem từng request trên trình duyệt) thay vì mitmdump")
	dry := flags.Bool("dry-run", false, "chỉ in lệnh sẽ chạy")
	assumeYes := flags.Bool("yes", false, "tự cài mitmproxy nếu thiếu, không hỏi")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if flags.NArg() != 0 || *port < 1 || *port > 65535 {
		return errors.New("cách dùng: wj proxy start [-port 8877] [-web] [-yes] [-dry-run]")
	}
	tool := "mitmdump"
	if *web {
		tool = "mitmweb"
	}
	fmt.Fprintf(out, "Proxy: http://127.0.0.1:%d   Log: %s\n", *port, proxyLogPath(home))
	if *dry {
		fmt.Fprintln(out, "Dry run:", strings.Join(mitmCommand(tool, home, *port)[1:], " "))
		return nil
	}
	if portOpen(*port) {
		return fmt.Errorf("cổng %d đang bận (có thể proxy đã chạy rồi); dùng -port khác hoặc xem: wj proxy status", *port)
	}
	bin, err := ensureMitm(home, tool, *assumeYes, input, out)
	if err != nil {
		return err
	}
	if err := writeAddon(home); err != nil {
		return err
	}
	command := mitmCommand(bin, home, *port)
	portFlag := ""
	if *port != defaultProxyPort {
		portFlag = " -port " + strconv.Itoa(*port)
	}
	fmt.Fprintf(out, "Để cửa sổ này mở. Ở cửa sổ khác chạy: wj proxy run%s claude  (hoặc codex)\nNhấn Ctrl+C để tắt proxy.\n\n", portFlag)
	return runChild(command, input, out, os.Environ())
}

// proxyEnv routes one child process through the proxy and trusts its CA, without
// touching any configuration file or the user's persistent environment.
func proxyEnv(env []string, proxy, ca string) []string {
	var result []string
	for _, item := range env {
		name, _, _ := strings.Cut(item, "=")
		switch strings.ToUpper(name) {
		case "HTTPS_PROXY", "HTTP_PROXY", "ALL_PROXY", "NO_PROXY", "NODE_EXTRA_CA_CERTS", "CODEX_CA_CERTIFICATE":
			continue
		}
		result = append(result, item)
	}
	return append(result,
		"HTTPS_PROXY="+proxy, "HTTP_PROXY="+proxy, "https_proxy="+proxy, "http_proxy="+proxy,
		"NO_PROXY=localhost,127.0.0.1,::1", "no_proxy=localhost,127.0.0.1,::1",
		"NODE_EXTRA_CA_CERTS="+ca, "CODEX_CA_CERTIFICATE="+ca)
}

func proxyRun(args []string, input io.Reader, out io.Writer, home string) error {
	flags := flag.NewFlagSet("proxy run", flag.ContinueOnError)
	flags.SetOutput(out)
	port := flags.Int("port", defaultProxyPort, "cổng của proxy start")
	custom := flags.String("proxy", "", "URL mitmproxy riêng, ví dụ http://192.168.1.10:8080")
	ca := flags.String("ca", "", "chứng chỉ CA (.pem) của mitmproxy riêng; mặc định ~/.mitmproxy/mitmproxy-ca-cert.pem")
	dry := flags.Bool("dry-run", false, "chỉ in cách chạy")
	assumeYes := flags.Bool("yes", false, "tự cài mitmproxy nếu thiếu, không hỏi")
	project := flags.String("project", "", "tên project để tách số liệu; mặc định = tên thư mục đang đứng")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	rest := flags.Args()
	if len(rest) == 0 {
		return errors.New("cách dùng: wj proxy run [-project TÊN] claude|codex|opencode|LỆNH [tham số của app]")
	}
	proxy := *custom
	if proxy == "" {
		proxy = fmt.Sprintf("http://127.0.0.1:%d", *port)
	}
	u, err := url.Parse(proxy)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return errors.New("-proxy phải có dạng http://HOST:PORT")
	}
	if *project == "" {
		*project = currentProject()
	}
	// The project travels as the proxy URL user name, so every client that honours
	// HTTPS_PROXY (Claude, Codex, OpenCode, ...) tags its own traffic.
	routed := *u
	routed.User = url.UserPassword(*project, "x")
	tagged := routed.String()
	fmt.Fprintf(out, "Project: %s\n", *project)
	if *ca == "" {
		*ca = mitmCAPath(home)
	}
	binary := rest[0]
	if app, err := appByID(rest[0]); err == nil {
		binary = findApp(app, home)
	} else if path, err := exec.LookPath(rest[0]); err == nil {
		binary = path
	} else {
		binary = ""
	}
	fmt.Fprintf(out, "Chạy %s qua proxy %s (CA: %s)\n", rest[0], proxy, *ca)
	if *dry {
		return nil
	}
	if binary == "" {
		return fmt.Errorf("không tìm thấy %s; cài bằng: wj apps install %s", rest[0], rest[0])
	}
	if conn, err := net.DialTimeout("tcp", u.Host, 2*time.Second); err == nil {
		conn.Close()
	} else if *custom != "" {
		return fmt.Errorf("không kết nối được proxy %s; kiểm tra mitmproxy của bạn đang chạy", proxy)
	} else {
		// One-command mode: start a private mitmdump for this app and stop it afterwards.
		stop, err := startBackgroundProxy(home, *port, *assumeYes, input, out)
		if err != nil {
			return err
		}
		defer stop()
	}
	if _, err := os.Stat(*ca); err != nil {
		return fmt.Errorf("chưa có chứng chỉ %s; chạy: wj proxy setup", *ca)
	}
	err = runChild(append([]string{binary}, rest[1:]...), input, out, proxyEnv(os.Environ(), tagged, *ca))
	if *custom == "" {
		fmt.Fprintln(out, "Xem token đã dùng: wj proxy usage")
	}
	return err
}

// currentProject names the project after the git repository root (or the
// current folder), so runs from sub-folders of one repo count together.
func currentProject() string {
	dir, err := os.Getwd()
	if err != nil {
		return "khac"
	}
	for d := dir; ; {
		if _, err := os.Stat(filepath.Join(d, ".git")); err == nil {
			dir = d
			break
		}
		parent := filepath.Dir(d)
		if parent == d {
			break
		}
		d = parent
	}
	name := filepath.Base(dir)
	if name == "" || name == "." || name == string(filepath.Separator) {
		return "khac"
	}
	return name
}

func mitmCommand(bin, home string, port int) []string {
	return []string{bin, "--listen-host", "127.0.0.1", "-p", strconv.Itoa(port), "-s", filepath.Join(proxyDir(home), "usage_meter.py"),
		"--set", "usage_log=" + proxyLogPath(home), "--set", "flow_detail=0"}
}

func writeAddon(home string) error {
	if err := os.MkdirAll(proxyDir(home), 0o700); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(proxyDir(home), "usage_meter.py"), proxyAddon, 0o600)
}

func portOpen(port int) bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

func interactive(input io.Reader) bool {
	file, ok := input.(*os.File)
	return ok && term.IsTerminal(int(file.Fd()))
}

// ensureMitm returns the mitmproxy tool, installing mitmproxy first when it is
// missing. A terminal user is asked (Enter = yes); scripts must pass -yes.
func ensureMitm(home, tool string, assumeYes bool, input io.Reader, out io.Writer) (string, error) {
	if bin := findMitm(home, tool); bin != "" {
		return bin, nil
	}
	if !assumeYes {
		if !interactive(input) {
			return "", errors.New("chưa có mitmproxy; thêm -yes để tự cài, hoặc: wj apps install mitmproxy")
		}
		answer := menuAnswer(input, out, "Máy chưa có mitmproxy. Cài ngay? [Y/n]")
		if answer != "" && !yes(answer) {
			return "", errors.New("đã huỷ; cài sau bằng: wj apps install mitmproxy")
		}
	}
	fmt.Fprintln(out, "Đang cài mitmproxy...")
	if err := installApps([]string{"install", "mitmproxy"}, input, out); err != nil {
		return "", err
	}
	if bin := findMitm(home, tool); bin != "" {
		return bin, nil
	}
	return "", errors.New("đã cài mitmproxy nhưng chưa thấy lệnh; mở terminal mới rồi chạy lại")
}

// installApps is replaced in tests so they never install software.
var installApps = func(args []string, input io.Reader, out io.Writer) error {
	return newAppManager().run(args, input, out)
}

// startBackgroundProxy runs mitmdump quietly (output in mitmdump.log), waits
// until it listens and its CA exists, and returns a function that stops it.
func startBackgroundProxy(home string, port int, assumeYes bool, input io.Reader, out io.Writer) (func(), error) {
	bin, err := ensureMitm(home, "mitmdump", assumeYes, input, out)
	if err != nil {
		return nil, err
	}
	if err := writeAddon(home); err != nil {
		return nil, err
	}
	logPath := filepath.Join(proxyDir(home), "mitmdump.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, err
	}
	command := mitmCommand(bin, home, port)
	cmd := exec.Command(command[0], command[1:]...)
	cmd.Stdout, cmd.Stderr = logFile, logFile
	if err := cmd.Start(); err != nil {
		logFile.Close()
		return nil, fmt.Errorf("không bật được mitmproxy: %w", err)
	}
	exited := make(chan struct{})
	go func() { _ = cmd.Wait(); close(exited) }()
	stop := func() {
		_ = cmd.Process.Kill()
		<-exited
		logFile.Close()
	}
	deadline := time.Now().Add(30 * time.Second)
	for {
		if _, err := os.Stat(mitmCAPath(home)); err == nil && portOpen(port) {
			break
		}
		select {
		case <-exited:
			logFile.Close()
			return nil, fmt.Errorf("mitmproxy tắt ngay khi bật; xem %s", logPath)
		case <-time.After(300 * time.Millisecond):
		}
		if time.Now().After(deadline) {
			stop()
			return nil, fmt.Errorf("mitmproxy không sẵn sàng sau 30 giây; xem %s", logPath)
		}
	}
	fmt.Fprintf(out, "Đã bật proxy tạm ở cổng %d (tự tắt khi thoát app).\n", port)
	return stop, nil
}

// proxySetup installs mitmproxy and creates its CA once, for the step-by-step mode.
func proxySetup(args []string, input io.Reader, out io.Writer, home string) error {
	flags := flag.NewFlagSet("proxy setup", flag.ContinueOnError)
	flags.SetOutput(out)
	assumeYes := flags.Bool("yes", false, "cài mitmproxy không cần hỏi")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("cách dùng: wj proxy setup [-yes]")
	}
	if _, err := ensureMitm(home, "mitmdump", *assumeYes, input, out); err != nil {
		return err
	}
	if _, err := os.Stat(mitmCAPath(home)); err != nil {
		port := defaultProxyPort
		for portOpen(port) {
			port++
		}
		stop, err := startBackgroundProxy(home, port, true, input, out)
		if err != nil {
			return err
		}
		stop()
	} else if err := writeAddon(home); err != nil {
		return err
	}
	fmt.Fprintln(out, "Proxy đã sẵn sàng:")
	_ = proxyStatus(out, home)
	fmt.Fprintln(out, "Dùng: wj proxy run claude   (hoặc proxy start để chạy lâu)")
	return nil
}

type usageRecord struct {
	TS         string `json:"ts"`
	Project    string `json:"project"`
	Client     string `json:"client"`
	Model      string `json:"model"`
	Status     int    `json:"status"`
	Input      int64  `json:"input"`
	Output     int64  `json:"output"`
	CacheRead  int64  `json:"cache_read"`
	CacheWrite int64  `json:"cache_write"`
}

type usageTotal struct {
	Project, Client, Model                      string
	Calls, Failed                               int
	Input, Output, CacheRead, CacheWrite, Total int64
}

// usageFilter selects records and how they are grouped. GroupBy lists the
// dimensions shown as columns: any of "project", "client", "model".
type usageFilter struct {
	Since           time.Time
	Project, Client string
	GroupBy         []string
}

func summarizeUsage(r io.Reader, since time.Time) ([]usageTotal, usageTotal, error) {
	return summarizeUsageBy(r, usageFilter{Since: since, GroupBy: []string{"client", "model"}})
}

func summarizeUsageBy(r io.Reader, f usageFilter) ([]usageTotal, usageTotal, error) {
	groups := map[string]*usageTotal{}
	var all usageTotal
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64<<10), 1<<20)
	for scanner.Scan() {
		var rec usageRecord
		if json.Unmarshal(scanner.Bytes(), &rec) != nil {
			continue
		}
		if rec.Project == "" {
			rec.Project = "(không rõ)"
		}
		if !f.Since.IsZero() {
			ts, err := time.Parse(time.RFC3339, rec.TS)
			if err != nil || ts.Before(f.Since) {
				continue
			}
		}
		if f.Project != "" && !strings.EqualFold(rec.Project, f.Project) || f.Client != "" && !strings.EqualFold(rec.Client, f.Client) {
			continue
		}
		var row usageTotal
		for _, dim := range f.GroupBy {
			switch dim {
			case "project":
				row.Project = rec.Project
			case "client":
				row.Client = rec.Client
			case "model":
				row.Model = rec.Model
			}
		}
		key := row.Project + "\x00" + row.Client + "\x00" + row.Model
		g := groups[key]
		if g == nil {
			g = &row
			groups[key] = g
		}
		for _, t := range []*usageTotal{g, &all} {
			t.Calls++
			if rec.Status >= 400 || rec.Status == 0 {
				t.Failed++
			}
			t.Input += rec.Input
			t.Output += rec.Output
			t.CacheRead += rec.CacheRead
			t.CacheWrite += rec.CacheWrite
			t.Total += rec.Input + rec.Output + rec.CacheRead + rec.CacheWrite
		}
	}
	var list []usageTotal
	for _, g := range groups {
		list = append(list, *g)
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].Total != list[j].Total {
			return list[i].Total > list[j].Total
		}
		return list[i].Project+list[i].Client+list[i].Model < list[j].Project+list[j].Client+list[j].Model
	})
	return list, all, scanner.Err()
}

func groupDigits(n int64) string {
	s := strconv.FormatInt(n, 10)
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return s
}

func proxyUsage(args []string, out io.Writer, home string) error {
	flags := flag.NewFlagSet("proxy usage", flag.ContinueOnError)
	flags.SetOutput(out)
	window := flags.Duration("since", 24*time.Hour, "khoảng thời gian, ví dụ 1h, 24h, 168h (7 ngày)")
	all := flags.Bool("all", false, "toàn bộ log")
	file := flags.String("log", "", "đọc file log khác")
	by := flags.String("by", "app", "gom theo: app (app + model), project, project-app, model")
	project := flags.String("project", "", "chỉ tính một project")
	client := flags.String("app", "", "chỉ tính một app: claude, codex, opencode...")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("cách dùng: wj proxy usage [-since 24h | -all] [-by app|project|project-app|model] [-project TÊN] [-app TÊN]")
	}
	groupBy, ok := map[string][]string{
		"app":         {"client", "model"},
		"project":     {"project"},
		"project-app": {"project", "client"},
		"model":       {"model"},
	}[*by]
	if !ok {
		return errors.New("-by chỉ nhận: app, project, project-app, model")
	}
	path := *file
	if path == "" {
		path = proxyLogPath(home)
	}
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		fmt.Fprintln(out, "Chưa có dữ liệu. Chạy proxy start, rồi mở app bằng proxy run claude.")
		return nil
	}
	if err != nil {
		return err
	}
	defer f.Close()
	since := time.Now().Add(-*window)
	label := "trong " + window.String()
	if *all {
		since, label = time.Time{}, "toàn bộ"
	}
	list, total, err := summarizeUsageBy(f, usageFilter{Since: since, Project: *project, Client: *client, GroupBy: groupBy})
	if err != nil {
		return err
	}
	scope := "cả máy, mọi app"
	if *project != "" || *client != "" {
		scope = strings.TrimSpace(strings.Join([]string{prefixed("project ", *project), prefixed("app ", *client)}, " "))
	}
	fmt.Fprintf(out, "Token đã dùng (%s; %s) - %s\n\n", label, scope, path)
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
	fmt.Fprintln(w, strings.Join(head, "\t")+"\tLƯỢT\tLỖI\tVÀO\tRA\tĐỌC CACHE\tGHI CACHE\tTỔNG\t")
	row := func(t usageTotal, first string) {
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
		fmt.Fprintf(w, "%s\t%d\t%d\t%s\t%s\t%s\t%s\t%s\t\n", strings.Join(cells, "\t"), t.Calls, t.Failed,
			groupDigits(t.Input), groupDigits(t.Output), groupDigits(t.CacheRead), groupDigits(t.CacheWrite), groupDigits(t.Total))
	}
	for _, t := range list {
		row(t, "")
	}
	row(total, "TỔNG")
	if err := w.Flush(); err != nil {
		return err
	}
	fmt.Fprintln(out, "\nVÀO = token prompt mới; ĐỌC CACHE thường rẻ hơn nhiều so với VÀO. Giá thật do gateway quyết định.")
	return nil
}

func prefixed(prefix, value string) string {
	if value == "" {
		return ""
	}
	return prefix + value
}

func proxyStatus(out io.Writer, home string) error {
	check := func(ok bool, good, bad string) {
		mark, text := "[ ]", bad
		if ok {
			mark, text = "[x]", good
		}
		fmt.Fprintln(out, " ", mark, text)
	}
	bin := findMitm(home, "mitmdump")
	check(bin != "", "mitmproxy: "+bin, "mitmproxy chưa cài -> wj proxy setup (hoặc cứ chạy proxy run claude, tool tự cài)")
	_, err := os.Stat(mitmCAPath(home))
	check(err == nil, "chứng chỉ CA: "+mitmCAPath(home), "chưa có chứng chỉ CA -> wj proxy setup")
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", defaultProxyPort), time.Second)
	if conn != nil {
		conn.Close()
	}
	check(err == nil, fmt.Sprintf("proxy đang chạy ở 127.0.0.1:%d", defaultProxyPort), fmt.Sprintf("proxy chưa chạy ở cổng %d -> wj proxy start", defaultProxyPort))
	info, err := os.Stat(proxyLogPath(home))
	check(err == nil, fmt.Sprintf("log: %s (%d KB)", proxyLogPath(home), sizeKB(info)), "chưa có log -> mở app bằng proxy run claude")
	return nil
}

func sizeKB(info os.FileInfo) int64 {
	if info == nil {
		return 0
	}
	return (info.Size() + 1023) / 1024
}

// proxyMenu is the interactive entry; starting the proxy opens its own window on
// Windows so the menu stays usable and Ctrl+C there does not close the menu.
func proxyMenu(input io.Reader, out io.Writer) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	_ = proxyStatus(out, home)
	fmt.Fprintln(out, "\n  1. Mở Claude qua proxy (tự cài + tự bật proxy)\n  2. Mở Codex qua proxy (tự cài + tự bật proxy)\n  3. Xem token đã dùng (24 giờ)\n  4. Xem token đã dùng (7 ngày)\n  5. Cài mitmproxy + tạo chứng chỉ\n  6. Bật proxy chạy lâu (cửa sổ riêng)\n  7. Hướng dẫn\n  8. Bật đo cho project hiện tại (Claude)\n  9. Xem project đang bật đo\n  0. Quay lại")
	switch menuAnswer(input, out, "Chọn [0-9]") {
	case "1":
		return proxyRun([]string{"claude"}, input, out, home)
	case "2":
		return proxyRun([]string{"codex"}, input, out, home)
	case "3":
		return proxyUsage(nil, out, home)
	case "4":
		return proxyUsage([]string{"-since", "168h"}, out, home)
	case "5":
		return proxySetup(nil, input, out, home)
	case "6":
		if _, err := ensureMitm(home, "mitmdump", false, input, out); err != nil {
			return err
		}
		if runtime.GOOS != "windows" {
			fmt.Fprintln(out, "Mở terminal khác và chạy: wj proxy start")
			return nil
		}
		self, err := os.Executable()
		if err != nil {
			return err
		}
		fmt.Fprintln(out, "Đã mở cửa sổ proxy riêng. Đóng cửa sổ đó để tắt proxy.")
		return exec.Command("cmd.exe", "/c", "start", "wj proxy", self, "proxy", "start").Run()
	case "7":
		fmt.Fprintln(out, proxyHelp)
	case "8":
		return projectOn(nil, out, home)
	case "9":
		return projectList(out, home)
	}
	return nil
}
