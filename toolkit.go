package main

import (
	"embed"
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

//go:embed toolkit/windows/*.ps1 toolkit/unix/*.sh
var bundledToolkit embed.FS

func runApp(args []string, input io.Reader, out io.Writer) error {
	if len(args) == 0 {
		return mainMenu(input, out)
	}
	switch args[0] {
	case "apps", "status":
		return newAppManager().run(args[1:], input, out)
	case "configure":
		return configureClient(args[1:], input, out)
	case "update":
		return updateSelf(args[1:], out)
	case "version", "--version", "-version":
		fmt.Fprintln(out, "wj", strings.TrimSpace(buildVersion))
		return nil
	case "auth":
		return runCLI(args[1:], input, out)
	case "tools":
		return runToolkit(args[1:], input, out)
	case "proxy":
		return runProxy(args[1:], input, out)
	case "check":
		return runCheck(args[1:], out)
	case "init":
		return runInit(args[1:], input, out)
	case "usage":
		return runUsage(args[1:], out)
	case "top":
		return runTop(args[1:], out)
	case "overlay":
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		return runOverlay(args[1:], out, home)
	case "models":
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		return runModels(args[1:], input, out, home)
	case "onboard":
		if len(args) != 1 {
			return errors.New("onboard chỉ chạy dạng hỏi-đáp, không nhận tham số; dùng tools hoặc auth nếu muốn chạy bằng lệnh")
		}
		return onboard(input, out)
	case "help", "-h", "--help", "-help", "/?":
		return printHelp(args[1:], input, out)
	default:
		return runCLI(args, input, out)
	}
}

// Do not buffer ahead across menu/child-process boundaries: the child owns stdin.
func menuAnswer(input io.Reader, out io.Writer, label string) string {
	fmt.Fprint(out, label+": ")
	var line []byte
	var b [1]byte
	for {
		n, err := input.Read(b[:])
		if n > 0 {
			if b[0] == '\n' {
				break
			}
			line = append(line, b[0])
		}
		if err != nil {
			break
		}
	}
	return strings.TrimSpace(string(line))
}

const mainMenuText = `
Wisjoy wj %s

 CÀI ĐẶT
   1. Cài máy mới (hỏi từng bước, chọn cái mình cần)
   2. Cài ứng dụng còn thiếu (Claude, Codex, Orca, mitmproxy...)
   3. Cài bộ công cụ dòng lệnh (terminal, git, node, python...)
   4. Cài quản lý cửa sổ (không bắt buộc)
  14. Setup project: rule chung + rule theo job (Unity, web...), skill Orca
 KẾT NỐI / ĐĂNG NHẬP
   5. Kết nối Claude Code (gateway, token OAuth, API key, đăng nhập web)
   6. Kết nối Codex / OpenCode / Cursor / Antigravity / Orca
   7. Đổi kết nối Claude đã lưu (profile)
 KIỂM TRA
   8. Xem ứng dụng đã cài
  15. Xem token mọi project, cập nhật liên tục (wj top)
  16. Bật overlay token luôn nổi trên màn hình (wj overlay)
   9. Đo token qua mitmproxy (cho app khác Claude/Codex)
  13. Xem các agent có khớp cấu hình không (wj check)
  17. Lấy lại danh sách model từ gateway (wj models)
 CẬP NHẬT / GỠ
  10. Cập nhật ứng dụng
  11. Cập nhật wj
  12. Gỡ bộ công cụ / quản lý cửa sổ

   h. Hướng dẫn nhanh      0. Thoát
`

const menuPrompt = "Chọn [0-17, h]"

func mainMenu(input io.Reader, out io.Writer) error {
	for {
		fmt.Fprintf(out, mainMenuText, strings.TrimSpace(buildVersion))
		var err error
		switch strings.ToLower(menuAnswer(input, out, menuPrompt)) {
		case "", "0", "q":
			return nil
		case "1":
			err = onboard(input, out)
		case "2":
			err = appMenu("install", input, out)
		case "3":
			err = runToolkit([]string{"install", "cli"}, input, out)
		case "4":
			err = runToolkit([]string{"install", "wm"}, input, out)
		case "5":
			err = configureClient([]string{"claude"}, input, out)
		case "6":
			err = configureClient(nil, input, out)
		case "7":
			err = pickProfile(input, out)
		case "8":
			err = newAppManager().run(nil, input, out)
		case "9":
			err = proxyMenu(input, out)
		case "10":
			err = appMenu("update", input, out)
		case "11":
			if yes(menuAnswer(input, out, "Tải bản wj mới nhất? [y/N]")) {
				err = updateSelf(nil, out)
			}
		case "12":
			switch menuAnswer(input, out, "Gỡ gì? 1 = bộ công cụ dòng lệnh, 2 = quản lý cửa sổ, Enter = quay lại") {
			case "1":
				err = runToolkit([]string{"uninstall", "cli"}, input, out)
			case "2":
				err = runToolkit([]string{"uninstall", "wm"}, input, out)
			}
		case "13":
			err = runCheck(nil, out)
		case "14":
			err = initMenu(input, out)
		case "15":
			err = runTop(nil, out)
		case "16":
			err = runApp([]string{"overlay"}, input, out)
		case "17":
			err = runApp([]string{"models"}, input, out)
		case "h", "?", "help":
			err = printHelp(nil, input, out)
		default:
			fmt.Fprintln(out, "Không có mục này. Gõ số từ 0 đến 17, hoặc h để xem hướng dẫn.")
		}
		if err != nil {
			fmt.Fprintln(out, "Lỗi:", err)
		}
	}
}

// yes accepts y/yes and the Vietnamese c/có.
func yes(answer string) bool {
	return contains([]string{"y", "yes", "c", "có", "co"}, strings.ToLower(strings.TrimSpace(answer)))
}

func appMenu(action string, input io.Reader, out io.Writer) error {
	m := newAppManager()
	if err := m.run(nil, input, out); err != nil {
		return err
	}
	answer := menuAnswer(input, out, "Gõ số ứng dụng, nhiều số cách nhau dấu cách (vd 1 2 8, 1-3), all = tất cả, Enter = quay lại")
	if answer == "" {
		return nil
	}
	ids, err := parseChoices(answer, appIDs())
	if err != nil {
		return err
	}
	return m.run([]string{action, strings.Join(ids, ",")}, input, out)
}

func appIDs() []string {
	var ids []string
	for _, app := range appCatalog {
		ids = append(ids, app.ID)
	}
	return ids
}

func onboard(input io.Reader, out io.Writer) error {
	fmt.Fprintln(out, "Cài máy mới: tool hỏi từng phần, phần nào không cần thì Enter để bỏ qua.")
	fmt.Fprintln(out, "\nBước 1/4 - Bộ công cụ dòng lệnh (terminal đẹp, git, node, python, ripgrep...)")
	if yes(menuAnswer(input, out, "Cài không? [y/N]")) {
		if err := runToolkit([]string{"install", "cli"}, input, out); err != nil {
			return err
		}
	}
	fmt.Fprintln(out, "\nBước 2/4 - Ứng dụng lập trình")
	if err := appMenu("install", input, out); err != nil {
		fmt.Fprintln(out, "Có ứng dụng cài chưa xong:", err)
	}
	fmt.Fprintln(out, "\nBước 3/4 - Quản lý cửa sổ kiểu i3 (không bắt buộc, đổi phím tắt và cách xếp cửa sổ)")
	if yes(menuAnswer(input, out, "Cài không? [y/N]")) {
		if err := runToolkit([]string{"install", "wm"}, input, out); err != nil {
			return err
		}
	}
	fmt.Fprintln(out, "\nBước 4/4 - Kết nối / đăng nhập ứng dụng")
	for {
		if !yes(menuAnswer(input, out, "Kết nối một ứng dụng? [y/N]")) {
			fmt.Fprintln(out, "Xong. Mở terminal mới để nhận PATH mới. Gõ wj help khi cần.")
			return nil
		}
		if err := configureClient(nil, input, out); err != nil {
			fmt.Fprintln(out, "Lỗi kết nối:", err)
		}
	}
}

func toolkitScript(platform, action, module string) (string, error) {
	if action != "install" && action != "uninstall" {
		return "", errors.New("chỉ có install hoặc uninstall")
	}
	if module != "cli" && module != "wm" {
		return "", errors.New("chỉ có cli (công cụ dòng lệnh) hoặc wm (quản lý cửa sổ)")
	}
	switch platform {
	case "windows":
		return "toolkit/windows/" + action + "-" + module + ".ps1", nil
	case "darwin", "linux":
		return "toolkit/unix/" + action + "-" + module + ".sh", nil
	default:
		return "", fmt.Errorf("toolkit is not supported on %s", platform)
	}
}

func toolkitList(out io.Writer) {
	fmt.Fprintln(out, "Bundled tools from Bo_cong_cu:\n  cli group 1: terminal, Starship, Nerd Font, fzf, zoxide; PowerShell/Clink on Windows\n  cli group 2: ripgrep, fd, bat, eza, jq, lazygit, delta, btop, GitHub CLI\n  cli group 3: Git, fnm/Node, uv/Python, Neovim, Antigravity IDE; optional Docker\n  cli group 4: Claude Code, Codex CLI, Antigravity CLI\n  wm Windows: Komorebi + whkd, i3-style keys, monitor workspaces and bars\n  wm macOS: AeroSpace and i3-style keys\n  wm Linux: configure an existing i3/Hyprland installation\n\nBoth modules include uninstall. Package availability depends on your OS.\nUse -dry-run to preview an action without extracting or executing scripts.")
}

func runToolkit(args []string, input io.Reader, out io.Writer) error {
	if len(args) == 0 || args[0] == "list" || args[0] == "-help" {
		toolkitList(out)
		return nil
	}
	if len(args) < 2 {
		return errors.New("cách dùng: wj tools install|uninstall cli|wm [-groups 1,2,4] [-dry-run]; xem: wj help tools")
	}
	action, module := args[0], args[1]
	script, err := toolkitScript(runtime.GOOS, action, module)
	if err != nil {
		return err
	}
	flags := flag.NewFlagSet("tools", flag.ContinueOnError)
	flags.SetOutput(out)
	groups := flags.String("groups", "", "CLI package groups, comma-separated: 1 terminal, 2 utilities, 3 development, 4 agents")
	dry := flags.Bool("dry-run", false, "show the bundled action without running it")
	if err := flags.Parse(args[2:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("có tham số không hiểu; xem: wj help tools")
	}
	selected := splitList(*groups)
	if len(selected) > 0 && module != "cli" {
		return errors.New("-groups chỉ dùng với cli")
	}
	for _, group := range selected {
		if !contains([]string{"1", "2", "3", "4"}, group) {
			return errors.New("-groups chỉ nhận các số 1,2,3,4 cách dấu phẩy")
		}
	}
	fmt.Fprintf(out, "%s %s on %s using bundled %s\n", action, module, runtime.GOOS, script)
	if module == "cli" {
		fmt.Fprintln(out, "Updates the toolkit's marked shell-profile block, command aliases, terminal styling and Git defaults.")
	} else {
		fmt.Fprintln(out, "Changes window-manager configuration, shortcuts, workspace layout and desktop/taskbar integration.")
	}
	if len(selected) > 0 {
		fmt.Fprintln(out, "Selected CLI groups:", strings.Join(selected, ", "))
	}
	if *dry {
		fmt.Fprintln(out, "Dry run: no extraction, package installs, configuration changes or processes.")
		return nil
	}
	if !yes(menuAnswer(input, out, "Tiếp tục? [y/N]")) {
		fmt.Fprintln(out, "Đã huỷ.")
		return nil
	}
	data, err := bundledToolkit.ReadFile(script)
	if err != nil {
		return err
	}
	dir, err := os.MkdirTemp("", "wisjoy-toolkit-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, filepath.Base(script))
	if runtime.GOOS == "windows" {
		data = append([]byte{0xef, 0xbb, 0xbf}, data...)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return err
	}
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("powershell.exe", "-NoProfile", "-ExecutionPolicy", "Bypass", "-File", path)
	} else {
		cmd = exec.Command("bash", path)
	}
	cmd.Env = cleanClaudeEnvironment(os.Environ())
	// The CLI flags own group selection; do not silently inherit an unattended selection.
	var env []string
	for _, value := range cmd.Env {
		if !strings.HasPrefix(strings.ToUpper(value), "WJ_") {
			env = append(env, value)
		}
	}
	cmd.Env = append(env, "WJ_GROUPS="+strings.Join(selected, " "))
	cmd.Stdin, cmd.Stdout, cmd.Stderr = input, out, out
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %s did not complete; review package results above: %w", action, module, err)
	}
	refreshProcessPath()
	fmt.Fprintln(out, "Xong. Mở terminal mới để nhận PATH/profile mới.")
	return nil
}
