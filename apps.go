package main

import (
	_ "embed"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

//go:embed VERSION
var buildVersion string

type appSpec struct {
	ID, Name, Command, WindowsScript, UnixScript, Winget, Cask string
}

var appCatalog = []appSpec{
	{"claude", "Claude Code", "claude", "https://claude.ai/install.ps1", "https://claude.ai/install.sh", "", ""},
	{"codex", "Codex CLI", "codex", "https://chatgpt.com/codex/install.ps1", "https://chatgpt.com/codex/install.sh", "", ""},
	{"opencode", "OpenCode", "opencode", "", "https://opencode.ai/install", "SST.opencode", ""},
	{"cursor-agent", "Cursor CLI", "agent", "https://cursor.com/install?win32=true", "https://cursor.com/install", "", ""},
	{"cursor", "Cursor editor", "cursor", "", "", "Anysphere.Cursor", "cursor"},
	{"antigravity", "Antigravity CLI", "agy", "https://antigravity.google/cli/install.ps1", "https://antigravity.google/cli/install.sh", "", ""},
	{"antigravity-ide", "Antigravity IDE", "antigravity-ide", "", "", "Google.AntigravityIDE", "antigravity-ide"},
	{"orca", "Orca", "orca", "", "", "", ""},
	{"mitmproxy", "mitmproxy", "mitmdump", "", "", "mitmproxy.mitmproxy", ""},
}

func appByID(id string) (appSpec, error) {
	if id == "agy" {
		id = "antigravity"
	}
	for _, app := range appCatalog {
		if app.ID == id {
			return app, nil
		}
	}
	return appSpec{}, fmt.Errorf("không có app %q; xem tên app: setup-gateway apps list", id)
}

func appCandidates(app appSpec, home, platform string) []string {
	name := app.Command
	if app.ID == "orca" && platform == "linux" {
		name = "orca-ide"
	}
	var paths []string
	for _, dir := range []string{filepath.Join(home, ".local", "bin"), filepath.Join(home, ".opencode", "bin"), filepath.Join(home, ".agy", "bin"), "/opt/homebrew/bin", "/usr/local/bin"} {
		if platform == "windows" {
			for _, ext := range []string{".exe", ".cmd"} {
				paths = append(paths, filepath.Join(dir, name+ext))
			}
		} else {
			paths = append(paths, filepath.Join(dir, name))
		}
	}
	if platform == "windows" {
		local := os.Getenv("LOCALAPPDATA")
		if local == "" {
			local = filepath.Join(home, "AppData", "Local")
		}
		for _, suffix := range map[string][]string{
			"codex":           {"Programs/OpenAI/Codex/bin/codex.exe"},
			"antigravity":     {"agy/bin/agy.exe"},
			"cursor-agent":    {"cursor-agent/agent.cmd", "cursor-agent/agent.exe"},
			"cursor":          {"Programs/cursor/Cursor.exe"},
			"antigravity-ide": {"Programs/Antigravity IDE/Antigravity IDE.exe"},
			"orca":            {"Programs/orca/Orca.exe"},
		}[app.ID] {
			paths = append(paths, filepath.Join(local, filepath.FromSlash(suffix)))
		}
		if app.ID == "mitmproxy" {
			programs := os.Getenv("ProgramFiles")
			if programs == "" {
				programs = `C:\Program Files`
			}
			paths = append(paths, filepath.Join(programs, "mitmproxy", "bin", "mitmdump.exe"))
		}
		paths = append(paths, filepath.Join(local, "Microsoft", "WinGet", "Links", name+".exe"), filepath.Join(home, "AppData", "Roaming", "npm", name+".cmd"))
	}
	if platform == "darwin" {
		for _, dir := range []string{filepath.Join(home, "Applications"), "/Applications"} {
			bundle := map[string]string{"cursor": "Cursor", "antigravity-ide": "Antigravity IDE", "orca": "Orca"}[app.ID]
			if bundle != "" {
				paths = append(paths, filepath.Join(dir, bundle+".app", "Contents", "MacOS", bundle))
			}
		}
	}
	return paths
}

func findApp(app appSpec, home string) string {
	// On Linux, bare orca is the GNOME screen reader, not the coding app.
	name := app.Command
	if app.ID == "orca" && runtime.GOOS == "linux" {
		name = "orca-ide"
	}
	if path, err := exec.LookPath(name); err == nil {
		return path
	}
	for _, path := range appCandidates(app, home, runtime.GOOS) {
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path
		}
	}
	return ""
}

type installPlan struct {
	URL, Repo, Asset string
	Command          []string
}

func planApp(app appSpec, platform, arch string, update bool) (installPlan, error) {
	if arch != "amd64" && arch != "arm64" {
		return installPlan{}, fmt.Errorf("unsupported architecture: %s", arch)
	}
	if app.ID == "orca" {
		asset := "orca-windows-setup.exe"
		switch platform {
		case "windows":
		case "darwin":
			asset = "-mac.zip"
			if arch == "arm64" {
				asset = "-arm64-mac.zip"
			}
		case "linux":
			asset = "orca-linux.AppImage"
			if arch == "arm64" {
				asset = "orca-linux-arm64.AppImage"
			}
		default:
			return installPlan{}, errors.New("unsupported operating system")
		}
		return installPlan{Repo: "stablyai/orca", Asset: asset}, nil
	}
	if app.ID == "mitmproxy" && platform != "windows" {
		action := "install"
		if update {
			action = "upgrade"
		}
		if platform == "darwin" {
			return installPlan{Command: []string{"brew", action, "mitmproxy"}}, nil
		}
		if platform == "linux" {
			return installPlan{Command: []string{"uv", "tool", action, "mitmproxy"}}, nil
		}
	}
	if platform == "windows" {
		if app.WindowsScript != "" {
			return installPlan{URL: app.WindowsScript}, nil
		}
		if app.Winget != "" {
			action := "install"
			if update {
				action = "upgrade"
			}
			return installPlan{Command: []string{"winget", action, "--id", app.Winget, "--exact", "--source", "winget", "--accept-source-agreements", "--accept-package-agreements", "--disable-interactivity"}}, nil
		}
	}
	if platform == "darwin" || platform == "linux" {
		if app.UnixScript != "" {
			return installPlan{URL: app.UnixScript}, nil
		}
		if platform == "darwin" && app.Cask != "" {
			action := "install"
			if update {
				action = "upgrade"
			}
			return installPlan{Command: []string{"brew", action, "--cask", app.Cask}}, nil
		}
	}
	return installPlan{}, fmt.Errorf("%s chưa cài tự động được trên %s; tải từ trang của hãng (xem HUONG-DAN.md)", app.Name, platform)
}

type appManager struct {
	home    string
	find    func(appSpec, string) string
	install func(appSpec, installPlan, io.Reader, io.Writer) error
}

func newAppManager() appManager {
	home, _ := os.UserHomeDir()
	return appManager{home: home, find: findApp, install: executeAppInstall}
}

func (m appManager) run(args []string, input io.Reader, out io.Writer) error {
	if len(args) == 0 || args[0] == "list" || args[0] == "status" {
		if len(args) > 1 {
			return errors.New("apps list không nhận thêm tham số")
		}
		fmt.Fprintln(out, "Ứng dụng trên máy (chỉ kiểm tra tại máy, không tải gì):")
		for _, app := range appCatalog {
			state := "chưa cài"
			if p := m.find(app, m.home); p != "" {
				state = "đã cài: " + p
			}
			fmt.Fprintf(out, "  %-17s %-19s %s\n", app.ID, app.Name, state)
		}
		return nil
	}
	if len(args) < 2 || (args[0] != "install" && args[0] != "update") {
		return errors.New("cách dùng: apps list | apps install TÊN,TÊN | apps update TÊN,TÊN [-dry-run]; TÊN có thể là all. Xem: setup-gateway help apps")
	}
	flags := flag.NewFlagSet("apps", flag.ContinueOnError)
	flags.SetOutput(out)
	dry := flags.Bool("dry-run", false, "preview without downloads or changes")
	if err := flags.Parse(args[2:]); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("có tham số không hiểu; xem: setup-gateway help apps")
	}
	ids := splitList(args[1])
	if args[1] == "all" {
		ids = nil
		for _, app := range appCatalog {
			ids = append(ids, app.ID)
		}
	}
	if len(ids) == 0 {
		return errors.New("chưa chọn app nào")
	}
	var apps []appSpec
	for _, id := range ids {
		app, err := appByID(id)
		if err != nil {
			return err
		}
		apps = append(apps, app)
	}
	var failures []error
	for _, app := range apps {
		path := m.find(app, m.home)
		if path != "" && args[0] == "install" {
			fmt.Fprintf(out, "SKIP %s: đã có sẵn (%s) - muốn lên bản mới dùng apps update\n", app.Name, path)
			continue
		}
		plan, err := planApp(app, runtime.GOOS, runtime.GOARCH, args[0] == "update" && path != "")
		if err == nil {
			source := plan.URL
			if plan.Repo != "" {
				source = plan.Repo + " / " + plan.Asset
			}
			if len(plan.Command) > 0 {
				source = strings.Join(plan.Command, " ")
			}
			fmt.Fprintf(out, "%s %s: %s\n", args[0], app.Name, source)
			if !*dry {
				err = m.install(app, plan, input, out)
			}
		}
		if err != nil {
			fmt.Fprintf(out, "LỖI %s: %v\n", app.Name, err)
			failures = append(failures, fmt.Errorf("%s: %w", app.ID, err))
		}
	}
	if *dry {
		fmt.Fprintln(out, "Dry run: chỉ xem trước, không tải, không cài.")
	}
	return errors.Join(failures...)
}

func runChild(command []string, input io.Reader, out io.Writer, env []string) error {
	if len(command) == 0 {
		return errors.New("empty command")
	}
	if runtime.GOOS == "windows" && (strings.EqualFold(filepath.Ext(command[0]), ".cmd") || strings.EqualFold(filepath.Ext(command[0]), ".bat")) {
		f, err := os.CreateTemp("", "setup-launch-*.ps1")
		if err != nil {
			return err
		}
		defer os.Remove(f.Name())
		_, err = f.WriteString("param([string]$Executable)\n& $Executable @args\nexit $LASTEXITCODE\n")
		closeErr := f.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
		command = append([]string{"powershell.exe", "-NoProfile", "-ExecutionPolicy", "Bypass", "-File", f.Name(), command[0]}, command[1:]...)
	}
	cmd := exec.Command(command[0], command[1:]...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = input, out, out
	cmd.Env = env
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s failed: %w", filepath.Base(command[0]), err)
	}
	return nil
}

func executeAppInstall(app appSpec, plan installPlan, input io.Reader, out io.Writer) error {
	defer refreshProcessPath()
	if plan.Repo != "" {
		return installOrca(plan, input, out)
	}
	if len(plan.Command) > 0 {
		return runChild(plan.Command, input, out, cleanClaudeEnvironment(os.Environ()))
	}
	data, err := fetchBytes(plan.URL, 4<<20)
	if err != nil {
		return err
	}
	suffix := ".sh"
	if runtime.GOOS == "windows" {
		suffix = ".ps1"
	}
	f, err := os.CreateTemp("", "setup-"+app.ID+"-*"+suffix)
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	command := []string{"bash", f.Name()}
	if runtime.GOOS == "windows" {
		command = []string{"powershell.exe", "-NoProfile", "-ExecutionPolicy", "Bypass", "-File", f.Name()}
	}
	if err = runChild(command, input, out, cleanClaudeEnvironment(os.Environ())); err != nil {
		return err
	}
	fmt.Fprintf(out, "Đã cài xong %s. Nếu gõ lệnh chưa thấy, mở terminal mới.\n", app.Name)
	return nil
}
